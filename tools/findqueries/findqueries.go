// Package findqueries scans Go source files for SQL queries that are not using the
// registered PreparedQuery pattern. These unregistered queries cannot be
// validated at compile time by the verify tool.
package findqueries

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// UnregisteredQuery represents a SQL query that was found without being registered.
type UnregisteredQuery struct {
	File     string // Source file path
	Line     int    // Line number
	Column   int    // Column number
	Function string // The function/method being called
	SQL      string // The SQL query if we can extract it
	Context  string // Surrounding code context
}

// Config holds configuration for the query finder.
type Config struct {
	// ExcludeTests excludes test files (*_test.go)
	ExcludeTests bool

	// SQLMaxLength truncates SQL queries to this length for display (0 = no limit)
	SQLMaxLength int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() Config {
	return Config{
		ExcludeTests: false,
		SQLMaxLength: 80,
	}
}

// SQL-like patterns to identify query strings
var sqlPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*(SELECT|INSERT|UPDATE|DELETE|CREATE|ALTER|DROP)\s+`),
}

// Find scans a directory for unregistered SQL queries.
func Find(root string, cfg ...Config) ([]UnregisteredQuery, error) {
	config := DefaultConfig()
	if len(cfg) > 0 {
		config = cfg[0]
	}

	var queries []UnregisteredQuery

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files if requested
		if config.ExcludeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip vendor directory
		if strings.Contains(path, "vendor/") {
			return nil
		}

		fileQueries, err := analyzeFile(path, config)
		if err != nil {
			return nil // Continue with other files
		}

		queries = append(queries, fileQueries...)
		return nil
	})

	return queries, err
}

func analyzeFile(path string, config Config) ([]UnregisteredQuery, error) {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var queries []UnregisteredQuery

	// Track SQL strings assigned to variables for later lookup
	sqlVars := make(map[string]string)

	ast.Inspect(node, func(n ast.Node) bool {
		// Track variable assignments that contain SQL
		if assign, ok := n.(*ast.AssignStmt); ok {
			for i, rhs := range assign.Rhs {
				if i < len(assign.Lhs) {
					if ident, ok := assign.Lhs[i].(*ast.Ident); ok {
						// Check if RHS is a Rebind call with SQL
						if call, ok := rhs.(*ast.CallExpr); ok {
							if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
								if sel.Sel.Name == "Rebind" {
									if sql := extractSQLArg(call, 0); sql != "" && looksLikeSQL(sql) {
										sqlVars[ident.Name] = sql
									}
								}
							}
						}
						// Check if RHS is a direct SQL string
						if sql := extractStringLiteral(rhs); sql != "" && looksLikeSQL(sql) {
							sqlVars[ident.Name] = sql
						}
					}
				}
			}
		}

		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		// Check for method calls (receiver.Method())
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			methodName := sel.Sel.Name

			// Detect Rebind() calls with SQL - these indicate raw query usage
			if methodName == "Rebind" {
				if sql := extractSQLArg(call, 0); sql != "" && looksLikeSQL(sql) {
					pos := fset.Position(call.Pos())
					queries = append(queries, UnregisteredQuery{
						File:     pos.Filename,
						Line:     pos.Line,
						Column:   pos.Column,
						Function: "Rebind",
						SQL:      truncateSQL(sql, config.SQLMaxLength),
						Context:  getReceiverName(sel),
					})
				}
				return true
			}

			// Skip if this is a PreparedQuery.Exec() call
			if methodName == "Exec" && !looksLikeRawExec(call, sqlVars) {
				return true
			}

			// Check for raw database operations
			switch methodName {
			case "Exec", "Query", "QueryRow", "Queryx", "Get", "Select":
				sql := extractSQLArg(call, 0)
				// Also check if first arg is a variable containing SQL
				if sql == "" && len(call.Args) > 0 {
					if ident, ok := call.Args[0].(*ast.Ident); ok {
						sql = sqlVars[ident.Name]
					}
				}
				if sql != "" && looksLikeSQL(sql) {
					pos := fset.Position(call.Pos())
					queries = append(queries, UnregisteredQuery{
						File:     pos.Filename,
						Line:     pos.Line,
						Column:   pos.Column,
						Function: methodName,
						SQL:      truncateSQL(sql, config.SQLMaxLength),
						Context:  getReceiverName(sel),
					})
				}
			}
		}

		// Check for package-level function calls (db.Insert, db.Update, etc.)
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "db" {
				funcName := sel.Sel.Name
				switch funcName {
				case "Insert", "Update", "Delete", "MustInsert", "MustUpdate", "MustDelete":
					// These take (querier, query, args...)
					sql := extractSQLArg(call, 1)
					if sql == "" && len(call.Args) > 1 {
						if ident, ok := call.Args[1].(*ast.Ident); ok {
							sql = sqlVars[ident.Name]
						}
					}
					if sql != "" && looksLikeSQL(sql) {
						pos := fset.Position(call.Pos())
						queries = append(queries, UnregisteredQuery{
							File:     pos.Filename,
							Line:     pos.Line,
							Column:   pos.Column,
							Function: "db." + funcName,
							SQL:      truncateSQL(sql, config.SQLMaxLength),
							Context:  "helper function",
						})
					}
				case "Query":
					// db.Query[T](querier, query, args...)
					sql := extractSQLArg(call, 1)
					if sql == "" && len(call.Args) > 1 {
						if ident, ok := call.Args[1].(*ast.Ident); ok {
							sql = sqlVars[ident.Name]
						}
					}
					if sql != "" && looksLikeSQL(sql) {
						pos := fset.Position(call.Pos())
						queries = append(queries, UnregisteredQuery{
							File:     pos.Filename,
							Line:     pos.Line,
							Column:   pos.Column,
							Function: "db.Query",
							SQL:      truncateSQL(sql, config.SQLMaxLength),
							Context:  "unregistered query",
						})
					}
				}
			}
		}

		return true
	})

	// Deduplicate queries
	queries = deduplicateQueries(queries)

	return queries, nil
}

func extractStringLiteral(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		s := lit.Value
		if len(s) >= 2 {
			if s[0] == '`' {
				return s[1 : len(s)-1]
			}
			if s[0] == '"' {
				return s[1 : len(s)-1]
			}
		}
		return s
	}
	return ""
}

func deduplicateQueries(queries []UnregisteredQuery) []UnregisteredQuery {
	if len(queries) <= 1 {
		return queries
	}

	// Sort by file then line
	sort.Slice(queries, func(i, j int) bool {
		if queries[i].File != queries[j].File {
			return queries[i].File < queries[j].File
		}
		return queries[i].Line < queries[j].Line
	})

	// Keep only unique SQL per file
	seen := make(map[string]bool)
	var result []UnregisteredQuery
	for _, q := range queries {
		key := q.File + ":" + q.SQL
		if !seen[key] {
			seen[key] = true
			result = append(result, q)
		}
	}
	return result
}

func looksLikeRawExec(call *ast.CallExpr, sqlVars map[string]string) bool {
	if len(call.Args) == 0 {
		return false
	}

	sql := extractSQLArg(call, 0)
	if sql == "" {
		if ident, ok := call.Args[0].(*ast.Ident); ok {
			sql = sqlVars[ident.Name]
		}
	}
	return sql != "" && looksLikeSQL(sql)
}

func extractSQLArg(call *ast.CallExpr, argIndex int) string {
	if len(call.Args) <= argIndex {
		return ""
	}

	arg := call.Args[argIndex]

	if lit, ok := arg.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		s := lit.Value
		if len(s) >= 2 {
			if s[0] == '`' {
				return s[1 : len(s)-1]
			}
			if s[0] == '"' {
				return s[1 : len(s)-1]
			}
		}
		return s
	}

	return ""
}

func looksLikeSQL(s string) bool {
	s = strings.TrimSpace(s)
	for _, pattern := range sqlPatterns {
		if pattern.MatchString(s) {
			return true
		}
	}
	return false
}

func getReceiverName(sel *ast.SelectorExpr) string {
	switch x := sel.X.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return getReceiverName(x) + "." + x.Sel.Name
	default:
		return "<expr>"
	}
}

func truncateSQL(sql string, maxLen int) string {
	// Normalize whitespace
	sql = strings.Join(strings.Fields(sql), " ")

	if maxLen > 0 && len(sql) > maxLen {
		return sql[:maxLen-3] + "..."
	}
	return sql
}

// FormatTable formats the results as a table.
func FormatTable(queries []UnregisteredQuery, showSQL bool) string {
	var sb strings.Builder

	sb.WriteString("Found unregistered queries:\n\n")
	sb.WriteString("FILE                                               LINE   FUNCTION        SQL\n")
	sb.WriteString("--------------------------------------------------  ----   --------------  ----------------------------------------\n")

	cwd, _ := os.Getwd()

	for _, q := range queries {
		// Make path relative if possible
		relPath := q.File
		if cwd != "" {
			if rel, err := filepath.Rel(cwd, q.File); err == nil {
				relPath = rel
			}
		}

		// Truncate path if too long
		if len(relPath) > 50 {
			relPath = "..." + relPath[len(relPath)-47:]
		}

		sql := ""
		if showSQL {
			sql = q.SQL
		}

		sb.WriteString(strings.ReplaceAll(
			"%-50s  %-4d   %-14s  %s\n",
			"", "",
		))
		sb.WriteString(relPath)
		for i := len(relPath); i < 50; i++ {
			sb.WriteByte(' ')
		}
		sb.WriteString("  ")
		sb.WriteString(strings.TrimSpace(strings.Repeat(" ", 4-len(itoa(q.Line))) + itoa(q.Line)))
		sb.WriteString("   ")
		sb.WriteString(q.Function)
		for i := len(q.Function); i < 14; i++ {
			sb.WriteByte(' ')
		}
		sb.WriteString("  ")
		sb.WriteString(sql)
		sb.WriteByte('\n')
	}

	return sb.String()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
