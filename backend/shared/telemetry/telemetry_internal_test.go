package telemetry

import (
	"context"
	"testing"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// TestUnsampledParentDropsChildren protects the SSE-suppression mechanism: the
// app plants a valid-but-unsampled parent span context on streaming requests so
// the per-poll gorm query spans are dropped instead of becoming orphan traces.
// This works only because our sampler is parent-based and honors that decision.
func TestUnsampledParentDropsChildren(t *testing.T) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sampler()))
	defer func() { _ = tp.Shutdown(context.Background()) }()

	unsampledParent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{0x01},
		SpanID:  trace.SpanID{0x01},
		// TraceFlags 0 → not sampled.
	})
	ctx := trace.ContextWithSpanContext(context.Background(), unsampledParent)

	_, span := tp.Tracer("test").Start(ctx, "child")
	if span.IsRecording() {
		t.Fatal("child of an unsampled parent should be non-recording (dropped)")
	}

	// Sanity: a root span (no parent) is still sampled at the default ratio.
	_, root := tp.Tracer("test").Start(context.Background(), "root")
	if !root.IsRecording() {
		t.Fatal("root span should be recording at the default sample ratio")
	}
}
