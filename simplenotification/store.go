package simplenotification

import (
	"encoding/json"
	"time"
)

// MessageRecord represents a notification message accepted by the mock.
type MessageRecord struct {
	ID        string
	GroupID   string
	Message   string
	CreatedAt time.Time
}

// ServiceItem is a control-plane resource (a notification destination, group, or
// routing) created via the CommonServiceItem API. Settings and Icon are stored
// as raw JSON and echoed back verbatim, so the mock need not model the
// polymorphic settings of each provider class.
type ServiceItem struct {
	ID            string
	Name          string
	Description   string
	Tags          []string
	ProviderClass string
	ServiceClass  string
	Settings      json.RawMessage
	Icon          json.RawMessage
	CreatedAt     time.Time
	ModifiedAt    time.Time
}

// Store is the interface for Simple Notification storage backends.
type Store interface {
	// Notification messages (data plane).
	Send(groupID, message string, now time.Time) (MessageRecord, error)
	List() []MessageRecord
	Reset()

	// Control-plane service items (destinations, groups, routings).
	CreateItem(item ServiceItem) ServiceItem
	GetItem(id string) (ServiceItem, bool)
	ListItems(providerClass string) []ServiceItem // empty providerClass returns all
	UpdateItem(id, name, description string, tags []string, settings, icon json.RawMessage) (ServiceItem, bool)
	DeleteItem(id string) (ServiceItem, bool)

	Close() error
}
