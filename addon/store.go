package addon

import (
	"encoding/json"
	"errors"
	"time"
)

// Kind identifies one add-on service family. The real API exposes the same
// resource-group lifecycle (list / create / get / delete / deployment status)
// under a different path per family; Kind is what tells the stored resources
// of one family apart from another's.
type Kind string

const (
	KindAI            Kind = "ai"
	KindCDN           Kind = "cdn"
	KindDDoS          Kind = "ddos"
	KindWAF           Kind = "waf"
	KindVulnerability Kind = "vulnerability"
	KindDataLake      Kind = "datalake"
	KindDWH           Kind = "dwh"
	KindETL           Kind = "etl"
	KindQuery         Kind = "query"
	KindSearch        Kind = "search"
	KindStreaming     Kind = "streaming"
)

// errNotFound is returned by the store when no resource matches the lookup.
var errNotFound = errors.New("resource not found")

// Resource is one add-on resource group. The real API deploys an Azure
// resource group per request and identifies it by the generated
// resourceGroupName; the deployment that created it is identified separately
// by deploymentName, which is what the status endpoint reports on.
type Resource struct {
	Kind              Kind
	ResourceGroupName string
	DeploymentName    string
	Location          string
	// Parameters is the create request body verbatim. The real API returns
	// the deployed Azure resource here; the mock echoes what it was given so
	// a test can assert on what its client sent.
	Parameters json.RawMessage
	CreatedAt  time.Time
	// ReadyAt is when provisioning completes. Until then the resource is
	// invisible to list/get (the real API 404s a resource group whose
	// deployment is still running) while the status endpoint reports
	// "Running". With no --provisioning-delay it equals CreatedAt.
	ReadyAt time.Time
}

// Provisioned reports whether the resource's deployment has completed as of
// now.
func (r Resource) Provisioned(now time.Time) bool { return !now.Before(r.ReadyAt) }

// Store is the storage backend for add-on resource groups.
type Store interface {
	// List returns every provisioned resource of the given family, oldest
	// first.
	List(kind Kind, now time.Time) []Resource
	// Read returns the provisioned resource with the given resource group
	// name, or errNotFound.
	Read(kind Kind, resourceGroupName string, now time.Time) (Resource, error)
	// ReadAny returns the resource whether or not it finished provisioning;
	// the status endpoint reports on a deployment that is still running.
	ReadAny(kind Kind, resourceGroupName string) (Resource, error)
	// Create records a new resource group and the deployment that created it.
	Create(kind Kind, location string, params json.RawMessage) Resource
	// Delete removes the resource group, provisioned or not.
	Delete(kind Kind, resourceGroupName string) error

	Close() error
}
