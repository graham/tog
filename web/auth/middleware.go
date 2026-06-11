package auth

import (
	"encoding/json"
	"net/http"
	"strings"
)

type authError struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(authError{
		Error:   http.StatusText(status),
		Message: message,
	})
}

// extractToken extracts the session token from the request.
// It checks the session_key cookie first, then the Authorization header.
// It handles both prefixed (sess_xxx) and non-prefixed tokens.
func extractToken(r *http.Request, cookieName string) string {
	// Check cookie first
	if cookie, err := r.Cookie(cookieName); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Check Authorization header
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}

	return ""
}

// authenticateRequest validates the token and returns the session and user.
// Uses a single JOIN query to fetch both session and user efficiently.
// Returns nil for both if authentication fails.
func authenticateRequest(r *http.Request, queries *Queries, cookieName string) (*Session, *User, string) {
	token := extractToken(r, cookieName)
	if token == "" {
		return nil, nil, "No session cookie or authorization header provided"
	}

	// Look up session and user in a single query
	result, err := queries.GetSessionWithUser.Exec(token).FirstE()
	if err != nil {
		return nil, nil, "Invalid or expired session"
	}

	session := result.Session()
	user := result.User()

	// Check session expiration
	if session.IsExpired() {
		return nil, nil, "Session has expired"
	}

	// Check user is active
	if !user.IsActiveUser() {
		return nil, nil, "User account is inactive"
	}

	return session, user, ""
}

// RequiresAuth creates middleware that validates session from cookie or Bearer token.
// It checks for a session_key cookie first, then falls back to Authorization header.
// The authenticated user and session are stored in the request context.
func RequiresAuth(queries *Queries) func(http.Handler) http.Handler {
	return RequiresAuthWithConfig(queries, DefaultCookieConfig())
}

// RequiresAuthWithConfig creates auth middleware with custom cookie configuration.
func RequiresAuthWithConfig(queries *Queries, config CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, user, errMsg := authenticateRequest(r, queries, config.Name)
			if errMsg != "" {
				writeAuthError(w, http.StatusUnauthorized, errMsg)
				return
			}

			// Add user and session to context
			setUserInContext(r.Context(), user)
			setSessionInContext(r.Context(), session)
			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuth creates middleware that loads the user if authenticated,
// but allows the request to proceed even without authentication.
// Use UserFromContext to check if a user is present.
func OptionalAuth(queries *Queries) func(http.Handler) http.Handler {
	return OptionalAuthWithConfig(queries, DefaultCookieConfig())
}

// OptionalAuthWithConfig creates optional auth middleware with custom cookie configuration.
func OptionalAuthWithConfig(queries *Queries, config CookieConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, user, _ := authenticateRequest(r, queries, config.Name)
			if user != nil {
				setUserInContext(r.Context(), user)
				setSessionInContext(r.Context(), session)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequiresScope creates middleware that checks if the session has the required scope.
// This middleware must be used after RequiresAuth.
func RequiresScope(scope string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "No session in context")
				return
			}

			if !session.HasScope(scope) {
				writeAuthError(w, http.StatusForbidden, "Missing required scope: "+scope)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequiresAnyScope creates middleware that checks if the session has any of the required scopes.
// This middleware must be used after RequiresAuth.
func RequiresAnyScope(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "No session in context")
				return
			}

			if !session.HasAnyScope(scopes...) {
				writeAuthError(w, http.StatusForbidden, "Missing required scope. Need one of: "+strings.Join(scopes, ", "))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequiresAllScopes creates middleware that checks if the session has all of the required scopes.
// This middleware must be used after RequiresAuth.
func RequiresAllScopes(scopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			session, ok := SessionFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "No session in context")
				return
			}

			if !session.HasAllScopes(scopes...) {
				writeAuthError(w, http.StatusForbidden, "Missing required scopes: "+strings.Join(scopes, ", "))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequiresAdmin creates middleware that checks if the user is an admin.
// This middleware must be used after RequiresAuth.
func RequiresAdmin() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok {
				writeAuthError(w, http.StatusUnauthorized, "No user in context")
				return
			}

			if !user.IsAdminUser() {
				writeAuthError(w, http.StatusForbidden, "Admin access required")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
