package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/buke/quickjs-go"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/graham/tog/db"
	"github.com/graham/tog/testkit"
	"github.com/graham/tog/web"
)

// jstestState holds the state shared between Go and JavaScript.
type jstestState struct {
	router    chi.Router
	database  *db.DB
	cookies   map[string]*http.Cookie
	authToken string
	tokenType string
	verbose   bool
	quiet     bool
	// Track created users for login
	users map[string]int64 // email -> userID
}

func cmdJstest(cfg Config, args []string) {
	description := fmt.Sprintf(`Execute JavaScript test scripts against an in-memory test database.

This command runs JavaScript files that can make HTTP requests, manage sessions,
and run assertions - all without starting an HTTP server.

Examples:
  %s jstest tests/api_test.js
  %s jstest --db tests/integration.js    # Use real database
  %s jstest -v tests/debug.js            # Verbose output
  %s jstest tests/*.js                   # Run multiple scripts

JavaScript API:
  client.get("/path")                     HTTP GET request
  client.post("/path", {data})            HTTP POST with JSON body
  client.put("/path", {data})             HTTP PUT with JSON body
  client.delete("/path")                  HTTP DELETE request

  client.createUser("email")              Create a user
  client.createUser("email", true)        Create an admin user
  client.login("email")                   Authenticate with session cookie
  client.loginWithApiKey("email")         Authenticate with API key
  client.logout()                         Clear authentication

  assert(condition, "message")            Assert condition is truthy
  assertEqual(a, b, "message")            Assert a === b
  assertStatus(resp, 200)                 Assert response status
  assertContains(str, "text")             Assert string contains text
  assertJSON(resp, ".path", "value")      Assert JSON field value

  print("message")                        Print to stdout
  console.log("message")                  Alias for print
`, cfg.Name, cfg.Name, cfg.Name, cfg.Name)

	fs := newFlagSet(cfg.Name, "jstest", description)

	useRealDB := fs.Bool("db", false, "Use real database from DATABASE_CONFIG instead of in-memory")
	verbose := fs.Bool("v", false, "Verbose output (show request/response details)")
	quiet := fs.Bool("q", false, "Quiet mode: only print failures")
	migrationsDir := fs.String("migrations", "migrations", "Path to migrations directory")

	fs.Parse(args)

	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "error: no script file specified")
		fs.Usage()
		os.Exit(1)
	}

	// Set up database
	var dbm *db.Manager
	var err error

	if *useRealDB {
		dbm, err = db.NewManagerFromEnvOrFile("")
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to load database config: %v\n", err)
			os.Exit(1)
		}
		if !*quiet {
			fmt.Println("Using real database from DATABASE_CONFIG")
		}
	} else {
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

	router.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("ok"))
	})
	router.Get("/health", func(w http.ResponseWriter, req *http.Request) {
		web.WriteJSON(w, map[string]string{"status": "healthy"})
	})

	if cfg.LoadRoutes != nil {
		if err := cfg.LoadRoutes(router, dbm); err != nil {
			fmt.Fprintf(os.Stderr, "failed to load routes: %v\n", err)
			os.Exit(1)
		}
	}

	if err := dbm.VerifyAll(); err != nil {
		fmt.Fprintf(os.Stderr, "query verification failed: %v\n", err)
		os.Exit(1)
	}

	// Create state
	state := &jstestState{
		router:   router,
		database: dbm.Default(),
		cookies:  make(map[string]*http.Cookie),
		verbose:  *verbose,
		quiet:    *quiet,
		users:    make(map[string]int64),
	}

	// Run each script file
	exitCode := 0
	for _, scriptFile := range fs.Args() {
		if !*quiet {
			fmt.Printf("Running %s...\n", scriptFile)
		}
		if err := runJsScript(state, scriptFile); err != nil {
			fmt.Fprintf(os.Stderr, "FAIL: %s: %v\n", scriptFile, err)
			exitCode = 1
		} else if !*quiet {
			fmt.Printf("PASS: %s\n", scriptFile)
		}
	}

	os.Exit(exitCode)
}

func runJsScript(state *jstestState, filename string) error {
	script, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read script: %w", err)
	}

	// Create QuickJS runtime
	rt := quickjs.NewRuntime()
	defer rt.Close()

	ctx := rt.NewContext()
	defer ctx.Close()

	// Register all the globals
	registerClientObject(ctx, state)
	registerTestUtils(ctx, state)
	registerConsole(ctx, state)

	// Execute the script
	result := ctx.Eval(string(script))
	defer result.Free()

	if result.IsException() {
		exc := ctx.Exception()
		return fmt.Errorf("exception: %v", exc)
	}

	return nil
}

// registerClientObject creates the `client` global object with HTTP methods.
func registerClientObject(ctx *quickjs.Context, state *jstestState) {
	client := ctx.Object()

	// client.get(path)
	client.Set("get", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("get() requires a path argument"))
		}
		path := args[0].String()
		resp := executeRequest(state.router, "GET", path, "", state.authToken, state.tokenType, state.verbose, state.quiet, state.cookies)
		return createResponseObject(ctx, resp)
	}))

	// client.post(path, body)
	client.Set("post", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("post() requires a path argument"))
		}
		path := args[0].String()
		body := ""
		if len(args) >= 2 {
			body = jsValueToJSON(args[1])
		}
		resp := executeRequest(state.router, "POST", path, body, state.authToken, state.tokenType, state.verbose, state.quiet, state.cookies)
		return createResponseObject(ctx, resp)
	}))

	// client.put(path, body)
	client.Set("put", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("put() requires a path argument"))
		}
		path := args[0].String()
		body := ""
		if len(args) >= 2 {
			body = jsValueToJSON(args[1])
		}
		resp := executeRequest(state.router, "PUT", path, body, state.authToken, state.tokenType, state.verbose, state.quiet, state.cookies)
		return createResponseObject(ctx, resp)
	}))

	// client.delete(path)
	client.Set("delete", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("delete() requires a path argument"))
		}
		path := args[0].String()
		resp := executeRequest(state.router, "DELETE", path, "", state.authToken, state.tokenType, state.verbose, state.quiet, state.cookies)
		return createResponseObject(ctx, resp)
	}))

	// client.createUser(email, isAdmin?)
	client.Set("createUser", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("createUser() requires an email argument"))
		}
		email := args[0].String()
		isAdmin := false
		if len(args) >= 2 && args[1].IsBool() {
			isAdmin = args[1].Bool()
		}

		userID, err := createInlineUser(state.database, email, isAdmin)
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to create user: %w", err))
		}
		state.users[email] = userID

		if state.verbose {
			fmt.Printf("Created user: %s (id=%d, admin=%v)\n", email, userID, isAdmin)
		}
		return ctx.Int64(userID)
	}))

	// client.login(email)
	client.Set("login", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("login() requires an email argument"))
		}
		email := args[0].String()

		userID, ok := state.users[email]
		if !ok {
			return ctx.ThrowError(fmt.Errorf("user %q not found - call createUser() first", email))
		}

		token, err := createInlineSession(state.database, userID, "session")
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to create session: %w", err))
		}
		state.authToken = token
		state.tokenType = "session"

		if state.verbose {
			fmt.Printf("Logged in as: %s (session)\n", email)
		}
		return ctx.Bool(true)
	}))

	// client.loginWithApiKey(email)
	client.Set("loginWithApiKey", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("loginWithApiKey() requires an email argument"))
		}
		email := args[0].String()

		userID, ok := state.users[email]
		if !ok {
			return ctx.ThrowError(fmt.Errorf("user %q not found - call createUser() first", email))
		}

		token, err := createInlineSession(state.database, userID, "api_key")
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("failed to create API key: %w", err))
		}
		state.authToken = token
		state.tokenType = "api_key"

		if state.verbose {
			fmt.Printf("Logged in as: %s (API key)\n", email)
		}
		return ctx.Bool(true)
	}))

	// client.logout()
	client.Set("logout", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		state.authToken = ""
		state.tokenType = ""
		state.cookies = make(map[string]*http.Cookie)
		if state.verbose {
			fmt.Println("Logged out")
		}
		return ctx.Bool(true)
	}))

	ctx.Globals().Set("client", client)
}

// createResponseObject wraps a lastResponse in a JavaScript object.
func createResponseObject(ctx *quickjs.Context, resp *lastResponse) *quickjs.Value {
	if resp == nil {
		return ctx.Null()
	}

	obj := ctx.Object()
	obj.Set("status", ctx.Int32(int32(resp.StatusCode)))
	obj.Set("body", ctx.String(resp.Body))

	// headers object
	headers := ctx.Object()
	for k, v := range resp.Headers {
		headers.Set(k, ctx.String(strings.Join(v, ", ")))
	}
	obj.Set("headers", headers)

	// json() method - parse body as JSON
	obj.Set("json", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		bodyProp := this.Get("body")
		defer bodyProp.Free()
		body := bodyProp.String()

		var data any
		if err := json.Unmarshal([]byte(body), &data); err != nil {
			return ctx.ThrowError(fmt.Errorf("invalid JSON: %w", err))
		}
		return goValueToJS(ctx, data)
	}))

	return obj
}

// registerTestUtils creates the assertion functions.
func registerTestUtils(ctx *quickjs.Context, state *jstestState) {
	// assert(condition, message?)
	ctx.Globals().Set("assert", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 1 {
			return ctx.ThrowError(fmt.Errorf("assert() requires a condition argument"))
		}
		condition := args[0].Bool()
		message := "assertion failed"
		if len(args) >= 2 {
			message = args[1].String()
		}
		if !condition {
			return ctx.ThrowError(fmt.Errorf("assert: %s", message))
		}
		return ctx.Bool(true)
	}))

	// assertEqual(a, b, message?)
	ctx.Globals().Set("assertEqual", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 2 {
			return ctx.ThrowError(fmt.Errorf("assertEqual() requires two arguments"))
		}
		a := jsValueToStringPtr(args[0])
		b := jsValueToStringPtr(args[1])
		message := fmt.Sprintf("expected %q == %q", a, b)
		if len(args) >= 3 {
			message = args[2].String() + ": " + message
		}
		if a != b {
			return ctx.ThrowError(fmt.Errorf("assertEqual: %s", message))
		}
		return ctx.Bool(true)
	}))

	// assertNotEqual(a, b, message?)
	ctx.Globals().Set("assertNotEqual", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 2 {
			return ctx.ThrowError(fmt.Errorf("assertNotEqual() requires two arguments"))
		}
		a := jsValueToStringPtr(args[0])
		b := jsValueToStringPtr(args[1])
		message := fmt.Sprintf("expected %q != %q", a, b)
		if len(args) >= 3 {
			message = args[2].String() + ": " + message
		}
		if a == b {
			return ctx.ThrowError(fmt.Errorf("assertNotEqual: %s", message))
		}
		return ctx.Bool(true)
	}))

	// assertStatus(response, ...codes)
	ctx.Globals().Set("assertStatus", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 2 {
			return ctx.ThrowError(fmt.Errorf("assertStatus() requires response and status code arguments"))
		}
		statusProp := args[0].Get("status")
		defer statusProp.Free()
		status := int(statusProp.Int32())

		// Collect expected codes
		var expected []int
		for i := 1; i < len(args); i++ {
			expected = append(expected, int(args[i].Int32()))
		}

		for _, exp := range expected {
			if status == exp {
				return ctx.Bool(true)
			}
		}
		return ctx.ThrowError(fmt.Errorf("assertStatus: got %d, expected one of %v", status, expected))
	}))

	// assertContains(str, substr, message?)
	ctx.Globals().Set("assertContains", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 2 {
			return ctx.ThrowError(fmt.Errorf("assertContains() requires string and substring arguments"))
		}
		str := args[0].String()
		substr := args[1].String()
		message := fmt.Sprintf("expected %q to contain %q", str, substr)
		if len(args) >= 3 {
			message = args[2].String()
		}
		if !strings.Contains(str, substr) {
			return ctx.ThrowError(fmt.Errorf("assertContains: %s", message))
		}
		return ctx.Bool(true)
	}))

	// assertJSON(response, path, expected, message?)
	ctx.Globals().Set("assertJSON", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		if len(args) < 3 {
			return ctx.ThrowError(fmt.Errorf("assertJSON() requires response, path, and expected value arguments"))
		}
		bodyProp := args[0].Get("body")
		defer bodyProp.Free()
		body := bodyProp.String()
		path := args[1].String()
		expected := jsValueToStringPtr(args[2])

		actual, err := extractJSONPath(body, path)
		if err != nil {
			return ctx.ThrowError(fmt.Errorf("assertJSON: %w", err))
		}

		if actual != expected {
			return ctx.ThrowError(fmt.Errorf("assertJSON: at %s got %q, expected %q", path, actual, expected))
		}
		return ctx.Bool(true)
	}))
}

// registerConsole creates the console.log and print functions.
func registerConsole(ctx *quickjs.Context, state *jstestState) {
	// print(args...)
	ctx.Globals().Set("print", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		var parts []string
		for _, arg := range args {
			parts = append(parts, jsValueToStringPtr(arg))
		}
		fmt.Println(strings.Join(parts, " "))
		return ctx.Undefined()
	}))

	// console.log(args...)
	console := ctx.Object()
	console.Set("log", ctx.Function(func(ctx *quickjs.Context, this *quickjs.Value, args []*quickjs.Value) *quickjs.Value {
		var parts []string
		for _, arg := range args {
			parts = append(parts, jsValueToStringPtr(arg))
		}
		fmt.Println(strings.Join(parts, " "))
		return ctx.Undefined()
	}))
	ctx.Globals().Set("console", console)
}

// jsValueToJSON converts a JavaScript value to a JSON string.
func jsValueToJSON(val *quickjs.Value) string {
	if val.IsString() {
		return val.String()
	}
	if val.IsObject() {
		// Convert JS object to Go map, then to JSON
		goVal := jsValueToGoPtr(val)
		data, err := json.Marshal(goVal)
		if err != nil {
			return "{}"
		}
		return string(data)
	}
	return val.String()
}

// jsValueToGoPtr converts a JavaScript value pointer to a Go value.
func jsValueToGoPtr(val *quickjs.Value) any {
	if val.IsNull() || val.IsUndefined() {
		return nil
	}
	if val.IsBool() {
		return val.Bool()
	}
	if val.IsNumber() {
		f := val.Float64()
		if f == float64(int64(f)) {
			return int64(f)
		}
		return f
	}
	if val.IsString() {
		return val.String()
	}
	if val.IsArray() {
		length := val.Get("length")
		defer length.Free()
		n := int(length.Int32())
		arr := make([]any, n)
		for i := 0; i < n; i++ {
			elem := val.GetIdx(int64(i))
			arr[i] = jsValueToGoPtr(elem)
			elem.Free()
		}
		return arr
	}
	if val.IsObject() {
		names, err := val.PropertyNames()
		if err != nil {
			return nil
		}
		obj := make(map[string]any)
		for _, name := range names {
			prop := val.Get(name)
			obj[name] = jsValueToGoPtr(prop)
			prop.Free()
		}
		return obj
	}
	return val.String()
}

// goValueToJS converts a Go value to a JavaScript value.
func goValueToJS(ctx *quickjs.Context, val any) *quickjs.Value {
	if val == nil {
		return ctx.Null()
	}
	switch v := val.(type) {
	case bool:
		return ctx.Bool(v)
	case int:
		return ctx.Int32(int32(v))
	case int32:
		return ctx.Int32(v)
	case int64:
		return ctx.Int64(v)
	case float64:
		return ctx.Float64(v)
	case string:
		return ctx.String(v)
	case []any:
		arr := ctx.NewObject()
		for i, elem := range v {
			arr.SetIdx(int64(i), goValueToJS(ctx, elem))
		}
		// Set length property so it behaves like an array
		arr.Set("length", ctx.Int32(int32(len(v))))
		return arr
	case map[string]any:
		obj := ctx.Object()
		for k, val := range v {
			obj.Set(k, goValueToJS(ctx, val))
		}
		return obj
	default:
		return ctx.String(fmt.Sprintf("%v", v))
	}
}

// jsValueToStringPtr converts a JavaScript value pointer to a string for comparison.
func jsValueToStringPtr(val *quickjs.Value) string {
	if val.IsNull() {
		return "null"
	}
	if val.IsUndefined() {
		return "undefined"
	}
	if val.IsBool() {
		if val.Bool() {
			return "true"
		}
		return "false"
	}
	if val.IsNumber() {
		f := val.Float64()
		if f == float64(int64(f)) {
			return fmt.Sprintf("%d", int64(f))
		}
		return fmt.Sprintf("%g", f)
	}
	return val.String()
}
