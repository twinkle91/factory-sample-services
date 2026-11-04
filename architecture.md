# Architecture of factory-sample-services (Backend)

## Overview

A small Go HTTP API that shortens URLs, started with `go run ./cmd/server` on port 8080. It uses Go modules. docker-compose.yml runs PostgreSQL and Redis for local development.

<!-- TODO: The HTTP framework or router in use (none was detected) and how the app is put together. -->

## Folders

- `cmd/`: <!-- TODO: what lives in cmd. The server entry point is run with `go run ./cmd/server`. -->
- `internal/`: <!-- TODO: what lives in internal -->
- `migrations/`: <!-- TODO: what lives in migrations -->

## Data and integrations

- PostgreSQL 16 stores the links (POSTGRES_URL, DATABASE_URL).
- Redis 7 stores the visit counters (REDIS_URL).

<!-- TODO: Any other APIs this app calls, and how data flows between the HTTP handlers, PostgreSQL and Redis. -->
