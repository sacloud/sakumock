package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alecthomas/kong"
)

// parseAll parses "all" with the given extra args against a fresh CLI and
// returns the populated AllCmd.
func parseAll(t *testing.T, args ...string) *AllCmd {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("sakumock"))
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	if _, err := parser.Parse(append([]string{"all"}, args...)); err != nil {
		t.Fatalf("parse: %v", err)
	}
	return &cli.All
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigFileYAML(t *testing.T) {
	// Both YAML extensions go through the same parser.
	for _, name := range []string{"sakumock.yaml", "sakumock.yml"} {
		t.Run(name, func(t *testing.T) {
			path := writeFile(t, name, "kms:\n  latency: 5s\nsimplemq:\n  addr: 127.0.0.1:29080\n  rate-limit-window: 2s\n")

			all := parseAll(t, "--config", path)
			if all.Kms.Latency != 5*time.Second {
				t.Errorf("kms latency = %v, want 5s", all.Kms.Latency)
			}
			if all.Simplemq.Addr != "127.0.0.1:29080" {
				t.Errorf("simplemq addr = %q, want 127.0.0.1:29080", all.Simplemq.Addr)
			}
			if all.Simplemq.RateLimitWindow != 2*time.Second {
				t.Errorf("simplemq rate-limit-window = %v, want 2s", all.Simplemq.RateLimitWindow)
			}
			// A service not mentioned keeps its default.
			if all.Secretmanager.Addr != "127.0.0.1:18082" {
				t.Errorf("secretmanager addr = %q, want default", all.Secretmanager.Addr)
			}
		})
	}
}

func TestConfigFileJSON(t *testing.T) {
	path := writeFile(t, "sakumock.json", `{"kms":{"latency":"5s"},"simplemq":{"addr":"127.0.0.1:29080"}}`)

	all := parseAll(t, "--config", path)
	if all.Kms.Latency != 5*time.Second {
		t.Errorf("kms latency = %v, want 5s", all.Kms.Latency)
	}
	if all.Simplemq.Addr != "127.0.0.1:29080" {
		t.Errorf("simplemq addr = %q, want 127.0.0.1:29080", all.Simplemq.Addr)
	}
}

func TestConfigFileFlagOverrides(t *testing.T) {
	path := writeFile(t, "sakumock.yaml", "kms:\n  latency: 5s\n")

	all := parseAll(t, "--config", path, "--kms-latency", "9s")
	if all.Kms.Latency != 9*time.Second {
		t.Errorf("kms latency = %v, want 9s (CLI flag should override config)", all.Kms.Latency)
	}
}

func TestConfigFileUnsupportedExtension(t *testing.T) {
	path := writeFile(t, "sakumock.toml", "kms.latency = \"5s\"\n")

	var cli CLI
	parser, err := kong.New(&cli, kong.Name("sakumock"))
	if err != nil {
		t.Fatalf("kong.New: %v", err)
	}
	if _, err := parser.Parse([]string{"all", "--config", path}); err == nil {
		t.Fatal("expected an error for an unsupported extension, got nil")
	}
}
