package main

import (
	"api-go/internal/database"
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

func main() {
	loadEnv()

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

	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", promhttp.Handler())
		if err := http.ListenAndServe(":9091", mux); err != nil {
			slog.Error("metrics server error", "error", err)
		}
	}()

	app := fiber.New()
	app.Use(logger.New())
	app.Use(recover.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
	}))
	app.Use(middleware.TracingMiddleware)

	database.Connect()

	go tasks.HandleTask()

	go socket.SocketI()

	routes.SetupRoutes(app)
	log.Fatal(app.Listen(":" + os.Getenv("PORT")))
}
