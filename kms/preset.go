package kms

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// presetKey is a key pre-created at startup from a --key ID=SECRET spec, so the
// same ID and key material survive a restart.
type presetKey struct {
	id       string
	material []byte
}

// parsePresetKeys parses --key specs of the form ID=SECRET. ID is the numeric
// resource ID the key is served under. SECRET is either 64 hex characters,
// used verbatim as the 32-byte AES-256 key, or any other non-empty string,
// whose SHA-256 digest becomes the key material.
func parsePresetKeys(specs []string) ([]presetKey, error) {
	seen := make(map[string]bool, len(specs))
	keys := make([]presetKey, 0, len(specs))
	for _, spec := range specs {
		id, secret, ok := strings.Cut(spec, "=")
		if !ok || id == "" || secret == "" {
			return nil, fmt.Errorf("invalid --key %q: expected ID=SECRET", spec)
		}
		for _, c := range id {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("invalid --key %q: ID must be numeric", spec)
			}
		}
		if seen[id] {
			return nil, fmt.Errorf("invalid --key %q: duplicate ID", spec)
		}
		seen[id] = true
		keys = append(keys, presetKey{id: id, material: presetMaterial(secret)})
	}
	return keys, nil
}

// presetMaterial derives the 32-byte AES key from a --key secret.
func presetMaterial(secret string) []byte {
	if len(secret) == 64 {
		if b, err := hex.DecodeString(secret); err == nil {
			return b
		}
	}
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
