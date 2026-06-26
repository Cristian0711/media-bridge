# Backend Quality & Reliability Improvements

Codebase-wide review (June 2026) of `backend/internal/*` and `backend/shared/*`,
covering correctness bugs, concurrency/lifecycle safety, performance, security, and
code-quality cleanup. This complements `docs/backend-refactor-roadmap.md` (which ranks
packages by refactor urgency) by listing **specific defects with file:line references
and concrete fixes**.

Treat this as the working backlog. Phases are ordered by risk: data-corruption and
lifecycle bugs first, then security and performance, then quality cleanup. Each phase
is independently shippable. Severities: **Critical** (data loss / corruption / hang),
**High** (incorrect behavior, leaks, security exposure), **Medium**, **Low**.

> Trust-model note used throughout: nginx (`nginx/lua/auth.lua`) validates the JWT and
> **overwrites** `X-User-ID/Username/Role` via `ngx.req.set_header` before proxying, so
> the backend's header-trust middleware is sound *as long as the backend is never
> directly reachable*. Items below treat this as a defense-in-depth concern, not a live
> bypass.

---

## Phase 1 — Correctness: data integrity & state machine

Bugs here can corrupt data, create duplicate downloads, or wrongly transition request
state. Highest priority.

### 1.1 Registration is not atomic — invite key can be consumed twice `[Critical]`
- **Where:** `internal/auth/service.go:56-112`
- **Problem:** `FindKey → check user → Create user → DisableKey` runs with no transaction
  or row lock. Two concurrent requests with the same single-use key both pass the
  `key.IsActive` check (`:66`) before either reaches `DisableKey` (`:107`), so one invite
  key can create two accounts. On partial failure (user created, `DisableKey` fails) the
  user is orphaned and the key stays active.
- **Fix:** Wrap the whole flow in `db.Transaction`. Consume the key first with a
  conditional update — `UPDATE keys SET is_active=false, used_at=now() WHERE value=? AND
  is_active=true` — and treat `RowsAffected == 0` as `ErrKeyInvalid` (claim-then-create).

### 1.2 Duplicate-request dedup race — SELECT-then-INSERT with no constraint `[Critical]`
- **Where:** `internal/requests/repository.go:281-297` (`createIfAbsent`)
- **Problem:** Dedup does `findExisting(...).First()` then `tx.Create()` under default
  READ COMMITTED with no `FOR UPDATE` and no unique index. Two concurrent POSTs for the
  same movie/quality both see "no existing row" and both insert → duplicate downloads
  added to qBittorrent. The "atomic" comment is incorrect.
- **Fix:** Add a partial unique index over the active-dedup scope (e.g. `(type, imdb_id,
  quality, status)` filtered to active statuses) and treat the duplicate-key error as
  "already in progress"; or take `clause.Locking{Strength: "UPDATE"}` on a stable parent
  row before the existence check.

### 1.3 Transient qBittorrent error burns retry budget, falsely fails request `[High]`
- **Where:** `internal/hardlink/service.go:229-238` (`errIfTorrentStillDownloading`)
- **Problem:** When `GetTorrent` fails (transient qBit outage), the function returns
  `nil`, so `runHardlink` returns a *plain* error instead of `ErrDeferRetry`. That counts
  toward `MaxAttempts` (`queue_processor.go:122`), so a brief qBit blip can mark a
  still-downloading request `failed`.
- **Fix:** On `GetTorrent` error return a retryable/defer error (`ErrDeferRetry`), not
  `nil`, so the job requeues without consuming a real attempt.

### 1.4 Watcher resurrects a deleted download on "torrent not found" `[High]`
- **Where:** `internal/requests/watcher.go:123-137`
- **Problem:** When `progress.Complete` is true but the hash is empty/absent from
  qBittorrent, the watcher calls `finalizeDownloaded`. If a remove is in flight, qBit may
  have dropped the torrent while files are mid-deletion, so the request flips to
  `downloaded` even though it's being removed. Ordering vs. `CancelDownloadsByMediaID` is
  not guaranteed.
- **Fix:** Only finalize on torrent-absent if library hardlinks still exist (re-verify),
  or require an explicit terminal signal rather than absence. Ensure remove's cancel
  always precedes file deletion.

### 1.5 Purge deletes `downloaded` rows still referenced by remove `[High]`
- **Where:** `internal/requests/repository.go:535-543` + `status.go:41`
- **Problem:** `terminalRequestStatuses` includes `StatusDownloaded`, and
  `PurgeTerminalOlderThan` deletes them after 90 days. But a later remove looks up the
  originating request by `media_id`; purging severs that provenance link while the media
  still exists.
- **Fix:** Exclude `downloaded` from purge unless the linked media row is gone (scope
  purge to rows whose `media_id` no longer exists).

### 1.6 `S00` specials silently dropped; `complete` flag misparsed `[High]`
- **Where:** `internal/indexer/parse.go:12-24`, consumed at `selection.go:88`
- **Problem:** `parseSeasonEpisode` parses `S00` (specials) to `Season:0`, which
  `filterAndSortShows` treats as "unparsed" (`s.Season > 0` is false) and drops. Titles
  with only `Exx` (no season) don't match at all; the `complete` semantics are ambiguous
  for multi-episode packs.
- **Fix:** Handle `S00` explicitly, support season-absent episode forms, and reconsider
  `complete` (a season pack legitimately has no `Exx`).

### 1.7 Symlinked source defeats inode matching (false "already linked") `[Medium]`
- **Where:** `internal/hardlink/service.go:268-289` (`hardlinkPresent`/`sameInode`)
- **Problem:** `os.Stat` follows symlinks, so a symlinked source returns the *target's*
  inode and `sameInode` can report a false match — the file is treated as already linked
  and never actually hardlinked. This is inconsistent with the remove walks which use
  `filepath.Walk` (no symlink follow).
- **Fix:** Use `os.Lstat` in `hardlinkPresent`/`sameInode`, or reject non-regular sources
  before linking.

### 1.8 Case-insensitive username writes vs. case-sensitive unique index `[Medium]`
- **Where:** `internal/auth/service.go:42,57` + `internal/users/models.go:11`; `users`
  `Create` path does not lowercase (`service.go:47`).
- **Problem:** The service lowercases on the auth path, but `users.service.Create` does
  not, and the unique index is case-sensitive — a direct `CreateInput` can insert
  mixed-case duplicates that bypass intended uniqueness.
- **Fix:** Normalize to lowercase in `users.service.Create`, or use a functional unique
  index on `lower(username)` / `citext`.

---

## Phase 2 — Lifecycle, concurrency & resource leaks

Process-lifetime correctness: shutdown safety, goroutine/connection leaks, and channel
hazards. These degrade a long-running daemon over time and break graceful shutdown.

### 2.1 `log.Fatal` in server goroutine bypasses graceful shutdown `[Critical]`
- **Where:** `cmd/api/main.go:26-30`
- **Problem:** `srv.Run()` runs in a goroutine and calls `log.Fatal` on failure;
  `zap.Fatal` → `os.Exit(1)` skips `logger.Sync()`, worker drain, and SSE broker
  shutdown. A startup failure (e.g. port in use) kills the process without draining.
- **Fix:** Send the failure back to `main` over an error channel and run `Shutdown`
  before exiting.

### 2.2 pgx pools never closed on shutdown (and on init failure) `[Critical]`
- **Where:** `shared/queueutil/queueutil.go:17-30`, `internal/requests/queue.go:50`,
  `internal/hardlink/queue_processor.go:41-56`
- **Problem:** Each processor opens its own `pgxpool.Pool` with `context.Background()` and
  never closes it; `Server.Shutdown` cancels the worker context but never calls
  `pool.Close()`. In `hardlink/queue_processor.go` the pool also leaks on any
  `EnsureTable`/`New` error path. Multiple pools multiply DB connections.
- **Fix:** Return the pool (or a closer) from `NewQueue`, register it in
  `Server.shutdownFns`, and `pool.Close()` on shutdown — ideally share one pool across
  queues. Add `pool.Close()` on the hardlink init error paths.

### 2.3 SSE hub: `AddClient`/`RemoveClient` block forever after `Shutdown` `[Critical]`
- **Where:** `shared/ssehub/hub.go:81-82,124-130,155,158`; `internal/sse/handler.go:37`;
  `internal/qbittorrent/handler.go:144`
- **Problem:** `addClient`/`removeClient` are unbuffered channels serviced only by
  `run()`. After `Shutdown` closes `h.shutdown`, `run()` returns and nothing reads them.
  Every SSE handler's `defer h.broker.RemoveClient(...)` then blocks forever when its
  request context cancels → leaked HTTP goroutines.
- **Fix:** Make `AddClient`/`RemoveClient` select against `h.shutdown`
  (`select { case h.addClient <- c: case <-h.shutdown: c.Close() }`) so they no-op once
  the hub is down.

### 2.4 Handler context not bounded by `WorkerTimeout` — hung jobs never interrupted `[High]`
- **Where:** `shared/processing-queue/worker.go:82-86`
- **Problem:** `handlerCtx` is `WithCancel(ctx)` only, not `WithTimeout(WorkerTimeout)`. A
  handler that hangs (e.g. a qBit HTTP call with no timeout) blocks the worker goroutine
  forever; DB recovery requeues the *row* but the original goroutine keeps running and
  keeps touching the lease, so the slot is lost permanently.
- **Fix:** Derive `handlerCtx` with `context.WithTimeout(ctx, q.opts.WorkerTimeout)`.

### 2.5 Terminal status writes use the cancelled context during shutdown `[High]`
- **Where:** `shared/processing-queue/worker.go:106-127`
- **Problem:** On ctx cancellation mid-handler, `Fail`/`Complete`/`Defer` run with the
  already-cancelled `ctx`, fail with `context.Canceled`, and the row is stuck in
  `processing` ("could not record failure").
- **Fix:** Write the terminal status with a short `context.WithTimeout(context.Background(),
  …)` so final state persists even during shutdown.

### 2.6 Recovery `recoveryStarted` global map never reset; binds first ctx `[High]`
- **Where:** `shared/processing-queue/worker.go:42-54`
- **Problem:** Process-global `map[string]bool` keyed by `table|name`, never cleared.
  Restarting a same-named queue with a fresh ctx returns early via `startRecoveryOnce`, so
  stale-job recovery silently stops; no cleanup on ctx cancel.
- **Fix:** Store recovery state per-`Queue[T]` instance (a `sync.Once` field), or delete
  the key when the recovery goroutine exits.

### 2.7 Shutdown does not wait for worker/scheduler goroutines `[Medium]`
- **Where:** `internal/app/server.go:43-53`
- **Problem:** `Shutdown` cancels the worker ctx and drains HTTP, but workers, scheduler,
  and recovery loops are fire-and-forget with no `WaitGroup`. `Shutdown` returns while
  background goroutines may still be writing to the DB (compounds 2.2).
- **Fix:** Expose `Wait()`/done channels from processors and join them within the shutdown
  deadline.

### 2.8 Shared cached `*Progress` pointer handed to concurrent callers `[High]`
- **Where:** `internal/hardlink/cache.go:55-59,79-83`; consumers
  `requests/watcher.go:110`, `requests/torrentinfo.go:102`
- **Problem:** `Progress`/`ProgressForMedia` return the same `*Progress` (with `Done`/
  `Remaining` slices) to every concurrent caller within the TTL. Any mutation/append by
  one races the others.
- **Fix:** Return a shallow copy (struct + slices) from the cache, or document strict
  read-only and audit that no caller mutates.

---

## Phase 3 — Security hardening

Mostly defense-in-depth given the nginx trust model, plus a real log-leak.

### 3.1 Query strings (incl. secrets/tokens) logged at Info `[High]`
- **Where:** `internal/app/request_logger.go:49`
- **Problem:** `zap.Any("query", c.Request.URL.Query())` logs all query params verbatim;
  any API key/token/PII passed as a query param lands in logs.
- **Fix:** Allowlist or redact sensitive query keys before logging.

### 3.2 Insecure secret defaults silently "work" `[Medium]`
- **Where:** `internal/config/config.go:69-79`
- **Problem:** `QBITTORRENT_PASSWORD` defaults to `"changeme"`, URL/username to a
  hardcoded LAN IP and `admin`. A missing env var yields a working misconfiguration
  rather than a startup error (contrast `JWTSecret`/`TMDB_API_KEY`/`DATABASE_URL`, which
  are correctly required).
- **Fix:** Require qBittorrent credentials (no defaults), or warn loudly when defaults are
  used.

### 3.3 Long-lived, non-revocable JWTs `[High]`
- **Where:** `internal/auth/jwt.go:10` (`tokenTTL = 90d`); `service.go:120-127`
- **Problem:** 90-day HS256 tokens with no refresh/revocation; `ValidateToken` doesn't
  re-check user existence when a role claim is present. A leaked token is valid for 3
  months.
- **Fix:** Shorten access-token TTL with a refresh mechanism, or keep a per-user
  "tokens-valid-after" timestamp checked at validation. At minimum re-verify the user
  exists.

### 3.4 JWT parser not pinned to HS256; no iss/aud `[Medium]`
- **Where:** `internal/auth/jwt.go:40-55`
- **Fix:** `jwt.ParseWithClaims(..., jwt.WithValidMethods([]string{"HS256"}))`; consider
  setting/validating issuer and audience.

### 3.5 Unbounded torrent-file download read `[Medium]`
- **Where:** `internal/indexer/prowlarr/client.go:104-116`
- **Problem:** `io.ReadAll` on the `.torrent` download is uncapped; a misbehaving indexer
  proxy could OOM the process.
- **Fix:** Wrap in `io.LimitReader` (a few MB is plenty).

### 3.6 IDOR-style reads & missing authz refinements `[Low]`
- **Where:** `internal/users/handler.go:46-66` + `routes.go:8` (`GET /users/:id` readable
  by any authenticated user); rename `authMiddleware` → `proxyAuthMiddleware` with an
  invariant comment (`internal/app/middleware.go:11-44`).
- **Fix:** Restrict `GetUser` to self/admin (or confirm intended); document the
  proxy-strips-headers invariant.

### 3.7 `ValidateToken` swallows DB errors as "invalid" `[Medium]`
- **Where:** `internal/auth/service.go:122-125`
- **Problem:** A transient DB error during the empty-role lookup returns `Valid:false`,
  conflating infra failure with invalid token → spurious auth failures.
- **Fix:** Distinguish `ErrNotFound` (→ invalid) from other errors (→ surface 500).

---

## Phase 4 — Performance

External-API fan-out, redundant queries, and HTTP client tuning.

### 4.1 qBittorrent client has no HTTP timeout; non-`Ctx` calls ignore cancellation `[High]`
- **Where:** `internal/qbittorrent/service.go:43-52` (no `Config.Timeout`);
  `ListTorrents` calls `GetTorrents` not `GetTorrentsCtx` (`:110-118`); see also `sse.go:125`.
- **Problem:** A hung qBit connection blocks the 2s monitor loop and request handlers
  indefinitely and delays graceful shutdown. (Root cause of 2.4's hang trigger.)
- **Fix:** Set `Config.Timeout` (15–30s) and switch list/delete/files calls to their
  `...Ctx` forms passing the in-scope `ctx`.

### 4.2 `FilesCompleteByHash` is an N+1 against qBittorrent every tick `[High]`
- **Where:** `internal/qbittorrent/complete.go:89-108`, called `requests/watcher.go:147`
- **Problem:** One `TorrentFilesComplete` HTTP call per downloading torrent, every
  `interval` (5s). N concurrent downloads → N serial round-trips holding the tick busy.
- **Fix:** Batch via a single files/properties call where the API allows, or bound a
  worker pool; cache completion for hashes already confirmed complete.

### 4.3 TMDB warmer fan-out without rate limiting or pooled connections `[High]`
- **Where:** `internal/search/browse_warmer.go:56-87`, `browse_catalog.go:68-113`,
  `tmdb_client.go:38-40`
- **Problem:** Nested concurrency (up to 4 catalogs × 4 lists = 16 concurrent calls) with
  no rate limiter, and `http.DefaultTransport`'s `MaxIdleConnsPerHost=2` forces repeated
  TLS handshakes to one host. 429s aren't retried (`tmdb_client.go:126` fails the whole
  catalog on any non-200). Same pooling gap on the Prowlarr client (`client.go:36-38`).
- **Fix:** Shared rate limiter / single bounded pool across the warm cycle; custom
  `http.Transport` with `MaxIdleConnsPerHost` matching concurrency; 429 backoff/retry.

### 4.4 `waitForTorrent`/`AddTorrent` fetch ALL torrents to look up one hash `[Medium]`
- **Where:** `internal/qbittorrent/service.go:74,237`
- **Problem:** Empty-filter `GetTorrents` transfers and scans the whole torrent list,
  repeatedly (every 500ms in `waitForTorrent`).
- **Fix:** `TorrentFilterOptions{Hashes: []string{hash}}` for a targeted lookup.

### 4.5 Redundant per-list aggregate queries `[Medium]`
- **Where:** `internal/media/repository.go:55-61` (`mediaCountAndSize` = separate
  `COUNT(*)` and `SUM`)
- **Fix:** `SELECT COUNT(*), COALESCE(SUM(size_bytes),0)` in one scan.

### 4.6 Filesystem audit serialized under one shared 2-min budget `[Medium]`
- **Where:** `internal/health/service.go:99-103`, `fsaudit.go:53-127`
- **Problem:** Three roots walked sequentially in one timeout; a large movies walk starves
  shows/downloads, whose ctx-cancelled partial result is reported as if complete. `d.Info()`
  per file adds an `lstat`.
- **Fix:** Per-root timeouts, run roots concurrently, mark timeout-cancelled results
  distinctly from `Truncated`.

### 4.7 Hardlink hot-path double-stats source `[Low]`
- **Where:** `internal/hardlink/service.go:268-273` — `hardlinkPresent` stats source, then
  `sameInode` re-stats it → 3 stats where 2 suffice, on a per-file + per-progress-poll path.
- **Fix:** Stat source once and reuse for the `SameFile` comparison.

---

## Phase 5 — Code quality & consistency

Lower risk; reduces drift and future bugs. Batch as cleanup PRs.

### 5.1 Verify possibly-inverted quality ranking `[High — verify first]`
- **Where:** `internal/indexer/quality.go:13-23`
- **Problem:** Plain `1080p` (priority 9) outranks `1080p BluRay` (8), `1080p Remux` (7),
  and `4K Remux` (6), so a plain 1080p web release is selected as "best" over a 4K Remux.
  Likely inverted; drives `filterAndSortMovies`/`sortShows` selection.
- **Fix:** Confirm intent; if unintended, reorder so Remux/BluRay/4K tiers rank above
  plain resolutions.

### 5.2 Error-type-blind HTTP status mapping `[High]`
- **Where:** `internal/requests/handler.go:57-62` (`handlePost` collapses every error to
  500, including `media.ErrMediaNotFound` and validation errors); contrast
  `GetRequestTorrent` (`:110-121`), which does it right.
- **Fix:** Switch on sentinel/type → 404 for not-found, 400 for validation.

### 5.3 De-duplicate cross-package helpers `[Medium]`
- **Where:** `normalizePagination`/`calcTotalPages` duplicated in
  `requests/service.go:339-354` and `media/service.go:373-388`; `userIDFromContext` in
  `requests/handler.go:125-137`, `media/handler.go:95-107`, `auth/handler_keys.go:10-17`,
  `users/handler.go:20-28`; SSE Broker/Client wrappers + handler loops duplicated across
  `internal/sse` and `internal/qbittorrent` (with naming drift `ClientCount` vs
  `GetClientCount`, and the qbt loop missing the keep-alive ping).
- **Fix:** Extract shared `httputil`/`pagination` helpers and a single
  `ssehub.ServeHTTP(w, ctx, client)`.

### 5.4 Inconsistent pagination caps `[Medium]`
- **Where:** `media/handler.go:115` caps at 20 while `media/service.go:377` and
  `requests/service.go:343` cap at 100 — the service cap is dead for media.
- **Fix:** One cap, applied once (service level).

### 5.5 Health checks hardcode queue/table names `[Medium]`
- **Where:** `internal/health/inflight.go:28-35`, `exclusions.go:72-79`,
  `queues.go:20-49,76-78` hardcode `processing_queue` and queue-name/type literals that
  duplicate `pipeline` constants and ignore `WithTable`.
- **Fix:** Source table/queue/type names from shared constants used by both construction
  and health queries.

### 5.6 Use `errors.Is` consistently `[Low]`
- **Where:** `internal/media/repository.go:189,271`, `service.go:186,330` use
  `err == gorm.ErrRecordNotFound` while the rest of the codebase uses `errors.Is`.
- **Fix:** `errors.Is(...)` everywhere (survives wrapping).

### 5.7 Add error wrapping (`%w`) `[Low]`
- **Where:** bare `return nil, err` / `fmt.Errorf("...")` without `%w` across `auth`,
  `users/repository.go`, `qbittorrent/service.go:233,235,253`, `helpers.go:18`,
  `tmdb_client.go:119-134`, `prowlarr/client.go:142`.
- **Fix:** Wrap with operation context; define sentinels for HTTP-status failures so
  callers can `errors.Is`.

### 5.8 Dead / deprecated / weak code `[Low]`
- `internal/download/service.go:113` `detectIndexerFromURL` ignores its arg, returns a
  constant — inline or implement.
- `internal/qbittorrent/complete.go:25-29` deprecated `DownloadComplete` — grep + remove.
- `internal/search/models.go:3-8,31-36` (`MovieSearchResult`/`ShowSearchResult`),
  `tmdb_client.go:45-50` duplicate response struct, `browse.go:212` unreachable
  `"trending"` case — remove.
- `internal/media/models.go:86-121` `GetMediaIdentifier`/`GetIdentifier` appear unused —
  confirm project-wide, remove.
- `internal/indexer/service.go:306-316` `parseID` digit-squash fallback can collide two
  releases to one numeric ID — hash the GUID instead.
- `internal/indexer/map.go:93` `categoryTreeIsTV` prefix `"tv"` over/under-matches —
  classify by category ID ranges (2000s movie / 5000s TV) instead of name heuristics.
- `internal/users/models.go:12` free-text `Role` with no FK/CHECK against the seeded
  `roles` table — validate against `RoleAdmin`/`RoleUser` or add a constraint.
- `shared/logger/logger.go:14-25` panics on init and forces JSON output — env-driven
  config / `NewDevelopment` for local dev.

---

## Suggested execution order

| Phase | Theme | Gate before merge |
|-------|-------|-------------------|
| 1 | Data integrity & state machine | Add tests reproducing the dedup/registration races and the watcher resurrection |
| 2 | Lifecycle & leaks | Verify clean graceful shutdown (no leaked goroutines/pools) under `go test -race` + a shutdown integration test |
| 3 | Security hardening | Confirm nginx still strips/overwrites `X-User-*`; log-redaction test |
| 4 | Performance | Load-test the watcher tick and the browse warmer; assert no 429 storms |
| 5 | Quality cleanup | `go vet ./...`, `go test ./... -race`, then ship as small PRs |

**Recommended first PR:** Phase 1.1 + 1.2 (the two `Critical` data-corruption races) and
Phase 2.1–2.3 (shutdown bypass + pool leak + SSE hub deadlock), since together they cover
the highest-blast-radius defects and share the "add tests + transaction/lifecycle"
groundwork.

### Verified-correct (no action)
Login uses constant-time bcrypt compare with a uniform error; SQL is parameterized
throughout (GORM bind params + `validateIdentifier` for dynamic identifiers);
`PasswordHash` is `json:"-"`; the queue dequeue uses `SELECT ... FOR UPDATE SKIP LOCKED`
with `status='processing'` guards; the SSE fan-out is single-goroutine and drops on full
buffer (no slow-consumer head-of-line blocking); `WriteTimeout:0` for SSE is deliberate
with `ReadTimeout`/`IdleTimeout` set; worker panic recovery converts handler panics into
retryable failures.
