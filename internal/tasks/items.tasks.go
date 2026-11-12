package tasks

import (
	"api-go/internal/database"
	"api-go/internal/repositories"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// cleanupSchedule: todo dia às 01:00:00 (formato com segundos).
const cleanupSchedule = "0 0 1 * * *"

// HandleTask agenda a limpeza diária (01:00) de itens anônimos expirados.
func HandleTask() {
	log.Println("Iniciando tarefas")
	repo := repositories.NewItemsRepository(database.DB)
	// WithSeconds: mantém o formato de 6 campos (com segundos) da v1.
	c := cron.New(cron.WithSeconds())
	c.Start()

	_, err := c.AddFunc(cleanupSchedule, func() {
		removed, err := repo.DeleteExpiredAnonymous(time.Now())
		if err != nil {
			log.Println("limpeza de itens anônimos expirados falhou:", err)
			return
		}
		log.Println("limpeza de itens anônimos expirados: removidos", removed)
	})
	if err != nil {
		log.Fatalln(err.Error())
	}
}
