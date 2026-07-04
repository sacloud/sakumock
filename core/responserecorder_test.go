package core

import (
	"net/http/httptest"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestResponseRecorderCapturesErrorBody(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w)
	rec.WriteHeader(400)
	rec.Write([]byte(`{"message":"bad request"}` + "\n"))

	if rec.Status != 400 {
		t.Errorf("Status = %d, want 400", rec.Status)
	}
	if got := rec.ErrorBody(); got != `{"message":"bad request"}` {
		t.Errorf("ErrorBody = %q", got)
	}
	if w.Body.String() != `{"message":"bad request"}`+"\n" {
		t.Errorf("underlying body = %q", w.Body.String())
	}
}

func TestResponseRecorderIgnoresSuccessBody(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.Write([]byte(`{"ok":true}`)) // implicit 200

	if rec.Status != 200 {
		t.Errorf("Status = %d, want 200", rec.Status)
	}
	if got := rec.ErrorBody(); got != "" {
		t.Errorf("ErrorBody = %q, want empty", got)
	}
}

func TestResponseRecorderBoundsErrorBody(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.WriteHeader(500)
	rec.Write([]byte(strings.Repeat("x", 10*maxErrorBodyLog)))

	if got := len(rec.ErrorBody()); got != maxErrorBodyLog {
		t.Errorf("captured %d bytes, want %d", got, maxErrorBodyLog)
	}
}

func TestRequestLogArgs(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.WriteHeader(404)
	rec.Write([]byte(`{"message":"not found"}`))
	r := httptest.NewRequest("GET", "/things/1", nil)

	args := RequestLogArgs(r, rec, "extra", "value")
	want := []any{"method", "GET", "path", "/things/1", "status", 404, "error", `{"message":"not found"}`, "extra", "value"}
	if len(args) != len(want) {
		t.Fatalf("args = %v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %v, want %v", i, args[i], want[i])
		}
	}
}

func TestRequestLogArgsTraceIDs(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.WriteHeader(200)

	traceID := trace.TraceID{0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0x10}
	spanID := trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8}
	sc := trace.NewSpanContext(trace.SpanContextConfig{TraceID: traceID, SpanID: spanID})
	r := httptest.NewRequest("GET", "/things/1", nil)
	r = r.WithContext(trace.ContextWithSpanContext(r.Context(), sc))

	args := RequestLogArgs(r, rec)
	want := []any{"method", "GET", "path", "/things/1", "status", 200,
		"trace_id", traceID.String(), "span_id", spanID.String()}
	if len(args) != len(want) {
		t.Fatalf("args = %v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %v, want %v", i, args[i], want[i])
		}
	}
}
