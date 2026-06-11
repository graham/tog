package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		appErr   *AppError
		expected string
	}{
		{
			name:     "message only",
			appErr:   &AppError{Message: "something went wrong"},
			expected: "something went wrong",
		},
		{
			name:     "with internal error",
			appErr:   &AppError{Message: "failed", Internal: errors.New("db connection lost")},
			expected: "failed: db connection lost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.appErr.Error(); got != tt.expected {
				t.Errorf("Error() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	internal := errors.New("internal error")
	appErr := &AppError{Internal: internal}

	if got := appErr.Unwrap(); got != internal {
		t.Errorf("Unwrap() = %v, want %v", got, internal)
	}

	// Test nil internal
	appErr2 := &AppError{}
	if got := appErr2.Unwrap(); got != nil {
		t.Errorf("Unwrap() = %v, want nil", got)
	}
}

func TestErrBadRequest(t *testing.T) {
	err := ErrBadRequest("invalid input", errors.New("parse error"))

	if err.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusBadRequest)
	}
	if err.Message != "invalid input" {
		t.Errorf("Message = %q, want %q", err.Message, "invalid input")
	}
	if err.TrackingID == "" {
		t.Error("TrackingID should not be empty")
	}
}

func TestErrUnauthorized(t *testing.T) {
	err := ErrUnauthorized("not logged in", nil)

	if err.Status != http.StatusUnauthorized {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusUnauthorized)
	}
	if err.Message != "not logged in" {
		t.Errorf("Message = %q, want %q", err.Message, "not logged in")
	}
}

func TestErrForbidden(t *testing.T) {
	err := ErrForbidden("access denied", nil)

	if err.Status != http.StatusForbidden {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusForbidden)
	}
	if err.Message != "access denied" {
		t.Errorf("Message = %q, want %q", err.Message, "access denied")
	}
}

func TestErrNotFound(t *testing.T) {
	err := ErrNotFound("resource not found", nil)

	if err.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusNotFound)
	}
	if err.Message != "resource not found" {
		t.Errorf("Message = %q, want %q", err.Message, "resource not found")
	}
}

func TestErrInternal(t *testing.T) {
	err := ErrInternal("server error", errors.New("db down"))

	if err.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusInternalServerError)
	}
	if err.Message != "server error" {
		t.Errorf("Message = %q, want %q", err.Message, "server error")
	}
}

func TestGenerateTrackingID(t *testing.T) {
	id1 := generateTrackingID()
	id2 := generateTrackingID()

	// Should be 12 hex characters (6 bytes = 12 hex chars)
	if len(id1) != 12 {
		t.Errorf("TrackingID length = %d, want 12", len(id1))
	}

	// Should be unique
	if id1 == id2 {
		t.Error("TrackingIDs should be unique")
	}

	// Should be valid hex
	for _, c := range id1 {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("TrackingID contains invalid character: %c", c)
		}
	}
}

func TestIsDevMode(t *testing.T) {
	// Save and restore env
	original := os.Getenv("ENVIRONMENT")
	defer os.Setenv("ENVIRONMENT", original)

	os.Setenv("ENVIRONMENT", "dev")
	if !isDevMode() {
		t.Error("isDevMode() should return true when ENVIRONMENT=dev")
	}

	os.Setenv("ENVIRONMENT", "production")
	if isDevMode() {
		t.Error("isDevMode() should return false when ENVIRONMENT=production")
	}

	os.Unsetenv("ENVIRONMENT")
	if isDevMode() {
		t.Error("isDevMode() should return false when ENVIRONMENT is unset")
	}
}

func TestWriteAppError(t *testing.T) {
	// Save and restore env
	original := os.Getenv("ENVIRONMENT")
	defer os.Setenv("ENVIRONMENT", original)

	t.Run("production mode", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "production")

		appErr := ErrBadRequest("invalid request", errors.New("internal details"))
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		WriteAppError(rec, req, appErr)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusBadRequest)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "invalid request") {
			t.Error("Response should contain message")
		}
		if strings.Contains(body, "internal details") {
			t.Error("Response should NOT contain internal error in production")
		}
		if !strings.Contains(body, "tracking_id") {
			t.Error("Response should contain tracking_id")
		}
	})

	t.Run("dev mode includes debug info", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "dev")

		appErr := ErrInternal("server error", errors.New("db connection failed"))
		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		WriteAppError(rec, req, appErr)

		body := rec.Body.String()
		if !strings.Contains(body, "db connection failed") {
			t.Error("Response should contain internal error in dev mode")
		}
		if !strings.Contains(body, "debug") {
			t.Error("Response should contain debug section in dev mode")
		}
	})

	t.Run("nil error creates internal error", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "production")

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		WriteAppError(rec, req, nil)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusInternalServerError)
		}
	})
}

func TestNewAppError(t *testing.T) {
	internal := errors.New("internal")
	err := newAppError(http.StatusTeapot, "I'm a teapot", internal)

	if err.Status != http.StatusTeapot {
		t.Errorf("Status = %d, want %d", err.Status, http.StatusTeapot)
	}
	if err.Message != "I'm a teapot" {
		t.Errorf("Message = %q, want %q", err.Message, "I'm a teapot")
	}
	if err.Internal != internal {
		t.Error("Internal error not set correctly")
	}
	if err.TrackingID == "" {
		t.Error("TrackingID should be generated")
	}
}
