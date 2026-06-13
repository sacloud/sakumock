package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderEnvFile(t *testing.T) {
	vars := []EnvVar{
		{Key: "SAKURA_ENDPOINTS_KMS", Value: "http://127.0.0.1:18081"},
		{Key: "SAKURA_ACCESS_TOKEN", Value: "dummy"},
	}
	out := RenderEnvFile(vars, false)

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
	if strings.Contains(out, "export ") {
		t.Errorf("non-export output should not contain 'export ':\n%s", out)
	}

	exported := RenderEnvFile(vars, true)
	for _, want := range []string{
		"export SAKURA_ENDPOINTS_KMS=http://127.0.0.1:18081\n",
		"export SAKURA_ACCESS_TOKEN=dummy\n",
	} {
		if !strings.Contains(exported, want) {
			t.Errorf("exported output missing %q\n%s", want, exported)
		}
	}
}

func TestWriteEnvFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sakumock.env")
	vars := []EnvVar{{Key: "SAKURA_ENDPOINTS_KMS", Value: "http://127.0.0.1:18081"}}

	if err := WriteEnvFile(path, vars, false); err != nil {
		t.Fatalf("WriteEnvFile: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != RenderEnvFile(vars, false) {
		t.Errorf("file content mismatch:\n got: %q\nwant: %q", got, RenderEnvFile(vars, false))
	}
}
