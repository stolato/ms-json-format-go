package models

import (
	"os"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	Id       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email    string             `json:"email" bson:"email" validate:"email,required"`
	Password string             `json:"password" bson:"password" validate:"required,min=6"`
	Name     string             `json:"name,omitempty" bson:"name" required:"required"`
	Active   bool               `json:"active" bson:"active"`
	Setting  string             `json:"settings,omitempty" bson:"settings"`
}

type ErrorsHandle struct {
	FailedField string
	Tag         string
	Value       interface{}
	Error       bool
}

func (u *User) Validate() []ErrorsHandle {
	validate := validator.New()
	var validationErrors []ErrorsHandle
	errs := validate.Struct(u)
	if errs != nil {
		for _, err := range errs.(validator.ValidationErrors) {
			var elem ErrorsHandle

			elem.FailedField = err.Field()
			elem.Tag = err.Tag()
			elem.Value = err.Value()
			elem.Error = true

			validationErrors = append(validationErrors, elem)
		}
	}
	return validationErrors
}

func (u *User) GenerateJWT() (map[string]string, error) {
	secret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"id":    u.Id,
		"email": u.Email,
		"name":  u.Name,
		"exp":   time.Now().Add(time.Hour * 24).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(secret))

	refreshClaims := jwt.MapClaims{
		"id":  u.Id,
		"exp": time.Now().Add(time.Hour * 48).Unix(),
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)

	refreshString, err := refreshToken.SignedString([]byte(secret))

	return map[string]string{
		"token":         tokenString,
		"refresh_token": refreshString,
	}, err
}
