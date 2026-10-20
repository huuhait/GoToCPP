// Package pclock is the lock that serialises the runs that use the Windows PC (Phase 14 step 14.7,
// ADR-111): two test runs (of different agents or checkouts) must never run their programs on the PC at
// the same time, because the programs share its 16 logical cores and the timing-sensitive ones would
// fail. It is a flock on a file of this machine that every run names the same way (GOCPP_WINDOWS_LOCK,
// default /tmp/gocpp-windows-pc.lock); the system releases it when the process ends, so a killed run
// never leaves a lock behind.
//
// The lock is counted inside one process: a second Acquire of the same file in the same process joins
// the first (flock locks conflict between two open files of one process, and a test binary can hold
// two Remotes), and the file is unlocked when the last holder releases.
package pclock

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/huuhait/gocpp/internal/filelock"
)

// DefaultPath is the lock file when nothing names one.
const DefaultPath = "/tmp/gocpp-windows-pc.lock"

// Options says where the lock is and how long to wait for it.
type Options struct {
	Path string        // the file; empty means $GOCPP_WINDOWS_LOCK, then DefaultPath
	Wait time.Duration // how long to wait; zero means $GOCPP_WINDOWS_LOCK_WAIT, then three hours
	Log  func(string)  // called about once a minute while waiting
	Poll time.Duration // how often a held lock is tried again; zero means two seconds
}

// PathFromEnv returns the lock file that the environment names.
func PathFromEnv() string {
	if p := os.Getenv("GOCPP_WINDOWS_LOCK"); p != "" {
		return p
	}
	return DefaultPath
}

func waitFromEnv() time.Duration {
	if s := os.Getenv("GOCPP_WINDOWS_LOCK_WAIT"); s != "" {
		if d, err := time.ParseDuration(s); err == nil && d > 0 {
			return d
		}
	}
	return 3 * time.Hour
}

type entry struct {
	gate chan struct{} // one acquirer at a time does the slow part
	mu   sync.Mutex    // n and f
	n    int
	f    *os.File
}

var (
	tableMu sync.Mutex
	table   = map[string]*entry{}
)

func entryFor(path string) *entry {
	tableMu.Lock()
	defer tableMu.Unlock()
	e := table[path]
	if e == nil {
		e = &entry{gate: make(chan struct{}, 1)}
		table[path] = e
	}
	return e
}

// Acquire takes the lock, waiting for another process that holds it, and returns the function that
// releases it. A context that ends, or the wait running out, is an error.
func Acquire(ctx context.Context, o Options) (release func(), err error) {
	if o.Path == "" {
		o.Path = PathFromEnv()
	}
	if o.Wait <= 0 {
		o.Wait = waitFromEnv()
	}
	if o.Poll <= 0 {
		o.Poll = 2 * time.Second
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	e := entryFor(o.Path)
	select {
	case e.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-e.gate }()
	e.mu.Lock()
	if e.n > 0 {
		e.n++
		e.mu.Unlock()
		return e.release, nil
	}
	e.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(o.Path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(o.Path, os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	lastLog := start
	for {
		ok, err := filelock.TryLock(f)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("locking %s: %v", o.Path, err)
		}
		if ok {
			e.mu.Lock()
			e.f, e.n = f, 1
			e.mu.Unlock()
			return e.release, nil
		}
		if time.Since(start) > o.Wait {
			f.Close()
			return nil, fmt.Errorf("the Windows machine is in use by another run (%s): waited %v", o.Path, o.Wait)
		}
		if o.Log != nil && time.Since(lastLog) >= time.Minute {
			o.Log(fmt.Sprintf("waiting for the Windows machine, which another run uses (%s), for %v", o.Path, time.Since(start).Round(time.Second)))
			lastLog = time.Now()
		}
		select {
		case <-ctx.Done():
			f.Close()
			return nil, ctx.Err()
		case <-time.After(o.Poll):
		}
	}
}

// release gives one hold back; the last one unlocks the file.
func (e *entry) release() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.n == 0 {
		return
	}
	if e.n--; e.n == 0 {
		filelock.Unlock(e.f)
		e.f.Close()
		e.f = nil
	}
}
