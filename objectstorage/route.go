package objectstorage

import (
	"net/http"

	"github.com/sacloud/sakumock/core"
)

// routeTable is the single source of truth driving both buildMux() and Routes().
//
// The federation API is served under /fed/v1 and the site (cluster) API under
// /{site}/v2, matching how the SDK's FedClient and SiteClient join their base
// URLs onto the configured endpoint.
func (s *Server) routeTable() []core.RegisteredRoute {
	rl := func(h http.HandlerFunc) http.HandlerFunc {
		return s.rateLimiter.Middleware(core.GlobalKey(), h)
	}
	api := func(method, path, desc string, h http.HandlerFunc) core.RegisteredRoute {
		return core.RegisteredRoute{Route: core.Route{Method: method, Path: path, Description: desc, Kind: "api"}, Handler: rl(h)}
	}
	return []core.RegisteredRoute{
		// Federation API (/fed/v1).
		api("GET", "/fed/v1/clusters", "List object storage clusters (sites)", s.handleListClusters),
		api("GET", "/fed/v1/clusters/{id}", "Get an object storage cluster (site)", s.handleGetCluster),
		api("PUT", "/fed/v1/buckets/{name}", "Create a bucket", s.handleCreateBucket),
		api("DELETE", "/fed/v1/buckets/{name}", "Delete a bucket", s.handleDeleteBucket),
		api("GET", "/fed/v1/buckets/{name}/replication", "Get a bucket's replication configuration", s.handleGetReplication),
		api("POST", "/fed/v1/buckets/{name}/replication", "Enable a bucket's replication", s.handlePostReplication),
		api("DELETE", "/fed/v1/buckets/{name}/replication", "Disable a bucket's replication", s.handleDeleteReplication),
		api("GET", "/fed/v1/buckets/{name}/replicable-targets", "List replication target candidates for a bucket", s.handleReplicableTargets),

		// Site API (/{site}/v2).
		api("GET", "/{site}/v2/buckets", "List buckets in a site", s.handleListBuckets),
		api("GET", "/{site}/v2/account", "Get the site account", s.handleGetAccount),
		api("POST", "/{site}/v2/account", "Create the site account", s.handleCreateAccount),
		api("DELETE", "/{site}/v2/account", "Delete the site account", s.handleDeleteAccount),
		api("GET", "/{site}/v2/account/keys", "List root access keys", s.handleListAccountKeys),
		api("POST", "/{site}/v2/account/keys", "Create a root access key", s.handleCreateAccountKey),
		api("GET", "/{site}/v2/account/keys/{id}", "Get a root access key", s.handleGetAccountKey),
		api("DELETE", "/{site}/v2/account/keys/{id}", "Delete a root access key", s.handleDeleteAccountKey),
		api("GET", "/{site}/v2/permissions", "List permissions", s.handleListPermissions),
		api("POST", "/{site}/v2/permissions", "Create a permission", s.handleCreatePermission),
		api("GET", "/{site}/v2/permissions/{id}", "Get a permission", s.handleGetPermission),
		api("PUT", "/{site}/v2/permissions/{id}", "Update a permission", s.handleUpdatePermission),
		api("DELETE", "/{site}/v2/permissions/{id}", "Delete a permission", s.handleDeletePermission),
		api("GET", "/{site}/v2/permissions/{id}/keys", "List a permission's access keys", s.handleListPermissionKeys),
		api("POST", "/{site}/v2/permissions/{id}/keys", "Create a permission access key", s.handleCreatePermissionKey),
		api("GET", "/{site}/v2/permissions/{id}/keys/{key_id}", "Get a permission access key", s.handleGetPermissionKey),
		api("DELETE", "/{site}/v2/permissions/{id}/keys/{key_id}", "Delete a permission access key", s.handleDeletePermissionKey),
		api("GET", "/{site}/v2/status", "Get site status", s.handleStatus),
		api("GET", "/{site}/v2/plans", "List bucket plans", s.handlePlans),
		api("GET", "/{site}/v2/quota", "Get site quota", s.handleQuota),
		api("GET", "/{site}/v2/metering/buckets/{name}", "Get a bucket's metering (billing)", s.handleBucketMetering),
		api("GET", "/{site}/v2/buckets/{name}/encryption", "Get a bucket's encryption configuration", s.handleGetEncryption),
		api("PUT", "/{site}/v2/buckets/{name}/encryption", "Enable a bucket's encryption", s.handlePutEncryption),
		api("DELETE", "/{site}/v2/buckets/{name}/encryption", "Disable a bucket's encryption", s.handleDeleteEncryption),
		api("GET", "/{site}/v2/buckets/{name}/penalty", "Get a bucket's penalty status", s.handleBucketPenalty),
		api("GET", "/{site}/v2/buckets/{name}/usage", "Get a bucket's usage", s.handleBucketUsage),
		api("GET", "/{site}/v2/buckets/{name}/quota", "Get a bucket's quota", s.handleBucketQuota),
		api("GET", "/{site}/v2/buckets/{name}/plan", "Get a bucket's plan and contract", s.handleGetBucketPlan),
		api("PUT", "/{site}/v2/buckets/{name}/plan", "Change a bucket's plan", s.handlePutBucketPlan),
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
func (s *Server) Routes() []core.Route {
	return core.RoutesOf(s.routeTable())
}
