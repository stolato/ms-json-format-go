package models

import (
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"os"
	"time"
)

type User struct {
	Id       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email    string             `json:"email" bson:"email" validate:"email,required"`
	Password string             `json:"password" bson:"password" validate:"required,min=6"`
	Name     string             `json:"name" bson:"name" required:"required"`
	Active   bool
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

func (u *User) GenerateJWT() (string, error) {
	secret := os.Getenv("JWT_SECRET")
	claims := jwt.MapClaims{
		"id":     u.Id,
		"email":  u.Email,
		"active": u.Active,
		"name":   u.Name,
		"exp":    time.Now().Add(time.Hour * 1).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString([]byte(secret))

	return tokenString, err
}
