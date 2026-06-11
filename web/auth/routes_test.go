package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/app"
	"github.com/graham/tog/db"
	"github.com/graham/tog/web"
)

// Pre-computed bcrypt hash of "testpass123" at cost 12
// Avoids slow bcrypt.GenerateFromPassword in tests
const testPasswordHash = "$2a$12$/LWPcUufGsMXPrkaA4Ull.yj74EnLjPY2FKCfbxcv3yhmBnXUtPMy"
const testPassword = "testpass123"

func setupRoutesTestDB(t *testing.T) (*db.DB, *Queries) {
	t.Helper()

	cfg := &db.Config{
		Databases: map[string]db.DatabaseConfig{
			"primary": {
				Driver: "sqlite3",
				DSN:    ":memory:",
			},
		},
		Default: "primary",
	}

	dbm, err := db.NewManager(cfg)
	if err != nil {
		t.Fatalf("failed to create db manager: %v", err)
	}
	t.Cleanup(func() { dbm.Close() })

	database := dbm.Default()

	// Create tables
	_, err = database.DB.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT DEFAULT '',
			is_admin INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1
		);
		CREATE TABLE sessions (
			id INTEGER PRIMARY KEY,
			key_value TEXT UNIQUE NOT NULL,
			key_type TEXT DEFAULT 'session',
			scopes TEXT DEFAULT '',
			created_at INTEGER DEFAULT 0,
			expires_at INTEGER DEFAULT 0,
			is_active INTEGER DEFAULT 1,
			for_user INTEGER NOT NULL
		);
		CREATE TABLE magic_links (
			id INTEGER PRIMARY KEY,
			token TEXT UNIQUE NOT NULL,
			email TEXT NOT NULL,
			created_at INTEGER DEFAULT (strftime('%s', 'now')),
			expires_at INTEGER NOT NULL,
			used_at INTEGER DEFAULT 0,
			for_user INTEGER NOT NULL
		);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	queries, err := RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	return database, queries
}

func TestNewRoutes(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	routes := NewRoutes(queries, true)
	if routes == nil {
		t.Fatal("NewRoutes returned nil")
	}
	if !routes.devMode {
		t.Error("expected devMode to be true")
	}
}

func TestNewRoutesWithConfig(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	config := CookieConfig{
		Name:   "custom_session",
		Path:   "/app",
		MaxAge: 3600,
	}

	routes := NewRoutesWithConfig(queries, false, config)
	if routes == nil {
		t.Fatal("NewRoutesWithConfig returned nil")
	}
	if routes.cookieConfig.Name != "custom_session" {
		t.Errorf("expected cookie name 'custom_session', got %s", routes.cookieConfig.Name)
	}
}

func TestNewRoutesWithAuth(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	config := CookieConfig{Name: "test"}
	authConfig := &app.AuthConfig{
		Password: app.PasswordAuthConfig{Enabled: true},
	}

	routes := NewRoutesWithAuth(queries, true, config, authConfig)
	if routes == nil {
		t.Fatal("NewRoutesWithAuth returned nil")
	}
	if routes.authConfig.Password.Enabled != true {
		t.Error("expected password auth to be enabled")
	}
}

func TestNewRoutesWithEmail(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	config := CookieConfig{Name: "test"}
	authConfig := &app.AuthConfig{}

	routes := NewRoutesWithEmail(queries, true, config, authConfig, nil)
	if routes == nil {
		t.Fatal("NewRoutesWithEmail returned nil")
	}
}

func TestRoutes_Mount(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{
		Password:  app.PasswordAuthConfig{Enabled: true},
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}

	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	// Test that routes are mounted - whoami should always be available
	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for /whoami, got %d", rec.Code)
	}
}

func TestRoutes_Whoami_Unauthenticated(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	if result["authenticated"] != false {
		t.Errorf("expected authenticated=false, got %v", result["authenticated"])
	}
}

func TestRoutes_Whoami_Authenticated(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	// Insert a user and session
	database.Exec(`INSERT INTO users (id, email, is_admin) VALUES (1, 'test@example.com', 0)`)
	database.Exec(`INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('test-token', 'session', 1, 1)`)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/whoami", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	if result["authenticated"] != true {
		t.Errorf("expected authenticated=true, got %v", result["authenticated"])
	}
	if result["email"] != "test@example.com" {
		t.Errorf("expected email=test@example.com, got %v", result["email"])
	}
}

func TestRoutes_Logout_NotLoggedIn(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	if result["message"] != "not logged in" {
		t.Errorf("expected 'not logged in', got %v", result["message"])
	}
}

func TestRoutes_Logout_LoggedIn(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	database.Exec(`INSERT INTO users (id, email) VALUES (1, 'test@example.com')`)
	database.Exec(`INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('logout-token', 'session', 1, 1)`)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer logout-token")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	if result["message"] != "logged out" {
		t.Errorf("expected 'logged out', got %v", result["message"])
	}

	// Check that the cookie was cleared
	cookies := rec.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "session_key" && c.MaxAge == -1 {
			return // Cookie was cleared correctly
		}
	}
}

func TestRoutes_LogoutAll(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	database.Exec(`INSERT INTO users (id, email) VALUES (1, 'test@example.com')`)
	database.Exec(`INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('token1', 'session', 1, 1)`)
	database.Exec(`INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('token2', 'session', 1, 1)`)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("POST", "/auth/logout-all", nil)
	req.Header.Set("Authorization", "Bearer token1")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	if result["message"] != "all sessions invalidated" {
		t.Errorf("expected 'all sessions invalidated', got %v", result["message"])
	}
}

func TestRoutes_Login_Success(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	// Create user with password
	hash := testPasswordHash
	database.Exec(`INSERT INTO users (id, email, password_hash, is_active) VALUES (1, 'test@example.com', ?, 1)`, hash)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com","password":"testpass123"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	if result["message"] != "logged in" {
		t.Errorf("expected 'logged in', got %v", result["message"])
	}
	if result["email"] != "test@example.com" {
		t.Errorf("expected email test@example.com, got %v", result["email"])
	}
	if result["session_key"] == nil || result["session_key"] == "" {
		t.Error("expected session_key to be returned")
	}
}

func TestRoutes_Login_InvalidCredentials(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	hash := testPasswordHash
	database.Exec(`INSERT INTO users (id, email, password_hash, is_active) VALUES (1, 'test@example.com', ?, 1)`, hash)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com","password":"wrongpassword"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRoutes_Login_UserNotFound(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"nonexistent@example.com","password":"testpass"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRoutes_Login_InactiveUser(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	hash := testPasswordHash
	database.Exec(`INSERT INTO users (id, email, password_hash, is_active) VALUES (1, 'test@example.com', ?, 0)`, hash)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com","password":"testpass123"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRoutes_Login_NoPassword(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	// User without password
	database.Exec(`INSERT INTO users (id, email, password_hash, is_active) VALUES (1, 'test@example.com', '', 1)`)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com","password":"anypassword"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestRoutes_Login_MissingFields(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com"}`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_Login_InvalidJSON(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `not valid json`
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_Assume(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'dev@example.com', 1)`)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/assume?email=dev@example.com", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	if result["message"] != "session created" {
		t.Errorf("expected 'session created', got %v", result["message"])
	}
	if result["email"] != "dev@example.com" {
		t.Errorf("expected email dev@example.com, got %v", result["email"])
	}

	// Check that session key starts with prefix
	sessionKey := result["session_key"].(string)
	if !strings.HasPrefix(sessionKey, TokenPrefixSession) {
		t.Errorf("expected session key to start with %s, got %s", TokenPrefixSession, sessionKey)
	}
}

func TestRoutes_Assume_MissingEmail(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/assume", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_Assume_UserNotFound(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/assume?email=nonexistent@example.com", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestRoutes_Assume_InactiveUser(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'inactive@example.com', 0)`)

	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/assume?email=inactive@example.com", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

func TestRoutes_Assume_NotInDevMode(t *testing.T) {
	_, queries := setupRoutesTestDB(t)

	// devMode = false
	authConfig := &app.AuthConfig{Password: app.PasswordAuthConfig{Enabled: true}}
	routes := NewRoutesWithAuth(queries, false, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/assume?email=test@example.com", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should return 404 because route is not mounted in non-dev mode
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 in non-dev mode, got %d", rec.Code)
	}
}

func TestRoutes_CreateSession(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	database.Exec(`INSERT INTO users (id, email) VALUES (1, 'test@example.com')`)

	routes := NewRoutes(queries, true)

	rec := httptest.NewRecorder()
	sessionKey, err := routes.CreateSession(rec, 1, "read,write", 0)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	if sessionKey == "" {
		t.Error("expected non-empty session key")
	}
	if !strings.HasPrefix(sessionKey, TokenPrefixSession) {
		t.Errorf("expected session key to start with %s", TokenPrefixSession)
	}

	// Check cookie was set
	cookies := rec.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "session_key" && c.Value == sessionKey {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}

func TestRoutes_CreateAPIKey(t *testing.T) {
	database, queries := setupRoutesTestDB(t)

	database.Exec(`INSERT INTO users (id, email) VALUES (1, 'test@example.com')`)

	routes := NewRoutes(queries, true)

	apiKey, err := routes.CreateAPIKey(1, "api:read")
	if err != nil {
		t.Fatalf("CreateAPIKey failed: %v", err)
	}

	if apiKey == "" {
		t.Error("expected non-empty API key")
	}
	if !strings.HasPrefix(apiKey, TokenPrefixAPIKey) {
		t.Errorf("expected API key to start with %s", TokenPrefixAPIKey)
	}
}

