package vet

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"maps"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// A Config says what Run vets.
type Config struct {
	// Roots are the packages to report on, in the order they are listed.
	// Run analyzes the packages they import too, so that the analyzers
	// with facts (printf) learn from them, but reports nothing there. A
	// root that was not type-checked is an error.
	Roots []*load.Package

	// Analyzers are the analyzers to run: Default, or a selection of
	// Analyzers.
	Analyzers []*Analyzer

	// GOARCH is the target, for the sizes of the types (the shift analyzer
	// needs the size of int). "" means amd64.
	GOARCH string
}

// A Finding is one problem that an analyzer reports.
type Finding struct {
	Package  string // the import path of the package
	Analyzer string
	Pos      token.Position // in the original .gp source
	End      token.Position // the zero Position if the analyzer gave no end
	Message  string
	Related  []Related
}

// Related is another place that a finding mentions.
type Related struct {
	Pos     token.Position
	Message string
}

// A Note is something Run says besides the findings.
type Note struct {
	Package  string
	Analyzer string

	// Skipped is true when the analyzer did not run on the package, or not
	// on all of it, because it has C/C++ names (see illTyped), which
	// analyzers that need complete types can't deal with. It is false when the analyzer crashed or
	// failed: that is a gocpp bug.
	Skipped bool

	Text string

	// Where are the places in the source of what was left out, if the note
	// is about that.
	Where []token.Position
}

// A Report is the result of Run.
type Report struct {
	// Findings are sorted by package (in the order of Config.Roots), file,
	// line, column, analyzer and message, with no repeats, so that the
	// output of vet doesn't depend on the order the analyzers finish in.
	Findings []Finding

	// Notes are sorted by package, analyzer and text. A skipped analyzer
	// of a root package is there; so is every crash, in any package.
	Notes []Note
}

// Crashed returns the notes of analyzers that crashed or failed.
func (r *Report) Crashed() []Note {
	return slices.DeleteFunc(slices.Clone(r.Notes), func(n Note) bool { return n.Skipped })
}

// Run vets the checked packages of the program with the analyzers of cfg.
// prog and res come from load.Load and check.Check; the type information is
// the one of go/types, over the syntax trees of the .gp files, so the
// positions of the findings are positions in the original source.
//
// Run assumes that the roots and the packages they import are type-checked
// (Package.Checked) and have no errors; it returns an error otherwise.
func Run(prog *load.Program, res *check.Result, cfg Config) (*Report, error) {
	if len(cfg.Analyzers) == 0 {
		return &Report{}, nil
	}
	goarch := cmp.Or(cfg.GOARCH, "amd64")
	sizes := types.SizesFor("gc", goarch)
	if sizes == nil {
		return nil, fmt.Errorf("vet: unknown GOARCH %q", goarch)
	}

	// The units: the roots and everything they import, each after its
	// imports (prog.Packages has that order).
	needed := make(map[*load.Package]bool)
	var mark func(p *load.Package) error
	mark = func(p *load.Package) error {
		if needed[p] {
			return nil
		}
		cp := res.Package(p)
		if cp == nil || !cp.Checked() {
			return fmt.Errorf("vet: package %s was not type-checked", p.ImportPath)
		}
		needed[p] = true
		for _, dep := range p.Imports {
			if err := mark(dep); err != nil {
				return err
			}
		}
		return nil
	}
	for _, root := range cfg.Roots {
		if err := mark(root); err != nil {
			return nil, err
		}
	}
	isRoot := make(map[*load.Package]bool)
	for _, r := range cfg.Roots {
		isRoot[r] = true
	}
	var units []*unit
	byLoad := make(map[*load.Package]*unit)
	for _, p := range prog.Packages {
		if !needed[p] {
			continue
		}
		cp := res.Package(p)
		files := make([]*ast.File, len(p.Files))
		for i, f := range p.Files {
			files[i] = f.AST
		}
		u := &unit{path: p.ImportPath, pkg: cp.Types, files: files, info: cp.Info, root: isRoot[p]}
		inputsOf.Store(cp.Types, &pkgInputs{path: p.ImportPath, must: cp.Must, macros: p.Macros})
		defer inputsOf.Delete(cp.Types)
		u.tainted = illTyped(cp.Info, prog.Fset)
		typeParenBlanks(cp.Info, files)
		for _, path := range slices.Sorted(maps.Keys(p.Imports)) {
			if dep := byLoad[p.Imports[path]]; dep != nil {
				u.deps = append(u.deps, dep)
			}
		}
		byLoad[p] = u
		units = append(units, u)
	}

	var as []*analysis.Analyzer
	tolerant := make(map[*analysis.Analyzer]bool)
	for _, a := range cfg.Analyzers {
		as = append(as, a.Analyzer)
		if a.Opaque {
			tolerant[a.Analyzer] = true
		}
	}
	found, notes, err := drive(units, as, tolerant, sizes, prog.Fset)
	if err != nil {
		return nil, fmt.Errorf("vet: %w", err)
	}

	rep := &Report{}
	rank := make(map[string]int)
	for i, r := range cfg.Roots {
		rank[r.ImportPath] = i
	}
	for u, fs := range found {
		for _, f := range fs {
			// The code that #Derive wrote is not the user's: the signature of the method (at the
			// #Derive line) says what is wrong with the type.
			if strings.HasPrefix(prog.Fset.PositionFor(f.diag.Pos, false).Filename, check.DerivedFilePrefix) {
				continue
			}
			fd := Finding{
				Package:  u.path,
				Analyzer: f.analyzer,
				Pos:      prog.Fset.Position(f.diag.Pos),
				Message:  f.diag.Message,
			}
			if f.diag.End.IsValid() {
				fd.End = prog.Fset.Position(f.diag.End)
			}
			for _, r := range f.diag.Related {
				fd.Related = append(fd.Related, Related{prog.Fset.Position(r.Pos), r.Message})
			}
			rep.Findings = append(rep.Findings, fd)
		}
	}
	sort.SliceStable(rep.Findings, func(i, j int) bool {
		a, b := rep.Findings[i], rep.Findings[j]
		return cmp.Or(
			cmp.Compare(rank[a.Package], rank[b.Package]),
			cmp.Compare(a.Package, b.Package),
			cmp.Compare(a.Pos.Filename, b.Pos.Filename),
			cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Pos.Column, b.Pos.Column),
			cmp.Compare(a.Analyzer, b.Analyzer),
			cmp.Compare(a.Message, b.Message),
		) < 0
	})
	rep.Findings = slices.CompactFunc(rep.Findings, func(a, b Finding) bool {
		return a.Package == b.Package && a.Analyzer == b.Analyzer && a.Pos == b.Pos && a.Message == b.Message
	})
	selected := make(map[string]bool)
	for _, a := range cfg.Analyzers {
		selected[a.Name] = true
	}
	for _, n := range notes {
		if n.skipped && !selected[n.analyzer] {
			continue // an analyzer that others need (buildssa), not one the user asked for
		}
		rep.Notes = append(rep.Notes, Note{n.pkg, n.analyzer, n.skipped, n.text, n.where})
	}
	sort.SliceStable(rep.Notes, func(i, j int) bool {
		a, b := rep.Notes[i], rep.Notes[j]
		return cmp.Or(
			cmp.Compare(rank[a.Package], rank[b.Package]),
			cmp.Compare(a.Package, b.Package),
			cmp.Compare(a.Analyzer, b.Analyzer),
			cmp.Compare(a.Text, b.Text),
		) < 0
	})
	return rep, nil
}
