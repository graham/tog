package auth

import (
	"testing"
)

// Pre-computed bcrypt hash of "testpassword123" at cost 12
// This avoids expensive hash computation in tests
const precomputedHash = "$2a$12$BENF8LkHQw82dS9iBwp/yebltUyMITs8vs1Ku62sGQs1bHTYjcNPq"
const precomputedPassword = "testpassword123"

func TestHashAndCheckPassword(t *testing.T) {
	// Test with pre-computed hash for speed
	if !CheckPassword(precomputedPassword, precomputedHash) {
		t.Error("CheckPassword should return true for correct password")
	}
	if CheckPassword("wrongpassword", precomputedHash) {
		t.Error("CheckPassword should return false for wrong password")
	}
	if CheckPassword(precomputedPassword, "invalid-hash") {
		t.Error("CheckPassword should return false for invalid hash")
	}

	// Single integration test to verify hash generation works
	hash, err := HashPassword("test-integration")
	if err != nil {
		t.Fatalf("HashPassword error = %v", err)
	}
	if !CheckPassword("test-integration", hash) {
		t.Error("CheckPassword should return true for freshly hashed password")
	}
}
