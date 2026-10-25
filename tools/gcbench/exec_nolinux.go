//go:build unix && !linux

package main

import "syscall"

func setPdeathsig(*syscall.SysProcAttr) {}

func readVmHWM(int) int64 { return 0 }
