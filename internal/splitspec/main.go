// Command splitspec extracts one service's spec from an OpenAPI document
// shared by several services. The portal publishes KMS, CloudHSM and
// SecretManager as a single security-encryption-api.yaml, while each sakumock
// service keeps its own openapi/ spec:
//
//	splitspec -prefix /kms/ -o kms/openapi/openapi.json security-encryption-api.yaml
//
// Only paths starting with -prefix (repeatable) are kept, and components are
// pruned to those transitively reachable through $ref from the kept paths
// (securitySchemes, referenced by name, are kept as is). Everything else is
// copied verbatim in source order and written as JSON, so a portal update
// shows up as a minimal diff. The input may be YAML or JSON ("-" reads stdin).
//
// This tool exists only because the portal bundles those services. Once the
// portal publishes per-service specs, fetch them directly and delete this
// package together with its Makefile calls.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type prefixes []string

func (p *prefixes) String() string     { return strings.Join(*p, ",") }
func (p *prefixes) Set(v string) error { *p = append(*p, v); return nil }

func main() {
	var pfx prefixes
	flag.Var(&pfx, "prefix", "keep paths starting with this prefix (repeatable)")
	out := flag.String("o", "", "output file (default stdout)")
	flag.Parse()
	if err := run(flag.Arg(0), *out, pfx); err != nil {
		fmt.Fprintln(os.Stderr, "splitspec:", err)
		os.Exit(1)
	}
}

func run(in, out string, pfx []string) error {
	if in == "" || flag.NArg() != 1 {
		return fmt.Errorf("usage: splitspec -prefix PREFIX [-o FILE] SPEC")
	}
	if len(pfx) == 0 {
		return fmt.Errorf("at least one -prefix is required")
	}
	var data []byte
	var err error
	if in == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(in)
	}
	if err != nil {
		return err
	}
	result, err := split(data, pfx)
	if err != nil {
		return err
	}
	if out == "" {
		_, err = os.Stdout.Write(result)
		return err
	}
	return os.WriteFile(out, result, 0o644)
}

// split returns the JSON-encoded sub-spec for the given path prefixes.
func split(data []byte, pfx []string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("spec is not a mapping")
	}
	root := doc.Content[0]

	paths := mapValue(root, "paths")
	if paths == nil {
		return nil, fmt.Errorf("spec has no paths")
	}
	filterMapping(paths, func(key string, _ *yaml.Node) bool {
		return slices.ContainsFunc(pfx, func(p string) bool { return strings.HasPrefix(key, p) })
	})
	if len(paths.Content) == 0 {
		return nil, fmt.Errorf("no paths match %q", pfx)
	}

	if components := mapValue(root, "components"); components != nil {
		if err := pruneComponents(components, paths); err != nil {
			return nil, err
		}
	}

	var buf bytes.Buffer
	if err := writeJSON(&buf, root, 0); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// pruneComponents keeps only the components reachable through $ref from
// paths. Sections never targeted by $ref (securitySchemes) are kept whole;
// sections left empty are removed.
func pruneComponents(components, paths *yaml.Node) error {
	reachable := map[string]bool{} // "section/name"
	queue := []*yaml.Node{paths}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]
		for _, ref := range collectRefs(n) {
			if reachable[ref] {
				continue
			}
			section, name, _ := strings.Cut(ref, "/")
			target := mapValue(mapValue(components, section), name)
			if target == nil {
				return fmt.Errorf("unresolved $ref #/components/%s", ref)
			}
			reachable[ref] = true
			queue = append(queue, target)
		}
	}
	filterMapping(components, func(section string, items *yaml.Node) bool {
		if section == "securitySchemes" || items.Kind != yaml.MappingNode {
			return true
		}
		filterMapping(items, func(name string, _ *yaml.Node) bool {
			return reachable[section+"/"+name]
		})
		return len(items.Content) > 0
	})
	return nil
}

// collectRefs returns the "section/name" of every local component $ref
// under n.
func collectRefs(n *yaml.Node) []string {
	var refs []string
	var walk func(*yaml.Node)
	walk = func(n *yaml.Node) {
		if n.Kind == yaml.AliasNode {
			n = n.Alias
		}
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				k, v := n.Content[i], n.Content[i+1]
				if k.Value == "$ref" && v.Kind == yaml.ScalarNode {
					if rest, ok := strings.CutPrefix(v.Value, "#/components/"); ok {
						section, name, _ := strings.Cut(rest, "/")
						refs = append(refs, section+"/"+unescapePointer(name))
					}
				}
			}
		}
		for _, c := range n.Content {
			walk(c)
		}
	}
	walk(n)
	return refs
}

func unescapePointer(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~1", "/"), "~0", "~")
}

func mapValue(m *yaml.Node, key string) *yaml.Node {
	if m == nil || m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

// filterMapping keeps the key/value pairs of m for which keep returns true,
// preserving their order.
func filterMapping(m *yaml.Node, keep func(key string, value *yaml.Node) bool) {
	kept := m.Content[:0]
	for i := 0; i+1 < len(m.Content); i += 2 {
		if keep(m.Content[i].Value, m.Content[i+1]) {
			kept = append(kept, m.Content[i], m.Content[i+1])
		}
	}
	m.Content = kept
}

// writeJSON encodes n as two-space-indented JSON, preserving mapping order
// (encoding/json would sort the keys of a map).
func writeJSON(w *bytes.Buffer, n *yaml.Node, depth int) error {
	indent := func(d int) {
		w.WriteByte('\n')
		w.WriteString(strings.Repeat("  ", d))
	}
	switch n.Kind {
	case yaml.AliasNode:
		return writeJSON(w, n.Alias, depth)
	case yaml.MappingNode:
		if len(n.Content) == 0 {
			w.WriteString("{}")
			return nil
		}
		w.WriteByte('{')
		for i := 0; i+1 < len(n.Content); i += 2 {
			if n.Content[i].Value == "<<" {
				return fmt.Errorf("line %d: YAML merge keys are not supported", n.Content[i].Line)
			}
			if i > 0 {
				w.WriteByte(',')
			}
			indent(depth + 1)
			writeScalar(w, n.Content[i].Value)
			w.WriteString(": ")
			if err := writeJSON(w, n.Content[i+1], depth+1); err != nil {
				return err
			}
		}
		indent(depth)
		w.WriteByte('}')
	case yaml.SequenceNode:
		if len(n.Content) == 0 {
			w.WriteString("[]")
			return nil
		}
		w.WriteByte('[')
		for i, c := range n.Content {
			if i > 0 {
				w.WriteByte(',')
			}
			indent(depth + 1)
			if err := writeJSON(w, c, depth+1); err != nil {
				return err
			}
		}
		indent(depth)
		w.WriteByte(']')
	case yaml.ScalarNode:
		if n.Tag == "!!timestamp" { // keep dates as written, not time.Time
			writeScalar(w, n.Value)
			return nil
		}
		var v any
		if err := n.Decode(&v); err != nil {
			return fmt.Errorf("line %d: %w", n.Line, err)
		}
		writeScalar(w, v)
	default:
		return fmt.Errorf("line %d: unexpected YAML node kind %d", n.Line, n.Kind)
	}
	return nil
}

func writeScalar(w *bytes.Buffer, v any) {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(v)           // scalars from YAML always encode
	w.Truncate(w.Len() - 1) // drop Encode's trailing newline
}
