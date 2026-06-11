package web

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
)

// DevRoutesConfig configures optional development routes.
type DevRoutesConfig struct {
	// EnableDocs enables the /docs endpoint for serving documentation.
	EnableDocs bool

	// DocsDir is the directory containing documentation files.
	// Default: "docs"
	DocsDir string

	// EnableSchema enables the /schema endpoint for database introspection.
	EnableSchema bool

	// DBManager is required when EnableSchema is true.
	// Provides multi-database schema introspection.
	DBManager *db.Manager
}

// MountDevRoutes mounts optional development routes on the given router.
// Returns a function that can be used with chi's Route() method.
func MountDevRoutes(cfg DevRoutesConfig) func(chi.Router) {
	return func(r chi.Router) {
		if cfg.EnableDocs {
			docsConfig := DocsConfig{}
			if cfg.DocsDir != "" {
				docsConfig.Dir = cfg.DocsDir
			}
			r.Handle("/docs", http.RedirectHandler("/dev/docs/", http.StatusMovedPermanently))
			r.Handle("/docs/*", http.StripPrefix("/dev/docs", DocsHandler(docsConfig)))
		}

		if cfg.EnableSchema && cfg.DBManager != nil {
			r.Route("/schema", MultiSchemaRoutes(cfg.DBManager))
		}
	}
}
