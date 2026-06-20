package apprun

import "time"

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
}

type Component struct {
	Name         string
	MaxCPU       string
	MaxMemory    string
	DeploySource DeploySource
	Env          []EnvVar
	Probe        *Probe
}

type DeploySource struct {
	ContainerRegistry *ContainerRegistry
}

type ContainerRegistry struct {
	Image    string
	Server   string
	Username string
}

type EnvVar struct {
	Key   string
	Value string
}

type Probe struct {
	HTTPGet *HTTPGetProbe
}

type HTTPGetProbe struct {
	Path    string
	Port    int
	Headers []Header
}

type Header struct {
	Name  string
	Value string
}

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
}

type TrafficItem struct {
	VersionName     string
	IsLatestVersion bool
	Percent         int
}

type PacketFilter struct {
	IsEnabled bool
	Settings  []PacketFilterSetting
}

type PacketFilterSetting struct {
	FromIP             string
	FromIPPrefixLength int
}

type ListParams struct {
	PageNum   int
	PageSize  int
	SortField string
	SortOrder string
}

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
