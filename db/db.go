package db

import (
	"context"
	"database/sql"
	"fmt"
	"iter"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// PoolConfig holds connection pool settings.
// These defaults are tuned for production PostgreSQL workloads.
type PoolConfig struct {
	// MaxOpenConns is the maximum number of open connections to the database.
	// For Postgres, a good starting point is (CPU cores * 2) + effective_spindle_count.
	// Default: 25
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections retained.
	// Should be less than or equal to MaxOpenConns.
	// Default: 10
	MaxIdleConns int

	// ConnMaxLifetime is the maximum time a connection can be reused.
	// Helps with load balancers and DNS changes in cloud environments.
	// Default: 30 minutes
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum time a connection can sit idle.
	// Helps free up resources when traffic is low.
	// Default: 5 minutes
	ConnMaxIdleTime time.Duration

	// PingOnOpen verifies database connectivity when opening.
	// Default: true
	PingOnOpen bool
}

// DefaultPoolConfig returns production-ready pool settings.
func DefaultPoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    10,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		PingOnOpen:      true,
	}
}

// SQLitePoolConfig returns settings optimized for SQLite.
// SQLite uses file locking, so concurrent writes are limited.
func SQLitePoolConfig() PoolConfig {
	return PoolConfig{
		MaxOpenConns:    1, // SQLite handles one writer at a time
		MaxIdleConns:    1,
		ConnMaxLifetime: 0, // no limit for file-based db
		ConnMaxIdleTime: 0,
		PingOnOpen:      true,
	}
}

// Querier is the common interface for DB and Tx.
// Both can be used with Query and Exec functions.
type Querier interface {
	Get(dest any, query string, args ...any) error
	Select(dest any, query string, args ...any) error
	Queryx(query string, args ...any) (*sqlx.Rows, error)
	Exec(query string, args ...any) (sql.Result, error)
}

// DB wraps sqlx.DB to provide our query interface.
type DB struct {
	*sqlx.DB
	registered []registeredQuery
	timed      bool // if true, log timing for all queries
}

// Get wraps sqlx.DB.Get with optional timing.
func (db *DB) Get(dest any, query string, args ...any) error {
	if db.timed {
		start := time.Now()
		err := db.DB.Get(dest, query, args...)
		logSQLTiming("", query, time.Since(start))
		return err
	}
	return db.DB.Get(dest, query, args...)
}

// Select wraps sqlx.DB.Select with optional timing.
func (db *DB) Select(dest any, query string, args ...any) error {
	if db.timed {
		start := time.Now()
		err := db.DB.Select(dest, query, args...)
		logSQLTiming("", query, time.Since(start))
		return err
	}
	return db.DB.Select(dest, query, args...)
}

// Queryx wraps sqlx.DB.Queryx with optional timing.
func (db *DB) Queryx(query string, args ...any) (*sqlx.Rows, error) {
	if db.timed {
		start := time.Now()
		rows, err := db.DB.Queryx(query, args...)
		logSQLTiming("", query, time.Since(start))
		return rows, err
	}
	return db.DB.Queryx(query, args...)
}

// Exec wraps sqlx.DB.Exec with optional timing.
func (db *DB) Exec(query string, args ...any) (sql.Result, error) {
	if db.timed {
		start := time.Now()
		result, err := db.DB.Exec(query, args...)
		logSQLTiming("", query, time.Since(start))
		return result, err
	}
	return db.DB.Exec(query, args...)
}

// Tx wraps sqlx.Tx for transaction support.
type Tx struct {
	*sqlx.Tx
	timed bool
}

// Get wraps sqlx.Tx.Get with optional timing.
func (tx *Tx) Get(dest any, query string, args ...any) error {
	if tx.timed {
		start := time.Now()
		err := tx.Tx.Get(dest, query, args...)
		logSQLTiming("", query, time.Since(start))
		return err
	}
	return tx.Tx.Get(dest, query, args...)
}

// Select wraps sqlx.Tx.Select with optional timing.
func (tx *Tx) Select(dest any, query string, args ...any) error {
	if tx.timed {
		start := time.Now()
		err := tx.Tx.Select(dest, query, args...)
		logSQLTiming("", query, time.Since(start))
		return err
	}
	return tx.Tx.Select(dest, query, args...)
}

// Queryx wraps sqlx.Tx.Queryx with optional timing.
func (tx *Tx) Queryx(query string, args ...any) (*sqlx.Rows, error) {
	if tx.timed {
		start := time.Now()
		rows, err := tx.Tx.Queryx(query, args...)
		logSQLTiming("", query, time.Since(start))
		return rows, err
	}
	return tx.Tx.Queryx(query, args...)
}

// Exec wraps sqlx.Tx.Exec with optional timing.
func (tx *Tx) Exec(query string, args ...any) (sql.Result, error) {
	if tx.timed {
		start := time.Now()
		result, err := tx.Tx.Exec(query, args...)
		logSQLTiming("", query, time.Since(start))
		return result, err
	}
	return tx.Tx.Exec(query, args...)
}

// registeredQuery is a non-generic interface for tracking queries.
type registeredQuery interface {
	sql() string
	verify(db *DB) error
	info() QueryInfo
}

// QueryInfo contains information about a registered query for documentation.
type QueryInfo struct {
	Name        string `json:"name"`
	SQL         string `json:"sql"`
	File        string `json:"file"`
	Line        int    `json:"line"`
	Type        string `json:"type"` // "select" or "exec"
	Description string `json:"description,omitempty"`
}

// Open opens a database connection with the given driver and DSN.
// Drivers: "sqlite3", "postgres", "mysql"
// For postgres, import: _ "github.com/lib/pq"
// For mysql, import: _ "github.com/go-sql-driver/mysql"
func Open(driver, dsn string, cfg PoolConfig) (*DB, error) {
	db, err := sqlx.Open(driver, dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)

	if cfg.PingOnOpen {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			db.Close()
			return nil, fmt.Errorf("database ping failed: %w", err)
		}
	}

	// Check if SQL stopwatch is enabled
	initSQLStopwatch()

	return &DB{DB: db, timed: sqlStopwatchEnabled}, nil
}

// OpenDB opens a SQLite database with default SQLite pool settings.
// For production PostgreSQL, use Open() with DefaultPoolConfig().
func OpenDB(dsn string) (*DB, error) {
	return Open("sqlite3", dsn, SQLitePoolConfig())
}

// Tx executes the given function within a transaction.
// If the function returns nil, the transaction is committed.
// If the function returns an error or panics, the transaction is rolled back.
func (db *DB) Tx(fn func(tx *Tx) error) (err error) {
	sqlxTx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	tx := &Tx{Tx: sqlxTx, timed: db.timed}

	defer func() {
		if p := recover(); p != nil {
			_ = sqlxTx.Rollback()
			panic(p) // re-panic after rollback
		} else if err != nil {
			_ = sqlxTx.Rollback()
		} else {
			err = sqlxTx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// MustTx is like Tx but panics on error.
func (db *DB) MustTx(fn func(tx *Tx) error) {
	if err := db.Tx(fn); err != nil {
		panic(fmt.Sprintf("transaction failed: %v", err))
	}
}

// Result holds a pending query that can be executed with First() or Rows().
type Result[T any] struct {
	q     Querier
	query string
	args  []any
	err   error
	ctx   context.Context // optional context for timing hooks
	name  string          // optional query name for logging
}

// logTiming calls the query hook if present in context.
func (r *Result[T]) logTiming(duration time.Duration) {
	if r.ctx == nil {
		return
	}
	if hook := queryHookFromContext(r.ctx); hook != nil {
		hook(r.name, r.query, duration)
	}
}

// Query creates a new Result for the given query and arguments.
// Works with both *DB and *Tx.
// For validation, use Register + VerifyAll instead.
func Query[T any](q Querier, query string, args ...any) *Result[T] {
	// Rebind placeholders for the current driver
	if r, ok := q.(interface{ Rebind(string) string }); ok {
		query = r.Rebind(query)
	}
	return &Result[T]{
		q:     q,
		query: query,
		args:  args,
	}
}

// First executes the query and returns a pointer to the first result.
// Returns nil if no rows found. Panics on database errors.
func (r *Result[T]) First() *T {
	if r.err != nil {
		panic(fmt.Sprintf("query error: %v", r.err))
	}

	start := time.Now()
	var result T
	err := r.q.Get(&result, r.query, r.args...)
	r.logTiming(time.Since(start))

	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		panic(fmt.Sprintf("query error: %v", err))
	}
	return &result
}

// FirstE executes the query and returns the first result with error handling.
// Returns sql.ErrNoRows if no rows found.
func (r *Result[T]) FirstE() (T, error) {
	var zero T
	if r.err != nil {
		return zero, r.err
	}

	start := time.Now()
	var result T
	err := r.q.Get(&result, r.query, r.args...)
	r.logTiming(time.Since(start))
	return result, err
}

// Unique executes the query and returns a pointer to the result, asserting at most one row.
// Returns nil if no rows found. Panics if more than one row or on database errors.
func (r *Result[T]) Unique() *T {
	if r.err != nil {
		panic(fmt.Sprintf("query error: %v", r.err))
	}

	results, err := r.All()
	if err != nil {
		panic(fmt.Sprintf("query error: %v", err))
	}

	if len(results) == 0 {
		return nil
	}
	if len(results) > 1 {
		panic(fmt.Sprintf("unique: expected 0 or 1 row, got %d", len(results)))
	}

	return &results[0]
}

// UniqueE executes the query and returns the result with error handling.
// Returns error if zero rows or more than one row.
func (r *Result[T]) UniqueE() (T, error) {
	var zero T
	if r.err != nil {
		return zero, r.err
	}

	results, err := r.All()
	if err != nil {
		return zero, err
	}

	if len(results) == 0 {
		return zero, fmt.Errorf("unique: no rows returned")
	}
	if len(results) > 1 {
		return zero, fmt.Errorf("unique: expected 1 row, got %d", len(results))
	}

	return results[0], nil
}

// Rows returns an iterator over all results.
// Each iteration yields (value, error). Check error on each iteration.
// Note: Timing is logged when the query starts, not when iteration completes.
func (r *Result[T]) Rows() iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var zero T

		if r.err != nil {
			yield(zero, r.err)
			return
		}

		start := time.Now()
		rows, err := r.q.Queryx(r.query, r.args...)
		r.logTiming(time.Since(start))

		if err != nil {
			yield(zero, err)
			return
		}
		defer rows.Close()

		for rows.Next() {
			var item T
			if err := rows.StructScan(&item); err != nil {
				yield(zero, err)
				return
			}
			if !yield(item, nil) {
				return
			}
		}

		if err := rows.Err(); err != nil {
			yield(zero, err)
		}
	}
}

// All collects all results into a slice.
func (r *Result[T]) All() ([]T, error) {
	var results []T
	for item, err := range r.Rows() {
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, nil
}

// Update executes an UPDATE query and returns the number of rows affected.
func Update(q Querier, query string, args ...any) (int64, error) {
	result, err := q.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// MustUpdate is like Update but panics on error.
func MustUpdate(q Querier, query string, args ...any) int64 {
	n, err := Update(q, query, args...)
	if err != nil {
		panic(fmt.Sprintf("update failed: %v", err))
	}
	return n
}

// Insert executes an INSERT query and returns the last inserted row ID.
// Note: LastInsertId behavior varies by database driver.
// SQLite and MySQL support it; PostgreSQL requires RETURNING clause instead.
func Insert(q Querier, query string, args ...any) (int64, error) {
	result, err := q.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// MustInsert is like Insert but panics on error.
func MustInsert(q Querier, query string, args ...any) int64 {
	id, err := Insert(q, query, args...)
	if err != nil {
		panic(fmt.Sprintf("insert failed: %v", err))
	}
	return id
}

// Delete executes a DELETE query and returns the number of rows affected.
func Delete(q Querier, query string, args ...any) (int64, error) {
	result, err := q.Exec(query, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// MustDelete is like Delete but panics on error.
func MustDelete(q Querier, query string, args ...any) int64 {
	n, err := Delete(q, query, args...)
	if err != nil {
		panic(fmt.Sprintf("delete failed: %v", err))
	}
	return n
}

// VerifyAll validates all registered queries against the database.
// Call this at startup after registering all queries to catch invalid
// table/column names early. Returns all errors encountered.
func (db *DB) VerifyAll() error {
	var errs []error
	for _, rq := range db.registered {
		if err := rq.verify(db); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("query verification failed: %v", errs)
	}
	return nil
}

// MustVerifyAll is like VerifyAll but panics on failure.
func (db *DB) MustVerifyAll() {
	if err := db.VerifyAll(); err != nil {
		panic(err)
	}
}

// RegisteredQueries returns information about all registered queries.
// Useful for documentation and debugging.
func (db *DB) RegisteredQueries() []QueryInfo {
	queries := make([]QueryInfo, len(db.registered))
	for i, rq := range db.registered {
		queries[i] = rq.info()
	}
	return queries
}
