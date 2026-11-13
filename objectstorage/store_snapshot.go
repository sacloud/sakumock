package objectstorage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
)

// storeSnapshot is the store's form in the state file (--state-dir).
type storeSnapshot struct {
	// Buckets is keyed by bucket name.
	Buckets map[string]*Bucket `json:"buckets"`
	// Accounts is keyed by site ID.
	Accounts map[string]*Account `json:"accounts"`
	// Permissions is keyed by site ID, then by permission ID.
	Permissions map[string]map[int64]*Permission `json:"permissions"`
}

// Snapshot returns every bucket, account and permission, access key secrets
// included, as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.MarshalIndent(storeSnapshot{
		Buckets:     s.buckets,
		Accounts:    s.accounts,
		Permissions: s.permissions,
	}, "", "  ")
}

// Restore replaces the store's buckets, accounts and permissions with a
// snapshot, and advances the ID generator past the restored resource IDs so
// new resources never reuse one.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	if snap.Buckets == nil {
		snap.Buckets = make(map[string]*Bucket)
	}
	if snap.Accounts == nil {
		snap.Accounts = make(map[string]*Account)
	}
	if snap.Permissions == nil {
		snap.Permissions = make(map[string]map[int64]*Permission)
	}
	for name, b := range snap.Buckets {
		if b == nil || b.Name != name {
			return fmt.Errorf("bucket %q: record is missing or has a different name", name)
		}
	}
	for siteID, a := range snap.Accounts {
		if a == nil {
			return fmt.Errorf("account of site %q: record is missing", siteID)
		}
	}
	for siteID, perms := range snap.Permissions {
		if perms == nil {
			snap.Permissions[siteID] = make(map[int64]*Permission)
		}
		for id, p := range perms {
			if p == nil || p.ID != id {
				return fmt.Errorf("site %q permission %d: record is missing or has a different id", siteID, id)
			}
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.buckets = snap.Buckets
	s.accounts = snap.Accounts
	s.permissions = snap.Permissions
	for _, b := range s.buckets {
		s.ids.Observe(b.ResourceID)
	}
	for _, a := range s.accounts {
		s.ids.Observe(a.ResourceID)
	}
	for _, perms := range s.permissions {
		for id := range perms {
			s.ids.Observe(strconv.FormatInt(id, 10))
		}
	}
	return nil
}

// accessKey is one issued access key (account or permission) as the data plane
// registers it.
type accessKey struct {
	ID     string
	Secret string
}

// accessKeys returns every account and permission access key the store holds,
// ordered by ID, so a restarted data plane can register them again.
func (s *MemoryStore) accessKeys() []accessKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []accessKey
	for _, a := range s.accounts {
		for _, k := range a.Keys {
			keys = append(keys, accessKey{ID: k.ID, Secret: k.Secret})
		}
	}
	for _, perms := range s.permissions {
		for _, p := range perms {
			for _, k := range p.Keys {
				keys = append(keys, accessKey{ID: k.ID, Secret: k.Secret})
			}
		}
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].ID < keys[j].ID })
	return keys
}
