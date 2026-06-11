package web

import (
	"encoding/json"
	"net/http"
	"os"
)

// WriteJSON encodes v as JSON and writes it to w.
// Pretty prints with indentation when ENVIRONMENT=dev.
func WriteJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	if os.Getenv("ENVIRONMENT") == "dev" {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// LogJSON encodes v as JSON and writes it to stdout.
// Pretty prints with indentation when ENVIRONMENT=dev.
// Optional label is prepended to the output.
func LogJSON(v any, label ...string) error {
	if len(label) > 0 {
		os.Stdout.WriteString(label[0] + ": ")
	}
	enc := json.NewEncoder(os.Stdout)
	if os.Getenv("ENVIRONMENT") == "dev" {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}

// WriteError writes a JSON error response with the given status code.
func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   http.StatusText(statusCode),
		"message": message,
	})
}
