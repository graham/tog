package app

import (
	"fmt"
	"os"
	"sort"

	"github.com/graham/tog/db"
)

// envVar represents an environment variable with its current value and description.
type envVar struct {
	Name        string
	Value       string
	Default     string
	Description string
}

func cmdEnv(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "env", "Show environment variables and database configuration.")
	fs.Parse(args)

	fmt.Println("Environment Variables:")
	fmt.Println("======================")
	fmt.Println()

	vars := []envVar{
		{"HOST", os.Getenv("HOST"), "", "Server bind address"},
		{"PORT", os.Getenv("PORT"), "8080", "Server port"},
		{"ENVIRONMENT", os.Getenv("ENVIRONMENT"), "", "Set to 'dev' for dev mode"},
		{"LOG_LEVEL", os.Getenv("LOG_LEVEL"), "info", "Logging level (debug/info/warn/error)"},
		{"NO_COLOR", os.Getenv("NO_COLOR"), "", "Disable colored output"},
		{"DATABASE_CONFIG", os.Getenv("DATABASE_CONFIG"), "databases.json", "Database config file path"},
		{"GOOSE_DBSTRING", os.Getenv("GOOSE_DBSTRING"), "", "Database connection string (fallback)"},
		{"GOOSE_DRIVER", os.Getenv("GOOSE_DRIVER"), "sqlite3", "Database driver (fallback)"},
		{"SQL_STOPWATCH", os.Getenv("SQL_STOPWATCH"), "", "Set to '1' to log SQL timing"},
		{"APP_CONFIG", os.Getenv("APP_CONFIG"), "config.json", "App config file path"},
	}

	for _, v := range vars {
		value := v.Value
		if value == "" {
			if v.Default != "" {
				value = fmt.Sprintf("(default: %s)", v.Default)
			} else {
				value = "(not set)"
			}
		}
		fmt.Printf("  %-18s %s\n", v.Name+":", value)
	}

	fmt.Println()
	fmt.Println("Database Configuration:")
	fmt.Println("=======================")
	fmt.Println()

	dbm, err := db.NewManagerFromEnvOrFile("")
	if err != nil {
		fmt.Printf("  Error loading database config: %v\n", err)
		return
	}
	defer dbm.Close()

	configPath := db.GetConfigPath("")
	if _, err := os.Stat(configPath); err == nil {
		fmt.Printf("  Config file: %s\n", configPath)
	} else {
		fmt.Println("  Config file: (using environment variables)")
	}
	fmt.Println()

	names := dbm.Names()
	sort.Strings(names)

	fmt.Printf("  Databases: %d\n", len(names))
	for _, name := range names {
		database := dbm.Get(name)
		if database == nil {
			continue
		}

		marker := ""
		if name == "primary" {
			marker = " (default)"
		}

		readOnly := ""
		if dbm.IsReadOnly(name) {
			readOnly = " [read-only]"
		}

		fmt.Printf("    - %s: %s%s%s\n", name, database.DriverName(), marker, readOnly)
	}
}
