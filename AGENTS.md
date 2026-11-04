# Rules for agents in factory-sample-services (Backend)

## Stack

Go (version 1.24 or newer), Go modules.

## You may

- Change application code in `cmd/`, `internal/`, `migrations/` to deliver the story.
- Add tests for every behavior you change using go test.

## You must not

- Edit MISSION.md, AGENTS.md, architecture.md, conventions.md, design.md or anything in `harness/`.
- Add, remove or upgrade dependencies, or edit lockfiles.
- Change CI, deployment or Docker files.
- Weaken or delete existing tests.
- Commit secrets or credentials.

## Checks

- Install: `go mod download`
- Build: `go build ./...`
- Test: `go test ./...`
- Start: `go run ./cmd/server` (listens on port 8080)

Tests run against empty test services that start beside the sandbox: PostgreSQL 16 at postgres:5432 (POSTGRES_URL, DATABASE_URL); Redis 7 at redis:6379 (REDIS_URL). Tests create their own tables, for example by running the migrations.
