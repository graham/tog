// integration_test.js - Full integration test across multiple features
//
// Run with: go run main.go jstest tests/integration_test.js

print("=== Integration Tests ===")
print("")
print("This test simulates a real user workflow:")
print("1. User signs up and authenticates")
print("2. User creates items")
print("3. User logs their activity")
print("4. User verifies their data")
print("")

// Simulate a new user workflow
print("--- Step 1: User Registration ---")
client.createUser("integration@example.com")
client.login("integration@example.com")

var resp = client.get("/auth/whoami")
assertStatus(resp, 200)
var user = resp.json()
print("  User authenticated: " + user.email)
print("  User ID: " + user.id)

// Log the signup
print("")
print("--- Step 2: Log Signup Activity ---")
resp = client.post("/api/logs/", {message: "User signed up"})
assertStatus(resp, 201)
print("  Logged: User signed up")

// Create some items
print("")
print("--- Step 3: Create Items ---")

resp = client.post("/api/items", {
    name: "My First Item",
    description: "Testing the integration",
    price: 10.00
})
assertStatus(resp, 201)
var item1Id = resp.json().id
print("  Created item: My First Item (id=" + item1Id + ")")

// Log the item creation
resp = client.post("/api/logs/", {message: "Created item: My First Item"})
assertStatus(resp, 201)
print("  Logged: item creation")

resp = client.post("/api/items", {
    name: "My Second Item",
    price: 20.00
})
assertStatus(resp, 201)
var item2Id = resp.json().id
print("  Created item: My Second Item (id=" + item2Id + ")")

resp = client.post("/api/logs/", {message: "Created item: My Second Item"})
assertStatus(resp, 201)
print("  Logged: item creation")

// Update an item
print("")
print("--- Step 4: Update Item ---")
resp = client.put("/api/items/" + item1Id, {
    name: "My Updated Item",
    description: "Changed the description",
    price: 15.00
})
assertStatus(resp, 200)
print("  Updated item " + item1Id)

resp = client.post("/api/logs/", {message: "Updated item " + item1Id + " - changed price to $15.00"})
assertStatus(resp, 201)
print("  Logged: item update")

// Verify all data
print("")
print("--- Step 5: Verify Data ---")

// Check items
resp = client.get("/api/items")
assertStatus(resp, 200)
var items = resp.json()
assertEqual(items.length, 2, "Should have 2 items")
print("  Items count: " + items.length)

// Print items
for (var i = 0; i < items.length; i++) {
    print("    - " + items[i].name + " ($" + items[i].price + ")")
}

// Check logs
resp = client.get("/api/logs/")
assertStatus(resp, 200)
var logs = resp.json()
assert(logs.length >= 4, "Should have at least 4 log entries")
print("  Logs count: " + logs.length)

// Print logs
print("  Activity log:")
for (var i = 0; i < logs.length; i++) {
    print("    - " + logs[i].message)
}

// Delete an item and log it
print("")
print("--- Step 6: Delete Item ---")
resp = client.delete("/api/items/" + item2Id)
assertStatus(resp, 200)
print("  Deleted item " + item2Id)

resp = client.post("/api/logs/", {message: "Deleted item " + item2Id})
assertStatus(resp, 201)
print("  Logged: item deletion")

// Final verification
print("")
print("--- Step 7: Final Verification ---")
resp = client.get("/api/items")
assertStatus(resp, 200)
items = resp.json()
assertEqual(items.length, 1, "Should have 1 item remaining")
print("  Items remaining: " + items.length)

resp = client.get("/api/logs/")
assertStatus(resp, 200)
logs = resp.json()
assert(logs.length >= 5, "Should have at least 5 log entries now")
print("  Total log entries: " + logs.length)

// Cleanup
print("")
print("--- Cleanup ---")
resp = client.delete("/api/items/" + item1Id)
assertStatus(resp, 200)
print("  Cleaned up remaining items")

print("")
print("=== Integration tests passed! ===")
print("")
print("Summary:")
print("  - User authentication works")
print("  - Items CRUD operations work")
print("  - User logs track activity")
print("  - All features work together")
