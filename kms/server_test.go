package kms_test

import (
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	ogenvalidate "github.com/ogen-go/ogen/validate"
	kmssdk "github.com/sacloud/sacloud-sdk-go/api/kms"
	v1 "github.com/sacloud/sacloud-sdk-go/api/kms/apis/v1"
	"github.com/sacloud/sacloud-sdk-go/common/saclient"

	"github.com/sacloud/sakumock/core"
	"github.com/sacloud/sakumock/kms"
)

func newTestKeyOp(t *testing.T, serverURL string) kmssdk.KeyAPI {
	t.Helper()
	var sa saclient.Client
	if err := sa.SetEnviron([]string{
		"SAKURA_ENDPOINTS_KMS=" + serverURL,
		"SAKURA_ACCESS_TOKEN=dummy",
		"SAKURA_ACCESS_TOKEN_SECRET=dummy",
	}); err != nil {
		t.Fatal(err)
	}
	client, err := kmssdk.NewClient(&sa)
	if err != nil {
		t.Fatal(err)
	}
	return kmssdk.NewKeyOp(client)
}

func TestKeyLifecycle(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	keys, err := keyOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(keys))
	}

	created, err := keyOp.Create(ctx, kmssdk.CreateParams{
		Name: "test-key",
		Tags: []string{"env:test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-key" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	if created.KeyOrigin != v1.CreateKeyResponseKeyOriginGenerated {
		t.Fatalf("unexpected origin: %s", created.KeyOrigin)
	}
	keyID := created.ID

	keys, err = keyOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].Name != "test-key" {
		t.Fatalf("unexpected name: %s", keys[0].Name)
	}
	if keys[0].Status != v1.KeyStatusActive {
		t.Fatalf("unexpected status: %s", keys[0].Status)
	}

	read, err := keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-key" || read.ID != keyID {
		t.Fatalf("unexpected read response: %+v", read)
	}

	updated, err := keyOp.Update(ctx, keyID, kmssdk.UpdateParams{
		Name:        "updated-key",
		Description: new("updated description"),
		Tags:        []string{"env:prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-key" || updated.Description != "updated description" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	read, err = keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "updated-key" {
		t.Fatalf("expected updated name, got: %s", read.Name)
	}
	if len(read.Tags) != 1 || read.Tags[0] != "env:prod" {
		t.Fatalf("unexpected tags: %v", read.Tags)
	}

	if err := keyOp.Delete(ctx, keyID); err != nil {
		t.Fatal(err)
	}

	keys, err = keyOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys after delete, got %d", len(keys))
	}

	// The handlers above must not have drifted from the OpenAPI spec.
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations recorded: %+v", v)
	}
}

func TestReadNotFound(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	_, err := keyOp.Read(ctx, "999999999999")
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}

func TestDeleteNotFound(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	err := keyOp.Delete(ctx, "999999999999")
	if err == nil {
		t.Fatal("expected error for non-existent key")
	}
}

func TestMultipleKeys(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	for _, name := range []string{"key-alpha", "key-beta", "key-gamma"} {
		_, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: name})
		if err != nil {
			t.Fatal(err)
		}
	}

	keys, err := keyOp.List(ctx, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}

func TestRotateKey(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "rotate-test"})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	rotated, err := keyOp.Rotate(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.LatestVersion != 2 {
		t.Fatalf("expected version 2 after rotate, got %v", rotated.LatestVersion)
	}
}

func TestChangeStatus(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "status-test"})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	status, err := keyOp.ChangeStatus(ctx, keyID, v1.ChangeKeyStateRequestStatusRestricted)
	if err != nil {
		t.Fatal(err)
	}
	if status != v1.ChangeKeyStateStatusRestricted {
		t.Fatalf("expected restricted in response, got %s", status)
	}
	read, err := keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Status != v1.KeyStatusRestricted {
		t.Fatalf("expected restricted, got %s", read.Status)
	}

	if _, err := keyOp.ChangeStatus(ctx, keyID, v1.ChangeKeyStateRequestStatusActive); err != nil {
		t.Fatal(err)
	}
	read, err = keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Status != v1.KeyStatusActive {
		t.Fatalf("expected active, got %s", read.Status)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "encrypt-test"})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	plaintext := []byte("hello, KMS!")
	cipher, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.EncryptionRequestAlgoAes256Gcm)
	if err != nil {
		t.Fatal(err)
	}
	if cipher == "" {
		t.Fatal("expected non-empty cipher")
	}

	decrypted, err := keyOp.Decrypt(ctx, keyID, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptDecryptAfterRotate(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "rotate-encrypt-test"})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	// Encrypt with v1
	plaintext := []byte("secret data")
	cipher1, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.EncryptionRequestAlgoAes256Gcm)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := keyOp.Rotate(ctx, keyID); err != nil {
		t.Fatal(err)
	}

	// Decrypt cipher1 should still work (tries all versions)
	decrypted, err := keyOp.Decrypt(ctx, keyID, cipher1)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("expected %q, got %q", plaintext, decrypted)
	}

	// Encrypt with v2
	cipher2, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.EncryptionRequestAlgoAes256Gcm)
	if err != nil {
		t.Fatal(err)
	}
	if cipher1 == cipher2 {
		t.Fatal("expected different ciphertexts after rotation")
	}
}

func TestScheduleDestruction(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "destroy-test"})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	before := time.Now()
	scheduled, err := keyOp.ScheduleDestruction(ctx, keyID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if scheduled.Status != v1.KeyScheduledDestructionStatusPendingDestruction {
		t.Fatalf("expected pending_destruction, got %s", scheduled.Status)
	}
	after, ok := scheduled.DeletionScheduledAfter.Get()
	if !ok {
		t.Fatal("expected DeletionScheduledAfter to be set")
	}
	if want := before.AddDate(0, 0, 30); after.Before(want.Add(-time.Second)) || after.After(want.Add(time.Minute)) {
		t.Fatalf("DeletionScheduledAfter = %v, want about %v", after, want)
	}

	read, err := keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Status != v1.KeyStatusPendingDestruction {
		t.Fatalf("expected pending_destruction, got %s", read.Status)
	}
	if !read.DeletionScheduledAfter.IsSet() || read.DeletionScheduledAfter.IsNull() {
		t.Fatal("expected DeletionScheduledAfter on the key")
	}

	// Reactivating the key cancels the scheduled destruction.
	if _, err := keyOp.ChangeStatus(ctx, keyID, v1.ChangeKeyStateRequestStatusActive); err != nil {
		t.Fatal(err)
	}
	read, err = keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if !read.DeletionScheduledAfter.IsNull() {
		t.Fatalf("expected null DeletionScheduledAfter after reactivation, got %+v", read.DeletionScheduledAfter)
	}
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations: %+v", v)
	}
}

func TestListKeysPagination(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	for _, name := range []string{"k1", "k2", "k3", "k4", "k5"} {
		if _, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: name}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		count, from *int
		want        []string
	}{
		{nil, nil, []string{"k1", "k2", "k3", "k4", "k5"}},
		{new(2), nil, []string{"k1", "k2"}},
		{new(2), new(2), []string{"k3", "k4"}},
		{new(10), new(3), []string{"k4", "k5"}},
		{nil, new(4), []string{"k5"}},
		{nil, new(10), []string{}},
	} {
		keys, err := keyOp.List(ctx, tc.count, tc.from)
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(keys))
		for i, k := range keys {
			got[i] = k.Name
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("List(count=%v, from=%v) = %v, want %v", deref(tc.count), deref(tc.from), got, tc.want)
		}
	}
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations: %+v", v)
	}
}

func deref(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestRotateInactiveKeyForbidden(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "suspended"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keyOp.ChangeStatus(ctx, created.ID, v1.ChangeKeyStateRequestStatusSuspended); err != nil {
		t.Fatal(err)
	}
	_, err = keyOp.Rotate(ctx, created.ID)
	var statusErr *ogenvalidate.UnexpectedStatusCodeError
	if !errors.As(err, &statusErr) || statusErr.StatusCode != http.StatusForbidden {
		t.Fatalf("Rotate on suspended key: err = %v, want 403", err)
	}
}

func TestPresetKey(t *testing.T) {
	const keyID = "123456789012"
	cfg := kms.Config{Keys: map[string]string{keyID: "my-dev-secret"}}
	plaintext := []byte("data encryption key")

	srv := kms.NewTestServer(cfg)
	defer srv.Close() // also covers an early t.Fatal; Close is safe to call twice
	keyOp := newTestKeyOp(t, srv.TestURL())
	ctx := t.Context()

	// The preset key is listed and readable under its fixed ID.
	got, err := keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != keyID {
		t.Errorf("ID = %q, want %q", got.ID, keyID)
	}
	cipher, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.EncryptionRequestAlgoAes256Gcm)
	if err != nil {
		t.Fatal(err)
	}
	// A key created afterwards must not reuse the preset ID.
	created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "after-preset"})
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == keyID {
		t.Errorf("generated ID collided with preset ID %q", keyID)
	}
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations: %+v", v)
	}
	srv.Close() // stop the first run before decrypting on a fresh one

	// A fresh server with the same --key decrypts the ciphertext from the first run.
	srv2 := kms.NewTestServer(cfg)
	defer srv2.Close()
	keyOp2 := newTestKeyOp(t, srv2.TestURL())
	decrypted, err := keyOp2.Decrypt(ctx, keyID, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}

	// A different secret yields different material and cannot decrypt it.
	srv3 := kms.NewTestServer(kms.Config{Keys: map[string]string{keyID: "other-secret"}})
	defer srv3.Close()
	if _, err := newTestKeyOp(t, srv3.TestURL()).Decrypt(ctx, keyID, cipher); err == nil {
		t.Error("decrypt with a different secret succeeded, want error")
	}
}

func TestPresetKeyRotated(t *testing.T) {
	const keyID = "123456789012"
	plaintext := []byte("data encryption key")
	ctx := t.Context()

	// Rotate a preset key twice and encrypt under version 3.
	srv := kms.NewTestServer(kms.Config{Keys: map[string]string{keyID: "my-dev-secret"}})
	defer srv.Close()
	keyOp := newTestKeyOp(t, srv.TestURL())
	for range 2 {
		if _, err := keyOp.Rotate(ctx, keyID); err != nil {
			t.Fatal(err)
		}
	}
	cipher, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.EncryptionRequestAlgoAes256Gcm)
	if err != nil {
		t.Fatal(err)
	}
	if v := srv.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations: %+v", v)
	}
	srv.Close()

	// A fresh server declaring the key at version 3 reports that version and
	// decrypts the ciphertext, because rotation is a deterministic chain.
	srv2 := kms.NewTestServer(kms.Config{Keys: map[string]string{keyID: "my-dev-secret@3"}})
	defer srv2.Close()
	keyOp2 := newTestKeyOp(t, srv2.TestURL())
	got, err := keyOp2.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if got.LatestVersion != 3 {
		t.Errorf("LatestVersion = %v, want 3", got.LatestVersion)
	}
	decrypted, err := keyOp2.Decrypt(ctx, keyID, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
	// Ciphertexts made before the restart at version 1 still decrypt too.
	cipher1, err := keyOp2.Encrypt(ctx, keyID, plaintext, v1.EncryptionRequestAlgoAes256Gcm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := keyOp2.Decrypt(ctx, keyID, cipher1); err != nil {
		t.Fatal(err)
	}
	if v := srv2.SpecViolations(); len(v) != 0 {
		t.Errorf("spec violations: %+v", v)
	}

	// A server still at version 1 does not know version 3 and cannot decrypt.
	srv3 := kms.NewTestServer(kms.Config{Keys: map[string]string{keyID: "my-dev-secret"}})
	defer srv3.Close()
	if _, err := newTestKeyOp(t, srv3.TestURL()).Decrypt(ctx, keyID, cipher); err == nil {
		t.Error("decrypt at version 1 of a version-3 ciphertext succeeded, want error")
	}
}

func TestPresetKeyInvalid(t *testing.T) {
	if _, err := kms.NewHandler(kms.Config{Keys: map[string]string{"bad": "secret"}}); err == nil {
		t.Error("NewHandler with invalid --key succeeded, want error")
	}
}

func TestPresetKeySharedIDGeneratorConflict(t *testing.T) {
	const keyID = "123456789012"
	gen := core.NewIDGenerator(0)
	// Another service under `sakumock all` already claimed the ID.
	if err := gen.Reserve(keyID, "other"); err != nil {
		t.Fatal(err)
	}
	cfg := kms.Config{Keys: map[string]string{keyID: "secret"}}
	if _, err := cfg.NewServer(core.ServerOptions{IDGen: gen}); err == nil {
		t.Error("NewServer with an ID reserved by another service succeeded, want error")
	}
	// With a fresh shared generator the same config starts fine.
	if _, err := cfg.NewServer(core.ServerOptions{IDGen: core.NewIDGenerator(0)}); err != nil {
		t.Errorf("NewServer with a free ID failed: %v", err)
	}
}
