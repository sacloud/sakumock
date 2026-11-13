package apprun

import "time"

// Application is an AppRun application: the deployable unit, addressed by its
// public URL. The json tags (here and on the other record types) are the form
// in the state file (--state-dir), not in API responses.
type Application struct {
	ID                     string      `json:"id"`
	Name                   string      `json:"name"`
	TimeoutSeconds         int         `json:"timeout_seconds"`
	Port                   int         `json:"port"`
	MinScale               int         `json:"min_scale"`
	MaxScale               int         `json:"max_scale"`
	ScaleTargetConcurrency int         `json:"scale_target_concurrency"`
	Components             []Component `json:"components"`
	// Status follows the application's container and PublicURL the data plane
	// address, so neither is saved: both are set again when the state is
	// restored.
	Status     string    `json:"-"`
	PublicURL  string    `json:"-"`
	ResourceID string    `json:"resource_id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// seq records the creation order. CreatedAt is truncated to the second
	// (as the real API reports it), so resources created within the same
	// second compare equal; seq keeps list ordering deterministic.
	seq int
}

// Component is one container of an application.
type Component struct {
	Name         string       `json:"name"`
	MaxCPU       string       `json:"max_cpu"`
	MaxMemory    string       `json:"max_memory"`
	DeploySource DeploySource `json:"deploy_source"`
	Env          []EnvVar     `json:"env"`
	// Secret holds the environment variables marked as secret. They reach the
	// container like Env, but the API reports only their keys.
	Secret []EnvVar `json:"secret"`
	Probe  *Probe   `json:"probe"`
}

// containerEnv returns the environment the container is started with: plain
// env vars followed by secrets.
func (c Component) containerEnv() []EnvVar {
	if len(c.Secret) == 0 {
		return c.Env
	}
	return append(append([]EnvVar{}, c.Env...), c.Secret...)
}

// DeploySource is where a component's image comes from.
type DeploySource struct {
	ContainerRegistry *ContainerRegistry `json:"container_registry"`
}

// ContainerRegistry is the registry a component's image is pulled from.
type ContainerRegistry struct {
	Image    string `json:"image"`
	Server   string `json:"server"`
	Username string `json:"username"`
}

// EnvVar is an environment variable (plain or secret) passed to a component.
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Probe is a component's health check.
type Probe struct {
	HTTPGet *HTTPGetProbe `json:"http_get"`
}

// HTTPGetProbe is a health check performed with an HTTP GET.
type HTTPGetProbe struct {
	Path    string   `json:"path"`
	Port    int      `json:"port"`
	Headers []Header `json:"headers"`
}

// Header is one HTTP header sent by an HTTPGetProbe.
type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Version is an immutable snapshot of an application, created on every update.
type Version struct {
	ID                     string      `json:"id"`
	AppID                  string      `json:"app_id"`
	Name                   string      `json:"name"`
	Status                 string      `json:"status"`
	TimeoutSeconds         int         `json:"timeout_seconds"`
	Port                   int         `json:"port"`
	MinScale               int         `json:"min_scale"`
	MaxScale               int         `json:"max_scale"`
	ScaleTargetConcurrency int         `json:"scale_target_concurrency"`
	Components             []Component `json:"components"`
	CreatedAt              time.Time   `json:"created_at"`

	// seq records the creation order; see Application.seq.
	seq int
}

// TrafficItem assigns a share of an application's traffic to one version.
type TrafficItem struct {
	VersionName     string `json:"version_name"`
	IsLatestVersion bool   `json:"is_latest_version"`
	Percent         int    `json:"percent"`
}

// PacketFilter restricts which clients may reach an application.
type PacketFilter struct {
	IsEnabled bool                  `json:"is_enabled"`
	Settings  []PacketFilterSetting `json:"settings"`
}

// PacketFilterSetting is one allow rule of a PacketFilter.
type PacketFilterSetting struct {
	FromIP             string `json:"from_ip"`
	FromIPPrefixLength int    `json:"from_ip_prefix_length"`
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
