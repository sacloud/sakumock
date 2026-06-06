package simplenotification

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/sacloud/sakumock/core"
)

// MemoryStore is an in-memory Store for accepted notification messages and
// control-plane service items.
type MemoryStore struct {
	mu       sync.Mutex
	messages []MessageRecord
	nextID   int64
	items    map[string]*ServiceItem
	ids      *core.IDGenerator
}

// NewMemoryStore creates a new empty MemoryStore.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextID: 1,
		items:  make(map[string]*ServiceItem),
		ids:    core.NewIDGenerator(core.DefaultIDBase),
	}
}

func cloneItem(it *ServiceItem) ServiceItem {
	c := *it
	c.Tags = append([]string(nil), it.Tags...)
	c.Settings = append(json.RawMessage(nil), it.Settings...)
	c.Icon = append(json.RawMessage(nil), it.Icon...)
	return c
}

// CreateItem stores a new control-plane item, assigning it an ID and timestamps.
func (s *MemoryStore) CreateItem(item ServiceItem) ServiceItem {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	item.ID = s.ids.Next()
	item.CreatedAt = now
	item.ModifiedAt = now
	stored := cloneItem(&item)
	s.items[item.ID] = &stored
	slog.Info("service item created", "id", item.ID, "class", item.ProviderClass, "name", item.Name)
	return cloneItem(&stored)
}

// GetItem returns a copy of the item with the given ID.
func (s *MemoryStore) GetItem(id string) (ServiceItem, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	it, ok := s.items[id]
	if !ok {
		return ServiceItem{}, false
	}
	return cloneItem(it), true
}

// ListItems returns copies of all items, optionally filtered by provider class,
// ordered by ID.
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

// UpdateItem mutates the item's name, description, tags, settings, and icon.
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

// DeleteItem removes the item and returns a copy of what was deleted.
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

// Send records a notification message and returns the stored record.
func (s *MemoryStore) Send(groupID, message string, now time.Time) (MessageRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rec := MessageRecord{
		ID:        fmt.Sprintf("%d", s.nextID),
		GroupID:   groupID,
		Message:   message,
		CreatedAt: now,
	}
	s.nextID++
	s.messages = append(s.messages, rec)
	slog.Debug("message stored", "id", rec.ID, "group_id", groupID)
	return rec, nil
}

// List returns a snapshot of all accepted messages in send order.
func (s *MemoryStore) List() []MessageRecord {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]MessageRecord, len(s.messages))
	copy(out, s.messages)
	return out
}

// Reset clears all accepted messages.
func (s *MemoryStore) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.messages = nil
}

// Close releases resources held by the store.
func (s *MemoryStore) Close() error {
	return nil
}
