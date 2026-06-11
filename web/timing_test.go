package web

import (
	"context"
	"testing"
	"time"
)

func TestContext_Start_Stop(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}

	ctx.Start("operation1")
	time.Sleep(10 * time.Millisecond)
	ctx.Stop("operation1")

	timings := ctx.Timings()
	if len(timings) != 1 {
		t.Fatalf("expected 1 timing, got %d", len(timings))
	}

	timing := timings[0]
	if timing.Key != "operation1" {
		t.Errorf("Key = %q, want %q", timing.Key, "operation1")
	}
	if !timing.Stopped {
		t.Error("expected timing to be stopped")
	}
	if timing.Duration < 10*time.Millisecond {
		t.Errorf("Duration = %v, expected >= 10ms", timing.Duration)
	}
}

func TestContext_Stop_WithoutStart(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}

	// Should be a no-op, not panic
	ctx.Stop("nonexistent")

	timings := ctx.Timings()
	if len(timings) != 0 {
		t.Errorf("expected 0 timings, got %d", len(timings))
	}
}

func TestContext_Stop_NilTimings(t *testing.T) {
	ctx := &Context{}

	// Should be a no-op when timings is nil
	ctx.Stop("anything")
	// No panic = success
}

func TestContext_MultipleTimings(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}

	ctx.Start("op1")
	ctx.Start("op2")
	time.Sleep(5 * time.Millisecond)
	ctx.Stop("op1")
	time.Sleep(5 * time.Millisecond)
	ctx.Stop("op2")

	timings := ctx.Timings()
	if len(timings) != 2 {
		t.Fatalf("expected 2 timings, got %d", len(timings))
	}

	// Both should be stopped
	for _, timing := range timings {
		if !timing.Stopped {
			t.Errorf("timing %q should be stopped", timing.Key)
		}
	}
}

func TestContext_LogQuery(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}

	ctx.LogQuery("GetUser", "SELECT * FROM users WHERE id = $1", 5*time.Millisecond)
	ctx.LogQuery("ListItems", "SELECT * FROM items", 10*time.Millisecond)

	queryTimings := ctx.QueryTimings()
	if len(queryTimings) != 2 {
		t.Fatalf("expected 2 query timings, got %d", len(queryTimings))
	}

	qt := queryTimings[0]
	if qt.Name != "GetUser" {
		t.Errorf("Name = %q, want %q", qt.Name, "GetUser")
	}
	if qt.SQL != "SELECT * FROM users WHERE id = $1" {
		t.Errorf("SQL = %q, want expected value", qt.SQL)
	}
	if qt.Duration != 5*time.Millisecond {
		t.Errorf("Duration = %v, want 5ms", qt.Duration)
	}
}

func TestContext_Timings_NilTimings(t *testing.T) {
	ctx := &Context{}

	timings := ctx.Timings()
	if timings != nil {
		t.Errorf("expected nil timings, got %v", timings)
	}
}

func TestContext_QueryTimings_NilTimings(t *testing.T) {
	ctx := &Context{}

	queryTimings := ctx.QueryTimings()
	if queryTimings != nil {
		t.Errorf("expected nil query timings, got %v", queryTimings)
	}
}

func TestContext_RequestDuration(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}
	ctx.ensureTimings()

	time.Sleep(10 * time.Millisecond)

	duration := ctx.RequestDuration()
	if duration < 10*time.Millisecond {
		t.Errorf("RequestDuration = %v, expected >= 10ms", duration)
	}
}

func TestContext_RequestDuration_NilTimings(t *testing.T) {
	ctx := &Context{}

	duration := ctx.RequestDuration()
	if duration != 0 {
		t.Errorf("RequestDuration = %v, expected 0", duration)
	}
}

func TestContext_ensureTimings(t *testing.T) {
	ctx := &Context{}

	if ctx.timings != nil {
		t.Error("expected timings to be nil initially")
	}

	ctx.ensureTimings()

	if ctx.timings == nil {
		t.Error("expected timings to be initialized")
	}
	if ctx.timings.stopwatches == nil {
		t.Error("expected stopwatches map to be initialized")
	}

	// Calling again should be a no-op
	first := ctx.timings
	ctx.ensureTimings()
	if ctx.timings != first {
		t.Error("ensureTimings should not replace existing timings")
	}
}

func TestContext_WithQueryLogging(t *testing.T) {
	webCtx := &Context{values: make(map[string]any)}

	ctx := webCtx.WithQueryLogging(context.Background())
	if ctx == nil {
		t.Fatal("WithQueryLogging returned nil")
	}

	// Verify timings were initialized
	if webCtx.timings == nil {
		t.Error("expected timings to be initialized")
	}
}

func TestNewTimings(t *testing.T) {
	tm := newTimings()

	if tm == nil {
		t.Fatal("newTimings returned nil")
	}
	if tm.stopwatches == nil {
		t.Error("stopwatches should be initialized")
	}
	if tm.requestStart.IsZero() {
		t.Error("requestStart should be set")
	}
}

func TestContext_UnstoppedTimings(t *testing.T) {
	ctx := &Context{values: make(map[string]any)}

	ctx.Start("stopped")
	ctx.Start("not_stopped")
	ctx.Stop("stopped")

	timings := ctx.Timings()
	if len(timings) != 2 {
		t.Fatalf("expected 2 timings, got %d", len(timings))
	}

	// Verify we can identify stopped vs unstopped
	var stopped, notStopped *Timing
	for i := range timings {
		if timings[i].Key == "stopped" {
			stopped = &timings[i]
		} else if timings[i].Key == "not_stopped" {
			notStopped = &timings[i]
		}
	}

	if stopped == nil || !stopped.Stopped {
		t.Error("'stopped' timing should be stopped")
	}
	if notStopped == nil || notStopped.Stopped {
		t.Error("'not_stopped' timing should not be stopped")
	}
}
