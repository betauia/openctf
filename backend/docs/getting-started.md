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
go test ./...
```

Run it from `backend/`. The leading `./` is required: without it Go looks for the package in the standard library.

## Docker

```sh
docker build --target development -t openctf-backend:dev .
docker build --target production  -t openctf-backend .
```
