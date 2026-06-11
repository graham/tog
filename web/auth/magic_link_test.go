package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/app"
	"github.com/graham/tog/db"
	"github.com/graham/tog/web"
)

func setupMagicLinkTestDB(t *testing.T) (*db.DB, *Queries) {
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

func TestRoutes_RequestMagicLink_Success(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'test@example.com', 1)`)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true, TokenLifetime: 15},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com"}`
	req := httptest.NewRequest("POST", "/auth/magic-link", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	// Without email sender, should return token
	if result["token"] == nil || result["token"] == "" {
		t.Error("expected token in response")
	}
	token := result["token"].(string)
	if !strings.HasPrefix(token, TokenPrefixMagicLink) {
		t.Errorf("expected token to start with %s, got %s", TokenPrefixMagicLink, token)
	}
}

func TestRoutes_RequestMagicLink_UserNotFound(t *testing.T) {
	_, queries := setupMagicLinkTestDB(t)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"nonexistent@example.com"}`
	req := httptest.NewRequest("POST", "/auth/magic-link", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should still return 200 to prevent enumeration
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	// Should return generic message (no token since user doesn't exist)
	if result["message"] == nil {
		t.Error("expected message in response")
	}
}

func TestRoutes_RequestMagicLink_InactiveUser(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'inactive@example.com', 0)`)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"inactive@example.com"}`
	req := httptest.NewRequest("POST", "/auth/magic-link", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should still return 200 to prevent enumeration
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRoutes_RequestMagicLink_MissingEmail(t *testing.T) {
	_, queries := setupMagicLinkTestDB(t)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{}`
	req := httptest.NewRequest("POST", "/auth/magic-link", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_RequestMagicLink_InvalidJSON(t *testing.T) {
	_, queries := setupMagicLinkTestDB(t)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `not valid json`
	req := httptest.NewRequest("POST", "/auth/magic-link", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_RequestMagicLink_RateLimited(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'test@example.com', 1)`)

	// Insert a recent magic link (created now)
	expiresAt := time.Now().Add(15 * time.Minute).Unix()
	database.Exec(`INSERT INTO magic_links (token, email, expires_at, for_user) VALUES ('ml_existing', 'test@example.com', ?, 1)`, expiresAt)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true, RateLimitSecs: 30},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com"}`
	req := httptest.NewRequest("POST", "/auth/magic-link", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	// Should return 200 with message (no new token)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	// Should have message but no token (rate limited)
	if result["message"] == nil {
		t.Error("expected message in response")
	}
	if result["token"] != nil {
		t.Error("should not have token when rate limited")
	}
}

func TestRoutes_VerifyMagicLink_Success(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'test@example.com', 1)`)

	// Insert a valid magic link
	expiresAt := time.Now().Add(15 * time.Minute).Unix()
	database.Exec(`INSERT INTO magic_links (token, email, expires_at, for_user, used_at) VALUES ('ml_validtoken123', 'test@example.com', ?, 1, 0)`, expiresAt)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/magic-link/verify?token=ml_validtoken123", nil)
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
	if result["session_key"] == nil {
		t.Error("expected session_key in response")
	}
}

func TestRoutes_VerifyMagicLink_MissingToken(t *testing.T) {
	_, queries := setupMagicLinkTestDB(t)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/magic-link/verify", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_VerifyMagicLink_InvalidToken(t *testing.T) {
	_, queries := setupMagicLinkTestDB(t)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/magic-link/verify?token=invalid_token", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_VerifyMagicLink_AlreadyUsed(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'test@example.com', 1)`)

	// Insert an already-used magic link
	expiresAt := time.Now().Add(15 * time.Minute).Unix()
	usedAt := time.Now().Unix()
	database.Exec(`INSERT INTO magic_links (token, email, expires_at, for_user, used_at) VALUES ('ml_usedtoken', 'test@example.com', ?, 1, ?)`, expiresAt, usedAt)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/magic-link/verify?token=ml_usedtoken", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_VerifyMagicLink_Expired(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'test@example.com', 1)`)

	// Insert an expired magic link
	expiresAt := time.Now().Add(-1 * time.Hour).Unix() // expired 1 hour ago
	database.Exec(`INSERT INTO magic_links (token, email, expires_at, for_user, used_at) VALUES ('ml_expiredtoken', 'test@example.com', ?, 1, 0)`, expiresAt)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/magic-link/verify?token=ml_expiredtoken", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestRoutes_VerifyMagicLink_InactiveUser(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'test@example.com', 0)`)

	// Insert a valid magic link
	expiresAt := time.Now().Add(15 * time.Minute).Unix()
	database.Exec(`INSERT INTO magic_links (token, email, expires_at, for_user, used_at) VALUES ('ml_inactiveuser', 'test@example.com', ?, 1, 0)`, expiresAt)

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{Enabled: true},
	}
	routes := NewRoutesWithAuth(queries, true, DefaultCookieConfig(), authConfig)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	req := httptest.NewRequest("GET", "/auth/magic-link/verify?token=ml_inactiveuser", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rec.Code)
	}
}

// Mock email sender for testing
type mockEmailSender struct {
	lastEmail     string
	lastVerifyURL string
	sendError     error
}

func (m *mockEmailSender) SendMagicLink(ctx context.Context, email, verifyURL string, expiresInMinutes int) error {
	m.lastEmail = email
	m.lastVerifyURL = verifyURL
	return m.sendError
}

func TestRoutes_RequestMagicLink_WithEmailSender(t *testing.T) {
	database, queries := setupMagicLinkTestDB(t)

	database.Exec(`INSERT INTO users (id, email, is_active) VALUES (1, 'test@example.com', 1)`)

	mockSender := &mockEmailSender{}

	authConfig := &app.AuthConfig{
		MagicLink: app.MagicLinkAuthConfig{
			Enabled:       true,
			TokenLifetime: 15,
			BaseURL:       "https://example.com",
		},
	}
	routes := NewRoutesWithEmail(queries, true, DefaultCookieConfig(), authConfig, mockSender)

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Route("/auth", routes.Mount())

	body := `{"email":"test@example.com"}`
	req := httptest.NewRequest("POST", "/auth/magic-link", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Email should have been sent
	if mockSender.lastEmail != "test@example.com" {
		t.Errorf("expected email to test@example.com, got %s", mockSender.lastEmail)
	}
	if !strings.HasPrefix(mockSender.lastVerifyURL, "https://example.com") {
		t.Errorf("expected verify URL with base URL, got %s", mockSender.lastVerifyURL)
	}

	var result map[string]any
	json.NewDecoder(rec.Body).Decode(&result)

	// Should return generic message (not token)
	if result["token"] != nil {
		t.Error("should not expose token when email is sent")
	}
	if result["message"] == nil {
		t.Error("expected message in response")
	}
}

