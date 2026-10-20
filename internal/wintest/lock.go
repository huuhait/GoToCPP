package wintest

import (
	"context"
	"time"

	"github.com/huuhait/gocpp/internal/pclock"
)

// A Locker serialises the runs that use the same machine: two runs of the suite (of different agents
// or checkouts) must never run their programs on the PC at the same time, because the programs share
// its 16 logical cores and the timing-sensitive ones would fail. Lock returns the function that
// releases it.
type Locker interface {
	Lock(ctx context.Context) (unlock func(), err error)
}

// A FileLock is the lock of internal/pclock: a flock on a file of this machine that every run which
// uses the PC names the same way (GOCPP_WINDOWS_LOCK), counted inside one process. The system
// releases it when the process ends.
type FileLock struct {
	Path string
	Wait time.Duration
	Log  func(string)
	Poll time.Duration
}

func (l *FileLock) Lock(ctx context.Context) (func(), error) {
	return pclock.Acquire(ctx, pclock.Options{Path: l.Path, Wait: l.Wait, Log: l.Log, Poll: l.Poll})
}
