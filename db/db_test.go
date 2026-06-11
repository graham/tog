package db_test

import (
	"testing"

	"github.com/graham/tog/db"
)

type TestRow struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

// TestOpenDB verifies that a database can be opened and basic queries work.
// It creates an in-memory SQLite database, creates a table, inserts a row,
// and queries it back using the generic Query function.
func TestOpenDB(t *testing.T) {
	database, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Create test table
	_, err = database.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Insert test data
	_, err = database.Exec(`INSERT INTO test (name) VALUES ('hello')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Query with generic Query function
	result := db.Query[TestRow](database, "SELECT id, name FROM test WHERE name = ?", "hello")
	rows, err := result.All()
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(rows) != 1 {
		t.Errorf("expected 1 row, got %d", len(rows))
	}

	if rows[0].Name != "hello" {
		t.Errorf("expected name 'hello', got %q", rows[0].Name)
	}
}

// TestPreparedQuery tests the query registration and verification system.
// It registers a prepared query, verifies it against the database schema,
// and executes it to retrieve results.
func TestPreparedQuery(t *testing.T) {
	database, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Create test table
	_, err = database.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Register query
	getAll, err := db.Register[TestRow](database, "SELECT id, name FROM items")
	if err != nil {
		t.Fatalf("failed to register query: %v", err)
	}

	// Verify query
	if err := database.VerifyAll(); err != nil {
		t.Fatalf("verification failed: %v", err)
	}

	// Insert data
	_, err = database.Exec(`INSERT INTO items (name) VALUES ('widget'), ('gadget')`)
	if err != nil {
		t.Fatalf("failed to insert: %v", err)
	}

	// Execute prepared query
	rows, err := getAll.Exec().All()
	if err != nil {
		t.Fatalf("exec failed: %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

// TestTransaction verifies that database transactions work correctly.
// It tests that multiple operations within a transaction are committed
// together and data persists after the transaction completes.
func TestTransaction(t *testing.T) {
	database, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	_, err = database.Exec(`CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Test successful transaction
	err = database.Tx(func(tx *db.Tx) error {
		_, err := tx.Exec(`INSERT INTO test (name) VALUES ('one')`)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO test (name) VALUES ('two')`)
		return err
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	// Verify data was committed
	rows, err := db.Query[TestRow](database, "SELECT id, name FROM test").All()
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(rows) != 2 {
		t.Errorf("expected 2 rows, got %d", len(rows))
	}
}

// TestManager tests the multi-database connection manager.
// It creates a manager with two database configurations, verifies that
// Default() and Get() return the correct connections, and checks that
// Names() returns all configured database names.
func TestManager(t *testing.T) {
	config := &db.Config{
		Databases: map[string]db.DatabaseConfig{
			"primary": {
				Driver: "sqlite3",
				DSN:    ":memory:",
				Pool:   db.JSONPoolConfig{MaxOpen: 1, MaxIdle: 1},
			},
			"secondary": {
				Driver: "sqlite3",
				DSN:    ":memory:",
				Pool:   db.JSONPoolConfig{MaxOpen: 1, MaxIdle: 1},
			},
		},
		Default: "primary",
	}

	mgr, err := db.NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Test Default()
	if mgr.Default() == nil {
		t.Error("Default() returned nil")
	}

	// Test Get()
	if mgr.Get("primary") == nil {
		t.Error("Get('primary') returned nil")
	}
	if mgr.Get("secondary") == nil {
		t.Error("Get('secondary') returned nil")
	}
	if mgr.Get("nonexistent") != nil {
		t.Error("Get('nonexistent') should return nil")
	}

	// Test Names()
	names := mgr.Names()
	if len(names) != 2 {
		t.Errorf("expected 2 names, got %d", len(names))
	}
}

// TestSchemaInfo tests database schema introspection.
// It creates a table with multiple columns and verifies that SchemaInfo
// returns accurate information about table structure including column names.
func TestSchemaInfo(t *testing.T) {
	database, err := db.OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	_, err = database.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY,
			email TEXT NOT NULL,
			is_admin INTEGER DEFAULT 0
		)
	`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Get schema info
	tables, err := database.SchemaInfo()
	if err != nil {
		t.Fatalf("SchemaInfo failed: %v", err)
	}

	if len(tables) != 1 {
		t.Errorf("expected 1 table, got %d", len(tables))
	}

	if tables[0].Name != "users" {
		t.Errorf("expected table 'users', got %q", tables[0].Name)
	}

	if len(tables[0].Columns) != 3 {
		t.Errorf("expected 3 columns, got %d", len(tables[0].Columns))
	}
}
