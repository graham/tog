package testkit

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
	"github.com/graham/tog/web"
)

// TestDBConfig specifies configuration for a test database.
type TestDBConfig struct {
	Name          string // Database name (e.g., "primary", "legacy")
	MigrationsDir string // Path to migrations directory relative to test file
}

// TestApp provides a fully configured application for integration testing.
// It creates in-memory databases, runs migrations, and sets up the router.
type TestApp struct {
	T      *testing.T
	DBM    *db.Manager
	Router chi.Router
}

// NewTestApp creates a new test application with in-memory database and migrations applied.
// migrationsDir is the path to the migrations directory relative to the test file.
// This is a convenience wrapper for NewTestAppWithDBs with a single primary database.
func NewTestApp(t *testing.T, migrationsDir string) *TestApp {
	return NewTestAppWithDBs(t, TestDBConfig{Name: "primary", MigrationsDir: migrationsDir})
}

// NewTestAppWithDBs creates a new test application with multiple in-memory databases.
// Each database is configured independently with its own migrations.
// The first config is set as the default database.
func NewTestAppWithDBs(t *testing.T, configs ...TestDBConfig) *TestApp {
	t.Helper()

	if len(configs) == 0 {
		t.Fatal("at least one database config is required")
	}

	// Build database config
	dbConfigs := make(map[string]db.DatabaseConfig)
	defaultDB := configs[0].Name

	for _, cfg := range configs {
		dbConfigs[cfg.Name] = db.DatabaseConfig{
			Driver: "sqlite3",
			DSN:    ":memory:",
			Pool: db.JSONPoolConfig{
				MaxOpen:            1,
				MaxIdle:            1,
				MaxLifetimeSeconds: 0,
			},
		}
	}

	config := &db.Config{
		Databases: dbConfigs,
		Default:   defaultDB,
	}

	dbm, err := db.NewManager(config)
	if err != nil {
		t.Fatalf("failed to create database manager: %v", err)
	}

	// Run migrations for each database
	for _, cfg := range configs {
		database := dbm.Get(cfg.Name)
		if database == nil {
			dbm.Close()
			t.Fatalf("database %q not found in manager", cfg.Name)
		}
		if err := RunMigrations(database.DB.DB, cfg.MigrationsDir); err != nil {
			dbm.Close()
			t.Fatalf("failed to run migrations for %q: %v", cfg.Name, err)
		}
	}

	// Build router with web context middleware
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(dbm))

	// Add basic health endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	app := &TestApp{
		T:      t,
		DBM:    dbm,
		Router: r,
	}

	t.Cleanup(func() {
		dbm.Close()
	})

	return app
}

// DB returns the primary database for convenience.
func (app *TestApp) DB() *db.DB {
	return app.DBM.Default()
}

// RouterFactory is a function that creates a fully configured router.
// It receives the database manager and returns a configured router.
type RouterFactory func(dbm *db.Manager) (chi.Router, error)

// WithRouter sets up the application using a router factory function.
// This allows tests to use the same router configuration as the production app.
// Returns the TestApp for method chaining.
//
// Example:
//
//	app := testkit.NewTestApp(t, "migrations").WithRouter(func(dbm *db.Manager) (chi.Router, error) {
//	    return routes.NewRouter(dbm)
//	})
func (app *TestApp) WithRouter(factory RouterFactory) *TestApp {
	app.T.Helper()
	router, err := factory(app.DBM)
	if err != nil {
		app.T.Fatalf("router factory failed: %v", err)
	}
	app.Router = router
	return app
}

// RouteSetupFunc is a function that registers queries and mounts routes.
// It receives the router and database, and should return an error if setup fails.
type RouteSetupFunc func(r chi.Router, database *db.DB) error

// WithRoutes sets up routes using the provided function and verifies all queries.
// This is a convenience method that reduces boilerplate in tests.
// Returns the TestApp for method chaining.
//
// Example:
//
//	app := testkit.NewTestApp(t, "migrations").WithRoutes(func(r chi.Router, db *db.DB) error {
//	    q, err := items.RegisterQueries(db)
//	    if err != nil {
//	        return err
//	    }
//	    r.Route("/api/items", items.NewRoutes(q).Mount("/api/items"))
//	    return nil
//	})
func (app *TestApp) WithRoutes(setup RouteSetupFunc) *TestApp {
	app.T.Helper()
	if err := setup(app.Router, app.DB()); err != nil {
		app.T.Fatalf("route setup failed: %v", err)
	}
	app.MustVerifyAll()
	return app
}

// GetDB returns a database by name. Returns nil if not found.
func (app *TestApp) GetDB(name string) *db.DB {
	return app.DBM.Get(name)
}

// Seed executes a function to populate the default database with test data.
// Returns the TestApp for method chaining.
func (app *TestApp) Seed(fn func(db *db.DB) error) *TestApp {
	return app.SeedDB("", fn)
}

// SeedDB executes a function to populate a named database with test data.
// If name is empty, uses the default database.
// Returns the TestApp for method chaining.
func (app *TestApp) SeedDB(name string, fn func(db *db.DB) error) *TestApp {
	app.T.Helper()
	var database *db.DB
	if name == "" {
		database = app.DBM.Default()
	} else {
		database = app.DBM.Get(name)
	}
	if database == nil {
		app.T.Fatalf("database %q not found", name)
	}
	if err := fn(database); err != nil {
		app.T.Fatalf("seed failed for %q: %v", name, err)
	}
	return app
}

// Request executes an HTTP request against the router and returns the response.
func (app *TestApp) Request(method, path string, body io.Reader) *httptest.ResponseRecorder {
	app.T.Helper()
	req := httptest.NewRequest(method, path, body)
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	return rec
}

// RequestWithHeader executes an HTTP request with custom headers.
func (app *TestApp) RequestWithHeader(method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	app.T.Helper()
	req := httptest.NewRequest(method, path, body)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	return rec
}

// RequestWithAuth executes an HTTP request with Bearer token authentication.
func (app *TestApp) RequestWithAuth(method, path string, body io.Reader, token string) *httptest.ResponseRecorder {
	app.T.Helper()
	return app.RequestWithHeader(method, path, body, map[string]string{
		"Authorization": "Bearer " + token,
	})
}

// RequestWithCookie executes an HTTP request with a cookie.
func (app *TestApp) RequestWithCookie(method, path string, body io.Reader, name, value string) *httptest.ResponseRecorder {
	app.T.Helper()
	req := httptest.NewRequest(method, path, body)
	req.AddCookie(&http.Cookie{Name: name, Value: value})
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, req)
	return rec
}

// LoadRoutesFunc is a function that sets up application routes.
// It matches the signature used by app.Config.LoadRoutes.
type LoadRoutesFunc func(r chi.Router, dbm *db.Manager) error

// WithLoadRoutes sets up routes using a function matching the app.Config.LoadRoutes signature.
// This method integrates with the app package's LoadRoutes pattern.
// Returns the TestApp for method chaining.
//
// Example:
//
//	app := testkit.NewTestApp(t, "migrations").WithLoadRoutes(routes.LoadRoutes)
func (app *TestApp) WithLoadRoutes(load LoadRoutesFunc) *TestApp {
	app.T.Helper()
	if err := load(app.Router, app.DBM); err != nil {
		app.T.Fatalf("load routes failed: %v", err)
	}
	app.MustVerifyAll()
	return app
}

// VerifyAll verifies all registered queries against the databases.
func (app *TestApp) VerifyAll() error {
	return app.DBM.VerifyAll()
}

// MustVerifyAll verifies all registered queries, failing the test on error.
func (app *TestApp) MustVerifyAll() {
	app.T.Helper()
	if err := app.DBM.VerifyAll(); err != nil {
		app.T.Fatalf("query verification failed: %v", err)
	}
}
