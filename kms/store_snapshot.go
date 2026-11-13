package kms

import (
	"encoding/json"
	"fmt"
)

// keyMaterialSize is the length of one version's AES-256 key material.
const keyMaterialSize = 32

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	Keys map[string]*KeyRecord `json:"keys"`
	// KeyMaterial is the key material per key ID per version, base64-encoded.
	KeyMaterial map[string]map[int][]byte `json:"key_material"`
}

// Snapshot returns every key and its key material as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(storeSnapshot{Keys: s.keys, KeyMaterial: s.keyMaterial}, "", "  ")
}

// Restore replaces the store's keys with a snapshot, and advances the ID
// generator past the restored IDs so new keys never reuse one. A snapshot
// whose keys lack usable material is rejected as a whole.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Keys == nil {
		snap.Keys = make(map[string]*KeyRecord)
	}
	if snap.KeyMaterial == nil {
		snap.KeyMaterial = make(map[string]map[int][]byte)
	}
	for id, k := range snap.Keys {
		if k == nil || k.ID != id {
			return fmt.Errorf("key %q: record is missing or has a different id", id)
		}
		// Every version is kept: a ciphertext is decrypted with the version it
		// was encrypted under, and each version derives from the one before.
		if k.LatestVersion < 1 {
			return fmt.Errorf("key %q: latest version %d, want at least 1", id, k.LatestVersion)
		}
		for v := 1; v <= k.LatestVersion; v++ {
			if len(snap.KeyMaterial[id][v]) != keyMaterialSize {
				return fmt.Errorf("key %q: no %d-byte key material for version %d", id, keyMaterialSize, v)
			}
		}
		if k.Tags == nil {
			k.Tags = []string{}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.keys = snap.Keys
	s.keyMaterial = snap.KeyMaterial
	for id := range s.keys {
		s.ids.Observe(id)
	}
	return nil
}
