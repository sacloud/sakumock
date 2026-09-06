package apprun

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	apprunsdk "github.com/sacloud/sacloud-sdk-go/api/apprun"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apprun/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"
)

// TestSecrets lives in the package so it can check the stored (never
// reported) secret values behind the SDK-visible behavior.
func TestSecrets(t *testing.T) {
	srv := NewTestServer(Config{})
	defer func() {
		if v := srv.SpecViolations(); len(v) != 0 {
			t.Errorf("spec violations recorded: %+v", v)
		}
		srv.Close()
	}()
	ctx := t.Context()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_APPRUN_SHARED=" + srv.TestURL(),
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := apprunsdk.NewClientWithAPIRootURL(&sa, srv.TestURL())
	if err != nil {
		t.Fatal(err)
	}
	appOp := apprunsdk.NewApplicationOp(client)
	versionOp := apprunsdk.NewVersionOp(client)

	created, err := appOp.Create(ctx, &v1.CreateApplicationBody{
		Name: "secret-app", TimeoutSeconds: 60, Port: 8080, MinScale: 0, MaxScale: 1,
		Components: []v1.CreateApplicationBodyComponentsItem{
			{
				Name:      "web",
				MaxCPU:    v1.CreateApplicationBodyComponentsItemMaxCPU05,
				MaxMemory: v1.CreateApplicationBodyComponentsItemMaxMemory1Gi,
				DeploySource: v1.CreateApplicationBodyComponentsItemDeploySource{
					ContainerRegistry: v1.NewOptCreateApplicationBodyComponentsItemDeploySourceContainerRegistry(
						v1.CreateApplicationBodyComponentsItemDeploySourceContainerRegistry{Image: "nginx:latest"},
					),
				},
				Env: v1.NewOptNilRequestEnv(v1.RequestEnv{{Key: "MY_ENV", Value: "plain"}}),
				Secret: v1.NewOptNilCreateApplicationBodyComponentsItemSecretItemArray([]v1.CreateApplicationBodyComponentsItemSecretItem{
					{Key: "MY_SECRET", Value: "s3cret"},
				}),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The response reports only the secret's key.
	if got := created.Components[0].Secret; len(got) != 1 || got[0].Key != "MY_SECRET" {
		t.Fatalf("unexpected secret in create response: %+v", got)
	}
	if got := created.Components[0].Env; len(got) != 1 || got[0].Value != "plain" {
		t.Fatalf("unexpected env in create response: %+v", got)
	}

	patchComponent := func(secrets []v1.PatchApplicationBodyComponentsItemSecretItem) *v1.PatchApplicationBody {
		return &v1.PatchApplicationBody{
			Components: []v1.PatchApplicationBodyComponentsItem{
				{
					Name:      "web",
					MaxCPU:    v1.PatchApplicationBodyComponentsItemMaxCPU05,
					MaxMemory: v1.PatchApplicationBodyComponentsItemMaxMemory1Gi,
					DeploySource: v1.PatchApplicationBodyComponentsItemDeploySource{
						ContainerRegistry: v1.NewOptPatchApplicationBodyComponentsItemDeploySourceContainerRegistry(
							v1.PatchApplicationBodyComponentsItemDeploySourceContainerRegistry{Image: "nginx:latest"},
						),
					},
					Secret: v1.NewOptNilPatchApplicationBodyComponentsItemSecretItemArray(secrets),
				},
			},
		}
	}

	t.Run("value omitted is inherited from the latest version", func(t *testing.T) {
		updated, err := appOp.Update(ctx, created.ID, patchComponent([]v1.PatchApplicationBodyComponentsItemSecretItem{
			{Key: "MY_SECRET"},
			{Key: "OTHER", Value: v1.NewOptString("other")},
		}))
		if err != nil {
			t.Fatal(err)
		}
		keys := make([]string, 0, len(updated.Components[0].Secret))
		for _, s := range updated.Components[0].Secret {
			keys = append(keys, s.Key)
		}
		if fmt.Sprint(keys) != "[MY_SECRET OTHER]" {
			t.Fatalf("unexpected secret keys after patch: %v", keys)
		}
		// The store keeps the inherited value, so the container still gets it.
		app, _ := srv.store.ReadApplication(created.ID)
		if got := app.Components[0].Secret; len(got) != 2 || got[0] != (EnvVar{"MY_SECRET", "s3cret"}) || got[1] != (EnvVar{"OTHER", "other"}) {
			t.Fatalf("stored secrets = %+v", got)
		}

		versions, err := versionOp.List(ctx, created.ID, &v1.ListApplicationVersionsParams{})
		if err != nil {
			t.Fatal(err)
		}
		version, err := versionOp.Read(ctx, created.ID, versions.Data[0].ID)
		if err != nil {
			t.Fatal(err)
		}
		if got := version.Components[0].Secret; len(got) != 2 {
			t.Fatalf("expected 2 secrets in version, got %+v", got)
		}
	})

	t.Run("unknown key without value is rejected", func(t *testing.T) {
		_, err := appOp.Update(ctx, created.ID, patchComponent([]v1.PatchApplicationBodyComponentsItemSecretItem{
			{Key: "NEVER_SET"},
		}))
		if err == nil {
			t.Fatal("expected error for a secret with no stored value")
		}
	})

	t.Run("key shared with env is rejected", func(t *testing.T) {
		body := patchComponent([]v1.PatchApplicationBodyComponentsItemSecretItem{
			{Key: "MY_ENV", Value: v1.NewOptString("x")},
		})
		body.Components[0].Env = v1.NewOptNilRequestEnv(v1.RequestEnv{{Key: "MY_ENV", Value: "plain"}})
		_, err := appOp.Update(ctx, created.ID, body)
		if err == nil {
			t.Fatal("expected error for a secret named like an env var")
		}
		if got := srv.SpecViolations(); len(got) != 0 {
			t.Fatalf("spec violations: %+v", got)
		}
	})

	t.Run("reserved key is rejected", func(t *testing.T) {
		_, err := appOp.Update(ctx, created.ID, patchComponent([]v1.PatchApplicationBodyComponentsItemSecretItem{
			{Key: "K_SERVICE", Value: v1.NewOptString("x")},
		}))
		if err == nil {
			t.Fatal("expected error for reserved secret key")
		}
	})

	t.Run("create requires a value (spec validation)", func(t *testing.T) {
		body := `{"name":"no-value","timeout_seconds":60,"port":8080,"min_scale":0,"max_scale":1,` +
			`"components":[{"name":"web","max_cpu":"0.5","max_memory":"1Gi",` +
			`"deploy_source":{"container_registry":{"image":"nginx:latest"}},"secret":[{"key":"MY_SECRET"}]}]}`
		resp, err := http.Post(srv.TestURL()+"/applications", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})
}
