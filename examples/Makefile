.PHONY: all build serve watch clean db-up db-reset test testdocs verify routes findqueries check generate-sqlite

# Database file - matches databases.json
DB_FILE := example.db
BIN := examples

# Default: build only
all: build

# Build the application
build:
	@echo "Building..."
	@go build -o $(BIN) .

# Generate SQLite migrations from PostgreSQL source
generate-sqlite:
	@echo "Generating SQLite migrations from PostgreSQL..."
	@go run ../tools/pg2sqlite/cmd/main.go migrations/postgres migrations/sqlite3

# Run the webserver (builds first, resets DB)
serve: build db-reset
	@echo "Starting webserver on :8080..."
	@./$(BIN) serve

# Watch for changes and auto-reload (requires: go install github.com/air-verse/air@latest)
watch: db-reset
	@go run . watch

# Remove database for clean slate
clean:
	@rm -f $(DB_FILE) $(BIN)

# Apply goose migrations (generates SQLite first)
db-up: generate-sqlite
	@echo "Running goose migrations..."
	@goose up

# Reset database (remove + migrate)
db-reset: clean db-up

# Run tests (includes core library)
test:
	@echo "Running tests..."
	@go test github.com/graham/tog/...

# Generate test documentation
testdocs:
	@echo "Generating test documentation..."
	@go run . testdocs -o docs/tests.html

# Verify queries against database
verify: db-reset
	@echo "Verifying queries..."
	@go run . verify -config=databases.json

# List routes
routes: db-reset
	@echo "Listing routes..."
	@go run . routes

# Find unregistered queries
findqueries:
	@echo "Finding unregistered queries..."
	@go run . findqueries -exclude-tests . || true

# Run all checks (test, verify, routes) - useful for CI
check: db-reset test verify routes
	@echo "All checks passed."
