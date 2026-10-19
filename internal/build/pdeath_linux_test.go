//go:build linux

package build

import "syscall"

// setPdeathsig makes a child die with the test process.
func setPdeathsig(a *syscall.SysProcAttr) { a.Pdeathsig = syscall.SIGKILL }
