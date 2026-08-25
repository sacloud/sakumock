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
	if keys[0].version != 1 || keys[1].version != 1 {
		t.Errorf("default version != 1: %+v", keys)
	}

	keys, err = parsePresetKeys(map[string]string{"1": "s@3", "2": rawHex + "@1"})
	if err != nil {
		t.Fatal(err)
	}
	if keys[0].version != 3 || keys[1].version != 1 {
		t.Errorf("@N not parsed: %+v", keys)
	}
	if !bytes.Equal(keys[0].material, presetMaterial("s")) || hex.EncodeToString(keys[1].material) != rawHex {
		t.Errorf("material with @N suffix: %+v", keys)
	}

	for _, bad := range []map[string]string{{"": "secret"}, {"123": ""}, {"abc": "secret"}, {"1234567890123": "secret"},
		{"1": "s@"}, {"1": "s@0"}, {"1": "s@-1"}, {"1": "s@x"}, {"1": "s@1001"}, {"1": "@2"}} {
		if _, err := parsePresetKeys(bad); err == nil {
			t.Errorf("parsePresetKeys(%v) succeeded, want error", bad)
		}
	}
}

func TestNextKeyMaterial(t *testing.T) {
	v1 := presetMaterial("secret")
	v2 := nextKeyMaterial(v1)
	want := sha256.Sum256(v1)
	if !bytes.Equal(v2, want[:]) {
		t.Errorf("nextKeyMaterial = %x, want sha256 of previous version", v2)
	}
	if bytes.Equal(v2, v1) {
		t.Error("rotated material equals previous version")
	}
}
