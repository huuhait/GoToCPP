//go:build !linux

package modget

import "os/exec"

func dieWithTest(cmd *exec.Cmd) {}
