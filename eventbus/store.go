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
	Secret json.RawMessage
	// Status is the last firing outcome of a schedule or trigger, populated by
	// the data plane when it fires the referenced process configuration. It is
	// nil until the resource fires for the first time, so a control-plane-only
	// server (no data plane) never reports a Status, matching the real API's
	// behavior for a resource that has not yet run.
	Status     *ItemStatus
	CreatedAt  time.Time
	ModifiedAt time.Time
}

// ItemStatus is the firing outcome reported in a CommonServiceItem's Status.
type ItemStatus struct {
	Success   bool
	Message   string
	UpdatedAt time.Time
}

// Store is the interface for EventBus storage backends.
type Store interface {
	CreateItem(item ServiceItem) ServiceItem
	GetItem(id string) (ServiceItem, bool)
	ListItems(providerClass string) []ServiceItem // empty providerClass returns all
	UpdateItem(id, name, description string, tags []string, settings, icon json.RawMessage) (ServiceItem, bool)
	DeleteItem(id string) (ServiceItem, bool)
	SetSecret(id string, secret json.RawMessage) (ServiceItem, bool)
	SetStatus(id string, status ItemStatus) (ServiceItem, bool)

	Close() error
}
