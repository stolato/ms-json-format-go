package models

import (
	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	Id       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Email    string             `json:"email,omitempty" bson:"email,omitempty" validate:"email,required"`
	Password string             `json:"password,omitempty" bson:"password,omitempty" validate:"required,min=6"`
	Name     string             `json:"name,omitempty" bson:"name,omitempty" validate:"required"`
	Active   bool               `json:"active,omitempty" bson:"active,omitempty"`
	Setting  string             `json:"settings,omitempty" bson:"settings,omitempty"`
	// EmailVerified: nil = conta anterior à verificação de email (tratada como
	// verificada); false = aguardando confirmação.
	EmailVerified *bool `json:"-" bson:"emailVerified,omitempty"`
}

// IsEmailVerified considera contas antigas (sem o campo) como verificadas.
func (u *User) IsEmailVerified() bool {
	return u.EmailVerified == nil || *u.EmailVerified
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
