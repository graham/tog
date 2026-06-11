package web_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graham/tog/web"
)

type testRequest struct {
	Name  string `json:"name" validate:"required,min=1,max=100"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=150"`
}

type optionalFieldsRequest struct {
	Name        string `json:"name" validate:"required"`
	Description string `json:"description" validate:"max=500"`
}

func TestBindJSON(t *testing.T) {
	t.Run("decodes valid JSON", func(t *testing.T) {
		body := `{"name": "test", "email": "test@example.com", "age": 25}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))

		var input testRequest
		err := web.BindJSON(req, &input)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if input.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", input.Name)
		}
		if input.Email != "test@example.com" {
			t.Errorf("expected email 'test@example.com', got '%s'", input.Email)
		}
		if input.Age != 25 {
			t.Errorf("expected age 25, got %d", input.Age)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		body := `{"name": "test", invalid}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))

		var input testRequest
		err := web.BindJSON(req, &input)

		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
	})

	t.Run("returns error for nil body", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/", nil)
		req.Body = nil

		var input testRequest
		err := web.BindJSON(req, &input)

		if err == nil {
			t.Fatal("expected error for nil body")
		}
	})
}

func TestValidate(t *testing.T) {
	t.Run("passes for valid struct", func(t *testing.T) {
		input := testRequest{
			Name:  "test",
			Email: "test@example.com",
			Age:   25,
		}

		err := web.Validate(&input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("fails for missing required field", func(t *testing.T) {
		input := testRequest{
			Email: "test@example.com",
			Age:   25,
		}

		err := web.Validate(&input)
		if err == nil {
			t.Fatal("expected validation error")
		}

		ve, ok := err.(*web.ValidationError)
		if !ok {
			t.Fatalf("expected ValidationError, got %T", err)
		}

		if _, exists := ve.Fields["name"]; !exists {
			t.Error("expected error for 'name' field")
		}
	})

	t.Run("fails for invalid email", func(t *testing.T) {
		input := testRequest{
			Name:  "test",
			Email: "not-an-email",
			Age:   25,
		}

		err := web.Validate(&input)
		if err == nil {
			t.Fatal("expected validation error")
		}

		ve := err.(*web.ValidationError)
		if _, exists := ve.Fields["email"]; !exists {
			t.Error("expected error for 'email' field")
		}
	})

	t.Run("fails for age out of range", func(t *testing.T) {
		input := testRequest{
			Name:  "test",
			Email: "test@example.com",
			Age:   200,
		}

		err := web.Validate(&input)
		if err == nil {
			t.Fatal("expected validation error")
		}

		ve := err.(*web.ValidationError)
		if _, exists := ve.Fields["age"]; !exists {
			t.Error("expected error for 'age' field")
		}
	})

	t.Run("returns multiple field errors", func(t *testing.T) {
		input := testRequest{} // All fields invalid

		err := web.Validate(&input)
		if err == nil {
			t.Fatal("expected validation error")
		}

		ve := err.(*web.ValidationError)
		if len(ve.Fields) < 2 {
			t.Errorf("expected multiple field errors, got %d", len(ve.Fields))
		}
	})

	t.Run("optional fields pass when empty", func(t *testing.T) {
		input := optionalFieldsRequest{
			Name: "test",
			// Description is optional
		}

		err := web.Validate(&input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestBind(t *testing.T) {
	t.Run("succeeds with valid JSON and validation", func(t *testing.T) {
		body := `{"name": "test", "email": "test@example.com", "age": 25}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		var input testRequest
		ok := web.Bind(req, rec, &input)

		if !ok {
			t.Fatalf("expected success, got failure: %s", rec.Body.String())
		}
		if input.Name != "test" {
			t.Errorf("expected name 'test', got '%s'", input.Name)
		}
	})

	t.Run("returns false and writes error for invalid JSON", func(t *testing.T) {
		body := `{"invalid json`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		var input testRequest
		ok := web.Bind(req, rec, &input)

		if ok {
			t.Fatal("expected failure for invalid JSON")
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Invalid JSON") {
			t.Errorf("expected 'Invalid JSON' in response, got: %s", rec.Body.String())
		}
	})

	t.Run("returns false and writes error for validation failure", func(t *testing.T) {
		body := `{"name": "", "email": "not-email", "age": 25}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		rec := httptest.NewRecorder()

		var input testRequest
		ok := web.Bind(req, rec, &input)

		if ok {
			t.Fatal("expected failure for validation error")
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "Validation Error") {
			t.Errorf("expected 'Validation Error' in response, got: %s", rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "fields") {
			t.Errorf("expected 'fields' in response, got: %s", rec.Body.String())
		}
	})
}

func TestValidationErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		field    string
		contains string
	}{
		{
			name:     "required",
			input:    &testRequest{Email: "test@test.com", Age: 25},
			field:    "name",
			contains: "required",
		},
		{
			name:     "email",
			input:    &testRequest{Name: "test", Email: "bad", Age: 25},
			field:    "email",
			contains: "email",
		},
		{
			name:     "max",
			input:    &optionalFieldsRequest{Name: "test", Description: strings.Repeat("x", 501)},
			field:    "description",
			contains: "at most",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := web.Validate(tt.input)
			if err == nil {
				t.Fatal("expected validation error")
			}

			ve := err.(*web.ValidationError)
			msg, exists := ve.Fields[tt.field]
			if !exists {
				t.Fatalf("expected error for field '%s', got: %v", tt.field, ve.Fields)
			}
			if !strings.Contains(strings.ToLower(msg), tt.contains) {
				t.Errorf("expected message to contain '%s', got: %s", tt.contains, msg)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	ve := &web.ValidationError{
		Fields: map[string]string{
			"name":  "name is required",
			"email": "email is invalid",
		},
	}

	errStr := ve.Error()
	if !strings.Contains(errStr, "name") {
		t.Error("expected error string to contain 'name'")
	}
	if !strings.Contains(errStr, "email") {
		t.Error("expected error string to contain 'email'")
	}
}
