package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// ActorType classifies who triggered a unit of work, so logs (and, from the
// tracing phase, spans) can answer "who did this — a user, or the system?".
type ActorType string

const (
	ActorUser      ActorType = "user"
	ActorSystem    ActorType = "system"
	ActorAnonymous ActorType = "anonymous"
)

// Actor identifies the originator of a request, queue job, or background loop.
// It is attached to the context and rendered into every log line by the handler.
type Actor struct {
	Type      ActorType
	UserID    uint
	Username  string
	Role      string
	Component string // system actors: which loop (scheduler, watcher, ...)
	Executor  string // deferred-user jobs: which queue ran it (queue.download, ...)
}

// UserActor is the originator of a live authenticated request.
func UserActor(id uint, username, role string) Actor {
	return Actor{Type: ActorUser, UserID: id, Username: username, Role: role}
}

// SystemActor is a background loop with no user (scheduler, watcher, ...).
func SystemActor(component string) Actor { return Actor{Type: ActorSystem, Component: component} }

// AnonymousActor is an unauthenticated caller (login/register, auth probe).
func AnonymousActor() Actor { return Actor{Type: ActorAnonymous} }

// WithExecutor tags a (deferred-)user actor with the queue that executed it, so
// a job run hours later still reads as "user X's work, run by the worker".
func (a Actor) WithExecutor(executor string) Actor {
	a.Executor = executor
	return a
}

func (a Actor) attrs() []slog.Attr {
	if a.Type == "" {
		return nil
	}
	out := []slog.Attr{slog.String("actor.type", string(a.Type))}
	if a.UserID != 0 {
		out = append(out, slog.Uint64("enduser.id", uint64(a.UserID)))
	}
	if a.Username != "" {
		out = append(out, slog.String("app.username", a.Username))
	}
	if a.Role != "" {
		out = append(out, slog.String("enduser.role", a.Role))
	}
	if a.Component != "" {
		out = append(out, slog.String("actor.component", a.Component))
	}
	if a.Executor != "" {
		out = append(out, slog.String("actor.executor", a.Executor))
	}
	return out
}

// --- context plumbing -------------------------------------------------------

type ctxKey int

const (
	ctxKeyRequestID ctxKey = iota
	ctxKeyActor
)

// WithRequestID stores the request id (nginx X-Request-ID) on the context.
func WithRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestID returns the request id carried on the context, or "".
func RequestID(ctx context.Context) string {
	id, _ := ctx.Value(ctxKeyRequestID).(string)
	return id
}

// WithActor stores the actor on the context.
func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, ctxKeyActor, a)
}

// WithSystem seeds a system actor for a background loop (scheduler, watcher, ...).
func WithSystem(ctx context.Context, component string) context.Context {
	return WithActor(ctx, SystemActor(component))
}

// ActorFrom returns the actor carried on the context.
func ActorFrom(ctx context.Context) (Actor, bool) {
	a, ok := ctx.Value(ctxKeyActor).(Actor)
	return a, ok
}

// --- handler ----------------------------------------------------------------

// contextHandler enriches every record with the request id and actor carried on
// the context, then delegates to inner. Trace/span ids will be added here in the
// tracing phase. Because enrichment reads the context, callers must use the
// *Context logging methods (InfoContext, …) for it to apply.
type contextHandler struct{ inner slog.Handler }

func (h contextHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.inner.Enabled(ctx, l)
}

func (h contextHandler) Handle(ctx context.Context, rec slog.Record) error {
	if id := RequestID(ctx); id != "" {
		rec.AddAttrs(slog.String("request_id", id))
	}
	if a, ok := ActorFrom(ctx); ok {
		rec.AddAttrs(a.attrs()...)
	}
	// Correlate logs with traces: stamp the active span's ids so a log line and
	// its trace are one lookup apart. Only when sampled — an unsampled context
	// (e.g. the synthetic parent we plant on untraced SSE streams) has no
	// exported trace, so emitting its id would point at nothing.
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() && sc.IsSampled() {
		rec.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	// Every ERROR is a reviewable signal: count it (by error.code) and mark its
	// span failed, so an error log lights up its trace and feeds alerting.
	if rec.Level >= slog.LevelError {
		recordError(ctx, rec)
	}
	return h.inner.Handle(ctx, rec)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{inner: h.inner.WithGroup(name)}
}

// --- init & accessors -------------------------------------------------------

// slogLogger is usable before Init (writes JSON to stderr); Init repoints it at
// stdout with the configured level and installs it as slog's default.
var slogLogger = slog.New(contextHandler{inner: slog.NewJSONHandler(os.Stderr, nil)})

// Init configures the process-wide structured logger: JSON to stdout at
// LOG_LEVEL (default info), enriched with context (request id, actor, and — in
// the tracing phase — trace ids). It also installs this as slog's default so
// packages already using slog (the processing-queue worker) share the format.
func Init() {
	h := contextHandler{inner: slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLevel(os.Getenv("LOG_LEVEL")),
	})}
	slogLogger = slog.New(h)
	slog.SetDefault(slogLogger)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// From returns the structured logger. Use its *Context methods (InfoContext, …)
// so the context enrichment (request id, actor, trace ids) is applied.
func From(_ context.Context) *slog.Logger { return slogLogger }

// Component returns the structured logger tagged with a component attribute.
func Component(name string) *slog.Logger { return slogLogger.With("component", name) }

// Info/Warn/Debug log at their level, attaching context enrichment. Always pass
// the request/job context so request id, actor (and later trace ids) attach.
func Info(ctx context.Context, msg string, args ...any)  { slogLogger.InfoContext(ctx, msg, args...) }
func Warn(ctx context.Context, msg string, args ...any)  { slogLogger.WarnContext(ctx, msg, args...) }
func Debug(ctx context.Context, msg string, args ...any) { slogLogger.DebugContext(ctx, msg, args...) }

// Error logs at error level. Per the severity policy this is reserved for
// unexpected, actionable failures, so it requires a stable error.code (e.g.
// "queue.dequeue_failed") — the grouping key for alerting and dashboards. The
// code and a standardized "error" attribute are attached automatically.
func Error(ctx context.Context, code, msg string, err error, args ...any) {
	all := make([]any, 0, len(args)+4)
	all = append(all, "code", code)
	if err != nil {
		all = append(all, "error", err.Error())
	}
	all = append(all, args...)
	slogLogger.ErrorContext(ctx, msg, all...)
}

// Err renders an error as a standardized "error" attribute (nil-safe), for use
// directly in slog argument lists at non-error levels:
// log.WarnContext(ctx, "msg", logger.Err(err)).
func Err(err error) slog.Attr {
	if err == nil {
		return slog.String("error", "")
	}
	return slog.String("error", err.Error())
}

var (
	errCounterOnce sync.Once
	errCounter     metric.Int64Counter
)

func errorCounter() metric.Int64Counter {
	errCounterOnce.Do(func() {
		errCounter, _ = otel.Meter("media-bridge/logger").Int64Counter(
			"app.log.errors",
			metric.WithDescription("Count of ERROR-level log records, labeled by error.code"),
		)
	})
	return errCounter
}

// recordError increments the error counter (keyed by error.code) and marks the
// active span as failed. Runs for every ERROR-level record, regardless of how it
// was logged.
func recordError(ctx context.Context, rec slog.Record) {
	code := "unspecified"
	rec.Attrs(func(a slog.Attr) bool {
		if a.Key == "code" {
			code = a.Value.String()
			return false
		}
		return true
	})
	if c := errorCounter(); c != nil {
		c.Add(ctx, 1, metric.WithAttributes(attribute.String("code", code)))
	}
	if span := trace.SpanFromContext(ctx); span.IsRecording() {
		span.SetStatus(codes.Error, rec.Message)
	}
}
