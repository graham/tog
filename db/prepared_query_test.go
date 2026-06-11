package db

import (
	"context"
	"testing"
	"time"
)

type preparedItem struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func setupPreparedQueryTestDB(t *testing.T) *DB {
	t.Helper()
	database, err := OpenDB(":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	_, err = database.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	return database
}

func TestPreparedQuery_sql(t *testing.T) {
	database := setupPreparedQueryTestDB(t)

	query := "SELECT id, name FROM items"
	pq, err := Register[preparedItem](database, query)
	if err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if pq.sql() != query {
		t.Errorf("sql() = %q, want %q", pq.sql(), query)
	}
}

func TestPreparedQuery_Desc(t *testing.T) {
	database := setupPreparedQueryTestDB(t)

	pq, _ := Register[preparedItem](database, "SELECT id, name FROM items")
	pq.Desc("Fetches all items from the database")

	info := pq.info()
	if info.Description != "Fetches all items from the database" {
		t.Errorf("Description = %q, want %q", info.Description, "Fetches all items from the database")
	}
}

func TestRegisterNamed(t *testing.T) {
	database := setupPreparedQueryTestDB(t)

	pq, err := RegisterNamed[preparedItem](database, "GetAllItems", "SELECT id, name FROM items")
	if err != nil {
		t.Fatalf("RegisterNamed failed: %v", err)
	}

	info := pq.info()
	if info.Name != "GetAllItems" {
		t.Errorf("Name = %q, want %q", info.Name, "GetAllItems")
	}
	if info.Type != "select" {
		t.Errorf("Type = %q, want %q", info.Type, "select")
	}
}

func TestMustRegister(t *testing.T) {
	database := setupPreparedQueryTestDB(t)

	pq := MustRegister[preparedItem](database, "SELECT id, name FROM items")
	if pq == nil {
		t.Fatal("MustRegister returned nil")
	}

	// Verify it works
	database.Exec(`INSERT INTO items (name) VALUES ('test')`)
	items, err := pq.Exec().All()
	if err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

func TestMustRegisterNamed(t *testing.T) {
	database := setupPreparedQueryTestDB(t)

	pq := MustRegisterNamed[preparedItem](database, "GetItems", "SELECT id, name FROM items")
	if pq == nil {
		t.Fatal("MustRegisterNamed returned nil")
	}

	info := pq.info()
	if info.Name != "GetItems" {
		t.Errorf("Name = %q, want %q", info.Name, "GetItems")
	}
}

func TestPreparedQuery_ExecCtx(t *testing.T) {
	database := setupPreparedQueryTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('test')`)

	var hookCalled bool
	var capturedName string
	var capturedDuration time.Duration

	hook := func(name, sql string, duration time.Duration) {
		hookCalled = true
		capturedName = name
		capturedDuration = duration
	}

	ctx := ContextWithQueryHook(context.Background(), hook)

	pq, _ := RegisterNamed[preparedItem](database, "GetItems", "SELECT id, name FROM items WHERE name = ?")

	items, err := pq.ExecCtx(ctx, "test").All()
	if err != nil {
		t.Fatalf("ExecCtx failed: %v", err)
	}

	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}

	if !hookCalled {
		t.Error("query hook was not called")
	}
	if capturedName != "GetItems" {
		t.Errorf("hook name = %q, want %q", capturedName, "GetItems")
	}
	if capturedDuration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestPreparedQuery_ExecTx(t *testing.T) {
	database := setupPreparedQueryTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('txtest')`)

	pq, _ := Register[preparedItem](database, "SELECT id, name FROM items WHERE name = ?")

	err := database.Tx(func(tx *Tx) error {
		items, err := pq.ExecTx(tx, "txtest").All()
		if err != nil {
			return err
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item in tx, got %d", len(items))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

func TestPreparedQuery_ExecTxCtx(t *testing.T) {
	database := setupPreparedQueryTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('txctxtest')`)

	var hookCalled bool
	hook := func(name, sql string, duration time.Duration) {
		hookCalled = true
	}

	ctx := ContextWithQueryHook(context.Background(), hook)

	pq, _ := RegisterNamed[preparedItem](database, "GetItem", "SELECT id, name FROM items WHERE name = ?")

	err := database.Tx(func(tx *Tx) error {
		items, err := pq.ExecTxCtx(ctx, tx, "txctxtest").All()
		if err != nil {
			return err
		}
		if len(items) != 1 {
			t.Errorf("expected 1 item in tx, got %d", len(items))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	if !hookCalled {
		t.Error("query hook was not called")
	}
}

func TestPreparedQuery_info_LongQueryName(t *testing.T) {
	database := setupPreparedQueryTestDB(t)

	longQuery := "SELECT id, name FROM items WHERE name = 'this is a very long value that exceeds fifty characters definitely'"
	pq, _ := Register[preparedItem](database, longQuery)

	info := pq.info()
	// Name should be truncated to 50 chars + "..."
	if len(info.Name) > 53 {
		t.Errorf("Name should be truncated, got %d chars", len(info.Name))
	}
	if len(info.Name) < 50 {
		t.Errorf("Name should be at least 50 chars, got %d", len(info.Name))
	}
}

func TestTx_Get(t *testing.T) {
	database := setupPreparedQueryTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('gettest')`)

	err := database.Tx(func(tx *Tx) error {
		var item preparedItem
		err := tx.Get(&item, "SELECT id, name FROM items WHERE name = ?", "gettest")
		if err != nil {
			return err
		}
		if item.Name != "gettest" {
			t.Errorf("Name = %q, want %q", item.Name, "gettest")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

func TestTx_Queryx(t *testing.T) {
	database := setupPreparedQueryTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('a'), ('b')`)

	err := database.Tx(func(tx *Tx) error {
		rows, err := tx.Queryx("SELECT id, name FROM items ORDER BY name")
		if err != nil {
			return err
		}
		defer rows.Close()

		count := 0
		for rows.Next() {
			count++
		}
		if count != 2 {
			t.Errorf("expected 2 rows, got %d", count)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}
