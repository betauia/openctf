# Adding an API

Every feature is a package under `internal/` with two exported functions: `New`, which takes its dependencies, and `Register`, which adds its routes. [`internal/health`](internal/health/health.go) is the smallest example.

**1. Create the package**, e.g. `internal/users/users.go`:

```go
// Package users serves the /api/users endpoints.
package users

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/betauia/openctf/backend/internal/auth"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/database/dbgen"
	"github.com/betauia/openctf/backend/internal/httpx"
	"github.com/betauia/openctf/backend/internal/policy"
)

type Handler struct {
	pool *pgxpool.Pool
	auth *auth.Auth
}

func New(pool *pgxpool.Pool, a *auth.Auth) *Handler {
	return &Handler{pool: pool, auth: a}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/users/{id}", h.get)                                        // public
	mux.HandleFunc("PATCH /api/users/me", h.auth.Require(h.update))                     // logged in only
	mux.HandleFunc("GET /api/admin/users", h.auth.Authorise(policy.UserList, h.list))   // permission, no resource
	mux.HandleFunc("PATCH /api/admin/users/{id}", h.auth.Optional(h.ban))               // permission on a resource
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	user, err := dbgen.New(h.pool).GetUserByID(r.Context(), id)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "username": user.Username})
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request, u auth.User) {
	var body struct {
		Username string `json:"username"`
	}
	if !httpx.Decode(w, r, &body) {
		return
	}
	// ... u is the logged-in user
}

// Authorise has already checked user:list, so u may be listed users.
func (h *Handler) list(w http.ResponseWriter, r *http.Request, u *auth.User) {
	// ...
}

// ban loads the target first, because the policies depend on it (NoSelfRoleChange, KeepLastAdmin).
func (h *Handler) ban(w http.ResponseWriter, r *http.Request, u *auth.User) {
	id, ok := httpx.PathID(w, r, "id")
	if !ok {
		return
	}
	target, err := dbgen.New(h.pool).GetUserByID(r.Context(), id)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	allowed, err := h.auth.IsAllowed(r, u, policy.UserBan, policy.Resource{
		ID:         id.String(),
		Attributes: map[string]any{"role": target.Role},
	})
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	if !allowed {
		auth.Deny(w, u)
		return
	}
	// ... ban the user
}
```

**2. Register it** in [`internal/httpserver/app.go`](internal/httpserver/app.go):

```go
users.New(pool, a).Register(mux)
```

That's all. The route is logged by the request middleware automatically.

**Conventions**

- **Routes:** put the method in the pattern (`"GET /api/..."`) so other methods get a `405`. Read path values with `r.PathValue("name")`, or `httpx.PathID` for UUIDs.
- **Login:** wrap handlers with `a.Require` (401 when logged out) or `a.Optional` (user may be `nil`). The handler gets the user as its third argument.
- **Permissions:** never check `u.Role` directly; ask PBAC (see [PBAC](policies.md)).
  - The action doesn't depend on a resource (`challenge:create`, `user:list`, `settings:*`): wrap the route in `a.Authorise(action, h)`.
  - It depends on one resource: wrap in `a.Optional`, load the row, then call `a.IsAllowed(r, u, action, resource)` and `auth.Deny(w, u)` when refused.
  - Filtering a list: call `check, err := a.Checker(r, u)` once and `check(action, resource)` per row, so the subject and env are loaded only once.
  - A new route also needs a statement in [`internal/policies/default.json`](internal/policies/default.json) whose `resources` cover its path, or every request is refused.
  - The check must agree with a write: get `check` before the transaction, lock the row inside `pgx.BeginFunc` (`SELECT ... FOR UPDATE`), call `check` on the locked row and return `auth.ErrDenied` when refused. Then answer the error with `auth.Fail(w, r, u, err, "Team not found")`, which maps a missing row to `404`, `ErrDenied` to `401`/`403` and anything else to `500`. [`internal/teams`](internal/teams/routes.go) does this for join, leave, kick, transfer captain and update.
  - Answer `404` when the user may not even read the resource, so hidden challenges can't be discovered.
- **Request bodies:** `httpx.Decode` caps them at 1 MiB and responds `422` itself when the JSON is invalid.
- **Responses:** `httpx.JSON` for success, `httpx.Error` for client errors, `httpx.InternalError` for unexpected errors. It logs the error and hides it from the client.
- **Database:** write SQL as sqlc queries (see [Changing SQL](database.md)) and call them through `dbgen.New(q)`, where `q` is the pool or a `pgx.Tx`. Don't write SQL strings in Go code. Use `database.IsNotFound` and `database.IsUniqueViolation` to check errors. Use `pgx.BeginFunc` when several writes must succeed together. Schema changes go in a new migration in `internal/database/migrations/`, which runs on startup.

