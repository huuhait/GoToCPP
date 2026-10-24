package build

import (
	"io/fs"

	"github.com/huuhait/gocpp/internal/load"
)

// A named pipe (a FIFO) with the name of a header waits for ever when the compiler opens it to
// read, until a writer opens it too: an `#include "pipe.h"` in a package's .c file or in a header
// it imports, or a header found through -I, would hang the build or the probe. gocpp's own reads
// refuse such a file (load.ReadRegular), but the headers that the compiler finds are its own
// business: gocpp can't know them before the compiler runs (the include path, macros, #if). So
// WatchPipes looks at the compiler while it runs: a process of its group that waits in the open of
// a named pipe is stopped, and the run's error is a *PipeError that names the pipe.
//
// Only the runs of a compiler are watched (the compiles and links of a build, the probes of
// interop, the language server's probes); a program that gocpp runs (gocpp run, gocpp test) may
// wait for a pipe on purpose.

// PipeError is the error of a compiler run that WatchPipes stopped because a process of the
// compiler waited to open a named pipe, which waits for a writer for ever.
type PipeError struct {
	// Path is the named pipe (absolute), or "" when the system did not say which file the
	// compiler was opening (a kernel that hides the system call of a process).
	Path string
}

func (e *PipeError) Error() string {
	if e.Path == "" {
		return "the compiler waited for a writer of a named pipe (a header or a source that is a named pipe?) and was stopped"
	}
	return e.Unwrap().Error() + " (the compiler waited for a writer of it and was stopped)"
}

// Unwrap returns load's error for the pipe ("open <path>: not a regular file: it is a named
// pipe"), so that load.IsNotRegular is true for it, or nil without a path.
func (e *PipeError) Unwrap() error {
	if e.Path == "" {
		return nil
	}
	return load.NotRegular(e.Path, fs.ModeNamedPipe)
}
