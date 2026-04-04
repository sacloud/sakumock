package kms_test

import (
	"testing"

	kmssdk "github.com/sacloud/kms-api-go"
	v1 "github.com/sacloud/kms-api-go/apis/v1"
	"github.com/sacloud/saclient-go"

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

	// List: initially empty
	keys, err := keyOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys, got %d", len(keys))
	}

	// Create key
	created, err := keyOp.Create(ctx, v1.CreateKey{
		Name:      "test-key",
		KeyOrigin: v1.KeyOriginEnumGenerated,
		Tags:      []string{"env:test"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Name != "test-key" {
		t.Fatalf("unexpected name: %s", created.Name)
	}
	if created.KeyOrigin != v1.KeyOriginEnumGenerated {
		t.Fatalf("unexpected origin: %s", created.KeyOrigin)
	}
	keyID := created.ID

	// List: 1 key
	keys, err = keyOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 1 {
		t.Fatalf("expected 1 key, got %d", len(keys))
	}
	if keys[0].Name != "test-key" {
		t.Fatalf("unexpected name: %s", keys[0].Name)
	}
	if keys[0].Status != v1.KeyStatusEnumActive {
		t.Fatalf("unexpected status: %s", keys[0].Status)
	}

	// Read key
	read, err := keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Name != "test-key" || read.ID != keyID {
		t.Fatalf("unexpected read response: %+v", read)
	}

	// Update key
	updated, err := keyOp.Update(ctx, keyID, v1.Key{
		Name:        "updated-key",
		Description: "updated description",
		KeyOrigin:   v1.KeyOriginEnumGenerated,
		Status:      v1.KeyStatusEnumActive,
		Tags:        []string{"env:prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Name != "updated-key" || updated.Description != "updated description" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	// Read updated key
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

	// Delete key
	if err := keyOp.Delete(ctx, keyID); err != nil {
		t.Fatal(err)
	}

	// List: empty again
	keys, err = keyOp.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected 0 keys after delete, got %d", len(keys))
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
		_, err := keyOp.Create(ctx, v1.CreateKey{
			Name:      name,
			KeyOrigin: v1.KeyOriginEnumGenerated,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	keys, err := keyOp.List(ctx)
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

	created, err := keyOp.Create(ctx, v1.CreateKey{
		Name:      "rotate-test",
		KeyOrigin: v1.KeyOriginEnumGenerated,
	})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	// Rotate
	rotated, err := keyOp.Rotate(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.LatestVersion.Or(0) != 2 {
		t.Fatalf("expected version 2 after rotate, got %v", rotated.LatestVersion)
	}
}

func TestChangeStatus(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, v1.CreateKey{
		Name:      "status-test",
		KeyOrigin: v1.KeyOriginEnumGenerated,
	})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	// Change to restricted
	if err := keyOp.ChangeStatus(ctx, keyID, v1.ChangeKeyStatusStatusRestricted); err != nil {
		t.Fatal(err)
	}
	read, err := keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Status != v1.KeyStatusEnumRestricted {
		t.Fatalf("expected restricted, got %s", read.Status)
	}

	// Change back to active
	if err := keyOp.ChangeStatus(ctx, keyID, v1.ChangeKeyStatusStatusActive); err != nil {
		t.Fatal(err)
	}
	read, err = keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Status != v1.KeyStatusEnumActive {
		t.Fatalf("expected active, got %s", read.Status)
	}
}

func TestEncryptDecrypt(t *testing.T) {
	srv := kms.NewTestServer(kms.Config{})
	defer srv.Close()
	ctx := t.Context()
	keyOp := newTestKeyOp(t, srv.TestURL())

	created, err := keyOp.Create(ctx, v1.CreateKey{
		Name:      "encrypt-test",
		KeyOrigin: v1.KeyOriginEnumGenerated,
	})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	plaintext := []byte("hello, KMS!")
	cipher, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.KeyEncryptAlgoEnumAes256Gcm)
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

	created, err := keyOp.Create(ctx, v1.CreateKey{
		Name:      "rotate-encrypt-test",
		KeyOrigin: v1.KeyOriginEnumGenerated,
	})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	// Encrypt with v1
	plaintext := []byte("secret data")
	cipher1, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.KeyEncryptAlgoEnumAes256Gcm)
	if err != nil {
		t.Fatal(err)
	}

	// Rotate key
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
	cipher2, err := keyOp.Encrypt(ctx, keyID, plaintext, v1.KeyEncryptAlgoEnumAes256Gcm)
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

	created, err := keyOp.Create(ctx, v1.CreateKey{
		Name:      "destroy-test",
		KeyOrigin: v1.KeyOriginEnumGenerated,
	})
	if err != nil {
		t.Fatal(err)
	}
	keyID := created.ID

	if err := keyOp.ScheduleDestruction(ctx, keyID, 7); err != nil {
		t.Fatal(err)
	}

	read, err := keyOp.Read(ctx, keyID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Status != v1.KeyStatusEnumPendingDestruction {
		t.Fatalf("expected pending_destruction, got %s", read.Status)
	}
}
