package web

import (
	"context"
	"sync"
	"time"

	"github.com/graham/tog/db"
)

// Timing represents a named timing measurement.
type Timing struct {
	Key       string
	StartTime time.Time
	Duration  time.Duration
	Stopped   bool
}

// QueryTiming represents a SQL query timing.
type QueryTiming struct {
	Name     string        // Query name (or truncated SQL)
	SQL      string        // Full SQL query
	Duration time.Duration // Time taken to execute
}

// timings holds all timing data for a request.
type timings struct {
	mu       sync.Mutex
	stopwatches map[string]*Timing
	queries  []QueryTiming
	requestStart time.Time
}

// newTimings creates a new timings instance.
func newTimings() *timings {
	return &timings{
		stopwatches: make(map[string]*Timing),
		requestStart: time.Now(),
	}
}

// Start begins timing for the given key.
// Call Stop(key) or use defer ctx.Stop(key) to end timing.
func (c *Context) Start(key string) {
	c.ensureTimings()
	c.timings.mu.Lock()
	defer c.timings.mu.Unlock()

	c.timings.stopwatches[key] = &Timing{
		Key:       key,
		StartTime: time.Now(),
	}
}

// Stop ends timing for the given key.
// Safe to call even if Start wasn't called (will be a no-op).
func (c *Context) Stop(key string) {
	if c.timings == nil {
		return
	}
	c.timings.mu.Lock()
	defer c.timings.mu.Unlock()

	if t, ok := c.timings.stopwatches[key]; ok && !t.Stopped {
		t.Duration = time.Since(t.StartTime)
		t.Stopped = true
	}
}

// LogQuery records a SQL query timing.
// Called automatically by ExecCtx methods when logging is enabled.
func (c *Context) LogQuery(name, sql string, duration time.Duration) {
	c.ensureTimings()
	c.timings.mu.Lock()
	defer c.timings.mu.Unlock()

	c.timings.queries = append(c.timings.queries, QueryTiming{
		Name:     name,
		SQL:      sql,
		Duration: duration,
	})
}

// Timings returns all stopwatch timings for the request.
func (c *Context) Timings() []Timing {
	if c.timings == nil {
		return nil
	}
	c.timings.mu.Lock()
	defer c.timings.mu.Unlock()

	result := make([]Timing, 0, len(c.timings.stopwatches))
	for _, t := range c.timings.stopwatches {
		result = append(result, *t)
	}
	return result
}

// QueryTimings returns all SQL query timings for the request.
func (c *Context) QueryTimings() []QueryTiming {
	if c.timings == nil {
		return nil
	}
	c.timings.mu.Lock()
	defer c.timings.mu.Unlock()

	result := make([]QueryTiming, len(c.timings.queries))
	copy(result, c.timings.queries)
	return result
}

// RequestDuration returns time since request started.
func (c *Context) RequestDuration() time.Duration {
	if c.timings == nil {
		return 0
	}
	return time.Since(c.timings.requestStart)
}

// ensureTimings initializes timings if nil.
func (c *Context) ensureTimings() {
	if c.timings == nil {
		c.timings = newTimings()
	}
}

// WithQueryLogging returns a context.Context that logs SQL query timings to this Context.
// Use this when calling ExecCtx methods on prepared queries:
//
//	ctx := webCtx.WithQueryLogging(r.Context())
//	items, err := queries.ListItems.ExecCtx(ctx).All()
func (c *Context) WithQueryLogging(ctx context.Context) context.Context {
	c.ensureTimings()
	return db.ContextWithQueryHook(ctx, func(name, sql string, duration time.Duration) {
		c.LogQuery(name, sql, duration)
	})
}
