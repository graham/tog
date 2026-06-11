package app

import (
	"os"
	"testing"
)

func TestDefaultAppConfig(t *testing.T) {
	cfg := DefaultAppConfig()

	// Auth.Password
	if !cfg.Auth.Password.Enabled {
		t.Error("Auth.Password.Enabled should be true by default")
	}

	// Auth.MagicLink
	if cfg.Auth.MagicLink.Enabled {
		t.Error("Auth.MagicLink.Enabled should be false by default")
	}
	if cfg.Auth.MagicLink.TokenLifetime != 15 {
		t.Errorf("Auth.MagicLink.TokenLifetime = %d, want 15", cfg.Auth.MagicLink.TokenLifetime)
	}
	if cfg.Auth.MagicLink.RateLimitSecs != 30 {
		t.Errorf("Auth.MagicLink.RateLimitSecs = %d, want 30", cfg.Auth.MagicLink.RateLimitSecs)
	}

	// Auth.OAuth
	if cfg.Auth.OAuth.SuccessURL != "/" {
		t.Errorf("Auth.OAuth.SuccessURL = %q, want %q", cfg.Auth.OAuth.SuccessURL, "/")
	}
	if cfg.Auth.OAuth.FailureURL != "/login?error=oauth_failed" {
		t.Errorf("Auth.OAuth.FailureURL = %q, want %q", cfg.Auth.OAuth.FailureURL, "/login?error=oauth_failed")
	}
	if cfg.Auth.OAuth.Google.Enabled {
		t.Error("Auth.OAuth.Google.Enabled should be false by default")
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set a test environment variable
	os.Setenv("TEST_VAR_FOR_CONFIG", "test_value")
	defer os.Unsetenv("TEST_VAR_FOR_CONFIG")

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no vars", "plain string", "plain string"},
		{"single var", "${TEST_VAR_FOR_CONFIG}", "test_value"},
		{"var in string", "prefix_${TEST_VAR_FOR_CONFIG}_suffix", "prefix_test_value_suffix"},
		{"undefined var", "${UNDEFINED_VAR_12345}", ""},
		{"empty string", "", ""},
		{"multiple vars", "${TEST_VAR_FOR_CONFIG}/${TEST_VAR_FOR_CONFIG}", "test_value/test_value"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := expandEnvVars(tt.input); got != tt.want {
				t.Errorf("expandEnvVars(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	// Create config with zero values
	cfg := &AppConfig{}

	applyDefaults(cfg)

	// Check that defaults were applied
	if cfg.Auth.MagicLink.TokenLifetime != 15 {
		t.Errorf("TokenLifetime = %d, want 15", cfg.Auth.MagicLink.TokenLifetime)
	}
	if cfg.Auth.MagicLink.RateLimitSecs != 30 {
		t.Errorf("RateLimitSecs = %d, want 30", cfg.Auth.MagicLink.RateLimitSecs)
	}
	if cfg.Auth.OAuth.SuccessURL != "/" {
		t.Errorf("SuccessURL = %q, want %q", cfg.Auth.OAuth.SuccessURL, "/")
	}
	if cfg.Auth.OAuth.FailureURL != "/login?error=oauth_failed" {
		t.Errorf("FailureURL = %q, want %q", cfg.Auth.OAuth.FailureURL, "/login?error=oauth_failed")
	}
}

func TestApplyDefaults_PreservesExistingValues(t *testing.T) {
	cfg := &AppConfig{
		Auth: AuthConfig{
			MagicLink: MagicLinkAuthConfig{
				TokenLifetime: 30,
				RateLimitSecs: 60,
			},
			OAuth: OAuthConfig{
				SuccessURL: "/custom",
				FailureURL: "/custom-error",
			},
		},
	}

	applyDefaults(cfg)

	// Existing values should be preserved
	if cfg.Auth.MagicLink.TokenLifetime != 30 {
		t.Errorf("TokenLifetime = %d, want 30 (preserved)", cfg.Auth.MagicLink.TokenLifetime)
	}
	if cfg.Auth.MagicLink.RateLimitSecs != 60 {
		t.Errorf("RateLimitSecs = %d, want 60 (preserved)", cfg.Auth.MagicLink.RateLimitSecs)
	}
	if cfg.Auth.OAuth.SuccessURL != "/custom" {
		t.Errorf("SuccessURL = %q, want %q (preserved)", cfg.Auth.OAuth.SuccessURL, "/custom")
	}
}

func TestLoadAppConfig_DefaultsWhenFileMissing(t *testing.T) {
	// Load from non-existent file
	cfg, err := LoadAppConfig("/nonexistent/path/to/config.json")
	if err != nil {
		t.Fatalf("LoadAppConfig error = %v, want nil for missing file", err)
	}

	// Should return defaults
	defaults := DefaultAppConfig()
	if cfg.Auth.Password.Enabled != defaults.Auth.Password.Enabled {
		t.Error("Should return default config when file is missing")
	}
}

func TestLoadAppConfig_ValidFile(t *testing.T) {
	// Create a temp config file
	content := `{
		"auth": {
			"password": {"enabled": false},
			"magic_link": {"enabled": true, "token_lifetime": 30},
			"oauth": {
				"success_url": "/dashboard",
				"google": {"enabled": true, "client_id": "test-id"}
			}
		},
		"email": {
			"from_address": "noreply@test.com",
			"from_name": "Test"
		}
	}`

	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	cfg, err := LoadAppConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadAppConfig error = %v", err)
	}

	if cfg.Auth.Password.Enabled {
		t.Error("Password.Enabled should be false")
	}
	if !cfg.Auth.MagicLink.Enabled {
		t.Error("MagicLink.Enabled should be true")
	}
	if cfg.Auth.MagicLink.TokenLifetime != 30 {
		t.Errorf("TokenLifetime = %d, want 30", cfg.Auth.MagicLink.TokenLifetime)
	}
	if cfg.Auth.OAuth.SuccessURL != "/dashboard" {
		t.Errorf("SuccessURL = %q, want /dashboard", cfg.Auth.OAuth.SuccessURL)
	}
	if !cfg.Auth.OAuth.Google.Enabled {
		t.Error("Google.Enabled should be true")
	}
	if cfg.Email.FromAddress != "noreply@test.com" {
		t.Errorf("FromAddress = %q, want noreply@test.com", cfg.Email.FromAddress)
	}
}

func TestLoadAppConfig_InvalidJSON(t *testing.T) {
	// Create a temp file with invalid JSON
	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString("{ invalid json }"); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	_, err = LoadAppConfig(tmpfile.Name())
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestLoadAppConfig_UsesEnvVar(t *testing.T) {
	// Create a temp config file
	content := `{"auth": {"password": {"enabled": false}}}`

	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	// Set the env var
	os.Setenv(DefaultAppConfigEnvVar, tmpfile.Name())
	defer os.Unsetenv(DefaultAppConfigEnvVar)

	// Load with empty path - should use env var
	cfg, err := LoadAppConfig("")
	if err != nil {
		t.Fatalf("LoadAppConfig error = %v", err)
	}

	if cfg.Auth.Password.Enabled {
		t.Error("Password.Enabled should be false (from file)")
	}
}

func TestLoadAppConfig_AppliesDefaults(t *testing.T) {
	// Create a config with minimal content - defaults should be applied
	content := `{"auth": {"magic_link": {"enabled": true}}}`

	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	cfg, err := LoadAppConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadAppConfig error = %v", err)
	}

	// These should have default values applied
	if cfg.Auth.MagicLink.TokenLifetime != 15 {
		t.Errorf("TokenLifetime = %d, want 15 (default)", cfg.Auth.MagicLink.TokenLifetime)
	}
	if cfg.Auth.MagicLink.RateLimitSecs != 30 {
		t.Errorf("RateLimitSecs = %d, want 30 (default)", cfg.Auth.MagicLink.RateLimitSecs)
	}
	if cfg.Auth.OAuth.SuccessURL != "/" {
		t.Errorf("SuccessURL = %q, want / (default)", cfg.Auth.OAuth.SuccessURL)
	}
}

func TestLoadAppConfig_ExpandsEnvVars(t *testing.T) {
	os.Setenv("TEST_CLIENT_ID_CFG", "expanded-client-id")
	os.Setenv("TEST_API_KEY_CFG", "expanded-api-key")
	defer os.Unsetenv("TEST_CLIENT_ID_CFG")
	defer os.Unsetenv("TEST_API_KEY_CFG")

	content := `{
		"auth": {
			"oauth": {
				"google": {"client_id": "${TEST_CLIENT_ID_CFG}"}
			}
		},
		"email": {
			"resend_api_key": "${TEST_API_KEY_CFG}"
		}
	}`

	tmpfile, err := os.CreateTemp("", "config-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name())

	if _, err := tmpfile.WriteString(content); err != nil {
		t.Fatal(err)
	}
	tmpfile.Close()

	cfg, err := LoadAppConfig(tmpfile.Name())
	if err != nil {
		t.Fatalf("LoadAppConfig error = %v", err)
	}

	if cfg.Auth.OAuth.Google.ClientID != "expanded-client-id" {
		t.Errorf("ClientID = %q, want expanded-client-id", cfg.Auth.OAuth.Google.ClientID)
	}
	if cfg.Email.ResendAPIKey != "expanded-api-key" {
		t.Errorf("ResendAPIKey = %q, want expanded-api-key", cfg.Email.ResendAPIKey)
	}
}

func TestExpandAuthConfig(t *testing.T) {
	os.Setenv("TEST_CLIENT_ID", "my-client-id")
	os.Setenv("TEST_SECRET", "my-secret")
	defer os.Unsetenv("TEST_CLIENT_ID")
	defer os.Unsetenv("TEST_SECRET")

	auth := &AuthConfig{
		OAuth: OAuthConfig{
			Google: GoogleOAuthConfig{
				ClientID:     "${TEST_CLIENT_ID}",
				ClientSecret: "${TEST_SECRET}",
				RedirectURL:  "http://localhost:8080/callback",
			},
			SuccessURL: "/success",
			FailureURL: "/failure",
		},
		MagicLink: MagicLinkAuthConfig{
			BaseURL: "${TEST_BASE_URL}",
		},
	}

	expandAuthConfig(auth)

	if auth.OAuth.Google.ClientID != "my-client-id" {
		t.Errorf("ClientID = %q, want %q", auth.OAuth.Google.ClientID, "my-client-id")
	}
	if auth.OAuth.Google.ClientSecret != "my-secret" {
		t.Errorf("ClientSecret = %q, want %q", auth.OAuth.Google.ClientSecret, "my-secret")
	}
	// RedirectURL should remain unchanged (no env var)
	if auth.OAuth.Google.RedirectURL != "http://localhost:8080/callback" {
		t.Errorf("RedirectURL = %q, want unchanged", auth.OAuth.Google.RedirectURL)
	}
	// Undefined env var should expand to empty
	if auth.MagicLink.BaseURL != "" {
		t.Errorf("BaseURL = %q, want empty (undefined env var)", auth.MagicLink.BaseURL)
	}
}

func TestExpandEmailConfig(t *testing.T) {
	os.Setenv("TEST_API_KEY", "re_123abc")
	defer os.Unsetenv("TEST_API_KEY")

	email := &EmailConfig{
		ResendAPIKey: "${TEST_API_KEY}",
		FromAddress:  "noreply@example.com",
		FromName:     "Test App",
	}

	expandEmailConfig(email)

	if email.ResendAPIKey != "re_123abc" {
		t.Errorf("ResendAPIKey = %q, want %q", email.ResendAPIKey, "re_123abc")
	}
	// Non-env-var values should remain unchanged
	if email.FromAddress != "noreply@example.com" {
		t.Errorf("FromAddress = %q, want unchanged", email.FromAddress)
	}
}
