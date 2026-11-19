package monitoringsuite

import (
	"testing"

	"github.com/sacloud/sakumock/core/coretest"
)

// untestedRoutes lists the spec routes no test gets a successful response
// from yet; see coretest.RouteCoverageMain. Add a test and remove the entry.
var untestedRoutes = []string{
	// The mock records no alert history, so reading one always answers 404.
	"GET /alerts/projects/{project_resource_id}/histories/{uid}/",
	"GET /alerts/projects/{project_resource_id}/rules/{rule_uid}/histories/{uid}/",
}

func TestMain(m *testing.M) {
	coretest.RouteCoverageMain(m, responseSchemas, untestedRoutes)
}
