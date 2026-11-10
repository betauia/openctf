package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

// CurrentUser returns the user from the session cookie, or nil if there is no valid session.
func (a *Auth) CurrentUser(r *http.Request) (*User, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, nil
	}
	id, ok := sessions.VerifyToken(a.secret, c.Value, time.Now())
	if !ok {
		return nil, nil
	}
	u, err := getUserByID(r.Context(), a.pool, id)
	if database.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Subject returns the policy subject for u. A visitor (u == nil) has no ID and no roles, so
// only statements for "*" apply. Every attribute is always set, because a missing attribute
// makes a condition fail and a deny statement reading it not apply.
func (a *Auth) Subject(ctx context.Context, u *User) (policy.Subject, error) {
	attrs := map[string]any{
		"role":        "",
		"banned":      false,
		"has_team":    false,
		"team_id":     "",
		"is_captain":  false,
		"team_banned": false,
	}
	if u == nil {
		return policy.Subject{Attributes: attrs}, nil
	}
	attrs["role"] = u.Role
	attrs["banned"] = u.Banned
	if u.TeamID != nil {
		var isCaptain, teamBanned bool
		// COALESCE: captain_id is NULL when the team has no captain.
		err := a.pool.QueryRow(ctx,
			`SELECT COALESCE(captain_id = $2, false), banned FROM teams WHERE id = $1`, *u.TeamID, u.ID,
		).Scan(&isCaptain, &teamBanned)
		if err != nil {
			return policy.Subject{}, err
		}
		attrs["has_team"] = true
		attrs["team_id"] = u.TeamID.String()
		attrs["is_captain"] = isCaptain
		attrs["team_banned"] = teamBanned
	}
	return policy.Subject{ID: u.ID.String(), Roles: []string{u.Role}, Attributes: attrs}, nil
}

// Env returns the request context the policies read: the CTF phase ("not_started", "running"
// or "ended"), its start and end as Unix seconds, the current time, the max team size and the
// number of admins.
func (a *Auth) Env(ctx context.Context) (map[string]any, error) {
	var start, end time.Time
	var maxTeamSize, adminCount int
	err := a.pool.QueryRow(ctx, `
		SELECT ctf_start, ctf_end, max_team_size,
		       (SELECT COUNT(*) FROM users WHERE role = 'admin')
		FROM settings`,
	).Scan(&start, &end, &maxTeamSize, &adminCount)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	phase := "running"
	switch {
	case now.Before(start):
		phase = "not_started"
	case !now.Before(end):
		phase = "ended"
	}
	return map[string]any{
		"ctf_phase":     phase,
		"ctf_start":     start.Unix(),
		"ctf_end":       end.Unix(),
		"now":           now.Unix(),
		"max_team_size": maxTeamSize,
		"admin_count":   adminCount,
	}, nil
}

// Optional passes the current user to h, or nil when not logged in.
func (a *Auth) Optional(h func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u, err := a.CurrentUser(r)
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		h(w, r, u)
	}
}

// Require responds 401 unless a user is logged in.
func (a *Auth) Require(h func(http.ResponseWriter, *http.Request, User)) http.HandlerFunc {
	return a.Optional(func(w http.ResponseWriter, r *http.Request, u *User) {
		if u == nil {
			httpx.Error(w, http.StatusUnauthorized, "Not authenticated")
			return
		}
		h(w, r, *u)
	})
}
