# Media Bridge

[![Backend CI](https://github.com/Cristian0711/media-bridge/actions/workflows/backend-ci.yml/badge.svg)](https://github.com/Cristian0711/media-bridge/actions/workflows/backend-ci.yml)
[![Frontend CI](https://github.com/Cristian0711/media-bridge/actions/workflows/frontend-ci.yml/badge.svg)](https://github.com/Cristian0711/media-bridge/actions/workflows/frontend-ci.yml)
[![Images](https://github.com/Cristian0711/media-bridge/actions/workflows/images.yml/badge.svg)](https://github.com/Cristian0711/media-bridge/actions/workflows/images.yml)
[![Security](https://github.com/Cristian0711/media-bridge/actions/workflows/security.yml/badge.svg)](https://github.com/Cristian0711/media-bridge/actions/workflows/security.yml)
[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white)](backend/go.mod)
[![SvelteKit](https://img.shields.io/badge/SvelteKit-2-FF3E00?logo=svelte&logoColor=white)](frontend/package.json)

Self-hosted media automation: search indexers for movies and shows, download
through qBittorrent, hardlink finished files into a Plex-style library, and track
everything from a mobile-friendly web app — exposed securely over a Cloudflare
Tunnel.

## Architecture

```
                       ┌─────────────────────────────┐
  Browser / iOS PWA ──▶│  nginx (OpenResty + SvelteKit build) │
                       └───────────────┬─────────────┘
                                       │  /api/v1/*
                                       ▼
                       ┌─────────────────────────────┐
                       │  Go API (Gin)               │
                       │  • requests / download      │
                       │  • hardlink / remove queues │
                       │  • health diagnostics + SSE │
                       └──────┬───────────────┬──────┘
                              │               │
                  ┌───────────▼──┐     ┌──────▼───────┐
                  │  PostgreSQL  │     │  qBittorrent │
                  └──────────────┘     └──────────────┘
```

The Go API is the source of truth. Long-running work (downloads, hardlinking,
removals) runs through a Postgres-backed processing queue with leased workers and
panic recovery. Real-time updates are pushed to the UI over Server-Sent Events.

## Tech stack

| Layer | Tech |
|-------|------|
| Backend | Go 1.25, Gin, GORM, PostgreSQL 16 |
| Frontend | SvelteKit 2, Svelte 5, Vite 8, Tailwind CSS v4 |
| Reverse proxy | OpenResty (nginx + Lua), serves the static frontend |
| Torrent client | qBittorrent (managed via its Web API) |
| Metadata | TMDB, Trakt |
| Indexers | [Prowlarr](https://prowlarr.com/) (FileList, TorrentLeech, Blutopia, etc.) |
| Edge | Cloudflare Tunnel (`cloudflared`) |

## Quick start

Requires Docker + Docker Compose. qBittorrent is expected to run on the host (or
uncomment the service in `docker-compose.yml`).

```bash
# 1. Configure environment
cp .env.example .env
# edit .env — set secrets, credentials and API keys

# 2. Build and start the stack
docker compose up -d --build

# 3. App is served by nginx on port 80 (and via the Cloudflare Tunnel)
```

Services started by `docker-compose.yml`:

- **postgres** — database (volume-backed)
- **backend** — Go API
- **nginx** — OpenResty serving the built frontend + proxying `/api`
- **cloudflared** — Cloudflare Tunnel

## Configuration

All configuration is via environment variables (see `.env.example`):

| Variable | Purpose |
|----------|---------|
| `POSTGRES_USER` / `POSTGRES_PASSWORD` / `POSTGRES_DB` | Database credentials |
| `DATABASE_URL` | Built from the Postgres vars by compose |
| `JWT_SECRET` | Signing secret for auth tokens |
| `PORT` | API port (default `8080`) |
| `TMDB_URL` / `TMDB_API_KEY` | TMDB metadata |
| `TRAKT_URL` / `TRAKT_API_KEY` | Trakt metadata |
| `PROWLARR_URL` / `PROWLARR_API_KEY` | Prowlarr API (all indexers) |
| `TUNNEL_TOKEN` | Cloudflare Tunnel token |

> `.env` holds real secrets and is git-ignored. Never commit it.

## Development

### Backend (Go)

```bash
cd backend
go build ./...
go vet ./...
go test ./...
```

Integration tests that need Postgres run only when `TEST_DATABASE_URL` is set
(they skip otherwise):

```bash
TEST_DATABASE_URL="postgres://test:test@localhost:5432/test?sslmode=disable" \
  go test -race ./...
```

Linting uses [golangci-lint](https://golangci-lint.run/) (config in
`backend/.golangci.yml`):

```bash
golangci-lint run
```

### Frontend (SvelteKit)

Requires Node ≥ 20.19 (Vite 8).

```bash
cd frontend
npm install
npm run dev      # http://localhost:5173
npm run check    # svelte-check / type check
npm run build    # production build (consumed by the nginx image)
```

## Health & diagnostics

The app ships a built-in diagnostics suite (`internal/health`) surfaced in the
settings dashboard, including:

- **Database / API / qBittorrent** reachability
- **Media ↔ torrent registry** — every media row maps to a real qBittorrent torrent
- **Media ↔ catalog consistency** — every media has a valid movie/show entry and
  vice-versa, with `media == movies + show entries`
- **Queues & pipeline** health
- **Filesystem hardlink audit** (full scan) — library files are hardlinked, not copies

## CI/CD

GitHub Actions workflows (see the badges above):

| Workflow | What it does |
|----------|--------------|
| **Backend CI** | gofmt, `go vet`, `go mod tidy` check, golangci-lint, build, unit + Postgres tests with `-race` and coverage |
| **Frontend CI** | `npm ci`, `svelte-check`, production build |
| **Images** | Builds the API and nginx images (Buildx); pushes to GHCR on `main`/tags |
| **Security** | `govulncheck`, Trivy filesystem scan (SARIF), hadolint, gitleaks; weekly schedule |

Dependencies are kept current by Dependabot (`gomod`, `npm`, `docker`,
`github-actions`). The CI/CD plan lives in [`docs/ci-cd-roadmap.md`](docs/ci-cd-roadmap.md).

## Documentation

- [`docs/ci-cd-roadmap.md`](docs/ci-cd-roadmap.md) — CI/CD roadmap
- [`docs/backend-refactor-roadmap.md`](docs/backend-refactor-roadmap.md) — backend refactor backlog
- [`docs/production-readiness.md`](docs/production-readiness.md) — production readiness notes
