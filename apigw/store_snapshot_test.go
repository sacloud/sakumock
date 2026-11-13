package apigw

import (
	"strings"
	"testing"
	"time"
)

// The certificate PEMs are write-only in the API, so no client can observe
// whether they survive a restart.
func TestSnapshotKeepsCertificatePEM(t *testing.T) {
	src := NewMemoryStore(nil)
	want := CertificateDetails{ExpiredAt: time.Now().UTC().Add(time.Hour), CertPEM: "cert-pem", KeyPEM: "key-pem"}
	created, err := src.CreateCertificate(Certificate{Name: "c", ECDSA: &want})
	if err != nil {
		t.Fatal(err)
	}
	data, err := src.Snapshot()
	if err != nil {
		t.Fatal(err)
	}

	dst := NewMemoryStore(nil)
	if err := dst.Restore(data); err != nil {
		t.Fatal(err)
	}
	got := dst.certificates[created.ID]
	if got == nil || got.RSA != nil || got.ECDSA == nil ||
		got.ECDSA.CertPEM != want.CertPEM || got.ECDSA.KeyPEM != want.KeyPEM || !got.ECDSA.ExpiredAt.Equal(want.ExpiredAt) {
		t.Errorf("restored certificate = %+v, want ecdsa %+v", got, want)
	}
}

func TestRestoreRejectsDanglingReferences(t *testing.T) {
	planID := seedPlans()[0].ID
	sub := `"subscriptions": {"s1": {"id": "s1", "name": "sub", "planId": "` + planID + `", "resourceId": 990000000001}}`
	svc := func(id string) string {
		return `"` + id + `": {"id": "` + id + `", "name": "` + id + `", "subscriptionId": "s1"}`
	}
	tests := map[string]struct{ snapshot, wantErr string }{
		"record under another id":        {`{"groups": {"g1": {"id": "g2"}}}`, "different id"},
		"subscription to unknown plan":   {`{"subscriptions": {"s1": {"id": "s1", "planId": "nope"}}}`, "unknown plan"},
		"service without subscription":   {`{"services": {` + svc("v1") + `}}`, "unknown subscription"},
		"subscription bound twice":       {`{` + sub + `, "services": {` + svc("v1") + `, ` + svc("v2") + `}}`, "bound to both"},
		"route without its service":      {`{"routes": {"r1": {"id": "r1", "serviceId": "v1"}}}`, "unknown service"},
		"user record without its fields": {`{"users": {"u1": {}}}`, "missing"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			s := NewMemoryStore(nil)
			if _, err := s.CreateGroup(Group{Name: "kept"}); err != nil {
				t.Fatal(err)
			}
			err := s.Restore([]byte(tt.snapshot))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Restore error = %v, want it to contain %q", err, tt.wantErr)
			}
			if len(s.groups) != 1 {
				t.Error("a rejected snapshot must leave the store untouched")
			}
		})
	}
}
