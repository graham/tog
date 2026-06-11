package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/graham/tog/db"
)

func TestContext_SetGet(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}

	ctx.Set("key1", "value1")
	ctx.Set("key2", 42)

	t.Run("get existing key", func(t *testing.T) {
		v, ok := ctx.Get("key1")
		if !ok {
			t.Error("expected key1 to exist")
		}
		if v != "value1" {
			t.Errorf("Get(key1) = %v, want %v", v, "value1")
		}
	})

	t.Run("get non-existing key", func(t *testing.T) {
		v, ok := ctx.Get("nonexistent")
		if ok {
			t.Error("expected nonexistent key to not exist")
		}
		if v != nil {
			t.Errorf("Get(nonexistent) = %v, want nil", v)
		}
	})

	t.Run("set on nil values initializes map", func(t *testing.T) {
		ctx2 := &Context{}
		ctx2.Set("key", "value")
		v, ok := ctx2.Get("key")
		if !ok || v != "value" {
			t.Error("Set should initialize values map")
		}
	})
}

func TestContext_MustGet(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}
	ctx.Set("exists", "value")

	t.Run("existing key", func(t *testing.T) {
		v := ctx.MustGet("exists")
		if v != "value" {
			t.Errorf("MustGet(exists) = %v, want %v", v, "value")
		}
	})

	t.Run("non-existing key panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for non-existing key")
			}
		}()
		ctx.MustGet("nonexistent")
	})
}

func TestGetTyped(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}
	ctx.Set("string", "hello")
	ctx.Set("int", 42)

	t.Run("correct type", func(t *testing.T) {
		v, ok := GetTyped[string](ctx, "string")
		if !ok {
			t.Error("expected to find string key")
		}
		if v != "hello" {
			t.Errorf("GetTyped = %v, want %v", v, "hello")
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		v, ok := GetTyped[string](ctx, "int")
		if ok {
			t.Error("expected false for wrong type")
		}
		if v != "" {
			t.Errorf("GetTyped should return zero value, got %v", v)
		}
	})

	t.Run("non-existing key", func(t *testing.T) {
		v, ok := GetTyped[string](ctx, "nonexistent")
		if ok {
			t.Error("expected false for non-existing key")
		}
		if v != "" {
			t.Errorf("GetTyped should return zero value, got %v", v)
		}
	})
}

func TestMustGetTyped(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}
	ctx.Set("string", "hello")
	ctx.Set("int", 42)

	t.Run("correct type", func(t *testing.T) {
		v := MustGetTyped[string](ctx, "string")
		if v != "hello" {
			t.Errorf("MustGetTyped = %v, want %v", v, "hello")
		}
	})

	t.Run("wrong type panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for wrong type")
			}
		}()
		MustGetTyped[string](ctx, "int")
	})

	t.Run("non-existing key panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for non-existing key")
			}
		}()
		MustGetTyped[string](ctx, "nonexistent")
	})
}

func TestGetContext(t *testing.T) {
	t.Run("returns existing context", func(t *testing.T) {
		wc := &Context{values: map[string]any{"test": "value"}}
		ctx := context.WithValue(context.Background(), ctxKey, wc)

		result := GetContext(ctx)
		if result != wc {
			t.Error("expected same Context instance")
		}
	})

	t.Run("creates new context if not exists", func(t *testing.T) {
		ctx := context.Background()
		result := GetContext(ctx)

		if result == nil {
			t.Error("expected non-nil Context")
		}
		if result.values == nil {
			t.Error("expected values map to be initialized")
		}
	})
}

func TestWithContext(t *testing.T) {
	wc := &Context{values: map[string]any{"key": "value"}}
	ctx := WithContext(context.Background(), wc)

	result := ctx.Value(ctxKey)
	if result != wc {
		t.Error("WithContext should store Context")
	}
}

func TestContextMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wc := GetContext(r.Context())
		if wc == nil {
			t.Error("expected Context to be set by middleware")
		}
		if wc.values == nil {
			t.Error("expected values to be initialized")
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware := ContextMiddleware(nil)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestSetCtxValue_GetCtxValue(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetCtxValue(r, "mykey", "myvalue")

		v, ok := GetCtxValue(r, "mykey")
		if !ok {
			t.Error("expected to find mykey")
		}
		if v != "myvalue" {
			t.Errorf("GetCtxValue = %v, want %v", v, "myvalue")
		}

		// Test non-existing key
		_, ok = GetCtxValue(r, "nonexistent")
		if ok {
			t.Error("expected false for non-existing key")
		}

		w.WriteHeader(http.StatusOK)
	})

	middleware := ContextMiddleware(nil)
	wrapped := middleware(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)
}

func TestContext_DB(t *testing.T) {
	t.Run("nil manager returns nil", func(t *testing.T) {
		ctx := &Context{}
		if ctx.DB() != nil {
			t.Error("expected nil when dbm is nil")
		}
	})

	t.Run("with manager returns default db", func(t *testing.T) {
		cfg := &db.Config{
			Databases: map[string]db.DatabaseConfig{
				"primary": {Driver: "sqlite3", DSN: ":memory:"},
			},
			Default: "primary",
		}
		mgr, err := db.NewManager(cfg)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		defer mgr.Close()

		ctx := &Context{dbm: mgr}
		if ctx.DB() == nil {
			t.Error("expected non-nil DB")
		}
	})
}

func TestContext_DBNamed(t *testing.T) {
	t.Run("nil manager returns nil", func(t *testing.T) {
		ctx := &Context{}
		if ctx.DBNamed("any") != nil {
			t.Error("expected nil when dbm is nil")
		}
	})

	t.Run("with manager returns named db", func(t *testing.T) {
		cfg := &db.Config{
			Databases: map[string]db.DatabaseConfig{
				"primary":   {Driver: "sqlite3", DSN: ":memory:"},
				"secondary": {Driver: "sqlite3", DSN: ":memory:"},
			},
			Default: "primary",
		}
		mgr, err := db.NewManager(cfg)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		defer mgr.Close()

		ctx := &Context{dbm: mgr}
		if ctx.DBNamed("secondary") == nil {
			t.Error("expected non-nil DB for secondary")
		}
		if ctx.DBNamed("nonexistent") != nil {
			t.Error("expected nil for nonexistent db")
		}
	})
}

func TestContext_DBManager(t *testing.T) {
	t.Run("nil manager", func(t *testing.T) {
		ctx := &Context{}
		if ctx.DBManager() != nil {
			t.Error("expected nil")
		}
	})

	t.Run("with manager", func(t *testing.T) {
		cfg := &db.Config{
			Databases: map[string]db.DatabaseConfig{
				"primary": {Driver: "sqlite3", DSN: ":memory:"},
			},
			Default: "primary",
		}
		mgr, err := db.NewManager(cfg)
		if err != nil {
			t.Fatalf("failed to create manager: %v", err)
		}
		defer mgr.Close()

		ctx := &Context{dbm: mgr}
		if ctx.DBManager() != mgr {
			t.Error("expected same manager")
		}
	})
}

func TestContext_Get_NilValues(t *testing.T) {
	ctx := &Context{} // values is nil

	v, ok := ctx.Get("anykey")
	if ok {
		t.Error("expected false for nil values")
	}
	if v != nil {
		t.Error("expected nil value")
	}
}
