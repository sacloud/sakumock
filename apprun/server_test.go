package apprun_test

import (
	"context"
	"fmt"
	"testing"

	apprunsdk "github.com/sacloud/sacloud-sdk-go/api/apprun"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apprun/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/apprun"
)

func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_APPRUN_SHARED=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := apprunsdk.NewClientWithAPIRootURL(&sa, serverURL)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestUserLifecycle(t *testing.T) {
	srv := apprun.NewTestServer(apprun.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	userOp := apprunsdk.NewUserOp(client)

	// Create user
	created, err := userOp.Create(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if created.Limit.ApplicationCount <= 0 {
		t.Fatalf("expected positive application_count, got %d", created.Limit.ApplicationCount)
	}

	// Read user
	user, err := userOp.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if user.Limit.ApplicationCount <= 0 {
		t.Fatalf("expected positive application_count, got %d", user.Limit.ApplicationCount)
	}
}

func createTestApp(ctx context.Context, t *testing.T, appOp apprunsdk.ApplicationAPI) *v1.HandlerPostApplication {
	t.Helper()
	created, err := appOp.Create(ctx, &v1.PostApplicationBody{
		Name:           "test-app",
		TimeoutSeconds: 60,
		Port:           8080,
		MinScale:       0,
		MaxScale:       1,
		Components: []v1.PostApplicationBodyComponentsItem{
			{
				Name:      "web",
				MaxCPU:    v1.PostApplicationBodyComponentsItemMaxCPU05,
				MaxMemory: v1.PostApplicationBodyComponentsItemMaxMemory1Gi,
				DeploySource: v1.PostApplicationBodyComponentsItemDeploySource{
					ContainerRegistry: v1.NewOptPostApplicationBodyComponentsItemDeploySourceContainerRegistry(
						v1.PostApplicationBodyComponentsItemDeploySourceContainerRegistry{
							Image: "nginx:latest",
						},
					),
				},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return created
}

func TestApplicationLifecycle(t *testing.T) {
	srv := apprun.NewTestServer(apprun.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	appOp := apprunsdk.NewApplicationOp(client)

	// Create application
	created := createTestApp(ctx, t, appOp)
	if created.Name != "test-app" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	if created.Port != 8080 {
		t.Fatalf("unexpected port: %d", created.Port)
	}
	if created.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if created.ResourceID == "" {
		t.Fatal("expected non-empty resource_id")
	}

	// Read application
	read, err := appOp.Read(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-app" {
		t.Fatalf("unexpected name: %s", read.Name)
	}

	// List applications
	list, err := appOp.List(ctx, &v1.ListApplicationsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Data) != 1 {
		t.Fatalf("expected 1 app, got %d", len(list.Data))
	}

	// Update application
	newTimeout := 120
	updated, err := appOp.Update(ctx, created.ID, &v1.PatchApplicationBody{
		TimeoutSeconds: v1.NewOptInt(newTimeout),
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.TimeoutSeconds != newTimeout {
		t.Fatalf("expected timeout %d, got %d", newTimeout, updated.TimeoutSeconds)
	}

	// Read status
	status, err := appOp.ReadStatus(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.Status != "Healthy" {
		t.Fatalf("expected Healthy, got %s", status.Status)
	}

	// Delete application
	if err := appOp.Delete(ctx, created.ID); err != nil {
		t.Fatal(err)
	}

	// List should be empty
	list, err = appOp.List(ctx, &v1.ListApplicationsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(list.Data) != 0 {
		t.Fatalf("expected 0 apps after delete, got %d", len(list.Data))
	}
}

func TestVersionLifecycle(t *testing.T) {
	srv := apprun.NewTestServer(apprun.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	appOp := apprunsdk.NewApplicationOp(client)
	versionOp := apprunsdk.NewVersionOp(client)

	// Create app (auto-creates version 1)
	created := createTestApp(ctx, t, appOp)

	// Update app (auto-creates version 2) and shift traffic to latest
	_, err := appOp.Update(ctx, created.ID, &v1.PatchApplicationBody{
		TimeoutSeconds:      v1.NewOptInt(120),
		AllTrafficAvailable: v1.NewOptBool(true),
	})
	if err != nil {
		t.Fatal(err)
	}

	// List versions
	versions, err := versionOp.List(ctx, created.ID, &v1.ListApplicationVersionsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(versions.Data) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(versions.Data))
	}

	// Read the latest version (Data[0] is newest due to desc sort)
	latest := versions.Data[0]
	version, err := versionOp.Read(ctx, created.ID, latest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if version.Name == "" {
		t.Fatal("expected non-empty version name")
	}

	// Read version status
	vStatus, err := versionOp.ReadStatus(ctx, created.ID, latest.ID)
	if err != nil {
		t.Fatal(err)
	}
	if vStatus.Status != "Healthy" {
		t.Fatalf("expected Healthy, got %s", vStatus.Status)
	}

	// Delete the older version (not the latest)
	older := versions.Data[1]
	if err := versionOp.Delete(ctx, created.ID, older.ID); err != nil {
		t.Fatal(err)
	}

	// Verify 1 version left
	versions, err = versionOp.List(ctx, created.ID, &v1.ListApplicationVersionsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(versions.Data) != 1 {
		t.Fatalf("expected 1 version after delete, got %d", len(versions.Data))
	}
}

func TestTrafficManagement(t *testing.T) {
	srv := apprun.NewTestServer(apprun.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	appOp := apprunsdk.NewApplicationOp(client)
	trafficOp := apprunsdk.NewTrafficOp(client)

	created := createTestApp(ctx, t, appOp)

	// Get traffic (should have default 100% to latest)
	traffic, err := trafficOp.List(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(traffic.Data) != 1 {
		t.Fatalf("expected 1 traffic item, got %d", len(traffic.Data))
	}
	if traffic.Data[0].Percent != 100 {
		t.Fatalf("expected 100%%, got %d%%", traffic.Data[0].Percent)
	}

	// Update app to create a second version
	_, err = appOp.Update(ctx, created.ID, &v1.PatchApplicationBody{
		TimeoutSeconds: v1.NewOptInt(120),
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update traffic distribution
	trafficBody := v1.PutTrafficsBody{
		v1.NewPutTrafficsBodyItem0PutTrafficsBodyItem(v1.PutTrafficsBodyItem0{
			IsLatestVersion: true,
			Percent:         70,
		}),
		v1.NewPutTrafficsBodyItem1PutTrafficsBodyItem(v1.PutTrafficsBodyItem1{
			VersionName: traffic.Data[0].VersionName,
			Percent:     30,
		}),
	}
	updated, err := trafficOp.Update(ctx, created.ID, &trafficBody)
	if err != nil {
		t.Fatal(err)
	}
	if len(updated.Data) != 2 {
		t.Fatalf("expected 2 traffic items, got %d", len(updated.Data))
	}
}

func TestPacketFilter(t *testing.T) {
	srv := apprun.NewTestServer(apprun.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	appOp := apprunsdk.NewApplicationOp(client)
	pfOp := apprunsdk.NewPacketFilterOp(client)

	created := createTestApp(ctx, t, appOp)

	// Get default packet filter
	pf, err := pfOp.Read(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if pf.IsEnabled {
		t.Fatal("expected packet filter to be disabled by default")
	}

	// Update packet filter
	updated, err := pfOp.Update(ctx, created.ID, &v1.PatchPacketFilter{
		IsEnabled: v1.NewOptBool(true),
		Settings: []v1.PatchPacketFilterSettingsItem{
			{
				FromIP:             "192.168.1.0",
				FromIPPrefixLength: 24,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.IsEnabled {
		t.Fatal("expected packet filter to be enabled")
	}
	if len(updated.Settings) != 1 {
		t.Fatalf("expected 1 setting, got %d", len(updated.Settings))
	}
}

func TestApplicationNotFound(t *testing.T) {
	srv := apprun.NewTestServer(apprun.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	appOp := apprunsdk.NewApplicationOp(client)

	_, err := appOp.Read(ctx, "nonexistent-id")
	if err == nil {
		t.Fatal("expected error for non-existent application")
	}
}

func TestValidation(t *testing.T) {
	srv := apprun.NewTestServer(apprun.Config{})
	defer srv.Close()
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	appOp := apprunsdk.NewApplicationOp(client)

	validComponents := []v1.PostApplicationBodyComponentsItem{
		{
			Name:      "web",
			MaxCPU:    v1.PostApplicationBodyComponentsItemMaxCPU05,
			MaxMemory: v1.PostApplicationBodyComponentsItemMaxMemory1Gi,
			DeploySource: v1.PostApplicationBodyComponentsItemDeploySource{
				ContainerRegistry: v1.NewOptPostApplicationBodyComponentsItemDeploySourceContainerRegistry(
					v1.PostApplicationBodyComponentsItemDeploySourceContainerRegistry{
						Image: "nginx:latest",
					},
				),
			},
		},
	}

	t.Run("reserved port", func(t *testing.T) {
		_, err := appOp.Create(ctx, &v1.PostApplicationBody{
			Name: "test", TimeoutSeconds: 60, Port: 8443, MinScale: 0, MaxScale: 1,
			Components: validComponents,
		})
		if err == nil {
			t.Fatal("expected error for reserved port 8443")
		}
	})

	t.Run("timeout out of range", func(t *testing.T) {
		_, err := appOp.Create(ctx, &v1.PostApplicationBody{
			Name: "test", TimeoutSeconds: 999, Port: 8080, MinScale: 0, MaxScale: 1,
			Components: validComponents,
		})
		if err == nil {
			t.Fatal("expected error for timeout > 300")
		}
	})

	t.Run("min_scale > max_scale", func(t *testing.T) {
		_, err := appOp.Create(ctx, &v1.PostApplicationBody{
			Name: "test", TimeoutSeconds: 60, Port: 8080, MinScale: 5, MaxScale: 2,
			Components: validComponents,
		})
		if err == nil {
			t.Fatal("expected error for min_scale > max_scale")
		}
	})

	t.Run("max_scale out of range", func(t *testing.T) {
		_, err := appOp.Create(ctx, &v1.PostApplicationBody{
			Name: "test", TimeoutSeconds: 60, Port: 8080, MinScale: 0, MaxScale: 20,
			Components: validComponents,
		})
		if err == nil {
			t.Fatal("expected error for max_scale > 10")
		}
	})

	t.Run("reserved env key", func(t *testing.T) {
		_, err := appOp.Create(ctx, &v1.PostApplicationBody{
			Name: "test", TimeoutSeconds: 60, Port: 8080, MinScale: 0, MaxScale: 1,
			Components: []v1.PostApplicationBodyComponentsItem{
				{
					Name:      "web",
					MaxCPU:    v1.PostApplicationBodyComponentsItemMaxCPU05,
					MaxMemory: v1.PostApplicationBodyComponentsItemMaxMemory1Gi,
					DeploySource: v1.PostApplicationBodyComponentsItemDeploySource{
						ContainerRegistry: v1.NewOptPostApplicationBodyComponentsItemDeploySourceContainerRegistry(
							v1.PostApplicationBodyComponentsItemDeploySourceContainerRegistry{
								Image: "nginx:latest",
							},
						),
					},
					Env: v1.NewOptNilPostApplicationBodyComponentsItemEnvItemArray([]v1.PostApplicationBodyComponentsItemEnvItem{
						{Key: v1.NewOptString("PORT"), Value: v1.NewOptString("3000")},
					}),
				},
			},
		})
		if err == nil {
			t.Fatal("expected error for reserved env key PORT")
		}
	})

	t.Run("application limit", func(t *testing.T) {
		for i := range 5 {
			_, err := appOp.Create(ctx, &v1.PostApplicationBody{
				Name: fmt.Sprintf("app-%d", i), TimeoutSeconds: 60, Port: 8080, MinScale: 0, MaxScale: 1,
				Components: validComponents,
			})
			if err != nil {
				t.Fatalf("create app %d: %v", i, err)
			}
		}
		_, err := appOp.Create(ctx, &v1.PostApplicationBody{
			Name: "app-over-limit", TimeoutSeconds: 60, Port: 8080, MinScale: 0, MaxScale: 1,
			Components: validComponents,
		})
		if err == nil {
			t.Fatal("expected error for exceeding application limit")
		}
	})
}
