//go:build terraform

package terraform

import (
	"bufio"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestTerraformEndToEnd starts the real `sakumock all` binary and runs a full
// terraform apply / plan(no-diff) / destroy against it through the
// sacloud/sakura provider, covering one resource per mocked service.
func TestTerraformEndToEnd(t *testing.T) {
	tfBin, err := exec.LookPath("terraform")
	if err != nil {
		t.Skip("terraform binary not found in PATH; skipping end-to-end test")
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	// Build the unified binary from current source (go.work picks up local changes).
	binDir := t.TempDir()
	sakumockBin := filepath.Join(binDir, "sakumock")
	build := exec.Command("go", "build", "-o", sakumockBin, "./cmd/sakumock")
	build.Dir = repoRoot
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := build.Run(); err != nil {
		t.Fatalf("build sakumock: %v", err)
	}

	// Start `sakumock all` on dynamically allocated free ports rather than the
	// fixed defaults, so the test never collides with — or accidentally talks
	// to — a process already listening on those ports. The chosen address for
	// each service is passed via its prefixed --<service>-addr flag.
	addrs := []string{freeAddr(t), freeAddr(t), freeAddr(t), freeAddr(t), freeAddr(t), freeAddr(t)}
	addrFlags := []string{
		"--simplemq-addr", addrs[0],
		"--kms-addr", addrs[1],
		"--secretmanager-addr", addrs[2],
		"--simplenotification-addr", addrs[3],
		"--monitoringsuite-addr", addrs[4],
		"--eventbus-addr", addrs[5],
	}

	// Write the client dotenv with the `env` subcommand (no server needed); the
	// endpoints point at the same dynamic addresses passed to `all` below.
	envFile := filepath.Join(binDir, "sakumock.env")
	genEnv := exec.Command(sakumockBin, append([]string{"env", "--output", envFile}, addrFlags...)...)
	genEnv.Stdout, genEnv.Stderr = os.Stdout, os.Stderr
	if err := genEnv.Run(); err != nil {
		t.Fatalf("sakumock env: %v", err)
	}

	srv := exec.Command(sakumockBin, append([]string{"all"}, addrFlags...)...)
	srv.Stdout, srv.Stderr = os.Stdout, os.Stderr
	if err := srv.Start(); err != nil {
		t.Fatalf("start sakumock all: %v", err)
	}
	t.Cleanup(func() { stopProcess(srv) })

	for _, addr := range addrs {
		waitPort(t, addr)
	}

	// Child env: drop any real SAKURA_* from the environment, then load the
	// dotenv the mock wrote (endpoints + dummy credentials) and set the zone.
	env := make([]string, 0, len(os.Environ()))
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "SAKURA_") {
			continue
		}
		env = append(env, kv)
	}
	env = append(env, readEnvFile(t, envFile)...)
	env = append(env, "SAKURA_ZONE=tk1v") // tk1v is SAKURA Cloud's sandbox zone

	// Run terraform in an isolated working dir holding a copy of the fixture.
	work := t.TempDir()
	copyFile(t, filepath.Join(repoRoot, "test", "terraform", "main.tf"), filepath.Join(work, "main.tf"))

	runTF := func(args ...string) {
		t.Helper()
		cmd := exec.Command(tfBin, args...)
		cmd.Dir = work
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("terraform %s: %v", strings.Join(args, " "), err)
		}
	}

	t.Cleanup(func() {
		cmd := exec.Command(tfBin, "destroy", "-auto-approve", "-no-color", "-input=false")
		cmd.Dir = work
		cmd.Env = env
		_ = cmd.Run()
	})

	runTF("init", "-no-color", "-input=false")
	runTF("apply", "-auto-approve", "-no-color", "-input=false")

	// A plan right after apply must show no changes (exit 0). Exit code 2 means
	// a diff — a mock that did not round-trip a field the provider reads back.
	plan := exec.Command(tfBin, "plan", "-detailed-exitcode", "-no-color", "-input=false")
	plan.Dir = work
	plan.Env = env
	plan.Stdout, plan.Stderr = os.Stdout, os.Stderr
	if err := plan.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 2 {
			t.Fatal("terraform plan after apply shows a diff: a mock is not round-tripping a field")
		}
		t.Fatalf("terraform plan: %v", err)
	}

	runTF("destroy", "-auto-approve", "-no-color", "-input=false")
}

// freeAddr returns a currently-free loopback address. There is a small window
// between closing the listener and sakumock binding it, which is acceptable for
// a test and far less likely than a clash on a fixed port.
func freeAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate free port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// stopProcess sends SIGTERM and waits for the process to exit, force-killing it
// if it does not stop in time so a stuck child can never hang the test.
func stopProcess(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func waitPort(t *testing.T, addr string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		c, err := net.DialTimeout("tcp", addr, time.Second)
		if err == nil {
			_ = c.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s to listen", addr)
}

func readEnvFile(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open env file: %v", err)
	}
	defer f.Close()
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read env file: %v", err)
	}
	return out
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", dst, err)
	}
}
