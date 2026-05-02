package kms_test

import (
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/sacloud/sakumock/kms"
)

func rawGet(t *testing.T, url string) (int, http.Header) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("drain: %v", err)
	}
	return resp.StatusCode, resp.Header
}

func TestRateLimitDisabled(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()

	for range 30 {
		if status, _ := rawGet(t, srv.TestURL()+"/kms/keys"); status != http.StatusOK {
			t.Fatalf("expected 200, got %d", status)
		}
	}
}

func TestRateLimitExceeded(t *testing.T) {
	// 2 events/sec, burst 2.
	srv := kms.NewTestServer(kms.Config{RateLimit: 2})
	defer srv.Close()

	for i := range 2 {
		if status, _ := rawGet(t, srv.TestURL()+"/kms/keys"); status != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
	status, hdr := rawGet(t, srv.TestURL()+"/kms/keys")
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

func TestRateLimitGlobalBucket(t *testing.T) {
	// All paths share one bucket; 1 event/sec, burst 1.
	srv := kms.NewTestServer(kms.Config{RateLimit: 1})
	defer srv.Close()

	if status, _ := rawGet(t, srv.TestURL()+"/kms/keys"); status != http.StatusOK {
		t.Fatalf("first list: expected 200, got %d", status)
	}
	// A different path should also be limited.
	status, _ := rawGet(t, srv.TestURL()+"/kms/keys/some-id")
	if status != http.StatusTooManyRequests {
		t.Fatalf("second hit on different path: expected 429, got %d", status)
	}
}

func TestRateLimitWindow(t *testing.T) {
	// 5 events per minute -> burst 5 then 429.
	srv := kms.NewTestServer(kms.Config{RateLimit: 5, RateLimitWindow: time.Minute})
	defer srv.Close()

	for i := range 5 {
		if status, _ := rawGet(t, srv.TestURL()+"/kms/keys"); status != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
	if status, _ := rawGet(t, srv.TestURL()+"/kms/keys"); status != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", status)
	}
}
