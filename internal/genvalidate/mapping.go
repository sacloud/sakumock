package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Mapping bridges spec paths to mock route paths for services whose route
// table does not mirror the spec verbatim (re-prefixed paths, renamed path
// parameters, unimplemented spec paths).
type Mapping struct {
	// Prefix is prepended to every spec path (e.g. "/{site}" for a server-URL
	// variable the mock serves as a path parameter).
	Prefix string `json:"prefix"`
	// PathRewrites renames path segments in every spec path
	// (e.g. "{resource_id}" -> "{vault_resource_id}").
	PathRewrites map[string]string `json:"pathRewrites"`
	// Routes overrides individual operations: "METHOD /spec/path" -> mock
	// route key. It wins over Prefix/PathRewrites.
	Routes map[string]string `json:"routes"`
	// SkipPaths lists spec paths the mock does not implement. Their operations
	// get no validation entries and are emitted in unimplementedRoutes instead
	// (served as 501 Not Implemented); an entry matching no spec path is an
	// error, so a stale one is noticed when the spec changes.
	SkipPaths []string `json:"skipPaths"`
}

func LoadMapping(path string) (*Mapping, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Mapping
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// routeKey maps a spec operation to its mock route key. skip reports that the
// path is in SkipPaths; the key is still returned so the operation can be
// registered as unimplemented.
func routeKey(method, path string, m *Mapping) (key string, skip bool) {
	if m == nil {
		return method + " " + path, false
	}
	skip = slices.Contains(m.SkipPaths, path)
	if override, ok := m.Routes[method+" "+path]; ok {
		return override, skip
	}
	mapped := m.Prefix + path
	for from, to := range m.PathRewrites {
		mapped = strings.ReplaceAll(mapped, from, to)
	}
	return method + " " + mapped, skip
}
