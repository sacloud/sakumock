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
	Routes    map[string]string `json:"routes"`
	SkipPaths []string          `json:"skipPaths"`
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

func routeKey(method, path string, m *Mapping) (string, bool) {
	if m == nil {
		return method + " " + path, false
	}
	if slices.Contains(m.SkipPaths, path) {
		return "", true
	}
	if override, ok := m.Routes[method+" "+path]; ok {
		return override, false
	}
	mapped := m.Prefix + path
	for from, to := range m.PathRewrites {
		mapped = strings.ReplaceAll(mapped, from, to)
	}
	return method + " " + mapped, false
}
