package objectstorage

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date. The mapping file re-prefixes the
// spec's bare paths onto the mock's route families: /{site}/v2 for the site
// API (the spec models the site as a server-URL variable) and explicit route
// overrides for the /fed/v1 federation operations (per-operation servers).
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/openapi.json -mapping validate_mapping.json -responses -out validate_gen.go
