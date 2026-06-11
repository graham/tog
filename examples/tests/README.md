# Inline Tests

Runtime tests using the `inlinetest` command. These tests verify routes work correctly without starting an HTTP server.

## Running Tests

Run individual test files:

```bash
cd examples
go run main.go inlinetest -f inline_tests/health.txt
go run main.go inlinetest -f inline_tests/auth_dev.txt
go run main.go inlinetest -f inline_tests/items_crud.txt
go run main.go inlinetest -f inline_tests/auth_required.txt
```

Run all tests:

```bash
cd examples
./inline_tests/run_all.sh
```

## Test File Format

Each line can be:
- `# comment` - Ignored
- `/path` - GET request
- `METHOD /path` - Request with HTTP method
- `METHOD /path {"json":"body"}` - Request with body
- `?assert status N` - Assert HTTP status code
- `?assert body TEXT` - Assert body contains text
- `?assert json .path "value"` - Assert JSON field equals value
- `?session` - Show current cookies
- `?response` - Show last response

## Test Files

| File | Description |
|------|-------------|
| health.txt | Basic health endpoint tests |
| auth_dev.txt | Dev authentication flow (create_and_assume) |
| auth_required.txt | Protected endpoint tests |
| items_crud.txt | Full CRUD lifecycle for items |

## Writing New Tests

1. Create a `.txt` file in this directory
2. Use `?assert` commands to verify responses
3. Session cookies are automatically persisted with `-f`
4. Use `?session` to debug cookie state
5. Use `?response` to see the last response
