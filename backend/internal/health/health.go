// Package health serves the health check endpoint.
package health

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler serves the health endpoints.
type Handler struct {
	pool *pgxpool.Pool
}

// New returns a Handler that checks the database through pool.
func New(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// Register adds the health routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health", h.health)
}

// health reports whether the server is up and can reach the database.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.pool.Ping(ctx); err != nil {
		slog.Error("health: database ping failed", "err", err)
		httpx.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "unavailable", "database": "down"})
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "database": "up"})
}
