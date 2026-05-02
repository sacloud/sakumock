package core_test

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/sacloud/sakumock/core"
)

func okHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestRateLimiterNilIsPassThrough(t *testing.T) {
	var rl *core.RateLimiter // disabled
	h := rl.Middleware(core.PathValueKey("queueName"), okHandler)

	mux := http.NewServeMux()
	mux.HandleFunc("/q/{queueName}", h)
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for range 10 {
		resp, err := http.Get(srv.URL + "/q/foo")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	}
}

func TestRateLimiterExceeded(t *testing.T) {
	rl := core.NewRateLimiter(2)
	mux := http.NewServeMux()
	mux.HandleFunc("/q/{queueName}", rl.Middleware(core.PathValueKey("queueName"), okHandler))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Burst = ceil(2) = 2: first 2 pass.
	for i := range 2 {
		resp, err := http.Get(srv.URL + "/q/aaa")
		if err != nil {
			t.Fatalf("get %d: %v", i, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, resp.StatusCode)
		}
	}

	resp, err := http.Get(srv.URL + "/q/aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", resp.StatusCode)
	}
	retry := resp.Header.Get("Retry-After")
	if retry == "" {
		t.Fatal("missing Retry-After header")
	}
	secs, err := strconv.Atoi(retry)
	if err != nil || secs < 1 {
		t.Errorf("invalid Retry-After: %q (err=%v)", retry, err)
	}
}

func TestRateLimiterPerKey(t *testing.T) {
	rl := core.NewRateLimiter(1)
	mux := http.NewServeMux()
	mux.HandleFunc("/q/{queueName}", rl.Middleware(core.PathValueKey("queueName"), okHandler))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Drain key A.
	r1, err := http.Get(srv.URL + "/q/aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	r1.Body.Close()
	if r1.StatusCode != http.StatusOK {
		t.Fatalf("A1: expected 200, got %d", r1.StatusCode)
	}
	r2, err := http.Get(srv.URL + "/q/aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	r2.Body.Close()
	if r2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("A2: expected 429, got %d", r2.StatusCode)
	}

	// Key B has its own bucket.
	r3, err := http.Get(srv.URL + "/q/bbb")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	r3.Body.Close()
	if r3.StatusCode != http.StatusOK {
		t.Fatalf("B1: expected 200, got %d", r3.StatusCode)
	}
}

func TestRateLimiterEmptyKeyBypasses(t *testing.T) {
	rl := core.NewRateLimiter(1)
	keyFn := func(_ *http.Request) string { return "" }

	mux := http.NewServeMux()
	mux.HandleFunc("/x", rl.Middleware(keyFn, okHandler))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for range 5 {
		resp, err := http.Get(srv.URL + "/x")
		if err != nil {
			t.Fatalf("get: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	}
}

func TestRateLimiterCustomErrorWriter(t *testing.T) {
	written := false
	custom := func(w http.ResponseWriter, status int, message string) {
		written = true
		w.Header().Set("Content-Type", "application/x-test")
		w.WriteHeader(status)
		_, _ = w.Write([]byte("custom: " + message))
	}
	rl := core.NewRateLimiter(1, core.WithRateLimitErrorWriter(custom))
	mux := http.NewServeMux()
	mux.HandleFunc("/q/{queueName}", rl.Middleware(core.PathValueKey("queueName"), okHandler))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	r1, _ := http.Get(srv.URL + "/q/aaa")
	r1.Body.Close()
	r2, err := http.Get(srv.URL + "/q/aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r2.Body.Close()
	if r2.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", r2.StatusCode)
	}
	if !written {
		t.Error("custom error writer not called")
	}
	if got := r2.Header.Get("Content-Type"); got != "application/x-test" {
		t.Errorf("unexpected Content-Type: %q", got)
	}
}

func TestRateLimiterWindow(t *testing.T) {
	// 100 events per minute = ~1.667/sec, but burst is the full 100.
	rl := core.NewRateLimiter(100, core.WithRateLimitWindow(time.Minute))
	mux := http.NewServeMux()
	mux.HandleFunc("/q/{queueName}", rl.Middleware(core.PathValueKey("queueName"), okHandler))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Drain the full burst of 100.
	for i := range 100 {
		r, err := http.Get(srv.URL + "/q/aaa")
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		r.Body.Close()
		if r.StatusCode != http.StatusOK {
			t.Fatalf("iter %d: expected 200, got %d", i, r.StatusCode)
		}
	}
	// The next one should be limited.
	r, err := http.Get(srv.URL + "/q/aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after 100 in-window, got %d", r.StatusCode)
	}
	// Refill rate ~1.667/sec means a token returns in <1s; Retry-After (rounded up) is 1.
	if got := r.Header.Get("Retry-After"); got != "1" {
		t.Errorf("expected Retry-After=1, got %q", got)
	}
}

func TestRateLimiterRecovery(t *testing.T) {
	rl := core.NewRateLimiter(10)
	mux := http.NewServeMux()
	mux.HandleFunc("/q/{queueName}", rl.Middleware(core.PathValueKey("queueName"), okHandler))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	for range 10 {
		r, _ := http.Get(srv.URL + "/q/aaa")
		r.Body.Close()
	}
	r, _ := http.Get(srv.URL + "/q/aaa")
	r.Body.Close()
	if r.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected 429 after drain, got %d", r.StatusCode)
	}

	time.Sleep(250 * time.Millisecond)

	r, err := http.Get(srv.URL + "/q/aaa")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 after refill, got %d", r.StatusCode)
	}
}
