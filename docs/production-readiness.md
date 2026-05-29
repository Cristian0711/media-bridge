# Production Readiness Review

This document records a pre-production audit of Media Bridge (May 2026). Use it as a checklist before exposing the stack beyond a trusted LAN or running it with real indexer credentials.

**Scope:** `docker-compose.yml`, nginx edge auth, backend config/bootstrap, secrets handling, networking, paths, database, and operational risks observed in this environment.

---

## Architecture (current deploy)

```
Internet / LAN
    → nginx:80 (OpenResty — JWT gate on /api/)
        → backend:8080 (Gin — trusts X-User-ID from nginx)
        → postgres:5432 (internal + host port published)
    → qbittorrent:8080 (host port published)
```

**Key files**

| Area | Path |
|------|------|
| Compose | `docker-compose.yml` |
| Edge auth | `nginx/nginx.conf` |
| Config / env | `backend/internal/config/config.go`, `.env` |
| Auth / keys | `backend/internal/auth/` |
| Proxy headers | `backend/internal/app/middleware.go` |
| Download paths | `backend/internal/download/service.go` |
| Bootstrap / health paths | `backend/internal/app/bootstrap.go` |
| Pipeline audit (separate) | `docs/requests-hardlink-remove-findings.md` |

---

## Findings

Severity: **Critical** · **High** · **Medium** · **Low**

### Critical

#### C1 — Indexer and qBittorrent credentials as code defaults

**Where:** `backend/internal/config/config.go`

FileList, TorrentLeech session values, and qBittorrent username/password have **non-empty fallbacks in source**. If any of those values were ever real, treat them as compromised: rotate on the trackers, move all secrets to `.env`, and remove defaults from the repo (fail startup when required vars are unset).

**Risk:** Anyone with repo or image access gets indexer session material; production may silently run with dev credentials.

**Fix:** Require env vars for all secrets; add `.env.example` with names only; never commit live cookies/passkeys.

---

#### C2 — Postgres exposed on the host

**Where:** `docker-compose.yml` — `postgres` service `ports: "5432:5432"`

**Risk:** Database reachable from the network if the host firewall is weak.

**Fix:** Remove host port mapping; keep Postgres on the internal Docker network only. Use `docker exec` or a VPN for admin access.

---

#### C3 — qBittorrent Web UI exposed on the host

**Where:** `docker-compose.yml` — `qbittorrent` ports `8080`, `6881`

**Risk:** Default qBittorrent credentials (`admin` / `adminadmin` in config defaults) plus a public Web UI equals full control of downloads and filesystem mounts.

**Fix:** Do not publish `8080` to `0.0.0.0`; bind to `127.0.0.1` if needed locally, or access only via VPN. Set a strong `QBITTORRENT_PASSWORD` immediately.

---

#### C4 — Production paths still point at `plex-debug`

**Where:**

- `backend/internal/config/config.go` — `MOVIES_PATH` / `SHOWS_PATH` defaults
- `backend/internal/download/service.go` — hardcoded `savePath`
- `backend/internal/app/bootstrap.go` — `health.Config.DownloadsPath`

**Risk:** Torrents and health checks target debug directories; hardlinks/library layout may not match real Plex paths.

**Fix:** Introduce `DOWNLOADS_PATH` (and use `MOVIES_PATH` / `SHOWS_PATH` everywhere); set all three in `.env` and compose volumes to match qBittorrent and library layout on one filesystem (required for hardlinks).

---

#### C5 — Backend trusts proxy auth headers without verifying the caller

**Where:** `backend/internal/app/middleware.go`

The API accepts `X-User-ID` and `X-Username` from any client. Nginx sets these after JWT validation, but **anything that can reach `backend:8080` on the Docker network** could forge headers and impersonate users.

**Risk:** Privilege escalation if backend is ever published, or another container on the same network is compromised.

**Fix:** Add a shared internal secret header (set only by nginx, verified in middleware), and/or restrict backend to an internal network reachable only from nginx.

---

#### C6 — No TLS on the application edge

**Where:** `nginx/nginx.conf` — `listen 80` only; `docker-compose.yml` — `80:80`

**Risk:** JWTs and credentials cross the network in cleartext.

**Fix:** Terminate HTTPS in front (Caddy, Traefik, existing reverse proxy, or Cloudflare tunnel). Redirect HTTP → HTTPS in production.

---

### High

#### H1 — Weak or placeholder `.env` values

**Where:** `.env` (gitignored but often copied from templates)

Examples observed: `POSTGRES_PASSWORD=changeme`, `JWT_SECRET=change-this-to-a-long-random-secret`.

**Fix:** Generate long random secrets before go-live; never reuse dev values in production.

---

#### H2 — Incomplete environment wiring in Compose

**Where:** `docker-compose.yml` — `backend.environment`

Only `DATABASE_URL`, `JWT_SECRET`, `PORT`, and TMDB vars are passed. Indexer cookies, qBittorrent, and media paths are **not** in compose and fall back to `config.go` defaults.

**Fix:** Pass through all required vars explicitly in compose (or `env_file: .env`) so production config is visible in one place.

---

#### H3 — Disk space and Docker growth

**Observed:** Postgres became unhealthy when the root filesystem filled (~14G). After cleanup, ~29G total with meaningful use of images/build cache.

**Risk:** Torrents, Postgres WAL, Docker layers, and logs can exhaust disk again and take down the DB.

**Fix:**

- Size data disk generously (separate volume for `/mnt/plexmedia` and Postgres if possible).
- Set Docker log rotation (`max-size` / `max-file`).
- Monitor disk usage; schedule cautious `docker system prune` for build cache.
- Add compose `mem_limit` / `cpus` so one service cannot starve the host.

---

#### H4 — No Postgres backup strategy in repo

**Where:** `postgres_data` named volume — no documented dump/restore.

**Fix:** Scheduled `pg_dump` (or volume snapshots), off-host copy, and a tested restore procedure before relying on production data.

---

#### H5 — First-user bootstrap is undocumented

**Flow:**

1. Registration requires a one-time key (`auth.Key` in DB).
2. `POST /api/v1/keys/generate` requires an **authenticated** user (`X-User-ID` from nginx).
3. Therefore the **first** invite key cannot be created via the API without a bootstrap step.

**Bootstrap SQL (example):**

```sql
INSERT INTO keys (value, is_active, created_at)
VALUES ('<uuid-from-uuidgen>', true, NOW());
```

Then open `/register` with that key, create the admin account, and use the app to generate further keys.

**Note:** `nginx/nginx.conf` whitelists `/api/v1/keys/generate` without a Bearer token, but the backend still requires proxy headers — so unauthenticated key generation through nginx does not work. The whitelist is misleading; either remove it or document that it does not enable public key generation.

---

#### H6 — GORM `AutoMigrate` at startup

**Where:** `backend/internal/app/bootstrap.go` — `migrate()`

**Risk:** Schema changes apply automatically on boot without versioned migration history; harder to review, roll back, or run blue/green deploys.

**Fix (when team/process grows):** Adopt versioned SQL migrations (goose, atlas, etc.). Acceptable for a single controlled host if migrations are reviewed in PRs.

---

#### H7 — qBittorrent volume and user alignment

**Where:** `docker-compose.yml` — qBittorrent `PUID`/`PGID`, mounts; backend mounts only `plex-debug` subtree.

**Risk:** Permission errors or hardlink failures if backend and qBittorrent do not share the same UID/GID and filesystem paths.

**Fix:** Align volumes, PUID/PGID, and path env vars so downloads, incomplete files, and library paths are on one filesystem.

---

### Medium

#### M1 — Long JWT lifetime

**Where:** `backend/internal/auth/jwt.go` — `tokenTTL = 90 * 24 * time.Hour`

**Fix:** Shorter access tokens, refresh flow, or explicit logout/revocation if needed for shared devices.

---

#### M2 — No rate limiting on auth or search

**Where:** Public routes `/api/v1/auth/login`, `/api/v1/auth/register`; protected indexer/search routes.

**Risk:** Brute force on login; abuse of indexer/TMDB from a leaked token.

**Fix:** nginx `limit_req` or application-level rate limits on login and expensive endpoints.

---

#### M3 — Gin release mode not set in compose

**Fix:** Set `GIN_MODE=release` for the backend service in production.

---

#### M4 — Missing container healthchecks

**Where:** Only `postgres` has a healthcheck.

**Fix:** Add healthchecks for `backend` and `nginx` (HTTP probe) for cleaner `depends_on` and orchestration.

---

#### M5 — `/api/v1/auth/validate` reachable without nginx JWT gate

**Where:** `backend/internal/auth/routes.go` — public validate route; nginx allows internal auth checks.

**Risk:** Low if backend is never published to the host; higher if the API port is exposed.

**Fix:** Keep backend internal-only; optionally require `X-Internal-Auth-Check` or shared secret on validate.

---

#### M6 — No deploy runbook in repo

**Fix:** Add `docs/DEPLOY.md` or extend this doc with: env template, bootstrap SQL, `docker compose up -d --build`, smoke tests, rollback, and credential rotation steps.

---

### Low

#### L1 — `keys/generate` nginx whitelist

**Where:** `nginx/nginx.conf` — bypasses Bearer check for `/api/v1/keys/generate`

Harmless given backend middleware, but confusing for operators. Remove from whitelist or document.

---

#### L2 — Orphan / naming consistency

Occasional compose warnings about orphan containers (e.g. `media-bridge-frontend`) if an older service name was used. Run `docker compose down --remove-orphans` after verifying nothing important is orphaned.

---

## What is already in good shape

| Item | Notes |
|------|--------|
| Edge JWT validation | OpenResty validates Bearer tokens before `/api/` reaches Gin; 60s token cache |
| Invite-only registration | One-time `keys` row consumed on successful register |
| API not on host | Backend uses `expose` only, not `ports` |
| SPA caching | Immutable `/_app/` assets; `no-store` for shell routes |
| Postgres healthcheck | `pg_isready` with restart policy |
| Structured nginx logs | JSON access log format |
| Recent pipeline work | Queues, completion watcher, health scans, TMDB, freeleech download flow |
| `.env` gitignored | Secrets file not tracked |

---

## Pre-production checklist

Use this in order before go-live:

- [ ] **Secrets:** Strong `POSTGRES_PASSWORD`, `JWT_SECRET`; all indexer and qBittorrent vars in `.env` only; rotate anything that ever appeared in `config.go` defaults.
- [ ] **Compose:** Remove `5432:5432` (and lock down qBittorrent `8080`); pass full `env_file` or explicit environment for backend.
- [ ] **Paths:** Set `MOVIES_PATH`, `SHOWS_PATH`, `DOWNLOADS_PATH` (once implemented) to production library paths; align compose volumes with qBittorrent.
- [ ] **TLS:** HTTPS in front of nginx; redirect HTTP.
- [ ] **Bootstrap:** Insert first `keys` row via SQL; register admin; verify login and key generation.
- [ ] **Backups:** `pg_dump` schedule + restore test.
- [ ] **Disk:** Adequate space on root and media volume; log rotation; monitoring alert on disk &gt; 80%.
- [ ] **Hardening:** `GIN_MODE=release`; qBittorrent password changed; optional proxy secret on backend.
- [ ] **Smoke test:** Search → download (freeleech quality) → request completes → hardlink → library; health log shows green.
- [ ] **Deploy:** `docker compose up -d --build` (backend + nginx; rebuild frontend via nginx image).

---

## Suggested implementation order (code changes)

If hardening in-repo (not only ops):

1. Remove secret defaults from `config.go`; fail on missing required env.
2. Add `DOWNLOADS_PATH` and use it in `download/service.go` and health config.
3. Update `docker-compose.yml` (env_file, no public DB port, optional internal network).
4. Add `.env.example` and bootstrap section to deploy docs.
5. Optional: internal proxy secret in nginx + `middleware.go`.
6. Optional: versioned SQL migrations replacing sole reliance on `AutoMigrate`.

---

## Related documents

- [Requests → Hardlink → Remove audit](./requests-hardlink-remove-findings.md) — pipeline performance and correctness (separate from deploy/security).

---

*Last updated: May 2026 — review before each major production deploy.*
