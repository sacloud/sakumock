package secretmanager

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date. The mapping file renames the
// spec's {resource_id} path parameter to the {vault_resource_id} the mock
// serves.
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/openapi-fixed.json -mapping validate_mapping.json -out validate_gen.go
