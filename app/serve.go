package app

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/graham/tog/db"
	"github.com/graham/tog/web"
)

func cmdServe(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "serve", "Run the HTTP server.")
	fs.Parse(args)

	router, dbm, err := createRouter(cfg)
	if err != nil {
		log.Fatalf("failed to create router: %v", err)
	}
	defer dbm.Close()

	// Start server
	host := os.Getenv("HOST")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := host + ":" + port
	log.Printf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// Router holds the configured router and database manager.
type Router struct {
	*chi.Mux
	DBManager *db.Manager
}

// createRouter creates the router with all middleware and standard routes,
// then calls the application's LoadRoutes function.
func createRouter(cfg Config) (*Router, *db.Manager, error) {
	// Load database configuration
	dbm, err := db.NewManagerFromEnvOrFile("")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create database manager: %w", err)
	}

	// Build router
	r := chi.NewRouter()

	// Standard middleware stack
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(web.ContextMiddleware(dbm))
	r.Use(web.LoggingMiddleware(web.DefaultLogger()))

	// Health endpoints
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
	})
	r.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		web.WriteJSON(w, map[string]string{"status": "healthy"})
	})

	// Documentation endpoints
	r.Get("/docs", web.DocsIndexHandler("docs"))
	r.Get("/docs/", web.DocsIndexHandler("docs"))
	r.Route("/docs/routes", web.RoutesDocHandler(r))
	r.Route("/docs/queries", web.QueriesDocHandler(dbm.Default()))
	r.Get("/docs/tests", func(w http.ResponseWriter, req *http.Request) {
		http.ServeFile(w, req, "docs/tests.html")
	})
	r.Get("/docs/tests/*", func(w http.ResponseWriter, req *http.Request) {
		http.StripPrefix("/docs/tests", http.FileServer(http.Dir("docs"))).ServeHTTP(w, req)
	})

	// Schema endpoint for database introspection
	r.Route("/schema", web.MultiSchemaRoutes(dbm))

	// Call application's LoadRoutes to set up app-specific routes
	if cfg.LoadRoutes != nil {
		if err := cfg.LoadRoutes(r, dbm); err != nil {
			dbm.Close()
			return nil, nil, fmt.Errorf("failed to load routes: %w", err)
		}
	}

	// Verify all registered queries
	if err := dbm.VerifyAll(); err != nil {
		dbm.Close()
		return nil, nil, fmt.Errorf("query verification failed: %w", err)
	}
	log.Printf("all queries verified successfully")

	return &Router{Mux: r, DBManager: dbm}, dbm, nil
}
