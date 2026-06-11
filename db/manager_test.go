package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManager_Primary(t *testing.T) {
	config := &Config{
		Databases: map[string]DatabaseConfig{
			"primary": {
				Driver: "sqlite3",
				DSN:    ":memory:",
				Pool:   JSONPoolConfig{MaxOpen: 1, MaxIdle: 1},
			},
		},
		Default: "primary",
	}

	mgr, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Primary() should be alias for Default()
	if mgr.Primary() != mgr.Default() {
		t.Error("Primary() should return same as Default()")
	}

	if mgr.Primary() == nil {
		t.Error("Primary() returned nil")
	}
}

func TestManager_IsReadOnly(t *testing.T) {
	config := &Config{
		Databases: map[string]DatabaseConfig{
			"primary": {
				Driver:   "sqlite3",
				DSN:      ":memory:",
				Pool:     JSONPoolConfig{MaxOpen: 1, MaxIdle: 1},
				ReadOnly: false,
			},
			"replica": {
				Driver:   "sqlite3",
				DSN:      ":memory:",
				Pool:     JSONPoolConfig{MaxOpen: 1, MaxIdle: 1},
				ReadOnly: true,
			},
		},
		Default: "primary",
	}

	mgr, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()

	if mgr.IsReadOnly("primary") {
		t.Error("primary should not be read-only")
	}

	if !mgr.IsReadOnly("replica") {
		t.Error("replica should be read-only")
	}

	// Non-existent database returns false
	if mgr.IsReadOnly("nonexistent") {
		t.Error("nonexistent database should return false")
	}
}

func TestManager_VerifyAll(t *testing.T) {
	config := &Config{
		Databases: map[string]DatabaseConfig{
			"primary": {
				Driver: "sqlite3",
				DSN:    ":memory:",
				Pool:   JSONPoolConfig{MaxOpen: 1, MaxIdle: 1},
			},
		},
		Default: "primary",
	}

	mgr, err := NewManager(config)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	defer mgr.Close()

	// Create table in the database
	db := mgr.Default()
	db.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`)

	// Register a valid query
	Register[struct {
		ID   int    `db:"id"`
		Name string `db:"name"`
	}](db, "SELECT id, name FROM items")

	// VerifyAll should succeed
	if err := mgr.VerifyAll(); err != nil {
		t.Errorf("VerifyAll failed: %v", err)
	}
}

func TestExpandEnvVars(t *testing.T) {
	// Set a test environment variable
	os.Setenv("TEST_DB_VAR", "test_value")
	defer os.Unsetenv("TEST_DB_VAR")

	result := expandEnvVars("prefix_${TEST_DB_VAR}_suffix")
	expected := "prefix_test_value_suffix"

	if result != expected {
		t.Errorf("expandEnvVars() = %q, want %q", result, expected)
	}

	// Test with non-existent var
	result = expandEnvVars("${NONEXISTENT_VAR}")
	if result != "" {
		t.Errorf("expected empty string for non-existent var, got %q", result)
	}
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "databases.json")

	configContent := `{
		"databases": {
			"primary": {
				"driver": "sqlite3",
				"dsn": ":memory:",
				"pool": {"max_open": 5, "max_idle": 2}
			}
		},
		"default": "primary"
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if config.Default != "primary" {
		t.Errorf("Default = %q, want %q", config.Default, "primary")
	}

	if dbConfig, ok := config.Databases["primary"]; !ok {
		t.Error("primary database not found")
	} else {
		if dbConfig.Driver != "sqlite3" {
			t.Errorf("Driver = %q, want %q", dbConfig.Driver, "sqlite3")
		}
		if dbConfig.Pool.MaxOpen != 5 {
			t.Errorf("MaxOpen = %d, want 5", dbConfig.Pool.MaxOpen)
		}
	}
}

func TestLoadConfig_NotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.json")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	if err := os.WriteFile(configPath, []byte("not valid json"), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewManagerFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "databases.json")

	configContent := `{
		"databases": {
			"primary": {
				"driver": "sqlite3",
				"dsn": ":memory:",
				"pool": {"max_open": 1, "max_idle": 1}
			}
		},
		"default": "primary"
	}`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	mgr, err := NewManagerFromFile(configPath)
	if err != nil {
		t.Fatalf("NewManagerFromFile failed: %v", err)
	}
	defer mgr.Close()

	if mgr.Default() == nil {
		t.Error("Default() returned nil")
	}
}

func TestGetConfigPath(t *testing.T) {
	// Test explicit path takes priority
	result := GetConfigPath("/explicit/path.json")
	if result != "/explicit/path.json" {
		t.Errorf("GetConfigPath with explicit path = %q, want %q", result, "/explicit/path.json")
	}

	// Test env var when no explicit path
	os.Setenv(DefaultConfigEnvVar, "/from/env.json")
	defer os.Unsetenv(DefaultConfigEnvVar)

	result = GetConfigPath("")
	if result != "/from/env.json" {
		t.Errorf("GetConfigPath with env = %q, want %q", result, "/from/env.json")
	}

	// Test default when no explicit path and no env
	os.Unsetenv(DefaultConfigEnvVar)
	result = GetConfigPath("")
	if result != DefaultConfigFile {
		t.Errorf("GetConfigPath default = %q, want %q", result, DefaultConfigFile)
	}
}

func TestNewManagerFromEnvOrFile_FallbackToEnv(t *testing.T) {
	// Use a non-existent config path so it falls back to env
	os.Setenv("GOOSE_DRIVER", "sqlite3")
	os.Setenv("GOOSE_DBSTRING", ":memory:")
	defer os.Unsetenv("GOOSE_DRIVER")
	defer os.Unsetenv("GOOSE_DBSTRING")

	mgr, err := NewManagerFromEnvOrFile("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("NewManagerFromEnvOrFile failed: %v", err)
	}
	defer mgr.Close()

	if mgr.Default() == nil {
		t.Error("Default() returned nil")
	}
}

func TestNewManagerFromEnv_Defaults(t *testing.T) {
	// Clear env vars to test defaults
	os.Unsetenv("GOOSE_DRIVER")
	os.Unsetenv("GOOSE_DBSTRING")

	// newManagerFromEnv will try to open db.sqlite3 which doesn't exist
	// but we can't easily test the default file creation without side effects
	// Instead test that the function handles environment variables correctly

	os.Setenv("GOOSE_DRIVER", "sqlite3")
	os.Setenv("GOOSE_DBSTRING", ":memory:")
	defer os.Unsetenv("GOOSE_DRIVER")
	defer os.Unsetenv("GOOSE_DBSTRING")

	mgr, err := newManagerFromEnv()
	if err != nil {
		t.Fatalf("newManagerFromEnv failed: %v", err)
	}
	defer mgr.Close()

	if mgr.Get("primary") == nil {
		t.Error("expected 'primary' database to exist")
	}
}
