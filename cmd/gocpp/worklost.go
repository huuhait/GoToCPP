package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// The work folder of a build is a new folder in the temporary folder (TMPDIR) that the compilers
// run in and the objects and the program are written to. Nothing in gocpp removes another build's
// work folder, but other things on a machine do: a cleaner of /tmp, a script that clears TMPDIR,
// a tool that shares the folder of another session. Seen on a shared machine with many builds at
// once: a compile that failed with "opening dependency file .../x.o.d: No such file or directory",
// a link whose object files were gone, "gocpp build: stat .../program: no such file or directory".
// All of them were reported as errors of gocpp ("internal error: generated C++ did not compile"),
// and all of them built fine when they were run again. So a build that finds its work folder gone
// makes a new one of the same name and builds again, at most maxWorkAttempts times (the folder
// is not gocpp's to protect: the point is that the program the user asked for gets built, and that
// an error says what happened when it can't).
//
// Two files in the folder (workSentinels) tell that the folder is as the build made it: the objects
// are in the folder too, so a cleaner that removes the contents but not the folder is seen as well.
// One name starts with a dot and one does not, so that `rm -rf folder/*` (which leaves the dotfile)
// and `rm -rf folder/.[!.]*` are both seen.
//
// The repository's own tests and campaigns must not hide a real bug behind this: GOCPP_WORKLOST=fail
// makes a lost work folder an error (status exitWorkLost, no new try), and they set it. Users keep
// the self-healing default. A removal in the milliseconds between the check of the program after the
// link and the start of the program (gocpp run) or the move of it (gocpp build) is not seen.

var workSentinels = [...]string{".gocpp-work", "gocpp-work"}

const (
	maxWorkAttempts = 3
	exitWorkLost    = 4 // GOCPP_WORKLOST=fail and the work folder was removed
)

// workLostFails reports whether GOCPP_WORKLOST=fail: a lost work folder is an error.
func workLostFails() bool { return os.Getenv("GOCPP_WORKLOST") == "fail" }

// testHookLink, if set, is called around each attempt of the link of a build (phase "before" and
// "after"; attempt 0 is the first one) with the work folder: the tests remove the folder there, as
// a cleaner would. Only the tests set it.
var testHookLink func(phase string, attempt int, work string)

// makeWorkFolder makes the work folder of a build: an absolute path in TMPDIR (made first if even
// TMPDIR is gone, as it is for a session that was shut down), with the markers in it.
func makeWorkFolder() (string, error) {
	work, err := os.MkdirTemp("", "gocpp-build")
	if errors.Is(err, fs.ErrNotExist) && os.MkdirAll(os.TempDir(), 0o700) == nil {
		work, err = os.MkdirTemp("", "gocpp-build")
	}
	if err == nil {
		work, err = filepath.Abs(work) // the compilers run in it
	}
	if err == nil {
		err = writeSentinels(work)
	}
	return work, err
}

// restoreWork makes the work folder work again, with the markers, after something removed it
// (and its parent, perhaps).
func restoreWork(work string) error {
	if err := os.MkdirAll(work, 0o700); err != nil {
		return err
	}
	return writeSentinels(work)
}

// writeSentinels writes the marker files of the work folder.
func writeSentinels(work string) error {
	for _, name := range workSentinels {
		if err := os.WriteFile(filepath.Join(work, name), nil, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// workLost reports whether the work folder was removed or emptied while the build ran: a marker file
// is gone, or the link succeeded (err is nil) and the program it wrote is gone. exe is "" for a
// build that is not a program in the work folder.
func workLost(work, exe string, dryRun bool, err error) bool {
	for _, name := range workSentinels {
		if _, serr := os.Stat(filepath.Join(work, name)); errors.Is(serr, fs.ErrNotExist) {
			return true
		}
	}
	if err == nil && exe != "" && !dryRun {
		_, serr := os.Stat(exe)
		return errors.Is(serr, fs.ErrNotExist)
	}
	return false
}
