package objectstorage

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestPosixMetadataArgs asserts the per-OS metadata flags: none on Unix
// (xattr, versitygw's default, works there), a pre-created sidecar directory
// on Windows (NTFS has no xattrs and versitygw checks at startup).
func TestPosixMetadataArgs(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "backend")
	args, err := posixMetadataArgs(dir)
	if err != nil {
		t.Fatalf("posixMetadataArgs: %v", err)
	}
	if runtime.GOOS != "windows" {
		if len(args) != 0 {
			t.Fatalf("expected no extra args on %s, got %v", runtime.GOOS, args)
		}
		return
	}
	want := []string{"--sidecar", sidecarDir(dir)}
	if len(args) != 2 || args[0] != want[0] || args[1] != want[1] {
		t.Fatalf("args = %v, want %v", args, want)
	}
	if fi, err := os.Stat(sidecarDir(dir)); err != nil || !fi.IsDir() {
		t.Fatalf("sidecar dir must exist as a directory (err=%v)", err)
	}
}

// TestIAMUserLifecycle asserts that createUser/deleteUser maintain
// versitygw's users.json: registered keys appear as admin accounts (in the
// exact schema versitygw's internal IAM service parses), existing entries
// survive updates, and deletion removes only the targeted key.
func TestIAMUserLifecycle(t *testing.T) {
	d := &dataPlane{iamDir: t.TempDir(), logger: slog.Default()}

	if err := d.createUser("KEYA", "secret-a"); err != nil {
		t.Fatalf("createUser KEYA: %v", err)
	}
	if err := d.createUser("KEYB", "secret-b"); err != nil {
		t.Fatalf("createUser KEYB: %v", err)
	}

	readConf := func() iamConfig {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(d.iamDir, iamUsersFile))
		if err != nil {
			t.Fatalf("read %s: %v", iamUsersFile, err)
		}
		var conf iamConfig
		if err := json.Unmarshal(b, &conf); err != nil {
			t.Fatalf("parse %s: %v", iamUsersFile, err)
		}
		return conf
	}

	conf := readConf()
	if len(conf.AccessAccounts) != 2 {
		t.Fatalf("accounts = %d, want 2 (%v)", len(conf.AccessAccounts), conf.AccessAccounts)
	}
	want := iamAccount{Access: "KEYA", Secret: "secret-a", Role: iamRoleAdmin}
	if got := conf.AccessAccounts["KEYA"]; got != want {
		t.Errorf("KEYA = %+v, want %+v", got, want)
	}

	d.deleteUser("KEYA")
	conf = readConf()
	if _, ok := conf.AccessAccounts["KEYA"]; ok {
		t.Error("KEYA must be removed after deleteUser")
	}
	if _, ok := conf.AccessAccounts["KEYB"]; !ok {
		t.Error("KEYB must survive deleting KEYA")
	}
}

// TestIAMUser_NilDataPlane asserts key mirroring is a no-op without a data
// plane, so handlers can call it unconditionally.
func TestIAMUser_NilDataPlane(t *testing.T) {
	var d *dataPlane
	if err := d.createUser("KEY", "secret"); err != nil {
		t.Fatalf("nil data plane createUser must be a no-op, got %v", err)
	}
	d.deleteUser("KEY")
}

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
