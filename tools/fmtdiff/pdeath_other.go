//go:build unix && !linux

package main

import "syscall"

func setPdeathsig(a *syscall.SysProcAttr) {}
