# Backend Observability Plan — Structured Logging + Distributed Tracing

A plan to turn the backend's ad-hoc logging into a disciplined, correlated
observability stack: **one structured logger (`log/slog`)**, **OpenTelemetry
traces exported over OTLP to a collector**, and a **severity policy where an
`ERROR` log genuinely means "something broke and a human should look."**

This follows the conventions in `docs/backend-quality-improvements.md`:
correctness/discipline first, then plumbing, then polish. Phases are ordered so
each is independently shippable and the codebase is never half-migrated in a way
that breaks builds.

---

## Why now — current state (June 2026)

| Area | Today | Problem |
|------|-------|---------|
| Log library | `zap` global singleton (`shared/logger`, 22 files) **+** `log/slog` default text handler in the queue worker (`shared/processing-queue/worker.go`) | Two libraries, two output formats (zap JSON vs slog text), no shared config |
| Context | `request_id` is set on the gin context (`internal/app/middleware.go`) and logged once at the boundary (`request_logger.go`) | Deep logs (services, repos, workers) carry **no** `request_id` — an error 5 layers down can't be tied to the request that caused it |
| Tracing | None (no OTel deps) | No way to see a request flow across HTTP → queue → qBittorrent → DB; no latency attribution |
| Severity | ~33 `Error`, ~41 `Warn`, ~59 `Info`, ~17 `Debug`, 1 `Fatal`. Request logger logs **every** request at `Info`, including 4xx/5xx | `ERROR` is not trustworthy as an alert signal; expected conditions and client mistakes are mixed in with real failures |
| Error flow | Errors wrapped with `%w` (after Phase 5) but logging-vs-returning is inconsistent — some errors logged at every layer | Double/triple logging of the same failure; noise |
| Edge (nginx) | Generates/forwards `X-Request-ID` only (`nginx/nginx.conf:64`); makes a `/auth/validate` sub-request per request; **no `traceparent`, no OTel** | The real entry point emits no trace context, so a trace can't start at the edge until nginx is instrumented (Phase 3.0) |

**Foundations we already have:** `context.Context` is threaded through nearly
every service/repo/worker signature, and an `X-Request-ID` exists at the HTTP
edge. That makes ctx-based logging and trace propagation low-friction.

**Decisions taken:** standardize on `log/slog`; export OpenTelemetry over **OTLP
to an OpenTelemetry Collector** (vendor-neutral — route to Tempo/Loki/Jaeger/etc.
without code changes).

---

## Target architecture

```
 client
   │ (may send W3C traceparent)
   ▼
┌──────────────────────────────────────────────────────────────┐
│ nginx / OpenResty  ── TRACE ORIGIN ──                          │
│  - OTel: start ROOT span "ingress"                             │
│  - generate/continue traceparent; keep X-Request-ID           │
│  - auth subrequest ─► backend /auth/validate  (same trace,    │
│    child span; today an orphan call — see Phase 3.0)          │
│  - inject `traceparent` + `X-Request-ID` on the proxied req   │
└───────────────┬────────────────────────────────────────────────┘
                │ traceparent + X-Request-ID
                ▼
┌──────────────────────────────────────────────────────────────┐
│ gin: trace middleware (otelgin)                                │
│  - CONTINUE the nginx trace (extract traceparent)             │
│  - seed ctx: trace_id, span_id, request_id, user              │
│  - put *slog.Logger in ctx                                    │
└───────────────┬────────────────────────────────────────────────┘
                │ ctx carries span + logger
   ┌────────────┼───────────────────────────────┬───────────────────────┐
   ▼            ▼                                ▼                       ▼
 service/repo  qBittorrent / TMDB        enqueue: inject traceparent   SSE / monitor
 (gorm otel)   (otelhttp child spans)    into job payload (producer    (background
                                          span records the enqueue)     spans)
                                                 │
                                                 │ job row carries traceparent
                                                 ▼   (may run minutes–hours later)
                                   processing-queue worker
                                    - NEW span per job, with a span LINK
                                      back to the producing request span
                                    - same trace_id propagated for LOG correlation
                                    - child spans: hardlink walk / qBit / DB
                ┌──────────────────────────────┴───────────────────────┐
                │ slog Handler (JSON): injects trace_id/span_id/        │
                │ request_id from ctx; on ERROR marks span + bumps      │
                │ error counter                                         │
                └──────────────────────────────┬────────────────────────┘
                                                │ OTLP (logs + traces + metrics)
                                                ▼
                              OpenTelemetry Collector ─► Tempo / Loki / Prometheus / …
```

Three signals, one correlation id: **the trace starts at nginx (the real edge)**
and **every log line carries `trace_id`**, so a log and its trace are one click
apart, and a trace shows the spans a log came from. Long-delayed queue work is
tied back to its originating request via a **span link** (not a multi-hour
parent-child span), while still sharing the `trace_id` for log correlation.

---

## The severity policy (the core of the ask)

This is a written contract enforced by code helpers and review. The rule of
thumb: **`ERROR` = unexpected + actionable + worth waking someone for. If it's
expected, handled, or the client's fault, it is not an ERROR.**

| Level | Meaning | Examples in this codebase | Alerting |
|-------|---------|---------------------------|----------|
| `ERROR` | Unexpected failure the system could not handle; a human should review. Rare. | DB unreachable; a queue job exhausted all retries and was marked `failed`; panic recovered in a worker; bug-class invariant violated | Page / ticket; alert on rate > 0 |
| `WARN` | Something went wrong but the system handled it (retry, fallback, skip). Expected under load/partial outage. | Transient qBittorrent error → `ErrDeferRetry`; SSE client buffer full (frame dropped); TMDB 429 backoff; fs-audit timed out (partial) | Dashboard; alert only on sustained rate |
| `INFO` | Business/lifecycle events worth seeing in normal operation. | Server start/stop; request completed (2xx/3xx); download finalized; media added/removed; queue worker started | None |
| `DEBUG` | Detailed flow for diagnosis; off in prod by default. | Per-file hardlink decisions; dequeue/poll detail; "deferred until torrent finishes" | None |

**Hard rules (codified, not just documented):**

1. **Client errors (4xx) are never `ERROR`.** A bad request, a 404, a validation
   failure → `INFO` (or `WARN` if suspicious). Only 5xx is server-side and may
   warrant `ERROR`. → fixes the request logger logging everything at `Info`, and
   the inverse (handlers logging expected 404s at error).
2. **Expected "absence" is not an error.** `gorm.ErrRecordNotFound`,
   `media.ErrMediaNotFound`, `users.ErrNotFound`, `ErrTorrentNotFound` when they
   are part of normal control flow → no `ERROR` log.
3. **Shutdown/cancellation is not an error.** `context.Canceled` /
   `context.DeadlineExceeded` during drain → `DEBUG`/`INFO`. (Directly relevant
   after Phase 2's graceful shutdown work.)
4. **Transient/retryable is `WARN`, terminal is `ERROR`.** A retryable job
   failure logs at `WARN` ("will retry"); only the final give-up (attempts
   exhausted, or `ErrPermanentFailure`) logs at `ERROR`. The queue worker already
   distinguishes these — the levels must follow.
5. **Log once, at the boundary.** Lower layers **return** wrapped errors; they do
   not log. The owning boundary (HTTP handler, queue worker, background loop)
   logs the error exactly once, with full context. No error is logged at every
   stack frame.
6. **Every `ERROR` is reviewable.** Each `ERROR` carries a stable `error.code`
   (component + short slug), increments a metric counter, and marks the active
   span's status `Error`. This is what makes "an error must be reviewed"
   enforceable (alert on the counter; group by code).

---

## Actor attribution — *who* did what (and what was the system)

Every span and every log line must answer "who caused this?" There are exactly
three actor kinds, and each request/job/loop resolves to one. This is a single
contract applied identically to **logs and spans** (same keys, so you filter a
trace and a log stream the same way).

### The actor model

| Actor | When | Identity source | Key attributes |
|-------|------|-----------------|----------------|
| **user** | A live authenticated request | nginx-injected `X-User-ID` / `X-Username` / `X-User-Role` (`internal/app/middleware.go`) | `enduser.id`, `app.username`, `enduser.role`, `actor.type=user` |
| **user (deferred)** | Background work *on behalf of* a user — queue jobs | the `user_id` / `username` already persisted in every job payload (`download`/`hardlink`/`remove`/`requests` `QueuePayload`) | same `enduser.*` as above, **plus** `actor.executor=queue.<name>` so it's clear a worker ran it, not a live request |
| **system** | Pure background loops with no user — scheduler, reconciler, download-completion watcher, browse warmer, torrent monitor, recovery, seed | none | `actor.type=system`, `actor.component=<scheduler\|reconciler\|watcher\|browse_warmer\|torrent_monitor\|queue_recovery>` |
| **anonymous** | Unauthenticated edge calls — `/auth/login`, `/auth/register`, and the nginx `/auth/validate` sub-request (`X-Internal-Auth-Check: 1`) | none yet | `actor.type=anonymous` (validate sub-request additionally tagged `actor.type=system`, `actor.component=nginx_auth`) |

### How it flows

- **Edge → ctx (Phase 1).** The proxy-auth middleware already reads
  `X-User-ID/Username/Role`; it will also resolve the **actor** and stash it in
  `ctx` (alongside `request_id`). The ctx `*slog.Logger` is pre-tagged with the
  actor fields, so *every* downstream log carries them with zero extra args.
- **ctx → span (Phase 3).** The trace middleware sets the actor attributes on the
  request span using OTel semantic conventions where they exist (`enduser.id`,
  `enduser.role`) plus `app.username` / `app.request_id`. Because child spans
  inherit the trace, the whole request tree is attributable.
- **Through the queue (Phase 3).** On enqueue we already persist `user_id` /
  `username` in the payload — the worker rebuilds the **same actor** from the
  payload and tags the job span + job logs with it, adding
  `actor.executor=queue.<name>`. So a hardlink that runs three hours later still
  reads as "user alice's download, executed by the hardlink worker," not an
  anonymous system event.
- **System loops** get a fixed system actor at startup (e.g. the scheduler's ctx
  is seeded `actor.type=system, actor.component=scheduler`), so their spans/logs
  are instantly distinguishable from user-driven work — answering "was this a
  person or the system?" at a glance.
- **`request_id` everywhere.** nginx's `X-Request-ID` becomes both a log field
  and the `app.request_id` span attribute, so you can pivot from an nginx access
  log line to the exact trace.

### PII note
`enduser.id` (a numeric id) is low-risk; `app.username` is mild PII. The plan
includes it because the explicit goal is "know who did what," but it routes
through the same redaction/scrubbing layer as secrets (Phase 4) and can be
demoted to id-only via config if a deployment needs to minimize PII in
telemetry.

---

## Phase 1 — Logging foundation: one `slog` handler + context plumbing

Goal: a single structured logger that every package uses, and a logger/trace
context that flows through `ctx`. No tracing backend yet — but logs already carry
correlation ids.

### Steps
1. **Rewrite `shared/logger`** around `log/slog`:
   - `Init(cfg)` builds the root `*slog.Logger` with a **custom `slog.Handler`**
     that wraps `slog.NewJSONHandler(os.Stdout, …)` and, in `Handle`, enriches
     each record from the `context.Context` with: `trace_id`, `span_id`,
     `request_id`, and the **actor fields** (`actor.type`, `enduser.id`,
     `app.username`, `enduser.role` or `actor.component`). Trace ids are no-ops
     until Phase 3; actor + request_id are populated from Phase 1.
   - `level` from `LOG_LEVEL` env (default `info`; `debug` in dev).
   - Keep a package default logger for the rare non-ctx call site.
2. **Context helpers** (`shared/logger`):
   - `logger.Into(ctx, attrs...) context.Context` — attaches a logger/fields.
   - `logger.From(ctx) *slog.Logger` — returns the ctx logger (or default).
   - Convenience: `logger.Error(ctx, msg, err, attrs...)` etc. that standardize
     the `error` field and `error.code` (see Phase 5).
3. **Seed the context in middleware** (`internal/app/middleware.go`): after
   resolving `request_id` and the **actor** (user from `X-User-*`, or anonymous),
   call `logger.Into(ctx, …)` and store it on the gin context + request context
   so handlers and everything downstream get a logger pre-tagged with actor +
   request id. Provide a `logger.SystemContext(component)` for the background
   loops (scheduler, watcher, reconciler, browse warmer, torrent monitor,
   recovery) so they seed `actor.type=system` at startup, and have the queue
   worker rebuild the **deferred-user actor** from the job payload's
   `user_id`/`username` (+ `actor.executor=queue.<name>`).
4. **Replace the request logger** (`internal/app/request_logger.go`): emit at a
   level derived from status — `INFO` for <500, `ERROR` for ≥500 (rule 1), and
   keep the redaction added in Phase 3 of the quality work.

### Exit criteria
- One JSON format everywhere; `LOG_LEVEL` works; request logs carry `request_id`
  and status-appropriate level. `go build ./... && go test ./...` green.

---

## Phase 2 — Migrate all call sites to `slog` (kill the zap/slog split)

Goal: delete `zap`, route the queue worker's `slog` through the same handler,
and apply the severity policy as a mechanical audit.

### Steps
1. **Codemod zap → slog** across the 22 files. Map:
   - `logger.Named("x").Info("m", zap.String("k", v))` →
     `logger.From(ctx).Info("m", "k", v)` (or the component logger where no ctx).
   - `zap.Error(err)` → `"error", err` (the handler renders it; Phase 5 adds the
     code).
   - Component identity (`Named`) becomes a stable `slog` attr `component=...` via
     `logger.With("component", "x")`.
2. **Worker logging** (`shared/processing-queue/worker.go`): it already uses
   `slog.*Context` — just ensure `slog.SetDefault` uses our handler so its output
   matches. Pass the job's ctx (which will carry the restored trace — Phase 3).
3. **Severity audit pass** — walk every `Error`/`Warn` call against the policy:
   - Downgrade expected-absence and client-error logs (rules 1–2).
   - Downgrade transient/retryable failures to `WARN`; keep only terminal at
     `ERROR` (rule 4) — especially in `download`/`hardlink`/`remove` processors
     and the watcher.
   - Remove duplicate logs where a lower layer logs an error it also returns
     (rule 5).
4. **Drop the `go.uber.org/zap` dependency** once no references remain.

### Exit criteria
- `grep -r zap` is empty; one handler; an `ERROR`-level grep reviewed line by line
  and each is a genuine, unexpected, actionable failure.

---

## Phase 3 — Tracing: OpenTelemetry SDK + instrument the hot paths

Goal: a span tree per request and per background job, with `trace_id` now
flowing into every log line (the Phase 1 handler starts populating it).

### Steps
0. **nginx is the trace origin (do this first).** Today nginx only forwards an
   `X-Request-ID` (`nginx/nginx.conf:64-66`) — there is no `traceparent` and no
   OTel, so without this step the trace would start at the Go backend, not the
   real edge. Two options:
   - **(Recommended) Instrument OpenResty with OTel** — the
     `opentelemetry-lua`/nginx OTel module starts a root `ingress` span per
     request and injects W3C `traceparent` on the proxied request. nginx timing
     (TLS, upstream wait) becomes the top of the trace.
   - **(Lighter) Emit `traceparent` in Lua** — generate a W3C `traceparent`
     (16-byte trace id, 8-byte span id) in `auth.lua`, derive it from / keep it
     alongside `X-Request-ID`, and set it as a proxy header. No nginx span, but
     one trace id from the edge through the backend and into logs.
   - **Auth sub-request:** nginx calls `backend:8080/api/v1/auth/validate` on
     every request (`nginx/lua/auth.lua`). Propagate the same `traceparent` to it
     so it is a child of the ingress span, **or** explicitly mark it
     `X-Internal-Auth-Check: 1` (already set) and have the backend trace
     middleware **skip span creation** for it — pick one so it isn't an orphan
     trace. (Recommended: keep it, as a cheap child — it surfaces auth-validate
     latency, which is on every request's critical path.)
1. **OTel SDK bootstrap** (`shared/telemetry` or `internal/app`): tracer provider
   with a `resource` (`service.name=media-bridge-backend`, version, env), batch
   span processor, a **W3C TraceContext propagator** (to read nginx's
   `traceparent`), and an exporter selected by env (stdout for dev; OTLP next
   phase). Shut it down in `Server.Shutdown` (slots into the Phase 2 shutdown
   ordering).
2. **HTTP server spans**: add `otelgin` as the outermost middleware so every
   request **continues** the nginx trace (extracts `traceparent`) rather than
   starting a fresh one; falls back to starting a root span if none is present
   (e.g. direct/dev calls). Set the **actor attributes** on the request span from
   the ctx resolved in Phase 1 (`enduser.id`, `app.username`, `enduser.role`,
   `actor.type`, `app.request_id`) — so the whole span tree is attributable to a
   person or marked `system`/`anonymous`. The queue worker (step 3) and the
   system loops do the same on their spans, including `actor.executor` for jobs.
3. **Propagate into background work** — the processing-queue is the key seam, and
   because jobs can run **minutes to hours later** (hardlink/remove timeouts are
   up to 3h), this is a producer/consumer hand-off, not a synchronous call:
   - On **enqueue**, inject the current `traceparent` into the job payload
     (`QueuePayload` is JSONB — add a `traceparent` field) within a short
     producer span that records "job enqueued".
   - On **dequeue**, the worker rebuilds the propagated span context and starts a
     **new span for the job with a span LINK** back to the originating request
     span — not a child of a long-since-finished HTTP root. The original
     `trace_id` is still carried in the worker's `ctx` so **all job logs
     correlate to the request** even though the span tree is separate. This gives
     a clean "request → (link) → job processing → qBit/DB" view without a
     3-hour-wide span.
4. **Outbound HTTP**: wrap the qBittorrent and TMDB/Prowlarr `http.Client`
   transports with `otelhttp` so external calls are child spans with status/
   latency (builds on the transport tuning from Phase 4 of the quality work).
5. **Database**: register the **gorm OTel plugin** so queries are spans (catches
   the N+1 and slow-query patterns the review flagged).
6. **Manual spans** at the meaningful seams: `pipeline` stages, hardlink walk,
   remove drain, fs-audit roots, the 5s download-completion watcher tick, SSE
   fan-out lifecycle.

### Exit criteria
- A trace **originates at nginx**, continues through the gin handler, and — via a
  span link — reaches the queue job that runs later, plus its qBittorrent/DB
  child spans. The auth-validate sub-request is either a child span or
  deliberately excluded. Every log emitted within any of these carries the
  matching `trace_id`.

---

## Phase 4 — Export over OTLP to a collector

Goal: ship logs + traces (+ metrics) to an OpenTelemetry Collector, configurable
per environment, with sane sampling.

### Steps
1. **OTLP exporters** for traces and logs (and metrics — Phase 5), endpoint from
   `OTEL_EXPORTER_OTLP_ENDPOINT`; gRPC by default. Logs go out via the OTel logs
   bridge (`otelslog`) **in addition to** stdout JSON, so you keep local
   greppable logs and get correlated logs in the backend.
2. **Collector** in `docker-compose.yml`: an `otel-collector` service with a
   pipeline that fans out to your chosen backends (e.g. Tempo for traces, Loki
   for logs, Prometheus for metrics). Document swapping backends here — **no app
   changes** required.
3. **Sampling**: parent-based + ratio sampler (e.g. 100% in dev, 5–10% in prod),
   with a rule to **always sample traces that contain an error span** (tail-based
   sampling at the collector) so failures are never dropped.
4. **Resource & env hygiene**: `service.name`, `service.version` (git SHA),
   `deployment.environment`; never export secrets (reuse the query redaction from
   the quality work; add span-attribute scrubbing).

### Exit criteria
- Traces and correlated logs visible in the backend; error traces always
  retained; backend swappable via collector config alone.

---

## Phase 5 — Make "an ERROR must be reviewed" real

Goal: turn the severity policy into an enforced, alertable signal — this is the
heart of the request.

### Steps
1. **`error.code` convention**: every `ERROR` (and notable `WARN`) carries a
   stable `error.code` like `download.qbit_unreachable`. Provide
   `logger.Error(ctx, code, msg, err, attrs...)` so the code is mandatory and
   greppable. Codes are the grouping key for alerts and dashboards.
2. **Error metric**: the handler increments a Prometheus counter
   `app_log_errors_total{component,code}` on every `ERROR`. **Alert: any sustained
   `ERROR` rate pages** — which is only meaningful because Phase 2 made `ERROR`
   trustworthy.
3. **Span correlation on error**: on an `ERROR` log within a span, set
   `span.SetStatus(Error)` and record the error as a span event — so an error log
   lights up its trace.
4. **Guardrails against regression**:
   - A tiny CI check (script or `golangci-lint` custom rule) that flags **new**
     `ERROR` call sites in a diff for explicit reviewer sign-off, and forbids
     `slog.Error` without an `error.code`.
   - Forbid `log.Fatal`/`os.Exit` outside `main` bootstrap (we already removed the
     goroutine `log.Fatal` in Phase 2 of the quality work — lint keeps it gone).
5. **Runbook**: each `error.code` gets a one-line entry (what it means, likely
   cause, where to look) so "review this error" has somewhere to land.

### Exit criteria
- An `ERROR` reliably means "review me," is counted, alertable, grouped by code,
  and linked to a trace. New `ERROR`s can't merge without review.

---

## Phase 6 — Dashboards, rollout, and docs

1. **Dashboards**: request rate/latency/error-rate by route (RED); queue depth,
   job latency, retry/defer/fail counts by pipeline; external-call latency
   (qBittorrent/TMDB); `app_log_errors_total` by code.
2. **Trace-driven SLOs**: p95 request latency, download→library end-to-end time.
3. **Rollout**: ship Phases 1–2 (logging) first and run for a week to shake out
   severity misclassifications before enabling sampling/alerting; turn on paging
   only after the `ERROR` stream is clean.
4. **Docs**: a short `docs/observability.md` for contributors — the severity
   table, how to add a span, how to add an `error.code`, and how to read a trace.

---

## Effort / sequencing

| Phase | Theme | Risk | Roughly |
|-------|-------|------|---------|
| 1 | slog foundation + ctx plumbing | Low | small |
| 2 | zap→slog migration + severity audit | Medium (wide, mechanical) | largest |
| 3 | OTel tracing — **nginx origin (3.0)** + backend spans + queue span-links | Medium | medium |
| 4 | OTLP export + collector | Low (mostly infra/config) | small–medium |
| 5 | ERROR discipline (codes, metric, CI, alerts) | Low | medium |
| 6 | dashboards + rollout | Low | small |

**Recommended first PR:** Phase 1 — it's self-contained, immediately makes logs
correlatable by `request_id`, and establishes the handler that every later phase
plugs into. Phases 1–2 deliver most of the "better logging / errors that are
really errors" value before any tracing infra exists.

## Decisions still open (can default during implementation)
- **Collector backends** (Tempo/Loki/Jaeger/SaaS) — deferred to collector config;
  no code impact.
- **Prod sampling ratio** and whether to run tail-sampling in the collector.
- **Logs transport**: stdout-only vs stdout + OTLP logs bridge (plan assumes
  both; can start stdout-only).
