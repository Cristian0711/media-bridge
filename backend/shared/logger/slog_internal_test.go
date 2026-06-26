package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func newCaptureLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(contextHandler{inner: slog.NewJSONHandler(buf, nil)})
}

func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal log line %q: %v", buf.String(), err)
	}
	return m
}

func TestContextHandlerEnrichesUserActor(t *testing.T) {
	var buf bytes.Buffer
	log := newCaptureLogger(&buf)

	ctx := WithRequestID(context.Background(), "req-123")
	ctx = WithActor(ctx, UserActor(7, "alice", "admin"))

	log.InfoContext(ctx, "hello")

	m := decode(t, &buf)
	if m["request_id"] != "req-123" {
		t.Errorf("request_id = %v, want req-123", m["request_id"])
	}
	if m["actor.type"] != "user" {
		t.Errorf("actor.type = %v, want user", m["actor.type"])
	}
	if m["enduser.id"] != float64(7) {
		t.Errorf("enduser.id = %v, want 7", m["enduser.id"])
	}
	if m["app.username"] != "alice" {
		t.Errorf("app.username = %v, want alice", m["app.username"])
	}
	if m["enduser.role"] != "admin" {
		t.Errorf("enduser.role = %v, want admin", m["enduser.role"])
	}
}

func TestContextHandlerSystemActor(t *testing.T) {
	var buf bytes.Buffer
	log := newCaptureLogger(&buf)

	ctx := WithSystem(context.Background(), "scheduler")
	log.InfoContext(ctx, "tick")

	m := decode(t, &buf)
	if m["actor.type"] != "system" {
		t.Errorf("actor.type = %v, want system", m["actor.type"])
	}
	if m["actor.component"] != "scheduler" {
		t.Errorf("actor.component = %v, want scheduler", m["actor.component"])
	}
	if _, ok := m["enduser.id"]; ok {
		t.Error("system actor must not carry enduser.id")
	}
}

func TestDeferredUserExecutor(t *testing.T) {
	var buf bytes.Buffer
	log := newCaptureLogger(&buf)

	ctx := WithActor(context.Background(), UserActor(9, "bob", "user").WithExecutor("queue.download"))
	log.InfoContext(ctx, "job")

	m := decode(t, &buf)
	if m["actor.type"] != "user" || m["actor.executor"] != "queue.download" {
		t.Errorf("expected deferred user actor with executor, got type=%v executor=%v", m["actor.type"], m["actor.executor"])
	}
}

func TestNoActorNoEnrichment(t *testing.T) {
	var buf bytes.Buffer
	log := newCaptureLogger(&buf)

	log.InfoContext(context.Background(), "bare")

	m := decode(t, &buf)
	if _, ok := m["actor.type"]; ok {
		t.Error("did not expect actor.type without an actor in context")
	}
	if _, ok := m["request_id"]; ok {
		t.Error("did not expect request_id without one in context")
	}
}

func TestErrorStampsCode(t *testing.T) {
	var buf bytes.Buffer
	prev := slogLogger
	slogLogger = newCaptureLogger(&buf)
	defer func() { slogLogger = prev }()

	Error(context.Background(), "queue.dequeue_failed", "boom", errSentinel{}, "queue", "downloads")

	m := decode(t, &buf)
	if m["code"] != "queue.dequeue_failed" {
		t.Errorf("code = %v, want queue.dequeue_failed", m["code"])
	}
	if m["error"] != "sentinel" {
		t.Errorf("error = %v, want sentinel", m["error"])
	}
	if m["queue"] != "downloads" {
		t.Errorf("queue = %v, want downloads", m["queue"])
	}
	if m["level"] != "ERROR" {
		t.Errorf("level = %v, want ERROR", m["level"])
	}
}

type errSentinel struct{}

func (errSentinel) Error() string { return "sentinel" }

func TestContextHandlerStampsTraceIDs(t *testing.T) {
	var buf bytes.Buffer
	log := newCaptureLogger(&buf)

	traceID, _ := trace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	spanID, _ := trace.SpanIDFromHex("0123456789abcdef")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
	})
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	log.InfoContext(ctx, "traced")

	m := decode(t, &buf)
	if m["trace_id"] != "0123456789abcdef0123456789abcdef" {
		t.Errorf("trace_id = %v, want the span's trace id", m["trace_id"])
	}
	if m["span_id"] != "0123456789abcdef" {
		t.Errorf("span_id = %v, want the span's span id", m["span_id"])
	}
}

func TestParseLevel(t *testing.T) {
	cases := map[string]slog.Level{
		"":        slog.LevelInfo,
		"info":    slog.LevelInfo,
		"DEBUG":   slog.LevelDebug,
		" warn ":  slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"bogus":   slog.LevelInfo,
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}
