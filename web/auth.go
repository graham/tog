package web

import (
	"net/http"
)

// AuthProvider defines how authentication is performed.
// Applications implement this interface for their auth strategy
// (e.g., JWT, session cookies, API keys).
type AuthProvider interface {
	// Authenticate validates the request and returns user data.
	// The returned value is stored in the context with the key from ContextKey().
	// Return nil, nil to indicate unauthenticated (no credentials provided).
	// Return nil, error to indicate authentication failed (invalid credentials).
	Authenticate(r *http.Request) (user any, err error)

	// ContextKey returns the key used to store the user in context.
	// Common values: "user", "auth_user", "principal"
	ContextKey() string
}

// AuthConfig configures the RequiresAuth middleware.
type AuthConfig struct {
	// Provider is the auth implementation.
	Provider AuthProvider

	// OnUnauthenticated is called when no credentials are provided.
	// If nil, writes a 401 JSON response with "Authentication required".
	OnUnauthenticated func(w http.ResponseWriter, r *http.Request)

	// OnAuthError is called when authentication fails with an error.
	// If nil, writes a 401 JSON response with the error message.
	OnAuthError func(w http.ResponseWriter, r *http.Request, err error)
}

// RequiresAuth creates middleware that enforces authentication.
// Returns 401 if the request is not authenticated.
func RequiresAuth(cfg AuthConfig) func(http.Handler) http.Handler {
	onUnauth := cfg.OnUnauthenticated
	if onUnauth == nil {
		onUnauth = func(w http.ResponseWriter, r *http.Request) {
			WriteError(w, http.StatusUnauthorized, "Authentication required")
		}
	}

	onError := cfg.OnAuthError
	if onError == nil {
		onError = func(w http.ResponseWriter, r *http.Request, err error) {
			WriteError(w, http.StatusUnauthorized, err.Error())
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, err := cfg.Provider.Authenticate(r)

			if err != nil {
				onError(w, r, err)
				return
			}

			if user == nil {
				onUnauth(w, r)
				return
			}

			// Store user in context
			ctx := GetContext(r.Context())
			ctx.Set(cfg.Provider.ContextKey(), user)

			next.ServeHTTP(w, r)
		})
	}
}

// OptionalAuth creates middleware that attempts auth but allows unauthenticated requests.
// If authentication succeeds, the user is stored in context.
// If authentication fails or no credentials are provided, the request continues without a user.
func OptionalAuth(provider AuthProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, _ := provider.Authenticate(r)

			if user != nil {
				ctx := GetContext(r.Context())
				ctx.Set(provider.ContextKey(), user)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequiresAuthFunc is a convenience function for creating auth middleware
// with a simple authenticate function.
func RequiresAuthFunc(contextKey string, authenticate func(r *http.Request) (any, error)) func(http.Handler) http.Handler {
	return RequiresAuth(AuthConfig{
		Provider: &funcAuthProvider{
			contextKey:   contextKey,
			authenticate: authenticate,
		},
	})
}

// funcAuthProvider wraps a function to implement AuthProvider.
type funcAuthProvider struct {
	contextKey   string
	authenticate func(r *http.Request) (any, error)
}

func (f *funcAuthProvider) Authenticate(r *http.Request) (any, error) {
	return f.authenticate(r)
}

func (f *funcAuthProvider) ContextKey() string {
	return f.contextKey
}
