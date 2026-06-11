// items_test.js - Comprehensive test for items API
//
// Run with: go run main.go jstest tests/items_test.js

print("=== Items API Tests ===")
print("")

// Setup: Create users
print("Setting up users...")
client.createUser("items-user@example.com")
client.createUser("other-user@example.com")
print("  Created test users")

// Login as first user
client.login("items-user@example.com")

// Test: List items (should be empty/null initially)
print("")
print("--- Initial State ---")
var resp = client.get("/api/items")
assertStatus(resp, 200)
var items = resp.json()
// Go returns null for empty slices
assert(items === null || items.length === 0, "Items should be empty initially")
print("  Items list is empty as expected")

// Test: Create items
print("")
print("--- Create Items ---")

print("Creating first item (Widget)...")
resp = client.post("/api/items", {
    name: "Widget",
    description: "A useful widget",
    price: 9.99
})
assertStatus(resp, 201)
var widget = resp.json()
assert(widget.id > 0, "Should return item ID")
assertEqual(widget.message, "item created", "Should return success message")
var widgetId = widget.id
print("  Created Widget with id=" + widgetId)

print("Creating second item (Gadget)...")
resp = client.post("/api/items", {
    name: "Gadget",
    description: "A fancy gadget",
    price: 29.99
})
assertStatus(resp, 201)
var gadget = resp.json()
var gadgetId = gadget.id
print("  Created Gadget with id=" + gadgetId)

print("Creating third item (Gizmo)...")
resp = client.post("/api/items", {
    name: "Gizmo",
    price: 49.99
})
assertStatus(resp, 201)
var gizmo = resp.json()
var gizmoId = gizmo.id
print("  Created Gizmo with id=" + gizmoId)

// Test: List all items
print("")
print("--- List Items ---")
resp = client.get("/api/items")
assertStatus(resp, 200)
items = resp.json()
assertEqual(items.length, 3, "Should have 3 items")
print("  User has " + items.length + " items")

// Verify item details
for (var i = 0; i < items.length; i++) {
    print("    - " + items[i].name + " ($" + items[i].price + ")")
}

// Test: Get single item
print("")
print("--- Get Single Item ---")
resp = client.get("/api/items/" + widgetId)
assertStatus(resp, 200)
var fetchedWidget = resp.json()
assertEqual(fetchedWidget.id, widgetId, "ID should match")
assertEqual(fetchedWidget.name, "Widget", "Name should match")
assertEqual(fetchedWidget.description, "A useful widget", "Description should match")
print("  Fetched Widget: " + fetchedWidget.name + " - " + fetchedWidget.description)

// Test: Update item
print("")
print("--- Update Item ---")
print("Updating Widget...")
resp = client.put("/api/items/" + widgetId, {
    name: "Super Widget",
    description: "An even more useful widget",
    price: 19.99
})
assertStatus(resp, 200)
var updateResult = resp.json()
assertEqual(updateResult.message, "item updated", "Should return success message")
print("  Update successful")

// Verify update
resp = client.get("/api/items/" + widgetId)
assertStatus(resp, 200)
var updatedWidget = resp.json()
assertEqual(updatedWidget.name, "Super Widget", "Name should be updated")
assertEqual(updatedWidget.description, "An even more useful widget", "Description should be updated")
print("  Verified: " + updatedWidget.name + " ($" + updatedWidget.price + ")")

// Test: User isolation - other user should not see these items
print("")
print("--- User Isolation ---")
client.login("other-user@example.com")

print("Checking other user cannot see first user's items...")
resp = client.get("/api/items")
assertStatus(resp, 200)
var otherItems = resp.json()
assert(otherItems === null || otherItems.length === 0, "Other user should have no items")
print("  Other user correctly has no items")

print("Checking other user cannot access first user's item by ID...")
resp = client.get("/api/items/" + widgetId)
assertStatus(resp, 404)
print("  Other user correctly gets 404 for first user's item")

// Create item for other user
print("Creating item for other user...")
resp = client.post("/api/items", {
    name: "Other User's Item",
    price: 5.00
})
assertStatus(resp, 201)
var otherItem = resp.json()
print("  Created item for other user (id=" + otherItem.id + ")")

// Switch back to first user
client.login("items-user@example.com")

// Test: Delete item
print("")
print("--- Delete Items ---")
print("Deleting Gizmo...")
resp = client.delete("/api/items/" + gizmoId)
assertStatus(resp, 200)
var deleteResult = resp.json()
assertEqual(deleteResult.message, "item deleted", "Should return success message")
print("  Gizmo deleted")

// Verify deletion
resp = client.get("/api/items/" + gizmoId)
assertStatus(resp, 404)
print("  Verified Gizmo no longer exists (404)")

// Verify remaining items
resp = client.get("/api/items")
assertStatus(resp, 200)
items = resp.json()
assertEqual(items.length, 2, "Should have 2 remaining items")
print("  Remaining items: " + items.length)

// Test: Cannot delete other user's item
print("")
print("--- Delete Protection ---")
print("Attempting to delete other user's item...")
resp = client.delete("/api/items/" + otherItem.id)
assertStatus(resp, 404)
print("  Correctly prevented from deleting other user's item")

// Clean up remaining items
print("")
print("--- Cleanup ---")
resp = client.delete("/api/items/" + widgetId)
assertStatus(resp, 200)
resp = client.delete("/api/items/" + gadgetId)
assertStatus(resp, 200)
print("  Cleaned up remaining items")

// Verify empty
resp = client.get("/api/items")
assertStatus(resp, 200)
items = resp.json()
assert(items === null || items.length === 0, "Should have no items after cleanup")
print("  Verified items list is empty")

print("")
print("=== All items API tests passed! ===")
