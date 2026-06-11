package web

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// LogLevel controls logging verbosity.
type LogLevel int

const (
	LogLevelSilent LogLevel = iota // No logging
	LogLevelError                  // Errors only
	LogLevelInfo                   // Requests + errors
	LogLevelDebug                  // Requests + timings + errors
)

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[90m"
	colorBold   = "\033[1m"
)

// Logger holds logging configuration.
type Logger struct {
	Level    LogLevel
	NoColor  bool // Disable color output
}

// DefaultLogger returns a logger configured from environment.
// LOG_LEVEL: silent, error, info, debug (default: info)
// NO_COLOR: set to disable colors
func DefaultLogger() *Logger {
	level := LogLevelInfo
	switch strings.ToLower(os.Getenv("LOG_LEVEL")) {
	case "silent", "none", "off":
		level = LogLevelSilent
	case "error", "err":
		level = LogLevelError
	case "debug", "verbose":
		level = LogLevelDebug
	case "info", "":
		level = LogLevelInfo
	}

	noColor := os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb"

	return &Logger{
		Level:   level,
		NoColor: noColor,
	}
}

// SilentLogger returns a logger that outputs nothing.
func SilentLogger() *Logger {
	return &Logger{Level: LogLevelSilent}
}

// colorDuration returns a colored duration string based on threshold.
// < 10ms: green, 10-100ms: yellow, > 100ms: red
func (l *Logger) colorDuration(d time.Duration) string {
	ms := d.Milliseconds()
	dur := formatDuration(d)

	if l.NoColor {
		return dur
	}

	switch {
	case ms < 10:
		return colorGreen + dur + colorReset
	case ms < 100:
		return colorYellow + dur + colorReset
	default:
		return colorRed + dur + colorReset
	}
}

// colorDurationPadded returns a right-justified colored duration string.
// Padding is applied before color codes so alignment works correctly.
func (l *Logger) colorDurationPadded(d time.Duration, width int) string {
	ms := d.Milliseconds()
	dur := formatDuration(d)
	padded := fmt.Sprintf("%*s", width, dur)

	if l.NoColor {
		return padded
	}

	switch {
	case ms < 10:
		return colorGreen + padded + colorReset
	case ms < 100:
		return colorYellow + padded + colorReset
	default:
		return colorRed + padded + colorReset
	}
}

// colorStatus returns a colored status code.
func (l *Logger) colorStatus(status int) string {
	s := fmt.Sprintf("%d", status)
	if l.NoColor {
		return s
	}

	switch {
	case status >= 500:
		return colorRed + s + colorReset
	case status >= 400:
		return colorYellow + s + colorReset
	case status >= 300:
		return colorBlue + s + colorReset
	default:
		return colorGreen + s + colorReset
	}
}

// colorMethod returns a colored HTTP method.
func (l *Logger) colorMethod(method string) string {
	if l.NoColor {
		return method
	}
	return colorBold + method + colorReset
}

// colorKey returns a cyan colored key for timing output.
func (l *Logger) colorKey(key string) string {
	if l.NoColor {
		return key
	}
	return colorCyan + key + colorReset
}

// formatDuration formats duration in a human-friendly way.
func formatDuration(d time.Duration) string {
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

// LogRequest logs a completed request with timings.
func (l *Logger) LogRequest(r *http.Request, status int, ctx *Context) {
	if l.Level < LogLevelInfo {
		return
	}

	reqID := middleware.GetReqID(r.Context())
	duration := ctx.RequestDuration()

	// Main request line
	line := fmt.Sprintf("%s %s -> %s (%s)",
		l.colorMethod(r.Method),
		r.URL.Path,
		l.colorStatus(status),
		l.colorDuration(duration),
	)
	if reqID != "" {
		line = fmt.Sprintf("[%s] %s", reqID, line)
	}
	fmt.Println(line)

	// Timings detail (only in debug mode)
	if l.Level >= LogLevelDebug {
		l.logTimings(ctx)
	}
}

// logTimings outputs detailed timing information.
func (l *Logger) logTimings(ctx *Context) {
	// SQL query timings
	for _, qt := range ctx.QueryTimings() {
		name := qt.Name
		if name == "" {
			name = truncateSQL(qt.SQL)
		}
		fmt.Printf("  %s  %s\n", l.colorDurationPadded(qt.Duration, 10), l.colorKey("sql:"+name))
	}

	// Stopwatch timings
	for _, t := range ctx.Timings() {
		if !t.Stopped {
			continue // Skip timings that weren't stopped
		}
		fmt.Printf("  %s  %s\n", l.colorDurationPadded(t.Duration, 10), l.colorKey(t.Key))
	}
}

// LogError logs an error.
func (l *Logger) LogError(r *http.Request, err error) {
	if l.Level < LogLevelError {
		return
	}

	reqID := middleware.GetReqID(r.Context())
	msg := fmt.Sprintf("ERROR: %v", err)
	if reqID != "" {
		msg = fmt.Sprintf("[%s] %s", reqID, msg)
	}

	if !l.NoColor {
		msg = colorRed + msg + colorReset
	}
	fmt.Fprintln(os.Stderr, msg)
}

// truncateSQL truncates SQL for display.
func truncateSQL(sql string) string {
	// Remove newlines and extra spaces
	sql = strings.Join(strings.Fields(sql), " ")
	if len(sql) > 40 {
		return sql[:40] + "..."
	}
	return sql
}

// responseWriter wraps http.ResponseWriter to capture status code.
type responseWriter struct {
	http.ResponseWriter
	status int
	written bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.written {
		rw.status = code
		rw.written = true
	}
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.written {
		rw.status = http.StatusOK
		rw.written = true
	}
	return rw.ResponseWriter.Write(b)
}

// Unwrap returns the underlying ResponseWriter (for chi compatibility).
func (rw *responseWriter) Unwrap() http.ResponseWriter {
	return rw.ResponseWriter
}

// LoggingMiddleware returns middleware that logs requests with timing.
// This replaces chi's middleware.Logger with timing-aware logging.
func LoggingMiddleware(logger *Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip logging for silent mode
			if logger.Level == LogLevelSilent {
				next.ServeHTTP(w, r)
				return
			}

			// Initialize timings on context
			ctx := GetContext(r.Context())
			ctx.ensureTimings()

			// Wrap response writer to capture status
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			// Execute handler
			next.ServeHTTP(wrapped, r)

			// Log the request
			logger.LogRequest(r, wrapped.status, ctx)
		})
	}
}
