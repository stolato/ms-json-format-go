package main

import (
	"api-go/internal/auth"
	"api-go/internal/config"
	"api-go/internal/database"
	"api-go/internal/encryption"
	"api-go/internal/mailer"
	"api-go/internal/middleware"
	"api-go/internal/repositories"
	"api-go/internal/routes"
	"api-go/internal/socket"
	"api-go/internal/tasks"
	"context"
	"errors"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"go.mongodb.org/mongo-driver/mongo"
)

// fiberConfig define de onde vem o IP do cliente (usado pelo rate limit).
//
//	PROXY_HEADER=X-Real-IP              header com o IP real, definido pelo proxy
//	TRUSTED_PROXIES=10.0.0.0/8,1.2.3.4  IPs/CIDRs dos proxies confiáveis
//
// O header só é aceito quando a conexão vem de um proxy confiável; caso
// contrário qualquer cliente poderia forjá-lo para burlar o rate limit.
// Prefira um header que o proxy SOBRESCREVE (X-Real-IP, CF-Connecting-IP):
// com X-Forwarded-For o Fiber usa o primeiro IP da lista, que o cliente controla.
func fiberConfig(cfg config.Config) fiber.Config {
	fc := fiber.Config{
		// Evita conexões lentas/ociosas presas indefinidamente (slowloris).
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	// config.Load já garante TRUSTED_PROXIES quando PROXY_HEADER está definido.
	if cfg.ProxyHeader == "" {
		return fc
	}
	if strings.EqualFold(cfg.ProxyHeader, fiber.HeaderXForwardedFor) {
		slog.Warn("PROXY_HEADER=X-Forwarded-For: o primeiro IP da lista é controlado pelo cliente; prefira X-Real-IP definido pelo proxy")
	}
	fc.ProxyHeader = cfg.ProxyHeader
	fc.TrustProxy = true
	fc.TrustProxyConfig = fiber.TrustProxyConfig{Proxies: cfg.TrustedProxies}
	return fc
}

const (
	// shutdownTimeout limita o encerramento inteiro. Precisa ser menor que o
	// prazo entre SIGTERM e SIGKILL do orquestrador (Docker: 10s por padrão).
	shutdownTimeout = 8 * time.Second
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config.LoadDotEnv()
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := encryption.Init(cfg.EncryptionKey); err != nil {
		return err
	}
	if err := auth.Init(cfg.JWTSecret); err != nil {
		return err
	}
	if err := mailer.Init(cfg.Mail); err != nil {
		return err
	}

	// Ctrl+C / SIGTERM iniciam o encerramento. Um segundo sinal mata na hora
	// (o stop() abaixo devolve o comportamento padrão).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	client, err := database.Connect(ctx, cfg.MongoURI)
	if err != nil {
		return err
	}
	repos := repositories.New(client.Database(cfg.MongoDatabase), repositories.Options{
		LegacyIDLinks: cfg.LegacyIDLinks,
	})
	if err := repos.EnsureIndexes(); err != nil {
		log.Println("falha ao criar índices:", err)
	}
	authMW := middleware.NewAuth(repos.Sessions, repos.Users)

	app := fiber.New(fiberConfig(cfg))
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
	}))
	app.Use(middleware.GlobalRateLimit)
	routes.SetupRoutes(app, repos, authMW, cfg.AppURL)

	scheduler, err := tasks.Start(repos.Items)
	if err != nil {
		return err
	}
	sock := socket.Start(":"+cfg.SocketPort, repos.Items, repos.Organizations, authMW.ValidateToken, cfg.SocketCORSOrigins)

	httpErr := make(chan error, 1)
	go func() {
		httpErr <- app.Listen(":" + cfg.Port)
	}()

	// Espera um sinal ou a queda de um dos servidores.
	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("sinal recebido, encerrando")
	case err := <-httpErr:
		runErr = err
	case err := <-sock.Err():
		runErr = err
	}
	stop()

	shutdown(app, sock, scheduler.Stop(), client)
	return runErr
}

// shutdown encerra tudo em ordem, dentro de shutdownTimeout:
//  1. HTTP: para de aceitar conexões e espera as requisições em andamento;
//  2. websocket: desconecta os clientes;
//  3. cron: espera a limpeza em andamento, se houver;
//  4. emails assíncronos pendentes (disparados pelas requisições do passo 1);
//  5. Mongo: por último, porque os passos anteriores ainda podem usá-lo.
func shutdown(app *fiber.App, sock *socket.Server, cronDone context.Context, client *mongo.Client) {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := app.ShutdownWithContext(ctx); err != nil && !errors.Is(err, fiber.ErrNotRunning) {
		slog.Error("shutdown: http", "error", err)
	}
	if err := sock.Shutdown(ctx); err != nil {
		slog.Error("shutdown: websocket", "error", err)
	}
	select {
	case <-cronDone.Done():
	case <-ctx.Done():
		slog.Error("shutdown: limpeza agendada não terminou a tempo")
	}
	if err := mailer.Wait(ctx); err != nil {
		slog.Error("shutdown", "error", err)
	}
	if err := client.Disconnect(ctx); err != nil {
		slog.Error("shutdown: mongo", "error", err)
	}
	slog.Info("encerrado")
}
