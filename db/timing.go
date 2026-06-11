package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// QueryHook is called after each query execution with timing information.
// name is the query name (if registered with a name), sql is the query text,
// and duration is how long the query took.
type QueryHook func(name, sql string, duration time.Duration)

// queryHookKey is the context key for query hooks.
type queryHookKey struct{}

// ContextWithQueryHook returns a context that will call hook after each query.
// Multiple hooks can be chained by wrapping contexts.
func ContextWithQueryHook(ctx context.Context, hook QueryHook) context.Context {
	return context.WithValue(ctx, queryHookKey{}, hook)
}

// queryHookFromContext retrieves the query hook from context, if any.
func queryHookFromContext(ctx context.Context) QueryHook {
	if hook, ok := ctx.Value(queryHookKey{}).(QueryHook); ok {
		return hook
	}
	return nil
}

// Global SQL stopwatch configuration
var (
	sqlStopwatchEnabled bool
	sqlStopwatchOnce    sync.Once
	sqlStopwatchHook    QueryHook
)

// initSQLStopwatch initializes the global SQL stopwatch from environment.
func initSQLStopwatch() {
	sqlStopwatchOnce.Do(func() {
		if os.Getenv("SQL_STOPWATCH") == "1" {
			sqlStopwatchEnabled = true
			sqlStopwatchHook = defaultSQLStopwatchHook
		}
	})
}

// SetSQLStopwatchHook sets a custom hook for SQL stopwatch output.
// If not set, defaults to printing to stdout.
func SetSQLStopwatchHook(hook QueryHook) {
	sqlStopwatchHook = hook
}

// ANSI color codes for SQL stopwatch output
const (
	colorReset = "\033[0m"
	colorGreen = "\033[32m"
	colorYellow = "\033[33m"
	colorRed   = "\033[31m"
	colorCyan  = "\033[36m"
)

// defaultSQLStopwatchHook prints SQL timing to stderr with colors.
func defaultSQLStopwatchHook(name, query string, duration time.Duration) {
	label := name
	if label == "" {
		label = truncateQuery(query)
	}

	// Color the duration based on threshold
	dur := formatDurationShort(duration)
	ms := duration.Milliseconds()
	var coloredDur string
	switch {
	case ms < 10:
		coloredDur = colorGreen + fmt.Sprintf("%10s", dur) + colorReset
	case ms < 100:
		coloredDur = colorYellow + fmt.Sprintf("%10s", dur) + colorReset
	default:
		coloredDur = colorRed + fmt.Sprintf("%10s", dur) + colorReset
	}

	fmt.Fprintf(os.Stderr, "  %s  %ssql:%s%s\n", coloredDur, colorCyan, label, colorReset)
}

// truncateQuery truncates and cleans a query for display.
func truncateQuery(query string) string {
	// Remove newlines and extra spaces
	query = strings.Join(strings.Fields(query), " ")
	if len(query) > 50 {
		return query[:50] + "..."
	}
	return query
}

// formatDurationShort formats duration in a compact way.
func formatDurationShort(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%.2fus", float64(d.Nanoseconds())/1000)
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d.Nanoseconds())/1000000)
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// logSQLTiming logs SQL timing if stopwatch is enabled.
func logSQLTiming(name, query string, duration time.Duration) {
	initSQLStopwatch()
	if sqlStopwatchEnabled && sqlStopwatchHook != nil {
		sqlStopwatchHook(name, query, duration)
	}
}
