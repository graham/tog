// health_test.js - Test basic health and system endpoints
//
// Run with: go run main.go jstest tests/health_test.js

print("=== Health and System Tests ===")
print("")

// Test: Health endpoint
print("Testing /health endpoint...")
var resp = client.get("/health")
assertStatus(resp, 200)
assertJSON(resp, ".status", "healthy")
print("  /health returned healthy status")

// Test: Root endpoint
print("Testing / endpoint...")
resp = client.get("/")
assertStatus(resp, 200)
assertContains(resp.body, "ok")
print("  / returned ok")

// Test: Non-existent endpoint returns 404
print("Testing non-existent endpoint...")
resp = client.get("/this/does/not/exist")
assertStatus(resp, 404)
print("  Non-existent endpoint correctly returns 404")

print("")
print("=== All health tests passed! ===")
