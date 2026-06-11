package verify

import (
	"github.com/graham/tog/db"
)

// Config configures the query verification.
type Config struct {
	// ConfigPath is the database config file path.
	// If empty, uses DATABASE_CONFIG env var or "databases.json".
	ConfigPath string

	// SetupFunc is called to register queries on the manager.
	// This typically calls all module RegisterQueries functions.
	// If nil, only verifies that the manager can be created.
	SetupFunc func(dbm *db.Manager) error
}

// Run verifies all registered queries against the database.
// Returns nil on success, error with details on failure.
func Run(cfg Config) error {
	dbm, err := db.NewManagerFromEnvOrFile(cfg.ConfigPath)
	if err != nil {
		return err
	}
	defer dbm.Close()

	if cfg.SetupFunc != nil {
		if err := cfg.SetupFunc(dbm); err != nil {
			return err
		}
	}

	return dbm.VerifyAll()
}

// RunWithManager verifies all queries against an existing manager.
// This is useful when you've already created and configured a manager.
func RunWithManager(dbm *db.Manager) error {
	return dbm.VerifyAll()
}
