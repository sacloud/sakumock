package cloudhsm

import _ "embed"

// Doc is this service's README, embedded so the binary can print its own
// documentation (`--docs`, `sakumock docs cloudhsm`).
//
//go:embed README.md
var Doc string
