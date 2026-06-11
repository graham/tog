package db

import (
	"context"
	"testing"
	"time"
)

func setupPreparedExecTestDB(t *testing.T) *DB {
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

func TestRegisterExec(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe, err := RegisterExec(database, "INSERT INTO items (name) VALUES (?)")
	if err != nil {
		t.Fatalf("RegisterExec failed: %v", err)
	}

	if pe == nil {
		t.Fatal("expected non-nil PreparedExec")
	}
}

func TestRegisterExecNamed(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe, err := RegisterExecNamed(database, "InsertItem", "INSERT INTO items (name) VALUES (?)")
	if err != nil {
		t.Fatalf("RegisterExecNamed failed: %v", err)
	}

	info := pe.info()
	if info.Name != "InsertItem" {
		t.Errorf("Name = %q, want %q", info.Name, "InsertItem")
	}
	if info.Type != "exec" {
		t.Errorf("Type = %q, want %q", info.Type, "exec")
	}
}

func TestMustRegisterExec(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe := MustRegisterExec(database, "INSERT INTO items (name) VALUES (?)")
	if pe == nil {
		t.Fatal("expected non-nil PreparedExec")
	}
}

func TestMustRegisterExecNamed(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe := MustRegisterExecNamed(database, "InsertItem", "INSERT INTO items (name) VALUES (?)")
	if pe == nil {
		t.Fatal("expected non-nil PreparedExec")
	}

	info := pe.info()
	if info.Name != "InsertItem" {
		t.Errorf("Name = %q, want %q", info.Name, "InsertItem")
	}
}

func TestPreparedExec_Exec(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe, _ := RegisterExec(database, "INSERT INTO items (name) VALUES (?)")

	result := pe.Exec("test")
	if result.Err() != nil {
		t.Fatalf("Exec failed: %v", result.Err())
	}

	id, err := result.LastInsertID()
	if err != nil {
		t.Fatalf("LastInsertID failed: %v", err)
	}
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
}

func TestPreparedExec_ExecCtx(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	var hookCalled bool
	var capturedDuration time.Duration

	hook := func(name, sql string, duration time.Duration) {
		hookCalled = true
		capturedDuration = duration
	}

	ctx := ContextWithQueryHook(context.Background(), hook)

	pe, _ := RegisterExecNamed(database, "InsertItem", "INSERT INTO items (name) VALUES (?)")

	result := pe.ExecCtx(ctx, "test")
	if result.Err() != nil {
		t.Fatalf("ExecCtx failed: %v", result.Err())
	}

	if !hookCalled {
		t.Error("query hook was not called")
	}
	if capturedDuration <= 0 {
		t.Error("duration should be positive")
	}
}

func TestPreparedExec_ExecTx(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe, _ := RegisterExec(database, "INSERT INTO items (name) VALUES (?)")

	err := database.Tx(func(tx *Tx) error {
		result := pe.ExecTx(tx, "txtest")
		if result.Err() != nil {
			return result.Err()
		}

		id, err := result.LastInsertID()
		if err != nil {
			return err
		}
		if id != 1 {
			t.Errorf("expected id 1, got %d", id)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

func TestPreparedExec_ExecTxCtx(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	var hookCalled bool
	hook := func(name, sql string, duration time.Duration) {
		hookCalled = true
	}

	ctx := ContextWithQueryHook(context.Background(), hook)

	pe, _ := RegisterExecNamed(database, "InsertItem", "INSERT INTO items (name) VALUES (?)")

	err := database.Tx(func(tx *Tx) error {
		result := pe.ExecTxCtx(ctx, tx, "txtest")
		return result.Err()
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	if !hookCalled {
		t.Error("query hook was not called")
	}
}

func TestExecResult_RowsAffected(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	// Insert some data first
	database.Exec(`INSERT INTO items (name) VALUES ('a'), ('b'), ('c')`)

	pe, _ := RegisterExec(database, "UPDATE items SET name = ? WHERE name = ?")

	result := pe.Exec("updated", "a")
	if result.Err() != nil {
		t.Fatalf("Exec failed: %v", result.Err())
	}

	affected, err := result.RowsAffected()
	if err != nil {
		t.Fatalf("RowsAffected failed: %v", err)
	}
	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}
}

func TestExecResult_MustLastInsertID(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe, _ := RegisterExec(database, "INSERT INTO items (name) VALUES (?)")

	result := pe.Exec("test")
	id := result.MustLastInsertID()

	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
}

func TestExecResult_MustRowsAffected(t *testing.T) {
	database := setupPreparedExecTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('toupdate')`)

	pe, _ := RegisterExec(database, "UPDATE items SET name = ?")

	result := pe.Exec("updated")
	affected := result.MustRowsAffected()

	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}
}

func TestExecResult_Error(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe, _ := RegisterExec(database, "INSERT INTO nonexistent (name) VALUES (?)")

	result := pe.Exec("test")
	if result.Err() == nil {
		t.Error("expected error for invalid table")
	}

	// LastInsertID should return error if exec failed
	_, err := result.LastInsertID()
	if err == nil {
		t.Error("expected error from LastInsertID after failed exec")
	}

	// RowsAffected should return error if exec failed
	_, err = result.RowsAffected()
	if err == nil {
		t.Error("expected error from RowsAffected after failed exec")
	}
}

func TestPreparedExec_Verify(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	_, _ = RegisterExec(database, "INSERT INTO items (name) VALUES (?)")

	if err := database.VerifyAll(); err != nil {
		t.Errorf("VerifyAll failed for valid query: %v", err)
	}

	// Register an invalid query
	RegisterExec(database, "INSERT INTO nonexistent (name) VALUES (?)")

	if err := database.VerifyAll(); err == nil {
		t.Error("expected VerifyAll to fail for invalid query")
	}
}

func TestPreparedExec_Desc(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	pe, _ := RegisterExec(database, "INSERT INTO items (name) VALUES (?)")
	pe.Desc("Inserts a new item into the database")

	info := pe.info()
	if info.Description != "Inserts a new item into the database" {
		t.Errorf("Description = %q, want %q", info.Description, "Inserts a new item into the database")
	}
}

func TestPreparedExec_Info_LongQuery(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	longQuery := "INSERT INTO items (name) VALUES ('this is a very long value that exceeds fifty characters definitely')"
	pe, _ := RegisterExec(database, longQuery)

	info := pe.info()
	if len(info.Name) > 53 { // 50 + "..."
		t.Errorf("Name should be truncated, got %d chars", len(info.Name))
	}
}

func TestPreparedExec_sql(t *testing.T) {
	database := setupPreparedExecTestDB(t)

	query := "INSERT INTO items (name) VALUES (?)"
	pe, _ := RegisterExec(database, query)

	if pe.sql() != query {
		t.Errorf("sql() = %q, want %q", pe.sql(), query)
	}
}
