package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRoutesDocHandler(t *testing.T) {
	// Create a test router with some routes
	router := chi.NewRouter()
	router.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {})
	router.Post("/api/users", func(w http.ResponseWriter, r *http.Request) {})
	router.Get("/api/items/{id}", func(w http.ResponseWriter, r *http.Request) {})

	// Mount the docs handler
	router.Route("/docs/routes", RoutesDocHandler(router))

	t.Run("HTML endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/routes/", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("Content-Type = %q, want text/html", contentType)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "<!DOCTYPE html>") {
			t.Error("expected HTML doctype")
		}
	})

	t.Run("JSON endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/routes/json", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			t.Errorf("Content-Type = %q, want application/json", contentType)
		}

		var result struct {
			Generated string `json:"generated"`
			Total     int    `json:"total"`
			Routes    []any  `json:"routes"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result.Total == 0 {
			t.Error("expected at least one route in JSON output")
		}
	})
}

func TestRoutesDocHandler_WithConfig(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {})

	config := RoutesDocConfig{
		Title: "Custom API Title",
		Descriptions: map[string]string{
			"GET /api/users": "List all users",
		},
	}

	router.Route("/docs/routes", RoutesDocHandler(router, config))

	req := httptest.NewRequest("GET", "/docs/routes/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "Custom API Title") {
		t.Error("expected custom title in HTML output")
	}
}

func TestRoutesDocHandler_DefaultConfig(t *testing.T) {
	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {})

	// No config provided - should use defaults
	router.Route("/docs/routes", RoutesDocHandler(router))

	req := httptest.NewRequest("GET", "/docs/routes/", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "API Routes") {
		t.Error("expected default title 'API Routes'")
	}
}

func TestDocsIndexHandler(t *testing.T) {
	handler := DocsIndexHandler("/docs")

	req := httptest.NewRequest("GET", "/docs", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", contentType)
	}

	body := rec.Body.String()
	expectedLinks := []string{
		"/docs/routes",
		"/docs/queries",
		"/docs/tests",
		"Documentation",
		"API Routes",
		"SQL Queries",
	}

	for _, link := range expectedLinks {
		if !strings.Contains(body, link) {
			t.Errorf("expected body to contain %q", link)
		}
	}
}
