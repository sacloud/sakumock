package core_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/core"
)

func TestParseFaultSpecsErrors(t *testing.T) {
	for _, specs := range [][]string{
		{"500"},                // missing rate
		{"abc:0.1"},            // non-numeric code
		{"42:0.1"},             // status out of range
		{"600:0.1"},            // status out of range
		{"500:0"},              // rate must be > 0
		{"500:-0.1"},           // rate must be > 0
		{"500:1.5"},            // rate must be <= 1
		{"500:x"},              // non-numeric rate
		{"500:0.1:sometime"},   // unknown phase
		{"500:0.1:after:x"},    // too many parts
		{"500:0.6", "429:0.6"}, // rates sum > 1
	} {
		if _, err := core.ParseFaultSpecs(specs); err == nil {
			t.Errorf("ParseFaultSpecs(%q): expected error, got nil", specs)
		}
	}
}

func TestParseFaultSpecsEmpty(t *testing.T) {
	fi, err := core.ParseFaultSpecs(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fi != nil {
		t.Fatalf("expected nil injector for empty specs, got %v", fi)
	}
	// A nil injector's Middleware must return the handler unchanged.
	called := false
	h := fi.Middleware(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	if !called || rec.Code != http.StatusOK {
		t.Fatalf("pass-through failed: called=%v status=%d", called, rec.Code)
	}
}

func TestFaultInjectorStatusBefore(t *testing.T) {
	written := false
	custom := func(w http.ResponseWriter, status int, message string) {
		written = true
		w.WriteHeader(status)
		_, _ = w.Write([]byte("custom: " + message))
	}
	fi, err := core.ParseFaultSpecs([]string{"500:1"}, core.WithFaultErrorWriter(custom))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	handlerCalled := false
	h := fi.Middleware(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("GET", "/", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if handlerCalled {
		t.Error("before-phase fault must not run the handler")
	}
	if !written || !strings.Contains(rec.Body.String(), "fault injection") {
		t.Errorf("unexpected body: %q (writer called=%v)", rec.Body.String(), written)
	}
}

func TestFaultInjectorStatusAfter(t *testing.T) {
	fi, err := core.ParseFaultSpecs([]string{"500:1:after"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	handlerCalled := false
	h := fi.Middleware(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	rec := httptest.NewRecorder()
	h(rec, httptest.NewRequest("POST", "/", nil))
	if !handlerCalled {
		t.Fatal("after-phase fault must run the handler")
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `"ok"`) {
		t.Errorf("handler response leaked to the client: %q", body)
	}
	if !strings.Contains(body, "replaced status 201") {
		t.Errorf("expected replaced-status message, got %q", body)
	}
}

func TestFaultInjectorCumulativeSelection(t *testing.T) {
	// Rules partition [0, 0.8): [0, 0.5) -> 500, [0.5, 0.8) -> 429, rest passes.
	for _, tc := range []struct {
		roll float64
		want int
	}{
		{0.49, http.StatusInternalServerError},
		{0.51, http.StatusTooManyRequests},
		{0.81, http.StatusOK},
	} {
		fi, err := core.ParseFaultSpecs(
			[]string{"500:0.5", "429:0.3"},
			core.WithFaultRand(func() float64 { return tc.roll }),
		)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		h := fi.Middleware(okHandler)
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("GET", "/", nil))
		if rec.Code != tc.want {
			t.Errorf("roll %v: expected %d, got %d", tc.roll, tc.want, rec.Code)
		}
	}
}

func TestFaultInjectorReset(t *testing.T) {
	fi, err := core.ParseFaultSpecs([]string{"reset:1"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	srv := httptest.NewServer(fi.Middleware(okHandler))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected a transport error, got %d %q", resp.StatusCode, body)
	}
	// The exact error text (connection reset vs EOF) is platform/timing
	// dependent; any transport error is a pass.
}

func TestFaultInjectorResetAfterRunsHandler(t *testing.T) {
	fi, err := core.ParseFaultSpecs([]string{"reset:1:after"})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	handlerCalled := false
	srv := httptest.NewServer(fi.Middleware(func(w http.ResponseWriter, _ *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if _, err := http.Get(srv.URL); err == nil {
		t.Fatal("expected a transport error, got response")
	}
	if !handlerCalled {
		t.Error("after-phase reset must run the handler first")
	}
}

func TestMarkDropped(t *testing.T) {
	rec := core.NewResponseRecorder(httptest.NewRecorder())
	rec.MarkDropped(499, "fault injection: connection reset")
	if rec.Status != 499 {
		t.Errorf("expected status 499, got %d", rec.Status)
	}
	if got := rec.ErrorBody(); got != "fault injection: connection reset" {
		t.Errorf("unexpected error body: %q", got)
	}
}
