// Package teams serves the /api/teams and /api/admin/teams endpoints.
package teams

import (
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/betauia/openctf/backend/internal/auth"
)

// Handler serves the team endpoints.
type Handler struct {
	pool *pgxpool.Pool
	a    *auth.Auth
}

// New returns a Handler that stores teams in pool and checks permissions with a.
func New(pool *pgxpool.Pool, a *auth.Auth) *Handler {
	return &Handler{pool: pool, a: a}
}
