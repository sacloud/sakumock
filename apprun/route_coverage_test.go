package apprun

import (
	"testing"

	"github.com/sacloud/sakumock/core/coretest"
)

// untestedRoutes lists the spec routes no test gets a successful response
// from yet; see coretest.RouteCoverageMain. Add a test and remove the entry.
var untestedRoutes = []string{}

func TestMain(m *testing.M) {
	coretest.RouteCoverageMain(m, responseSchemas, untestedRoutes)
}
