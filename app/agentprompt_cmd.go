package app

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// AgentPrompt contains AI-optimized documentation for a command.
// The structure is designed to help LLMs understand when and how to use each command.
//
// Design principles for effective LLM prompts:
// 1. Be explicit and specific - avoid ambiguity
// 2. Provide concrete examples - more effective than abstract descriptions
// 3. Use structured formats - easy to parse and reference
// 4. Include decision criteria - help the LLM know when to use each command
// 5. Anticipate common mistakes - document pitfalls upfront
// 6. Match verbosity to complexity - short prompts for simple commands
type AgentPrompt struct {
	Command  string // Command name
	Summary  string // One-line description for listing
	FullText string // Complete prompt with examples and guidance
}

// agentPrompts contains all command prompts, indexed by command name.
var agentPrompts = map[string]AgentPrompt{
	"inlinetest": {
		Command: "inlinetest",
		Summary: "Test HTTP routes against an in-memory database without starting a server. Primary tool for AI agents to verify route behavior.",
		FullText: `# inlinetest - Test Routes Without HTTP Server

## Purpose
Execute HTTP requests against your application's routes using an in-memory SQLite database. This command is specifically designed for AI agents to verify that routes work correctly after making code changes.

## When to Use
- After modifying a route handler, verify it still works
- After changing database queries, test the affected endpoints
- To validate authentication flows work correctly
- To test request/response formats without HTTP overhead
- When you need to run multiple sequential requests (login then access protected route)

## When NOT to Use
- For load testing or performance benchmarks (use actual HTTP server)
- When you need to test WebSocket or streaming responses
- When testing middleware that depends on real network conditions

## Quick Start
` + "```bash" + `
# Test a simple endpoint
go run main.go inlinetest /health

# Test with authentication
go run main.go inlinetest --with-user=test@example.com --with-session /auth/whoami
` + "```" + `

## Key Flags

| Flag | Purpose | Example |
|------|---------|---------|
| --with-user=EMAIL | Create test user with email | --with-user=test@example.com |
| --with-session | Add session cookie auth | --with-session |
| --with-api-key | Add Bearer token auth | --with-api-key |
| --admin | Make user an admin | --admin |
| -X METHOD | HTTP method | -X POST |
| -d BODY | Request body JSON | -d '{"name":"Test"}' |
| -f FILE | Read commands from file | -f tests.txt |
| -i | Interactive/stdin mode | -i |
| -r | Enable readline with tab completion and persistent history | -r |
| -s | Persist cookies across requests | -s |
| -p FILE | Persist session to file (survives restarts) | -p .session.json |
| -w | Watch for .go changes and restart | -w |
| --db | Use real database from DATABASE_CONFIG | --db |
| --init FILE | Run setup commands from file before interactive mode | --init setup.txt |
| -q | Quiet mode: only print failures | -q |
| -v | Show response headers | -v |

## Examples

### Test unauthenticated endpoint
` + "```bash" + `
go run main.go inlinetest /health
# Output: GET /health -> 200
# {"status":"healthy"}
` + "```" + `

### Test authenticated endpoint
` + "```bash" + `
go run main.go inlinetest --with-user=test@example.com --with-session /auth/whoami
# Output: GET /auth/whoami -> 200
# {"authenticated":true,"email":"test@example.com",...}
` + "```" + `

### Create a resource with POST
` + "```bash" + `
go run main.go inlinetest --with-user=test@example.com --with-session \
  -X POST -d '{"name":"Widget","price":9.99}' /api/items
# Output: POST /api/items -> 201
# {"id":1,"name":"Widget","price":9.99}
` + "```" + `

### Test admin-only endpoint
` + "```bash" + `
go run main.go inlinetest --with-user=admin@example.com --with-session --admin /admin/users
` + "```" + `

### Multi-step testing with file input
` + "```bash" + `
cat > /tmp/test.txt << 'EOF'
# Login flow test
/health
?assert status 200

/dev/create_and_assume?email=test@example.com
?assert status 200
?session

/auth/whoami
?assert status 200
?assert body test@example.com
EOF

go run main.go inlinetest -f /tmp/test.txt
` + "```" + `

### Interactive testing with session persistence
` + "```bash" + `
echo -e "/dev/create_and_assume?email=test@example.com\n/auth/whoami" | \
  go run main.go inlinetest -i -s
` + "```" + `

### Run test suite quietly (only show failures)
` + "```bash" + `
# When running many tests, use -q to only see failures
go run main.go inlinetest -q -f tests/auth.txt
go run main.go inlinetest -q -f tests/items.txt

# Combine with session persistence for multi-request tests
go run main.go inlinetest -q -s -f tests/full_workflow.txt
` + "```" + `

### Persist session to file with real database
` + "```bash" + `
# Use --db to connect to real database (sessions persist across restarts)
# Session cookies are saved to file and loaded on next run
go run main.go inlinetest --db -p .session.json /dev/create_and_assume?email=dev@example.com

# Later runs will load the session and it will work because the session is in the real DB
go run main.go inlinetest --db -p .session.json /auth/whoami
` + "```" + `

### Watch mode for development
` + "```bash" + `
# Auto-restart REPL when .go files change (great for iterative development)
# Use --db with -p for persistent sessions that survive restarts
go run main.go inlinetest -i -r -w --db -p .session.json
` + "```" + `

## Interactive Commands

When using -i or -f mode, these special commands are available:

| Command | Action |
|---------|--------|
| /path | GET request to /path |
| METHOD /path BODY | Request with method and optional JSON body |
| .term | Search routes containing "term" |
| ? | Show help |
| ?routes | List all routes |
| ?session | Show current cookies |
| ?response | Show last response |
| ?config | Show REPL configuration/launch parameters |
| ?run FILE | Run commands from file |
| ?assert status N | Assert status code equals N |
| ?assert body TEXT | Assert body contains TEXT |
| ?assert json .path VAL | Assert JSON field at path equals value |
| quit | Exit interactive mode |

## JSON Path Assertions

The ` + "`?assert json`" + ` command extracts values from JSON responses using dot notation:

` + "```bash" + `
# Assert a top-level field
?assert json .email "test@example.com"

# Assert a nested field
?assert json .user.name "John"

# Assert array element (0-indexed)
?assert json .items.0.name "First Item"

# Assert boolean values
?assert json .authenticated true
?assert json .is_admin false
` + "```" + `

## Common Patterns

### Verify a CRUD workflow
` + "```bash" + `
cat > /tmp/crud.txt << 'EOF'
# Create
POST /api/items {"name":"Test","price":5.00}
?assert status 201

# Read
GET /api/items/1
?assert status 200
?assert body Test

# Update
PUT /api/items/1 {"name":"Updated","price":10.00}
?assert status 200

# Delete
DELETE /api/items/1
?assert status 200

# Verify deleted
GET /api/items/1
?assert status 404
EOF

go run main.go inlinetest --with-user=test@example.com --with-session -f /tmp/crud.txt
` + "```" + `

### Test authentication flow
` + "```bash" + `
cat > /tmp/auth.txt << 'EOF'
# Unauthenticated should fail
/auth/whoami
?assert status 401

# Create session via dev endpoint
/dev/create_and_assume?email=test@example.com
?assert status 200
?session

# Now authenticated
/auth/whoami
?assert status 200
?assert body test@example.com
EOF

go run main.go inlinetest -f /tmp/auth.txt
` + "```" + `

## Troubleshooting

**"no path specified" error**
- Ensure the URL starts with /
- Check that you're passing the path as an argument, not a flag value

**"failed to run migrations" error**
- Check that the migrations/ directory exists
- Verify migration files are valid SQL

**Authentication not working**
- Use --with-session for cookie auth OR --with-api-key for Bearer token
- Ensure the user is created with --with-user first
- For session persistence across requests, use -s or -f (which implies -s)

**Route not found (404)**
- Run 'go run main.go routes' to see available routes
- Check if the route requires authentication
- Use .search in interactive mode to find routes

## AI Agent Tips

1. Always test after modifying route handlers
2. Use assertions (-f with ?assert) to make tests repeatable
3. For login flows, use -f with session persistence (enabled by default)
4. Check routes list before testing unfamiliar endpoints
5. Use ?response to debug unexpected behavior
6. The --admin flag is only needed for admin-protected routes
7. Use ?assert json .field "value" to verify specific JSON fields
8. Example tests are in inline_tests/ directory - run with: go run main.go inlinetest -f inline_tests/health.txt
`,
	},

	"jstest": {
		Command: "jstest",
		Summary: "Execute JavaScript test scripts against an in-memory database. Use for complex multi-step API tests with control flow.",
		FullText: `# jstest - JavaScript-Based API Testing

## Purpose
Execute JavaScript test scripts against your application's routes using an in-memory SQLite database. This command allows writing complex, multi-step API tests with full JavaScript control flow (loops, conditionals, variables).

## When to Use
- When you need to test complex workflows with multiple related API calls
- When you want to store response data in variables for later assertions
- When you need loops or conditionals in your test logic
- For integration tests that verify multiple endpoints work together
- When tests need to be more readable than inlinetest's line-by-line format

## When NOT to Use
- For simple single-request tests (use inlinetest instead)
- For quick endpoint verification during development (use inlinetest)
- When you need async/Promise support (QuickJS is synchronous)

## Quick Start
` + "```bash" + `
# Run a test script
go run main.go jstest tests/api_test.js

# Run with verbose output (shows request/response details)
go run main.go jstest -v tests/api_test.js

# Run with real database instead of in-memory
go run main.go jstest --db tests/integration.js

# Run multiple test files
go run main.go jstest tests/*.js
` + "```" + `

## Key Flags

| Flag | Purpose | Example |
|------|---------|---------|
| --db | Use real database from DATABASE_CONFIG | --db |
| -v | Verbose output (show HTTP details) | -v |
| -q | Quiet mode (only show failures) | -q |
| --migrations DIR | Path to migrations directory | --migrations ./db |

## JavaScript API

### HTTP Client

` + "```javascript" + `
// Make HTTP requests
var resp = client.get("/health")
var resp = client.post("/api/items", {name: "Widget", price: 9.99})
var resp = client.put("/api/items/1", {name: "Updated"})
var resp = client.delete("/api/items/1")

// Response object
resp.status      // HTTP status code (number)
resp.body        // Raw response body (string)
resp.json()      // Parse body as JSON object
resp.headers     // Headers object
` + "```" + `

### Authentication

` + "```javascript" + `
// Create users
client.createUser("test@example.com")           // Regular user
client.createUser("admin@example.com", true)    // Admin user

// Authenticate
client.login("test@example.com")                // Session cookie auth
client.loginWithApiKey("test@example.com")      // API key auth

// Clear authentication
client.logout()
` + "```" + `

### Assertions

` + "```javascript" + `
// Basic assertions
assert(condition, "message")                    // Fails if condition is falsy
assertEqual(a, b, "message")                    // Fails if a !== b
assertNotEqual(a, b, "message")                 // Fails if a === b

// HTTP response assertions
assertStatus(resp, 200)                         // Assert exact status
assertStatus(resp, 200, 201)                    // Assert one of multiple codes
assertContains(resp.body, "expected text")      // Assert body contains text
assertJSON(resp, ".field.path", "expected")     // Assert JSON field value

// Output
print("message")                                // Print to stdout
console.log("message")                          // Alias for print
` + "```" + `

## Example Test Script

` + "```javascript" + `
// tests/items_test.js

// Setup: create user and authenticate
client.createUser("test@example.com")
client.login("test@example.com")

// Test: Create an item
var resp = client.post("/api/items", {
    name: "Widget",
    price: 9.99
})
assertStatus(resp, 201)
var itemId = resp.json().id
print("Created item with id=" + itemId)

// Test: List items
resp = client.get("/api/items")
assertStatus(resp, 200)
var items = resp.json()
assert(items.length > 0, "Should have items")

// Test: Update item
resp = client.put("/api/items/" + itemId, {
    name: "Updated Widget"
})
assertStatus(resp, 200)

// Test: Delete item
resp = client.delete("/api/items/" + itemId)
assertStatus(resp, 200)

// Verify deletion
resp = client.get("/api/items/" + itemId)
assertStatus(resp, 404)

print("All tests passed!")
` + "```" + `

## Common Patterns

### Testing CRUD Operations

` + "```javascript" + `
client.createUser("test@example.com")
client.login("test@example.com")

// Create
var resp = client.post("/api/items", {name: "Test"})
assertStatus(resp, 201)
var id = resp.json().id

// Read
resp = client.get("/api/items/" + id)
assertStatus(resp, 200)
assertEqual(resp.json().name, "Test")

// Update
resp = client.put("/api/items/" + id, {name: "Updated"})
assertStatus(resp, 200)

// Delete
resp = client.delete("/api/items/" + id)
assertStatus(resp, 200)
` + "```" + `

### Testing Multiple Users

` + "```javascript" + `
// Create two users
client.createUser("alice@example.com")
client.createUser("bob@example.com")

// Alice creates an item
client.login("alice@example.com")
var resp = client.post("/api/items", {name: "Alice's Item"})
var aliceItemId = resp.json().id

// Bob should not see Alice's item (if items are user-scoped)
client.login("bob@example.com")
resp = client.get("/api/items/" + aliceItemId)
assertStatus(resp, 404)

print("User isolation test passed!")
` + "```" + `

### Testing Admin Features

` + "```javascript" + `
client.createUser("user@example.com")
client.createUser("admin@example.com", true)  // true = admin

// Regular user cannot access admin endpoint
client.login("user@example.com")
var resp = client.get("/admin/users")
assertStatus(resp, 403)

// Admin can access
client.login("admin@example.com")
resp = client.get("/admin/users")
assertStatus(resp, 200)

print("Admin access test passed!")
` + "```" + `

## Troubleshooting

**"createUser() requires an email argument"**
- Ensure you're passing a string: client.createUser("email@example.com")

**"user not found - call createUser() first"**
- You must call client.createUser(email) before client.login(email)

**"invalid JSON" error from resp.json()**
- Check that the response body is valid JSON
- Use resp.body to see the raw response

**Script not finding routes**
- Run 'go run main.go routes' to verify endpoints exist
- Check authentication - some routes require auth

## AI Agent Tips

1. Use jstest for complex workflows, inlinetest for quick checks
2. Each script runs with a fresh in-memory database
3. Variables persist throughout the script - store IDs for later use
4. Use print() liberally to show progress
5. Tests fail fast on first assertion failure
6. Check resp.body when debugging unexpected responses
7. No async/await - all calls are synchronous
`,
	},

	"verify": {
		Command: "verify",
		Summary: "Validate all registered SQL queries against the database schema. Run after schema changes or before commits.",
		FullText: `# verify - Validate SQL Queries

## Purpose
Check that all registered SQL queries are valid against the current database schema. This catches typos, missing columns, and schema mismatches before they cause runtime errors.

## When to Use
- After modifying database migrations
- After adding new queries with db.Register()
- Before committing code changes
- In CI/CD pipelines

## When NOT to Use
- This is a read-only check, never skip it before deployment

## Quick Start
` + "```bash" + `
go run main.go verify
# Output: all queries verified successfully
` + "```" + `

## Example Output

Success:
` + "```" + `
all queries verified successfully
` + "```" + `

Failure:
` + "```" + `
query verification failed: table "nonexistent" does not exist
` + "```" + `

## AI Agent Tips

1. Run verify after any migration changes
2. If verification fails, check recent query or schema changes
3. This command loads routes to discover queries, so route errors may appear
`,
	},

	"routes": {
		Command: "routes",
		Summary: "List all registered HTTP routes. Use to discover available endpoints and their authentication requirements.",
		FullText: `# routes - List HTTP Routes

## Purpose
Display all HTTP routes registered in the application, showing the method, path, authentication requirement, and handler function.

## When to Use
- To discover what endpoints are available
- To check if a route is registered correctly
- To verify authentication requirements
- To generate API documentation (with -md flag)

## Quick Start
` + "```bash" + `
go run main.go routes
` + "```" + `

## Key Flags

| Flag | Purpose |
|------|---------|
| -all | Include middleware internal routes |
| -md | Output as markdown to api.md file |

## Example Output
` + "```" + `
AUTH     METHOD   PATH                    HANDLER
----     ------   ----                    -------
PUBLIC   GET      /health                 app.createRouter.<closure>
PUBLIC   GET      /api/items/             items.(*Routes).list
AUTH     POST     /api/items/             items.(*Routes).create
AUTH     GET      /auth/whoami            auth.(*Routes).whoami
` + "```" + `

## AI Agent Tips

1. Check routes before using inlinetest to know what endpoints exist
2. AUTH column shows if authentication is required
3. Use -md to generate documentation for the user
`,
	},

	"schema": {
		Command: "schema",
		Summary: "Dump database schema as JSON including foreign keys and indexes. Use to understand table structures and relationships.",
		FullText: `# schema - Export Database Schema

## Purpose
Output the complete database schema as JSON, showing all tables, columns, foreign keys, and indexes. Works with both SQLite and PostgreSQL.

## When to Use
- To understand what tables and columns exist
- To discover foreign key relationships between tables
- To verify migrations created expected schema
- To document database structure

## Quick Start
` + "```bash" + `
go run main.go schema
go run main.go schema > schema.json  # SQL timing goes to stderr
` + "```" + `

## Example Output
` + "```json" + `
{
  "databases": {
    "primary": {
      "tables": [
        {
          "name": "items",
          "columns": [
            {"name": "id", "type": "INTEGER", "nullable": false, "primary_key": true},
            {"name": "name", "type": "TEXT", "nullable": false},
            {"name": "owner_id", "type": "INTEGER", "nullable": false}
          ],
          "foreign_keys": [
            {
              "column": "owner_id",
              "references_table": "users",
              "references_column": "id",
              "on_delete": "CASCADE",
              "on_update": "NO ACTION"
            }
          ],
          "indexes": [
            {
              "name": "idx_items_owner",
              "columns": ["owner_id"],
              "unique": false
            }
          ]
        }
      ]
    }
  }
}
` + "```" + `

## AI Agent Tips

1. Pipe to jq for specific queries: go run main.go schema 2>/dev/null | jq '.databases.primary.tables[].name'
2. Use before writing SQL queries to verify column names
3. Check foreign_keys to understand table relationships
4. Use 2>/dev/null to suppress SQL timing when piping to files
5. Works with both SQLite and PostgreSQL databases
`,
	},

	"openapi": {
		Command: "openapi",
		Summary: "Generate OpenAPI 3.0 specification from routes. Shows typed request/response schemas when available.",
		FullText: `# openapi - Generate OpenAPI Specification

## Purpose
Generate an OpenAPI 3.0 specification from registered routes. When routes use typed handlers (web.Post, web.Get, etc.), the spec includes request/response JSON schemas.

## When to Use
- To generate API documentation
- To understand request/response formats
- To validate API structure
- For client SDK generation

## Quick Start
` + "```bash" + `
go run main.go openapi > openapi.json
go run main.go openapi --title "My API" --version "2.0.0"
` + "```" + `

## Key Flags

| Flag | Purpose | Default |
|------|---------|---------|
| --title | API title in spec | app name |
| --version | API version | 1.0.0 |

## Example Output
` + "```json" + `
{
  "openapi": "3.0.3",
  "info": {"title": "examples", "version": "1.0.0"},
  "paths": {
    "/api/items": {
      "post": {
        "requestBody": {
          "content": {
            "application/json": {
              "schema": {"$ref": "#/components/schemas/CreateItemInput"}
            }
          }
        },
        "responses": {
          "200": {
            "content": {
              "application/json": {
                "schema": {"$ref": "#/components/schemas/CreateItemOutput"}
              }
            }
          }
        },
        "security": [{"cookieAuth": []}, {"bearerAuth": []}]
      }
    }
  },
  "components": {
    "schemas": {
      "CreateItemInput": {
        "type": "object",
        "properties": {
          "name": {"type": "string"},
          "price": {"type": "number"}
        },
        "required": ["name"]
      }
    }
  }
}
` + "```" + `

## Typed Routes

For routes to have schemas in the OpenAPI spec, they must use typed handlers:

` + "```go" + `
// Define input/output types
type CreateItemInput struct {
    Name  string  ` + "`" + `json:"name" validate:"required"` + "`" + `
    Price float64 ` + "`" + `json:"price" validate:"gte=0"` + "`" + `
}

type CreateItemOutput struct {
    ID      int    ` + "`" + `json:"id"` + "`" + `
    Message string ` + "`" + `json:"message"` + "`" + `
}

// Use typed handler
func (r *Routes) create(ctx context.Context, input CreateItemInput) (*CreateItemOutput, error) {
    // input is already validated
    return &CreateItemOutput{ID: 1, Message: "created"}, nil
}

// Register with web.Post (captures types for OpenAPI)
web.Post(router, "/api/items", r.create, 201)
` + "```" + `

## AI Agent Tips

1. Routes without typed handlers still appear but have no schema
2. Validation tags (validate:"required,min=1") are converted to JSON Schema constraints
3. Use 2>/dev/null to suppress SQL timing when piping
4. The security field shows which auth methods are required
`,
	},

	"env": {
		Command: "env",
		Summary: "Show environment variables and database configuration. Use to debug configuration issues.",
		FullText: `# env - Show Configuration

## Purpose
Display current environment variable values and database configuration. Helps debug configuration issues.

## When to Use
- To verify environment is configured correctly
- To check which databases are connected
- To debug "database not found" errors

## Quick Start
` + "```bash" + `
go run main.go env
` + "```" + `

## Example Output
` + "```" + `
Environment Variables:
======================

  HOST:              (not set)
  PORT:              (default: 8080)
  ENVIRONMENT:       dev
  LOG_LEVEL:         (default: info)

Database Configuration:
=======================

  Config file: databases.json

  Databases: 2
    - primary: sqlite3 (default)
    - analytics: postgres [read-only]
` + "```" + `

## AI Agent Tips

1. Check ENVIRONMENT=dev for development mode features
2. Verify database connections if queries fail
`,
	},

	"serve": {
		Command: "serve",
		Summary: "Start the HTTP server. Use when you need to test with actual HTTP requests or run in production.",
		FullText: `# serve - Run HTTP Server

## Purpose
Start the HTTP server to handle incoming requests. This is the main way to run the application.

## When to Use
- To run the application for manual testing
- For production deployment
- When you need actual HTTP (WebSocket, streaming, etc.)

## When NOT to Use
- For quick route testing - use inlinetest instead
- For development with auto-reload - use watch instead

## Quick Start
` + "```bash" + `
go run main.go serve
# Server starts on :8080
` + "```" + `

## Environment Variables

| Variable | Default | Purpose |
|----------|---------|---------|
| HOST | (all interfaces) | Bind address |
| PORT | 8080 | Listen port |
| ENVIRONMENT | (production) | Set to "dev" for dev mode |
| LOG_LEVEL | info | debug/info/warn/error |

## AI Agent Tips

1. Use inlinetest for quick verification instead of starting server
2. Check /health endpoint to verify server is running
3. Server verifies all queries on startup
`,
	},

	"watch": {
		Command: "watch",
		Summary: "Run server with auto-reload on file changes. Requires 'air' tool to be installed.",
		FullText: `# watch - Auto-reload Development Server

## Purpose
Run the application with automatic reloading when source files change. Delegates to the 'air' tool.

## When to Use
- During active development for fast iteration
- When making frequent code changes

## Prerequisites
` + "```bash" + `
go install github.com/air-verse/air@latest
` + "```" + `

## Quick Start
` + "```bash" + `
go run main.go watch
` + "```" + `

## AI Agent Tips

1. If air is not installed, the command will show installation instructions
2. For quick testing, use inlinetest instead of watch
`,
	},

	"findqueries": {
		Command: "findqueries",
		Summary: "Scan code for SQL queries not registered with db.Register(). Use in code review to ensure query tracking.",
		FullText: `# findqueries - Find Unregistered Queries

## Purpose
Scan source code for SQL queries that haven't been registered with the query tracking system. Helps ensure all database access is tracked.

## When to Use
- Before code review to catch untracked queries
- In CI/CD to enforce query registration
- After adding new database code

## Quick Start
` + "```bash" + `
go run main.go findqueries
go run main.go findqueries ./models  # scan specific directory
` + "```" + `

## Key Flags

| Flag | Purpose |
|------|---------|
| -exclude-tests | Don't scan test files |
| -show-sql | Show the actual SQL in output |

## Example Output
` + "```" + `
Found 2 unregistered queries:

File                 Line    Query
models/user.go       45      SELECT * FROM users...
queries/item.go      102     INSERT INTO items...
` + "```" + `

## AI Agent Tips

1. Exit code 1 means unregistered queries were found
2. Use -show-sql to see what queries need registration
`,
	},

	"testdocs": {
		Command: "testdocs",
		Summary: "Generate HTML documentation from Go tests. Requires -pkg flag with package pattern.",
		FullText: `# testdocs - Generate Test Documentation

## Purpose
Generate HTML documentation from Go test files, showing test names, descriptions, and results.

## When to Use
- To generate test reports for documentation
- In CI/CD to publish test results
- To understand test coverage

## Quick Start
` + "```bash" + `
go run main.go testdocs -pkg github.com/yourorg/yourapp/...
` + "```" + `

## Key Flags

| Flag | Purpose | Default |
|------|---------|---------|
| -pkg | Package pattern (required) | - |
| -o | Output file path | docs/tests.html |
| -root | Root directory to scan | . |
| -title | HTML page title | app name |

## AI Agent Tips

1. The -pkg flag is required
2. Output is HTML, useful for CI/CD artifacts
`,
	},
}

func cmdAgentPrompt(cfg Config, args []string) {
	fs := newFlagSet(cfg.Name, "agent-prompt", "Show AI-optimized documentation for commands.")
	showAll := fs.Bool("all", false, "Output all prompts (for embedding in documentation)")
	fs.Parse(args)

	remaining := fs.Args()

	if *showAll {
		// Output all prompts in a format suitable for CLAUDE.md
		printAllPrompts()
		return
	}

	if len(remaining) == 0 {
		// List all commands with summaries
		printPromptList()
		return
	}

	// Show specific command prompt
	cmdName := remaining[0]
	prompt, ok := agentPrompts[cmdName]
	if !ok {
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmdName)
		printPromptList()
		os.Exit(1)
	}

	fmt.Println(prompt.FullText)
}

func printPromptList() {
	fmt.Println("AI Agent Command Reference")
	fmt.Println("==========================")
	fmt.Println()
	fmt.Println("Use 'agent-prompt <command>' for detailed documentation.")
	fmt.Println("Use 'agent-prompt --all' to output all prompts.")
	fmt.Println()

	// Sort commands for consistent output
	var commands []string
	for cmd := range agentPrompts {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)

	// Group by priority
	primary := []string{"inlinetest", "jstest", "verify", "routes"}
	secondary := []string{"schema", "openapi", "env", "serve"}
	other := []string{}

	for _, cmd := range commands {
		isPrimary := false
		isSecondary := false
		for _, p := range primary {
			if cmd == p {
				isPrimary = true
				break
			}
		}
		for _, s := range secondary {
			if cmd == s {
				isSecondary = true
				break
			}
		}
		if !isPrimary && !isSecondary {
			other = append(other, cmd)
		}
	}

	fmt.Println("Primary Commands (use these most often):")
	for _, cmd := range primary {
		if p, ok := agentPrompts[cmd]; ok {
			fmt.Printf("  %-12s  %s\n", cmd, p.Summary)
		}
	}

	fmt.Println()
	fmt.Println("Utility Commands:")
	for _, cmd := range secondary {
		if p, ok := agentPrompts[cmd]; ok {
			fmt.Printf("  %-12s  %s\n", cmd, p.Summary)
		}
	}

	fmt.Println()
	fmt.Println("Other Commands:")
	sort.Strings(other)
	for _, cmd := range other {
		if p, ok := agentPrompts[cmd]; ok {
			fmt.Printf("  %-12s  %s\n", cmd, p.Summary)
		}
	}
}

func printAllPrompts() {
	fmt.Println("# tog CLI - AI Agent Reference")
	fmt.Println()
	fmt.Println("This document provides comprehensive documentation for AI agents working with tog applications.")
	fmt.Println()
	fmt.Println("## Quick Reference")
	fmt.Println()
	fmt.Println("| Command | Purpose |")
	fmt.Println("|---------|---------|")

	var commands []string
	for cmd := range agentPrompts {
		commands = append(commands, cmd)
	}
	sort.Strings(commands)

	for _, cmd := range commands {
		p := agentPrompts[cmd]
		// Truncate summary for table
		summary := p.Summary
		if len(summary) > 80 {
			summary = summary[:77] + "..."
		}
		fmt.Printf("| %s | %s |\n", cmd, summary)
	}

	fmt.Println()
	fmt.Println("---")
	fmt.Println()

	// Output each prompt with separator
	for _, cmd := range commands {
		p := agentPrompts[cmd]
		fmt.Println(strings.TrimSpace(p.FullText))
		fmt.Println()
		fmt.Println("---")
		fmt.Println()
	}
}
