package sakumock

import "embed"

// README is the repository README, embedded so the unified binary can print
// the suite-level documentation (`sakumock docs readme`).
//
//go:embed README.md
var README string

// Changelog is the repository CHANGELOG (`sakumock docs changelog`).
//
//go:embed CHANGELOG.md
var Changelog string

// ComposeExample is the docker compose example from examples/
// (`sakumock docs compose`).
//
//go:embed examples/compose.yaml
var ComposeExample string

// TerraformExamples holds the Terraform configurations of the end-to-end test
// (test/terraform/*.tf and the files they reference), which the sacloud/sakura
// provider applies against `sakumock all`; every resource in them is known to
// work on the mock (`sakumock docs terraform`).
//
//go:embed test/terraform/*.tf test/terraform/*.yaml
var TerraformExamples embed.FS
