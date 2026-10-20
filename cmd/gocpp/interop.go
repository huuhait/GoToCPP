package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/icheck"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The interop mode of the commands (Phase 12 step 12.11, ADR-101, docs/spec/typed-interop.md §13).
//
//	-interop=typed|opaque|auto     how the C/C++ names of the program are understood
//
// opaque is Phase 04's way (the C++ compiler decides everything about C++), typed asks clang
// (probes) for the real types: early errors, constants like [cstdio.BUFSIZ]byte, promoted
// members of embedded C++ classes, exact . and ->, and more exact C++; auto is typed when clang++
// is installed and opaque otherwise. The flag wins over GOCPP_INTEROP, which wins over
// GOCPP_TYPED_INTEROP (1 is typed, 0 is opaque); without any of them the mode is
// typedinterop.DefaultMode. The rules live in internal/typedinterop/mode.go (the language server
// uses them too).

// exitNoClang is the exit status of -interop=typed when clang++ is not installed (the status of
// gocpp bindgen without clang++, too).
const exitNoClang = 3

// interopHelp is the paragraph of the help texts that tells about -interop.
const interopHelp = `-interop=typed|opaque|auto chooses how the C/C++ names of the program are
understood. typed asks clang++ for their real types: a mistake in a call of
C++ code is an error in Go's words before the C++ compile, constant macros
work as array lengths and constants ([cstdio.BUFSIZ]byte), the members of
embedded C++ classes are promoted, and the generated C++ is more exact.
opaque leaves all of that to the C++ compiler. auto is typed when clang++ is
installed and opaque otherwise, with no message (-v says which). typed
without clang++ is an error (status 3). GOCPP_INTEROP takes the same
values; the flag wins. See docs/spec/typed-interop.md, section 13.

`

// interopFlag is the value of -interop of the commands that check packages.
type interopFlag struct {
	value string
}

// register adds -interop to flags.
func (f *interopFlag) register(flags *flag.FlagSet) {
	flags.Var(interopValue{f}, "interop", "how C/C++ names are understood: typed (clang answers the types), opaque (the C++ compiler decides) or auto (typed when clang++ is installed); default: GOCPP_INTEROP, else "+typedinterop.DefaultMode.String())
}

type interopValue struct{ f *interopFlag }

func (v interopValue) String() string {
	if v.f == nil {
		return ""
	}
	return v.f.value
}

func (v interopValue) Set(s string) error {
	if _, ok := typedinterop.ParseMode(s); !ok {
		return errors.New("want typed, opaque or auto")
	}
	v.f.value = s
	return nil
}

// typedState is the typed pipeline of one command: the mode, the decision about clang, the cache
// of compiler runs in memory that the hooks and the analysis share, and the reports that the
// analysis of check leaves for the code generation of build, run and emit.
type typedState struct {
	setting typedinterop.Setting
	dec     typedinterop.Decision
	runner  *probe.Runner

	mu      sync.Mutex
	reports map[*check.Package]*typedinterop.Report
}

// report returns the analysis of p, or nil (not analyzed: opaque mode, a package of the standard
// library, a probe that failed, a cross build with headers other than the C++ library's).
func (s *typedState) report(p *check.Package) *typedinterop.Report {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reports[p]
}

func (s *typedState) setReport(p *check.Package, r *typedinterop.Report) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.reports == nil {
		s.reports = make(map[*check.Package]*typedinterop.Report)
	}
	s.reports[p] = r
}

// interopSetting returns the mode of the command: its flag, else the environment.
func (c *checkCommand) interopSetting() (typedinterop.Setting, error) {
	return typedinterop.ResolveMode(c.interop.value, os.Getenv)
}

// prepareTyped decides, after the packages are loaded and before they are checked, whether typed
// interop runs (typedinterop.Decide) and makes c.typed. It returns false and the exit status when
// the command must stop: a mode that does not parse (2), or typed without clang++ (3). With -v the
// note about the decision is printed.
func (c *checkCommand) prepareTyped(prog *load.Program) (int, bool) {
	if c.typed != nil {
		return 0, true
	}
	set, err := c.interopSetting()
	if err != nil {
		fmt.Fprintf(c.stderr, "gocpp: %v\n", err)
		return exitInternal, false
	}
	ctx := c.context()
	needed := typedinterop.Needed(prog.Packages)
	if needed && set.Mode != typedinterop.Opaque {
		// Looking for clang++ asks the compiler for its version and its macros: that answer is kept
		// between runs, as in gocpp build, so that a warm check does not start the compiler twice.
		build.SetDetectCache(cacheDir())
	}
	dec := typedinterop.DecideFor(ctx, set, needed, c.goos, c.goarch)
	if dec.Err != nil {
		fmt.Fprintf(c.stderr, "gocpp: %v\n", dec.Err)
		return exitNoClang, false
	}
	if c.verbose && dec.Note != "" {
		fmt.Fprintln(c.stderr, dec.Note)
	}
	c.typed = &typedState{setting: set, dec: dec, runner: new(probe.Runner)}
	return 0, true
}

// context returns the context of the command (the tests of the library have none).
func (c *checkCommand) context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// typedOn reports whether typed interop runs in this command.
func (c *checkCommand) typedOn() bool {
	return c.typed != nil && c.typed.dec.On && !c.noTyped
}

// typedHooks sets the hooks of typed interop in cfg: the constants of gap E1 and the second pass
// for the promoted members of gap E2. The hidden commands of typed interop have the second pass
// whatever the mode is.
func (c *checkCommand) typedHooks(cfg *check.Config) {
	if !c.typedOn() && !c.typedMembers {
		return
	}
	var rn *probe.Runner
	if c.typed != nil {
		rn = c.typed.runner
	}
	consts, members := typedinterop.Hooks(typedinterop.HookOptions{GOOS: c.goos, GOARCH: c.goarch, Runner: rn})
	if c.typedOn() {
		cfg.Constants = consts
	}
	cfg.CppMembers = members
}

// analyzeTyped runs the typed pipeline on the packages of the program that checked without
// errors: the probes and declarations of each package that imports a header (typedinterop.Analyze
// with the options of the code generation), then the interop checker on that report, whose
// errors and warnings join the package's errors. The reports stay for the code generation
// (attachTyped). A package that can't be analyzed keeps the opaque pipeline: with auto the reason
// is a note for -v; with typed it is a warning on standard error, because the user asked for
// typed. A build for another system than this one is typed only for the headers of the C++
// library and gets no checker errors (the probes read this machine's headers), except a build for
// windows/amd64, whose probes run a clang with the MinGW-w64 headers and the Win32 defaults
// (typedinterop.ProbesTarget, ADR-116).
func (c *checkCommand) analyzeTyped(prog *load.Program, r *check.Result) {
	if !c.typedOn() {
		return
	}
	// The probes cannot see the headers of a target that is not probed.
	cross := !typedinterop.ProbesTarget(c.goos, c.goarch)
	// The family of the build compiler: g++ makes the checker's errors warnings.
	fam := build.Clang
	if !cross {
		if cxx, err := build.FindCXXFor(c.context(), c.goos, c.goarch); err == nil {
			fam = cxx.Family
		}
	}
	var todo []*check.Package
	for _, lp := range packageList(prog) {
		p := r.Package(lp)
		if p == nil || lp.Std || c.dropped[lp] || !p.Checked() || p.Errors.HasErrors() {
			continue
		}
		if len(typedinterop.UnitIncludes(p)) == 0 {
			continue
		}
		if cross && !typedinterop.StdHeadersOnly(p) {
			continue
		}
		todo = append(todo, p)
	}
	// The packages are independent; the probes bound the compiler runs of the whole process.
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(2, min(4, runtime.NumCPU()/2)))
	msgs := make([]string, len(todo))
	for i, p := range todo {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			msgs[i] = c.analyzePackage(prog, p, cross, fam)
		}()
	}
	wg.Wait()
	for _, m := range msgs {
		if m != "" {
			fmt.Fprintln(c.stderr, m)
		}
	}
}

// analyzePackage is analyzeTyped for one package; it returns the line to print, or "".
func (c *checkCommand) analyzePackage(prog *load.Program, p *check.Package, cross bool, fam build.Family) string {
	opts := typedinterop.Options{
		Decls: true, StringToCStr: true, Target: ftype.LP64, Runner: c.typed.runner,
		GOOS: c.goos, GOARCH: c.goarch,
	}
	if c.goos == "windows" {
		opts.Target = ftype.LLP64
	}
	rep, err := typedinterop.Analyze(c.context(), p, prog.Fset, opts)
	if err != nil {
		if c.context().Err() != nil {
			return "" // interrupted: check() reports it
		}
		if !errors.Is(err, typedinterop.ErrNoClang) && (c.typed.setting.Mode == typedinterop.Typed || c.verbose) {
			return fmt.Sprintf("gocpp: typed interop: %v (using opaque mode for %s)", err, p.Load.ImportPath)
		}
		return ""
	}
	c.typed.setReport(p, rep)
	if cross {
		return ""
	}
	res := icheck.CheckReport(p, prog.Fset, rep, fam)
	for _, d := range res.Diags {
		p.Errors.Add(d)
	}
	if res.Skipped != "" && c.verbose {
		return fmt.Sprintf("interop: %s: the early errors of typed mode are skipped: %s", p.Load.ImportPath, res.Skipped)
	}
	return ""
}
