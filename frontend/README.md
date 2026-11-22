# OpenCTF frontend

SvelteKit 2 + Svelte 5 (runes), Tailwind v4, TypeScript. Server-rendered via
`adapter-node`, on port 4000.

```sh
npm install
npm run dev      # vite dev
npm run check    # svelte-check, must stay at 0 errors
npm run build    # production build
```

Needs the Go backend reachable at `BACKEND_URL` (default `http://backend:8000`,
the compose service name).

## Status: blocked on the backend

This branch is the finished frontend for an API that is only partly implemented.
Nothing here is mocked — every page calls the real endpoints and fails visibly
when they are missing, so the gaps stay obvious:

| Area | Endpoint | Backend |
| --- | --- | --- |
| Auth | `POST /api/auth/{login,register,logout}`, `GET /api/auth/me`, `GET /api/auth/solves` | implemented |
| Teams | `GET/POST /api/teams`, `GET /api/teams/mine`, `POST /api/teams/join`, `POST /api/teams/mine/leave`, `DELETE /api/teams/mine/members/{uid}` | implemented |
| Settings | `GET/PUT /api/admin/settings` | implemented |
| Public settings | `GET /api/settings` | **missing** |
| Challenges | `GET /api/challenges`, `POST /api/challenges/submit`, `GET /api/challenges/{id}/solves`, `GET /api/challenges/{id}/files/{name}` | **missing** |
| Instances | `GET/POST /api/instances`, `DELETE /api/instances/{id}` | **missing** |
| Members | `GET /api/teams/members/{id}` | **missing** (existed in the Python backend) |
| Activity | `GET /api/teams/{id}/activity` | **missing** (existed in the Python backend) |

Shapes the frontend expects from the endpoints that are still to be written:

- `POST /api/challenges/submit` takes `{flag, challenge_id?}` and returns
  `{correct, challenge_id, points}`. `challenge_id` in the request scopes the
  check to one challenge (the modal); omitting it lets the backend work out which
  challenge the flag belongs to (the global flag bar). Returning the id means the
  UI never has to match challenges by title.
- `GET /api/challenges` should return challenges in display order; the frontend
  does not re-sort them.
- `GET /api/settings` should return the publicly readable settings —
  `ctf_name`, `ctf_start`, `ctf_end`, `registration`, `max_team_size` and
  `rules` — with no auth. `/api/admin/settings` is gated behind SettingsRead, so
  nothing public can read the CTF name or the rules today. The `settings` table
  also needs a `rules` column; it is organiser-authored text edited in the admin
  panel and rendered as-is on /rules. The home page ("customistable from the
  admin panel") will want the same endpoint.

### Regressions from the Python backend

`git show 43cfeeb:backend/app/api/teams.py` had `member_profile`, `team_activity`
and `team_activity_public`, and computed a team score and per-member solve counts.
The Go rewrite kept neither, so the member profile view, the activity feed and
every score column on the teams page have no data source. These are regressions,
not unbuilt features.

Fields the handlers do not return, so the UI shows `—`:

| Field | Needed for | Note |
| --- | --- | --- |
| team score | leaderboard Score column, team rank | no score column on `teams`; own-team score is summed from the roster instead |
| team roster in `GET /api/teams` | member list of an inspected team | only `member_count` is returned; `/api/teams/mine` does include members |
| per-member solve count | Solves column, contribution | `listMembers` selects `id, username, score` only |

`GET /api/auth/me` returns `{id, username, score, role, rank}` — no email, team_id
or banned flag — so types.ts matches that, not the users table.

So today `/challenges` and `/admin/status` render an error page, and the teams
page works except for member profiles and the activity feed. When an endpoint
lands, nothing in the frontend needs changing — the call is already there.

## Real-time data

The app is server-rendered on first paint and then navigates client-side, so
there are no document reloads between pages. Live values (solve counts, scores,
the leaderboard) are the reason for that, and they need a push channel the
backend does not have yet.

Page data is therefore held as `$state` seeded from the load function and
re-seeded by an effect when the load re-runs, rather than as immutable props —
so a channel can patch rows in place instead of triggering a full refetch. See
`src/routes/challenges/+page.svelte`.

What the backend needs to add: one server-to-client stream of solve and score
events. **SSE is the smaller option** — it needs no Go dependency (plain
`net/http` with `text/event-stream`), reconnects automatically in the browser via
`EventSource`, and passes through the existing `/api` proxy untouched, which
streams response bodies already. Websockets would need `coder/websocket` or
`gorilla/websocket` plus an upgrade path in the proxy, and only pay off if
messages have to travel client-to-server too.

Until that exists, a correct flag calls `invalidateAll()` and the page re-reads
from the backend. There is no polling.

## Layout

```
src/
  routes/
    +layout.svelte          nav, global css
    +layout.ts              optional signed-in user
    +error.svelte           shows the real status and message
    api/[...path]/+server.ts  proxies every /api/* call to the Go backend
    challenges/, teams/, login/, register/, admin/status/
  lib/
    api.ts          one typed function per endpoint, throws ApiError
    load.ts         required(): API failure -> SvelteKit error with real status
    types.ts        mirrors backend/internal/database/migrations/00001_init.sql
    flag.svelte.ts  shared flag-submission state
    utils.ts, categories.ts
    components/
  styles/           global.css + themes; category and difficulty colors
```

Conventions: Tailwind classes inline in markup — no `@apply`, no style-only
components, no class strings hoisted into `<script>`. Data loading belongs in
`+page.ts`, not in `$effect`.
