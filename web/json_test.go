package web

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	// Save and restore env
	original := os.Getenv("ENVIRONMENT")
	defer os.Setenv("ENVIRONMENT", original)

	t.Run("production mode - compact JSON", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "production")
		rec := httptest.NewRecorder()

		data := map[string]string{"key": "value", "foo": "bar"}
		err := WriteJSON(rec, data)

		if err != nil {
			t.Fatalf("WriteJSON error: %v", err)
		}
		if rec.Header().Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
		}
		body := rec.Body.String()
		// Should not have extra newlines/indentation (compact)
		if strings.Contains(body, "\n  ") {
			t.Error("Production mode should not use indentation")
		}
	})

	t.Run("dev mode - pretty JSON", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "dev")
		rec := httptest.NewRecorder()

		data := map[string]string{"key": "value"}
		err := WriteJSON(rec, data)

		if err != nil {
			t.Fatalf("WriteJSON error: %v", err)
		}
		body := rec.Body.String()
		// Should have indentation in dev mode
		if !strings.Contains(body, "  ") {
			t.Error("Dev mode should use indentation")
		}
	})

	t.Run("encodes struct", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "production")
		rec := httptest.NewRecorder()

		type User struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}
		err := WriteJSON(rec, User{Name: "Alice", Email: "alice@example.com"})

		if err != nil {
			t.Fatalf("WriteJSON error: %v", err)
		}
		body := rec.Body.String()
		if !strings.Contains(body, `"name":"Alice"`) {
			t.Errorf("Body should contain name field, got %s", body)
		}
		if !strings.Contains(body, `"email":"alice@example.com"`) {
			t.Errorf("Body should contain email field, got %s", body)
		}
	})
}

func TestLogJSON(t *testing.T) {
	// Save and restore env
	original := os.Getenv("ENVIRONMENT")
	defer os.Setenv("ENVIRONMENT", original)

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	defer func() {
		os.Stdout = oldStdout
	}()

	t.Run("without label", func(t *testing.T) {
		os.Setenv("ENVIRONMENT", "production")

		data := map[string]int{"count": 42}
		err := LogJSON(data)
		if err != nil {
			t.Fatalf("LogJSON error: %v", err)
		}

		w.Close()
		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if !strings.Contains(output, "42") {
			t.Errorf("Output should contain value, got %s", output)
		}
	})
}

func TestLogJSON_WithLabel(t *testing.T) {
	// Save and restore env
	original := os.Getenv("ENVIRONMENT")
	defer os.Setenv("ENVIRONMENT", original)
	os.Setenv("ENVIRONMENT", "production")

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	data := map[string]string{"test": "data"}
	err := LogJSON(data, "MyLabel")
	if err != nil {
		t.Fatalf("LogJSON error: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	if !strings.Contains(output, "MyLabel:") {
		t.Errorf("Output should contain label, got %s", output)
	}
}

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		message    string
		wantError  string
	}{
		{"bad request", http.StatusBadRequest, "invalid input", "Bad Request"},
		{"not found", http.StatusNotFound, "resource missing", "Not Found"},
		{"internal error", http.StatusInternalServerError, "server error", "Internal Server Error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, tt.statusCode, tt.message)

			if rec.Code != tt.statusCode {
				t.Errorf("Status = %d, want %d", rec.Code, tt.statusCode)
			}
			if rec.Header().Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", rec.Header().Get("Content-Type"))
			}

			body := rec.Body.String()
			if !strings.Contains(body, tt.wantError) {
				t.Errorf("Body should contain %q, got %s", tt.wantError, body)
			}
			if !strings.Contains(body, tt.message) {
				t.Errorf("Body should contain message %q, got %s", tt.message, body)
			}
		})
	}
}
