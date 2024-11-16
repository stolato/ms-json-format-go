package tasks

import (
	"api-go/internal/repository"
	"fmt"
	"github.com/robfig/cron"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

type ItemTask struct {
	DB *mongo.Client
}

func (r *ItemTask) HandleTask() {
	repo := repository.RepositoryMain{DB: r.DB}
	c := cron.New()
	c.Start()

	err := c.AddFunc("0 0 1 * * *", func() {
		fmt.Println("Minha tarefa foi executada às", time.Now())
		results, err := repo.Repositorys().Items.FindAll(bson.D{
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
			_, err := repo.Repositorys().Items.DeleteItem(data.Id, nil)
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
