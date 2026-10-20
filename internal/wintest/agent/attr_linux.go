package agent

import "syscall"

// dieWithAgent ends the program with the agent (the driver's unit tests must not leak processes).
func dieWithAgent(a *syscall.SysProcAttr) { a.Pdeathsig = syscall.SIGKILL }
