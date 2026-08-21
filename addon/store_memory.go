package addon

import (
	"encoding/json"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryStore is the in-memory Store implementation.
type MemoryStore struct {
	mu                sync.RWMutex
	resources         map[Kind]map[string]*Resource
	provisioningDelay time.Duration
	logger            *slog.Logger
}

var _ Store = (*MemoryStore)(nil)

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore(logger *slog.Logger, provisioningDelay time.Duration) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		resources:         make(map[Kind]map[string]*Resource),
		provisioningDelay: provisioningDelay,
		logger:            logger,
	}
}

// shortID returns the 8 leading hex digits of a random UUID, the suffix that
// keeps generated resource group and deployment names unique.
func shortID() string {
	return strings.SplitN(uuid.NewString(), "-", 2)[0]
}

// List returns every provisioned resource of the family, oldest first.
func (s *MemoryStore) List(kind Kind, now time.Time) []Resource {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]Resource, 0, len(s.resources[kind]))
	for _, r := range s.resources[kind] {
		if r.Provisioned(now) {
			result = append(result, *r)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].CreatedAt.Equal(result[j].CreatedAt) {
			return result[i].ResourceGroupName < result[j].ResourceGroupName
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// Read returns the provisioned resource with the given resource group name.
func (s *MemoryStore) Read(kind Kind, resourceGroupName string, now time.Time) (Resource, error) {
	r, err := s.ReadAny(kind, resourceGroupName)
	if err != nil {
		return Resource{}, err
	}
	if !r.Provisioned(now) {
		return Resource{}, errNotFound
	}
	return r, nil
}

// ReadAny returns the resource whether or not it finished provisioning.
func (s *MemoryStore) ReadAny(kind Kind, resourceGroupName string) (Resource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	r, ok := s.resources[kind][resourceGroupName]
	if !ok {
		return Resource{}, errNotFound
	}
	return *r, nil
}

// Create records a new resource group and the deployment that created it.
func (s *MemoryStore) Create(kind Kind, location string, params json.RawMessage) Resource {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	r := &Resource{
		Kind:              kind,
		ResourceGroupName: string(kind) + "-" + shortID(),
		DeploymentName:    string(kind) + "-deployment-" + shortID(),
		Location:          location,
		Parameters:        params,
		CreatedAt:         now,
		ReadyAt:           now.Add(s.provisioningDelay),
	}
	if s.resources[kind] == nil {
		s.resources[kind] = make(map[string]*Resource)
	}
	s.resources[kind][r.ResourceGroupName] = r
	s.logger.Debug("addon resource created",
		"kind", kind, "resource_group", r.ResourceGroupName, "deployment", r.DeploymentName)
	return *r
}

// Delete removes the resource group, provisioned or not.
func (s *MemoryStore) Delete(kind Kind, resourceGroupName string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.resources[kind][resourceGroupName]; !ok {
		return errNotFound
	}
	delete(s.resources[kind], resourceGroupName)
	s.logger.Debug("addon resource deleted", "kind", kind, "resource_group", resourceGroupName)
	return nil
}

// Close releases the store. The in-memory implementation has nothing to
// release.
func (s *MemoryStore) Close() error { return nil }
