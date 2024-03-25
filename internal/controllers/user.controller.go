package controllers

import (
	"api-go/internal/models"
	"api-go/internal/repository"
	"encoding/json"
	"fmt"
	"github.com/go-chi/jwtauth/v5"
	"github.com/go-chi/render"
	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"net/http"
	"os"
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

func (repo *RepositoryUser) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	_, claims, _ := jwtauth.FromContext(r.Context())
	id := fmt.Sprintf("%v", claims["id"])
	var settings models.User
	err := decoder.Decode(&settings)
	if err != nil {
		w.WriteHeader(400)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	repo.User.UpdateSettings(id, settings.Setting)
	render.JSON(w, r, settings)
}

func (repo *RepositoryUser) GetSettings(w http.ResponseWriter, r *http.Request) {
	_, claims, _ := jwtauth.FromContext(r.Context())
	id := fmt.Sprintf("%v", claims["id"])
	_id, _ := primitive.ObjectIDFromHex(id)
	mUser := models.User{Id: _id}
	user, _ := repo.User.FindMe(&mUser)
	render.JSON(w, r, map[string]string{"settings": user.Setting})
	return
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
	render.JSON(w, r, token)
}

func (repo *RepositoryUser) RefreshToken(w http.ResponseWriter, r *http.Request) {
	type tokenReqBody struct {
		RefreshToken string `json:"refresh_token"`
	}
	decoder := json.NewDecoder(r.Body)
	var refresh tokenReqBody
	if err := decoder.Decode(&refresh); err != nil {
		w.WriteHeader(401)
		render.JSON(w, r, map[string]string{"message": err.Error()})
		return
	}
	token, _ := jwt.Parse(refresh.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(os.Getenv("JWT_SECRET")), nil
	})
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		id := fmt.Sprintf("%v", claims["id"])
		_id, _ := primitive.ObjectIDFromHex(id)
		userModel := models.User{
			Id:     _id,
			Active: true,
		}
		user, err := repo.User.FindMe(&userModel)
		if err != nil {
			w.WriteHeader(401)
			render.JSON(w, r, map[string]string{"message": err.Error()})
			return
		}
		token, err := user.GenerateJWT()
		render.JSON(w, r, token)
		return
	}

	w.WriteHeader(401)
	render.JSON(w, r, map[string]string{"message": "ERRO"})
	return
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
