//go:build !unix

package gotest

import "os/exec"

func setGroup(cmd *exec.Cmd) {}

func killGroup(cmd *exec.Cmd) error { return cmd.Process.Kill() }
