# OpenCTF frontend

SvelteKit 2 + Svelte 5 (runes), Tailwind v4, TypeScript, served by `adapter-node` on
port 4000. Pages render in the browser (`ssr = false` in `src/routes/+layout.ts`); the
Node server only serves the app shell and proxies `/api/*` to the Go backend.

```sh
npm install
npm run dev      # vite dev
npm run check    # svelte-check, must stay at 0 errors
npm run build    # production build
```

Needs the Go backend reachable at `BACKEND_URL` (default `http://backend:8000`, the
compose service name). With `docker compose up` from the repo root, the frontend is on
http://localhost:4000 and the backend on http://localhost:8000.

## Accounts for local testing

`.env.dev` holds the dev values:

- `ADMIN_EMAIL` / `ADMIN_PASSWORD` — the backend creates this admin (username `admin`)
  on startup, so it survives a database wipe.
- `OCTF_USER` / `OCTF_PASS` — a player in a team, used by the layout tests. It lives in
  the database only; after `docker compose down -v`, register it again.

Seed challenges with:

```sh
cat backend/seed/challenges.sql | docker compose exec -T database psql -U openctf -d openctf
```

## Layout tests

`tests/layout.spec.ts` (Playwright) opens every page at phone through super-ultrawide
sizes and fails on sideways scrolling, elements past the screen edge, text under 8px or
a page error, and checks each page survives live resizing. `tests/login.ts` signs in
first, so set a player that is in a team:

```sh
OCTF_USER=… OCTF_PASS=… npm run test:layout          # local Playwright
OCTF_USER=… OCTF_PASS=… npm run test:layout:docker   # pinned Playwright image, no local browsers
```

`OCTF_URL` overrides the default `http://localhost:4000`.

## Backend status

Every page calls the real endpoints; nothing is mocked. All of them exist except the
container instances API, which the backend has not built yet:

| Area         | Endpoints                                                                                                                                                                       | Backend                                                   |
| ------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------- |
| Auth         | `POST /api/auth/{login,register,logout,refresh}`, `GET /api/auth/me`, `GET /api/auth/solves`                                                                                    | implemented                                               |
| Teams        | `GET/POST /api/teams`, `GET/PATCH /api/teams/mine`, `POST /api/teams/join`, `POST /api/teams/mine/leave`, `PUT /api/teams/mine/captain`, `DELETE /api/teams/mine/members/{uid}` | implemented                                               |
| Team stats   | `GET /api/teams/{id}/{activity,progress,compare}`, `GET /api/users/{id}`                                                                                                        | implemented                                               |
| Scoreboard   | `GET /api/scoreboard`, `GET /api/scoreboard/players`                                                                                                                            | implemented                                               |
| Challenges   | `GET /api/challenges`, `POST /api/challenges/submit`, `GET /api/challenges/{id}/{solves,files}`, `GET /api/challenges/{id}/files/{fid}`                                         | implemented                                               |
| Settings     | `GET /api/settings` (public), `PUT /api/admin/settings`                                                                                                                         | implemented                                               |
| Live updates | `GET /api/live`                                                                                                                                                                 | implemented                                               |
| Instances    | `GET/POST /api/instances`, `DELETE /api/instances/{id}`                                                                                                                         | **missing** — `/admin/status` says so instead of erroring |

`POST /api/challenges/submit` takes `{flag, challenge_id?}` and returns
`{correct, challenge_id, points}`. `challenge_id` scopes the check to one challenge (the
modal); without it the backend finds the challenge the flag belongs to (the global flag
bar). Players must be in a team to submit.

Still not returned by the backend: the roster of a team other than your own
(`GET /api/teams` has `member_count` only).

Access control is decided by the backend's policy engine
(`backend/internal/policies/default.json`). The frontend only hides what a user can't
use; it never grants anything.

## Auth

The backend sets `HttpOnly` `session` (15 min) and `refresh` (14 days) cookies. The
`/api` proxy turns them into `x-session` / `x-refresh` response headers, `lib/api.ts`
keeps them in `localStorage` and sends them back as headers, and the proxy turns those
into cookies again. On a 401, `api.ts` calls `/api/auth/refresh` once and retries.

Known weakness: tokens in `localStorage` are readable by any script on the page, so an
XSS bug would leak a 14-day refresh token. Svelte escapes everything rendered, and there
is no `{@html}` on user data, but there is also no Content-Security-Policy yet.

## Themes

Each theme is a CSS file in `src/styles/themes/` that sets the colour variables, and is
listed in `THEMES` in `src/lib/themes.ts`. `uiactf.css` is the default under `:root`;
other themes override it under `:root[data-theme="<id>"]`, so a theme only needs the
variables it changes. `special.css` holds colours shared by every theme. An admin picks
the theme for the whole site on `/admin/config`; it is saved as `theme` in the backend
settings, so every visitor gets it. The inline script in `src/app.html` applies the last
seen theme from `localStorage` before the page draws, to avoid a flash of the default.

To add a theme: add the CSS file, import it in `src/styles/global.css`, add it to
`THEMES` and to the `ThemeId` type in `src/lib/types.ts`.

## Real-time data

The root layout opens one stream, `GET /api/live` (server-sent events), and pages patch
their state from it — solves update counts, scores and the leaderboard in place, and a
toast shows who solved what. `api.listen` reads the stream with `fetch` rather than
`EventSource`, because the session travels in a header, and reconnects when it drops.

Page data is held as `$state` seeded from the load function and re-seeded by an effect
when the load re-runs, rather than as immutable props, so the stream can patch rows in
place instead of refetching. See `src/routes/challenges/+page.svelte`.

## Layout

```
src/
  app.html                  shell; applies the cached theme before first paint
  routes/
    +layout.svelte          nav, live stream, global css
    +layout.ts              signed-in user and public settings (ssr off)
    +error.svelte           shows the real status and message
    api/[...path]/+server.ts  proxies every /api/* call to the Go backend
    challenges/  teams/  teams/scout/  scoreboard/  rules/
    login/  register/
    admin/status/  admin/config/
  lib/
    api.ts          one typed function per endpoint, throws ApiError; token handling
    load.ts         required() / optional() / tolerated() for load functions
    redirect.ts     safe ?redirectTo= handling for login
    themes.ts       theme list and applyTheme()
    types.ts        response shapes as the API returns them
    utils.ts, categories.ts
    components/     challenges/, teams/, CopyButton
  styles/           global.css + themes; category and difficulty colours
tests/              Playwright layout tests
```

Conventions: Tailwind classes inline in markup — no `@apply`, no style-only components,
no class strings hoisted into `<script>`. Data loading belongs in `+page.ts`, not in
`$effect`.
