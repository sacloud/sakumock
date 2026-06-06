package secretmanager

import (
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/sacloud/sakumock/core"
)

type secret struct {
	name     string
	versions map[int]string
	latest   int
}

// MemoryStore is an in-memory Store for vaults and their versioned secrets.
type MemoryStore struct {
	mu           sync.RWMutex
	vaults       map[string]map[string]*secret // vaultID -> secretName -> secret
	vaultRecords map[string]*Vault             // vaultID -> vault metadata
	ids          *core.IDGenerator
	logger       *slog.Logger
}

// NewMemoryStore creates a new empty MemoryStore. logger is the service-tagged
// logger used for operation logs; nil falls back to the default.
func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		vaults:       make(map[string]map[string]*secret),
		vaultRecords: make(map[string]*Vault),
		ids:          core.NewIDGenerator(core.DefaultIDBase),
		logger:       logger,
	}
}

func cloneVault(v *Vault) *Vault {
	c := *v
	c.Tags = append([]string(nil), v.Tags...)
	return &c
}

// CreateVault creates a new vault and returns a copy of it.
func (s *MemoryStore) CreateVault(name, kmsKeyID, description string, tags []string) *Vault {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	v := &Vault{
		ID:          s.ids.Next(),
		Name:        name,
		Description: description,
		KmsKeyID:    kmsKeyID,
		Tags:        append([]string(nil), tags...),
		CreatedAt:   now,
		ModifiedAt:  now,
	}
	s.vaultRecords[v.ID] = v
	s.logger.Info("vault created", "vault_id", v.ID, "name", name)
	return cloneVault(v)
}

// GetVault returns a copy of the vault with the given ID.
func (s *MemoryStore) GetVault(id string) (*Vault, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vaultRecords[id]
	if !ok {
		return nil, false
	}
	return cloneVault(v), true
}

// ListVaults returns copies of all vaults, ordered by ID.
func (s *MemoryStore) ListVaults() []*Vault {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Vault, 0, len(s.vaultRecords))
	for _, v := range s.vaultRecords {
		out = append(out, cloneVault(v))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// UpdateVault mutates the vault's name, description, and tags (KmsKeyID is
// read-only). It returns a copy of the updated vault.
func (s *MemoryStore) UpdateVault(id, name, description string, tags []string) (*Vault, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.vaultRecords[id]
	if !ok {
		return nil, false
	}
	v.Name = name
	v.Description = description
	v.Tags = append([]string(nil), tags...)
	v.ModifiedAt = time.Now()
	return cloneVault(v), true
}

// DeleteVault removes the vault and any secrets stored under it.
func (s *MemoryStore) DeleteVault(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.vaultRecords[id]; !ok {
		return false
	}
	delete(s.vaultRecords, id)
	delete(s.vaults, id)
	return true
}

func (s *MemoryStore) getOrCreateVault(vaultID string) map[string]*secret {
	v, ok := s.vaults[vaultID]
	if !ok {
		v = make(map[string]*secret)
		s.vaults[vaultID] = v
		s.logger.Info("vault created", "vault_id", vaultID)
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
