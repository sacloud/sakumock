package seg

import "time"

// SwitchRemark identifies the user's switch the appliance connects to. The
// json tags of the record types are their form in the state file
// (--state-dir), not in API responses.
type SwitchRemark struct {
	ID string `json:"id"`
}

// NetworkRemark describes the network mask applied to the appliance's user
// interface.
type NetworkRemark struct {
	NetworkMaskLen int `json:"network_mask_len"`
}

// ServerRemark is one server's IP address behind the gateway.
type ServerRemark struct {
	IPAddress string `json:"ip_address"`
}

// InterfaceRecord is one network interface exposed on the appliance.
type InterfaceRecord struct {
	SwitchID       string `json:"switch_id"`
	SwitchName     string `json:"switch_name"`
	Scope          string `json:"scope"`           // "user" or "shared"
	IPAddress      string `json:"ip_address"`      // "" means null (shared-scope interface only)
	UserIPAddress  string `json:"user_ip_address"` // "" means null (user-scope interface only)
	HasSubnet      bool   `json:"has_subnet"`      // true for the shared-scope interface
	NetworkAddress string `json:"network_address"`
	NetworkMaskLen int    `json:"network_mask_len"`
	DefaultRoute   string `json:"default_route"`
}

// EnabledServiceConfig is the endpoint/mode configuration for one managed
// service the gateway forwards to.
type EnabledServiceConfig struct {
	Endpoints []string `json:"endpoints"`
	Mode      string   `json:"mode"` // "Managed" or ""
}

// EnabledServiceRecord is one managed service the gateway is configured to
// reach.
type EnabledServiceRecord struct {
	Type   string               `json:"type"` // "ObjectStorage", "ContainerRegistry", "MonitoringSuite", "AIEngine", "SimpleAI", "AppRunDedicatedControlPlane"
	Config EnabledServiceConfig `json:"config"`
}

// DNSForwardingRecord is the private-hosted-zone DNS forwarding config.
type DNSForwardingRecord struct {
	Enabled           bool   `json:"enabled"`
	PrivateHostedZone string `json:"private_hosted_zone"`
	UpstreamDNS1      string `json:"upstream_dns1"`
	UpstreamDNS2      string `json:"upstream_dns2"`
}

// SettingsRecord is the appliance's applied configuration. A nil
// *SettingsRecord on ApplianceRecord means no configuration has been applied
// yet (the state right after Create).
type SettingsRecord struct {
	EnabledServices []EnabledServiceRecord `json:"enabled_services"`
	MonitoringSuite *bool                  `json:"monitoring_suite,omitempty"` // nil means unset
	DNSForwarding   *DNSForwardingRecord   `json:"dns_forwarding,omitempty"`
}

// ApplianceRecord represents a Service Endpoint Gateway appliance stored in
// the backend.
type ApplianceRecord struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Tags            []string          `json:"tags"`
	Switch          SwitchRemark      `json:"switch"`
	Network         NetworkRemark     `json:"network"`
	Servers         []ServerRemark    `json:"servers"`
	ZoneID          string            `json:"zone_id"`
	Availability    string            `json:"availability"` // "available", "unavailable", "migrating"
	PowerStatus     string            `json:"power_status"` // "up", "down", "cleaning"
	StatusChangedAt time.Time         `json:"status_changed_at"`
	Settings        *SettingsRecord   `json:"settings,omitempty"`
	SettingsHash    string            `json:"settings_hash"` // "" means null
	Interfaces      []InterfaceRecord `json:"interfaces"`
	CreatedAt       time.Time         `json:"created_at"`
}

// Store is the storage backend for Service Endpoint Gateway appliances.
type Store interface {
	List() []ApplianceRecord
	Create(name, description string, tags []string, sw SwitchRemark, network NetworkRemark, servers []ServerRemark) (ApplianceRecord, error)
	Read(id string) (ApplianceRecord, error)
	Update(id string, settings SettingsRecord) (ApplianceRecord, error)
	Delete(id string) error
	Apply(id string) (ApplianceRecord, error)

	ReadInterface(id, interfaceID string) (InterfaceRecord, error)
	PowerOn(id string) (ApplianceRecord, error)
	PowerOff(id string) (ApplianceRecord, error)
	Reset(id string) (ApplianceRecord, error)

	Close() error
}
