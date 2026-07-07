package simplenotification

import (
	"runtime"
	"slices"
	"testing"
)

func TestShellCommand(t *testing.T) {
	cmd := shellCommand("echo hi")
	want := []string{"sh", "-c", "echo hi"}
	if runtime.GOOS == "windows" {
		want = []string{"cmd", "/c", "echo hi"}
	}
	if !slices.Equal(cmd.Args, want) {
		t.Fatalf("shellCommand args = %v, want %v", cmd.Args, want)
	}
}
