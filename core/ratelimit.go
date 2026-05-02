package core

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimitKeyFunc extracts a bucket key from a request. Returning an empty string
// makes the limiter skip the request (no token consumed, no 429).
type RateLimitKeyFunc func(*http.Request) string

// RateLimitErrorWriter writes the response body for a rate-limited request.
// The middleware has already set the Retry-After header before calling this; the
// writer is responsible for calling WriteHeader(status) and writing the body in
// whatever shape the service uses for errors.
type RateLimitErrorWriter func(w http.ResponseWriter, status int, message string)

// RateLimiter applies a per-key token-bucket limit to incoming HTTP requests.
// A nil *RateLimiter is valid and means "rate limiting disabled" — Middleware
// returns the wrapped handler unchanged.
type RateLimiter struct {
	limit    rate.Limit
	burst    int
	errWrite RateLimitErrorWriter

	mu       sync.Mutex
	limiters map[string]*rate.Limiter
}

type rateLimiterConfig struct {
	window   time.Duration
	errWrite RateLimitErrorWriter
}

// RateLimiterOption configures a RateLimiter at construction time.
type RateLimiterOption func(*rateLimiterConfig)

// WithRateLimitErrorWriter overrides the response body written when a request is
// rate limited. The default writes a small JSON object: {"code":429,"message":"..."}.
func WithRateLimitErrorWriter(fn RateLimitErrorWriter) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		if fn != nil {
			c.errWrite = fn
		}
	}
}

// WithRateLimitWindow sets the time window the events count refers to.
// For example, NewRateLimiter(100, WithRateLimitWindow(time.Minute)) limits
// each key to 100 requests per minute. The default window is one second.
// Non-positive durations are ignored.
func WithRateLimitWindow(d time.Duration) RateLimiterOption {
	return func(c *rateLimiterConfig) {
		if d > 0 {
			c.window = d
		}
	}
}

// NewRateLimiter returns a *RateLimiter that allows events requests per window
// (default 1 second) per key, with a burst equal to ceil(events). It returns
// nil when events <= 0 so callers can wire the result directly into Middleware
// without branching.
func NewRateLimiter(events float64, opts ...RateLimiterOption) *RateLimiter {
	if events <= 0 {
		return nil
	}
	cfg := rateLimiterConfig{
		window:   time.Second,
		errWrite: defaultRateLimitErrorWriter,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return &RateLimiter{
		limit:    rate.Limit(events / cfg.window.Seconds()),
		burst:    max(int(math.Ceil(events)), 1),
		errWrite: cfg.errWrite,
		limiters: make(map[string]*rate.Limiter),
	}
}

// Middleware wraps next with rate limiting. If rl is nil the original handler is
// returned. If keyFn is nil or returns "" for a request, the limiter is bypassed.
func (rl *RateLimiter) Middleware(keyFn RateLimitKeyFunc, next http.HandlerFunc) http.HandlerFunc {
	if rl == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		var key string
		if keyFn != nil {
			key = keyFn(r)
		}
		if key == "" {
			next(w, r)
			return
		}
		l := rl.getLimiter(key)
		now := time.Now()
		if !l.AllowN(now, 1) {
			w.Header().Set("Retry-After", strconv.Itoa(rl.retryAfterSeconds(l, now)))
			rl.errWrite(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}
		next(w, r)
	}
}

func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if l, ok := rl.limiters[key]; ok {
		return l
	}
	l := rate.NewLimiter(rl.limit, rl.burst)
	rl.limiters[key] = l
	return l
}

func (rl *RateLimiter) retryAfterSeconds(l *rate.Limiter, now time.Time) int {
	r := l.ReserveN(now, 1)
	defer r.Cancel()
	if !r.OK() {
		return 1
	}
	return max(int(math.Ceil(r.DelayFrom(now).Seconds())), 1)
}

// PathValueKey returns a RateLimitKeyFunc that uses r.PathValue(name) as the bucket key.
// Useful with Go 1.22+ ServeMux patterns like "/v1/queues/{queueName}/messages".
func PathValueKey(name string) RateLimitKeyFunc {
	return func(r *http.Request) string {
		return r.PathValue(name)
	}
}

func defaultRateLimitErrorWriter(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"code":%d,"message":%q}`+"\n", status, message)
}
