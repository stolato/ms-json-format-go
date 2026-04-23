package tasks

import (
	"api-go/internal/database"
	"api-go/internal/repositories"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron"
	"go.mongodb.org/mongo-driver/bson"
)

func HandleTask() {
	log.Println("Iniciando tarefas")
	repo := repositories.NewItemsRepository(database.DB)
	c := cron.New()
	c.Start()

	err := c.AddFunc("0 0 1 * * *", func() {
		fmt.Println("Minha tarefa foi executada às", time.Now())
		results, err := repo.FindAll(bson.D{
			{"user_id", bson.D{
				{"$in", bson.A{nil}},
			}},
			{"expirateAt", bson.D{
				{"$lte", time.Now()},
			}},
		}, 0, 0)
		if err != nil {
			log.Println(err.Error())
		}
		for _, data := range results.Data {
			_, err := repo.DeleteItem(data.Id, "")
			if err != nil {
				log.Println(err.Error())
			}
		}
		log.Println("removed", len(results.Data))
	})
	if err != nil {
		log.Fatalln(err.Error())
	}
}
