package httpserver

import (
	"log/slog"
	"net/http"
	"runtime/debug"
	"slices"
	"time"

	"github.com/betauia/openctf/backend/internal/httpx"
)

// Middleware wraps a handler with behaviour that applies to every request.
type Middleware func(http.Handler) http.Handler

// Chain wraps h in middlewares. The first one listed is the outermost, so it sees the request
// first and the response last.
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for _, m := range slices.Backward(middlewares) {
		h = m(h)
	}
	return h
}

// Log logs one line per request.
func Log(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "status", sw.status, "duration", time.Since(start))
	})
}

// Recover turns a panic in next into a logged 500 instead of a dropped connection.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			// net/http uses this value to abort a response on purpose.
			if v == http.ErrAbortHandler {
				panic(v)
			}
			slog.Error("panic", "method", r.Method, "path", r.URL.Path, "panic", v, "stack", string(debug.Stack()))
			httpx.Error(w, http.StatusInternalServerError, "Internal Server Error")
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Unwrap lets http.ResponseController and the websocket library reach the
// underlying writer, e.g. to hijack the connection.
func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
