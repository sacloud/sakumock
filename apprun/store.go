package apprun

import "time"

// Application is an AppRun application: the deployable unit, addressed by its
// public URL.
type Application struct {
	ID                     string
	Name                   string
	TimeoutSeconds         int
	Port                   int
	MinScale               int
	MaxScale               int
	ScaleTargetConcurrency int
	Components             []Component
	Status                 string
	PublicURL              string
	ResourceID             string
	CreatedAt              time.Time
	UpdatedAt              time.Time

	// seq records the creation order. CreatedAt is truncated to the second
	// (as the real API reports it), so resources created within the same
	// second compare equal; seq keeps list ordering deterministic.
	seq int
}

// Component is one container of an application.
type Component struct {
	Name         string
	MaxCPU       string
	MaxMemory    string
	DeploySource DeploySource
	Env          []EnvVar
	Probe        *Probe
}

// DeploySource is where a component's image comes from.
type DeploySource struct {
	ContainerRegistry *ContainerRegistry
}

// ContainerRegistry is the registry a component's image is pulled from.
type ContainerRegistry struct {
	Image    string
	Server   string
	Username string
}

// EnvVar is an environment variable passed to a component.
type EnvVar struct {
	Key   string
	Value string
}

// Probe is a component's health check.
type Probe struct {
	HTTPGet *HTTPGetProbe
}

// HTTPGetProbe is a health check performed with an HTTP GET.
type HTTPGetProbe struct {
	Path    string
	Port    int
	Headers []Header
}

// Header is one HTTP header sent by an HTTPGetProbe.
type Header struct {
	Name  string
	Value string
}

// Version is an immutable snapshot of an application, created on every update.
type Version struct {
	ID                     string
	AppID                  string
	Name                   string
	Status                 string
	TimeoutSeconds         int
	Port                   int
	MinScale               int
	MaxScale               int
	ScaleTargetConcurrency int
	Components             []Component
	CreatedAt              time.Time

	// seq records the creation order; see Application.seq.
	seq int
}

// TrafficItem assigns a share of an application's traffic to one version.
type TrafficItem struct {
	VersionName     string
	IsLatestVersion bool
	Percent         int
}

// PacketFilter restricts which clients may reach an application.
type PacketFilter struct {
	IsEnabled bool
	Settings  []PacketFilterSetting
}

// PacketFilterSetting is one allow rule of a PacketFilter.
type PacketFilterSetting struct {
	FromIP             string
	FromIPPrefixLength int
}

// ListParams carries the paging and sorting options of the list endpoints.
type ListParams struct {
	PageNum   int
	PageSize  int
	SortField string
	SortOrder string
}

// Store is the storage backend for AppRun applications and their versions.
type Store interface {
	UserCreated() bool
	CreateUser()

	ListApplications(params ListParams) ([]*Application, int)
	CreateApplication(app *Application) error
	ReadApplication(id string) (*Application, bool)
	UpdateApplication(id string, app *Application) error
	DeleteApplication(id string) error

	ListVersions(appID string, params ListParams) ([]*Version, int)
	ReadVersion(appID, versionID string) (*Version, bool)
	DeleteVersion(appID, versionID string) error

	GetTraffic(appID string) ([]TrafficItem, bool)
	PutTraffic(appID string, items []TrafficItem) error

	GetPacketFilter(appID string) (*PacketFilter, bool)
	PatchPacketFilter(appID string, pf *PacketFilter) error

	Close()
}
