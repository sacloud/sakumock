package cloudhsm

import (
	"encoding/json"
	"fmt"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	CloudHSMs map[string]*CloudHSMRecord `json:"cloudhsms"`
	// Clients and Peers are keyed by CloudHSM ID, then by their own ID.
	Clients  map[string]map[string]*ClientRecord `json:"clients"`
	Peers    map[string]map[string]*PeerRecord   `json:"peers"`
	Licenses map[string]*LicenseRecord           `json:"licenses"`
	// Documents is keyed by license ID, then by document ID.
	Documents map[string]map[string]*DocumentRecord `json:"documents"`
}

// Snapshot returns every CloudHSM with its clients and peers, and every
// license with its documents, as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(storeSnapshot{
		CloudHSMs: s.hsms,
		Clients:   s.clients,
		Peers:     s.peers,
		Licenses:  s.licenses,
		Documents: s.docs,
	}, "", "  ")
}

// Restore replaces the store's state with a snapshot, and advances the ID
// generator past the restored IDs so new resources never reuse one. A
// snapshot whose clients, peers or documents belong to no CloudHSM or license
// is rejected as a whole.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.CloudHSMs == nil {
		snap.CloudHSMs = make(map[string]*CloudHSMRecord)
	}
	if snap.Clients == nil {
		snap.Clients = make(map[string]map[string]*ClientRecord)
	}
	if snap.Peers == nil {
		snap.Peers = make(map[string]map[string]*PeerRecord)
	}
	if snap.Licenses == nil {
		snap.Licenses = make(map[string]*LicenseRecord)
	}
	if snap.Documents == nil {
		snap.Documents = make(map[string]map[string]*DocumentRecord)
	}

	for id, h := range snap.CloudHSMs {
		if h == nil || h.ID != id {
			return fmt.Errorf("cloudhsm %q: record is missing or has a different id", id)
		}
		if h.Tags == nil {
			h.Tags = []string{}
		}
		// The per-CloudHSM maps double as the existence check of the nested
		// endpoints, so every CloudHSM needs both.
		if snap.Clients[id] == nil {
			snap.Clients[id] = make(map[string]*ClientRecord)
		}
		if snap.Peers[id] == nil {
			snap.Peers[id] = make(map[string]*PeerRecord)
		}
	}
	for hsmID, clients := range snap.Clients {
		if _, ok := snap.CloudHSMs[hsmID]; !ok {
			return fmt.Errorf("clients of cloudhsm %q: no such cloudhsm", hsmID)
		}
		for id, c := range clients {
			if c == nil || c.ID != id || c.CloudHSMID != hsmID {
				return fmt.Errorf("cloudhsm %q client %q: record is missing or has a different id", hsmID, id)
			}
		}
	}
	for hsmID, peers := range snap.Peers {
		if _, ok := snap.CloudHSMs[hsmID]; !ok {
			return fmt.Errorf("peers of cloudhsm %q: no such cloudhsm", hsmID)
		}
		for id, p := range peers {
			if p == nil || p.ID != id || p.CloudHSMID != hsmID {
				return fmt.Errorf("cloudhsm %q peer %q: record is missing or has a different id", hsmID, id)
			}
		}
	}
	for id, l := range snap.Licenses {
		if l == nil || l.ID != id {
			return fmt.Errorf("license %q: record is missing or has a different id", id)
		}
		if l.Tags == nil {
			l.Tags = []string{}
		}
		if snap.Documents[id] == nil {
			snap.Documents[id] = make(map[string]*DocumentRecord)
		}
	}
	for licenseID, docs := range snap.Documents {
		if _, ok := snap.Licenses[licenseID]; !ok {
			return fmt.Errorf("documents of license %q: no such license", licenseID)
		}
		for id, d := range docs {
			if d == nil || d.ID != id || d.LicenseID != licenseID {
				return fmt.Errorf("license %q document %q: record is missing or has a different id", licenseID, id)
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.hsms = snap.CloudHSMs
	s.clients = snap.Clients
	s.peers = snap.Peers
	s.licenses = snap.Licenses
	s.docs = snap.Documents
	// Peer IDs are the user's router IDs, not generated ones.
	for id := range s.hsms {
		s.ids.Observe(id)
	}
	for _, clients := range s.clients {
		for id := range clients {
			s.ids.Observe(id)
		}
	}
	for id := range s.licenses {
		s.ids.Observe(id)
	}
	for _, docs := range s.docs {
		for id := range docs {
			s.ids.Observe(id)
		}
	}
	return nil
}
