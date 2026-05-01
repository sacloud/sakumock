package core_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/core"
)

func TestRoutesOf(t *testing.T) {
	table := []core.RegisteredRoute{
		{Route: core.Route{Method: "GET", Path: "/a"}, Handler: func(http.ResponseWriter, *http.Request) {}},
		{Route: core.Route{Method: "POST", Path: "/b"}, Handler: func(http.ResponseWriter, *http.Request) {}},
	}
	routes := core.RoutesOf(table)
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	if routes[0].Path != "/a" || routes[1].Path != "/b" {
		t.Fatalf("unexpected order: %+v", routes)
	}
}

func TestPrintRoutes_GroupsAndOrder(t *testing.T) {
	routes := []core.Route{
		{Method: "GET", Path: "/_sakumock/x", Description: "inspect x", Kind: "inspection"},
		{Method: "POST", Path: "/api/y", Description: "y", Kind: "api"},
		{Method: "GET", Path: "/api/x", Description: "x", Kind: "api"},
	}
	var buf bytes.Buffer
	if err := core.PrintRoutes(&buf, routes); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "API:\n") {
		t.Fatalf("expected API: heading, got %q", out)
	}
	if !strings.Contains(out, "Inspection:\n") {
		t.Fatalf("expected Inspection: heading, got %q", out)
	}
	apiIdx := strings.Index(out, "API:")
	inspectIdx := strings.Index(out, "Inspection:")
	if apiIdx >= inspectIdx {
		t.Fatalf("API section should come before Inspection section: %q", out)
	}
	if !strings.Contains(out, "/api/y") || !strings.Contains(out, "/api/x") {
		t.Fatalf("expected api paths in output: %q", out)
	}
}

func TestPrintRoutes_OmitsEmptyGroups(t *testing.T) {
	routes := []core.Route{
		{Method: "GET", Path: "/api/x", Description: "x", Kind: "api"},
	}
	var buf bytes.Buffer
	if err := core.PrintRoutes(&buf, routes); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "Inspection:") {
		t.Fatalf("expected no Inspection: heading when no inspection routes, got %q", buf.String())
	}
}
