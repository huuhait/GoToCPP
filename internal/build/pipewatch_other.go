//go:build !linux

package build

import "os/exec"

// WatchPipes does nothing here (see pipewatch_linux.go): Windows has no named pipes in its
// folders, and the other systems are not targets (ADR-021).
func WatchPipes(*exec.Cmd) (stop func() error) {
	return func() error { return nil }
}
