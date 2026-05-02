package simplenotification_test

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sacloud/sakumock/simplenotification"
)

const testGroupID = "123456789012"

func sendURL(base string) string {
	return base + "/commonserviceitem/" + testGroupID + "/simplenotification/message"
}

func postSend(t *testing.T, url string) (int, http.Header) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(`{"Message":"hi"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("drain: %v", err)
	}
	return resp.StatusCode, resp.Header
}

func TestRateLimitDisabled(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{})
	defer srv.Close()

	url := sendURL(srv.TestURL())
	for range 30 {
		if status, _ := postSend(t, url); status != http.StatusAccepted {
			t.Fatalf("expected 200, got %d", status)
		}
	}
}

func TestRateLimitExceeded(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{RateLimit: 2})
	defer srv.Close()

	url := sendURL(srv.TestURL())
	for i := range 2 {
		if status, _ := postSend(t, url); status != http.StatusAccepted {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
	status, hdr := postSend(t, url)
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", status)
	}
	retry := hdr.Get("Retry-After")
	if retry == "" {
		t.Fatal("missing Retry-After header")
	}
	secs, err := strconv.Atoi(retry)
	if err != nil || secs < 1 {
		t.Errorf("invalid Retry-After: %q (err=%v)", retry, err)
	}
}

func TestRateLimitInspectionBypassed(t *testing.T) {
	// Inspection endpoints (/_sakumock/messages) must not consume tokens.
	srv := simplenotification.NewTestServer(simplenotification.Config{RateLimit: 1})
	defer srv.Close()

	// Drain the API bucket first.
	if status, _ := postSend(t, sendURL(srv.TestURL())); status != http.StatusAccepted {
		t.Fatalf("first send: expected 202, got %d", status)
	}
	if status, _ := postSend(t, sendURL(srv.TestURL())); status != http.StatusTooManyRequests {
		t.Fatalf("second send: expected 429, got %d", status)
	}

	// Inspection should still respond regardless.
	for range 10 {
		resp, err := http.Get(srv.TestURL() + "/_sakumock/messages")
		if err != nil {
			t.Fatalf("inspect get: %v", err)
		}
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("inspection: expected 200, got %d", resp.StatusCode)
		}
	}
}

func TestRateLimitWindow(t *testing.T) {
	srv := simplenotification.NewTestServer(simplenotification.Config{
		RateLimit:       5,
		RateLimitWindow: time.Minute,
	})
	defer srv.Close()

	url := sendURL(srv.TestURL())
	for i := range 5 {
		if status, _ := postSend(t, url); status != http.StatusAccepted {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
	if status, _ := postSend(t, url); status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", status)
	}
}
