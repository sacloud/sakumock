package secretmanager_test

import (
	"sort"
	"testing"

	"github.com/sacloud/saclient-go"
	sm "github.com/sacloud/secretmanager-api-go"
	v1 "github.com/sacloud/secretmanager-api-go/apis/v1"

	"github.com/sacloud/sakumock/secretmanager"
)

const testVaultID = "test-vault-123"

func newTestSecretOp(t *testing.T, serverURL, vaultID string) sm.SecretAPI {
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
	return sm.NewSecretOp(client, vaultID)
}

func TestSecretLifecycle(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer srv.Close()
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	// List: initially empty
	secrets, err := secOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets, got %d", len(secrets))
	}

	// Create secret "foo"
	created, err := secOp.Create(ctx, v1.CreateSecret{Name: "foo", Value: "bar"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "foo" || created.LatestVersion != 1 {
		t.Fatalf("unexpected create response: %+v", created)
	}

	// List: 1 secret
	secrets, err = secOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	if secrets[0].Name != "foo" || secrets[0].LatestVersion != 1 {
		t.Fatalf("unexpected list item: %+v", secrets[0])
	}

	// Unveil secret "foo" (latest)
	unveiled, err := secOp.Unveil(ctx, v1.Unveil{Name: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if unveiled.Name != "foo" || unveiled.Version != v1.NewOptNilInt(1) || unveiled.Value != "bar" {
		t.Fatalf("unexpected unveil response: %+v", unveiled)
	}

	// Update secret "foo" (create v2)
	updated, err := secOp.Update(ctx, v1.CreateSecret{Name: "foo", Value: "baz"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LatestVersion != 2 {
		t.Fatalf("expected version 2, got %d", updated.LatestVersion)
	}

	// Unveil version 1
	unveiledV1, err := secOp.Unveil(ctx, v1.Unveil{
		Name:    "foo",
		Version: v1.NewOptNilInt(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if unveiledV1.Value != "bar" || unveiledV1.Version != v1.NewOptNilInt(1) {
		t.Fatalf("expected v1 value 'bar', got: %+v", unveiledV1)
	}

	// Unveil latest (should be v2)
	unveiledLatest, err := secOp.Unveil(ctx, v1.Unveil{Name: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if unveiledLatest.Value != "baz" || unveiledLatest.Version != v1.NewOptNilInt(2) {
		t.Fatalf("expected v2 value 'baz', got: %+v", unveiledLatest)
	}

	// Delete secret "foo"
	if err := secOp.Delete(ctx, v1.DeleteSecret{Name: "foo"}); err != nil {
		t.Fatal(err)
	}

	// List: empty again
	secrets, err = secOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets after delete, got %d", len(secrets))
	}
}

func TestUnveilNotFound(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer srv.Close()
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	_, err := secOp.Unveil(ctx, v1.Unveil{Name: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent secret")
	}
}

func TestDeleteNotFound(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer srv.Close()
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	err := secOp.Delete(ctx, v1.DeleteSecret{Name: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent secret")
	}
}

func TestMultipleSecrets(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer srv.Close()
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		_, err := secOp.Create(ctx, v1.CreateSecret{Name: name, Value: "value-" + name})
		if err != nil {
			t.Fatal(err)
		}
	}

	secrets, err := secOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 3 {
		t.Fatalf("expected 3 secrets, got %d", len(secrets))
	}
	sort.Slice(secrets, func(i, j int) bool { return secrets[i].Name < secrets[j].Name })
	if secrets[0].Name != "alpha" || secrets[1].Name != "beta" || secrets[2].Name != "gamma" {
		t.Fatalf("unexpected secrets: %+v", secrets)
	}
}

func TestDifferentVaults(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer srv.Close()
	ctx := t.Context()

	secOp1 := newTestSecretOp(t, srv.TestURL(), "vault-1")
	secOp2 := newTestSecretOp(t, srv.TestURL(), "vault-2")

	if _, err := secOp1.Create(ctx, v1.CreateSecret{Name: "secret1", Value: "value1"}); err != nil {
		t.Fatal(err)
	}

	// vault-2 should be empty
	secrets2, err := secOp2.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets2) != 0 {
		t.Fatalf("vault-2 should be empty, got %d", len(secrets2))
	}

	// vault-1 should have 1
	secrets1, err := secOp1.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets1) != 1 {
		t.Fatalf("vault-1 should have 1 secret, got %d", len(secrets1))
	}
}
