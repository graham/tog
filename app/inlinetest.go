package app

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/peterh/liner"
	"github.com/fsnotify/fsnotify"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/graham/tog/db"
	"github.com/graham/tog/testkit"
	"github.com/graham/tog/tools/routes"
	"github.com/graham/tog/web"
)

func cmdInlinetest(cfg Config, args []string) {
	description := fmt.Sprintf(`Execute HTTP requests against an in-memory test database.

This command is designed for AI agents to test routes without running an HTTP server.
It creates an in-memory database, runs migrations, sets up authentication, and executes requests.

Examples:
  # Simple request
  %s inlinetest /api/items

  # With user authentication
  %s inlinetest --with-user=test@example.com --with-session /api/whoami

  # With API key authentication
  %s inlinetest --with-user=test@example.com --with-api-key /api/items

  # POST request with body
  %s inlinetest --with-user=test@example.com --with-session -X POST -d '{"name":"Widget"}' /api/items

  # Read URLs from stdin (one per line, format: METHOD URL [BODY])
  echo "GET /api/items" | %s inlinetest --with-user=test@example.com --with-session -i

  # Multiple requests from stdin
  cat <<EOF | %s inlinetest --with-user=test@example.com --with-session -i
  GET /api/items
  POST /api/items {"name":"Widget","price":9.99}
  GET /api/items/1
  EOF`, cfg.Name, cfg.Name, cfg.Name, cfg.Name, cfg.Name, cfg.Name)

	fs := newFlagSet(cfg.Name, "inlinetest", description)

	// Flags
	withUser := fs.String("with-user", "", "Create a user with this email")
	withSession := fs.Bool("with-session", false, "Create a session for the user")
	withAPIKey := fs.Bool("with-api-key", false, "Create an API key for the user")
	isAdmin := fs.Bool("admin", false, "Make the user an admin")
	method := fs.String("X", "GET", "HTTP method (GET, POST, PUT, DELETE)")
	body := fs.String("d", "", "Request body (for POST, PUT)")
	inputFile := fs.String("f", "", "Read commands from file (implies -s)")
	interactive := fs.Bool("i", false, "Read URLs from stdin (format: METHOD URL [BODY])")
	useReadline := fs.Bool("r", false, "Enable readline support (history, editing)")
	persistSession := fs.Bool("s", false, "Persist cookies across requests (enables login flows)")
	noSession := fs.Bool("S", false, "Disable session persistence (overrides -f default)")
	sessionFile := fs.String("p", "", "Persist session to file (implies -s). Load at start, save after each request")
	initFile := fs.String("init", "", "Run commands from file before interactive mode (setup script)")
	watchMode := fs.Bool("w", false, "Watch for .go file changes and restart (requires -i or -r)")
	useRealDB := fs.Bool("db", false, "Use real database from DATABASE_CONFIG instead of in-memory (for persistent sessions)")
	quiet := fs.Bool("q", false, "Quiet mode: only print failures (useful for large test suites)")
	migrationsDir := fs.String("migrations", "migrations", "Path to migrations directory")
	verbose := fs.Bool("v", false, "Verbose output (include headers)")

	fs.Parse(args)

	var dbm *db.Manager
	var err error

	if *useRealDB {
		// Use real database from DATABASE_CONFIG
		dbm, err = db.NewManagerFromEnvOrFile("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load database config: %v\n", err)
			fmt.Fprintf(os.Stderr, "hint: set DATABASE_CONFIG or create databases.json\n")
			os.Exit(1)
		}
		if !*quiet {
			fmt.Println("Using real database from DATABASE_CONFIG")
		}
	} else {
		// Create in-memory test database
		dbConfig := &db.Config{
			Databases: map[string]db.DatabaseConfig{
				"primary": {
					Driver: "sqlite3",
					DSN:    ":memory:",
					Pool: db.JSONPoolConfig{
						MaxOpen:            1,
						MaxIdle:            1,
						MaxLifetimeSeconds: 0,
					},
				},
			},
			Default: "primary",
		}

		dbm, err = db.NewManager(dbConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create database: %v\n", err)
			os.Exit(1)
		}

		// Run migrations only for in-memory database
		if err := testkit.RunMigrations(dbm.Default().DB.DB, *migrationsDir); err != nil {
			fmt.Fprintf(os.Stderr, "failed to run migrations: %v\n", err)
			os.Exit(1)
		}
	}
	defer dbm.Close()

	// Build router
	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))
	router.Use(web.ContextMiddleware(dbm))

	// Enable logging only in debug mode
	logger := web.DefaultLogger()
	if logger.Level >= web.LogLevelDebug {
		router.Use(web.LoggingMiddleware(logger))
	}

	// Health endpoints
	router.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
	})
	router.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		web.WriteJSON(w, map[string]string{"status": "healthy"})
	})

	// Call application's LoadRoutes
	if cfg.LoadRoutes != nil {
		if err := cfg.LoadRoutes(router, dbm); err != nil {
			fmt.Fprintf(os.Stderr, "failed to load routes: %v\n", err)
			os.Exit(1)
		}
	}

	// Verify queries
	if err := dbm.VerifyAll(); err != nil {
		fmt.Fprintf(os.Stderr, "query verification failed: %v\n", err)
		os.Exit(1)
	}

	// Set up authentication if requested
	var authToken string
	var tokenType string
	if *withUser != "" {
		userID, err := createInlineUser(dbm.Default(), *withUser, *isAdmin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to create user: %v\n", err)
			os.Exit(1)
		}

		if *withSession {
			authToken, err = createInlineSession(dbm.Default(), userID, "session")
			tokenType = "session"
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create session: %v\n", err)
				os.Exit(1)
			}
		} else if *withAPIKey {
			authToken, err = createInlineSession(dbm.Default(), userID, "api_key")
			tokenType = "api_key"
			if err != nil {
				fmt.Fprintf(os.Stderr, "failed to create API key: %v\n", err)
				os.Exit(1)
			}
		}
	}

	// Create session cookie jar if -s is enabled, -p is set, or if -f is set (unless -S overrides)
	var cookies map[string]*http.Cookie
	enableSession := *persistSession || *sessionFile != "" || (*inputFile != "" && !*noSession)
	if enableSession {
		cookies = make(map[string]*http.Cookie)
	}

	// Load session from file if -p is set
	if *sessionFile != "" {
		if loaded, err := loadSessionFile(*sessionFile); err == nil && loaded != nil {
			cookies = loaded
			if !*quiet {
				fmt.Printf("Loaded %d cookies from %s\n", len(cookies), *sessionFile)
			}
		}
	}

	// Start file watcher if -w is set
	if *watchMode {
		if !*interactive && !*useReadline {
			fmt.Fprintln(os.Stderr, "Warning: -w (watch mode) is most useful with -i or -r (REPL mode)")
		}
		startFileWatcher()
	}

	// Track last response for assertions
	var resp *lastResponse

	// Create REPL config
	replCfg := &replConfig{
		UseRealDB:   *useRealDB,
		SessionFile: *sessionFile,
		InitFile:    *initFile,
		WatchMode:   *watchMode,
		Verbose:     *verbose,
		Quiet:       *quiet,
		WithUser:    *withUser,
		WithSession: *withSession,
		WithAPIKey:  *withAPIKey,
		IsAdmin:     *isAdmin,
		AuthToken:   authToken,
		TokenType:   tokenType,
	}

	// Run init file if specified (setup commands before interactive mode)
	if *initFile != "" {
		if !*quiet {
			fmt.Printf("Running init file: %s\n", *initFile)
		}
		if err := runInitFile(*initFile, router, replCfg, cookies, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "init file error: %v\n", err)
			os.Exit(1)
		}
		// Save session after init if -p is set
		if *sessionFile != "" && cookies != nil {
			if err := saveSessionFile(*sessionFile, cookies); err != nil && !*quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
			}
		}
	}

	// Execute requests
	if *inputFile != "" {
		// Read commands from file
		file, err := os.Open(*inputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to open file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			if !processInteractiveLine(router, line, replCfg, cookies, &resp) {
				break
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "error reading file: %v\n", err)
			os.Exit(1)
		}

		// Save session after file input if -p is set
		if *sessionFile != "" && cookies != nil {
			if err := saveSessionFile(*sessionFile, cookies); err != nil && !*quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
			}
		}
	} else if *interactive {
		if *useReadline {
			runInteractiveWithLiner(router, replCfg, cookies, &resp, *sessionFile, *quiet)
		} else {
			// Read from stdin without readline
			scanner := bufio.NewScanner(os.Stdin)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}

				if !processInteractiveLine(router, line, replCfg, cookies, &resp) {
					break
				}

				// Save session after each request if -p is set
				if *sessionFile != "" && cookies != nil {
					if err := saveSessionFile(*sessionFile, cookies); err != nil && !*quiet {
						fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
					}
				}
			}
			if err := scanner.Err(); err != nil {
				fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		// Execute single request from args
		remaining := fs.Args()
		if len(remaining) == 0 {
			fmt.Fprintln(os.Stderr, "error: no URL specified")
			fs.Usage()
			os.Exit(1)
		}

		path := remaining[0]
		resp = executeRequest(router, *method, path, *body, authToken, tokenType, *verbose, *quiet, cookies)
		_ = resp // Single request mode doesn't use assertions

		// Save session after single request if -p is set
		if *sessionFile != "" && cookies != nil {
			if err := saveSessionFile(*sessionFile, cookies); err != nil && !*quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
			}
		}
	}
}

// parseRequestLine parses a line in format: METHOD URL [BODY]
// If the line starts with '/', it's treated as a GET request to that path.
func parseRequestLine(line string) (method, path, body string) {
	// If line starts with /, treat it as GET <path>
	if strings.HasPrefix(line, "/") {
		parts := strings.SplitN(line, " ", 2)
		method = "GET"
		path = parts[0]
		if len(parts) >= 2 {
			body = parts[1]
		}
		return
	}

	parts := strings.SplitN(line, " ", 3)
	if len(parts) >= 1 {
		method = strings.ToUpper(parts[0])
	}
	if len(parts) >= 2 {
		path = parts[1]
	}
	if len(parts) >= 3 {
		body = parts[2]
	}
	if method == "" {
		method = "GET"
	}
	return
}

// executeRequest executes a single HTTP request and prints the result.
// If cookies is non-nil, cookies from responses will be stored and sent with subsequent requests.
// If quiet is true, suppresses request/response output (only assertions print).
// Returns the response information for use by assertions.
func executeRequest(router chi.Router, method, path, body, authToken, tokenType string, verbose, quiet bool, cookies map[string]*http.Cookie) *lastResponse {
	// Recover from panics (e.g., invalid method/path)
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", r)
		}
	}()

	// Validate inputs
	if path == "" {
		fmt.Fprintln(os.Stderr, "error: no path specified")
		return nil
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, path, bodyReader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add authentication from flags
	if authToken != "" {
		if tokenType == "api_key" {
			req.Header.Set("Authorization", "Bearer "+authToken)
		} else {
			req.AddCookie(&http.Cookie{Name: "session_key", Value: authToken})
		}
	}

	// Add cookies from session jar
	if cookies != nil {
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	// Store cookies from response in session jar
	if cookies != nil {
		for _, cookie := range rec.Result().Cookies() {
			if cookie.MaxAge < 0 {
				// Cookie is being deleted
				delete(cookies, cookie.Name)
			} else {
				cookies[cookie.Name] = cookie
			}
		}
	}

	// Print output (unless quiet mode)
	responseBody := rec.Body.String()
	if !quiet {
		fmt.Printf("%s %s -> %d\n", method, path, rec.Code)
		if verbose {
			for k, v := range rec.Header() {
				fmt.Printf("  %s: %s\n", k, strings.Join(v, ", "))
			}
			fmt.Println()
		}
		fmt.Println(responseBody)
	}

	return &lastResponse{
		Method:     method,
		Path:       path,
		StatusCode: rec.Code,
		Body:       responseBody,
		Headers:    rec.Header(),
	}
}

// runInteractiveWithLiner runs the interactive REPL with liner for tab completion.
// This is extracted to a separate function to ensure defer cleanup runs properly.
func runInteractiveWithLiner(router chi.Router, replCfg *replConfig, cookies map[string]*http.Cookie, resp **lastResponse, sessionFile string, quiet bool) {
	line := liner.NewLiner()
	defer line.Close() // This MUST run to restore terminal state

	// Set up tab completion for routes and meta commands
	routeInfos := routes.CollectRoutes(router, routes.Config{})
	var routePaths []string
	for _, r := range routeInfos {
		routePaths = append(routePaths, r.Path)
	}
	metaCommands := []string{
		"?", "?help", "?routes", "?search", "?session", "?response",
		"?config", "?run", "?assert", "?quit", "quit", "exit",
	}

	line.SetCompleter(func(input string) []string {
		var completions []string
		inputLower := strings.ToLower(input)

		// Complete meta commands
		if strings.HasPrefix(input, "?") || input == "" {
			for _, cmd := range metaCommands {
				if strings.HasPrefix(strings.ToLower(cmd), inputLower) {
					completions = append(completions, cmd)
				}
			}
		}

		// Complete routes (paths starting with /)
		if strings.HasPrefix(input, "/") || input == "" {
			for _, path := range routePaths {
				if strings.HasPrefix(path, input) {
					completions = append(completions, path)
				}
			}
		}

		// Complete HTTP methods
		methods := []string{"GET ", "POST ", "PUT ", "DELETE ", "PATCH "}
		for _, m := range methods {
			if strings.HasPrefix(m, strings.ToUpper(input)) {
				completions = append(completions, m)
			}
		}

		// If input starts with a method, complete the path part
		for _, m := range []string{"GET ", "POST ", "PUT ", "DELETE ", "PATCH "} {
			if strings.HasPrefix(strings.ToUpper(input), m) {
				pathPart := input[len(m):]
				for _, path := range routePaths {
					if strings.HasPrefix(path, pathPart) {
						completions = append(completions, m+path)
					}
				}
			}
		}

		return completions
	})

	// Load history
	historyFile := ""
	if home, err := os.UserHomeDir(); err == nil {
		historyFile = filepath.Join(home, ".inlinetest_history")
		if f, err := os.Open(historyFile); err == nil {
			line.ReadHistory(f)
			f.Close()
		}
	}

	for {
		input, err := line.Prompt("> ")
		if err != nil { // EOF (Ctrl+D) or error
			break
		}
		input = strings.TrimSpace(input)
		if input == "" || strings.HasPrefix(input, "#") {
			continue
		}

		line.AppendHistory(input)

		if !processInteractiveLine(router, input, replCfg, cookies, resp) {
			break
		}

		// Save session after each request if -p is set
		if sessionFile != "" && cookies != nil {
			if err := saveSessionFile(sessionFile, cookies); err != nil && !quiet {
				fmt.Fprintf(os.Stderr, "Warning: failed to save session: %v\n", err)
			}
		}
	}

	// Save history before exiting
	if historyFile != "" {
		if f, err := os.Create(historyFile); err == nil {
			line.WriteHistory(f)
			f.Close()
		}
	}
}

// createInlineUser creates a user in the database.
func createInlineUser(database *db.DB, email string, isAdmin bool) (int64, error) {
	isAdminInt := 0
	if isAdmin {
		isAdminInt = 1
	}

	result, err := database.DB.Exec(
		database.Rebind("INSERT INTO users (email, is_admin, is_active) VALUES ($1, $2, 1)"),
		email, isAdminInt)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

// createInlineSession creates a session or API key for a user.
func createInlineSession(database *db.DB, userID int64, keyType string) (string, error) {
	token := generateInlineToken(keyType)

	_, err := database.DB.Exec(
		database.Rebind("INSERT INTO sessions (key_value, key_type, for_user, is_active, expires_at) VALUES ($1, $2, $3, 1, 0)"),
		token, keyType, userID)
	if err != nil {
		return "", err
	}

	return token, nil
}

// lastResponse holds information about the most recent HTTP response.
type lastResponse struct {
	Method     string
	Path       string
	StatusCode int
	Body       string
	Headers    http.Header
}

// replConfig holds configuration for the REPL session.
type replConfig struct {
	UseRealDB   bool
	SessionFile string
	InitFile    string
	WatchMode   bool
	Verbose     bool
	Quiet       bool
	WithUser    string
	WithSession bool
	WithAPIKey  bool
	IsAdmin     bool
	AuthToken   string
	TokenType   string
}

// generateInlineToken generates a random token with the appropriate prefix.
func generateInlineToken(keyType string) string {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	prefix := "sess_"
	if keyType == "api_key" {
		prefix = "key_"
	}
	return prefix + hex.EncodeToString(bytes)
}

// processInteractiveLine handles a single line of interactive input.
// Returns false if the session should end (e.g., "quit" command).
func processInteractiveLine(router chi.Router, line string, cfg *replConfig, cookies map[string]*http.Cookie, resp **lastResponse) bool {
	// Handle route listing with '.'
	if strings.HasPrefix(line, ".") {
		filter := strings.TrimPrefix(line, ".")
		printFilteredRoutes(router, filter)
		return true
	}

	// Handle ? commands
	if strings.HasPrefix(line, "?") {
		return handleMetaCommand(router, strings.TrimPrefix(line, "?"), cfg, cookies, resp)
	}

	// Handle quit commands
	if line == "quit" || line == "exit" || line == "q" {
		return false
	}

	reqMethod, reqPath, reqBody := parseRequestLine(line)
	*resp = executeRequest(router, reqMethod, reqPath, reqBody, cfg.AuthToken, cfg.TokenType, cfg.Verbose, cfg.Quiet, cookies)
	return true
}

// handleMetaCommand handles ? commands. Returns false if session should end.
func handleMetaCommand(router chi.Router, cmd string, cfg *replConfig, cookies map[string]*http.Cookie, resp **lastResponse) bool {
	parts := strings.SplitN(strings.TrimSpace(cmd), " ", 2)
	command := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	switch command {
	case "":
		printMetaHelp()
	case "help", "h":
		printMetaHelp()
	case "search", "s":
		printFilteredRoutes(router, arg)
	case "routes", "r":
		printFilteredRoutes(router, "")
	case "session":
		printSessionInfo(cookies)
	case "response", "resp":
		printLastResponse(*resp)
	case "assert", "a":
		runAssertion(arg, *resp, cfg.Quiet)
	case "config", "cfg":
		printReplConfig(cfg)
	case "run":
		if arg == "" {
			fmt.Println("Usage: ?run <filename>")
			return true
		}
		if err := runInitFile(arg, router, cfg, cookies, resp); err != nil {
			fmt.Fprintf(os.Stderr, "Error running file: %v\n", err)
		}
	case "quit", "exit", "q":
		return false
	default:
		fmt.Printf("Unknown command: ?%s\n", command)
		fmt.Println("Type ? for help")
	}
	return true
}

// printMetaHelp prints help for ? commands.
func printMetaHelp() {
	fmt.Println("Commands:")
	fmt.Println("  ?                       Show this help")
	fmt.Println("  ?search <term>          Search routes (shortcut: .term)")
	fmt.Println("  ?routes                 List all routes (shortcut: .)")
	fmt.Println("  ?session                Show current session cookies")
	fmt.Println("  ?response               Show last response")
	fmt.Println("  ?config                 Show REPL configuration/launch parameters")
	fmt.Println("  ?run <file>             Run commands from file")
	fmt.Println("  ?assert status N        Assert last response status code")
	fmt.Println("  ?assert body TEXT       Assert last response body contains text")
	fmt.Println("  ?assert json .path VAL  Assert JSON field at path equals value")
	fmt.Println("  ?quit                   Exit (shortcuts: quit, exit, q)")
	fmt.Println()
	fmt.Println("Request formats:")
	fmt.Println("  /path              GET request to /path")
	fmt.Println("  METHOD /path       Request with method")
	fmt.Println("  METHOD /path JSON  Request with body")
}

// printLastResponse prints the last HTTP response.
func printLastResponse(resp *lastResponse) {
	if resp == nil {
		fmt.Println("No response yet")
		return
	}
	fmt.Printf("Last response: %s %s -> %d\n", resp.Method, resp.Path, resp.StatusCode)
	fmt.Println(resp.Body)
}

// runAssertion runs an assertion against the last response.
// If quiet is true, only failures are printed.
func runAssertion(arg string, resp *lastResponse, quiet bool) {
	if resp == nil {
		printAssertResult(false, "no response to assert against", quiet)
		return
	}

	parts := strings.SplitN(strings.TrimSpace(arg), " ", 2)
	if len(parts) < 2 {
		fmt.Println("Usage: ?assert status <code> | ?assert body <text>")
		return
	}

	assertType := strings.ToLower(parts[0])
	assertValue := parts[1]

	switch assertType {
	case "status", "code":
		expected := 0
		fmt.Sscanf(assertValue, "%d", &expected)
		if expected == 0 {
			printAssertResult(false, fmt.Sprintf("invalid status code: %s", assertValue), quiet)
			return
		}
		if resp.StatusCode == expected {
			printAssertResult(true, fmt.Sprintf("status %d == %d", resp.StatusCode, expected), quiet)
		} else {
			printAssertResult(false, fmt.Sprintf("status %d != %d", resp.StatusCode, expected), quiet)
		}

	case "body", "contains":
		// Remove quotes if present
		text := strings.Trim(assertValue, `"'`)
		if strings.Contains(resp.Body, text) {
			printAssertResult(true, fmt.Sprintf("body contains %q", text), quiet)
		} else {
			printAssertResult(false, fmt.Sprintf("body does not contain %q", text), quiet)
		}

	case "json", "j":
		// Parse: .path.to.field "expected value" or .path.to.field expected
		jsonParts := parseJSONAssertArgs(assertValue)
		if len(jsonParts) < 2 {
			fmt.Println("Usage: ?assert json .path.to.field \"expected value\"")
			return
		}
		jsonPath := jsonParts[0]
		expected := jsonParts[1]

		actual, err := extractJSONPath(resp.Body, jsonPath)
		if err != nil {
			printAssertResult(false, fmt.Sprintf("json path %s: %v", jsonPath, err), quiet)
			return
		}

		if actual == expected {
			printAssertResult(true, fmt.Sprintf("json %s == %q", jsonPath, expected), quiet)
		} else {
			printAssertResult(false, fmt.Sprintf("json %s: got %q, want %q", jsonPath, actual, expected), quiet)
		}

	default:
		fmt.Printf("Unknown assertion type: %s\n", assertType)
		fmt.Println("Usage: ?assert status <code> | ?assert body <text> | ?assert json .path \"value\"")
	}
}

// printAssertResult prints a pass/fail assertion result with color.
// If quiet is true, only failures are printed.
func printAssertResult(pass bool, message string, quiet bool) {
	if pass {
		if !quiet {
			fmt.Printf("\033[32mPASS\033[0m: %s\n", message)
		}
	} else {
		fmt.Printf("\033[31mFAIL\033[0m: %s\n", message)
	}
}

// parseJSONAssertArgs parses arguments for JSON assertions.
// Handles: .path "quoted value" or .path unquoted_value
func parseJSONAssertArgs(args string) []string {
	args = strings.TrimSpace(args)
	if args == "" {
		return nil
	}

	// Find the first space to separate path from value
	spaceIdx := strings.Index(args, " ")
	if spaceIdx == -1 {
		return []string{args}
	}

	path := args[:spaceIdx]
	valueStr := strings.TrimSpace(args[spaceIdx+1:])

	// Handle quoted values
	if strings.HasPrefix(valueStr, `"`) && strings.HasSuffix(valueStr, `"`) && len(valueStr) >= 2 {
		valueStr = valueStr[1 : len(valueStr)-1]
	} else if strings.HasPrefix(valueStr, `'`) && strings.HasSuffix(valueStr, `'`) && len(valueStr) >= 2 {
		valueStr = valueStr[1 : len(valueStr)-1]
	}

	return []string{path, valueStr}
}

// extractJSONPath extracts a value from JSON using a simple dot-notation path.
// Supports: .field, .nested.field, .array.0.field (numeric indices for arrays)
func extractJSONPath(jsonStr, path string) (string, error) {
	if !strings.HasPrefix(path, ".") {
		return "", fmt.Errorf("path must start with '.'")
	}

	var data any
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		return "", fmt.Errorf("invalid JSON: %v", err)
	}

	// Split path into parts, skipping the leading empty string from "."
	parts := strings.Split(path[1:], ".")
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "") {
		return "", fmt.Errorf("empty path")
	}

	current := data
	for _, part := range parts {
		if part == "" {
			continue
		}

		switch v := current.(type) {
		case map[string]any:
			val, ok := v[part]
			if !ok {
				return "", fmt.Errorf("key %q not found", part)
			}
			current = val

		case []any:
			// Try to parse as array index
			idx, err := strconv.Atoi(part)
			if err != nil {
				return "", fmt.Errorf("expected array index, got %q", part)
			}
			if idx < 0 || idx >= len(v) {
				return "", fmt.Errorf("array index %d out of bounds (len=%d)", idx, len(v))
			}
			current = v[idx]

		default:
			return "", fmt.Errorf("cannot access %q on %T", part, current)
		}
	}

	// Convert the final value to string
	switch v := current.(type) {
	case string:
		return v, nil
	case float64:
		// Check if it's an integer
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10), nil
		}
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case bool:
		return strconv.FormatBool(v), nil
	case nil:
		return "null", nil
	default:
		// For complex types, return JSON representation
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("cannot stringify %T: %v", v, err)
		}
		return string(b), nil
	}
}

// printSessionInfo prints current session cookie information.
func printSessionInfo(cookies map[string]*http.Cookie) {
	if cookies == nil {
		fmt.Println("Session persistence disabled (use -s flag)")
		return
	}
	if len(cookies) == 0 {
		fmt.Println("No cookies in session")
		return
	}
	fmt.Printf("Session cookies (%d):\n", len(cookies))
	for name, cookie := range cookies {
		value := cookie.Value
		if len(value) > 40 {
			value = value[:37] + "..."
		}
		fmt.Printf("  %s: %s\n", name, value)
	}
}

// runInitFile runs commands from a file (used for setup before interactive mode).
func runInitFile(filename string, router chi.Router, cfg *replConfig, cookies map[string]*http.Cookie, resp **lastResponse) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if !processInteractiveLine(router, line, cfg, cookies, resp) {
			break
		}
	}
	return scanner.Err()
}

// printReplConfig prints the REPL configuration.
func printReplConfig(cfg *replConfig) {
	fmt.Println("REPL Configuration:")
	fmt.Printf("  Database:       %s\n", ifelse(cfg.UseRealDB, "real (DATABASE_CONFIG)", "in-memory"))
	if cfg.WithUser != "" {
		fmt.Printf("  User:           %s\n", cfg.WithUser)
		fmt.Printf("  Admin:          %v\n", cfg.IsAdmin)
	}
	if cfg.WithSession {
		fmt.Printf("  Auth:           session cookie\n")
	} else if cfg.WithAPIKey {
		fmt.Printf("  Auth:           API key\n")
	}
	if cfg.SessionFile != "" {
		fmt.Printf("  Session file:   %s\n", cfg.SessionFile)
	}
	if cfg.InitFile != "" {
		fmt.Printf("  Init file:      %s\n", cfg.InitFile)
	}
	if cfg.WatchMode {
		fmt.Printf("  Watch mode:     enabled\n")
	}
	fmt.Printf("  Verbose:        %v\n", cfg.Verbose)
	fmt.Printf("  Quiet:          %v\n", cfg.Quiet)
}

func ifelse(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// printFilteredRoutes prints routes, optionally filtered by a search term.
func printFilteredRoutes(router chi.Router, filter string) {
	routeInfos := routes.CollectRoutes(router, routes.Config{})

	filter = strings.ToLower(filter)
	for _, r := range routeInfos {
		if filter == "" || strings.Contains(strings.ToLower(r.Path), filter) {
			fmt.Printf("  %-8s %s\n", r.Method, r.Path)
		}
	}
}

// sessionFileData represents the JSON structure for persistent session storage.
type sessionFileData struct {
	Cookies []cookieData `json:"cookies"`
}

type cookieData struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// loadSessionFile loads cookies from a JSON file.
func loadSessionFile(filename string) (map[string]*http.Cookie, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // File doesn't exist yet, that's OK
		}
		return nil, err
	}

	var fileData sessionFileData
	if err := json.Unmarshal(data, &fileData); err != nil {
		return nil, err
	}

	cookies := make(map[string]*http.Cookie)
	for _, c := range fileData.Cookies {
		cookies[c.Name] = &http.Cookie{Name: c.Name, Value: c.Value}
	}
	return cookies, nil
}

// saveSessionFile saves cookies to a JSON file.
func saveSessionFile(filename string, cookies map[string]*http.Cookie) error {
	fileData := sessionFileData{
		Cookies: make([]cookieData, 0, len(cookies)),
	}
	for _, cookie := range cookies {
		fileData.Cookies = append(fileData.Cookies, cookieData{
			Name:  cookie.Name,
			Value: cookie.Value,
		})
	}

	data, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0600)
}

// startFileWatcher starts watching for .go file changes in the current directory.
// When a .go file changes, it immediately restarts the process.
// Returns a channel for compatibility (but restart happens automatically).
func startFileWatcher() chan struct{} {
	changed := make(chan struct{}, 1)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to start file watcher: %v\n", err)
		return nil
	}

	// Watch current directory and subdirectories
	// Also watch parent directories to catch changes in the app package
	watchCount := 0
	dirsToWatch := []string{"."}

	// Also watch parent app directory if we're in examples
	cwd, _ := os.Getwd()
	if strings.HasSuffix(cwd, "/examples") || strings.HasSuffix(cwd, "\\examples") {
		dirsToWatch = append(dirsToWatch, "../app")
	}

	for _, startDir := range dirsToWatch {
		err = filepath.Walk(startDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // Skip errors
			}
			if info.IsDir() {
				name := info.Name()
				// Skip hidden directories (but allow "." and "..") and vendor
				if (strings.HasPrefix(name, ".") && name != "." && name != "..") || name == "vendor" {
					return filepath.SkipDir
				}
				if err := watcher.Add(path); err == nil {
					watchCount++
				}
				return nil
			}
			return nil
		})
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to watch directories: %v\n", err)
		return nil
	}

	fmt.Fprintf(os.Stderr, "Watching %d directories for .go file changes...\n", watchCount)

	go func() {
		defer watcher.Close()
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Only trigger on .go file changes (write or create)
				if strings.HasSuffix(event.Name, ".go") {
					if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
						fmt.Fprintf(os.Stderr, "\n\033[33m%s changed. Restarting...\033[0m\n", event.Name)
						// Small delay to let file writes complete
						time.Sleep(100 * time.Millisecond)
						restartProcess()
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
			}
		}
	}()

	return changed
}

// restartProcess re-executes the current process with the same arguments.
// It detects if running via "go run" and recompiles, otherwise re-executes the binary.
func restartProcess() {
	// Check if running from a temp directory (indicates "go run")
	// go run compiles to /tmp/go-build* or similar
	executable := os.Args[0]
	isGoRun := strings.Contains(executable, "go-build") ||
		strings.Contains(executable, "/tmp/") ||
		strings.Contains(executable, "/var/folders/") // macOS temp

	if isGoRun {
		// Re-run with go run to recompile
		goBinary, err := exec.LookPath("go")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Cannot find 'go' to restart: %v\n", err)
			os.Exit(1)
		}
		// Reconstruct go run command
		args := append([]string{"go", "run", "main.go"}, os.Args[1:]...)
		err = syscall.Exec(goBinary, args, os.Environ())
		// If we get here, exec failed
		fmt.Fprintf(os.Stderr, "Failed to restart: %v\n", err)
		os.Exit(1)
	} else {
		// Re-execute the compiled binary directly
		err := syscall.Exec(executable, os.Args, os.Environ())
		fmt.Fprintf(os.Stderr, "Failed to restart: %v\n", err)
		os.Exit(1)
	}
}
