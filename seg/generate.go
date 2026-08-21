package seg

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date. No -mapping is needed: every
// route is served at its bare spec path (see route.go).
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/openapi.json -responses -out validate_gen.go
