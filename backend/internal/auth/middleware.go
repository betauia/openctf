package auth

import (
	"net/http"
	"time"

	"github.com/betauia/openctf/backend/internal/auth/sessions"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/httpx"
)

// CurrentUser returns the user from the session cookie, or nil if there is no valid session.
func (a *Auth) CurrentUser(r *http.Request) (*dbgen.User, error) {
	c, err := r.Cookie(cookieName)
	if err != nil {
		return nil, nil
	}
	id, ok := sessions.VerifyToken(a.secret, c.Value, time.Now())
	if !ok {
		return nil, nil
	}
	u, err := a.store.GetUserByID(r.Context(), id)
	if database.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Optional passes the current user to h, or nil when not logged in.
func (a *Auth) Optional(h func(http.ResponseWriter, *http.Request, *dbgen.User)) http.HandlerFunc {
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
func (a *Auth) Require(h func(http.ResponseWriter, *http.Request, dbgen.User)) http.HandlerFunc {
	return a.Optional(func(w http.ResponseWriter, r *http.Request, u *dbgen.User) {
		if u == nil {
			httpx.Error(w, http.StatusUnauthorized, "Not authenticated")
			return
		}
		h(w, r, *u)
	})
}

// RequireAdmin responds 403 unless the logged-in user is an admin.
func (a *Auth) RequireAdmin(h func(http.ResponseWriter, *http.Request, dbgen.User)) http.HandlerFunc {
	return a.Require(func(w http.ResponseWriter, r *http.Request, u dbgen.User) {
		if !u.IsAdmin {
			httpx.Error(w, http.StatusForbidden, "Naughty naughty, admin only")
			return
		}
		h(w, r, u)
	})
}
