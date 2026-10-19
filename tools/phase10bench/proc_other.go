//go:build !unix

package main

import (
	"os/exec"
	"time"
)

func setupChild(cmd *exec.Cmd) { cmd.WaitDelay = 2 * time.Second }
