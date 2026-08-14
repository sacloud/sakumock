package core

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sacloud/sakumock"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.34.0"
	"go.opentelemetry.io/otel/trace"
)

const tracingShutdownTimeout = 5 * time.Second

// TracingEnabled reports whether OpenTelemetry tracing is turned on for this
// process. Tracing is configured exclusively through the standard OTEL
// environment variables — there are no sakumock flags: it is enabled when
// OTEL_EXPORTER_OTLP_TRACES_ENDPOINT or OTEL_EXPORTER_OTLP_ENDPOINT is set,
// unless OTEL_SDK_DISABLED=true.
func TracingEnabled() bool {
	if strings.EqualFold(os.Getenv("OTEL_SDK_DISABLED"), "true") {
		return false
	}
	return os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != "" ||
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != ""
}

// SetupTracing installs the global TracerProvider (OTLP exporter) and
// the W3C tracecontext+baggage propagator when TracingEnabled(); otherwise it
// is a no-op. The returned shutdown flushes remaining spans with an internal
// timeout and logs failures, so callers just `defer shutdown()`.
func SetupTracing(ctx context.Context) (shutdown func(), err error) {
	if !TracingEnabled() {
		return func() {}, nil
	}

	// http/protobuf unless OTEL_EXPORTER_OTLP_(TRACES_)PROTOCOL selects grpc
	// (the same default the Go contrib autoexport uses).
	proto := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
	if proto == "" {
		proto = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}
	var exporter sdktrace.SpanExporter
	if proto == "grpc" {
		exporter, err = otlptracegrpc.New(ctx)
	} else {
		exporter, err = otlptracehttp.New(ctx)
	}
	if err != nil {
		return nil, err
	}

	attrs := []attribute.KeyValue{semconv.ServiceVersion(sakumock.Version)}
	if os.Getenv("OTEL_SERVICE_NAME") == "" {
		attrs = append(attrs, semconv.ServiceName("sakumock"))
	}
	// NewSchemaless avoids the schema-URL conflict Merge reports when this
	// package's semconv version drifts from the SDK's default resource.
	res, err := sdkresource.Merge(sdkresource.Default(), sdkresource.NewSchemaless(attrs...))
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	slog.Info("opentelemetry tracing enabled", "endpoint", endpoint)

	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), tracingShutdownTimeout)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			slog.Warn("opentelemetry tracer shutdown failed", "error", err)
		}
	}, nil
}

// TraceHandler wraps h with otelhttp server-span instrumentation when
// TracingEnabled(); otherwise it returns h unchanged. The service name (the
// same short name used for subcommands) becomes the sakumock.service span
// attribute — every listener in a process shares one service.name resource,
// so this attribute is what tells the services apart.
func TraceHandler(service string, h http.Handler) http.Handler {
	if !TracingEnabled() {
		return h
	}
	// otelhttp derives the span *name* from the ServeMux pattern after h
	// returns, but the http.route attribute only at span start — when the mux
	// hasn't matched yet. Set it here once the pattern is known.
	routed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r)
		if idx := strings.IndexByte(r.Pattern, '/'); idx >= 0 {
			trace.SpanFromContext(r.Context()).SetAttributes(semconv.HTTPRoute(r.Pattern[idx:]))
		}
	})
	return otelhttp.NewHandler(routed, "",
		otelhttp.WithSpanOptions(trace.WithAttributes(attribute.String("sakumock.service", service))),
	)
}
