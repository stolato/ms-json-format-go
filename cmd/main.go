package main

import (
	"api-go/config"
	"api-go/internal/api"
	"api-go/internal/socket"
	"api-go/internal/tasks"
	"github.com/joho/godotenv"
	"log"
	"log/slog"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		slog.Info(err.Error())
	}

	db, err := config.DB()
	if err != nil {
		log.Fatalln(err.Error())
	}

	task := tasks.ItemTask{DB: db}
	go task.HandleTask()

	go socket.SocketI()

	router := api.RouterAPI{DB: db}
	router.InitRouter()
}
