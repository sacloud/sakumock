package seg

import "time"

// SwitchRemark identifies the user's switch the appliance connects to.
type SwitchRemark struct {
	ID string
}

// NetworkRemark describes the network mask applied to the appliance's user
// interface.
type NetworkRemark struct {
	NetworkMaskLen int
}

// ServerRemark is one server's IP address behind the gateway.
type ServerRemark struct {
	IPAddress string
}

// InterfaceRecord is one network interface exposed on the appliance.
type InterfaceRecord struct {
	SwitchID       string
	SwitchName     string
	Scope          string // "user" or "shared"
	IPAddress      string // "" means null (shared-scope interface only)
	UserIPAddress  string // "" means null (user-scope interface only)
	HasSubnet      bool   // true for the shared-scope interface
	NetworkAddress string
	NetworkMaskLen int
	DefaultRoute   string
}

// EnabledServiceConfig is the endpoint/mode configuration for one managed
// service the gateway forwards to.
type EnabledServiceConfig struct {
	Endpoints []string
	Mode      string // "Managed" or ""
}

// EnabledServiceRecord is one managed service the gateway is configured to
// reach.
type EnabledServiceRecord struct {
	Type   string // "ObjectStorage", "ContainerRegistry", "MonitoringSuite", "AppRunDedicatedControlPlane"
	Config EnabledServiceConfig
}

// DNSForwardingRecord is the private-hosted-zone DNS forwarding config.
type DNSForwardingRecord struct {
	Enabled           bool
	PrivateHostedZone string
	UpstreamDNS1      string
	UpstreamDNS2      string
}

// SettingsRecord is the appliance's applied configuration. A nil
// *SettingsRecord on ApplianceRecord means no configuration has been applied
// yet (the state right after Create).
type SettingsRecord struct {
	EnabledServices []EnabledServiceRecord
	MonitoringSuite *bool // nil means unset
	DNSForwarding   *DNSForwardingRecord
}

// ApplianceRecord represents a Service Endpoint Gateway appliance stored in
// the backend.
type ApplianceRecord struct {
	ID              string
	Name            string
	Description     string
	Tags            []string
	Switch          SwitchRemark
	Network         NetworkRemark
	Servers         []ServerRemark
	ZoneID          string
	Availability    string // "available", "unavailable", "migrating"
	PowerStatus     string // "up", "down", "cleaning"
	StatusChangedAt time.Time
	Settings        *SettingsRecord
	SettingsHash    string // "" means null
	Interfaces      []InterfaceRecord
	CreatedAt       time.Time
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
