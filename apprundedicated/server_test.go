package apprundedicated_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	sdk "github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated/apis/version"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/apprundedicated"
)

// closeAndCheck closes srv, failing the test if any handler response
// diverged from the OpenAPI spec.
func closeAndCheck(t *testing.T, srv *apprundedicated.Server) {
	t.Helper()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
	srv.Close()
}

func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_APPRUN_DEDICATED=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := sdk.NewClientWithAPIRootURL(&sa, serverURL)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func createTestCluster(t *testing.T, ctx context.Context, client *v1.Client) v1.ClusterID {
	t.Helper()
	resp, err := client.CreateCluster(ctx, &v1.CreateCluster{
		Name:               "test-cluster",
		ServicePrincipalID: "123456789012",
		Ports: []v1.CreateLoadBalancerPort{
			{Port: 443, Protocol: v1.CreateLoadBalancerPortProtocolHTTPS},
		},
	})
	if err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	return resp.Cluster.GetClusterID()
}

func TestClusterLifecycle(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	clusterID := createTestCluster(t, ctx, client)

	getResp, err := client.GetCluster(ctx, v1.GetClusterParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("GetCluster: %v", err)
	}
	if getResp.Cluster.GetName() != "test-cluster" {
		t.Fatalf("expected name 'test-cluster', got %q", getResp.Cluster.GetName())
	}
	if getResp.Cluster.GetServicePrincipalID() != "123456789012" {
		t.Fatalf("expected servicePrincipalID '123456789012', got %q", getResp.Cluster.GetServicePrincipalID())
	}

	listResp, err := client.ListClusters(ctx, v1.ListClustersParams{MaxItems: 10})
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(listResp.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(listResp.Clusters))
	}

	err = client.UpdateCluster(ctx, &v1.UpdateCluster{
		ServicePrincipalID: "999999999999",
	}, v1.UpdateClusterParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("UpdateCluster: %v", err)
	}

	err = client.DeleteCluster(ctx, v1.DeleteClusterParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("DeleteCluster: %v", err)
	}

	listResp, err = client.ListClusters(ctx, v1.ListClustersParams{MaxItems: 10})
	if err != nil {
		t.Fatalf("ListClusters after delete: %v", err)
	}
	if len(listResp.Clusters) != 0 {
		t.Fatalf("expected 0 clusters after delete, got %d", len(listResp.Clusters))
	}
}

func TestApplicationVersionLifecycle(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	clusterID := createTestCluster(t, ctx, client)

	appResp, err := client.CreateApplication(ctx, &v1.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterID,
	})
	if err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}
	appID := appResp.Application.GetApplicationID()

	getAppResp, err := client.GetApplication(ctx, v1.GetApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("GetApplication: %v", err)
	}
	if getAppResp.Application.GetName() != "test-app" {
		t.Fatalf("expected name 'test-app', got %q", getAppResp.Application.GetName())
	}

	versionOp := sdk.NewVersionOp(client, appID)
	fixedScale := int32(1)
	verResp, err := versionOp.Create(ctx, version.CreateParams{
		CPU:                    1000,
		Memory:                 2048,
		ScalingMode:            v1.ScalingModeManual,
		FixedScale:             &fixedScale,
		Image:                  "nginx:latest",
		RegistryPasswordAction: v1.RegistryPasswordActionKeep,
		ExposedPorts: []version.ExposedPort{
			{TargetPort: 80},
		},
		EnvVars: []version.EnvironmentVariable{
			{Key: "FOO", Value: new("bar")},
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	versionNum := verResp.GetVersion()
	if versionNum < 1 {
		t.Fatalf("expected version >= 1, got %d", versionNum)
	}

	versions, _, err := versionOp.List(ctx, 30, nil)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	verDetail, err := versionOp.Read(ctx, versionNum)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if verDetail.Image != "nginx:latest" {
		t.Fatalf("expected image 'nginx:latest', got %q", verDetail.Image)
	}
	if verDetail.CPU != 1000 {
		t.Fatalf("expected cpu 1000, got %d", verDetail.CPU)
	}

	// Update application (set active version)
	err = client.UpdateApplication(ctx, &v1.UpdateApplication{
		ActiveVersion: v1.NilInt32{Value: int32(versionNum), Null: false},
	}, v1.UpdateApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("UpdateApplication: %v", err)
	}

	err = client.UpdateApplication(ctx, &v1.UpdateApplication{
		ActiveVersion: v1.NilInt32{Null: true},
	}, v1.UpdateApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("UpdateApplication (deactivate): %v", err)
	}

	err = versionOp.Delete(ctx, versionNum)
	if err != nil {
		t.Fatalf("DeleteVersion: %v", err)
	}

	err = client.DeleteApplication(ctx, v1.DeleteApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("DeleteApplication: %v", err)
	}
}

func TestCertificateLifecycle(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	clusterID := createTestCluster(t, ctx, client)

	createCertResp, err := client.CreateCertificate(ctx, &v1.CreateCertificate{
		Name:           "test-cert",
		CertificatePem: "-----BEGIN CERTIFICATE-----\nMIItest\n-----END CERTIFICATE-----",
		PrivatekeyPem:  "-----BEGIN PRIVATE KEY-----\nMIItest\n-----END PRIVATE KEY-----",
	}, v1.CreateCertificateParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certID := createCertResp.Certificate.GetCertificateID()

	getCertResp, err := client.GetCertificate(ctx, v1.GetCertificateParams{ClusterID: clusterID, CertificateID: certID})
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if getCertResp.Certificate.GetName() != "test-cert" {
		t.Fatalf("expected name 'test-cert', got %q", getCertResp.Certificate.GetName())
	}

	err = client.UpdateCertificate(ctx, &v1.UpdateCertificate{
		Name:           "updated-cert",
		CertificatePem: "-----BEGIN CERTIFICATE-----\nMIIupdated\n-----END CERTIFICATE-----",
		PrivatekeyPem:  "-----BEGIN PRIVATE KEY-----\nMIIupdated\n-----END PRIVATE KEY-----",
	}, v1.UpdateCertificateParams{ClusterID: clusterID, CertificateID: certID})
	if err != nil {
		t.Fatalf("UpdateCertificate: %v", err)
	}

	err = client.DeleteCertificate(ctx, v1.DeleteCertificateParams{ClusterID: clusterID, CertificateID: certID})
	if err != nil {
		t.Fatalf("DeleteCertificate: %v", err)
	}
}

func TestServiceClasses(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	lbResp, err := client.ListLbServiceClasses(ctx)
	if err != nil {
		t.Fatalf("ListLbServiceClasses: %v", err)
	}
	if len(lbResp.LbServiceClasses) == 0 {
		t.Fatal("expected at least one LB service class")
	}

	workerResp, err := client.ListWorkerServiceClasses(ctx)
	if err != nil {
		t.Fatalf("ListWorkerServiceClasses: %v", err)
	}
	if len(workerResp.WorkerServiceClasses) == 0 {
		t.Fatal("expected at least one worker service class")
	}
}

// TestUpdateClusterPorts covers the ports field of PUT /clusters/{clusterID}.
// The SDK's UpdateCluster cannot send ports yet, so the request is raw HTTP.
func TestUpdateClusterPorts(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())
	clusterID := createTestCluster(t, ctx, client)

	put := func(body string) int {
		t.Helper()
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, srv.TestURL()+"/clusters/"+uuid.UUID(clusterID).String(), strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}
	ports := func() []v1.ReadLoadBalancerPort {
		t.Helper()
		resp, err := client.GetCluster(ctx, v1.GetClusterParams{ClusterID: clusterID})
		if err != nil {
			t.Fatalf("GetCluster: %v", err)
		}
		return resp.Cluster.GetPorts()
	}

	if code := put(`{"servicePrincipalID":"123456789012","ports":[{"port":80,"protocol":"http"},{"port":8443,"protocol":"https"}]}`); code != http.StatusNoContent {
		t.Fatalf("update with ports: status %d", code)
	}
	if got := ports(); len(got) != 2 || got[0].GetPort() != 80 || got[1].GetPort() != 8443 {
		t.Fatalf("unexpected ports after update: %+v", got)
	}

	if code := put(`{"servicePrincipalID":"123456789012"}`); code != http.StatusNoContent {
		t.Fatalf("update without ports: status %d", code)
	}
	if got := ports(); len(got) != 2 {
		t.Fatalf("omitting ports should keep them, got %+v", got)
	}

	if code := put(`{"servicePrincipalID":"123456789012","ports":[{"port":5955,"protocol":"tcp"}]}`); code != http.StatusBadRequest {
		t.Fatalf("reserved port: expected 400, got %d", code)
	}
}

// TestApplicationEnoughResources checks the enoughResources field, which the
// SDK does not decode yet.
func TestApplicationEnoughResources(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())
	clusterID := createTestCluster(t, ctx, client)
	appResp, err := client.CreateApplication(ctx, &v1.CreateApplication{Name: "test-app", ClusterID: clusterID})
	if err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}

	resp, err := http.Get(srv.TestURL() + "/applications/" + uuid.UUID(appResp.Application.GetApplicationID()).String())
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body struct {
		Application struct {
			EnoughResources struct {
				CPU    *bool `json:"cpu"`
				Memory *bool `json:"memory"`
			} `json:"enoughResources"`
		} `json:"application"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	er := body.Application.EnoughResources
	if er.CPU == nil || !*er.CPU || er.Memory == nil || !*er.Memory {
		t.Fatalf("expected enoughResources cpu/memory true, got %+v", er)
	}
}
