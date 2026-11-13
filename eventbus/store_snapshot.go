package eventbus

import (
	"encoding/json"
	"fmt"
	"time"
)

// storeSnapshot is the store's form in the state file (--state-dir). The
// recorded deliveries and the scheduler's tick baseline belong to the data
// plane and are deliberately absent: a restarted mock starts both afresh.
type storeSnapshot struct {
	Items map[string]itemSnapshot `json:"items"`
}

// itemSnapshot is one ServiceItem. The raw JSON fields are kept as strings so
// they come back byte for byte: embedded as JSON they would be re-indented
// with the file, which changes the SettingsHash derived from those bytes.
type itemSnapshot struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	Tags          []string    `json:"tags"`
	ProviderClass string      `json:"provider_class"`
	ServiceClass  string      `json:"service_class"`
	Settings      string      `json:"settings"`
	Icon          string      `json:"icon,omitempty"`
	Secret        string      `json:"secret,omitempty"`
	Status        *ItemStatus `json:"status,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	ModifiedAt    time.Time   `json:"modified_at"`
}

// rawJSON turns a snapshot string back into the raw JSON it was taken from,
// keeping "absent" as nil.
func rawJSON(s string) json.RawMessage {
	if s == "" {
		return nil
	}
	return json.RawMessage(s)
}

// Snapshot returns every service item, secrets and last firing status
// included, as JSON, for core.StateFile.
func (s *MemoryStore) Snapshot() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	snap := storeSnapshot{Items: make(map[string]itemSnapshot, len(s.items))}
	for id, it := range s.items {
		snap.Items[id] = itemSnapshot{
			ID:            it.ID,
			Name:          it.Name,
			Description:   it.Description,
			Tags:          it.Tags,
			ProviderClass: it.ProviderClass,
			ServiceClass:  it.ServiceClass,
			Settings:      string(it.Settings),
			Icon:          string(it.Icon),
			Secret:        string(it.Secret),
			Status:        it.Status,
			CreatedAt:     it.CreatedAt,
			ModifiedAt:    it.ModifiedAt,
		}
	}
	return json.MarshalIndent(snap, "", "  ")
}

// Restore replaces the store's items with a snapshot, and advances the ID
// generator past the restored IDs so new items never reuse one. Nothing else
// is needed for restored schedules and triggers to fire: the data plane reads
// them from the store on every tick and every injected event.
func (s *MemoryStore) Restore(data []byte) error {
	var snap storeSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return err
	}
	items := make(map[string]*ServiceItem, len(snap.Items))
	for id, it := range snap.Items {
		if it.ID != id {
			return fmt.Errorf("item %q: record has a different id %q", id, it.ID)
		}
		switch it.ProviderClass {
		case classProcessConfiguration, classSchedule, classTrigger:
		default:
			return fmt.Errorf("item %q: unknown provider class %q", id, it.ProviderClass)
		}
		for name, raw := range map[string]string{"settings": it.Settings, "icon": it.Icon, "secret": it.Secret} {
			if raw != "" && !json.Valid([]byte(raw)) {
				return fmt.Errorf("item %q: %s is not valid JSON", id, name)
			}
		}
		items[id] = &ServiceItem{
			ID:            it.ID,
			Name:          it.Name,
			Description:   it.Description,
			Tags:          it.Tags,
			ProviderClass: it.ProviderClass,
			ServiceClass:  it.ServiceClass,
			Settings:      rawJSON(it.Settings),
			Icon:          rawJSON(it.Icon),
			Secret:        rawJSON(it.Secret),
			Status:        it.Status,
			CreatedAt:     it.CreatedAt,
			ModifiedAt:    it.ModifiedAt,
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = items
	for id := range s.items {
		s.ids.Observe(id)
	}
	return nil
}
