package core_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sacloud/sakumock/core"
)

func TestUnimplementedRoutes(t *testing.T) {
	var gotStatus int
	var gotMsg string
	writeErr := func(w http.ResponseWriter, status int, message string) {
		gotStatus, gotMsg = status, message
		w.WriteHeader(status)
	}
	routes := core.UnimplementedRoutes([]string{"POST /certificates/acme", "GET /limits"}, writeErr)
	if len(routes) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(routes))
	}
	r := routes[0]
	if r.Method != "POST" || r.Path != "/certificates/acme" || r.Kind != core.UnimplementedKind {
		t.Fatalf("unexpected route: %+v", r.Route)
	}

	mux := http.NewServeMux()
	for _, r := range routes {
		mux.HandleFunc(r.Method+" "+r.Path, r.Handler)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/certificates/acme", nil))
	if rec.Code != http.StatusNotImplemented || gotStatus != http.StatusNotImplemented {
		t.Fatalf("status = %d (writer got %d), want 501", rec.Code, gotStatus)
	}
	if gotMsg != "POST /certificates/acme is not implemented by sakumock" {
		t.Fatalf("message = %q", gotMsg)
	}
}

func TestUnimplementedRoutes_Empty(t *testing.T) {
	if got := core.UnimplementedRoutes(nil, nil); len(got) != 0 {
		t.Fatalf("expected no routes, got %d", len(got))
	}
}

func TestUnimplementedRoutes_MalformedKeyPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for malformed key")
		}
	}()
	core.UnimplementedRoutes([]string{"/no-method"}, nil)
}

func TestPrintRoutes_UnimplementedGroup(t *testing.T) {
	routes := []core.Route{
		{Method: "GET", Path: "/_sakumock/health", Description: "h", Kind: "inspection"},
		{Method: "GET", Path: "/limits", Description: "Not implemented (501)", Kind: core.UnimplementedKind},
		{Method: "GET", Path: "/x", Description: "x", Kind: "api"},
	}
	var buf bytes.Buffer
	if err := core.PrintRoutes(&buf, routes); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	api, unimpl, insp := strings.Index(out, "API:"), strings.Index(out, "Not implemented:"), strings.Index(out, "Inspection:")
	if api < 0 || unimpl < 0 || insp < 0 || !(api < unimpl && unimpl < insp) {
		t.Fatalf("expected API, Not implemented, Inspection in order, got %q", out)
	}
}
