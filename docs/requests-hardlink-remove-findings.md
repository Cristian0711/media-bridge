# Requests → Hardlink → Remove: Audit Findings & Fix Order

This document records a backend audit of the download/remove pipeline (May 2026). Use it as the single source of truth for what to fix and in what order.

## Pipeline overview

```
HTTP POST
  → requests_processing_queue
       ├─ download  → download_processing_queue → media row + hardlink_processing_queue
       └─ remove    → remove_processing_queue   → disk cleanup + delete media row

DownloadCompletionWatcher (poll ~5s)
  → marks request "downloaded" when hardlinks complete AND qBittorrent reports ready
```

**Key files**

| Area | Path |
|------|------|
| Bootstrap / workers | `backend/internal/app/bootstrap.go` |
| Requests routing | `backend/internal/requests/queue_processor.go`, `service.go`, `repository.go` |
| Download worker | `backend/internal/download/queue_processor.go`, `service.go` |
| Hardlink | `backend/internal/hardlink/service.go`, `progress.go`, `queue_processor.go`, `cache.go` |
| Remove | `backend/internal/remove/service.go`, `queue_processor.go` |
| Finalize | `backend/internal/requests/completion_watcher.go` |
| Shared queue | `backend/shared/processing-queue/` |
| qBittorrent readiness | `backend/internal/qbittorrent/complete.go` |

---

## Findings

Severity: **Critical** · **High** · **Medium** · **Low**

### Critical

#### C1 — Download queue retries are not idempotent

**Where:** `download/queue_processor.go` — `Add` → `CreateFromRequest` → `hardlinkProcessor.Enqueue`

**Problem:** If the job fails after media is created (or after hardlink enqueue) and retries (up to 100×), the worker runs the full path again. `CreateFromRequest` always inserts new movie/show + media rows. Each retry can enqueue another hardlink job.

**Trigger:** Container restart mid-job, DB/queue blip after `CreateFromRequest`, hardlink enqueue error.

**Impact:** Duplicate media rows, duplicate hardlink jobs, orphaned DB state, wrong Plex/library paths.

**Note:** `Add` tolerates `ErrTorrentExists`; media creation does not.

---

#### C2 — Requests queue can double-enqueue downstream jobs

**Where:** `requests/queue_processor.go` — enqueue download/remove, then `UpdateStatus`

**Problem:** If child `Enqueue` succeeds and `UpdateStatus` fails, the requests job retries and enqueues a **second** download or remove job for the same `request_entry_id`. No dedup on `(queue_name, request_entry_id)`.

**Impact:** Same as C1 at the requests layer; duplicate work and confusing queue state.

---

#### C3 — Completion watcher vs remove (race on download status)

**Where:** `completion_watcher.go` + `remove/queue_processor.go`

**Problem:** Remove sets the **remove** request to `processed` early, but the **download** request stays `downloading` until `CancelDownloadsByMediaID` runs **after** disk cleanup and media deletion. While remove runs, the watcher can still set the download to `downloaded` if hardlinks + qBit look ready.

**Impact:** Status flicker `downloading` → `downloaded` → `cancelled`; possible finalize during active deletion (mitigated by inode cleanup, still wrong UX/state).

---

### High

#### H1 — Remove marked `processed` before work finishes

**Where:** `requests/queue_processor.go` sets `processed` when forwarding to remove queue.

**Problem:** UI/dedup treat remove as done while `remove_processing_queue` may still be running or retrying.

**Impact:** Misleading status; `FindActiveRemoveByMediaID` blocks duplicate removes correctly but label is wrong.

---

#### H2 — Download failures rarely update request status

**Where:** Only `hardlink/queue_processor.go` sets request `failed`.

**Problem:** Indexer errors, qBit add failures (except exists), `CreateFromRequest` errors leave request at `downloading` for many retries.

**Impact:** Stuck “downloading” in UI with no terminal state.

---

#### H3 — `UpdateMediaID` failure is non-fatal

**Where:** `download/queue_processor.go`

**Problem:** If request row is not linked to `media_id`, `CancelDownloadsByMediaID` during remove may not find the download request.

**Impact:** Download request not cancelled after remove; weaker cross-flow cleanup.

---

#### H4 — Stale job recovery can overlap long handlers

**Where:** `processing-queue/worker.go` — default `WorkerTimeout` 10m, recovery every 1m.

**Problem:** A long hardlink/remove run can be requeued as `pending` while the first handler still runs. Single worker limits concurrent handlers today, but logic still assumes one execution per job id.

**Impact:** Duplicate hardlink attempts or duplicate remove walks on retry.

---

#### H5 — In-flight hardlink vs remove (partial mitigation)

**Where:** `hardlink.CancelByMediaID` + `remove/service.go`

**Problem:** Cancel marks queue rows `failed` but does not stop a running hardlink handler. Remove uses inode matching and a second dest walk.

**Impact:** Residual window for new hardlinks after remove’s second pass; drain check should force retry (documented in remove package).

---

### Medium

#### M1 — Dedup TOCTOU on concurrent POSTs

**Where:** `requests/service.go` — `FindActive*Download` then `Create`

**Problem:** No unique DB constraint; two simultaneous POSTs can both pass dedup.

**Impact:** Two torrents for same movie/quality in qBit.

---

#### M2 — `downloading` set before torrent/media exist

**Where:** `requests/queue_processor.go`

**Problem:** Status becomes `downloading` when forwarding to download queue, not when qBit/media exist.

**Impact:** UI shows downloading while job is only queued.

---

#### M3 — Finalize blocked if torrent missing from qBit

**Where:** `completion_watcher.go` — `torrentByHash` lookup

**Problem:** If torrent was removed from qBit but library hardlinks exist, request never becomes `downloaded`.

**Impact:** Stuck `downloading` forever.

---

#### M4 — `resolvePaths` failure still deletes media row

**Where:** `remove/service.go` returns `nil` on path resolution failure; `remove/queue_processor.go` still calls `RemoveFromRequest`.

**Impact:** Orphan files on disk, empty library entry in DB.

---

#### M5 — Per-file hardlink errors vs `total = len(files)`

**Where:** `hardlink/service.go` — `runHardlink`

**Problem:** Cross-device or permission errors skip `linked++` but count toward `total`; retries until max attempts → `failed` without a dedicated permanent error for “wrong filesystem layout”.

**Impact:** Long retry loops; unclear operator message.

---

#### M6 — Progress / torrent-info cache staleness

**Where:** `hardlink/cache.go`, `requests/torrent_info_cache.go` (3s TTL)

**Problem:** UI and watcher can lag briefly after state changes.

**Impact:** Cosmetic / timing only if caches invalidated correctly (hardlink invalidates on `Hardlink()`).

---

#### M7 — Watcher does not re-check status before finalize

**Where:** `completion_watcher.tryFinalize`

**Problem:** No `WHERE status = 'downloading'` on finalize update; no check for `cancelled` immediately before write.

**Impact:** Contributes to C3.

---

### Low

#### L1 — Hardcoded download paths

**Where:** `download/service.go` — `/mnt/plexmedia/plex-debug/downloads`

**Impact:** Mismatch with configured movies/shows if env paths differ.

---

#### L2 — Single worker per queue

**Impact:** Head-of-line blocking; one slow job delays all others in that queue.

---

#### L3 — Watcher load: `GetTorrentFiles` per request per tick

**Where:** `ReadyForLibrary` inside `tryFinalize`

**Impact:** qBittorrent API load with many active downloads.

---

#### L4 — Torrent hash map key normalization

**Where:** `TorrentsByHash` vs stored `TorrentHash`

**Impact:** Possible missed lookup if casing/format differs (environment-dependent).

---

### Already solid (keep when fixing)

- Postgres queue: `FOR UPDATE SKIP LOCKED`, conditional `Complete`/`Fail`
- Hardlink success path checks queue status after cancel (`GetStatus`)
- Remove: qBit removed first, inode-based hardlink cleanup, second dest walk, save-path drain before DB delete
- Completion watcher: `tickBusy` + requires hardlink complete **and** qBit ready
- Hardlink: `sameInode` / stale dest replacement

---

## Remediation order

Work in this order. Do not skip earlier items without accepting the risk they guard against.

| Phase | ID | Title | Findings | Goal |
|-------|-----|--------|----------|------|
| **1** | **1.1** | Idempotent download worker | C1 | **Done** — reload request; reuse `media_id` / scope lookup; skip create; hardlink enqueue only if no active job |
| **1** | **1.2** | Idempotent requests forward | C2 | **Done** — skip enqueue if child job exists (pending/processing/completed) or request status ≠ pending |
| **1** | **1.3** | Conditional status updates | C3, M7 | **Done** — `MarkDownloadedIfDownloading`; cancel downloads at **start** of remove |
| **2** | **2.1** | Download failure → request `failed` | H2 | **Done** — `MarkFailedIfInFlight` on permanent/final download queue attempt |
| **2** | **2.2** | Fail-fast `UpdateMediaID` | H3 | **Done** (C1) — link failure fails the download job |
| **2** | **2.3** | Remove status lifecycle | H1 | **Done** — `pending` → `removing` → `removed`; `failed` on final remove error |
| **3** | **3.1** | Queue recovery / long jobs | H4 | **Done** — `LongRunningQueueOptions()` (3h timeout) for hardlink/remove; 30m download; 10m requests |
| **3** | **3.2** | Hardlink↔remove hardening | H5 | **Done** — hardlink defers when remove queue has active job; `media_id` on remove payload |
| **4** | **4.1** | Dedup constraint or lock | M1 | **Done** — transactional `CreateMovieDownloadIfAbsent` / `CreateShowDownloadIfAbsent`; `queued` in `activeDownloadStatuses` |
| **4** | **4.2** | Finer download status | M2 | **Done** — forward → `queued`; download worker → `downloading` via `MarkDownloadingIfQueued` |
| **4** | **4.3** | Finalize without qBit torrent | M3 | **Done** — watcher finalizes when hardlink complete and torrent missing from qBit map |
| **4** | **4.4** | Remove path resolution failures | M4 | **Done** — `resolvePaths` error surfaces to queue for retry |
| **5** | **5.1** | Hardlink permanent errors | M5 | **Done** — `EXDEV` → `ErrPermanentFailure` with operator-facing message; request marked `failed` |
| **5** | **5.2** | Configurable download paths | L1 | **Skipped** — keep hardcoded download path (explicit product decision) |
| **5** | **5.3** | Hash normalization | L4 | **Done** — `qbittorrent.NormalizeHash` on store, map keys, and lookups |
| **6** | **6.1** | Cache / watcher tuning | M6, L3 | **Done** — torrent-info cache invalidated on status change; watcher batches `GetTorrentFiles` per hash per tick |
| **6** | **6.2** | Parallel workers (optional) | L2 | **Done** — `*_QUEUE_WORKERS` env vars; `StartWorker` × N per queue |

---

## Definition of done (per phase)

### Phase 1 — Correctness under retry and remove

- [x] Re-running download queue job for same `request_entry_id` does not create a second media row (C1 / 1.1)
- [x] Re-running requests queue job does not create duplicate download/remove queue rows (C2 / 1.2)
- [x] Remove in progress: download request never transitions to `downloaded` (C3 / 1.3)
- [ ] Tests or manual script: restart backend mid-download job → single media row

### Phase 2 — Observable request lifecycle

- [x] Indexer/qBit/media create failure ends in `failed` (or explicit retry cap message) (2.1)
- [x] Every successful download has `request.media_id` set (2.2 / C1)
- [x] Remove request status reflects in-flight vs completed removal (2.3)

### Phase 3 — Queue reliability

- [x] No duplicate handler side effects when recovery requeues a slow job (3.1 — 3h worker timeout on hardlink/remove)
- [x] Documented timeout values for production torrent sizes (3.1 — see table below)

#### Queue worker timeouts (`processing-queue/timeouts.go`)

| Queue | Worker timeout | Recovery interval |
|-------|----------------|-------------------|
| `requests_processing_queue` | 10 min | 1 min (default) |
| `download_processing_queue` | 30 min | 2 min |
| `hardlink_processing_queue` | **3 hours** | 5 min |
| `remove_processing_queue` | **3 hours** | 5 min |

Large 50–80 GB releases can spend over an hour hardlinking; the previous 10 min default caused stale recovery to requeue jobs while the first handler was still running.

### Phase 4 — Dedup, status granularity, finalize policy

- [x] Concurrent duplicate POSTs deduped in a transaction (4.1 / M1)
- [x] Download status: `pending` → `queued` → `downloading` → `downloaded` (4.2 / M2)
- [x] Watcher finalizes when hardlinks complete but torrent gone from qBit (4.3 / M3)
- [x] Remove retries when path resolution fails (4.4 / M4)

### Phase 5 — Hardening

- [x] Cross-device hardlink permanent failure surfaced (5.1 / M5)
- [x] Hash normalization in map/DB/watcher lookups (5.3 / L4)
- [ ] Configurable download paths (5.2 / L1) — **skipped**; hardcoded path retained

### Phase 6 — Optional tuning

- [x] Torrent-info cache invalidated on request status change (6.1 / M6)
- [x] Completion watcher batches qBittorrent file checks per hash per tick (6.1 / L3)
- [x] Configurable parallel queue workers (6.2 / L2)

---

## Testing checklist (after each phase)

1. **Happy path:** movie download → hardlinks appear → watcher sets `downloaded`
2. **Restart:** kill backend during download queue job → resume → one media row, one hardlink job
3. **Remove during download:** remove while `downloading` → no `downloaded`; ends `cancelled` or removed cleanly
4. **Remove after complete:** library + downloads empty; media row gone; download request cancelled
5. **Failed indexer:** request reaches `failed`, not infinite `downloading`
6. **Torrent modal:** progress lists all torrent files; poll interval stable (no request storm)

---

## Related frontend fixes (separate track)

| Item | Status |
|------|--------|
| Torrent modal `$effect` loop (poll every ms) | Fixed — `untrack` + in-flight guard |
| Show all files in hardlink progress (no sidecar filter) | Fixed — count every qBit file |

---

## Changelog

| Date | Author | Notes |
|------|--------|-------|
| 2026-05-22 | Audit | Initial findings and fix order |
| 2026-05-22 | C1 fix | Idempotent `download/queue_processor`, `FindExistingDownloadMediaID`, `HasActiveJobForMediaID` |
| 2026-05-22 | C2 fix | Idempotent `requests/queue_processor`, `HasForwardJobByPayloadField`, `HasForwardJobForRequest` |
| 2026-05-22 | C3 fix | `MarkDownloadedIfDownloading`, cancel downloads before `remove.Process` |
| 2026-05-22 | Phase 2 | Download/remove `failed` on final attempt; remove `removing` → `removed` |
| 2026-05-22 | Phase 3 | Long-running queue timeouts; hardlink defers during active remove |
| 2026-05-22 | Phase 4 | Transactional download dedup; `queued` status; watcher hardlink-only finalize; remove path retry |
| 2026-05-22 | Phase 5 | Cross-device hardlink permanent failure; torrent hash normalization (5.2 skipped) |
| 2026-05-22 | Phase 6 | Torrent-info cache invalidation; batched watcher file checks; parallel queue workers |

#### Queue worker concurrency (6.2)

| Env var | Default | Queue |
|---------|---------|--------|
| `REQUESTS_QUEUE_WORKERS` | 1 | `requests_processing_queue` |
| `DOWNLOAD_QUEUE_WORKERS` | 2 | `download_processing_queue` |
| `HARDLINK_QUEUE_WORKERS` | 2 | `hardlink_processing_queue` |
| `REMOVE_QUEUE_WORKERS` | 1 | `remove_processing_queue` |

Workers use `FOR UPDATE SKIP LOCKED` — safe to run >1 per queue after phase-1 idempotency fixes.
