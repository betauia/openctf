package teams

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

// Register adds the team routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	// Player
	mux.HandleFunc("GET /api/teams/{id}", h.a.Optional(h.get))
	mux.HandleFunc("POST /api/teams", h.a.Authorise(policy.TeamCreate, h.create))
	mux.HandleFunc("GET /api/teams", h.a.Authorise(policy.TeamReadPublic, h.getTeams))
	mux.HandleFunc("GET /api/teams/mine", h.a.Require(h.mine))
	mux.HandleFunc("POST /api/teams/join", h.a.Require(h.join))
	mux.HandleFunc("POST /api/teams/mine/leave", h.a.Require(h.leave))
	mux.HandleFunc("DELETE /api/teams/mine/members/{uid}", h.a.Require(h.kick))
	mux.HandleFunc("PUT /api/teams/mine/captain", h.a.Require(h.transferCaptain))
	mux.HandleFunc("PATCH /api/teams/mine", h.a.Require(h.updateTeam))

	// Admins
	mux.HandleFunc("PATCH /api/admin/teams/{id}", h.a.Authorise(policy.TeamUpdate, h.adminUpdate))
	mux.HandleFunc("GET /api/admin/teams/{id}", h.a.Authorise(policy.TeamReadPrivate, h.adminGetTeam))
	mux.HandleFunc("DELETE /api/admin/teams/{id}", h.a.Authorise(policy.TeamDelete, h.adminDelete))
	mux.HandleFunc("GET /api/admin/teams", h.a.Authorise(policy.TeamReadPrivate, h.adminGetTeams))
	mux.HandleFunc("DELETE /api/admin/teams/{id}/members/{uid}", h.a.Authorise(policy.TeamKick, h.adminKick))
}

func (h *Handler) getTeams(w http.ResponseWriter, r *http.Request, u *auth.User) {
	limit, ok := queryInt(w, r, "limit", 50, 1, 100)
	if !ok {
		return
	}
	offset, ok := queryInt(w, r, "offset", 0, 0, math.MaxInt32)
	if !ok {
		return
	}

	teams, err := listTeams(r.Context(), h.pool, limit, offset, false)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}

	for i := range teams {
		teams[i].InviteCode = nil
	}
	httpx.JSON(w, http.StatusOK, teams)
}

func queryInt(w http.ResponseWriter, r *http.Request, key string, def, min, max int) (int, bool) {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def, true
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < min || n > max {
		httpx.Error(w, http.StatusBadRequest, fmt.Sprintf("%s must be an integer between %d and %d", key, min, max))
		return 0, false
	}
	return n, true
}

func (h *Handler) updateTeam(w http.ResponseWriter, r *http.Request, u auth.User) {
	if u.TeamID == nil {
		httpx.Error(w, http.StatusBadRequest, "You are not in a team")
		return
	}
	var body struct {
		Name             *string `json:"name"`
		RegenerateInvite bool    `json:"regenerate_invite"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if body.Name != nil && !validName(w, *body.Name) {
		return
	}
	if body.Name == nil && !body.RegenerateInvite {
		httpx.Error(w, http.StatusBadRequest, "Nothing to update")
		return
	}

	var newCode *string
	if body.RegenerateInvite {
		c := rand.Text()
		newCode = &c
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, *u.TeamID)
		if err != nil {
			return err
		}
		if !check(policy.TeamUpdate, t.resource()) { // TeamCaptain
			return auth.ErrDenied
		}
		_, err = tx.Exec(ctx,
			`UPDATE teams SET name = COALESCE($2, name), invite_code = COALESCE($3, invite_code) WHERE id = $1`,
			t.ID, body.Name, newCode)
		return err
	})
	if database.IsUniqueViolation(err) {
		httpx.Error(w, http.StatusBadRequest, "Team name already taken")
		return
	}
	if err != nil {
		auth.Fail(w, r, &u, err, "Team not found")
		return
	}
	resp := map[string]any{"ok": true}
	if newCode != nil {
		resp["invite_code"] = *newCode
	}
	httpx.JSON(w, http.StatusOK, resp)
}

// get returns a team's public profile. The invite code is never included.
func (h *Handler) get(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	t, err := getTeam(ctx, h.pool, id, false)
	if err != nil {
		auth.Fail(w, r, u, err, "Team not found")
		return
	}
	allowed, err := h.a.IsAllowed(r, u, policy.TeamReadPublic, t.resource())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !allowed {
		auth.Deny(w, u)
		return
	}
	t.InviteCode = nil
	httpx.JSON(w, http.StatusOK, t)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request, u *auth.User) {
	var body struct {
		Name string `json:"name"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if !validName(w, body.Name) {
		return
	}
	ctx := r.Context()
	id := uuid.NewV7()
	err := pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO teams (id, name, captain_id, invite_code) VALUES ($1, $2, $3, $4)`,
			id, body.Name, u.ID, rand.Text())
		if err != nil {
			return err
		}
		// team_id IS NULL: the user may have joined a team since Authorise loaded them.
		tag, err := tx.Exec(ctx, `UPDATE users SET team_id = $1 WHERE id = $2 AND team_id IS NULL`, id, u.ID)
		if err == nil && tag.RowsAffected() == 0 {
			return auth.ErrDenied
		}
		return err
	})
	if database.IsUniqueViolation(err) {
		httpx.Error(w, http.StatusBadRequest, "Team name already taken")
		return
	}
	if err != nil {
		auth.Fail(w, r, u, err, "")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]uuid.UUID{"id": id})
}

func (h *Handler) mine(w http.ResponseWriter, r *http.Request, u auth.User) {
	if u.TeamID == nil {
		httpx.Error(w, http.StatusNotFound, "You are not in a team")
		return
	}
	ctx := r.Context()
	t, err := getTeam(ctx, h.pool, *u.TeamID, true) // members always see their own team
	if err != nil {
		auth.Fail(w, r, &u, err, "Team not found")
		return
	}
	allowed, err := h.a.IsAllowed(r, &u, policy.TeamReadPrivate, t.resource())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !allowed {
		auth.Deny(w, &u)
		return
	}
	members, err := listMembers(ctx, h.pool, t.ID)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, struct {
		team
		Members []member `json:"members"`
	}{t, members})
}

func (h *Handler) join(w http.ResponseWriter, r *http.Request, u auth.User) {
	if u.TeamID != nil {
		httpx.Error(w, http.StatusBadRequest, "You are already in a team")
		return
	}
	var body struct {
		InviteCode string `json:"invite_code"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	var teamID uuid.UUID
	err = h.pool.QueryRow(ctx, `SELECT id FROM teams WHERE invite_code = $1`, body.InviteCode).Scan(&teamID)
	if err != nil {
		auth.Fail(w, r, &u, err, "Invalid invite code")
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, teamID)
		if err != nil {
			return err
		}
		if !check(policy.TeamJoin, t.resource()) { // TeamJoin*, NoJoinBannedTeam, RosterFrozen, DenyBanned
			return auth.ErrDenied
		}

		tag, err := tx.Exec(ctx, `UPDATE users SET team_id = $1 WHERE id = $2 AND team_id IS NULL`, t.ID, u.ID)
		if err == nil && tag.RowsAffected() == 0 {
			return auth.ErrDenied
		}
		return err
	})
	if err != nil {
		auth.Fail(w, r, &u, err, "Invalid invite code")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) leave(w http.ResponseWriter, r *http.Request, u auth.User) {
	if u.TeamID == nil {
		httpx.Error(w, http.StatusBadRequest, "You are not in a team")
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, *u.TeamID)
		if err != nil {
			return err
		}
		if !check(policy.TeamLeave, t.resource()) { // TeamMember, CaptainMustTransfer, RosterFrozen
			return auth.ErrDenied
		}
		if _, err := tx.Exec(ctx, `UPDATE users SET team_id = NULL WHERE id = $1`, u.ID); err != nil {
			return err
		}
		if t.MemberCount == 1 { // the last member left: remove the empty team
			_, err = tx.Exec(ctx, `DELETE FROM teams WHERE id = $1`, t.ID)
		}
		return err
	})
	if err != nil {
		auth.Fail(w, r, &u, err, "Team not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) kick(w http.ResponseWriter, r *http.Request, u auth.User) {
	if u.TeamID == nil {
		httpx.Error(w, http.StatusBadRequest, "You are not in a team")
		return
	}
	target, ok := httpx.PathID(w, r, "uid")
	if !ok {
		return
	}
	if target == u.ID {
		httpx.Error(w, http.StatusBadRequest, "Use leave to leave your own team")
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, *u.TeamID)
		if err != nil {
			return err
		}
		if !check(policy.TeamKick, t.resource()) { // TeamCaptain, DenyBannedTeam, RosterFrozen
			return auth.ErrDenied
		}
		tag, err := tx.Exec(ctx, `UPDATE users SET team_id = NULL WHERE id = $1 AND team_id = $2`, target, t.ID)
		if err == nil && tag.RowsAffected() == 0 {
			return pgx.ErrNoRows // not a member of this team
		}
		return err
	})
	if err != nil {
		auth.Fail(w, r, &u, err, "Member not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// transferCaptain makes another member the captain.
func (h *Handler) transferCaptain(w http.ResponseWriter, r *http.Request, u auth.User) {
	if u.TeamID == nil {
		httpx.Error(w, http.StatusBadRequest, "You are not in a team")
		return
	}
	var body struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, *u.TeamID)
		if err != nil {
			return err
		}
		if !check(policy.TeamTransferCaptain, t.resource()) { // TeamCaptain
			return auth.ErrDenied
		}
		tag, err := tx.Exec(ctx, `
			UPDATE teams SET captain_id = $2
			WHERE id = $1 AND EXISTS (SELECT 1 FROM users WHERE id = $2 AND team_id = $1)`,
			t.ID, body.UserID)
		if err == nil && tag.RowsAffected() == 0 {
			return pgx.ErrNoRows // not a member of this team
		}
		return err
	})
	if err != nil {
		auth.Fail(w, r, &u, err, "Member not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// errNotMember is returned from a transaction when the new captain isn't in the team.
var errNotMember = errors.New("not a member of this team")

// adminUpdate renames, bans, hides and/or changes the captain of a team. Fields left out of
// the body are unchanged.
func (h *Handler) adminUpdate(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Name      *string    `json:"name"`
		Banned    *bool      `json:"banned"`
		Hidden    *bool      `json:"hidden"`
		CaptainID *uuid.UUID `json:"captain_id"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if body.Name == nil && body.Banned == nil && body.Hidden == nil && body.CaptainID == nil {
		httpx.Error(w, http.StatusBadRequest, "Nothing to update")
		return
	}
	if body.Name != nil && !validName(w, *body.Name) {
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		if !check(policy.TeamUpdate, t.resource()) {
			return auth.ErrDenied
		}
		if body.Banned != nil && !check(policy.TeamBan, t.resource()) {
			return auth.ErrDenied
		}
		if body.CaptainID != nil && !check(policy.TeamTransferCaptain, t.resource()) {
			return auth.ErrDenied
		}
		// The EXISTS keeps the update from running at all when captain_id isn't a member.
		tag, err := tx.Exec(ctx, `
			UPDATE teams SET
				name       = COALESCE($2, name),
				banned     = COALESCE($3, banned),
				hidden     = COALESCE($4, hidden),
				captain_id = COALESCE($5, captain_id)
			WHERE id = $1
			  AND ($5::uuid IS NULL OR EXISTS (SELECT 1 FROM users WHERE id = $5 AND team_id = $1))`,
			t.ID, body.Name, body.Banned, body.Hidden, body.CaptainID)
		if err == nil && tag.RowsAffected() == 0 {
			return errNotMember
		}
		return err
	})
	if database.IsUniqueViolation(err) {
		httpx.Error(w, http.StatusBadRequest, "Team name already taken")
		return
	}
	if errors.Is(err, errNotMember) {
		httpx.Error(w, http.StatusBadRequest, "The new captain must be a member of the team")
		return
	}
	if err != nil {
		auth.Fail(w, r, u, err, "Team not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// adminDelete deletes a team. Its members' team_id becomes NULL through the foreign key.
func (h *Handler) adminDelete(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		if !check(policy.TeamDelete, t.resource()) {
			return auth.ErrDenied
		}
		_, err = tx.Exec(ctx, `DELETE FROM teams WHERE id = $1`, t.ID)
		return err
	})
	if err != nil {
		auth.Fail(w, r, u, err, "Team not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// validName writes a 400 and returns false unless name is 3-32 characters.
func validName(w http.ResponseWriter, name string) bool {
	if n := utf8.RuneCountInString(name); n < 3 || n > 32 {
		httpx.Error(w, http.StatusBadRequest, "Team name must be 3-32 characters")
		return false
	}
	return true
}
func (h *Handler) adminGetTeams(w http.ResponseWriter, r *http.Request, u *auth.User) {
	limit, ok := queryInt(w, r, "limit", 50, 1, 100)
	if !ok {
		return
	}
	offset, ok := queryInt(w, r, "offset", 0, 0, math.MaxInt32)
	if !ok {
		return
	}
	teams, err := listTeams(r.Context(), h.pool, limit, offset, true)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, teams)
}

func (h *Handler) adminKick(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	target, ok := httpx.PathID(w, r, "uid")
	if !ok {
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(r, u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		if !check(policy.TeamKick, t.resource()) {
			return auth.ErrDenied
		}
		tag, err := tx.Exec(ctx, `UPDATE users SET team_id = NULL WHERE id = $1 AND team_id = $2`, target, t.ID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return pgx.ErrNoRows // not a member of this team
		}
		if t.MemberCount == 1 { // the last member was kicked: remove the empty team
			_, err = tx.Exec(ctx, `DELETE FROM teams WHERE id = $1`, t.ID)
			return err
		}
		if t.CaptainID != nil && *t.CaptainID == target {
			// UUIDv7 ids sort by creation time, so this picks the oldest remaining account.
			_, err = tx.Exec(ctx, `
				UPDATE teams SET captain_id = (SELECT id FROM users WHERE team_id = $1 ORDER BY id LIMIT 1)
				WHERE id = $1`, t.ID)
		}
		return err
	})
	if err != nil {
		auth.Fail(w, r, u, err, "Member not found")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (h *Handler) adminGetTeam(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	t, err := getTeam(ctx, h.pool, id, true)
	if err != nil {
		auth.Fail(w, r, u, err, "Team not found")
		return
	}
	allowed, err := h.a.IsAllowed(r, u, policy.TeamReadPrivate, t.resource())
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !allowed {
		auth.Deny(w, u)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}
