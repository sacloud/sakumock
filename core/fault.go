package core

import (
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// FaultErrorWriter writes the response body for an injected HTTP-status fault,
// in whatever shape the service uses for errors. It is responsible for calling
// WriteHeader(status) and writing the body (same contract as
// RateLimitErrorWriter).
type FaultErrorWriter func(w http.ResponseWriter, status int, message string)

// faultRateEpsilon absorbs float accumulation error in the sum-of-rates check,
// so legitimate configs like 0.3 + 0.7 are not rejected.
const faultRateEpsilon = 1e-9

// faultRule is one parsed fault spec. Rules partition [0, sum) with cumulative
// thresholds, so a single random roll per request selects at most one rule and
// each configured rate is exact.
type faultRule struct {
	status int  // HTTP status to inject; 0 means drop the connection ("reset")
	after  bool // fire after running the handler (side effects persist)
	cum    float64
}

// FaultInjector probabilistically injects faults into HTTP requests: an error
// status code, or an abrupt connection drop. A nil *FaultInjector is valid and
// means "fault injection disabled" — Middleware returns the wrapped handler
// unchanged.
type FaultInjector struct {
	rules    []faultRule
	errWrite FaultErrorWriter
	randFn   func() float64
}

type faultConfig struct {
	errWrite FaultErrorWriter
	randFn   func() float64
}

// FaultOption configures a FaultInjector at construction time.
type FaultOption func(*faultConfig)

// WithFaultErrorWriter overrides the response body written for an injected
// status fault. The default writes a small JSON object: {"code":N,"message":"..."}.
func WithFaultErrorWriter(fn FaultErrorWriter) FaultOption {
	return func(c *faultConfig) {
		if fn != nil {
			c.errWrite = fn
		}
	}
}

// WithFaultRand overrides the random source with a function returning values
// in [0, 1). For deterministic tests.
func WithFaultRand(fn func() float64) FaultOption {
	return func(c *faultConfig) {
		if fn != nil {
			c.randFn = fn
		}
	}
}

// ParseFaultSpecs parses fault specs of the form "CODE:RATE" or
// "CODE:RATE:PHASE", where CODE is an HTTP status (100-599) or "reset" (drop
// the TCP connection), RATE is a probability in (0, 1], and PHASE is "before"
// (default: fire without running the handler, no side effects) or "after"
// (run the handler first — side effects persist — then discard its response
// and fire the fault instead). The rates must sum to at most 1. It returns
// (nil, nil) when specs is empty so callers can wire the result directly into
// Middleware without branching.
func ParseFaultSpecs(specs []string, opts ...FaultOption) (*FaultInjector, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	cfg := faultConfig{
		errWrite: defaultFaultErrorWriter,
		randFn:   rand.Float64,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	rules := make([]faultRule, 0, len(specs))
	var sum float64
	for _, spec := range specs {
		parts := strings.Split(spec, ":")
		if len(parts) != 2 && len(parts) != 3 {
			return nil, fmt.Errorf("invalid fault spec %q: want CODE:RATE or CODE:RATE:PHASE", spec)
		}
		var status int
		if parts[0] != "reset" {
			n, err := strconv.Atoi(parts[0])
			if err != nil || n < 100 || n > 599 {
				return nil, fmt.Errorf("invalid fault spec %q: code must be an HTTP status (100-599) or \"reset\"", spec)
			}
			status = n
		}
		rate, err := strconv.ParseFloat(parts[1], 64)
		if err != nil || rate <= 0 || rate > 1 {
			return nil, fmt.Errorf("invalid fault spec %q: rate must be a number in (0, 1]", spec)
		}
		var after bool
		if len(parts) == 3 {
			switch parts[2] {
			case "before":
			case "after":
				after = true
			default:
				return nil, fmt.Errorf("invalid fault spec %q: phase must be \"before\" or \"after\"", spec)
			}
		}
		sum += rate
		rules = append(rules, faultRule{status: status, after: after, cum: sum})
	}
	if sum > 1+faultRateEpsilon {
		return nil, fmt.Errorf("fault rates sum to %v, must not exceed 1", sum)
	}
	return &FaultInjector{rules: rules, errWrite: cfg.errWrite, randFn: cfg.randFn}, nil
}

// Middleware wraps next with fault injection. If fi is nil the original
// handler is returned.
func (fi *FaultInjector) Middleware(next http.HandlerFunc) http.HandlerFunc {
	if fi == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		roll := fi.randFn()
		for _, rule := range fi.rules {
			if roll < rule.cum {
				fi.inject(rule, w, r, next)
				return
			}
		}
		next(w, r)
	}
}

func (fi *FaultInjector) inject(rule faultRule, w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	// Mark the request's span so an injected fault is distinguishable from a
	// real error in traces. With tracing off the span is a no-op.
	span := trace.SpanFromContext(r.Context())
	span.SetAttributes(
		attribute.String("sakumock.fault.code", rule.codeString()),
		attribute.String("sakumock.fault.phase", rule.phaseString()),
	)
	msg := "fault injection"
	if rule.after {
		discard := newDiscardResponseWriter()
		next(discard, r)
		span.SetAttributes(attribute.Int("sakumock.fault.replaced_status", discard.status))
		if rule.status != 0 {
			// Naming the replaced status distinguishes "the handler succeeded
			// but the response was swapped" from a before-phase fault in logs.
			msg = fmt.Sprintf("fault injection (replaced status %d)", discard.status)
		}
	}
	if rule.status == 0 {
		// A dropped connection writes no status code for otelhttp to record,
		// so mark the span failed explicitly.
		span.SetStatus(codes.Error, "fault injection: connection reset")
		abortConnection(w)
		return
	}
	fi.errWrite(w, rule.status, msg)
}

func (r faultRule) codeString() string {
	if r.status == 0 {
		return "reset"
	}
	return strconv.Itoa(r.status)
}

func (r faultRule) phaseString() string {
	if r.after {
		return "after"
	}
	return "before"
}

// discardResponseWriter swallows a handler's response so an "after" fault can
// replace it. It deliberately has no Unwrap method: the handler must not be
// able to reach the real connection through http.ResponseController.
type discardResponseWriter struct {
	header http.Header
	status int
	wrote  bool
}

func newDiscardResponseWriter() *discardResponseWriter {
	return &discardResponseWriter{header: make(http.Header), status: http.StatusOK}
}

func (d *discardResponseWriter) Header() http.Header { return d.header }

func (d *discardResponseWriter) WriteHeader(code int) {
	if !d.wrote {
		d.status = code
		d.wrote = true
	}
}

func (d *discardResponseWriter) Write(b []byte) (int, error) {
	d.wrote = true
	return len(b), nil
}

// abortConnection drops the client connection without writing a response,
// emulating a network-level failure. On a plain TCP connection SetLinger(0)
// makes the close send an RST instead of a FIN; under TLS the raw connection
// is closed directly so no close_notify alert is sent. Either way the client
// sees a hard transport error, not a clean EOF.
func abortConnection(w http.ResponseWriter) {
	if rec, ok := w.(*ResponseRecorder); ok {
		rec.MarkDropped(499, "fault injection: connection reset")
	}
	conn, _, err := http.NewResponseController(w).Hijack()
	if err != nil {
		// No hijack support (e.g. HTTP/2). ErrAbortHandler makes the server
		// reset the stream / drop the connection without a response; the
		// panic unwinds ServeHTTP, so the per-request log is skipped on this
		// path.
		panic(http.ErrAbortHandler)
	}
	raw := conn
	if nc, ok := raw.(interface{ NetConn() net.Conn }); ok { // *tls.Conn
		raw = nc.NetConn()
	}
	if tcp, ok := raw.(*net.TCPConn); ok {
		_ = tcp.SetLinger(0)
	}
	_ = raw.Close()
}

// FaultHint renders a human-readable description of a fault-injection setting
// for startup logs.
func FaultHint(specs []string) string {
	if len(specs) == 0 {
		return "(disabled)"
	}
	return strings.Join(specs, ", ")
}

func defaultFaultErrorWriter(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"code":%d,"message":%q}`+"\n", status, message)
}
