package apprundedicated_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	sdk "github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated"
	v1 "github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/api/apprun-dedicated/apis/version"

	"github.com/sacloud/sakumock/apprundedicated"
	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/core/coretest"
)

// createNginxVersion adds a version running nginx to the application and
// returns its number.
func createNginxVersion(t *testing.T, ctx context.Context, client *v1.Client, appID v1.ApplicationID) v1.ApplicationVersionNumber {
	t.Helper()
	resp, err := sdk.NewVersionOp(client, appID).Create(ctx, version.CreateParams{
		CPU:                    1000,
		Memory:                 2048,
		ScalingMode:            v1.ScalingModeManual,
		FixedScale:             new(int32(1)),
		Image:                  "nginx:latest",
		RegistryPasswordAction: v1.RegistryPasswordActionKeep,
		ExposedPorts:           []version.ExposedPort{{TargetPort: 80}},
		EnvVars:                []version.EnvironmentVariable{{Key: "FOO", Value: new("bar")}},
	})
	if err != nil {
		t.Fatalf("CreateVersion: %v", err)
	}
	return resp.GetVersion()
}

func activateVersion(t *testing.T, ctx context.Context, client *v1.Client, appID v1.ApplicationID, ver v1.ApplicationVersionNumber) {
	t.Helper()
	if err := client.UpdateApplication(ctx, &v1.UpdateApplicationRequest{
		ActiveVersion: v1.NilApplicationVersionNumber{Value: ver},
	}, v1.UpdateApplicationParams{ApplicationID: appID}); err != nil {
		t.Fatalf("UpdateApplication: %v", err)
	}
}

func createTestASG(t *testing.T, ctx context.Context, client *v1.Client, clusterID v1.ClusterID, name string) v1.AutoScalingGroupID {
	t.Helper()
	resp, err := client.CreateAutoScalingGroup(ctx, &v1.CreateAutoScalingGroupRequest{
		Name:                   name,
		Zone:                   "is1a",
		WorkerServiceClassPath: "cloud/apprun/dedicated/worker/1vcpu_2gb",
		MinNodes:               2,
		MaxNodes:               3,
		NameServers:            []v1.Ipv4{"210.188.224.10"},
		Interfaces:             []v1.AutoScalingGroupNodeInterface{{InterfaceIndex: 0, Upstream: "shared"}},
	}, v1.CreateAutoScalingGroupParams{ClusterID: clusterID})
	if err != nil {
		t.Fatalf("CreateAutoScalingGroup: %v", err)
	}
	return resp.AutoScalingGroup.GetAutoScalingGroupID()
}

func TestStateSurvivesRestart(t *testing.T) {
	var (
		clusterID     v1.ClusterID
		appID         v1.ApplicationID
		activeVersion v1.ApplicationVersionNumber
		asgID         v1.AutoScalingGroupID
		lbID          v1.LoadBalancerID
		lbNodeIDs     []v1.LoadBalancerNodeID
		workerNodeIDs []v1.WorkerNodeID
		certID        v1.CertificateID
	)

	coretest.StateRestart(t, apprundedicated.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)

			// Two clusters, to check that the list order is restored too.
			clusterID = createTestCluster(t, ctx, client)
			createTestCluster(t, ctx, client)
			if err := client.UpdateCluster(ctx, &v1.UpdateClusterRequest{
				ServicePrincipalID: "999999999999",
				Ports:              []v1.ClusterLoadBalancerPort{{Port: 8443, Protocol: v1.ClusterLoadBalancerPortProtocolHTTPS}},
			}, v1.UpdateClusterParams{ClusterID: clusterID}); err != nil {
				t.Fatalf("UpdateCluster: %v", err)
			}

			appResp, err := client.CreateApplication(ctx, &v1.CreateApplicationRequest{Name: "test-app", ClusterID: clusterID})
			if err != nil {
				t.Fatalf("CreateApplication: %v", err)
			}
			appID = appResp.Application.GetApplicationID()
			// Versions 1 to 3; version 1 is deleted and version 2 activated.
			first := createNginxVersion(t, ctx, client, appID)
			activeVersion = createNginxVersion(t, ctx, client, appID)
			createNginxVersion(t, ctx, client, appID)
			if err := sdk.NewVersionOp(client, appID).Delete(ctx, first); err != nil {
				t.Fatalf("DeleteVersion: %v", err)
			}
			activateVersion(t, ctx, client, appID, activeVersion)

			// A deleted group takes its worker nodes with it.
			deletedASG := createTestASG(t, ctx, client, clusterID, "deleted-asg")
			asgID = createTestASG(t, ctx, client, clusterID, "test-asg")
			if err := client.DeleteAutoScalingGroup(ctx, v1.DeleteAutoScalingGroupParams{ClusterID: clusterID, AutoScalingGroupID: deletedASG}); err != nil {
				t.Fatalf("DeleteAutoScalingGroup: %v", err)
			}
			wns, err := client.ListWorkerNodes(ctx, v1.ListWorkerNodesParams{ClusterID: clusterID, AutoScalingGroupID: asgID, MaxItems: 10})
			if err != nil {
				t.Fatalf("ListWorkerNodes: %v", err)
			}
			for _, wn := range wns.WorkerNodes {
				workerNodeIDs = append(workerNodeIDs, wn.WorkerNodeID)
			}
			if len(workerNodeIDs) != 2 {
				t.Fatalf("got %d worker nodes, want 2", len(workerNodeIDs))
			}
			if err := client.UpdateWorkerNodeDraining(ctx, &v1.UpdateWorkerNodeDrainingRequest{Draining: true},
				v1.UpdateWorkerNodeDrainingParams{ClusterID: clusterID, AutoScalingGroupID: asgID, WorkerNodeID: workerNodeIDs[1]}); err != nil {
				t.Fatalf("UpdateWorkerNodeDraining: %v", err)
			}

			lbResp, err := client.CreateLoadBalancer(ctx, &v1.CreateLoadBalancerRequest{
				Name:             "test-lb",
				ServiceClassPath: "cloud/apprun/dedicated/lb/1vcpu_2gb",
				NameServers:      []v1.Ipv4{"210.188.224.10"},
				Interfaces:       []v1.LoadBalancerInterface{{InterfaceIndex: 0, Upstream: "shared"}},
			}, v1.CreateLoadBalancerParams{ClusterID: clusterID, AutoScalingGroupID: asgID})
			if err != nil {
				t.Fatalf("CreateLoadBalancer: %v", err)
			}
			lbID = lbResp.LoadBalancer.GetLoadBalancerID()
			lbNodes, err := client.ListLoadBalancerNodes(ctx, v1.ListLoadBalancerNodesParams{ClusterID: clusterID, AutoScalingGroupID: asgID, LoadBalancerID: lbID, MaxItems: 10})
			if err != nil {
				t.Fatalf("ListLoadBalancerNodes: %v", err)
			}
			for _, n := range lbNodes.LoadBalancerNodes {
				lbNodeIDs = append(lbNodeIDs, n.LoadBalancerNodeID)
			}
			if len(lbNodeIDs) == 0 {
				t.Fatal("load balancer has no node")
			}

			certResp, err := client.CreateCertificate(ctx, &v1.CreateCertificateRequest{
				Name:           "test-cert",
				CertificatePem: "-----BEGIN CERTIFICATE-----\nMIItest\n-----END CERTIFICATE-----",
				PrivatekeyPem:  "-----BEGIN PRIVATE KEY-----\nMIItest\n-----END PRIVATE KEY-----",
			}, v1.CreateCertificateParams{ClusterID: clusterID})
			if err != nil {
				t.Fatalf("CreateCertificate: %v", err)
			}
			certID = certResp.Certificate.GetCertificateID()
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)

			clusters, err := client.ListClusters(ctx, v1.ListClustersParams{MaxItems: 10})
			if err != nil {
				t.Fatalf("ListClusters: %v", err)
			}
			if len(clusters.Clusters) != 2 || clusters.Clusters[0].GetClusterID() != clusterID {
				t.Fatalf("restored clusters = %+v, want two with %v first", clusters.Clusters, clusterID)
			}
			cluster, err := client.ReadCluster(ctx, v1.ReadClusterParams{ClusterID: clusterID})
			if err != nil {
				t.Fatalf("ReadCluster: %v", err)
			}
			if ports := cluster.Cluster.GetPorts(); cluster.Cluster.GetServicePrincipalID() != "999999999999" || len(ports) != 1 || ports[0].GetPort() != 8443 {
				t.Errorf("restored cluster = %+v", cluster.Cluster)
			}

			app, err := client.ReadApplication(ctx, v1.ReadApplicationParams{ApplicationID: appID})
			if err != nil {
				t.Fatalf("ReadApplication: %v", err)
			}
			if got, ok := app.Application.GetActiveVersion().Get(); app.Application.GetName() != "test-app" || !ok || got != activeVersion {
				t.Errorf("restored application = %+v, want active version %d", app.Application, activeVersion)
			}
			versionOp := sdk.NewVersionOp(client, appID)
			versions, _, err := versionOp.List(ctx, 30, nil)
			if err != nil {
				t.Fatalf("ListVersions: %v", err)
			}
			if len(versions) != 2 || versions[0].Version != 3 || versions[1].Version != 2 {
				t.Errorf("restored versions = %+v, want 3 and 2", versions)
			}
			detail, err := versionOp.Read(ctx, activeVersion)
			if err != nil {
				t.Fatalf("ReadVersion: %v", err)
			}
			if detail.Image != "nginx:latest" || detail.CPU != 1000 || len(detail.ExposedPorts) != 1 || len(detail.EnvVars) != 1 {
				t.Errorf("restored version = %+v", detail)
			}

			asgs, err := client.ListAutoScalingGroups(ctx, v1.ListAutoScalingGroupsParams{ClusterID: clusterID, MaxItems: 10})
			if err != nil {
				t.Fatalf("ListAutoScalingGroups: %v", err)
			}
			if len(asgs.AutoScalingGroups) != 1 || asgs.AutoScalingGroups[0].GetAutoScalingGroupID() != asgID {
				t.Errorf("restored auto scaling groups = %+v, want only %v", asgs.AutoScalingGroups, asgID)
			}
			wns, err := client.ListWorkerNodes(ctx, v1.ListWorkerNodesParams{ClusterID: clusterID, AutoScalingGroupID: asgID, MaxItems: 10})
			if err != nil {
				t.Fatalf("ListWorkerNodes: %v", err)
			}
			if len(wns.WorkerNodes) != 2 || wns.WorkerNodes[0].WorkerNodeID != workerNodeIDs[0] || wns.WorkerNodes[1].WorkerNodeID != workerNodeIDs[1] ||
				wns.WorkerNodes[0].Draining || !wns.WorkerNodes[1].Draining {
				t.Errorf("restored worker nodes = %+v, want %v with the second one draining", wns.WorkerNodes, workerNodeIDs)
			}

			lb, err := client.ReadLoadBalancer(ctx, v1.ReadLoadBalancerParams{ClusterID: clusterID, AutoScalingGroupID: asgID, LoadBalancerID: lbID})
			if err != nil {
				t.Fatalf("ReadLoadBalancer: %v", err)
			}
			if lb.LoadBalancer.GetName() != "test-lb" {
				t.Errorf("restored load balancer = %+v", lb.LoadBalancer)
			}
			lbNodes, err := client.ListLoadBalancerNodes(ctx, v1.ListLoadBalancerNodesParams{ClusterID: clusterID, AutoScalingGroupID: asgID, LoadBalancerID: lbID, MaxItems: 10})
			if err != nil {
				t.Fatalf("ListLoadBalancerNodes: %v", err)
			}
			if len(lbNodes.LoadBalancerNodes) != len(lbNodeIDs) || lbNodes.LoadBalancerNodes[0].LoadBalancerNodeID != lbNodeIDs[0] {
				t.Errorf("restored load balancer nodes = %+v, want %v", lbNodes.LoadBalancerNodes, lbNodeIDs)
			}

			cert, err := client.ReadCertificate(ctx, v1.ReadCertificateParams{ClusterID: clusterID, CertificateID: certID})
			if err != nil {
				t.Fatalf("ReadCertificate: %v", err)
			}
			if cert.Certificate.GetName() != "test-cert" {
				t.Errorf("restored certificate = %+v", cert.Certificate)
			}
		})
}

// A version created after a restart never reuses a version number, not even
// the one of a version deleted before the restart.
func TestStateRestoredVersionNumbersAreNotReused(t *testing.T) {
	var appID v1.ApplicationID
	coretest.StateRestart(t, apprundedicated.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			appResp, err := client.CreateApplication(ctx, &v1.CreateApplicationRequest{Name: "test-app", ClusterID: createTestCluster(t, ctx, client)})
			if err != nil {
				t.Fatalf("CreateApplication: %v", err)
			}
			appID = appResp.Application.GetApplicationID()
			createNginxVersion(t, ctx, client, appID)
			second := createNginxVersion(t, ctx, client, appID)
			if err := sdk.NewVersionOp(client, appID).Delete(ctx, second); err != nil {
				t.Fatalf("DeleteVersion: %v", err)
			}
		},
		func(t *testing.T, url string) {
			if got := createNginxVersion(t, t.Context(), newTestClient(t, url), appID); got != 3 {
				t.Errorf("version created after restart = %d, want 3", got)
			}
		}, true)
}

// With the data plane enabled, a restored application with an active version
// is served again: its container is restarted, whether the previous process
// removed its containers on shutdown or was killed and left them behind. An
// application without an active version gets none.
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
			start := func() (client *v1.Client, dpAddr string, stop func()) {
				t.Helper()
				dpAddr = freeAddr(t)
				srv, err := apprundedicated.Config{EnableDataPlane: true, DataPlaneAddr: dpAddr}.NewServer(core.ServerOptions{StateDir: dir})
				if err != nil {
					t.Fatal(err)
				}
				ts := httptest.NewServer(srv)
				stop = sync.OnceFunc(func() {
					ts.Close()
					srv.Close()
				})
				t.Cleanup(stop)
				return newTestClient(t, ts.URL), dpAddr, stop
			}

			client, dpAddr, stop := start()
			clusterID := createTestCluster(t, ctx, client)
			createTestASG(t, ctx, client, clusterID, "test-asg")
			var appIDs [2]v1.ApplicationID
			for i, name := range []string{"active", "inactive"} {
				resp, err := client.CreateApplication(ctx, &v1.CreateApplicationRequest{Name: name, ClusterID: clusterID})
				if err != nil {
					t.Fatalf("CreateApplication: %v", err)
				}
				appIDs[i] = resp.Application.GetApplicationID()
				createNginxVersion(t, ctx, client, appIDs[i])
			}
			active, inactive := uuid.UUID(appIDs[0]).String(), uuid.UUID(appIDs[1]).String()
			activateVersion(t, ctx, client, appIDs[0], 1)
			expectDataPlane(ctx, t, dpAddr, active, http.StatusOK)
			// A killed process removes nothing: its container is still there when
			// the next one starts.
			if !tt.killed {
				stop()
			}

			client, dpAddr, _ = start()
			expectDataPlane(ctx, t, dpAddr, active, http.StatusOK)
			expectDataPlane(ctx, t, dpAddr, inactive, http.StatusBadGateway)

			// The API reports the restarted container as running.
			nodes, err := client.ListApplicationContainers(ctx, v1.ListApplicationContainersParams{ApplicationID: appIDs[0]})
			if err != nil {
				t.Fatalf("ListApplicationContainers: %v", err)
			}
			if len(nodes.Nodes) == 0 {
				t.Fatal("no container placement reported after restart")
			}
			for _, n := range nodes.Nodes {
				for _, c := range n.ContainersStats.Value.Containers {
					if c.State != "running" {
						t.Errorf("container %s on node %v is %s after restart, want running", c.ID, n.NodeID, c.State)
					}
				}
			}
		})
	}
}

// expectDataPlane fails unless the data plane at dpAddr answers the
// application's requests with the given status, allowing its container a few
// seconds to come up.
func expectDataPlane(ctx context.Context, t *testing.T, dpAddr, appID string, want int) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	var last string
	for range 30 {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+dpAddr+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Host = appID + ".localhost:" + portOfAddr(dpAddr)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == want {
				return
			}
			last = resp.Status
		} else {
			last = err.Error()
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("data plane on %s answers %s for application %s, want %d", dpAddr, last, appID, want)
}
