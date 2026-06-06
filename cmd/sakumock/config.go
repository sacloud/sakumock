package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecthomas/kong"
	"gopkg.in/yaml.v3"
)

// configFileFlag is a flag value that loads service options from a YAML or JSON
// file (selected by extension) and feeds them to kong as a resolver, so config
// values fill in any flag the user did not pass on the command line.
type configFileFlag string

// BeforeResolve installs the file's values as a kong resolver before the
// remaining flags are resolved, so explicit CLI flags still take precedence.
func (configFileFlag) BeforeResolve(_ *kong.Kong, ctx *kong.Context, trace *kong.Path) error {
	path := string(ctx.FlagValue(trace.Flag).(configFileFlag))
	if path == "" {
		return nil
	}
	resolver, err := newConfigResolver(path)
	if err != nil {
		return err
	}
	ctx.AddResolver(resolver)
	return nil
}

// newConfigResolver reads path (YAML or JSON, by extension) into a nested map
// and returns a resolver that maps each flag to it.
func newConfigResolver(path string) (kong.Resolver, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	values := map[string]any{}
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".json":
		if err := json.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &values); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file extension %q (use .json, .yaml, or .yml)", ext)
	}
	return configResolver(values), nil
}

// configResolver maps a flag to the config map. Service-prefixed flags such as
// "kms-latency" are looked up as nested keys ("kms" -> "latency"), so the file
// groups options per service. Splitting on the first "-" needs no service list,
// so a new service is configurable without touching this resolver. Names that
// do not match a nested group fall back to a top-level key.
func configResolver(values map[string]any) kong.ResolverFunc {
	return func(_ *kong.Context, _ *kong.Path, flag *kong.Flag) (any, error) {
		if group, option, ok := strings.Cut(flag.Name, "-"); ok {
			if sub, ok := values[group].(map[string]any); ok {
				if v, ok := sub[option]; ok {
					return v, nil
				}
			}
		}
		if v, ok := values[flag.Name]; ok {
			return v, nil
		}
		return nil, nil
	}
}
