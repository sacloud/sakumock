package addon

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date. The mock serves the spec paths
// as-is (the addon SDK client uses SAKURA_ENDPOINTS_ADDON as the whole API
// root URL); the mapping file only lists the spec paths the mock does not
// implement yet.
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/openapi.yaml -mapping validate_mapping.json -responses -out validate_gen.go
