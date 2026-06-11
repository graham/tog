package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"github.com/go-chi/chi/v5/middleware"
)

// AppError represents a structured error for API responses.
// It provides user-friendly messages with optional debug info in development.
type AppError struct {
	// HTTP status code
	Status int
	// User-friendly message (always shown to clients)
	Message string
	// Internal error details (only shown in dev mode)
	Internal error
	// Unique ID for error tracking/reporting
	TrackingID string
}

// Error implements the error interface.
func (e *AppError) Error() string {
	if e.Internal != nil {
		return e.Message + ": " + e.Internal.Error()
	}
	return e.Message
}

// Unwrap returns the internal error for errors.Is/As support.
func (e *AppError) Unwrap() error {
	return e.Internal
}

// newAppError creates a new AppError with the given status and message.
func newAppError(status int, message string, internal error) *AppError {
	return &AppError{
		Status:     status,
		Message:    message,
		Internal:   internal,
		TrackingID: generateTrackingID(),
	}
}

// ErrBadRequest creates a 400 Bad Request error.
func ErrBadRequest(msg string, err error) *AppError {
	return newAppError(http.StatusBadRequest, msg, err)
}

// ErrUnauthorized creates a 401 Unauthorized error.
func ErrUnauthorized(msg string, err error) *AppError {
	return newAppError(http.StatusUnauthorized, msg, err)
}

// ErrForbidden creates a 403 Forbidden error.
func ErrForbidden(msg string, err error) *AppError {
	return newAppError(http.StatusForbidden, msg, err)
}

// ErrNotFound creates a 404 Not Found error.
func ErrNotFound(msg string, err error) *AppError {
	return newAppError(http.StatusNotFound, msg, err)
}

// ErrInternal creates a 500 Internal Server Error.
func ErrInternal(msg string, err error) *AppError {
	return newAppError(http.StatusInternalServerError, msg, err)
}

// errorResponse is the JSON structure for error responses.
type errorResponse struct {
	Error      string         `json:"error"`
	Message    string         `json:"message"`
	TrackingID string         `json:"tracking_id,omitempty"`
	Debug      *debugInfo     `json:"debug,omitempty"`
}

// debugInfo contains development-only error details.
type debugInfo struct {
	Internal string `json:"internal,omitempty"`
	File     string `json:"file,omitempty"`
	Line     int    `json:"line,omitempty"`
	Function string `json:"function,omitempty"`
}

// WriteAppError writes a structured JSON error response.
// In dev mode (ENVIRONMENT=dev), includes file, function, and internal error.
// In prod mode, includes only the user-friendly message and tracking ID.
func WriteAppError(w http.ResponseWriter, r *http.Request, appErr *AppError) {
	if appErr == nil {
		appErr = ErrInternal("Unknown error", nil)
	}

	// Use request ID if available, otherwise use generated tracking ID
	trackingID := appErr.TrackingID
	if reqID := middleware.GetReqID(r.Context()); reqID != "" {
		trackingID = reqID
	}

	resp := errorResponse{
		Error:      http.StatusText(appErr.Status),
		Message:    appErr.Message,
		TrackingID: trackingID,
	}

	// Include debug info in dev mode
	if isDevMode() {
		debug := &debugInfo{}
		if appErr.Internal != nil {
			debug.Internal = appErr.Internal.Error()
		}

		// Get caller info (skip WriteAppError and the caller)
		if pc, file, line, ok := runtime.Caller(1); ok {
			debug.File = filepath.Base(file)
			debug.Line = line
			if fn := runtime.FuncForPC(pc); fn != nil {
				debug.Function = filepath.Base(fn.Name())
			}
		}

		resp.Debug = debug
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(appErr.Status)

	enc := json.NewEncoder(w)
	if isDevMode() {
		enc.SetIndent("", "  ")
	}
	enc.Encode(resp)
}

// isDevMode returns true if ENVIRONMENT is set to "dev".
func isDevMode() bool {
	return os.Getenv("ENVIRONMENT") == "dev"
}

// generateTrackingID generates a short random ID for error tracking.
func generateTrackingID() string {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "err-unknown"
	}
	return hex.EncodeToString(b)
}
