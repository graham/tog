package auth

import (
	"context"

	"github.com/graham/tog/web"
)

const (
	userContextKey    = "auth_user"
	sessionContextKey = "auth_session"
)

// UserFromContext retrieves the authenticated user from the request context.
// Returns nil and false if no user is authenticated.
func UserFromContext(ctx context.Context) (*User, bool) {
	wc := web.GetContext(ctx)
	return web.GetTyped[*User](wc, userContextKey)
}

// MustUserFromContext retrieves the authenticated user from the request context.
// Panics if no user is authenticated.
func MustUserFromContext(ctx context.Context) *User {
	user, ok := UserFromContext(ctx)
	if !ok {
		panic("auth: no authenticated user in context")
	}
	return user
}

// SessionFromContext retrieves the current session from the request context.
// Returns nil and false if no session is present.
func SessionFromContext(ctx context.Context) (*Session, bool) {
	wc := web.GetContext(ctx)
	return web.GetTyped[*Session](wc, sessionContextKey)
}

// MustSessionFromContext retrieves the current session from the request context.
// Panics if no session is present.
func MustSessionFromContext(ctx context.Context) *Session {
	session, ok := SessionFromContext(ctx)
	if !ok {
		panic("auth: no session in context")
	}
	return session
}

// setUserInContext stores the authenticated user in the web context.
func setUserInContext(ctx context.Context, user *User) {
	wc := web.GetContext(ctx)
	wc.Set(userContextKey, user)
}

// setSessionInContext stores the session in the web context.
func setSessionInContext(ctx context.Context, session *Session) {
	wc := web.GetContext(ctx)
	wc.Set(sessionContextKey, session)
}

// Related Entity Helpers
// These functions help manage application-specific entities in the request context.
// Common use cases: current project, organization, team, etc.

// SetRelatedEntity stores an entity in the request context with a prefixed key.
// Use this for caching entities fetched in middleware (e.g., current project).
// Example: SetRelatedEntity(ctx, "project", &project)
func SetRelatedEntity[T any](ctx context.Context, key string, entity *T) {
	wc := web.GetContext(ctx)
	wc.Set("related_"+key, entity)
}

// GetRelatedEntity retrieves a related entity from the request context.
// Returns nil and false if not found or wrong type.
// Example: project, ok := GetRelatedEntity[Project](ctx, "project")
func GetRelatedEntity[T any](ctx context.Context, key string) (*T, bool) {
	wc := web.GetContext(ctx)
	return web.GetTyped[*T](wc, "related_"+key)
}

// MustGetRelatedEntity retrieves a related entity from the request context.
// Panics if the entity is not found (use after middleware that sets it).
func MustGetRelatedEntity[T any](ctx context.Context, key string) *T {
	entity, ok := GetRelatedEntity[T](ctx, key)
	if !ok {
		panic("auth: related entity not found: " + key)
	}
	return entity
}
