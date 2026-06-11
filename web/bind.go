package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New(validator.WithRequiredStructEnabled())

	// Use JSON field names in error messages
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return fld.Name
		}
		if name == "" {
			return fld.Name
		}
		return name
	})
}

// ValidationError represents validation failures with field-level details.
type ValidationError struct {
	Fields map[string]string `json:"fields"`
}

func (e *ValidationError) Error() string {
	var parts []string
	for field, msg := range e.Fields {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	return strings.Join(parts, "; ")
}

// BindJSON decodes JSON from the request body into v.
// This is a simple decoder with no validation.
// Use Bind() if you want automatic validation.
func BindJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	decoder := json.NewDecoder(r.Body)
	return decoder.Decode(v)
}

// Bind decodes JSON from the request body into v and validates it.
// If decoding or validation fails, it writes an error response and returns false.
// Returns true if binding and validation succeeded.
//
// Usage:
//
//	var input CreateItemRequest
//	if !web.Bind(r, w, &input) {
//	    return // Error already written
//	}
//	// input is valid, use it
func Bind(r *http.Request, w http.ResponseWriter, v any) bool {
	// Decode JSON
	if err := BindJSON(r, v); err != nil {
		WriteError(w, http.StatusBadRequest, "Invalid JSON: "+err.Error())
		return false
	}

	// Validate
	if err := Validate(v); err != nil {
		WriteValidationError(w, err)
		return false
	}

	return true
}

// Validate validates a struct using validation tags.
// Returns nil if validation passes, or a *ValidationError with field details.
//
// Common validation tags:
//   - required: field must be present and non-zero
//   - min=n, max=n: length constraints for strings/slices, value for numbers
//   - gte=n, lte=n: greater/less than or equal for numbers
//   - email: valid email format
//   - url: valid URL format
//   - oneof=a b c: value must be one of the listed options
//   - dive: validate each element in a slice
//
// Example:
//
//	type Request struct {
//	    Name  string `validate:"required,min=1,max=100"`
//	    Email string `validate:"required,email"`
//	    Age   int    `validate:"gte=0,lte=150"`
//	}
func Validate(v any) error {
	err := validate.Struct(v)
	if err == nil {
		return nil
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		return err
	}

	fields := make(map[string]string)
	for _, fe := range validationErrors {
		fields[fe.Field()] = formatValidationError(fe)
	}

	return &ValidationError{Fields: fields}
}

// WriteValidationError writes a validation error response.
func WriteValidationError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)

	response := map[string]any{
		"error":   "Validation Error",
		"message": "Invalid request data",
	}

	if ve, ok := err.(*ValidationError); ok {
		response["fields"] = ve.Fields
	} else {
		response["message"] = err.Error()
	}

	json.NewEncoder(w).Encode(response)
}

// formatValidationError converts a validator.FieldError to a human-readable message.
func formatValidationError(fe validator.FieldError) string {
	field := fe.Field()

	switch fe.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at least %s characters", field, fe.Param())
		}
		return fmt.Sprintf("%s must be at least %s", field, fe.Param())
	case "max":
		if fe.Kind() == reflect.String {
			return fmt.Sprintf("%s must be at most %s characters", field, fe.Param())
		}
		return fmt.Sprintf("%s must be at most %s", field, fe.Param())
	case "gte":
		return fmt.Sprintf("%s must be %s or greater", field, fe.Param())
	case "lte":
		return fmt.Sprintf("%s must be %s or less", field, fe.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, fe.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, fe.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, fe.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, fe.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "alphanum":
		return fmt.Sprintf("%s must contain only letters and numbers", field)
	case "numeric":
		return fmt.Sprintf("%s must be numeric", field)
	case "boolean":
		return fmt.Sprintf("%s must be a boolean", field)
	case "datetime":
		return fmt.Sprintf("%s must be a valid datetime in format %s", field, fe.Param())
	default:
		return fmt.Sprintf("%s failed %s validation", field, fe.Tag())
	}
}

// RegisterValidation registers a custom validation function.
// This allows you to add project-specific validations.
//
// Example:
//
//	web.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
//	    return regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(fl.Field().String())
//	})
//
// Then use it:
//
//	type Post struct {
//	    Slug string `validate:"required,slug"`
//	}
func RegisterValidation(tag string, fn validator.Func) error {
	return validate.RegisterValidation(tag, fn)
}
