package tasks

import (
	"api-go/internal/repositories"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

// cleanupSchedule: todo dia às 01:00:00 (formato com segundos).
const cleanupSchedule = "0 0 1 * * *"

// Start agenda a limpeza diária (01:00) de itens anônimos expirados. Quem
// chama deve encerrar com Stop(), cujo contexto termina quando a execução em
// andamento (se houver) acabar.
func Start(repo repositories.ItemsRepository) (*cron.Cron, error) {
	log.Println("Iniciando tarefas")
	// WithSeconds: mantém o formato de 6 campos (com segundos) da v1.
	c := cron.New(cron.WithSeconds())
	_, err := c.AddFunc(cleanupSchedule, func() {
		removed, err := repo.DeleteExpiredAnonymous(time.Now())
		if err != nil {
			log.Println("limpeza de itens anônimos expirados falhou:", err)
			return
		}
		log.Println("limpeza de itens anônimos expirados: removidos", removed)
	})
	if err != nil {
		return nil, err
	}
	c.Start()
	return c, nil
}
