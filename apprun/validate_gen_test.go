package apprun

import (
	"testing"

	"github.com/sacloud/sakumock/core"
)

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
	for key := range responseSchemas {
		if !routes[key] {
			t.Errorf("responseSchemas key %q has no matching route", key)
		}
	}
}

// TestUnimplementedRoutesRegistered verifies every generated
// unimplementedRoutes key is registered as a 501 route and never shadows an
// implemented one (a path implemented but left in skipPaths).
func TestUnimplementedRoutesRegistered(t *testing.T) {
	s := &Server{}
	kinds := map[string][]string{}
	for _, r := range s.routeTable() {
		key := r.Method + " " + r.Path
		kinds[key] = append(kinds[key], r.Kind)
	}
	for _, key := range unimplementedRoutes {
		if got := kinds[key]; len(got) != 1 || got[0] != core.UnimplementedKind {
			t.Errorf("unimplementedRoutes key %q registered as %v, want only %q", key, got, core.UnimplementedKind)
		}
	}
}
