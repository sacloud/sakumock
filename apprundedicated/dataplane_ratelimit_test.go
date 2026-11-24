package apprundedicated_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/sacloud/sakumock/apprundedicated"
)

func TestDataPlaneRateLimit(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{
		EnableDataPlane:          true,
		DataPlaneAddr:            freeAddr(t),
		DataPlaneRateLimit:       1,
		DataPlaneRateLimitWindow: time.Minute,
	})
	defer srv.Close()

	// No application is addressed (no app ID in the Host header), so the proxy
	// answers 400 without touching Docker; the limit covers the listener.
	get := func() int {
		t.Helper()
		resp, err := http.Get("http://" + srv.DataPlaneAddr() + "/")
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	if code := get(); code != http.StatusBadRequest {
		t.Fatalf("first request status = %d, want %d", code, http.StatusBadRequest)
	}
	if code := get(); code != http.StatusTooManyRequests {
		t.Errorf("second request status = %d, want %d", code, http.StatusTooManyRequests)
	}
}
