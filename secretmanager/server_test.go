package secretmanager_test

import (
	"fmt"
	"slices"
	"sort"
	"testing"

	sm "github.com/sacloud/sacloud-sdk-go/api/secretmanager"
	v1 "github.com/sacloud/sacloud-sdk-go/api/secretmanager/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/secretmanager"
)

const testVaultID = "test-vault-123"

// closeAndCheck closes srv and fails the test if any response drifted from the
// OpenAPI spec (go test swallows the WARN logs).
func closeAndCheck(t *testing.T, srv *secretmanager.Server) {
	t.Helper()
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
	srv.Close()
}

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
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	secrets, err := secOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets, got %d", len(secrets))
	}

	created, err := secOp.Create(ctx, v1.CreateSecretRequest{Name: "foo", Value: "bar"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "foo" || created.LatestVersion != 1 {
		t.Fatalf("unexpected create response: %+v", created)
	}

	secrets, err = secOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(secrets))
	}
	if secrets[0].Name != "foo" || secrets[0].LatestVersion != 1 {
		t.Fatalf("unexpected list item: %+v", secrets[0])
	}

	unveiled, err := secOp.Unveil(ctx, sm.UnveilParams{Name: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if unveiled.Name != "foo" || unveiled.Version != 1 || unveiled.Value != "bar" {
		t.Fatalf("unexpected unveil response: %+v", unveiled)
	}

	// Update secret "foo" (create v2)
	updated, err := secOp.Update(ctx, v1.CreateSecretRequest{Name: "foo", Value: "baz"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LatestVersion != 2 {
		t.Fatalf("expected version 2, got %d", updated.LatestVersion)
	}

	unveiledV1, err := secOp.Unveil(ctx, sm.UnveilParams{
		Name:    "foo",
		Version: new(1),
	})
	if err != nil {
		t.Fatal(err)
	}
	if unveiledV1.Value != "bar" || unveiledV1.Version != 1 {
		t.Fatalf("expected v1 value 'bar', got: %+v", unveiledV1)
	}

	// Unveil latest (should be v2)
	unveiledLatest, err := secOp.Unveil(ctx, sm.UnveilParams{Name: "foo"})
	if err != nil {
		t.Fatal(err)
	}
	if unveiledLatest.Value != "baz" || unveiledLatest.Version != 2 {
		t.Fatalf("expected v2 value 'baz', got: %+v", unveiledLatest)
	}

	if err := secOp.Delete(ctx, "foo"); err != nil {
		t.Fatal(err)
	}

	secrets, err = secOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets) != 0 {
		t.Fatalf("expected 0 secrets after delete, got %d", len(secrets))
	}
}

func TestUnveilNotFound(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	_, err := secOp.Unveil(ctx, sm.UnveilParams{Name: "nonexistent"})
	if err == nil {
		t.Fatal("expected error for non-existent secret")
	}
}

func TestDeleteNotFound(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	err := secOp.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent secret")
	}
}

func TestMultipleSecrets(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		_, err := secOp.Create(ctx, v1.CreateSecretRequest{Name: name, Value: "value-" + name})
		if err != nil {
			t.Fatal(err)
		}
	}

	secrets, err := secOp.List(ctx, nil, nil)
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
	defer closeAndCheck(t, srv)
	ctx := t.Context()

	secOp1 := newTestSecretOp(t, srv.TestURL(), "vault-1")
	secOp2 := newTestSecretOp(t, srv.TestURL(), "vault-2")

	if _, err := secOp1.Create(ctx, v1.CreateSecretRequest{Name: "secret1", Value: "value1"}); err != nil {
		t.Fatal(err)
	}

	// vault-2 should be empty
	secrets2, err := secOp2.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets2) != 0 {
		t.Fatalf("vault-2 should be empty, got %d", len(secrets2))
	}

	// vault-1 should have 1
	secrets1, err := secOp1.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(secrets1) != 1 {
		t.Fatalf("vault-1 should have 1 secret, got %d", len(secrets1))
	}
}

func TestListSecretsPaging(t *testing.T) {
	srv := secretmanager.NewTestServer(secretmanager.Config{})
	defer closeAndCheck(t, srv)
	ctx := t.Context()
	secOp := newTestSecretOp(t, srv.TestURL(), testVaultID)

	for _, name := range []string{"a", "b", "c", "d", "e"} {
		if _, err := secOp.Create(ctx, v1.CreateSecretRequest{Name: name, Value: "v"}); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		count, from *int
		want        []string
	}{
		{nil, nil, []string{"a", "b", "c", "d", "e"}},
		{new(2), nil, []string{"a", "b"}},
		{new(2), new(3), []string{"d", "e"}},
		{nil, new(4), []string{"e"}},
		{new(2), new(10), nil},
	}
	for _, tt := range tests {
		secrets, err := secOp.List(ctx, tt.count, tt.from)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, s := range secrets {
			got = append(got, s.Name)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("List(count=%v, from=%v) = %v, want %v", tt.count, tt.from, got, tt.want)
		}
	}
}

func TestSecretVersionRetention(t *testing.T) {
	// Exercised on the store directly: 51 SDK round trips are slow.
	store := secretmanager.NewMemoryStore(nil)
	for i := 1; i <= 51; i++ {
		if _, err := store.Create(testVaultID, "foo", fmt.Sprint(i)); err != nil {
			t.Fatal(err)
		}
	}

	// Only the latest 50 versions (2..51) are retained.
	if _, _, err := store.Unveil(testVaultID, "foo", 1); err == nil {
		t.Error("expected version 1 to be dropped after 51 versions")
	}
	value, _, err := store.Unveil(testVaultID, "foo", 2)
	if err != nil {
		t.Fatalf("unveil version 2: %v", err)
	}
	if value != "2" {
		t.Errorf("version 2 value = %q, want 2", value)
	}
}
