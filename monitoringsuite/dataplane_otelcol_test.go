//go:build otelcollector

// This end-to-end test drives the real sacloud-otel-collector binary as the
// telemetry agent, configured with its SAKURA-specific `sacloud` exporter, and
// uses telemetrygen as the source. telemetrygen sends OTLP to the collector,
// whose sacloud exporter forwards metrics as Prometheus remote-write and
// logs/traces as OTLP/HTTP (gzip) to the mock's data plane. This validates the
// data plane against the real agent's wire output.
//
// sacloud-otel-collector v0.7.0+ accepts http endpoints, so the mock data plane
// is served over plain HTTP (no cert needed).
//
// It is behind the `otelcollector` build tag (normal `go test ./...` skips it)
// and skips unless both `sacloud-otel-collector` and `telemetrygen` are on PATH
// (the CI job installs them; install them locally to run it). Run with:
//
//	go test -tags otelcollector ./monitoringsuite/

package monitoringsuite_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sacloud/sakumock/monitoringsuite"
)

func TestDataPlaneOtelCollectorE2E(t *testing.T) {
	for _, bin := range []string{"sacloud-otel-collector", "telemetrygen"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not found in PATH; skipping otel-collector e2e", bin)
		}
	}

	dumpDir := t.TempDir()
	// The collector's OTLP receiver is an external process, so its port must be
	// picked before the collector starts. The mock data plane, by contrast, can
	// bind :0 and report its real address.
	otlpAddr := freeLoopbackAddr(t) // collector OTLP/HTTP receiver (telemetrygen sends here)

	srv := monitoringsuite.NewTestServer(monitoringsuite.Config{
		EnableDataPlane:  true,
		DataPlaneAddr:    "127.0.0.1:0",
		DataPlaneDumpDir: dumpDir,
	})
	defer srv.Close()
	dpAddr := srv.DataPlaneAddr() // mock data plane (collector exports here)

	// The collector's `sacloud` exporter forwards metrics (remote-write) and
	// logs/traces (OTLP/HTTP) to the mock over plain HTTP.
	cfg := fmt.Sprintf(`
receivers:
  otlp:
    protocols:
      http:
        endpoint: %[1]s
exporters:
  sacloud:
    metrics:
      endpoint: http://%[2]s/prometheus/api/v1/write
      token: met-dummy
    logs:
      endpoint: http://%[2]s
      token: log-dummy
    traces:
      endpoint: http://%[2]s
      token: trc-dummy
service:
  telemetry:
    metrics:
      level: none
  pipelines:
    metrics:
      receivers: [otlp]
      exporters: [sacloud]
    logs:
      receivers: [otlp]
      exporters: [sacloud]
    traces:
      receivers: [otlp]
      exporters: [sacloud]
`, otlpAddr, dpAddr)
	cfgFile := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(cfgFile, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	logFile := startCollector(t, cfgFile)

	waitListen(t, otlpAddr, 60*time.Second)
	time.Sleep(2 * time.Second) // let the pipelines settle after the port opens

	for _, signal := range []string{"metrics", "logs", "traces"} {
		sendTelemetry(t, signal, otlpAddr)
	}

	for _, prefix := range []string{"metrics-remotewrite-", "otlp-logs-", "otlp-traces-"} {
		if !waitForDump(dumpDir, prefix, 30*time.Second) {
			t.Errorf("no %s* dump produced; collector log:\n%s", prefix, readFile(logFile))
		}
	}
}

// startCollector runs sacloud-otel-collector as a subprocess that is killed on
// test cleanup. It returns the path of the file capturing the collector output.
func startCollector(t *testing.T, cfgFile string) string {
	t.Helper()
	logf, err := os.CreateTemp(t.TempDir(), "otelcol-*.log")
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sacloud-otel-collector", "--config", cfgFile)
	cmd.Stdout, cmd.Stderr = logf, logf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start collector: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = logf.Close()
	})
	return logf.Name()
}

// sendTelemetry runs telemetrygen to send one item of the given signal to the
// collector's OTLP/HTTP receiver.
func sendTelemetry(t *testing.T, signal, otlpAddr string) {
	t.Helper()
	out, err := exec.Command("telemetrygen", signal,
		"--otlp-endpoint", otlpAddr,
		"--otlp-http",
		"--otlp-insecure",
		"--"+signal, "1",
	).CombinedOutput()
	if err != nil {
		t.Fatalf("telemetrygen %s: %v\n%s", signal, err, out)
	}
}

func readFile(path string) string {
	b, _ := os.ReadFile(path)
	return string(b)
}

// ---- addr / wait helpers ----

// freeLoopbackAddr reserves a loopback port by binding and immediately closing a
// listener. It is inherently racy (the port can be taken before the caller
// rebinds), so it is used only for the collector's OTLP receiver, whose port —
// being an external process — must be known before the collector starts. The
// mock data plane instead binds :0 and reports its real address.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func waitListen(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to listen", addr)
}

func waitForDump(dir, prefix string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), prefix) {
				return true
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}
