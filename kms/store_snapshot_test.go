package kms

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// Restore rejects a snapshot whose keys could not be served, instead of
// loading it and failing on the first request.
func TestRestoreRejectsInconsistentSnapshot(t *testing.T) {
	store := NewMemoryStore(nil)
	k, err := store.Create("k", "", "generated", nil)
	if err != nil {
		t.Fatal(err)
	}
	good, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if err := NewMemoryStore(nil).Restore(good); err != nil {
		t.Fatalf("restore of an untouched snapshot: %v", err)
	}

	// A rotated key needs the material of every version, not only the latest:
	// a ciphertext made before the rotation is decrypted with the older one.
	if _, err := store.Rotate(k.ID); err != nil {
		t.Fatal(err)
	}
	var rotated storeSnapshot
	if data, err := store.Snapshot(); err != nil {
		t.Fatal(err)
	} else if err := json.Unmarshal(data, &rotated); err != nil {
		t.Fatal(err)
	}
	delete(rotated.KeyMaterial[k.ID], 1)
	missingOldVersion, err := json.Marshal(rotated)
	if err != nil {
		t.Fatal(err)
	}

	for name, snapshot := range map[string]string{
		"older version without material":  string(missingOldVersion),
		"latest version without material": strings.Replace(string(good), `"latest_version": 1`, `"latest_version": 7`, 1),
		"record id differs from its key":  fmt.Sprintf(`{"keys":{"1":{"id":%q,"latest_version":1}}}`, k.ID),
	} {
		t.Run(name, func(t *testing.T) {
			if err := NewMemoryStore(nil).Restore([]byte(snapshot)); err == nil {
				t.Error("Restore succeeded, want error")
			}
		})
	}
}
