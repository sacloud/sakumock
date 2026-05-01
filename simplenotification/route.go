package simplenotification

import (
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
)

// Route describes a single HTTP endpoint exposed by the mock server.
type Route struct {
	Method      string
	Path        string
	Description string
	Kind        string // "api" for SAKURA Cloud-compatible endpoints, "inspection" for sakumock-only helpers
}

type registeredRoute struct {
	Route
	handler http.HandlerFunc
}

func (s *Server) routeTable() []registeredRoute {
	return []registeredRoute{
		{Route{"POST", "/commonserviceitem/{id}/simplenotification/message", "Send a notification message to the specified group", "api"}, s.handleSendMessage},
		{Route{"GET", "/_sakumock/messages", "List accepted notification messages", "inspection"}, s.handleInspectMessages},
		{Route{"DELETE", "/_sakumock/messages", "Clear accepted notification messages", "inspection"}, s.handleResetMessages},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
// Useful for listing supported APIs (see PrintRoutes for a human-readable form).
func (s *Server) Routes() []Route {
	table := s.routeTable()
	out := make([]Route, len(table))
	for i, r := range table {
		out[i] = r.Route
	}
	return out
}

// PrintRoutes writes a human-readable summary of the server's HTTP routes to w,
// grouped by Kind ("api" first, then "inspection").
func (s *Server) PrintRoutes(w io.Writer) error {
	groups := []struct {
		title string
		kind  string
	}{
		{"API:", "api"},
		{"Inspection:", "inspection"},
	}
	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', 0)
	for i, g := range groups {
		if i > 0 {
			if _, err := fmt.Fprintln(tw); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(tw, g.title); err != nil {
			return err
		}
		for _, r := range s.Routes() {
			if r.Kind != g.kind {
				continue
			}
			if _, err := fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.Method, r.Path, r.Description); err != nil {
				return err
			}
		}
	}
	return tw.Flush()
}
