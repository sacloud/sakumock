package kms

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// maxPresetVersion bounds the version a --key spec may declare, so a typo does
// not derive millions of key versions at startup.
const maxPresetVersion = 1000

// presetKey is a key pre-created at startup from a --key ID=SECRET[@N] spec,
// so the same ID and key material survive a restart.
type presetKey struct {
	id       string
	material []byte // version 1 material
	version  int    // latest version; versions 2..N are derived by chaining
}

// parsePresetKeys validates the --key ID=SECRET[@N] map and derives each
// key's material, in ID order. ID is the numeric resource ID the key is served
// under, at most 12 digits like a real SAKURA Cloud ID. SECRET is either 64
// hex characters, used verbatim as the 32-byte AES-256 key, or any other
// non-empty string, whose SHA-256 digest becomes the key material. The
// optional @N (N >= 1, default 1) declares the key as already rotated to
// version N; versions 2..N are derived from version 1 by nextKeyMaterial.
func parsePresetKeys(specs map[string]string) ([]presetKey, error) {
	keys := make([]presetKey, 0, len(specs))
	for id, spec := range specs {
		secret, versionStr, hasVersion := strings.Cut(spec, "@")
		if id == "" || secret == "" {
			return nil, fmt.Errorf("invalid --key %q=%q: expected ID=SECRET[@N]", id, spec)
		}
		if len(id) > 12 {
			return nil, fmt.Errorf("invalid --key %q: ID must be at most 12 digits", id)
		}
		for _, c := range id {
			if c < '0' || c > '9' {
				return nil, fmt.Errorf("invalid --key %q: ID must be numeric", id)
			}
		}
		version := 1
		if hasVersion {
			n, err := strconv.Atoi(versionStr)
			if err != nil || n < 1 || n > maxPresetVersion {
				return nil, fmt.Errorf("invalid --key %q=%q: version must be an integer between 1 and %d", id, spec, maxPresetVersion)
			}
			version = n
		}
		keys = append(keys, presetKey{id: id, material: presetMaterial(secret), version: version})
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

// nextKeyMaterial derives the material of the next key version from the
// current one: version N+1 is SHA-256 of version N's 32 raw bytes. Rotation
// is therefore deterministic given version 1, which lets a --key ID=SECRET@N
// spec reproduce every version of a rotated key after a restart.
func nextKeyMaterial(material []byte) []byte {
	sum := sha256.Sum256(material)
	return sum[:]
}
