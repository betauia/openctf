// Package httpx holds the JSON request and response helpers shared by the feature packages.
package httpx

import (
	"encoding/json/v2"
	"log/slog"
	"net/http"
)

// JSON writes v as a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.MarshalWrite(w, v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

// Error writes {"detail": detail}, the error shape FastAPI used and the frontend reads.
func Error(w http.ResponseWriter, status int, detail string) {
	JSON(w, status, map[string]string{"detail": detail})
}

// InternalError logs err and writes a generic 500.
func InternalError(w http.ResponseWriter, r *http.Request, err error) {
	slog.Error("request failed", "method", r.Method, "path", r.URL.Path, "err", err)
	Error(w, http.StatusInternalServerError, "Internal Server Error")
}
