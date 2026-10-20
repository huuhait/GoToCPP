//go:build unix

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

// A group ends a program and what it started: its process group.
type group struct{ pid int }

func newGroup(cmd *exec.Cmd) (*group, error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	dieWithAgent(cmd.SysProcAttr)
	return &group{}, nil
}

func (g *group) attach(p *os.Process) { g.pid = p.Pid }

func (g *group) kill() {
	if g.pid > 0 {
		syscall.Kill(-g.pid, syscall.SIGKILL)
	}
}

func (g *group) close() {}
