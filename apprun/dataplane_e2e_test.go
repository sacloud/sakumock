package apprun_test

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sacloud/sakumock/apprun"
)

// TestDataPlaneEndToEnd exercises the full apprun data plane path:
// deploy an nginx container via apprun-cli, request the public URL,
// verify a 200 response, and delete the application.
// Requires Docker and apprun-cli on PATH; skips otherwise.
func TestDataPlaneEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH; skipping data plane e2e test")
	}
	if _, err := exec.LookPath("apprun-cli"); err != nil {
		t.Skip("apprun-cli not found in PATH; skipping data plane e2e test")
	}

	dpAddr := freeAddr(t)

	srv := apprun.NewTestServer(apprun.Config{
		EnableDataPlane: true,
		DataPlaneAddr:   dpAddr,
	})
	defer srv.Close()

	appDef := map[string]any{
		"name":            "e2e-nginx",
		"timeout_seconds": 30,
		"port":            80,
		"min_scale":       0,
		"max_scale":       1,
		"components": []map[string]any{
			{
				"name":       "web",
				"max_cpu":    "0.5",
				"max_memory": "1Gi",
				"deploy_source": map[string]any{
					"container_registry": map[string]any{
						"image": "nginx:latest",
					},
				},
			},
		},
	}
	appJSON, err := json.Marshal(appDef)
	if err != nil {
		t.Fatal(err)
	}
	appFile := filepath.Join(t.TempDir(), "app.json")
	if err := os.WriteFile(appFile, appJSON, 0644); err != nil {
		t.Fatal(err)
	}

	env := []string{
		"SAKURA_ENDPOINTS_APPRUN_SHARED=" + srv.TestURL(),
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
		"APPRUN_CLI_APP=" + appFile,
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}

	// Deploy
	deploy := exec.Command("apprun-cli", "deploy")
	deploy.Env = env
	if out, err := deploy.CombinedOutput(); err != nil {
		t.Fatalf("apprun-cli deploy failed: %v\n%s", err, out)
	}

	// Get the public URL
	urlCmd := exec.Command("apprun-cli", "url")
	urlCmd.Env = env
	urlOut, err := urlCmd.Output()
	if err != nil {
		t.Fatalf("apprun-cli url failed: %v", err)
	}
	publicURL := strings.TrimSpace(string(urlOut))
	if publicURL == "" {
		t.Fatal("apprun-cli url returned empty")
	}
	t.Logf("public URL: %s", publicURL)

	// Wait for the container to be ready, then request.
	client := &http.Client{Timeout: 30 * time.Second}
	var resp *http.Response
	for i := range 30 {
		resp, err = client.Get(publicURL)
		if err == nil {
			break
		}
		t.Logf("attempt %d: %v", i+1, err)
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("failed to reach %s after retries: %v", publicURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from nginx, got %d", resp.StatusCode)
	}
	t.Logf("nginx responded with %d", resp.StatusCode)

	// Delete
	del := exec.Command("apprun-cli", "delete", "--force")
	del.Env = env
	if out, err := del.CombinedOutput(); err != nil {
		t.Fatalf("apprun-cli delete failed: %v\n%s", err, out)
	}
}

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
