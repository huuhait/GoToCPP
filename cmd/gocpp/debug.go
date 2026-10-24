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
	"runtime"
	"slices"
	"strings"
	"syscall"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/toolchain"
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

On Windows the program is a PE file with DWARF debug information; the debugger is
the gdb of MinGW-w64 (WinLibs, MSYS2) or the lldb of llvm-mingw, found in PATH and
in the folders of those installs. gocpp debug runs the program on this machine only:
with -os windows on another system it can show the command line (-print) but not
start it. -vscode prints a launch.json for VS Code (see docs/guides/debugging.md).

The debug flags are:
`

// debugFlags are the flags of gocpp debug that the build flags don't have.
type debugFlags struct {
	debugger string
	cmds     stringList // -ex
	maps     stringList // -map
	batch    bool
	show     bool
	vscode   bool

	// hostOS is the system that runs the debugger (runtime.GOOS when empty): the Windows rules
	// for paths are used for "windows". A field, so that the tests can check them anywhere.
	hostOS string
}

// keepsDebugInfo reports whether the user's flags put the debug information of a -release build back.
// -release ends its compiles and its link with -g0, so both need a later -g flag (-g, -g1, -ggdb, ...)
// of the user's: the compiles from -cflags or -cxxflags (or GOCPP_CFLAGS, GOCPP_CXXFLAGS), the link
// from -ldflags (or GOCPP_LDFLAGS). With -flto the link writes the debug information, so a -g that
// only the compiles have leaves none.
func keepsDebugInfo(e *extraFlags) bool {
	env, err := build.ReadEnvFlags()
	if err != nil {
		return false // the build reports the bad value itself
	}
	compile := slices.Concat(env.CFlags, env.CXXFlags, e.cflags, e.cxxflags)
	link := slices.Concat(env.LDFlags, e.ldflags)
	return lastDebugLevelOn(compile) && lastDebugLevelOn(link)
}

// lastDebugLevelOn reports whether the last -g flag in flags asks for debug information (not -g0).
func lastDebugLevelOn(flags []string) bool {
	on := false
	for _, f := range flags {
		if strings.HasPrefix(f, "-g") && !strings.HasPrefix(f, "-gno-") {
			on = f != "-g0"
		}
	}
	return on
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
	f.extra.registerCompdb(flags)
	flags.StringVar(&df.debugger, "debugger", "", "the debugger: gdb or lldb (default gdb, or lldb when there is no gdb)")
	flags.Var(&df.cmds, "ex", "run this debugger command after the program is loaded (repeatable: -ex 'break hello.gp:12' -ex run)")
	flags.Var(&df.maps, "map", "FROM=TO: show the sources that the debug info names under FROM from TO (a program built with -trimpath, or moved)")
	flags.BoolVar(&df.batch, "batch", false, "run the -ex commands and exit, without a prompt (for scripts and tests)")
	flags.BoolVar(&df.show, "print", false, "print the debugger's command line and don't start it")
	flags.BoolVar(&df.vscode, "vscode", false,
		"print a launch.json for VS Code (the C/C++ extension with gdb, or lldb-dap with -debugger lldb) "+
			"and don't build or start anything")
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
	// A stripped program has nothing for a debugger to show. A -release program has no debug
	// information, unless the flags put it back (ADR-136): one line says so, and the session goes on.
	if f.extra.mode.Strip {
		fmt.Fprintln(stderr, "gocpp debug: -strip drops the symbols and the debug information that the debugger needs")
		return 2
	}
	if f.extra.mode.Release && !keepsDebugInfo(&f.extra) {
		if f.extra.mode.Trace {
			fmt.Fprintln(stderr, "gocpp debug: -release -trace builds have only the line tables, no variables (add -cflags=-g and -ldflags=-g to keep them)")
		} else {
			fmt.Fprintln(stderr, "gocpp debug: -release builds have no debug information (add -cflags=-g and -ldflags=-g to keep it)")
		}
	}
	args = flags.Args()
	if df.vscode {
		return printVSCode(&f, &df, args, stdout, stderr)
	}
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

// findTool finds a debugger program: in PATH, and on a Windows host in the folders of the known
// MinGW-w64 installs and gocpp's own llvm-mingw (toolchain.FindTool). A variable, for the tests.
var findTool = defaultFindTool

// defaultFindTool is findTool without a stub.
func defaultFindTool(name string) (path, source string, ok bool) {
	cfg, err := toolchain.FromEnv(false)
	if err != nil {
		cfg = toolchain.Config{}
	}
	return cfg.FindTool(name)
}

// defaultDebugger returns gdb, or lldb when only that is installed.
func defaultDebugger() string {
	if _, _, ok := findTool("gdb"); !ok {
		if _, _, ok := findTool("lldb"); ok {
			return "lldb"
		}
	}
	return "gdb"
}

// noDebuggerMessage says that the debugger is not installed, and on Windows where to get one.
func noDebuggerMessage(name, hostOS string) string {
	if hostOS != "windows" {
		return fmt.Sprintf("%s was not found in PATH (the nix dev shell has gdb and lldb; -debugger chooses)", name)
	}
	if name == "lldb" {
		return "lldb was not found in PATH, in the folders of MSYS2, WinGet, Scoop or Chocolatey, " +
			"or in gocpp's llvm-mingw " +
			"(install llvm-mingw, which has lldb: scoop install llvm-mingw, " +
			"or MSYS2: pacman -S mingw-w64-ucrt-x86_64-lldb; -debugger gdb uses gdb)"
	}
	return "gdb was not found in PATH, in the folders of MSYS2, WinGet, Scoop or Chocolatey " +
		"(install MinGW-w64 with gdb: winget install BrechtSanders.WinLibs.POSIX.UCRT, " +
		"or MSYS2: pacman -S mingw-w64-ucrt-x86_64-gdb; " +
		"-debugger lldb uses the lldb of llvm-mingw)"
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

// debuggerPath writes a path of this machine as the debugger's commands should name it: on
// Windows with slashes, which gdb and lldb read as well as backslashes, and which a command line of
// gdb does not mangle (gdb takes a backslash as the start of an escape).
func debuggerPath(hostOS, p string) string {
	if hostOS == "windows" {
		return strings.ReplaceAll(p, `\`, "/")
	}
	return p
}

// needsHandLoad says that gdb's safe path list can't name the program, so that gocpp debug loads
// the script by hand: the list is separated by colons (semicolons on Windows), so a program whose
// name has the separator is not on it.
func needsHandLoad(hostOS, exe string) bool {
	sep := ":"
	if hostOS == "windows" {
		sep = ";"
	}
	return strings.Contains(exe, sep)
}

// debugArgs returns the arguments of the debugger df.debugger for the program exe with the
// arguments args. script is the file with the debugger's script: the LLDB summaries, or for gdb
// (only used when exe is not a safe path name: needsHandLoad) gocpp-gdb.py.
func debugArgs(df *debugFlags, exe string, args []string, script string) []string {
	hostOS := df.hostOS
	if hostOS == "" {
		hostOS = runtime.GOOS
	}
	exe = debuggerPath(hostOS, exe)
	script = debuggerPath(hostOS, script)
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
		// The program carries gocpp's gdb script; gdb runs it for a safe path only. A safe path
		// is a list with a separator: a program whose name has it (TMPDIR with a colon) gets the
		// script by hand instead.
		if needsHandLoad(hostOS, exe) && script != "" {
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
	hostOS := df.hostOS
	if hostOS == "" {
		hostOS = runtime.GOOS
	}
	cross := !isHostTarget(d.check.goos, d.check.goarch)
	if cross && !df.show {
		// The debugger starts the program, which must run here: a Windows program on Linux (Wine
		// has no debugger interface that gdb can use) or the other way round does not.
		fmt.Fprintf(d.stderr, "gocpp debug: a program for %s/%s cannot be debugged on %s/%s: "+
			"run gocpp debug on a %s machine "+
			"(the program built there carries the same debug information and printers), "+
			"or use -print to see the command line\n",
			d.check.goos, d.check.goarch, runtime.GOOS, runtime.GOARCH, d.check.goos)
		return exitErrors
	}
	// The debugger is looked for first: a build that ends in "gdb was not found" is wasted time.
	var path string
	if !df.show {
		var source string
		var ok bool
		if path, source, ok = findTool(df.debugger); !ok {
			fmt.Fprintf(d.stderr, "gocpp debug: %s\n", noDebuggerMessage(df.debugger, hostOS))
			return exitErrors
		}
		if d.flags.x {
			fmt.Fprintf(d.stderr, "# %s: %s (%s)\n", df.debugger, path, source)
		}
	}
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
	case needsHandLoad(hostOS, debuggerPath(hostOS, p.exe)):
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
	err := cmd.Wait()
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
