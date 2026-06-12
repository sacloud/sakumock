package eventbus

import (
	"encoding/json"
	"time"
)

// ServiceItem is a control-plane resource (a process configuration, schedule,
// or trigger) created via the CommonServiceItem API. Settings and Icon are
// stored as raw JSON and echoed back verbatim, so the mock need not model the
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
	// Secret is set via the set-secret endpoint on process configurations.
	// It is write-only in the API and never included in CSI responses.
	Secret     json.RawMessage
	CreatedAt  time.Time
	ModifiedAt time.Time
}

// Store is the interface for EventBus storage backends.
type Store interface {
	CreateItem(item ServiceItem) ServiceItem
	GetItem(id string) (ServiceItem, bool)
	ListItems(providerClass string) []ServiceItem // empty providerClass returns all
	UpdateItem(id, name, description string, tags []string, settings, icon json.RawMessage) (ServiceItem, bool)
	DeleteItem(id string) (ServiceItem, bool)
	SetSecret(id string, secret json.RawMessage) (ServiceItem, bool)

	Close() error
}
