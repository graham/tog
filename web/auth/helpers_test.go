package auth

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseSameSite(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  http.SameSite
	}{
		{"Strict", "Strict", http.SameSiteStrictMode},
		{"None", "None", http.SameSiteNoneMode},
		{"Lax", "Lax", http.SameSiteLaxMode},
		{"empty defaults to Lax", "", http.SameSiteLaxMode},
		{"unknown defaults to Lax", "Unknown", http.SameSiteLaxMode},
		{"lowercase defaults to Lax", "strict", http.SameSiteLaxMode},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseSameSite(tt.input); got != tt.want {
				t.Errorf("parseSameSite(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateSessionKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
	}{
		{"session prefix", TokenPrefixSession},
		{"api key prefix", TokenPrefixAPIKey},
		{"empty prefix", ""},
		{"custom prefix", "custom_"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, err := generateSessionKey(tt.prefix)
			if err != nil {
				t.Fatalf("generateSessionKey(%q) error = %v", tt.prefix, err)
			}

			// Should start with prefix
			if !strings.HasPrefix(key, tt.prefix) {
				t.Errorf("key %q should start with prefix %q", key, tt.prefix)
			}

			// Should be prefix + 64 hex chars (32 bytes)
			expectedLen := len(tt.prefix) + 64
			if len(key) != expectedLen {
				t.Errorf("len(key) = %d, want %d", len(key), expectedLen)
			}

			// Generate another and ensure they're different (randomness check)
			key2, _ := generateSessionKey(tt.prefix)
			if key == key2 {
				t.Error("generateSessionKey should produce unique keys")
			}
		})
	}
}

func TestGenerateMagicLinkToken(t *testing.T) {
	token, err := generateMagicLinkToken()
	if err != nil {
		t.Fatalf("generateMagicLinkToken() error = %v", err)
	}

	// Should start with magic link prefix
	if !strings.HasPrefix(token, TokenPrefixMagicLink) {
		t.Errorf("token %q should start with %q", token, TokenPrefixMagicLink)
	}

	// Should be prefix + 64 hex chars (32 bytes)
	expectedLen := len(TokenPrefixMagicLink) + 64
	if len(token) != expectedLen {
		t.Errorf("len(token) = %d, want %d", len(token), expectedLen)
	}

	// Generate another and ensure they're different
	token2, _ := generateMagicLinkToken()
	if token == token2 {
		t.Error("generateMagicLinkToken should produce unique tokens")
	}
}

func TestGenerateOAuthState(t *testing.T) {
	state, err := generateOAuthState()
	if err != nil {
		t.Fatalf("generateOAuthState() error = %v", err)
	}

	// Should be 64 hex chars (32 bytes)
	if len(state) != 64 {
		t.Errorf("len(state) = %d, want 64", len(state))
	}

	// Should be valid hex
	for _, c := range state {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("state contains invalid hex char: %c", c)
			break
		}
	}

	// Generate another and ensure they're different
	state2, _ := generateOAuthState()
	if state == state2 {
		t.Error("generateOAuthState should produce unique states")
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		substr string
		want   bool
	}{
		{"found at start", "hello world", "hello", true},
		{"found at end", "hello world", "world", true},
		{"found in middle", "hello world", "lo wo", true},
		{"not found", "hello world", "xyz", false},
		{"empty substring", "hello", "", true},
		{"empty string", "", "hello", false},
		{"both empty", "", "", true},
		{"exact match", "hello", "hello", true},
		{"substring longer than string", "hi", "hello", false},
		{"single char found", "hello", "e", true},
		{"single char not found", "hello", "x", false},
		{"case sensitive", "Hello", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := contains(tt.s, tt.substr); got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, got, tt.want)
			}
		})
	}
}
