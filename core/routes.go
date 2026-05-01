// Package core provides shared building blocks for sakumock service modules.
package core

import (
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"
)

// Route describes a single HTTP endpoint exposed by a sakumock server.
type Route struct {
	Method      string
	Path        string
	Description string
	// Kind is a free-form group label used by PrintRoutes ("api" for SAKURA Cloud-compatible
	// endpoints, "inspection" for sakumock-only helpers).
	Kind string
}

// RegisteredRoute pairs a Route with its handler so that a service's route table
// can drive both the mux registration and the publicly-visible Routes() listing.
type RegisteredRoute struct {
	Route
	Handler http.HandlerFunc
}

// RoutesOf strips handlers from a route table, leaving only public metadata.
func RoutesOf(table []RegisteredRoute) []Route {
	out := make([]Route, len(table))
	for i, r := range table {
		out[i] = r.Route
	}
	return out
}

// PrintRoutes writes a human-readable summary of routes to w, grouped by Kind
// in the order ("api", "inspection") with any other kinds appended in the order
// they first appear.
func PrintRoutes(w io.Writer, routes []Route) error {
	groups := []string{"api", "inspection"}
	seen := map[string]bool{"api": true, "inspection": true}
	for _, r := range routes {
		if !seen[r.Kind] {
			seen[r.Kind] = true
			groups = append(groups, r.Kind)
		}
	}

	tw := tabwriter.NewWriter(w, 0, 0, 4, ' ', 0)
	first := true
	for _, kind := range groups {
		var matched []Route
		for _, r := range routes {
			if r.Kind == kind {
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
		if _, err := fmt.Fprintln(tw, titleFor(kind)); err != nil {
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

func titleFor(kind string) string {
	switch kind {
	case "api":
		return "API:"
	case "inspection":
		return "Inspection:"
	default:
		return kind + ":"
	}
}
