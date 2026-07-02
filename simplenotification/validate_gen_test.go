package simplenotification

import "testing"

// TestBodySchemasMatchRoutes verifies every generated bodySchemas key
// corresponds to a registered route, catching drift between the OpenAPI spec
// and routeTable (e.g. a renamed path parameter would orphan its schema and
// silently disable validation for that route).
func TestBodySchemasMatchRoutes(t *testing.T) {
	s := &Server{} // rateLimiter and validator are nil-safe, so routeTable works
	routes := map[string]bool{}
	for _, r := range s.routeTable() {
		routes[r.Method+" "+r.Path] = true
	}
	for key := range bodySchemas {
		if !routes[key] {
			t.Errorf("bodySchemas key %q has no matching route", key)
		}
	}
}
