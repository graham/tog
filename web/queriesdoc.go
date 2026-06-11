package web

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
)

// QueriesDocHandler returns a chi router function that serves query documentation.
// It dynamically generates documentation from the database's registered queries.
//
// Endpoints:
//   - GET /     - HTML documentation
//   - GET /json - JSON documentation
func QueriesDocHandler(database *db.DB) func(chi.Router) {
	return func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, req *http.Request) {
			queries := database.RegisteredQueries()
			html := formatQueriesHTML(queries)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write([]byte(html))
		})

		r.Get("/json", func(w http.ResponseWriter, req *http.Request) {
			queries := database.RegisteredQueries()
			output := struct {
				Generated string         `json:"generated"`
				Total     int            `json:"total"`
				Queries   []db.QueryInfo `json:"queries"`
			}{
				Generated: time.Now().Format(time.RFC3339),
				Total:     len(queries),
				Queries:   queries,
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(output)
		})
	}
}

func formatQueriesHTML(queries []db.QueryInfo) string {
	// Group by file
	groups := make(map[string][]db.QueryInfo)
	for _, q := range queries {
		file := filepath.Base(q.File)
		groups[file] = append(groups[file], q)
	}

	// Sort group names
	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	data := struct {
		Generated string
		Total     int
		Groups    []struct {
			Name    string
			Queries []db.QueryInfo
		}
	}{
		Generated: time.Now().Format("2006-01-02 15:04:05"),
		Total:     len(queries),
	}

	for _, name := range groupNames {
		data.Groups = append(data.Groups, struct {
			Name    string
			Queries []db.QueryInfo
		}{
			Name:    name,
			Queries: groups[name],
		})
	}

	var sb strings.Builder
	tmpl := template.Must(template.New("queries").Funcs(template.FuncMap{
		"upper": strings.ToUpper,
	}).Parse(queriesHTMLTemplate))
	tmpl.Execute(&sb, data)
	return sb.String()
}

const queriesHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Registered Queries</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background: #f5f5f5;
        }
        h1 { color: #333; border-bottom: 2px solid #007bff; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .summary {
            background: #fff;
            padding: 15px 20px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
            display: flex;
            gap: 30px;
            flex-wrap: wrap;
        }
        .summary-item { display: flex; align-items: center; gap: 8px; }
        .summary-label { color: #666; }
        .summary-value { font-weight: bold; font-size: 1.2em; }
        .generated { color: #888; font-size: 0.9em; }
        .search-box {
            margin-bottom: 20px;
            padding: 10px 15px;
            width: 100%;
            max-width: 400px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 1em;
        }
        .query-card {
            background: #fff;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 15px;
            overflow: hidden;
        }
        .query-header {
            padding: 12px 15px;
            background: #f8f9fa;
            border-bottom: 1px solid #eee;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        .query-name { font-weight: 500; }
        .query-type {
            padding: 3px 8px;
            border-radius: 4px;
            font-size: 0.8em;
            font-weight: 500;
        }
        .type-select { background: #28a745; color: #fff; }
        .type-exec { background: #007bff; color: #fff; }
        .query-body { padding: 15px; }
        .query-sql {
            font-family: 'Monaco', 'Menlo', monospace;
            font-size: 0.9em;
            background: #2d2d2d;
            color: #f8f8f2;
            padding: 12px 15px;
            border-radius: 4px;
            overflow-x: auto;
            white-space: pre-wrap;
            word-break: break-word;
        }
        .query-meta {
            margin-top: 10px;
            font-size: 0.85em;
            color: #666;
        }
        .query-file { font-family: monospace; }
        .query-description {
            color: #495057;
            font-size: 0.9em;
            margin-bottom: 10px;
            padding: 8px 12px;
            background: #e9ecef;
            border-radius: 4px;
        }
    </style>
</head>
<body>
    <h1>Registered Queries</h1>

    <div class="summary">
        <div class="summary-item">
            <span class="summary-label">Total:</span>
            <span class="summary-value">{{.Total}}</span>
        </div>
        <div class="summary-item generated">
            Generated: {{.Generated}}
        </div>
    </div>

    <input type="text" class="search-box" placeholder="Filter queries..." id="search" onkeyup="filterQueries()">

    {{range .Groups}}
    <h2>{{.Name}}</h2>
    {{range .Queries}}
    <div class="query-card">
        <div class="query-header">
            <span class="query-name">{{.Name}}</span>
            <span class="query-type type-{{.Type}}">{{.Type | upper}}</span>
        </div>
        <div class="query-body">
            {{if .Description}}<div class="query-description">{{.Description}}</div>{{end}}
            <div class="query-sql">{{.SQL}}</div>
            <div class="query-meta">
                <span class="query-file">{{.File}}:{{.Line}}</span>
            </div>
        </div>
    </div>
    {{end}}
    {{end}}

    <script>
        function filterQueries() {
            const query = document.getElementById('search').value.toLowerCase();
            const cards = document.querySelectorAll('.query-card');
            const headers = document.querySelectorAll('h2');

            cards.forEach(card => {
                const text = card.textContent.toLowerCase();
                card.style.display = text.includes(query) ? '' : 'none';
            });

            // Hide headers with no visible cards
            headers.forEach(header => {
                let next = header.nextElementSibling;
                let hasVisible = false;
                while (next && !next.matches('h2')) {
                    if (next.matches('.query-card') && next.style.display !== 'none') {
                        hasVisible = true;
                        break;
                    }
                    next = next.nextElementSibling;
                }
                header.style.display = hasVisible ? '' : 'none';
            });
        }
    </script>
</body>
</html>`
