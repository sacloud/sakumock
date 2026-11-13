package secretmanager

import (
	"encoding/json"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	Vaults map[string]*Vault `json:"vaults"`
	// Secrets is keyed by vault ID, then by secret name.
	Secrets map[string]map[string]secretSnapshot `json:"secrets"`
}

// secretSnapshot is one secret with the versions the store still retains.
type secretSnapshot struct {
	LatestVersion int            `json:"latest_version"`
	Versions      map[int]string `json:"versions"`
}

// Snapshot returns every vault and secret, values included, as JSON, for
// core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	snap := storeSnapshot{
		Vaults:  s.vaultRecords,
		Secrets: make(map[string]map[string]secretSnapshot, len(s.vaults)),
	}
	for vaultID, secrets := range s.vaults {
		out := make(map[string]secretSnapshot, len(secrets))
		for name, sec := range secrets {
			out[name] = secretSnapshot{LatestVersion: sec.latest, Versions: sec.versions}
		}
		snap.Secrets[vaultID] = out
	}
	return json.MarshalIndent(snap, "", "  ")
}

// Restore replaces the store's vaults and secrets with a snapshot, and
// advances the ID generator past the restored vault IDs so new vaults never
// reuse one.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Vaults == nil {
		snap.Vaults = make(map[string]*Vault)
	}
	for id, v := range snap.Vaults {
		if v == nil || v.ID != id {
			return fmt.Errorf("vault %q: record is missing or has a different id", id)
		}
	}
	vaults := make(map[string]map[string]*secret, len(snap.Secrets))
	for vaultID, secrets := range snap.Secrets {
		out := make(map[string]*secret, len(secrets))
		for name, sec := range secrets {
			if _, ok := sec.Versions[sec.LatestVersion]; !ok {
				return fmt.Errorf("vault %q secret %q: no value for latest version %d", vaultID, name, sec.LatestVersion)
			}
			out[name] = &secret{name: name, versions: sec.Versions, latest: sec.LatestVersion}
		}
		vaults[vaultID] = out
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.vaultRecords = snap.Vaults
	s.vaults = vaults
	for id := range s.vaultRecords {
		s.ids.Observe(id)
	}
	return nil
}
