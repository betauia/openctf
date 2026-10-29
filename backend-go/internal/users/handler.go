// Package users serves the /api/users endpoints: public user profiles.
package users

import (
	"net/http"
	"time"
	"uuid"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
)

// Handler serves the /api/users endpoints.
type Handler struct {
	store *database.Store
}

// New returns a Handler backed by store.
func New(store *database.Store) *Handler {
	return &Handler{store: store}
}

// Register adds the /api/users routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users/{id}", h.profile)
}

// profile is a user's public profile with their solves.
func (h *Handler) profile(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	u, err := h.store.GetUserByID(ctx, id)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "404 Not Found: User not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	solves, err := h.store.ListUserSolves(ctx, u.ID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	type solveOut struct {
		ChallengeID uuid.UUID `json:"challenge_id"`
		Challenge   string    `json:"challenge"`
		Category    string    `json:"category"`
		Points      int32     `json:"points"`
		SolvedAt    time.Time `json:"solved_at"`
	}
	out := struct {
		ID       uuid.UUID  `json:"id"`
		Username string     `json:"username"`
		Score    int32      `json:"score"`
		TeamID   *uuid.UUID `json:"team_id"`
		Solves   []solveOut `json:"solves"`
	}{u.ID, u.Username, u.Score, u.TeamID, make([]solveOut, len(solves))}
	for i, s := range solves {
		out.Solves[i] = solveOut{s.ChallengeID, s.Challenge, s.Category, s.Points, s.SolvedAt}
	}
	httpx.JSON(w, http.StatusOK, out)
}
