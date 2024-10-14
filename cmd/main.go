package main

import (
	"api-go/config"
	"api-go/internal/api"
	"log"
	"log/slog"

	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		slog.Info(err.Error())
	}
	db, err := config.DB()
	if err != nil {
		log.Fatalln(err.Error())
	}
	router := api.RouterAPI{DB: db}
	router.InitRouter()
}
