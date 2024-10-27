package controllers

import (
	"api-go/internal/midleware"
	"api-go/internal/models"
	"api-go/internal/repository"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/prometheus/client_golang/prometheus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Repository struct {
	Duration prometheus.HistogramVec
	Summary  prometheus.Summary
	MainResp repository.Respositorys
}

func (repo *Repository) FindAllItems(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	limit := getQuery(r, "limit", 20)
	page := getQuery(r, "page", 0)
	resultsOrgs, err := repo.MainResp.Organization.FindAllMyOrgs(claims["id"], 0, 100)
	var orgsId []string
	for _, value := range resultsOrgs {
		orgsId = append(orgsId, value.Id.Hex())
	}
	if len(orgsId) == 0 {
		orgsId = append(orgsId, "")
	}
	results, err := repo.MainResp.Items.FindAll(bson.D{{
		"$or", bson.A{
			bson.D{{"user_id", claims["id"]}},
			bson.D{{"organizationId", bson.D{{"$in", orgsId}}}},
		},
	}}, page, limit)
	if err != nil {
		slog.Error(err.Error())
	}
	render.JSON(w, r, results)
}

func (repo *Repository) FindOneItem(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	status := "200"
	_, claims, _ := jwtauth.FromContext(r.Context())
	idstr := chi.URLParam(r, "id")
	_id, errP := primitive.ObjectIDFromHex(idstr)
	if errP != nil {
		render.Status(r, 403)
		status = "403"
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}
	result, err := repo.MainResp.Items.FindOne(_id)
	if err != nil {
		render.Status(r, 404)
		status = "404"
		render.JSON(w, r, map[string]string{"message": "NOT_FOUND"})
		return
	}
	resultsOrgs, err := repo.MainResp.Organization.FindAllMyOrgs(claims["id"], 0, 100)
	var orgsId []string
	for _, value := range resultsOrgs {
		orgsId = append(orgsId, value.Id.Hex())
	}

	if result.OrganizationId != "" && !contains(orgsId, result.OrganizationId) {
		render.Status(r, 404)
		status = "404"
		render.JSON(w, r, map[string]string{"message": "NOT_FOUND_ORG"})
		return
	}

	if result.Private && claims["id"] != result.UserId {
		render.Status(r, 404)
		status = "404"
		render.JSON(w, r, map[string]string{"message": "NOT_FOUND_ORG"})
		return
	}
	defer repo.Duration.WithLabelValues(midleware.GetRoutePattern(r), r.Method, status).Observe(time.Since(start).Seconds())
	defer repo.Summary.Observe(time.Since(start).Seconds())
	render.JSON(w, r, result)
}

func (repo *Repository) AddItem(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	decoder := json.NewDecoder(r.Body)
	var item models.Item
	if err := decoder.Decode(&item); err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	if item.OrganizationId != "" {
		_id, errP := primitive.ObjectIDFromHex(item.OrganizationId)
		if errP != nil {
			render.Status(r, 400)
			render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
			return
		}
		org, err := repo.MainResp.Organization.FindOne(bson.D{{"_id", _id}})
		if err != nil {
			render.Status(r, 400)
			render.JSON(w, r, map[string]string{"message": err.Error()})
			return
		}

		resultsOrgs, err := repo.MainResp.Organization.FindAllMyOrgs(claims["id"], 0, 100)
		var orgsId []string
		for _, value := range resultsOrgs {
			orgsId = append(orgsId, value.Id.Hex())
		}

		if !contains(orgsId, item.OrganizationId) {
			render.Status(r, 404)
			render.JSON(w, r, map[string]string{"message": "NOT_FOUND_ORG"})
			return
		}
		item.Organization = models.OrganizationModel{
			Name: org.Name,
			Id:   org.Id,
		}
	}
	item.UserId = claims["id"]
	item.Ip = ReadUserIP(r)
	item.CreatedAt = time.Now()
	item.UpdateAt = time.Now()
	saveItem, err := repo.MainResp.Items.AddItem(item)
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
	saveItem, err := repo.MainResp.Items.UpdateItem(item, _id)
	if err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	render.JSON(w, r, saveItem)
}

func (repo *Repository) DeleteItem(w http.ResponseWriter, r *http.Request) {
	idstr := chi.URLParam(r, "id")
	_, claims, _ := jwtauth.FromContext(r.Context())
	_id, errP := primitive.ObjectIDFromHex(idstr)
	if errP != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}
	result, err := repo.MainResp.Items.DeleteItem(_id, claims["id"])
	if err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}
	render.JSON(w, r, result)
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

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
