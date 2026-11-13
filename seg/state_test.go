package seg_test

import (
	"testing"

	segsdk "github.com/sacloud/sacloud-sdk-go/api/service-endpoint-gateway"
	v1 "github.com/sacloud/sacloud-sdk-go/api/service-endpoint-gateway/apis/v1"

	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/seg"
)

func stateCreateRequest() v1.ModelsApplianceApplianceCreateRequest {
	return v1.ModelsApplianceApplianceCreateRequest{
		Appliance: v1.ModelsApplianceApplianceCreateBody{
			Remark: v1.ModelsRemarkApplianceCreateRemark{
				Switch:  v1.ModelsRemarkSwitchRemark{ID: "123456789012"},
				Network: v1.ModelsRemarkNetworkRemark{NetworkMaskLen: 24},
				Servers: []v1.ModelsRemarkServerRemark{{IPAddress: "192.0.2.15"}},
			},
		},
	}
}

func TestStateSurvivesRestart(t *testing.T) {
	var id, sharedSwitchID, sharedIP, settingsHash string

	coretest.StateRestart(t, seg.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			op := segsdk.NewServiceEndpointGatewayOp(newTestClient(t, url))
			created, err := op.Create(ctx, stateCreateRequest())
			if err != nil {
				t.Fatal(err)
			}
			id = created.Appliance.ID
			sharedSwitchID = created.Appliance.Interfaces[0].Switch.ID
			sharedIP = created.Appliance.Interfaces[0].IPAddress.Value

			updated, err := op.Update(ctx, id, v1.ModelsApplianceApplianceUpdateRequest{
				Appliance: v1.ModelsApplianceApplianceUpdateBody{
					Settings: v1.ModelsSettingsApplianceSettings{
						ServiceEndpointGateway: v1.ModelsSettingsServiceEndpointGatewaySettings{
							EnabledServices: []v1.ModelsSettingsEnabledService{
								{
									Type:   v1.ModelsSettingsEnabledServiceTypeObjectStorage,
									Config: v1.ModelsSettingsServiceConfig{Endpoints: []string{"s3.isk01.sakurastorage.jp"}},
								},
							},
						},
					},
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			settingsHash = updated.Appliance.SettingsHash.Value
			if err := op.Shutdown(ctx, id); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			op := segsdk.NewServiceEndpointGatewayOp(newTestClient(t, url))
			listed, err := op.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(listed.Appliances) != 1 {
				t.Fatalf("got %d appliances after restart, want 1", len(listed.Appliances))
			}
			read, err := op.Read(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			a := read.Appliance
			if a.SettingsHash.Value != settingsHash || settingsHash == "" {
				t.Errorf("restored SettingsHash = %q, want %q", a.SettingsHash.Value, settingsHash)
			}
			services := a.Settings.Value.ServiceEndpointGateway.EnabledServices
			if len(services) != 1 || services[0].Type != v1.ModelsSettingsEnabledServiceTypeObjectStorage ||
				len(services[0].Config.Endpoints) != 1 || services[0].Config.Endpoints[0] != "s3.isk01.sakurastorage.jp" {
				t.Errorf("restored enabled services = %+v", services)
			}
			if len(a.Interfaces) != 2 || a.Interfaces[0].Switch.ID != sharedSwitchID ||
				a.Interfaces[0].IPAddress.Value != sharedIP || a.Interfaces[1].Switch.ID != "123456789012" ||
				a.Interfaces[1].UserIPAddress.Value != "192.0.2.15" {
				t.Errorf("restored interfaces = %+v", a.Interfaces)
			}
			if _, err := op.ReadInterface(ctx, id, sharedSwitchID); err != nil {
				t.Errorf("read restored shared interface: %v", err)
			}
			power, err := op.ReadPowerStatus(ctx, id)
			if err != nil {
				t.Fatal(err)
			}
			if power.Instance.Status != v1.ModelsInstanceInstanceForPowerStatusDown {
				t.Errorf("restored power status = %s, want down", power.Instance.Status)
			}
		})
}

// An appliance created after a restart shares neither an ID nor the synthetic
// shared-switch IP with a restored one.
func TestStateRestoredIDsAndSharedIPsAreNotReused(t *testing.T) {
	var id, sharedSwitchID, sharedIP string

	coretest.StateRestart(t, seg.Config{},
		func(t *testing.T, url string) {
			created, err := segsdk.NewServiceEndpointGatewayOp(newTestClient(t, url)).Create(t.Context(), stateCreateRequest())
			if err != nil {
				t.Fatal(err)
			}
			id = created.Appliance.ID
			sharedSwitchID = created.Appliance.Interfaces[0].Switch.ID
			sharedIP = created.Appliance.Interfaces[0].IPAddress.Value
		},
		func(t *testing.T, url string) {
			created, err := segsdk.NewServiceEndpointGatewayOp(newTestClient(t, url)).Create(t.Context(), stateCreateRequest())
			if err != nil {
				t.Fatal(err)
			}
			restored := []string{id, sharedSwitchID}
			for _, got := range []string{created.Appliance.ID, created.Appliance.Interfaces[0].Switch.ID} {
				for _, old := range restored {
					if got == old {
						t.Errorf("new appliance reused the restored ID %s", got)
					}
				}
			}
			if ip := created.Appliance.Interfaces[0].IPAddress.Value; ip == sharedIP {
				t.Errorf("new appliance reused the restored shared IP %s", ip)
			}
		}, true)
}
