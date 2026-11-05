package teams

import (
	"crypto/rand"
	"net/http"
	"unicode/utf8"
	"uuid"

	"github.com/jackc/pgx/v5"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/authz"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
)

// Register adds the team routes to mux.
func (h *Handler) Register(mux *http.ServeMux) {
	// Player
	mux.HandleFunc("GET /api/teams/{id}", h.a.Optional(h.get))
	mux.HandleFunc("POST /api/teams", h.a.Authorise(authz.TeamCreate, h.create))
	mux.HandleFunc("GET /api/teams/mine", h.a.Require(h.mine))
	mux.HandleFunc("POST /api/teams/join", h.a.Require(h.join))
	mux.HandleFunc("POST /api/teams/mine/leave", h.a.Require(h.leave))
	mux.HandleFunc("DELETE /api/teams/mine/members/{uid}", h.a.Require(h.kick))
	mux.HandleFunc("PUT /api/teams/mine/captain", h.a.Require(h.transferCaptain))

	// Admins
	mux.HandleFunc("PATCH /api/admin/teams/{id}", h.a.Optional(h.adminUpdate))
	mux.HandleFunc("DELETE /api/admin/teams/{id}", h.a.Optional(h.adminDelete))
}

// get returns a team's public profile. The invite code is never included.
func (h *Handler) get(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	t, err := getTeam(ctx, h.pool, id)
	if err != nil {
		auth.Fail(w, r, u, err, "Team not found")
		return
	}
	allowed, err := h.a.IsAllowed(ctx, u, authz.TeamReadPublic, t.resource())
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
	t, err := getTeam(ctx, h.pool, *u.TeamID)
	if err != nil {
		auth.Fail(w, r, &u, err, "Team not found")
		return
	}
	allowed, err := h.a.IsAllowed(ctx, &u, authz.TeamReadPrivate, t.resource())
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
	check, err := h.a.Checker(ctx, &u)
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
		if !check(authz.TeamJoin, t.resource()) { // no_team, TeamFull, NoJoinBannedTeam, RosterFrozen, DenyBanned
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
	check, err := h.a.Checker(ctx, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, *u.TeamID)
		if err != nil {
			return err
		}
		if !check(authz.TeamLeave, t.resource()) { // is_member, CaptainMustTransfer, RosterFrozen
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
	check, err := h.a.Checker(ctx, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, *u.TeamID)
		if err != nil {
			return err
		}
		if !check(authz.TeamKick, t.resource()) { // is_captain, DenyBannedTeam, RosterFrozen
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
	check, err := h.a.Checker(ctx, &u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, *u.TeamID)
		if err != nil {
			return err
		}
		if !check(authz.TeamTransferCaptain, t.resource()) { // is_captain
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

// adminUpdate renames and/or bans a team. Fields left out of the body are unchanged.
func (h *Handler) adminUpdate(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Name   *string `json:"name"`
		Banned *bool   `json:"banned"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	if body.Name != nil && !validName(w, *body.Name) {
		return
	}
	ctx := r.Context()
	check, err := h.a.Checker(ctx, u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		if !check(authz.TeamUpdate, t.resource()) {
			return auth.ErrDenied
		}
		if body.Banned != nil && !check(authz.TeamBan, t.resource()) {
			return auth.ErrDenied
		}
		_, err = tx.Exec(ctx,
			`UPDATE teams SET name = COALESCE($2, name), banned = COALESCE($3, banned) WHERE id = $1`,
			t.ID, body.Name, body.Banned)
		return err
	})
	if database.IsUniqueViolation(err) {
		httpx.Error(w, http.StatusBadRequest, "Team name already taken")
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
	check, err := h.a.Checker(ctx, u)
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	err = pgx.BeginFunc(ctx, h.pool, func(tx pgx.Tx) error {
		t, err := lockTeam(ctx, tx, id)
		if err != nil {
			return err
		}
		if !check(authz.TeamDelete, t.resource()) {
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
