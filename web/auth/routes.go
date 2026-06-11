package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/app"
	"github.com/graham/tog/web"
)

// Routes provides authentication endpoints.
type Routes struct {
	queries      *Queries
	devMode      bool
	cookieConfig CookieConfig
	authConfig   *app.AuthConfig
	emailSender  MagicLinkEmailSender // optional, nil = return token in response
}

// NewRoutes creates a new Routes instance with default configuration.
// Set devMode to true to enable the /assume endpoint for development.
func NewRoutes(queries *Queries, devMode bool) *Routes {
	defaultConfig := app.DefaultAppConfig()
	return &Routes{
		queries:      queries,
		devMode:      devMode,
		cookieConfig: DefaultCookieConfig(),
		authConfig:   &defaultConfig.Auth,
	}
}

// NewRoutesWithConfig creates a new Routes instance with custom cookie configuration.
func NewRoutesWithConfig(queries *Queries, devMode bool, config CookieConfig) *Routes {
	defaultConfig := app.DefaultAppConfig()
	return &Routes{
		queries:      queries,
		devMode:      devMode,
		cookieConfig: config,
		authConfig:   &defaultConfig.Auth,
	}
}

// NewRoutesWithAuth creates a new Routes instance with full configuration.
func NewRoutesWithAuth(queries *Queries, devMode bool, cookieConfig CookieConfig, authConfig *app.AuthConfig) *Routes {
	return &Routes{
		queries:      queries,
		devMode:      devMode,
		cookieConfig: cookieConfig,
		authConfig:   authConfig,
	}
}

// NewRoutesWithEmail creates a new Routes instance with email sending support.
// When emailSender is non-nil, magic link requests will send emails instead of
// returning tokens in the response.
func NewRoutesWithEmail(queries *Queries, devMode bool, cookieConfig CookieConfig, authConfig *app.AuthConfig, emailSender MagicLinkEmailSender) *Routes {
	return &Routes{
		queries:      queries,
		devMode:      devMode,
		cookieConfig: cookieConfig,
		authConfig:   authConfig,
		emailSender:  emailSender,
	}
}

// Mount returns a function that mounts auth routes on a chi router.
// Routes are conditionally mounted based on auth configuration.
func (r *Routes) Mount() func(chi.Router) {
	return func(router chi.Router) {
		// Password login (if enabled)
		if r.authConfig.Password.Enabled {
			router.Post("/login", r.login)
		}

		// Magic link (if enabled)
		if r.authConfig.MagicLink.Enabled {
			router.Post("/magic-link", r.requestMagicLink)
			router.Get("/magic-link/verify", r.verifyMagicLink)
		}

		// OAuth - Google (if enabled)
		if r.authConfig.OAuth.Google.Enabled {
			router.Get("/oauth/google", r.oauthGoogleStart)
			router.Get("/oauth/google/callback", r.oauthGoogleCallback)
		}

		// Routes with optional auth (work with or without authentication)
		router.Group(func(router chi.Router) {
			router.Use(OptionalAuthWithConfig(r.queries, r.cookieConfig))
			router.Get("/whoami", r.whoami)
			router.Get("/logout", r.logout)
			router.Post("/logout", r.logout)
		})

		// Protected routes (require authentication)
		router.Group(func(router chi.Router) {
			router.Use(RequiresAuthWithConfig(r.queries, r.cookieConfig))
			router.Post("/logout-all", r.logoutAll)
		})

		// Development-only routes (no auth required)
		if r.devMode {
			router.Get("/assume", r.assume)
		}
	}
}

// whoami returns information about the authenticated user.
// Returns consistent structure with defaults if not authenticated.
func (r *Routes) whoami(w http.ResponseWriter, req *http.Request) {
	user, ok := UserFromContext(req.Context())
	if !ok {
		web.WriteJSON(w, map[string]any{
			"authenticated": false,
			"id":            0,
			"email":         "",
			"is_admin":      false,
		})
		return
	}

	web.WriteJSON(w, map[string]any{
		"authenticated": true,
		"id":            user.ID,
		"email":         user.Email,
		"is_admin":      user.IsAdmin == 1,
	})
}

// logout invalidates the current session.
// No-op if not authenticated.
func (r *Routes) logout(w http.ResponseWriter, req *http.Request) {
	session, ok := SessionFromContext(req.Context())
	if !ok {
		// Not logged in - no-op
		web.WriteJSON(w, map[string]any{
			"message": "not logged in",
		})
		return
	}

	// Invalidate the session
	result := r.queries.InvalidateSession.Exec(session.KeyValue)
	if result.Err() != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to invalidate session", result.Err()))
		return
	}

	// Clear the session cookie
	r.clearSessionCookie(w)

	web.WriteJSON(w, map[string]any{
		"message": "logged out",
	})
}

// logoutAll invalidates all sessions for the current user.
func (r *Routes) logoutAll(w http.ResponseWriter, req *http.Request) {
	user, ok := UserFromContext(req.Context())
	if !ok {
		web.WriteAppError(w, req, web.ErrUnauthorized("No user in context", nil))
		return
	}

	// Invalidate all sessions for this user
	result := r.queries.InvalidateAllSessions.Exec(user.ID)
	if result.Err() != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to invalidate sessions", result.Err()))
		return
	}

	// Clear the session cookie
	r.clearSessionCookie(w)

	web.WriteJSON(w, map[string]any{
		"message": "all sessions invalidated",
	})
}

// clearSessionCookie clears the session cookie.
func (r *Routes) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     r.cookieConfig.Name,
		Value:    "",
		Path:     r.cookieConfig.Path,
		Domain:   r.cookieConfig.Domain,
		MaxAge:   -1,
		Secure:   r.cookieConfig.Secure,
		HttpOnly: r.cookieConfig.HttpOnly,
		SameSite: parseSameSite(r.cookieConfig.SameSite),
	})
}

// loginRequest is the expected JSON body for login.
type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// login validates credentials and creates a session.
// POST /auth/login
func (r *Routes) login(w http.ResponseWriter, req *http.Request) {
	var body loginRequest
	if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
		web.WriteAppError(w, req, web.ErrBadRequest("Invalid request body", err))
		return
	}

	if body.Email == "" || body.Password == "" {
		web.WriteAppError(w, req, web.ErrBadRequest("Email and password required", nil))
		return
	}

	// Look up user
	user, err := r.queries.GetUserByEmail.Exec(body.Email).FirstE()
	if err != nil {
		web.WriteAppError(w, req, web.ErrUnauthorized("Invalid credentials", nil))
		return
	}

	// Check password
	if !user.HasPassword() || !CheckPassword(body.Password, user.PasswordHash) {
		web.WriteAppError(w, req, web.ErrUnauthorized("Invalid credentials", nil))
		return
	}

	// Check user is active
	if !user.IsActiveUser() {
		web.WriteAppError(w, req, web.ErrForbidden("Account is inactive", nil))
		return
	}

	// Create session (30 day expiration)
	sessionKey, err := r.CreateSession(w, user.ID, "", 30*24*time.Hour)
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create session", err))
		return
	}

	web.WriteJSON(w, map[string]any{
		"message":     "logged in",
		"session_key": sessionKey,
		"user_id":     user.ID,
		"email":       user.Email,
	})
}

// assume creates a session for a user by email (development only).
// Usage: GET /auth/assume?email=user@example.com
func (r *Routes) assume(w http.ResponseWriter, req *http.Request) {
	email := req.URL.Query().Get("email")
	if email == "" {
		web.WriteAppError(w, req, web.ErrBadRequest("Email query parameter required", nil))
		return
	}

	// Look up user by email
	user, err := r.queries.GetUserByEmail.Exec(email).FirstE()
	if err != nil {
		web.WriteAppError(w, req, web.ErrNotFound("User not found", err))
		return
	}

	// Check user is active
	if !user.IsActiveUser() {
		web.WriteAppError(w, req, web.ErrForbidden("User account is inactive", nil))
		return
	}

	// Generate session key with prefix
	sessionKey, err := generateSessionKey(TokenPrefixSession)
	if err != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to generate session", err))
		return
	}

	// Create session (no scopes, no expiration for dev sessions)
	result := r.queries.InsertSession.Exec(sessionKey, "session", "", user.ID, 0)
	if result.Err() != nil {
		web.WriteAppError(w, req, web.ErrInternal("Failed to create session", result.Err()))
		return
	}

	// Set session cookie using config
	r.setSessionCookie(w, sessionKey)

	web.WriteJSON(w, map[string]any{
		"message":     "session created",
		"session_key": sessionKey,
		"user_id":     user.ID,
		"email":       user.Email,
	})
}

// setSessionCookie sets the session cookie using the configured settings.
func (r *Routes) setSessionCookie(w http.ResponseWriter, sessionKey string) {
	http.SetCookie(w, &http.Cookie{
		Name:     r.cookieConfig.Name,
		Value:    sessionKey,
		Path:     r.cookieConfig.Path,
		Domain:   r.cookieConfig.Domain,
		MaxAge:   r.cookieConfig.MaxAge,
		Secure:   r.cookieConfig.Secure,
		HttpOnly: r.cookieConfig.HttpOnly,
		SameSite: parseSameSite(r.cookieConfig.SameSite),
	})
}

// parseSameSite converts a string to http.SameSite.
func parseSameSite(s string) http.SameSite {
	switch s {
	case "Strict":
		return http.SameSiteStrictMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}

// generateSessionKey generates a random session key with the given prefix.
func generateSessionKey(prefix string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(bytes), nil
}

// CreateSession creates a new session for a user with the given parameters.
// This is useful for creating sessions programmatically (e.g., after OAuth login).
func (r *Routes) CreateSession(w http.ResponseWriter, userID int, scopes string, expiresIn time.Duration) (string, error) {
	sessionKey, err := generateSessionKey(TokenPrefixSession)
	if err != nil {
		return "", err
	}

	var expiresAt int64
	if expiresIn > 0 {
		expiresAt = time.Now().Add(expiresIn).Unix()
	}

	result := r.queries.InsertSession.Exec(sessionKey, "session", scopes, userID, expiresAt)
	if result.Err() != nil {
		return "", result.Err()
	}

	r.setSessionCookie(w, sessionKey)
	return sessionKey, nil
}

// CreateAPIKey creates a new API key for a user with the given scopes.
// API keys don't expire by default (expiresAt=0).
func (r *Routes) CreateAPIKey(userID int, scopes string) (string, error) {
	apiKey, err := generateSessionKey(TokenPrefixAPIKey)
	if err != nil {
		return "", err
	}

	result := r.queries.InsertSession.Exec(apiKey, "api_key", scopes, userID, 0)
	if result.Err() != nil {
		return "", result.Err()
	}

	return apiKey, nil
}
