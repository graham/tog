package db

import (
	"context"
	"testing"
	"time"
)

type testItem struct {
	ID   int    `db:"id"`
	Name string `db:"name"`
}

func setupTestDB(t *testing.T) *DB {
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

func TestResult_First(t *testing.T) {
	database := setupTestDB(t)

	// Insert test data
	database.Exec(`INSERT INTO items (name) VALUES ('first'), ('second')`)

	t.Run("returns first row", func(t *testing.T) {
		result := Query[testItem](database, "SELECT id, name FROM items ORDER BY id LIMIT 1")
		item := result.First()
		if item == nil {
			t.Fatal("expected non-nil result")
		}
		if item.Name != "first" {
			t.Errorf("Name = %q, want %q", item.Name, "first")
		}
	})

	t.Run("returns nil for no rows", func(t *testing.T) {
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "nonexistent")
		item := result.First()
		if item != nil {
			t.Errorf("expected nil for no rows, got %+v", item)
		}
	})
}

func TestResult_FirstE(t *testing.T) {
	database := setupTestDB(t)

	database.Exec(`INSERT INTO items (name) VALUES ('test')`)

	t.Run("returns row and nil error", func(t *testing.T) {
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "test")
		item, err := result.FirstE()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.Name != "test" {
			t.Errorf("Name = %q, want %q", item.Name, "test")
		}
	})

	t.Run("returns error for no rows", func(t *testing.T) {
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "nonexistent")
		_, err := result.FirstE()
		if err == nil {
			t.Error("expected error for no rows")
		}
	})
}

func TestResult_Unique(t *testing.T) {
	database := setupTestDB(t)

	t.Run("returns nil for zero rows", func(t *testing.T) {
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "nonexistent")
		item := result.Unique()
		if item != nil {
			t.Errorf("expected nil for zero rows, got %+v", item)
		}
	})

	t.Run("returns item for exactly one row", func(t *testing.T) {
		database.Exec(`INSERT INTO items (name) VALUES ('unique')`)
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "unique")
		item := result.Unique()
		if item == nil {
			t.Fatal("expected non-nil result")
		}
		if item.Name != "unique" {
			t.Errorf("Name = %q, want %q", item.Name, "unique")
		}
	})

	t.Run("panics for multiple rows", func(t *testing.T) {
		database.Exec(`INSERT INTO items (name) VALUES ('dup'), ('dup')`)

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for multiple rows")
			}
		}()

		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "dup")
		result.Unique()
	})
}

func TestResult_UniqueE(t *testing.T) {
	database := setupTestDB(t)

	t.Run("returns error for zero rows", func(t *testing.T) {
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "nonexistent")
		_, err := result.UniqueE()
		if err == nil {
			t.Error("expected error for zero rows")
		}
	})

	t.Run("returns item for exactly one row", func(t *testing.T) {
		database.Exec(`INSERT INTO items (name) VALUES ('unique2')`)
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "unique2")
		item, err := result.UniqueE()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if item.Name != "unique2" {
			t.Errorf("Name = %q, want %q", item.Name, "unique2")
		}
	})

	t.Run("returns error for multiple rows", func(t *testing.T) {
		database.Exec(`INSERT INTO items (name) VALUES ('dup2'), ('dup2')`)
		result := Query[testItem](database, "SELECT id, name FROM items WHERE name = ?", "dup2")
		_, err := result.UniqueE()
		if err == nil {
			t.Error("expected error for multiple rows")
		}
	})
}

func TestResult_All(t *testing.T) {
	database := setupTestDB(t)

	database.Exec(`INSERT INTO items (name) VALUES ('a'), ('b'), ('c')`)

	result := Query[testItem](database, "SELECT id, name FROM items ORDER BY name")
	items, err := result.All()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 3 {
		t.Errorf("len(items) = %d, want 3", len(items))
	}

	names := []string{items[0].Name, items[1].Name, items[2].Name}
	expected := []string{"a", "b", "c"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("items[%d].Name = %q, want %q", i, name, expected[i])
		}
	}
}

func TestResult_Rows(t *testing.T) {
	database := setupTestDB(t)

	database.Exec(`INSERT INTO items (name) VALUES ('x'), ('y'), ('z')`)

	result := Query[testItem](database, "SELECT id, name FROM items ORDER BY name")

	var names []string
	for item, err := range result.Rows() {
		if err != nil {
			t.Fatalf("unexpected error during iteration: %v", err)
		}
		names = append(names, item.Name)
	}

	if len(names) != 3 {
		t.Errorf("len(names) = %d, want 3", len(names))
	}

	expected := []string{"x", "y", "z"}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestResult_WithQueryHook(t *testing.T) {
	database := setupTestDB(t)
	database.Exec(`INSERT INTO items (name) VALUES ('hooked')`)

	var hookCalled bool
	var capturedDuration time.Duration

	hook := func(name, sql string, duration time.Duration) {
		hookCalled = true
		capturedDuration = duration
	}

	ctx := ContextWithQueryHook(context.Background(), hook)

	// Create a result with context
	r := &Result[testItem]{
		q:     database,
		query: "SELECT id, name FROM items WHERE name = ?",
		args:  []any{"hooked"},
		ctx:   ctx,
	}

	_, err := r.FirstE()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !hookCalled {
		t.Error("query hook was not called")
	}
	if capturedDuration <= 0 {
		t.Error("duration should be positive")
	}
}
