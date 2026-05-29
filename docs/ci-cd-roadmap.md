# CI/CD Roadmap (GitHub Actions)

Plan for adding continuous integration and delivery to media-bridge. There are
currently **no workflows** under `.github/workflows/`; this is the working
backlog to get from "nothing" to "every PR is tested, every image is built,
scanned, and published, and `main` deploys safely".

Treat phases as incremental and independently shippable. Phases 1–3 are the core
ask ("test + build an image"); 4–6 are the "nice and useful" extras.

## Repository facts the pipeline must respect

| Component | Stack | Build / test command | Image |
|-----------|-------|----------------------|-------|
| Backend | Go `1.25.5`, entrypoint `./cmd/api` | `go build ./...`, `go vet ./...`, `go test ./...` | `backend/Dockerfile` (multi-stage, `golang:1.25.5-alpine` → `alpine:3.19`) |
| Frontend | SvelteKit 2 + Svelte 5 + Vite 6 + TS, static adapter | `npm ci`, `npm run check`, `npm run build` | built **inside** `nginx/Dockerfile` (node:20 → openresty) |
| Reverse proxy | OpenResty + Lua | n/a | `nginx/Dockerfile` (context = repo root) |
| Database | Postgres 16 | — | `postgres:16-alpine` (compose only) |

Key constraints discovered in the repo:

- **PG-gated tests**: integration tests skip unless `TEST_DATABASE_URL` is set
  (e.g. `backend/tests/processingqueue/worker_pg_test.go`). CI must stand up a
  Postgres service and export that var to actually exercise them.
- **Two publishable images**, not one: the Go API (`backend/Dockerfile`) and the
  OpenResty+frontend bundle (`nginx/Dockerfile`, build context = repo root).
- **`nginx/Dockerfile` builds the frontend** via `npm ci && npm run build` and
  then runs `imagemagick` to generate icons — so the frontend has no standalone
  Dockerfile; its "image build" is the nginx image.
- **Secrets**: `.env` is gitignored. No secrets may be echoed in logs; deploy
  secrets live in GitHub Environments.
- No `.golangci.yml` and no frontend lint script yet — linting config is part of
  the work, not a precondition.

---

## Phase 0 — Foundations (shared conventions)

Cross-cutting decisions every workflow below depends on.

- **Triggers**: `pull_request` (all branches into `main`) + `push` to `main`.
  Tags `v*` trigger release/publish.
- **Concurrency**: cancel superseded runs per ref:
  ```yaml
  concurrency:
    group: ${{ github.workflow }}-${{ github.ref }}
    cancel-in-progress: true
  ```
- **Path filters** (via `dorny/paths-filter` or `paths:` on triggers) so a
  frontend-only PR doesn't run the Go suite and vice-versa. Buckets: `backend/**`,
  `frontend/**`, `nginx/**`, `.github/**`, `docker-compose.yml`.
- **Least-privilege permissions**: default `permissions: contents: read`; opt into
  `packages: write`, `security-events: write`, etc. per-job only where needed.
- **Pinned actions**: pin third-party actions by full commit SHA (supply-chain
  hygiene); first-party `actions/*` may use major tags.
- **Caching**: `actions/setup-go` with `cache: true` (keyed on `go.sum`),
  `actions/setup-node` with `cache: npm` (keyed on `package-lock.json`),
  and Buildx layer cache (`type=gha`) for images.

Deliverable: a short `CONTRIBUTING`/`.github/README` note documenting required
checks + the SHA-pinning convention.

---

## Phase 1 — Backend CI (`backend-ci.yml`)  ← core

Runs on changes to `backend/**`. Working directory `backend/`.

Jobs / steps:
1. **Format check** — `gofmt -l .` must print nothing (fail if it does). This has
   bitten the codebase repeatedly during refactors, so gate it.
2. **Vet** — `go vet ./...`.
3. **Lint** — `golangci-lint` via `golangci/golangci-lint-action`. Ship a minimal
   `backend/.golangci.yml` (start with `govet`, `staticcheck`, `errcheck`,
   `ineffassign`, `unused`, `misspell`) and tighten over time.
4. **Build** — `go build ./...`.
5. **Unit tests** — `go test ./...` (fast, no DB).
6. **Integration tests (PG)** — same `go test ./...` but with a Postgres
   **service container** and `TEST_DATABASE_URL` exported so the gated suites run:
   ```yaml
   services:
     postgres:
       image: postgres:16-alpine
       env: { POSTGRES_USER: test, POSTGRES_PASSWORD: test, POSTGRES_DB: test }
       ports: ["5432:5432"]
       options: >-
         --health-cmd "pg_isready -U test -d test"
         --health-interval 5s --health-timeout 5s --health-retries 5
   env:
     TEST_DATABASE_URL: postgres://test:test@localhost:5432/test?sslmode=disable
   ```
7. **Race + coverage** — `go test -race -coverprofile=coverage.out ./...`; upload
   `coverage.out` as an artifact (and optionally Codecov). Race is cheap insurance
   for the worker/queue goroutines and SSE hub.

Nice-to-have: `go mod tidy` diff check (fail if `go.mod`/`go.sum` would change),
`govulncheck ./...` (can also live in Phase 4).

---

## Phase 2 — Frontend CI (`frontend-ci.yml`)  ← core

Runs on changes to `frontend/**`. Working directory `frontend/`.

1. `npm ci` (cached on `package-lock.json`).
2. **Type/Svelte check** — `npm run check` (`svelte-kit sync && svelte-check`).
3. **Build** — `npm run build` (catches adapter-static / route errors before the
   image build does).
4. *(Add later)* a lint script — introduce `eslint` + `prettier --check` and a
   `npm run lint` script; until then, `prettier --check` can run standalone.

Upload the `frontend/build` output as an artifact for inspection on PRs.

---

## Phase 3 — Docker image build & publish (`images.yml`)  ← core

Build **both** images with Buildx; publish to **GHCR** (`ghcr.io/<owner>/...`).

- **On PRs**: build only (no push) to validate the Dockerfiles, with
  `type=gha` layer cache. Use a matrix:
  ```yaml
  strategy:
    matrix:
      include:
        - { name: api,   context: ./backend, dockerfile: backend/Dockerfile, image: media-bridge-api }
        - { name: nginx, context: .,          dockerfile: nginx/Dockerfile,   image: media-bridge-nginx }
  ```
  (Note the **different build contexts** — nginx needs the repo root because it
  copies `frontend/` and `nginx/`.)
- **On `main` / tags**: `docker/login-action` → GHCR, then push. Generate tags +
  OCI labels with `docker/metadata-action` (`sha-<short>`, `edge` for main,
  semver for `v*` tags, `latest` for the newest release).
- **Multi-arch**: `linux/amd64` always; add `linux/arm64` via QEMU if the deploy
  target benefits (self-hosted boxes / Pi-class hardware). Skip arm64 on PRs to
  keep them fast.
- **Build hygiene**: confirm `.dockerignore` (root + `backend/`) excludes
  `node_modules`, `.svelte-kit`, `tests`, `.env` so contexts stay small and clean.

`permissions: { contents: read, packages: write }` on the publish job only.

---

## Phase 4 — Security & supply chain (`security.yml`)  ← high-value extra

- **Dependency vuln scan**:
  - Go: `govulncheck ./...`.
  - npm: `npm audit --omit=dev` (informational) / `osv-scanner`.
- **Image scanning**: Trivy (or Grype) against the built images; upload SARIF to
  the **Security** tab (`security-events: write`). Fail on HIGH/CRITICAL with a
  documented allowlist.
- **Static analysis / CodeQL**: enable CodeQL for `go` and `javascript-typescript`.
- **Secret scanning**: `gitleaks` on PRs (defense-in-depth; `.env` is already
  gitignored but catch accidental inline keys / the `JWT_SECRET`, `TMDB_API_KEY`,
  `TUNNEL_TOKEN` patterns).
- **Dependabot** (`.github/dependabot.yml`): ecosystems `gomod`, `npm`,
  `docker` (both Dockerfiles), and `github-actions`. Weekly, grouped minor/patch.
- **Hadolint** on `backend/Dockerfile` and `nginx/Dockerfile`.

---

## Phase 5 — Release & deploy (`release.yml` / `deploy.yml`)

- **Versioning**: tag-driven (`v*`). Optionally adopt `release-please` or
  `git-cliff` to automate changelog + version bumps from Conventional Commits.
- **Release artifacts**: attach a compose bundle / changelog to the GitHub Release;
  images are already in GHCR from Phase 3.
- **Deploy** (the app ships via `docker-compose.yml` with nginx + cloudflared):
  - Use a protected **GitHub Environment** (`production`) holding deploy secrets
    and requiring manual approval.
  - Trigger on tag publish (or `workflow_dispatch`). Deploy by SSH to the host
    (`appleboy/ssh-action`) running `docker compose pull && docker compose up -d`,
    or via a self-hosted runner on the box.
  - **Smoke test** post-deploy: hit the backend `health` endpoint and assert 200
    before marking the deploy green; otherwise roll back to the previous tag.
- Never bake `.env` into images — it's mounted/`env_file` at runtime by compose.

---

## Phase 6 — Developer-experience extras

- **Status badges** in `README.md` (CI, image, coverage).
- **Required status checks** + branch protection on `main` (backend-ci,
  frontend-ci, image-build must pass; linear history; up-to-date before merge).
- **PR automation**: `actions/labeler` (label by changed paths), auto-assign,
  and a "needs PG" label when `backend/**` changes touch `tests/`.
- **Auto-format bot**: a `workflow_dispatch`/comment-triggered job that runs
  `gofmt -w` + `prettier --write` and pushes the fix.
- **Stale workflow caches cleanup** and a nightly `schedule:` run of the full
  suite + `govulncheck` (catches newly-disclosed CVEs without a code change).
- **Build matrix for Go** (optional): test against `1.25.x` and `tip` to get early
  warning on toolchain regressions.

---

## Suggested file layout

```
.github/
  workflows/
    backend-ci.yml      # Phase 1
    frontend-ci.yml     # Phase 2
    images.yml          # Phase 3
    security.yml        # Phase 4
    release.yml         # Phase 5 (build/push on tags)
    deploy.yml          # Phase 5 (deploy to host)
  dependabot.yml        # Phase 4
  labeler.yml           # Phase 6
backend/.golangci.yml   # Phase 1
```

## Execution order (recommended)

1. **Phase 1 + 2** together — fastest feedback, unblocks branch protection.
2. **Phase 3** — start with PR build-only, then add GHCR push on `main`.
3. **Phase 4** — Dependabot + Trivy + govulncheck first (cheapest wins).
4. **Phase 0 hardening** — flip on required checks + concurrency once green is stable.
5. **Phase 5** — only after images publish reliably and a health endpoint smoke
   test exists.
6. **Phase 6** — opportunistic polish.

## Open questions to settle before Phase 3/5

- Registry: **GHCR** (assumed) vs Docker Hub?
- Deploy target: SSH to a single host, a self-hosted runner, or a PaaS? (Affects
  Phase 5 mechanics and whether `arm64` is worth building.)
- Is there a stable backend **health/readiness endpoint** suitable for the
  post-deploy smoke test? (`internal/health` exists — confirm it exposes an HTTP
  route, not just the internal scheduler.)
