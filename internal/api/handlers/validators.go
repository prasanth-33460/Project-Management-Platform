package handlers

import (
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

func init() {
	// Register a custom "uppercase" tag used by project key validation
	_ = validate.RegisterValidation("uppercase", func(fl validator.FieldLevel) bool {
		s := fl.Field().String()
		return s == strings.ToUpper(s)
	})
}

func validateStruct(s interface{}) error {
	return validate.Struct(s)
}
