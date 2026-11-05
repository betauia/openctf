package settings

import (
	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool *pgxpool.Pool
	a    *auth.Auth
}

func New(pool *pgxpool.Pool, a *auth.Auth) *Handler {
	return &Handler{pool: pool, a: a}
}
