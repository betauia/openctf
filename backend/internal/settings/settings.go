package settings

import (
	"os"
	"slices"
	"sync/atomic"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	pool  *pgxpool.Pool
	a     *auth.Auth
	theme atomic.Value
	rules atomic.Value
}

func New(pool *pgxpool.Pool, a *auth.Auth) *Handler {
	h := &Handler{pool: pool, a: a}
	theme := os.Getenv("THEME")
	if !slices.Contains(Themes, theme) {
		theme = Themes[0]
	}
	h.theme.Store(theme)
	h.rules.Store("")
	return h
}
