package addon_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	addonsdk "github.com/sacloud/sacloud-sdk-go/api/addon"
	v1 "github.com/sacloud/sacloud-sdk-go/api/addon/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/addon"
)

// testLocation is the region the tests deploy to.
const testLocation = "japaneast"

func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_ADDON=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
		// The SDK client throttles to 5 requests/second by default, which
		// would make this suite (and the timing in TestProvisioningDelay)
		// needlessly slow against a local mock.
		"SAKURA_RATE_LIMIT=1000",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := addonsdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func closeAndCheck(t *testing.T, srv *addon.Server) {
	t.Helper()
	srv.Close()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
}

// opSet adapts one family's SDK operations to a common shape so the lifecycle
// test can run over every family.
type opSet struct {
	name   string
	create func(context.Context) (resourceGroupName, deploymentName string, err error)
	list   func(context.Context) ([]v1.ResourceGroupResource, error)
	read   func(context.Context, string) (*v1.GetResourceResponse, error)
	// status is nil for the vulnerability detection family, which has no
	// deployment status endpoint.
	status func(context.Context, string, string) (*v1.DeploymentStatus, error)
	remove func(context.Context, string) error
	// data are the paths in the free-form GetResourceResponse.data that the
	// Terraform provider reads to recover the create parameters, with the
	// values the create call above must round-trip to. A change that breaks
	// one of these breaks `terraform plan` against the mock.
	data map[string]any
}

// at resolves a dot-separated path (numeric segments index arrays) in a JSON
// document, so a test can assert on the free-form data the way the provider
// reads it.
func at(t *testing.T, raw []byte, path string) any {
	t.Helper()
	var cur any
	if err := json.Unmarshal(raw, &cur); err != nil {
		t.Fatal(err)
	}
	for _, seg := range strings.Split(path, ".") {
		switch node := cur.(type) {
		case map[string]any:
			v, ok := node[seg]
			if !ok {
				t.Fatalf("path %q: no key %q in %v", path, seg, node)
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(seg)
			if err != nil || i >= len(node) {
				t.Fatalf("path %q: cannot index %q into a %d-element array", path, seg, len(node))
			}
			cur = node[i]
		default:
			t.Fatalf("path %q: %q has no children", path, seg)
		}
	}
	return cur
}

// deploymentNames unwraps the create response shared by every family but
// vulnerability detection.
func deploymentNames(res *v1.PostDeploymentResponse, err error) (string, string, error) {
	if err != nil {
		return "", "", err
	}
	rg, _ := res.ResourceGroupName.Get()
	dn, _ := res.DeploymentName.Get()
	return rg, dn, nil
}

var testOrigin = v1.FrontDoorOrigin{HostName: "origin.example.com", HostHeader: "cdn.example.com"}

// frontDoorData is what the CDN / WAF / DDoS families must report back for the
// Front Door settings the tests create them with.
func frontDoorData(skuName string) map[string]any {
	return map[string]any{
		"location": testLocation,
		"sku.name": skuName,
		"endpoints.0.routes.0.properties.patternsToMatch.0":    "/*",
		"originGroups.0.origins.0.properties.hostName":         testOrigin.HostName,
		"originGroups.0.origins.0.properties.originHostHeader": testOrigin.HostHeader,
	}
}

func opSets(t *testing.T, client *v1.Client) []opSet {
	t.Helper()
	ai := addonsdk.NewAIOp(client)
	cdn := addonsdk.NewCDNOp(client)
	ddos := addonsdk.NewDDoSOp(client)
	waf := addonsdk.NewWAFOp(client)
	vuln := addonsdk.NewVulnerabilityOp(client)
	datalake := addonsdk.NewDataLakeOp(client)
	dwh := addonsdk.NewDWHOp(client)
	etl := addonsdk.NewETLOp(client)
	query := addonsdk.NewQueryOp(client)
	search := addonsdk.NewSearchOp(client)
	streaming := addonsdk.NewStreamingOp(client)

	return []opSet{
		{
			name: "ai",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(ai.Create(ctx, testLocation, v1.AiServiceSku1))
			},
			list: ai.List, read: ai.Read, status: ai.Status, remove: ai.Delete,
			data: map[string]any{"location.name": testLocation, "sku.name": "S0"},
		},
		{
			name: "cdn",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(cdn.Create(ctx, addonsdk.CDNCreateParams{
					Location: testLocation, PricingLevel: v1.PricingLevel1,
					Patterns: []string{"/*"}, Origin: testOrigin,
				}))
			},
			list: cdn.List, read: cdn.Read, status: cdn.Status, remove: cdn.Delete,
			data: frontDoorData("Standard_AzureFrontDoor"),
		},
		{
			name: "ddos",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(ddos.Create(ctx, addonsdk.DDoSCreateParams{
					Location: testLocation, PricingLevel: v1.PricingLevel2,
					Patterns: []string{"/*"}, Origin: testOrigin,
				}))
			},
			list: ddos.List, read: ddos.Read, status: ddos.Status, remove: ddos.Delete,
			data: frontDoorData("Premium_AzureFrontDoor"),
		},
		{
			name: "waf",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(waf.Create(ctx, addonsdk.WAFCreateParams{
					Location: testLocation, PricingLevel: v1.PricingLevel1,
					Patterns: []string{"/*"}, Origin: testOrigin,
				}))
			},
			list: waf.List, read: waf.Read, status: waf.Status, remove: waf.Delete,
			data: frontDoorData("Standard_AzureFrontDoor"),
		},
		{
			name: "vulnerability",
			create: func(ctx context.Context) (string, string, error) {
				res, err := vuln.Create(ctx, addonsdk.VulnerabilityCreateParams{
					Location: testLocation, Os: v1.ServerOsType2,
				})
				if err != nil {
					return "", "", err
				}
				rg, _ := res.ResourceGroupName.Get()
				return rg, "", nil
			},
			list: vuln.List, read: vuln.Read, remove: vuln.Delete,
			data: map[string]any{"location": testLocation},
		},
		{
			name: "datalake",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(datalake.Create(ctx, addonsdk.DataLakeCreateParams{
					Location: testLocation, Performance: v1.DataLakePerformance1, Redundancy: v1.DataLakeRedundancy1,
				}))
			},
			list: datalake.List, read: datalake.Read, status: datalake.Status, remove: datalake.Delete,
			data: map[string]any{"location": testLocation, "sku.name": "Standard_LRS"},
		},
		{
			name: "dwh",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(dwh.Create(ctx, testLocation))
			},
			list: dwh.List, read: dwh.Read, status: dwh.Status, remove: dwh.Delete,
			data: map[string]any{"location": testLocation},
		},
		{
			name: "etl",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(etl.Create(ctx, testLocation))
			},
			list: etl.List, read: etl.Read, status: etl.Status, remove: etl.Delete,
			data: map[string]any{"location": testLocation},
		},
		{
			name: "query",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(query.Create(ctx, testLocation))
			},
			list: query.List, read: query.Read, status: query.Status, remove: query.Delete,
			data: map[string]any{"location": testLocation},
		},
		{
			name: "search",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(search.Create(ctx, addonsdk.SearchCreateParams{
					Location: testLocation, Sku: v1.SearchSku3, ReplicaCount: 2, PartitionCount: 2,
				}))
			},
			list: search.List, read: search.Read, status: search.Status, remove: search.Delete,
			data: map[string]any{
				"location": testLocation, "sku.name": "standard1",
				"properties.partitionCount": float64(2), "properties.replicaCount": float64(2),
				"properties.hostingMode": "default",
			},
		},
		{
			name: "streaming",
			create: func(ctx context.Context) (string, string, error) {
				return deploymentNames(streaming.Create(ctx, testLocation, "30"))
			},
			list: streaming.List, read: streaming.Read, status: streaming.Status, remove: streaming.Delete,
			data: map[string]any{"location": testLocation, "sku.capacity": float64(30)},
		},
	}
}

// TestFamilyLifecycle runs the full resource-group lifecycle of every add-on
// family against the mock with the real SDK.
func TestFamilyLifecycle(t *testing.T) {
	srv := addon.NewTestServer(addon.Config{})
	defer closeAndCheck(t, srv)
	client := newTestClient(t, srv.TestURL())

	for _, ops := range opSets(t, client) {
		t.Run(ops.name, func(t *testing.T) {
			ctx := t.Context()

			resources, err := ops.list(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(resources) != 0 {
				t.Fatalf("expected 0 resources, got %d", len(resources))
			}

			rg, deployment, err := ops.create(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if rg == "" {
				t.Fatal("expected a resource group name")
			}
			if ops.status != nil && deployment == "" {
				t.Fatal("expected a deployment name")
			}

			resources, err = ops.list(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(resources))
			}
			if name, _ := resources[0].ID.Name.Get(); name != rg {
				t.Fatalf("unexpected resource group name in list: %q", name)
			}

			read, err := ops.read(ctx, rg)
			if err != nil {
				t.Fatal(err)
			}
			if got := at(t, read.Data, "name"); got != rg {
				t.Fatalf("unexpected resource name in data: %v", got)
			}
			// Every setting the provider recovers from the free-form data
			// must round-trip to what the create call sent.
			for path, want := range ops.data {
				if got := at(t, read.Data, path); got != want {
					t.Errorf("data.%s = %v, want %v", path, got, want)
				}
			}

			if ops.status != nil {
				status, err := ops.status(ctx, rg, deployment)
				if err != nil {
					t.Fatal(err)
				}
				props, ok := status.Properties.Get()
				if !ok {
					t.Fatal("expected deployment status properties")
				}
				if state, _ := props.ProvisioningState.Get(); state != "Succeeded" {
					t.Fatalf("unexpected provisioning state: %q", state)
				}
			}

			if err := ops.remove(ctx, rg); err != nil {
				t.Fatal(err)
			}
			resources, err = ops.list(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(resources) != 0 {
				t.Fatalf("expected 0 resources after delete, got %d", len(resources))
			}
		})
	}
}

func TestReadAndDeleteNotFound(t *testing.T) {
	srv := addon.NewTestServer(addon.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	ai := addonsdk.NewAIOp(newTestClient(t, srv.TestURL()))

	if _, err := ai.Read(ctx, "no-such-rg"); err == nil {
		t.Fatal("expected an error reading a non-existent resource group")
	} else if !saclient.IsNotFoundError(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
	if err := ai.Delete(ctx, "no-such-rg"); err == nil {
		t.Fatal("expected an error deleting a non-existent resource group")
	} else if !saclient.IsNotFoundError(err) {
		t.Fatalf("expected a not-found error, got %v", err)
	}
}

// TestFamiliesAreIsolated verifies a resource group created for one family is
// not visible through another's endpoints.
func TestFamiliesAreIsolated(t *testing.T) {
	srv := addon.NewTestServer(addon.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())

	created, err := addonsdk.NewAIOp(client).Create(ctx, testLocation, v1.AiServiceSku1)
	if err != nil {
		t.Fatal(err)
	}
	rg, _ := created.ResourceGroupName.Get()

	resources, err := addonsdk.NewQueryOp(client).List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(resources) != 0 {
		t.Fatalf("expected the query family to be empty, got %d resources", len(resources))
	}
	if _, err := addonsdk.NewQueryOp(client).Read(ctx, rg); err == nil {
		t.Fatal("expected the AI resource group to be invisible to the query family")
	}
}

func TestVulnerabilityInstallScript(t *testing.T) {
	srv := addon.NewTestServer(addon.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	vuln := addonsdk.NewVulnerabilityOp(newTestClient(t, srv.TestURL()))

	for _, tc := range []struct {
		name string
		os   v1.ServerOsType
		want string
	}{
		{name: "linux", os: v1.ServerOsType2, want: "#!/bin/sh"},
		{name: "windows", os: v1.ServerOsType1, want: "Write-Host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := vuln.Create(ctx, addonsdk.VulnerabilityCreateParams{Location: testLocation, Os: tc.os})
			if err != nil {
				t.Fatal(err)
			}
			script, ok := res.InstallScript.Get()
			if !ok || !strings.Contains(script, tc.want) {
				t.Fatalf("unexpected install script: %q", script)
			}
			rg, _ := res.ResourceGroupName.Get()
			if !strings.Contains(script, rg) {
				t.Fatalf("install script does not mention the resource group: %q", script)
			}
		})
	}
}

// TestProvisioningDelay covers the deployment still running: the status
// endpoint reports it while list and get behave as the real API does and
// report nothing until it completes.
func TestProvisioningDelay(t *testing.T) {
	srv := addon.NewTestServer(addon.Config{ProvisioningDelay: 100 * time.Millisecond})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	ai := addonsdk.NewAIOp(newTestClient(t, srv.TestURL()))

	created, err := ai.Create(ctx, testLocation, v1.AiServiceSku1)
	if err != nil {
		t.Fatal(err)
	}
	rg, _ := created.ResourceGroupName.Get()
	deployment, _ := created.DeploymentName.Get()

	status, err := ai.Status(ctx, rg, deployment)
	if err != nil {
		t.Fatal(err)
	}
	props, _ := status.Properties.Get()
	if state, _ := props.ProvisioningState.Get(); state != "Running" {
		t.Fatalf("expected the deployment to still be running, got %q", state)
	}
	if _, err := ai.Read(ctx, rg); err == nil {
		t.Fatal("expected a still-provisioning resource group to be invisible to get")
	}
	if resources, err := ai.List(ctx); err != nil {
		t.Fatal(err)
	} else if len(resources) != 0 {
		t.Fatalf("expected a still-provisioning resource group to be invisible to list, got %d", len(resources))
	}

	time.Sleep(150 * time.Millisecond)

	if _, err := ai.Read(ctx, rg); err != nil {
		t.Fatalf("expected the resource group to be readable once provisioned: %v", err)
	}
	status, err = ai.Status(ctx, rg, deployment)
	if err != nil {
		t.Fatal(err)
	}
	props, _ = status.Properties.Get()
	if state, _ := props.ProvisioningState.Get(); state != "Succeeded" {
		t.Fatalf("unexpected provisioning state: %q", state)
	}
}

// TestSpecDerivedBodyValidation covers the request-body constraints the
// generated bodySchemas table enforces before a handler runs.
func TestSpecDerivedBodyValidation(t *testing.T) {
	srv := addon.NewTestServer(addon.Config{})
	defer closeAndCheck(t, srv)

	// sku is required and enumerated, so a body without it is rejected before
	// the handler runs. The SDK's typed API cannot send this.
	res, err := http.Post(srv.TestURL()+"/ai", "application/json",
		strings.NewReader(`{"location":"japaneast"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", res.StatusCode)
	}
	var body struct {
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Errors) != 1 || body.Errors[0].Message == "" {
		t.Fatalf("unexpected error envelope: %+v", body)
	}
}
