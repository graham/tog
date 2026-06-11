package auth

import (
	"testing"
	"time"
)

func TestMagicLink_IsExpired(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name      string
		expiresAt int
		want      bool
	}{
		{"expired (past)", int(now - 3600), true},
		{"not expired (future)", int(now + 3600), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MagicLink{ExpiresAt: tt.expiresAt}
			if got := m.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_IsExpired(t *testing.T) {
	now := time.Now().Unix()

	tests := []struct {
		name      string
		expiresAt int
		want      bool
	}{
		{"never expires (0)", 0, false},
		{"expired (past)", int(now - 3600), true},
		{"not expired (future)", int(now + 3600), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{ExpiresAt: tt.expiresAt}
			if got := s.IsExpired(); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSession_HasScope(t *testing.T) {
	tests := []struct {
		name   string
		scopes string
		scope  string
		want   bool
	}{
		{"empty scopes", "", "read", false},
		{"single scope match", "read", "read", true},
		{"single scope no match", "read", "write", false},
		{"multiple scopes match", "read,write,delete", "write", true},
		{"multiple scopes no match", "read,write,delete", "admin", false},
		{"scopes with spaces", "read, write, delete", "write", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{Scopes: tt.scopes}
			if got := s.HasScope(tt.scope); got != tt.want {
				t.Errorf("HasScope(%q) = %v, want %v", tt.scope, got, tt.want)
			}
		})
	}
}

func TestSession_HasAnyScope(t *testing.T) {
	s := &Session{Scopes: "read,write"}

	if s.HasAnyScope("admin", "delete") {
		t.Error("HasAnyScope should return false when no scopes match")
	}
	if !s.HasAnyScope("read", "admin") {
		t.Error("HasAnyScope should return true when at least one scope matches")
	}
}

func TestSession_HasAllScopes(t *testing.T) {
	s := &Session{Scopes: "read,write,delete"}

	if !s.HasAllScopes("read", "write") {
		t.Error("HasAllScopes should return true when all requested scopes present")
	}
	if s.HasAllScopes("read", "admin") {
		t.Error("HasAllScopes should return false when any scope is missing")
	}
}

func TestSession_GetScopes(t *testing.T) {
	tests := []struct {
		scopes string
		want   int
	}{
		{"", 0},
		{"read", 1},
		{"read,write,delete", 3},
	}

	for _, tt := range tests {
		s := &Session{Scopes: tt.scopes}
		if got := len(s.GetScopes()); got != tt.want {
			t.Errorf("GetScopes() len = %d, want %d for scopes %q", got, tt.want, tt.scopes)
		}
	}
}

func TestDefaultCookieConfig(t *testing.T) {
	cfg := DefaultCookieConfig()

	if cfg.Name != "session_key" {
		t.Errorf("Name = %q, want %q", cfg.Name, "session_key")
	}
	if cfg.HttpOnly != true {
		t.Errorf("HttpOnly = %v, want true", cfg.HttpOnly)
	}
	if cfg.SameSite != "Lax" {
		t.Errorf("SameSite = %q, want %q", cfg.SameSite, "Lax")
	}
}
