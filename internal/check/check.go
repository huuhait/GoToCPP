package check

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// GoVersion is the Go language version of every package: the loop
// variables of Go 1.22, range over int and over functions, and so on
// (types.Config.GoVersion). A file can still ask for an older version with
// a //go:build go1.N line, as in Go (types.Info.FileVersions).
const GoVersion = "go1.25"

// A Config says how Check checks.
type Config struct {
	// Phase is the last finished phase of the plan: the phase gates
	// (gates.go) reject in user packages every feature of a later phase,
	// with a message that names it. Zero means version.Phase, and
	// LastPhase turns the gates off (but not the ones of the features
	// that gocpp never has). Phase 02's subset is always allowed, so the
	// phases 00 to 02 give the same result.
	Phase int

	// GOOS and GOARCH are the target that the program was loaded for
	// (load.Config.GOOS and GOARCH): the namespace probe must run the C++
	// compiler of that target, whose headers the files include
	// (<windows.h> for windows). "" means the system that runs gocpp
	// (runtime.GOOS, runtime.GOARCH). Only the default CXX uses them.
	GOOS, GOARCH string

	// Context, if not nil, can stop the C++ compiler runs of the
	// namespace probe (see probeNamespaces). When it is canceled before
	// or during the probe, Check stops right after the probe and returns
	// a Result without packages (Result.Package returns nil): the caller
	// is stopping. nil means context.Background(). (Check can't take a
	// context argument without changing its callers; the probe is the
	// only part that waits.)
	Context context.Context

	// CXX returns the C++ compiler that the namespace probe runs, which
	// must be the compiler of the load target. Check calls it at most
	// once, and only when a file of a package that it checks imports a
	// header (a program without header imports needs no compiler). nil
	// means build.FindCXXFor with GOOS and GOARCH: for the host,
	// GOCPP_CXX, CXX, clang++, g++; for windows/amd64 from another
	// system, GOCPP_CXX or x86_64-w64-mingw32-g++. When it fails, each
	// package with header imports gets one error, at its first header
	// import ("cannot check the header imports: no C++ compiler found:
	// ..."), and its namespace uses stay opaque.
	CXX func(ctx context.Context) (build.CXX, error)

	// probe, if not nil, replaces interop.ProbeNamespaces (for the tests,
	// whose headers mostly don't exist).
	probe probeFunc
}

// A Result is what Check found out about a loaded program.
type Result struct {
	// Fset is the FileSet of the program (load.Program.Fset).
	Fset *token.FileSet

	// Packages are the packages of the program, in the order of
	// load.Program.Packages: each one after the packages it imports.
	Packages []*Package

	byLoad map[*load.Package]*Package
}

// Package returns the checked package of the loaded package p, or nil if
// p is not a package of the program.
func (r *Result) Package(p *load.Package) *Package {
	return r.byLoad[p]
}

// A Package is a loaded package after type checking.
type Package struct {
	// Load is the loaded package: its files (whose ASTs go/types
	// checked), imports and load errors.
	Load *load.Package

	// Types is the package that go/types made, or nil if the package was
	// not type-checked (Checked is false). It is there also when go/types
	// reported errors; then parts of it can be invalid.
	Types *types.Package

	// Info holds what go/types recorded about the ASTs: every map of
	// types.Info is filled (Types, Defs, Uses, Implicits, Selections,
	// Scopes, Instances, FileVersions), and so is InitOrder. Info also
	// has the types of the type arguments of opaque instantiations
	// (std.vector[strings.Builder]), which go/types itself doesn't look
	// at. It is nil if the package was not type-checked.
	Info *types.Info

	// Errors are the diagnostics of the package, as go build prints them.
	// For a package that was not type-checked, it is Load.Errors. For a
	// checked package, it is a new list: the warnings of Load.Errors (such
	// a package has no load errors), then the errors of the namespace
	// probe about the header imports and the namespace renames (the
	// "undefined: stdd" of a name that is no namespace comes from
	// go/types, see Check), then the errors of go/types and of
	// the type arguments of opaque instantiations, in the order they were
	// found, then the errors of the expressions that go/types skipped next
	// to opaque ones (checkSkipped), then the internal errors of the opaque
	// pass. Check never changes Load.Errors.
	Errors *diag.List

	// Conversions are the conversions of opaque values in the files, T(x)
	// with x from C/C++, which Check rewrote to gp_cgoh.As[T](x) (gp_cgoh
	// is the import of the reserved path gocpp/cgoh) in the ASTs of Load,
	// so that they have the type T (see Conversion). They are in source
	// order, an outer conversion before the ones inside it. Only a checked
	// package with C in it (see Check) can have them.
	Conversions []*Conversion

	// conversions maps the Call of each conversion to it.
	conversions map[*ast.CallExpr]*Conversion

	// checks is how many times go/types checked the package: 1, or more
	// after rewrites of conversions.
	checks int

	// Opaque says which expressions are opaque (they come from C/C++ and
	// have no complete type) and keeps the type expressions of the objects
	// declared with C/C++ types (see Opaque). It is nil if the package
	// was not type-checked.
	Opaque *Opaque
}

// Checked reports whether the package was type-checked. A package is not
// type-checked when it or a package that it imports, directly or not, has
// a load error (load.Package.Incomplete), like go build, which builds no
// package whose dependencies failed and reports nothing more for it.
func (p *Package) Checked() bool {
	return p.Types != nil
}

// Check type-checks the packages of prog with the stock go/types, each one
// after the packages it imports (Phase 01 README §2.7). The packages are
// checked with FakeImportC, so C.X, which the loader made out of header
// and namespace uses (raylib.X, std.X), is "opaque": go/types accepts it
// without a type.
//
// After go/types, Check type-checks the type arguments of each opaque
// instantiation (std.vector[strings.Builder]), which go/types skips
// because the generic type is opaque: see checkOpaqueArgs.
//
// In a package that has C in it (a file with import "C", or an import of
// such a package, directly or not), Check then rewrites each conversion of
// an opaque value, T(x), to gp_cgoh.As[T](x), where gp_cgoh is the import
// of the hidden package with the reserved path gocpp/cgoh (load.CgohPath),
// and checks the package again from the start, so that the conversion has
// the type T (Phase 01 README §2.5 step 5): see Conversion. A package
// without such conversions is checked once.
//
// Before any of this, Check runs the namespace probe for every file with
// header imports or namespace renames (Phase 01 README §2.5 step 4, see
// probeNamespaces): a name that the loader took as a namespace (stdd in
// stdd.vector) but that the file's headers don't declare as one is
// "undefined: stdd" at each use, from go/types itself, and a header that
// isn't found or doesn't compile is an error at its import. The probe
// runs the C++ compiler of Config.CXX.
//
// Check changes the program: it undoes the rewrites of the names that are
// not namespaces, in the ASTs and in load.Rewrite, and it rewrites
// conversions of opaque values in the ASTs. So a Program is checked once
// (a second Check gives the same result, but sees the changed ASTs), and
// after a header or a file changes, the program must be loaded again.
//
// Then Check runs the phase gates over each user package (gates.go): every
// feature of a phase after Config.Phase is an error that names the phase
// ("composite types are not supported yet (Phase 03)"). They run on the
// Info of the last check.
//
// A package with a load error (load.Package.Incomplete) is not checked, and
// neither is a package that imports one, directly or not. Note that go
// build does a little less for some load errors: when an import can't be
// resolved, it prints the load errors and checks no package at all (go
// list fails before any compile). For a syntax error it compiles the other
// packages, like Check. The caller (gocpp check) decides whether to look
// at the other packages.
func Check(prog *load.Program, cfg Config) *Result {
	gates := newGates(prog.Fset, cfg.phase())
	r := &Result{Fset: prog.Fset, byLoad: make(map[*load.Package]*Package, len(prog.Packages))}
	checked := make(map[*load.Package]*types.Package)
	// failed holds the packages with hard errors, or that import one,
	// directly or not, and hasC those with import "C" in a file, or that
	// import one, directly or not: only they can have opaque values.
	failed := make(map[*load.Package]bool)
	hasC := make(map[*load.Package]bool)
	cgoh := newCgohPackage()
	probeErrs := probeNamespaces(prog, cfg)
	if cfg.Context != nil && cfg.Context.Err() != nil {
		return r
	}
	for _, lp := range prog.Packages {
		var p *Package
		if lp.Incomplete {
			p = &Package{Load: lp, Errors: lp.Errors}
			failed[lp] = true
		} else {
			depsFailed, c := false, usesC(lp)
			for _, dep := range lp.Imports {
				depsFailed = depsFailed || failed[dep]
				c = c || hasC[dep]
			}
			hasC[lp] = c
			p = checkPackage(prog.Fset, lp, checked, cgoh, c && !depsFailed, probeErrs[lp])
			p.Opaque = findOpaque(prog.Fset, p)
			checked[lp] = p.Types
			failed[lp] = depsFailed || hasHardErrors(p.Errors)
		}
		gates.apply(p)
		r.Packages = append(r.Packages, p)
		r.byLoad[lp] = p
	}
	return r
}

// hasHardErrors reports whether l has an error that is not a soft error
// of go/types.
func hasHardErrors(l *diag.List) bool {
	for _, d := range l.Diagnostics() {
		if d.Severity == diag.Error && !d.Soft {
			return true
		}
	}
	return false
}

// checkPackage type-checks the loaded package lp. checked holds the
// packages checked so far, which lp's imports come from, and cgoh is the
// hidden package of the conversions (newCgohPackage). With convert, it
// then rewrites the conversions of opaque values and checks the package
// again, until no rewrite is left (see converter), and it drops the false
// errors of go/types on opaque code (see falseErrors). Check passes
// convert only for a package with C in it, and none whose imports have
// errors (an invalid value from there can come from an error, not only
// from C).
// probeErrs are the errors of the namespace probe, which each check puts
// first (after the load warnings).
func checkPackage(fset *token.FileSet, lp *load.Package, checked map[*load.Package]*types.Package,
	cgoh *types.Package, convert bool, probeErrs []diag.Diagnostic) *Package {
	cv := newConverter(fset, lp, cgoh)
	p, errs := checkFiles(fset, lp, checked, cv, probeErrs, convert)
	p.checks = 1
	for convert && cv.rewrite(p, errs) {
		checks := p.checks + 1
		p, errs = checkFiles(fset, lp, checked, cv, probeErrs, convert)
		p.checks = checks
	}
	if len(cv.names) > 0 {
		p.setConversions(findConversions(lp, p.Info))
	}
	return p
}

// checkFiles type-checks the files of lp once, with fresh Info and
// diagnostics. It returns the places of the errors that can make a value
// invalid (collector.blocking), in the order they were found. cv
// gives the hidden package gocpp/cgoh to the importer and maps
// gp_cgoh.As[T](x) back to T(x) in the messages. The errors of the
// namespace probe, probeErrs, come before those of go/types: they were
// found first. With dropFalse, the false errors of go/types on opaque code
// are dropped (see falseErrors).
func checkFiles(fset *token.FileSet, lp *load.Package, checked map[*load.Package]*types.Package,
	cv *converter, probeErrs []diag.Diagnostic, dropFalse bool) (*Package, []token.Pos) {
	errs := new(diag.List)
	for _, d := range lp.Errors.Diagnostics() {
		// For a package that Check checks, these are only warnings (else
		// the package would be incomplete). Warnings don't count for the
		// limit of 10 errors, so their sorted order is as good as the
		// order in which they were found.
		errs.Add(d)
	}
	for _, d := range probeErrs {
		errs.Add(d)
	}
	// Every package buffers: an expression that go/types skips next to an
	// opaque one can be in a file without import "C" (make(lib.Vec, n),
	// with type Vec = C.vec in another file or package).
	c := &collector{fset: fset, list: errs, seen: make(map[errorKey]bool), fix: cv.fixMessage,
		buffer: true}
	imp := &importer{from: lp, checked: checked, refused: make(map[string]bool), cgoh: cv.cgoh}
	if lp.Incomplete {
		// Check doesn't check such a package, but a caller that wants
		// the types of a broken package can (the language server,
		// Phase 11). Then each import that the loader couldn't give it
		// has a load error already, and go/types' "could not import"
		// would say it again.
		c.refused = imp.refused
	}
	info := &types.Info{
		Types:        make(map[ast.Expr]types.TypeAndValue),
		Instances:    make(map[*ast.Ident]types.Instance),
		Defs:         make(map[*ast.Ident]types.Object),
		Uses:         make(map[*ast.Ident]types.Object),
		Implicits:    make(map[ast.Node]types.Object),
		Selections:   make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:       make(map[ast.Node]*types.Scope),
		FileVersions: make(map[*ast.File]string),
	}
	conf := types.Config{
		GoVersion:   GoVersion,
		FakeImportC: true,
		Importer:    imp,
		Sizes:       gocppSizes{},
		Error:       c.collect,
	}
	files := make([]*ast.File, len(lp.Files))
	for i, f := range lp.Files {
		files[i] = f.AST
	}
	// The errors go to c; the returned error is the first of them.
	pkg, _ := conf.Check(lp.ImportPath, fset, files, info)
	// checkOpaqueArgs and checkSkipped write into info and into the scope
	// tree of pkg (types.CheckExpr), so they run once in each round of
	// checks, after conf.Check, on the round's own fresh info and pkg.
	used := checkOpaqueArgs(fset, pkg, files, info, c.checkExprError)
	checkSkipped(fset, pkg, files, info, c.checkExprError, used)
	falseErrs := newFalseErrors(pkg, info, files)
	c.flush(func(err types.Error) bool {
		if diag.ErrorCode(err) == diag.UnusedVar {
			return used.vars[err.Pos]
		}
		pn := pkgNameAt(files, info, err.Pos)
		return pn != nil && used.imports[pn]
	}, func(err types.Error) bool {
		return dropFalse && falseErrs.drop(err)
	})
	return &Package{Load: lp, Types: pkg, Info: info, Errors: errs}, c.blocking
}

// An importer gives go/types the packages that one package imports. It is
// keyed by the importing *load.Package, not by import path, because with
// tests (load.Config.Tests) several packages can have the same import
// path: the test variant of p and p itself. The loader resolved each
// package's imports (load.Package.Imports), so an import gets exactly the
// package that the loader chose for it.
type importer struct {
	from    *load.Package
	checked map[*load.Package]*types.Package

	// refused holds the paths that the importer had no package for.
	refused map[string]bool

	// cgoh is the hidden package of the conversions of opaque values
	// (load.CgohPath), which no gocpp package can import (the loader
	// rejects the path).
	cgoh *types.Package
}

// errNotChecked is the error of an import whose package was not
// type-checked or not loaded.
var errNotChecked = errors.New("package not loaded or not type-checked")

// Import implements types.Importer.
func (im *importer) Import(path string) (*types.Package, error) {
	return im.ImportFrom(path, "", 0)
}

// ImportFrom implements types.ImporterFrom. "unsafe" is go/types' own
// package, "gocpp/cgoh" is Check's own (only the rewrite of conversions
// imports it), and "C" never comes here: FakeImportC handles it.
func (im *importer) ImportFrom(path, _ string, _ types.ImportMode) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
	}
	if path == load.CgohPath && im.cgoh != nil {
		return im.cgoh, nil
	}
	if dep := im.from.Imports[path]; dep != nil {
		if pkg := im.checked[dep]; pkg != nil {
			return pkg, nil
		}
	}
	im.refused[path] = true
	return nil, errNotChecked
}

// brokenImport is the go/types error code of `could not import X`
// (BrokenImport in Go's internal/types/errors; Go doesn't change the
// values).
const brokenImport = 6

// An errorKey is the place and the message of an error.
type errorKey struct {
	pos token.Pos
	msg string
}

// A collector is the Error function of types.Config: a filter in front of
// diag.List.Collect. It drops a "could not import X" when the loader
// reported X already.
//
// An import or a local variable can be used only in the type arguments of
// opaque instantiations (std.vector[strings.Builder],
// std.array[int, len(arr)]) or in expressions that go/types skips next to
// opaque ones (cv.(strings.Builder), make(lib.Vec, n): checkSkipped),
// which are checked after go/types. So the collector keeps every go/types
// error, in the order they were found, until flush: the "imported and not
// used" (diag.UnusedImport) and "declared and not used" (diag.UnusedVar)
// errors are held, and flush drops those whose import or variable turned
// out to be used. All of them have to wait, not only the held ones, to
// keep the order in which go/types found them (the limit of 10 errors
// keeps the first 10 found): go/types reports the unused variables at the
// end of each function body, and the unused imports after all bodies. The
// errors of the type arguments (CheckExpr) come after them, and then those
// of the skipped expressions.
type collector struct {
	fset *token.FileSet
	list *diag.List

	// buffer says to keep the go/types errors in found until flush.
	buffer bool

	// found are the go/types errors kept until flush (with their
	// continuation lines, which start with a tab), in the order they were
	// found.
	found []foundError

	// argErrors are the errors of the type arguments and then of the
	// skipped expressions, which flush adds after found.
	argErrors []types.Error

	// seen holds the errors added so far (to found or to list), so that
	// an error that CheckExpr finds again (in an argument that go/types
	// checked too) is not reported twice.
	seen map[errorKey]bool

	// refused holds the import paths whose "could not import" errors are
	// dropped, or is nil.
	refused map[string]bool

	// last says what happened to the last error that isn't a continuation
	// line: its continuation lines go the same way.
	last action

	// blocking holds the places of the errors of go/types and of the type
	// arguments that can make a value invalid, in the order they were
	// found, also those that the list doesn't get: the hard errors and the
	// soft ones (a range clause with two variables over an int leaves them
	// invalid), but not an unused import, variable or expression, which
	// make no type wrong, and not an error about an operand that is
	// invalid already (followOnError).
	blocking []token.Pos

	// fix, if not nil, changes the message of each error before the list
	// gets it: it maps gp_cgoh.As[T](x) back to T(x).
	fix func(msg string) string
}

// A foundError is an error of go/types that the collector keeps.
type foundError struct {
	err  types.Error
	held bool // an unused import or variable, which flush may drop

	// blocking says that the error's place is in collector.blocking.
	blocking bool
}

type action int

const (
	passed action = iota
	held
	dropped
)

// collect takes an error from go/types.
func (c *collector) collect(err error) {
	terr, ok := err.(types.Error)
	if !ok {
		c.list.Collect(err)
		return
	}
	blocking := false
	if !strings.HasPrefix(terr.Msg, "\t") {
		switch diag.ErrorCode(terr) {
		case diag.UnusedImport, diag.UnusedVar, unusedExpr:
		default:
			if !followOnError(terr.Msg) {
				c.blocking = append(c.blocking, terr.Pos)
				blocking = true
			}
		}
		c.last = c.decide(terr)
	}
	if c.fix != nil {
		terr.Msg = c.fix(terr.Msg)
	}
	if c.last == dropped {
		return
	}
	c.seen[errorKey{terr.Pos, terr.Msg}] = true
	if !c.buffer {
		c.list.Collect(terr)
		return
	}
	c.found = append(c.found, foundError{err: terr, held: c.last == held, blocking: blocking})
}

// decide says what happens to an error that isn't a continuation line.
func (c *collector) decide(err types.Error) action {
	code := diag.ErrorCode(err)
	switch {
	case c.buffer && (code == diag.UnusedImport || code == diag.UnusedVar):
		return held
	case code == brokenImport && c.refused != nil && c.refusedImport(err.Msg):
		return dropped
	}
	return passed
}

// refusedImport reports whether msg is a "could not import X (...)" error
// of a path X that the importer refused.
func (c *collector) refusedImport(msg string) bool {
	rest, ok := strings.CutPrefix(msg, "could not import ")
	if !ok {
		return false
	}
	path, _, ok := strings.Cut(rest, " (")
	return ok && c.refused[path]
}

// checkExprError takes the error of a types.CheckExpr call at pos, for
// flush. An error that is already there (same place, same message) is not
// added again.
func (c *collector) checkExprError(pos token.Pos, err error) {
	var terr types.Error
	if !errors.As(err, &terr) {
		terr = types.Error{
			Fset: c.fset,
			Pos:  pos,
			Msg:  fmt.Sprintf("internal error: checking a type argument: %v", err),
		}
	}
	c.blocking = append(c.blocking, terr.Pos)
	if c.fix != nil {
		terr.Msg = c.fix(terr.Msg)
	}
	if key := (errorKey{terr.Pos, terr.Msg}); !c.seen[key] {
		c.seen[key] = true
		c.argErrors = append(c.argErrors, terr)
	}
}

// flush adds the kept errors to the list: the go/types errors in the order
// they were found, without the held ones for which used reports that their
// import or variable is used after all and without the false errors on
// opaque code, for which falseError reports true (see falseErrors), and
// then the errors of the type arguments and of the skipped expressions. A
// dropped false error leaves blocking too: it is no mistake that could
// have made a value invalid.
func (c *collector) flush(used, falseError func(err types.Error) bool) {
	keep := true
	for _, f := range c.found {
		if !strings.HasPrefix(f.err.Msg, "\t") {
			keep = !f.held || !used(f.err)
			if keep && !f.held && falseError(f.err) {
				keep = false
				if f.blocking {
					c.unblock(f.err.Pos)
				}
			}
		}
		if keep {
			c.list.Collect(f.err)
		}
	}
	for _, err := range c.argErrors {
		c.list.Collect(err)
	}
	c.found, c.argErrors = nil, nil
}

// unblock removes one place pos from blocking.
func (c *collector) unblock(pos token.Pos) {
	if i := slices.Index(c.blocking, pos); i >= 0 {
		c.blocking = slices.Delete(c.blocking, i, i+1)
	}
}

// pkgNameAt returns the package name that the import at pos in files
// declares (the place of go/types' "imported and not used" error), or nil.
func pkgNameAt(files []*ast.File, info *types.Info, pos token.Pos) *types.PkgName {
	for _, f := range files {
		for _, spec := range f.Imports {
			if spec.Pos() != pos {
				continue
			}
			var obj types.Object
			if spec.Name != nil {
				obj = info.Defs[spec.Name]
			} else {
				obj = info.Implicits[spec]
			}
			pn, _ := obj.(*types.PkgName)
			return pn
		}
	}
	return nil
}
