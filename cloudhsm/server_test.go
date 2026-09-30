package cloudhsm_test

import (
	"slices"
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

	hsms, err := hsmOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hsms) != 0 {
		t.Fatalf("expected 0 cloudhsms, got %d", len(hsms))
	}

	created, err := hsmOp.Create(ctx, cloudhsmsdk.CloudHSMCreateParams{
		Name:               "test-hsm",
		IPv4NetworkAddress: "192.168.100.0",
		IPv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-hsm" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	if created.Availability.Value != v1.CreateCloudHSMAvailabilityAvailable {
		t.Fatalf("unexpected availability: %s", created.Availability.Value)
	}
	id := created.ID.Value

	hsms, err = hsmOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hsms) != 1 {
		t.Fatalf("expected 1 cloudhsm, got %d", len(hsms))
	}

	read, err := hsmOp.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-hsm" || read.ID != id {
		t.Fatalf("unexpected read response: %+v", read)
	}
	if read.IPv4Address == "" {
		t.Fatal("expected non-empty IPv4Address")
	}

	updated, err := hsmOp.Update(ctx, id, cloudhsmsdk.CloudHSMUpdateParams{
		Name:               "updated-hsm",
		IPv4NetworkAddress: "192.168.100.0",
		IPv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-hsm" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	if err := hsmOp.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	hsms, err = hsmOp.List(ctx, nil, nil)
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
		IPv4NetworkAddress: "192.168.101.0",
		IPv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	hsm, err := hsmOp.Read(ctx, created.ID.Value)
	if err != nil {
		t.Fatal(err)
	}

	clientOp, err := cloudhsmsdk.NewClientOp(client, hsm)
	if err != nil {
		t.Fatal(err)
	}

	clients, err := clientOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(clients) != 0 {
		t.Fatalf("expected 0 clients, got %d", len(clients))
	}

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

	readClient, err := clientOp.Read(ctx, clientID)
	if err != nil {
		t.Fatal(err)
	}
	if readClient.Name != "client1" {
		t.Fatalf("unexpected read response: %+v", readClient)
	}

	updatedClient, err := clientOp.Update(ctx, clientID, "client1-renamed")
	if err != nil {
		t.Fatal(err)
	}
	if updatedClient.Name != "client1-renamed" || updatedClient.Certificate != readClient.Certificate {
		t.Fatalf("unexpected update response: %+v", updatedClient)
	}

	if err := clientOp.Delete(ctx, clientID); err != nil {
		t.Fatal(err)
	}
	clients, err = clientOp.List(ctx, nil, nil)
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
		IPv4NetworkAddress: "192.168.102.0",
		IPv4PrefixLength:   24,
	})
	if err != nil {
		t.Fatal(err)
	}
	hsm, err := hsmOp.Read(ctx, created.ID.Value)
	if err != nil {
		t.Fatal(err)
	}

	peerOp, err := cloudhsmsdk.NewPeerOp(client, hsm)
	if err != nil {
		t.Fatal(err)
	}

	peers, err := peerOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 0 {
		t.Fatalf("expected 0 peers, got %d", len(peers))
	}

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
	if peers[0].ID != peerID || peers[0].SecretKey != "pairing-secret" {
		t.Fatalf("unexpected peer: %+v", peers[0])
	}

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

	licenses, err := licenseOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(licenses) != 0 {
		t.Fatalf("expected 0 licenses, got %d", len(licenses))
	}

	created, err := licenseOp.Create(ctx, cloudhsmsdk.CloudHSMSoftwareLicenseCreateParams{Name: "license1"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "license1" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	id := created.ID.Value

	read, err := licenseOp.Read(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "license1" {
		t.Fatalf("unexpected read response: %+v", read)
	}

	updated, err := licenseOp.Update(ctx, id, cloudhsmsdk.CloudHSMSoftwareLicenseUpdateParams{Name: "license1-renamed"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "license1-renamed" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	if err := licenseOp.Delete(ctx, id); err != nil {
		t.Fatal(err)
	}
	licenses, err = licenseOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(licenses) != 0 {
		t.Fatalf("expected 0 licenses after delete, got %d", len(licenses))
	}
}

func TestListPagination(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	licenseOp := cloudhsmsdk.NewLicenseOp(newTestClient(t, srv.TestURL()))

	var ids []string
	for _, name := range []string{"l1", "l2", "l3"} {
		created, err := licenseOp.Create(ctx, cloudhsmsdk.CloudHSMSoftwareLicenseCreateParams{Name: name})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, created.ID.Value)
	}

	for _, tc := range []struct {
		count, from *int
		want        []string
	}{
		{nil, nil, ids},
		{new(2), nil, ids[:2]},
		{new(2), new(2), ids[2:]},
		{nil, new(1), ids[1:]},
		{new(1), new(5), nil},
	} {
		licenses, err := licenseOp.List(ctx, tc.count, tc.from)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, l := range licenses {
			got = append(got, l.ID)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("List(count=%v, from=%v) = %v, want %v", tc.count, tc.from, got, tc.want)
		}
	}
}

func TestLicenseDocuments(t *testing.T) {
	srv := cloudhsm.NewTestServer(cloudhsm.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	client := newTestClient(t, srv.TestURL())
	licenseOp := cloudhsmsdk.NewLicenseOp(client)

	created, err := licenseOp.Create(ctx, cloudhsmsdk.CloudHSMSoftwareLicenseCreateParams{Name: "license-with-docs"})
	if err != nil {
		t.Fatal(err)
	}
	docOp := cloudhsmsdk.NewDocumentOp(client, created.ID.Value)

	docs, err := docOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("expected 1 document, got %d", len(docs))
	}

	dl, err := docOp.Download(ctx, docs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if u := dl.GetURL(); u.Scheme != "https" || u.Host == "" {
		t.Fatalf("unexpected download URL: %s", u.String())
	}

	if _, err := docOp.Download(ctx, "999999999999"); err == nil {
		t.Fatal("expected error for non-existent document")
	}
	if _, err := cloudhsmsdk.NewDocumentOp(client, "999999999999").List(ctx, nil, nil); err == nil {
		t.Fatal("expected error for non-existent license")
	}
}
