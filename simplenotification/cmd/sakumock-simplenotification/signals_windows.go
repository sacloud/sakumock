//go:build windows

package main

import "os"

func signals() []os.Signal {
	return []os.Signal{os.Interrupt}
}
