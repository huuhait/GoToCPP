//go:build !linux

package main

import "os/exec"

func setPdeathsig(*exec.Cmd) {}
