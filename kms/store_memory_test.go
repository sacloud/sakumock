package kms

import (
	"encoding/base64"
	"encoding/binary"
	"strings"
	"testing"
)

func TestMemoryStoreDecryptSelectsVersion(t *testing.T) {
	s := NewMemoryStore(nil)
	const id = "123456789012"
	if err := s.Preset(id, presetMaterial("secret"), 1); err != nil {
		t.Fatal(err)
	}
	c1, err := s.Encrypt(id, []byte("v1"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Rotate(id); err != nil {
		t.Fatal(err)
	}
	c2, err := s.Encrypt(id, []byte("v2"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		cipher  string
		version uint32
		want    string
	}{{c1, 1, "v1"}, {c2, 2, "v2"}} {
		raw, err := base64.StdEncoding.DecodeString(tc.cipher)
		if err != nil {
			t.Fatal(err)
		}
		if v := binary.BigEndian.Uint32(raw); v != tc.version {
			t.Errorf("ciphertext version prefix = %d, want %d", v, tc.version)
		}
		got, err := s.Decrypt(id, tc.cipher)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Errorf("decrypt = %q, want %q", got, tc.want)
		}
	}

	// A ciphertext from a version the key does not have is rejected by its
	// prefix, without trying any material.
	fresh := NewMemoryStore(nil)
	if err := fresh.Preset(id, presetMaterial("secret"), 1); err != nil {
		t.Fatal(err)
	}
	_, err = fresh.Decrypt(id, c2)
	if err == nil || !strings.Contains(err.Error(), "no version 2") {
		t.Errorf("decrypt with unknown version: err = %v, want version mismatch", err)
	}
	// But it succeeds once the key is preset at that version.
	rotated := NewMemoryStore(nil)
	if err := rotated.Preset(id, presetMaterial("secret"), 2); err != nil {
		t.Fatal(err)
	}
	if got, err := rotated.Decrypt(id, c2); err != nil || string(got) != "v2" {
		t.Errorf("decrypt at preset version 2 = %q, %v", got, err)
	}

	// A tampered version prefix fails authentication even when that version
	// exists: the prefix is covered by the GCM tag.
	raw, _ := base64.StdEncoding.DecodeString(c1)
	binary.BigEndian.PutUint32(raw, 2)
	if _, err := s.Decrypt(id, base64.StdEncoding.EncodeToString(raw)); err == nil {
		t.Error("decrypt with a tampered version prefix succeeded, want error")
	}
	if err := s.Preset("123456789013", presetMaterial("x"), 0); err == nil {
		t.Error("Preset with version 0 succeeded, want error")
	}

	for _, bad := range []string{"", base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 1}), base64.StdEncoding.EncodeToString([]byte{0, 0, 0, 1, 1, 2, 3})} {
		if _, err := s.Decrypt(id, bad); err == nil {
			t.Errorf("Decrypt(%q) succeeded, want error", bad)
		}
	}
}
