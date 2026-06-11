package web

import (
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
)

// MultiSchemaRoutes returns a chi router for multi-database schema introspection.
// Provides:
//   - GET /         - List all available databases
//   - GET /{db}     - List all tables in a database
//   - GET /{db}/{table} - Get schema for a specific table
func MultiSchemaRoutes(dbm *db.Manager) func(chi.Router) {
	return func(r chi.Router) {
		// GET / - list all databases
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			names := dbm.Names()
			sort.Strings(names)
			WriteJSON(w, map[string]any{"databases": names})
		})

		// GET /{db} - list tables in database
		r.Get("/{db}", func(w http.ResponseWriter, req *http.Request) {
			dbName := chi.URLParam(req, "db")
			database := dbm.Get(dbName)
			if database == nil {
				WriteAppError(w, req, ErrNotFound("Database not found", nil))
				return
			}
			tables, err := database.SchemaInfo()
			if err != nil {
				WriteAppError(w, req, ErrInternal("Failed to fetch schema", err))
				return
			}
			// Return just the table names
			tableNames := make([]string, len(tables))
			for i, t := range tables {
				tableNames[i] = t.Name
			}
			WriteJSON(w, map[string]any{"tables": tableNames})
		})

		// GET /{db}/{table} - specific table schema
		r.Get("/{db}/{table}", func(w http.ResponseWriter, req *http.Request) {
			dbName := chi.URLParam(req, "db")
			tableName := chi.URLParam(req, "table")
			database := dbm.Get(dbName)
			if database == nil {
				WriteAppError(w, req, ErrNotFound("Database not found", nil))
				return
			}
			table, err := database.TableSchema(tableName)
			if err != nil {
				WriteAppError(w, req, ErrInternal("Failed to fetch table schema", err))
				return
			}
			if table == nil {
				WriteAppError(w, req, ErrNotFound("Table not found", nil))
				return
			}
			WriteJSON(w, table)
		})
	}
}

// SchemaRoutes returns a chi router that mounts database schema endpoints.
// Provides /tables to list all tables and /tables/{name} for specific table schema.
// Deprecated: Use MultiSchemaRoutes with a db.Manager for multi-database support.
func SchemaRoutes(database *db.DB) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			tables, err := database.SchemaInfo()
			if err != nil {
				WriteAppError(w, req, ErrInternal("Failed to fetch schema", err))
				return
			}
			WriteJSON(w, tables)
		})

		r.Get("/{name}", func(w http.ResponseWriter, req *http.Request) {
			name := chi.URLParam(req, "name")
			table, err := database.TableSchema(name)
			if err != nil {
				WriteAppError(w, req, ErrInternal("Failed to fetch table schema", err))
				return
			}
			if table == nil {
				WriteAppError(w, req, ErrNotFound("Table not found", nil))
				return
			}
			WriteJSON(w, table)
		})
	}
}
