package cloudhsm_test

import (
	"testing"

	cloudhsmsdk "github.com/sacloud/sacloud-sdk-go/api/cloudhsm"
	v1 "github.com/sacloud/sacloud-sdk-go/api/cloudhsm/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/cloudhsm"
)

func newTestClient(t *testing.T, serverURL string) *v1.Client {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_CLOUDHSM=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := cloudhsmsdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func closeAndCheck(t *testing.T, srv *cloudhsm.Server) {
	t.Helper()
	srv.Close()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
}

func TestCloudHSMLifecycle(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	hsmOp := cloudhsmsdk.NewCloudHSMOp(client)

	// List: initially empty
	hsms, err := hsmOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hsms) != 0 {
		t.Fatalf("expected 0 cloudhsms, got %d", len(hsms))
	}

	// Create
	created, err := hsmOp.Create(ctx, cloudhsmsdk.CloudHSMCreateParams{
		Name:               "test-hsm",
		Ipv4NetworkAddress: "192.168.100.0",
		Ipv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-hsm" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	if created.Availability != v1.AvailabilityEnumAvailable {
		t.Fatalf("unexpected availability: %s", created.Availability)
	}
	id := created.ID

	// List: 1 item
	hsms, err = hsmOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hsms) != 1 {
		t.Fatalf("expected 1 cloudhsm, got %d", len(hsms))
	}

	// Read
	read, err := hsmOp.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-hsm" || read.ID != id {
		t.Fatalf("unexpected read response: %+v", read)
	}
	if read.Ipv4Address == "" {
		t.Fatal("expected non-empty Ipv4Address")
	}

	// Update
	updated, err := hsmOp.Update(ctx, id, cloudhsmsdk.CloudHSMUpdateParams{
		Name:               "updated-hsm",
		Ipv4NetworkAddress: "192.168.100.0",
		Ipv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-hsm" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	// Delete
	if err := hsmOp.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	hsms, err = hsmOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hsms) != 0 {
		t.Fatalf("expected 0 cloudhsms after delete, got %d", len(hsms))
	}
}

func TestCloudHSMReadNotFound(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	hsmOp := cloudhsmsdk.NewCloudHSMOp(newTestClient(t, srv.TestURL()))

	if _, err := hsmOp.Read(ctx, "999999999999"); err == nil {
		t.Fatal("expected error for non-existent cloudhsm")
	}
}

func TestClientLifecycle(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	hsmOp := cloudhsmsdk.NewCloudHSMOp(client)

	created, err := hsmOp.Create(ctx, cloudhsmsdk.CloudHSMCreateParams{
		Name:               "client-test-hsm",
		Ipv4NetworkAddress: "192.168.101.0",
		Ipv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	hsm, err := hsmOp.Read(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	clientOp, err := cloudhsmsdk.NewClientOp(client, hsm)
	if err != nil {
		t.Fatal(err)
	}

	// List: initially empty
	clients, err := clientOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 0 {
		t.Fatalf("expected 0 clients, got %d", len(clients))
	}

	// Create
	createdClient, err := clientOp.Create(ctx, cloudhsmsdk.CloudHSMClientCreateParams{
		Name:        "client1",
		Certificate: "-----BEGIN CERTIFICATE-----\nMIIC...\n-----END CERTIFICATE-----",
	})
	if err != nil {
		t.Fatal(err)
	}
	if createdClient.Name != "client1" {
		t.Fatalf("unexpected name: %s", createdClient.Name)
	}
	clientID := createdClient.ID

	// Read
	readClient, err := clientOp.Read(ctx, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if readClient.Name != "client1" {
		t.Fatalf("unexpected read response: %+v", readClient)
	}

	// Update
	updatedClient, err := clientOp.Update(ctx, clientID, cloudhsmsdk.CloudHSMClientUpdateParams{Name: "client1-renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if updatedClient.Name != "client1-renamed" {
		t.Fatalf("unexpected update response: %+v", updatedClient)
	}

	// Delete
	if err := clientOp.Delete(ctx, clientID); err != nil {
		t.Fatal(err)
	}
	clients, err = clientOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 0 {
		t.Fatalf("expected 0 clients after delete, got %d", len(clients))
	}
}

func TestPeerLifecycle(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	hsmOp := cloudhsmsdk.NewCloudHSMOp(client)

	created, err := hsmOp.Create(ctx, cloudhsmsdk.CloudHSMCreateParams{
		Name:               "peer-test-hsm",
		Ipv4NetworkAddress: "192.168.102.0",
		Ipv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	hsm, err := hsmOp.Read(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}

	peerOp, err := cloudhsmsdk.NewPeerOp(client, hsm)
	if err != nil {
		t.Fatal(err)
	}

	// List: initially empty
	peers, err := peerOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(peers))
	}

	// Create
	peerID := "110000000099"
	if err := peerOp.Create(ctx, cloudhsmsdk.CloudHSMPeerCreateParams{RouterID: peerID, SecretKey: "pairing-secret"}); err != nil {
		t.Fatal(err)
	}

	peers, err = peerOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers))
	}
	if peers[0].ID != peerID {
		t.Fatalf("unexpected peer id: %s", peers[0].ID)
	}

	// Delete
	if err := peerOp.Delete(ctx, peerID); err != nil {
		t.Fatal(err)
	}
	peers, err = peerOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers after delete, got %d", len(peers))
	}
}

func TestLicenseLifecycle(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	licenseOp := cloudhsmsdk.NewLicenseOp(client)

	// List: initially empty
	licenses, err := licenseOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(licenses) != 0 {
		t.Fatalf("expected 0 licenses, got %d", len(licenses))
	}

	// Create
	created, err := licenseOp.Create(ctx, cloudhsmsdk.CloudHSMSoftwareLicenseCreateParams{Name: "license1"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "license1" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	id := created.ID

	// Read
	read, err := licenseOp.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "license1" {
		t.Fatalf("unexpected read response: %+v", read)
	}

	// Update
	updated, err := licenseOp.Update(ctx, id, cloudhsmsdk.CloudHSMSoftwareLicenseUpdateParams{Name: "license1-renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "license1-renamed" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	// Delete
	if err := licenseOp.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	licenses, err = licenseOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(licenses) != 0 {
		t.Fatalf("expected 0 licenses after delete, got %d", len(licenses))
	}
}
