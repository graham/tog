package db

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// DefaultConfigEnvVar is the environment variable name for the database config file path.
const DefaultConfigEnvVar = "DATABASE_CONFIG"

// DefaultConfigFile is the default config file name if not specified.
const DefaultConfigFile = "databases.json"

// DatabaseConfig holds configuration for a single database connection.
type DatabaseConfig struct {
	Driver   string         `json:"driver"`
	DSN      string         `json:"dsn"`
	ReadOnly bool           `json:"read_only,omitempty"`
	Pool     JSONPoolConfig `json:"pool"`
}

// JSONPoolConfig holds connection pool settings for JSON configuration.
type JSONPoolConfig struct {
	MaxOpen            int `json:"max_open"`
	MaxIdle            int `json:"max_idle"`
	MaxLifetimeSeconds int `json:"max_lifetime_seconds"`
}

// Config holds the full database configuration.
type Config struct {
	Databases map[string]DatabaseConfig `json:"databases"`
	Default   string                    `json:"default"`
}

// Manager holds multiple named database connections.
type Manager struct {
	databases map[string]*DB
	config    Config
}

// LoadConfig loads database configuration from a JSON file.
// Environment variables in DSN values are expanded (e.g., ${GOOSE_DBSTRING}).
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Expand environment variables in DSN values
	for name, dbConfig := range config.Databases {
		dbConfig.DSN = expandEnvVars(dbConfig.DSN)
		config.Databases[name] = dbConfig
	}

	return &config, nil
}

// expandEnvVars expands ${VAR} patterns in a string.
func expandEnvVars(s string) string {
	return os.Expand(s, func(key string) string {
		return os.Getenv(key)
	})
}

// NewManager creates a new database manager from configuration.
// It opens connections to all configured databases.
func NewManager(config *Config) (*Manager, error) {
	m := &Manager{
		databases: make(map[string]*DB),
		config:    *config,
	}

	for name, dbConfig := range config.Databases {
		poolConfig := PoolConfig{
			MaxOpenConns:    dbConfig.Pool.MaxOpen,
			MaxIdleConns:    dbConfig.Pool.MaxIdle,
			ConnMaxLifetime: time.Duration(dbConfig.Pool.MaxLifetimeSeconds) * time.Second,
		}

		// Apply defaults if not set
		if poolConfig.MaxOpenConns == 0 {
			poolConfig.MaxOpenConns = 25
		}
		if poolConfig.MaxIdleConns == 0 {
			poolConfig.MaxIdleConns = 5
		}
		if poolConfig.ConnMaxLifetime == 0 {
			poolConfig.ConnMaxLifetime = 5 * time.Minute
		}

		db, err := Open(dbConfig.Driver, dbConfig.DSN, poolConfig)
		if err != nil {
			// Close any already opened connections
			m.Close()
			return nil, fmt.Errorf("failed to open database %q: %w", name, err)
		}

		m.databases[name] = db
	}

	return m, nil
}

// NewManagerFromFile loads config from a JSON file and creates a manager.
func NewManagerFromFile(path string) (*Manager, error) {
	config, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return NewManager(config)
}

// Get returns a database connection by name.
// Returns nil if the database doesn't exist.
func (m *Manager) Get(name string) *DB {
	return m.databases[name]
}

// Default returns the default database connection.
func (m *Manager) Default() *DB {
	return m.databases[m.config.Default]
}

// Primary is an alias for Default (common naming convention).
func (m *Manager) Primary() *DB {
	return m.Default()
}

// Names returns all configured database names.
func (m *Manager) Names() []string {
	names := make([]string, 0, len(m.databases))
	for name := range m.databases {
		names = append(names, name)
	}
	return names
}

// IsReadOnly returns whether a database is configured as read-only.
func (m *Manager) IsReadOnly(name string) bool {
	if config, ok := m.config.Databases[name]; ok {
		return config.ReadOnly
	}
	return false
}

// Close closes all database connections.
func (m *Manager) Close() error {
	var errs []string
	for name, db := range m.databases {
		if err := db.Close(); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("errors closing databases: %s", strings.Join(errs, "; "))
	}
	return nil
}

// VerifyAll verifies all prepared queries across all databases.
func (m *Manager) VerifyAll() error {
	for name, db := range m.databases {
		if err := db.VerifyAll(); err != nil {
			return fmt.Errorf("database %q: %w", name, err)
		}
	}
	return nil
}

// GetConfigPath returns the database config file path.
// Priority: 1) explicit path if non-empty, 2) DATABASE_CONFIG env var, 3) "databases.json"
func GetConfigPath(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if envPath := os.Getenv(DefaultConfigEnvVar); envPath != "" {
		return envPath
	}
	return DefaultConfigFile
}

// NewManagerFromEnvOrFile creates a database manager using the following priority:
// 1. Config file at explicit path (if provided)
// 2. Config file at DATABASE_CONFIG env var path
// 3. Config file at "databases.json"
// 4. Fallback to GOOSE_DBSTRING/GOOSE_DRIVER env vars for single database
//
// This is the recommended way to create a Manager in application code.
func NewManagerFromEnvOrFile(explicitPath string) (*Manager, error) {
	configPath := GetConfigPath(explicitPath)

	// Try to load from config file
	if _, err := os.Stat(configPath); err == nil {
		return NewManagerFromFile(configPath)
	}

	// Fallback to environment variables for backwards compatibility
	return newManagerFromEnv()
}

// newManagerFromEnv creates a database manager from environment variables.
// Used as fallback when no config file is present.
func newManagerFromEnv() (*Manager, error) {
	dsn := os.Getenv("GOOSE_DBSTRING")
	if dsn == "" {
		dsn = "db.sqlite3"
	}
	driver := os.Getenv("GOOSE_DRIVER")
	if driver == "" {
		driver = "sqlite3"
	}

	config := &Config{
		Databases: map[string]DatabaseConfig{
			"primary": {
				Driver: driver,
				DSN:    dsn,
				Pool: JSONPoolConfig{
					MaxOpen:            25,
					MaxIdle:            5,
					MaxLifetimeSeconds: 300,
				},
			},
		},
		Default: "primary",
	}

	return NewManager(config)
}
