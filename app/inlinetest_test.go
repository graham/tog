package app

import (
	"strings"
	"testing"
)

func TestParseRequestLine(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		wantMethod string
		wantPath   string
		wantBody   string
	}{
		{
			name:       "GET request",
			line:       "GET /api/items",
			wantMethod: "GET",
			wantPath:   "/api/items",
			wantBody:   "",
		},
		{
			name:       "POST with body",
			line:       `POST /api/items {"name":"Widget"}`,
			wantMethod: "POST",
			wantPath:   "/api/items",
			wantBody:   `{"name":"Widget"}`,
		},
		{
			name:       "lowercase method",
			line:       "get /health",
			wantMethod: "GET",
			wantPath:   "/health",
			wantBody:   "",
		},
		{
			name:       "PUT with JSON body",
			line:       `PUT /api/items/1 {"name":"Updated","price":19.99}`,
			wantMethod: "PUT",
			wantPath:   "/api/items/1",
			wantBody:   `{"name":"Updated","price":19.99}`,
		},
		{
			name:       "DELETE request",
			line:       "DELETE /api/items/1",
			wantMethod: "DELETE",
			wantPath:   "/api/items/1",
			wantBody:   "",
		},
		{
			name:       "path only defaults to GET",
			line:       "/api/items",
			wantMethod: "GET",
			wantPath:   "/api/items",
			wantBody:   "",
		},
		{
			name:       "empty line defaults to GET",
			line:       "",
			wantMethod: "GET",
			wantPath:   "",
			wantBody:   "",
		},
		{
			name:       "body with spaces",
			line:       `POST /api/items {"name":"My Widget","description":"A cool thing"}`,
			wantMethod: "POST",
			wantPath:   "/api/items",
			wantBody:   `{"name":"My Widget","description":"A cool thing"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, path, body := parseRequestLine(tt.line)

			if method != tt.wantMethod {
				t.Errorf("method = %q, want %q", method, tt.wantMethod)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

func TestGenerateInlineToken(t *testing.T) {
	t.Run("session token", func(t *testing.T) {
		token := generateInlineToken("session")

		if !strings.HasPrefix(token, "sess_") {
			t.Errorf("session token should start with 'sess_', got %q", token)
		}

		// sess_ (5) + 64 hex chars = 69
		if len(token) != 69 {
			t.Errorf("token length = %d, want 69", len(token))
		}
	})

	t.Run("api_key token", func(t *testing.T) {
		token := generateInlineToken("api_key")

		if !strings.HasPrefix(token, "key_") {
			t.Errorf("api_key token should start with 'key_', got %q", token)
		}

		// key_ (4) + 64 hex chars = 68
		if len(token) != 68 {
			t.Errorf("token length = %d, want 68", len(token))
		}
	})

	t.Run("tokens are unique", func(t *testing.T) {
		token1 := generateInlineToken("session")
		token2 := generateInlineToken("session")

		if token1 == token2 {
			t.Error("tokens should be unique")
		}
	})

	t.Run("unknown type defaults to session prefix", func(t *testing.T) {
		token := generateInlineToken("unknown")

		if !strings.HasPrefix(token, "sess_") {
			t.Errorf("unknown type should default to 'sess_', got %q", token)
		}
	})
}

func TestExtractJSONPath(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name: "simple string field",
			json: `{"email":"test@example.com"}`,
			path: ".email",
			want: "test@example.com",
		},
		{
			name: "integer field",
			json: `{"id":42}`,
			path: ".id",
			want: "42",
		},
		{
			name: "float field",
			json: `{"price":19.99}`,
			path: ".price",
			want: "19.99",
		},
		{
			name: "boolean true",
			json: `{"authenticated":true}`,
			path: ".authenticated",
			want: "true",
		},
		{
			name: "boolean false",
			json: `{"is_admin":false}`,
			path: ".is_admin",
			want: "false",
		},
		{
			name: "nested field",
			json: `{"user":{"name":"John"}}`,
			path: ".user.name",
			want: "John",
		},
		{
			name: "array element",
			json: `{"items":[{"name":"First"},{"name":"Second"}]}`,
			path: ".items.0.name",
			want: "First",
		},
		{
			name: "null value",
			json: `{"value":null}`,
			path: ".value",
			want: "null",
		},
		{
			name:    "missing field",
			json:    `{"email":"test@example.com"}`,
			path:    ".missing",
			wantErr: true,
		},
		{
			name:    "invalid json",
			json:    `not json`,
			path:    ".field",
			wantErr: true,
		},
		{
			name:    "path without dot prefix",
			json:    `{"email":"test@example.com"}`,
			path:    "email",
			wantErr: true,
		},
		{
			name:    "array index out of bounds",
			json:    `{"items":[{"name":"First"}]}`,
			path:    ".items.5.name",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractJSONPath(tt.json, tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseJSONAssertArgs(t *testing.T) {
	tests := []struct {
		name      string
		args      string
		wantPath  string
		wantValue string
	}{
		{
			name:      "quoted value",
			args:      `.email "test@example.com"`,
			wantPath:  ".email",
			wantValue: "test@example.com",
		},
		{
			name:      "single quoted value",
			args:      `.email 'test@example.com'`,
			wantPath:  ".email",
			wantValue: "test@example.com",
		},
		{
			name:      "unquoted value",
			args:      ".authenticated true",
			wantPath:  ".authenticated",
			wantValue: "true",
		},
		{
			name:      "value with spaces in quotes",
			args:      `.name "John Doe"`,
			wantPath:  ".name",
			wantValue: "John Doe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := parseJSONAssertArgs(tt.args)

			if len(parts) < 2 {
				t.Fatalf("expected 2 parts, got %d", len(parts))
			}

			if parts[0] != tt.wantPath {
				t.Errorf("path = %q, want %q", parts[0], tt.wantPath)
			}
			if parts[1] != tt.wantValue {
				t.Errorf("value = %q, want %q", parts[1], tt.wantValue)
			}
		})
	}
}
