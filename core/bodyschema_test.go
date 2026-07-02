package core_test

import (
	"encoding/json"
	"testing"

	"github.com/sacloud/sakumock/core"
)

// decode unmarshals a JSON literal the same way BodyValidator does before
// evaluation.
func decode(t *testing.T, body string) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(body), &v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestBodySchemaValidate(t *testing.T) {
	ruleSchema := &core.BodySchema{
		Type:     "object",
		Required: []string{"metrics_storage_id", "query"},
		Properties: map[string]*core.BodySchema{
			"metrics_storage_id": {Type: "integer", Nullable: true},
			"query":              {Type: "string", MinLength: core.IntPtr(1), MaxLength: core.IntPtr(10)},
			"name":               {Type: "string", MaxLength: core.IntPtr(4)},
			"threshold":          {Type: "string", Nullable: true, MinLength: core.IntPtr(1)},
			"open":               {Type: "boolean"},
			"level":              {Type: "string", Enum: []any{"warning", "critical"}},
			"count":              {Type: "integer", Minimum: core.Float64Ptr(1), Maximum: core.Float64Ptr(10)},
			"rate":               {Type: "number", Minimum: core.Float64Ptr(0), ExclusiveMinimum: true},
			"code":               {Type: "string", Pattern: `^[0-9]{3}$`},
			"tags": {
				Type:     "array",
				Items:    &core.BodySchema{Type: "string", MinLength: core.IntPtr(1)},
				MinItems: core.IntPtr(1),
				MaxItems: core.IntPtr(3),
			},
			"settings": {
				Type:     "object",
				Required: []string{"source"},
				Properties: map[string]*core.BodySchema{
					"source": {Type: "string"},
				},
			},
		},
	}

	valid := `{"metrics_storage_id": 42, "query": "up"}`

	cases := []struct {
		name   string
		schema *core.BodySchema
		body   string
		want   string
	}{
		{"valid minimal", ruleSchema, valid, ""},
		{"nil schema is permissive", nil, `{"anything": true}`, ""},
		{"missing required", ruleSchema, `{"query": "up"}`, "metrics_storage_id is required"},
		{"required nullable accepts null", ruleSchema, `{"metrics_storage_id": null, "query": "up"}`, ""},
		{"required non-nullable rejects null", ruleSchema, `{"metrics_storage_id": 1, "query": null}`, "query must not be null"},
		{"not an object", ruleSchema, `[1, 2]`, "request body must be an object"},
		{"type mismatch string", ruleSchema, `{"metrics_storage_id": 1, "query": 5}`, "query must be a string"},
		{"type mismatch boolean", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "open": "yes"}`, "open must be a boolean"},
		{"integer rejects fraction", ruleSchema, `{"metrics_storage_id": 1.5, "query": "up"}`, "metrics_storage_id must be an integer"},
		{"minLength 1 empty", ruleSchema, `{"metrics_storage_id": 1, "query": ""}`, "query must not be empty"},
		{"maxLength", ruleSchema, `{"metrics_storage_id": 1, "query": "12345678901"}`, "query must be at most 10 characters"},
		{"maxLength counts runes", ruleSchema, `{"metrics_storage_id": 1, "query": "あいうえおかきくけこ"}`, ""},
		{"nullable string accepts null", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "threshold": null}`, ""},
		{"nullable string checks minLength", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "threshold": ""}`, "threshold must not be empty"},
		{"enum ok", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "level": "warning"}`, ""},
		{"enum violation", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "level": "fatal"}`, "level must be one of warning, critical"},
		{"minimum inclusive ok", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "count": 1}`, ""},
		{"minimum violation", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "count": 0}`, "count must be at least 1"},
		{"maximum violation", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "count": 11}`, "count must be at most 10"},
		{"exclusive minimum boundary", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "rate": 0}`, "rate must be greater than 0"},
		{"exclusive minimum ok", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "rate": 0.1}`, ""},
		{"pattern ok", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "code": "123"}`, ""},
		{"pattern violation", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "code": "12a"}`, `code must match "^[0-9]{3}$"`},
		{"array ok", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "tags": ["a", "b"]}`, ""},
		{"array minItems", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "tags": []}`, "tags must have at least 1 items"},
		{"array maxItems", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "tags": ["a","b","c","d"]}`, "tags must have at most 3 items"},
		{"array item violation", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "tags": ["a", ""]}`, "tags[1] must not be empty"},
		{"nested required", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "settings": {}}`, "settings.source is required"},
		{"nested type", ruleSchema, `{"metrics_storage_id": 1, "query": "up", "settings": {"source": 1}}`, "settings.source must be a string"},
		{"untyped schema applies matching constraints", &core.BodySchema{MaxLength: core.IntPtr(2)}, `"abc"`, "request body must be at most 2 characters"},
		{"untyped schema ignores other types", &core.BodySchema{MaxLength: core.IntPtr(2)}, `123`, ""},
		{"top-level enum", &core.BodySchema{Enum: []any{float64(1), float64(2)}}, `3`, "request body must be one of 1, 2"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.schema.Validate(decode(t, c.body))
			if got != c.want {
				t.Fatalf("Validate() = %q, want %q", got, c.want)
			}
		})
	}
}
