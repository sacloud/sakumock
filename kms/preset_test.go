package kms

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestParsePresetKeys(t *testing.T) {
	rawHex := strings.Repeat("ab", 32)
	keys, err := parsePresetKeys(map[string]string{"123456789013": "my-dev-secret", "123456789012": rawHex})
	if err != nil {
		t.Fatal(err)
	}
	if len(keys) != 2 {
		t.Fatalf("got %d keys, want 2", len(keys))
	}
	if keys[0].id != "123456789012" || hex.EncodeToString(keys[0].material) != rawHex {
		t.Errorf("hex secret not used verbatim: %+v", keys[0])
	}
	sum := sha256.Sum256([]byte("my-dev-secret"))
	if keys[1].id != "123456789013" || !bytes.Equal(keys[1].material, sum[:]) {
		t.Errorf("non-hex secret not hashed: %+v", keys[1])
	}

	for _, bad := range []map[string]string{{"": "secret"}, {"123": ""}, {"abc": "secret"}} {
		if _, err := parsePresetKeys(bad); err == nil {
			t.Errorf("parsePresetKeys(%v) succeeded, want error", bad)
		}
	}
}
