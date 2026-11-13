package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/sacloud/sakumock"
)

// stateVersion is the version recorded in every state file. It is a variable
// only so tests can stand in for another release.
var stateVersion = sakumock.Version

// stateEnvelope is the content of a state file: the store's snapshot and the
// sakumock version that wrote it.
type stateEnvelope struct {
	Version string          `json:"version"`
	State   json.RawMessage `json:"state"`
}

// Snapshotter is implemented by a store whose whole state can be written out
// and read back, so it survives a restart of the mock (see StateFile).
type Snapshotter interface {
	// Snapshot returns the store's state as JSON. The encoding must be
	// deterministic (the same state yields the same bytes): StateFile compares
	// snapshots to skip writes that would change nothing.
	Snapshot() ([]byte, error)
	// Restore replaces the store's state with a snapshot taken earlier,
	// possibly by an older version of the mock.
	Restore(data []byte) error
}

// StateFile persists one service's store as <dir>/<service>.json (the
// --state-dir flag), so a restarted mock comes back with the resources it had.
// A nil *StateFile is valid and does nothing, which is what OpenState returns
// when persistence is not configured.
//
// The state is a convenience for local work, not a storage format: nothing
// keeps it compatible across sakumock versions. The file records the version
// that wrote it only so a restore by another version can say so.
type StateFile struct {
	path   string
	store  Snapshotter
	logger *slog.Logger

	mu   sync.Mutex
	last []byte // the snapshot currently on disk, compacted
}

// OpenState restores store from <dir>/<service>.json when that file exists and
// returns the StateFile that keeps it up to date. An empty dir disables
// persistence and returns nil. A state file that cannot be read or restored is
// an error rather than a silent fresh start, because the next save would
// overwrite it. One saved by another sakumock version is restored as it is,
// with a warning: it usually works, and when it does not the user knows why.
func OpenState(dir, service string, store Snapshotter, logger *slog.Logger) (*StateFile, error) {
	if dir == "" {
		return nil, nil
	}
	if logger == nil {
		logger = slog.Default()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("state dir: %w", err)
	}
	f := &StateFile{
		path:   filepath.Join(dir, service+".json"),
		store:  store,
		logger: logger,
	}
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return f, nil
	}
	if err != nil {
		return nil, fmt.Errorf("state file: %w", err)
	}
	var env stateEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("state file %s: %w (remove it to start empty)", f.path, err)
	}
	if len(env.State) == 0 || bytes.Equal(bytes.TrimSpace(env.State), []byte("null")) {
		return nil, fmt.Errorf("state file %s: not a sakumock state file (remove it to start empty)", f.path)
	}
	if err := store.Restore(env.State); err != nil {
		if env.Version != stateVersion {
			return nil, fmt.Errorf("state file %s was saved by sakumock %q and cannot be restored by %s: %w (remove it to start empty)", f.path, env.Version, stateVersion, err)
		}
		return nil, fmt.Errorf("state file %s: %w (remove it to start empty)", f.path, err)
	}
	if env.Version != stateVersion {
		logger.Warn("state was saved by a different sakumock version; remove the file if anything misbehaves",
			"path", f.path, "saved_by", env.Version, "running", stateVersion)
	}
	f.last = compactJSON(env.State)
	logger.Info("state restored", "path", f.path)
	return f, nil
}

// compactJSON strips insignificant whitespace, so a snapshot compares equal to
// itself however the file indented it. Invalid JSON is returned unchanged.
func compactJSON(data []byte) []byte {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return data
	}
	return buf.Bytes()
}

// Path is the file the state is saved to, or "" when persistence is disabled.
func (f *StateFile) Path() string {
	if f == nil {
		return ""
	}
	return f.path
}

// Save writes the store's current state, unless it is what the file already
// holds. The file is replaced atomically, so a crash mid-write leaves the
// previous state intact.
func (f *StateFile) Save() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.saveLocked()
}

func (f *StateFile) saveLocked() error {
	snapshot, err := f.store.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot state: %w", err)
	}
	compact := compactJSON(snapshot)
	if bytes.Equal(compact, f.last) {
		return nil
	}
	data, err := json.MarshalIndent(stateEnvelope{Version: stateVersion, State: snapshot}, "", "  ")
	if err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	if err := writeFileAtomic(f.path, append(data, '\n')); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	f.last = compact
	f.logger.Debug("state saved", "path", f.path, "bytes", len(data))
	return nil
}

// SaveAfter saves the state once a request that may have changed it has been
// handled; call it at the end of Server.ServeHTTP. Safe methods are skipped,
// and a failure is logged rather than returned since the response is already
// written.
func (f *StateFile) SaveAfter(r *http.Request) {
	if f == nil || isSafeMethod(r.Method) {
		return
	}
	if err := f.Save(); err != nil {
		f.logger.Error("failed to save state", "path", f.path, "error", err)
	}
}

// Serve runs next for r and saves the state afterwards, like SaveAfter, but
// holds the lock snapshots are taken under while a request that may change the
// state is being handled. Such requests are thereby handled one at a time and
// never overlap a snapshot. It is for services whose handlers change stored
// records in place, outside the store's own locks; next must not call Save.
func (f *StateFile) Serve(w http.ResponseWriter, r *http.Request, next http.Handler) {
	if f == nil || isSafeMethod(r.Method) {
		next.ServeHTTP(w, r)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	next.ServeHTTP(w, r)
	if err := f.saveLocked(); err != nil {
		f.logger.Error("failed to save state", "path", f.path, "error", err)
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

// writeFileAtomic writes data to a temporary file next to path and renames it
// over path. The file is private to the user (0600, where the OS has such
// permissions): a state file can hold secrets such as key material.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
