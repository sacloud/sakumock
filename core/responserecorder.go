package core

import (
	"bytes"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/trace"
)

// maxErrorBodyLog bounds how much of an error response body ResponseRecorder
// retains for logging.
const maxErrorBodyLog = 2048

// ResponseRecorder wraps http.ResponseWriter recording the status code and,
// for error responses (status >= 400), a bounded prefix of the body — so a
// mock's request log can say why a request was rejected, not just that it
// was. The reason otherwise only reaches the client.
type ResponseRecorder struct {
	http.ResponseWriter
	Status  int
	errBody bytes.Buffer
}

func NewResponseRecorder(w http.ResponseWriter) *ResponseRecorder {
	return &ResponseRecorder{ResponseWriter: w, Status: http.StatusOK}
}

func (r *ResponseRecorder) WriteHeader(code int) {
	r.Status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *ResponseRecorder) Write(b []byte) (int, error) {
	if r.Status >= 400 && r.errBody.Len() < maxErrorBodyLog {
		r.errBody.Write(b[:min(len(b), maxErrorBodyLog-r.errBody.Len())])
	}
	return r.ResponseWriter.Write(b)
}

// ErrorBody returns the captured error response body (whitespace-trimmed),
// or "" for non-error responses.
func (r *ResponseRecorder) ErrorBody() string {
	return strings.TrimSpace(r.errBody.String())
}

// Unwrap lets http.ResponseController reach the underlying writer's optional
// interfaces (Flusher, Hijacker, ...).
func (r *ResponseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// MarkDropped records a synthetic status and reason for a request whose
// connection was deliberately dropped without a response (fault injection),
// so the per-request log can report it. Nothing is written to the client;
// 499 (the nginx "client got no response" convention) is the expected status.
func (r *ResponseRecorder) MarkDropped(status int, reason string) {
	r.Status = status
	r.errBody.Reset()
	r.errBody.WriteString(reason)
}

// Flush forwards to the underlying writer so streaming responses (e.g. a
// reverse-proxy data plane) keep flushing through the wrapper.
func (r *ResponseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// RequestLogArgs builds the standard per-request log attributes: method,
// path, status, trace/span IDs when the request carries a span (see
// SetupTracing), the error reason when the response is an error, and any
// service-specific extras.
func RequestLogArgs(r *http.Request, rec *ResponseRecorder, extra ...any) []any {
	args := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"status", rec.Status,
	}
	if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
		args = append(args, "trace_id", sc.TraceID().String(), "span_id", sc.SpanID().String())
	}
	if msg := rec.ErrorBody(); msg != "" {
		args = append(args, "error", msg)
	}
	return append(args, extra...)
}
