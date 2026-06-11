package routes

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"net/http"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

// Registry stores route metadata for source location lookups.
// Use RegisterSource to add entries when routes are mounted.
var registry = &routeRegistry{
	entries: make(map[string]sourceInfo),
}

type sourceInfo struct {
	file string
	line int
}

type routeRegistry struct {
	mu      sync.RWMutex
	entries map[string]sourceInfo
}

// RegisterSource records the source location for a route.
// Call this when registering routes to capture file/line info.
// The depth parameter is how many stack frames to skip (0 = caller of RegisterSource).
func RegisterSource(method, path string, depth int) {
	_, file, line, ok := runtime.Caller(depth + 1)
	if !ok {
		return
	}
	key := method + " " + path
	registry.mu.Lock()
	registry.entries[key] = sourceInfo{file: file, line: line}
	registry.mu.Unlock()
}

// lookupSource returns the registered source info for a route.
func lookupSource(method, path string) (file string, line int, ok bool) {
	key := method + " " + path
	registry.mu.RLock()
	info, ok := registry.entries[key]
	registry.mu.RUnlock()
	if ok {
		return info.file, info.line, true
	}
	return "", 0, false
}

// godocCache caches parsed AST files to avoid re-parsing.
var godocCache = &astCache{
	files: make(map[string]*ast.File),
	fset:  token.NewFileSet(),
}

type astCache struct {
	mu    sync.RWMutex
	files map[string]*ast.File
	fset  *token.FileSet
}

func (c *astCache) getFile(path string) (*ast.File, *token.FileSet) {
	c.mu.RLock()
	f, ok := c.files[path]
	c.mu.RUnlock()
	if ok {
		return f, c.fset
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if f, ok := c.files[path]; ok {
		return f, c.fset
	}

	// Parse the file
	f, err := parser.ParseFile(c.fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, c.fset
	}

	c.files[path] = f
	return f, c.fset
}

// ExtractGodoc extracts the godoc comment for a function at the given file and line.
// It returns the comment text or an empty string if not found.
func ExtractGodoc(file string, line int) string {
	if file == "" || line <= 0 {
		return ""
	}

	f, fset := godocCache.getFile(file)
	if f == nil {
		return ""
	}

	// Find the function declaration at or near the given line
	var bestFunc *ast.FuncDecl
	bestDistance := int(^uint(0) >> 1) // max int

	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		fnLine := fset.Position(fn.Pos()).Line
		distance := line - fnLine
		// Only consider functions that start at or before the target line
		// and are closer than the current best
		if distance >= 0 && distance < bestDistance {
			bestFunc = fn
			bestDistance = distance
		}
	}

	if bestFunc == nil || bestFunc.Doc == nil {
		return ""
	}

	return strings.TrimSpace(bestFunc.Doc.Text())
}

// Router wraps chi.Router to automatically register source locations.
// Use this when mounting routes with method values to get accurate file/line info.
type Router struct {
	chi.Router
	prefix string
}

// Wrap returns a Router that wraps the given chi.Router with a path prefix.
// The prefix is prepended to all paths when registering source locations.
func Wrap(r chi.Router, prefix string) *Router {
	return &Router{Router: r, prefix: prefix}
}

func (r *Router) fullPath(path string) string {
	if r.prefix == "" {
		return path
	}
	if path == "/" {
		return r.prefix + "/"
	}
	return r.prefix + path
}

func (r *Router) Get(pattern string, h http.HandlerFunc) {
	RegisterSource("GET", r.fullPath(pattern), 1)
	r.Router.Get(pattern, h)
}

func (r *Router) Post(pattern string, h http.HandlerFunc) {
	RegisterSource("POST", r.fullPath(pattern), 1)
	r.Router.Post(pattern, h)
}

func (r *Router) Put(pattern string, h http.HandlerFunc) {
	RegisterSource("PUT", r.fullPath(pattern), 1)
	r.Router.Put(pattern, h)
}

func (r *Router) Delete(pattern string, h http.HandlerFunc) {
	RegisterSource("DELETE", r.fullPath(pattern), 1)
	r.Router.Delete(pattern, h)
}

func (r *Router) Patch(pattern string, h http.HandlerFunc) {
	RegisterSource("PATCH", r.fullPath(pattern), 1)
	r.Router.Patch(pattern, h)
}

// RouteInfo holds information about a single route.
type RouteInfo struct {
	Method      string
	Path        string
	HasAuth     bool
	Handler     string
	Middleware  []string
	Description string // Optional description of the route
	File        string // Source file where handler is defined
	Line        int    // Source line where handler is defined
}

// Config holds configuration for route collection.
type Config struct {
	// ShowAll includes internal chi routes (e.g., /*).
	ShowAll bool

	// AuthPatterns are patterns to detect auth middleware.
	// Default: ["RequiresAuth", "Auth"]
	AuthPatterns []string

	// Descriptions maps "METHOD /path" to a description string.
	// Example: {"GET /api/users": "List all users"}
	Descriptions map[string]string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		AuthPatterns: []string{"RequiresAuth", "Auth"},
	}
}

// CollectRoutes walks a chi router and collects route information.
func CollectRoutes(r chi.Router, cfg ...Config) []RouteInfo {
	config := DefaultConfig()
	if len(cfg) > 0 {
		config = cfg[0]
		if len(config.AuthPatterns) == 0 {
			config.AuthPatterns = DefaultConfig().AuthPatterns
		}
	}

	var routes []RouteInfo

	chi.Walk(r, func(method, path string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		if !config.ShowAll && strings.HasPrefix(path, "/*") {
			return nil
		}

		handlerInfo := getFuncInfo(handler)
		file, line := handlerInfo.file, handlerInfo.line

		// If file is autogenerated (method values), look up in registry
		if file == "<autogenerated>" || file == "" {
			if regFile, regLine, ok := lookupSource(method, path); ok {
				file, line = regFile, regLine
			}
		}

		info := RouteInfo{
			Method:  method,
			Path:    path,
			Handler: handlerInfo.name,
			File:    file,
			Line:    line,
		}

		// Look up description from config, or extract from godoc
		if config.Descriptions != nil {
			key := method + " " + path
			if desc, ok := config.Descriptions[key]; ok {
				info.Description = desc
			}
		}
		// If no description from config, try to extract from godoc
		if info.Description == "" && file != "" && line > 0 {
			info.Description = ExtractGodoc(file, line)
		}

		for _, mw := range middlewares {
			mwName := getFuncName(mw)
			info.Middleware = append(info.Middleware, mwName)

			for _, authPattern := range config.AuthPatterns {
				if strings.Contains(mwName, authPattern) {
					info.HasAuth = true
					break
				}
			}
		}

		routes = append(routes, info)
		return nil
	})

	return routes
}

// FormatTable formats routes as a text table.
func FormatTable(routes []RouteInfo) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("%-8s %-8s %-40s %s\n", "AUTH", "METHOD", "PATH", "HANDLER"))
	sb.WriteString(fmt.Sprintf("%-8s %-8s %-40s %s\n", "----", "------", "----", "-------"))

	for _, info := range routes {
		authStatus := "PUBLIC"
		if info.HasAuth {
			authStatus = "AUTH"
		}

		handlerName := shortenFuncName(info.Handler)
		sb.WriteString(fmt.Sprintf("%-8s %-8s %-40s %s\n", authStatus, info.Method, info.Path, handlerName))
	}

	// Summary
	authCount, publicCount := countRoutes(routes)
	sb.WriteString(fmt.Sprintf("\nSummary: %d routes (%d authenticated, %d public)\n", len(routes), authCount, publicCount))

	return sb.String()
}

// FormatMarkdown generates markdown documentation for routes.
func FormatMarkdown(routes []RouteInfo) string {
	var sb strings.Builder

	sb.WriteString("# API Routes\n\n")
	sb.WriteString(fmt.Sprintf("Generated: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	// Group routes by path prefix
	groups := groupRoutes(routes)

	// Sort group names
	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	for _, groupName := range groupNames {
		groupRoutes := groups[groupName]

		sb.WriteString(fmt.Sprintf("## %s\n\n", groupName))
		sb.WriteString("| Method | Path | Auth | Handler |\n")
		sb.WriteString("|--------|------|------|--------|\n")

		for _, info := range groupRoutes {
			authStatus := "Public"
			if info.HasAuth {
				authStatus = "Required"
			}

			handlerName := shortenFuncName(info.Handler)
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s | %s |\n",
				info.Method, info.Path, authStatus, handlerName))
		}

		sb.WriteString("\n")
	}

	// Summary
	authCount, publicCount := countRoutes(routes)
	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("- **Total routes:** %d\n", len(routes)))
	sb.WriteString(fmt.Sprintf("- **Authenticated:** %d\n", authCount))
	sb.WriteString(fmt.Sprintf("- **Public:** %d\n", publicCount))

	return sb.String()
}

func groupRoutes(routes []RouteInfo) map[string][]RouteInfo {
	// First pass: count routes per two-segment prefix
	// This determines whether routes should be grouped by two segments or fall back to one
	twoSegmentCounts := make(map[string]int)
	for _, info := range routes {
		path := strings.TrimPrefix(info.Path, "/")
		parts := strings.Split(path, "/")

		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" && !strings.HasPrefix(parts[1], "{") {
			prefix := "/" + parts[0] + "/" + parts[1]
			twoSegmentCounts[prefix]++
		}
	}

	// Second pass: assign groups based on prefix sharing
	groups := make(map[string][]RouteInfo)
	for _, info := range routes {
		path := strings.TrimPrefix(info.Path, "/")
		parts := strings.Split(path, "/")

		groupName := "Root"
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" && !strings.HasPrefix(parts[1], "{") {
			twoSegPrefix := "/" + parts[0] + "/" + parts[1]
			if twoSegmentCounts[twoSegPrefix] > 1 {
				// Multiple routes share this prefix - use two segments
				groupName = twoSegPrefix
			} else {
				// Only one route has this prefix - fall back to first segment
				groupName = "/" + parts[0]
			}
		} else if len(parts) > 0 && parts[0] != "" {
			groupName = "/" + parts[0]
		}

		groups[groupName] = append(groups[groupName], info)
	}

	// Sort routes within each group for consistent ordering
	for _, groupRoutes := range groups {
		sort.Slice(groupRoutes, func(i, j int) bool {
			// Sort by path first, then by method
			if groupRoutes[i].Path != groupRoutes[j].Path {
				return groupRoutes[i].Path < groupRoutes[j].Path
			}
			return methodOrder(groupRoutes[i].Method) < methodOrder(groupRoutes[j].Method)
		})
	}

	return groups
}

// methodOrder returns a sort order for HTTP methods (GET, POST, PUT, PATCH, DELETE)
func methodOrder(method string) int {
	switch method {
	case "GET":
		return 0
	case "POST":
		return 1
	case "PUT":
		return 2
	case "PATCH":
		return 3
	case "DELETE":
		return 4
	default:
		return 5
	}
}

func countRoutes(routes []RouteInfo) (authCount, publicCount int) {
	for _, info := range routes {
		if info.HasAuth {
			authCount++
		} else {
			publicCount++
		}
	}
	return
}

// funcInfo holds information extracted from a function pointer.
type funcInfo struct {
	name string
	file string
	line int
}

// getFuncInfo returns the name, file, and line of a function using reflection.
func getFuncInfo(fn any) funcInfo {
	val := reflect.ValueOf(fn)
	if val.Kind() == reflect.Func {
		pc := val.Pointer()
		if f := runtime.FuncForPC(pc); f != nil {
			file, line := f.FileLine(pc)
			return funcInfo{
				name: f.Name(),
				file: file,
				line: line,
			}
		}
	}
	if val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		pc := val.Pointer()
		if f := runtime.FuncForPC(pc); f != nil {
			file, line := f.FileLine(pc)
			return funcInfo{
				name: f.Name(),
				file: file,
				line: line,
			}
		}
	}
	return funcInfo{name: fmt.Sprintf("%T", fn)}
}

// getFuncName returns the name of a function using reflection.
func getFuncName(fn any) string {
	return getFuncInfo(fn).name
}

// shortenFuncName removes the full package path for readability.
func shortenFuncName(name string) string {
	// Remove package path, keep just package.func
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		name = name[idx+1:]
	}
	// Remove -fm suffix from method values
	name = strings.TrimSuffix(name, "-fm")
	// Remove .func1, .func2 suffixes from closures
	if idx := strings.Index(name, ".func"); idx >= 0 {
		name = name[:idx] + ".<closure>"
	}
	return name
}

// shortenFilePath shortens a file path to show only the last 2-3 components.
func shortenFilePath(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, "/")
	if len(parts) <= 3 {
		return path
	}
	return strings.Join(parts[len(parts)-3:], "/")
}

// JSONRoute is the JSON representation of a route.
type JSONRoute struct {
	Method      string   `json:"method"`
	Path        string   `json:"path"`
	Handler     string   `json:"handler"`
	Auth        bool     `json:"auth"`
	Middleware  []string `json:"middleware,omitempty"`
	Description string   `json:"description,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
}

// JSONOutput is the JSON output structure for routes.
type JSONOutput struct {
	Generated     string      `json:"generated"`
	Total         int         `json:"total"`
	Authenticated int         `json:"authenticated"`
	Public        int         `json:"public"`
	Routes        []JSONRoute `json:"routes"`
}

// FormatJSON returns routes as JSON bytes.
func FormatJSON(routes []RouteInfo) ([]byte, error) {
	authCount, publicCount := countRoutes(routes)

	output := JSONOutput{
		Generated:     time.Now().Format(time.RFC3339),
		Total:         len(routes),
		Authenticated: authCount,
		Public:        publicCount,
		Routes:        make([]JSONRoute, len(routes)),
	}

	for i, r := range routes {
		output.Routes[i] = JSONRoute{
			Method:      r.Method,
			Path:        r.Path,
			Handler:     shortenFuncName(r.Handler),
			Auth:        r.HasAuth,
			Middleware:  r.Middleware,
			Description: r.Description,
			File:        r.File,
			Line:        r.Line,
		}
	}

	return json.MarshalIndent(output, "", "  ")
}

// FormatHTML generates HTML documentation for routes.
func FormatHTML(routes []RouteInfo, title string) string {
	if title == "" {
		title = "API Routes"
	}

	authCount, publicCount := countRoutes(routes)
	groups := groupRoutes(routes)

	// Sort group names
	var groupNames []string
	for name := range groups {
		groupNames = append(groupNames, name)
	}
	sort.Strings(groupNames)

	data := struct {
		Title         string
		Generated     string
		Total         int
		Authenticated int
		Public        int
		Groups        []struct {
			Name   string
			Routes []RouteInfo
		}
	}{
		Title:         title,
		Generated:     time.Now().Format("2006-01-02 15:04:05"),
		Total:         len(routes),
		Authenticated: authCount,
		Public:        publicCount,
	}

	for _, name := range groupNames {
		data.Groups = append(data.Groups, struct {
			Name   string
			Routes []RouteInfo
		}{
			Name:   name,
			Routes: groups[name],
		})
	}

	var sb strings.Builder
	tmpl := template.Must(template.New("routes").Funcs(template.FuncMap{
		"shorten":     shortenFuncName,
		"lower":       strings.ToLower,
		"shortenFile": shortenFilePath,
	}).Parse(routesHTMLTemplate))
	tmpl.Execute(&sb, data)
	return sb.String()
}

const routesHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
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
        table {
            width: 100%;
            border-collapse: collapse;
            background: #fff;
            border-radius: 8px;
            overflow: hidden;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
            margin-bottom: 20px;
        }
        th, td { padding: 12px 15px; text-align: left; }
        th { background: #007bff; color: #fff; font-weight: 500; }
        tr:nth-child(even) { background: #f8f9fa; }
        tr:hover { background: #e9ecef; }
        .method {
            font-family: monospace;
            font-weight: bold;
            padding: 3px 8px;
            border-radius: 4px;
            font-size: 0.85em;
        }
        .method-get { background: #28a745; color: #fff; }
        .method-post { background: #007bff; color: #fff; }
        .method-put { background: #ffc107; color: #000; }
        .method-delete { background: #dc3545; color: #fff; }
        .method-patch { background: #17a2b8; color: #fff; }
        .path { font-family: monospace; }
        .handler { font-family: monospace; color: #6c757d; font-size: 0.9em; }
        .description { color: #495057; font-size: 0.9em; }
        .source { font-family: monospace; color: #6c757d; font-size: 0.85em; }
        .auth-badge {
            padding: 3px 8px;
            border-radius: 4px;
            font-size: 0.8em;
            font-weight: 500;
        }
        .auth-required { background: #ffc107; color: #000; }
        .auth-public { background: #28a745; color: #fff; }
        .search-box {
            margin-bottom: 20px;
            padding: 10px 15px;
            width: 100%;
            max-width: 400px;
            border: 1px solid #ddd;
            border-radius: 4px;
            font-size: 1em;
        }
    </style>
</head>
<body>
    <h1>{{.Title}}</h1>

    <div class="summary">
        <div class="summary-item">
            <span class="summary-label">Total:</span>
            <span class="summary-value">{{.Total}}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">Authenticated:</span>
            <span class="summary-value">{{.Authenticated}}</span>
        </div>
        <div class="summary-item">
            <span class="summary-label">Public:</span>
            <span class="summary-value">{{.Public}}</span>
        </div>
        <div class="summary-item generated">
            Generated: {{.Generated}}
        </div>
    </div>

    <input type="text" class="search-box" placeholder="Filter routes..." id="search" onkeyup="filterRoutes()">

    {{range .Groups}}
    <h2>{{.Name}}</h2>
    <table class="routes-table">
        <thead>
            <tr>
                <th style="width: 80px">Method</th>
                <th>Path</th>
                <th style="width: 80px">Auth</th>
                <th>Description</th>
                <th>Source</th>
            </tr>
        </thead>
        <tbody>
            {{range .Routes}}
            <tr>
                <td><span class="method method-{{.Method | lower}}">{{.Method}}</span></td>
                <td class="path">{{.Path}}</td>
                <td>
                    {{if .HasAuth}}
                    <span class="auth-badge auth-required">Required</span>
                    {{else}}
                    <span class="auth-badge auth-public">Public</span>
                    {{end}}
                </td>
                <td class="description">{{if .Description}}{{.Description}}{{else}}<span class="handler">{{shorten .Handler}}</span>{{end}}</td>
                <td class="source">{{if .File}}{{shortenFile .File}}:{{.Line}}{{end}}</td>
            </tr>
            {{end}}
        </tbody>
    </table>
    {{end}}

    <script>
        function filterRoutes() {
            const query = document.getElementById('search').value.toLowerCase();
            const tables = document.querySelectorAll('.routes-table');
            tables.forEach(table => {
                const rows = table.querySelectorAll('tbody tr');
                let visibleCount = 0;
                rows.forEach(row => {
                    const text = row.textContent.toLowerCase();
                    const visible = text.includes(query);
                    row.style.display = visible ? '' : 'none';
                    if (visible) visibleCount++;
                });
                // Hide the group header (h2) if no routes match
                const header = table.previousElementSibling;
                if (header && header.tagName === 'H2') {
                    header.style.display = visibleCount > 0 ? '' : 'none';
                }
                table.style.display = visibleCount > 0 ? '' : 'none';
            });
        }
    </script>
</body>
</html>`
