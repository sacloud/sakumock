package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderEnvFile(t *testing.T) {
	out := RenderEnvFile([]EnvVar{
		{Key: "SAKURA_ENDPOINTS_KMS", Value: "http://127.0.0.1:18081"},
		{Key: "SAKURA_ACCESS_TOKEN", Value: "dummy"},
	})

	if !strings.HasPrefix(out, "#") {
		t.Errorf("expected a comment header, got: %q", out)
	}
	for _, want := range []string{
		"SAKURA_ENDPOINTS_KMS=http://127.0.0.1:18081\n",
		"SAKURA_ACCESS_TOKEN=dummy\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output missing %q\n%s", want, out)
		}
	}
}

func TestWriteEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sakumock.env")
	vars := []EnvVar{{Key: "SAKURA_ENDPOINTS_KMS", Value: "http://127.0.0.1:18081"}}

	if err := WriteEnvFile(path, vars); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != RenderEnvFile(vars) {
		t.Errorf("file content mismatch:\n got: %q\nwant: %q", got, RenderEnvFile(vars))
	}
}
