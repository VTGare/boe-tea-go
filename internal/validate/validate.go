package validate

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

type ErrorResponse struct {
	FailedField string `json:"failed_field"`
	Tag         string `json:"tag"`
	Value       string `json:"value"`
}

func (er *ErrorResponse) Error() string {
	return fmt.Sprintf("Failed to validate field: %v. Tags: %v", er.FailedField, er.Tag)
}

func Struct(s interface{}) []*ErrorResponse {
	var errors []*ErrorResponse
	validate := validator.New()

	err := validate.Struct(s)
	if err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			var element ErrorResponse
			element.FailedField = err.Field()
			element.Tag = err.Tag()
			element.Value = err.Param()
			errors = append(errors, &element)
		}
	}
	return errors
}
