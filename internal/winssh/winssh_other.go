//go:build !linux

package winssh

import (
	"context"
	"errors"
	"io"
	"time"
)

// A Remote is not available on this system: the tests that use it run on Linux.
type Remote struct{}

type Result struct {
	Stdout, Stderr []byte
	Code           int
	TimedOut       bool
}

func New(host string) (*Remote, error) {
	return nil, errors.New("running on a Windows machine over ssh needs a Linux host")
}
func (r *Remote) Close() {}
func (r *Remote) Run(context.Context, time.Duration, string, []string, []string, bool) (Result, error) {
	return Result{}, errors.New("not available")
}

// A Process is not available on this system either.
type Process struct {
	Stdin  io.WriteCloser
	Stdout io.Reader
}

func (r *Remote) Start(context.Context, []string, string) (*Process, error) {
	return nil, errors.New("not available")
}
func (p *Process) Stderr() string                    { return "" }
func (p *Process) Done() <-chan struct{}             { return nil }
func (p *Process) Wait(context.Context) (int, error) { return 0, errors.New("not available") }
func (p *Process) Kill()                             {}

func EscapeArg(s string) string { return s }
func (r *Remote) RunConsole(context.Context, time.Duration, string, string, string) (string, int, error) {
	return "", 0, errors.New("not available")
}

func (r *Remote) RunID() string { return "" }
func (r *Remote) Dir() string   { return "" }
func (r *Remote) Home(context.Context) (string, error) {
	return "", errors.New("not available")
}
func (r *Remote) Abs(context.Context, string) (string, error) {
	return "", errors.New("not available")
}
func (r *Remote) Put(context.Context, string, string) error { return errors.New("not available") }
func (r *Remote) Get(context.Context, string, string) error { return errors.New("not available") }
func (r *Remote) Exec(context.Context, time.Duration, []string, string) (Result, error) {
	return Result{}, errors.New("not available")
}

func (r *Remote) KillRun() {}

func NewUnlocked(host string) (*Remote, error) { return New(host) }
func (r *Remote) CleanStale(context.Context, time.Duration) error {
	return errors.New("not available")
}
func (r *Remote) FreeBytes(context.Context) (int64, error) { return 0, errors.New("not available") }
func (r *Remote) Remove(context.Context, ...string) error  { return errors.New("not available") }

func Interrupt() {}
func Resume()    {}
