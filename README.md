# tog

A type-safe, validated Go web backend framework that catches errors at startup, not in production.

## Why tog?

- **Type-safe database queries** - Generic queries with compile-time type checking
- **Startup validation** - All SQL queries verified against your schema before the server starts
- **Explicit over magic** - No ORMs, no hidden behavior, just validated SQL with strong typing
- **Built-in auth** - Session management, scopes, and middleware ready to use
- **Production-ready errors** - Structured errors with dev/prod modes and tracking IDs
- **Testing-first** - Comprehensive testkit with database assertions for integration testing
- **Built for AI agents** - First-class tooling for AI coding agents to test routes, inspect schemas, and verify changes without a running server (see [AI Agent Features](#ai-agent-features))

## Quick Start

### Installation

```bash
go get github.com/graham/tog
```

### Minimal Example

```go
package main

import (
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/graham/tog/db"
    "github.com/graham/tog/web"
)

type Item struct {
    ID   int    `db:"id" json:"id"`
    Name string `db:"name" json:"name"`
}

func main() {
    // Open database
    database, _ := db.Open("sqlite3", ":memory:", db.SQLitePoolConfig())
    database.DB.Exec(`CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT)`)
    database.DB.Exec(`INSERT INTO items (name) VALUES ('Widget')`)

    // Register type-safe query
    getItems, _ := db.Register[Item](database, `SELECT id, name FROM items`)

    // Verify all queries at startup
    database.MustVerifyAll()

    // Build router
    r := chi.NewRouter()
    r.Get("/items", func(w http.ResponseWriter, r *http.Request) {
        items, _ := getItems.Exec().All()
        web.WriteJSON(w, items)
    })

    log.Println("Server running on :8080")
    http.ListenAndServe(":8080", r)
}
```

### Run the Example Application

```bash
cd examples
make          # Build and run
make check    # Run tests, verify queries, list routes
```

---

## Core Concepts

tog is built on three principles:

1. **Catch errors early** - Register and validate queries at startup, not when users hit your endpoints
2. **Type safety everywhere** - Generic queries return typed results, not `map[string]any`
3. **Explicit is better** - Write SQL, see exactly what runs, no magic query builders

---

## Database Layer

### Opening a Connection

```go
import "github.com/graham/tog/db"

// PostgreSQL with production settings
database, err := db.Open("postgres", "postgres://localhost/mydb", db.DefaultPoolConfig())

// SQLite with file-locking optimized settings
database, err := db.Open("sqlite3", "app.db", db.SQLitePoolConfig())

// Custom pool configuration
cfg := db.PoolConfig{
    MaxOpenConns:    25,
    MaxIdleConns:    5,
    ConnMaxLifetime: 30 * time.Minute,
    PingOnOpen:      true,
}
database, err := db.Open("postgres", dsn, cfg)
```

### Registering Queries

Register queries at startup to get type-safe, validated query objects:

```go
type User struct {
    ID    int    `db:"id" json:"id"`
    Email string `db:"email" json:"email"`
}

// SELECT queries - returns PreparedQuery[T]
getUser, err := db.Register[User](database,
    `SELECT id, email FROM users WHERE id = $1`)
getUser.Desc("Retrieves a user by ID")  // For documentation

// INSERT/UPDATE/DELETE - returns PreparedExec
insertUser, err := db.RegisterExec(database,
    `INSERT INTO users (email) VALUES ($1)`)

// Panic on error (useful at package level)
var getAllUsers = db.MustRegister[User](database,
    `SELECT id, email FROM users`)
```

### Executing Queries

```go
// Get first result - returns *T (nil if no rows, panics on db errors)
user := getUser.Exec(userID).First()
if user != nil {
    fmt.Println(user.Email)
}

// Get first result with error handling
user, err := getUser.Exec(userID).FirstE()

// Assert exactly 0 or 1 row - returns *T (nil if no rows, panics if >1)
user := getUser.Exec(userID).Unique()

// Assert exactly 1 row with error handling
user, err := getUser.Exec(userID).UniqueE()

// Get all results
users, err := getAllUsers.Exec().All()

// Iterate with Go 1.22+ range
for user, err := range getAllUsers.Exec().Rows() {
    if err != nil {
        break
    }
    fmt.Println(user.Email)
}

// Execute INSERT/UPDATE/DELETE
result := insertUser.Exec(email)
if err := result.Err(); err != nil {
    // handle error
}
id, _ := result.LastInsertID()
affected, _ := result.RowsAffected()
```

### Transactions

```go
err := database.Tx(func(tx *db.Tx) error {
    // All queries in this function use the same transaction
    result := insertUser.ExecTx(tx, email)
    if result.Err() != nil {
        return result.Err()  // Triggers rollback
    }

    user, err := getUser.ExecTx(tx, id).FirstE()
    if err != nil {
        return err  // Triggers rollback
    }

    return nil  // Commits transaction
})
// Panics are also caught and trigger rollback
```

### Query Verification

Verify all registered queries against your database schema at startup:

```go
// Returns error with details
if err := database.VerifyAll(); err != nil {
    log.Fatalf("Query verification failed: %v", err)
}

// Or panic on failure
database.MustVerifyAll()
```

Verification checks:
- Tables exist
- Columns exist and match struct tags
- SQL syntax is valid

### Multi-Database Support

```go
// databases.json
{
  "databases": {
    "primary": {
      "driver": "postgres",
      "dsn": "postgres://localhost/myapp"
    },
    "analytics": {
      "driver": "postgres",
      "dsn": "postgres://localhost/analytics",
      "read_only": true
    }
  },
  "default": "primary"
}
```

```go
// Load from config
manager, err := db.NewManagerFromFile("databases.json")
defer manager.Close()

// Access databases
primary := manager.Default()
analytics := manager.Get("analytics")

// Verify all queries across all databases
manager.VerifyAll()
```

### Schema Introspection

```go
// Get all tables
tables, err := database.SchemaInfo()

// Get specific table
table, err := database.TableSchema("users")
for _, col := range table.Columns {
    fmt.Printf("%s: %s (nullable: %v)\n", col.Name, col.Type, col.Nullable)
}
```

---

## Web Layer

### Context Middleware

Attach database access and request-scoped values to every request:

```go
router.Use(web.ContextMiddleware(dbManager))

// In handlers, access the database
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := web.GetContext(r.Context())
    db := ctx.DB()  // Get default database

    // Store request-scoped values
    ctx.Set("request_id", uuid.New())

    // Type-safe retrieval
    reqID, ok := web.GetTyped[uuid.UUID](ctx, "request_id")
}
```

### Related Entity Context

Store and retrieve related entities (projects, organizations, etc.) in request context:

```go
import "github.com/graham/tog/web/auth"

// In middleware - fetch and store entity once
func ProjectMiddleware(queries *Queries) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            projectID := chi.URLParam(r, "projectID")
            project, err := queries.GetProject.Exec(projectID).FirstE()
            if err != nil {
                web.WriteAppError(w, r, web.ErrNotFound("Project not found", err))
                return
            }

            // Store in context for downstream handlers
            auth.SetRelatedEntity(r.Context(), "project", &project)
            next.ServeHTTP(w, r)
        })
    }
}

// In handlers - retrieve without another DB query
func handler(w http.ResponseWriter, r *http.Request) {
    // Safe retrieval
    project, ok := auth.GetRelatedEntity[Project](r.Context(), "project")
    if !ok {
        // Handle missing entity
    }

    // Or panic if entity must exist (after middleware)
    project := auth.MustGetRelatedEntity[Project](r.Context(), "project")
}
```

This pattern is useful for:
- Multi-tenant applications (current organization/workspace)
- Resource hierarchies (project → tasks)
- Avoiding N+1 queries in nested routes

### JSON Helpers

```go
// Write JSON response (pretty-printed when ENVIRONMENT=dev)
web.WriteJSON(w, user)

// Log JSON to stdout
web.LogJSON(data, "response")
```

### Structured Error Handling

tog provides structured error handling that behaves differently in development and production:

```go
// Create errors with user-friendly messages
err := web.ErrNotFound("Item not found", originalErr)
err := web.ErrBadRequest("Invalid email format", nil)
err := web.ErrUnauthorized("Please log in", nil)
err := web.ErrForbidden("Admin access required", nil)
err := web.ErrInternal("Failed to save item", dbErr)

// Write error response
web.WriteAppError(w, r, err)
```

**Development mode** (`ENVIRONMENT=dev`) - includes debug info for developers:
```json
{
  "error": "Not Found",
  "message": "Item not found",
  "tracking_id": "a1b2c3d4e5f6",
  "debug": {
    "internal": "sql: no rows in result set",
    "file": "routes.go",
    "line": 50,
    "function": "items.(*Routes).getByID"
  }
}
```

**Production mode** - safe for end users, includes tracking ID for support:
```json
{
  "error": "Not Found",
  "message": "Item not found",
  "tracking_id": "a1b2c3d4e5f6"
}
```

The tracking ID uses the chi RequestID middleware when available, making it easy to correlate errors with logs.

**Handler pattern:**
```go
func (rt *Routes) getByID(w http.ResponseWriter, r *http.Request) {
    id := chi.URLParam(r, "id")
    item, err := rt.queries.GetByID.Exec(id).FirstE()
    if err != nil {
        web.WriteAppError(w, r, web.ErrNotFound("Item not found", err))
        return
    }
    web.WriteJSON(w, item)
}
```

**Available error constructors:**

| Function | Status | Use Case |
|----------|--------|----------|
| `ErrBadRequest(msg, err)` | 400 | Invalid input, malformed JSON |
| `ErrUnauthorized(msg, err)` | 401 | Not logged in, invalid token |
| `ErrForbidden(msg, err)` | 403 | Insufficient permissions |
| `ErrNotFound(msg, err)` | 404 | Resource doesn't exist |
| `ErrInternal(msg, err)` | 500 | Database errors, unexpected failures |

### Request Logging & Timing

tog provides a logging system with timing support and color-coded output:

```go
// Automatically added to serve command middleware stack
router.Use(web.LoggingMiddleware(web.DefaultLogger()))
```

**Log output format:**
```
[req-123] GET /api/items -> 200 (12.34ms)
  sql:list_items  3.21ms
  sql:get_user    1.02ms
  process_items   7.45ms
```

**Duration colors** (helps identify slow operations):
- Green: < 10ms
- Yellow: 10-100ms
- Red: > 100ms

**Configuration via environment:**

| Variable | Values | Default |
|----------|--------|---------|
| `LOG_LEVEL` | `silent`, `error`, `info`, `debug` | `info` |
| `NO_COLOR` | (any value) | (not set) |

- `silent` - No logging output
- `error` - Errors only
- `info` - Requests with status and duration
- `debug` - Requests + SQL timing + stopwatches

**Stopwatches for custom timing:**

```go
func handler(w http.ResponseWriter, r *http.Request) {
    ctx := web.GetContext(r.Context())

    // Time a section of code
    ctx.Start("process_items")
    defer ctx.Stop("process_items")

    // ... do work
}
```

**SQL query timing:**

Use `ExecCtx` to log query timing to the request context:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    webCtx := web.GetContext(r.Context())
    ctx := webCtx.WithQueryLogging(r.Context())

    // This query's timing will appear in logs
    items, err := queries.ListItems.ExecCtx(ctx).All()
}
```

### Request Binding & Validation

Bind and validate request bodies in one step:

```go
type CreateUserRequest struct {
    Name  string `json:"name" validate:"required,min=1,max=100"`
    Email string `json:"email" validate:"required,email"`
    Age   int    `json:"age" validate:"gte=0,lte=150"`
}

func createUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if !web.Bind(r, w, &req) {
        return  // Error response already written
    }
    // req is validated and ready to use
}
```

Validation error response:
```json
{
  "error": "Validation Error",
  "message": "Invalid request data",
  "fields": {
    "name": "name is required",
    "email": "email must be a valid email address"
  }
}
```

**Common validation tags:**

| Tag | Example | Description |
|-----|---------|-------------|
| `required` | `validate:"required"` | Field must be present |
| `min`, `max` | `validate:"min=1,max=100"` | Length or value bounds |
| `gte`, `lte` | `validate:"gte=0"` | Greater/less than or equal |
| `email` | `validate:"email"` | Valid email format |
| `url` | `validate:"url"` | Valid URL format |
| `oneof` | `validate:"oneof=draft published"` | One of listed values |
| `dive` | `validate:"dive,min=1"` | Validate slice elements |

**Custom validators:**

```go
web.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
    return regexp.MustCompile(`^[a-z0-9-]+$`).MatchString(fl.Field().String())
})

type Post struct {
    Slug string `validate:"required,slug"`
}
```

**Without validation:**

```go
// Just decode JSON, no validation
var req MyRequest
err := web.BindJSON(r, &req)
```

---

## Authentication

### Setup

```go
import "github.com/graham/tog/web/auth"

// Register auth queries
queries, err := auth.RegisterQueries(database)

// Create auth routes
authRoutes := auth.NewRoutes(queries, devMode)
router.Route("/auth", authRoutes.Mount())
```

Required database tables:
```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    is_admin INTEGER DEFAULT 0,
    is_active INTEGER DEFAULT 1
);

CREATE TABLE sessions (
    id INTEGER PRIMARY KEY,
    key_value TEXT UNIQUE NOT NULL,
    key_type TEXT DEFAULT 'session',
    scopes TEXT DEFAULT '',
    created_at INTEGER,
    expires_at INTEGER DEFAULT 0,
    is_active INTEGER DEFAULT 1,
    for_user INTEGER NOT NULL
);
```

### Authentication Middleware

```go
// Require authentication - returns 401 if not authenticated
router.Group(func(r chi.Router) {
    r.Use(auth.RequiresAuth(queries))
    r.Get("/profile", profileHandler)
})

// Optional authentication - allows anonymous access
router.Group(func(r chi.Router) {
    r.Use(auth.OptionalAuth(queries))
    r.Get("/items", listItems)  // Works with or without user
})
```

### Accessing User and Session

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // Safe access (returns ok=false if not authenticated)
    user, ok := auth.UserFromContext(r.Context())
    session, ok := auth.SessionFromContext(r.Context())

    // Panic if not authenticated (use after RequiresAuth)
    user := auth.MustUserFromContext(r.Context())
    session := auth.MustSessionFromContext(r.Context())

    // User helper methods
    if user.IsAdminUser() { }
    if user.IsActiveUser() { }

    // Session helper methods
    if session.IsExpired() { }
    if session.HasScope("write") { }
    scopes := session.GetScopes()  // []string
}
```

### Scope-Based Authorization

```go
// Require specific scope
router.Use(auth.RequiresScope("admin"))

// Require any of these scopes
router.Use(auth.RequiresAnyScope("read", "write"))

// Require all of these scopes
router.Use(auth.RequiresAllScopes("read", "write", "delete"))

// Require admin user
router.Use(auth.RequiresAdmin())
```

### Auth Endpoints

| Endpoint | Method | Auth | Description |
|----------|--------|------|-------------|
| `/auth/whoami` | GET | Required | Returns current user info |
| `/auth/logout` | POST | Required | Invalidates current session |
| `/auth/logout-all` | POST | Required | Invalidates all user sessions |
| `/auth/assume` | GET | None | Dev-only: Login as any user by email |

### Creating Sessions Programmatically

```go
// After OAuth login, password verification, etc.
sessionKey, err := authRoutes.CreateSession(w, userID, "read,write", 24*time.Hour)

// Create API key (no expiration)
apiKey, err := authRoutes.CreateAPIKey(userID, "read")
```

### Token Prefixes

Sessions use prefixes for easy identification:
- `sess_` - Session tokens
- `key_` - API keys

---

## Testing

### Test App Setup

```go
import "github.com/graham/tog/testkit"

func TestMyAPI(t *testing.T) {
    // Create test app with in-memory database
    app := testkit.NewTestApp(t, "../../migrations").
        WithRouter(func(dbm *db.Manager) (chi.Router, error) {
            return routes.NewRouter(dbm)
        })

    // Make requests
    resp := app.Request("GET", "/health", nil)
    if resp.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.Code)
    }
}
```

### Authentication Testing

```go
// Create users
admin := app.CreateUser("admin@example.com", true)   // isAdmin=true
user := app.CreateUser("user@example.com", false)

// Or login as existing user (from seed data)
admin := app.LoginAs("admin@example.com")

// Make authenticated requests
resp := admin.Request("GET", "/api/items", nil)
resp := user.Request("POST", "/api/items", strings.NewReader(body))
```

### Database Seeding

```go
app := testkit.NewTestApp(t, "../../migrations").
    Seed(func(database *db.DB) error {
        _, err := database.DB.Exec(`INSERT INTO items (name) VALUES ('Test')`)
        return err
    }).
    WithRouter(routerFactory)
```

### Query Helpers

```go
// Execute query and get results (fails test on error)
items := testkit.Query[Item](app, "SELECT * FROM items")
item := testkit.QueryOne[Item](app, "SELECT * FROM items WHERE id = $1", 1)

// Execute command
testkit.Exec(app, "DELETE FROM items WHERE id = $1", 1)
```

### Database Assertions

Verify database state after mutations:

```go
func TestCreateItem(t *testing.T) {
    app := testkit.NewTestApp(t, "../../migrations").
        WithLoadRoutes(routes.LoadRoutes)

    admin := app.LoginAs("admin@example.com")

    // Create item via API
    resp := admin.Request("POST", "/api/items",
        strings.NewReader(`{"name":"Widget","price":9.99}`))

    if resp.Code != http.StatusCreated {
        t.Fatalf("expected 201, got %d", resp.Code)
    }

    // Verify database was updated correctly
    app.AssertRowExists(t, "items", map[string]any{
        "name":     "Widget",
        "owner_id": 1,  // admin user ID
    })

    app.AssertRowValue(t, "items", map[string]any{"name": "Widget"}, "price", 9.99)
}

func TestDeleteItem(t *testing.T) {
    app := testkit.NewTestApp(t, "../../migrations").
        WithLoadRoutes(routes.LoadRoutes)

    admin := app.LoginAs("admin@example.com")

    // Delete item
    resp := admin.Request("DELETE", "/api/items/1", nil)

    // Verify item no longer exists
    app.AssertRowNotExists(t, "items", map[string]any{"id": 1})
}
```

**Available assertions:**

| Method | Description |
|--------|-------------|
| `AssertRowExists(t, table, where)` | Verify at least one matching row exists |
| `AssertRowNotExists(t, table, where)` | Verify no matching rows exist |
| `AssertRowCount(t, table, where, n)` | Verify exact count of matching rows |
| `AssertRowValue(t, table, where, col, val)` | Verify a specific column value |

**Raw SQL for custom assertions:**

```go
// Single row query
var count int
app.QueryRow(t, "SELECT COUNT(*) FROM items WHERE price > ?", 10.0).Scan(&count)

// Multiple rows
rows, err := app.QueryRows(t, "SELECT id, name FROM items")
defer rows.Close()

// Execute SQL directly
app.Exec(t, "UPDATE items SET price = price * 1.1 WHERE owner_id = ?", 1)
```

---

## AI Agent Features

tog is designed to be operated by AI coding agents as well as humans. Agents work best with fast feedback loops, machine-readable output, and self-describing tools — so tog ships several features built explicitly for them. Everything below works from the command line with no running server and no browser.

### agent-prompt - Self-Describing CLI

Every command ships with documentation written for LLM consumption: explicit "when to use" / "when not to use" decision criteria, concrete examples, common pitfalls, and agent-specific tips.

```bash
go run . agent-prompt              # List all commands with one-line summaries
go run . agent-prompt inlinetest   # Full documentation for one command
go run . agent-prompt --all        # Dump all prompts (embed in CLAUDE.md or similar)
```

An agent that has never seen a tog application can bootstrap itself by reading these prompts instead of the source.

### inlinetest - Verify Routes Without a Server

Executes HTTP requests directly against the application's router using an in-memory SQLite database — no server process, no port, no cleanup. An agent can verify a route change in a single command, including authenticated flows:

```bash
# Creates the user, the session, runs migrations, executes the request
go run . inlinetest --with-user=test@example.com --with-session /auth/whoami
```

Output is deterministic and easy to parse (`METHOD PATH -> STATUS` followed by the body). Test files support assertions for repeatable verification:

```
POST /api/items {"name":"Widget","price":9.99}
?assert status 201
?assert json .name "Widget"
```

```bash
go run . inlinetest -f tests/items.txt -q   # -q prints only failures
```

See [inlinetest](#inlinetest---ai-agent-testing-interface) under CLI Tools for the full flag reference.

### jstest - Scripted Multi-Step API Tests

For workflows that need control flow — loops, conditionals, captured IDs — agents can write JavaScript test scripts instead of chaining shell commands. Scripts run against an in-memory database with a synchronous HTTP client:

```javascript
client.createUser("test@example.com")
client.login("test@example.com")

var resp = client.post("/api/items", {name: "Widget", price: 9.99})
assertStatus(resp, 201)
var id = resp.json().id

resp = client.delete("/api/items/" + id)
assertStatus(resp, 200)
```

```bash
go run . jstest tests/items_test.js       # Fresh in-memory DB per script
go run . jstest --db tests/integration.js # Or against the real database
```

The API surface is small and predictable: `client.get/post/put/delete`, `client.createUser/login/loginWithApiKey/logout`, and assertion helpers (`assertStatus`, `assertJSON`, `assertContains`, `assertEqual`). Tests fail fast on the first failed assertion.

### Machine-Readable Introspection

Agents shouldn't have to grep source code to learn what an application looks like. Every structural fact is available as JSON or structured text:

| Command | What it answers |
|---------|----------------|
| `go run . routes` (also `-md`) | What endpoints exist, and which require auth? |
| `go run . schema` | What tables, columns, foreign keys, and indexes exist? |
| `go run . openapi` | What are the request/response schemas? (full JSON Schema for typed routes) |
| `go run . verify` | Are all registered SQL queries valid against the current schema? |
| `go run . findqueries` | Is any SQL bypassing query registration? (exit code 1 if so) |
| `go run . env` | How is the environment and database configured? |

SQL timing output goes to stderr, so stdout is always clean for piping:

```bash
go run . schema 2>/dev/null | jq '.databases.primary.tables[].name'
```

The same information is served over HTTP for running applications: `/docs/routes/json`, `/docs/queries/json`, and `/schema/{db}/{table}`.

### Dev Routes - Instant Authentication

With `ENVIRONMENT=dev`, the optional dev routes let an agent (or a test script) obtain an authenticated session with a single GET request — no password setup, no email verification:

```
GET /dev/create_and_assume?email=test@example.com   # Create user + session cookie
GET /dev/assume?email=existing@example.com          # Session for an existing user
GET /dev/schema/{db}/{table}                        # Schema inspection over HTTP
```

These are mounted explicitly and only in dev environments — they never exist in production builds unless you mount them.

### CLAUDE_TEMPLATE.md - Project Onboarding for Agents

The repo includes [CLAUDE_TEMPLATE.md](CLAUDE_TEMPLATE.md), a ready-made `CLAUDE.md` for projects built on tog. Copy it into your project and customize it; it teaches an agent the development commands, the inlinetest/jstest workflows, and the typed-route conventions so it can work productively from the first prompt.

### Startup Validation as a Feedback Loop

Because every query is verified against the schema at startup (and via `go run . verify`), an agent gets immediate, specific errors after changing SQL or migrations — "column X does not exist in table Y" at compile-and-verify time, instead of a runtime 500 discovered through testing.

---

## CLI Tools

### verify - Query Validation

Validates all registered queries against the database schema:

```bash
go run . verify
```

### routes - Route Documentation

Lists all registered routes with auth status:

```bash
go run . routes          # Table format
go run . routes -md      # Markdown
go run . routes -all     # Include middleware internal routes
```

JSON route data is available from a running application at `/docs/routes/json`.

Output:
```
AUTH    METHOD  PATH              HANDLER
----    ------  ----              -------
PUBLIC  GET     /health           <closure>
AUTH    GET     /api/items        items.(*Routes).list
AUTH    POST    /api/items        items.(*Routes).create
```

### testdocs - Test Documentation

Generates HTML documentation from test files:

```bash
go run . testdocs -o docs/tests.html
```

### findqueries - Find Unregistered SQL

Scans code for SQL queries that bypass registration:

```bash
go run . findqueries .
```

### inlinetest - AI Agent Testing Interface

**This command is specifically designed for AI agents** to test routes without running an HTTP server. It creates an in-memory database, runs migrations, optionally creates authenticated users, and executes HTTP requests.

```bash
# Simple unauthenticated request
./app inlinetest /health

# Create user and session, then request
./app inlinetest --with-user=test@example.com --with-session /auth/whoami

# Create admin user with API key
./app inlinetest --with-user=admin@example.com --with-api-key --admin /api/items

# POST request with JSON body
./app inlinetest --with-user=test@example.com --with-session \
  -X POST -d '{"name":"Widget","price":9.99}' /api/items

# Execute multiple requests from stdin
echo -e "GET /api/items\nPOST /api/items {\"name\":\"Test\"}\nGET /api/items" | \
  ./app inlinetest --with-user=test@example.com --with-session -i
```

**Flags:**

| Flag | Description |
|------|-------------|
| `--with-user=EMAIL` | Create a user with this email |
| `--with-session` | Create a session cookie for the user |
| `--with-api-key` | Create an API key (Bearer token) for the user |
| `--admin` | Make the user an admin |
| `-X METHOD` | HTTP method (default: GET) |
| `-d BODY` | Request body (JSON) |
| `-i` | Read requests from stdin (format: `METHOD URL [BODY]`) |
| `-v` | Verbose output (include response headers) |
| `--migrations=PATH` | Path to migrations directory (default: migrations) |

**Stdin format:**
```
GET /api/items
POST /api/items {"name":"Widget","price":9.99}
DELETE /api/items/1
# Lines starting with # are ignored
```

**Output format:**
```
METHOD PATH -> STATUS_CODE
RESPONSE_BODY
```

This enables AI agents to:
- Verify routes work correctly after code changes
- Test authentication flows without manual server startup
- Execute integration tests in a single command
- Batch multiple requests for efficient validation

---

## Documentation Endpoints

### Route Documentation

```go
router.Route("/docs/routes", web.RoutesDocHandler(router, web.RoutesDocConfig{
    Title: "My API",
    Descriptions: map[string]string{
        "GET /api/items": "List all items",
        "POST /api/items": "Create a new item",
    },
}))
```

Endpoints:
- `GET /docs/routes/` - HTML documentation
- `GET /docs/routes/json` - JSON format

### Query Documentation

```go
router.Route("/docs/queries", web.QueriesDocHandler(database))
```

Endpoints:
- `GET /docs/queries/` - Searchable HTML with SQL highlighting
- `GET /docs/queries/json` - JSON format

### Schema Documentation

```go
router.Route("/schema", web.SchemaRoutes(database))
```

Endpoints:
- `GET /schema/` - All tables
- `GET /schema/{name}` - Specific table

---

## Project Structure

Recommended layout:

```
myapp/
├── main.go                 # CLI commands (serve, verify, routes, etc.)
├── databases.json          # Database configuration
├── Makefile               # Build targets
├── routes/
│   ├── router.go          # Main router factory
│   └── items/
│       ├── types.go       # Data types with db/json tags
│       ├── queries.go     # Query registration
│       ├── routes.go      # HTTP handlers
│       └── routes_test.go # Integration tests
├── migrations/
│   └── sqlite3/
│       └── 00001_init.sql # Goose migrations
└── docs/
    └── tests.html         # Generated test docs
```

### Makefile Targets

```makefile
.PHONY: build serve test verify routes check

build:
	go build -o app .

serve: build
	./app serve

test:
	go test ./...

verify:
	go run . verify

routes:
	go run . routes

check: test verify routes
	@echo "All checks passed"
```

---

## Complete Example

```go
package main

import (
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/go-chi/chi/v5/middleware"
    "github.com/graham/tog/db"
    "github.com/graham/tog/web"
    "github.com/graham/tog/web/auth"
)

// Types
type Item struct {
    ID      int     `db:"id" json:"id"`
    Name    string  `db:"name" json:"name"`
    Price   float64 `db:"price" json:"price"`
    OwnerID int     `db:"owner_id" json:"owner_id"`
}

type CreateItemRequest struct {
    Name  string  `json:"name" validate:"required,min=1,max=100"`
    Price float64 `json:"price" validate:"gte=0"`
}

// Queries
type Queries struct {
    ListByOwner *db.PreparedQuery[Item]
    InsertItem  *db.PreparedExec
}

func RegisterQueries(database *db.DB) (*Queries, error) {
    list, err := db.Register[Item](database,
        `SELECT id, name, price, owner_id FROM items WHERE owner_id = $1`)
    if err != nil {
        return nil, err
    }

    insert, err := db.RegisterExec(database,
        `INSERT INTO items (name, price, owner_id) VALUES ($1, $2, $3)`)
    if err != nil {
        return nil, err
    }

    return &Queries{ListByOwner: list, InsertItem: insert}, nil
}

// Handlers
type Routes struct {
    queries *Queries
}

func (rt *Routes) list(w http.ResponseWriter, r *http.Request) {
    user := auth.MustUserFromContext(r.Context())
    items, err := rt.queries.ListByOwner.Exec(user.ID).All()
    if err != nil {
        web.WriteAppError(w, r, web.ErrInternal("Failed to fetch items", err))
        return
    }
    web.WriteJSON(w, items)
}

func (rt *Routes) create(w http.ResponseWriter, r *http.Request) {
    user := auth.MustUserFromContext(r.Context())

    var req CreateItemRequest
    if !web.Bind(r, w, &req) {
        return
    }

    result := rt.queries.InsertItem.Exec(req.Name, req.Price, user.ID)
    if err := result.Err(); err != nil {
        web.WriteAppError(w, r, web.ErrInternal("Failed to create item", err))
        return
    }

    id, _ := result.LastInsertID()
    w.WriteHeader(http.StatusCreated)
    web.WriteJSON(w, map[string]any{"id": id})
}

func main() {
    // Setup database
    database, _ := db.Open("sqlite3", "app.db", db.SQLitePoolConfig())

    // Register queries
    queries, _ := RegisterQueries(database)
    authQueries, _ := auth.RegisterQueries(database)

    // Verify at startup
    database.MustVerifyAll()

    // Build router
    r := chi.NewRouter()
    r.Use(middleware.Logger)
    r.Use(middleware.Recoverer)

    // Auth routes
    authRoutes := auth.NewRoutes(authQueries, true)
    r.Route("/auth", authRoutes.Mount())

    // Protected API routes
    routes := &Routes{queries: queries}
    r.Group(func(r chi.Router) {
        r.Use(auth.RequiresAuth(authQueries))
        r.Get("/api/items", routes.list)
        r.Post("/api/items", routes.create)
    })

    log.Println("Server running on :8080")
    http.ListenAndServe(":8080", r)
}
```

---

## Dependencies

tog builds on proven, stable libraries:

- [chi](https://github.com/go-chi/chi) - HTTP router
- [sqlx](https://github.com/jmoiron/sqlx) - Database extensions
- [goose](https://github.com/pressly/goose) - Migrations
- [validator](https://github.com/go-playground/validator) - Struct validation

## License

MIT
