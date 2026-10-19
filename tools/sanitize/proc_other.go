//go:build !unix

package main

import "os/exec"

// Without Unix process groups, canceling a command kills only the command.
func inGroup(*exec.Cmd)   {}
func killGroup(*exec.Cmd) {}
func noCoreFiles()        {}
