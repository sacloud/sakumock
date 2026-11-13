package kms_test

import (
	"bytes"
	"testing"

	kmssdk "github.com/sacloud/sacloud-sdk-go/api/kms"
	v1 "github.com/sacloud/sacloud-sdk-go/api/kms/apis/v1"

	"github.com/sacloud/sakumock/core/coretest"
	"github.com/sacloud/sakumock/kms"
)

func TestStateSurvivesRestart(t *testing.T) {
	plaintext := []byte("hello")
	var keyID, cipher string

	coretest.StateRestart(t, kms.Config{},
		func(t *testing.T, url string) {
			ctx := t.Context()
			keyOp := newTestKeyOp(t, url)
			created, err := keyOp.Create(ctx, kmssdk.CreateParams{Name: "persisted", Tags: []string{"a"}})
			if err != nil {
				t.Fatal(err)
			}
			keyID = created.ID
			if _, err := keyOp.Rotate(ctx, keyID); err != nil {
				t.Fatal(err)
			}
			cipher, err = keyOp.Encrypt(ctx, keyID, plaintext, v1.EncryptionRequestAlgoAes256Gcm)
			if err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			ctx := t.Context()
			keyOp := newTestKeyOp(t, url)
			keys, err := keyOp.List(ctx, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(keys) != 1 {
				t.Fatalf("got %d keys after restart, want 1", len(keys))
			}
			got, err := keyOp.Read(ctx, keyID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != "persisted" || got.LatestVersion != 2 || got.Status != v1.KeyStatusActive {
				t.Errorf("restored key = %+v", got)
			}
			// The key material is restored too, not regenerated.
			plain, err := keyOp.Decrypt(ctx, keyID, cipher)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(plain, plaintext) {
				t.Errorf("decrypt after restart = %q, want %q", plain, plaintext)
			}
		})
}

// A key created after a restart never reuses a restored ID.
func TestStateRestoredIDsAreNotReused(t *testing.T) {
	var keyID string
	coretest.StateRestart(t, kms.Config{},
		func(t *testing.T, url string) {
			created, err := newTestKeyOp(t, url).Create(t.Context(), kmssdk.CreateParams{Name: "first"})
			if err != nil {
				t.Fatal(err)
			}
			keyID = created.ID
		},
		func(t *testing.T, url string) {
			created, err := newTestKeyOp(t, url).Create(t.Context(), kmssdk.CreateParams{Name: "second"})
			if err != nil {
				t.Fatal(err)
			}
			if created.ID == keyID {
				t.Errorf("new key reused the restored ID %s", keyID)
			}
		}, true)
}

// The same --key on restart must not reset what happened to the key.
func TestStateKeepsRestoredPresetKey(t *testing.T) {
	const id = "123456789012"
	cfg := kms.Config{Keys: map[string]string{id: "my-dev-secret"}}

	coretest.StateRestart(t, cfg,
		func(t *testing.T, url string) {
			ctx := t.Context()
			keyOp := newTestKeyOp(t, url)
			if _, err := keyOp.Rotate(ctx, id); err != nil {
				t.Fatal(err)
			}
			if _, err := keyOp.Update(ctx, id, kmssdk.UpdateParams{Name: "renamed"}); err != nil {
				t.Fatal(err)
			}
		},
		func(t *testing.T, url string) {
			got, err := newTestKeyOp(t, url).Read(t.Context(), id)
			if err != nil {
				t.Fatal(err)
			}
			if got.LatestVersion != 2 || got.Name != "renamed" {
				t.Errorf("preset key after restart = %+v, want the restored version 2 named \"renamed\"", got)
			}
		})
}
