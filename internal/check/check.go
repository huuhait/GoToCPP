package check

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
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

	// Env, if not nil, are the trusted flags that the namespace probe uses instead of the
	// environment's alone (build.ReadEnvFlags): the environment's with the flags of the build
	// (gocpp build -cxxflags, -race), so that the probe parses headers like the compile does.
	Env *build.EnvFlags

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

	// Deps, if not nil, gives what Check needs of each package that a
	// package of the program imports but that is not in
	// load.Program.Packages: a package that load.Config.Deps gave the
	// loader, which Check does not check again. Check calls it once for
	// each such package. The language server (Phase 11) gives the types
	// of such a package from its export data, and the rest from the
	// Result.Dep of the check that checked it.
	Deps func(lp *load.Package) Dep

	// Probe, if not nil, replaces interop.ProbeNamespaces: Check calls it once
	// per include set of a package, with the compiler that CXX gave, the
	// package's flags (Package.ProbeFlags) and the namespace candidates.
	// The language server (Phase 11, step 11.4) answers from what it knows
	// at once (the names it doesn't know yet are Unknown, so their uses stay
	// opaque) and runs the real probe in the background; the tests of this
	// package use it for headers that mostly don't exist.
	Probe func(ctx context.Context, cxx build.CXX, dir string, flags, includes, candidates []string) (interop.Result, error)

	// CHeader, if not nil, replaces interop.CheckCHeaderFor: Check calls it
	// once per local .h header of a package that no //gocpp:extern-c or
	// no-extern-c names, with the C++ compiler that CXX gave, the
	// package's folder, its C flags (GOCPP_CFLAGS, then Package.Flags.Compile)
	// and the #include text ("./legacy.h"), to decide whether the build
	// includes the header inside extern "C" { } (externc.go). The language
	// server answers from what it knows at once (a header it has no answer
	// for yet is not wrapped) and runs the real check in the background,
	// like the probe.
	CHeader func(ctx context.Context, cxx build.CXX, dir string, flags []string, include string) (interop.CHeader, error)

	// PkgConfig, if not nil, replaces build.PkgConfig, which runs
	// pkg-config for each //gocpp:pkg-config line of a package that Check
	// checks (see probeNamespaces). The language server answers from what
	// it knows and runs pkg-config in the background; while it has no
	// answer, it returns build.ErrPending, and the package is not probed.
	PkgConfig build.PkgConfigFunc

	// Partial, if set, type-checks the packages with load errors too, for
	// Package.Partial only: the language server (Phase 11, step 11.5)
	// answers hover and go to definition in a file that doesn't parse from
	// what go/types finds anyway. It changes nothing else: such a package
	// still has only its load errors, is not Checked, and its importers
	// get no types from it (Result.Dep).
	Partial bool

	// LazyPartial, with Partial, makes the partial check of a package with load errors on the first call of
	// Package.PartialResult (or MustPartialInfo) instead of in Check: the language server needs it for
	// the requests that look at a file with a syntax error, not for its diagnostics, which come from the
	// load errors alone, and a keystroke must not pay for a type check that nobody asked for.
	LazyPartial bool

	// NoDerivedBodies says that the caller does not lower the program (the language server,
	// gocpp check, gocpp vet): the generators of #Derive (derive.go) run for their errors, but the
	// methods are not formatted, not put in the files and not kept (Package.Derived is empty), and the
	// package is not checked a second time (which costs as much as the first check). The stubs stay,
	// with the signatures that every caller needs.
	NoDerivedBodies bool

	// AllowBodyless accepts a function without a body and without //gocpp:extern in a package of
	// the program. The default is the error "missing function body" (externfuncs.go), like Go's
	// compiler for a package without assembly files. The runner of Go's own test files
	// (tools/gotest) sets it: Go compiles those tests, which declare functions for assembly, and
	// never links them.
	AllowBodyless bool

	// Constants, if not nil, gives the compile-time values of the C/C++ names that a package uses where
	// Go needs a constant (an array length, a const declaration, a case, a shift count), so that
	// [cstdio.BUFSIZ]byte and case cstdio.EOF: type-check with the stock go/types: Check puts the
	// values into the syntax tree before it checks the package (constsubst.go; Phase 12, step 12.8,
	// ADR-097). nil leaves such names opaque, as before. Only typed mode sets it.
	Constants ConstFunc

	// CppMembers, if not nil, turns on the second pass for the promoted members of embedded C++
	// classes (cppmembers.go, Phase 12 step 12.9): typed interop answers which embedded class has
	// which member name. Check calls it at most once per package, and only when the first pass
	// has a selector it cannot decide. nil is opaque mode: the first pass stands.
	CppMembers CppMembers

	// HResults, if not nil, makes the calls that return an HRESULT Go errors in the program
	// (hresults.go, ADR-137): typed interop says which calls they are, and Check wraps each in
	// error(...) before it is done with the package, so that go/types gives it the type error. nil
	// is opaque mode: the call is a C/C++ value, as it is.
	HResults HResultCalls

	// SkipOpaque leaves out the opaque pass (findOpaque) and the check of the
	// expressions that go/types skips (checkSkipped) for a package that has no C
	// or C++ in it or in the packages it imports, directly or not (Dep.HasC is
	// false for all of them, and no file has import "C"), and that go/types
	// found no error in, or only errors about an unused import or variable
	// (they leave every type valid, typesMayBeIncomplete): then every type is
	// complete and nothing comes from C, so nothing can be opaque, both passes
	// find nothing, and Package.Opaque is empty, as the pass would make it.
	// (With another error, a type can be invalid, and the pass calls what has
	// one opaque, also without C.) The only thing
	// lost is the pass's internal error for an expression without a type that a
	// gocpp bug leaves behind. The language server sets it, which does not
	// generate code.
	SkipOpaque bool

	// Focus, if not empty, is the name of one file (as load.File.Path has it): the function bodies
	// of the other files of the package are not checked (stripOtherBodies; init and the generic
	// functions keep theirs), and the gates look at that file alone. The package is Focused then. See focus.go.
	Focus string
}

// A Dep is what Check needs of a package that it imports but does not
// check (Config.Deps), and what Result.Dep says about a package it checked.
type Dep struct {
	// Types is the type-checked package, or nil if it was not
	// type-checked (Package.Checked): then importing it fails, like
	// importing an Incomplete package.
	Types *types.Package

	// HasC reports whether a file of the package has import "C", or a
	// package that it imports, directly or not, has one: only then can a
	// value of an importer be opaque (see Check).
	HasC bool

	// Failed reports whether the package has a hard error (not a soft
	// error of go/types), or imports such a package, directly or not:
	// then its importers' conversions of invalid values are not rewritten
	// (see checkPackage).
	Failed bool
}

// A Result is what Check found out about a loaded program.
type Result struct {
	// Fset is the FileSet of the program (load.Program.Fset).
	Fset *token.FileSet

	// Packages are the packages of the program, in the order of
	// load.Program.Packages: each one after the packages it imports.
	Packages []*Package

	byLoad map[*load.Package]*Package

	// hasC and failed are Dep.HasC and Dep.Failed of each package.
	hasC, failed map[*load.Package]bool
}

// Package returns the checked package of the loaded package p, or nil if
// p is not a package of the program.
func (r *Result) Package(p *load.Package) *Package {
	return r.byLoad[p]
}

// Dep returns what a later Check needs of the package p of the program
// when p is not checked again there (Config.Deps): its types (nil if p was
// not type-checked) and how it counts for its importers. ok is false when p
// is not a package of the program.
func (r *Result) Dep(p *load.Package) (dep Dep, ok bool) {
	cp := r.byLoad[p]
	if cp == nil {
		return Dep{}, false
	}
	return Dep{Types: cp.Types, HasC: r.hasC[p], Failed: r.failed[p]}, true
}

// A Package is a loaded package after type checking.
type Package struct {
	// typesClean says that the opaque pass found nothing to do and was left out
	// (Config.SkipOpaque): the last check recorded no error but unused imports
	// and variables, and nothing in the package or its imports is C.
	typesClean bool

	// typeErrors says that the package has an error of go/types that is not about something unused
	// (a false error on opaque code, which Check drops, does not count): a type can be invalid for a
	// reason that is not C/C++, so the limits of C/C++ types are not reported next to it (genericc.go).
	typeErrors bool

	// notConst holds the values of const declarations that are errors because they hold a variable or a
	// call of a Go function (constvalues.go): the gate of the C names in constant expressions leaves them.
	notConst map[ast.Expr]bool

	// hresultCalls are the calls that Config.HResults made errors, hresultVars the variables that hold
	// one (hresults.go).
	hresultCalls map[*ast.CallExpr]bool
	hresultVars  map[types.Object]bool

	// Focused says that the function bodies of the files other than Config.Focus were left out
	// (but those of init and of the generic functions).
	Focused bool

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
	// a package has no load errors), then the errors of its flags (at the
	// //gocpp: directives), then the errors of the namespace
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

	// hidden holds the names of the imports of gocpp/cgoh that Check added
	// to the files (gp_cgoh), for UserText.
	hidden map[string]bool

	// holders are the C.cstr holders of the package (Holders), found on first use.
	holders     *Holders
	holdersOnce sync.Once

	// sourceUses are the loader's rewritten selectors (SourceText), found on first use.
	sourceUses     map[*ast.SelectorExpr]*load.CppUse
	sourceUsesOnce sync.Once

	// slices maps the call gp_cgoh.SliceC(a, ...) that Check made of a slice
	// expression of an opaque value (slicecalls.go) to that expression.
	slices map[*ast.CallExpr]*ast.SliceExpr

	// checks is how many times go/types checked the package: 1, or more
	// after rewrites of conversions.
	checks int

	// cppSels is what the second pass for promoted C++ members decided
	// (cppmembers.go); nil without Config.CppMembers or when the first pass
	// had nothing to ask. cppAsked is the number of questions it asked.
	cppSels  map[*ast.SelectorExpr]*cppSel
	cppAsked int

	// Derived are the methods that #Derive generated (derive.go), with their source text, in
	// the order of the #Derive lines: gocpp expand prints them.
	Derived []DerivedMethod

	// Must says where the `!` forms of the source were (they are plain Go in the trees of Load
	// after the rewrite, ADR-037); nil if the package has none. gocpp vet reads it.
	Must *MustInfo

	// MustPartial is what the pass knows of a package that has errors, where the lowering did not run
	// and Must is nil: the places of the `!` forms, for the editor (MustInfo.partialInfo). Nil
	// for a package without errors and for one with no `!` form.
	MustPartial *MustInfo

	// Opaque says which expressions are opaque (they come from C/C++ and
	// have no complete type) and keeps the type expressions of the objects
	// declared with C/C++ types (see Opaque). It is nil if the package
	// was not type-checked.
	Opaque *Opaque

	// Consts are the C/C++ names that Check replaced with their values before go/types ran (Config.Constants):
	// what the user wrote, where, and the constant. Empty without Config.Constants.
	Consts []ConstUse

	// Flags are the C/C++ flags of the package's directives and of
	// pkg-config, after the flag allow-list (build.ReadPackageFlags): the
	// build compiles the package's C++ with Flags.Compile and links the
	// program with Flags.Link. They are empty for a package that was not
	// type-checked, and they lack the flags of a directive with an error.
	Flags build.PackageFlags

	// ProbeFlags are the flags that the namespace probe got for the
	// package (build.ProbeFlags: the trusted flags of the environment, then
	// Flags.Compile). The lowerer passes the same flags to
	// interop.ProbeKinds, so that its probe shares the cache of the check.
	ProbeFlags []string

	// ExternC holds the #include texts of the package's header imports
	// (<lua.h>, "./legacy.h") that the build includes inside extern "C"
	// { } (Phase 04 README §2.9, step 4.14; see externc.go): the C headers
	// without a __cplusplus guard, and the headers of //gocpp:extern-c.
	// The namespace probe got these includes marked with interop.ExternC,
	// and the lowerer passes them so to interop.ProbeKinds and
	// interop.ProbeMacros too, so that the probes see the headers as the
	// build does.
	ExternC map[string]bool

	// Partial is what go/types found in a package that was not
	// type-checked because it has load errors (a syntax error, an import
	// that can't be resolved, or one of a package with errors), when
	// Config.Partial asked for it; else it is nil. Its Info can miss
	// parts, and types can be invalid where something is broken. Nothing
	// else uses it: not the errors, not the phase gates, not the importers.
	Partial *Partial

	// partialFn makes Partial and MustPartial on the first call of PartialResult, for Config.LazyPartial.
	partialFn   func() (*Partial, *MustInfo)
	partialOnce sync.Once
	hasPartial  bool // set when the package is made, never changed
}

// HasPartial reports whether the package has a partial check (made already, or made on demand).
func (p *Package) HasPartial() bool { return p.hasPartial }

// PartialResult returns Package.Partial, making it first when Config.LazyPartial left it for now. It makes the
// check once; callers wait for it. The check changes the syntax trees of the package (the `!` forms are prepared
// as for any check), so a caller that reads the trees must call it before.
func (p *Package) PartialResult() *Partial {
	if p.partialFn != nil {
		p.partialOnce.Do(func() { p.Partial, p.MustPartial = p.partialFn() })
	}
	return p.Partial
}

// MustPartialInfo returns Package.MustPartial after PartialResult.
func (p *Package) MustPartialInfo() *MustInfo {
	p.PartialResult()
	return p.MustPartial
}

// A Partial is the result of go/types on the files of a package that has
// load errors (Package.Partial). The imports that the loader couldn't
// give, or whose packages were not type-checked, are missing, so their
// names have no type.
type Partial struct {
	Types  *types.Package
	Info   *types.Info // every map filled, as in Package.Info
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
// Before any of this, Check reads the C/C++ flags of each package
// (Package.Flags: its //gocpp: directives and pkg-config, through the flag
// allow-list) and runs the namespace probe, with them, for every file with
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
	sums := newGenericSums() // genericcinst.go
	r := &Result{Fset: prog.Fset, byLoad: make(map[*load.Package]*Package, len(prog.Packages))}
	checked := make(map[*load.Package]*types.Package)
	// failed holds the packages with hard errors, or that import one,
	// directly or not, and hasC those with import "C" in a file, or that
	// import one, directly or not: only they can have opaque values.
	failed := make(map[*load.Package]bool)
	hasC := make(map[*load.Package]bool)
	r.hasC, r.failed = hasC, failed
	cgoh := newCgohPackage()
	probeErrs, flagsOf := probeNamespaces(prog, cfg)
	if cfg.Context != nil && cfg.Context.Err() != nil {
		return r
	}
	if cfg.Deps != nil {
		// The imported packages that are not checked here.
		inProg := make(map[*load.Package]bool, len(prog.Packages))
		for _, lp := range prog.Packages {
			inProg[lp] = true
		}
		asked := make(map[*load.Package]bool)
		for _, lp := range prog.Packages {
			for _, dep := range lp.Imports {
				if !inProg[dep] && !asked[dep] {
					asked[dep] = true
					d := cfg.Deps(dep)
					if d.Types != nil {
						checked[dep] = d.Types
					}
					hasC[dep], failed[dep] = d.HasC, d.Failed || d.Types == nil
				}
			}
		}
	}
	for _, lp := range prog.Packages {
		var p *Package
		if lp.Incomplete {
			p = &Package{Load: lp, Errors: lp.Errors}
			failed[lp] = true
			if cfg.Partial && cfg.LazyPartial {
				p.partialFn = func() (*Partial, *MustInfo) { return checkPartial(prog.Fset, lp, checked, cgoh, false) }
				p.hasPartial = true
			} else if cfg.Partial {
				p.Partial, p.MustPartial = checkPartial(prog.Fset, lp, checked, cgoh, true)
				p.hasPartial = p.Partial != nil
			}
		} else {
			depsFailed, c := false, usesC(lp)
			for _, dep := range lp.Imports {
				depsFailed = depsFailed || failed[dep]
				c = c || hasC[dep]
			}
			hasC[lp] = c
			perrs := probeErrs[lp]
			cs, cerrs := substituteConstants(cfg.context(), prog.Fset, lp, flagsOf[lp], cfg)
			if len(cerrs) > 0 {
				perrs = append(slices.Clone(perrs), cerrs...)
			}
			focused := cfg.Focus != "" && stripOtherBodies(lp, cfg.Focus)
			var hresults func(*Package) []*ast.CallExpr
			if cfg.HResults != nil && c && !depsFailed && !lp.Std {
				hresults = func(p *Package) []*ast.CallExpr {
					// The hook reads what Check sets on the package when it is done (the flags of the probes).
					p.Flags, p.ProbeFlags, p.ExternC = flagsOf[lp].flags, flagsOf[lp].probe, flagsOf[lp].externC
					p.Opaque = provisionalOpaque(prog.Fset, p)
					ctx := cfg.Context
					if ctx == nil {
						ctx = context.Background()
					}
					calls, err := cfg.HResults(ctx, prog.Fset, p)
					if err != nil {
					}
					return calls
				}
			}
			p = checkPackageConsts(prog.Fset, lp, checked, cgoh, c && !depsFailed, perrs, cfg.NoDerivedBodies, cs, cfg.SkipOpaque && !c,
				cfg.AllowBodyless, hresults)
			p.Focused = focused
			p.Consts = cs.uses()
			p.Flags, p.ProbeFlags, p.ExternC = flagsOf[lp].flags, flagsOf[lp].probe, flagsOf[lp].externC
			if cfg.CppMembers != nil && c && !depsFailed && !lp.Std {
				// The second pass for the members of embedded C++ classes (cppmembers.go): it needs the
				// first pass's Opaque (the type expressions of the embedded fields), and changes it.
				ctx := cfg.Context
				if ctx == nil {
					ctx = context.Background()
				}
				p.resolveCppMembers(prog.Fset, ctx, cfg.CppMembers)
			}
			if p.typesClean && !typesMayBeIncomplete(p.Errors) {
				p.Opaque = &Opaque{exprs: map[ast.Expr]bool{}, typeExprs: map[types.Object]ast.Expr{}}
			} else {
				p.Opaque = findOpaque(prog.Fset, p)
			}
			checked[lp] = p.Types
			failed[lp] = depsFailed || hasHardErrors(p.Errors)
			sums.add(p) // its generic functions, for itself and its importers
			p.reportForeignCxxGenerics(prog.Fset, sums)
			if !p.typesClean {
				// After the opaque pass: limits of the lowering, not errors that leave a type
				// incomplete, so they do not turn the pass's sanity rule off, nor the checks of the
				// importers, whose types they do not change (genericc.go, pkgcvars.go).
				p.reportGenericCArgs(prog.Fset, sums)
				p.reportPkgCVars(prog.Fset)
			}
		}
		gates.focus = ""
		if p.Focused {
			gates.focus = cfg.Focus
		}
		gates.apply(p)
		r.Packages = append(r.Packages, p)
		r.byLoad[lp] = p
	}
	return r
}

// checkPartial type-checks the files of lp, a package with load errors,
// for Package.Partial: once, without the namespace probe (whose rewrites
// stay, so those names are opaque) and without the rewrite of conversions.
// Its errors are dropped: lp's own are the ones that count.
//
// The `!` forms of the files that parsed are prepared as for a package with type errors, so that their names
// mean what they mean in a clean file (the editor shows a file that is being typed: a syntax error in one
// place must not take the features of the others), and the places of the forms are the second result
// (Package.MustPartial). With rounds the calls that a must function handles without a `!` are found as well (more
// checks: one for each round); without, only the written ones are in the list. The parser's tree of a broken file is walked by the pass; if the pass panics on
// it, the package is checked as it is and has no list.
func checkPartial(fset *token.FileSet, lp *load.Package, checked map[*load.Package]*types.Package, cgoh *types.Package, rounds bool) (part *Partial, info *MustInfo) {
	cv := newConverter(fset, lp, cgoh)
	mp := newMustPass(fset, lp)
	var user map[*ast.Ident]bool
	if mp != nil {
		user = identsOf(lp)
		func() {
			defer func() {
				if recover() != nil {
					mp = nil
				}
			}()
			mp.prepare()
		}()
	}
	p, _ := checkFiles(fset, lp, checked, cv, nil, false)
	if mp != nil {
		func() {
			defer func() {
				if recover() != nil {
					info = nil
				}
			}()
			for range maxMustRounds {
				if !rounds || !mp.rewriteDropped(p.Info) {
					break
				}
				p, _ = checkFiles(fset, lp, checked, cv, nil, false)
			}
			markSynthetic(lp, user)
			info = mp.partialInfo(p.Info)
		}()
	}
	return &Partial{Types: p.Types, Info: p.Info, Opaque: findOpaque(fset, p)}, info
}

// hasErrors reports whether l has an error, soft or not.
func hasErrors(l *diag.List) bool {
	for _, d := range l.Diagnostics() {
		if d.Severity == diag.Error {
			return true
		}
	}
	return false
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
	cgoh *types.Package, convert bool, probeErrs []diag.Diagnostic, noDerived bool) *Package {
	return checkPackageConsts(fset, lp, checked, cgoh, convert, probeErrs, noDerived, nil, false, false, nil)
}

// checkPackageConsts is checkPackage for a package whose C/C++ constants were put into the tree
// (cs: constsubst.go): a substitution in a place that Go allows a variable in (a case, a shift
// count) that go/types rejects is taken back, and the package is checked again.
func checkPackageConsts(fset *token.FileSet, lp *load.Package, checked map[*load.Package]*types.Package,
	cgoh *types.Package, convert bool, probeErrs []diag.Diagnostic, noDerived bool, cs *constSubst, noOpaque bool, allowBodyless bool,
	hresults func(*Package) []*ast.CallExpr) *Package {
	cv := newConverter(fset, lp, cgoh)
	cv.consts = cs
	cv.allowBodyless = allowBodyless
	mp := newMustPass(fset, lp) // the ! forms of Phase 10 (must.go), or nil
	// The ! forms and #Derive make code that go/types can type with no error
	// while a rewrite or an error of ours leaves invalid types.
	cv.noOpaque = noOpaque && mp == nil && len(lp.Derives) == 0
	var userIdents map[*ast.Ident]bool
	if mp != nil || len(lp.Derives) > 0 {
		userIdents = identsOf(lp)
	}
	if mp != nil {
		mp.prepare()
	}
	if convert {
		cv.rewriteHelpers() // C.GoString(p) and the other cgo helpers (cgohelpers.go)
	}
	p, errs := checkFiles(fset, lp, checked, cv, probeErrs, convert)
	p.checks = 1
	for cs.revert(p.Errors) {
		checks := p.checks + 1
		p, errs = checkFiles(fset, lp, checked, cv, probeErrs, convert)
		p.checks = checks
	}
	if hresults != nil && cv.wrapHResults(hresults(p)) {
		// The calls that return an HRESULT are errors (hresults.go), before any rewrite: the rewrites
		// of the uses of a value that go/types has no type for (a slice, a comparison, a conversion)
		// would otherwise be made for the variable that holds the error.
		checks := p.checks + 1
		p, errs = checkFiles(fset, lp, checked, cv, probeErrs, convert)
		p.checks = checks
	}
	for {
		// Both rewrites look at the same check; the package is checked again
		// when one of them changed it.
		changed := convert && cv.rewrite(p, errs)
		if mp != nil && mp.rewriteDropped(p.Info) {
			changed = true
		}
		if !changed {
			break
		}
		checks := p.checks + 1
		p, errs = checkFiles(fset, lp, checked, cv, probeErrs, convert)
		p.checks = checks
	}
	if mp != nil {
		// The helpers of the ! forms go: the post-check says what is wrong with
		// them, then the lowering writes the code they stand for and the
		// package is checked for the last time (mustlower.go).
		mp.postCheck(p)
		if !hasErrors(p.Errors) {
			// Not even a soft error (no new variables, declared and not used): the
			// check of the lowered code would drop it, and its messages would name the
			// code that the lowering wrote.
			before := len(mp.errs)
			if mp.lower(p.Info, p.Types) {
				checks := p.checks + 1
				p, _ = checkFiles(fset, lp, checked, cv, probeErrs, convert)
				p.checks = checks
			} else {
				for _, d := range mp.errs[before:] {
					p.Errors.Add(d)
				}
			}
		}
	}
	if convert {
		cv.checkWideArgs(p, fset) // C.GoStringW(badType): a message of ours, not the C++ compiler's
	}
	if len(lp.Derives) > 0 && !hasErrors(p.Errors) {
		// #Derive, second step: the bodies of the methods that the loader stubbed (derive.go).
		if deriveBodies(fset, lp, p, noDerived) {
			checks := p.checks + 1
			derived := p.Derived
			p, _ = checkFiles(fset, lp, checked, cv, probeErrs, convert)
			p.checks, p.Derived = checks, derived
		}
	}
	if userIdents != nil {
		markSynthetic(lp, userIdents)
	}
	if mp != nil && !hasErrors(p.Errors) {
		p.Must = mp.mustInfo()
	} else if mp != nil {
		p.MustPartial = mp.partialInfo(p.Info)
	}
	cv.hintHResults(p)
	if len(cv.names) > 0 {
		p.setConversions(findConversions(lp, p.Info))
		p.slices = findSlices(lp, p.Info)
		p.hidden = maps.Clone(cv.names)
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
	fix := cv.fixMessage
	if mf := macroFixer(lp); mf != nil {
		fix = func(msg string) string { return mf(cv.fixMessage(msg)) }
	}
	fixAt := deriveFixer(lp)
	if cv.consts != nil {
		derived := fixAt
		fixAt = func(pos token.Pos, msg string) string {
			if derived != nil {
				msg = derived(pos, msg)
			}
			return cv.consts.fixAt(pos, msg)
		}
	}
	c := &collector{fset: fset, list: errs, seen: make(map[errorKey]bool), fix: fix,
		buffer: true, fixAt: fixAt}
	imp := &importer{from: lp, checked: checked, refused: make(map[string]bool), cgoh: cv.cgoh}
	if lp.Incomplete {
		// Check doesn't check such a package, but a caller that wants
		// the types of a broken package can (the language server,
		// Phase 11). Then each import that the loader couldn't give it
		// has a load error already, and go/types' "could not import"
		// would say it again.
		c.refused = imp.refused
	}
	info := newInfoFor(lp)
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
	pkg, crash := safeTypesCheck(&conf, lp.ImportPath, fset, files, info)
	if crash != "" {
		// go/types panicked (Go 1.25: "nil underlying" for type T T.x.y): the package gets an
		// error instead of a crash of gocpp check or of the language server, and an empty Info: the
		// passes that follow read the result of go/types and would meet its broken types. (Found by the 15.8 fuzzer.)
		var at token.Position
		if len(files) > 0 && files[0] != nil {
			at = fset.Position(files[0].Package)
		}
		errs.Add(diag.Diagnostic{Pos: at, Severity: diag.Error, Msg: "internal error: the type checker failed on this package: " + crash})
		return &Package{Load: lp, Types: pkg, Info: newInfo(), Errors: errs}, []token.Pos{token.NoPos}
	}
	// checkOpaqueArgs and checkSkipped write into info and into the scope
	// tree of pkg (types.CheckExpr), so they run once in each round of
	// checks, after conf.Check, on the round's own fresh info and pkg.
	used := checkOpaqueArgs(fset, pkg, files, info, c.checkExprError)
	// go/types recorded no error: no type is invalid, so with no C in the package
	// and its imports (cv.noOpaque), nothing can be opaque (Config.SkipOpaque).
	clean := cv.noOpaque && c.onlyUnused() && len(c.argErrors) == 0
	if !clean {
		checkSkipped(fset, pkg, files, info, c.checkExprError, used)
	}
	cppNames := make(map[token.Pos]string)
	for _, f := range lp.Files {
		if f.Rewrite != nil {
			for _, u := range f.Rewrite.Uses {
				cppNames[u.Ident.Pos()] = u.Ident.Name
			}
		}
	}
	if !dropFalse {
		// (With dropFalse, which is convert, the rewrite wraps such a call in gp_cgoh.UnsafeSize, which is no
		// constant: unsafecalls.go.)
		checkSizeCParts(fset, files, info, cppNames, c.checkExprError)
	}
	checkPrintSlices(fset, pkg, files, info, c.checkExprError)
	checkExternFuncs(fset, lp, files, info, cv.allowBodyless, c.checkExprError)
	falseErrs := newFalseErrors(pkg, info, files)
	inPlainMacroUse := func(token.Pos) bool { return false }
	if plain := plainMacroUses(lp, files, info); plain != nil {
		inPlainMacroUse = reportPlainMacroUses(fset, errs, plain) // macromsg.go
	}
	deriveJSONFiles := map[string]bool{} // the files that have a #Derive(JSON)
	for _, d := range lp.Derives {
		for _, g := range d.Gens {
			if g.Name == "JSON" {
				deriveJSONFiles[fset.Position(d.File.AST.Package).Filename] = true
			}
		}
	}
	c.hinter = newHeaderHinter(fset, lp, info, pkg) // hints.go
	c.flush(func(err types.Error) bool {
		if diag.ErrorCode(err) == diag.UnusedVar {
			return used.vars[err.Pos]
		}
		pn := pkgNameAt(files, info, err.Pos)
		if pn != nil && pn.Imported() != nil && pn.Imported().Path() == "encoding/json" && deriveJSONFiles[fset.Position(err.Pos).Filename] {
			return true // #Derive(JSON) writes calls of it in this file once go/types is done (derive.go)
		}
		return pn != nil && used.imports[pn]
	}, func(err types.Error) bool {
		return inPlainMacroUse(err.Pos) || dropFalse && falseErrs.drop(err)
	})
	return &Package{Load: lp, Types: pkg, Info: info, Errors: errs, typesClean: clean, typeErrors: c.typeErrors}, c.blocking
}

// newInfo returns an empty types.Info that records everything.
func newInfo() *types.Info {
	return newInfoSized(0)
}

// newInfoFor returns an empty types.Info with room for what go/types records for the files of lp:
// the maps would otherwise grow, and be copied, several times.
func newInfoFor(lp *load.Package) *types.Info {
	return newInfoSized(checkedBytes(lp))
}

// checkedBytes is the size of the source that go/types looks at for the files of lp: the size of the
// files, but for a file whose function bodies the parser left out (load.File.BodiesSkipped; the focused
// check of the language server, which sizes its maps for one file of bodies, not for every file of the
// package) the size of its declarations.
func checkedBytes(lp *load.Package) int {
	bytes := 0
	for _, f := range lp.Files {
		if !f.BodiesSkipped || f.AST == nil {
			bytes += len(f.Src)
			continue
		}
		for _, decl := range f.AST.Decls {
			bytes += int(decl.End() - decl.Pos())
		}
	}
	return bytes
}

// newInfoSized is newInfo for files of about bytes bytes of source. (Measured on generated
// packages: a byte of source has 0.09 typed expressions, 0.02 uses, and so on.)
func newInfoSized(bytes int) *types.Info {
	return &types.Info{
		Types:        make(map[ast.Expr]types.TypeAndValue, bytes/11),
		Instances:    make(map[*ast.Ident]types.Instance),
		Defs:         make(map[*ast.Ident]types.Object, bytes/48),
		Uses:         make(map[*ast.Ident]types.Object, bytes/16),
		Implicits:    make(map[ast.Node]types.Object, bytes/128),
		Selections:   make(map[*ast.SelectorExpr]*types.Selection, bytes/100),
		Scopes:       make(map[ast.Node]*types.Scope, bytes/64),
		FileVersions: make(map[*ast.File]string),
	}
}

// safeTypesCheck is conf.Check with a recover: the panic of go/types is returned as text, and the
// package is an empty one of the right path and name.
func safeTypesCheck(conf *types.Config, path string, fset *token.FileSet, files []*ast.File, info *types.Info) (pkg *types.Package, crash string) {
	defer func() {
		if r := recover(); r != nil {
			name := "main"
			if len(files) > 0 && files[0] != nil && files[0].Name != nil {
				name = files[0].Name.Name
			}
			pkg, crash = types.NewPackage(path, name), fmt.Sprint(r)
		}
	}()
	pkg, _ = conf.Check(path, fset, files, info)
	return pkg, ""
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
	if path == load.MustPath {
		return hiddenMust(), nil // only the rewrite of the ! forms imports it (must.go)
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

	// typeErrors says that flush kept an error of go/types that is not about something unused.
	typeErrors bool

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

	// fixAt is fix for a message that depends on the place (the stubs of #Derive, derive.go), or nil.
	fixAt func(pos token.Pos, msg string) string

	// hinter adds the hint for a header without a name to the errors that its name causes (hints.go), or is nil.
	hinter *headerHinter
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
	if c.fixAt != nil {
		terr.Msg = c.fixAt(terr.Pos, terr.Msg)
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
			if msg, ok := c.hinter.hint(c.fset, f.err); ok && !strings.HasPrefix(f.err.Msg, "\t") {
				f.err.Msg = msg
			}
			if code := diag.ErrorCode(f.err); code != diag.UnusedImport && code != diag.UnusedVar {
				c.typeErrors = true
			}
			c.list.Collect(f.err)
		}
	}
	typeErrors := !c.onlyUnused()
	for _, err := range c.argErrors {
		if typeErrors && isElidedCError(err.Msg) {
			continue // a type that go/types left invalid because of its own error is no C/C++ type (mapvalue.go)
		}
		c.list.Collect(err)
	}
	c.found, c.argErrors = nil, nil
}

// onlyUnused reports whether the errors that go/types found are all about an unused import or
// variable: such an error leaves every type valid and complete (typesMayBeIncomplete), so with no
// C in the package nothing in it can be opaque. A focused check always has them, for the imports
// that only the bodies it left out use; a file that is being typed in has them too.
func (c *collector) onlyUnused() bool {
	for _, f := range c.found {
		if code := diag.ErrorCode(f.err); code != diag.UnusedImport && code != diag.UnusedVar {
			return false
		}
	}
	return true
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
