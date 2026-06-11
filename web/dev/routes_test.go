package dev

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
)

func setupTestDB(t *testing.T) *db.DB {
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
	t.Cleanup(func() { mgr.Close() })

	database := mgr.Default()

	// Create users and sessions tables
	_, err = database.DB.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT,
			is_admin INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1
		)
	`)
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	_, err = database.DB.Exec(`
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key_value TEXT NOT NULL,
			key_type TEXT NOT NULL,
			for_user INTEGER NOT NULL,
			is_active INTEGER DEFAULT 1,
			expires_at INTEGER DEFAULT 0,
			FOREIGN KEY (for_user) REFERENCES users(id)
		)
	`)
	if err != nil {
		t.Fatalf("failed to create sessions table: %v", err)
	}

	return database
}

func TestNewRoutes(t *testing.T) {
	database := setupTestDB(t)
	routes := NewRoutes(database)

	if routes == nil {
		t.Fatal("NewRoutes returned nil")
	}
	if routes.database != database {
		t.Error("database not set correctly")
	}
	if routes.lookupUser == nil {
		t.Error("lookupUser should be set to default")
	}
	if routes.createSession == nil {
		t.Error("createSession should be set to default")
	}
	if routes.createUser == nil {
		t.Error("createUser should be set to default")
	}
}

func TestNewRoutesWithConfig(t *testing.T) {
	database := setupTestDB(t)

	customLookupCalled := false
	customSessionCalled := false
	customCreateCalled := false

	cfg := Config{
		Database: database,
		LookupUser: func(email string) (any, string, error) {
			customLookupCalled = true
			return 1, email, nil
		},
		CreateSession: func(userID any) (string, error) {
			customSessionCalled = true
			return "test_session", nil
		},
		CreateUser: func(email string) (any, error) {
			customCreateCalled = true
			return 1, nil
		},
	}

	routes := NewRoutesWithConfig(cfg)

	// Test that custom functions are used
	routes.lookupUser("test@example.com")
	if !customLookupCalled {
		t.Error("custom lookupUser should be called")
	}

	routes.createSession(1)
	if !customSessionCalled {
		t.Error("custom createSession should be called")
	}

	routes.createUser("test@example.com")
	if !customCreateCalled {
		t.Error("custom createUser should be called")
	}
}

func TestNewRoutesWithConfig_NilFunctions(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Database: database,
		// All functions nil - should use defaults
	}

	routes := NewRoutesWithConfig(cfg)

	if routes.lookupUser == nil {
		t.Error("lookupUser should default to defaultLookupUser")
	}
	if routes.createSession == nil {
		t.Error("createSession should default to defaultCreateSession")
	}
	if routes.createUser == nil {
		t.Error("createUser should default to defaultCreateUser")
	}
}

func TestRoutes_Mount(t *testing.T) {
	database := setupTestDB(t)
	routes := NewRoutes(database)

	router := chi.NewRouter()
	router.Route("/dev", routes.Mount())

	// Test that routes are mounted
	req := httptest.NewRequest("GET", "/dev/assume?email=test@example.com", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Should get 404 (user not found) not 405 (method not allowed)
	if rec.Code == http.StatusMethodNotAllowed {
		t.Error("route should be mounted")
	}
}

func TestRoutes_Assume(t *testing.T) {
	database := setupTestDB(t)

	// Create a test user
	_, err := database.DB.Exec("INSERT INTO users (email, is_active) VALUES ('test@example.com', 1)")
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}

	routes := NewRoutes(database)
	router := chi.NewRouter()
	router.Route("/dev", routes.Mount())

	t.Run("missing email parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dev/assume", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("user not found", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dev/assume?email=nonexistent@example.com", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotFound {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusNotFound)
		}
	})

	t.Run("successful assume", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dev/assume?email=test@example.com", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}

		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if result["message"] != "session created" {
			t.Errorf("message = %v, want 'session created'", result["message"])
		}
		if result["email"] != "test@example.com" {
			t.Errorf("email = %v, want 'test@example.com'", result["email"])
		}
		if result["session_key"] == nil || result["session_key"] == "" {
			t.Error("session_key should be set")
		}

		// Check cookie
		cookies := rec.Result().Cookies()
		found := false
		for _, c := range cookies {
			if c.Name == "session_key" {
				found = true
				if !c.HttpOnly {
					t.Error("cookie should be HttpOnly")
				}
			}
		}
		if !found {
			t.Error("session_key cookie should be set")
		}
	})
}

func TestRoutes_Assume_SessionCreationError(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Database: database,
		LookupUser: func(email string) (any, string, error) {
			return 1, email, nil
		},
		CreateSession: func(userID any) (string, error) {
			return "", errors.New("session creation failed")
		},
	}

	routes := NewRoutesWithConfig(cfg)
	router := chi.NewRouter()
	router.Route("/dev", routes.Mount())

	req := httptest.NewRequest("GET", "/dev/assume?email=test@example.com", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRoutes_CreateAndAssume(t *testing.T) {
	database := setupTestDB(t)
	routes := NewRoutes(database)
	router := chi.NewRouter()
	router.Route("/dev", routes.Mount())

	t.Run("missing email parameter", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dev/create_and_assume", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusBadRequest)
		}
	})

	t.Run("creates new user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/dev/create_and_assume?email=newuser@example.com", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}

		var result map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}

		if result["email"] != "newuser@example.com" {
			t.Errorf("email = %v, want 'newuser@example.com'", result["email"])
		}
	})

	t.Run("assumes existing user", func(t *testing.T) {
		// Second call should find existing user
		req := httptest.NewRequest("GET", "/dev/create_and_assume?email=newuser@example.com", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}
	})
}

func TestRoutes_CreateAndAssume_CreateUserError(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Database: database,
		LookupUser: func(email string) (any, string, error) {
			return nil, "", errors.New("not found")
		},
		CreateUser: func(email string) (any, error) {
			return nil, errors.New("user creation failed")
		},
	}

	routes := NewRoutesWithConfig(cfg)
	router := chi.NewRouter()
	router.Route("/dev", routes.Mount())

	req := httptest.NewRequest("GET", "/dev/create_and_assume?email=test@example.com", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRoutes_CreateAndAssume_SessionError(t *testing.T) {
	database := setupTestDB(t)

	cfg := Config{
		Database: database,
		LookupUser: func(email string) (any, string, error) {
			return 1, email, nil
		},
		CreateSession: func(userID any) (string, error) {
			return "", errors.New("session failed")
		},
	}

	routes := NewRoutesWithConfig(cfg)
	router := chi.NewRouter()
	router.Route("/dev", routes.Mount())

	req := httptest.NewRequest("GET", "/dev/create_and_assume?email=test@example.com", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestRoutes_Schema(t *testing.T) {
	database := setupTestDB(t)
	routes := NewRoutes(database)
	router := chi.NewRouter()
	router.Route("/dev", routes.Mount())

	req := httptest.NewRequest("GET", "/dev/schema", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}

	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	tables, ok := result["tables"]
	if !ok {
		t.Error("expected 'tables' in response")
	}

	tableList, ok := tables.([]any)
	if !ok {
		t.Fatalf("tables should be an array, got %T", tables)
	}

	// Should have at least users and sessions tables
	if len(tableList) < 2 {
		t.Errorf("expected at least 2 tables, got %d", len(tableList))
	}
}

func TestGenerateToken(t *testing.T) {
	token1 := generateToken()
	token2 := generateToken()

	if !strings.HasPrefix(token1, "sess_") {
		t.Errorf("token should start with 'sess_', got %q", token1)
	}

	// Should be unique
	if token1 == token2 {
		t.Error("tokens should be unique")
	}

	// Should be appropriate length (sess_ + 64 hex chars)
	if len(token1) != 5+64 {
		t.Errorf("token length = %d, want %d", len(token1), 5+64)
	}
}
