// auth_test.js - Test authentication flows
//
// Run with: go run main.go jstest tests/auth_test.js

print("=== Authentication Tests ===")
print("")

// Test: Unauthenticated access to whoami (returns authenticated=false, not 401)
print("Testing unauthenticated access to /auth/whoami...")
var resp = client.get("/auth/whoami")
assertStatus(resp, 200)
var unauthUser = resp.json()
assertEqual(unauthUser.authenticated, false, "Should not be authenticated")
assertEqual(unauthUser.id, 0, "ID should be 0 when not authenticated")
print("  Correctly shows unauthenticated state")

// Test: Create user and authenticate
print("Creating test user...")
client.createUser("auth-test@example.com")
client.login("auth-test@example.com")
print("  User created and logged in")

// Test: Authenticated access to whoami
print("Testing authenticated /auth/whoami...")
resp = client.get("/auth/whoami")
assertStatus(resp, 200)
var whoami = resp.json()
assertEqual(whoami.email, "auth-test@example.com", "Email should match")
assertEqual(whoami.authenticated, true, "Should be authenticated")
assertEqual(whoami.is_admin, false, "Should not be admin")
print("  whoami returned correct user data")
print("    email: " + whoami.email)
print("    is_admin: " + whoami.is_admin)

// Test: Logout
print("Testing logout...")
client.logout()
resp = client.get("/auth/whoami")
assertStatus(resp, 200)
var loggedOutUser = resp.json()
assertEqual(loggedOutUser.authenticated, false, "Should not be authenticated after logout")
print("  Logout successful, correctly shows unauthenticated")

// Test: Admin user
print("Creating admin user...")
client.createUser("admin-test@example.com", true)
client.login("admin-test@example.com")

resp = client.get("/auth/whoami")
assertStatus(resp, 200)
whoami = resp.json()
assertEqual(whoami.is_admin, true, "Should be admin")
print("  Admin user verified")
print("    email: " + whoami.email)
print("    is_admin: " + whoami.is_admin)

// Test: API key authentication
print("Testing API key authentication...")
client.logout()
client.createUser("apikey-test@example.com")
client.loginWithApiKey("apikey-test@example.com")

resp = client.get("/auth/whoami")
assertStatus(resp, 200)
whoami = resp.json()
assertEqual(whoami.email, "apikey-test@example.com", "Email should match with API key auth")
print("  API key authentication works")

print("")
print("=== All authentication tests passed! ===")
