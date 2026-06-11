// logs_test.js - Test user logs functionality
//
// Run with: go run main.go jstest tests/logs_test.js

print("=== User Logs Tests ===")
print("")

// Setup: Create users
print("Setting up users...")
client.createUser("alice@example.com")
client.createUser("bob@example.com")
print("  Created alice and bob")

// Test: Alice creates some logs
print("")
print("--- Alice's Logs ---")
client.login("alice@example.com")

print("Creating logs for Alice...")
var resp = client.post("/api/logs/", {message: "Alice's first log entry"})
assertStatus(resp, 201)
var log1 = resp.json()
print("  Created log id=" + log1.id)

resp = client.post("/api/logs/", {message: "Alice's second log entry"})
assertStatus(resp, 201)
var log2 = resp.json()
print("  Created log id=" + log2.id)

resp = client.post("/api/logs/", {message: "Alice's third log entry"})
assertStatus(resp, 201)
var log3 = resp.json()
print("  Created log id=" + log3.id)

// Test: Alice lists her own logs
print("Listing Alice's logs...")
resp = client.get("/api/logs/")
assertStatus(resp, 200)
var aliceLogs = resp.json()
assert(aliceLogs !== null, "Logs should not be null")
assertEqual(aliceLogs.length, 3, "Alice should have 3 logs")
print("  Alice has " + aliceLogs.length + " logs")

// Verify log content
var foundFirst = false
var foundSecond = false
for (var i = 0; i < aliceLogs.length; i++) {
    if (aliceLogs[i].message === "Alice's first log entry") foundFirst = true
    if (aliceLogs[i].message === "Alice's second log entry") foundSecond = true
}
assert(foundFirst, "Should find first log entry")
assert(foundSecond, "Should find second log entry")
print("  Log content verified")

// Test: Bob creates logs
print("")
print("--- Bob's Logs ---")
client.login("bob@example.com")

print("Creating logs for Bob...")
resp = client.post("/api/logs/", {message: "Bob was here"})
assertStatus(resp, 201)
print("  Created Bob's log")

resp = client.post("/api/logs/", {message: "Bob's second entry"})
assertStatus(resp, 201)
print("  Created Bob's second log")

// Test: Bob lists his own logs
print("Listing Bob's logs...")
resp = client.get("/api/logs/")
assertStatus(resp, 200)
var bobLogs = resp.json()
assertEqual(bobLogs.length, 2, "Bob should have 2 logs")
print("  Bob has " + bobLogs.length + " logs")

// Test: Bob can view Alice's logs by user_id
print("")
print("--- Cross-User Access ---")
// First we need Alice's user ID - we can get it from the log entries
client.login("alice@example.com")
resp = client.get("/auth/whoami")
var aliceUser = resp.json()
var aliceId = aliceUser.id

client.login("bob@example.com")
print("Bob viewing Alice's logs (user_id=" + aliceId + ")...")
resp = client.get("/api/logs/?user_id=" + aliceId)
assertStatus(resp, 200)
var aliceLogsFromBob = resp.json()
assertEqual(aliceLogsFromBob.length, 3, "Should see Alice's 3 logs")
print("  Bob can see Alice's " + aliceLogsFromBob.length + " logs")

// Test: List all logs
print("")
print("--- List All Logs ---")
print("Listing all logs with ?all=true...")
resp = client.get("/api/logs/?all=true")
assertStatus(resp, 200)
var allLogs = resp.json()
assert(allLogs.length >= 5, "Should have at least 5 total logs")
print("  Total logs in system: " + allLogs.length)

// Test: Limit parameter
print("")
print("--- Limit Parameter ---")
print("Testing limit parameter...")
resp = client.get("/api/logs/?all=true&limit=2")
assertStatus(resp, 200)
var limitedLogs = resp.json()
assertEqual(limitedLogs.length, 2, "Should only return 2 logs with limit=2")
print("  Limit=2 correctly returned " + limitedLogs.length + " logs")

// Test: Empty message should fail validation
print("")
print("--- Validation ---")
print("Testing empty message validation...")
resp = client.post("/api/logs/", {message: ""})
assertStatus(resp, 400)
print("  Empty message correctly rejected with 400")

print("")
print("=== All user logs tests passed! ===")
