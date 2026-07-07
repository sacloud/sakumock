//go:build !windows

package objectstorage

import (
	"os"
	"syscall"
)

// posixMetadataArgs returns extra versitygw posix-backend flags for metadata
// storage. Unix filesystems support xattrs — versitygw's default — so none
// are needed.
func posixMetadataArgs(string) ([]string, error) {
	return nil, nil
}

// terminateProcess asks the data plane process to shut down gracefully.
// SIGTERM lets versitygw finish in-flight requests; exec's WaitDelay
// hard-kills it if it does not exit in time.
func terminateProcess(p *os.Process) error {
	return p.Signal(syscall.SIGTERM)
}
