// Package httpserver wires the feature packages into one HTTP handler and server.
package httpserver

import (
	"net/http"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/challenges"
	"github.com/betauia/openctf/backend/internal/config"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/instances"
	"github.com/betauia/openctf/backend/internal/scoreboard"
	"github.com/betauia/openctf/backend/internal/teams"
	"github.com/betauia/openctf/backend/internal/users"
)

// New wires every feature's routes. The caller runs the returned scoreboard's
// broadcast loop.
func New(store *database.Store, cfg config.Config, provider instances.Provider) (http.Handler, *scoreboard.Handler) {
	a := auth.New(store, cfg.SecretKey)
	sb := scoreboard.New(store)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", func(w http.ResponseWriter, r *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"message": "pong"})
	})
	a.Register(mux)
	users.New(store).Register(mux)
	teams.New(store, a, sb.Notify).Register(mux)
	challenges.New(store, a, cfg.UploadDir, sb.Notify).Register(mux)
	instances.NewHandler(store, a, provider).Register(mux)
	sb.Register(mux)
	return mux, sb
}
