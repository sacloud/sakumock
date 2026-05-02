package simplemq_test

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sacloud/sakumock/simplemq"
)

// doRawRequest is like doRequest but exposes the response headers.
func doRawRequest(t *testing.T, method, url, token, body string) (int, http.Header) {
	t.Helper()
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("failed to drain response: %v", err)
	}
	return resp.StatusCode, resp.Header
}

func sendURL(base, queue string) string {
	return fmt.Sprintf("%s/v1/queues/%s/messages", base, queue)
}

func TestRateLimitDisabled(t *testing.T) {
	srv := simplemq.NewTestServer(simplemq.Config{})
	defer srv.Close()

	url := sendURL(srv.TestURL(), "rate-test-queue")
	body := `{"content":"aGVsbG8="}`
	for i := range 50 {
		status, _ := doRawRequest(t, "POST", url, "test-api-key", body)
		if status != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
}

func TestRateLimitExceeded(t *testing.T) {
	// 2 req/sec with burst 2 -> first 2 pass, 3rd is rate limited.
	srv := simplemq.NewTestServer(simplemq.Config{RateLimit: 2})
	defer srv.Close()

	url := sendURL(srv.TestURL(), "rate-test-queue")
	body := `{"content":"aGVsbG8="}`

	for i := range 2 {
		status, _ := doRawRequest(t, "POST", url, "test-api-key", body)
		if status != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}

	status, hdr := doRawRequest(t, "POST", url, "test-api-key", body)
	if status != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", status)
	}
	retry := hdr.Get("Retry-After")
	if retry == "" {
		t.Fatal("expected Retry-After header, got none")
	}
	secs, err := strconv.Atoi(retry)
	if err != nil {
		t.Fatalf("Retry-After not an integer: %q (%v)", retry, err)
	}
	if secs < 1 {
		t.Errorf("expected Retry-After >= 1, got %d", secs)
	}
}

func TestRateLimitPerQueue(t *testing.T) {
	srv := simplemq.NewTestServer(simplemq.Config{RateLimit: 1})
	defer srv.Close()

	body := `{"content":"aGVsbG8="}`

	// Exhaust queue A's bucket.
	urlA := sendURL(srv.TestURL(), "queue-aaaa")
	if status, _ := doRawRequest(t, "POST", urlA, "test-api-key", body); status != http.StatusOK {
		t.Fatalf("queue A first request: expected 200, got %d", status)
	}
	if status, _ := doRawRequest(t, "POST", urlA, "test-api-key", body); status != http.StatusTooManyRequests {
		t.Fatalf("queue A second request: expected 429, got %d", status)
	}

	// Queue B has its own bucket and should still pass.
	urlB := sendURL(srv.TestURL(), "queue-bbbb")
	if status, _ := doRawRequest(t, "POST", urlB, "test-api-key", body); status != http.StatusOK {
		t.Fatalf("queue B first request: expected 200, got %d", status)
	}
}

func TestRateLimitRecovery(t *testing.T) {
	// 10 req/sec -> a single token refills in ~100ms.
	srv := simplemq.NewTestServer(simplemq.Config{RateLimit: 10})
	defer srv.Close()

	url := sendURL(srv.TestURL(), "rate-test-queue")
	body := `{"content":"aGVsbG8="}`

	// Drain the burst (10).
	for i := range 10 {
		if status, _ := doRawRequest(t, "POST", url, "test-api-key", body); status != http.StatusOK {
			t.Fatalf("drain iter %d: expected 200, got %d", i, status)
		}
	}
	if status, _ := doRawRequest(t, "POST", url, "test-api-key", body); status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after drain, got %d", status)
	}

	// Wait long enough for at least one token to refill.
	time.Sleep(250 * time.Millisecond)

	if status, _ := doRawRequest(t, "POST", url, "test-api-key", body); status != http.StatusOK {
		t.Fatalf("expected 200 after refill, got %d", status)
	}
}

func TestRateLimitWindow(t *testing.T) {
	// 5 events per minute -> burst 5 immediately, then 429.
	srv := simplemq.NewTestServer(simplemq.Config{
		RateLimit:       5,
		RateLimitWindow: time.Minute,
	})
	defer srv.Close()

	url := sendURL(srv.TestURL(), "rate-test-queue")
	body := `{"content":"aGVsbG8="}`

	for i := range 5 {
		if status, _ := doRawRequest(t, "POST", url, "test-api-key", body); status != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, status)
		}
	}
	if status, _ := doRawRequest(t, "POST", url, "test-api-key", body); status != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after burst, got %d", status)
	}
}

func TestRateLimitAppliesToAllMethods(t *testing.T) {
	// 1 req/sec, burst 1 -> after one request of any kind, the next is limited.
	srv := simplemq.NewTestServer(simplemq.Config{RateLimit: 1})
	defer srv.Close()

	queueURL := sendURL(srv.TestURL(), "rate-test-queue")

	// First request (POST) consumes the only token.
	if status, _ := doRawRequest(t, "POST", queueURL, "test-api-key", `{"content":"aGVsbG8="}`); status != http.StatusOK {
		t.Fatalf("send: expected 200, got %d", status)
	}
	// A different method to the same queue should now be limited.
	if status, _ := doRawRequest(t, "GET", queueURL, "test-api-key", ""); status != http.StatusTooManyRequests {
		t.Fatalf("receive: expected 429, got %d", status)
	}
}
