package core

import (
	"context"
	"net/http"
	"strings"
	"time"
)

// InspectionPathPrefix is the reserved path prefix of mock-only endpoints
// (Route Kind "inspection").
const InspectionPathPrefix = "/_sakumock/"

// APILatency delays a control-plane request by d before it is served (the
// --latency flag). Mock-only /_sakumock/ endpoints are exempt, like rate
// limiting and fault injection. It returns early when the request is cancelled.
func APILatency(r *http.Request, d time.Duration) {
	if strings.HasPrefix(r.URL.Path, InspectionPathPrefix) {
		return
	}
	sleepCtx(r.Context(), d)
}

// LatencyHandler delays every request by d before h serves it (the
// --data-plane-latency flag). It returns h unchanged when d is not positive.
// Unlike APILatency no path is exempt: a data plane serves user paths.
func LatencyHandler(d time.Duration, h http.Handler) http.Handler {
	if d <= 0 {
		return h
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sleepCtx(r.Context(), d)
		h.ServeHTTP(w, r)
	})
}

func sleepCtx(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
	}
}
