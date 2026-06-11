package testkit

import "github.com/graham/tog/db"

// Query executes a query on the default database and returns all results.
// Fails the test on error.
func Query[T any](app *TestApp, query string, args ...any) []T {
	app.T.Helper()
	result := db.Query[T](app.DB(), query, args...)
	rows, err := result.All()
	if err != nil {
		app.T.Fatalf("query failed: %v", err)
	}
	return rows
}

// QueryOne executes a query on the default database and returns the first result.
// Fails the test on error.
func QueryOne[T any](app *TestApp, query string, args ...any) T {
	app.T.Helper()
	result := db.Query[T](app.DB(), query, args...)
	row, err := result.FirstE()
	if err != nil {
		app.T.Fatalf("query failed: %v", err)
	}
	return row
}

// QueryDB executes a query against a named database and returns all results.
// Fails the test on error.
func QueryDB[T any](app *TestApp, dbName string, query string, args ...any) []T {
	app.T.Helper()
	database := app.GetDB(dbName)
	if database == nil {
		app.T.Fatalf("database %q not found", dbName)
	}
	result := db.Query[T](database, query, args...)
	rows, err := result.All()
	if err != nil {
		app.T.Fatalf("query failed: %v", err)
	}
	return rows
}

// QueryOneDB executes a query against a named database and returns the first result.
// Fails the test on error.
func QueryOneDB[T any](app *TestApp, dbName string, query string, args ...any) T {
	app.T.Helper()
	database := app.GetDB(dbName)
	if database == nil {
		var zero T
		app.T.Fatalf("database %q not found", dbName)
		return zero
	}
	result := db.Query[T](database, query, args...)
	row, err := result.FirstE()
	if err != nil {
		app.T.Fatalf("query failed: %v", err)
	}
	return row
}

// Exec executes a command on the default database.
// Fails the test on error.
func Exec(app *TestApp, query string, args ...any) {
	app.T.Helper()
	if _, err := app.DB().Exec(query, args...); err != nil {
		app.T.Fatalf("exec failed: %v", err)
	}
}

// ExecDB executes a command on a named database.
// Fails the test on error.
func ExecDB(app *TestApp, dbName string, query string, args ...any) {
	app.T.Helper()
	database := app.GetDB(dbName)
	if database == nil {
		app.T.Fatalf("database %q not found", dbName)
	}
	if _, err := database.Exec(query, args...); err != nil {
		app.T.Fatalf("exec failed: %v", err)
	}
}
