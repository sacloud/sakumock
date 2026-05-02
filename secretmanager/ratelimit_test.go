package secretmanager_test

import (
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/sacloud/sakumock/secretmanager"
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

func vaultListURL(base string) string {
	return base + "/secretmanager/vaults/" + testVaultID + "/secrets"
}

func TestRateLimitDisabled(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer srv.Close()

	url := vaultListURL(srv.TestURL())
	for range 30 {
		if status, _ := rawGet(t, url); status != http.StatusOK {
			t.Fatalf("expected 200, got %d", status)
		}
	}
}

func TestRateLimitExceeded(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{RateLimit: 2})
	defer srv.Close()

	url := vaultListURL(srv.TestURL())
	for i := range 2 {
		if status, _ := rawGet(t, url); status != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
	status, hdr := rawGet(t, url)
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

func TestRateLimitWindowMatchesProductionQuota(t *testing.T) {
	// Production-like 100 req/min: burst is 100, then 429.
	srv := secretmanager.NewTestServer(secretmanager.Config{
		RateLimit:       100,
		RateLimitWindow: time.Minute,
	})
	defer srv.Close()

	url := vaultListURL(srv.TestURL())
	for i := range 100 {
		if status, _ := rawGet(t, url); status != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
	if status, _ := rawGet(t, url); status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", status)
	}
}
