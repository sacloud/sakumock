package seg

import (
	"encoding/json"
	"errors"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	Appliances map[string]*ApplianceRecord `json:"appliances"`
	// SharedIPN is the counter behind the synthetic shared-switch IPs; it is
	// saved so an appliance created after a restart does not get the IP of a
	// restored one.
	SharedIPN int `json:"shared_ip_n"`
}

// Snapshot returns every appliance and the shared-IP counter as JSON, for
// core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(storeSnapshot{Appliances: s.appliances, SharedIPN: s.sharedIPN}, "", "  ")
}

// Restore replaces the store's appliances with a snapshot, and advances the
// ID generator past the restored appliance and shared-switch IDs so new ones
// never reuse them.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Appliances == nil {
		snap.Appliances = make(map[string]*ApplianceRecord)
	}
	if snap.SharedIPN < 0 {
		return errors.New("shared_ip_n is negative")
	}
	for id, a := range snap.Appliances {
		if a == nil || a.ID != id {
			return fmt.Errorf("appliance %q: record is missing or has a different id", id)
		}
		if a.Tags == nil {
			a.Tags = []string{}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.appliances = snap.Appliances
	s.sharedIPN = snap.SharedIPN
	for id, a := range s.appliances {
		s.ids.Observe(id)
		for _, iface := range a.Interfaces {
			// The user-scope switch ID is the user's own, not a generated one.
			if iface.Scope == "shared" {
				s.ids.Observe(iface.SwitchID)
			}
		}
	}
	return nil
}
