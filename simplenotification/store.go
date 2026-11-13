package simplenotification

import (
	"encoding/json"
	"time"
)

// MessageRecord represents a notification message accepted by the mock. The
// json tags are its form in the state file (--state-dir), not in API
// responses.
type MessageRecord struct {
	ID        string    `json:"id"`
	GroupID   string    `json:"group_id"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

// ServiceItem is a control-plane resource (a notification destination, group, or
// routing) created via the CommonServiceItem API. Settings and Icon are stored
// as raw JSON and echoed back verbatim, so the mock need not model the
// polymorphic settings of each provider class. The json tags are its form in
// the state file (--state-dir), not in API responses.
type ServiceItem struct {
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Description   string          `json:"description"`
	Tags          []string        `json:"tags"`
	ProviderClass string          `json:"provider_class"`
	ServiceClass  string          `json:"service_class"`
	Settings      json.RawMessage `json:"settings,omitempty"`
	Icon          json.RawMessage `json:"icon,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	ModifiedAt    time.Time       `json:"modified_at"`
}

// Store is the storage backend for accepted notifications and control-plane
// items.
type Store interface {
	Send(groupID, message string, now time.Time) (MessageRecord, error)
	List() []MessageRecord
	Reset()

	CreateItem(item ServiceItem) ServiceItem
	GetItem(id string) (ServiceItem, bool)
	ListItems(providerClass string) []ServiceItem // empty providerClass returns all
	UpdateItem(id, name, description string, tags []string, settings, icon json.RawMessage) (ServiceItem, bool)
	// UpdateItemSettings replaces only the item's Settings, leaving the other
	// fields untouched (used by routing reorder to patch PriorityRank).
	UpdateItemSettings(id string, settings json.RawMessage) (ServiceItem, bool)
	DeleteItem(id string) (ServiceItem, bool)

	Close() error
}
