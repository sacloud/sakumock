// Package terraform contains an end-to-end test that drives Terraform with the
// sacloud/sakura provider against the unified `sakumock all` binary, verifying
// that a real provider can create/read/destroy resources for every mocked
// service. The test is behind the "terraform" build tag and is skipped when the
// terraform binary is absent:
//
//	go test -tags terraform ./test/terraform/
//
// Maintenance notes:
//
//   - To see which resources the provider exposes for a service, ask the
//     provider rather than grepping its binary (terraform-plugin-framework
//     composes type names at runtime, so "sakura_<x>" is never a literal):
//
//     terraform -chdir=test/terraform init
//     terraform -chdir=test/terraform providers schema -json | jq -r '.provider_schemas[].resource_schemas | keys[]'
//
//     Append ["<resource>"].block.attributes to inspect one resource's arguments.
//
//   - A failed manual run leaves terraform.tfstate holding the previous mock
//     process's resource IDs, so the next run fails during refresh with
//     unrelated 404s. Delete terraform.tfstate* (gitignored) before re-running.
package terraform
