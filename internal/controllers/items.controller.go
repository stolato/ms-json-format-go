package controllers

import (
	"api-go/internal/models"
	"api-go/internal/repository"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type Repository struct {
	Items repository.ItemsRepository
}

func (repo *Repository) FindAllItems(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	limit := getQuery(r, "limit", 20)
	page := getQuery(r, "page", 0)
	findOptions := options.Find()
	findOptions.SetLimit(limit)
	findOptions.SetSkip(limit * page)
	results, err := repo.Items.FindAll(bson.D{{"user_id", claims["id"]}}, findOptions)
	if err != nil {
		slog.Error(err.Error())
	}
	render.JSON(w, r, results)
}

func (repo *Repository) FindOneItem(w http.ResponseWriter, r *http.Request) {
	idstr := chi.URLParam(r, "id")
	_id, errP := primitive.ObjectIDFromHex(idstr)
	if errP != nil {
		w.WriteHeader(403)
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}
	result, err := repo.Items.FindOne(_id)
	if err != nil {
		w.WriteHeader(404)
		render.JSON(w, r, map[string]string{"message": "NOT_FOUND"})
		return
	}
	render.JSON(w, r, result)
}

func (repo *Repository) AddItem(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	fmt.Print(claims["id"])
	decoder := json.NewDecoder(r.Body)
	var item models.Item
	if err := decoder.Decode(&item); err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	item.UserId = claims["id"]
	item.Ip = ReadUserIP(r)
	item.CreatedAt = time.Now()
	item.UpdateAt = time.Now()
	saveItem, err := repo.Items.AddItem(item)
	if err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	render.JSON(w, r, saveItem)
}

func (repo *Repository) UpdateItem(w http.ResponseWriter, r *http.Request) {
	idstr := chi.URLParam(r, "id")
	_id, errP := primitive.ObjectIDFromHex(idstr)
	if errP != nil {
		w.WriteHeader(403)
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}
	decoder := json.NewDecoder(r.Body)
	var item models.Item
	if err := decoder.Decode(&item); err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	saveItem, err := repo.Items.UpdateItem(item, _id)
	if err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	render.JSON(w, r, saveItem)
}

func getQuery(r *http.Request, query string, d int64) int64 {
	init := r.URL.Query().Get(query)
	if init != "" {
		value, err := strconv.Atoi(init)
		if err != nil {
			log.Fatal(err)
		}
		return int64(value)
	}
	return d
}

func ReadUserIP(r *http.Request) string {
	IPAddress := r.Header.Get("X-Real-Ip")
	if IPAddress == "" {
		IPAddress = r.Header.Get("X-Forwarded-For")
	}
	if IPAddress == "" {
		IPAddress = r.RemoteAddr
	}
	return IPAddress
}
