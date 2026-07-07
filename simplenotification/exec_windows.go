//go:build windows

package simplenotification

import "os/exec"

// shellCommand builds the command that runs the configured --exec script
// through the platform shell.
func shellCommand(script string) *exec.Cmd {
	return exec.Command("cmd", "/c", script)
}
