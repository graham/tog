package app

import (
	"testing"
)

func TestNewFlagSet(t *testing.T) {
	fs := newFlagSet("myapp", "serve", "Start the HTTP server")

	if fs == nil {
		t.Fatal("newFlagSet returned nil")
	}

	if fs.Name() != "serve" {
		t.Errorf("Name() = %q, want %q", fs.Name(), "serve")
	}

	// Verify Usage function is set
	if fs.Usage == nil {
		t.Error("Usage function should be set")
	}
}
