package simplemq

// Regenerate validate_gen.go after updating openapi/ (make openapi). CI
// verifies the generated file is up to date. Both specs feed one table: the
// control plane (queue.yaml) and the data plane (message.yaml) share the
// route table on different path families.
//go:generate go run github.com/sacloud/sakumock/internal/genvalidate -spec openapi/queue.yaml -spec openapi/message.yaml -responses -out validate_gen.go
