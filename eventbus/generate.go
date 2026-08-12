package eventbus

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date.
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/openapi.json -responses -out validate_gen.go
