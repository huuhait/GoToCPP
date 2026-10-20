package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/modfetch"
	"github.com/huuhait/gocpp/internal/pclock"
	"github.com/huuhait/gocpp/internal/toolchain"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/version"
)

// gocpp env (Phase 06 step 6.12): the environment variables that gocpp reads, with the values it
// uses, and what gocpp found out (the compilers, the folders). Like go env, it prints NAME='value'
// lines, one name (and the values only) per argument, or -json.

// envUsage is the help text of "gocpp env".
const envUsage = `usage: gocpp env [-json] [var ...]

gocpp env prints the environment variables that gocpp reads, each with the value it uses (the one
in the environment, else the default), and the values it found out: the compilers, the cache folder,
the module. It prints one NAME='value' line for each, sorted by name (on Windows set NAME=value).
With names, it prints only their values, one per line. -json prints one JSON object that maps each
name to its value.

The names that don't start with GOCPP_ are found out, not read: GOCPPVERSION, GOCPPCACHE (the cache
folder), GOCPPRUNTIME (the folder where the runtime is extracted), GOCPPMOD (the gocpp.mod of the current
folder, or empty), GOCPPCXX, GOCPPCXXVERSION and GOCPPCC (the compilers a build here would use; empty,
with a message, when there is none). GOOS and GOARCH are the target that gocpp builds for.

Every variable is described in docs/spec/build.md.

The flags are:
`

// An envVar is an environment variable that gocpp reads (or a value that it finds out).
type envVar struct {
	name string

	// found marks a value that gocpp finds out: it is not read from the environment.
	found bool

	// def returns the value used when the variable is not set (or the found value); nil is "".
	def func(e *envState) string

	// doc is one line of docs/spec/build.md's table.
	doc string
}

// An envState finds out the values that need work (the compilers), once.
type envState struct {
	ctx    context.Context
	stderr io.Writer
	dir    string

	goos, goarch string

	cxx    *build.CXX
	cxxErr error
	cxxRan bool
	cc     *build.CXX
	ccRan  bool
}

func (e *envState) compiler() *build.CXX {
	if !e.cxxRan {
		e.cxxRan = true
		cxx, err := build.FindCXXFor(e.ctx, e.goos, e.goarch)
		if err != nil {
			e.cxxErr = err
			fmt.Fprintf(e.stderr, "gocpp env: %v\n", err)
		} else {
			e.cxx = &cxx
		}
	}
	return e.cxx
}

func (e *envState) cCompiler() *build.CXX {
	if !e.ccRan {
		e.ccRan = true
		if cxx := e.compiler(); cxx != nil {
			cc, err := build.FindCC(e.ctx, *cxx)
			if err != nil {
				fmt.Fprintf(e.stderr, "gocpp env: %v\n", err)
			} else {
				e.cc = &cc
			}
		}
	}
	return e.cc
}

func words(c *build.CXX) string {
	if c == nil {
		return ""
	}
	return strings.Join(append([]string{c.Path}, c.Args...), " ")
}

// envVars lists every variable of gocpp env. A test (TestEnvListsEveryVariable) checks that every
// GOCPP_ name that the Go code of the repository reads is here, so a new variable gets a line.
var envVars = []envVar{
	{name: "CXX", doc: "the C++ compiler, when GOCPP_CXX is not set (a program, maybe with arguments: CXX=\"ccache g++\")"},
	{name: "GOARCH", def: func(e *envState) string { return e.goarch }, doc: "the architecture to build for (build constraints and the compiler)"},
	{name: "GOCPPCACHE", found: true, def: func(*envState) string { return cacheDir() }, doc: "the cache folder: $XDG_CACHE_HOME/gocpp (~/.cache/gocpp); on Windows %LocalAppData%\\gocpp"},
	{name: "GOCPPCC", found: true, def: func(e *envState) string { return words(e.cCompiler()) }, doc: "the C compiler for .c files that a build would use"},
	{name: "GOCPPCXX", found: true, def: func(e *envState) string { return words(e.compiler()) }, doc: "the C++ compiler that a build would use: GOCPP_CXX, else CXX, else clang++, else g++ (for a Windows target: GOCPP_TOOLCHAIN's rule)"},
	{name: "GOCPPCXXSOURCE", found: true, def: func(e *envState) string {
		if c := e.compiler(); c != nil {
			return c.Source
		}
		return ""
	}, doc: "where the C++ compiler of a Windows target came from: PATH, an install gocpp knows (MSYS2 ucrt64 ...) or the pinned llvm-mingw; empty for GOCPP_CXX and for Linux targets"},
	{name: "GOCPPCXXVERSION", found: true, def: func(e *envState) string {
		if c := e.compiler(); c != nil {
			return c.Version
		}
		return ""
	}, doc: "the version line of the C++ compiler"},
	{name: "GOCPPMOD", found: true, def: func(e *envState) string {
		if m := load.FindModule(e.dir); m != nil && m.Dir != "" {
			return filepath.Join(m.Dir, "gocpp.mod")
		}
		return ""
	}, doc: "the gocpp.mod file of the current folder (the nearest one above it), or empty"},
	{name: "GOCPPRUNTIME", found: true, def: func(*envState) string {
		if c := cacheDir(); c != "" {
			return build.RuntimeFolder(c)
		}
		return ""
	}, doc: "the folder in which the runtime is extracted"},
	{name: "GOCPPVERSION", found: true, def: func(*envState) string { return version.Version }, doc: "the version of gocpp"},
	{name: "GOCPP_AR", doc: "the archiver (and its arguments) for the runtime library and -lto, instead of the search for ar, gcc-ar or llvm-ar"},
	{name: "GOCPP_ARCH", doc: "the architecture to build for, like -arch (amd64 or arm64); default: GOARCH, else this system's"},
	{name: "GOCPP_CC", doc: "the C compiler for the .c files of packages (a program, maybe with arguments); default: the C compiler of the C++ compiler's toolchain (gcc for g++, clang for clang++)"},
	{name: "GOCPP_CFLAGS", doc: "trusted flags for every C and C++ compile (also the probes and the runtime), not checked; split like a command (quotes keep spaces)"},
	{name: "GOCPP_CFLAGS_ALLOW", doc: "regular expression: compile flags of //gocpp: directives that are allowed besides the allow-list"},
	{name: "GOCPP_CFLAGS_DISALLOW", doc: "regular expression: compile flags of //gocpp: directives that are refused, also allowed ones"},
	{name: "GOCPP_CLANG", doc: "the clang++ that the typed interop probes use (a program); default: clang++ in PATH"},
	{name: "GOCPP_COLOR", doc: "always or never: color the diagnostics (the -color flag wins; NO_COLOR and CLICOLOR_FORCE are also read); default: color on a terminal"},
	{name: "GOCPP_CPUPROFILE", doc: "a file: gocpp writes a CPU profile of its own work there (for go tool pprof)"},
	{name: "GOCPP_CXX", doc: "the C++ compiler, with arguments if needed; default: CXX, else clang++, else g++"},
	{name: "GOCPP_CXXFLAGS", doc: "trusted flags for every C++ compile (also the probes and the runtime), not checked: may load compiler plugins (-fpass-plugin=x.so, -fplugin=x.so)"},
	{name: "GOCPP_EXCERPT", doc: "always or never: show the source line and a caret under each error (the -excerpt flag wins); default: on a terminal"},
	{name: "GOCPP_EXEC", doc: "the program that gocpp run starts the program with, with arguments if needed (wine for windows, qemu-aarch64), like go run -exec"},
	{name: "GOCPP_GATE_PHASE", doc: "two digits: accept the features of the plan's phases up to this one (gocpp's own development and tests)"},
	{name: "GOCPP_INTEROP", def: func(*envState) string { return typedinterop.DefaultMode.String() }, doc: "typed, opaque or auto: how C/C++ names are understood (the -interop flag wins). typed asks clang++ for the real types (early errors, constant macros, promoted members); opaque leaves C++ to the C++ compiler; auto is typed when clang++ is installed, else opaque"},
	{name: "GOCPP_LDFLAGS", doc: "trusted flags for every link, not checked"},
	{name: "GOCPP_LDFLAGS_ALLOW", doc: "regular expression: link flags of //gocpp: directives that are allowed besides the allow-list"},
	{name: "GOCPP_LDFLAGS_DISALLOW", doc: "regular expression: link flags of //gocpp: directives that are refused, also allowed ones"},
	{name: "GOCPP_LSP_CACHE", doc: "the folder of the language server's cache (an absolute name), or off; default: gocpp/lsp in the cache folder"},
	{name: "GOCPP_LSP_CACHE_AGE", def: func(*envState) string { return "120h" }, doc: "how long the language server's cache keeps an entry that is not used"},
	{name: "GOCPP_LSP_CACHE_SIZE", def: func(*envState) string { return "1G" }, doc: "the size limit of the language server's cache (a number with K, M or G)"},
	{name: "GOCPP_LSP_NOFOCUS", doc: "any value: the language server checks the whole package after an edit inside a function body, and does not publish the diagnostics of the edited file from the focused check first (a debugging switch; by default it does)"},
	{name: "GOCPP_LSP_NOSTALE", doc: "any value: the language server does not answer from the last repaired check of a file with a syntax error while it is being typed (a debugging switch; by default it does)"},
	{name: "GOCPP_MODAUTH", doc: "credentials for module downloads: host=token pairs (the value is not shown by gocpp env)"},
	{name: "GOCPP_MODCACERT", doc: "a file of CA certificates for module downloads"},
	{name: "GOCPP_MODCACHE", def: func(*envState) string {
		if base, err := os.UserCacheDir(); err == nil && base != "" {
			return filepath.Join(base, "gocpp", "mod")
		}
		return ""
	}, doc: "the module cache folder (an absolute name), or off for none"},
	{name: "GOCPP_MODFLAGS", doc: "-mod=readonly: never write gocpp.sum (for CI); -mod=mod: may"},
	{name: "GOCPP_MODPROXY", doc: "mirrors for module downloads: names separated by , or |, direct, off"},
	{name: "GOCPP_MODTIMEOUT", def: func(*envState) string { return modfetch.DefaultTimeout.String() }, doc: "the longest a module download may take (a duration)"},
	{name: "GOCPP_MODURL", doc: "where the archives of some modules are: prefix=template rules"},
	{name: "GOCPP_NOOBJCACHE", doc: "any value but empty, 0 and false: no object cache"},
	{name: "GOCPP_NOPCH", doc: "any non-empty value: no precompiled header of the runtime"},
	{name: "GOCPP_NOPROBECACHE", doc: "any value but empty, 0 and false: no cache on disk for the compiler's answers about headers"},
	{name: "GOCPP_NORTLIB", doc: "any value but empty, 0 and false: no runtime library; the runtime's object files are linked one by one"},
	{name: "GOCPP_NOUPTODATE", doc: "any value but empty, 0 and false: gocpp build does not check whether the program is up to date"},
	{name: "GOCPP_NO_MANIFEST", doc: "any value but empty, 0 and false: no application manifest in a Windows program (longPathAware, UTF-8 code page, asInvoker); for a program that brings its own resource"},
	{name: "GOCPP_OBJCACHE_SIZE", def: func(*envState) string { return "2G" }, doc: "the size limit of the object cache (a number with K, M or G); the oldest files go first"},
	{name: "GOCPP_OFFLINE", doc: "any value but empty, 0 and false: never use the network (module downloads and the Windows toolchain)"},
	{name: "GOCPP_OS", doc: "the operating system to build for, like -os (linux or windows); default: GOOS, else this system's"},
	{name: "GOCPP_PROBE_BATCH", doc: "the least number of references that one compiler run of the typed probes has when several run (a number); default 400"},
	{name: "GOCPP_PROBE_CACHE", doc: "the folder of the probe cache on disk (an absolute name; the typed interop's answers are in its subfolder typed-v1), or off for none; default: gocpp/probe in the cache folder"},
	{name: "GOCPP_PROBE_JOBS", doc: "how many compiler runs of the typed interop probes may go at the same time in a process (a number); default min(8, CPUs/2)"},
	{name: "GOCPP_PROBE_TIMEOUT", doc: "the longest one probe compiler run of the typed interop may take (a duration); default 5m in the tests"},
	{name: "GOCPP_TOOLCHAIN", def: func(*envState) string { return string(toolchain.DefaultMode) }, doc: "where the Windows compiler comes from: auto (MinGW-w64 in PATH or in the folders of MSYS2, Chocolatey, Scoop and WinGet's WinLibs, else the pinned llvm-mingw, downloaded on first use), local (PATH and, on Windows, the usual MinGW-w64 folders; never downloads) or pinned (always the pinned one)"},
	{name: "GOCPP_TOOLCHAIN_URL", doc: "an https folder URL with the toolchain archives under their names, instead of llvm-mingw's GitHub release (a mirror)"},
	{name: "GOCPP_TYPED_INTEROP", doc: "older spelling of GOCPP_INTEROP: 1 means typed, 0 means opaque (GOCPP_INTEROP and -interop win)"},
	{name: "GOCPP_WINDOWS_LOCK", def: func(*envState) string { return pclock.DefaultPath }, doc: "the lock file that makes the tests and tools that use the real Windows PC take turns"},
	{name: "GOCPP_WINDOWS_LOCK_WAIT", def: func(*envState) string { return "3h" }, doc: "how long they wait for that lock (a Go duration)"},
	{name: "GOCPP_WORKLOST", doc: "fail: a build whose work folder is removed by something else while it runs stops with status 4 and a message; by default it makes the folder again and builds again, up to three times (the tests and campaigns of the repository set fail, so that a real bug is not hidden)"},
	{name: "GOOS", def: func(e *envState) string { return e.goos }, doc: "the operating system to build for (build constraints and the compiler)"},
	{name: "PKG_CONFIG", def: func(*envState) string { return "pkg-config" }, doc: "the pkg-config program, for //gocpp:pkg-config lines"},
	{name: "XDG_CACHE_HOME", def: func(*envState) string {
		if base, err := os.UserCacheDir(); err == nil {
			return base
		}
		return ""
	}, doc: "where the cache folder is (on Linux; ~/.cache by default)"},
}

// secretVars are the variables whose value gocpp env does not print.
var secretVars = map[string]bool{"GOCPP_MODAUTH": true}

// value returns the value of v: the environment's, else the default.
func (e *envState) value(v envVar) string {
	if !v.found {
		if s := os.Getenv(v.name); s != "" {
			if secretVars[v.name] {
				return "(set)"
			}
			return s
		}
	}
	if v.def != nil {
		return v.def(e)
	}
	return ""
}

// runEnv is "gocpp env".
func runEnv(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	var asJSON bool
	flags := flag.NewFlagSet("gocpp env", flag.ContinueOnError)
	flags.BoolVar(&asJSON, "json", false, "print the variables as one JSON object")
	if code, done := parseFlags("env", envUsage, flags, args, stderr); done {
		return code
	}
	e := &envState{ctx: context.Background(), stderr: stderr}
	var err error
	if e.dir, err = getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp env: %v\n", err)
		return exitErrors
	}
	if e.goos, e.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err) // go exits with 2 too
		return 2
	}
	build.SetDetectCache(cacheDir())
	want := flags.Args()
	var vars []envVar
	if len(want) == 0 {
		vars = envVars
	} else {
		for _, n := range want {
			i := slices.IndexFunc(envVars, func(v envVar) bool { return v.name == n })
			if i < 0 {
				fmt.Fprintf(stderr, "gocpp env: unknown variable %s (gocpp env lists them)\n", n)
				return 2
			}
			vars = append(vars, envVars[i])
		}
	}
	values := make([]string, len(vars))
	for i, v := range vars {
		values[i] = e.value(v)
	}
	switch {
	case asJSON:
		m := make(map[string]string, len(vars))
		for i, v := range vars {
			m[v.name] = values[i]
		}
		data, err := json.MarshalIndent(m, "", "\t") // the keys come out sorted
		if err != nil {
			fmt.Fprintf(stderr, "gocpp env: %v\n", err)
			return exitErrors
		}
		fmt.Fprintf(stdout, "%s\n", data)
	case len(want) > 0:
		for _, s := range values {
			fmt.Fprintln(stdout, s)
		}
	default:
		for i, v := range vars {
			fmt.Fprintln(stdout, envLine(v.name, values[i]))
		}
	}
	return 0
}

// envLine returns the line of go env for the variable name with the value value: NAME='value' (a
// single quote in the value is written '"'"'), or set NAME=value on Windows.
func envLine(name, value string) string {
	if runtime.GOOS == "windows" {
		return "set " + name + "=" + value
	}
	return name + "='" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
