package main

// gocpp debug (Phase 13, step 13.2): build a program with debug information and start gdb or
// lldb on it, with gocpp's printers loaded. docs/guides/debugging.md describes it.

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/huuhait/gocpp/internal/build"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

const debugUsage = `usage: gocpp debug [build flags] [debug flags] [package | file.gp...] [arguments...]

gocpp debug builds the program like gocpp run (see "gocpp build -h"; the
default build has debug information and no optimization), in a work folder, and
starts gdb on it (lldb with -debugger lldb, or when there is no gdb). The
arguments after the program go to the program. gdb finds gocpp's printers in
the program itself (the .debug_gdb_scripts section of every program: it shows
gp::string, gp::slice, gp::map, gp::ptr, interfaces, channels and closures as
Go values, Go names in backtraces, and the commands "gp goroutines", "gp bt",
"gp locals" and "gp print"); gocpp debug lets gdb run that script (the
program is a safe path for auto-loading). lldb gets the summaries of
runtime/debug/gocpp_lldb.py, which gocpp debug writes to the work folder.

Breakpoints on .gp lines work as in Go: break hello.gp:12 (gdb), or
breakpoint set -f hello.gp -l 12 (lldb). The debugger takes over the terminal;
gocpp waits for it, removes the work folder, and exits with its status.

The debug flags are:
`

// debugFlags are the flags of gocpp debug that the build flags don't have.
type debugFlags struct {
	debugger string
	cmds     stringList // -ex
	maps     stringList // -map
	batch    bool
	show     bool
}

// A stringList is a flag that can be given more than once.
type stringList []string

func (l *stringList) String() string     { return strings.Join(*l, " ") }
func (l *stringList) Set(s string) error { *l = append(*l, s); return nil }

// runDebug is "gocpp debug".
func runDebug(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var f buildFlags
	var df debugFlags
	flags := flag.NewFlagSet("gocpp debug", flag.ContinueOnError)
	f.register(flags, false)
	flags.StringVar(&df.debugger, "debugger", "", "the debugger: gdb or lldb (default gdb, or lldb when there is no gdb)")
	flags.Var(&df.cmds, "ex", "run this debugger command after the program is loaded (repeatable: -ex 'break hello.gp:12' -ex run)")
	flags.Var(&df.maps, "map", "FROM=TO: show the sources that the debug info names under FROM from TO (a program built with -trimpath, or moved)")
	flags.BoolVar(&df.batch, "batch", false, "run the -ex commands and exit, without a prompt (for scripts and tests)")
	flags.BoolVar(&df.show, "print", false, "print the debugger's command line and don't start it")
	if code, done := parseFlags("debug", debugUsage, flags, args, stderr); done {
		return code
	}
	if df.debugger != "" && df.debugger != "gdb" && df.debugger != "lldb" {
		fmt.Fprintf(stderr, "gocpp debug: -debugger %s: want gdb or lldb\n", df.debugger)
		return 2
	}
	for _, m := range df.maps {
		if from, to, ok := strings.Cut(m, "="); !ok || from == "" || to == "" {
			fmt.Fprintf(stderr, "gocpp debug: -map %s: want FROM=TO\n", m)
			return 2
		}
	}
	args = flags.Args()
	n := 0
	for n < len(args) && strings.HasSuffix(args[n], ".gp") {
		n++
	}
	switch {
	case n == 0 && (len(args) == 0 || strings.HasPrefix(args[0], "-")):
		fmt.Fprintln(stderr, "gocpp debug: no .gp files listed")
		return exitErrors
	case n == 0:
		n = 1
	}
	if df.debugger == "" {
		df.debugger = defaultDebugger()
	}
	d, code := newDriver("debug", &f, stdout, stderr)
	if d == nil {
		return code
	}
	defer d.stopSignals()
	d.stdin = stdin
	return d.debug(args[:n], args[n:], &df)
}

// defaultDebugger returns gdb, or lldb when only that is installed.
func defaultDebugger() string {
	if _, err := exec.LookPath("gdb"); err != nil {
		if _, err := exec.LookPath("lldb"); err == nil {
			return "lldb"
		}
	}
	return "gdb"
}

// quoteArg quotes a path or a word for the command line of gdb or lldb: unchanged when it
// has no space, tab, quote or backslash, else in double quotes with backslashes before the
// quotes and backslashes.
func quoteArg(s string) string {
	if !strings.ContainsAny(s, " \t\"\\'") {
		return s
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// debugArgs returns the arguments of the debugger df.debugger for the program exe with the
// arguments args. script is the file with the debugger's script: the LLDB summaries, or for gdb
// (only used when exe has a colon in its name, which a safe path can't hold) gocpp-gdb.py.
func debugArgs(df *debugFlags, exe string, args []string, script string) []string {
	var a []string
	switch df.debugger {
	case "lldb":
		a = append(a, "-O", "command script import "+quoteArg(script))
		if df.batch {
			a = append(a, "-b")
		}
		for _, m := range df.maps {
			from, to, _ := strings.Cut(m, "=")
			a = append(a, "-o", "settings append target.source-map "+quoteArg(from)+" "+quoteArg(to))
		}
		for _, c := range df.cmds {
			a = append(a, "-o", c)
		}
		a = append(a, "--", exe)
	default:
		a = append(a, "-q")
		if df.batch {
			a = append(a, "-batch")
		}
		// The program carries gocpp's gdb script; gdb runs it for a safe path only. A safe
		// path is a colon-separated list: a program whose name has a colon (TMPDIR) gets the
		// script by hand instead.
		if strings.Contains(exe, ":") && script != "" {
			a = append(a, "-iex", "set auto-load python-scripts off", "-ex", "source "+script)
		} else {
			a = append(a, "-iex", "add-auto-load-safe-path "+exe)
		}
		for _, m := range df.maps {
			from, to, _ := strings.Cut(m, "=")
			a = append(a, "-iex", "set substitute-path "+quoteArg(from)+" "+quoteArg(to))
		}
		for _, c := range df.cmds {
			a = append(a, "-ex", c)
		}
		a = append(a, "--args", exe)
	}
	return append(a, args...)
}

// debug builds the program and runs the debugger on it.
func (d *driver) debug(patterns, args []string, df *debugFlags) int {
	p, code := d.compile(patterns)
	if p == nil {
		return code
	}
	defer d.removeWork()
	script := ""
	switch {
	case df.debugger == "lldb":
		script = filepath.Join(p.work, "gocpp_lldb.py")
		if err := os.WriteFile(script, rtfiles.LLDBScript(), 0o644); err != nil {
			fmt.Fprintf(d.stderr, "gocpp debug: %v\n", err)
			return exitErrors
		}
	case strings.Contains(p.exe, ":"):
		script = filepath.Join(p.work, "gocpp-gdb.py")
		if err := os.WriteFile(script, rtfiles.GDBScript(), 0o644); err != nil {
			fmt.Fprintf(d.stderr, "gocpp debug: %v\n", err)
			return exitErrors
		}
	}
	argv := debugArgs(df, p.exe, args, script)
	if df.show || d.flags.x {
		words := []string{df.debugger}
		for _, a := range argv {
			words = append(words, build.ShellQuoteWork(a, p.work))
		}
		out := d.stdout
		if !df.show {
			out = d.stderr
		}
		fmt.Fprintln(out, strings.Join(words, " "))
		if df.show {
			return 0
		}
	}
	path, err := exec.LookPath(df.debugger)
	if err != nil {
		fmt.Fprintf(d.stderr, "gocpp debug: %v (the nix dev shell has gdb and lldb; -debugger chooses)\n", err)
		return exitErrors
	}
	// The debugger stays in gocpp's process group, so that the terminal's signals reach it, and
	// it takes over the terminal; SIGINT and SIGQUIT are its own (watchSignals ignores them
	// while it runs), SIGTERM and SIGHUP are passed on.
	notify(d.sigs, syscall.SIGQUIT)
	if sig := d.interrupted(); sig != nil {
		return stopStatus(sig)
	}
	cmd := exec.CommandContext(d.ctx, path, argv...)
	cmd.Dir = d.check.dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = d.stdin, d.stdout, d.stderr
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(d.stderr, "gocpp debug: %v\n", err)
		return exitErrors
	}
	d.mu.Lock()
	d.running = cmd
	d.mu.Unlock()
	err = cmd.Wait()
	d.mu.Lock()
	d.running = nil
	d.mu.Unlock()
	state := cmd.ProcessState
	if state == nil {
		fmt.Fprintf(d.stderr, "gocpp debug: %v\n", err)
		return exitErrors
	}
	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return stopStatus(ws.Signal())
	}
	if err != nil && !errors.As(err, new(*exec.ExitError)) {
		fmt.Fprintf(d.stderr, "gocpp debug: %v\n", err)
		return exitErrors
	}
	return state.ExitCode()
}
