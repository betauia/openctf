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

A team object is `{"id","name","captain_id","banned","member_count"}`. `captain_id` is `null` when the captain's account was deleted. Team names are 3-32 characters and unique.

| Status | Method   | Path                              | Who                   | Request → Response                                                                         |
| ------ | -------- | --------------------------------- | --------------------- | ------------------------------------------------------------------------------------------ |
| 📝     | `GET`    | `/api/teams`                      | team:read_public      | List and search all visible teams                                                          |
| ✅     | `GET`    | `/api/teams/{id}`                 | team:read_public      | Team object. The invite code is never included                                             |
| ✅     | `POST`   | `/api/teams`                      | team:create           | `{"name"}` → `{"id"}`. The creator becomes captain                                         |
| ✅     | `GET`    | `/api/teams/mine`                 | team:read_private     | Team object plus `"invite_code"` and `"members": [{"id","username","score"}]`. `404` when not in a team |
| 📝     | `PATCH`  | `/api/teams/mine`                 | team:update           | Rename the team, edit team fields                                                          |
| ✅     | `POST`   | `/api/teams/join`                 | team:join             | `{"invite_code"}` → `{"ok":true}`. Refused when the team is full or banned                 |
| ✅     | `POST`   | `/api/teams/mine/leave`           | team:leave            | `{"ok":true}`. A captain with other members must transfer first. The last member leaving deletes the team |
| ✅     | `DELETE` | `/api/teams/mine/members/{uid}`   | team:kick             | `{"ok":true}`. Captain only. Use leave to remove yourself                                  |
| ✅     | `PUT`    | `/api/teams/mine/captain`         | team:transfer_captain | `{"user_id"}` → `{"ok":true}`. The new captain must be a member                            |

Joining, leaving and kicking are refused for non-admins after the CTF has ended (`RosterFrozen`).

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
| 📝     | `GET`    | `/api/admin/teams`                             | team:read_private      | List and search teams                                         |
| 📝     | `GET`    | `/api/admin/teams/{id}`                        | team:read_private      | Full details about a team                                     |
| ✅     | `PATCH`  | `/api/admin/teams/{id}`                        | team:update + team:ban | `{"name"?,"banned"?}` → `{"ok":true}`. Setting `banned` also needs team:ban |
| 📝     | `DELETE` | `/api/admin/teams/{id}/members/{uid}`          | team:kick              | Remove a member from a team                                   |
| ✅     | `DELETE` | `/api/admin/teams/{id}`                        | team:delete            | `{"ok":true}`. Members are left without a team                |
| 📝     | `GET`    | `/api/admin/instances`                         | instance:read          | List all running instances                                    |
| 📝     | `DELETE` | `/api/admin/instances/{id}`                    | instance:stop          | Kill any instance                                             |
| 📝     | `GET`    | `/api/admin/submissions`                       | submission:read        | View all flag attempts                                        |
| 📝     | `DELETE` | `/api/admin/submissions/{id}`                  | submission:delete      | Delete a flag attempt                                         |
| 📝     | `POST`   | `/api/admin/announcements`                     | announcement:create    | Create an announcement                                        |
| 📝     | `DELETE` | `/api/admin/announcements/{id}`                | announcement:delete    | Delete an announcement                                        |
| 📝     | `GET`    | `/api/admin/audit-log`                         | audit:read             | View audit logs                                               |
| 🚧     | `GET`    | `/api/admin/settings`                          | settings:read          | Settings object                                               |
| 🚧     | `PUT`    | `/api/admin/settings`                          | settings:update        | Any of `{"ctf_name","ctf_start","ctf_end","registration","max_team_size"}` → settings object. Fields left out are unchanged. Name 1-64 characters, team size at least 1, start before end |
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
	"github.com/betauia/openctf/backend/internal/authz"
	"github.com/betauia/openctf/backend/internal/database"
	"github.com/betauia/openctf/backend/internal/httpx"
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
	mux.HandleFunc("GET /api/admin/users", h.auth.Authorise(authz.UserList, h.list))    // permission, no resource
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

// ban loads the target first, because the policies depend on it (is_self, target_is_last_admin).
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
	allowed, err := h.auth.IsAllowed(r.Context(), u, authz.UserBan, authz.Resource{ID: id, Role: authz.Role(role)})
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
  - It depends on one resource: wrap in `a.Optional`, load the row, then call `a.IsAllowed(ctx, u, action, resource)` and `auth.Deny(w, u)` when refused.
  - Filtering a list: call `check, err := a.Checker(ctx, u)` once and `check(action, resource)` per row, so the subject and env are loaded only once.
  - The check must agree with a write: get `check` before the transaction, lock the row inside `pgx.BeginFunc` (`SELECT ... FOR UPDATE`), call `check` on the locked row and return `auth.ErrDenied` when refused. Then answer the error with `auth.Fail(w, r, u, err, "Team not found")`, which maps a missing row to `404`, `ErrDenied` to `401`/`403` and anything else to `500`. [`internal/teams`](internal/teams/routes.go) does this for join, leave and kick.
  - Answer `404` when the user may not even read the resource, so hidden challenges can't be discovered.
- **Request bodies:** `httpx.Decode` caps them at 1 MiB and responds `422` itself when the JSON is invalid.
- **Responses:** `httpx.JSON` for success, `httpx.Error` for client errors, `httpx.InternalError` for unexpected errors. It logs the error and hides it from the client.
- **Database:** use `database.IsNotFound` and `database.IsUniqueViolation` to check errors. Use `pgx.BeginFunc` when several writes must succeed together. Schema changes go in a new migration in `internal/database/migrations/`, which runs on startup.

## PBAC

Authorisation is policy based. The rules live in [`internal/authz/pbac.go`](internal/authz/pbac.go), which is pure Go with no database or HTTP, and are tested in [`pbac_test.go`](internal/authz/pbac_test.go). [`internal/auth`](internal/auth/authorise.go) collects the facts a check needs and connects it to HTTP.

### How a check works

```
a.Authorise(action, h) / a.IsAllowed(ctx, u, action, resource)
  └─ a.Checker(ctx, u)
       ├─ a.Subject(ctx, u)   who: id, role, banned, team, captain, team banned   (users + teams)
       ├─ a.Env(ctx)          when: ctf_start, ctf_end, max_team_size, admin count (settings + users)
       └─ authz.IsActionAllowed(subject, action, resource, env)
            1. take SystemPolicies + EveryonePolicies + the policies of the subject's role
            2. a policy matches when it lists the action and all its conditions hold
            3. any matching deny policy  → refused
            4. any matching allow policy → allowed, otherwise refused
```

A logged out visitor is an empty `Subject` with no role, so only the public and system policies apply. A refusal is `401` for visitors and `403` for logged-in users (`auth.Deny`).

The handler builds the `authz.Resource` from the row it loaded. **Every field a deny policy reads must be set**, because an empty field makes that policy not apply: `Role` for users (`KeepLastAdmin`), and `MemberCount` and `Banned` for teams (`TeamFull`, `CaptainMustTransfer`, `NoJoinBannedTeam`).

### Conditions

| Condition      | True when                            |
|----------------|--------------------------------------|
| always         | always                               |
| is_self        | resource.id == subject.id            |
| is_owner       | resource.created_by_id == subject.id |
| has_team       | subject.team_id is set               |
| no_team        | subject.team_id is not set           |
| is_member      | subject.team_id == resource.id       |
| is_captain     | subject.is_captain and is_member     |
| same_team      | resource.team_id == subject.team_id  |
| published      | not resource.hidden                  |
| needs_instance | the challenge has a docker_image     |
| ctf_started    | now >= ctf_start                     |
| ctf_running    | ctf_start <= now < ctf_end           |
| ctf_ended      | now >= ctf_end                       |

A zero subject ID or team ID never matches, so a visitor is never `is_self`, `is_owner` or a team member. A condition on `ctf_start` or `ctf_end` fails when that time isn't set. Unknown conditions fail.

Used by the system (deny) policies:

| Condition            | True when                                         |
|----------------------|---------------------------------------------------|
| subject_banned       | subject.banned                                    |
| team_banned          | the subject's team is banned                      |
| target_banned        | the resource itself is banned (teams)             |
| target_is_last_admin | resource.role is admin and there is at most one admin |
| team_has_others      | resource.member_count > 1                         |
| team_full            | max_team_size > 0 and member_count >= max_team_size |
| not_admin            | the subject's role isn't admin                    |

### Actions

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

### Allow policies

#### Player Policies

| Policy              | Action                                                        | Resource  | Conditions                                       |
|---------------------|---------------------------------------------------------------|-----------|--------------------------------------------------|
| PlayerChallengeRead | challenge:read, challenge:download, challenge:view_solves     | Challenge | published, ctf_started                           |
| PlayerSubmit        | challenge:submit                                              | Challenge | published, has_team, ctf_running                 |
| TeamInstanceStart   | instance:start                                                | Challenge | published, needs_instance, has_team, ctf_running |
| TeamInstanceControl | instance:read, instance:reset, instance:extend, instance:stop | Instance  | same_team, ctf_running                           |
| SelfAccount         | user:read_private, user:update, user:manage_keys, user:list_keys, user:revoke_keys | User | is_self                             |
| TeamFounding        | team:create, team:join                                        | Team      | no_team                                          |
| TeamMember          | team:read_private, team:leave                                 | Team      | is_member                                        |
| TeamCaptain         | team:update, team:kick, team:transfer_captain, team:delete   | Team      | is_captain                                       |

#### Admin policies

| Policy             | Action                                                                                                 | Resource   | Conditions |
|--------------------|--------------------------------------------------------------------------------------------------------|------------|------------|
| ChallengeAuthoring | challenge:create                                                                                       | Challenge  | always     |
| ChallengeOwner     | challenge:update, challenge:read_secrets, challenge:manage_files                                       | Challenge  | is_owner   |
| ChallengeReadAll   | challenge:read, challenge:download, challenge:view_solves                                              | Challenge  | always     |
| InstanceOperator   | instance:read, instance:stop                                                                           | Instance   | always     |
| ChallengeAdmin     | challenge:update, challenge:read_secrets, challenge:manage_files, challenge:delete                     | Challenge  | always     |
| InstanceAdmin      | instance:read, instance:reset, instance:stop, instance:extend                                           | Instance   | always     |
| UserAdmin          | user:list, user:read_private, user:create, user:update, user:ban, user:set_role, user:revoke_keys, user:delete | User       | always     |
| TeamAdmin          | team:read_private, team:update, team:ban, team:kick, team:delete                                      | Team       | always     |
| SubmissionAdmin    | submission:read, submission:delete                                                                     | Submission | always     |
| EventAdmin         | settings:read, settings:update, announcement:create, announcement:delete, audit:read               | -          | always     |

#### Public policy

Applies to everyone, also logged out visitors.

| Policy     | Action                          | Resource   | Conditions |
|------------|---------------------------------|------------|------------|
| PublicRead | user:read_public, team:read_public | User, Team | always     |

#### Role Mapping

Each role inherits every policy of the role above it in the table, so everyone can play. A role not listed here gets only the public and system policies.

| Role       | Inherits   | Attached policies                                                                                                              |
|------------|------------|--------------------------------------------------------------------------------------------------------------------------------|
| user       | -          | PlayerChallengeRead, PlayerSubmit, TeamInstanceStart, TeamInstanceControl, SelfAccount, TeamFounding, TeamMember, TeamCaptain |
| maintainer | user       | ChallengeAuthoring, ChallengeOwner, ChallengeReadAll, InstanceOperator                                                        |
| admin      | maintainer | ChallengeAdmin, InstanceAdmin, UserAdmin, TeamAdmin, SubmissionAdmin, EventAdmin                                              |

#### System Policies (always active)

These apply to every subject, whatever its role, and a match always refuses.

| Policy              | Effect | Action                               | Denies when                                           |
|---------------------|--------|--------------------------------------|-------------------------------------------------------|
| DenyBanned          | Deny   | *                                    | subject.banned                                        |
| DenyBannedTeam      | Deny   | challenge:submit, instance:start, team:kick | the subject's team is banned                   |
| NoJoinBannedTeam    | Deny   | team:join                            | the team being joined is banned                       |
| NoSelfRoleChange    | Deny   | user:set_role                        | the target user is the subject                        |
| NoSelfDestruct      | Deny   | user:ban, user:delete                | the target user is the subject                        |
| KeepLastAdmin       | Deny   | user:set_role, user:ban, user:delete | the target is the only admin                          |
| CaptainMustTransfer | Deny   | team:leave                           | the subject is captain and the team has other members |
| TeamFull            | Deny   | team:join                            | member_count >= settings.max_team_size                |
| RosterFrozen        | Deny   | team:join, team:leave, team:kick     | ctf_ended, and the subject isn't an admin             |

## Layout

```
cmd/server/            entrypoint: config, database, default admin, server lifecycle
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
internal/authz/        PBAC: actions, conditions, policies, roles, IsActionAllowed
internal/teams/        /api/teams and /api/admin/teams endpoints
internal/settings/     /api/admin/settings endpoints (not registered in app.go yet)
```

## Docker

```sh
docker build --target development -t openctf-backend:dev .
docker build --target production  -t openctf-backend .
```
