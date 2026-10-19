//go:build unix && !linux

package build

import "syscall"

func setPdeathsig(*syscall.SysProcAttr) {}
