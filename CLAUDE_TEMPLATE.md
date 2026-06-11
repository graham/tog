# [Project Name] Development Guidelines

> Copy this file to your tog-based project as `CLAUDE.md` and customize the sections below.

## Project Overview

[Describe your project here - what it does, its purpose, etc.]

**Tech Stack:** Go, tog framework, chi router, goose migrations

## Development Commands

```bash
make            # Default: db-reset + run (full restart)
make build      # Build the application
make run        # Build and run the server
make clean      # Remove database
make db-up      # Run goose migrations
make db-reset   # Clean + db-up
make test       # Run tests
make verify     # Verify queries against database schema
make routes     # List all registered routes
make check      # Run all checks (test, verify, routes)
```

Always use `go run main.go` instead of building a binary when testing changes.

## Testing Routes with inlinetest

The `inlinetest` command tests API routes without starting an HTTP server. Use this after making code changes.

### Basic Usage

```bash
# Test unauthenticated endpoint
go run main.go inlinetest /health

# Test authenticated endpoint
go run main.go inlinetest --with-user=test@example.com --with-session /api/endpoint

# POST request with JSON body
go run main.go inlinetest --with-user=test@example.com --with-session \
  -X POST -d '{"name":"Test","value":123}' /api/endpoint

# PUT/DELETE requests
go run main.go inlinetest --with-user=test@example.com --with-session \
  -X DELETE /api/endpoint/123
```

### Session Persistence

Use `-s` to persist sessions across requests:

```bash
# First request creates session
go run main.go inlinetest -s .session.json --with-user=test@example.com --with-session /api/items

# Subsequent requests reuse session (no --with-user needed)
go run main.go inlinetest -s .session.json /api/items
go run main.go inlinetest -s .session.json -X POST -d '{"name":"New"}' /api/items
```

### Interactive Mode

```bash
# Interactive REPL with tab completion
go run main.go inlinetest -s .session.json --with-user=test@example.com --with-session -i

# In REPL:
# GET /api/items
# POST /api/items {"name":"Test"}
# ? routes          (search routes)
# ? routes items    (search routes containing "items")
```

### Flags Reference

| Flag | Description |
|------|-------------|
| `--with-user=EMAIL` | Create/use user with this email |
| `--with-session` | Create session cookie authentication |
| `--with-api-key` | Create API key Bearer token authentication |
| `--admin` | Make the user an admin |
| `-X METHOD` | HTTP method (GET, POST, PUT, DELETE) |
| `-d BODY` | Request body as JSON string |
| `-s FILE` | Session file for persistence |
| `-v` | Verbose output (include response headers) |
| `-i` | Interactive mode (REPL) |

### Output Format

```
METHOD PATH -> STATUS_CODE
RESPONSE_BODY
```

## JavaScript Testing with jstest

For complex multi-step API tests with control flow, use the `jstest` command which executes JavaScript test scripts.

### Basic Usage

```bash
# Run a test script
go run main.go jstest tests/api_test.js

# Run with verbose output
go run main.go jstest -v tests/api_test.js

# Run with real database
go run main.go jstest --db tests/api_test.js

# Run multiple scripts
go run main.go jstest tests/*.js
```

### When to Use jstest vs inlinetest

Use `jstest` when:
- You need loops, conditionals, or variables in your tests
- You're testing complex workflows with multiple related API calls
- You want to store response data (like IDs) for later assertions

Use `inlinetest` when:
- You need quick single-request verification
- You're doing interactive debugging
- You want simpler test syntax

### JavaScript API

```javascript
// HTTP Client
var resp = client.get("/health")
var resp = client.post("/api/items", {name: "Widget", price: 9.99})
var resp = client.put("/api/items/1", {name: "Updated"})
var resp = client.delete("/api/items/1")

// Response object
resp.status      // HTTP status code
resp.body        // Raw body string
resp.json()      // Parsed JSON object

// Authentication
client.createUser("test@example.com")        // Create user
client.createUser("admin@example.com", true) // Create admin
client.login("test@example.com")             // Session cookie auth
client.loginWithApiKey("test@example.com")   // API key auth
client.logout()                              // Clear auth

// Assertions
assert(condition, "message")
assertEqual(a, b, "message")
assertNotEqual(a, b, "message")
assertStatus(resp, 200)
assertStatus(resp, 200, 201)  // Multiple acceptable codes
assertContains(resp.body, "text")
assertJSON(resp, ".path.to.field", "expected")

// Output
print("message")
console.log("message")
```

### Example Test Script

```javascript
// tests/items_test.js
client.createUser("test@example.com")
client.login("test@example.com")

// Create item
var resp = client.post("/api/items", {name: "Widget", price: 9.99})
assertStatus(resp, 201)
var itemId = resp.json().id

// Verify and delete
resp = client.get("/api/items/" + itemId)
assertStatus(resp, 200)

resp = client.delete("/api/items/" + itemId)
assertStatus(resp, 200)

resp = client.get("/api/items/" + itemId)
assertStatus(resp, 404)

print("All tests passed!")
```

### Flags Reference

| Flag | Description |
|------|-------------|
| `--db` | Use real database instead of in-memory |
| `-v` | Verbose output (show HTTP details) |
| `-q` | Quiet mode (only show failures) |
| `--migrations DIR` | Path to migrations directory |

## Route Patterns

### Directory Structure

```
routes/{resource}/
  types.go    # Struct definitions with db/json tags
  queries.go  # Query registration
  routes.go   # HTTP handlers
```

### Types (types.go)

```go
package myresource

type MyResource struct {
    ID        int    `db:"id" json:"id"`
    Name      string `db:"name" json:"name"`
    CreatedAt string `db:"created_at" json:"created_at"`
}
```

### Queries (queries.go)

```go
package myresource

import "github.com/graham/tog/db"

type Queries struct {
    List   *db.PreparedQuery[MyResource]
    GetByID *db.PreparedQuery[MyResource]
    Insert *db.PreparedExec
}

func RegisterQueries(database *db.DB) (*Queries, error) {
    list, err := db.Register[MyResource](database,
        `SELECT id, name, created_at FROM my_resources ORDER BY created_at DESC LIMIT $1`)
    if err != nil {
        return nil, err
    }
    list.Desc("List all resources with limit")

    getByID, err := db.Register[MyResource](database,
        `SELECT id, name, created_at FROM my_resources WHERE id = $1`)
    if err != nil {
        return nil, err
    }

    insert, err := db.RegisterExec(database,
        `INSERT INTO my_resources (name) VALUES ($1)`)
    if err != nil {
        return nil, err
    }

    return &Queries{List: list, GetByID: getByID, Insert: insert}, nil
}
```

### Routes (routes.go)

```go
package myresource

import (
    "net/http"
    "github.com/go-chi/chi/v5"
    "github.com/graham/tog/tools/routes"
    "github.com/graham/tog/web"
    "github.com/graham/tog/web/auth"
)

type Routes struct {
    queries *Queries
}

func NewRoutes(q *Queries) *Routes {
    return &Routes{queries: q}
}

func (r *Routes) Mount(prefix string) func(chi.Router) {
    return func(router chi.Router) {
        wrapped := routes.Wrap(router, prefix)
        wrapped.Get("/", r.list)
        wrapped.Post("/", r.create)
    }
}

func (r *Routes) list(w http.ResponseWriter, req *http.Request) {
    // Handler implementation
}
```

### Registering Routes (routes/routes.go)

```go
// In SetupRoutes function
myQueries, err := myresource.RegisterQueries(dbm.Default())
if err != nil {
    return nil, err
}
myRoutes := myresource.NewRoutes(myQueries)

// Under authenticated routes
r.Route("/api/myresource", myRoutes.Mount("/api/myresource"))
```

## Database Migrations

### Location

- PostgreSQL: `migrations/postgres/NNNNN_name.sql`
- SQLite: `migrations/sqlite3/NNNNN_name.sql`

### Format (goose)

```sql
-- +goose Up
-- +goose StatementBegin
CREATE TABLE my_table (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS my_table;
-- +goose StatementEnd
```

### SQLite Differences

- Use `INTEGER PRIMARY KEY` instead of `SERIAL PRIMARY KEY`
- Use `TEXT` for timestamps instead of `TIMESTAMP`
- Foreign keys require separate `FOREIGN KEY` clause

## Verification

Before committing changes:

```bash
make check
```

This runs tests, verifies all queries against the database schema, and lists routes.

## Common Patterns

### Get Current User

```go
user := auth.MustUserFromContext(req.Context())
```

### Parse Query Parameters

```go
limit := 100
if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
    if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
        limit = parsed
    }
}
```

### JSON Response

```go
web.WriteJSON(w, data)
```

### Error Response

```go
web.WriteAppError(w, req, web.ErrInternal("Error message", err))
web.WriteAppError(w, req, web.ErrNotFound("Resource not found", nil))
web.WriteAppError(w, req, web.ErrBadRequest("Invalid input", nil))
```

### Request Body Binding (Traditional)

```go
type createRequest struct {
    Name string `json:"name" validate:"required,min=1,max=100"`
}

var input createRequest
if !web.Bind(req, w, &input) {
    return
}
```

## Typed Routes (Recommended)

Use typed handlers for cleaner code and automatic OpenAPI schema generation.

### Defining Types

```go
// Input type with validation tags
type CreateItemInput struct {
    Name  string  `json:"name" validate:"required,min=1,max=100"`
    Price float64 `json:"price" validate:"gte=0"`
}

// Output type
type CreateItemOutput struct {
    ID      int    `json:"id"`
    Message string `json:"message"`
}
```

### Typed Handler

```go
// Clean signature: receives validated input, returns output or error
func (r *Routes) create(ctx context.Context, input CreateItemInput) (*CreateItemOutput, error) {
    user := auth.MustUserFromContext(ctx)

    result := r.queries.InsertItem.Exec(input.Name, input.Price, user.ID)
    if err := result.Err(); err != nil {
        return nil, web.ErrInternal("Failed to create item", err)
    }

    id, _ := result.LastInsertID()
    return &CreateItemOutput{ID: int(id), Message: "item created"}, nil
}
```

### Registration

```go
// Use web.Post/Get/Put/Delete - types are captured for OpenAPI
web.Post(router, "/api/items", r.create, http.StatusCreated)  // 201 status
web.Get(router, "/api/items", r.list)
web.Put(router, "/api/items/{id}", r.update)
web.Delete(router, "/api/items/{id}", r.delete)
```

### GET/DELETE Handlers (No Input Body)

```go
func (r *Routes) list(ctx context.Context) ([]Item, error) {
    user := auth.MustUserFromContext(ctx)
    items, err := r.queries.ListByOwner.ExecCtx(ctx, user.ID).All()
    if err != nil {
        return nil, web.ErrInternal("Failed to fetch items", err)
    }
    return items, nil
}

func (r *Routes) delete(ctx context.Context) (*DeleteResponse, error) {
    // chi.URLParam still works via the request in context
    // ... implementation
}
```

### Benefits

- **Cleaner handlers**: No manual `web.Bind()` or `web.WriteJSON()` boilerplate
- **Automatic validation**: Input is validated before your handler runs
- **OpenAPI generation**: Types are captured for `go run main.go openapi`
- **Type safety**: Compile-time checking of input/output types

## OpenAPI Generation

Generate OpenAPI 3.0 specification from your routes:

```bash
# Generate spec
go run main.go openapi > openapi.json

# With custom title and version
go run main.go openapi --title "My API" --version "2.0.0" > openapi.json
```

Routes using typed handlers (`web.Post`, `web.Get`, etc.) will have full request/response schemas. Traditional handlers still appear but without schemas.

## Database Schema Inspection

View database schema including foreign keys and indexes:

```bash
# Full schema as JSON
go run main.go schema > schema.json

# Query specific info with jq
go run main.go schema 2>/dev/null | jq '.databases.primary.tables[] | select(.name=="items")'
```

The schema output includes:
- Column names, types, nullability, defaults
- Foreign key relationships with `on_delete`/`on_update` rules
- Index definitions with column lists and uniqueness
