package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"go/token"
	"io"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lower"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
	"github.com/huuhait/gocpp/stdlib"
)

// checkUsage is the help text of "gocpp check". "gocpp check -h" prints
// it, followed by the flags.
const checkUsage = `usage: gocpp check [-e] [-types] [packages]

gocpp check parses and type-checks the packages and prints their errors
the way go build prints compile errors: a line "# <import path>" before
the errors of each package that has some ("# command-line-arguments" for
.gp files named on the command line), then one line per error:

	./main.gp:12:5: undefined: count

It prints at most 10 errors per package, then "too many errors"; -e
prints all of them. A package with syntax errors is not type-checked,
and the errors of a package that imports a package with errors are not
shown. When the packages can't be loaded (an import that can't be found,
an import cycle, an error in a package clause or in the imports), gocpp
check prints only those errors, without "#" lines, and checks nothing,
like go build. Warnings (file:line:col: warning: message) don't change
the exit status.

gocpp check writes the errors to standard error and exits with status 1
if there is one, else 0. A package is a folder (".", "./geom"), a list of
.gp files of one folder, an import path, or a pattern with "..."
("./..." is the current folder and every package folder under it, like
in the go command). Without packages, it checks the package in the
current folder. The build constraints are for the system that runs
gocpp, or for the GOOS and GOARCH environment variables.

Features of a later phase of gocpp's plan are errors ("... not supported
yet (Phase 03)"). For gocpp's own development and tests,
GOCPP_GATE_PHASE=NN (two digits) accepts them up to phase NN; build, run
and emit read it too. It can't turn the gates of finished phases back on.

-types prints the type of every expression of the packages named on the
command line to standard output, one per line in source order: its
position, its mode (value, var, const, type, ...), its source text and
its type. A value that comes from C or C++ prints as opaque. A
definition (def var, def func, ...) prints the type of the new name.

The flags are:
`

// runCheck is "gocpp check" (phase 01 README §2.12): it loads the
// packages (load.Load), type-checks them (check.Check) and prints the
// errors like go build.
func runCheck(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	c := &checkCommand{stdout: stdout, stderr: stderr}
	flags := flag.NewFlagSet("gocpp check", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.BoolVar(&c.all, "e", false, "report all errors, not only the first 10 of each package")
	flags.BoolVar(&c.types, "types", false, "print the type of every expression to standard output")
	c.diagOpts.register(flags)
	flags.Usage = func() {
		fmt.Fprint(stderr, checkUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0 // like gocpp fmt -h
		}
		return 2 // flag has printed the error and the usage
	}
	var err error
	if c.goos, c.goarch, err = target(); err != nil {
		fmt.Fprintf(stderr, "gocpp: %v\n", err) // go exits with 2 too
		return 2
	}
	if c.dir, err = os.Getwd(); err != nil {
		fmt.Fprintf(stderr, "gocpp check: %v\n", err)
		return 1
	}
	var stop func()
	c.ctx, c.stopped, stop = signalContext()
	defer stop()
	return c.run(flags.Args())
}

// signalContext returns a context that SIGINT, SIGTERM or SIGHUP cancels,
// a function that returns the signal that came (or nil), and a function
// that ends the signal handling. After the first signal, gocpp no longer
// catches them: a second ^C kills it the default way.
func signalContext() (context.Context, func() os.Signal, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	notify(sigs, stopSigs...)
	var got atomic.Pointer[os.Signal]
	done := make(chan struct{})
	go func() {
		select {
		case sig := <-sigs:
			got.Store(&sig)
			signal.Stop(sigs)
			cancel()
		case <-done:
		}
	}()
	stopped := func() os.Signal {
		if p := got.Load(); p != nil {
			return *p
		}
		return nil
	}
	var once sync.Once
	return ctx, stopped, func() {
		once.Do(func() {
			signal.Stop(sigs)
			close(done)
			cancel()
		})
	}
}

// A checkCommand is one run of "gocpp check".
type checkCommand struct {
	all           bool        // -e
	diagOpts      diagOptions // -color, -excerpt
	types         bool        // -types
	tests         bool        // load the _test.gp files too (gocpp vet)
	dir           string      // the current folder
	goos, goarch  string      // the target of the build constraints
	stdout        io.Writer
	stderr        io.Writer
	fset          *token.FileSet
	files         bool                   // the patterns are a list of .gp files
	headers       map[string]*header     // see header
	dropped       map[*load.Package]bool // see droppedPackages
	parserErrors  map[string]*fileErrors // see parserErrorsOf
	reportedPaths map[string]bool        // the unresolved imports reported

	// ctx stops the C++ compiler runs of the namespace probe when gocpp
	// gets SIGINT, SIGTERM or SIGHUP; stopped then returns the signal (see
	// signalContext, and the driver of build, run and emit).
	ctx     context.Context
	stopped func() os.Signal

	// fs, if not nil, is where the loader reads files and folders instead of the operating system
	// directly (load.Config.FS): gocpp build records what it reads (build.FSRecorder).
	fs load.FileSystem

	// pkgConfig, if not nil, runs pkg-config for the flags of the packages
	// (check.Config.PkgConfig): gocpp build -x and run -x print the runs.
	pkgConfig build.PkgConfigFunc

	// kinds: the namespace probe asks the kind probe also about the C/C++
	// paths that the lowerer will ask about (lower.ProbePaths), so that
	// the build runs the compiler once per include set. build, run and
	// emit set it; check and the language server keep the cheaper
	// namespace mode.
	kinds bool
}

// run checks the packages of the patterns and returns the exit status.
func (c *checkCommand) run(patterns []string) int {
	prog, r, code := c.check(patterns)
	if r == nil || !c.types {
		return code
	}
	for _, root := range prog.Roots {
		if p := r.Package(root); p != nil && p.Checked() && !c.dropped[root] {
			if err := printTypes(c.stdout, c.dir, prog.Fset, p); err != nil {
				fmt.Fprintf(c.stderr, "gocpp check: %v\n", err)
				code = 1
			}
		}
	}
	return code
}

// allowInternalEnv names the environment variable that lets a program import the standard
// library's internal packages (load.Config.AllowStdInternal). Only the test harness sets it, for the
// tests that say "// std-internal:".
const allowInternalEnv = "GOCPP_ALLOW_INTERNAL"

// check loads and type-checks the packages of the patterns and prints
// their errors and warnings to stderr, like go build. It returns the
// program, the result of the type checking (nil when the packages couldn't
// be loaded: then nothing was checked) and the exit status: 1 if there is
// an error (a phase gate counts), else 0. gocpp build, run and emit use
// it too, so that they fail exactly like gocpp check. GOCPP_GATE_PHASE
// (gatePhase) can open the gates of a later phase; a malformed value is exit
// status 2.
func (c *checkCommand) check(patterns []string) (*load.Program, *check.Result, int) {
	phase, err := gatePhase(os.Getenv(gatePhaseEnv))
	if err != nil {
		fmt.Fprintf(c.stderr, "gocpp: %v\n", err)
		return &load.Program{Fset: token.NewFileSet(), Errors: new(diag.List)}, nil, exitInternal
	}
	m := matchPatterns(c.dir, patterns)
	for _, w := range m.warnings {
		fmt.Fprintln(c.stderr, w)
	}
	prog := &load.Program{Fset: token.NewFileSet(), Errors: new(diag.List)}
	if len(patterns) == 0 || len(m.patterns) > 0 {
		prog = load.Load(load.Config{
			Dir:       c.dir,
			Stdlib:    stdlib.FS(),
			StdlibDir: stdlibDir,
			GoStd:     stdlib.IsStd,
			GOOS:      c.goos,
			GOARCH:    c.goarch,
			FS:        c.fs,
			// GOCPP_ALLOW_INTERNAL=1: the tests of the standard library's internal packages
			// (tests/README.md, "// std-internal:") import them from test programs.
			AllowStdInternal: os.Getenv(allowInternalEnv) == "1",
			// Always all the syntax errors: firstSyntaxErrorPerLine keeps
			// what the go compiler shows, and the Printer stops at 10.
			AllErrors: true,
			Tests:     c.tests,
			// The syntax of Phase 10 (F!(x), func f!()) parses only where
			// the gates allow it; before, it is a syntax error that the
			// gates explain (internal/check, gateUnchecked).
			Phase10Syntax: phase >= 10,
		}, m.patterns...)
	}
	c.fset = prog.Fset
	c.files = m.files
	c.headers = make(map[string]*header)
	c.parserErrors = make(map[string]*fileErrors)
	c.dropped = droppedPackages(prog, m)
	c.warnNoMatch(m)

	if len(m.errors) > 0 || c.loadFailed(prog) {
		return prog, nil, c.printLoadErrors(prog, m)
	}
	// The warnings of the load (a cxx recipe in gocpp.mod for a module that
	// is not used there): the errors are printed above.
	if len(prog.Errors.Diagnostics()) > 0 {
		c.print(c.stderr, prog.Errors, true)
	}

	// The namespace probe runs the C++ compiler of the target (GOOS,
	// GOARCH) for the files with header imports. ^C (or SIGTERM, SIGHUP)
	// stops it: the compiler runs in its own process group, which the
	// terminal's signal doesn't reach, so the canceled context is what
	// kills it. Then gocpp prints nothing more and dies of the signal
	// (stopStatus), like build and run.
	cfg := check.Config{Phase: phase, GOOS: c.goos, GOARCH: c.goarch, Context: c.ctx, PkgConfig: c.pkgConfig}
	if c.kinds {
		paths := lower.ProbePaths(prog)
		cfg.Probe = func(ctx context.Context, cxx build.CXX, dir string, flags, includes, cands []string) (interop.Result, error) {
			all := slices.Clip(cands)
			for _, p := range paths(dir, includes) {
				if !slices.Contains(cands, p) {
					all = append(all, p)
				}
			}
			return interop.ProbeKinds(ctx, cxx, dir, flags, includes, all)
		}
	}
	r := check.Check(prog, cfg)
	if c.ctx != nil && c.ctx.Err() != nil {
		if sig := c.stopped(); sig != nil {
			return prog, nil, stopStatus(sig)
		}
		return prog, nil, exitErrors
	}
	failed := false
	// broken holds the packages with errors and the packages that import
	// one, directly or not. Like go build, which doesn't compile a package
	// whose dependency failed, gocpp check shows only the warnings of the
	// second kind: their errors could follow from the broken dependency.
	// (check skips a package only for load errors, and it still checks a
	// package whose dependency has type errors.) A package whose only
	// errors are phase gates ("... not supported yet (Phase 03)") is not
	// broken: the gates reject features that the types describe well, so
	// the packages that import it are checked as usual, and all the gates
	// of a program show at once. packageList has each package after its
	// imports.
	broken := make(map[*load.Package]bool)
	for _, lp := range packageList(prog) {
		p := r.Package(lp)
		if c.dropped[lp] || p == nil {
			continue
		}
		errs := p.Errors
		switch {
		case importsBroken(p.Load, broken):
			broken[p.Load] = true
			errs = filter(p.Load.Errors, isWarning)
		case !p.Checked():
			errs = c.firstSyntaxErrorPerLine(p.Load, errs)
		}
		if len(errs.Diagnostics()) == 0 {
			continue
		}
		fmt.Fprintf(c.stderr, "# %s\n", p.Load.ImportPath)
		c.print(c.stderr, errs, c.all)
		failed = failed || errs.HasErrors()
		if !p.Checked() || slices.ContainsFunc(errs.Diagnostics(), isBreaking) {
			broken[p.Load] = true
		}
	}
	if failed {
		return prog, r, 1
	}
	return prog, r, 0
}

// packageList returns the packages of prog in the order of go's
// PackageList: from the roots, in the order of the patterns, each package
// after its imports, which come in the order of their import paths. go
// build -p 1 prints its errors in this order. (With more than one job at a
// time, the order of the packages that compile at the same time depends
// on which one finishes first.)
func packageList(prog *load.Program) []*load.Package {
	var list []*load.Package
	seen := make(map[*load.Package]bool)
	var visit func(p *load.Package)
	visit = func(p *load.Package) {
		if seen[p] {
			return
		}
		seen[p] = true
		for _, path := range slices.Sorted(maps.Keys(p.Imports)) {
			visit(p.Imports[path])
		}
		list = append(list, p)
	}
	for _, p := range prog.Roots {
		visit(p)
	}
	return list
}

// filter returns the diagnostics of l for which keep returns true, in the
// order they were found (for the limit of 10 errors); keep sees them in
// that order.
func filter(l *diag.List, keep func(diag.Diagnostic) bool) *diag.List {
	return l.Rewrite(func(d diag.Diagnostic) []diag.Diagnostic {
		if keep(d) {
			return []diag.Diagnostic{d}
		}
		return nil
	})
}

// isBreaking reports whether d is an error that is not a phase gate.
func isBreaking(d diag.Diagnostic) bool {
	return d.Severity == diag.Error && !d.Gate
}

func isWarning(d diag.Diagnostic) bool {
	return d.Severity == diag.Warning
}

// importsBroken reports whether the package p imports a package of broken.
func importsBroken(p *load.Package, broken map[*load.Package]bool) bool {
	for _, dep := range p.Imports {
		if broken[dep] {
			return true
		}
	}
	return false
}

// print prints the diagnostics l like go build. all prints every error.
func (c *checkCommand) print(w io.Writer, l *diag.List, all bool) {
	p := c.diagOpts.printer(w, c.dir, all)
	if err := p.Print(w, l); err != nil {
		fmt.Fprintf(c.stderr, "gocpp check: %v\n", err)
	}
}

// droppedPackages returns the root packages that the go command would
// leave out: a pattern with "..." matched them and no other pattern names
// them, no package imports them, and their folders have no .gp files to
// build (load.Package.NoFiles: only test files, or only files that the
// build constraints exclude). A folder with a file that can't be read or
// a broken //go:build line is not dropped.
func droppedPackages(prog *load.Program, m *patternMatch) map[*load.Package]bool {
	imported := make(map[*load.Package]bool)
	for _, p := range prog.Packages {
		for _, dep := range p.Imports {
			imported[dep] = true
		}
	}
	dropped := make(map[*load.Package]bool)
	for _, p := range prog.Roots {
		wild := m.wildDirs[p.Dir] || m.wildPaths[p.ImportPath]
		explicit := m.explicitDirs[p.Dir] || m.explicitPaths[p.ImportPath]
		if wild && !explicit && p.NoFiles != load.HasFiles && !imported[p] {
			dropped[p] = true
		}
	}
	return dropped
}

// warnNoMatch prints go's warning for each pattern with "..." that
// matched no packages, or only packages that droppedPackages dropped
// because they have no files or only excluded ones (go's NoGoError).
func (c *checkCommand) warnNoMatch(m *patternMatch) {
	gone := make(map[string]bool)
	for p := range c.dropped {
		// go matches a folder of tests, and then leaves it out of the
		// build without a word (omitTestOnly).
		if p.NoFiles != load.OnlyTestFiles {
			gone[p.Dir], gone[p.ImportPath] = true, true
		}
	}
	for _, pat := range m.wild {
		if !slices.ContainsFunc(m.byPattern[pat], func(k string) bool { return !gone[k] }) {
			fmt.Fprintf(c.stderr, "gocpp: warning: %q matched no packages\n", pat)
		}
	}
}

// loadFailed reports whether the packages can't be loaded as a whole, as
// "go list" would fail inside go build: a pattern error (prog.Errors), or
// a package with a load error (see isLoadError) or a syntax error in the
// header of a file.
//
// The loader doesn't sort its errors into kinds, so gocpp check goes by
// where an error is. An import that can't be found, an import cycle, a
// folder without .gp files, two package names in one folder, a syntax
// error in the package clause or the imports, a bad import or a //gocpp:
// directive error (go list reads #cgo lines too) are all there. The errors
// after the imports, such as a syntax error in a function body, don't
// stop the other packages: go build compiles them.
func (c *checkCommand) loadFailed(prog *load.Program) bool {
	if prog.Errors.HasErrors() {
		return true
	}
	for _, p := range prog.Packages {
		if c.dropped[p] {
			continue
		}
		for _, f := range p.Files {
			if c.header(f.Path).first != nil {
				return true
			}
		}
		for _, d := range p.Errors.Diagnostics() {
			if c.isLoadError(d) {
				return true
			}
		}
	}
	return false
}

// printLoadErrors prints the errors of the patterns and of a program that
// loadFailed, like go build prints the errors of go list, and returns the
// exit status, 1. go list goes through the packages in the order of
// PackageList: the roots in the order of the patterns, each package after
// its imports, which come by import path. It shows each package once, and
// each import that can't be resolved once, at its first place. Of the
// syntax errors of a package, it shows only the first one (go/build stops
// at the first file with one). There are no "# <import path>" lines and no
// limit, and the files of the current folder have no "./". The errors
// after the header of a file (a syntax error in a function body) wait, as
// go list doesn't parse the bodies. Warnings are shown: they don't depend
// on the failure.
func (c *checkCommand) printLoadErrors(prog *load.Program, m *patternMatch) int {
	var b bytes.Buffer
	for _, e := range m.errors {
		fmt.Fprintln(&b, e)
	}
	c.print(&b, prog.Errors, true)
	c.reportedPaths = make(map[string]bool)
	seen := make(map[*load.Package]bool)
	for _, p := range prog.Roots {
		if !c.dropped[p] {
			c.visit(&b, p, seen)
		}
	}
	// The go command prints these positions with base.ShortPath, which
	// gives "main.go" where a compile error has "./main.go" (".\main.go"
	// on Windows).
	sc := bufio.NewScanner(&b)
	for sc.Scan() {
		fmt.Fprintln(c.stderr, strings.TrimPrefix(sc.Text(), "."+string(filepath.Separator)))
	}
	return 1
}

// visit prints the load errors of p and of the packages it imports, for
// printLoadErrors.
func (c *checkCommand) visit(w io.Writer, p *load.Package, seen map[*load.Package]bool) {
	if seen[p] {
		return
	}
	seen[p] = true
	// The place of the first import of each path, where the loader
	// reports an import that it can't resolve.
	first := make(map[string]token.Position)
	for _, f := range p.Files {
		for _, spec := range f.Info.Packages {
			if path, err := strconv.Unquote(spec.Path.Value); err == nil {
				if _, ok := first[path]; !ok {
					first[path] = c.fset.Position(spec.Pos())
				}
			}
		}
	}
	own := slices.DeleteFunc(p.Errors.Diagnostics(), func(d diag.Diagnostic) bool {
		return d.Severity != diag.Warning && !c.isLoadError(d)
	})
	paths := slices.Sorted(maps.Keys(first))
	for _, path := range paths {
		if dep := p.Imports[path]; dep != nil {
			c.visit(w, dep, seen)
			continue
		}
		if path == "unsafe" {
			continue
		}
		// An import that the loader couldn't resolve: its errors are at
		// the import.
		at := first[path]
		l := new(diag.List)
		own = slices.DeleteFunc(own, func(d diag.Diagnostic) bool {
			if d.Severity == diag.Error && d.Pos == at {
				l.Add(d)
				return true
			}
			return false
		})
		if !c.reportedPaths[path] {
			c.reportedPaths[path] = true
			c.print(w, l, true)
		}
	}
	// Of the syntax errors, only the first one of the headers: go/build
	// keeps the first error of the first file with one, and go list
	// doesn't parse further.
	l := new(diag.List)
	for _, f := range p.Files {
		if first := c.header(f.Path).first; first != nil {
			l.Add(*first)
			break
		}
	}
	for _, d := range own {
		if !c.isSyntaxError(d) {
			l.Add(d)
		}
	}
	c.print(w, l, true)
}

// isSyntaxError reports whether d is an error of the parser or of its
// scanner (see parserErrorsOf).
func (c *checkCommand) isSyntaxError(d diag.Diagnostic) bool {
	if d.Pos.Filename == "" || d.Severity != diag.Error {
		return false
	}
	fe := c.parserErrorsOf(d.Pos.Filename)
	k := errorAt{d.Pos.Filename, d.Pos.Line, d.Pos.Column, d.Msg}
	return fe.parser[k] || fe.scanner[k]
}

// A header is the header of a file: the part of the file that go list
// reads, from the start of the file to the end of its imports (with the
// //gocpp: directives before the package clause).
type header struct {
	read bool // whether the file could be read

	// end is the offset of the end of the import declarations (or of the
	// package clause), as a parse with parser.ImportsOnly of what
	// go/build reads (goHeader) finds them: the errors before it are go
	// list's.
	end int

	// first is go list's error for the file, or nil: the first syntax
	// error of that parse (without parser.AllErrors, as go/build parses),
	// or go/build's error for a NUL byte.
	first *diag.Diagnostic
}

// header returns the header of the file name.
func (c *checkCommand) header(name string) *header {
	if h := c.headers[name]; h != nil {
		return h
	}
	h := &header{}
	c.headers[name] = h
	src, err := os.ReadFile(name)
	if err != nil {
		return h
	}
	h.read = true
	n, nul := goHeader(src)
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src[:n], parser.ImportsOnly|parser.ParseComments)
	if f.Name.Pos().IsValid() {
		h.end = fset.Position(f.Name.End()).Offset
	}
	for _, d := range f.Decls {
		h.end = max(h.end, fset.Position(d.End()).Offset)
	}
	var list scanner.ErrorList
	switch {
	case nul && c.files:
		// go: go/build's message for a file named on the command line
		// is fatal: "go: reading a.go: unexpected NUL in input".
		h.first = &diag.Diagnostic{Msg: "gocpp: reading " + diag.ShortPath(c.dir, name) + ": unexpected NUL in input"}
	case nul:
		h.first = &diag.Diagnostic{Msg: "read " + name + ": unexpected NUL in input"}
	case errors.As(err, &list) && len(list) > 0:
		h.first = &diag.Diagnostic{Pos: list[0].Pos, Msg: list[0].Msg}
	}
	return h
}

// isLoadError reports whether the diagnostic d of a package is an error
// that makes go list fail: an error without a place (an import cycle, a
// folder that can't be read), in a file that can't be read, or before the
// end of the imports of a file (the errors of the imports and of the
// directives, and the syntax errors there). Warnings never are.
func (c *checkCommand) isLoadError(d diag.Diagnostic) bool {
	if d.Severity != diag.Error {
		return false
	}
	if d.Pos.Filename == "" || d.Pos.Line == 0 {
		return true
	}
	h := c.header(d.Pos.Filename)
	return !h.read || d.Pos.Offset < h.end
}

// firstSyntaxErrorPerLine returns the errors of the package p, which was
// not type-checked, with only the first error of the parser on each line,
// like the go compiler, with and without -e: it drops a syntax error on
// the line of the syntax error before it (cmd/compile/internal/base,
// ErrorfAt). The loader parses with parser.AllErrors, which can report
// several parser errors on one line. The errors of the scanner ("unknown
// escape sequence") all stay, in the compiler too.
//
// The loader doesn't mark which errors come from the parser, so this
// parses and scans the files with errors again to find out. The result
// keeps the order in which the errors were found, for the limit of 10.
func (c *checkCommand) firstSyntaxErrorPerLine(p *load.Package, errs *diag.List) *diag.List {
	type line struct {
		file string
		line int
	}
	seen := make(map[line]bool)
	failed := make(map[string]bool) // the files with a syntax error so far
	return filter(errs, func(d diag.Diagnostic) bool {
		name := d.Pos.Filename
		if !slices.ContainsFunc(p.Files, func(f *load.File) bool { return f.Path == name }) {
			return true
		}
		pe := c.parserErrorsOf(name)
		if d.Severity != diag.Error {
			return true
		}
		if !pe.parser[errorAt{name, d.Pos.Line, d.Pos.Column, d.Msg}] {
			failed[name] = failed[name] || pe.scanner[errorAt{name, d.Pos.Line, d.Pos.Column, d.Msg}]
			return true
		}
		// The compiler drops a parser error at the end of the file after
		// another error ("avoid meaningless follow-up errors", syntax
		// package), and one on the line of the parser error before it.
		// Its scanner skips a character that it reports (U+0023 '#'), so
		// its parser never sees the ILLEGAL token that go/scanner gives
		// go/parser there: a parser error at the place of a scanner error
		// goes too.
		//
		// go/parser sorts its errors, so the first parser error of a line
		// here is not always the one it found first. A parse without
		// AllErrors keeps the one found first (if no scanner error came
		// before it on the line): that one wins.
		l := line{name, d.Pos.Line}
		k := errorAt{name, d.Pos.Line, d.Pos.Column, d.Msg}
		if want, ok := pe.first[d.Pos.Line]; ok && want != k {
			return false
		}
		if seen[l] || failed[name] && d.Pos.Offset == pe.size || pe.scannerAt[[2]int{d.Pos.Line, d.Pos.Column}] {
			return false
		}
		seen[l], failed[name] = true, true
		return true
	})
}

// An errorAt is the place and the message of a syntax error.
type errorAt struct {
	file      string
	line, col int
	msg       string
}

// fileErrors are the syntax errors of a file, from a parse with
// parser.AllErrors: those of the parser itself, and those of its scanner.
type fileErrors struct {
	parser, scanner map[errorAt]bool
	scannerAt       map[[2]int]bool // the line and column of each scanner error
	size            int             // the size of the file: the offset of its end

	// first holds, by line, the parser error that a parse without
	// AllErrors reports on the line, which is the one it found first.
	first map[int]errorAt
}

// parserErrorsOf returns the syntax errors of the file name, for
// firstSyntaxErrorPerLine: it parses the file with parser.AllErrors and
// scans it alone, so a parse error that the scan doesn't find too comes
// from the parser.
func (c *checkCommand) parserErrorsOf(name string) *fileErrors {
	if fe, ok := c.parserErrors[name]; ok {
		return fe
	}
	fe := &fileErrors{parser: make(map[errorAt]bool), scanner: make(map[errorAt]bool), scannerAt: make(map[[2]int]bool), first: make(map[int]errorAt), size: -1}
	c.parserErrors[name] = fe
	src, err := os.ReadFile(name)
	if err != nil {
		return fe
	}
	fe.size = len(src)
	fset := token.NewFileSet()
	_, err = parser.ParseFile(fset, name, src, parser.ParseComments|parser.AllErrors)
	var list scanner.ErrorList
	if !errors.As(err, &list) {
		return fe
	}
	var s scanner.Scanner
	s.Init(fset.AddFile(name, -1, len(src)), src, func(pos token.Position, msg string) {
		fe.scanner[errorAt{pos.Filename, pos.Line, pos.Column, msg}] = true
		fe.scannerAt[[2]int{pos.Line, pos.Column}] = true
	}, scanner.ScanComments)
	for {
		if _, tok, _ := s.Scan(); tok == token.EOF {
			break
		}
	}
	for _, e := range list {
		if k := (errorAt{e.Pos.Filename, e.Pos.Line, e.Pos.Column, e.Msg}); !fe.scanner[k] {
			fe.parser[k] = true
		}
	}
	_, err = parser.ParseFile(token.NewFileSet(), name, src, parser.ParseComments)
	if errors.As(err, &list) {
		for _, e := range list {
			if k := (errorAt{e.Pos.Filename, e.Pos.Line, e.Pos.Column, e.Msg}); fe.parser[k] {
				fe.first[e.Pos.Line] = k
			}
		}
	}
	return fe
}
