package main

import (
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sacloud/sakumock/core"
)

// operation is one generated map entry: a route key, its compiled schema, and
// the notes (skipped constructs) emitted as comments above the entry.
type operation struct {
	key    string
	schema *core.BodySchema
	notes  []string
}

// responseOp is one generated responseSchemas entry: a route key and the
// per-status response schemas the spec declares for it. A nil schema means
// the status is declared but its body carries no checkable constraints. A nil
// statuses map means the whole route was degraded to permissive (default
// response) and only notes are emitted.
type responseOp struct {
	key      string
	statuses map[int]*core.BodySchema
	notes    []string
}

// httpMethods lists the OpenAPI operation keys that can carry a request body.
var httpMethods = []string{"post", "put", "patch", "delete", "get"}

// loadSpec decodes a JSON or YAML OpenAPI document into a generic map.
// JSON is a subset of YAML, so yaml.v3 handles both.
func loadSpec(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return doc, nil
}

// collectOperations walks one spec document and compiles the request-body
// schema of every operation that declares an application/json body.
func collectOperations(doc map[string]any, mapping *Mapping, warn io.Writer) ([]operation, error) {
	paths, _ := doc["paths"].(map[string]any)
	var ops []operation
	for path, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			rb, ok := op["requestBody"].(map[string]any)
			if !ok {
				continue
			}
			c := &compiler{doc: doc, warn: warn}
			rb, err := c.resolveRef(rb)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", method, path, err)
			}
			key, skip := routeKey(strings.ToUpper(method), path, mapping)
			if skip {
				continue
			}
			content, _ := rb["content"].(map[string]any)
			js, ok := content["application/json"].(map[string]any)
			if !ok {
				// Non-JSON bodies (e.g. form-urlencoded) are out of scope.
				ops = append(ops, operation{key: key, notes: []string{
					fmt.Sprintf("%s %s has a non-JSON request body; skipped", strings.ToUpper(method), path),
				}})
				continue
			}
			rawSchema, ok := js["schema"]
			if !ok {
				continue
			}
			loc := fmt.Sprintf("%s %s", strings.ToUpper(method), path)
			schema := c.compile(rawSchema, loc, nil)
			if schema == nil && len(c.notes) == 0 {
				continue
			}
			ops = append(ops, operation{key: key, schema: schema, notes: c.notes})
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].key < ops[j].key })
	return ops, nil
}

// collectResponses walks one spec document and compiles the response-body
// schema of every status each operation declares. Statuses without an
// application/json body (e.g. 204, or non-JSON content) are recorded with a
// nil schema so status membership can still be checked.
func collectResponses(doc map[string]any, mapping *Mapping, warn io.Writer) ([]responseOp, error) {
	paths, _ := doc["paths"].(map[string]any)
	var ops []responseOp
	for path, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok {
			continue
		}
		for _, method := range httpMethods {
			op, ok := item[method].(map[string]any)
			if !ok {
				continue
			}
			key, skip := routeKey(strings.ToUpper(method), path, mapping)
			if skip {
				continue
			}
			resps, ok := op["responses"].(map[string]any)
			if !ok {
				continue
			}
			c := &compiler{doc: doc, warn: warn, resp: true}
			statuses := map[int]*core.BodySchema{}
			wildcard := false
			for _, code := range slices.Sorted(maps.Keys(resps)) {
				// "default" and range patterns like "4XX" accept statuses the
				// int-keyed table cannot express, so the route degrades to
				// permissive below.
				if code == "default" || strings.HasSuffix(code, "XX") {
					wildcard = true
					continue
				}
				status, err := strconv.Atoi(code)
				if err != nil {
					c.note("response status %q at %s %s is not a status code; skipped", code, strings.ToUpper(method), path)
					continue
				}
				resp, ok := resps[code].(map[string]any)
				if !ok {
					statuses[status] = nil
					continue
				}
				resp, err = c.resolveRef(resp)
				if err != nil {
					return nil, fmt.Errorf("%s %s response %d: %w", method, path, status, err)
				}
				content, _ := resp["content"].(map[string]any)
				js, ok := content["application/json"].(map[string]any)
				if !ok {
					// No JSON body declared for this status (204, or a
					// non-JSON content type): membership only.
					statuses[status] = nil
					continue
				}
				rawSchema, ok := js["schema"]
				if !ok {
					statuses[status] = nil
					continue
				}
				loc := fmt.Sprintf("%s %s response %d", strings.ToUpper(method), path, status)
				statuses[status] = c.compile(rawSchema, loc, nil)
			}
			if wildcard {
				// A default/range response accepts statuses the table cannot
				// enumerate, so membership cannot be enforced; leave the whole
				// route permissive.
				c.note("%s %s declares a default or range response; response validation left permissive", strings.ToUpper(method), path)
				ops = append(ops, responseOp{key: key, notes: c.notes})
				continue
			}
			if len(statuses) == 0 && len(c.notes) == 0 {
				continue
			}
			ops = append(ops, responseOp{key: key, statuses: statuses, notes: c.notes})
		}
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].key < ops[j].key })
	return ops, nil
}

// compiler compiles one operation's schema, accumulating notes about
// constructs that were degraded to permissive.
type compiler struct {
	doc  map[string]any
	warn io.Writer
	// resp switches required-field handling to response semantics: writeOnly
	// properties (request-only per the OpenAPI spec) are skipped instead of
	// readOnly ones.
	resp  bool
	notes []string
}

func (c *compiler) note(format string, args ...any) {
	c.notes = append(c.notes, fmt.Sprintf(format, args...))
}

// resolveRef follows a local $ref one level; non-ref maps are returned as-is.
func (c *compiler) resolveRef(m map[string]any) (map[string]any, error) {
	ref, ok := m["$ref"].(string)
	if !ok {
		return m, nil
	}
	target, err := c.lookup(ref)
	if err != nil {
		return nil, err
	}
	return target, nil
}

func (c *compiler) lookup(ref string) (map[string]any, error) {
	if !strings.HasPrefix(ref, "#/") {
		return nil, fmt.Errorf("non-local $ref %q is not supported", ref)
	}
	cur := any(c.doc)
	for seg := range strings.SplitSeq(strings.TrimPrefix(ref, "#/"), "/") {
		seg = strings.ReplaceAll(strings.ReplaceAll(seg, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("cannot resolve $ref %q", ref)
		}
		cur, ok = m[seg]
		if !ok {
			return nil, fmt.Errorf("cannot resolve $ref %q", ref)
		}
	}
	m, ok := cur.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("$ref %q does not point to a schema object", ref)
	}
	return m, nil
}

// compile turns an OpenAPI schema (3.0 or 3.1) into a *core.BodySchema.
// A nil result means "permissive": either the schema uses a construct we
// degrade (oneOf/anyOf/not, ref cycles) or it carries no checkable
// constraints. seen guards $ref cycles.
func (c *compiler) compile(raw any, loc string, seen []string) *core.BodySchema {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	if ref, ok := m["$ref"].(string); ok {
		if slices.Contains(seen, ref) {
			c.note("recursive $ref %s at %s left permissive", ref, loc)
			return nil
		}
		target, err := c.lookup(ref)
		if err != nil {
			c.note("%v at %s left permissive", err, loc)
			return nil
		}
		return c.compile(target, loc, append(seen, ref))
	}
	for _, kw := range []string{"oneOf", "anyOf", "not"} {
		if _, ok := m[kw]; ok {
			c.note("%s at %s left permissive", kw, loc)
			return nil
		}
	}
	if allOf, ok := m["allOf"].([]any); ok {
		return c.compileAllOf(m, allOf, loc, seen)
	}

	s := &core.BodySchema{}
	types, nullable := schemaTypes(m)
	s.Nullable = nullable
	switch len(types) {
	case 0:
	case 1:
		s.Type = types[0]
	default:
		// A 3.1 multi-type union (e.g. ["string", "integer"]) has no single
		// core type; leave Type empty so no type is rejected. The keyword
		// constraints below still compile and apply per dynamic type.
		c.note("multiple types %v at %s; type check left permissive", types, loc)
	}
	if b, ok := m["nullable"].(bool); ok && b {
		s.Nullable = true
	}

	// An omitted type means the keywords decide what applies (JSON Schema
	// allows constraints without a type), so each keyword group compiles both
	// for its own type and for an untyped schema.
	if s.Type == "object" || s.Type == "" {
		if props, ok := m["properties"].(map[string]any); ok {
			// Compile in sorted order so notes are emitted deterministically.
			for _, name := range slices.Sorted(maps.Keys(props)) {
				prop := c.compile(props[name], loc+"."+name, seen)
				if prop == nil {
					continue
				}
				if s.Properties == nil {
					s.Properties = map[string]*core.BodySchema{}
				}
				s.Properties[name] = prop
			}
		}
		if req, ok := m["required"].([]any); ok {
			for _, r := range req {
				name, ok := r.(string)
				if !ok {
					continue
				}
				// readOnly properties are response-only; per the OpenAPI spec
				// their required-ness does not apply to request bodies.
				// writeOnly is the mirror for response schemas.
				skipFlag := "readOnly"
				if c.resp {
					skipFlag = "writeOnly"
				}
				if props, ok := m["properties"].(map[string]any); ok {
					// A required name with no property definition alongside it
					// is a spec inconsistency (ogen-generated SDK types drop
					// it too); enforcing it would reject bodies the SDK
					// accepts. Schemas without a properties map keep their
					// required list — that is the valid required-only form.
					if _, defined := props[name]; !defined {
						c.note("required property %q at %s is not defined in properties; requirement skipped", name, loc)
						continue
					}
					if p, ok := props[name].(map[string]any); ok {
						if ro, ok := p[skipFlag].(bool); ok && ro {
							continue
						}
					}
				}
				s.Required = append(s.Required, name)
			}
			sort.Strings(s.Required)
		}
	}
	if s.Type == "array" || s.Type == "" {
		if items, ok := m["items"]; ok {
			s.Items = c.compile(items, loc+"[]", seen)
		}
		s.MinItems = intVal(m["minItems"])
		s.MaxItems = intVal(m["maxItems"])
	}
	if s.Type == "string" || s.Type == "" {
		s.MinLength = intVal(m["minLength"])
		s.MaxLength = intVal(m["maxLength"])
		if p, ok := m["pattern"].(string); ok && p != "" {
			if _, err := regexp.Compile(p); err != nil {
				c.note("pattern %q at %s is not RE2-compatible; skipped", p, loc)
			} else {
				s.Pattern = p
			}
		}
	}
	if s.Type == "integer" || s.Type == "number" || s.Type == "" {
		s.Minimum = floatVal(m["minimum"])
		s.Maximum = floatVal(m["maximum"])
		// 3.0 uses boolean exclusiveMinimum/Maximum flags alongside
		// minimum/maximum; 3.1 uses standalone numeric keywords.
		switch ex := m["exclusiveMinimum"].(type) {
		case bool:
			s.ExclusiveMinimum = ex
		default:
			if f := floatVal(ex); f != nil {
				s.Minimum = f
				s.ExclusiveMinimum = true
			}
		}
		switch ex := m["exclusiveMaximum"].(type) {
		case bool:
			s.ExclusiveMaximum = ex
		default:
			if f := floatVal(ex); f != nil {
				s.Maximum = f
				s.ExclusiveMaximum = true
			}
		}
	}

	if enum, ok := m["enum"].([]any); ok {
		for _, e := range enum {
			if e == nil {
				s.Nullable = true
				continue
			}
			s.Enum = append(s.Enum, normalizeEnumValue(e))
		}
	}

	if emptySchema(s) {
		return nil
	}
	return s
}

// compileAllOf merges every allOf part plus the sibling keywords of the
// enclosing schema (the common `{"allOf":[{"$ref":...}], "nullable": true}`
// pattern) into one schema.
func (c *compiler) compileAllOf(m map[string]any, allOf []any, loc string, seen []string) *core.BodySchema {
	parts := make([]*core.BodySchema, 0, len(allOf)+1)
	for i, part := range allOf {
		if p := c.compile(part, fmt.Sprintf("%s.allOf[%d]", loc, i), seen); p != nil {
			parts = append(parts, p)
		}
	}
	siblings := make(map[string]any, len(m))
	for k, v := range m {
		if k != "allOf" {
			siblings[k] = v
		}
	}
	if len(siblings) > 0 {
		if p := c.compile(siblings, loc, seen); p != nil {
			parts = append(parts, p)
		} else if b, ok := siblings["nullable"].(bool); ok && b {
			parts = append(parts, &core.BodySchema{Nullable: true})
		}
	}
	if len(parts) == 0 {
		return nil
	}
	merged := parts[0]
	for _, p := range parts[1:] {
		merged = c.merge(merged, p, loc)
	}
	if emptySchema(merged) {
		return nil
	}
	return merged
}

// merge combines two allOf parts. Bounds take the most restrictive value;
// nullable is ORed (the DRF `allOf + nullable: true` idiom means nullable).
func (c *compiler) merge(a, b *core.BodySchema, loc string) *core.BodySchema {
	out := *a
	if out.Type == "" {
		out.Type = b.Type
	} else if b.Type != "" && b.Type != out.Type {
		fmt.Fprintf(c.warn, "genvalidate: conflicting types %q vs %q in allOf at %s; keeping %q\n", out.Type, b.Type, loc, out.Type)
	}
	out.Nullable = out.Nullable || b.Nullable
	out.Required = append(out.Required, b.Required...)
	sort.Strings(out.Required)
	out.Required = dedupe(out.Required)
	if b.Properties != nil {
		if out.Properties == nil {
			out.Properties = map[string]*core.BodySchema{}
		}
		for _, k := range slices.Sorted(maps.Keys(b.Properties)) {
			if existing, ok := out.Properties[k]; ok {
				out.Properties[k] = c.merge(existing, b.Properties[k], loc+"."+k)
			} else {
				out.Properties[k] = b.Properties[k]
			}
		}
	}
	switch {
	case out.Items == nil:
		out.Items = b.Items
	case b.Items != nil:
		out.Items = c.merge(out.Items, b.Items, loc+".items")
	}
	out.MinItems = maxInt(out.MinItems, b.MinItems)
	out.MaxItems = minInt(out.MaxItems, b.MaxItems)
	out.MinLength = maxInt(out.MinLength, b.MinLength)
	out.MaxLength = minInt(out.MaxLength, b.MaxLength)
	switch {
	case out.Pattern == "":
		out.Pattern = b.Pattern
	case b.Pattern != "" && b.Pattern != out.Pattern:
		// allOf requires both patterns to hold, but BodySchema holds one;
		// enforcing either alone could reject spec-valid values' complement,
		// so drop the check rather than pick a side.
		c.note("conflicting patterns %q and %q in allOf at %s; pattern left unenforced", out.Pattern, b.Pattern, loc)
		out.Pattern = ""
	}
	out.Minimum = maxFloat(out.Minimum, b.Minimum)
	out.Maximum = minFloat(out.Maximum, b.Maximum)
	out.ExclusiveMinimum = out.ExclusiveMinimum || b.ExclusiveMinimum
	out.ExclusiveMaximum = out.ExclusiveMaximum || b.ExclusiveMaximum
	switch {
	case out.Enum == nil:
		out.Enum = b.Enum
	case b.Enum != nil:
		// allOf semantics: the value must satisfy both enums, so intersect.
		out.Enum = intersectEnums(out.Enum, b.Enum)
		if len(out.Enum) == 0 {
			c.note("enums in allOf at %s have an empty intersection; enum left unenforced", loc)
		}
	}
	return &out
}

// intersectEnums keeps the values of a that also appear in b, preserving a's
// order. Values are the generator-normalized literals (string/float64/bool),
// so direct equality is the right comparison.
func intersectEnums(a, b []any) []any {
	var out []any
	for _, v := range a {
		if slices.Contains(b, v) {
			out = append(out, v)
		}
	}
	return out
}

// schemaTypes normalizes the type keyword: 3.0 `type: "string"` yields one
// type; the 3.1 array form `type: ["string", "null"]` yields the non-null
// types plus the nullable flag.
func schemaTypes(m map[string]any) ([]string, bool) {
	switch t := m["type"].(type) {
	case string:
		return []string{t}, false
	case []any:
		var types []string
		nullable := false
		for _, e := range t {
			s, ok := e.(string)
			if !ok {
				continue
			}
			if s == "null" {
				nullable = true
			} else {
				types = append(types, s)
			}
		}
		return types, nullable
	}
	return nil, false
}

// emptySchema reports whether s carries no checkable constraint at all.
func emptySchema(s *core.BodySchema) bool {
	return s.Type == "" && !s.Nullable &&
		len(s.Required) == 0 && len(s.Properties) == 0 &&
		s.Items == nil && s.MinItems == nil && s.MaxItems == nil &&
		s.MinLength == nil && s.MaxLength == nil && s.Pattern == "" &&
		s.Minimum == nil && s.Maximum == nil && len(s.Enum) == 0
}

func normalizeEnumValue(e any) any {
	switch v := e.(type) {
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case float64, string, bool:
		return v
	}
	return e
}

func intVal(v any) *int {
	switch n := v.(type) {
	case int:
		return core.IntPtr(n)
	case int64:
		return core.IntPtr(int(n))
	case float64:
		return core.IntPtr(int(n))
	}
	return nil
}

func floatVal(v any) *float64 {
	switch n := v.(type) {
	case int:
		return core.Float64Ptr(float64(n))
	case int64:
		return core.Float64Ptr(float64(n))
	case float64:
		return core.Float64Ptr(n)
	}
	return nil
}

func dedupe(in []string) []string {
	out := in[:0]
	for i, s := range in {
		if i == 0 || in[i-1] != s {
			out = append(out, s)
		}
	}
	return out
}

func maxInt(a, b *int) *int {
	if a == nil {
		return b
	}
	if b != nil && *b > *a {
		return b
	}
	return a
}

func minInt(a, b *int) *int {
	if a == nil {
		return b
	}
	if b != nil && *b < *a {
		return b
	}
	return a
}

func maxFloat(a, b *float64) *float64 {
	if a == nil {
		return b
	}
	if b != nil && *b > *a {
		return b
	}
	return a
}

func minFloat(a, b *float64) *float64 {
	if a == nil {
		return b
	}
	if b != nil && *b < *a {
		return b
	}
	return a
}
