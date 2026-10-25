//go:build unix

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

// A genLock is an advisory lock (flock) on a file: whoever holds it is the one build of its program
// that writes the generated files and compiles them (see lockGen).
type genLock struct{ f *os.File }

// tryLockFile tries to take the lock of the file name without waiting. busy says that another
// holder has it; err is any other failure (a file system without locks).
func tryLockFile(name string) (l *genLock, busy bool, err error) {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, false, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err != syscall.EINTR {
			break
		}
	}
	switch err {
	case nil:
		// The holder's pid, for a build that waits (lockHolder), a diagnostic only. It is one
		// write of a fixed width (11 bytes) over the old content and then a cut to that length, never
		// a truncate to empty first, so a waiter reads the previous holder's pid or this one's, not
		// nothing. It can be stale (the holder of the flock is whoever has the lock; a holder that
		// died has released it, and the file keeps its pid until the next one writes) and wrong
		// under pid namespaces (containers that share a cache folder see different pids), so the
		// notice says "pid N" as a hint, not as a proof.
		f.WriteAt([]byte(fmt.Sprintf("%-10d\n", os.Getpid())), 0)
		f.Truncate(11)
		return &genLock{f}, false, nil
	case syscall.EWOULDBLOCK:
		f.Close()
		return nil, true, nil
	}
	f.Close()
	return nil, false, err
}

// unlock releases the lock. (A process that dies releases it too.)
func (l *genLock) unlock() {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	l.f.Close()
}

// lockHolder returns the pid that the holder of the lock file name wrote into it, or "".
func lockHolder(name string) string {
	data, err := os.ReadFile(name)
	if err != nil {
		return ""
	}
	pid := strings.TrimSpace(string(data))
	if _, err := strconv.Atoi(pid); err != nil {
		return ""
	}
	return pid
}
