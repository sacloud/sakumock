package cloudhsm_test

import (
	"slices"
	"testing"

	cloudhsmsdk "github.com/sacloud/sacloud-sdk-go/api/cloudhsm"

	"github.com/sacloud/sakumock/cloudhsm"
	"github.com/sacloud/sakumock/core/coretest"
)

const (
	stateCertificate = "-----BEGIN CERTIFICATE-----\nMIIC...\n-----END CERTIFICATE-----"
	statePeerID      = "110000000099"
)

func TestStateSurvivesRestart(t *testing.T) {
	var hsmID, clientID, licenseID, docID string

	coretest.StateRestart(t, cloudhsm.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			hsmOp := cloudhsmsdk.NewCloudHSMOp(client)
			created, err := hsmOp.Create(ctx, cloudhsmsdk.CloudHSMCreateParams{
				Name:               "persisted",
				Description:        new("desc"),
				Tags:               []string{"a", "b"},
				IPv4NetworkAddress: "192.168.100.0",
				IPv4PrefixLength:   24,
			})
			if err != nil {
				t.Fatal(err)
			}
			hsmID = created.ID.Value
			hsm, err := hsmOp.Read(ctx, hsmID)
			if err != nil {
				t.Fatal(err)
			}

			clientOp, err := cloudhsmsdk.NewClientOp(client, hsm)
			if err != nil {
				t.Fatal(err)
			}
			createdClient, err := clientOp.Create(ctx, cloudhsmsdk.CloudHSMClientCreateParams{Name: "client1", Certificate: stateCertificate})
			if err != nil {
				t.Fatal(err)
			}
			clientID = createdClient.ID

			peerOp, err := cloudhsmsdk.NewPeerOp(client, hsm)
			if err != nil {
				t.Fatal(err)
			}
			if err := peerOp.Create(ctx, cloudhsmsdk.CloudHSMPeerCreateParams{RouterID: statePeerID, SecretKey: "pairing-secret"}); err != nil {
				t.Fatal(err)
			}

			license, err := cloudhsmsdk.NewLicenseOp(client).Create(ctx, cloudhsmsdk.CloudHSMSoftwareLicenseCreateParams{
				Name: "license1",
				Tags: []string{"l"},
			})
			if err != nil {
				t.Fatal(err)
			}
			licenseID = license.ID.Value
			docs, err := cloudhsmsdk.NewDocumentOp(client, licenseID).List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(docs) != 1 {
				t.Fatalf("got %d documents, want 1", len(docs))
			}
			docID = docs[0].ID
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			client := newTestClient(t, url)
			hsm, err := cloudhsmsdk.NewCloudHSMOp(client).Read(ctx, hsmID)
			if err != nil {
				t.Fatal(err)
			}
			if hsm.Name != "persisted" || hsm.Description != "desc" || !slices.Equal(hsm.Tags, []string{"a", "b"}) ||
				hsm.IPv4NetworkAddress != "192.168.100.0" || hsm.IPv4PrefixLength != 24 || hsm.IPv4Address != "192.168.100.1" {
				t.Errorf("restored cloudhsm = %+v", hsm)
			}

			clientOp, err := cloudhsmsdk.NewClientOp(client, hsm)
			if err != nil {
				t.Fatal(err)
			}
			gotClient, err := clientOp.Read(ctx, clientID)
			if err != nil {
				t.Fatal(err)
			}
			if gotClient.Name != "client1" || gotClient.Certificate != stateCertificate {
				t.Errorf("restored client = %+v", gotClient)
			}

			peerOp, err := cloudhsmsdk.NewPeerOp(client, hsm)
			if err != nil {
				t.Fatal(err)
			}
			peers, err := peerOp.List(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(peers) != 1 || peers[0].ID != statePeerID || peers[0].SecretKey != "pairing-secret" {
				t.Errorf("restored peers = %+v", peers)
			}

			license, err := cloudhsmsdk.NewLicenseOp(client).Read(ctx, licenseID)
			if err != nil {
				t.Fatal(err)
			}
			if license.Name != "license1" || !slices.Equal(license.Tags, []string{"l"}) {
				t.Errorf("restored license = %+v", license)
			}
			docs, err := cloudhsmsdk.NewDocumentOp(client, licenseID).List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(docs) != 1 || docs[0].ID != docID {
				t.Errorf("restored documents = %+v, want only %s", docs, docID)
			}
		})
}

// Resources created after a restart never reuse a restored ID, whichever kind
// of resource held it.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	// create makes one resource of every kind with a generated ID and returns
	// those IDs.
	create := func(t *testing.T, url string) []string {
		t.Helper()
		ctx := t.Context()
		client := newTestClient(t, url)
		hsmOp := cloudhsmsdk.NewCloudHSMOp(client)
		created, err := hsmOp.Create(ctx, cloudhsmsdk.CloudHSMCreateParams{
			Name: "hsm", IPv4NetworkAddress: "192.168.100.0", IPv4PrefixLength: 24,
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
		createdClient, err := clientOp.Create(ctx, cloudhsmsdk.CloudHSMClientCreateParams{Name: "client", Certificate: stateCertificate})
		if err != nil {
			t.Fatal(err)
		}
		license, err := cloudhsmsdk.NewLicenseOp(client).Create(ctx, cloudhsmsdk.CloudHSMSoftwareLicenseCreateParams{Name: "license"})
		if err != nil {
			t.Fatal(err)
		}
		docs, err := cloudhsmsdk.NewDocumentOp(client, license.ID.Value).List(ctx, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		ids := []string{hsm.ID, createdClient.ID, license.ID.Value}
		for _, d := range docs {
			ids = append(ids, d.ID)
		}
		return ids
	}

	var restored []string
	coretest.StateRestart(t, cloudhsm.Config{},
		func(t *testing.T, url string) { restored = create(t, url) },
		func(t *testing.T, url string) {
			for _, id := range create(t, url) {
				if slices.Contains(restored, id) {
					t.Errorf("new resource reused the restored ID %s", id)
				}
			}
		}, true)
}
