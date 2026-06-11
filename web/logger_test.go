package web

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDefaultLogger(t *testing.T) {
	// Save and restore env
	origLevel := os.Getenv("LOG_LEVEL")
	origNoColor := os.Getenv("NO_COLOR")
	origTerm := os.Getenv("TERM")
	defer func() {
		os.Setenv("LOG_LEVEL", origLevel)
		os.Setenv("NO_COLOR", origNoColor)
		os.Setenv("TERM", origTerm)
	}()

	tests := []struct {
		name      string
		logLevel  string
		noColor   string
		term      string
		wantLevel LogLevel
		wantColor bool
	}{
		{"default", "", "", "xterm", LogLevelInfo, true},
		{"silent", "silent", "", "xterm", LogLevelSilent, true},
		{"none", "none", "", "xterm", LogLevelSilent, true},
		{"off", "off", "", "xterm", LogLevelSilent, true},
		{"error", "error", "", "xterm", LogLevelError, true},
		{"err", "err", "", "xterm", LogLevelError, true},
		{"debug", "debug", "", "xterm", LogLevelDebug, true},
		{"verbose", "verbose", "", "xterm", LogLevelDebug, true},
		{"info", "info", "", "xterm", LogLevelInfo, true},
		{"no color env", "", "1", "xterm", LogLevelInfo, false},
		{"dumb term", "", "", "dumb", LogLevelInfo, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("LOG_LEVEL", tt.logLevel)
			if tt.noColor != "" {
				os.Setenv("NO_COLOR", tt.noColor)
			} else {
				os.Unsetenv("NO_COLOR")
			}
			os.Setenv("TERM", tt.term)

			logger := DefaultLogger()

			if logger.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", logger.Level, tt.wantLevel)
			}
			if logger.NoColor != !tt.wantColor {
				t.Errorf("NoColor = %v, want %v", logger.NoColor, !tt.wantColor)
			}
		})
	}
}

func TestSilentLogger(t *testing.T) {
	logger := SilentLogger()

	if logger.Level != LogLevelSilent {
		t.Errorf("Level = %v, want LogLevelSilent", logger.Level)
	}
}

func TestLogger_colorDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		noColor  bool
		wantGreen bool
		wantYellow bool
		wantRed bool
	}{
		{"fast no color", 5 * time.Millisecond, true, false, false, false},
		{"fast with color", 5 * time.Millisecond, false, true, false, false},
		{"medium with color", 50 * time.Millisecond, false, false, true, false},
		{"slow with color", 200 * time.Millisecond, false, false, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := &Logger{NoColor: tt.noColor}
			result := logger.colorDuration(tt.duration)

			if tt.noColor {
				if strings.Contains(result, "\033[") {
					t.Error("should not contain color codes when NoColor is true")
				}
			} else {
				if tt.wantGreen && !strings.Contains(result, colorGreen) {
					t.Error("expected green color")
				}
				if tt.wantYellow && !strings.Contains(result, colorYellow) {
					t.Error("expected yellow color")
				}
				if tt.wantRed && !strings.Contains(result, colorRed) {
					t.Error("expected red color")
				}
			}
		})
	}
}

func TestLogger_colorDurationPadded(t *testing.T) {
	logger := &Logger{NoColor: false}
	result := logger.colorDurationPadded(5*time.Millisecond, 10)

	if !strings.Contains(result, colorGreen) {
		t.Error("expected green color for fast duration")
	}

	loggerNoColor := &Logger{NoColor: true}
	resultNoColor := loggerNoColor.colorDurationPadded(5*time.Millisecond, 10)
	if strings.Contains(resultNoColor, "\033[") {
		t.Error("should not contain color codes")
	}
}

func TestLogger_colorStatus(t *testing.T) {
	logger := &Logger{NoColor: false}

	tests := []struct {
		status    int
		wantColor string
	}{
		{200, colorGreen},
		{201, colorGreen},
		{301, colorBlue},
		{400, colorYellow},
		{404, colorYellow},
		{500, colorRed},
		{503, colorRed},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			result := logger.colorStatus(tt.status)
			if !strings.Contains(result, tt.wantColor) {
				t.Errorf("status %d: expected color %q in %q", tt.status, tt.wantColor, result)
			}
		})
	}

	// Test no color
	loggerNoColor := &Logger{NoColor: true}
	result := loggerNoColor.colorStatus(200)
	if strings.Contains(result, "\033[") {
		t.Error("should not contain color codes")
	}
}

func TestLogger_colorMethod(t *testing.T) {
	logger := &Logger{NoColor: false}
	result := logger.colorMethod("GET")

	if !strings.Contains(result, colorBold) {
		t.Error("expected bold formatting")
	}
	if !strings.Contains(result, "GET") {
		t.Error("expected method name")
	}

	loggerNoColor := &Logger{NoColor: true}
	resultNoColor := loggerNoColor.colorMethod("GET")
	if resultNoColor != "GET" {
		t.Errorf("expected plain 'GET', got %q", resultNoColor)
	}
}

func TestLogger_colorKey(t *testing.T) {
	logger := &Logger{NoColor: false}
	result := logger.colorKey("sql:query")

	if !strings.Contains(result, colorCyan) {
		t.Error("expected cyan color")
	}

	loggerNoColor := &Logger{NoColor: true}
	resultNoColor := loggerNoColor.colorKey("sql:query")
	if resultNoColor != "sql:query" {
		t.Errorf("expected plain key, got %q", resultNoColor)
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		want     string
	}{
		{500 * time.Nanosecond, "500ns"},
		{1500 * time.Nanosecond, "1.50us"},
		{500 * time.Microsecond, "500.00us"},
		{1500 * time.Microsecond, "1.50ms"},
		{500 * time.Millisecond, "500.00ms"},
		{1500 * time.Millisecond, "1.50s"},
		{2 * time.Second, "2.00s"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			result := formatDuration(tt.duration)
			if result != tt.want {
				t.Errorf("formatDuration(%v) = %q, want %q", tt.duration, result, tt.want)
			}
		})
	}
}

func TestTruncateSQL(t *testing.T) {
	tests := []struct {
		sql  string
		want string
	}{
		{"SELECT * FROM users", "SELECT * FROM users"},
		{"SELECT\n*\nFROM\nusers", "SELECT * FROM users"},
		{strings.Repeat("a", 50), strings.Repeat("a", 40) + "..."},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			result := truncateSQL(tt.sql)
			if result != tt.want {
				t.Errorf("truncateSQL = %q, want %q", result, tt.want)
			}
		})
	}
}

func TestResponseWriter(t *testing.T) {
	t.Run("WriteHeader sets status", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

		rw.WriteHeader(http.StatusNotFound)

		if rw.status != http.StatusNotFound {
			t.Errorf("status = %d, want %d", rw.status, http.StatusNotFound)
		}
		if !rw.written {
			t.Error("expected written to be true")
		}
	})

	t.Run("WriteHeader only sets once", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}

		rw.WriteHeader(http.StatusNotFound)
		rw.WriteHeader(http.StatusInternalServerError)

		if rw.status != http.StatusNotFound {
			t.Errorf("status = %d, want %d (first value)", rw.status, http.StatusNotFound)
		}
	})

	t.Run("Write sets status to 200 if not written", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec, status: 0}

		rw.Write([]byte("hello"))

		if rw.status != http.StatusOK {
			t.Errorf("status = %d, want %d", rw.status, http.StatusOK)
		}
		if !rw.written {
			t.Error("expected written to be true")
		}
	})

	t.Run("Unwrap returns underlying writer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		rw := &responseWriter{ResponseWriter: rec}

		if rw.Unwrap() != rec {
			t.Error("Unwrap should return underlying ResponseWriter")
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	t.Run("silent mode skips logging", func(t *testing.T) {
		logger := SilentLogger()

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := LoggingMiddleware(logger)
		wrapped := ContextMiddleware(nil)(middleware(handler))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Status = %d, want %d", rec.Code, http.StatusOK)
		}
	})

	t.Run("logs request with info level", func(t *testing.T) {
		logger := &Logger{Level: LogLevelInfo, NoColor: true}

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})

		middleware := LoggingMiddleware(logger)
		wrapped := ContextMiddleware(nil)(middleware(handler))

		req := httptest.NewRequest("GET", "/test", nil)
		rec := httptest.NewRecorder()

		wrapped.ServeHTTP(rec, req)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if !strings.Contains(output, "GET") {
			t.Error("expected output to contain method")
		}
		if !strings.Contains(output, "/test") {
			t.Error("expected output to contain path")
		}
		if !strings.Contains(output, "200") {
			t.Error("expected output to contain status")
		}
	})
}

func TestLogger_LogRequest(t *testing.T) {
	t.Run("silent level skips logging", func(t *testing.T) {
		logger := &Logger{Level: LogLevelSilent}
		ctx := &Context{values: make(map[string]any)}
		ctx.ensureTimings()
		req := httptest.NewRequest("GET", "/test", nil)

		// Should not panic or output anything
		logger.LogRequest(req, 200, ctx)
	})

	t.Run("info level logs basic info", func(t *testing.T) {
		logger := &Logger{Level: LogLevelInfo, NoColor: true}
		ctx := &Context{values: make(map[string]any)}
		ctx.ensureTimings()
		req := httptest.NewRequest("GET", "/test", nil)

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		logger.LogRequest(req, 200, ctx)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if !strings.Contains(output, "GET") {
			t.Error("expected output to contain method")
		}
	})

	t.Run("debug level includes timings", func(t *testing.T) {
		logger := &Logger{Level: LogLevelDebug, NoColor: true}
		ctx := &Context{values: make(map[string]any)}
		ctx.ensureTimings()
		ctx.LogQuery("TestQuery", "SELECT 1", 5*time.Millisecond)
		ctx.Start("operation")
		ctx.Stop("operation")

		req := httptest.NewRequest("GET", "/test", nil)

		// Capture stdout
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		logger.LogRequest(req, 200, ctx)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if !strings.Contains(output, "sql:TestQuery") {
			t.Error("expected output to contain query timing")
		}
		if !strings.Contains(output, "operation") {
			t.Error("expected output to contain stopwatch timing")
		}
	})
}

func TestLogger_LogError(t *testing.T) {
	t.Run("silent level skips logging", func(t *testing.T) {
		logger := &Logger{Level: LogLevelSilent}
		req := httptest.NewRequest("GET", "/test", nil)

		// Should not panic or output anything
		logger.LogError(req, errors.New("test error"))
	})

	t.Run("error level logs to stderr", func(t *testing.T) {
		logger := &Logger{Level: LogLevelError, NoColor: true}
		req := httptest.NewRequest("GET", "/test", nil)

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		logger.LogError(req, errors.New("test error"))

		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if !strings.Contains(output, "ERROR") {
			t.Error("expected output to contain ERROR")
		}
		if !strings.Contains(output, "test error") {
			t.Error("expected output to contain error message")
		}
	})

	t.Run("with color", func(t *testing.T) {
		logger := &Logger{Level: LogLevelError, NoColor: false}
		req := httptest.NewRequest("GET", "/test", nil)

		// Capture stderr
		oldStderr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		logger.LogError(req, errors.New("test error"))

		w.Close()
		os.Stderr = oldStderr

		var buf bytes.Buffer
		io.Copy(&buf, r)
		output := buf.String()

		if !strings.Contains(output, colorRed) {
			t.Error("expected red color in output")
		}
	})
}
