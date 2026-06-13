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
	"syscall"
	"time"
)

// versitygwBinary is the external S3 gateway sakumock launches for the data
// plane. It is looked up on PATH; sakumock never bundles it (it would bloat the
// released single binary and the distroless image), so the data plane is a
// local development / test convenience that the user opts into with
// --enable-data-plane and must have installed.
const versitygwBinary = "versitygw"

// dataPlane manages an external versitygw process that serves the S3-compatible
// data plane backed by a local POSIX directory.
//
// The integration is intentionally loose: sakumock only mirrors bucket
// existence into the backend (one directory per bucket, which versitygw's posix
// backend exposes as an S3 bucket), and versitygw authenticates with a single
// fixed root credential. Control-plane access keys and permissions are NOT
// enforced on the data plane.
type dataPlane struct {
	cancel  context.CancelFunc
	done    chan struct{}
	dir     string
	tempDir bool
	addr    string
	logger  *slog.Logger
}

// startDataPlane launches versitygw with a POSIX backend. It returns (nil, nil)
// when versitygw is not on PATH, so the data plane simply stays off without
// failing the control plane — the same best-effort policy as the terraform e2e
// skipping when terraform is absent. The same nil result is returned when the
// process starts but never begins listening, so a broken data plane never takes
// the control plane down.
func startDataPlane(cfg Config, logger *slog.Logger) (*dataPlane, error) {
	path, err := exec.LookPath(versitygwBinary)
	if err != nil {
		logger.Warn("data plane requested but versitygw not found in PATH; data plane disabled. Install versitygw (https://github.com/versity/versitygw) to enable it",
			"binary", versitygwBinary)
		return nil, nil
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

	// exec.CommandContext + Cancel/WaitDelay gives a graceful SIGTERM with a
	// hard-kill fallback when Close cancels the context.
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, path,
		"--access", cfg.DataPlaneAccessKey,
		"--secret", cfg.DataPlaneSecretKey,
		"--region", cfg.DataPlaneRegion,
		"--port", cfg.DataPlaneAddr,
		"posix", dir,
	)
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = 5 * time.Second
	lw := &logWriter{logger: logger}
	cmd.Stdout, cmd.Stderr = lw, lw

	if err := cmd.Start(); err != nil {
		cancel()
		if tempDir {
			_ = os.RemoveAll(dir)
		}
		return nil, fmt.Errorf("start versitygw: %w", err)
	}

	d := &dataPlane{
		cancel:  cancel,
		done:    make(chan struct{}),
		dir:     dir,
		tempDir: tempDir,
		addr:    cfg.DataPlaneAddr,
		logger:  logger,
	}
	go func() {
		defer close(d.done)
		if err := cmd.Wait(); err != nil && ctx.Err() == nil {
			logger.Error("versitygw exited unexpectedly", "error", err)
		}
	}()

	if err := waitListen(cfg.DataPlaneAddr, 10*time.Second); err != nil {
		logger.Error("versitygw did not start listening; data plane disabled", "addr", cfg.DataPlaneAddr, "error", err)
		d.Close()
		return nil, nil
	}

	logger.Info("data plane (S3) started",
		"binary", path,
		"addr", cfg.DataPlaneAddr,
		"dir", dir,
		"region", cfg.DataPlaneRegion,
		"access_key", cfg.DataPlaneAccessKey,
	)
	return d, nil
}

// createBucket mirrors a control-plane bucket into the data plane backend as a
// directory, which versitygw's posix backend exposes as an S3 bucket. It is a
// no-op on a nil receiver, so callers need not check whether the data plane is
// enabled.
func (d *dataPlane) createBucket(name string) {
	if d == nil {
		return
	}
	if err := os.Mkdir(filepath.Join(d.dir, name), 0o755); err != nil && !os.IsExist(err) {
		d.logger.Warn("data plane: failed to create bucket directory", "bucket", name, "error", err)
	}
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
	}
}

// logWriter forwards a child process's stdout/stderr to slog at debug level,
// one line per log entry.
type logWriter struct {
	logger *slog.Logger
}

func (w *logWriter) Write(p []byte) (int, error) {
	sc := bufio.NewScanner(bytes.NewReader(p))
	for sc.Scan() {
		if line := sc.Text(); line != "" {
			w.logger.Debug("versitygw", "log", line)
		}
	}
	return len(p), nil
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
