package objectstorage

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateBucket_NilDataPlane(t *testing.T) {
	var d *dataPlane
	if err := d.createBucket("bucket"); err != nil {
		t.Fatalf("nil data plane must be a no-op, got %v", err)
	}
}

func TestCreateBucket_DirFailure(t *testing.T) {
	d := &dataPlane{
		dir:    filepath.Join(t.TempDir(), "missing"),
		logger: slog.Default(),
	}
	if err := d.createBucket("bucket"); err == nil {
		t.Fatal("expected an error when the backend directory cannot be created")
	}
}

// TestHandleCreateBucket_DataPlaneFailure asserts that a data-plane bucket
// directory failure surfaces as a 500 to the client and rolls the bucket back
// from the store (a retry must not hit 409).
func TestHandleCreateBucket_DataPlaneFailure(t *testing.T) {
	s, err := NewHandler(Config{})
	if err != nil {
		t.Fatalf("NewHandler: %v", err)
	}
	done := make(chan struct{})
	close(done)
	s.dataPlane = &dataPlane{
		dir:    filepath.Join(t.TempDir(), "missing"),
		logger: s.logger,
		cancel: func() {},
		done:   done,
	}
	defer s.Close()

	create := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPut, "/fed/v1/buckets/bucket1", strings.NewReader(`{"cluster_id":"isk01"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		s.ServeHTTP(rr, req)
		return rr
	}

	rr := create()
	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d (body: %s)", rr.Code, rr.Body.String())
	}
	if _, ok := s.store.GetBucket("bucket1"); ok {
		t.Fatal("bucket must be rolled back from the store on data plane failure")
	}
	if rr := create(); rr.Code == http.StatusConflict {
		t.Fatalf("retry after rollback must not hit 409 (body: %s)", rr.Body.String())
	}
}
