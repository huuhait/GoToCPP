//go:build !unix

package diff

import "os/exec"

func inGroup(cmd *exec.Cmd)         {}
func killGroup(cmd *exec.Cmd)       {}
func terminate(cmd *exec.Cmd) error { return cmd.Process.Kill() }
