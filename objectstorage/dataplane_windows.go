//go:build windows

package objectstorage

import "os"

// terminateProcess stops the data plane process. Windows has no SIGTERM
// (Process.Signal supports nothing but Kill there), so the process is
// killed directly instead of waiting for exec's WaitDelay to do it.
func terminateProcess(p *os.Process) error {
	return p.Kill()
}
