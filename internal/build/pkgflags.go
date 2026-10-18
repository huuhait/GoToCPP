package build

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// The C and C++ flags of a package (Phase 04 README §2.9, step 4.15).
//
// A package gets flags from the //gocpp: directives of its files:
//   - //gocpp:cflags -DFAST -Iinclude: compile flags, as written;
//   - //gocpp:ldflags -L/opt/foo/lib: link flags, as written;
//   - //gocpp:link m pthread: the link flags -lm -lpthread;
//   - //gocpp:pkg-config sdl2 x11: the output of pkg-config --cflags (compile
//     flags) and pkg-config --libs (link flags) for these packages, one run of
//     each per line.
//
// A relative path in cflags and ldflags is made absolute, from the package
// folder, as cgo does for -I and -L (absPaths): the compiles run in a work
// folder, and the probes in the package folder. Every flag, also the output of pkg-config,
// goes through the flag allow-list of the go command (security.go) before
// any probe, compiler or linker sees it: some flags run code at build time
// (-fplugin=x.so, -specs=, @file), and gocpp check and the language
// server run the probes with these flags as soon as a package is opened.
// gocpp refuses a few more (refused.go): the choices for the whole program,
// which a package can't make (the system root, the toolchain, the linker,
// the target and the ABI: --sysroot, -fuse-ld=, -m32, -stdlib=,
// -D_GLIBCXX_DEBUG). A line with a flag
// that is refused is an error at the line, and none of its flags are used.
// In the compile of the generated C++ and in the probes, gocpp's language
// flags (LanguageFlags) come again after all these flags, so that a package
// can't turn them off. GOCPP_CFLAGS_ALLOW and GOCPP_LDFLAGS_ALLOW (regular
// expressions that must match a whole flag) allow more compile or link flags;
// GOCPP_CFLAGS_DISALLOW and GOCPP_LDFLAGS_DISALLOW refuse flags, also allowed
// ones, like CGO_CFLAGS_ALLOW and its friends.
//
// The folders of the modules whose headers the package imports
// (load.Package.IncludeDirs, Phase 06 step 6.1b) come after these, as
// -iquote compile flags: they are the loader's own, so the allow-list doesn't see them.
//
// The flags add up in source order: the lines of a file from top to bottom,
// the files in the package's order (by name). Directives are package-wide:
// the flags of every file apply to the whole package.
//
// Unlike cgo, gocpp doesn't check the characters of each argument as
// go/build's safeCgoName does (with Go 1.25, #cgo CFLAGS: -DNAME=\"x\" is a
// "malformed #cgo argument"): gocpp never passes flags through a shell, only
// as arguments of a program, and the allow-list already refuses what a
// compiler or linker would split off as a flag or a response file of its own
// (@x, -x after -I and so on). So -DMSG=\"hi\" works, a string macro that
// cgo can't define.
//
// The environment gives trusted flags, which are not checked: the user chose
// them, as with CGO_CFLAGS. They are split like CC is split by the go command
// (SplitCommand: quotes keep spaces, no escapes):
//   - GOCPP_CFLAGS: every C and C++ compile, and the probes;
//   - GOCPP_CXXFLAGS: every C++ compile, and the probes (which are C++);
//   - GOCPP_LDFLAGS: every link.
//
// They also go to the compiles of the runtime, as they are the user's choice
// for the whole program (-stdlib=libc++ must be the same everywhere). The
// compiler itself reads CPATH, CPLUS_INCLUDE_PATH and C_INCLUDE_PATH, and a
// Nix wrapper NIX_CFLAGS_COMPILE, in the probes and the builds alike.

// PackageFlags are the C/C++ flags of one package that come from its
// directives and pkg-config, after the allow-list (ReadPackageFlags).
type PackageFlags struct {
	// Compile are the flags of the package's compiles and probes: those of
	// //gocpp:cflags and pkg-config --cflags, in source order.
	Compile []string

	// Directive is the number of flags at the start of Compile that come from
	// the package's directives and pkg-config; after them come the public flags
	// of the cxx recipes of its modules (Libs), then the -iquote folders of the
	// modules whose headers the package imports.
	Directive int

	// Libs are the C and C++ libraries of the modules whose headers the
	// package imports and that have a cxx recipe (Phase 06 step 6.1d,
	// cxxlib.go): their public flags are in Compile, their link flags in Link,
	// and the Builder compiles their sources.
	Libs []*CxxLib

	// Link are the flags of the link of a program with the package: those
	// of //gocpp:link, //gocpp:ldflags and pkg-config --libs, in source
	// order. They come after the object files.
	Link []string
}

// EnvFlags are the trusted flags of the environment (see above).
type EnvFlags struct {
	CFlags   []string // GOCPP_CFLAGS
	CXXFlags []string // GOCPP_CXXFLAGS
	LDFlags  []string // GOCPP_LDFLAGS
}

// CXXCompile returns the flags of every C++ compile from the environment:
// GOCPP_CFLAGS, then GOCPP_CXXFLAGS.
func (e EnvFlags) CXXCompile() []string {
	return slices.Concat(e.CFlags, e.CXXFlags)
}

// ReadEnvFlags reads the trusted flags of the environment. A value with a
// quote that isn't closed is an error.
func ReadEnvFlags() (EnvFlags, error) {
	var e EnvFlags
	for _, v := range []struct {
		name string
		dst  *[]string
	}{{"GOCPP_CFLAGS", &e.CFlags}, {"GOCPP_CXXFLAGS", &e.CXXFlags}, {"GOCPP_LDFLAGS", &e.LDFlags}} {
		words, err := SplitCommand(os.Getenv(v.name))
		if err != nil {
			return EnvFlags{}, fmt.Errorf("%s: %v", v.name, err)
		}
		*v.dst = words
	}
	return e, nil
}

// languageFlags are the flags whose meaning the generated C++ and gocpp's
// runtime rely on: the language flags of debugFlags (C++20, wrapping
// signed overflow, no strict aliasing, no contraction of floating-point
// operations), exceptions (panics are C++ exceptions), and no fast math
// (it would make NaN == NaN true, for one). -fno-fast-math comes first:
// with clang it sets -ffp-contract again. RTTI is not on the list: the
// runtime doesn't use it.
var languageFlags = []string{"-fno-fast-math", "-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-fexceptions"}

// LanguageFlags returns the flags that come again after the flags of the
// environment and of the package (when there are any) in the compile of
// the generated C++ and in the probes, so that those can't turn them off (with g++ and clang++, the
// last of -std=, -f(no-)fast-math, -f(no-)strict-aliasing, -ffp-contract=
// and -f(no-)exceptions wins). A package can still choose -O and -g.
func LanguageFlags() []string { return slices.Clone(languageFlags) }

// ProbeFlags returns the flags of the probes of a package with the flags
// pkg, in an environment with the flags env: those of every C++ compile of
// the environment, then the package's compile flags, then LanguageFlags,
// as in the compile of the generated C++. internal/check passes them to
// the namespace probe (check.Package.ProbeFlags), and the lowerer passes
// the same flags to interop.ProbeKinds, so that both share the probe's
// cache. Without flags of the environment and of the package, it returns
// nil: the probe has -std=c++20 of its own, and the other language flags
// are the compilers' defaults.
func ProbeFlags(env EnvFlags, pkg PackageFlags) []string {
	user := slices.Concat(env.CXXCompile(), pkg.Compile)
	if len(user) == 0 {
		return nil
	}
	return slices.Concat(user, languageFlags)
}

// A PkgConfigFunc runs pkg-config for the packages of one //gocpp:pkg-config
// line, in the folder dir, and returns the flags of --cflags and of --libs,
// before the allow-list. PkgConfig is the real one.
type PkgConfigFunc func(ctx context.Context, dir string, pkgs []string) (cflags, libs []string, err error)

// ErrPending is the error of a PkgConfigFunc that doesn't have the answer
// yet and gets it in the background (the language server): then
// ReadPackageFlags reports that the flags are pending, not an error.
var ErrPending = errors.New("not known yet")

// pkgConfigTimeout limits one run of pkg-config (plus the wait for its output, see waitDelay).
const pkgConfigTimeout = time.Minute

// maxPkgConfigOutput limits what is read of pkg-config's output.
const maxPkgConfigOutput = 1 << 20

// PkgConfig runs pkg-config --cflags and then pkg-config --libs for the
// packages pkgs in the folder dir, like the go command for a #cgo
// pkg-config line: the program is the first word of PKG_CONFIG, or
// pkg-config, and its output is split like a shell would split it
// (splitPkgConfigOutput). pkg-config reads its own environment
// (PKG_CONFIG_PATH, PKG_CONFIG_LIBDIR, PKG_CONFIG_SYSROOT_DIR, ...). The
// package names come from load.ParseDirectives, which allows no flags
// there, and a "--" before them ends pkg-config's options anyway.
func PkgConfig(ctx context.Context, dir string, pkgs []string) (cflags, libs []string, err error) {
	return pkgConfig(ctx, dir, pkgs, nil)
}

// TracePkgConfig returns PkgConfig that also gives each run to trace
// before it starts, as shell lines (cd to the folder, then the command):
// for gocpp build -x.
func TracePkgConfig(trace func(line string)) PkgConfigFunc {
	return func(ctx context.Context, dir string, pkgs []string) ([]string, []string, error) {
		return pkgConfig(ctx, dir, pkgs, trace)
	}
}

// pkgConfig is PkgConfig, with trace (or nil) for TracePkgConfig.
func pkgConfig(ctx context.Context, dir string, pkgs []string, trace func(string)) (cflags, libs []string, err error) {
	prog := "pkg-config"
	if v := os.Getenv("PKG_CONFIG"); v != "" {
		words, err := SplitCommand(v)
		if err != nil {
			return nil, nil, fmt.Errorf("PKG_CONFIG: %v", err)
		}
		if len(words) > 0 {
			prog = words[0]
		}
	}
	if cflags, err = runPkgConfig(ctx, dir, prog, "--cflags", pkgs, trace); err != nil {
		return nil, nil, err
	}
	if libs, err = runPkgConfig(ctx, dir, prog, "--libs", pkgs, trace); err != nil {
		return nil, nil, err
	}
	return cflags, libs, nil
}

// runPkgConfig runs the program prog of pkg-config with mode (--cflags or
// --libs) for pkgs in dir, and returns the flags of its output. The
// messages show the command as it runs, with prog as PKG_CONFIG gives it.
func runPkgConfig(ctx context.Context, dir, prog, mode string, pkgs []string, trace func(string)) ([]string, error) {
	args := append([]string{mode, "--"}, pkgs...)
	words := make([]string, 0, 1+len(args))
	for _, w := range append([]string{prog}, args...) {
		words = append(words, quoteWord(w))
	}
	desc := strings.Join(words, " ")
	if trace != nil {
		trace("cd " + quoteWord(dir))
		trace(desc)
	}
	var stdout, stderr *limitBuffer
	var cmd *exec.Cmd
	var err error
	var rctx context.Context // the context of the last run: its timeout is per run
	// The output is data: the flags. When pkg-config exited with a success status, but its
	// output was not complete in time (a loaded machine), it can't be used: run it again, and
	// wait longer.
	for _, delay := range []time.Duration{waitDelay, retryWaitDelay} {
		stdout = &limitBuffer{max: maxPkgConfigOutput}
		stderr = &limitBuffer{max: maxPkgConfigOutput}
		var cancel context.CancelFunc
		rctx, cancel = context.WithTimeout(ctx, pkgConfigTimeout+delay)
		defer cancel()
		cmd, err = Start(rctx, func() *exec.Cmd {
			cmd := exec.CommandContext(rctx, prog, args...)
			cmd.Dir = dir
			cmd.Stdout, cmd.Stderr = stdout, stderr
			cmd.WaitDelay = delay
			InGroup(cmd) // a process that it left running is killed below, and with the time limit
			return cmd
		})
		if err == nil {
			err = cmd.Wait()
			KillGroup(cmd) // whatever it left running (a process that held the output)
		}
		if !outputCut(cmd, err) {
			break
		}
	}
	switch {
	case rctx.Err() != nil:
		return nil, fmt.Errorf("%s: %w", desc, context.Cause(rctx))
	case err != nil && cmd.ProcessState == nil:
		return nil, fmt.Errorf("cannot run %s: %v", prog, err)
	case err != nil:
		msg := desc + ": " + err.Error()
		for line := range strings.SplitSeq(strings.TrimSpace(stderr.buf.String()+stdout.buf.String()), "\n") {
			if line = strings.TrimRight(line, " \t\r"); line != "" {
				msg += "\n\t" + line
			}
		}
		return nil, errors.New(msg)
	}
	flags, err := splitPkgConfigOutput(bytes.TrimSpace(stdout.buf.Bytes()))
	if err != nil {
		return nil, fmt.Errorf("%s: %v", desc, err)
	}
	return flags, nil
}

// ReadPackageFlags returns the flags of the directives of the package lp
// (see above), and the errors, in source order, at the directives, whose
// positions are in fset. A directive with an error adds no flags. pkgConfig
// runs pkg-config; nil means PkgConfig. When it returns ErrPending, the
// result has pending set and lacks that line's flags.
func ReadPackageFlags(ctx context.Context, fset *token.FileSet, lp *load.Package, pkgConfig PkgConfigFunc) (flags PackageFlags, errs []diag.Diagnostic, pending bool) {
	if pkgConfig == nil {
		pkgConfig = PkgConfig
	}
	for _, f := range lp.Files {
		for _, d := range f.Directives {
			fail := func(err error) {
				errs = append(errs, diag.Diagnostic{
					Pos: fset.Position(d.Pos),
					End: fset.Position(d.Pos + token.Pos(len("//gocpp:")+len(d.Name))),
					Msg: err.Error(),
				})
			}
			switch d.Name {
			case "cflags":
				args := absPaths(slices.Clone(d.Args), lp.Dir, false)
				if err := checkCompilerFlags("CFLAGS", "//gocpp:cflags", args); err != nil {
					fail(err)
					continue
				}
				flags.Compile = append(flags.Compile, args...)
			case "ldflags":
				args := absPaths(slices.Clone(d.Args), lp.Dir, true)
				if err := checkLinkerFlags("LDFLAGS", "//gocpp:ldflags", args); err != nil {
					fail(err)
					continue
				}
				flags.Link = append(flags.Link, args...)
			case "link":
				args := make([]string, len(d.Args))
				for i, lib := range d.Args {
					args[i] = "-l" + lib
				}
				// -lto_library is refused (on macOS, the linker loads that
				// file); the names themselves are checked by the loader.
				if err := checkLinkerFlags("LDFLAGS", "//gocpp:link", args); err != nil {
					fail(err)
					continue
				}
				flags.Link = append(flags.Link, args...)
			case "pkg-config":
				cflags, libs, err := pkgConfig(ctx, lp.Dir, d.Args)
				if errors.Is(err, ErrPending) {
					pending = true
					continue
				}
				if err == nil {
					err = checkCompilerFlags("CFLAGS", "pkg-config --cflags", cflags)
				}
				if err == nil {
					err = checkLinkerFlags("LDFLAGS", "pkg-config --libs", libs)
				}
				if err != nil {
					fail(err)
					continue
				}
				flags.Compile = append(flags.Compile, cflags...)
				flags.Link = append(flags.Link, libs...)
			}
		}
	}
	flags.Directive = len(flags.Compile)
	rerrs, rpending := readRecipes(ctx, lp, pkgConfig, &flags)
	errs = append(errs, rerrs...)
	pending = pending || rpending
	// The folders of the modules whose headers the package imports
	// (load.Package.IncludeDirs): a module header is included by its
	// absolute name, and the headers of its library include each other by
	// their names in the module, so the module's folders are on the include
	// path of every compile and probe of the package. They come from the
	// loader, not from a directive, so the allow-list doesn't check them.
	// The flags of the module's own gocpp.mod do not come here (yet).
	//
	// They are -iquote folders, like the package's own folder in the probes: a
	// quoted include ("imconfig.h", "backends/x.h") finds the module's files,
	// and an angle include (<cstring>, <string.h>) never does, so a file of
	// the module that is named like a system header can't hide the real one.
	// (-I would put the folder before the system folders.) The headers that a
	// library wants found as <x.h> belong to the include folders of its recipe
	// (job 6.1d), which can use -I or -isystem.
	for _, dir := range lp.IncludeDirs {
		flags.Compile = append(flags.Compile, "-iquote", dir)
	}
	return flags, errs, pending
}

// absPaths makes the relative paths of the compile flags args (the link
// flags with link) absolute, from the folder dir, and returns args. These
// are the flags with a path that the allow-list lets through:
//   - compile: -I, -F (joined or with the path after them), -isystem and
//     -include (the path after them);
//   - link: -L and -F (joined or not), and the files that go to the linker
//     as they are (libfoo.a, ./x.o).
//
// The other flags that take the next argument (-D, -U, -l, -framework,
// -arch, -target, -x, -Wl,...) keep it. A path that starts with = or
// $SYSROOT is relative to the sysroot and stays. A relative -include file
// is made absolute only when dir has it: else it is looked up through the
// include path, in the probe and the build alike.
func absPaths(args []string, dir string, link bool) []string {
	sysroot := func(p string) bool { return strings.HasPrefix(p, "=") || strings.HasPrefix(p, "$SYSROOT") }
	abs := func(p string) string {
		if p == "" || filepath.IsAbs(p) || sysroot(p) {
			return p
		}
		return filepath.Join(dir, p)
	}
	joined := []string{"-I", "-F"}
	pathNext := []string{"-I", "-F", "-isystem", "-include"}
	if link {
		joined = []string{"-L", "-F"}
		pathNext = []string{"-L", "-F"}
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case slices.Contains(pathNext, arg) && i+1 < len(args):
			i++
			if arg == "-include" {
				if fi, err := os.Stat(abs(args[i])); err != nil || fi.IsDir() {
					continue
				}
			}
			args[i] = abs(args[i])
		case strings.HasPrefix(arg, "-"):
			if len(arg) > 2 && slices.Contains(joined, arg[:2]) {
				args[i] = arg[:2] + abs(arg[2:])
			} else if i+1 < len(args) && takesNext(arg, link) {
				i++ // -D NAME, -l lib, ...: not a path
			}
		case link:
			args[i] = abs(arg) // a file for the linker
		}
	}
	return args
}

// takesNext reports whether the flag arg takes the next argument, by the
// allow-list's tables.
func takesNext(arg string, link bool) bool {
	if link {
		return slices.Contains(validLinkerFlagsWithNextArg, arg)
	}
	return slices.Contains(validCompilerFlagsWithNextArg, arg)
}
