package secretmanager_test

import (
	"testing"

	sm "github.com/sacloud/sacloud-sdk-go/api/secretmanager"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/secretmanager"
)

func newTestVaultOp(t *testing.T, serverURL string) sm.VaultAPI {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_SECRETMANAGER=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := sm.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return sm.NewVaultOp(client)
}

func TestVaultLifecycle(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	vaultOp := newTestVaultOp(t, srv.TestURL())

	created, err := vaultOp.Create(ctx, sm.CreateVaultParams{
		Name:        "my-vault",
		KmsKeyID:    "990000000123",
		Description: new("desc"),
		Tags:        []string{"a", "b"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.ID.Or("") == "" {
		t.Fatal("expected a non-empty vault ID")
	}
	if created.Name != "my-vault" || created.KmsKeyID != "990000000123" {
		t.Errorf("unexpected create result: %+v", created)
	}
	id := created.ID.Value

	got, err := vaultOp.Read(ctx, id)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.ID != id || got.Name != "my-vault" || got.KmsKeyID != "990000000123" {
		t.Errorf("unexpected read result: %+v", got)
	}

	list, err := vaultOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, v := range list {
		if v.ID == id {
			found = true
		}
	}
	if !found {
		t.Errorf("created vault %s not found in list of %d", id, len(list))
	}

	updated, err := vaultOp.Update(ctx, id, sm.UpdateVaultParams{
		Name: "renamed",
		Tags: []string{"c"},
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Name != "renamed" {
		t.Errorf("update name = %q, want renamed", updated.Name)
	}

	if err := vaultOp.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := vaultOp.Read(ctx, id); err == nil {
		t.Error("expected read after delete to fail")
	}
}
