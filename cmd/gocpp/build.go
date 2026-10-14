package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/types"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// The help texts of gocpp build, run and emit. "gocpp build -h" prints
// one, followed by the flags.
const (
	buildUsage = `usage: gocpp build [-o output] [build flags] [packages]

gocpp build compiles the package named by the arguments and its C++,
and writes the program. A package is a folder ("."), a list of .gp files
of one folder, an import path, or a pattern with "..." that matches one
package (until Phase 06, a program is one package: main). Without
arguments, it builds the package in the current folder.

The program is named after the package: after the first .gp file
(hello.gp gives hello), the folder of a package outside a module, or the
last element of the import path. -o names it, or with a folder (or a
name that ends in a slash) puts it there.

gocpp build first checks the packages like gocpp check: when there are
errors, it prints them the same way and exits with status 1, and nothing
is compiled. Then it writes the C++ (gocpp emit shows it) into a work
folder, compiles it with the C++ compiler and links it with gocpp's
runtime. The compiler is the one in GOCPP_CXX, else in CXX, else
clang++, else g++ (clang 17 or gcc 13, or newer); GOCPP_CXX and CXX can
hold arguments after the program (CXX="ccache g++"). The runtime is
extracted once into the user cache folder (gocpp/rt in $XDG_CACHE_HOME,
or in ~/.cache on Linux), and its object files are kept there.

The C++ that gocpp writes must always compile. If it doesn't, that is a
bug in gocpp: gocpp build prints "internal error: generated C++ did not
compile", the first errors of the compiler, which point at the .gp
lines, and the work folder, which it keeps; it exits with status 2.

The build flags (of build and run) are:

	-asan
		build with AddressSanitizer and UndefinedBehaviorSanitizer:
		the program stops at the first report, with exit status 1
	-nochecks
		build without the bounds and nil checks, for speed: an index
		or slice expression out of range, a slice to array conversion
		that doesn't fit and a nil pointer dereference are then
		undefined behavior, as in C++, instead of a panic. Division by
		zero and a negative shift count still panic, and the results
		that Go defines stay (MIN / -1, shifts by the width or more)
	-work
		print the name of the work folder and keep it
	-x
		print the commands

The flags are:
`
	runUsage = `usage: gocpp run [build flags] [package | file.gp...] [arguments...]

gocpp run builds the program like gocpp build (see "gocpp build -h"),
in a work folder, and runs it with the arguments. The program is a
folder ("."), an import path, or the .gp files at the start of the
arguments; the arguments after it go to the program. It runs in the
current folder with gocpp's standard input, output and error.

gocpp run exits like the program: with its exit status (a panic exits
with 2, like in Go), and when a signal kills the program, gocpp run
removes its work folder and dies of the same signal, so that a shell or
a test sees what it would see for the program itself. (go run instead
prints "exit status 3" or "signal: killed" and exits with 1.) On Linux
the signal is raised again; elsewhere gocpp exits with status 128 plus
the signal's number, like a shell. Ctrl-C (SIGINT) and SIGQUIT go to
the program from the terminal; gocpp run waits for it. gocpp run passes
SIGTERM and SIGHUP on to the program, removes its work folder at once
and waits for the program: a kill of gocpp alone stops the program too.
(When the whole process group gets the signal, the program gets it
twice.) When gocpp gets SIGINT, SIGTERM or SIGHUP while it builds, it
stops the compilers, removes the work folder and dies of that signal.

The flags are:
`
	emitUsage = `usage: gocpp emit [file.gp... | package]

gocpp emit checks the package like gocpp check and prints the C++ that
gocpp build compiles for it to standard output. The arguments are those
of gocpp build. Errors print like gocpp check's, with exit status 1; a
construct that the checker accepts but the emitter can't translate yet
is an internal error, with exit status 2.
`
)

// The exit statuses of build, run and emit: 1 for errors in the program,
// like the go command, and 2 for an internal error, a bug in gocpp. (go
// build exits with 1 for both; its compiler exits with 2 after an
// internal compiler error, so that a test can tell a bug from an error
// in the program.)
const (
	exitErrors   = 1
	exitInternal = 2
)

// testHookCPP, when it isn't nil, changes the C++ that the emitter wrote
// before it is compiled. Only the tests set it, to inject a fault (C++
// that doesn't compile) or a program that the emitter can't write yet.
var testHookCPP func(cpp []byte) []byte

// buildFlags are the flags of gocpp build and gocpp run.
type buildFlags struct {
	output   string // -o, build only
	x        bool   // -x
	work     bool   // -work
	asan     bool   // -asan
	nochecks bool   // -nochecks
}

func (f *buildFlags) register(flags *flag.FlagSet, withOutput bool) {
	if withOutput {
		flags.StringVar(&f.output, "o", "", "write the program to this file or folder")
	}
	flags.BoolVar(&f.asan, "asan", false, "build with AddressSanitizer and UndefinedBehaviorSanitizer")
	flags.BoolVar(&f.nochecks, "nochecks", false, "build without the bounds and nil checks (division and shift checks stay)")
	flags.BoolVar(&f.work, "work", false, "print the name of the work folder and keep it")
	flags.BoolVar(&f.x, "x", false, "print the commands")
}

// parseFlags parses the flags of the command name. done is true when the
// command must stop and return code: after -h (0) or a bad flag (2).
func parseFlags(name, usage string, flags *flag.FlagSet, args []string, stderr io.Writer) (code int, done bool) {
	flags.SetOutput(stderr)
	flags.Usage = func() {
		fmt.Fprint(stderr, usage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, true // like gocpp check -h
		}
		return 2, true // flag has printed the error and the usage
	}
	return 0, false
}

// runBuild is "gocpp build" (Phase 02 README §2.7).
func runBuild(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var f buildFlags
	flags := flag.NewFlagSet("gocpp build", flag.ContinueOnError)
	f.register(flags, true)
	if code, done := parseFlags("build", buildUsage, flags, args, stderr); done {
		return code
	}
	d, code := newDriver("build", &f, stdout, stderr)
	if d == nil {
		return code
	}
	defer d.stopSignals()
	return d.build(flags.Args())
}

// runRun is "gocpp run".
func runRun(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var f buildFlags
	flags := flag.NewFlagSet("gocpp run", flag.ContinueOnError)
	f.register(flags, false)
	if code, done := parseFlags("run", runUsage, flags, args, stderr); done {
		return code
	}
	// Like go run: the .gp files at the start are the program, else the
	// first argument is the package. The rest goes to the program.
	args = flags.Args()
	n := 0
	for n < len(args) && strings.HasSuffix(args[n], ".gp") {
		n++
	}
	switch {
	case n == 0 && (len(args) == 0 || strings.HasPrefix(args[0], "-")):
		fmt.Fprintln(stderr, "gocpp run: no .gp files listed")
		return exitErrors
	case n == 0:
		n = 1
	}
	d, code := newDriver("run", &f, stdout, stderr)
	if d == nil {
		return code
	}
	defer d.stopSignals()
	d.stdin = stdin
	return d.run(args[:n], args[n:])
}

// runEmit is "gocpp emit".
func runEmit(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gocpp emit", flag.ContinueOnError)
	if code, done := parseFlags("emit", emitUsage, flags, args, stderr); done {
		return code
	}
	d, code := newDriver("emit", &buildFlags{}, stdout, stderr)
	if d == nil {
		return code
	}
	defer d.stopSignals()
	pkg, code := d.load(flags.Args())
	if pkg == nil {
		return code
	}
	var b bytes.Buffer
	if code := d.emit(&b, pkg, ""); code != 0 {
		return code
	}
	if _, err := stdout.Write(b.Bytes()); err != nil {
		fmt.Fprintf(stderr, "gocpp emit: %v\n", err)
		return exitErrors
	}
	return 0
}

// A driver is one run of gocpp build, run or emit.
type driver struct {
	name   string // "build", "run", "emit"
	flags  *buildFlags
	check  *checkCommand
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer

	// files says that the package is a list of .gp files.
	files bool

	// ctx is canceled when gocpp gets SIGINT, SIGTERM or SIGHUP while it
	// builds; stopped is then the signal (see watchSignals).
	ctx     context.Context
	cancel  context.CancelFunc
	sigs    chan os.Signal
	mu      sync.Mutex
	stopped os.Signal
	running *exec.Cmd // the program, while gocpp run runs it
	work    string    // the work folder, while there is one
	removed bool      // the work folder was removed early (SIGTERM)
	done    chan struct{}
}

// notify is signal.Notify for the signals sigs that gocpp doesn't ignore
// already: one that gocpp's parent made it ignore (nohup ignores SIGHUP;
// sh ignores SIGINT for a command run with &) stays ignored, in gocpp and
// in the program that gocpp run starts, which inherits that.
func notify(c chan<- os.Signal, sigs ...os.Signal) {
	for _, sig := range sigs {
		if !signal.Ignored(sig) {
			signal.Notify(c, sig)
		}
	}
}

// stopSigs are the signals that stop a build. While gocpp run runs the
// program, SIGINT and SIGQUIT go to the program instead (from the
// terminal), and gocpp passes SIGTERM and SIGHUP on (see watchSignals).
var stopSigs = []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}

// newDriver returns the driver of the command name, or nil and the exit
// status when it can't start.
func newDriver(name string, f *buildFlags, stdout, stderr io.Writer) (*driver, int) {
	c := &checkCommand{stdout: stdout, stderr: stderr}
	var err error
	if c.goos, c.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err) // go exits with 2 too
		return nil, 2
	}
	if c.dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp %s: %v\n", name, err)
		return nil, exitErrors
	}
	d := &driver{name: name, flags: f, check: c, stdout: stdout, stderr: stderr, done: make(chan struct{})}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	// The checks (the namespace probe) stop like the build.
	c.ctx, c.stopped = d.ctx, d.interrupted
	d.sigs = make(chan os.Signal, 4)
	// SIGQUIT only from the start of the program (d.run): during the
	// build, Ctrl-\ dumps the goroutines and stops gocpp as usual.
	notify(d.sigs, stopSigs...)
	go d.watchSignals()
	return d, 0
}

// watchSignals handles the signals that gocpp gets while it builds and
// runs a program. While it builds, SIGINT, SIGTERM and SIGHUP stop the
// build: the compilers are killed (they run in process groups of their
// own, so they don't get the terminal's signals), the work folder is
// removed, and gocpp dies of the signal (stopStatus). While the program
// runs, like go run, gocpp ignores SIGINT and SIGQUIT, which the terminal
// sends to the program too; it passes SIGTERM and SIGHUP on to the
// program, which a kill of gocpp alone would not reach, and removes the
// work folder at once (Linux runs a program whose file is gone): a test
// harness that stops gocpp with SIGTERM and kills it soon after finds no
// work folder left behind. (A signal to the whole process group reaches
// the program twice then: gocpp can't tell cheaply where it came from.)
func (d *driver) watchSignals() {
	for {
		var sig os.Signal
		select {
		case sig = <-d.sigs:
		case <-d.done:
			return
		}
		d.mu.Lock()
		switch {
		case d.running != nil:
			if sig == syscall.SIGTERM || sig == syscall.SIGHUP {
				d.running.Process.Signal(sig)
				if d.work != "" && !d.flags.work && runtime.GOOS != "windows" {
					os.RemoveAll(d.work)
					d.removed = true
				}
			}
		case sig != syscall.SIGQUIT:
			if d.stopped == nil {
				d.stopped = sig
			}
			d.cancel()
		}
		d.mu.Unlock()
	}
}

// stopSignals ends the signal handling of the driver.
func (d *driver) stopSignals() {
	signal.Stop(d.sigs)
	close(d.done)
	d.cancel()
}

// stopStatus returns the exit status after the signal sig stopped the
// build or killed the program: 128 plus its number, like a shell. It
// records sig for main, which dies of it (exitSignal).
func stopStatus(sig os.Signal) int {
	s, ok := sig.(syscall.Signal)
	if !ok || s <= 0 {
		return exitErrors
	}
	exitSignal.Store(&s)
	return 128 + int(s)
}

// exitSignal is the signal that gocpp must die of after the command
// returned (see stopStatus), or nil. main raises it (dieOf); the tests,
// which call run, read it.
var exitSignal atomic.Pointer[syscall.Signal]

// interrupted returns the signal that stopped the build, or nil.
func (d *driver) interrupted() os.Signal {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stopped
}

// load checks the package of the patterns like gocpp check and returns it
// when it can be built: one main package, for the system that runs gocpp.
// Else it prints why and returns nil and the exit status.
func (d *driver) load(patterns []string) (*check.Package, int) {
	c := d.check
	// gocpp emit compiles nothing, so it writes the C++ for any target,
	// like gocpp check checks for any.
	if d.name != "emit" && (c.goos != runtime.GOOS || c.goarch != runtime.GOARCH) {
		fmt.Fprintf(d.stderr, "gocpp %s: building for %s/%s on %s/%s is not supported yet (Phase 06)\n",
			d.name, c.goos, c.goarch, runtime.GOOS, runtime.GOARCH)
		return nil, exitErrors
	}
	prog, r, code := c.check(patterns)
	d.files = c.files
	if code != 0 {
		return nil, code
	}
	var roots []*load.Package
	for _, p := range prog.Roots {
		if !c.dropped[p] {
			roots = append(roots, p)
		}
	}
	switch {
	case len(roots) == 0 && d.name == "build":
		return nil, 0 // like go build: check warned that a pattern matched nothing
	case len(roots) == 0:
		fmt.Fprintf(d.stderr, "gocpp %s: no packages loaded from %s\n", d.name, strings.Join(patterns, " "))
		return nil, exitErrors
	case len(roots) > 1:
		var paths []string
		for _, p := range roots {
			paths = append(paths, p.ImportPath)
		}
		fmt.Fprintf(d.stderr, "gocpp %s: the patterns match %d packages, and a program is one package until Phase 06:\n\t%s\n",
			d.name, len(roots), strings.Join(paths, "\n\t"))
		return nil, exitErrors
	}
	pkg := r.Package(roots[0])
	if pkg == nil || !pkg.Checked() {
		fmt.Fprintf(d.stderr, "gocpp %s: internal error: package %s was not checked\n", d.name, roots[0].ImportPath)
		return nil, exitInternal
	}
	if pkg.Types.Name() != "main" {
		fmt.Fprintf(d.stderr, "package %s is not a main package (until Phase 06, gocpp %s builds only programs)\n", pkg.Load.ImportPath, d.name)
		return nil, exitErrors
	}
	if _, ok := pkg.Types.Scope().Lookup("main").(*types.Func); !ok {
		// go build's message (from its linker).
		fmt.Fprintf(d.stderr, "# %s\nfunction main is undeclared in the main package\n", pkg.Load.ImportPath)
		return nil, exitErrors
	}
	return pkg, 0
}

// emit writes the C++ of pkg to w, with the output path out for the
// #line directives of the generated lines (cppgen.Config.OutputPath; ""
// for gocpp emit). It returns the exit status: an emitter error is an
// internal error, as the checker accepted the package.
func (d *driver) emit(w io.Writer, pkg *check.Package, out string) int {
	err := cppgen.EmitPackage(w, d.check.fset, pkg, cppgen.Config{OutputPath: out})
	if err == nil {
		return 0
	}
	var e *cppgen.Error
	if errors.As(err, &e) && e.Pos.IsValid() {
		fmt.Fprintf(d.stderr, "# %s\n%s:%d:%d: internal error: %s\n", pkg.Load.ImportPath,
			diag.ShortFileName(d.check.dir, e.Pos.Filename), e.Pos.Line, e.Pos.Column, e.Msg)
	} else {
		fmt.Fprintf(d.stderr, "# %s\ninternal error: %v\n", pkg.Load.ImportPath, err)
	}
	fmt.Fprintln(d.stderr, "(gocpp checked the package, but its C++ emitter can't write it: this is a bug in gocpp, or a feature that is not built yet)")
	return exitInternal
}

// programName returns the name of the program of pkg, without .exe: the
// first .gp file without .gp, the folder of a package outside a module,
// or the last element of the import path (without a major version suffix
// like v2), like go build.
func (d *driver) programName(pkg *check.Package) string {
	lp := pkg.Load
	if d.files && len(lp.Files) > 0 {
		return strings.TrimSuffix(filepath.Base(lp.Files[0].Path), ".gp")
	}
	if lp.ImportPath == "command-line-arguments" {
		return filepath.Base(lp.Dir)
	}
	elem := path.Base(lp.ImportPath)
	if majorSuffix.MatchString(elem) && strings.Contains(lp.ImportPath, "/") {
		elem = path.Base(path.Dir(lp.ImportPath))
	}
	return elem
}

var majorSuffix = regexp.MustCompile(`^v[2-9][0-9]*$|^v[1-9][0-9]+$`)

// exeSuffix ends the names of programs.
var exeSuffix = func() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}()

// A program is a built program in the work folder.
type program struct {
	work string // the work folder
	exe  string // the program file
}

// compile checks and builds the package of the patterns into a program
// in a new work folder. On failure it returns nil and the exit status, and
// it has removed the work folder (unless it keeps it: -work, or an
// internal error).
func (d *driver) compile(patterns []string) (*program, int) {
	pkg, code := d.load(patterns)
	if pkg == nil {
		return nil, code
	}
	cxx, err := build.FindCXXFor(d.ctx, d.check.goos, d.check.goarch) // the probe's compiler too
	if err != nil {
		if sig := d.interrupted(); sig != nil {
			return nil, stopStatus(sig)
		}
		fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
		return nil, exitErrors
	}
	work, err := os.MkdirTemp("", "gocpp-build")
	if err == nil {
		work, err = filepath.Abs(work) // the compilers run in it
	}
	if err != nil {
		fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
		return nil, exitErrors
	}
	d.mu.Lock()
	d.work = work
	d.mu.Unlock()
	if d.flags.x || d.flags.work {
		fmt.Fprintf(d.stderr, "WORK=%s\n", work)
	}
	keep := d.flags.work
	defer func() {
		if !keep {
			d.removeWork()
		}
	}()

	name := d.programName(pkg)
	cppName := name + ".cpp"
	if d.files && len(pkg.Load.Files) == 1 {
		cppName = filepath.Base(pkg.Load.Files[0].Path) + ".cpp" // hello.gp.cpp, as its first line says
	}
	src := filepath.Join(work, cppName)
	var cpp bytes.Buffer
	if code := d.emit(&cpp, pkg, src); code != 0 {
		return nil, code
	}
	data := cpp.Bytes()
	if testHookCPP != nil {
		data = testHookCPP(data)
	}
	if d.flags.x {
		fmt.Fprintf(d.stderr, "# gocpp emit > %s\n", build.ShellQuoteWork(src, work))
	}
	if err := os.WriteFile(src, data, 0o644); err != nil {
		fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
		return nil, exitErrors
	}

	b := &build.Builder{CXX: cxx, ASan: d.flags.asan, NoChecks: d.flags.nochecks, GOOS: d.check.goos, Work: work, Stderr: d.stderr}
	if cache, err := os.UserCacheDir(); err == nil {
		b.Cache = filepath.Join(cache, "gocpp")
	}
	if d.flags.x {
		b.Trace = d.stderr
	}
	exe := filepath.Join(work, name+exeSuffix)
	if err := b.Program(d.ctx, src, exe); err != nil {
		if sig := d.interrupted(); sig != nil {
			return nil, stopStatus(sig)
		}
		var ce *build.CompileError
		if errors.As(err, &ce) {
			keep = true
			d.compileFailed(pkg, ce, work)
			return nil, exitInternal
		}
		fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
		return nil, exitErrors
	}
	keep = true // the caller removes it
	return &program{work: work, exe: exe}, 0
}

// removeWork removes the work folder, if it is there and -work doesn't
// keep it.
func (d *driver) removeWork() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.work != "" && !d.flags.work && !d.removed {
		os.RemoveAll(d.work)
	}
	d.work = ""
}

// maxShownErrors is how many of the compiler's errors an internal error
// shows: like go build, which shows 10 errors per package.
const maxShownErrors = 10

// cxxError matches the first line of an error of gcc or clang:
// "file:line:col: error: message", or "fatal error:".
var cxxError = regexp.MustCompile(`^(.+?):(\d+):(\d+): (?:fatal )?error: `)

// compileFailed prints the internal error of a compile or link that
// failed (Phase 02 README §2.7, step 2.9): in Phase 02 the C++ is all
// gocpp's, so it is a bug. It shows the first errors of the compiler,
// which point at the .gp lines thanks to the #line directives, and keeps
// the work folder with the C++ and the compiler's whole output.
func (d *driver) compileFailed(pkg *check.Package, e *build.CompileError, work string) {
	log := "compile.log"
	verb := "did not compile"
	if e.Link {
		log, verb = "link.log", "did not link"
	}
	logPath := filepath.Join(work, log)
	os.WriteFile(logPath, append([]byte(e.Command+"\n"), e.Output...), 0o644)

	what := "generated C++"
	if e.What == "the runtime" {
		what = "gocpp's runtime"
	}
	fmt.Fprintf(d.stderr, "# %s\ninternal error: %s %s\n", pkg.Load.ImportPath, what, verb)
	var shown []string
	sc := bufio.NewScanner(bytes.NewReader(e.Output))
	sc.Buffer(nil, 1<<20)
	for sc.Scan() && len(shown) < maxShownErrors {
		line := sc.Text()
		if m := cxxError.FindStringSubmatch(line); m != nil {
			file := m[1]
			if strings.HasSuffix(file, ".gp") {
				file = diag.ShortFileName(d.check.dir, file)
			}
			shown = append(shown, file+line[len(m[1]):])
		}
	}
	if len(shown) == 0 {
		// A link error, or a message of another form: its first lines.
		for _, line := range strings.SplitN(strings.TrimSpace(string(e.Output)), "\n", maxShownErrors+1) {
			if len(shown) < maxShownErrors && line != "" {
				shown = append(shown, line)
			}
		}
	}
	if len(shown) == 0 {
		shown = append(shown, fmt.Sprintf("%s: %v (it printed nothing)", e.Command, e.Err))
	}
	for _, line := range shown {
		fmt.Fprintln(d.stderr, line)
	}
	fmt.Fprintf(d.stderr, "This is a bug in gocpp. The work folder is kept: %s\n(the C++ is there, and the compiler's whole output is in %s)\n", work, log)
}

// build is gocpp build: it builds the program and moves it to its place.
func (d *driver) build(patterns []string) int {
	p, code := d.compile(patterns)
	if p == nil {
		return code
	}
	defer d.removeWork()
	out := d.flags.output
	name := filepath.Base(p.exe)
	switch {
	case out == "":
		out = name
	case strings.HasSuffix(out, "/") || strings.HasSuffix(out, string(filepath.Separator)) || isDir(out):
		out = filepath.Join(out, name)
	}
	if !filepath.IsAbs(out) {
		out = filepath.Join(d.check.dir, out)
	}
	if d.flags.x {
		fmt.Fprintf(d.stderr, "mv %s %s\n", build.ShellQuoteWork(p.exe, p.work), build.ShellQuoteWork(out, ""))
	}
	if err := build.MoveFile(p.exe, out); err != nil {
		fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
		return exitErrors
	}
	return 0
}

// isDir reports whether name is a folder.
func isDir(name string) bool {
	fi, err := os.Stat(name)
	return err == nil && fi.IsDir()
}

// run is gocpp run: it builds the program, runs it with args and returns
// its exit status (see runUsage).
func (d *driver) run(patterns, args []string) int {
	p, code := d.compile(patterns)
	if p == nil {
		return code
	}
	defer d.removeWork()
	if d.flags.x {
		words := []string{build.ShellQuoteWork(p.exe, p.work)}
		for _, a := range args {
			words = append(words, build.ShellQuoteWork(a, ""))
		}
		fmt.Fprintln(d.stderr, strings.Join(words, " "))
	}
	newCmd := func() *exec.Cmd {
		cmd := exec.Command(p.exe, args...)
		cmd.Dir = d.check.dir
		cmd.Stdin, cmd.Stdout, cmd.Stderr = d.stdin, d.stdout, d.stderr
		return cmd
	}
	// The program stays in gocpp's process group, so that the terminal's
	// signals (Ctrl-C) and a kill of the group reach it, like go run.
	notify(d.sigs, syscall.SIGQUIT) // for the program, from the terminal
	if sig := d.interrupted(); sig != nil {
		return stopStatus(sig) // a signal came after the build
	}
	cmd, err := build.Start(d.ctx, newCmd) // again after ETXTBSY, like go run
	if err != nil {
		if sig := d.interrupted(); sig != nil {
			return stopStatus(sig)
		}
		fmt.Fprintf(d.stderr, "gocpp run: %v\n", err)
		return exitErrors
	}
	d.mu.Lock()
	d.running = cmd
	if d.stopped != nil {
		// The signal came while the program started, so it may have
		// missed it: pass it on; the program then dies of it, and gocpp
		// too (below).
		if cmd.Process.Signal(d.stopped) != nil {
			cmd.Process.Kill()
		}
	}
	d.mu.Unlock()
	err = cmd.Wait()
	d.mu.Lock()
	d.running = nil
	d.mu.Unlock()
	state := cmd.ProcessState
	if state == nil {
		fmt.Fprintf(d.stderr, "gocpp run: %v\n", err)
		return exitErrors
	}
	if ws, ok := state.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
		return stopStatus(ws.Signal()) // gocpp dies of it too, after the cleanup
	}
	if err != nil && !errors.As(err, new(*exec.ExitError)) {
		// The program exited, but copying its output failed.
		fmt.Fprintf(d.stderr, "gocpp run: %v\n", err)
		if state.ExitCode() == 0 {
			return exitErrors
		}
	}
	return state.ExitCode()
}
