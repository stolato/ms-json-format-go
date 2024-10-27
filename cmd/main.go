package main

import (
	"api-go/config"
	"api-go/internal/api"
	"api-go/internal/socket"
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
	go socket.SocketI()
	router := api.RouterAPI{DB: db}
	router.InitRouter()
}
