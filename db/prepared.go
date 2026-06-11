package db

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"runtime"
	"time"
)

// PreparedQuery is a pre-validated query that can be executed with arguments.
type PreparedQuery[T any] struct {
	db          *DB
	query       string
	name        string // optional name for error messages
	description string // optional description for documentation
	err         error  // validation error captured at registration
	file        string // source file where query was registered
	line        int    // source line where query was registered
}

// sql implements registeredQuery.
func (pq *PreparedQuery[T]) sql() string {
	return pq.query
}

// info implements registeredQuery.
func (pq *PreparedQuery[T]) info() QueryInfo {
	name := pq.name
	if name == "" {
		name = pq.query
		if len(name) > 50 {
			name = name[:50] + "..."
		}
	}
	return QueryInfo{
		Name:        name,
		SQL:         pq.query,
		File:        pq.file,
		Line:        pq.line,
		Type:        "select",
		Description: pq.description,
	}
}

// Desc sets the description for documentation and returns the query for chaining.
func (pq *PreparedQuery[T]) Desc(description string) *PreparedQuery[T] {
	pq.description = description
	return pq
}

// verify implements registeredQuery. It executes the query against the database
// to validate table/column names and checks that returned columns match struct tags.
func (pq *PreparedQuery[T]) verify(db *DB) error {
	queryName := pq.query
	if pq.name != "" {
		queryName = pq.name
	}

	// Get column names by executing query wrapped to return no rows
	columns, err := db.getQueryColumns(pq.query)
	if err != nil {
		return fmt.Errorf("%s:%d: query %q: %w", pq.file, pq.line, queryName, err)
	}

	// Check that all returned columns have matching struct tags
	dbTags := getDBTags[T]()
	for _, col := range columns {
		if _, ok := dbTags[col]; !ok {
			return fmt.Errorf("%s:%d: query %q: column %q has no matching db: tag in struct", pq.file, pq.line, queryName, col)
		}
	}

	return nil
}

// getQueryColumns executes a query wrapped to return no rows and returns column names.
func (db *DB) getQueryColumns(query string) ([]string, error) {
	// Rebind placeholders for the current driver
	query = db.Rebind(query)

	// Wrap query to return 0 rows while preserving column metadata
	wrappedQuery := fmt.Sprintf("SELECT * FROM (%s) AS _q WHERE 1=0", query)

	// Count placeholders to provide dummy args (count $ or ? placeholders)
	argCount := countPlaceholders(query)
	args := make([]any, argCount)
	for i := range args {
		args[i] = nil
	}

	rows, err := db.Queryx(wrappedQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return rows.Columns()
}

// countPlaceholders counts placeholders in a query (both ? and $N style).
func countPlaceholders(query string) int {
	count := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			count++
		} else if query[i] == '$' && i+1 < len(query) && query[i+1] >= '1' && query[i+1] <= '9' {
			count++
			// Skip the digits
			for i+1 < len(query) && query[i+1] >= '0' && query[i+1] <= '9' {
				i++
			}
		}
	}
	return count
}

// Register creates a reusable query and adds it to the verification registry.
// Call VerifyAll() after registering all queries to validate them against the database.
// Use MustRegister for queries that should panic on registration failure.
func Register[T any](db *DB, query string) (*PreparedQuery[T], error) {
	_, file, line, _ := runtime.Caller(1)
	pq := &PreparedQuery[T]{
		db:    db,
		query: query,
		file:  file,
		line:  line,
	}

	db.registered = append(db.registered, pq)
	return pq, nil
}

// RegisterNamed is like Register but includes a name for better error messages.
func RegisterNamed[T any](db *DB, name, query string) (*PreparedQuery[T], error) {
	_, file, line, _ := runtime.Caller(1)
	pq, err := Register[T](db, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	pq.name = name
	pq.file = file
	pq.line = line
	return pq, nil
}

// MustRegister is like Register but panics if validation fails.
// Useful for package-level query registration at startup.
func MustRegister[T any](db *DB, query string) *PreparedQuery[T] {
	_, file, line, _ := runtime.Caller(1)
	pq, err := Register[T](db, query)
	if err != nil {
		panic(fmt.Sprintf("query registration failed: %v", err))
	}
	pq.file = file
	pq.line = line
	return pq
}

// MustRegisterNamed is like RegisterNamed but panics if validation fails.
func MustRegisterNamed[T any](db *DB, name, query string) *PreparedQuery[T] {
	_, file, line, _ := runtime.Caller(1)
	pq, err := RegisterNamed[T](db, name, query)
	if err != nil {
		panic(fmt.Sprintf("query registration failed: %v", err))
	}
	pq.file = file
	pq.line = line
	return pq
}

// Exec executes the prepared query with the given arguments using the original DB.
func (pq *PreparedQuery[T]) Exec(args ...any) *Result[T] {
	return &Result[T]{
		q:     pq.db,
		query: pq.db.Rebind(pq.query),
		args:  args,
		err:   pq.err,
	}
}

// ExecCtx executes the prepared query with context for timing/logging.
// If the context contains a QueryHook (via ContextWithQueryHook), it will be
// called with timing information after query execution.
func (pq *PreparedQuery[T]) ExecCtx(ctx context.Context, args ...any) *Result[T] {
	return &Result[T]{
		q:     pq.db,
		query: pq.db.Rebind(pq.query),
		args:  args,
		err:   pq.err,
		ctx:   ctx,
		name:  pq.name,
	}
}

// ExecTx executes the prepared query within a transaction.
func (pq *PreparedQuery[T]) ExecTx(tx *Tx, args ...any) *Result[T] {
	return &Result[T]{
		q:     tx,
		query: tx.Rebind(pq.query),
		args:  args,
		err:   pq.err,
	}
}

// ExecTxCtx executes the prepared query within a transaction with context for timing.
func (pq *PreparedQuery[T]) ExecTxCtx(ctx context.Context, tx *Tx, args ...any) *Result[T] {
	return &Result[T]{
		q:     tx,
		query: tx.Rebind(pq.query),
		args:  args,
		err:   pq.err,
		ctx:   ctx,
		name:  pq.name,
	}
}

// getDBTags extracts all db: tag values from struct T.
func getDBTags[T any]() map[string]bool {
	tags := make(map[string]bool)
	var t T
	typ := reflect.TypeOf(t)

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if tag := field.Tag.Get("db"); tag != "" && tag != "-" {
			tags[tag] = true
		}
	}
	return tags
}

// PreparedExec is a pre-validated exec query (INSERT/UPDATE/DELETE).
// Unlike PreparedQuery[T], it doesn't return rows - use for mutations.
type PreparedExec struct {
	db          *DB
	query       string
	name        string // optional name for error messages
	description string // optional description for documentation
	file        string // source file where query was registered
	line        int    // source line where query was registered
}

// sql implements registeredQuery.
func (pe *PreparedExec) sql() string {
	return pe.query
}

// info implements registeredQuery.
func (pe *PreparedExec) info() QueryInfo {
	name := pe.name
	if name == "" {
		name = pe.query
		if len(name) > 50 {
			name = name[:50] + "..."
		}
	}
	return QueryInfo{
		Name:        name,
		SQL:         pe.query,
		File:        pe.file,
		Line:        pe.line,
		Type:        "exec",
		Description: pe.description,
	}
}

// Desc sets the description for documentation and returns the query for chaining.
func (pe *PreparedExec) Desc(description string) *PreparedExec {
	pe.description = description
	return pe
}

// verify implements registeredQuery. It uses PREPARE to validate the query.
// PREPARE validates table/column names exist and SQL syntax is correct.
func (pe *PreparedExec) verify(db *DB) error {
	queryName := pe.query
	if pe.name != "" {
		queryName = pe.name
	}

	// Rebind placeholders for the current driver
	query := db.Rebind(pe.query)

	// PREPARE validates table/column names exist
	stmt, err := db.Preparex(query)
	if err != nil {
		return fmt.Errorf("%s:%d: query %q: %w", pe.file, pe.line, queryName, err)
	}
	stmt.Close()
	return nil
}

// RegisterExec registers an INSERT/UPDATE/DELETE query for verification.
// Call VerifyAll() after registering all queries to validate them against the database.
func RegisterExec(db *DB, query string) (*PreparedExec, error) {
	_, file, line, _ := runtime.Caller(1)
	pe := &PreparedExec{
		db:    db,
		query: query,
		file:  file,
		line:  line,
	}

	db.registered = append(db.registered, pe)
	return pe, nil
}

// RegisterExecNamed is like RegisterExec but includes a name for better error messages.
func RegisterExecNamed(db *DB, name, query string) (*PreparedExec, error) {
	_, file, line, _ := runtime.Caller(1)
	pe, err := RegisterExec(db, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	pe.name = name
	pe.file = file
	pe.line = line
	return pe, nil
}

// MustRegisterExec is like RegisterExec but panics on error.
func MustRegisterExec(db *DB, query string) *PreparedExec {
	_, file, line, _ := runtime.Caller(1)
	pe, err := RegisterExec(db, query)
	if err != nil {
		panic(fmt.Sprintf("exec registration failed: %v", err))
	}
	pe.file = file
	pe.line = line
	return pe
}

// MustRegisterExecNamed is like RegisterExecNamed but panics on error.
func MustRegisterExecNamed(db *DB, name, query string) *PreparedExec {
	_, file, line, _ := runtime.Caller(1)
	pe, err := RegisterExecNamed(db, name, query)
	if err != nil {
		panic(fmt.Sprintf("exec registration failed: %v", err))
	}
	pe.file = file
	pe.line = line
	return pe
}

// ExecResult wraps sql.Result with convenience methods.
type ExecResult struct {
	result sql.Result
	err    error
}

// Exec executes the prepared exec query with the given arguments.
func (pe *PreparedExec) Exec(args ...any) *ExecResult {
	query := pe.db.Rebind(pe.query)
	result, err := pe.db.DB.Exec(query, args...)
	return &ExecResult{result: result, err: err}
}

// ExecCtx executes the prepared exec query with context for timing/logging.
func (pe *PreparedExec) ExecCtx(ctx context.Context, args ...any) *ExecResult {
	query := pe.db.Rebind(pe.query)
	start := time.Now()
	result, err := pe.db.DB.Exec(query, args...)
	duration := time.Since(start)

	// Log timing if hook present
	if hook := queryHookFromContext(ctx); hook != nil {
		hook(pe.name, pe.query, duration)
	}

	return &ExecResult{result: result, err: err}
}

// ExecTx executes the prepared exec query within a transaction.
func (pe *PreparedExec) ExecTx(tx *Tx, args ...any) *ExecResult {
	query := tx.Rebind(pe.query)
	result, err := tx.Exec(query, args...)
	return &ExecResult{result: result, err: err}
}

// ExecTxCtx executes the prepared exec query within a transaction with context for timing.
func (pe *PreparedExec) ExecTxCtx(ctx context.Context, tx *Tx, args ...any) *ExecResult {
	query := tx.Rebind(pe.query)
	start := time.Now()
	result, err := tx.Exec(query, args...)
	duration := time.Since(start)

	// Log timing if hook present
	if hook := queryHookFromContext(ctx); hook != nil {
		hook(pe.name, pe.query, duration)
	}

	return &ExecResult{result: result, err: err}
}

// Err returns any error from the exec operation.
func (r *ExecResult) Err() error {
	return r.err
}

// LastInsertID returns the last inserted row ID.
// Note: Behavior varies by driver. SQLite and MySQL support it;
// PostgreSQL requires RETURNING clause instead.
func (r *ExecResult) LastInsertID() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.result.LastInsertId()
}

// RowsAffected returns the number of rows affected by the query.
func (r *ExecResult) RowsAffected() (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.result.RowsAffected()
}

// MustLastInsertID returns the last inserted row ID or panics on error.
func (r *ExecResult) MustLastInsertID() int64 {
	id, err := r.LastInsertID()
	if err != nil {
		panic(fmt.Sprintf("last insert id failed: %v", err))
	}
	return id
}

// MustRowsAffected returns the number of rows affected or panics on error.
func (r *ExecResult) MustRowsAffected() int64 {
	n, err := r.RowsAffected()
	if err != nil {
		panic(fmt.Sprintf("rows affected failed: %v", err))
	}
	return n
}
