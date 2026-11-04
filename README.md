# Short links API — Software Factory sample (Go, PostgreSQL, Redis)

A small Go HTTP API that shortens URLs. Links live in PostgreSQL and visit
counters live in Redis. The integration tests need both services running for
real, so this repository exercises the Software Factory's **Go runtime** and its
**test services** (PostgreSQL and Redis containers beside the sandbox).

- Go 1.24+, standard library `net/http`
- [`github.com/jackc/pgx/v5`](https://github.com/jackc/pgx) for PostgreSQL 16
- [`github.com/redis/go-redis/v9`](https://github.com/redis/go-redis) for Redis 7
- Plain SQL migrations in `migrations/`, applied on startup

## API

| Method | Path | What it does |
| --- | --- | --- |
| `GET` | `/health` | `{"status":"ok"}` when PostgreSQL and Redis both respond, otherwise `503` with the failing check |
| `POST` | `/links` | Body `{"url": "https://..."}`. Creates a link with a random 7-character code and returns `201` |
| `GET` | `/links` | Newest links first. `?limit=` defaults to 20, at most 100 |
| `GET` | `/links/{code}` | Returns the link and adds one visit in Redis |
| `GET` | `/links/{code}/stats` | Returns `{"code": "...", "visits": N}` |

Errors are JSON: `{"error": "url must start with http:// or https://"}`.

## Configuration

The Software Factory provides these variables when it runs the app and the tests.

| Variable | Required | Example |
| --- | --- | --- |
| `DATABASE_URL` | yes | `postgres://factory:factory@postgres:5432/app?sslmode=disable` |
| `REDIS_URL` | yes | `redis://redis:6379/0` |
| `PORT` | no, default `8080` | `8080` |

The server listens on `0.0.0.0:$PORT`. It exits with a clear message when
`DATABASE_URL` or `REDIS_URL` is missing, or when either service does not answer
at startup.

## Running locally

```bash
docker compose up -d
export DATABASE_URL='postgres://factory:factory@localhost:5432/app?sslmode=disable'
export REDIS_URL='redis://localhost:6379/0'
go run ./cmd/server
```

```bash
curl -s localhost:8080/health
curl -s -XPOST localhost:8080/links -d '{"url":"https://go.dev"}'
```

No Go on your machine? Run it in the Go image on the compose network:

```bash
docker run --rm --network factory-sample-services_default -v "$PWD":/src -w /src \
  -e DATABASE_URL='postgres://factory:factory@postgres:5432/app?sslmode=disable' \
  -e REDIS_URL='redis://redis:6379/0' golang:1.24 go test ./...
```

## Tests

```bash
go test ./...
```

- **Unit tests** (`*_test.go` without `Integration` in the name) need no services:
  handlers against in-memory fakes, validation, config and migration file parsing.
- **Integration tests** (`TestIntegration...`) use `DATABASE_URL` and `REDIS_URL`.
  Each test gets its own PostgreSQL schema and its own Redis key prefix, removed
  when it ends, so the suite runs repeatedly and in parallel. When a variable is
  missing they **fail** with a message saying how to start the services; they
  never skip, so a missing service is visible.

The whole suite takes a few seconds.

## Layout

```
cmd/server/          main.go (startup, shutdown), config.go (environment)
internal/links/      HTTP handlers, validation, PostgreSQL store
internal/cache/      Redis visit counters
internal/migrate/    migration runner (records versions in schema_migrations)
internal/testsupport/ per-test schema and key prefix for integration tests
migrations/          001_create_links.sql, 002_..., embedded into the binary
```

### Migrations

Add a file named `NNN_description.sql` with the next number. The runner applies
pending files in order on startup, each in its own transaction, and records them
in `schema_migrations`. An advisory lock stops two servers from migrating at
once. Never edit a migration that has already run; add a new one.

## Software Factory walkthrough

This repository is a test fixture for the Factory. It deliberately has **no**
Factory guidance files (`AGENTS.md`, `MISSION.md`, `design.md`,
`harness/harness.config.json`), so onboarding runs the starter-files flow.

### 1. What the scan should detect

| Signal | Source |
| --- | --- |
| Go 1.24+ | `go.mod` (`go 1.24.0`) |
| Start command `go run ./cmd/server` | `cmd/server/main.go` is the only `main` package |
| PostgreSQL needed | `docker-compose.yml` service `postgres` (`postgres:16`), `github.com/jackc/pgx/v5` in `go.mod` |
| Redis needed | `docker-compose.yml` service `redis` (`redis:7`), `github.com/redis/go-redis/v9` in `go.mod` |
| Port 8080, health `/health` | `cmd/server/config.go`, `internal/links/handlers.go` |

### 2. Mapping values to enter

| Field | Value |
| --- | --- |
| Install | `go mod download` |
| Build | `go build ./...` |
| Test | `go test ./...` |
| Start | `go run ./cmd/server` |
| Port | `8080` |
| Health path | `/health` |
| Source folders | `cmd`, `internal`, `migrations` |
| Test services | PostgreSQL 16, Redis 7 |

With the services selected, the Factory sets `DATABASE_URL` and `REDIS_URL` to
the `postgres` and `redis` hostnames on the sandbox network. Testing the mapping
with the services left out should fail in the integration tests with
`DATABASE_URL is not set`, which shows the services really are needed.

### 3. Starter files

Accept the starter files the Factory proposes. Nothing in the repository
conflicts with them.

### 4. Stories and the seeded bug

[`docs/sample-stories.md`](docs/sample-stories.md) has five Jira-ready stories
(a migration, a Redis feature, a small endpoint, a validation change and one
deliberately ambiguous story) and a bug report for a real defect that the
current tests do not catch.
