# AGENT.md

## Purpose
This repo is the Agentra backend (Go + Gin + PostgreSQL + Fx). Keep behavior deterministic and push domain logic into `internal/usecase`.

## Prerequisites
- Go `1.26`
- Docker (optional, for `DOCKERTEST=1`)
- `ffmpeg` and `ffprobe` available in `PATH` (required for analyzer, metadata extraction, and thumbnails)

## Run Locally
1. Copy env file: `Copy-Item .env.example .env`
2. Start: `$env:DOCKERTEST="1"; go run ./cmd/agentra` (auto dev infra) or `go run ./cmd/agentra` (own infra)
3. API: `http://localhost:8080`

`DOCKERTEST=1` reuses named containers `agentra-dev-postgres` / `agentra-dev-minio`. Wipe: `docker rm -f agentra-dev-postgres agentra-dev-minio`

## Build / Test
```
go build ./...
go test ./...
```

## Architecture Map
- `cmd/agentra`: entrypoint and Fx wiring
- `internal/domain`: core entities and enums
- `internal/usecase`: business rules
- `internal/repository/postgres`: data access + migrations
- `internal/adapter/http`: handlers/routes/middleware
- `internal/adapter/storage`: MinIO/SMB adapters
- `internal/adapter/ffmpeg`: ffmpeg helpers

## Media Domain (Current)
- Upload is org-level: `POST /media/upload` (form field `file`), extensions: `.mov .mp4 .jpg .jpeg .png .wav .mp3`
- Upload stores original, generates thumbnail, extracts timestamp via `ffprobe` tags (fallback: server now), writes `media_assets` row
- Org list: `GET /media` — Project-scoped: `GET /projects/:id/media`
- Scope update: `PUT /projects/:id/media-scope` — modes: `global | date_range | selected`

## Migrations
- Files in `internal/repository/postgres/migrations`. Keep simple and linear; prefer plain SQL. App-level validation over DB constraints.

## Coding Rules
- Keep domain model, postgres repo SQL, usecase validation, and HTTP structs in sync for any domain change.
- Handlers stay thin — no hidden side effects.
- Logs: useful and deterministic.
