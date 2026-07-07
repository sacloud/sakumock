//go:build windows

package objectstorage

import (
	"fmt"
	"os"
)

// posixMetadataArgs returns extra versitygw posix-backend flags for metadata
// storage. NTFS has no xattr support — versitygw's default metadata store,
// verified at startup, so without this it exits immediately — and the sidecar
// directory must already exist (versitygw only stats it).
func posixMetadataArgs(dir string) ([]string, error) {
	sc := sidecarDir(dir)
	if err := os.MkdirAll(sc, 0o755); err != nil {
		return nil, fmt.Errorf("create sidecar metadata dir %q: %w", sc, err)
	}
	return []string{"--sidecar", sc}, nil
}

// terminateProcess stops the data plane process. Windows has no SIGTERM
// (Process.Signal supports nothing but Kill there), so the process is
// killed directly instead of waiting for exec's WaitDelay to do it.
func terminateProcess(p *os.Process) error {
	return p.Kill()
}
