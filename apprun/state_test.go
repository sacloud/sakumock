package apprun_test

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"
	"time"

	apprunsdk "github.com/sacloud/sacloud-sdk-go/api/apprun"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apprun/apis/v1"

	"github.com/sacloud/sakumock/apprun"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/core/coretest"
)

func TestStateSurvivesRestart(t *testing.T) {
	var (
		created     *v1.HandlerCreateApplication
		versionName string
	)

	coretest.StateRestart(t, apprun.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			appOp := apprunsdk.NewApplicationOp(client)
			if _, err := apprunsdk.NewUserOp(client).Create(ctx); err != nil {
				t.Fatal(err)
			}
			created = createTestApp(ctx, t, appOp)
			// A second and a third version; the second one is deleted.
			for _, timeout := range []int{120, 180} {
				if _, err := appOp.Update(ctx, created.ID, &v1.PatchApplicationBody{TimeoutSeconds: v1.NewOptInt(timeout)}); err != nil {
					t.Fatal(err)
				}
			}
			versionOp := apprunsdk.NewVersionOp(client)
			versions, err := versionOp.List(ctx, created.ID, &v1.ListApplicationVersionsParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(versions.Data) != 3 {
				t.Fatalf("got %d versions, want 3", len(versions.Data))
			}
			if err := versionOp.Delete(ctx, created.ID, versions.Data[1].ID); err != nil {
				t.Fatal(err)
			}
			versionName = versions.Data[2].Name
			traffic := v1.UpdateTrafficBody{
				v1.NewUpdateTrafficBodyItem0UpdateTrafficBodyItem(v1.UpdateTrafficBodyItem0{IsLatestVersion: true, Percent: 70}),
				v1.NewUpdateTrafficBodyItem1UpdateTrafficBodyItem(v1.UpdateTrafficBodyItem1{VersionName: versionName, Percent: 30}),
			}
			if _, err := apprunsdk.NewTrafficOp(client).Update(ctx, created.ID, &traffic); err != nil {
				t.Fatal(err)
			}
			if _, err := apprunsdk.NewPacketFilterOp(client).Update(ctx, created.ID, &v1.PatchPacketFilterBody{
				IsEnabled: v1.NewOptBool(true),
				Settings:  []v1.PatchPacketFilterBodySettingsItem{{FromIP: "192.168.1.0", FromIPPrefixLength: 24}},
			}); err != nil {
				t.Fatal(err)
			}
			// A deleted application does not come back.
			deleted := createTestApp(ctx, t, appOp)
			if err := appOp.Delete(ctx, deleted.ID); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			appOp := apprunsdk.NewApplicationOp(client)
			if _, err := apprunsdk.NewUserOp(client).Read(ctx); err != nil {
				t.Errorf("user not restored: %v", err)
			}
			list, err := appOp.List(ctx, &v1.ListApplicationsParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Data) != 1 {
				t.Fatalf("got %d applications after restart, want 1", len(list.Data))
			}
			app, err := appOp.Read(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if app.Name != created.Name || app.TimeoutSeconds != 180 || app.ResourceID != created.ResourceID ||
				app.PublicURL != created.PublicURL || app.CreatedAt != created.CreatedAt ||
				len(app.Components) != 1 || app.Components[0].Name != "web" {
				t.Errorf("restored application = %+v, created as %+v", app, created)
			}
			status, err := appOp.ReadStatus(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if status.Status != "Healthy" {
				t.Errorf("restored status = %s, want Healthy", status.Status)
			}
			versions, err := apprunsdk.NewVersionOp(client).List(ctx, created.ID, &v1.ListApplicationVersionsParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(versions.Data) != 2 || versions.Data[1].Name != versionName {
				t.Errorf("restored versions = %+v, want the latest and %s", versions.Data, versionName)
			}
			traffic, err := apprunsdk.NewTrafficOp(client).List(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(traffic.Data) != 2 || traffic.Data[0].Percent != 70 || traffic.Data[1].Percent != 30 || traffic.Data[1].VersionName != versionName {
				t.Errorf("restored traffic = %+v", traffic.Data)
			}
			pf, err := apprunsdk.NewPacketFilterOp(client).Read(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if !pf.IsEnabled || len(pf.Settings) != 1 || pf.Settings[0].FromIP != "192.168.1.0" {
				t.Errorf("restored packet filter = %+v", pf)
			}
		})
}

// An application or version created after a restart never reuses a restored
// resource ID or version name, and sorts after the restored ones.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var first *v1.HandlerCreateApplication
	coretest.StateRestart(t, apprun.Config{},
		func(t *testing.T, url string) {
			first = createTestApp(t.Context(), t, apprunsdk.NewApplicationOp(newTestClient(t, url)))
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			appOp := apprunsdk.NewApplicationOp(client)
			second := createTestApp(ctx, t, appOp)
			if second.ResourceID == first.ResourceID {
				t.Errorf("new application reused the restored resource ID %s", second.ResourceID)
			}
			// Newest first, even when both were created within the same second.
			list, err := appOp.List(ctx, &v1.ListApplicationsParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(list.Data) != 2 || list.Data[0].ID != second.ID {
				t.Errorf("list after restart = %+v, want the new application first", list.Data)
			}

			if _, err := appOp.Update(ctx, first.ID, &v1.PatchApplicationBody{TimeoutSeconds: v1.NewOptInt(120)}); err != nil {
				t.Fatal(err)
			}
			versions, err := apprunsdk.NewVersionOp(client).List(ctx, first.ID, &v1.ListApplicationVersionsParams{})
			if err != nil {
				t.Fatal(err)
			}
			if len(versions.Data) != 2 || versions.Data[0].Name == versions.Data[1].Name {
				t.Errorf("versions after restart = %+v, want two distinct names", versions.Data)
			}
		}, true)
}

// With the data plane enabled, a restored application is served again: its
// container is restarted, whether the previous process removed its containers
// on shutdown or was killed and left them behind.
func TestStateRestartsContainers(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not found in PATH; skipping data plane state test")
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skipf("docker daemon not usable (%v); skipping data plane state test", err)
	}

	for _, tt := range []struct {
		name   string
		killed bool
	}{
		{"clean shutdown", false},
		{"killed", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := t.Context()
			dir := t.TempDir()
			start := func() (url, dpAddr string, stop func()) {
				t.Helper()
				dpAddr = freeAddr(t)
				srv, err := apprun.Config{EnableDataPlane: true, DataPlaneAddr: dpAddr}.NewServer(core.ServerOptions{StateDir: dir})
				if err != nil {
					t.Fatal(err)
				}
				ts := httptest.NewServer(srv)
				stop = sync.OnceFunc(func() {
					ts.Close()
					srv.Close()
				})
				t.Cleanup(stop)
				return ts.URL, dpAddr, stop
			}

			url, dpAddr, stop := start()
			created, err := apprunsdk.NewApplicationOp(newTestClient(t, url)).Create(ctx, nginxApplication())
			if err != nil {
				t.Fatal(err)
			}
			expectServed(ctx, t, dpAddr, created.ID)
			// A killed process removes nothing: its container is still there when
			// the next one starts.
			if !tt.killed {
				stop()
			}

			url, dpAddr, _ = start()
			appOp := apprunsdk.NewApplicationOp(newTestClient(t, url))
			status, err := appOp.ReadStatus(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			if status.Status != "Healthy" {
				t.Errorf("status after restart = %s, want Healthy", status.Status)
			}
			expectServed(ctx, t, dpAddr, created.ID)

			// The public URL follows the data plane address of the running process.
			app, err := appOp.Read(ctx, created.ID)
			if err != nil {
				t.Fatal(err)
			}
			_, port, _ := net.SplitHostPort(dpAddr)
			if want := "http://" + created.ID + ".localhost:" + port; app.PublicURL != want {
				t.Errorf("public URL after restart = %s, want %s", app.PublicURL, want)
			}
		})
	}
}

func nginxApplication() *v1.CreateApplicationBody {
	return &v1.CreateApplicationBody{
		Name: "state-nginx", TimeoutSeconds: 30, Port: 80, MinScale: 0, MaxScale: 1,
		Components: []v1.CreateApplicationBodyComponentsItem{{
			Name:      "web",
			MaxCPU:    v1.CreateApplicationBodyComponentsItemMaxCPU05,
			MaxMemory: v1.CreateApplicationBodyComponentsItemMaxMemory1Gi,
			DeploySource: v1.CreateApplicationBodyComponentsItemDeploySource{
				ContainerRegistry: v1.NewOptCreateApplicationBodyComponentsItemDeploySourceContainerRegistry(
					v1.CreateApplicationBodyComponentsItemDeploySourceContainerRegistry{Image: "nginx:latest"},
				),
			},
		}},
	}
}

// expectServed fails unless the data plane at dpAddr answers 200 for the
// application, allowing its container a few seconds to come up.
func expectServed(ctx context.Context, t *testing.T, dpAddr, appID string) {
	t.Helper()
	_, port, _ := net.SplitHostPort(dpAddr)
	client := &http.Client{Timeout: 5 * time.Second}
	var last string
	for range 30 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+dpAddr+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = appID + ".localhost:" + port
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			last = resp.Status
		} else {
			last = err.Error()
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("application %s is not served on %s: %s", appID, dpAddr, last)
}
