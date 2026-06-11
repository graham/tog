package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
)

func TestQueriesDocHandler(t *testing.T) {
	// Create an in-memory SQLite database
	cfg := &db.Config{
		Databases: map[string]db.DatabaseConfig{
			"primary": {Driver: "sqlite3", DSN: ":memory:"},
		},
		Default: "primary",
	}
	mgr, err := db.NewManager(cfg)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()

	database := mgr.Default()

	// Create a test table
	_, err = database.DB.Exec("CREATE TABLE test_items (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Register some queries
	type TestItem struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}

	db.RegisterNamed[TestItem](database, "ListTestItems", "SELECT * FROM test_items")
	db.RegisterExecNamed(database, "InsertTestItem", "INSERT INTO test_items (name) VALUES ($1)")

	// Create a router with the docs handler
	router := chi.NewRouter()
	router.Route("/docs/queries", QueriesDocHandler(database))

	t.Run("HTML endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/queries/", nil)
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
		if !strings.Contains(body, "Registered Queries") {
			t.Error("expected HTML to contain title")
		}
		if !strings.Contains(body, "ListTestItems") {
			t.Error("expected HTML to contain query name")
		}
		if !strings.Contains(body, "InsertTestItem") {
			t.Error("expected HTML to contain exec name")
		}
	})

	t.Run("JSON endpoint", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/docs/queries/json", nil)
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
			Generated string         `json:"generated"`
			Total     int            `json:"total"`
			Queries   []db.QueryInfo `json:"queries"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		if result.Total == 0 {
			t.Error("expected at least one query")
		}
		if result.Generated == "" {
			t.Error("expected generated timestamp")
		}

		// Check that our queries are in the result
		foundList := false
		foundInsert := false
		for _, q := range result.Queries {
			if q.Name == "ListTestItems" {
				foundList = true
			}
			if q.Name == "InsertTestItem" {
				foundInsert = true
			}
		}
		if !foundList {
			t.Error("expected to find ListTestItems query")
		}
		if !foundInsert {
			t.Error("expected to find InsertTestItem query")
		}
	})
}

func TestFormatQueriesHTML(t *testing.T) {
	queries := []db.QueryInfo{
		{
			Name:        "GetUser",
			SQL:         "SELECT * FROM users WHERE id = $1",
			Type:        "select",
			File:        "web/users.go",
			Line:        42,
			Description: "Retrieves a user by ID",
		},
		{
			Name: "ListItems",
			SQL:  "SELECT * FROM items",
			Type: "select",
			File: "web/items.go",
			Line: 15,
		},
	}

	html := formatQueriesHTML(queries)

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(html, "GetUser") {
		t.Error("expected query name")
	}
	if !strings.Contains(html, "Retrieves a user by ID") {
		t.Error("expected description")
	}
	if !strings.Contains(html, "users.go") {
		t.Error("expected file name (basename)")
	}
	if !strings.Contains(html, "items.go") {
		t.Error("expected file name")
	}
}

func TestFormatQueriesHTML_EmptyQueries(t *testing.T) {
	html := formatQueriesHTML(nil)

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(html, "Total:") {
		t.Error("expected total label")
	}
}

func TestFormatQueriesHTML_GroupsByFile(t *testing.T) {
	queries := []db.QueryInfo{
		{Name: "Q1", File: "pkg/a.go", SQL: "SELECT 1"},
		{Name: "Q2", File: "pkg/a.go", SQL: "SELECT 2"},
		{Name: "Q3", File: "pkg/b.go", SQL: "SELECT 3"},
	}

	html := formatQueriesHTML(queries)

	// Should group by file basename
	if !strings.Contains(html, "a.go") {
		t.Error("expected file group a.go")
	}
	if !strings.Contains(html, "b.go") {
		t.Error("expected file group b.go")
	}
}
