package testkit

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
)

// AssertRowExists verifies that at least one row exists matching the given conditions.
// Fails the test with a helpful message if no row is found.
//
// Example:
//
//	app.AssertRowExists(t, "users", map[string]any{"email": "test@example.com"})
func (app *TestApp) AssertRowExists(t *testing.T, table string, where map[string]any) {
	t.Helper()
	count := app.countRows(t, table, where)
	if count == 0 {
		t.Errorf("expected row to exist in %q with %v, but found none", table, where)
	}
}

// AssertRowNotExists verifies that no rows exist matching the given conditions.
// Fails the test with a helpful message if a row is found.
//
// Example:
//
//	app.AssertRowNotExists(t, "users", map[string]any{"email": "deleted@example.com"})
func (app *TestApp) AssertRowNotExists(t *testing.T, table string, where map[string]any) {
	t.Helper()
	count := app.countRows(t, table, where)
	if count > 0 {
		t.Errorf("expected no rows in %q with %v, but found %d", table, where, count)
	}
}

// AssertRowCount verifies the exact number of rows matching the given conditions.
// Fails the test if the count doesn't match the expected value.
//
// Example:
//
//	app.AssertRowCount(t, "items", map[string]any{"owner_id": 1}, 3)
func (app *TestApp) AssertRowCount(t *testing.T, table string, where map[string]any, expected int) {
	t.Helper()
	count := app.countRows(t, table, where)
	if count != expected {
		t.Errorf("expected %d rows in %q with %v, but found %d", expected, table, where, count)
	}
}

// AssertRowValue verifies that a row exists with the given conditions and checks a specific column's value.
// Fails the test if the row doesn't exist or the value doesn't match.
//
// Example:
//
//	app.AssertRowValue(t, "users", map[string]any{"id": 1}, "email", "admin@example.com")
func (app *TestApp) AssertRowValue(t *testing.T, table string, where map[string]any, column string, expected any) {
	t.Helper()

	query, args := buildSelectQuery(table, column, where)
	row := app.DB().DB.DB.QueryRow(query, args...)

	var value any
	if err := row.Scan(&value); err != nil {
		if err == sql.ErrNoRows {
			t.Errorf("no row found in %q with %v to check column %q", table, where, column)
		} else {
			t.Errorf("failed to query %q: %v", table, err)
		}
		return
	}

	// Handle type conversions for comparison
	if !valuesEqual(value, expected) {
		t.Errorf("expected %q.%s = %v (type %T), but got %v (type %T) for row %v",
			table, column, expected, expected, value, value, where)
	}
}

// QueryRow executes a raw SQL query and returns a *sql.Row for custom scanning.
// This is useful for complex assertions that the helper methods don't cover.
//
// Example:
//
//	var count int
//	app.QueryRow(t, "SELECT COUNT(*) FROM items WHERE price > ?", 10.0).Scan(&count)
func (app *TestApp) QueryRow(t *testing.T, query string, args ...any) *sql.Row {
	t.Helper()
	return app.DB().DB.DB.QueryRow(query, args...)
}

// QueryRows executes a raw SQL query and returns *sql.Rows for custom iteration.
// The caller is responsible for closing the rows.
//
// Example:
//
//	rows, err := app.QueryRows(t, "SELECT id, name FROM items WHERE owner_id = ?", 1)
//	if err != nil {
//	    t.Fatal(err)
//	}
//	defer rows.Close()
func (app *TestApp) QueryRows(t *testing.T, query string, args ...any) (*sql.Rows, error) {
	t.Helper()
	return app.DB().DB.DB.Query(query, args...)
}

// Exec executes a raw SQL statement for test setup or teardown.
// Returns the result for checking rows affected if needed.
//
// Example:
//
//	app.Exec(t, "DELETE FROM items WHERE id = ?", 999)
func (app *TestApp) Exec(t *testing.T, query string, args ...any) sql.Result {
	t.Helper()
	result, err := app.DB().DB.DB.Exec(query, args...)
	if err != nil {
		t.Fatalf("failed to exec query: %v", err)
	}
	return result
}

// countRows counts rows matching the given conditions.
func (app *TestApp) countRows(t *testing.T, table string, where map[string]any) int {
	t.Helper()

	query, args := buildCountQuery(table, where)
	row := app.DB().DB.DB.QueryRow(query, args...)

	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count rows in %q: %v", table, err)
	}
	return count
}

// buildCountQuery builds a COUNT query with WHERE conditions.
func buildCountQuery(table string, where map[string]any) (string, []any) {
	if len(where) == 0 {
		return fmt.Sprintf("SELECT COUNT(*) FROM %s", table), nil
	}

	conditions := make([]string, 0, len(where))
	args := make([]any, 0, len(where))
	i := 1

	for col, val := range where {
		if val == nil {
			conditions = append(conditions, fmt.Sprintf("%s IS NULL", col))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", col, i))
			args = append(args, val)
			i++
		}
	}

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", table, strings.Join(conditions, " AND "))
	return query, args
}

// buildSelectQuery builds a SELECT query for a single column with WHERE conditions.
func buildSelectQuery(table, column string, where map[string]any) (string, []any) {
	if len(where) == 0 {
		return fmt.Sprintf("SELECT %s FROM %s LIMIT 1", column, table), nil
	}

	conditions := make([]string, 0, len(where))
	args := make([]any, 0, len(where))
	i := 1

	for col, val := range where {
		if val == nil {
			conditions = append(conditions, fmt.Sprintf("%s IS NULL", col))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s = $%d", col, i))
			args = append(args, val)
			i++
		}
	}

	query := fmt.Sprintf("SELECT %s FROM %s WHERE %s LIMIT 1", column, table, strings.Join(conditions, " AND "))
	return query, args
}

// valuesEqual compares two values, handling common type mismatches.
func valuesEqual(got, expected any) bool {
	// Direct equality
	if got == expected {
		return true
	}

	// Handle numeric type conversions (database may return int64 for int)
	switch e := expected.(type) {
	case int:
		if g, ok := got.(int64); ok {
			return int64(e) == g
		}
	case int64:
		if g, ok := got.(int); ok {
			return e == int64(g)
		}
	case float64:
		if g, ok := got.(float64); ok {
			return e == g
		}
	case string:
		if g, ok := got.(string); ok {
			return e == g
		}
		// SQLite may return []byte for TEXT
		if g, ok := got.([]byte); ok {
			return e == string(g)
		}
	}

	return false
}
