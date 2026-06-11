package auth

import (
	"context"
	"strings"
	"time"
)

// Token prefixes for different token types.
const (
	TokenPrefixSession   = "sess_"
	TokenPrefixAPIKey    = "key_"
	TokenPrefixMagicLink = "ml_"
)

// MagicLink represents a magic link token for passwordless login.
type MagicLink struct {
	ID        int    `db:"id"`
	Token     string `db:"token"`
	Email     string `db:"email"`
	CreatedAt int    `db:"created_at"`
	ExpiresAt int    `db:"expires_at"`
	UsedAt    int    `db:"used_at"`
	ForUser   int    `db:"for_user"`
}

// IsExpired returns true if the magic link has expired.
func (m *MagicLink) IsExpired() bool {
	return time.Now().Unix() > int64(m.ExpiresAt)
}

// IsUsed returns true if the magic link has already been used.
func (m *MagicLink) IsUsed() bool {
	return m.UsedAt > 0
}

// MagicLinkEmailSender sends magic link emails. Implement this interface
// to integrate with your email provider (Resend, SendGrid, AWS SES, etc.).
type MagicLinkEmailSender interface {
	// SendMagicLink sends a magic link email to the specified address.
	// verifyURL is the full URL the user should click to verify.
	// expiresInMinutes indicates how long the link is valid.
	SendMagicLink(ctx context.Context, email, verifyURL string, expiresInMinutes int) error
}

// User represents an authenticated user.
type User struct {
	ID           int    `db:"id" json:"id"`
	Email        string `db:"email" json:"email"`
	PasswordHash string `db:"password_hash" json:"-"` // Never expose in JSON
	IsAdmin      int    `db:"is_admin" json:"is_admin"`
	IsActive     int    `db:"is_active" json:"is_active"`
}

// HasPassword returns true if the user has a password set.
func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}

// IsAdminUser returns true if the user has admin privileges.
func (u *User) IsAdminUser() bool {
	return u.IsAdmin == 1
}

// IsActiveUser returns true if the user account is active.
func (u *User) IsActiveUser() bool {
	return u.IsActive == 1
}

// Session represents an authentication session or API key.
type Session struct {
	ID        int    `db:"id" json:"id"`
	KeyValue  string `db:"key_value" json:"-"` // Never expose in JSON
	KeyType   string `db:"key_type" json:"key_type"`
	Scopes    string `db:"scopes" json:"scopes"`
	CreatedAt int    `db:"created_at" json:"created_at"`
	ExpiresAt int    `db:"expires_at" json:"expires_at"`
	IsActive  int    `db:"is_active" json:"is_active"`
	ForUser   int    `db:"for_user" json:"for_user"`
}

// IsExpired returns true if the session has expired.
// A session with expires_at = 0 never expires.
func (s *Session) IsExpired() bool {
	if s.ExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() > int64(s.ExpiresAt)
}

// HasScope returns true if the session has the specified scope.
// Scopes are stored as comma-separated values.
func (s *Session) HasScope(scope string) bool {
	if s.Scopes == "" {
		return false
	}
	for _, sc := range strings.Split(s.Scopes, ",") {
		if strings.TrimSpace(sc) == scope {
			return true
		}
	}
	return false
}

// HasAnyScope returns true if the session has any of the specified scopes.
func (s *Session) HasAnyScope(scopes ...string) bool {
	for _, scope := range scopes {
		if s.HasScope(scope) {
			return true
		}
	}
	return false
}

// HasAllScopes returns true if the session has all of the specified scopes.
func (s *Session) HasAllScopes(scopes ...string) bool {
	for _, scope := range scopes {
		if !s.HasScope(scope) {
			return false
		}
	}
	return true
}

// GetScopes returns the scopes as a slice.
func (s *Session) GetScopes() []string {
	if s.Scopes == "" {
		return nil
	}
	parts := strings.Split(s.Scopes, ",")
	scopes := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			scopes = append(scopes, trimmed)
		}
	}
	return scopes
}

// IsSessionToken returns true if this is a session token (not an API key).
func (s *Session) IsSessionToken() bool {
	return s.KeyType == "session"
}

// IsAPIKey returns true if this is an API key.
func (s *Session) IsAPIKey() bool {
	return s.KeyType == "api_key"
}

// SessionWithUser combines session and user data for efficient auth lookups.
// This type is populated by a JOIN query to avoid N+1 queries during authentication.
type SessionWithUser struct {
	// Session fields
	SessionID        int    `db:"session_id"`
	SessionKeyValue  string `db:"session_key_value"`
	SessionKeyType   string `db:"session_key_type"`
	SessionScopes    string `db:"session_scopes"`
	SessionCreatedAt int    `db:"session_created_at"`
	SessionExpiresAt int    `db:"session_expires_at"`
	SessionIsActive  int    `db:"session_is_active"`
	SessionForUser   int    `db:"session_for_user"`
	// User fields
	UserID           int    `db:"user_id"`
	UserEmail        string `db:"user_email"`
	UserPasswordHash string `db:"user_password_hash"`
	UserIsAdmin      int    `db:"user_is_admin"`
	UserIsActive     int    `db:"user_is_active"`
}

// Session returns a Session struct from the combined data.
func (su *SessionWithUser) Session() *Session {
	return &Session{
		ID:        su.SessionID,
		KeyValue:  su.SessionKeyValue,
		KeyType:   su.SessionKeyType,
		Scopes:    su.SessionScopes,
		CreatedAt: su.SessionCreatedAt,
		ExpiresAt: su.SessionExpiresAt,
		IsActive:  su.SessionIsActive,
		ForUser:   su.SessionForUser,
	}
}

// User returns a User struct from the combined data.
func (su *SessionWithUser) User() *User {
	return &User{
		ID:           su.UserID,
		Email:        su.UserEmail,
		PasswordHash: su.UserPasswordHash,
		IsAdmin:      su.UserIsAdmin,
		IsActive:     su.UserIsActive,
	}
}

// CookieConfig holds configuration for session cookies.
type CookieConfig struct {
	Name     string
	Path     string
	Domain   string
	Secure   bool // Should be true in production (HTTPS)
	HttpOnly bool
	SameSite string // "Strict", "Lax", or "None"
	MaxAge   int    // In seconds, 0 means session cookie
}

// DefaultCookieConfig returns sensible defaults for session cookies.
// Set Secure=true in production.
func DefaultCookieConfig() CookieConfig {
	return CookieConfig{
		Name:     "session_key",
		Path:     "/",
		Secure:   false, // Set to true in production
		HttpOnly: true,
		SameSite: "Lax",
		MaxAge:   30 * 24 * 60 * 60, // 30 days
	}
}
