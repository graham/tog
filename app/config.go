package app

import (
	"encoding/json"
	"fmt"
	"os"
)

// DefaultAppConfigEnvVar is the environment variable name for the app config file path.
const DefaultAppConfigEnvVar = "APP_CONFIG"

// DefaultAppConfigFile is the default config file name if not specified.
const DefaultAppConfigFile = "config.json"

// AppConfig holds the full application configuration.
type AppConfig struct {
	Auth  AuthConfig  `json:"auth"`
	Email EmailConfig `json:"email"`
}

// EmailConfig holds email sending configuration.
type EmailConfig struct {
	ResendAPIKey string `json:"resend_api_key"` // Resend API key (supports ${ENV_VAR})
	FromAddress  string `json:"from_address"`   // Sender email address
	FromName     string `json:"from_name"`      // Sender display name
}

// AuthConfig holds authentication configuration.
type AuthConfig struct {
	Password  PasswordAuthConfig  `json:"password"`
	MagicLink MagicLinkAuthConfig `json:"magic_link"`
	OAuth     OAuthConfig         `json:"oauth"`
}

// PasswordAuthConfig holds password authentication settings.
type PasswordAuthConfig struct {
	Enabled bool `json:"enabled"`
}

// MagicLinkAuthConfig holds magic link authentication settings.
type MagicLinkAuthConfig struct {
	Enabled       bool   `json:"enabled"`
	TokenLifetime int    `json:"token_lifetime"`  // minutes
	RateLimitSecs int    `json:"rate_limit_secs"` // seconds between requests per email (default 30)
	BaseURL       string `json:"base_url"`        // base URL for verification links (e.g., "https://app.example.com")
}

// OAuthConfig holds OAuth provider configurations.
type OAuthConfig struct {
	Google      GoogleOAuthConfig `json:"google"`
	SuccessURL  string            `json:"success_url"`  // redirect after successful OAuth login
	FailureURL  string            `json:"failure_url"`  // redirect after failed OAuth login
}

// GoogleOAuthConfig holds Google OAuth settings.
type GoogleOAuthConfig struct {
	Enabled      bool   `json:"enabled"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	RedirectURL  string `json:"redirect_url"`
}

// DefaultAppConfig returns configuration with sensible defaults.
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Auth: AuthConfig{
			Password: PasswordAuthConfig{
				Enabled: true,
			},
			MagicLink: MagicLinkAuthConfig{
				Enabled:       false,
				TokenLifetime: 15, // 15 minutes
				RateLimitSecs: 30, // 30 seconds between requests
			},
			OAuth: OAuthConfig{
				SuccessURL: "/",
				FailureURL: "/login?error=oauth_failed",
				Google: GoogleOAuthConfig{
					Enabled: false,
				},
			},
		},
	}
}

// LoadAppConfig loads application configuration from a JSON file.
// Environment variables in string values are expanded (e.g., ${GOOGLE_CLIENT_ID}).
// If the file doesn't exist, returns default configuration.
func LoadAppConfig(path string) (*AppConfig, error) {
	// If no path provided, try environment variable, then default
	if path == "" {
		path = os.Getenv(DefaultAppConfigEnvVar)
		if path == "" {
			path = DefaultAppConfigFile
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if config file doesn't exist
			return DefaultAppConfig(), nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config AppConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand environment variables in sensitive fields
	expandAuthConfig(&config.Auth)
	expandEmailConfig(&config.Email)

	// Apply defaults for zero values
	applyDefaults(&config)

	return &config, nil
}

// expandAuthConfig expands environment variables in auth configuration.
func expandAuthConfig(auth *AuthConfig) {
	auth.OAuth.Google.ClientID = expandEnvVars(auth.OAuth.Google.ClientID)
	auth.OAuth.Google.ClientSecret = expandEnvVars(auth.OAuth.Google.ClientSecret)
	auth.OAuth.Google.RedirectURL = expandEnvVars(auth.OAuth.Google.RedirectURL)
	auth.OAuth.SuccessURL = expandEnvVars(auth.OAuth.SuccessURL)
	auth.OAuth.FailureURL = expandEnvVars(auth.OAuth.FailureURL)
	auth.MagicLink.BaseURL = expandEnvVars(auth.MagicLink.BaseURL)
}

// expandEmailConfig expands environment variables in email configuration.
func expandEmailConfig(email *EmailConfig) {
	email.ResendAPIKey = expandEnvVars(email.ResendAPIKey)
	email.FromAddress = expandEnvVars(email.FromAddress)
	email.FromName = expandEnvVars(email.FromName)
}

// applyDefaults applies default values where config has zero values.
func applyDefaults(config *AppConfig) {
	defaults := DefaultAppConfig()

	if config.Auth.MagicLink.TokenLifetime == 0 {
		config.Auth.MagicLink.TokenLifetime = defaults.Auth.MagicLink.TokenLifetime
	}
	if config.Auth.MagicLink.RateLimitSecs == 0 {
		config.Auth.MagicLink.RateLimitSecs = defaults.Auth.MagicLink.RateLimitSecs
	}
	if config.Auth.OAuth.SuccessURL == "" {
		config.Auth.OAuth.SuccessURL = defaults.Auth.OAuth.SuccessURL
	}
	if config.Auth.OAuth.FailureURL == "" {
		config.Auth.OAuth.FailureURL = defaults.Auth.OAuth.FailureURL
	}
}

// expandEnvVars expands ${VAR} patterns in a string.
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})
}
