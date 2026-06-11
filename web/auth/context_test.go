package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/web"
)

func TestMustSessionFromContext(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("panicked"))
			}
		}()
		MustSessionFromContext(r.Context())
		w.Write([]byte("no panic"))
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Body.String() != "panicked" {
		t.Errorf("expected panic, got %s", rec.Body.String())
	}
}

func TestMustSessionFromContext_WithSession(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/session", func(w http.ResponseWriter, r *http.Request) {
		// Set a session in context
		session := &Session{ID: 1, KeyValue: "test-key"}
		setSessionInContext(r.Context(), session)

		// Should not panic
		s := MustSessionFromContext(r.Context())
		if s.KeyValue != "test-key" {
			t.Errorf("expected 'test-key', got %s", s.KeyValue)
		}
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/session", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

type testProject struct {
	ID   int
	Name string
}

func TestSetRelatedEntity(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/set", func(w http.ResponseWriter, r *http.Request) {
		project := &testProject{ID: 1, Name: "Test Project"}
		SetRelatedEntity(r.Context(), "project", project)

		// Retrieve it back
		p, ok := GetRelatedEntity[testProject](r.Context(), "project")
		if !ok {
			t.Error("expected to find project")
			return
		}
		if p.Name != "Test Project" {
			t.Errorf("expected 'Test Project', got %s", p.Name)
		}
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/set", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGetRelatedEntity_NotFound(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/notfound", func(w http.ResponseWriter, r *http.Request) {
		_, ok := GetRelatedEntity[testProject](r.Context(), "nonexistent")
		if ok {
			t.Error("expected not found")
			return
		}
		w.Write([]byte("not found"))
	})

	req := httptest.NewRequest("GET", "/notfound", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Body.String() != "not found" {
		t.Errorf("expected 'not found', got %s", rec.Body.String())
	}
}

func TestMustGetRelatedEntity(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/must", func(w http.ResponseWriter, r *http.Request) {
		project := &testProject{ID: 1, Name: "Must Project"}
		SetRelatedEntity(r.Context(), "project", project)

		// Should not panic
		p := MustGetRelatedEntity[testProject](r.Context(), "project")
		if p.Name != "Must Project" {
			t.Errorf("expected 'Must Project', got %s", p.Name)
		}
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/must", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestMustGetRelatedEntity_Panics(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/panic", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("panicked"))
			}
		}()
		MustGetRelatedEntity[testProject](r.Context(), "nonexistent")
		w.Write([]byte("no panic"))
	})

	req := httptest.NewRequest("GET", "/panic", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Body.String() != "panicked" {
		t.Errorf("expected panic, got %s", rec.Body.String())
	}
}

func TestSetUserInContext(t *testing.T) {
	r := chi.NewRouter()
	r.Use(web.ContextMiddleware(nil))
	r.Get("/user", func(w http.ResponseWriter, req *http.Request) {
		user := &User{ID: 1, Email: "test@example.com"}
		setUserInContext(req.Context(), user)

		u, ok := UserFromContext(req.Context())
		if !ok {
			t.Error("expected to find user")
			return
		}
		if u.Email != "test@example.com" {
			t.Errorf("expected 'test@example.com', got %s", u.Email)
		}
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest("GET", "/user", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestUserFromContext_NoWebContext(t *testing.T) {
	// Test with a plain context (no web context)
	ctx := context.Background()
	_, ok := UserFromContext(ctx)
	if ok {
		t.Error("expected not found with no web context")
	}
}

func TestSessionFromContext_NoWebContext(t *testing.T) {
	// Test with a plain context (no web context)
	ctx := context.Background()
	_, ok := SessionFromContext(ctx)
	if ok {
		t.Error("expected not found with no web context")
	}
}
