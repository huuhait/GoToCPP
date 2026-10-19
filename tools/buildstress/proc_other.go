//go:build !unix

package main

import "os/exec"

func setupCmd(*exec.Cmd) {}

func setNice(int) {}
