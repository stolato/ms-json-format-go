package controllers

import (
	"api-go/internal/models"
	"api-go/internal/repository"
	"api-go/internal/validation"
	"api-go/internal/validation/dtos"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type OrganizationController struct {
	Repo repository.Respositorys
}

func (repo *OrganizationController) FindAllTimes(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	limit := getQuery(r, "limit", 20)
	page := getQuery(r, "page", 0)

	results, err := repo.Repo.Organization.FindAllMyOrgs(claims["id"], page, limit)
	if err != nil {
		slog.Error(err.Error())
	}
	render.JSON(w, r, results)
}

func (repo *OrganizationController) AddTime(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	decoder := json.NewDecoder(r.Body)
	var dto dtos.OrganizationStoreDTO
	var itemSave models.OrganizationModel
	if err := decoder.Decode(&dto); err != nil {
		render.Status(r, 400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	if errs := validation.Validator(dto); errs != nil {
		render.Status(r, 400)
		render.JSON(w, r, errs)
		return
	}
	userOwner := []models.User{}
	id := fmt.Sprintf("%s", claims["id"])
	_id, _ := primitive.ObjectIDFromHex(id)
	userOwner = append(userOwner, models.User{Id: _id})
	itemSave.Users = userOwner
	itemSave.Name = dto.Name
	itemSave.OwnerId = id
	itemSave.CreatedAt = time.Now()
	itemSave.UpdateAt = time.Now()
	saveItem, err := repo.Repo.Organization.Store(itemSave)
	if err != nil {
		render.Status(r, 400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	render.JSON(w, r, saveItem)
}

func (repo *OrganizationController) DeleteOrganization(w http.ResponseWriter, r *http.Request) {
	idstr := chi.URLParam(r, "id")
	_, claims, _ := jwtauth.FromContext(r.Context())
	_id, errP := primitive.ObjectIDFromHex(idstr)
	if errP != nil {
		render.Status(r, 400)
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}

	result, err := repo.Repo.Organization.Delete(bson.D{{"_id", _id}, {"ownerId", claims["id"]}})
	if err != nil {
		render.Status(r, 400)
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}
	render.JSON(w, r, result)
}

func (repo *OrganizationController) AddUser(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	idstr := chi.URLParam(r, "id")
	_id, errP := primitive.ObjectIDFromHex(idstr)
	if errP != nil {
		render.Status(r, 400)
		render.JSON(w, r, map[string]string{"message": "Not object to mongoID"})
		return
	}
	decoder := json.NewDecoder(r.Body)
	var addDto dtos.AddUserOrganizationDTO
	if err := decoder.Decode(&addDto); err != nil {
		render.Status(r, 400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	if errs := validation.Validator(addDto); errs != nil {
		render.Status(r, 400)
		render.JSON(w, r, errs)
		return
	}
	saveItem, err := repo.Repo.Organization.AddUser(addDto, _id, claims["id"])
	if err != nil {
		render.Status(r, 400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	render.JSON(w, r, saveItem)
}
