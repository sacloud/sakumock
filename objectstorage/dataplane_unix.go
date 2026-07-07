//go:build !windows

package objectstorage

import (
	"os"
	"syscall"
)

// terminateProcess asks the data plane process to shut down gracefully.
// SIGTERM lets versitygw finish in-flight requests; exec's WaitDelay
// hard-kills it if it does not exit in time.
func terminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
