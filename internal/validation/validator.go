package validation

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

type ErrorsHandle struct {
	FailedField string
	Tag         string
	Value       interface{}
	Error       bool
}

func Validator(item interface{}) []ErrorsHandle {
	validate := validator.New()
	var validationErrors []ErrorsHandle
	errs := validate.Struct(item)
	if errs != nil {
		for _, err := range errs.(validator.ValidationErrors) {
			var elem ErrorsHandle

			elem.FailedField = strings.ToLower(err.Field())
			elem.Tag = err.Tag()
			elem.Error = true

			validationErrors = append(validationErrors, elem)
		}
	}
	return validationErrors
}
