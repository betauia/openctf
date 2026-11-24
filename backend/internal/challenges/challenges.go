package challenges

import (
	"time"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler serves the challenge endpoints.
type Handler struct {
	pool        *pgxpool.Pool
	a           *auth.Auth
	filesDir    string // root directory that challenge_files.filepath is relative to
	publish     func(event string, data any)
	submitLimit *httpx.Limiter
}

// New returns a Handler that reads challenges from pool, checks permissions with a,
// and serves challenge files from filesDir.
func New(pool *pgxpool.Pool, a *auth.Auth, filesDir string, publish func(event string, data any)) *Handler {
	return &Handler{pool: pool, a: a, filesDir: filesDir, publish: publish, submitLimit: httpx.NewLimiter(10, time.Minute)}
}
