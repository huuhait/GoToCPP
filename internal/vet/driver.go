package vet

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"sync"

	"golang.org/x/tools/go/analysis"
)

// A unit is one package for the driver: what an analysis.Pass needs of
// it, the packages it imports and the facts the analyzers learned in it.
type unit struct {
	path  string
	pkg   *types.Package
	files []*ast.File
	info  *types.Info
	deps  []*unit // the packages it imports, directly
	root  bool    // findings of a root are reported, those of others are not

	// tainted is why analyzers that can't cope with C/C++ names don't run
	// here ("" for a package that go/types types completely); see
	// illTyped.
	tainted string

	done chan struct{} // closed when every analyzer ran on the package

	mu       sync.Mutex
	objFacts map[objFactKey]analysis.Fact
	pkgFacts map[pkgFactKey]analysis.Fact
}

type objFactKey struct {
	obj types.Object
	typ reflect.Type
}

type pkgFactKey struct {
	pkg *types.Package
	typ reflect.Type
}

// A note is something the driver tells besides the findings: an analyzer
// that did not run on a package, or one that crashed.
type note struct {
	pkg, analyzer string
	skipped       bool // not run because of the package (see Report.Notes); else it crashed
	text          string
	where         []token.Position
}

// drive runs the analyzers on the units: first on the packages a unit
// imports, so that facts flow from a package to its importers, like in
// the checker of x/tools, but with the guard that gocpp needs. An analyzer
// that has no facts and isn't needed by one runs on the root packages
// only.
//
// Differences from checker.Analyze, which it replaces on purpose:
//   - a panic of an analyzer (it happens on code with C/C++ names, see
//     illTyped) is recovered and becomes a note, in every package, also in
//     the ones that are only imported. checker.Analyze can't do that: its
//     analyzers run in goroutines without a recover.
//   - a package with C/C++ names skips the analyzers that don't declare
//     RunDespiteErrors, in the roots and in the dependencies, without
//     failing the analyzers of its importers ("failed prerequisites").
//   - the work is spread over the packages and the analyzers by the
//     dependencies, with at most GOMAXPROCS passes at a time.
//
// It returns the findings of the roots (unsorted) and the notes.
func drive(units []*unit, analyzers []*analysis.Analyzer, tolerant map[*analysis.Analyzer]bool, sizes types.Sizes, fset *token.FileSet) (map[*unit][]found, []note, error) {
	if err := analysis.Validate(analyzers); err != nil {
		return nil, nil, err
	}
	order := topo(analyzers)
	// An analyzer that can run in a package with C/C++ names also needs
	// the analyzers that it requires there.
	tolerates := make(map[*analysis.Analyzer]bool)
	var allow func(a *analysis.Analyzer)
	allow = func(a *analysis.Analyzer) {
		tolerates[a] = true
		for _, r := range a.Requires {
			allow(r)
		}
	}
	for a := range tolerant {
		allow(a)
	}
	// forFacts: the analyzers that dependencies must run too, the ones
	// with facts and everything they need.
	forFacts := make(map[*analysis.Analyzer]bool)
	var need func(a *analysis.Analyzer)
	need = func(a *analysis.Analyzer) {
		if forFacts[a] {
			return
		}
		forFacts[a] = true
		for _, r := range a.Requires {
			need(r)
		}
	}
	for _, a := range order {
		if len(a.FactTypes) > 0 {
			need(a)
		}
	}
	byPkg := make(map[*types.Package]*unit, len(units))
	for _, u := range units {
		byPkg[u.pkg] = u
		u.done = make(chan struct{})
		u.objFacts = make(map[objFactKey]analysis.Fact)
		u.pkgFacts = make(map[pkgFactKey]analysis.Fact)
	}

	selectedNames := make(map[string]bool)
	for _, a := range analyzers {
		selectedNames[a.Name] = true
	}
	d := &driver{
		fset:     fset,
		sizes:    sizes,
		tolerant: tolerates,
		byPkg:    byPkg,
		slots:    make(chan struct{}, runtime.GOMAXPROCS(0)),
		found:    make(map[*unit][]found),
		factsOf:  make(map[*unit][]*unit),
	}
	var wg sync.WaitGroup
	for _, u := range units {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(u.done)
			for _, dep := range u.deps {
				<-dep.done
			}
			var as []*analysis.Analyzer
			for _, a := range order {
				if u.root || forFacts[a] {
					as = append(as, a)
				}
			}
			d.runUnit(u, as)
			// What pruning did, for the roots; and the cache of the
			// package is done with.
			for _, pn := range takePruned(u.info, fset) {
				if u.root && selectedNames[pn.analyzer] {
					d.note(note{u.path, pn.analyzer, true, pn.text(), pn.where})
				}
			}
		}()
	}
	wg.Wait()
	return d.found, d.notes, nil
}

// topo returns the analyzers and the ones they require, each after the
// ones it requires, in the order of the list otherwise.
func topo(as []*analysis.Analyzer) []*analysis.Analyzer {
	var out []*analysis.Analyzer
	seen := make(map[*analysis.Analyzer]bool)
	var visit func(a *analysis.Analyzer)
	visit = func(a *analysis.Analyzer) {
		if seen[a] {
			return
		}
		seen[a] = true
		for _, r := range a.Requires {
			visit(r)
		}
		out = append(out, a)
	}
	for _, a := range as {
		visit(a)
	}
	return out
}

type driver struct {
	fset     *token.FileSet
	sizes    types.Sizes
	byPkg    map[*types.Package]*unit
	slots    chan struct{}               // limits the passes that run at a time
	tolerant map[*analysis.Analyzer]bool // run in packages with C/C++ names although they are not RunDespiteErrors

	mu    sync.Mutex
	found map[*unit][]found
	notes []note

	factsMu sync.Mutex
	factsOf map[*unit][]*unit // the packages whose facts a unit sees: itself and its dependencies, direct or not
}

// A found is a diagnostic of an analyzer.
type found struct {
	analyzer string
	diag     analysis.Diagnostic
}

func (d *driver) note(n note) {
	d.mu.Lock()
	d.notes = append(d.notes, n)
	d.mu.Unlock()
}

// runUnit runs the analyzers as (each after the ones it requires) on u,
// the ones that don't wait for each other at the same time.
func (d *driver) runUnit(u *unit, as []*analysis.Analyzer) {
	type result struct {
		val  any
		ok   bool // the analyzer ran (or was skipped by the guard) without a failure
		done chan struct{}
	}
	results := make(map[*analysis.Analyzer]*result, len(as))
	for _, a := range as {
		results[a] = &result{done: make(chan struct{})}
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, a := range as {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res := results[a]
			defer close(res.done)
			inputs := make(map[*analysis.Analyzer]any, len(a.Requires))
			failed := ""
			for _, r := range a.Requires {
				rr := results[r]
				<-rr.done
				if !rr.ok {
					failed = r.Name
				}
				inputs[r] = rr.val
			}
			if failed != "" {
				d.note(note{u.path, a.Name, false, "not run: analyzer " + failed + ", which it needs, failed", nil})
				return
			}
			d.slots <- struct{}{}
			val, ok := d.pass(u, a, inputs)
			<-d.slots
			mu.Lock()
			res.val, res.ok = val, ok
			mu.Unlock()
		}()
	}
	wg.Wait()
}

// pass runs analyzer a on u and returns its result. ok is false when the
// analyzer crashed.
func (d *driver) pass(u *unit, a *analysis.Analyzer, inputs map[*analysis.Analyzer]any) (val any, ok bool) {
	if u.tainted != "" && !a.RunDespiteErrors && !d.tolerant[a] {
		if u.root {
			d.note(note{u.path, a.Name, true, u.tainted, nil})
		}
		// The importers of u get no facts from it, and no result: a typed
		// nil, as the analyzers that need this one are skipped here too
		// (they are not RunDespiteErrors either, see TestRequiresOfGuarded).
		if a.ResultType != nil {
			val = reflect.Zero(a.ResultType).Interface()
		}
		return val, true
	}
	var diags []analysis.Diagnostic
	pass := &analysis.Pass{
		Analyzer:   a,
		Fset:       d.fset,
		Files:      u.files,
		Pkg:        u.pkg,
		TypesInfo:  u.info,
		TypesSizes: d.sizes,
		Module:     &analysis.Module{GoVersion: "go1.25"},
		ResultOf:   inputs,
		ReadFile:   os.ReadFile,
		Report: func(dg analysis.Diagnostic) {
			if u.root {
				diags = append(diags, dg)
			}
		},
	}
	d.factFuncs(u, pass)
	defer func() {
		if r := recover(); r != nil {
			stack := strings.TrimSpace(string(debug.Stack()))
			d.note(note{u.path, a.Name, false, fmt.Sprintf("crashed: %v\n%s", r, stack), nil})
			val, ok = nil, false
		}
	}()
	res, err := a.Run(pass)
	if err != nil {
		d.note(note{u.path, a.Name, false, "failed: " + err.Error(), nil})
		return nil, false
	}
	if got, want := reflect.TypeOf(res), a.ResultType; got != want {
		d.note(note{u.path, a.Name, false, fmt.Sprintf("returned a result of type %v, not %v", got, want), nil})
		return nil, false
	}
	if u.root && len(diags) > 0 {
		d.mu.Lock()
		for _, dg := range diags {
			d.found[u] = append(d.found[u], found{a.Name, dg})
		}
		d.mu.Unlock()
	}
	return res, true
}

// factFuncs sets the fact functions of pass: facts are kept per package,
// the pass of a package sees its own and those of the packages it
// imports, directly or not.
func (d *driver) factFuncs(u *unit, pass *analysis.Pass) {
	allowed := make(map[reflect.Type]bool)
	for _, f := range pass.Analyzer.FactTypes {
		allowed[reflect.TypeOf(f)] = true
	}
	check := func(f analysis.Fact) reflect.Type {
		t := reflect.TypeOf(f)
		if !allowed[t] {
			panic(fmt.Sprintf("analyzer %s does not declare the fact type %T", pass.Analyzer.Name, f))
		}
		return t
	}
	pass.ExportObjectFact = func(obj types.Object, f analysis.Fact) {
		if obj.Pkg() != u.pkg {
			panic(fmt.Sprintf("analyzer %s exports a fact of %v, which is not an object of %s", pass.Analyzer.Name, obj, u.path))
		}
		t := check(f)
		u.mu.Lock()
		u.objFacts[objFactKey{obj, t}] = f
		u.mu.Unlock()
	}
	pass.ExportPackageFact = func(f analysis.Fact) {
		t := check(f)
		u.mu.Lock()
		u.pkgFacts[pkgFactKey{u.pkg, t}] = f
		u.mu.Unlock()
	}
	pass.ImportObjectFact = func(obj types.Object, ptr analysis.Fact) bool {
		t := check(ptr)
		src := d.byPkg[obj.Pkg()]
		if src == nil {
			return false
		}
		src.mu.Lock()
		f, ok := src.objFacts[objFactKey{obj, t}]
		src.mu.Unlock()
		if ok {
			reflect.ValueOf(ptr).Elem().Set(reflect.ValueOf(f).Elem())
		}
		return ok
	}
	pass.ImportPackageFact = func(pkg *types.Package, ptr analysis.Fact) bool {
		t := check(ptr)
		src := d.byPkg[pkg]
		if src == nil {
			return false
		}
		src.mu.Lock()
		f, ok := src.pkgFacts[pkgFactKey{pkg, t}]
		src.mu.Unlock()
		if ok {
			reflect.ValueOf(ptr).Elem().Set(reflect.ValueOf(f).Elem())
		}
		return ok
	}
	pass.AllObjectFacts = func() []analysis.ObjectFact {
		var out []analysis.ObjectFact
		for _, src := range d.visible(u) {
			src.mu.Lock()
			for k, f := range src.objFacts {
				if allowed[k.typ] {
					out = append(out, analysis.ObjectFact{Object: k.obj, Fact: f})
				}
			}
			src.mu.Unlock()
		}
		sort.SliceStable(out, func(i, j int) bool {
			a, b := out[i].Object, out[j].Object
			if a.Pkg() != b.Pkg() {
				return a.Pkg().Path() < b.Pkg().Path()
			}
			if a.Pos() != b.Pos() {
				return a.Pos() < b.Pos()
			}
			return a.Name() < b.Name()
		})
		return out
	}
	pass.AllPackageFacts = func() []analysis.PackageFact {
		var out []analysis.PackageFact
		for _, src := range d.visible(u) {
			src.mu.Lock()
			for k, f := range src.pkgFacts {
				if allowed[k.typ] {
					out = append(out, analysis.PackageFact{Package: k.pkg, Fact: f})
				}
			}
			src.mu.Unlock()
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].Package.Path() < out[j].Package.Path() })
		return out
	}
}

// visible returns u and the packages that u imports, directly or not.
func (d *driver) visible(u *unit) []*unit {
	d.factsMu.Lock()
	defer d.factsMu.Unlock()
	if vs, ok := d.factsOf[u]; ok {
		return vs
	}
	var vs []*unit
	seen := make(map[*unit]bool)
	var walk func(x *unit)
	walk = func(x *unit) {
		if seen[x] {
			return
		}
		seen[x] = true
		vs = append(vs, x)
		for _, dep := range x.deps {
			walk(dep)
		}
	}
	walk(u)
	d.factsOf[u] = vs
	return vs
}
