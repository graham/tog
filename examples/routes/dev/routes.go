package dev

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
	"github.com/graham/tog/web"
	"github.com/graham/tog/web/auth"
)

// Routes provides development-only endpoints.
type Routes struct {
	dbm *db.Manager
}

// NewRoutes creates a new dev Routes instance.
func NewRoutes(dbm *db.Manager) *Routes {
	return &Routes{dbm: dbm}
}

// Mount returns a function that mounts dev routes on a chi router.
func (r *Routes) Mount() func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/assume", r.assume)
		router.Get("/create_and_assume", r.createAndAssume)
		router.Route("/schema", web.MultiSchemaRoutes(r.dbm))
	}
}

// assume creates a session for an existing user by email.
// Usage: GET /dev/assume?email=user@example.com
func (r *Routes) assume(w http.ResponseWriter, req *http.Request) {
	email := req.URL.Query().Get("email")
	if email == "" {
		web.WriteAppError(w, req, web.ErrBadRequest("email query parameter required", nil))
		return
	}

	// Look up user by email
	var userID int
	err := r.dbm.Default().DB.QueryRow("SELECT id FROM users WHERE email = ?", email).Scan(&userID)
	if err != nil {
		web.WriteAppError(w, req, web.ErrNotFound("User not found", err))
		return
	}

	// Create session
	sessionKey, err := r.createSession(userID)
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create session", err))
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_key",
		Value:    sessionKey,
		Path:     "/",
		HttpOnly: true,
	})

	web.WriteJSON(w, map[string]any{
		"message":     "session created",
		"session_key": sessionKey,
		"user_id":     userID,
		"email":       email,
	})
}

// createAndAssume creates a user if they don't exist, then creates a session.
// Usage: GET /dev/create_and_assume?email=user@example.com&password=secret123
func (r *Routes) createAndAssume(w http.ResponseWriter, req *http.Request) {
	email := req.URL.Query().Get("email")
	if email == "" {
		web.WriteAppError(w, req, web.ErrBadRequest("email query parameter required", nil))
		return
	}
	password := req.URL.Query().Get("password")

	// Hash password if provided
	var passwordHash string
	if password != "" {
		var err error
		passwordHash, err = auth.HashPassword(password)
		if err != nil {
			web.WriteAppError(w, req, web.ErrInternal("Failed to hash password", err))
			return
		}
	}

	// Try to find existing user
	var userID int
	err := r.dbm.Default().DB.QueryRow("SELECT id FROM users WHERE email = ?", email).Scan(&userID)
	if err != nil {
		// User doesn't exist, create them
		result, err := r.dbm.Default().DB.Exec(
			"INSERT INTO users (email, password_hash, is_admin, is_active) VALUES (?, ?, 0, 1)",
			email, passwordHash,
		)
		if err != nil {
			web.WriteAppError(w, req, web.ErrInternal("Failed to create user", err))
			return
		}
		id, _ := result.LastInsertId()
		userID = int(id)
	} else if password != "" {
		// User exists and password provided, update the password
		_, err := r.dbm.Default().DB.Exec(
			"UPDATE users SET password_hash = ? WHERE id = ?",
			passwordHash, userID,
		)
		if err != nil {
			web.WriteAppError(w, req, web.ErrInternal("Failed to update password", err))
			return
		}
	}

	// Create session
	sessionKey, err := r.createSession(userID)
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create session", err))
		return
	}

	// Set cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_key",
		Value:    sessionKey,
		Path:     "/",
		HttpOnly: true,
	})

	web.WriteJSON(w, map[string]any{
		"message":     "session created",
		"session_key": sessionKey,
		"user_id":     userID,
		"email":       email,
	})
}

// createSession creates a session for a user and returns the session key.
func (r *Routes) createSession(userID int) (string, error) {
	sessionKey := generateToken()

	_, err := r.dbm.Default().DB.Exec(
		"INSERT INTO sessions (key_value, key_type, for_user, is_active, expires_at) VALUES (?, 'session', ?, 1, 0)",
		sessionKey, userID,
	)
	if err != nil {
		return "", err
	}

	return sessionKey, nil
}

// generateToken generates a random session token.
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return "sess_" + hex.EncodeToString(bytes)
}
