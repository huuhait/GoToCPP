package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"go/types"
	"io"
	"maps"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/fsretry"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lower"
)

// The help texts of gocpp build, run and emit. "gocpp build -h" prints
// one, followed by the flags.
const (
	buildUsage = `usage: gocpp build [-o output] [build flags] [packages]

gocpp build compiles the main package named by the arguments, the packages
it imports and their C++, and writes the program. A package is a folder
("."), a list of .gp files of one folder, an import path, or a pattern with
"..." (of the matched packages, one must be a main package: it is the
program, and the others are checked, and built when it imports them).
Without arguments, it builds the package in the current folder.

The program is named after the package: after the first .gp file
(hello.gp gives hello), the folder of a package outside a module, or the
last element of the import path. -o names it, or with a folder (or a
name that ends in a slash) puts it there.

-os and -arch (or GOCPP_OS and GOCPP_ARCH, else GOOS and GOARCH) build for
another system: windows/amd64 (a .exe, with MinGW-w64 or a toolchain that
gocpp downloads: GOCPP_TOOLCHAIN) and linux/arm64. gocpp run needs
GOCPP_EXEC (for example wine) to run such a program.

gocpp build first checks the packages like gocpp check: when there are
errors, it prints them the same way and exits with status 1, and nothing
is compiled. Then it writes the C++ (gocpp emit shows it: for each package
a header, a file for each .gp file, pkg_desc.cpp and pkg_init.cpp) into a
folder of the cache (gen/<hash> in the cache folder of gocpp, the same for
every build of the program; -work puts it in the work folder), compiles
every file with the C++ compiler, at the same time, and links the objects
with gocpp's runtime. The compiler is the one in GOCPP_CXX, else in CXX, else
clang++, else g++ (clang 17 or gcc 13, or newer); GOCPP_CXX and CXX can
hold arguments after the program (CXX="ccache g++"). The runtime is
extracted once into the user cache folder (gocpp/rt in $XDG_CACHE_HOME,
or in ~/.cache on Linux), and its object files are kept there. So are the
C++ compiler's answers about C/C++ headers (gocpp/probe), as long as the
headers, flags and compiler are the same; GOCPP_NOPROBECACHE=1 turns that
off.

The object files that the C++ compiler makes from the generated C++ and
from the package's C and C++ files are kept there too (gocpp/obj), named
by a hash of everything the compile read: the C++, the flags, the
compiler and every header it included. A build in which none of that
changed doesn't compile again, it only links. A compile that printed a
warning or failed is never kept, so the warning comes every time, and
neither is one with flags that read files the cache can't see (@file,
-march=native, -save-temps), with __DATE__ or __has_include, or in a
package with a very large number of headers in many folders; -x prints
"# not cached: <reason>" for them. -work doesn't use it. GOCPP_NOOBJCACHE (any value except empty, 0 and false)
turns it off, and GOCPP_OBJCACHE_SIZE (bytes, or with a K, M or G suffix;
2G by default) limits its size: the oldest files go first.

gocpp build also remembers each program it built (in the cache folder): the files that the
loader read, the headers the C++ compiler read, the files the linker opened, pkg-config's answers,
the compilers and the environment, and the program itself. The next gocpp build with the same
arguments checks that record first, and when none of it changed it prints nothing and exits with
status 0, without loading the packages (a few milliseconds). -x says why a build is not up to date
or not recorded. A build that printed a warning is never recorded, so the warning comes every time.
GOCPP_NOUPTODATE=1 turns the check off. gocpp clean -cache removes the cache. See docs/spec/build.md.

The C and C++ flags of a package come from its //gocpp:cflags, ldflags,
link and pkg-config lines, which an allow-list like go build's checks
(GOCPP_CFLAGS_ALLOW and GOCPP_LDFLAGS_ALLOW allow more flags), and from
GOCPP_CFLAGS and GOCPP_CXXFLAGS (every compile) and GOCPP_LDFLAGS (the
link), which are trusted and not checked. PKG_CONFIG names pkg-config.

The C and C++ files in the folder of the package (.c, and .cc, .cpp,
.cxx), with the build constraints of .gp files, are compiled and linked
into the program too: the .c files with -std=gnu17 and the C compiler in
GOCPP_CC, else the one that goes with the C++ compiler (gcc for g++,
clang for clang++), the others with -std=c++20 and the C++ compiler. An
error in them is an error of the program: exit status 1.

The C and C++ files of a library module (a header import such as
"github.com/ocornut/imgui/imgui.h") are built from the "cxx" recipe in
gocpp.mod (cxx <module path> { src ... include ... define ... }), or the one
the library ships in its own gocpp.mod: each file is compiled once into the
object cache, for every project that has the same recipe, compiler and
flags, and linked in. The recipe's flags go through the same allow-list as
the package's. A library that does not compile is an error of the build:
the message names the module and the recipe line, and -x shows the command.

In a package that uses C or C++ (a header import, or C), an error of
the C++ compiler that comes from a .gp line with C or C++ code (a use of
C, of a header package or a namespace, or of a C/C++ value; a header
import) is an error of the program:
a C function called with the wrong arguments, a C++ template that
doesn't accept a type. gocpp build prints it at the .gp line, like gocpp
check's errors, with "C++: ", the compiler's first message and the
source line, and exits with status 1. The notes and template backtraces
are left out; -cpperrors=full shows the compiler's whole output. A
function that the headers declare but that nothing in the program
defines (a missing library or C file) is such an error too, at the .gp
line that uses it.

Any other C++ that gocpp writes must compile. If it doesn't, that is a
bug in gocpp: gocpp build prints "internal error: generated C++ did not
compile", the first errors of the compiler, which point at the .gp
lines, and the work folder, which it keeps; it exits with status 2.

The build flags (of build and run; -emit-llvm, -n and -o are of build only) are:

	-a
		compile everything again: the object cache and the up-to-date
		check are not used (the new objects are stored)
	-arch arch
		the target architecture (GOARCH): amd64 or arm64; default
		GOCPP_ARCH, GOARCH or this system's
	-asan
		build with AddressSanitizer and UndefinedBehaviorSanitizer:
		the program stops at the first report, with exit status 1
	-buildmode mode
		what to build: exe (a program, the default), c-archive, c-shared
		or cpp-lib (libraries; see docs/spec/build.md). Only gocpp build
	-cflags flags
		trusted flags for every C and C++ compile, like GOCPP_CFLAGS
		(see -cxxflags)
	-color mode
		color the diagnostics: auto (on a terminal), always or never
		(GOCPP_COLOR, NO_COLOR and CLICOLOR_FORCE also count)
	-compdb[=path]
		write compile_commands.json, the JSON compilation database of
		clangd and other tools, with the exact command of every
		translation unit of the program (the generated C++, the
		package's C and C++ files, the libraries of modules, the
		runtime). A debug build (the default, -debug, -race) writes it
		without this flag, so that clangd and the debugger find the
		generated C++: gocpp build to the folder of the generated C++
		and to the folder of the module (gocpp.mod), a native build of
		a program in a module only (with several programs it holds the
		last one built); gocpp run, test and debug to the folder of the
		generated C++. Nothing is written when there is no cache folder
		(the build is in a folder that is removed). The automatic write
		never replaces a symbolic link or a file that gocpp did not
		write (it says so); a failure to write it is a warning. A
		-release build and -n write it only with this flag. -compdb
		goes to the two places even for -release and replaces a file; a
		symbolic link is replaced by the file, never written through
		(to fill the file behind a link: gocpp export compdb -o path).
		-compdb=path goes to that file or folder, -compdb=false turns
		the automatic write off (a file that is there stays). gocpp
		export compdb does only this
	-cpperrors mode
		short (the default) or full: full shows the whole output of
		the C++ compiler or the linker when the program's C++ doesn't
		compile or link
	-cxxflags flags
		trusted flags for every C++ compile, the runtime's too, like
		GOCPP_CXXFLAGS: not checked, and added after those of the
		environment. A quote keeps spaces only at the start of a word
		("'-DA=a b'"); the flag may be given again.
		They may load compiler plugins that you chose: LLVM pass
		plugins (-cxxflags=-fpass-plugin=/path/p.so, clang) or front-end
		plugins (-fplugin=/path/p.so, g++ and clang); a //gocpp:cflags
		line never may (its allow-list refuses them). The plugin file is
		part of the object cache's key and of the up-to-date check, so a
		new build of it compiles again; so are the files of @file,
		-include and -imacros
	-debug
		build with -O0 -g, which is what a build is without a flag
	-emit-llvm[=ll|bc]
		also write the LLVM IR of every translation unit, text (ll, the
		default) or bitcode (bc), into <program>.llvm next to the
		program, for opt -load-pass-plugin=... and llvm-dis. Only with
		clang (GOCPP_CXX=clang++): with g++ it is an error. The folder
		is emptied by the next build (it has a .gocpp-llvm marker)
	-excerpt mode
		show the source line and a caret under each error: auto (on a
		terminal), always or never (GOCPP_EXCERPT)
	-gp:concepts
		also write C++20 concepts for Go's constraints into the package
		headers (gp_concept_<Name> for each named interface type,
		gp_sat_<Func> for each generic function), for C++ code that works
		with the package; the generated code does not use them
	-gp:m[=2]
		print the decisions of the escape analysis, like go build
		-gcflags=-m: what stays on the C++ stack ("&x does not escape"),
		what is moved to the heap, the parameters that leak, the slice
		elements that are borrowed and the func literals that do not
		escape. -gp:m=2 adds the reason of each decision. The standard
		library is not reported
	-gp:noescape
		turn the escape analysis off (the baseline for comparisons): a
		variable whose address is taken, &T{} and new(T) are in heap
		boxes, closures capture by value or in a box, and a slice
		element is copied
	-ldflags flags
		trusted flags for the link, like GOCPP_LDFLAGS: added after the
		package's link flags, like -cxxflags otherwise
	-n
		print the commands that the build would run, and run none of
		them (the generated C++ is written, nothing is compiled or
		linked, no program is written)
	-interop mode
		how C/C++ names are understood: typed (clang++ answers their
		types), opaque (the C++ compiler decides) or auto (typed when
		clang++ is installed, else opaque); default GOCPP_INTEROP, else
		auto
	-lto
		link-time optimization: the runtime and the generated C++
		compile with -flto (-flto=auto for g++, -flto=thin for clang++)
		and the link optimizes across them. The runtime's library is
		made with the archiver that belongs to the compiler (gcc-ar,
		llvm-ar, or ar with the compiler's plugin; GOCPP_AR names
		one); the build stops when there is none. Combine it with
		an -O level (GOCPP_CXXFLAGS=-O2)
	-nochecks
		build without the bounds and nil checks, for speed: an index
		or slice expression out of range, a slice to array conversion
		that doesn't fit and a nil pointer dereference are then
		undefined behavior, as in C++, instead of a panic. Division by
		zero and a negative shift count still panic, and the results
		that Go defines stay (MIN / -1, shifts by the width or more)
	-o file
		write the program to this file, or into this folder (a name that
		ends in a slash, or a folder that exists)
	-os os
		the target operating system (GOOS): linux or windows; default
		GOCPP_OS, GOOS or this system's
	-p n
		run at most n compiles at the same time (GOMAXPROCS by default,
		like go build). The compiles start in the order of the packages;
		a compile that fails does not stop the others, so one build
		shows all the errors it can (after 20 failures no new compile
		starts), and nothing is linked. With -p 1 the compiles run one
		after the other. The messages of the compiles that worked
		(warnings) come in the same order whatever the speed.
	-race
		build with ThreadSanitizer (-fsanitize=thread): the program
		reports a data race it sees, on linux in Go's form ("WARNING:
		DATA RACE") with exit code 66; GORACE sets the options
		(halt_on_error=1, exitcode=N, log_path=file, history_size=N)
		(docs/guides/race.md). Not with -asan or -release, and not for
		windows
	-release
		build with -O2 -flto
	-tags list
		more build tags that are satisfied, separated by commas, like
		go build -tags: for //go:build lines and file names of .gp, C
		and C++ files. The last -tags is the list
	-v
		print the import paths of the packages that are built
	-work
		print the name of the work folder and keep it (the object cache
		is not used, so that its files have real paths in them)
	-x
		print the commands, a line for each compile that the object cache
		answered, why a build was not up to date, and at the end the
		counts of what the build did ("# actions: ...")

` + interopHelp + `The flags are:
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

` + interopHelp + `The flags are:
`
	emitUsage = `usage: gocpp emit [-gp:concepts] [-tags list] [-annotate [-json] [-nochecks] [-kinds k1,k2] [-std] [-o file]] [file.gp... | package]

gocpp emit checks the package like gocpp check and prints the C++ that
gocpp build compiles for it to standard output. The arguments are those
of gocpp build. Errors print like gocpp check's, with exit status 1; a
construct that the checker accepts but the emitter can't translate yet
is an internal error, with exit status 2.

With -annotate the output is for reading: the C++ with the .gp line that
each part comes from and the reasons for the lowering's choices (gocpp
explain TOPIC says what a mechanism is). See docs/guides/explain.md.

` + interopHelp + `The flags are:
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
	exitUsage    = 2 // a flag that can't be used here (-emit-llvm with g++): like a flag that does not parse

	// exitNoTestFiles is what load returns for gocpp test when the package has no test files
	// (nothing is printed: gocpp test says "[no test files]").
	exitNoTestFiles = -1
)

// testHookCPP, when it isn't nil, changes the C++ that the emitter wrote
// before it is compiled: it is called for each generated file (a header too)
// with its path relative to the folder of the generated files, and returns the
// bytes to write. Only the tests set it, to inject a fault (C++ that doesn't
// compile) or a program that the emitter can't write yet.
var testHookCPP func(path string, cpp []byte) []byte

// buildFlags are the flags of gocpp build and gocpp run.
type buildFlags struct {
	output   string // -o, build only
	x        bool   // -x
	work     bool   // -work
	asan     bool   // -asan
	nochecks bool   // -nochecks
	lto      bool   // -lto
	jobs     int    // -p, 0 for the default (GOMAXPROCS)
	concepts bool   // -gp:concepts

	escape   escapeLevel // -gp:m: print the escape analysis (escape.go)
	noEscape bool        // -gp:noescape

	mode build.Mode // -buildmode, build only (modes.go); "" is a program

	cppErrors cppErrorsMode // -cpperrors: "short" or "full"
	diagOpts  diagOptions   // -color, -excerpt

	extra extraFlags // the flags of step 6.12 (flags.go)
	cross crossFlags // -os and -arch (cross.go)

	interop interopFlag // -interop (interop.go)
}

func (f *buildFlags) register(flags *flag.FlagSet, withOutput bool) {
	if withOutput {
		flags.StringVar(&f.output, "o", "", "write the program to this file or folder")
	}
	f.cppErrors = "short"
	f.diagOpts.register(flags)
	f.cross.register(flags)
	f.interop.register(flags)
	flags.Var(&f.cppErrors, "cpperrors", "how much of the C++ compiler's errors to show: short or full")
	flags.BoolVar(&f.asan, "asan", false, "build with AddressSanitizer and UndefinedBehaviorSanitizer")
	flags.BoolVar(&f.lto, "lto", false, "link-time optimization (-flto; the runtime library gets the compiler's archiver)")
	flags.BoolVar(&f.nochecks, "nochecks", false, "build without the bounds and nil checks (division and shift checks stay)")
	flags.Func("p", "the number of compiles that run at the same time (default GOMAXPROCS)", func(v string) error {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 {
			return errors.New("must be a positive number")
		}
		f.jobs = n
		return nil
	})
	flags.BoolVar(&f.concepts, "gp:concepts", false, "also write C++20 concepts for Go's constraints into the package headers")
	f.registerEscape(flags)
	if withOutput {
		f.registerMode(flags)
	}
	flags.BoolVar(&f.work, "work", false, "print the name of the work folder and keep it (without the object cache)")
	flags.BoolVar(&f.x, "x", false, "print the commands")
	f.extra.register(flags, withOutput)
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
	f.extra.registerCompdb(flags)
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
	var ef buildFlags
	ef.cross.register(flags)
	ef.interop.register(flags)
	ef.diagOpts.register(flags)
	flags.BoolVar(&ef.concepts, "gp:concepts", false, "also write C++20 concepts for Go's constraints into the package headers")
	ef.registerEscape(flags)
	var ann annotateFlags
	ann.register(flags) // annotate.go
	flags.Var(tagsFlag{&ef.extra.tags}, "tags", "a comma-separated `list` of build tags that are satisfied, like go build -tags")
	if code, done := parseFlags("emit", emitUsage, flags, args, stderr); done {
		return code
	}
	if code, bad := ann.validate(stderr); bad {
		return code
	}
	d, code := newDriver("emit", &ef, stdout, stderr)
	if d == nil {
		return code
	}
	defer d.stopSignals()
	set, code := d.load(flags.Args())
	if set == nil {
		return code
	}
	if ann.on {
		return ann.run(d, set, stdout, stderr)
	}
	files, code := d.emit(set, "", nil)
	if files == nil {
		return code
	}
	var b bytes.Buffer
	for _, f := range files.Files {
		fmt.Fprintf(&b, "//// %s\n", f.Path)
		b.Write(f.Data)
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

	// why, when it is not nil, collects the reasons for the lowering's choices (gocpp emit
	// -annotate, annotate.go).
	why *lower.Why

	// pkgLine says that the "# <package>" line of the output of the
	// package's C and C++ files is printed (sourcesHeader).
	pkgLine bool

	// ctx is canceled when gocpp gets SIGINT, SIGTERM or SIGHUP while it
	// builds; stopped is then the signal (see watchSignals).
	ctx     context.Context
	cancel  context.CancelFunc
	sigs    chan os.Signal
	mu      sync.Mutex
	stopped os.Signal
	running *exec.Cmd // the program, while gocpp run runs it
	// runningGroup says that the program runs in a process group of its own (gocpp test): then the
	// terminal's signals do not reach it, and watchSignals passes SIGINT on too, to the group.
	runningGroup  bool
	groupSignaled bool   // a signal was passed on to the group: execTest kills what it leaves
	work          string // the work folder, while there is one
	gen           string // the folder of the generated C++ of the program that is built
	removed       bool   // the work folder was removed early (SIGTERM)

	lib      *libState // the library that a library mode builds (modes.go)
	libRoots []string  // the import paths of the packages of a cpp-lib

	env   build.EnvFlags // the trusted flags of the build (flags.go: userEnv), when envOK
	envOK bool

	// afterPlan, if not nil, is called by compile with the plan of the build when its commands are
	// known (and run, or listed with -n): gocpp export uses it to write its files.
	afterPlan func(set *buildSet, plan *build.Plan, name string) error

	// What a debug build wrote by itself (flags.go: compdbAuto): compdbWritten is the copy in the
	// module's folder, which is no change of the package for the up-to-date check; compdbOwned is the
	// copy in the folder of the generated files of the program, an output that the check looks after
	// (the module's copy holds the commands of the last program built, so it can't be one).
	compdbWritten []string
	compdbOwned   []string

	stamp         *stampState // the up-to-date check of this build, or nil (uptodate.go)
	upToDateBuild bool        // the build found its program current
	noStamp       string      // why there is none

	test *testRun // gocpp test: what the generated main package found (nil for the other commands)

	stats  build.Stats // what the compiles and the link did (printed by -x)
	counts struct {    // what the front end did (printed by -x)
		load  int // loads of the program (a build that is up to date has none)
		check int // packages type-checked
		emit  int // packages emitted
		write int // generated files written (their bytes changed)
	}

	// moreFailures says that failures of the same build are printed after this one, so that the
	// message about the work folder comes once, after the last (buildFailed, compileFailed).
	moreFailures bool

	// lastShown and lastShownPkg are the errors that compileFailed printed last, and their package.
	lastShown    []string
	lastShownPkg string

	logCount int // the logs of failed compiles written so far (compileFailed)
	done     chan struct{}
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
	c := &checkCommand{stdout: stdout, stderr: stderr, kinds: true, diagOpts: f.diagOpts, interop: f.interop, verbose: f.extra.verbose}
	var err error
	if c.goos, c.goarch, err = f.cross.resolve(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err) // go exits with 2 too
		return nil, 2
	}
	if c.dir, err = getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp %s: %v\n", name, err)
		return nil, exitErrors
	}
	c.tags = f.extra.tags
	if _, _, err := build.ProfileFlags(f.extra.mode, f.asan, c.goos); err != nil {
		fmt.Fprintf(stderr, "gocpp %s: %v\n", name, err)
		return nil, 2
	}
	d := &driver{name: name, flags: f, check: c, stdout: stdout, stderr: stderr, done: make(chan struct{})}
	// The trusted flags of the build: the environment's with the mode's and the build flags'
	// (flags.go). A value of the environment that does not parse is reported by compile, as before.
	if env, err := f.extra.userEnv(f.asan, c.goos, c.dir); err == nil {
		d.env, d.envOK = env, true
		c.env = &d.env
	}
	build.SetDetectCache(cacheDir()) // the compilers are detected once, not in every run
	if f.x {
		// -x shows the runs of pkg-config too, as go build -x does.
		c.pkgConfig = build.TracePkgConfig(func(line string) { fmt.Fprintln(stderr, line) })
	}
	d.ctx, d.cancel = context.WithCancel(context.Background())
	if name == "build" || name == "run" || name == "debug" {
		// Only a build (gocpp build, run and debug) may download the Windows toolchain (not check, emit or the server).
		d.ctx = build.WithToolchainDownload(d.ctx, stderr)
	}
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
			s, _ := sig.(syscall.Signal)
			switch {
			case (sig == syscall.SIGINT || sig == syscall.SIGQUIT) && d.runningGroup:
				// The terminal sent it to gocpp only: the test binary and the processes of its
				// group get it (execTest kills what is left when the binary is gone).
				d.groupSignaled = true
				signalTest(d.running, s, true)
			case sig == syscall.SIGTERM || sig == syscall.SIGHUP:
				if d.runningGroup {
					d.groupSignaled = true
					signalTest(d.running, s, true)
				} else {
					d.running.Process.Signal(sig)
				}
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

// A buildSet is the program that gocpp build, run or emit makes: the main package and the
// packages it imports, directly or not, that gocpp writes C++ for.
type buildSet struct {
	main *check.Package

	// pkgs are the packages with C++ of their own, in the order in which Go initializes them:
	// each one after the packages it imports, the main package last (load.Program.Packages).
	// The packages whose functions the runtime has (fmt, weak) are not in it.
	pkgs []*check.Package
}

// byPath returns the package with the import path path, or the main package.
func (s *buildSet) byPath(path string) *check.Package {
	for _, p := range s.pkgs {
		if p.Load.ImportPath == path {
			return p
		}
	}
	return s.main
}

// shimPackages are the standard packages that gocpp lowers to the runtime's own functions, not to
// C++ of their own: their uses are written by the lowering of the package that imports them.
var shimPackages = map[string]bool{"fmt": true, "weak": true}

// notLowered reports whether the package lp gets no C++ of its own: a shim (shimPackages), or a
// standard package that is still a declaration stub (Phase 01: functions without a body; the real
// packages come with Phase 09). A standard package with bodies is built like any other package,
// so a package of stdlib/ written in .gp can be imported like one of the project's. A function
// without a body that has a //gocpp:native directive is a native function (internal/load/native.go):
// the C++ function of the runtime that it names; it does not make the package a stub.
func notLowered(lp *load.Package) bool {
	return lp.Std && (shimPackages[lp.ImportPath] || lp.IsStub())
}

// load checks the packages of the patterns like gocpp check and returns the program when it can
// be built: one main package and its imports, for the system that runs gocpp. Else it prints
// why and returns nil and the exit status.
func (d *driver) load(patterns []string) (*buildSet, int) {
	c := d.check
	// gocpp emit compiles nothing, so it writes the C++ for any target,
	// like gocpp check checks for any.
	if d.name != "emit" {
		err := buildTarget(c.goos, c.goarch)
		if err != nil {
			fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
			return nil, exitErrors
		}
	}
	prog, r, code := c.check(patterns)
	d.files = c.files
	if code != 0 {
		return nil, code
	}
	if d.test != nil && d.test.noTestFiles {
		return nil, exitNoTestFiles
	}
	var roots []*load.Package
	for _, p := range prog.Roots {
		if !c.dropped[p] {
			roots = append(roots, p)
		}
	}
	// The program is the one main package among the roots; the other roots are packages of the
	// same program (gocpp build ./... checks all of them), built only when the main package
	// imports them.
	var mains []*load.Package
	for _, p := range roots {
		if d.test != nil {
			// gocpp test: the program is the generated main package of the test binary (the
			// package under test may be a main package too).
			if p.ForTest != "" && p.ImportPath == p.ForTest+".test" {
				mains = append(mains, p)
			}
		} else if p.Name == "main" {
			mains = append(mains, p)
		}
	}
	cppLib := d.flags.mode == build.ModeCPPLib
	if cppLib {
		mains = d.libMains(prog, roots) // a library has no main package: its roots, the last one is the entry
		d.libRoots = nil
		for _, p := range mains {
			d.libRoots = append(d.libRoots, p.ImportPath)
		}
	}
	switch {
	case len(roots) == 0 && d.name == "build":
		return nil, 0 // like go build: check warned that a pattern matched nothing
	case len(roots) == 0:
		fmt.Fprintf(d.stderr, "gocpp %s: no packages loaded from %s\n", d.name, strings.Join(patterns, " "))
		return nil, exitErrors
	case cppLib:
	case len(mains) == 0 && len(roots) == 1:
		fmt.Fprintf(d.stderr, "package %s is not a main package (gocpp %s builds programs)\n", roots[0].ImportPath, d.name)
		return nil, exitErrors
	case len(mains) == 0:
		fmt.Fprintf(d.stderr, "gocpp %s: the patterns match %d packages and none of them is a main package\n", d.name, len(roots))
		return nil, exitErrors
	case len(mains) > 1:
		var paths []string
		for _, p := range mains {
			paths = append(paths, p.ImportPath)
		}
		fmt.Fprintf(d.stderr, "gocpp %s: the patterns match %d main packages, and a program is one:\n\t%s\n",
			d.name, len(mains), strings.Join(paths, "\n\t"))
		return nil, exitErrors
	}
	entry := mains[0]
	if cppLib {
		entry = mains[len(mains)-1]
	}
	mainPkg := r.Package(entry)
	if mainPkg == nil || !mainPkg.Checked() {
		fmt.Fprintf(d.stderr, "gocpp %s: internal error: package %s was not checked\n", d.name, entry.ImportPath)
		return nil, exitInternal
	}
	if _, ok := mainPkg.Types.Scope().Lookup("main").(*types.Func); !ok && !cppLib {
		// go build's message (from its linker).
		fmt.Fprintf(d.stderr, "# %s\nfunction main is undeclared in the main package\n", mainPkg.Load.ImportPath)
		return nil, exitErrors
	}
	// The packages of the program: what the main package imports, in the order of Packages.
	reach := make(map[*load.Package]bool)
	var visit func(p *load.Package)
	visit = func(p *load.Package) {
		if reach[p] {
			return
		}
		reach[p] = true
		if notLowered(p) {
			return // the runtime has its functions, or the package is a stub: what it imports is not needed
		}
		for _, dep := range p.Imports {
			visit(dep)
		}
		// fmt is the runtime's, but its Fprint and Fscan functions take an io.Writer or io.Reader, and
		// Go's fmt imports io: a program that calls them has the package io, also when it does not
		// import it (a type with a Write method is converted to io.Writer at the call).
		if cp := r.Package(p); cp != nil && usesFmtIO(cp) {
			for _, dep := range prog.Packages {
				if dep.Std && dep.ImportPath == "io" {
					if p.Imports["io"] == nil {
						p.Imports["io"] = dep // as if the package imported it: its header and its init
					}
					visit(dep)
				}
			}
		}
	}
	if cppLib {
		for _, p := range mains {
			visit(p)
		}
	} else {
		visit(mains[0])
	}
	d.counts.load++
	d.counts.check += len(prog.Packages)
	set := &buildSet{main: mainPkg}
	for _, lp := range prog.Packages {
		if !reach[lp] || notLowered(lp) {
			continue
		}
		cp := r.Package(lp)
		if cp == nil || !cp.Checked() {
			fmt.Fprintf(d.stderr, "gocpp %s: internal error: package %s was not checked\n", d.name, lp.ImportPath)
			return nil, exitInternal
		}
		set.pkgs = append(set.pkgs, cp)
	}
	return set, 0
}

// emit writes the C++ of the program: the files of its packages, with root, the folder that
// they will be written in, for the #line directives of the generated lines (cppgen's EmitProgram;
// "" for gocpp emit). cxx is the C++ compiler of the build, for the probes of the packages
// with header imports, or nil (gocpp emit finds it then). It returns the files, or nil and the
// exit status: an emitter error is an internal error, as the checker accepted the packages,
// except for the errors of C/C++ names that only the compiler's probes find
// (cppgen.Error.User).
func (d *driver) emit(set *buildSet, root string, cxx *build.CXX) (*cppgen.ProgramFiles, int) {
	in := make([]cppgen.ProgramPackage, len(set.pkgs))
	for i, pkg := range set.pkgs {
		interop, code := d.probe(pkg, cxx)
		if code != 0 {
			return nil, code
		}
		in[i] = cppgen.ProgramPackage{Pkg: pkg, Interop: interop}
	}
	escape := d.flags.newEscapeLog()
	files, err := cppgen.EmitProgram(d.check.fset, in, root, cppgen.Config{Concepts: d.flags.concepts, Library: d.flags.mode.Library(), Why: d.why, NoEscape: d.flags.noEscape, Escape: escape})
	if err == nil {
		printEscape(d.stderr, escape)
		d.counts.emit += len(set.pkgs)
		return files, 0
	}
	return nil, d.emitFailed(set, err)
}

// emitFailed prints the error err of the emitter and returns the exit status (see emit).
func (d *driver) emitFailed(set *buildSet, err error) int {
	var e *cppgen.Error
	pkg := set.main
	if errors.As(err, &e) && e.Pos.IsValid() {
		// The package of the place of the error.
		for _, p := range set.pkgs {
			if slices.ContainsFunc(p.Load.Files, func(f *load.File) bool { return f.Path == e.Pos.Filename }) {
				pkg = p
			}
		}
	}
	if errors.As(err, &e) && e.User && e.Pos.IsValid() {
		fmt.Fprintf(d.stderr, "# %s\n%s:%d:%d: %s\n", pkg.Load.ImportPath,
			diag.ShortFileName(d.check.dir, e.Pos.Filename), e.Pos.Line, e.Pos.Column, e.Msg)
		return exitErrors
	}
	// The lowering writes some of its messages with "internal error: "
	// already (a state that must not happen): the prefix comes once.
	switch {
	case errors.As(err, &e) && e.Pos.IsValid():
		fmt.Fprintf(d.stderr, "# %s\n%s:%d:%d: internal error: %s\n", pkg.Load.ImportPath,
			diag.ShortFileName(d.check.dir, e.Pos.Filename), e.Pos.Line, e.Pos.Column, strings.TrimPrefix(e.Msg, "internal error: "))
	case errors.As(err, &e):
		fmt.Fprintf(d.stderr, "# %s\ninternal error: %s\n", pkg.Load.ImportPath, strings.TrimPrefix(e.Msg, "internal error: "))
	default:
		fmt.Fprintf(d.stderr, "# %s\ninternal error: %v\n", pkg.Load.ImportPath, err)
	}
	fmt.Fprintln(d.stderr, "(gocpp checked the package, but its C++ emitter can't write it: this is a bug in gocpp, or a feature that is not built yet)")
	return exitInternal
}

// probe runs the C++ compiler's probes for the C/C++ names of pkg
// (lower.ProbeInterop, Phase 04 §2.2) with the compiler cxx, or with the
// compiler of the target when cxx is nil. A package without header imports
// needs none. It returns the probes' answers, or nil and the exit status.
//
// The probes get the flags that the check's namespace probe got
// (check.Package.ProbeFlags: the environment's trusted flags, then the
// package's flags after the allow-list), so that the kind probe finds the
// check's run in interop's cache.
func (d *driver) probe(pkg *check.Package, cxx *build.CXX) (*lower.Interop, int) {
	has := false
	for _, f := range pkg.Load.Files {
		has = has || f.Info != nil && len(f.Info.Includes) > 0
	}
	if !has {
		return nil, 0
	}
	if cxx == nil {
		c, err := build.FindCXXFor(d.ctx, d.check.goos, d.check.goarch)
		if err != nil {
			if sig := d.interrupted(); sig != nil {
				return nil, stopStatus(sig)
			}
			fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
			return nil, exitErrors
		}
		cxx = &c
	}
	in, err := lower.ProbeInterop(d.ctx, *cxx, d.check.fset, pkg, pkg.ProbeFlags)
	if err == nil {
		d.attachTyped(in, pkg) // typed code generation, when GOCPP_TYPED_INTEROP=1 (typedlower.go)
		return in, 0
	}
	if sig := d.interrupted(); sig != nil {
		return nil, stopStatus(sig)
	}
	var e *lower.Error
	if errors.As(err, &e) && e.Pos.IsValid() {
		fmt.Fprintf(d.stderr, "# %s\n%s:%d:%d: %s\n", pkg.Load.ImportPath,
			diag.ShortFileName(d.check.dir, e.Pos.Filename), e.Pos.Line, e.Pos.Column, e.Msg)
	} else {
		fmt.Fprintf(d.stderr, "# %s\n%v\n", pkg.Load.ImportPath, err)
	}
	return nil, exitErrors
}

// directiveCompileFlags returns the compile flags of pkg that come from its
// //gocpp: directives and pkg-config: the first Flags.Directive of Flags.Compile.
// After them come the public flags of the cxx recipes of its modules
// (recipeCompileFlags) and the include folders of the modules whose headers it
// imports (build.ReadPackageFlags puts those last, as -iquote folder), which
// are the loader's, not the user's flags.
func directiveCompileFlags(pkg *check.Package) []string {
	end := min(pkg.Flags.Directive, len(pkg.Flags.Compile))
	return pkg.Flags.Compile[min(pkg.Flags.Defaults, end):end] // (gocpp's own Win32 defaults come first)
}

// recipeCompileFlags returns the compile flags of pkg that come from the cxx
// recipes of the modules whose headers it imports (-I, -D, ...).
func recipeCompileFlags(pkg *check.Package) []string {
	c := pkg.Flags.Compile
	start := min(pkg.Flags.Directive, len(c))
	end := max(start, len(c)-2*len(pkg.Load.IncludeDirs))
	return c[start:end]
}

// importsLocalHeader reports whether a file of pkg imports a local header
// ("./physics.h").
func importsLocalHeader(pkg *check.Package) bool {
	for _, f := range pkg.Load.Files {
		if f.Info == nil {
			continue
		}
		for _, inc := range f.Info.Includes {
			if inc.Kind == load.LocalHeader {
				return true
			}
		}
	}
	return false
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
	// final is the file name the program gets when it is moved out of the work folder (exe has a name the compiler can read).
	final string
	dir   string // the folder of the main package (gocpp test runs the program there)
}

// compile checks and builds the program of the patterns in a new work folder. On failure it
// returns nil and the exit status, and it has removed the work folder (unless it keeps it:
// -work, or an internal error).
func (d *driver) compile(patterns []string) (*program, int) {
	set, code := d.load(patterns)
	if set == nil {
		return nil, code
	}
	if d.stamp != nil {
		d.watchHeaders(set)
	}
	pkg := set.main
	env, err := build.ReadEnvFlags()
	if err != nil {
		fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
		return nil, exitErrors
	}
	if d.envOK {
		env = d.env // with the mode's and the build flags' (flags.go)
	}
	x := &d.flags.extra
	needC, hasCFiles := false, false
	for _, p := range set.pkgs {
		nc, code := d.resolveLibs(p) // the sources of the modules' libraries (recipes)
		if code != 0 {
			return nil, code
		}
		needC = needC || nc
		hasCFiles = hasCFiles || len(p.Load.CFiles) > 0
	}
	cxx, err := build.FindCXXFor(d.ctx, d.check.goos, d.check.goarch) // the probe's compiler too
	var cc build.CXX
	if err == nil && (hasCFiles || needC) {
		cc, err = d.findCC(cxx)
	}
	if err != nil {
		if sig := d.interrupted(); sig != nil {
			return nil, stopStatus(sig)
		}
		fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
		return nil, exitErrors
	}
	if x.llvm != "" {
		err := emitLLVMError("C++", cxx)
		if err == nil && (hasCFiles || needC) {
			err = emitLLVMError("C", cc)
		}
		if err != nil {
			fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
			return nil, exitUsage
		}
	}
	work, err := makeWorkFolder()
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

	cache := cacheDir()
	name := d.programName(pkg)
	genRoot, releaseGen := d.lockGen(d.genRoot(set, work, cache), work)
	built := false
	defer func() { releaseGen(built) }() // after the compiles: another build of the program may write then
	d.gen = genRoot
	files, code := d.emit(set, genRoot, &cxx)
	if files == nil {
		return nil, code
	}
	if d.flags.mode.Library() {
		if code := d.prepareLibrary(set, files, cxx, env); code != 0 {
			return nil, code
		}
	}
	if d.flags.x {
		fmt.Fprintf(d.stderr, "# gocpp emit > %s\n", build.ShellQuoteWork(genRoot, work))
	}
	if d.stamp != nil {
		d.stamp.pkgs = d.stampSummaries(set, files)
		d.traceSummaries(d.stamp.pkgs)
	}
	units := make(map[string][]string) // the translation units by import path (absolute names)
	for i, pf := range files.Packages {
		for _, rel := range pf.Compile {
			units[set.pkgs[i].Load.ImportPath] = append(units[set.pkgs[i].Load.ImportPath], filepath.Join(genRoot, filepath.FromSlash(rel)))
		}
	}
	if err := os.MkdirAll(genRoot, 0o755); err == nil && !d.flags.work && cache != "" {
		useGen(genRoot)
	}
	for _, f := range files.Files {
		data := f.Data
		if testHookCPP != nil {
			data = testHookCPP(f.Path, data)
		}
		changed, err := writeIfChanged(filepath.Join(genRoot, filepath.FromSlash(f.Path)), data)
		if err != nil {
			fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
			return nil, exitErrors
		}
		if changed {
			d.counts.write++
		}
	}

	// The linker writes the program under a name the compiler can read (ASCII when the program's name has a character
	// outside the ANSI code page, as in a folder 日本): the move to the final name is gocpp's (Go's file functions
	// have no such limit).
	final := name + exeSuffixFor(d.check.goos)
	exe := filepath.Join(work, workFileName(final, "prog"))
	if d.lib != nil {
		final = d.lib.file
		exe = filepath.Join(work, d.lib.workFile)
	}
	// newBuilder makes the Builder of one attempt (a build that lost its work folder, workLost,
	// makes a second one: a Builder is used once).
	newBuilder := func() (*build.Builder, int) {
		b := &build.Builder{CXX: cxx, ASan: d.flags.asan, NoChecks: d.flags.nochecks, LTO: d.flags.lto, GOOS: d.check.goos, Work: work, Stderr: d.stderr, KeepWork: d.flags.work,
			Env: env, CC: cc, GenRoot: genRoot, Cache: cache, Jobs: d.flags.jobs, Stats: &d.stats, Mode: d.flags.mode,
			DryRun: x.dryRun || d.afterPlan != nil, Rebuild: x.rebuild, EmitLLVM: x.llvm, NoManifest: build.EnvOff("GOCPP_NO_MANIFEST")}
		if d.lib != nil {
			if d.check.goos == "windows" {
				if d.lib.implib != "" {
					b.ImportLib = filepath.Join(work, d.lib.workImplib)
				}
			} else {
				b.Soname = d.lib.file
			}
			if d.lib.script != nil {
				b.VersionScript = filepath.Join(work, "exports.map")
				if err := os.WriteFile(b.VersionScript, d.lib.script, 0o644); err != nil {
					fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
					return nil, exitErrors
				}
			}
		}
		if d.flags.x || x.dryRun && d.name == "build" {
			b.Trace = d.stderr // -n prints the commands too
		}
		if x.verbose {
			for _, p := range set.pkgs {
				fmt.Fprintln(d.stderr, p.Load.ImportPath)
			}
		}
		if d.wantsPlan() {
			b.Plan = &build.Plan{}
		}
		if x.llvm != "" {
			b.LLVMDir = d.outputPath(name+exeSuffixFor(d.check.goos)) + ".llvm"
			if !b.DryRun {
				if err := build.PrepareIRDir(b.LLVMDir); err != nil {
					fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
					return nil, exitErrors
				}
			}
		}
		if d.stamp != nil {
			b.Rec = d.stamp.rec
		}
		return b, 0
	}
	b, code := newBuilder()
	if b == nil {
		return nil, code
	}
	if testHookGenWritten != nil {
		testHookGenWritten()
	}
	pbs := d.packageBuilds(set, files, units)
	link := func(attempt int) error {
		if testHookLink != nil {
			testHookLink("before", attempt, work)
		}
		err := b.Link(d.ctx, pbs, exe)
		if testHookLink != nil {
			testHookLink("after", attempt, work)
		}
		return err
	}
	err = link(0)
	checkExe := exe
	if d.lib != nil {
		checkExe = "" // a library mode's output is not always a file of that name
	}
	for attempt := 1; workLost(work, checkExe, b.DryRun, err) && d.interrupted() == nil; attempt++ {
		// Something else removed the work folder (or the temporary folder that holds it) while
		// the compilers ran in it: a temporary-file cleaner, another tool that clears TMPDIR. No
		// failure of the program or of gocpp: the build is made again in a work folder of the
		// same name, a few times at most.
		if workLostFails() {
			fmt.Fprintf(d.stderr, "gocpp %s: the work folder %s was removed while the build ran: another process or a cleanup of the temporary folder %s deleted it (GOCPP_WORKLOST=fail: not building again)\n",
				d.name, work, filepath.Dir(work))
			return nil, exitWorkLost
		}
		if attempt > maxWorkAttempts {
			fmt.Fprintf(d.stderr, "gocpp %s: the work folder %s was removed %d times while the build ran: another process or a cleanup of the temporary folder %s keeps deleting it\n",
				d.name, work, attempt, filepath.Dir(work))
			return nil, exitErrors
		}
		fmt.Fprintf(d.stderr, "gocpp %s: the work folder %s was removed while the build ran (another process or a cleanup of the temporary folder deleted it; GOCPP_WORKLOST=fail makes this an error): building again\n", d.name, work)
		if err := restoreWork(work); err != nil {
			fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
			return nil, exitErrors
		}
		if strings.HasPrefix(genRoot, work+string(os.PathSeparator)) {
			for _, f := range files.Files { // the generated files were in the work folder too
				if _, err := writeIfChanged(filepath.Join(genRoot, filepath.FromSlash(f.Path)), f.Data); err != nil {
					fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
					return nil, exitErrors
				}
			}
		}
		if d.stamp != nil {
			d.stamp.rec = &build.Record{}
		}
		d.stats.Reset() // the counts of -x are those of the build that worked
		if b, code = newBuilder(); b == nil {
			return nil, code
		}
		err = link(attempt)
	}
	if err != nil {
		if sig := d.interrupted(); sig != nil {
			return nil, stopStatus(sig)
		}
		code, kept := d.buildFailed(set, err, env, work)
		keep = keep || kept
		return nil, code
	}
	if b.Plan != nil {
		if code := d.finishPlan(set, genRoot, b.Plan, name); code != 0 {
			return nil, code
		}
	}
	built = true
	keep = true // the caller removes it
	if st := d.stamp; st != nil {
		st.built = true
		st.compilers = []string{build.CompilerID(cxx)}
		if hasCFiles || needC {
			st.compilers = append(st.compilers, build.CompilerID(cc))
		}
	}
	return &program{work: work, exe: exe, final: final, dir: set.main.Load.Dir}, 0
}

// buildFailed prints the error err of Build.Link and returns the exit status and whether the
// work folder must be kept. err holds one failure, or all of those of a build in which more than
// one compile failed (build.Errors), in a fixed order; each is printed like a single one, and the
// status is the highest of them (an internal error, a bug in gocpp, over an error of the user).
func (d *driver) buildFailed(set *buildSet, err error, env build.EnvFlags, work string) (code int, keep bool) {
	errs := build.Flatten(err)
	for i, e := range errs {
		d.moreFailures = i < len(errs)-1
		c, k := d.reportFailure(set, e, env, work)
		code = max(code, c)
		keep = keep || k
	}
	var multi *build.Errors
	if errors.As(err, &multi) && multi.NotStarted > 0 {
		fmt.Fprintf(d.stderr, "gocpp %s: stopped after %d failed compiles: %d more were not started\n", d.name, len(errs), multi.NotStarted)
	}
	return code, keep
}

// reportFailure prints one failure of the build (buildFailed) and returns its exit status and
// whether the work folder is kept for it.
func (d *driver) reportFailure(set *buildSet, err error, env build.EnvFlags, work string) (code int, kept bool) {
	pkg := set.main
	// The package of the unit or the source that failed (the link: the first package that
	// has C/C++ files, link flags or C/C++ imports).
	var ce *build.CompileError
	src := ""
	if errors.As(err, &ce) {
		src = ce.Unit
		switch {
		case ce.PkgPath != "":
			pkg = set.byPath(ce.PkgPath)
		case ce.Link:
			pkg = set.linkPackage()
		}
	}
	if errors.As(err, &ce) && ce.Lib != nil {
		d.libFailed(pkg, ce)
		return exitErrors, kept
	}
	if errors.As(err, &ce) && ce.Source != "" {
		d.sourceFailed(pkg, ce)
		return exitErrors, kept
	}
	if errors.As(err, &ce) && ce.Link && d.cppLinkFailed(pkg, ce, env) {
		return exitErrors, kept
	}
	if errors.As(err, &ce) && !ce.Link && ce.What == "the generated C++" && d.cppCompileFailed(pkg, ce, src) {
		return exitErrors, kept
	}
	if errors.As(err, &ce) && ce.Link && (len(pkg.Flags.Link) > 0 || len(env.LDFlags) > 0 || hasSources(pkg.Load)) {
		d.linkFailed(pkg, ce, env)
		return exitErrors, kept
	}
	if errors.As(err, &ce) {
		kept = true
		// With flags of the package or of the environment, the flags
		// may be the cause: not reported as a bug in gocpp.
		var flags []string
		if directive := directiveCompileFlags(pkg); ce.What != "the runtime" && len(directive) > 0 {
			flags = append(flags, "the compile flags of the package's //gocpp: directives: "+strings.Join(directive, " "))
		}
		if recipe := recipeCompileFlags(pkg); ce.What != "the runtime" && len(recipe) > 0 {
			flags = append(flags, "the compile flags of the cxx recipes of the package's modules: "+strings.Join(recipe, " "))
			flags = append(flags, shadowHints(recipe)...)
		}
		for _, v := range []struct {
			name  string
			flags []string
		}{{"trusted C and C++ flags (GOCPP_CFLAGS, -cflags, the mode)", env.CFlags}, {"trusted C++ flags (GOCPP_CXXFLAGS, -cxxflags)", env.CXXFlags}} {
			if len(v.flags) > 0 {
				flags = append(flags, v.name+": "+strings.Join(v.flags, " "))
			}
		}
		d.compileFailed(pkg, ce, work, flags)
		if len(flags) > 0 {
			return exitErrors, kept
		}
		return exitInternal, true
	}
	fmt.Fprintf(d.stderr, "gocpp %s: %v\n", d.name, err)
	return exitErrors, kept
}

// traceActions prints what the build did, as the line "# actions: ..." of -x. The counts are of
// operations, not of time (the tests of the incremental builds read them): load, check and emit
// are what the front end did (load=0 for a build that was up to date), write the generated files
// whose bytes changed, and the rest is build.Stats.
func (d *driver) traceActions() {
	if !d.flags.x || d.counts.load == 0 && !d.upToDateBuild {
		return // (the build stopped before it loaded the packages: a usage error, the errors of check)
	}
	fmt.Fprintf(d.stderr, "# actions: load=%d check=%d emit=%d write=%d %s\n", d.counts.load, d.counts.check, d.counts.emit, d.counts.write, d.stats.String())
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
// the work folder with the C++ and the compiler's whole output. flags,
// when there are any, describe the flags of the package or of the
// environment that the compile had: then the error says that the build
// failed with them, not that it is a bug in gocpp.
func (d *driver) compileFailed(pkg *check.Package, e *build.CompileError, work string, flags []string) {
	log := "compile.log"
	verb := "did not compile"
	if e.Link {
		log, verb = "link.log", "did not link"
	}
	// Several units can fail in one build (buildFailed): the first failure has compile.log,
	// the next ones compile-2.log, compile-3.log, ...
	d.logCount++
	if d.logCount > 1 {
		log = fmt.Sprintf("%s-%d.log", strings.TrimSuffix(log, ".log"), d.logCount)
	}
	logPath := filepath.Join(work, log)
	os.WriteFile(logPath, append([]byte(e.Command+"\n"), e.Output...), 0o644)

	what := "generated C++"
	if e.What == "the runtime" {
		what = "gocpp's runtime"
	}
	exportGlue := d.lib != nil && (strings.HasSuffix(e.Unit, "gocpp_export.cpp") || strings.Contains(e.Command, "gocpp_export.cpp"))
	var shown []string
	if d.flags.cppErrors.full() {
		if out := strings.TrimRight(string(e.Output), "\n"); out != "" {
			shown = strings.Split(d.shortenDir(pkg, out), "\n")
		}
	}
	full := len(shown) > 0
	sc := bufio.NewScanner(bytes.NewReader(e.Output))
	sc.Buffer(nil, 1<<20)
	for !full && sc.Scan() && len(shown) < maxShownErrors {
		line := cleanCompilerLine(sc.Text())
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
	// Units of one package that fail for one reason (a header that does not compile) show the same
	// errors: the second one says so.
	same := d.lastShown != nil && d.lastShownPkg == pkg.Load.ImportPath && slices.Equal(d.lastShown, shown)
	d.lastShown, d.lastShownPkg = shown, pkg.Load.ImportPath
	switch {
	case same:
		unit := "the link"
		if e.Unit != "" {
			unit = filepath.Base(e.Unit)
		}
		fmt.Fprintf(d.stderr, "# %s\n(the same errors in %s)\n", pkg.Load.ImportPath, unit)
	case e.What == "the archiver":
		fmt.Fprintf(d.stderr, "# %s\nthe archiver failed while making gocpp's runtime library (GOCPP_AR names it; -lto needs gcc-ar or llvm-ar)\n", pkg.Load.ImportPath)
	case len(flags) > 0:
		fmt.Fprintf(d.stderr, "# %s\n%s %s with the flags below\n", pkg.Load.ImportPath, what, verb)
	case exportGlue:
		// The C functions of the //export directives are gocpp's glue, but the mistake is the name:
		// one that a header of the packages or of the C library declares differently.
		fmt.Fprintf(d.stderr, "# %s\nthe exported functions %s %s: an exported name is probably one that a C header declares "+
			"differently (macros and functions of the C library); choose another name\n", pkg.Load.ImportPath, what, verb)
	default:
		fmt.Fprintf(d.stderr, "# %s\ninternal error: %s %s\n", pkg.Load.ImportPath, what, verb)
	}
	if !same {
		for _, line := range shown {
			fmt.Fprintln(d.stderr, line)
		}
	}
	if d.moreFailures {
		fmt.Fprintf(d.stderr, "(the compiler's whole output is in %s)\n", log)
		return
	}
	if len(flags) > 0 || exportGlue {
		for _, f := range flags {
			fmt.Fprintf(d.stderr, "(%s)\n", f)
		}
		fmt.Fprintf(d.stderr, "The work folder is kept: %s\n(the generated C++ is in %s, and the compiler's whole output is in %s)\n", work, d.gen, log)
		return
	}
	if e.What == "the archiver" {
		fmt.Fprintf(d.stderr, "(the archiver's whole output is in %s)\n", log)
		return
	}
	fmt.Fprintf(d.stderr, "This is a bug in gocpp. The work folder is kept: %s\n(the generated C++ is in %s, and the compiler's whole output is in %s)\n", work, d.gen, log)
}

// linkFailed prints the error of a link that failed with link flags of the
// package's directives or of GOCPP_LDFLAGS, or with the package's own C and
// C++ files: a library that isn't installed, a wrong flag or a function
// that the user's C doesn't define is more likely than a bug in gocpp. It shows
// the linker's first lines, like go build shows those of an external link
// that fails, and the flags.
func (d *driver) linkFailed(pkg *check.Package, e *build.CompileError, env build.EnvFlags) {
	fmt.Fprintf(d.stderr, "# %s\nthe program did not link: %v\n", pkg.Load.ImportPath, e.Err)
	lines := strings.SplitN(strings.TrimSpace(string(e.Output)), "\n", maxShownErrors+1)
	if d.flags.cppErrors.full() {
		lines = strings.Split(strings.TrimSpace(string(e.Output)), "\n")
	}
	for i, line := range lines {
		if (i < maxShownErrors || d.flags.cppErrors.full()) && line != "" {
			fmt.Fprintln(d.stderr, line)
		}
	}
	if len(pkg.Flags.Link) > 0 {
		fmt.Fprintf(d.stderr, "(the link flags of the package's //gocpp: directives: %s)\n", strings.Join(pkg.Flags.Link, " "))
	}
	if len(env.LDFlags) > 0 {
		fmt.Fprintf(d.stderr, "(trusted link flags (GOCPP_LDFLAGS, -ldflags, the mode): %s)\n", strings.Join(env.LDFlags, " "))
	}
	if hasSources(pkg.Load) {
		fmt.Fprintf(d.stderr, "(the package's C and C++ files: %s)\n", strings.Join(slices.Concat(pkg.Load.CFiles, pkg.Load.CXXFiles), " "))
	}
	for _, h := range noRecipeHints(pkg, e.Output) {
		fmt.Fprintln(d.stderr, h)
	}
}

// build is gocpp build: it builds the program and moves it to its place.
func (d *driver) build(patterns []string) int {
	defer d.traceActions()
	if d.stamp = d.beginStamp(patterns); d.stamp != nil && d.upToDate() {
		return 0
	}
	p, code := d.compile(patterns)
	if p == nil {
		return code
	}
	defer d.removeWork()
	if d.flags.extra.dryRun {
		return 0 // -n: nothing was run, and there is no program to move
	}
	if d.lib != nil {
		built, code := d.installLibrary(p, d.flags.output)
		if code != 0 {
			return code
		}
		d.saveStamp(built)
		return 0
	}
	out := d.outputPath(p.final)
	if d.flags.x {
		fmt.Fprintf(d.stderr, "mv %s %s\n", build.ShellQuoteWork(p.exe, p.work), build.ShellQuoteWork(out, ""))
	}
	err := os.MkdirAll(filepath.Dir(out), 0o755) // -o dir/ names a folder that may not exist yet
	if err == nil {
		err = build.MoveFile(p.exe, out)
	}
	if err != nil {
		fmt.Fprintf(d.stderr, "gocpp build: %v\n", err)
		return exitErrors
	}
	d.saveStamp(out)
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
	// A program for another target runs under GOCPP_EXEC (wine, qemu-aarch64), and that is
	// known before the build.
	err := buildTarget(d.check.goos, d.check.goarch)
	var runner []string
	if err == nil {
		runner, err = crossRunner(d.check.goos, d.check.goarch)
	}
	if err != nil {
		fmt.Fprintf(d.stderr, "gocpp run: %v\n", err)
		return exitErrors
	}
	p, code := d.compile(patterns)
	if p == nil {
		return code
	}
	defer d.removeWork()
	if d.flags.x {
		words := []string{}
		for _, w := range runner {
			words = append(words, build.ShellQuoteWork(w, ""))
		}
		words = append(words, build.ShellQuoteWork(p.exe, p.work))
		for _, a := range args {
			words = append(words, build.ShellQuoteWork(a, ""))
		}
		fmt.Fprintln(d.stderr, strings.Join(words, " "))
	}
	newCmd := func() *exec.Cmd {
		argv := append(append(slices.Clone(runner), p.exe), args...)
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Dir = d.check.dir
		cmd.Env = runProgramEnv(cmd.Dir, runtime.GOOS) // the user's TEMP and LOCALAPPDATA, not the ones gocpp made for the compilers (envpaths.go)
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

// linkPackage returns the package that reports a failed link: the first one that has C or C++
// files, link flags or C/C++ imports (the conditions of cppLinkFailed and linkFailed), else the
// main package.
func (s *buildSet) linkPackage() *check.Package {
	for _, p := range s.pkgs {
		if hasSources(p.Load) || len(p.Flags.Link) > 0 || pkgUsesCpp(p) {
			return p
		}
	}
	return s.main
}

// genRoot returns the folder that the generated files of the program are written to. It is
// stable, so that the -iquote folder and the paths of the files in the debug information and in the
// object cache's keys are the same in every build: gen/<hash> in the cache folder, where the hash
// is of the main package (its folder, import path and files) and the target. With -work, without a
// cache folder or with one that can't be written, it is gen in the work folder.
func (d *driver) genRoot(set *buildSet, work, cache string) string {
	if d.flags.work || cache == "" {
		return filepath.Join(work, "gen")
	}
	h := sha256.New()
	fmt.Fprintf(h, "%s\x00%s\x00%s/%s\x00", set.main.Load.Dir, set.main.Load.ImportPath, d.check.goos, d.check.goarch)
	if d.flags.mode != "" {
		fmt.Fprintf(h, "%s\x00%s\x00", d.flags.mode, strings.Join(d.libRoots, "\x01")) // a library's files are not a program's
	}
	for _, f := range set.main.Load.Files {
		fmt.Fprintf(h, "%s\x00", f.Path)
	}
	dir := filepath.Join(cache, "gen", hex.EncodeToString(h.Sum(nil))[:16])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return filepath.Join(work, "gen") // the cache folder can't be written (a read-only home): no cache
	}
	return dir
}

// Limits of the folder of the generated files of the programs (cache/gen): trimGen removes the
// folders of programs that were not built for maxGenAge, when there are more than maxGenFolders.
const (
	maxGenFolders = 100
	maxGenAge     = 7 * 24 * time.Hour
)

// useGen marks the folder gen of a program as used now, and removes the old folders of other
// programs next to it (trimGen). A failure is no failure of the build.
func useGen(gen string) {
	now := time.Now()
	os.Chtimes(gen, now, now)
	trimGen(filepath.Dir(gen), gen, now)
}

// trimGen removes the folders in dir, other than keep, that are older than maxGenAge, beginning with
// the oldest, while there are more than maxGenFolders of them. A folder of a program that was built
// within the age is never removed, so a gocpp that builds at the same time never loses its files.
func trimGen(dir, keep string, now time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type old struct {
		path string
		mod  time.Time
	}
	var olds []old
	folders := 0
	for _, e := range entries {
		p := filepath.Join(dir, e.Name())
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if !e.IsDir() {
			// The lock file of a folder that is gone (lockGen).
			if strings.HasSuffix(e.Name(), ".lock") && now.Sub(fi.ModTime()) > maxGenAge {
				if _, err := os.Stat(strings.TrimSuffix(p, ".lock")); err != nil {
					os.Remove(p)
				}
			}
			continue
		}
		folders++
		if p != keep && now.Sub(fi.ModTime()) > maxGenAge {
			olds = append(olds, old{p, fi.ModTime()})
		}
	}
	slices.SortFunc(olds, func(a, b old) int { return a.mod.Compare(b.mod) })
	for _, o := range olds {
		if folders <= maxGenFolders {
			break
		}
		if removeGenFolder(o.path, now) {
			folders--
		}
	}
}

// removeGenFolder removes the folder gen of a program unless a build uses it, or used it since trimGen
// looked at its time. The time of a folder is its last use only from the moment its build has
// written its files (useGen): a build that holds the lock of the folder (lockGen) and has not got so
// far looks as old as the folder was. So the lock decides: the folder is removed while the remover
// holds its lock, and stays when the lock is taken (a build that comes later waits for the remover,
// then writes its files into a new folder); and the time is read again under the lock, for a build
// that finished between the look of trimGen and now. The lock file is made when it is not there:
// a build that is just starting (it made the folder, not yet the lock file) waits for the remover
// too. Where locks can't be taken, every build has a folder of its own (lockGen), and the old
// folder is nobody's.
func removeGenFolder(gen string, now time.Time) bool {
	l, busy, err := lockFile(gen + ".lock")
	if busy {
		return false
	}
	if err == nil {
		defer l.unlock()
	}
	if fi, err := os.Stat(gen); err != nil || now.Sub(fi.ModTime()) <= maxGenAge {
		return false // gone already, or used since
	}
	if testHookGenRemove != nil {
		testHookGenRemove(gen)
	}
	return os.RemoveAll(gen) == nil
}

// testHookGenRemove, if set, is called by removeGenFolder when it holds the lock and is about to
// remove the folder. Only the tests set it.
var testHookGenRemove func(gen string)

// writeIfChanged writes data to the file name, unless it has these bytes already: editors and
// the object cache see no change then. The file is replaced by a rename, so that a gocpp that
// builds the same program at the same time never reads half a file.
func writeIfChanged(name string, data []byte) (written bool, err error) {
	if old, err := os.ReadFile(name); err == nil && bytes.Equal(old, data) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		return false, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(name), ".tmp-"+filepath.Base(name)+"-")
	if err != nil {
		return false, err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return false, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return false, err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		os.Remove(tmp.Name())
		return false, err
	}
	if err := fsretry.Rename(tmp.Name(), name); err != nil {
		os.Remove(tmp.Name())
		return false, err
	}
	return true, nil
}

// packageBuilds returns the packages of the program for build.Builder.Link, in the order of
// the program, with their units, flags and the public flags of the packages whose headers they
// include.
func (d *driver) packageBuilds(set *buildSet, files *cppgen.ProgramFiles, units map[string][]string) []build.PackageBuild {
	byLoad := make(map[*load.Package]*check.Package)
	headerC := make(map[*load.Package]bool)
	for i, p := range set.pkgs {
		byLoad[p.Load] = p
		headerC[p.Load] = files.Packages[i].HeaderC
	}
	pbs := make([]build.PackageBuild, len(set.pkgs))
	for i, p := range set.pkgs {
		pb := build.PackageBuild{Path: p.Load.ImportPath, Dir: p.Load.Dir, Flags: p.Flags, CFiles: p.Load.CFiles, CXXFiles: p.Load.CXXFiles,
			Units: units[p.Load.ImportPath], SourceWarnings: d.sourceWarnings(p)}
		if importsLocalHeader(p) {
			pb.QuoteDir = p.Load.Dir // -iquote for "./physics.h"
		}
		// The public flags: those of the packages whose pkg.hpp includes C/C++ headers.
		seen := map[*load.Package]bool{p.Load: true}
		var visit func(lp *load.Package)
		visit = func(lp *load.Package) {
			for _, path := range slices.Sorted(maps.Keys(lp.Imports)) {
				dep := lp.Imports[path]
				if seen[dep] {
					continue
				}
				seen[dep] = true
				if cp := byLoad[dep]; cp != nil && headerC[dep] {
					if !slices.Equal(cp.Flags.Compile, p.Flags.Compile) {
						pb.Public = append(pb.Public, cp.Flags.Compile...)
					}
					if importsLocalHeader(cp) {
						// pkg.hpp includes "./geo.h": found in the folder of its package.
						pb.Public = append(pb.Public, "-iquote", cp.Load.Dir)
					}
				}
				visit(dep)
			}
		}
		visit(p.Load)
		pbs[i] = pb
	}
	return pbs
}

// workFileName returns name, or, when the compiler cannot be handed it (a character outside the ANSI code page on a Windows
// host), the ASCII name stem plus the extension of name: the file the linker writes in the work folder.
func workFileName(name, stem string) string {
	if build.CompilerSafe(name) {
		return name
	}
	return stem + filepath.Ext(name)
}
