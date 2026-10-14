package check

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
)

// GoVersion is the Go language version of every package: the loop
// variables of Go 1.22, range over int and over functions, and so on
// (types.Config.GoVersion). A file can still ask for an older version with
// a //go:build go1.N line, as in Go (types.Info.FileVersions).
const GoVersion = "go1.25"

// A Config says how Check checks. It has no settings yet; the namespace
// probe (job 1.10c) adds the C++ compiler that it runs.
type Config struct{}

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
	// a package has no load errors), then the errors of go/types and of
	// the type arguments of opaque instantiations, in the order they were
	// found. Check never changes Load.Errors.
	Errors *diag.List
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
// A package with a load error (load.Package.Incomplete) is not checked, and
// neither is a package that imports one, directly or not. Note that go
// build does a little less for some load errors: when an import can't be
// resolved, it prints the load errors and checks no package at all (go
// list fails before any compile). For a syntax error it compiles the other
// packages, like Check. The caller (gocpp check) decides whether to look
// at the other packages.
func Check(prog *load.Program, cfg Config) *Result {
	_ = cfg
	r := &Result{Fset: prog.Fset, byLoad: make(map[*load.Package]*Package, len(prog.Packages))}
	checked := make(map[*load.Package]*types.Package)
	for _, lp := range prog.Packages {
		var p *Package
		if lp.Incomplete {
			p = &Package{Load: lp, Errors: lp.Errors}
		} else {
			p = checkPackage(prog.Fset, lp, checked)
			checked[lp] = p.Types
		}
		r.Packages = append(r.Packages, p)
		r.byLoad[lp] = p
	}
	return r
}

// checkPackage type-checks the loaded package lp. checked holds the
// packages checked so far, which lp's imports come from.
func checkPackage(fset *token.FileSet, lp *load.Package, checked map[*load.Package]*types.Package) *Package {
	errs := new(diag.List)
	for _, d := range lp.Errors.Diagnostics() {
		// For a package that Check checks, these are only warnings (else
		// the package would be incomplete). Warnings don't count for the
		// limit of 10 errors, so their sorted order is as good as the
		// order in which they were found.
		errs.Add(d)
	}
	c := &collector{fset: fset, list: errs, seen: make(map[errorKey]bool)}
	for _, f := range lp.Files {
		// Only a file with import "C" can have opaque instantiations.
		c.buffer = c.buffer || importsC(f.AST)
	}
	imp := &importer{from: lp, checked: checked, refused: make(map[string]bool)}
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
	if !c.buffer {
		return &Package{Load: lp, Types: pkg, Info: info, Errors: errs}
	}
	used := checkOpaqueArgs(fset, pkg, files, info, c.checkExprError)
	c.flush(func(err types.Error) bool {
		if diag.ErrorCode(err) == diag.UnusedVar {
			return used.vars[err.Pos]
		}
		pn := pkgNameAt(files, info, err.Pos)
		return pn != nil && used.imports[pn]
	})
	return &Package{Load: lp, Types: pkg, Info: info, Errors: errs}
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
}

// errNotChecked is the error of an import whose package was not
// type-checked or not loaded.
var errNotChecked = errors.New("package not loaded or not type-checked")

// Import implements types.Importer.
func (im *importer) Import(path string) (*types.Package, error) {
	return im.ImportFrom(path, "", 0)
}

// ImportFrom implements types.ImporterFrom. "unsafe" is go/types' own
// package, and "C" never comes here: FakeImportC handles it.
func (im *importer) ImportFrom(path, _ string, _ types.ImportMode) (*types.Package, error) {
	if path == "unsafe" {
		return types.Unsafe, nil
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
// In a package with import "C" (buffer), an import or a local variable can
// be used only in the type arguments of opaque instantiations
// (std.vector[strings.Builder], std.array[int, len(arr)]), which are
// checked after go/types. So there the collector keeps every go/types
// error, in the order they were found, until flush: the "imported and not
// used" (diag.UnusedImport) and "declared and not used" (diag.UnusedVar)
// errors are held, and flush drops those whose import or variable turned
// out to be used. All of them have to wait, not only the held ones, to
// keep the order in which go/types found them (the limit of 10 errors
// keeps the first 10 found): go/types reports the unused variables at the
// end of each function body, and the unused imports after all bodies. The
// errors of the type arguments (CheckExpr) come after them.
type collector struct {
	fset *token.FileSet
	list *diag.List

	// buffer says to keep the go/types errors in found until flush.
	buffer bool

	// found are the go/types errors kept until flush (with their
	// continuation lines, which start with a tab), in the order they were
	// found.
	found []foundError

	// argErrors are the errors of the type arguments, which flush adds
	// after found.
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
}

// A foundError is an error of go/types that the collector keeps.
type foundError struct {
	err  types.Error
	held bool // an unused import or variable, which flush may drop
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
	if !strings.HasPrefix(terr.Msg, "\t") {
		c.last = c.decide(terr)
	}
	if c.last == dropped {
		return
	}
	c.seen[errorKey{terr.Pos, terr.Msg}] = true
	if !c.buffer {
		c.list.Collect(terr)
		return
	}
	c.found = append(c.found, foundError{err: terr, held: c.last == held})
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
	if key := (errorKey{terr.Pos, terr.Msg}); !c.seen[key] {
		c.seen[key] = true
		c.argErrors = append(c.argErrors, terr)
	}
}

// flush adds the kept errors to the list: the go/types errors in the order
// they were found, without the held ones for which used reports that their
// import or variable is used after all, and then the errors of the type
// arguments.
func (c *collector) flush(used func(err types.Error) bool) {
	keep := true
	for _, f := range c.found {
		if !strings.HasPrefix(f.err.Msg, "\t") {
			keep = !f.held || !used(f.err)
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
