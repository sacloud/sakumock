// Package terraform contains an end-to-end test that drives Terraform with the
// sacloud/sakura provider against the unified `sakumock all` binary, verifying
// that a real provider can create/read/destroy resources for every mocked
// service. The test is behind the "terraform" build tag and is skipped when the
// terraform binary is absent:
//
//	go test -tags terraform ./test/terraform/
package terraform
