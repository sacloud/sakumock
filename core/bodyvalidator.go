package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"slices"
	"strings"
)

// BodyValidationErrorWriter writes the 400 response in the service's own
// error envelope. Every service's writeError already has this signature.
type BodyValidationErrorWriter func(w http.ResponseWriter, status int, message string)

// BodyValidator validates JSON request bodies against per-route schemas
// (typically the generated bodySchemas table of a service). A nil
// *BodyValidator is valid and disables validation, so services can build
// their route table before the validator is configured.
type BodyValidator struct {
	schemas  map[string]*BodySchema
	errWrite BodyValidationErrorWriter
}

// BodyValidatorOption customizes a BodyValidator at construction.
type BodyValidatorOption func(*BodyValidator)

// NewBodyValidator creates a validator over schemas keyed by
// "METHOD /path" (the mux pattern registered in the route table).
func NewBodyValidator(schemas map[string]*BodySchema, errWrite BodyValidationErrorWriter, opts ...BodyValidatorOption) *BodyValidator {
	bv := &BodyValidator{schemas: schemas, errWrite: errWrite}
	for _, opt := range opts {
		opt(bv)
	}
	return bv
}

// WithNonEmpty overlays a MinLength-1 constraint onto the given string
// fields, addressed as dotted object-property paths per route key (e.g.
// {"POST /kms/keys": {"Key.Name"}}). It exists for the recurring spec gap
// where a field is declared required but has no minLength, while the real
// API rejects an empty string — the overlay closes the gap without touching
// the generated bodySchemas (the schemas passed in are never mutated; nodes
// along each path are copied). A field that already carries a spec minLength
// keeps it.
//
// A route key or path that does not resolve to a string field in the schema
// panics at construction: overrides are static configuration, and a spec
// update that renames a field must fail loudly instead of silently dropping
// the constraint.
func WithNonEmpty(fields map[string][]string) BodyValidatorOption {
	return func(bv *BodyValidator) {
		overlaid := maps.Clone(bv.schemas)
		for _, key := range slices.Sorted(maps.Keys(fields)) {
			if overlaid[key] == nil {
				panic(fmt.Sprintf("core.WithNonEmpty: no schema for route %q", key))
			}
			for _, path := range fields[key] {
				overlaid[key] = withMinLength(overlaid[key], key, path)
			}
		}
		bv.schemas = overlaid
	}
}

// withMinLength returns a copy of schema with MinLength 1 set on the string
// field at the dotted object-property path, copying only the nodes along the
// path so shared schema literals stay untouched.
func withMinLength(schema *BodySchema, key, path string) *BodySchema {
	root := *schema
	cur := &root
	segs := strings.Split(path, ".")
	for i, seg := range segs {
		prop := cur.Properties[seg]
		if prop == nil {
			panic(fmt.Sprintf("core.WithNonEmpty: route %q has no property %q", key, strings.Join(segs[:i+1], ".")))
		}
		cur.Properties = maps.Clone(cur.Properties)
		child := *prop
		cur.Properties[seg] = &child
		cur = &child
	}
	if cur.Type != "string" {
		panic(fmt.Sprintf("core.WithNonEmpty: route %q property %q is %q, not a string", key, path, cur.Type))
	}
	if cur.MinLength == nil {
		cur.MinLength = IntPtr(1)
	}
	return &root
}

// Middleware wraps next with request-body validation for the route registered
// as method+" "+path. The schema lookup happens once at wrap time; routes
// without a schema (e.g. /_sakumock/ helpers) get next back unchanged.
//
// Empty and non-JSON bodies pass through so the handler's own ReadJSON error
// handling stays authoritative for those; the validator only rejects bodies
// that parse but violate the schema. The body is restored before next runs.
func (bv *BodyValidator) Middleware(method, path string, next http.HandlerFunc) http.HandlerFunc {
	if bv == nil {
		return next
	}
	schema, ok := bv.schemas[method+" "+path]
	if !ok || schema == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			bv.errWrite(w, http.StatusBadRequest, "failed to read request body")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		if len(body) == 0 {
			next(w, r)
			return
		}
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			next(w, r)
			return
		}
		if msg := schema.Validate(v); msg != "" {
			bv.errWrite(w, http.StatusBadRequest, msg)
			return
		}
		next(w, r)
	}
}
