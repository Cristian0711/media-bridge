package processingqueue

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// TestStartJobSpanContinuesTrace verifies the job span continues the trace that
// enqueued it (same trace id), so the whole pipeline shares one trace
// end-to-end — rather than starting a fresh root.
func TestStartJobSpanContinuesTrace(t *testing.T) {
	otel.SetTracerProvider(sdktrace.NewTracerProvider()) // default sampler = AlwaysSample
	otel.SetTextMapPropagator(propagation.TraceContext{})

	const producerTraceID = "0123456789abcdef0123456789abcdef"
	q := &Queue[struct{}]{name: "test", opts: defaultOptions()}
	job := &Job[struct{}]{
		ID:          uuid.New(),
		Traceparent: "00-" + producerTraceID + "-0123456789abcdef-01",
	}

	_, span := q.startJobSpan(context.Background(), job)
	defer span.End()

	if got := span.SpanContext().TraceID().String(); got != producerTraceID {
		t.Fatalf("job span trace id = %s, want %s (continuation of producer trace)", got, producerTraceID)
	}
}
