//go:build !unix && !windows

package agent

import (
	"os"
	"os/exec"
)

type group struct{ p *os.Process }

func newGroup(*exec.Cmd) (*group, error) { return &group{}, nil }
func (g *group) attach(p *os.Process)    { g.p = p }
func (g *group) kill() {
	if g.p != nil {
		g.p.Kill()
	}
}
func (g *group) close() {}
