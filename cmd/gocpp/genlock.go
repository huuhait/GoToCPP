package main

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// Two builds of the same program (an editor that builds on save next to a build by hand) use the
// same folder of generated files, gen/<hash>, so that its paths are stable. The files are written
// one by one (each atomically), so a build that writes while another compiles could mix the old
// and the new files in the other's compiles. So the build that writes the files of a program holds
// the flock of gen/<hash>.lock until its compiles are done (the link reads objects only); another
// build of the program waits for it, then writes its own files. When the lock can't be taken (a
// file system without locks, a system without flock) the build gets a folder of its own,
// gen/<hash>-<pid>-<n>, and removes it when it succeeds: the object cache then misses (the paths in
// the text differ), but no build reads another's files.

var (
	// testHookGenWait, if set, is called when a build finds the lock of its program taken, before it
	// waits; testHookGenWritten when a build holds the lock and has written its files. Only the tests
	// set them.
	testHookGenWait    func()
	testHookGenWritten func()

	privateGens atomic.Int64

	// lockFile takes the lock; the tests replace it with a system that has none.
	lockFile = tryLockFile
)

// lockGen returns the folder in which this build writes and compiles the generated files of its
// program, after it took the lock of gen (a folder of the cache, from genRoot), and a function that
// ends the use of the folder: it releases the lock, or removes a private folder if ok. It returns
// gen itself, without a lock, for a folder that is in the work folder (-work: nobody else uses it).
func (d *driver) lockGen(gen, work string) (dir string, release func(ok bool)) {
	if strings.HasPrefix(gen, work+string(os.PathSeparator)) {
		return gen, func(bool) {}
	}
	private := func() (string, func(bool)) {
		dir := fmt.Sprintf("%s-%d-%d", gen, os.Getpid(), privateGens.Add(1))
		return dir, func(ok bool) {
			if ok {
				os.RemoveAll(dir)
			}
		}
	}
	if err := os.MkdirAll(gen, 0o755); err != nil {
		return private()
	}
	waited := false
	for {
		l, busy, err := lockFile(gen + ".lock")
		switch {
		case err != nil:
			return private()
		case !busy:
			return gen, func(bool) { l.unlock() }
		}
		if !waited {
			waited = true
			if testHookGenWait != nil {
				testHookGenWait()
			}
		}
		select {
		case <-d.ctx.Done():
			// The build is stopped (a signal): the caller finds out; use a folder of its own.
			return private()
		case <-time.After(20 * time.Millisecond):
		}
	}
}
