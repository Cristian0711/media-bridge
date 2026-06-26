// Package telemetry configures OpenTelemetry tracing for the backend: a tracer
// provider, the W3C trace-context propagator (so a trace started at nginx is
// continued here), and an exporter selected by environment.
//
// Exporter selection (OTEL_TRACES_EXPORTER):
//   - "otlp"        → OTLP/gRPC to OTEL_EXPORTER_OTLP_ENDPOINT (the collector)
//   - "stdout"      → JSON spans to stdout (local dev)
//   - "none"/unset  → no exporter; spans are still created (so every log line
//     gets a real trace_id) but nothing is shipped off-box.
//
// Sampling (OTEL_TRACES_SAMPLER_ARG): parent-based ratio, default 1.0. A request
// arriving with a sampled parent (from nginx) is always honored.
package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init installs the global tracer provider and propagator. It returns a
// shutdown function (flushes and stops the provider) that should be called on
// process exit. Tracing is always active enough to stamp trace ids into logs;
// whether spans leave the process depends on the exporter env.
func Init(ctx context.Context, serviceName, version string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithFromEnv(), // OTEL_RESOURCE_ATTRIBUTES, e.g. deployment.environment
		resource.WithProcess(),
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.version", version),
		),
	)
	if err != nil {
		res = resource.Default()
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler()),
	}
	exp, err := exporter(ctx)
	if err != nil {
		return nil, fmt.Errorf("telemetry exporter: %w", err)
	}
	if exp != nil {
		opts = append(opts, sdktrace.WithBatcher(exp))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	// W3C TraceContext continues an inbound traceparent (from nginx); Baggage
	// carries cross-cutting key/values.
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))
	return tp.Shutdown, nil
}

func sampler() sdktrace.Sampler {
	ratio := 1.0
	if v := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_ARG")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 && f <= 1 {
			ratio = f
		}
	}
	// Honor the parent's decision (e.g. nginx already sampled this trace),
	// otherwise sample at the configured ratio.
	return sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
}

func exporter(ctx context.Context) (sdktrace.SpanExporter, error) {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("OTEL_TRACES_EXPORTER"))) {
	case "otlp":
		// Endpoint and TLS come from the standard OTEL_EXPORTER_OTLP_* env vars.
		return otlptracegrpc.New(ctx)
	case "stdout":
		return stdouttrace.New()
	default:
		// No exporter: spans are still created (valid trace ids for logs) but
		// not shipped.
		return nil, nil
	}
}
