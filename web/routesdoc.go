package web

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/tools/routes"
)

// RoutesDocConfig configures the routes documentation handler.
type RoutesDocConfig struct {
	// Title is the title for the HTML documentation.
	// Default: "API Routes"
	Title string

	// Descriptions maps "METHOD /path" to a description string.
	// Example: {"GET /api/users": "List all users"}
	Descriptions map[string]string
}

// RoutesDocHandler returns a chi router function that serves route documentation.
// It dynamically generates documentation from the provided router.
//
// Endpoints:
//   - GET /        - HTML documentation
//   - GET /json    - JSON documentation
func RoutesDocHandler(router chi.Router, cfg ...RoutesDocConfig) func(chi.Router) {
	config := RoutesDocConfig{
		Title: "API Routes",
	}
	if len(cfg) > 0 {
		if cfg[0].Title != "" {
			config.Title = cfg[0].Title
		}
		if cfg[0].Descriptions != nil {
			config.Descriptions = cfg[0].Descriptions
		}
	}

	return func(r chi.Router) {
		routesCfg := routes.Config{
			Descriptions: config.Descriptions,
		}

		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			routeInfos := routes.CollectRoutes(router, routesCfg)
			html := routes.FormatHTML(routeInfos, config.Title)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
		})

		r.Get("/json", func(w http.ResponseWriter, req *http.Request) {
			routeInfos := routes.CollectRoutes(router, routesCfg)
			jsonBytes, err := routes.FormatJSON(routeInfos)
			if err != nil {
				WriteError(w, http.StatusInternalServerError, err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write(jsonBytes)
		})
	}
}

// DocsIndexHandler returns a handler that serves an index page for all documentation.
func DocsIndexHandler(docsDir string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Documentation</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            max-width: 800px;
            margin: 50px auto;
            padding: 20px;
            background: #f5f5f5;
        }
        h1 { color: #333; }
        .doc-list {
            list-style: none;
            padding: 0;
        }
        .doc-list li {
            background: #fff;
            margin: 10px 0;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .doc-list a {
            display: block;
            padding: 20px;
            text-decoration: none;
            color: #007bff;
            font-size: 1.1em;
        }
        .doc-list a:hover {
            background: #f8f9fa;
            border-radius: 8px;
        }
        .doc-list .desc {
            color: #666;
            font-size: 0.9em;
            margin-top: 5px;
        }
    </style>
</head>
<body>
    <h1>Documentation</h1>
    <ul class="doc-list">
        <li>
            <a href="/docs/routes">
                API Routes
                <div class="desc">Browse all registered HTTP endpoints</div>
            </a>
        </li>
        <li>
            <a href="/docs/queries">
                SQL Queries
                <div class="desc">Browse all registered SQL queries</div>
            </a>
        </li>
        <li>
            <a href="/docs/tests">
                Test Documentation
                <div class="desc">Generated test documentation with results</div>
            </a>
        </li>
    </ul>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(strings.TrimSpace(html)))
	}
}
