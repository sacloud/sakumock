package seg_test

import (
	"testing"

	segsdk "github.com/sacloud/sacloud-sdk-go/api/service-endpoint-gateway"
	v1 "github.com/sacloud/sacloud-sdk-go/api/service-endpoint-gateway/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/seg"
)

// newTestClient builds an SDK client pointed at the mock. SAKURA_ZONE is
// deliberately left unset: the upstream SDK ignores the
// SAKURA_ENDPOINTS_SERVICE_ENDPOINT_GATEWAY override entirely once a zone is
// set (see seg/README.md).
func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_SERVICE_ENDPOINT_GATEWAY=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := segsdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func closeAndCheck(t *testing.T, srv *seg.Server) {
	t.Helper()
	srv.Close()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
}

func TestApplianceLifecycle(t *testing.T) {
	srv := seg.NewTestServer(seg.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	op := segsdk.NewServiceEndpointGatewayOp(newTestClient(t, srv.TestURL()))

	listed, err := op.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Appliances) != 0 {
		t.Fatalf("expected 0 appliances, got %d", len(listed.Appliances))
	}

	created, err := op.Create(ctx, v1.ModelsApplianceApplianceCreateRequest{
		Appliance: v1.ModelsApplianceApplianceCreateBody{
			Remark: v1.ModelsRemarkApplianceCreateRemark{
				Switch:  v1.ModelsRemarkSwitchRemark{ID: "123456789012"},
				Network: v1.ModelsRemarkNetworkRemark{NetworkMaskLen: 24},
				Servers: []v1.ModelsRemarkServerRemark{{IPAddress: "192.0.2.15"}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Appliance.Availability != v1.ModelsApplianceApplianceAvailabilityAvailable {
		t.Fatalf("unexpected availability: %s", created.Appliance.Availability)
	}
	if len(created.Appliance.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(created.Appliance.Interfaces))
	}
	id := created.Appliance.ID
	userSwitchID := created.Appliance.Interfaces[1].Switch.ID
	sharedSwitchID := created.Appliance.Interfaces[0].Switch.ID
	if userSwitchID != "123456789012" {
		t.Fatalf("unexpected user-scope switch ID: %s", userSwitchID)
	}

	listed, err = op.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Appliances) != 1 {
		t.Fatalf("expected 1 appliance, got %d", len(listed.Appliances))
	}

	read, err := op.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if read.Appliance.ID != id {
		t.Fatalf("unexpected read response: %+v", read)
	}

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
	if updated.Appliance.SettingsHash.Null {
		t.Fatalf("expected non-null SettingsHash after update: %+v", updated.Appliance.SettingsHash)
	}

	if err := op.Apply(ctx, id); err != nil {
		t.Fatal(err)
	}

	sharedIface, err := op.ReadInterface(ctx, id, sharedSwitchID)
	if err != nil {
		t.Fatal(err)
	}
	if sharedIface.Interface.Switch.Scope != v1.ModelsNetworkSimpleInterfaceSwitchScopeShared {
		t.Fatalf("unexpected shared interface scope: %s", sharedIface.Interface.Switch.Scope)
	}

	userIface, err := op.ReadInterface(ctx, id, userSwitchID)
	if err != nil {
		t.Fatal(err)
	}
	if userIface.Interface.Switch.Scope != v1.ModelsNetworkSimpleInterfaceSwitchScopeUser {
		t.Fatalf("unexpected user interface scope: %s", userIface.Interface.Switch.Scope)
	}

	powerStatus, err := op.ReadPowerStatus(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if powerStatus.Instance.Status != v1.ModelsInstanceInstanceForPowerStatusUp {
		t.Fatalf("expected appliance to be up after create, got %s", powerStatus.Instance.Status)
	}

	if err := op.Shutdown(ctx, id); err != nil {
		t.Fatal(err)
	}
	powerStatus, err = op.ReadPowerStatus(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if powerStatus.Instance.Status != v1.ModelsInstanceInstanceForPowerStatusDown {
		t.Fatalf("expected appliance to be down after shutdown, got %s", powerStatus.Instance.Status)
	}

	if _, err := op.PowerOn(ctx, id); err != nil {
		t.Fatal(err)
	}
	if err := op.Reset(ctx, id); err != nil {
		t.Fatal(err)
	}

	if err := op.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	listed, err = op.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Appliances) != 0 {
		t.Fatalf("expected 0 appliances after delete, got %d", len(listed.Appliances))
	}
}

func TestReadNotFound(t *testing.T) {
	srv := seg.NewTestServer(seg.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	op := segsdk.NewServiceEndpointGatewayOp(newTestClient(t, srv.TestURL()))

	if _, err := op.Read(ctx, "999999999999"); err == nil {
		t.Fatal("expected error for non-existent appliance")
	}
}

func TestCreateRejectsEmptyServerIPAddress(t *testing.T) {
	srv := seg.NewTestServer(seg.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	op := segsdk.NewServiceEndpointGatewayOp(newTestClient(t, srv.TestURL()))

	_, err := op.Create(ctx, v1.ModelsApplianceApplianceCreateRequest{
		Appliance: v1.ModelsApplianceApplianceCreateBody{
			Remark: v1.ModelsRemarkApplianceCreateRemark{
				Switch:  v1.ModelsRemarkSwitchRemark{ID: "123456789012"},
				Network: v1.ModelsRemarkNetworkRemark{NetworkMaskLen: 24},
				Servers: []v1.ModelsRemarkServerRemark{{IPAddress: ""}},
			},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty Servers[].IPAddress")
	}
	listed, err := op.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Appliances) != 0 {
		t.Fatalf("rejected create must not be stored, got %d appliances", len(listed.Appliances))
	}
}
