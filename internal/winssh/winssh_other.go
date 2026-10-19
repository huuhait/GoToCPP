//go:build !linux

package winssh

import (
	"context"
	"errors"
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
func EscapeArg(s string) string { return s }
