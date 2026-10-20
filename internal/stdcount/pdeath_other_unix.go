//go:build unix && !linux

package stdcount

import "syscall"

func setParentDeathSignal(attr *syscall.SysProcAttr) {}
