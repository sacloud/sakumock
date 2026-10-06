package core_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sacloud/sakumock/core"
)

const testLatency = 200 * time.Millisecond

func elapsed(f func()) time.Duration {
	start := time.Now()
	f()
	return time.Since(start)
}

func TestAPILatency(t *testing.T) {
	for _, tc := range []struct {
		path    string
		delayed bool
	}{
		{"/alerts/projects/", true},
		{"/_sakumock/spec-violations", false},
		{"/_sakumock/alerts/123/fire", false},
		{"/_sakumockx", true},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.path, nil)
		d := elapsed(func() { core.APILatency(r, testLatency) })
		if got := d >= testLatency; got != tc.delayed {
			t.Errorf("%s: elapsed %v, want delayed=%v", tc.path, d, tc.delayed)
		}
	}
}

func TestAPILatencyCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx)
	if d := elapsed(func() { core.APILatency(r, time.Hour) }); d >= testLatency {
		t.Errorf("cancelled request still delayed %v", d)
	}
}
