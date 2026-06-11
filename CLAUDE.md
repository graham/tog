# tog Development Guidelines

## Example Application

The example application in `examples/` must always build and run successfully.

### Running from scratch

```bash
cd examples
make        # Removes DB, runs migrations, starts server
```

Or from project root:
```bash
make -C examples
```

### Available targets

```bash
make            # Default: db-reset + run
make build      # Build the application
make run        # Build and run the server
make clean      # Remove database
make db-up      # Run goose migrations
make db-reset   # Clean + db-up
make test       # Run tests
make testdocs   # Generate test documentation (outputs to docs/tests.html)
make verify     # Verify queries against database
make routes     # List registered routes
make check      # Run all checks (test, verify, routes)
```

### Verification

Before committing changes, ensure the example works:
```bash
cd examples && make check
```

This runs tests, verifies queries, and lists routes to ensure everything is working.

## Running Examples

**Always use `go run main.go` instead of building a binary** when running the examples application, unless explicitly testing binary compilation. This ensures you're testing the latest code changes.

```bash
cd examples

# Run the server
go run main.go serve

# Run inlinetest
go run main.go inlinetest /health

# Run other commands
go run main.go routes
go run main.go verify
```

## AI Agent Testing with inlinetest

**The `inlinetest` command is specifically designed for AI agents** to verify routes work correctly without starting an HTTP server. Use this command to test API endpoints after making code changes.

### Quick Reference

```bash
cd examples

# Test health endpoint (no auth needed)
go run main.go inlinetest /health

# Test authenticated endpoint (creates user + session automatically)
go run main.go inlinetest --with-user=test@example.com --with-session /auth/whoami

# Test creating an item
go run main.go inlinetest --with-user=test@example.com --with-session \
  -X POST -d '{"name":"Widget","price":9.99}' /api/items

# Test as admin user
go run main.go inlinetest --with-user=admin@example.com --with-session --admin /api/items

# Run multiple requests in sequence
echo -e "GET /api/items\nPOST /api/items {\"name\":\"Test\",\"price\":5.00}\nGET /api/items" | \
  go run main.go inlinetest --with-user=test@example.com --with-session -i
```

### When to Use

Use `inlinetest` when:
- You've modified a route handler and want to verify it works
- You need to test authentication flows
- You want to verify request/response formats
- You're debugging an endpoint without running the full server

### Output Format

```
METHOD PATH -> STATUS_CODE
RESPONSE_BODY
```

Example:
```
GET /auth/whoami -> 200
{
  "authenticated": true,
  "email": "test@example.com",
  "id": 3,
  "is_admin": false
}
```

### Flags

| Flag | Description |
|------|-------------|
| `--with-user=EMAIL` | Create a user with this email |
| `--with-session` | Create a session (cookie auth) |
| `--with-api-key` | Create an API key (Bearer token auth) |
| `--admin` | Make the user an admin |
| `-X METHOD` | HTTP method (GET, POST, PUT, DELETE) |
| `-d BODY` | Request body JSON |
| `-i` | Read multiple requests from stdin |
| `-v` | Verbose (include response headers) |

## JavaScript Testing with jstest

For complex multi-step API tests with control flow, use the `jstest` command which executes JavaScript test scripts.

### Quick Reference

```bash
cd examples

# Run a test script
go run main.go jstest tests/items_test.js

# Run with verbose output
go run main.go jstest -v tests/items_test.js

# Run with real database
go run main.go jstest --db tests/items_test.js
```

### When to Use

Use `jstest` when:
- You need loops, conditionals, or variables in your tests
- You're testing complex workflows with multiple related API calls
- You want to store response data (like IDs) for later assertions
- Tests need to be more readable than line-by-line inlinetest format

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

// Verify item exists
resp = client.get("/api/items/" + itemId)
assertStatus(resp, 200)

// Delete and verify
resp = client.delete("/api/items/" + itemId)
assertStatus(resp, 200)

resp = client.get("/api/items/" + itemId)
assertStatus(resp, 404)

print("All tests passed!")
```

### Flags

| Flag | Description |
|------|-------------|
| `--db` | Use real database instead of in-memory |
| `-v` | Verbose output (show HTTP details) |
| `-q` | Quiet mode (only show failures) |
| `--migrations DIR` | Path to migrations directory |

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
```

### Benefits

- **Cleaner handlers**: No manual `web.Bind()` or `web.WriteJSON()` boilerplate
- **Automatic validation**: Input is validated before your handler runs
- **OpenAPI generation**: Types are captured for `go run main.go openapi`
- **Type safety**: Compile-time checking of input/output types

## OpenAPI Generation

Generate OpenAPI 3.0 specification from your routes:

```bash
cd examples

# Generate spec
go run main.go openapi > openapi.json

# With custom title and version
go run main.go openapi --title "My API" --version "2.0.0" > openapi.json
```

Routes using typed handlers (`web.Post`, `web.Get`, etc.) will have full request/response schemas. Traditional handlers still appear but without schemas.

## Database Schema Inspection

View database schema including foreign keys and indexes:

```bash
cd examples

# Full schema as JSON
go run main.go schema > schema.json

# Query specific info with jq
go run main.go schema 2>/dev/null | jq '.databases.primary.tables[] | select(.name=="items")'
```

The schema output includes:
- Column names, types, nullability, defaults
- Foreign key relationships with `on_delete`/`on_update` rules
- Index definitions with column lists and uniqueness

Note: SQL timing output goes to stderr, so use `2>/dev/null` to suppress it when piping to files.
