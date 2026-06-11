package auth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
	"github.com/graham/tog/web"
	"github.com/graham/tog/web/auth"
)

// setupTestDB creates an in-memory database with auth tables.
func setupTestDB(t *testing.T) *db.DB {
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
		INSERT INTO users (id, email, is_admin, is_active) VALUES (1, 'test@example.com', 0, 1);
		INSERT INTO users (id, email, is_admin, is_active) VALUES (2, 'admin@example.com', 1, 1);
		INSERT INTO users (id, email, is_admin, is_active) VALUES (3, 'inactive@example.com', 0, 0);
		INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('valid-token', 'session', 1, 1);
		INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('admin-token', 'session', 2, 1);
		INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('inactive-session', 'session', 1, 0);
		INSERT INTO sessions (key_value, key_type, for_user, is_active) VALUES ('inactive-user-token', 'session', 3, 1);
	`)
	if err != nil {
		t.Fatalf("failed to create tables: %v", err)
	}

	return database
}

// TestRequiresAuth tests the auth middleware with various scenarios.
func TestRequiresAuth(t *testing.T) {
	database := setupTestDB(t)

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	// Create a test handler that returns user info
	protectedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if !ok {
			http.Error(w, "no user in context", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":    user.ID,
			"email": user.Email,
		})
	})

	// Build router with auth middleware
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Group(func(r chi.Router) {
		r.Use(auth.RequiresAuth(queries))
		r.Get("/protected", protectedHandler)
	})

	t.Run("no auth returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("valid cookie allows access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.AddCookie(&http.Cookie{Name: "session_key", Value: "valid-token"})
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result map[string]any
		json.NewDecoder(rec.Body).Decode(&result)
		if result["email"] != "test@example.com" {
			t.Errorf("expected test@example.com, got %v", result["email"])
		}
	})

	t.Run("valid bearer token allows access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result map[string]any
		json.NewDecoder(rec.Body).Decode(&result)
		if result["email"] != "test@example.com" {
			t.Errorf("expected test@example.com, got %v", result["email"])
		}
	})

	t.Run("invalid token returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("inactive session returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer inactive-session")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("inactive user returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer inactive-user-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("admin token provides admin user", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}

		var result map[string]any
		json.NewDecoder(rec.Body).Decode(&result)
		if result["email"] != "admin@example.com" {
			t.Errorf("expected admin@example.com, got %v", result["email"])
		}
	})
}

// TestUserFromContext tests the context helper functions.
func TestUserFromContext(t *testing.T) {
	database := setupTestDB(t)

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))

	// Route with auth - should have user
	r.Group(func(r chi.Router) {
		r.Use(auth.RequiresAuth(queries))
		r.Get("/with-user", func(w http.ResponseWriter, r *http.Request) {
			user, ok := auth.UserFromContext(r.Context())
			if !ok {
				http.Error(w, "no user", http.StatusInternalServerError)
				return
			}
			w.Write([]byte(user.Email))
		})
	})

	// Route without auth - should not have user
	r.Get("/without-user", func(w http.ResponseWriter, r *http.Request) {
		_, ok := auth.UserFromContext(r.Context())
		if ok {
			http.Error(w, "unexpected user", http.StatusInternalServerError)
			return
		}
		w.Write([]byte("no user"))
	})

	t.Run("UserFromContext returns user when authenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/with-user", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "test@example.com" {
			t.Errorf("expected test@example.com, got %s", rec.Body.String())
		}
	})

	t.Run("UserFromContext returns false when not authenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/without-user", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "no user" {
			t.Errorf("expected 'no user', got %s", rec.Body.String())
		}
	})
}

// TestMustUserFromContext tests that MustUserFromContext panics when no user.
func TestMustUserFromContext(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("panicked"))
			}
		}()
		auth.MustUserFromContext(r.Context())
		w.Write([]byte("no panic"))
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Body.String() != "panicked" {
		t.Errorf("expected panic, got %s", rec.Body.String())
	}
}

// TestSessionExpiration tests that expired sessions are rejected.
func TestSessionExpiration(t *testing.T) {
	database := setupTestDB(t)

	// Add an expired session
	_, err := database.DB.Exec(`
		INSERT INTO sessions (key_value, key_type, for_user, is_active, expires_at)
		VALUES ('expired-token', 'session', 1, 1, 1)
	`)
	if err != nil {
		t.Fatalf("failed to insert expired session: %v", err)
	}

	// Add a non-expiring session (expires_at = 0)
	_, err = database.DB.Exec(`
		INSERT INTO sessions (key_value, key_type, for_user, is_active, expires_at)
		VALUES ('never-expires', 'session', 1, 1, 0)
	`)
	if err != nil {
		t.Fatalf("failed to insert non-expiring session: %v", err)
	}

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Use(auth.RequiresAuth(queries))
	r.Get("/protected", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	t.Run("expired session returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer expired-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401, got %d", rec.Code)
		}
	})

	t.Run("non-expiring session allows access", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/protected", nil)
		req.Header.Set("Authorization", "Bearer never-expires")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

// TestOptionalAuth tests the optional auth middleware.
func TestOptionalAuth(t *testing.T) {
	database := setupTestDB(t)

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Use(auth.OptionalAuth(queries))
	r.Get("/optional", func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFromContext(r.Context())
		if ok {
			w.Write([]byte("user:" + user.Email))
		} else {
			w.Write([]byte("anonymous"))
		}
	})

	t.Run("allows access without auth", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/optional", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "anonymous" {
			t.Errorf("expected 'anonymous', got %s", rec.Body.String())
		}
	})

	t.Run("loads user when authenticated", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/optional", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "user:test@example.com" {
			t.Errorf("expected 'user:test@example.com', got %s", rec.Body.String())
		}
	})

	t.Run("allows access with invalid token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/optional", nil)
		req.Header.Set("Authorization", "Bearer invalid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
		if rec.Body.String() != "anonymous" {
			t.Errorf("expected 'anonymous', got %s", rec.Body.String())
		}
	})
}

// TestRequiresScope tests the scope middleware.
func TestRequiresScope(t *testing.T) {
	database := setupTestDB(t)

	// Add sessions with scopes
	_, err := database.DB.Exec(`
		INSERT INTO sessions (key_value, key_type, for_user, is_active, scopes)
		VALUES ('read-token', 'session', 1, 1, 'read');
		INSERT INTO sessions (key_value, key_type, for_user, is_active, scopes)
		VALUES ('multi-scope-token', 'session', 1, 1, 'read,write,delete');
		INSERT INTO sessions (key_value, key_type, for_user, is_active, scopes)
		VALUES ('no-scope-token', 'session', 1, 1, '');
	`)
	if err != nil {
		t.Fatalf("failed to insert sessions with scopes: %v", err)
	}

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Use(auth.RequiresAuth(queries))

	// Route requiring single scope
	r.Group(func(r chi.Router) {
		r.Use(auth.RequiresScope("write"))
		r.Get("/needs-write", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})
	})

	// Route requiring any of multiple scopes
	r.Group(func(r chi.Router) {
		r.Use(auth.RequiresAnyScope("admin", "write"))
		r.Get("/needs-any", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})
	})

	// Route requiring all scopes
	r.Group(func(r chi.Router) {
		r.Use(auth.RequiresAllScopes("read", "write"))
		r.Get("/needs-all", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok"))
		})
	})

	t.Run("RequiresScope rejects missing scope", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/needs-write", nil)
		req.Header.Set("Authorization", "Bearer read-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("RequiresScope allows matching scope", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/needs-write", nil)
		req.Header.Set("Authorization", "Bearer multi-scope-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("RequiresAnyScope allows if one scope matches", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/needs-any", nil)
		req.Header.Set("Authorization", "Bearer multi-scope-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("RequiresAnyScope rejects if no scope matches", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/needs-any", nil)
		req.Header.Set("Authorization", "Bearer read-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("RequiresAllScopes allows if all scopes present", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/needs-all", nil)
		req.Header.Set("Authorization", "Bearer multi-scope-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("RequiresAllScopes rejects if missing any scope", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/needs-all", nil)
		req.Header.Set("Authorization", "Bearer read-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("empty scopes rejected for any scope check", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/needs-write", nil)
		req.Header.Set("Authorization", "Bearer no-scope-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})
}

// TestRequiresAdmin tests the admin middleware.
func TestRequiresAdmin(t *testing.T) {
	database := setupTestDB(t)

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Use(auth.RequiresAuth(queries))
	r.Use(auth.RequiresAdmin())
	r.Get("/admin-only", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("admin access granted"))
	})

	t.Run("admin user allowed", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin-only", nil)
		req.Header.Set("Authorization", "Bearer admin-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("non-admin user rejected", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/admin-only", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})
}

// TestSessionFromContext tests session context helpers.
func TestSessionFromContext(t *testing.T) {
	database := setupTestDB(t)

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Use(auth.RequiresAuth(queries))
	r.Get("/session", func(w http.ResponseWriter, r *http.Request) {
		session, ok := auth.SessionFromContext(r.Context())
		if !ok {
			http.Error(w, "no session", http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"key_type":  session.KeyType,
			"is_active": session.IsActive,
		})
	})

	t.Run("session available in context", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/session", nil)
		req.Header.Set("Authorization", "Bearer valid-token")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var result map[string]any
		json.NewDecoder(rec.Body).Decode(&result)
		if result["key_type"] != "session" {
			t.Errorf("expected key_type 'session', got %v", result["key_type"])
		}
	})
}

// TestTokenPrefixes tests that token prefixes are recognized.
func TestTokenPrefixes(t *testing.T) {
	database := setupTestDB(t)

	// Add sessions with prefixed tokens
	_, err := database.DB.Exec(`
		INSERT INTO sessions (key_value, key_type, for_user, is_active)
		VALUES ('sess_abc123', 'session', 1, 1);
		INSERT INTO sessions (key_value, key_type, for_user, is_active)
		VALUES ('key_xyz789', 'api_key', 1, 1);
	`)
	if err != nil {
		t.Fatalf("failed to insert prefixed sessions: %v", err)
	}

	queries, err := auth.RegisterQueries(database)
	if err != nil {
		t.Fatalf("failed to register queries: %v", err)
	}

	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Use(auth.RequiresAuth(queries))
	r.Get("/check", func(w http.ResponseWriter, r *http.Request) {
		session, _ := auth.SessionFromContext(r.Context())
		json.NewEncoder(w).Encode(map[string]any{
			"is_session": session.IsSessionToken(),
			"is_api_key": session.IsAPIKey(),
		})
	})

	t.Run("session token recognized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/check", nil)
		req.Header.Set("Authorization", "Bearer sess_abc123")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var result map[string]any
		json.NewDecoder(rec.Body).Decode(&result)
		if result["is_session"] != true {
			t.Errorf("expected is_session true")
		}
	})

	t.Run("api key recognized", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/check", nil)
		req.Header.Set("Authorization", "Bearer key_xyz789")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", rec.Code)
		}

		var result map[string]any
		json.NewDecoder(rec.Body).Decode(&result)
		if result["is_api_key"] != true {
			t.Errorf("expected is_api_key true")
		}
	})
}
