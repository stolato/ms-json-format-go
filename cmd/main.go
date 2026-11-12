package main

import (
	"api-go/internal/auth"
	"api-go/internal/database"
	"api-go/internal/encryption"
	"api-go/internal/mailer"
	"api-go/internal/middleware"
	"api-go/internal/routes"
	"api-go/internal/socket"
	"api-go/internal/tasks"
	"api-go/internal/telemetry"
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/logger"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func loadEnv() {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)

	for _, path := range []string{".env", filepath.Join(exeDir, "../.env")} {
		if err := godotenv.Overload(path); err == nil {
			return
		}
	}
}

// fiberConfig define de onde vem o IP do cliente (usado pelo rate limit).
//
//	PROXY_HEADER=X-Real-IP              header com o IP real, definido pelo proxy
//	TRUSTED_PROXIES=10.0.0.0/8,1.2.3.4  IPs/CIDRs dos proxies confiáveis
//
// O header só é aceito quando a conexão vem de um proxy confiável; caso
// contrário qualquer cliente poderia forjá-lo para burlar o rate limit.
// Prefira um header que o proxy SOBRESCREVE (X-Real-IP, CF-Connecting-IP):
// com X-Forwarded-For o Fiber usa o primeiro IP da lista, que o cliente controla.
func fiberConfig() fiber.Config {
	cfg := fiber.Config{
		// Evita conexões lentas/ociosas presas indefinidamente (slowloris).
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	header := strings.TrimSpace(os.Getenv("PROXY_HEADER"))
	if header == "" {
		return cfg
	}
	var proxies []string
	for _, p := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	if len(proxies) == 0 {
		log.Fatal("PROXY_HEADER definido sem TRUSTED_PROXIES: o header poderia ser forjado por qualquer cliente")
	}
	if strings.EqualFold(header, fiber.HeaderXForwardedFor) {
		slog.Warn("PROXY_HEADER=X-Forwarded-For: o primeiro IP da lista é controlado pelo cliente; prefira X-Real-IP definido pelo proxy")
	}
	cfg.ProxyHeader = header
	cfg.TrustProxy = true
	cfg.TrustProxyConfig = fiber.TrustProxyConfig{Proxies: proxies}
	return cfg
}

// startMetricsServer expõe /metrics (Prometheus) em METRICS_ADDR.
// Padrão 127.0.0.1:9091: só acessível de dentro do próprio host/container.
// Em docker-compose de dev use METRICS_ADDR=:9091; METRICS_ADDR=off desliga.
func startMetricsServer() {
	addr := os.Getenv("METRICS_ADDR")
	if addr == "" {
		addr = "127.0.0.1:9091"
	}
	if addr == "off" {
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			slog.Error("metrics server error", "error", err)
		}
	}()
}

func main() {
	loadEnv()

	if err := encryption.Init(); err != nil {
		log.Fatal(err)
	}
	if err := auth.Init(); err != nil {
		log.Fatal(err)
	}
	if err := mailer.Init(); err != nil {
		log.Fatal(err)
	}

	shutdown, err := telemetry.InitTracer()
	if err != nil {
		slog.Warn("tracing unavailable", "error", err)
	} else {
		defer func() {
			if err := shutdown(context.Background()); err != nil {
				slog.Error("tracer shutdown error", "error", err)
			}
		}()
	}

	startMetricsServer()

	app := fiber.New(fiberConfig())
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
	}))
	app.Use(middleware.GlobalRateLimit)
	app.Use(middleware.TracingMiddleware)

	database.Connect()

	go tasks.HandleTask()

	go socket.SocketI()

	routes.SetupRoutes(app)
	log.Fatal(app.Listen(":" + os.Getenv("PORT")))
}
