package db

import (
	"context"
	"testing"
	"time"
)

func TestFormatDurationShort(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"nanoseconds", 500 * time.Nanosecond, "500ns"},
		{"microseconds", 123 * time.Microsecond, "123.00us"},
		{"microseconds fractional", 1500 * time.Nanosecond, "1.50us"},
		{"milliseconds", 45 * time.Millisecond, "45.00ms"},
		{"milliseconds fractional", 1500 * time.Microsecond, "1.50ms"},
		{"seconds", 2 * time.Second, "2.00s"},
		{"seconds fractional", 1500 * time.Millisecond, "1.50s"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatDurationShort(tt.duration); got != tt.want {
				t.Errorf("formatDurationShort(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestTruncateQuery(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"short query", "SELECT * FROM users", "SELECT * FROM users"},
		{"exactly 50 chars", "12345678901234567890123456789012345678901234567890", "12345678901234567890123456789012345678901234567890"},
		{"long query truncated", "SELECT * FROM users WHERE id = 1 AND name = 'something very long'", "SELECT * FROM users WHERE id = 1 AND name = 'somet..."},
		{"newlines removed", "SELECT *\nFROM users\nWHERE id = 1", "SELECT * FROM users WHERE id = 1"},
		{"multiple spaces collapsed", "SELECT *   FROM    users", "SELECT * FROM users"},
		{"tabs removed", "SELECT *\t\tFROM\tusers", "SELECT * FROM users"},
		{"empty query", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := truncateQuery(tt.query); got != tt.want {
				t.Errorf("truncateQuery(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestContextWithQueryHook(t *testing.T) {
	var called bool
	var capturedName, capturedSQL string
	var capturedDuration time.Duration

	hook := func(name, sql string, duration time.Duration) {
		called = true
		capturedName = name
		capturedSQL = sql
		capturedDuration = duration
	}

	ctx := ContextWithQueryHook(context.Background(), hook)

	// Retrieve and call the hook
	retrievedHook := queryHookFromContext(ctx)
	if retrievedHook == nil {
		t.Fatal("expected hook to be retrievable from context")
	}

	retrievedHook("test_query", "SELECT 1", 100*time.Millisecond)

	if !called {
		t.Error("hook was not called")
	}
	if capturedName != "test_query" {
		t.Errorf("name = %q, want %q", capturedName, "test_query")
	}
	if capturedSQL != "SELECT 1" {
		t.Errorf("sql = %q, want %q", capturedSQL, "SELECT 1")
	}
	if capturedDuration != 100*time.Millisecond {
		t.Errorf("duration = %v, want %v", capturedDuration, 100*time.Millisecond)
	}
}

func TestQueryHookFromContext_NoHook(t *testing.T) {
	ctx := context.Background()
	hook := queryHookFromContext(ctx)
	if hook != nil {
		t.Error("expected nil hook from context without hook")
	}
}

func TestSetSQLStopwatchHook(t *testing.T) {
	// Save original
	original := sqlStopwatchHook

	var called bool
	testHook := func(name, sql string, duration time.Duration) {
		called = true
	}

	SetSQLStopwatchHook(testHook)

	if sqlStopwatchHook == nil {
		t.Error("hook should be set")
	}

	// Call it to verify
	sqlStopwatchHook("", "", 0)
	if !called {
		t.Error("custom hook was not called")
	}

	// Restore original
	sqlStopwatchHook = original
}
