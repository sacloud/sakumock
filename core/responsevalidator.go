package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

// SpecViolation is one way a handler's response diverged from the OpenAPI
// spec. Identical violations (same route, status, and message) are collapsed
// into a single entry with a Count, so a long-running mock reports each
// distinct divergence once.
type SpecViolation struct {
	Method  string `json:"method"`
	Path    string `json:"path"`
	Status  int    `json:"status"`
	Message string `json:"message"`
	Count   int    `json:"count"`
}

// ResponseValidator checks handler responses against per-route, per-status
// schemas (typically the generated responseSchemas table of a service).
// Violations never alter the response the client receives: they are logged at
// Warn level and recorded for inspection via Violations(), so spec drift is
// observable from the mock itself without breaking clients. A nil
// *ResponseValidator is valid and disables validation.
type ResponseValidator struct {
	schemas map[string]map[int]*BodySchema
	logger  *slog.Logger

	mu         sync.Mutex
	violations []*SpecViolation
	seen       map[string]*SpecViolation
}

// NewResponseValidator creates a validator over schemas keyed by
// "METHOD /path" (the mux pattern registered in the route table), each
// mapping declared status codes to their response-body schema.
func NewResponseValidator(schemas map[string]map[int]*BodySchema, logger *slog.Logger) *ResponseValidator {
	if logger == nil {
		logger = slog.Default()
	}
	return &ResponseValidator{
		schemas: schemas,
		logger:  logger,
		seen:    map[string]*SpecViolation{},
	}
}

// Violations returns a snapshot of the recorded violations in first-seen
// order.
func (rv *ResponseValidator) Violations() []SpecViolation {
	if rv == nil {
		return nil
	}
	rv.mu.Lock()
	defer rv.mu.Unlock()
	out := make([]SpecViolation, len(rv.violations))
	for i, v := range rv.violations {
		out[i] = *v
	}
	return out
}

// Reset clears the recorded violations.
func (rv *ResponseValidator) Reset() {
	if rv == nil {
		return
	}
	rv.mu.Lock()
	defer rv.mu.Unlock()
	rv.violations = nil
	rv.seen = map[string]*SpecViolation{}
}

func (rv *ResponseValidator) record(method, path string, status int, message string) {
	rv.mu.Lock()
	key := fmt.Sprintf("%s %s %d %s", method, path, status, message)
	v, ok := rv.seen[key]
	if !ok {
		v = &SpecViolation{Method: method, Path: path, Status: status, Message: message}
		rv.seen[key] = v
		rv.violations = append(rv.violations, v)
	}
	v.Count++
	rv.mu.Unlock()
	rv.logger.Warn("response does not conform to the OpenAPI spec",
		"method", method, "path", path, "status", status, "error", message)
}

// specViolationList is the response envelope of GET /_sakumock/spec-violations.
type specViolationList struct {
	Violations []SpecViolation `json:"violations"`
}

// ListHandler serves the recorded violations as {"violations": [...]}.
func (rv *ResponseValidator) ListHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		violations := rv.Violations()
		if violations == nil {
			violations = []SpecViolation{}
		}
		WriteJSON(w, http.StatusOK, specViolationList{Violations: violations})
	}
}

// ClearHandler clears the recorded violations and responds 204.
func (rv *ResponseValidator) ClearHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		rv.Reset()
		w.WriteHeader(http.StatusNoContent)
	}
}

// SpecViolationRoutes returns the standard mock-only inspection routes
// exposing rv — GET/DELETE /_sakumock/spec-violations — for a service's
// routeTable() to append as raw entries (outside the fault / rate-limit /
// validation closures, per the /_sakumock/ convention).
func SpecViolationRoutes(rv *ResponseValidator) []RegisteredRoute {
	return []RegisteredRoute{
		{Route: Route{Method: "GET", Path: "/_sakumock/spec-violations", Description: "List recorded OpenAPI spec violations", Kind: "inspection"}, Handler: rv.ListHandler()},
		{Route: Route{Method: "DELETE", Path: "/_sakumock/spec-violations", Description: "Clear recorded OpenAPI spec violations", Kind: "inspection"}, Handler: rv.ClearHandler()},
	}
}

// responseCapture records the status and body a handler writes while passing
// everything through to the underlying writer.
type responseCapture struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (c *responseCapture) WriteHeader(code int) {
	c.status = code
	c.ResponseWriter.WriteHeader(code)
}

func (c *responseCapture) Write(b []byte) (int, error) {
	c.body.Write(b)
	return c.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer's optional
// interfaces (Flusher, Hijacker, ...).
func (c *responseCapture) Unwrap() http.ResponseWriter { return c.ResponseWriter }

// Middleware wraps next with response validation for the route registered as
// method+" "+path. The schema lookup happens once at wrap time; routes with
// no responseSchemas entry (e.g. /_sakumock/ helpers) get next back
// unchanged. Wrap it innermost — directly around the handler — so only what
// the handler itself produces is checked, not a rate-limit or request-
// validation rejection written by an outer middleware.
func (rv *ResponseValidator) Middleware(method, path string, next http.HandlerFunc) http.HandlerFunc {
	if rv == nil {
		return next
	}
	statuses, ok := rv.schemas[method+" "+path]
	if !ok {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		capture := &responseCapture{ResponseWriter: w, status: http.StatusOK}
		next(capture, r)
		schema, declared := statuses[capture.status]
		switch {
		case !declared:
			// Only undeclared success statuses are violations. Specs routinely
			// omit error statuses the real API does return (a 404 for a
			// missing resource, say), and the SDK treats an undeclared error
			// the same against the mock as against the real service — so an
			// undeclared 4xx/5xx is a spec gap, not mock drift. Declared error
			// statuses still get their body validated below.
			if capture.status < 400 {
				rv.record(method, path, capture.status, fmt.Sprintf("status %d is not declared in the spec", capture.status))
			}
		case schema != nil:
			body := capture.body.Bytes()
			if len(body) == 0 {
				rv.record(method, path, capture.status, "response body is empty but the spec declares a JSON body")
				return
			}
			var v any
			if err := json.Unmarshal(body, &v); err != nil {
				rv.record(method, path, capture.status, "response body is not valid JSON")
				return
			}
			if msg := schema.Validate(v); msg != "" {
				rv.record(method, path, capture.status, msg)
			}
		}
	}
}
