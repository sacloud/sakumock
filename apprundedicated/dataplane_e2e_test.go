package apprundedicated_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated/apis/v1"

	"github.com/sacloud/sakumock/apprundedicated"
)

// TestDataPlaneEndToEnd exercises the full apprun-dedicated data plane path:
// 1. Start mock with data plane enabled
// 2. Create cluster and ASG via SDK (infrastructure)
// 3. Deploy nginx via apprun-dedicated-cli (deploy --wait)
// 4. Request the data plane URL and verify 200
// 5. Delete the application via apprun-dedicated-cli (delete --force)
// Requires Docker and apprun-dedicated-cli on PATH; skips otherwise.
func TestDataPlaneEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH; skipping data plane e2e test")
	}
	if _, err := exec.LookPath("apprun-dedicated-cli"); err != nil {
		t.Skip("apprun-dedicated-cli not found in PATH; skipping data plane e2e test")
	}

	dpAddr := freeAddr(t)

	srv := apprundedicated.NewTestServer(apprundedicated.Config{
		EnableDataPlane: true,
		DataPlaneAddr:   dpAddr,
	})
	defer srv.Close()

	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	// Create infrastructure: cluster + ASG (which auto-creates worker nodes)
	clusterID := createTestCluster(t, ctx, client)

	_, err := client.CreateAutoScalingGroup(ctx, &v1.CreateAutoScalingGroup{
		Name:                   "e2e-asg",
		Zone:                   "is1a",
		WorkerServiceClassPath: "cloud/apprun/dedicated/worker/1vcpu_2gb",
		MinNodes:               1,
		MaxNodes:               1,
		NameServers:            []v1.IPv4{"210.188.224.10"},
		Interfaces: []v1.AutoScalingGroupNodeInterface{
			{InterfaceIndex: 0, Upstream: "shared"},
		},
	}, v1.CreateAutoScalingGroupParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("CreateAutoScalingGroup: %v", err)
	}

	// Write application definition for apprun-dedicated-cli
	appDef := map[string]any{
		"cluster":     "test-cluster",
		"name":        "e2e-nginx",
		"cpu":         1000,
		"memory":      2048,
		"scalingMode": "manual",
		"fixedScale":  1,
		"image":       "nginx:latest",
		"exposedPorts": []map[string]any{
			{
				"targetPort": 80,
				"healthCheck": map[string]any{
					"path":            "/",
					"intervalSeconds": 10,
					"timeoutSeconds":  5,
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
		"SAKURA_ENDPOINTS_APPRUN_DEDICATED=" + srv.TestURL(),
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
		"APPRUN_DEDICATED_APP=" + appFile,
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}

	// Deploy with --wait
	deploy := exec.Command("apprun-dedicated-cli", "deploy", "--wait", "--wait-timeout=60s")
	deploy.Env = env
	deploy.Stdout, deploy.Stderr = os.Stdout, os.Stderr
	if err := deploy.Run(); err != nil {
		t.Fatalf("apprun-dedicated-cli deploy failed: %v", err)
	}

	// Find the application ID to construct the data plane URL.
	// List applications and find "e2e-nginx".
	listResp, err := client.ListApplications(ctx, v1.ListApplicationsParams{})
	if err != nil {
		t.Fatalf("ListApplications: %v", err)
	}
	var appID string
	for _, a := range listResp.Applications {
		if a.GetName() == "e2e-nginx" {
			appID = uuid.UUID(a.GetApplicationID()).String()
			break
		}
	}
	if appID == "" {
		t.Fatal("application e2e-nginx not found after deploy")
	}
	t.Logf("application ID: %s", appID)

	// Access the data plane: http://{appID}.localhost:{port}
	dataPlaneURL := "http://" + appID + ".localhost:" + portOfAddr(dpAddr)
	t.Logf("data plane URL: %s", dataPlaneURL)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	var resp *http.Response
	for i := range 30 {
		resp, err = httpClient.Get(dataPlaneURL)
		if err == nil {
			break
		}
		t.Logf("attempt %d: %v", i+1, err)
		time.Sleep(time.Second)
	}
	if err != nil {
		t.Fatalf("failed to reach %s after retries: %v", dataPlaneURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from nginx, got %d", resp.StatusCode)
	}
	t.Logf("nginx responded with %d", resp.StatusCode)

	// Delete
	del := exec.Command("apprun-dedicated-cli", "delete", "--force", "--wait-timeout=60s")
	del.Env = env
	del.Stdout, del.Stderr = os.Stdout, os.Stderr
	if err := del.Run(); err != nil {
		t.Fatalf("apprun-dedicated-cli delete failed: %v", err)
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

func portOfAddr(addr string) string {
	_, port, _ := net.SplitHostPort(addr)
	if port == "" {
		return addr
	}
	return port
}
