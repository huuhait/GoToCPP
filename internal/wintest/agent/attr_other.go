//go:build unix && !linux

package agent

import "syscall"

func dieWithAgent(*syscall.SysProcAttr) {}
