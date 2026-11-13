package core

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
)

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
type StateFile struct {
	path   string
	store  Snapshotter
	logger *slog.Logger

	mu   sync.Mutex
	last []byte // the snapshot currently on disk
}

// OpenState restores store from <dir>/<service>.json when that file exists and
// returns the StateFile that keeps it up to date. An empty dir disables
// persistence and returns nil. A state file that cannot be read or restored is
// an error rather than a silent fresh start, because the next save would
// overwrite it.
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
	if err := store.Restore(data); err != nil {
		return nil, fmt.Errorf("state file %s: %w (remove it to start empty)", f.path, err)
	}
	f.last = data
	logger.Info("state restored", "path", f.path)
	return f, nil
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

	data, err := f.store.Snapshot()
	if err != nil {
		return fmt.Errorf("snapshot state: %w", err)
	}
	if bytes.Equal(data, f.last) {
		return nil
	}
	if err := writeFileAtomic(f.path, data); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	f.last = data
	f.logger.Debug("state saved", "path", f.path, "bytes", len(data))
	return nil
}

// SaveAfter saves the state once a request that may have changed it has been
// handled; call it at the end of Server.ServeHTTP. Safe methods are skipped,
// and a failure is logged rather than returned since the response is already
// written.
func (f *StateFile) SaveAfter(r *http.Request) {
	if f == nil {
		return
	}
	switch r.Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return
	}
	if err := f.Save(); err != nil {
		f.logger.Error("failed to save state", "path", f.path, "error", err)
	}
}

// writeFileAtomic writes data to a temporary file next to path and renames it
// over path. The file is private to the user (0600): a state file can hold
// secrets such as key material.
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
