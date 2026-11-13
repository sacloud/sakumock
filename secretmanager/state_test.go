package secretmanager_test

import (
	"testing"

	sm "github.com/sacloud/sacloud-sdk-go/api/secretmanager"
	v1 "github.com/sacloud/sacloud-sdk-go/api/secretmanager/apis/v1"

	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/secretmanager"
)

func TestStateSurvivesRestart(t *testing.T) {
	var vaultID string

	coretest.StateRestart(t, secretmanager.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			vault, err := newTestVaultOp(t, url).Create(ctx, sm.CreateVaultParams{
				Name:        "persisted",
				KmsKeyID:    "990000000123",
				Description: new("desc"),
				Tags:        []string{"a", "b"},
			})
			if err != nil {
				t.Fatal(err)
			}
			vaultID = vault.ID.Value
			secOp := newTestSecretOp(t, url, vaultID)
			for _, value := range []string{"v1", "v2"} {
				if _, err := secOp.Create(ctx, v1.CreateSecretRequest{Name: "foo", Value: value}); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := secOp.Create(ctx, v1.CreateSecretRequest{Name: "deleted", Value: "x"}); err != nil {
				t.Fatal(err)
			}
			if err := secOp.Delete(ctx, "deleted"); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			vault, err := newTestVaultOp(t, url).Read(ctx, vaultID)
			if err != nil {
				t.Fatal(err)
			}
			if vault.Name != "persisted" || vault.KmsKeyID != "990000000123" || len(vault.Tags) != 2 {
				t.Errorf("restored vault = %+v", vault)
			}
			secOp := newTestSecretOp(t, url, vaultID)
			secrets, err := secOp.List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(secrets) != 1 || secrets[0].Name != "foo" || secrets[0].LatestVersion != 2 {
				t.Errorf("restored secrets = %+v, want only foo at version 2", secrets)
			}
			latest, err := secOp.Unveil(ctx, sm.UnveilParams{Name: "foo"})
			if err != nil {
				t.Fatal(err)
			}
			if latest.Version != 2 || latest.Value != "v2" {
				t.Errorf("restored latest = %+v, want v2", latest)
			}
			// Older versions are restored too.
			first, err := secOp.Unveil(ctx, sm.UnveilParams{Name: "foo", Version: new(1)})
			if err != nil {
				t.Fatal(err)
			}
			if first.Value != "v1" {
				t.Errorf("restored version 1 = %q, want v1", first.Value)
			}
		})
}

// A vault created after a restart never reuses a restored ID.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var vaultID string
	create := func(t *testing.T, url string) string {
		t.Helper()
		vault, err := newTestVaultOp(t, url).Create(t.Context(), sm.CreateVaultParams{Name: "v", KmsKeyID: "990000000123"})
		if err != nil {
			t.Fatal(err)
		}
		return vault.ID.Value
	}
	coretest.StateRestart(t, secretmanager.Config{},
		func(t *testing.T, url string) { vaultID = create(t, url) },
		func(t *testing.T, url string) {
			if id := create(t, url); id == vaultID {
				t.Errorf("new vault reused the restored ID %s", id)
			}
		}, true)
}
