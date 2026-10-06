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
