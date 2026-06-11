package db

import (
	"errors"
	"testing"
)

type crudItem struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func setupCRUDTestDB(t *testing.T) *DB {
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

func TestInsert(t *testing.T) {
	database := setupCRUDTestDB(t)

	id, err := Insert(database, "INSERT INTO items (name) VALUES (?)", "test")
	if err != nil {
		t.Fatalf("Insert failed: %v", err)
	}

	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}

	// Verify the row was inserted
	result := Query[crudItem](database, "SELECT id, name FROM items WHERE id = ?", id)
	item := result.First()
	if item == nil {
		t.Fatal("inserted row not found")
	}
	if item.Name != "test" {
		t.Errorf("Name = %q, want %q", item.Name, "test")
	}
}

func TestInsert_Error(t *testing.T) {
	database := setupCRUDTestDB(t)

	_, err := Insert(database, "INSERT INTO nonexistent (name) VALUES (?)", "test")
	if err == nil {
		t.Error("expected error for nonexistent table")
	}
}

func TestMustInsert(t *testing.T) {
	database := setupCRUDTestDB(t)

	id := MustInsert(database, "INSERT INTO items (name) VALUES (?)", "must")
	if id != 1 {
		t.Errorf("expected id 1, got %d", id)
	}
}

func TestMustInsert_Panics(t *testing.T) {
	database := setupCRUDTestDB(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid insert")
		}
	}()

	MustInsert(database, "INSERT INTO nonexistent (name) VALUES (?)", "test")
}

func TestUpdate(t *testing.T) {
	database := setupCRUDTestDB(t)

	// Insert test data
	database.Exec(`INSERT INTO items (name) VALUES ('old')`)

	affected, err := Update(database, "UPDATE items SET name = ? WHERE name = ?", "new", "old")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}

	// Verify the update
	result := Query[crudItem](database, "SELECT id, name FROM items WHERE name = ?", "new")
	item := result.First()
	if item == nil {
		t.Fatal("updated row not found")
	}
}

func TestUpdate_NoRows(t *testing.T) {
	database := setupCRUDTestDB(t)

	affected, err := Update(database, "UPDATE items SET name = ? WHERE name = ?", "new", "nonexistent")
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if affected != 0 {
		t.Errorf("expected 0 rows affected, got %d", affected)
	}
}

func TestUpdate_Error(t *testing.T) {
	database := setupCRUDTestDB(t)

	_, err := Update(database, "UPDATE nonexistent SET name = ?", "test")
	if err == nil {
		t.Error("expected error for nonexistent table")
	}
}

func TestMustUpdate(t *testing.T) {
	database := setupCRUDTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('old')`)

	affected := MustUpdate(database, "UPDATE items SET name = ? WHERE name = ?", "new", "old")
	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}
}

func TestMustUpdate_Panics(t *testing.T) {
	database := setupCRUDTestDB(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid update")
		}
	}()

	MustUpdate(database, "UPDATE nonexistent SET name = ?", "test")
}

func TestDelete(t *testing.T) {
	database := setupCRUDTestDB(t)

	// Insert test data
	database.Exec(`INSERT INTO items (name) VALUES ('todelete'), ('tokeep')`)

	affected, err := Delete(database, "DELETE FROM items WHERE name = ?", "todelete")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}

	// Verify the delete
	result := Query[crudItem](database, "SELECT id, name FROM items WHERE name = ?", "todelete")
	item := result.First()
	if item != nil {
		t.Error("deleted row still exists")
	}
}

func TestDelete_NoRows(t *testing.T) {
	database := setupCRUDTestDB(t)

	affected, err := Delete(database, "DELETE FROM items WHERE name = ?", "nonexistent")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if affected != 0 {
		t.Errorf("expected 0 rows affected, got %d", affected)
	}
}

func TestDelete_Error(t *testing.T) {
	database := setupCRUDTestDB(t)

	_, err := Delete(database, "DELETE FROM nonexistent WHERE name = ?", "test")
	if err == nil {
		t.Error("expected error for nonexistent table")
	}
}

func TestMustDelete(t *testing.T) {
	database := setupCRUDTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('todelete')`)

	affected := MustDelete(database, "DELETE FROM items WHERE name = ?", "todelete")
	if affected != 1 {
		t.Errorf("expected 1 row affected, got %d", affected)
	}
}

func TestMustDelete_Panics(t *testing.T) {
	database := setupCRUDTestDB(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid delete")
		}
	}()

	MustDelete(database, "DELETE FROM nonexistent WHERE name = ?", "test")
}

func TestDB_Select(t *testing.T) {
	database := setupCRUDTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('a'), ('b'), ('c')`)

	var items []crudItem
	err := database.Select(&items, "SELECT id, name FROM items ORDER BY name")
	if err != nil {
		t.Fatalf("Select failed: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("expected 3 items, got %d", len(items))
	}
}

func TestMustTx(t *testing.T) {
	database := setupCRUDTestDB(t)

	database.MustTx(func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO items (name) VALUES ('txtest')`)
		return err
	})

	// Verify data was committed
	result := Query[crudItem](database, "SELECT id, name FROM items WHERE name = ?", "txtest")
	item := result.First()
	if item == nil {
		t.Error("expected row from committed transaction")
	}
}

func TestMustTx_Panics(t *testing.T) {
	database := setupCRUDTestDB(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for failed transaction")
		}
	}()

	database.MustTx(func(tx *Tx) error {
		return errors.New("intentional error")
	})
}

func TestTx_Rollback(t *testing.T) {
	database := setupCRUDTestDB(t)

	err := database.Tx(func(tx *Tx) error {
		tx.Exec(`INSERT INTO items (name) VALUES ('rollback')`)
		return errors.New("trigger rollback")
	})
	if err == nil {
		t.Error("expected error from transaction")
	}

	// Verify data was rolled back
	result := Query[crudItem](database, "SELECT id, name FROM items WHERE name = ?", "rollback")
	item := result.First()
	if item != nil {
		t.Error("expected no row after rollback")
	}
}

func TestTx_PanicRollback(t *testing.T) {
	database := setupCRUDTestDB(t)

	defer func() {
		recover() // Catch the panic
	}()

	database.Tx(func(tx *Tx) error {
		tx.Exec(`INSERT INTO items (name) VALUES ('panicrollback')`)
		panic("intentional panic")
	})

	// Verify data was rolled back
	result := Query[crudItem](database, "SELECT id, name FROM items WHERE name = ?", "panicrollback")
	item := result.First()
	if item != nil {
		t.Error("expected no row after panic rollback")
	}
}

func TestTx_Select(t *testing.T) {
	database := setupCRUDTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('x'), ('y')`)

	err := database.Tx(func(tx *Tx) error {
		var items []crudItem
		err := tx.Select(&items, "SELECT id, name FROM items ORDER BY name")
		if err != nil {
			return err
		}
		if len(items) != 2 {
			t.Errorf("expected 2 items in tx, got %d", len(items))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

func TestRegisteredQueries(t *testing.T) {
	database := setupCRUDTestDB(t)

	// Register a query
	_, err := Register[crudItem](database, "SELECT id, name FROM items")
	if err != nil {
		t.Fatalf("failed to register query: %v", err)
	}

	queries := database.RegisteredQueries()
	if len(queries) != 1 {
		t.Errorf("expected 1 registered query, got %d", len(queries))
	}

	if queries[0].Type != "select" {
		t.Errorf("expected type 'select', got %q", queries[0].Type)
	}
}

func TestMustVerifyAll(t *testing.T) {
	database := setupCRUDTestDB(t)

	// Register a valid query
	_, err := Register[crudItem](database, "SELECT id, name FROM items")
	if err != nil {
		t.Fatalf("failed to register query: %v", err)
	}

	// Should not panic with valid queries
	database.MustVerifyAll()
}
