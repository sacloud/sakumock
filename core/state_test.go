package core_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/sacloud/sakumock/core"
)

// counterStore is a minimal Snapshotter.
type counterStore struct {
	N          int `json:"n"`
	snapshots  int
	failOnSnap bool
}

func (s *counterStore) Snapshot() ([]byte, error) {
	s.snapshots++
	if s.failOnSnap {
		return nil, errors.New("boom")
	}
	return json.Marshal(s)
}

func (s *counterStore) Restore(data []byte) error {
	return json.Unmarshal(data, s)
}

func TestStateFileDisabled(t *testing.T) {
	f, err := core.OpenState("", "svc", &counterStore{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if f != nil {
		t.Fatalf("OpenState with empty dir = %v, want nil", f)
	}
	// A nil StateFile is a no-op.
	if err := f.Save(); err != nil {
		t.Errorf("Save on nil: %v", err)
	}
	f.SaveAfter(httptest.NewRequest(http.MethodPost, "/", nil))
	if f.Path() != "" {
		t.Errorf("Path on nil = %q", f.Path())
	}
}

func TestStateFileSaveAndRestore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state") // created on demand
	store := &counterStore{}
	f, err := core.OpenState(dir, "svc", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "svc.json")
	if f.Path() != path {
		t.Errorf("Path = %q, want %q", f.Path(), path)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("state file exists before the first save: %v", err)
	}

	store.N = 42
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no group/other permission bits to check.
	if perm := info.Mode().Perm(); runtime.GOOS != "windows" && perm&0o077 != 0 {
		t.Errorf("state file mode = %o, want no group/other access", perm)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("state dir has %d entries, want only the state file (no leftover temp file)", len(entries))
	}

	restored := &counterStore{}
	if _, err := core.OpenState(dir, "svc", restored, nil); err != nil {
		t.Fatal(err)
	}
	if restored.N != 42 {
		t.Errorf("restored N = %d, want 42", restored.N)
	}
}

func TestStateFileSkipsUnchangedWrite(t *testing.T) {
	dir := t.TempDir()
	store := &counterStore{N: 1}
	f, err := core.OpenState(dir, "svc", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	// Replace the file behind the StateFile's back: an unchanged snapshot must
	// not be written again.
	if err := os.WriteFile(f.Path(), []byte("sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.Path()); string(got) != "sentinel" {
		t.Errorf("unchanged state was rewritten: %q", got)
	}
	store.N = 2
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.Path()); !strings.Contains(string(got), `"n": 2`) {
		t.Errorf("changed state not written: %q", got)
	}
}

func TestStateFileSaveAfter(t *testing.T) {
	store := &counterStore{}
	f, err := core.OpenState(t.TempDir(), "svc", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		f.SaveAfter(httptest.NewRequest(method, "/", nil))
	}
	if store.snapshots != 0 {
		t.Errorf("safe methods took %d snapshots, want 0", store.snapshots)
	}
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		f.SaveAfter(httptest.NewRequest(method, "/", nil))
	}
	if store.snapshots != 4 {
		t.Errorf("unsafe methods took %d snapshots, want 4", store.snapshots)
	}
	if _, err := os.Stat(f.Path()); err != nil {
		t.Errorf("state file not written: %v", err)
	}

	// A failing snapshot is logged, not propagated, and leaves the file alone.
	store.failOnSnap = true
	f.SaveAfter(httptest.NewRequest(http.MethodPost, "/", nil))
	if err := f.Save(); err == nil {
		t.Error("Save with a failing snapshot succeeded, want error")
	}
	if got, _ := os.ReadFile(f.Path()); !strings.Contains(string(got), `"n": 0`) {
		t.Errorf("state file changed by a failed save: %q", got)
	}
}

func TestStateFileCorrupt(t *testing.T) {
	for name, content := range map[string]string{
		"syntax error":     "{not json",
		"not a state file": `{"n":1}`,
		"null state":       `{"version":"x","state":null}`,
		"unrestorable":     `{"version":"x","state":{"n":"not a number"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "svc.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := core.OpenState(dir, "svc", &counterStore{}, nil)
			if err == nil {
				t.Fatal("OpenState with a corrupt file succeeded, want error")
			}
			if !strings.Contains(err.Error(), path) {
				t.Errorf("error does not name the state file: %v", err)
			}
			// The unreadable file is kept for the user to inspect.
			if got, _ := os.ReadFile(path); string(got) != content {
				t.Errorf("corrupt state file was modified: %q", got)
			}
		})
	}
}

// A state saved by another sakumock version is restored with a warning, and
// the file takes the running version at the next change.
func TestStateFileOtherVersion(t *testing.T) {
	dir := t.TempDir()
	restore := core.SetStateVersion("0.0.1")
	old, err := core.OpenState(dir, "svc", &counterStore{N: 7}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := old.Save(); err != nil {
		t.Fatal(err)
	}
	restore()

	var logs bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logs, nil))
	store := &counterStore{}
	f, err := core.OpenState(dir, "svc", store, logger)
	if err != nil {
		t.Fatalf("OpenState of another version's state: %v", err)
	}
	if store.N != 7 {
		t.Errorf("restored N = %d, want 7", store.N)
	}
	if out := logs.String(); !strings.Contains(out, "level=WARN") || !strings.Contains(out, "saved_by=0.0.1") {
		t.Errorf("no warning naming the saving version:\n%s", out)
	}

	// Unchanged state is not rewritten just to bump the version.
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.Path()); !strings.Contains(string(got), `"version": "0.0.1"`) {
		t.Errorf("unchanged state was rewritten: %s", got)
	}
	store.N = 8
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(f.Path()); strings.Contains(string(got), "0.0.1") {
		t.Errorf("changed state still carries the old version: %s", got)
	}

	// The same version restores silently.
	logs.Reset()
	if _, err := core.OpenState(dir, "svc", &counterStore{}, logger); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(logs.String(), "level=WARN") {
		t.Errorf("warning for a state of the running version:\n%s", logs.String())
	}
}

// Serve handles requests that may change the state one at a time and saves
// after each, so a snapshot never sees a record half-updated by a handler.
func TestStateFileServe(t *testing.T) {
	store := &counterStore{}
	f, err := core.OpenState(t.TempDir(), "svc", store, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Unsynchronized on purpose: the race detector fails this test unless
	// Serve serializes the handlers and the snapshots.
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { store.N++ })

	const requests = 50
	var wg sync.WaitGroup
	for range requests {
		wg.Go(func() {
			f.Serve(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil), next)
		})
	}
	wg.Wait()

	restored := &counterStore{}
	if _, err := core.OpenState(filepath.Dir(f.Path()), "svc", restored, nil); err != nil {
		t.Fatal(err)
	}
	if restored.N != requests {
		t.Errorf("restored N = %d, want %d", restored.N, requests)
	}

	// Safe methods and a nil StateFile just run the handler.
	before := store.snapshots
	f.Serve(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil), http.NotFoundHandler())
	if store.snapshots != before {
		t.Error("a GET took a snapshot")
	}
	var none *core.StateFile
	called := false
	none.Serve(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", nil),
		http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	if !called {
		t.Error("a nil StateFile did not run the handler")
	}
}
