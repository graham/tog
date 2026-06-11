package app

import (
	"fmt"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
)

func cmdVerify(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "verify", "Verify all SQL queries against the database schema.")
	configFile := fs.String("config", "", "database config file (default: databases.json)")
	fs.Parse(args)

	configPath := *configFile
	if configPath == "" {
		configPath = os.Getenv("DATABASE_CONFIG")
	}

	dbm, err := db.NewManagerFromEnvOrFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create database manager: %v\n", err)
		os.Exit(1)
	}
	defer dbm.Close()

	// Create a dummy router to call LoadRoutes (which registers queries)
	r := chi.NewRouter()
	if cfg.LoadRoutes != nil {
		if err := cfg.LoadRoutes(r, dbm); err != nil {
			fmt.Fprintf(os.Stderr, "failed to load routes: %v\n", err)
			os.Exit(1)
		}
	}

	// Verify all queries
	if err := dbm.VerifyAll(); err != nil {
		fmt.Fprintf(os.Stderr, "verification failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("all queries verified successfully")
}
