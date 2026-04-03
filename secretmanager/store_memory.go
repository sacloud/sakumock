package secretmanager

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
)

type secret struct {
	name     string
	versions map[int]string
	latest   int
}

// MemoryStore is an in-memory Store for secrets with versioning.
type MemoryStore struct {
	mu     sync.RWMutex
	vaults map[string]map[string]*secret // vaultID -> secretName -> secret
}

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		vaults: make(map[string]map[string]*secret),
	}
}

func (s *MemoryStore) getOrCreateVault(vaultID string) map[string]*secret {
	v, ok := s.vaults[vaultID]
	if !ok {
		v = make(map[string]*secret)
		s.vaults[vaultID] = v
		slog.Info("vault created", "vault_id", vaultID)
	}
	return v
}

func (s *MemoryStore) List(vaultID string) []SecretMeta {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vault, ok := s.vaults[vaultID]
	if !ok {
		return nil
	}
	result := make([]SecretMeta, 0, len(vault))
	for _, sec := range vault {
		result = append(result, SecretMeta{
			Name:          sec.name,
			LatestVersion: sec.latest,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}

func (s *MemoryStore) Create(vaultID, name, value string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	vault := s.getOrCreateVault(vaultID)
	sec, ok := vault[name]
	if !ok {
		sec = &secret{
			name:     name,
			versions: make(map[int]string),
		}
		vault[name] = sec
	}
	sec.latest++
	sec.versions[sec.latest] = value
	return sec.latest, nil
}

func (s *MemoryStore) Unveil(vaultID, name string, version int) (string, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	vault, ok := s.vaults[vaultID]
	if !ok {
		return "", 0, fmt.Errorf("secret %q not found", name)
	}
	sec, ok := vault[name]
	if !ok {
		return "", 0, fmt.Errorf("secret %q not found", name)
	}
	if version == 0 {
		version = sec.latest
	}
	value, ok := sec.versions[version]
	if !ok {
		return "", 0, fmt.Errorf("secret %q version %d not found", name, version)
	}
	return value, version, nil
}

func (s *MemoryStore) Delete(vaultID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	vault, ok := s.vaults[vaultID]
	if !ok {
		return fmt.Errorf("secret %q not found", name)
	}
	if _, ok := vault[name]; !ok {
		return fmt.Errorf("secret %q not found", name)
	}
	delete(vault, name)
	return nil
}

func (s *MemoryStore) Close() error {
	return nil
}
