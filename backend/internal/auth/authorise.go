package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/betauia/openctf/backend/internal/authz"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
)

// ErrDenied stops a transaction because the user may not perform the action.
// Returning it from pgx.BeginFunc rolls the transaction back.
var ErrDenied = errors.New("permission denied")

// Checker returns a function that checks if a user is allowed to perform an action on a resource
func (a *Auth) Checker(ctx context.Context, u *User) (func(authz.Action, authz.Resource) bool, error) {
	s, err := a.Subject(ctx, u)
	if err != nil {
		return nil, err
	}
	env, err := a.Env(ctx)
	if err != nil {
		return nil, err
	}
	return func(action authz.Action, resource authz.Resource) bool {
		return authz.IsActionAllowed(s, action, resource, env)
	}, nil
}

// Checks if user may perform an action on a resource
func (a *Auth) IsAllowed(ctx context.Context, u *User, action authz.Action, resource authz.Resource) (bool, error) {
	check, err := a.Checker(ctx, u)
	if err != nil {
		return false, err
	}
	return check(action, resource), nil
}

// Authorise runs h only if the user (nil for a visitor) may perform action. Use it for
// actions that don't depend on a resource, such as challenge:create or settings:update.
func (a *Auth) Authorise(action authz.Action, h func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return a.Optional(func(w http.ResponseWriter, r *http.Request, u *User) {
		ok, err := a.IsAllowed(r.Context(), u, action, authz.Resource{})
		if err != nil {
			httpx.InternalError(w, r, err)
			return
		}
		if !ok {
			Deny(w, u)
			return
		}
		h(w, r, u)
	})
}

// Deny writes 401 to visitors (they should log in) and 403 to logged-in users.
func Deny(w http.ResponseWriter, u *User) {
	if u == nil {
		httpx.Error(w, http.StatusUnauthorized, "Not authenticated")
		return
	}
	httpx.Error(w, http.StatusForbidden, "Not allowed")
}

// Fail answers err from a handler: 404 with notFound for a missing row, 401/403 for
// ErrDenied, 500 for anything else.
func Fail(w http.ResponseWriter, r *http.Request, u *User, err error, notFound string) {
	switch {
	case database.IsNotFound(err):
		httpx.Error(w, http.StatusNotFound, notFound)
	case errors.Is(err, ErrDenied):
		Deny(w, u)
	default:
		httpx.InternalError(w, r, err)
	}
}
