# Request Removal & Hardlink — Live Bugs, Races & Fix Plan

Fresh audit (May 2026) of the **remove request** and **hardlink** pipelines against the
*current* code. The earlier audit (`docs/requests-hardlink-remove-findings.md`) is mostly
marked **Done**; this document only lists issues that are **still present in the code today**,
each with a concrete fix. Treat this as the working backlog.

> Terminology: "deleting a request" = removing media from the library via the async
> `movie_remove` / `show_remove` pipeline. No rows are ever `DELETE`d from the `requests`
> table; requests transition to `removed` / `failed`.

## Pipeline (current)

```
POST /requests/movies|shows/remove
  → requests.Service.RequestMovieRemove / RequestShowRemove   (dedup + create + enqueue)
  → requests_processing_queue  → QueueProcessor.forwardRemove  (status → "removing")
  → remove_processing_queue    → remove.Processor.processRemoveJob
       1. CancelDownloadsByMediaID         (stop completion watcher)
       2. remove.Service.Process           (cancel hardlink jobs, qBit, disk cleanup)
       3. media.Service.RemoveFromRequest  (DB cascade: media → movie/show_entry → show)
       4. MarkRemovedIfRemoving            (status → "removed")
```

Key files:

| Area | Path |
|------|------|
| Remove request entry | `backend/internal/requests/service.go` |
| Requests forwarder | `backend/internal/requests/queue_processor.go` |
| Remove worker | `backend/internal/remove/queue_processor.go` |
| Disk cleanup | `backend/internal/remove/service.go` |
| Media DB cascade | `backend/internal/media/service.go`, `repository.go` |
| Hardlink worker | `backend/internal/hardlink/queue_processor.go`, `service.go` |
| Completion watcher | `backend/internal/requests/completion_watcher.go` |
| Shared queue | `backend/shared/processing-queue/queue.go`, `worker.go` |

---

## Findings

Severity: **Critical** · **High** · **Medium** · **Low**

### R1 — Remove dedup is a non-transactional check-then-create (TOCTOU) · **High**

**Where:** `requests/service.go` — `RequestMovieRemove` (192–243) and `RequestShowRemove` (245–296).

```go
if active, err := s.repo.FindActiveRemoveByMediaID(ctx, req.MediaID, "movie_remove"); err != nil {
    return nil, err
} else if active != nil {
    return &RequestAck{Status: "accepted", Message: "movie remove already in progress"}, nil
}
// ... gap ...
if err := s.repo.Create(ctx, entry); err != nil { return nil, err }
```

The download path was hardened with a transactional `CreateMovieDownloadIfAbsent` /
`CreateShowDownloadIfAbsent` (`repository.go` 250–318), but **remove was never given the same
treatment**. Two near-simultaneous remove POSTs for the same `media_id` can both pass
`FindActiveRemoveByMediaID` and both `Create`, producing **two remove request rows → two
`remove_processing_queue` jobs → two workers racing to delete the same files**.

**Impact:** Mostly self-healing (disk ops tolerate `ENOENT`; the second worker's
`GetMediaByID` returns `ErrMediaNotFound` and short-circuits), but it churns the queue, double-fires
SSE/`MarkRemovedIfRemoving`, and is the same class of bug already fixed for downloads.

**Fix:** Add a transactional `CreateRemoveIfAbsent(ctx, req, mediaID, type)` to `Repository`
mirroring `CreateMovieDownloadIfAbsent`: inside one `db.Transaction`, `SELECT ... WHERE media_id
= ? AND type = ? AND status IN activeRemoveStatuses` then `Create` only if none found. Call it
from both remove service methods and drop the standalone `FindActiveRemoveByMediaID` + `Create`.

---

### R2 — `Create` then `Enqueue` is not atomic → poison-pill stuck request · **Critical**

**Where:** all four request entry points in `requests/service.go` (105/115, 170/180, 230/233,
283/286). Pattern:

```go
if err := s.repo.Create(ctx, entry); err != nil { return nil, err }     // row committed
if err := s.processor.Enqueue(ctx, QueuePayload{...}); err != nil {      // separate tx
    return nil, err                                                      // row already exists!
}
```

If `Enqueue` (insert into `requests_processing_queue`) fails — pool exhaustion, DB blip,
context cancel after the request row commits — the **request row is left in `pending` with no
queue job to ever drive it forward**. Nothing reconciles orphaned `pending` rows.

Worse, that orphan **poisons future dedup**:
- Removes: `activeRemoveStatuses = {pending, removing}` → every later remove POST returns
  *"already in progress"* forever.
- Downloads: `activeDownloadStatuses = {pending, queued, downloading}` → same permanent block.

**Impact:** A single transient enqueue error permanently wedges remove/download for that
media/title until someone manually edits the DB. Highest-severity correctness issue found.

**Fix (pick one):**
1. **Transactional outbox (preferred):** insert the request row and the queue job in the *same*
   Postgres transaction (the queue already lives in Postgres). Either push enqueue into
   `Create*IfAbsent`'s `tx`, or add an outbox row consumed by the worker.
2. **Reconciler sweep:** a startup + periodic job that finds `pending` requests (and `removing`
   with no `remove_processing_queue` job, `queued` with no download job) older than N seconds and
   re-enqueues them. Cheap, also recovers from older orphans.
3. **At minimum**, age out the dedup: ignore `pending` rows with no matching queue job when
   checking "already in progress."

---

### R3 — Media DB cascade is not transactional → orphan `movie`/`show`/`show_entry` rows · **High**

**Where:** `media/service.go` `removeMovie` (306–353) and `removeShow` (355–429); deletes via
`repository.go` `DeleteMediaByIDs` / `DeleteMoviesByIDs` / `DeleteShowEntriesByIDs` /
`DeleteShowByID` — each a **separate statement, no enclosing transaction**.

`removeShow` does: delete media row → count → delete show_entry → count → delete show. A crash,
context cancellation, or DB error **between** these statements leaves orphan `show_entry` / `show`
(or orphan `movie`) rows. Because `DeleteMediaByIDs` already hard-deleted the `media` row, the
retry path can never finish the cascade:

- `remove.Service.Process` retry: `GetMediaByID` → `ErrMediaNotFound` → returns `nil` (success).
- `RemoveFromRequest` retry: `repo.FindByID(media)` → `ErrRecordNotFound` → returns `nil`.

So the orphan parent rows are **never** cleaned up.

**Impact:** Orphaned `movie`/`show`/`show_entry` rows accumulate; a stale `show` row can keep a
title visible in the library or skew "already exists" dedup on re-download.

**Fix:** Wrap the whole cascade in a single `repo.WithTx` / `db.Transaction` so media + parent
deletes commit atomically. Long-term, prefer DB-level `FOREIGN KEY ... ON DELETE CASCADE`
(or `RESTRICT` + explicit cascade) so the relational integrity isn't reimplemented in Go.

---

### R4 — Drain gate ignores `destPath`; in-flight hardlink can survive removal · **Medium**

**Where:** `remove/service.go` `Process` steps 5–8 (178–212). The post-delete gate only verifies
**source** files are gone:

```go
// (8) Gate: confirm savePath holds zero regular files.
if paths.SavePath != "" {
    remaining, _ := countRegularFiles(ctx, paths.SavePath)
    if remaining > 0 { return fmt.Errorf("...still contains %d regular file(s)...") }
}
```

`CancelByPayloadField` does **not** stop an already-running hardlink handler (documented in
`queue.go` 291–293). The second dest walk (step 7) mitigates most of the window, but a racing
hardlink worker can still `os.Link` a file into the **library destination** *after* step 7 and
*before* the source is unlinked. The gate never re-checks `destPath`, so that orphan hardlink
survives in `/movies` or `/shows` even though the media row is then deleted — Plex keeps showing
a file for "removed" media.

**Impact:** Rare orphan hardlink in the library after removal; invisible to the DB.

**Fix:** Make the gate cover the destination too. Either:
- Loop steps 5–7 until **two consecutive passes** remove zero hardlinks *and* `countRegularFiles`
  on both `savePath` and a `destPath` inode re-scan return zero; or
- Add a final `removeHardlinksByInode(destPath, keys)` pass inside the gate and fail (retry) if it
  removed anything. Long-term, give the hardlink worker a per-file cancellation check (consult
  `GetStatus`/remove-guard between links) so it stops promptly when cancelled.

---

### R5 — Completion watcher can finalize `downloaded` in the window before remove cancels · **Medium**

**Where:** `remove/queue_processor.go` `processRemoveJob` (119–134) cancels downloads at job start;
`completion_watcher.go` `finalizeDownloaded` (167–190) uses `MarkDownloadedIfDownloading`.

`CancelDownloadsByMediaID` only flips rows in `activeDownloadStatuses = {pending, queued,
downloading}` (`repository.go` 152–182). If the watcher's `MarkDownloadedIfDownloading` commits a
`downloading → downloaded` transition **just before** the remove worker runs its cancel, the
download request is now `downloaded` and the cancel's `WHERE status IN (...)` no longer matches it.
The media gets removed but the download request is **left as `downloaded`** — an inconsistent,
orphaned status row.

**Impact:** Cosmetic/state inconsistency: a "downloaded" request for media that no longer exists.

**Fix:** Make `CancelDownloadsByMediaID` also clean up a recently-`downloaded` download request for
that `media_id` (e.g. include `downloaded` in the cancel set for the remove path, or flip it to a
`removed`/`cancelled` terminal state). Alternatively gate the watcher with a "remove in progress"
check (it already has `removeGuard` plumbing on the hardlink side).

---

### R6 — Stale-job recovery can overlap genuinely long handlers · **Medium**

**Where:** `processing-queue/worker.go` `recoverStale` (155–179); timeout from
`LongRunningQueueOptions()` = **3h** worker timeout, 5m recovery (used by hardlink + remove queues).

Recovery requeues any row in `processing` whose `started_at` is older than the worker timeout —
it does **not** check whether the original worker is still alive. A hardlink/remove job that
legitimately runs > 3h (very large library, slow disk) gets a *second* concurrent worker for the
same `media_id`. Remove double-walk and hardlink double-link are mostly idempotent, but the
codebase otherwise assumes one execution per job id.

**Impact:** Low frequency, but duplicated disk work and the theoretical races R1/R4 guard against.

**Fix:** Replace the fixed timeout with a **lease/heartbeat**: the running worker periodically
bumps `last_processed_at` (or a `lease_until`), and recovery only requeues rows whose lease has
expired. Until then, ensure handlers stay strictly idempotent (they mostly are).

---

### R7 — Forward status updates are unconditional · **Low**

**Where:** `requests/queue_processor.go` `forwardRemove` (164–185) and `forwardDownload`
(142–162) end with `p.repo.UpdateStatus(ctx, req.ID, "removing"/"queued")` — an unconditional
write, not a guarded transition.

The `req.Status != "pending"` early-return in `processRequest` (100–106) prevents re-entry today,
but with `>1` requests workers or recovery overlap an unconditional `UpdateStatus` could clobber a
later `removed`/`failed`/`cancelled` state back to `removing`/`queued`.

**Fix:** Use guarded transitions (`MarkRemovingIfPending` / `MarkQueuedIfPending` returning
`bool`), consistent with the rest of the status machine (`updateStatusWhen`).

---

### R8 — Inconsistent hard vs soft delete leaves orphan `show_entry` rows · **Low**

**Where:** `media/repository.go`. `DeleteMediaByIDs` (236–238) uses `.Unscoped()` (hard delete),
but `Media` has a `DeletedAt`. `DeleteShowEntriesByIDs` (306–308) does **not** use `.Unscoped()`
and `ShowEntry` *has* a `DeletedAt` (`models.go` 85) → **soft delete**. `Movie` and `Show` have no
`DeletedAt` → hard delete.

So on show removal, `show_entry` rows are soft-deleted and linger forever, while everything else is
hard-deleted. Functionally benign for queries (GORM filters soft-deleted), but the rows accumulate
and the policy is inconsistent.

**Fix:** Decide one policy. Simplest: add `.Unscoped()` to `DeleteShowEntriesByIDs` to hard-delete
(matching media), or give all four tables `DeletedAt` and soft-delete uniformly.

---

### R9 — Request rows grow unbounded · **Low**

`requests` rows are never pruned; `removed`/`failed`/`downloaded`/`cancelled` history accumulates
forever. Consider a retention sweep (e.g. delete terminal rows older than N days) or an archive
table. Same applies to `completed`/`failed` rows in `processing_queue`.

---

### R10 — Destination path derived from live metadata at remove time · **Low**

`remove/service.go` `resolvePaths` (406–439) rebuilds the library folder from the *current*
`row.Name` / `row.Quality` / show name. If that metadata was edited after the hardlink was
created, the computed `destPath` won't match the real folder and `removeHardlinksByInode` walks the
wrong directory → orphan library files. (Inode matching saves us only if we walk the right tree.)

**Fix:** Persist the literal destination folder (and save path) on the media row at hardlink time
and use the stored value for removal instead of recomputing.

---

### R11 — No automated coverage for the remove/hardlink race paths · **Low** · **Done**

Tests live under `backend/tests/`:

| Package | File | Covers |
|---------|------|--------|
| `tests/requests` | `repository_test.go` | R1 dedup, R2 atomic enqueue/rollback, R5 cancel downloaded, R7 guarded transitions, reconciler list helpers, R9 purge |
| `tests/requests` | `handler_remove_test.go` | Remove HTTP handlers |
| `tests/media` | `remove_cascade_test.go` | R3 transactional cascade, R8 hard delete, R10 library path |
| `tests/remove` | `process_test.go` | R4 disk cleanup + library_path removal, idempotent media-gone |
| `tests/processingqueue` | `gorm_enqueue_test.go` | R2 GormTx enqueue commit/rollback |
| `tests/testhelpers` | `db.go` | Shared SQLite + queue table setup |

---

## Suggested fix order

| Phase | IDs | Why first |
|-------|-----|-----------|
| **1 — Correctness** | R2, R3, R1 | Poison-pill stuck requests + orphan DB rows are the only issues that need manual DB surgery to recover; R1 closes the matching dedup hole. |
| **2 — Disk/library integrity** | R4, R5 | Eliminate orphan library hardlinks and inconsistent download status after removal. |
| **3 — Robustness** | R6, R7 | Lease-based recovery + guarded status transitions remove the "assumes one execution" foot-guns. |
| **4 — Hygiene** | R8, R9, R10, R11 | Delete-policy consistency, retention, path persistence, tests. |

## Definition of done

- [x] R1: transactional `CreateRemoveIfAbsent`; concurrent duplicate remove POSTs create one row.
- [x] R2: request row + queue job are atomic (or a reconciler re-drives orphan `pending`/`removing`/`queued`); a forced enqueue failure no longer wedges future requests.
- [x] R3: media cascade is atomic; killing the process mid-cascade leaves no orphan `movie`/`show`/`show_entry`.
- [x] R4: after remove, neither `savePath` nor `destPath` contains a file with a source inode; a hardlink worker racing the delete cannot leave a library file.
- [x] R5: no download request remains `downloaded` for a removed `media_id`.
- [x] R6: a >timeout handler is not double-executed (lease/heartbeat).
- [x] R7: status transitions are guarded (`*If*` helpers) everywhere.
- [x] R8: one delete policy across `media`/`movie`/`show`/`show_entry`.
- [x] R9/R10/R11: retention sweep, persisted dest path, race tests.

## Already solid — keep when fixing

- Postgres queue: `FOR UPDATE SKIP LOCKED`, conditional `Complete`/`Fail` (`rowsAffected == 0` detection).
- Remove ordering: cancel hardlink jobs → qBit forget (closes FDs) first → inode-based delete → source delete → second dest walk → savePath drain gate before DB delete.
- Hardlink success path re-checks queue status (`GetStatus`) before flipping request to `downloaded` (mid-flight cancel guard).
- `sameInode` / stale-dest replacement; `EXDEV` surfaced as `ErrPermanentFailure`.
- Completion watcher `tickBusy` mutex; requires hardlink complete **and** qBit ready.
- Download dedup is already transactional (`Create*DownloadIfAbsent`) — mirror it for removes (R1).
