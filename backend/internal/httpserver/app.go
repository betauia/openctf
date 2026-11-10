package httpserver

import (
	"net/http"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/config"
	"github.com/betauia/openctf/backend/internal/health"
	"github.com/betauia/openctf/backend/internal/policy"
	"github.com/betauia/openctf/backend/internal/settings"
	"github.com/betauia/openctf/backend/internal/teams"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewMux returns a mux with every feature's routes registered.
//
// Each feature package exposes New(deps...) and Register(mux). To add one, construct
// it here with what it needs and call Register. Features with routes that need the
// logged-in user take a and wrap those handlers in a.Require, a.Optional, a.Authorise or a.IsAllowed.
// See "Adding an API" in backend/README.md.
func NewMux(pool *pgxpool.Pool, cfg config.Config, policies *policy.Engine) *http.ServeMux {
	mux := http.NewServeMux()
	a := auth.New(pool, cfg.SecretKey, policies)

	health.New(pool).Register(mux)
	a.Register(mux)
	teams.New(pool, a).Register(mux)
	settings.New(pool, a).Register(mux)

	return mux
}
