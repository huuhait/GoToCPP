//go:build !linux

package wine

import (
	"context"
	"errors"
	"os/exec"
)

// Overrides is the WINEDLLOVERRIDES of every Wine process that the tests start.
const Overrides = "winemenubuilder.exe=d;mscoree,mshtml=;winewayland.drv=d"

// Available returns why Wine can't be used: only Linux runs it in gocpp's tests.
func Available() string { return "Wine runs the Windows tests on Linux only" }

// An Env is a Wine prefix and a server for it (Linux only).
type Env struct{}

func New(string) (*Env, error)                            { return nil, errors.New(Available()) }
func (*Env) Prefix() string                               { return "" }
func (*Env) Wine() string                                 { return "" }
func (*Env) Environ() []string                            { return nil }
func (*Env) Command(context.Context, ...string) *exec.Cmd { return nil }
func (*Env) Close()                                       {}
func (*Env) Processes() []int                             { return nil }
