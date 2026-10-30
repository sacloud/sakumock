package apprundedicated_test

import (
	"context"
	"strings"
	"testing"

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
	resp, err := client.CreateCluster(ctx, &v1.CreateClusterRequest{
		Name:               "test-cluster",
		ServicePrincipalID: "123456789012",
		Ports: []v1.ClusterLoadBalancerPort{
			{Port: 443, Protocol: v1.ClusterLoadBalancerPortProtocolHTTPS},
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

	getResp, err := client.ReadCluster(ctx, v1.ReadClusterParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("ReadCluster: %v", err)
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

	err = client.UpdateCluster(ctx, &v1.UpdateClusterRequest{
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

	appResp, err := client.CreateApplication(ctx, &v1.CreateApplicationRequest{
		Name:      "test-app",
		ClusterID: clusterID,
	})
	if err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}
	appID := appResp.Application.GetApplicationID()

	getAppResp, err := client.ReadApplication(ctx, v1.ReadApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("ReadApplication: %v", err)
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
	err = client.UpdateApplication(ctx, &v1.UpdateApplicationRequest{
		ActiveVersion: v1.NilApplicationVersionNumber{Value: v1.ApplicationVersionNumber(versionNum)},
	}, v1.UpdateApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("UpdateApplication: %v", err)
	}

	err = client.UpdateApplication(ctx, &v1.UpdateApplicationRequest{
		ActiveVersion: v1.NilApplicationVersionNumber{Null: true},
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

	createCertResp, err := client.CreateCertificate(ctx, &v1.CreateCertificateRequest{
		Name:           "test-cert",
		CertificatePem: "-----BEGIN CERTIFICATE-----\nMIItest\n-----END CERTIFICATE-----",
		PrivatekeyPem:  "-----BEGIN PRIVATE KEY-----\nMIItest\n-----END PRIVATE KEY-----",
	}, v1.CreateCertificateParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certID := createCertResp.Certificate.GetCertificateID()

	getCertResp, err := client.ReadCertificate(ctx, v1.ReadCertificateParams{ClusterID: clusterID, CertificateID: certID})
	if err != nil {
		t.Fatalf("ReadCertificate: %v", err)
	}
	if getCertResp.Certificate.GetName() != "test-cert" {
		t.Fatalf("expected name 'test-cert', got %q", getCertResp.Certificate.GetName())
	}

	err = client.UpdateCertificate(ctx, &v1.UpdateCertificateRequest{
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

	lbResp, err := client.ListLoadBalancerServiceClasses(ctx)
	if err != nil {
		t.Fatalf("ListLoadBalancerServiceClasses: %v", err)
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
func TestUpdateClusterPorts(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())
	clusterID := createTestCluster(t, ctx, client)

	update := func(ports []v1.ClusterLoadBalancerPort) error {
		t.Helper()
		return client.UpdateCluster(ctx, &v1.UpdateClusterRequest{
			ServicePrincipalID: "123456789012",
			Ports:              ports,
		}, v1.UpdateClusterParams{ClusterID: clusterID})
	}
	ports := func() []v1.ClusterLoadBalancerPort {
		t.Helper()
		resp, err := client.ReadCluster(ctx, v1.ReadClusterParams{ClusterID: clusterID})
		if err != nil {
			t.Fatalf("ReadCluster: %v", err)
		}
		return resp.Cluster.GetPorts()
	}

	if err := update([]v1.ClusterLoadBalancerPort{
		{Port: 80, Protocol: v1.ClusterLoadBalancerPortProtocolHTTP},
		{Port: 8443, Protocol: v1.ClusterLoadBalancerPortProtocolHTTPS},
	}); err != nil {
		t.Fatalf("update with ports: %v", err)
	}
	if got := ports(); len(got) != 2 || got[0].GetPort() != 80 || got[1].GetPort() != 8443 {
		t.Fatalf("unexpected ports after update: %+v", got)
	}

	// A nil Ports is omitted from the request body.
	if err := update(nil); err != nil {
		t.Fatalf("update without ports: %v", err)
	}
	if got := ports(); len(got) != 2 {
		t.Fatalf("omitting ports should keep them, got %+v", got)
	}

	err := update([]v1.ClusterLoadBalancerPort{{Port: 5955, Protocol: v1.ClusterLoadBalancerPortProtocolTCP}})
	if err == nil {
		t.Fatal("reserved port: expected error")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("reserved port: expected 400, got %v", err)
	}
}

func TestApplicationEnoughResources(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer closeAndCheck(t, srv)
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())
	clusterID := createTestCluster(t, ctx, client)
	appResp, err := client.CreateApplication(ctx, &v1.CreateApplicationRequest{Name: "test-app", ClusterID: clusterID})
	if err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}

	resp, err := client.ReadApplication(ctx, v1.ReadApplicationParams{ApplicationID: appResp.Application.GetApplicationID()})
	if err != nil {
		t.Fatalf("ReadApplication: %v", err)
	}
	er := resp.Application.GetEnoughResources()
	if cpu, ok := er.CPU.Get(); !ok || !cpu {
		t.Fatalf("expected enoughResources.cpu true, got %+v", er.CPU)
	}
	if mem, ok := er.Memory.Get(); !ok || !mem {
		t.Fatalf("expected enoughResources.memory true, got %+v", er.Memory)
	}
}
