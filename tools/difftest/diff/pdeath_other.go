//go:build unix && !linux

package diff

import "syscall"

func setPdeathsig(a *syscall.SysProcAttr) {}
