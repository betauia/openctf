// Package teams serves the /api/teams endpoints: listing teams, membership
// and team activity.
package teams

import (
	"cmp"
	"context"
	"crypto/rand"
	"net/http"
	"slices"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/httpx"
)

// Handler serves the /api/teams endpoints.
type Handler struct {
	store    *database.Store
	auth     *auth.Auth
	onChange func()
}

// New returns a Handler backed by store. onChange is called after team
// membership changes, so the scoreboard can be refreshed.
func New(store *database.Store, a *auth.Auth, onChange func()) *Handler {
	return &Handler{store: store, auth: a, onChange: onChange}
}

// Register adds the /api/teams routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	req := h.auth.Require

	mux.HandleFunc("GET /api/teams", h.listTeams)
	mux.HandleFunc("POST /api/teams", req(h.createTeam))
	mux.HandleFunc("POST /api/teams/join", req(h.joinTeam))
	mux.HandleFunc("GET /api/teams/mine", req(h.myTeam))
	mux.HandleFunc("POST /api/teams/mine/leave", req(h.leaveTeam))
	mux.HandleFunc("GET /api/teams/mine/activity", req(h.myActivity))
	mux.HandleFunc("GET /api/teams/{id}/activity", h.teamActivity)
}

func (h *Handler) notifyChange() {
	if h.onChange != nil {
		h.onChange()
	}
}

// Member is a team member as shown in team listings.
type Member struct {
	ID       uuid.UUID `json:"id"`
	Username string    `json:"username"`
	Score    int32     `json:"score"`
	Solves   int64     `json:"solves"`
}

// Team is a team with its members and total score.
type Team struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	Members    []Member  `json:"members"`
	Score      int64     `json:"score"`
	InviteCode *string   `json:"invite_code"` // only set for the caller's own team
}

func newTeam(t dbgen.Team) Team {
	return Team{ID: t.ID, Name: t.Name, Members: []Member{}}
}

func (t *Team) add(m Member) {
	t.Members = append(t.Members, m)
	t.Score += int64(m.Score)
}

// Ranking returns every team with its members, highest score first.
func Ranking(ctx context.Context, store *database.Store) ([]Team, error) {
	teams, err := store.ListTeams(ctx)
	if err != nil {
		return nil, err
	}
	members, err := store.ListMembers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Team, len(teams))
	index := make(map[uuid.UUID]int, len(teams))
	for i, t := range teams {
		out[i] = newTeam(t)
		index[t.ID] = i
	}
	for _, m := range members {
		if i, ok := index[*m.TeamID]; ok {
			out[i].add(Member{m.ID, m.Username, m.Score, m.Solves})
		}
	}
	slices.SortStableFunc(out, func(a, b Team) int { return cmp.Compare(b.Score, a.Score) })
	return out, nil
}

// ownTeam formats the caller's team, including its invite code.
func (h *Handler) ownTeam(ctx context.Context, t dbgen.Team) (Team, error) {
	members, err := h.store.ListTeamMembers(ctx, &t.ID)
	if err != nil {
		return Team{}, err
	}
	out := newTeam(t)
	out.InviteCode = t.InviteCode
	for _, m := range members {
		out.add(Member{m.ID, m.Username, m.Score, m.Solves})
	}
	return out, nil
}

func (h *Handler) writeOwnTeam(w http.ResponseWriter, r *http.Request, t dbgen.Team) {
	out, err := h.ownTeam(r.Context(), t)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := Ranking(r.Context(), h.store)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, teams)
}

// myTeam returns the caller's team, or null when they aren't in one.
func (h *Handler) myTeam(w http.ResponseWriter, r *http.Request, u dbgen.User) {
	if u.TeamID == nil {
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	t, err := h.store.GetTeamByID(r.Context(), *u.TeamID)
	if database.IsNotFound(err) {
		httpx.JSON(w, http.StatusOK, nil)
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	h.writeOwnTeam(w, r, t)
}

func (h *Handler) joinTeam(w http.ResponseWriter, r *http.Request, u dbgen.User) {
	var body struct {
		InviteCode string `json:"invite_code"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if u.TeamID != nil {
		httpx.Error(w, http.StatusBadRequest, "400 Bad Request: Already in a team")
		return
	}
	ctx := r.Context()
	t, err := h.store.GetTeamByInviteCode(ctx, &body.InviteCode)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "404 Not Found: Team not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if err := h.store.SetUserTeam(ctx, dbgen.SetUserTeamParams{ID: u.ID, TeamID: &t.ID}); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	h.notifyChange()
	h.writeOwnTeam(w, r, t)
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request, u dbgen.User) {
	var body struct {
		Name string `json:"name"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if u.TeamID != nil {
		httpx.Error(w, http.StatusBadRequest, "400 Bad Request: Already in a team")
		return
	}
	if n := utf8.RuneCountInString(body.Name); n < 2 || n > 32 {
		httpx.Error(w, http.StatusBadRequest, "400 Bad Request: Team name must be 2–32 characters")
		return
	}
	ctx := r.Context()
	var t dbgen.Team
	err := h.store.InTx(ctx, func(q *dbgen.Queries) error {
		code := rand.Text()[:12]
		var err error
		t, err = q.CreateTeam(ctx, dbgen.CreateTeamParams{ID: uuid.NewV7(), Name: body.Name, InviteCode: &code})
		if err != nil {
			return err
		}
		return q.SetUserTeam(ctx, dbgen.SetUserTeamParams{ID: u.ID, TeamID: &t.ID})
	})
	if database.IsUniqueViolation(err) {
		httpx.Error(w, http.StatusBadRequest, "400 Bad Request: Team name already taken")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	h.notifyChange()
	h.writeOwnTeam(w, r, t)
}

func (h *Handler) leaveTeam(w http.ResponseWriter, r *http.Request, u dbgen.User) {
	if u.TeamID == nil {
		httpx.Error(w, http.StatusBadRequest, "400 Bad Request: Not in a team")
		return
	}
	if err := h.store.SetUserTeam(r.Context(), dbgen.SetUserTeamParams{ID: u.ID}); err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	h.notifyChange()
	httpx.JSON(w, http.StatusOK, map[string]string{"detail": "Successfully left the team"})
}

func (h *Handler) myActivity(w http.ResponseWriter, r *http.Request, u dbgen.User) {
	if u.TeamID == nil {
		httpx.Error(w, http.StatusBadRequest, "400 Bad Request: Not in a team")
		return
	}
	h.writeActivity(w, r, *u.TeamID)
}

func (h *Handler) teamActivity(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	if _, err := h.store.GetTeamByID(r.Context(), id); database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "404 Not Found: Team not found")
		return
	} else if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	h.writeActivity(w, r, id)
}

// writeActivity writes the team's most recent solves.
func (h *Handler) writeActivity(w http.ResponseWriter, r *http.Request, teamID uuid.UUID) {
	rows, err := h.store.ListTeamActivity(r.Context(), &teamID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	type entry struct {
		UserID    uuid.UUID `json:"user_id"`
		Username  string    `json:"username"`
		Challenge string    `json:"challenge"`
		Category  string    `json:"category"`
		Points    int32     `json:"points"`
		SolvedAt  time.Time `json:"solved_at"`
	}
	out := make([]entry, len(rows))
	for i, a := range rows {
		out[i] = entry{a.UserID, a.Username, a.Challenge, a.Category, a.Points, a.SolvedAt}
	}
	httpx.JSON(w, http.StatusOK, out)
}
