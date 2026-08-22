package kms

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
)

// presetKey is a key pre-created at startup from a --key ID=SECRET spec, so the
// same ID and key material survive a restart.
type presetKey struct {
	id       string
	material []byte
}

// parsePresetKeys validates the --key ID=SECRET map and derives each key's
// material, in ID order. ID is the numeric resource ID the key is served
// under. SECRET is either 64 hex characters, used verbatim as the 32-byte
// AES-256 key, or any other non-empty string, whose SHA-256 digest becomes
// the key material.
func parsePresetKeys(specs map[string]string) ([]presetKey, error) {
	keys := make([]presetKey, 0, len(specs))
	for id, secret := range specs {
		if id == "" || secret == "" {
			return nil, fmt.Errorf("invalid --key %q=%q: expected ID=SECRET", id, secret)
		}
		for _, c := range id {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("invalid --key %q: ID must be numeric", id)
			}
		}
		keys = append(keys, presetKey{id: id, material: presetMaterial(secret)})
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].id < keys[j].id })
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
