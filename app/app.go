package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/db"
	"github.com/joho/godotenv"
)

// Config holds configuration for a tog application.
type Config struct {
	// Name is the application name (used in usage output and testdocs).
	Name string

	// LoadRoutes is called to set up application-specific routes.
	// It receives the chi router (with standard middleware already applied
	// and health/docs endpoints mounted) and the database manager.
	// The function should:
	//   - Register queries using db.Register/db.RegisterExec
	//   - Mount application routes on the router
	LoadRoutes func(r chi.Router, dbm *db.Manager) error

	// Testdocs configuration (optional).
	Testdocs *TestdocsConfig
}

// TestdocsConfig holds configuration for the testdocs command.
type TestdocsConfig struct {
	// OutputPath is the output HTML file path. Default: "docs/tests.html"
	OutputPath string
	// PkgPattern is the Go package pattern to test. Default: inferred from go.mod
	PkgPattern string
	// RootDir is the root directory to scan for test files. Default: "."
	RootDir string
}

// Run is the main entry point for tog applications.
// It handles CLI argument parsing and dispatches to the appropriate command.
func Run(cfg Config) {
	_ = godotenv.Load()

	if cfg.Name == "" {
		cfg.Name = "app"
	}

	if len(os.Args) < 2 {
		printUsage(cfg.Name)
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "serve":
		cmdServe(cfg, args)
	case "watch":
		cmdWatch(cfg, args)
	case "verify":
		cmdVerify(cfg, args)
	case "routes":
		cmdRoutes(cfg, args)
	case "testdocs":
		cmdTestdocs(cfg, args)
	case "findqueries":
		cmdFindqueries(cfg, args)
	case "inlinetest":
		cmdInlinetest(cfg, args)
	case "jstest":
		cmdJstest(cfg, args)
	case "schema":
		cmdSchema(cfg, args)
	case "openapi":
		cmdOpenAPI(cfg, args)
	case "env":
		cmdEnv(cfg, args)
	case "agent-prompt":
		cmdAgentPrompt(cfg, args)
	case "completion":
		cmdCompletion(cfg, args)
	case "help", "-h", "--help":
		printUsage(cfg.Name)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage(cfg.Name)
		os.Exit(1)
	}
}

func printUsage(name string) {
	fmt.Printf(`Usage: %s <command> [options]

Commands:
  serve        Run the HTTP server
  watch        Run with auto-reload on file changes (requires air)
  verify       Verify all queries against the database
  routes       List all registered routes
  schema       Dump database schema as JSON (includes foreign keys and indexes)
  openapi      Generate OpenAPI 3.0 specification from routes
  env          Show environment variables and database configuration
  testdocs     Generate test documentation HTML
  findqueries  Find unregistered SQL queries
  inlinetest   Execute requests against in-memory test database (for AI agents)
  jstest       Execute JavaScript test scripts against in-memory test database
  agent-prompt Show AI-optimized documentation for commands
  completion   Generate shell completion scripts (bash, zsh, fish)

Environment Variables:
  HOST              Server bind address (default: all interfaces)
  PORT              Server port (default: 8080)
  ENVIRONMENT       Set to "dev" to enable dev routes and verbose errors
  LOG_LEVEL         Logging level: debug, info, warn, error (default: info)
  NO_COLOR          Disable colored output when set
  DATABASE_CONFIG   Path to databases.json (default: databases.json)
  GOOSE_DBSTRING    Database connection string (fallback if no config file)
  GOOSE_DRIVER      Database driver: sqlite3, postgres (fallback if no config file)
  SQL_STOPWATCH     Set to "1" to log SQL query timing
  APP_CONFIG        Path to config.json for app settings

Run '%s <command> -h' for command-specific options.
`, name, name)
}

// newFlagSet creates a new flag set with a custom usage function.
func newFlagSet(name, cmdName, description string) *flag.FlagSet {
	fs := flag.NewFlagSet(cmdName, flag.ExitOnError)
	fs.Usage = func() {
		fmt.Printf("Usage: %s %s [options]\n\n%s\n", name, cmdName, description)
		fmt.Println("\nOptions:")
		fs.PrintDefaults()
	}
	return fs
}
