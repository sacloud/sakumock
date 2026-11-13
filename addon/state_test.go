package addon_test

import (
	"testing"
	"time"

	addonsdk "github.com/sacloud/sacloud-sdk-go/api/addon"
	v1 "github.com/sacloud/sacloud-sdk-go/api/addon/apis/v1"

	"github.com/sacloud/sakumock/addon"
	"github.com/sacloud/sakumock/core/coretest"
)

// One resource group of every family survives a restart, with the create
// parameters the Terraform provider recovers from it.
func TestStateSurvivesRestart(t *testing.T) {
	type names struct{ resourceGroup, deployment string }
	created := make(map[string]names)

	coretest.StateRestart(t, addon.Config{},
		func(t *testing.T, url string) {
			for _, ops := range opSets(t, newTestClient(t, url)) {
				rg, deployment, err := ops.create(t.Context())
				if err != nil {
					t.Fatalf("%s: %v", ops.name, err)
				}
				created[ops.name] = names{rg, deployment}
			}
		},
		func(t *testing.T, url string) {
			for _, ops := range opSets(t, newTestClient(t, url)) {
				t.Run(ops.name, func(t *testing.T) {
					ctx := t.Context()
					want := created[ops.name]

					resources, err := ops.list(ctx)
					if err != nil {
						t.Fatal(err)
					}
					if len(resources) != 1 {
						t.Fatalf("got %d resources after restart, want 1", len(resources))
					}
					if name, _ := resources[0].ID.Name.Get(); name != want.resourceGroup {
						t.Errorf("restored resource group = %q, want %q", name, want.resourceGroup)
					}

					var data []byte
					read, err := ops.read(ctx, want.resourceGroup)
					if expectSDKGap(t, "get-resource-group", err) {
						data = rawResourceData(t, url+ops.path+"/"+want.resourceGroup)
					} else {
						data = read.Data
					}
					for path, value := range ops.data {
						if got := at(t, data, path); got != value {
							t.Errorf("restored data.%s = %v, want %v", path, got, value)
						}
					}

					if ops.status == nil {
						return
					}
					status, err := ops.status(ctx, want.resourceGroup, want.deployment)
					if err != nil {
						t.Fatal(err)
					}
					props, _ := status.Properties.Get()
					if state, _ := props.ProvisioningState.Get(); state != "Succeeded" {
						t.Errorf("restored provisioning state = %q, want Succeeded", state)
					}
				})
			}
		})
}

// A deployment that was still running when the mock stopped keeps its
// original completion time: it is neither lost nor completed by the restart.
func TestStateKeepsProvisioningDeployment(t *testing.T) {
	var rg, deployment string

	coretest.StateRestart(t, addon.Config{ProvisioningDelay: time.Hour},
		func(t *testing.T, url string) {
			created, err := addonsdk.NewAIOp(newTestClient(t, url)).Create(t.Context(), testLocation, v1.AiServiceSku1)
			if err != nil {
				t.Fatal(err)
			}
			rg, _ = created.ResourceGroupName.Get()
			deployment, _ = created.DeploymentName.Get()
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			ai := addonsdk.NewAIOp(newTestClient(t, url))
			status, err := ai.Status(ctx, rg, deployment)
			if err != nil {
				t.Fatal(err)
			}
			props, _ := status.Properties.Get()
			if state, _ := props.ProvisioningState.Get(); state != "Running" {
				t.Errorf("restored provisioning state = %q, want Running", state)
			}
			resources, err := ai.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(resources) != 0 {
				t.Errorf("a still-provisioning resource group became visible after restart: %+v", resources)
			}
		})
}
