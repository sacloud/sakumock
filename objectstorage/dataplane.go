package objectstorage

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// versitygwBinary is the external S3 gateway sakumock launches for the data
// plane. It is looked up on PATH (exec.LookPath, which on Windows resolves
// versitygw.exe via PATHEXT); sakumock never bundles it (it would bloat the
// released single binary and the distroless image), so the data plane is a
// local development / test convenience that the user opts into with
// --enable-data-plane and must have installed.
const versitygwBinary = "versitygw"

// Fixed root credentials the bundled data plane (versitygw) accepts. sakumock is
// a local mock, so these are well-known development defaults rather than real
// secrets — they are intentionally not configurable, mirroring the fixed dummy
// SAKURA credentials core.DummyCredentialEnv emits.
const (
	dataPlaneRootID    = "sakumock"
	dataPlaneRootValue = "sakumocksecret"
)

// dataPlane manages an external versitygw process that serves the S3-compatible
// data plane backed by a local POSIX directory.
//
// The integration is intentionally loose: sakumock mirrors bucket existence
// into the backend (one directory per bucket, which versitygw's posix backend
// exposes as an S3 bucket) and mirrors control-plane access keys into
// versitygw's internal IAM service (see dataplane_iam.go), so both the fixed
// root credential and issued keys authenticate. Permissions (per-bucket
// controls) are NOT enforced on the data plane.
type dataPlane struct {
	cancel  context.CancelFunc
	done    chan struct{}
	dir     string
	tempDir bool
	addr    string
	logger  *slog.Logger

	// iamDir holds versitygw's internal IAM database (users.json), where
	// control-plane access keys are mirrored. Always a temp dir removed on
	// Close; see startDataPlane for why it is never persisted.
	iamDir string
	iamMu  sync.Mutex
}

// startDataPlane launches versitygw with a POSIX backend. It returns an error
// when versitygw is not on PATH or never starts listening: --enable-data-plane
// is an explicit opt-in, so silently running without the data plane the user
// asked for would be a trap. (This differs from the terraform e2e skipping when
// terraform is absent, which is optional test tooling rather than a requested
// feature.)
func startDataPlane(cfg Config, logger *slog.Logger) (*dataPlane, error) {
	path, err := exec.LookPath(versitygwBinary)
	if err != nil {
		return nil, fmt.Errorf("data plane enabled but %s not found in PATH; install it (https://github.com/versity/versitygw) or omit --enable-data-plane", versitygwBinary)
	}

	dir := cfg.DataPlaneDir
	tempDir := false
	if dir == "" {
		dir, err = os.MkdirTemp("", "sakumock-objectstorage-")
		if err != nil {
			return nil, fmt.Errorf("create data plane dir: %w", err)
		}
		tempDir = true
	} else if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create data plane dir %q: %w", dir, err)
	}

	// The IAM dir is always a fresh temp dir, even when the backend dir is
	// user-configured and persistent: the control-plane key store is in-memory
	// and resets with the process, so persisting users.json would leave stale
	// keys authenticating after a restart.
	iamDir, err := os.MkdirTemp("", "sakumock-objectstorage-iam-")
	if err != nil {
		if tempDir {
			_ = os.RemoveAll(dir)
		}
		return nil, fmt.Errorf("create data plane IAM dir: %w", err)
	}
	cleanupDirs := func() {
		if tempDir {
			_ = os.RemoveAll(dir)
		}
		_ = os.RemoveAll(iamDir)
	}

	// exec.CommandContext + Cancel/WaitDelay gives a graceful stop
	// (terminateProcess, per-OS) with a hard-kill fallback when Close cancels
	// the context.
	ctx, cancel := context.WithCancel(context.Background())
	// --iam-dir enables versitygw's internal IAM service so control-plane
	// access keys mirrored into it (dataplane_iam.go) authenticate;
	// --iam-cache-disable makes key deletion take effect immediately instead
	// of after the cache TTL (the cache only fronts a local file read here).
	args := []string{
		"--access", dataPlaneRootID,
		"--secret", dataPlaneRootValue,
		"--region", cfg.DataPlaneRegion,
		"--port", cfg.DataPlaneAddr,
		"--iam-dir", iamDir,
		"--iam-cache-disable",
	}
	// versitygw serves the data plane over TLS itself when given a cert/key, so
	// the common TLS files are passed through rather than terminated by sakumock.
	if cfg.tls.Enabled() {
		args = append(args, "--cert", cfg.tls.CertFile, "--key", cfg.tls.KeyFile)
	}
	metaArgs, err := posixMetadataArgs(dir)
	if err != nil {
		cancel()
		cleanupDirs()
		return nil, err
	}
	args = append(args, "posix")
	args = append(args, metaArgs...)
	args = append(args, dir)
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Cancel = func() error { return terminateProcess(cmd.Process) }
	cmd.WaitDelay = 5 * time.Second
	lw := &logWriter{logger: logger}
	cmd.Stdout, cmd.Stderr = lw, lw

	if err := cmd.Start(); err != nil {
		cancel()
		cleanupDirs()
		return nil, fmt.Errorf("start versitygw: %w", err)
	}

	d := &dataPlane{
		cancel:  cancel,
		done:    make(chan struct{}),
		dir:     dir,
		tempDir: tempDir,
		addr:    cfg.DataPlaneAddr,
		logger:  logger,
		iamDir:  iamDir,
	}
	go func() {
		defer close(d.done)
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			logger.Error("versitygw exited unexpectedly", "error", err, "output", lw.tail())
		}
	}()

	if err := waitListen(cfg.DataPlaneAddr, 10*time.Second); err != nil {
		d.Close()
		return nil, fmt.Errorf("data plane: versitygw did not start listening on %s: %w (recent output: %s)", cfg.DataPlaneAddr, err, lw.tail())
	}

	logger.Info("data plane (S3) started",
		"binary", path,
		"addr", cfg.DataPlaneAddr,
		"scheme", cfg.tls.Scheme(),
		"dir", dir,
		"region", cfg.DataPlaneRegion,
		"access_key", dataPlaneRootID,
	)
	return d, nil
}

// createBucket mirrors a control-plane bucket into the data plane backend as a
// directory, which versitygw's posix backend exposes as an S3 bucket. It is a
// no-op on a nil receiver, so callers need not check whether the data plane is
// enabled. A failure is returned rather than just logged so the control plane
// never claims a bucket the data plane cannot serve (e.g. a name the local
// filesystem rejects, such as a Windows reserved device name like "con").
func (d *dataPlane) createBucket(name string) error {
	if d == nil {
		return nil
	}
	if err := os.Mkdir(filepath.Join(d.dir, name), 0o755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("data plane: create bucket directory: %w", err)
	}
	return nil
}

// deleteBucket removes a bucket (and its objects) from the data plane backend.
func (d *dataPlane) deleteBucket(name string) {
	if d == nil {
		return
	}
	if err := os.RemoveAll(filepath.Join(d.dir, name)); err != nil {
		d.logger.Warn("data plane: failed to remove bucket directory", "bucket", name, "error", err)
	}
}

// Close stops the versitygw process and removes the backend directory if it was
// a temporary one created by sakumock.
func (d *dataPlane) Close() {
	if d == nil {
		return
	}
	d.cancel()
	<-d.done
	if d.tempDir {
		_ = os.RemoveAll(d.dir)
		// Windows keeps sidecar metadata next to the backend dir (see
		// posixMetadataArgs); a no-op elsewhere.
		_ = os.RemoveAll(sidecarDir(d.dir))
	}
	// The IAM dir is always temporary (see startDataPlane).
	if d.iamDir != "" {
		_ = os.RemoveAll(d.iamDir)
	}
}

// sidecarDir is where posixMetadataArgs places versitygw's sidecar metadata on
// platforms without xattr support: a sibling of the backend dir (inside it,
// versitygw's posix backend would list it as a bucket). The name prepends
// "meta-" rather than appending a suffix because versitygw rejects any sidecar
// path that string-prefix-matches the root dir — a naive containment check
// that a "<dir>.meta" sibling would trip.
func sidecarDir(dir string) string {
	return filepath.Join(filepath.Dir(dir), "meta-"+filepath.Base(dir))
}

// logWriter forwards a child process's stdout/stderr to slog at debug level,
// one line per log entry, and keeps the most recent lines so a startup or
// crash report can show why the process failed even when debug logging is off.
type logWriter struct {
	logger *slog.Logger

	mu   sync.Mutex
	last []string
}

// logWriterTailLines bounds the retained output.
const logWriterTailLines = 20

func (w *logWriter) Write(p []byte) (int, error) {
	sc := bufio.NewScanner(bytes.NewReader(p))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		w.logger.Debug("versitygw", "log", line)
		w.mu.Lock()
		w.last = append(w.last, line)
		if len(w.last) > logWriterTailLines {
			w.last = w.last[1:]
		}
		w.mu.Unlock()
	}
	return len(p), nil
}

// tail returns the most recent output lines as one string.
func (w *logWriter) tail() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return strings.Join(w.last, "\n")
}

// waitListen blocks until addr accepts a TCP connection or the timeout elapses.
func waitListen(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
			_ = c.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for %s to listen", addr)
}
