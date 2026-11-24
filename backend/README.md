# OpenCTF backend

The Go backend for OpenCTF: a JSON API on top of PostgreSQL, with session cookies for auth and policy-based access control (PBAC) for permissions.

## Quick start

```sh
docker compose up backend   # from the repository root
```

For requirements, configuration and running without Docker, see [Getting started](docs/getting-started.md).

## Documentation

| Doc                                      | What's in it                                                                 |
| ---------------------------------------- | ---------------------------------------------------------------------------- |
| [Getting started](docs/getting-started.md) | Requirements, configuration, running, testing and Docker builds            |
| [API reference](docs/api.md)             | Every endpoint, who may call it, and what it takes and returns               |
| [Adding an API](docs/adding-an-api.md)   | Conventions and a walkthrough for adding a new route                         |
| [Changing SQL](docs/database.md)         | sqlc queries, generated code and migrations                                  |
| [PBAC](docs/policies.md)                 | How permission checks work, the policy file format, actions and default rules |
| [Layout](docs/layout.md)                 | What lives in each directory under `cmd/` and `internal/`                    |
