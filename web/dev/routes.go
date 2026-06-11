// Package dev provides development-only endpoints for tog applications.
// These routes should only be enabled in development environments.
package dev

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
	"github.com/graham/tog/web"
)

// Config configures the dev routes behavior.
type Config struct {
	// Database is the primary database for schema/fallback operations.
	Database *db.DB

	// LookupUser is called to find a user by email.
	// If nil, uses the default lookup from the users table.
	// Returns (userID, email, error). userID can be int, string (UUID), etc.
	LookupUser func(email string) (any, string, error)

	// CreateSession is called to create a session for a user.
	// If nil, uses the default session creation.
	// userID is whatever was returned by LookupUser.
	// Returns (sessionKey, error).
	CreateSession func(userID any) (string, error)

	// CreateUser is called to create a new user (for create_and_assume).
	// If nil, uses the default user creation in the users table.
	// Returns (userID, error).
	CreateUser func(email string) (any, error)
}

// Routes provides development-only endpoints.
type Routes struct {
	database      *db.DB
	lookupUser    func(email string) (any, string, error)
	createSession func(userID any) (string, error)
	createUser    func(email string) (any, error)
}

// NewRoutes creates a new dev Routes instance with default configuration.
func NewRoutes(database *db.DB) *Routes {
	r := &Routes{database: database}
	r.lookupUser = r.defaultLookupUser
	r.createSession = r.defaultCreateSession
	r.createUser = r.defaultCreateUser
	return r
}

// NewRoutesWithConfig creates a new dev Routes instance with custom configuration.
func NewRoutesWithConfig(cfg Config) *Routes {
	r := &Routes{database: cfg.Database}

	if cfg.LookupUser != nil {
		r.lookupUser = cfg.LookupUser
	} else {
		r.lookupUser = r.defaultLookupUser
	}

	if cfg.CreateSession != nil {
		r.createSession = cfg.CreateSession
	} else {
		r.createSession = r.defaultCreateSession
	}

	if cfg.CreateUser != nil {
		r.createUser = cfg.CreateUser
	} else {
		r.createUser = r.defaultCreateUser
	}

	return r
}

// Mount returns a function that mounts dev routes on a chi router.
func (r *Routes) Mount() func(chi.Router) {
	return func(router chi.Router) {
		router.Get("/assume", r.assume)
		router.Get("/create_and_assume", r.createAndAssume)
		router.Get("/schema", r.schema)
	}
}

// defaultLookupUser looks up a user by email in the users table.
func (r *Routes) defaultLookupUser(email string) (any, string, error) {
	var userID int
	var userEmail string
	err := r.database.DB.QueryRow(
		r.database.Rebind("SELECT id, email FROM users WHERE email = $1"),
		email,
	).Scan(&userID, &userEmail)
	return userID, userEmail, err
}

// defaultCreateSession creates a session in the sessions table.
func (r *Routes) defaultCreateSession(userID any) (string, error) {
	sessionKey := generateToken()

	_, err := r.database.DB.Exec(
		r.database.Rebind("INSERT INTO sessions (key_value, key_type, for_user, is_active, expires_at) VALUES ($1, 'session', $2, 1, 0)"),
		sessionKey, userID,
	)
	if err != nil {
		return "", err
	}

	return sessionKey, nil
}

// defaultCreateUser creates a user in the users table.
func (r *Routes) defaultCreateUser(email string) (any, error) {
	result, err := r.database.DB.Exec(
		r.database.Rebind("INSERT INTO users (email, is_admin, is_active) VALUES ($1, 0, 1)"),
		email,
	)
	if err != nil {
		return nil, err
	}
	id, _ := result.LastInsertId()
	return int(id), nil
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
	userID, userEmail, err := r.lookupUser(email)
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
		"email":       userEmail,
	})
}

// createAndAssume creates a user if they don't exist, then creates a session.
// Usage: GET /dev/create_and_assume?email=user@example.com
func (r *Routes) createAndAssume(w http.ResponseWriter, req *http.Request) {
	email := req.URL.Query().Get("email")
	if email == "" {
		web.WriteAppError(w, req, web.ErrBadRequest("email query parameter required", nil))
		return
	}

	// Try to find existing user
	userID, userEmail, err := r.lookupUser(email)
	if err != nil {
		// User doesn't exist, create them
		userID, err = r.createUser(email)
		if err != nil {
			web.WriteAppError(w, req, web.ErrInternal("Failed to create user", err))
			return
		}
		userEmail = email
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
		"email":       userEmail,
	})
}

// schema returns the database schema information.
func (r *Routes) schema(w http.ResponseWriter, req *http.Request) {
	tables, err := r.database.SchemaInfo()
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to get schema", err))
		return
	}
	web.WriteJSON(w, map[string]any{
		"tables": tables,
	})
}

// generateToken generates a random session token.
func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return "sess_" + hex.EncodeToString(bytes)
}
