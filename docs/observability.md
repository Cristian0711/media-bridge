# Observability — operating & contributing guide

How logging and tracing work in the backend, the rules for using them, and how to
read what they produce. The design and rollout rationale live in
`docs/backend-observability-plan.md`; this is the day-to-day reference.

## What you get

- **One structured logger** (`log/slog`, JSON to stdout). Every line carries
  `request_id`, the **actor** (who), and — inside a span — `trace_id`/`span_id`.
- **Distributed traces** (OpenTelemetry). A trace starts at nginx, flows through
  the HTTP handler, DB and external calls, and links to the queue job that runs
  later. Exported via OTLP to the collector.
- **`app_log_errors_total{code}`** — every `ERROR` log increments this and marks
  its span failed, so errors are alertable and one click from their trace.

Path of a request:
```
client → Cloudflare → cloudflared → nginx (generates/continues traceparent)
       → backend (otelgin continues the trace) → services / gorm DB / TMDB·Prowlarr
       → enqueue (traceparent saved on the job) ⇢ (link) worker job span, hours later
every log line above shares the same trace_id
```

## Severity policy

**`ERROR` = unexpected + actionable + worth a human's attention. If it's expected,
handled, or the client's fault, it is not an ERROR.**

| Level | Use for | Examples |
|-------|---------|----------|
| `ERROR` | Unexpected failure the system couldn't handle; review it | DB unreachable; queue job exhausted retries; recovered panic; 5xx response |
| `WARN`  | Handled / expected-under-load; system recovered | transient qBittorrent error → defer-retry; SSE buffer full (frame dropped); TMDB failure the warmer retries |
| `INFO`  | Normal lifecycle / business events | server start/stop; request completed (<500); download finalized; media added/removed |
| `DEBUG` | Detailed flow for diagnosis (off in prod) | per-file hardlink decisions; queue poll detail; "deferred until torrent finishes"; external-call detail |

Hard rules: 4xx is never `ERROR` (→ `WARN`); expected "not found" is never `ERROR`;
`context.Canceled` during shutdown is never `ERROR`; transient/retryable is `WARN`
and only the terminal give-up is `ERROR`; **log an error once, at the boundary**
(lower layers return wrapped errors).

## Actor model — who did what

Every log/span resolves to one actor (`actor.type`):

| `actor.type` | When | Key fields |
|--------------|------|-----------|
| `user` | live authenticated request | `enduser.id`, `app.username`, `enduser.role` |
| `user` + `actor.executor` | queue job on behalf of a user | same, plus `actor.executor=queue.<name>` |
| `system` | background loop | `actor.component` (scheduler / reconciler / download_watcher / browse_warmer / torrent_monitor) |
| `anonymous` | login/register, auth probe | — |

## How to log (Go)

```go
// Always pass ctx — request_id, actor, and trace ids attach automatically.
logger.Info(ctx, "media added", "media_id", id)
logger.Warn(ctx, "tmdb degraded, using cache", logger.Err(err))
logger.Debug(ctx, "hardlink decision", "file", name, "linked", ok)

// ERROR requires a stable error.code (component.slug) — the alert/runbook key.
logger.Error(ctx, "download.qbit_unreachable", "add torrent failed", err, "media_id", id)
```

Rules enforced by `backend/scripts/check-logging.sh` (run in CI):
- ERROR logs must go through `logger.Error(...)` (so they carry an `error.code`).
  Direct `*.ErrorContext(` outside the logger package fails the check.
- `os.Exit` / `Fatal` only in `cmd/api/main.go`.

When you add a new `ERROR`, **add its code to the runbook below** in the same PR.

## What isn't traced (on purpose)

SSE/streaming endpoints (paths ending in `/events`) and the internal nginx
auth-validate probe are **excluded from tracing** (`skipTracing` in
`internal/app/middleware.go`). A long-lived SSE connection would otherwise be one
span open for the whole stream — showing in Tempo as "root span not yet received"
— accreting a DB-poll span every tick. Those requests still log (with
`request_id` + actor) so they remain correlatable; they just don't create spans.
Their poll queries are suppressed via an unsampled parent context so they don't
become orphan spans either.

## How to trace (Go)

A span already exists for every HTTP request and queue job. To add a child span
around a meaningful operation:

```go
ctx, span := otel.Tracer("media-bridge/<pkg>").Start(ctx, "operation.name")
defer span.End()
// ... use ctx for downstream calls (DB/HTTP) so they nest ...
```
Pass `ctx` onward and the gorm/otelhttp instrumentation nests automatically. Don't
manually set span status to Error — `logger.Error` does it for you.

## The stack (docker-compose)

`docker compose up -d` brings up the full observability stack alongside the app:

| Service | Port | Role |
|---------|------|------|
| otel-collector | 4317/4318, 8889 | receives OTLP from the backend; tail-samples traces → Tempo; spanmetrics + `app_log_errors_total` → Prometheus (`:8889`) |
| tempo | 3200 | trace storage |
| loki | 3100 | log storage |
| promtail | — | scrapes container stdout (JSON logs) → Loki |
| prometheus | 9090 | scrapes the collector; evaluates `otel/prometheus-alerts.yml` |
| grafana | **3000** | UI — datasources auto-provisioned (admin/admin by default; override `GRAFANA_ADMIN_PASSWORD`) |

Open **Grafana at http://localhost:3000** → Explore:
- **Traces:** pick the *Tempo* datasource (search by service / trace id / duration).
- **Logs:** pick the *Loki* datasource (e.g. `{level="ERROR"}`, `{container="media-bridge-api"}`).
- Provisioned dashboards: **"Media Bridge — Observability"** (errors-by-code,
  request error %, recent ERROR logs), **"Media Bridge — Traces (RED + recent)"**
  (request rate/latency/error% + queue throughput from spanmetrics, plus recent
  & error trace tables from Tempo), and **"Logging via Loki"** (the community
  per-service log explorer, grafana.com #18042).

Promtail emits the labels the Loki dashboard expects: `container_name` (docker
container), `service_name` (compose service), `instance` (node), plus `level` /
`component` from the JSON. Adjust `instance` in `otel/promtail-config.yaml` if you
run multiple nodes.

## Reading it (with correlation wired in Grafana)

- **Log → trace:** in a Loki log line, the `trace_id` is a clickable **derived
  field** that opens the trace in Tempo.
- **Trace → logs:** in a Tempo trace, "Logs for this span" jumps to Loki filtered
  by that trace id (tracesToLogsV2).
- **Queue work:** a job span is a separate trace **linked** to the request; the
  link (and the `enqueue.trace_id` attribute) connects them.
- **Who:** filter by `actor.type` / `enduser.id` / `actor.component`; HTTP traces
  also carry `cloudflare.ray` and `client.address`.

## error.code runbook

| code | level | meaning | likely cause | where to look |
|------|-------|---------|--------------|---------------|
| `http.server_error` | ERROR | a request returned 5xx | a handler/service returned an unexpected error | the request's `trace_id`; the deeper error log on the same trace |
| `queue.dequeue_failed` | ERROR | worker couldn't claim a job | Postgres unreachable / queue table issue | DB health; collector/db spans |
| `queue.fail_record_failed` | ERROR | couldn't persist a job's failure | DB blip during the (detached) terminal write | DB; the job row's status |
| `queue.complete_failed` | ERROR | couldn't mark a job completed | DB blip; row cancelled mid-flight | DB; whether a remove cancelled the job |
| `queue.recovery_failed` | ERROR | stale-job recovery sweep failed | DB unreachable | DB health (recurs each RecoveryInterval) |
| `queue.handler_panic` | ERROR | a job handler panicked (recovered) | a bug in the handler | the `stack`/`panic` fields on the log; the job span |
| `app.bootstrap_failed` | ERROR | startup failed | bad config / DB / qBittorrent login | the wrapped error; env/config |
| `app.server_failed` | ERROR | HTTP server stopped unexpectedly | port in use; listener error | the wrapped error |
| `app.shutdown_failed` | ERROR | graceful shutdown errored | in-flight drain exceeded deadline | shutdown logs; what was still running |
| `app.telemetry_init_failed` | ERROR | tracing/metrics init failed | bad `OTEL_*` config / unreachable collector at init | `OTEL_*` env; collector reachability |
| `app.telemetry_shutdown_failed` | ERROR | span/metric flush on exit failed | collector unreachable at shutdown | collector |

## Dashboards (PromQL)

Scrape the collector (`otel-collector:8889`). Span metric names may be `calls_total`
/ `duration_milliseconds_*` or `traces_span_metrics_*` depending on collector
version — check `/metrics`.

- **Errors by code:** `sum by (code) (rate(app_log_errors_total[5m]))`
- **Request rate (RED):** `sum by (http_route) (rate(calls_total[1m]))`
- **Request error %:** `sum(rate(calls_total{status_code="STATUS_CODE_ERROR"}[5m])) / sum(rate(calls_total[5m]))`
- **Request p95 latency:** `histogram_quantile(0.95, sum by (le) (rate(duration_milliseconds_bucket[5m])))`
- **Queue job rate/latency:** same `calls_total`/`duration` filtered to
  `span_name=~"queue.process.*"`.

Alert rules: `otel/prometheus-alerts.yml`.

## Environment reference

| Var | Default | Meaning |
|-----|---------|---------|
| `LOG_LEVEL` | `info` | `debug`/`info`/`warn`/`error` |
| `OTEL_TRACES_EXPORTER` | (compose) `otlp` | `otlp` / `stdout` / `none` |
| `OTEL_METRICS_EXPORTER` | (compose) `otlp` | `otlp` / `stdout` / `none` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `http://otel-collector:4317` | collector OTLP/gRPC endpoint |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` (compose) | plaintext to the in-cluster collector |
| `OTEL_TRACES_SAMPLER_ARG` | `1.0` | head sample ratio; collector tail-samples (keeps all errors) |
| `OTEL_RESOURCE_ATTRIBUTES` | (compose sets env/namespace) | extra resource attrs, e.g. `deployment.environment=production` |
| `APP_VERSION` | `dev` | stamped as `service.version` |

Disable telemetry entirely with `OTEL_TRACES_EXPORTER=none` / `OTEL_METRICS_EXPORTER=none`
(spans still get ids for log correlation; nothing is shipped).

## Rollout

1. Ship logging (Phases 1–2) and run a week — shake out any severity
   misclassifications while `ERROR` is observed but not yet paging.
2. Enable tracing export (Phase 4 env) and confirm traces + correlated logs.
3. Turn on the `app_log_errors_total` page only once the `ERROR` stream is clean
   — that's the contract that makes the alert trustworthy.
