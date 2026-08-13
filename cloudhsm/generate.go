package cloudhsm

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date. The mapping file prepends the
// {zone}/api/cloud/1.1 prefix the mock serves under (see zonePrefix in
// route.go) to every spec path.
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/openapi.json -mapping validate_mapping.json -responses -out validate_gen.go
