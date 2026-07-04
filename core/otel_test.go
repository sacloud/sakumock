package core

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"go.opentelemetry.io/otel"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/proto"
)

// clearOTelEnv blanks every env var TracingEnabled/SetupTracing reads, so a
// developer's environment doesn't leak into the test.
func clearOTelEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_PROTOCOL",
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL",
		"OTEL_SDK_DISABLED",
		"OTEL_SERVICE_NAME",
	} {
		t.Setenv(k, "")
	}
}

func TestTracingEnabled(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want bool
	}{
		{"no env", nil, false},
		{"endpoint", map[string]string{"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318"}, true},
		{"traces endpoint", map[string]string{"OTEL_EXPORTER_OTLP_TRACES_ENDPOINT": "http://localhost:4318/v1/traces"}, true},
		{"sdk disabled", map[string]string{
			"OTEL_EXPORTER_OTLP_ENDPOINT": "http://localhost:4318",
			"OTEL_SDK_DISABLED":           "true",
		}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearOTelEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if got := TracingEnabled(); got != tt.want {
				t.Errorf("TracingEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTraceHandlerDisabled(t *testing.T) {
	clearOTelEnv(t)
	mux := http.NewServeMux()
	if got := TraceHandler("kms", mux); got != http.Handler(mux) {
		t.Errorf("TraceHandler with tracing disabled = %T, want the handler unchanged", got)
	}
}

func TestSetupTracingDisabled(t *testing.T) {
	clearOTelEnv(t)
	shutdown, err := SetupTracing(context.Background())
	if err != nil {
		t.Fatalf("SetupTracing: %v", err)
	}
	shutdown() // no-op
}

// TestSetupTracingExport drives the whole pipeline: traceparent extraction,
// route-pattern span naming, the sakumock.service attribute, OTLP/HTTP
// export, and the trace_id/span_id correlation in RequestLogArgs.
func TestSetupTracingExport(t *testing.T) {
	var (
		mu   sync.Mutex
		body []byte
	)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" {
			t.Errorf("unexpected export path %s", r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		body = b
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer collector.Close()

	clearOTelEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", collector.URL)

	prevTP := otel.GetTracerProvider()
	prevProp := otel.GetTextMapPropagator()
	t.Cleanup(func() {
		otel.SetTracerProvider(prevTP)
		otel.SetTextMapPropagator(prevProp)
	})

	shutdown, err := SetupTracing(context.Background())
	if err != nil {
		t.Fatalf("SetupTracing: %v", err)
	}

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuf, nil))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /things/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	h := TraceHandler("kms", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := NewResponseRecorder(w)
		mux.ServeHTTP(rw, r)
		logger.Info("request", RequestLogArgs(r, rw)...)
	}))

	const (
		wantTraceID      = "0af7651916cd43dd8448eb211c80319c"
		wantParentSpanID = "b7ad6b7169203331"
	)
	req := httptest.NewRequest("GET", "/things/42", nil)
	req.Header.Set("traceparent", "00-"+wantTraceID+"-"+wantParentSpanID+"-01")
	h.ServeHTTP(httptest.NewRecorder(), req)

	shutdown() // flushes the span to the collector

	mu.Lock()
	exported := body
	mu.Unlock()
	if len(exported) == 0 {
		t.Fatal("no spans exported")
	}
	// The request body is an ExportTraceServiceRequest, wire-compatible with
	// TracesData (both carry the resource list at field 1) — the same trick
	// monitoringsuite's ingest uses to avoid the collector service packages.
	var td tracepb.TracesData
	if err := proto.Unmarshal(exported, &td); err != nil {
		t.Fatalf("decode exported traces: %v", err)
	}
	if len(td.ResourceSpans) != 1 {
		t.Fatalf("ResourceSpans = %d, want 1", len(td.ResourceSpans))
	}
	rs := td.ResourceSpans[0]

	resAttrs := map[string]string{}
	for _, kv := range rs.Resource.GetAttributes() {
		resAttrs[kv.Key] = kv.Value.GetStringValue()
	}
	if got := resAttrs["service.name"]; got != "sakumock" {
		t.Errorf("service.name = %q, want %q", got, "sakumock")
	}
	if got := resAttrs["service.version"]; got == "" {
		t.Error("service.version resource attribute missing")
	}

	if len(rs.ScopeSpans) != 1 || len(rs.ScopeSpans[0].Spans) != 1 {
		t.Fatalf("expected exactly one span, got %+v", rs.ScopeSpans)
	}
	span := rs.ScopeSpans[0].Spans[0]
	if got := hex.EncodeToString(span.TraceId); got != wantTraceID {
		t.Errorf("trace ID = %s, want %s (inbound traceparent not honored)", got, wantTraceID)
	}
	if got := hex.EncodeToString(span.ParentSpanId); got != wantParentSpanID {
		t.Errorf("parent span ID = %s, want %s", got, wantParentSpanID)
	}
	if span.Name != "GET /things/{id}" {
		t.Errorf("span name = %q, want %q", span.Name, "GET /things/{id}")
	}
	spanAttrs := map[string]string{}
	for _, kv := range span.Attributes {
		spanAttrs[kv.Key] = kv.Value.GetStringValue()
	}
	if got := spanAttrs["sakumock.service"]; got != "kms" {
		t.Errorf("sakumock.service = %q, want %q", got, "kms")
	}
	if got := spanAttrs["http.route"]; got != "/things/{id}" {
		t.Errorf("http.route = %q, want %q (all attrs: %v)", got, "/things/{id}", span.Attributes)
	}

	var logLine struct {
		TraceID string `json:"trace_id"`
		SpanID  string `json:"span_id"`
	}
	if err := json.Unmarshal(logBuf.Bytes(), &logLine); err != nil {
		t.Fatalf("parse request log %q: %v", logBuf.String(), err)
	}
	if logLine.TraceID != wantTraceID {
		t.Errorf("log trace_id = %q, want %q", logLine.TraceID, wantTraceID)
	}
	if got := hex.EncodeToString(span.SpanId); logLine.SpanID != got {
		t.Errorf("log span_id = %q, want exported span ID %q", logLine.SpanID, got)
	}
}
