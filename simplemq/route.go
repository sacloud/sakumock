package simplemq

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
		{Route{"POST", "/v1/queues/{queueName}/messages", "Send a message to the queue", "api"}, s.authMiddleware(s.handleSend)},
		{Route{"GET", "/v1/queues/{queueName}/messages", "Receive messages from the queue", "api"}, s.authMiddleware(s.handleReceive)},
		{Route{"PUT", "/v1/queues/{queueName}/messages/{messageId}", "Extend the visibility timeout of a message", "api"}, s.authMiddleware(s.handleExtendTimeout)},
		{Route{"DELETE", "/v1/queues/{queueName}/messages/{messageId}", "Delete a message from the queue", "api"}, s.authMiddleware(s.handleDelete)},
	}
}

// Routes returns metadata for every HTTP endpoint registered on the server.
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
	first := true
	for _, g := range groups {
		var matched []Route
		for _, r := range s.Routes() {
			if r.Kind == g.kind {
				matched = append(matched, r)
			}
		}
		if len(matched) == 0 {
			continue
		}
		if !first {
			if _, err := fmt.Fprintln(tw); err != nil {
				return err
			}
		}
		first = false
		if _, err := fmt.Fprintln(tw, g.title); err != nil {
			return err
		}
		for _, r := range matched {
			if _, err := fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.Method, r.Path, r.Description); err != nil {
				return err
			}
		}
	}
	return tw.Flush()
}
