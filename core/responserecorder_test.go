package core

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResponseRecorderCapturesErrorBody(t *testing.T) {
	w := httptest.NewRecorder()
	rec := NewResponseRecorder(w)
	rec.WriteHeader(400)
	rec.Write([]byte(`{"message":"bad request"}` + "\n"))

	if rec.Status != 400 {
		t.Errorf("Status = %d, want 400", rec.Status)
	}
	if got := rec.ErrorBody(); got != `{"message":"bad request"}` {
		t.Errorf("ErrorBody = %q", got)
	}
	if w.Body.String() != `{"message":"bad request"}`+"\n" {
		t.Errorf("underlying body = %q", w.Body.String())
	}
}

func TestResponseRecorderIgnoresSuccessBody(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.Write([]byte(`{"ok":true}`)) // implicit 200

	if rec.Status != 200 {
		t.Errorf("Status = %d, want 200", rec.Status)
	}
	if got := rec.ErrorBody(); got != "" {
		t.Errorf("ErrorBody = %q, want empty", got)
	}
}

func TestResponseRecorderBoundsErrorBody(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.WriteHeader(500)
	rec.Write([]byte(strings.Repeat("x", 10*maxErrorBodyLog)))

	if got := len(rec.ErrorBody()); got != maxErrorBodyLog {
		t.Errorf("captured %d bytes, want %d", got, maxErrorBodyLog)
	}
}

func TestRequestLogArgs(t *testing.T) {
	rec := NewResponseRecorder(httptest.NewRecorder())
	rec.WriteHeader(404)
	rec.Write([]byte(`{"message":"not found"}`))
	r := httptest.NewRequest("GET", "/things/1", nil)

	args := RequestLogArgs(r, rec, "extra", "value")
	want := []any{"method", "GET", "path", "/things/1", "status", 404, "error", `{"message":"not found"}`, "extra", "value"}
	if len(args) != len(want) {
		t.Fatalf("args = %v", args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %v, want %v", i, args[i], want[i])
		}
	}
}
