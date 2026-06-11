package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// DocsConfig configures the documentation handler.
type DocsConfig struct {
	// Dir is the directory containing documentation files.
	// Default: "docs"
	Dir string

	// IndexFile is the default file to serve for directory requests.
	// Default: "tests.html"
	IndexFile string
}

// DocsHandler returns an http.Handler that serves documentation files.
// It serves static files from the configured directory and redirects
// directory requests to the index file.
func DocsHandler(cfg ...DocsConfig) http.Handler {
	config := DocsConfig{
		Dir:       "docs",
		IndexFile: "tests.html",
	}
	if len(cfg) > 0 {
		if cfg[0].Dir != "" {
			config.Dir = cfg[0].Dir
		}
		if cfg[0].IndexFile != "" {
			config.IndexFile = cfg[0].IndexFile
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the path after /docs
		path := strings.TrimPrefix(r.URL.Path, "/docs")
		path = strings.TrimPrefix(path, "/")

		// Default to index file
		if path == "" {
			path = config.IndexFile
		}

		// Construct full file path
		filePath := filepath.Join(config.Dir, path)

		// Security: prevent directory traversal
		absDir, err := filepath.Abs(config.Dir)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		absFile, err := filepath.Abs(filePath)
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		if !strings.HasPrefix(absFile, absDir) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Check if file exists
		info, err := os.Stat(filePath)
		if os.IsNotExist(err) {
			http.Error(w, "Documentation not found. Run 'make testdocs' to generate.", http.StatusNotFound)
			return
		}
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// If it's a directory, serve the index file
		if info.IsDir() {
			filePath = filepath.Join(filePath, config.IndexFile)
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				http.Error(w, "Documentation not found. Run 'make testdocs' to generate.", http.StatusNotFound)
				return
			}
		}

		http.ServeFile(w, r, filePath)
	})
}
