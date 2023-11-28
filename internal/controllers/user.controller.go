package controllers

import (
	"api-go/internal/models"
	"api-go/internal/repository"
	"encoding/json"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"net/http"
)

type RepositoryUser struct {
	User repository.UserRepository
}

func (repo *RepositoryUser) RegisterController(w http.ResponseWriter, r *http.Request) {
	user, err, errs := validateUser(r)
	if err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	if errs != nil {
		render.JSON(w, r, err)
		return
	}
	save, err := repo.User.Register(user)
	if err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	render.JSON(w, r, save)
}

func (repo *RepositoryUser) Me(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	render.JSON(w, r, claims)
}

func (repo *RepositoryUser) AuthController(w http.ResponseWriter, r *http.Request) {
	user, err, errs := validateUser(r)
	if err != nil {
		w.WriteHeader(401)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	if errs != nil {
		render.JSON(w, r, err)
		return
	}
	result, err := repo.User.FindOne(user)
	if err != nil {
		w.WriteHeader(401)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	token, err := result.GenerateJWT()
	if err != nil {
		w.WriteHeader(401)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	render.JSON(w, r, map[string]string{"token": token})
}

func validateUser(r *http.Request) (*models.User, error, []models.ErrorsHandle) {
	decoder := json.NewDecoder(r.Body)
	var user models.User
	if err := decoder.Decode(&user); err != nil {
		return nil, err, nil
	}
	if err := user.Validate(); err != nil {
		return nil, nil, err
	}
	return &user, nil, nil
}
