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
	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// lspUsage is the help text of "gocpp lsp". "gocpp lsp -h" prints it,
// followed by the flags.
const lspUsage = `usage: gocpp lsp [-logfile file] [-slow duration] [-profile dir [-exec-trace]] [-trace file] [-wide-family]

gocpp lsp runs the gocpp language server. It speaks the Language Server
Protocol (LSP 3.17) over standard input and standard output: an editor
starts it and talks to it. Its log goes to standard error, or with
-logfile to the end of that file.

It exits with status 0 after the editor's shutdown request and exit
notification, and with status 1 when the editor exits it without
shutdown or closes the connection first.

The server keeps the text of the open files up to date and publishes the
diagnostics of gocpp check as you type, for the packages of the open files
(the C++ namespace probe of header imports runs in the background, and its
errors come when it is done). It answers hover (the declaration and doc of
a name), go to definition and type definition (also into the packages that
a package imports, and from a header import to the header file), signature
help, completion (also of the packages to import), formatting and organize
imports, references and highlights, rename (a safe rewrite of the module's
names, tests included), document and workspace symbols, semantic tokens,
inlay hints, document links (header imports) and the quick fix "Add
import". For C/C++ names (raylib.InitWindow, std.vector) it knows only where
they come from: they have no references and are not renamed. The features
for ! and macros come in the later steps of Phase 11.

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

A request, a notification or a diagnostics run that takes -slow or longer
(default 500ms; a negative value turns the log off) is logged with the time
of its phases (parse, load, typecheck, ...) and the state of the session.
-profile dir writes the CPU profile (cpu.pprof) and the heap profile
(heap.pprof) of the server in dir when it ends (go tool pprof reads them),
and -exec-trace the Go execution trace (exec.trace) too; -trace file writes
the spans of the work as a Chrome trace (chrome://tracing, Perfetto,
speedscope); it keeps up to a million spans in memory (about 150 MB) until
the server ends. After SIGINT or SIGTERM the files are written and the
process exits at once, without waiting for the cache of summaries (its writes
in flight are lost; its entries are checked when read). The request gocpp/stats, also the command gocpp.stats of
workspace/executeCommand, returns the cache hit rates, the snapshots and the
memory.

The server has a soft memory limit that follows its live heap (256 MiB or
1.4 times the live heap and 64 MiB, whichever is more; GOMEMLIMIT, when set,
is used instead), and, once the first diagnostics of a package are out, checks
the packages that import it in the background (at most 96, one or two at a
time, while it is idle; -no-warm turns that off), so that the first references
or rename of an exported name finds them done. -idle-gc makes it collect
garbage in the gaps between two pieces of work too (off: it did not pay).

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
	modfetch.SetOffline(true) // an editor never starts a module download
	defer modfetch.SetOffline(false)
	flags := flag.NewFlagSet("gocpp lsp", flag.ContinueOnError)
	flags.SetOutput(stderr)
	logfile := flags.String("logfile", "", "append the log to `file`, not to standard error")
	slow := flags.Duration("slow", server.DefaultSlow, "log the work that takes `duration` or longer, with its phases (negative: off)")
	profile := flags.String("profile", "", "write the CPU and heap profiles to the folder `dir` when the server ends")
	execTrace := flags.Bool("exec-trace", false, "with -profile, write the Go execution trace (exec.trace) too")
	traceFile := flags.String("trace", "", "write the spans of the work to `file` as a Chrome trace when the server ends")
	checkCache := flags.Int("check-cache", 0, "the cap of the cache of checked packages, in `MB` (0: 10% of the memory of the machine, 64 to 300 MB)")
	dynLimit := flags.Int("dynamic-limit", 0, "refuse a rename that would also rename more than `n` methods only because a type assertion could reach their types (0: 20, negative: no limit)")
	wide := flags.Bool("wide-family", false, "references and rename follow a method through every interface of the module that its type could satisfy, not only through the interfaces that a use needs (slower)")
	idleGC := flags.Bool("idle-gc", false, "collect garbage in the gaps between two pieces of work, besides when the Go runtime does it (the heap has doubled)")
	noWarm := flags.Bool("no-warm", false, "do not check the packages that import an open package in the background (the first references or rename of an exported name then checks them)")
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
		fmt.Fprintln(stderr, "usage: gocpp lsp [-logfile file] [-slow duration] [-profile dir [-exec-trace]] [-trace file] [-wide-family]")
		return 2
	}
	// The compiler runs of the server are background work: all the cores up to eight, not half
	// (GOCPP_PROBE_JOBS still decides).
	probe.SetInteractive(true)
	defer probe.SetInteractive(false)
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
	prof, err := startLSPProfile(*profile, *execTrace, *traceFile, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "gocpp lsp: %v\n", err)
		return 2
	}
	// The gates of the phases are those of gocpp check: the editor shows the
	// errors that a build would give (GOCPP_GATE_PHASE opens them for the
	// development of a phase).
	phase, err := gatePhase(os.Getenv(gatePhaseEnv))
	if err != nil {
		fmt.Fprintf(stderr, "gocpp lsp: %v\n", err)
		return 2
	}
	opts := server.Options{Slow: *slow, Phase: phase, WideFamily: *wide, DynamicLimit: *dynLimit, CheckCacheMB: *checkCache, IdleGC: *idleGC, WarmImporters: !*noWarm}
	if prof != nil {
		defer prof.stop()
		opts.Trace = prof.rec
	}
	return server.RunWith(context.Background(), stdin, stdout, logger, opts)
}
