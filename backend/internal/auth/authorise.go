package auth

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

// ErrDenied stops a transaction because the user may not perform the action.
// Returning it from pgx.BeginFunc rolls the transaction back.
var ErrDenied = errors.New("permission denied")

// Check reports whether the user may perform action on resource. A resource without a Path
// gets the request's path, so handlers only set the ID and attributes.
type Check func(action string, resource policy.Resource) bool

// Checker loads the subject and context once and returns a Check for this request.
func (a *Auth) Checker(r *http.Request, u *User) (Check, error) {
	ctx := r.Context()
	s, err := a.Subject(ctx, u)
	if err != nil {
		return nil, err
	}
	env, err := a.Env(ctx)
	if err != nil {
		return nil, err
	}
	return func(action string, resource policy.Resource) bool {
		if resource.Path == "" {
			resource.Path = r.URL.Path
		}
		d := a.policies.Evaluate(policy.Request{Subject: s, Action: action, Resource: resource, Context: env})
		if !d.Allowed {
			slog.DebugContext(ctx, "policy denied", "action", action, "path", resource.Path,
				"subject", s.ID, "reason", d.Reason, "policy", d.PolicyID, "statement", d.StatementID)
		}
		return d.Allowed
	}, nil
}

// IsAllowed reports whether u may perform action on resource in request r.
func (a *Auth) IsAllowed(r *http.Request, u *User, action string, resource policy.Resource) (bool, error) {
	check, err := a.Checker(r, u)
	if err != nil {
		return false, err
	}
	return check(action, resource), nil
}

// Authorise runs h only if the user (nil for a visitor) may perform action on the request
// path. Use it for actions that don't depend on a loaded resource, such as challenge:create
// or settings:update.
func (a *Auth) Authorise(action string, h func(http.ResponseWriter, *http.Request, *User)) http.HandlerFunc {
	return a.Optional(func(w http.ResponseWriter, r *http.Request, u *User) {
		ok, err := a.IsAllowed(r, u, action, policy.Resource{})
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
