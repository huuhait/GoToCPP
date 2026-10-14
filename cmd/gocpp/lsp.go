package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/huuhait/gocpp/internal/lsp/server"
)

// lspUsage is the help text of "gocpp lsp". "gocpp lsp -h" prints it,
// followed by the flags.
const lspUsage = `usage: gocpp lsp [-logfile file]

gocpp lsp runs the gocpp language server. It speaks the Language Server
Protocol (LSP 3.17) over standard input and standard output: an editor
starts it and talks to it. Its log goes to standard error, or with
-logfile to the end of that file.

It exits with status 0 after the editor's shutdown request and exit
notification, and with status 1 when the editor exits it without
shutdown or closes the connection first.

For now the server keeps the text of the open files up to date and
publishes the diagnostics of gocpp check as you type, for the packages of
the open files (the C++ namespace probe of header imports runs in the
background, and its errors come when it is done). It answers hover (the
declaration and doc of a name), go to definition and type definition (also
into the packages that a package imports, and from a header import to the
header file), and signature help. For C/C++ names (raylib.InitWindow,
std.vector) it knows only where they come from. Completion and the other
features come in the later steps of Phase 11.

The server keeps a summary of each package it checks (its export data) in
a cache on disk, so that its next start loads the packages it doesn't show
from there instead of checking them again. The cache is in gocpp/lsp in the
user's cache folder ($XDG_CACHE_HOME or ~/.cache on Linux, %LocalAppData%
on Windows), each gocpp binary with its own entries. GOCPP_LSP_CACHE sets
another folder (an absolute name), or "off" turns the cache off;
GOCPP_LSP_CACHE_SIZE caps its size (default 1G; for example 512M) and
GOCPP_LSP_CACHE_AGE the time an entry is kept without use (default 120h;
the entries of other gocpp binaries go after a day). The server trims the
cache in the background, at most once an hour, and removes only the files
it wrote there. The folder can be deleted at any time.

The flags are:
`

// runLSP is "gocpp lsp" (phase 11 README, step 11.1).
func runLSP(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	// A client that closes our stdout must not kill the server with
	// SIGPIPE (Go's default for a write to a closed pipe on fd 1): the
	// write fails instead, the server logs it and exits like after the
	// end of its input. (SIGPIPE exists in package syscall on Windows too,
	// where this does nothing.)
	signal.Ignore(syscall.SIGPIPE)
	flags := flag.NewFlagSet("gocpp lsp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	logfile := flags.String("logfile", "", "append the log to `file`, not to standard error")
	flags.Usage = func() {
		fmt.Fprint(stderr, lspUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2 // flag has printed the error and the usage
	}
	if flags.NArg() > 0 {
		fmt.Fprintln(stderr, "usage: gocpp lsp [-logfile file]")
		return 2
	}
	logOut := stderr
	if *logfile != "" {
		f, err := os.OpenFile(*logfile, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
		if err != nil {
			fmt.Fprintf(stderr, "gocpp lsp: %v\n", err)
			return 2
		}
		defer f.Close()
		logOut = f
	}
	logger := log.New(logOut, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	return server.Run(context.Background(), stdin, stdout, logger)
}
