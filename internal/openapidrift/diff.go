package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// result is the comparison outcome for one local spec.
type result struct {
	Spec          apiSpec
	PortalURL     string
	LocalVersion  string
	PortalVersion string
	// Differences lists every structural difference (and a version mismatch)
	// in a deterministic order.
	Differences []string
}

func (r result) Drifted() bool { return len(r.Differences) > 0 }

// StructuralCount excludes the info.version line.
func (r result) StructuralCount() int {
	if r.LocalVersion != r.PortalVersion {
		return len(r.Differences) - 1
	}
	return len(r.Differences)
}

func compareSpec(spec apiSpec, portalURL string, local, upstream map[string]any) result {
	r := result{
		Spec:          spec,
		PortalURL:     portalURL,
		LocalVersion:  nestedString(local, "info", "version"),
		PortalVersion: nestedString(upstream, "info", "version"),
	}
	var d differences
	d.addComponentDifferences(nestedMap(local, "components"), nestedMap(upstream, "components"), spec.SkipComponentSetChanges)
	if !spec.SkipPaths {
		d.compare("paths", nestedMap(local, "paths"), nestedMap(upstream, "paths"))
	}
	if r.LocalVersion != r.PortalVersion {
		d.add(fmt.Sprintf("info.version: local %q; portal %q", r.LocalVersion, r.PortalVersion))
	}
	r.Differences = d
	return r
}

type differences []string

func (d *differences) add(s string) { *d = append(*d, s) }

// addComponentDifferences compares components section by section. With
// skipSetChanges only the entries both sides define are compared, since a
// shared portal spec carries other services' components.
func (d *differences) addComponentDifferences(local, upstream map[string]any, skipSetChanges bool) {
	if !skipSetChanges {
		d.compare("components", local, upstream)
		return
	}
	for _, section := range intersection(local, upstream) {
		localItems, ok1 := local[section].(map[string]any)
		upstreamItems, ok2 := upstream[section].(map[string]any)
		if !ok1 || !ok2 {
			continue
		}
		for _, name := range intersection(localItems, upstreamItems) {
			d.compare("components."+section+"."+name, localItems[name], upstreamItems[name])
		}
	}
}

func (d *differences) compare(label string, local, upstream any) {
	localMap, localIsMap := local.(map[string]any)
	upstreamMap, upstreamIsMap := upstream.(map[string]any)
	if localIsMap && upstreamIsMap {
		for _, key := range union(localMap, upstreamMap) {
			if isDocKey(label, key) {
				continue
			}
			localValue, localOK := localMap[key]
			upstreamValue, upstreamOK := upstreamMap[key]
			switch {
			case !localOK:
				d.add(fmt.Sprintf("%s.%s: present in portal, missing locally", label, key))
			case !upstreamOK:
				d.add(fmt.Sprintf("%s.%s: present locally, missing from portal", label, key))
			default:
				d.compare(label+"."+key, localValue, upstreamValue)
			}
		}
		return
	}

	localArray, localIsArray := local.([]any)
	upstreamArray, upstreamIsArray := upstream.([]any)
	if localIsArray && upstreamIsArray {
		if len(localArray) != len(upstreamArray) {
			d.add(fmt.Sprintf("%s: array length local %d; portal %d", label, len(localArray), len(upstreamArray)))
		}
		for i := range min(len(localArray), len(upstreamArray)) {
			d.compare(fmt.Sprintf("%s[%d]", label, i), localArray[i], upstreamArray[i])
		}
		return
	}

	if compactJSON(local, 0) != compactJSON(upstream, 0) {
		d.add(fmt.Sprintf("%s: value differs (local %s; portal %s)", label, compactJSON(local, 100), compactJSON(upstream, 100)))
	}
}

// docKeys are OpenAPI keywords that only document the API; their changes do
// not affect what the mock must accept or return.
var docKeys = map[string]bool{
	"description": true,
	"summary":     true,
	"example":     true,
	"examples":    true,
	"title":       true,
	"tags":        true,
}

// isDocKey reports whether key under label is a documentation keyword or a
// vendor extension (x-*). Keys of a properties map are field names, never
// keywords.
func isDocKey(label, key string) bool {
	if strings.HasSuffix(label, ".properties") {
		return false
	}
	return docKeys[key] || strings.HasPrefix(key, "x-")
}

// compactJSON renders v as JSON, truncated to limit bytes when limit > 0.
func compactJSON(v any, limit int) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	s := string(b)
	if limit > 0 && len(s) > limit {
		return s[:limit-3] + "..."
	}
	return s
}

func union(a, b map[string]any) []string {
	keys := slices.Collect(maps.Keys(a))
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

func intersection(a, b map[string]any) []string {
	var keys []string
	for k := range a {
		if _, ok := b[k]; ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}
