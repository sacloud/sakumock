package simplenotification

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date. The mapping file skips the spec
// paths the mock does not implement.
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/openapi.yaml -mapping validate_mapping.json -out validate_gen.go
