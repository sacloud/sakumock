package apprundedicated_test

import (
	"context"
	"testing"

	sdk "github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated/apis/version"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/apprundedicated"
)

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
	defer srv.Close()
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	// Create
	clusterID := createTestCluster(t, ctx, client)

	// Read
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

	// List
	listResp, err := client.ListClusters(ctx, v1.ListClustersParams{MaxItems: 10})
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(listResp.Clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(listResp.Clusters))
	}

	// Update
	err = client.UpdateCluster(ctx, &v1.UpdateCluster{
		ServicePrincipalID: "999999999999",
	}, v1.UpdateClusterParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("UpdateCluster: %v", err)
	}

	// Delete
	err = client.DeleteCluster(ctx, v1.DeleteClusterParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("DeleteCluster: %v", err)
	}

	// Verify deleted
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
	defer srv.Close()
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	clusterID := createTestCluster(t, ctx, client)

	// Create application
	appResp, err := client.CreateApplication(ctx, &v1.CreateApplication{
		Name:      "test-app",
		ClusterID: clusterID,
	})
	if err != nil {
		t.Fatalf("CreateApplication: %v", err)
	}
	appID := appResp.Application.GetApplicationID()

	// Read application
	getAppResp, err := client.GetApplication(ctx, v1.GetApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("GetApplication: %v", err)
	}
	if getAppResp.Application.GetName() != "test-app" {
		t.Fatalf("expected name 'test-app', got %q", getAppResp.Application.GetName())
	}

	// Create version
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
			{Key: "FOO", Value: strPtr("bar")},
		},
	})
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	versionNum := verResp.GetVersion()
	if versionNum < 1 {
		t.Fatalf("expected version >= 1, got %d", versionNum)
	}

	// List versions
	versions, _, err := versionOp.List(ctx, 30, nil)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version, got %d", len(versions))
	}

	// Read version
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

	// Deactivate
	err = client.UpdateApplication(ctx, &v1.UpdateApplication{
		ActiveVersion: v1.NilInt32{Null: true},
	}, v1.UpdateApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("UpdateApplication (deactivate): %v", err)
	}

	// Delete version
	err = versionOp.Delete(ctx, versionNum)
	if err != nil {
		t.Fatalf("DeleteVersion: %v", err)
	}

	// Delete application
	err = client.DeleteApplication(ctx, v1.DeleteApplicationParams{ApplicationID: appID})
	if err != nil {
		t.Fatalf("DeleteApplication: %v", err)
	}
}

func TestCertificateLifecycle(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer srv.Close()
	ctx := context.Background()
	client := newTestClient(t, srv.TestURL())

	clusterID := createTestCluster(t, ctx, client)

	// Create certificate
	createCertResp, err := client.CreateCertificate(ctx, &v1.CreateCertificate{
		Name:           "test-cert",
		CertificatePem: "-----BEGIN CERTIFICATE-----\nMIItest\n-----END CERTIFICATE-----",
		PrivatekeyPem:  "-----BEGIN PRIVATE KEY-----\nMIItest\n-----END PRIVATE KEY-----",
	}, v1.CreateCertificateParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	certID := createCertResp.Certificate.GetCertificateID()

	// Read
	getCertResp, err := client.GetCertificate(ctx, v1.GetCertificateParams{ClusterID: clusterID, CertificateID: certID})
	if err != nil {
		t.Fatalf("GetCertificate: %v", err)
	}
	if getCertResp.Certificate.GetName() != "test-cert" {
		t.Fatalf("expected name 'test-cert', got %q", getCertResp.Certificate.GetName())
	}

	// Update
	err = client.UpdateCertificate(ctx, &v1.UpdateCertificate{
		Name:           "updated-cert",
		CertificatePem: "-----BEGIN CERTIFICATE-----\nMIIupdated\n-----END CERTIFICATE-----",
		PrivatekeyPem:  "-----BEGIN PRIVATE KEY-----\nMIIupdated\n-----END PRIVATE KEY-----",
	}, v1.UpdateCertificateParams{ClusterID: clusterID, CertificateID: certID})
	if err != nil {
		t.Fatalf("UpdateCertificate: %v", err)
	}

	// Delete
	err = client.DeleteCertificate(ctx, v1.DeleteCertificateParams{ClusterID: clusterID, CertificateID: certID})
	if err != nil {
		t.Fatalf("DeleteCertificate: %v", err)
	}
}

func TestServiceClasses(t *testing.T) {
	srv := apprundedicated.NewTestServer(apprundedicated.Config{})
	defer srv.Close()
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

func strPtr(s string) *string { return &s }
