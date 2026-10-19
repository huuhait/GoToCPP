//go:build !linux

package main

import "os/exec"

// setProcessGroup: the snippet tests only start compilers on Linux (and the other systems kill
// just the command).
func setProcessGroup(cmd *exec.Cmd) {}
