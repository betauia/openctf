package auth

import (
	"context"
	"net/http"
	"time"

	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/authz"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
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

// Subject returns the authz.Subject for the given user, or an empty subject if u is nil.
func (a *Auth) Subject(ctx context.Context, u *User) (authz.Subject, error) {
	// No role: only EveryonePolicies and SystemPolicies apply.
	if u == nil {
		return authz.Subject{}, nil
	}
	s := authz.Subject{ID: u.ID, Role: authz.Role(u.Role), Banned: u.Banned}
	if u.TeamID != nil {
		s.TeamID = *u.TeamID
		// COALESCE: captain_id is NULL when the team has no captain.
		err := a.pool.QueryRow(ctx,
			`SELECT COALESCE(captain_id = $2, false), banned FROM teams WHERE id = $1`, *u.TeamID, u.ID,
		).Scan(&s.IsCaptain, &s.TeamBanned)
		if err != nil {
			return authz.Subject{}, err
		}
	}
	return s, nil
}

// Env returns the current environment: CTF start/end times, max team size and admin count.
func (a *Auth) Env(ctx context.Context) (authz.Env, error) {
	env := authz.Env{Now: time.Now()}
	var start, end *time.Time
	err := a.pool.QueryRow(ctx, `
		SELECT ctf_start, ctf_end, max_team_size,
		       (SELECT COUNT(*) FROM users WHERE role = 'admin')
		FROM settings`,
	).Scan(&start, &end, &env.MaxTeamSize, &env.AdminCount)
	if start != nil {
		env.CTFStart = *start
	}
	if end != nil {
		env.CTFEnd = *end
	}
	return env, err
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
