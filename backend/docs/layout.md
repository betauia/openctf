# Layout

```
cmd/server/            entrypoint: config, database, default admin, policies, server lifecycle
internal/config/       environment configuration
internal/database/     connection pool, migrations and error helpers
  migrations/          goose migrations, embedded in the binary
  queries/             sqlc query files
  dbgen/               sqlc-generated code (don't edit)
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
seed/                  sample data for development databases (see Seeding), not part of the server
```

