package addon

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	// Resources is keyed by family, then by resource group name.
	Resources map[Kind]map[string]*Resource `json:"resources"`
}

// Snapshot returns every resource group, still provisioning or not, as JSON,
// for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(storeSnapshot{Resources: s.resources}, "", "  ")
}

// Restore replaces the store's resource groups with a snapshot. Provisioning
// needs no reconciliation: it is derived from each resource's ReadyAt, an
// absolute time, so a deployment that was running when the mock stopped
// completes on its original schedule.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Resources == nil {
		snap.Resources = make(map[Kind]map[string]*Resource)
	}
	known := make(map[Kind]bool, len(families))
	for _, f := range families {
		known[f.kind] = true
	}
	for kind, resources := range snap.Resources {
		if !known[kind] {
			return fmt.Errorf("resources of %q: no such add-on family", kind)
		}
		for name, r := range resources {
			if r == nil || r.Kind != kind || r.ResourceGroupName != name {
				return fmt.Errorf("%s resource %q: record is missing or has a different kind or name", kind, name)
			}
			// The state file is indented; keep the parameters as compact as
			// the request body that created them.
			var compact bytes.Buffer
			if err := json.Compact(&compact, r.Parameters); err != nil {
				return fmt.Errorf("%s resource %q: parameters: %w", kind, name, err)
			}
			r.Parameters = compact.Bytes()
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.resources = snap.Resources
	return nil
}
