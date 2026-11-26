# API

Every route lives under `/api/`, except the `/health` probe. Admin-only routes live under `/api/admin/`.

- **Status:** ✅ is implemented. 🚧 is implemented but not registered in [`app.go`](internal/httpserver/app.go) yet. 📝 is planned.
- **Who:** the PBAC action the route checks (see [PBAC](policies.md)). _Public_ means anyone, including logged-out visitors. _Logged in_ means only a session is required.
- **Errors:** always `{"detail": "message"}`. A refused permission is `401` for visitors and `403` for logged-in users. A resource the user may not see is `404`. Invalid JSON is `422`.
- **Responses:** routes that only change something answer `{"ok":true}`.

## Health

| Status | Method | Path      | Who    | Request → Response                                                         |
| ------ | ------ | --------- | ------ | -------------------------------------------------------------------------- |
| ✅     | `GET`  | `/health` | Public | `200 {"status":"ok","database":"up"}`, or `503` if Postgres is unreachable |

## Authentication

| Status | Method | Path                 | Who       | Request → Response                                                                                   |
| ------ | ------ | -------------------- | --------- | ---------------------------------------------------------------------------------------------------- |
| ✅     | `POST` | `/api/auth/register` | Public    | `{"username","email","password"}` → `{"id","username"}`, sets session and refresh cookies. Username 5-32 characters, password at least 12 |
| ✅     | `POST` | `/api/auth/login`    | Public    | `{"username","password"}` → `{"id","username"}`, sets session and refresh cookies. `409` if already logged in |
| ✅     | `POST` | `/api/auth/refresh`  | Public    | Rotates the refresh cookie and issues a new session cookie                                           |
| ✅     | `POST` | `/api/auth/logout`   | Public    | Revokes the refresh tokens and clears both cookies                                                   |
| ✅     | `GET`  | `/api/auth/me`       | Logged in | `{"id","username","score","role","rank"}`                                                            |
| ✅     | `PUT`  | `/api/auth/password` | Logged in | `{"current_password","new_password"}` → `{"ok":true}`, logs out other devices and issues new cookies |
| ✅     | `GET`  | `/api/auth/solves`   | Public    | IDs of the challenges the user solved, `[]` when logged out                                          |

## Users

| Status | Method  | Path                    | Who               | Purpose                                         |
| ------ | ------- | ----------------------- | ----------------- | ----------------------------------------------- |
| 📝     | `GET`   | `/api/users/{id}`       | user:read_public  | Public profile                                  |
| 📝     | `PATCH` | `/api/users/me`         | user:update       | Update own profile (see `GET /api/auth/me` to read it) |

## Teams

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

## Challenges

| Status | Method | Path                                  | Who                   | Purpose                        |
| ------ | ------ | ------------------------------------- | --------------------- | ------------------------------ |
| 📝     | `GET`  | `/api/challenges`                     | challenge:read        | List visible challenges        |
| 📝     | `GET`  | `/api/challenges/categories`          | challenge:read        | List visible categories        |
| 📝     | `GET`  | `/api/challenges/{cid}`               | challenge:read        | Challenge details              |
| 📝     | `GET`  | `/api/challenges/{cid}/solves`        | challenge:view_solves | Which teams have solved it     |
| 📝     | `GET`  | `/api/challenges/{cid}/files/{fid}`   | challenge:download    | Download attachments           |
| 📝     | `POST` | `/api/challenges/{cid}/submit`        | challenge:submit      | Submit flag for this challenge |
| 📝     | `POST` | `/api/challenges/submit`              | challenge:submit      | Submit flag for any challenge  |

## Instances

| Status | Method   | Path                                       | Who             | Purpose                         |
| ------ | -------- | ------------------------------------------ | --------------- | ------------------------------- |
| 📝     | `GET`    | `/api/challenges/{cid}/instance`           | instance:read   | Info about your team's instance |
| 📝     | `POST`   | `/api/challenges/{cid}/instance`           | instance:start  | Start your team's instance      |
| 📝     | `POST`   | `/api/challenges/{cid}/instance/reset`     | instance:reset  | Restart your team's instance    |
| 📝     | `POST`   | `/api/challenges/{cid}/instance/extend`    | instance:extend | Push the expiry time back       |
| 📝     | `DELETE` | `/api/challenges/{cid}/instance`           | instance:stop   | Stop your team's instance       |

## Scoreboard

| Status | Method | Path                          | Who    | Purpose                 |
| ------ | ------ | ----------------------------- | ------ | ----------------------- |
| 📝     | `GET`  | `/api/scoreboard`             | Public | Team ranking and score  |
| 📝     | `GET`  | `/api/scoreboard/history`     | Public | Score over time (graph) |

## Administration

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
