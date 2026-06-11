.PHONY: all build test testdocs verify routes findqueries clean tidy fmt lint help

# Default target
all: build test

# Build all packages
build:
	@echo "Building all packages..."
	@go build ./...

# Run all tests
test:
	@echo "Running tests..."
	@go test ./...

# Run tests with coverage
test-cover:
	@echo "Running tests with coverage..."
	@go test ./... -cover -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# Generate test documentation (uses example binary)
testdocs:
	@echo "Generating test documentation..."
	@mkdir -p docs
	@cd examples && go run . testdocs -o ../docs/tests.html

# Verify queries against the example database (uses example binary)
verify:
	@echo "Verifying queries..."
	@cd examples && go run . verify -config=databases.json

# List routes from example application (uses example binary)
routes:
	@echo "Listing routes..."
	@cd examples && go run . routes

# Find unregistered queries in the codebase (uses example binary)
findqueries:
	@echo "Finding unregistered queries..."
	@cd examples && go run . findqueries -exclude-tests .. || true

# Run example application
run-example:
	@echo "Running example application..."
	@cd examples && go run . serve

# Format code
fmt:
	@echo "Formatting code..."
	@go fmt ./...

# Run go mod tidy
tidy:
	@echo "Tidying modules..."
	@go mod tidy

# Lint (requires golangci-lint)
lint:
	@echo "Linting..."
	@golangci-lint run ./... || echo "Install golangci-lint for linting"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f coverage.out coverage.html
	@rm -f examples/example.db

# Help
help:
	@echo "tog - Go web framework library"
	@echo ""
	@echo "Usage:"
	@echo "  make              Build and test"
	@echo "  make build        Build all packages"
	@echo "  make test         Run all tests"
	@echo "  make test-cover   Run tests with coverage report"
	@echo "  make testdocs     Generate test documentation HTML"
	@echo "  make verify       Verify queries against example database"
	@echo "  make routes       List routes from example application"
	@echo "  make findqueries  Find unregistered SQL queries"
	@echo "  make run-example  Run the example application"
	@echo "  make fmt          Format code"
	@echo "  make tidy         Run go mod tidy"
	@echo "  make lint         Run linter (requires golangci-lint)"
	@echo "  make clean        Remove build artifacts"
	@echo "  make help         Show this help"
