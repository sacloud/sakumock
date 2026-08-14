package eventbus

import (
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/sacloud/sakumock/core"
)

// MemoryStore is the in-memory Store implementation.
type MemoryStore struct {
	mu     sync.Mutex
	items  map[string]*ServiceItem
	ids    *core.IDGenerator
	logger *slog.Logger
}

// NewMemoryStore returns an empty MemoryStore.
func NewMemoryStore(logger *slog.Logger) *MemoryStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &MemoryStore{
		items:  make(map[string]*ServiceItem),
		ids:    core.NewIDGenerator(core.DefaultIDBase()),
		logger: logger,
	}
}

func cloneItem(it *ServiceItem) ServiceItem {
	c := *it
	c.Tags = append([]string(nil), it.Tags...)
	c.Settings = append(json.RawMessage(nil), it.Settings...)
	c.Icon = append(json.RawMessage(nil), it.Icon...)
	c.Secret = append(json.RawMessage(nil), it.Secret...)
	if it.Status != nil {
		st := *it.Status
		c.Status = &st
	}
	return c
}

// CreateItem stores a new item, assigning its ID and timestamps.
func (s *MemoryStore) CreateItem(item ServiceItem) ServiceItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.ids.Next()
	item.CreatedAt = now
	item.ModifiedAt = now
	stored := cloneItem(&item)
	s.items[item.ID] = &stored
	s.logger.Info("service item created", "id", item.ID, "class", item.ProviderClass, "name", item.Name)
	return cloneItem(&stored)
}

// GetItem returns the item with the given ID.
func (s *MemoryStore) GetItem(id string) (ServiceItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return ServiceItem{}, false
	}
	return cloneItem(it), true
}

// ListItems returns every item, optionally filtered by provider class.
func (s *MemoryStore) ListItems(providerClass string) []ServiceItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ServiceItem, 0, len(s.items))
	for _, it := range s.items {
		if providerClass != "" && it.ProviderClass != providerClass {
			continue
		}
		out = append(out, cloneItem(it))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// UpdateItem applies the given values to an existing item.
func (s *MemoryStore) UpdateItem(id, name, description string, tags []string, settings, icon json.RawMessage) (ServiceItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return ServiceItem{}, false
	}
	it.Name = name
	it.Description = description
	it.Tags = append([]string(nil), tags...)
	if settings != nil {
		it.Settings = append(json.RawMessage(nil), settings...)
	}
	it.Icon = append(json.RawMessage(nil), icon...)
	it.ModifiedAt = time.Now()
	return cloneItem(it), true
}

// DeleteItem removes the item and returns what was deleted.
func (s *MemoryStore) DeleteItem(id string) (ServiceItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return ServiceItem{}, false
	}
	deleted := cloneItem(it)
	delete(s.items, id)
	return deleted, true
}

// SetSecret stores the write-only secret of a process configuration.
func (s *MemoryStore) SetSecret(id string, secret json.RawMessage) (ServiceItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return ServiceItem{}, false
	}
	it.Secret = append(json.RawMessage(nil), secret...)
	it.ModifiedAt = time.Now()
	s.logger.Debug("secret set", "id", id)
	return cloneItem(it), true
}

// SetStatus records the latest firing outcome of a schedule or trigger. Unlike
// the other mutators it leaves ModifiedAt untouched: Status reflects a run, not
// a configuration change, so it must not look like the resource was edited.
func (s *MemoryStore) SetStatus(id string, status ItemStatus) (ServiceItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return ServiceItem{}, false
	}
	st := status
	it.Status = &st
	return cloneItem(it), true
}

// Close releases the resources held by the store.
func (s *MemoryStore) Close() error {
	return nil
}
