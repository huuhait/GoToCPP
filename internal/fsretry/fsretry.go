// Package fsretry renames files the way Windows needs it (Phase 14 step 14.2). On Windows a rename
// fails with "Access is denied" or a sharing violation while another process has the file or a file
// below the folder open: a virus scanner that looks at what was just written, the search indexer, a
// backup, or a program that is still running when its file is replaced. These conditions pass in
// milliseconds, so Rename waits and tries again for a short time before it gives up. On other systems
// it is os.Rename.
package fsretry

import (
	"os"
	"time"
)

// Rename is os.Rename, repeated on Windows for the errors that pass (access denied, a sharing or lock
// violation) for about three seconds. A rename of a folder onto a folder that exists is not repeated:
// that never works, and callers read the failure as "another process was first".
func Rename(from, to string) error {
	err := os.Rename(from, to)
	if err == nil || !transient(err) {
		return err
	}
	if isDir(from) && isDir(to) {
		return err
	}
	return retry(err, func() error { return os.Rename(from, to) })
}

// Open is os.Open, repeated on Windows for the errors that pass, as Rename does. A file that another
// process replaces by a rename is, for a moment, not open-able for reading (a sharing violation), so
// a reader of a file that other gocpp processes write must not give up at the first try. On other
// systems it is os.Open.
func Open(name string) (*os.File, error) {
	file, err := os.Open(name)
	if err == nil || !transient(err) {
		return file, err
	}
	err = retry(err, func() error {
		var openErr error
		file, openErr = os.Open(name)
		return openErr
	})
	return file, err
}

// retry runs op again, with a growing pause, for about three seconds as long as it fails with an
// error that passes. first is the error of the try that was already made. It returns the error of the
// last try.
func retry(first error, op func() error) error {
	err := first
	delay := 5 * time.Millisecond
	for waited := time.Duration(0); waited < 3*time.Second; waited += delay {
		time.Sleep(delay)
		if err = op(); err == nil || !transient(err) {
			return err
		}
		if delay < 200*time.Millisecond {
			delay *= 2
		}
	}
	return err
}

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}
