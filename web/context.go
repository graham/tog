package web

import (
	"context"
	"net/http"

	"github.com/graham/tog/db"
)

type webContextKey string

const ctxKey webContextKey = "tog_webctx"

// CtxKey is exported for applications that need direct context access.
// Prefer using GetContext() when possible.
var CtxKey any = ctxKey

// Context holds arbitrary values for the request lifecycle.
// Applications store their own user types and other request-scoped data.
type Context struct {
	values  map[string]any
	dbm     *db.Manager
	timings *timings
}

// GetContext returns the Context from the request context.
// Creates a new one if it doesn't exist.
func GetContext(ctx context.Context) *Context {
	if wc, ok := ctx.Value(ctxKey).(*Context); ok {
		return wc
	}
	return &Context{values: make(map[string]any)}
}

// WithContext adds a Context to the context.
func WithContext(ctx context.Context, wc *Context) context.Context {
	return context.WithValue(ctx, ctxKey, wc)
}

// Set stores a value in the Context.
func (c *Context) Set(key string, value any) {
	if c.values == nil {
		c.values = make(map[string]any)
	}
	c.values[key] = value
}

// Get retrieves a value from the Context.
func (c *Context) Get(key string) (any, bool) {
	if c.values == nil {
		return nil, false
	}
	v, ok := c.values[key]
	return v, ok
}

// MustGet retrieves a value from the Context, panics if not found.
func (c *Context) MustGet(key string) any {
	v, ok := c.Get(key)
	if !ok {
		panic("context: key not found: " + key)
	}
	return v
}

// GetTyped retrieves a typed value from the Context.
// Returns the value and true if found and of correct type, otherwise zero value and false.
func GetTyped[T any](c *Context, key string) (T, bool) {
	var zero T
	v, ok := c.Get(key)
	if !ok {
		return zero, false
	}
	typed, ok := v.(T)
	return typed, ok
}

// MustGetTyped retrieves a typed value from the Context.
// Panics if the key is not found or the value is of wrong type.
func MustGetTyped[T any](c *Context, key string) T {
	v, ok := GetTyped[T](c, key)
	if !ok {
		panic("context: key not found or wrong type: " + key)
	}
	return v
}

// ContextMiddleware initializes a Context for each request.
func ContextMiddleware(dbm *db.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wc := &Context{values: make(map[string]any), dbm: dbm}
			ctx := WithContext(r.Context(), wc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// SetCtxValue is a convenience function to set a value in the request's Context.
func SetCtxValue(r *http.Request, key string, value any) {
	GetContext(r.Context()).Set(key, value)
}

// GetCtxValue is a convenience function to get a value from the request's Context.
func GetCtxValue(r *http.Request, key string) (any, bool) {
	return GetContext(r.Context()).Get(key)
}

// DB returns the default (primary) database connection.
func (c *Context) DB() *db.DB {
	if c.dbm == nil {
		return nil
	}
	return c.dbm.Default()
}

// DBNamed returns a database connection by name.
// Returns nil if the database doesn't exist.
func (c *Context) DBNamed(name string) *db.DB {
	if c.dbm == nil {
		return nil
	}
	return c.dbm.Get(name)
}

// DBManager returns the database manager for advanced operations.
func (c *Context) DBManager() *db.Manager {
	return c.dbm
}
