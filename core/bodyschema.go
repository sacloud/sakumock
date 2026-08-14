package core

import (
	"fmt"
	"math"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"
)

// BodySchema describes JSON request-body constraints derived from an OpenAPI
// schema. Instances are typically generated from a service's openapi/ spec by
// internal/genvalidate and evaluated by BodyValidator before the handler runs.
// A nil *BodySchema is permissive (everything validates).
type BodySchema struct {
	// Type is one of "object", "array", "string", "integer", "number",
	// "boolean". Empty means any type.
	Type     string
	Nullable bool

	Required   []string
	Properties map[string]*BodySchema

	Items    *BodySchema
	MinItems *int
	MaxItems *int

	// Lengths are counted in runes.
	MinLength *int
	MaxLength *int
	Pattern   string // RE2; compiled lazily and cached

	Minimum          *float64
	Maximum          *float64
	ExclusiveMinimum bool
	ExclusiveMaximum bool

	Enum []any
}

// IntPtr returns a pointer to v, for building schema literals.
func IntPtr(v int) *int { return &v }

// Float64Ptr returns a pointer to v, for building schema literals.
func Float64Ptr(v float64) *float64 { return &v }

// patternCache keeps compiled patterns out of the schemas themselves, which
// are shared package-level literals and must not be mutated.
var patternCache sync.Map // string -> *regexp.Regexp

// Validate checks a decoded JSON value (the result of json.Unmarshal into
// any) against the schema. It returns "" when the value is valid, otherwise a
// human-readable message such as "query must be at most 4096 characters" or
// "settings.name is required", suitable for the service's 400 error body.
func (s *BodySchema) Validate(v any) string {
	return s.validate("", v)
}

func fieldLabel(path string) string {
	if path == "" {
		return "request body"
	}
	return path
}

func joinPath(path, key string) string {
	if path == "" {
		return key
	}
	return path + "." + key
}

func (s *BodySchema) validate(path string, v any) string {
	if s == nil {
		return ""
	}
	if v == nil {
		if s.Nullable {
			return ""
		}
		return fieldLabel(path) + " must not be null"
	}
	if len(s.Enum) > 0 && !enumContains(s.Enum, v) {
		return fmt.Sprintf("%s must be one of %s", fieldLabel(path), enumList(s.Enum))
	}
	switch s.Type {
	case "object":
		obj, ok := v.(map[string]any)
		if !ok {
			return fieldLabel(path) + " must be an object"
		}
		return s.validateObject(path, obj)
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return fieldLabel(path) + " must be an array"
		}
		return s.validateArray(path, arr)
	case "string":
		str, ok := v.(string)
		if !ok {
			return fieldLabel(path) + " must be a string"
		}
		return s.validateString(path, str)
	case "integer", "number":
		n, ok := v.(float64)
		if !ok {
			return fieldLabel(path) + " must be a number"
		}
		if s.Type == "integer" && n != math.Trunc(n) {
			return fieldLabel(path) + " must be an integer"
		}
		return s.validateNumber(path, n)
	case "boolean":
		if _, ok := v.(bool); !ok {
			return fieldLabel(path) + " must be a boolean"
		}
		return ""
	case "":
		switch tv := v.(type) {
		case map[string]any:
			return s.validateObject(path, tv)
		case []any:
			return s.validateArray(path, tv)
		case string:
			return s.validateString(path, tv)
		case float64:
			return s.validateNumber(path, tv)
		}
		return ""
	default:
		return ""
	}
}

func (s *BodySchema) validateObject(path string, obj map[string]any) string {
	for _, key := range s.Required {
		if _, ok := obj[key]; !ok {
			return joinPath(path, key) + " is required"
		}
	}
	// Iterate properties deterministically so the first violation reported is
	// stable across runs.
	keys := make([]string, 0, len(s.Properties))
	for key := range s.Properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		val, ok := obj[key]
		if !ok {
			continue
		}
		if msg := s.Properties[key].validate(joinPath(path, key), val); msg != "" {
			return msg
		}
	}
	return ""
}

func (s *BodySchema) validateArray(path string, arr []any) string {
	if s.MinItems != nil && len(arr) < *s.MinItems {
		return fmt.Sprintf("%s must have at least %d items", fieldLabel(path), *s.MinItems)
	}
	if s.MaxItems != nil && len(arr) > *s.MaxItems {
		return fmt.Sprintf("%s must have at most %d items", fieldLabel(path), *s.MaxItems)
	}
	for i, item := range arr {
		if msg := s.Items.validate(fmt.Sprintf("%s[%d]", fieldLabel(path), i), item); msg != "" {
			return msg
		}
	}
	return ""
}

func (s *BodySchema) validateString(path, str string) string {
	n := utf8.RuneCountInString(str)
	if s.MinLength != nil && n < *s.MinLength {
		if *s.MinLength == 1 {
			return fieldLabel(path) + " must not be empty"
		}
		return fmt.Sprintf("%s must be at least %d characters", fieldLabel(path), *s.MinLength)
	}
	if s.MaxLength != nil && n > *s.MaxLength {
		return fmt.Sprintf("%s must be at most %d characters", fieldLabel(path), *s.MaxLength)
	}
	if s.Pattern != "" {
		re, err := compilePattern(s.Pattern)
		// An uncompilable pattern is skipped (the generator already filters
		// these out; stay permissive rather than failing requests).
		if err == nil && !re.MatchString(str) {
			return fmt.Sprintf("%s must match %q", fieldLabel(path), s.Pattern)
		}
	}
	return ""
}

func (s *BodySchema) validateNumber(path string, n float64) string {
	if s.Minimum != nil {
		if s.ExclusiveMinimum && n <= *s.Minimum {
			return fmt.Sprintf("%s must be greater than %s", fieldLabel(path), formatNumber(*s.Minimum))
		}
		if !s.ExclusiveMinimum && n < *s.Minimum {
			return fmt.Sprintf("%s must be at least %s", fieldLabel(path), formatNumber(*s.Minimum))
		}
	}
	if s.Maximum != nil {
		if s.ExclusiveMaximum && n >= *s.Maximum {
			return fmt.Sprintf("%s must be less than %s", fieldLabel(path), formatNumber(*s.Maximum))
		}
		if !s.ExclusiveMaximum && n > *s.Maximum {
			return fmt.Sprintf("%s must be at most %s", fieldLabel(path), formatNumber(*s.Maximum))
		}
	}
	return ""
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	if cached, ok := patternCache.Load(pattern); ok {
		return cached.(*regexp.Regexp), nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	patternCache.Store(pattern, re)
	return re, nil
}

func enumContains(enum []any, v any) bool {
	for _, e := range enum {
		if enumEqual(e, v) {
			return true
		}
	}
	return false
}

// enumEqual compares an enum literal with a decoded JSON value. Numbers are
// compared numerically because generated literals are float64 while decoded
// values are always float64 too; other types compare directly.
func enumEqual(a, b any) bool {
	if af, ok := toFloat(a); ok {
		bf, ok := toFloat(b)
		return ok && af == bf
	}
	return a == b
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

func enumList(enum []any) string {
	parts := make([]string, len(enum))
	for i, e := range enum {
		if s, ok := e.(string); ok {
			parts[i] = s
		} else {
			parts[i] = fmt.Sprintf("%v", e)
		}
	}
	return strings.Join(parts, ", ")
}

func formatNumber(f float64) string {
	if f == math.Trunc(f) && math.Abs(f) < 1e15 {
		return fmt.Sprintf("%d", int64(f))
	}
	return fmt.Sprintf("%v", f)
}
