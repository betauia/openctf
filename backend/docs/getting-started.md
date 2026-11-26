# Getting started

## Requirements

- Go 1.27+
- PostgreSQL
- [sqlc](https://docs.sqlc.dev/en/latest/overview/install.html) (only needed when changing SQL)

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
go test -p 4 ./...
```

`-p 4` runs four packages at a time. Every package starts its own Postgres container. On Windows, Docker Desktop can refuse some start-ups when all packages start at once and fail a package with `testcontainers cannot run: ... rootless Docker is not supported on Windows` or a reaper error. The coverage tool below already uses `-p 4`.

Run it from `backend/`. The leading `./` is required: without it Go looks for the package in the standard library.

The database tests start a Postgres container with testcontainers, so Docker must be running. If it can't be reached the run fails with `testcontainers cannot run`; the tests are never skipped.

### Coverage

```sh
go run ./tools/coverage            # coverage per package and in total
go run ./tools/coverage -min 80    # also exit 1 if the total is below 80%
go run ./tools/coverage -html      # also write coverage.html to browse uncovered lines
```

This runs the whole test suite and measures statement coverage of the hand-written code. The generated sqlc code (`internal/database/dbgen`), `cmd/`, `tools/` and the test helpers in `internal/testutil` are left out. A test in one package counts towards the code it exercises in another. `coverage.out` and `coverage.html` are git-ignored. The tool stops without a report if any test fails.

### Writing tests

Tests that need the database share two helper packages:

- `internal/testutil/testdb` starts the Postgres container (`func TestMain(m *testing.M) { os.Exit(testdb.Run(m)) }`) and creates users, teams, challenges and settings changes. Each package gets its own container, and its tests share one database, so give things unique names (`testdb.Unique()`) and clean up what changes global state.
- `internal/testutil/testenv` builds the routes on top of that with the default policy, logs users in through `POST /api/auth/login` and sends requests as them.

### Vet

```sh
go vet ./...
```

## Docker

```sh
docker build --target development -t openctf-backend:dev .
docker build --target production  -t openctf-backend .
```
