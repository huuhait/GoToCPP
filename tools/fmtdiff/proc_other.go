//go:build !unix

package main

import "os/exec"

func inGroup(cmd *exec.Cmd) {}
