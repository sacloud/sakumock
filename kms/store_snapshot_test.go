package kms

import (
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

	for name, snapshot := range map[string]string{
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
