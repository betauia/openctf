# OpenCTF backend

## Requirements

- Go 1.27+
- PostgreSQL

## Configuration

| Variable       | Default      | Notes                               |
| -------------- | ------------ | ----------------------------------- |
| `DATABASE_URL` | _(required)_ | `postgres://user:pass@host:5432/db` |
| `ADDR`         | `:8000`      | Listen address                      |
| `SECRET_KEY`   | _(required)_ | Signs session cookies               |
| `ADMIN_EMAIL`    | _(required)_ | Email of the default `admin` user, created on first start |
| `ADMIN_PASSWORD` | _(required)_ | Password of the default `admin` user                      |
| `POLICY_FILE`    | `internal/policies/default.json` | PBAC policy JSON, relative to the working directory. The production image sets `/policies/default.json` |

## Running

```sh
go run ./cmd/server
```

Migrations in `internal/database/migrations` are embedded in the binary and applied on startup. This means you won't need to run `goose` manually, but you can still use it if you want to inspect the database. This also means that the database must be reachable when starting the server, otherwise it will fail to start.

With Docker Compose (from the repository root), the `database` service starts first and the backend waits for its healthcheck:

```sh
docker compose up backend 
```

## Testing

```sh
go test ./...
```

Run it from `backend/`. The leading `./` is required: without it Go looks for the package in the standard library.

## API

Every route lives under `/api/`, except the `/health` probe. Admin-only routes live under `/api/admin/`.

- **Status:** ✅ is implemented. 🚧 is implemented but not registered in [`app.go`](internal/httpserver/app.go) yet. 📝 is planned.
- **Who:** the PBAC action the route checks (see [PBAC](#pbac)). _Public_ means anyone, including logged-out visitors. _Logged in_ means only a session is required.
- **Errors:** always `{"detail": "message"}`. A refused permission is `401` for visitors and `403` for logged-in users. A resource the user may not see is `404`. Invalid JSON is `422`.
- **Responses:** routes that only change something answer `{"ok":true}`.

### Health

| Status | Method | Path      | Who    | Request → Response                                                         |
| ------ | ------ | --------- | ------ | -------------------------------------------------------------------------- |
| ✅     | `GET`  | `/health` | Public | `200 {"status":"ok","database":"up"}`, or `503` if Postgres is unreachable |

### Authentication

| Status | Method | Path                 | Who       | Request → Response                                                                                   |
| ------ | ------ | -------------------- | --------- | ---------------------------------------------------------------------------------------------------- |
| ✅     | `POST` | `/api/auth/register` | Public    | `{"username","email","password"}` → `{"id","username"}`, sets session and refresh cookies. Username 5-32 characters, password at least 12 |
| ✅     | `POST` | `/api/auth/login`    | Public    | `{"username","password"}` → `{"id","username"}`, sets session and refresh cookies                    |
| ✅     | `POST` | `/api/auth/refresh`  | Public    | Rotates the refresh cookie and issues a new session cookie                                           |
| ✅     | `POST` | `/api/auth/logout`   | Public    | Revokes the refresh tokens and clears both cookies                                                   |
| ✅     | `GET`  | `/api/auth/me`       | Logged in | `{"id","username","score","role","rank"}`                                                            |
| ✅     | `PUT`  | `/api/auth/password` | Logged in | `{"current_password","new_password"}` → `{"ok":true}`, logs out other devices and issues new cookies |
| ✅     | `GET`  | `/api/auth/solves`   | Public    | IDs of the challenges the user solved, `[]` when logged out                                          |

### Users

| Status | Method  | Path                    | Who               | Purpose                                         |
| ------ | ------- | ----------------------- | ----------------- | ----------------------------------------------- |
| 📝     | `GET`   | `/api/users/{id}`       | user:read_public  | Public profile                                  |
| 📝     | `PATCH` | `/api/users/me`         | user:update       | Update own profile (see `GET /api/auth/me` to read it) |

### Teams

A team object is `{"id","name","captain_id","banned","hidden","member_count"}`. `captain_id` is `null` when the captain's account was deleted. Team names have surrounding whitespace trimmed, must be 3-32 characters without control characters, and are unique regardless of case. Hidden teams are left out of `GET /api/teams` and answer `404` on `GET /api/teams/{id}`, but their members can still read and manage them, and admins see them.

| Status | Method   | Path                              | Who                   | Request → Response                                                                         |
| ------ | -------- | --------------------------------- | --------------------- | ------------------------------------------------------------------------------------------ |
| ✅     | `GET`    | `/api/teams`                      | team:read_public      | `?limit` (1-100, default 50) and `?offset` → array of team objects sorted by name. No invite codes |
| ✅     | `GET`    | `/api/teams/{id}`                 | team:read_public      | Team object. The invite code is never included                                             |
| ✅     | `POST`   | `/api/teams`                      | team:create           | `{"name"}` → `{"id"}`. The creator becomes captain                                         |
| ✅     | `GET`    | `/api/teams/mine`                 | team:read_private     | Team object plus `"invite_code"` and `"members": [{"id","username","score"}]`. `404` when not in a team |
| ✅     | `PATCH`  | `/api/teams/mine`                 | team:update           | `{"name"?,"regenerate_invite"?}` → `{"ok":true}`, plus the new `"invite_code"` when one was generated. Captain only. Fields left out are unchanged; `400` when neither is given |
| ✅     | `POST`   | `/api/teams/join`                 | team:join             | `{"invite_code"}` → `{"ok":true}`. Refused when the team is full or banned                 |
| ✅     | `POST`   | `/api/teams/mine/leave`           | team:leave            | `{"ok":true}`. A captain with other members must transfer first. The last member leaving deletes the team |
| ✅     | `DELETE` | `/api/teams/mine/members/{uid}`   | team:kick             | `{"ok":true}`. Captain only. Use leave to remove yourself                                  |
| ✅     | `PUT`    | `/api/teams/mine/captain`         | team:transfer_captain | `{"user_id"}` → `{"ok":true}`. The new captain must be a member                            |

Joining, leaving and kicking are refused for non-admins after the CTF has ended (`RosterFrozen`).

Members of a banned team can still read their team (`GET /api/teams/mine`), but every other team action is refused: update, kick, transfer captain, leave and delete (`DenyBannedTeam`). Only an admin can unban the team through `PATCH /api/admin/teams/{id}`.

### Challenges

| Status | Method | Path                                  | Who                   | Purpose                        |
| ------ | ------ | ------------------------------------- | --------------------- | ------------------------------ |
| 📝     | `GET`  | `/api/challenges`                     | challenge:read        | List visible challenges        |
| 📝     | `GET`  | `/api/challenges/categories`          | challenge:read        | List visible categories        |
| 📝     | `GET`  | `/api/challenges/{cid}`               | challenge:read        | Challenge details              |
| 📝     | `GET`  | `/api/challenges/{cid}/solves`        | challenge:view_solves | Which teams have solved it     |
| 📝     | `GET`  | `/api/challenges/{cid}/files/{fid}`   | challenge:download    | Download attachments           |
| 📝     | `POST` | `/api/challenges/{cid}/submit`        | challenge:submit      | Submit flag for this challenge |
| 📝     | `POST` | `/api/challenges/submit`              | challenge:submit      | Submit flag for any challenge  |

### Instances

| Status | Method   | Path                                       | Who             | Purpose                         |
| ------ | -------- | ------------------------------------------ | --------------- | ------------------------------- |
| 📝     | `GET`    | `/api/challenges/{cid}/instance`           | instance:read   | Info about your team's instance |
| 📝     | `POST`   | `/api/challenges/{cid}/instance`           | instance:start  | Start your team's instance      |
| 📝     | `POST`   | `/api/challenges/{cid}/instance/reset`     | instance:reset  | Restart your team's instance    |
| 📝     | `POST`   | `/api/challenges/{cid}/instance/extend`    | instance:extend | Push the expiry time back       |
| 📝     | `DELETE` | `/api/challenges/{cid}/instance`           | instance:stop   | Stop your team's instance       |

### Scoreboard

| Status | Method | Path                          | Who    | Purpose                 |
| ------ | ------ | ----------------------------- | ------ | ----------------------- |
| 📝     | `GET`  | `/api/scoreboard`             | Public | Team ranking and score  |
| 📝     | `GET`  | `/api/scoreboard/history`     | Public | Score over time (graph) |

### Administration

A settings object is `{"ctf_name","ctf_start","ctf_end","registration","max_team_size","updated_at"}`. Times are RFC 3339.

| Status | Method   | Path                                           | Who                    | Purpose                                                       |
| ------ | -------- | ---------------------------------------------- | ---------------------- | ------------------------------------------------------------- |
| 📝     | `GET`    | `/api/admin/users`                             | user:list              | List and search users                                         |
| 📝     | `POST`   | `/api/admin/users`                             | user:create            | Create a user                                                 |
| 📝     | `GET`    | `/api/admin/users/{id}`                        | user:read_private      | Full details about a user                                     |
| 📝     | `PATCH`  | `/api/admin/users/{id}`                        | user:update + user:ban | Edit or ban a user                                            |
| 📝     | `PUT`    | `/api/admin/users/{id}/role`                   | user:set_role          | Change role                                                   |
| 📝     | `DELETE` | `/api/admin/users/{id}`                        | user:delete            | Delete a user                                                 |
| ✅     | `GET`    | `/api/admin/teams`                             | team:read_private      | `?limit` (1-100, default 50), `?offset` and `?q` (name contains, ignoring case, at most 32 characters) → array of team objects sorted by name, including hidden teams and `"invite_code"` |
| ✅     | `GET`    | `/api/admin/teams/{id}`                        | team:read_private      | Team object plus `"invite_code"` and `"members": [{"id","username","score"}]`, hidden teams included |
| ✅     | `PATCH`  | `/api/admin/teams/{id}`                        | team:update (+ team:ban, team:transfer_captain) | `{"name"?,"banned"?,"hidden"?,"captain_id"?}` → `{"ok":true}`. Fields left out are unchanged; `400` when none is given. Setting `banned` also needs team:ban, `captain_id` also needs team:transfer_captain and must be a member |
| ✅     | `DELETE` | `/api/admin/teams/{id}/members/{uid}`          | team:kick              | `{"ok":true}`. Kicking the last member deletes the team. Kicking the captain makes the oldest remaining account captain. `404` when the user isn't a member |
| ✅     | `DELETE` | `/api/admin/teams/{id}`                        | team:delete            | `{"ok":true}`. Members are left without a team                |
| 📝     | `GET`    | `/api/admin/instances`                         | instance:read          | List all running instances                                    |
| 📝     | `DELETE` | `/api/admin/instances/{id}`                    | instance:stop          | Kill any instance                                             |
| 📝     | `GET`    | `/api/admin/submissions`                       | submission:read        | View all flag attempts                                        |
| 📝     | `DELETE` | `/api/admin/submissions/{id}`                  | submission:delete      | Delete a flag attempt                                         |
| 📝     | `POST`   | `/api/admin/announcements`                     | announcement:create    | Create an announcement                                        |
| 📝     | `DELETE` | `/api/admin/announcements/{id}`                | announcement:delete    | Delete an announcement                                        |
| 📝     | `GET`    | `/api/admin/audit-log`                         | audit:read             | View audit logs                                               |
| ✅     | `GET`    | `/api/admin/settings`                          | settings:read          | Settings object                                               |
| ✅     | `PUT`    | `/api/admin/settings`                          | settings:update        | Any of `{"ctf_name","ctf_start","ctf_end","registration","max_team_size"}` → settings object. Fields left out are unchanged. Name 1-64 characters, team size at least 1, start before end |
| 📝     | `POST`   | `/api/admin/challenges`                        | challenge:create       | Create a challenge                                            |
| 📝     | `PATCH`  | `/api/admin/challenges/{cid}`                  | challenge:update       | Edit a challenge                                              |
| 📝     | `DELETE` | `/api/admin/challenges/{cid}`                  | challenge:delete       | Delete a challenge                                            |
| 📝     | `POST`   | `/api/admin/challenges/{cid}/files`            | challenge:manage_files | Upload files to a challenge                                   |
| 📝     | `DELETE` | `/api/admin/challenges/{cid}/files/{fid}`      | challenge:manage_files | Delete files from a challenge                                 |
## Adding an API

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
	var username string
	err := h.pool.QueryRow(r.Context(), `SELECT username FROM users WHERE id = $1`, id).Scan(&username)
	if database.IsNotFound(err) {
		httpx.Error(w, http.StatusNotFound, "User not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"id": id, "username": username})
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
	var role string
	err := h.pool.QueryRow(r.Context(), `SELECT role FROM users WHERE id = $1`, id).Scan(&role)
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
		Attributes: map[string]any{"role": role},
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
- **Permissions:** never check `u.Role` directly; ask PBAC (see [PBAC](#pbac)).
  - The action doesn't depend on a resource (`challenge:create`, `user:list`, `settings:*`): wrap the route in `a.Authorise(action, h)`.
  - It depends on one resource: wrap in `a.Optional`, load the row, then call `a.IsAllowed(r, u, action, resource)` and `auth.Deny(w, u)` when refused.
  - Filtering a list: call `check, err := a.Checker(r, u)` once and `check(action, resource)` per row, so the subject and env are loaded only once.
  - A new route also needs a statement in [`internal/policies/default.json`](internal/policies/default.json) whose `resources` cover its path, or every request is refused.
  - The check must agree with a write: get `check` before the transaction, lock the row inside `pgx.BeginFunc` (`SELECT ... FOR UPDATE`), call `check` on the locked row and return `auth.ErrDenied` when refused. Then answer the error with `auth.Fail(w, r, u, err, "Team not found")`, which maps a missing row to `404`, `ErrDenied` to `401`/`403` and anything else to `500`. [`internal/teams`](internal/teams/routes.go) does this for join, leave, kick, transfer captain and update.
  - Answer `404` when the user may not even read the resource, so hidden challenges can't be discovered.
- **Request bodies:** `httpx.Decode` caps them at 1 MiB and responds `422` itself when the JSON is invalid.
- **Responses:** `httpx.JSON` for success, `httpx.Error` for client errors, `httpx.InternalError` for unexpected errors. It logs the error and hides it from the client.
- **Database:** use `database.IsNotFound` and `database.IsUniqueViolation` to check errors. Use `pgx.BeginFunc` when several writes must succeed together. Schema changes go in a new migration in `internal/database/migrations/`, which runs on startup.

## PBAC

Authorisation is policy based. The rules are JSON in [`internal/policies/default.json`](internal/policies/default.json), which `main` loads at startup from `POLICY_FILE`. A missing or invalid file stops the server. [`internal/policy`](internal/policy/engine.go) is the engine, which is pure Go with no database or HTTP. [`internal/auth`](internal/auth/authorise.go) collects the attributes a check needs and connects it to HTTP.

### How a check works

```
a.Authorise(action, h) / a.IsAllowed(r, u, action, resource)
  └─ a.Checker(r, u)
       ├─ a.Subject(ctx, u)   who: id, role, banned, team, captain, team banned    (users + teams)
       ├─ a.Env(ctx)          context: ctf_phase, max_team_size, admin_count, ...  (settings + users)
       └─ engine.Evaluate(request)
            1. a statement matches when its subjects, actions and resources patterns match
               and all of its conditions hold
            2. any matching deny statement  → refused
            3. any matching allow statement → allowed, otherwise refused
```

A resource's `Path` defaults to the request path (`r.URL.Path`), so statements' `resources` are API paths. A logged out visitor has no ID and no roles, so only statements for `"*"` apply. A refusal is `401` for visitors and `403` for logged-in users (`auth.Deny`). Refusals are logged at debug level with the deciding policy and statement.

The handler builds the `policy.Resource` (ID and attributes) from the row it loaded. **Every attribute a deny statement reads must be set**, because a missing attribute makes the condition fail and the deny not apply: `role` for users (`KeepLastAdmin`), and `member_count` and `banned` for teams (`CaptainMustTransfer`, `NoJoinBannedTeam`, `TeamJoinNotFull`). See `team.resource()` in [`internal/teams/store.go`](internal/teams/store.go).

### Policy file

```json
[
  {
    "id": "player",
    "statements": [
      {
        "sid": "TeamCaptain",
        "effect": "allow",
        "subjects": ["role:user", "role:maintainer", "role:admin"],
        "actions": ["team:update", "team:kick", "team:transfer_captain"],
        "resources": ["/api/teams/mine", "/api/teams/mine/*"],
        "conditions": [
          { "operator": "Equals", "key": "subject.is_captain", "value": true },
          { "operator": "Equals", "key": "resource.id", "value": "${subject.team_id}" }
        ]
      }
    ]
  }
]
```

- **subjects:** `role:<role>`, `user:<id>` or `*`. Roles don't inherit, so list every role a statement applies to.
- **actions:** see [Actions](#actions). `*` and `team:*` work too.
- **resources:** API paths. `*` matches any characters including `/`, so `/api/teams/*` doesn't match `/api/teams` itself; list both when needed.
- **conditions:** all must hold. Operators are `Equals`, `NotEquals`, `In` and `NotIn` (value is a list), `LessThan` and `GreaterThan`. A value `"${scope.attr}"` compares against another attribute. A condition on a missing attribute fails.

The loader rejects unknown fields, effects, operators, condition keys and actions at startup, so a typo can't silently disable a rule. [`internal/teams/routes_pbac_test.go`](internal/teams/routes_pbac_test.go) calls every route as every kind of user; add the new routes there when you change a rule.

### Attributes

| Key                     | Type   | Notes                                                       |
|-------------------------|--------|-------------------------------------------------------------|
| `subject.id`            | string | empty for a visitor                                         |
| `subject.role`          | string | `user`, `maintainer`, `admin`, or empty for a visitor       |
| `subject.banned`        | bool   |                                                             |
| `subject.has_team`      | bool   |                                                             |
| `subject.team_id`       | string | empty when not in a team                                    |
| `subject.is_captain`    | bool   | captain of `subject.team_id`                                |
| `subject.team_banned`   | bool   | `subject.team_id` is banned                                 |
| `resource.id`           | string | the loaded row's ID                                         |
| `resource.path`         | string | the request path                                            |
| `resource.*`            | any    | set by the handler, e.g. `member_count`, `banned`, `hidden` (teams), `visible`, `needs_instance`, `owner_id` (challenges), `team_id` (instances), `role` (users) |
| `context.ctf_phase`     | string | `not_started`, `running` or `ended`                         |
| `context.ctf_start`, `context.ctf_end`, `context.now` | number | Unix seconds                  |
| `context.max_team_size` | number |                                                             |
| `context.admin_count`   | number | users with role `admin`                                     |
| `request.action`        | string | the action being checked                                    |

### Actions

The constants live in [`internal/policy/actions.go`](internal/policy/actions.go). Add new actions there before using them in the policy file.

| Resource     | Actions                                                                                       |
|--------------|-----------------------------------------------------------------------------------------------|
| challenge    | read, download, view_solves, read_secrets, create, update, manage_files, delete, submit       |
| instance     | start, read, reset, extend, stop                                                              |
| team         | create, join, read_private, read_public, update, leave, kick, transfer_captain, delete, ban   |
| user         | read_private, read_public, update, manage_keys, create, ban, set_role, revoke_keys, delete, list, list_keys |
| submission   | read, delete                                                                                  |
| announcement | create, delete                                                                                |
| settings     | read, update                                                                                  |
| audit        | read                                                                                          |

### Default policies

| Policy       | Applies to                 | Statements                                                                 |
|--------------|----------------------------|----------------------------------------------------------------------------|
| `system-deny`| everyone                   | DenyBanned, DenyBannedTeam, NoJoinBannedTeam, NoSelfRoleChange, KeepLastAdmin, CaptainMustTransfer, RosterFrozen |
| `public`     | everyone, also visitors    | PublicRead                                                                 |
| `player`     | user, maintainer, admin    | PlayerChallengeRead, PlayerSubmit, TeamInstanceStart, TeamInstanceControl, SelfAccount, TeamCreate, TeamJoinUnlimited, TeamJoinNotFull, TeamMember, TeamCaptain |
| `maintainer` | maintainer, admin          | ChallengeAuthoring, ChallengeOwner, ChallengeReadAll, InstanceOperator     |
| `admin`      | admin                      | ChallengeAdmin, InstanceAdmin, UserAdmin, TeamAdmin, SubmissionAdmin, EventAdmin |

The deny statements:

| Statement           | Actions                                                       | Denies when                                           |
|---------------------|---------------------------------------------------------------|-------------------------------------------------------|
| DenyBanned          | * except user:read_public, team:read_public                   | the subject is banned                                 |
| DenyBannedTeam      | challenge:submit, instance:start, team:update, team:kick, team:transfer_captain, team:leave, team:delete | the subject's team is banned (player routes only) |
| NoJoinBannedTeam    | team:join                                                     | the team being joined is banned                       |
| NoSelfRoleChange    | user:set_role, user:ban, user:delete                          | the target user is the subject                        |
| KeepLastAdmin       | user:set_role, user:ban, user:delete                          | the target is an admin and there is only one          |
| CaptainMustTransfer | team:leave                                                    | the subject is captain and the team has other members |
| RosterFrozen        | team:join, team:leave, team:kick                              | the CTF has ended and the subject isn't an admin      |

A full team is handled on the allow side instead: `team:join` is allowed by `TeamJoinUnlimited` (`max_team_size` is 0) or `TeamJoinNotFull` (`member_count < max_team_size`), because the operators have no `>=`.

## Layout

```
cmd/server/            entrypoint: config, database, default admin, policies, server lifecycle
internal/config/       environment configuration
internal/database/     connection pool and migrations
  migrations/          goose migrations, embedded in the binary
internal/httpserver/   route wiring (app.go), http.Server, request logging middleware
internal/httpx/        JSON request/response helpers shared by every feature
internal/health/       GET /health
internal/auth/         /api/auth endpoints, session cookies, Require/Optional, default admin,
                       PBAC glue: Subject/Env, Checker/IsAllowed/Authorise/Deny/Fail
  passwords/           password hashing
  sessions/            session and refresh tokens
internal/policy/       PBAC engine: policy model, JSON loader, matching, conditions, action constants
internal/policies/     default.json (the PBAC rules, loaded by main) and its tests
internal/teams/        /api/teams and /api/admin/teams endpoints
internal/settings/     /api/admin/settings endpoints
```

## Docker

```sh
docker build --target development -t openctf-backend:dev .
docker build --target production  -t openctf-backend .
```
