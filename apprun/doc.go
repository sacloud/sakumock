package apprun

import _ "embed"

// Doc is this service's README, embedded so the binary can print its own
// documentation (`--docs`, `sakumock docs apprun`).
//
//go:embed README.md
var Doc string
