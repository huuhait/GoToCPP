package cache

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"sort"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// apiHash returns the API hash of the loaded and checked package lp (with
// its check.Dep dep): the hash of everything that the packages that import
// it can see of it (README §2.2 item 5). When an edit leaves it the same,
// such as an edit in a function body, the importers are not checked again
// (their keys have the API hashes of their imports, see keyOf).
//
// It has the import path, the package name, the exported package-level
// names that the loader gives a dot import, whether the package is
// incomplete, has C or failed (check.Dep), its errors about the package as
// a whole (which its importers report, see reportImportErrors), and the
// declarations of the
// package's scope without their positions: every type (also an unexported
// one, which an exported declaration can use: its fields and methods are
// visible through it) with its type parameters, underlying type and
// methods, and the exported functions, variables and constants with their
// types and constant values. Positions are left out, so a line added above
// a declaration changes nothing (the positions of the objects of an
// import may then be old in an importer's types.Info until it is checked
// again). A type from C or C++ (opaque) is "invalid type" here, as in the
// export data, so the hash has the side table of opaque types too
// (opaque, see opaqueTableOf): an edit that changes only such a type
// (C.int to C.long) changes the hash.
//
// An exported declaration can show the types of another package (a
// function that returns a *bufio.Reader), whose API can change while this
// package's declarations stay the same; so the hash has the API hashes of
// the imports too (deps): a change of an API changes the API hashes of
// all the packages that import it, directly or not.
func apiHash(lp *load.Package, dep check.Dep, deps []*summary, opaque []opaqueEntry) Hash {
	h := newHasher("api")
	h.str(lp.ImportPath)
	h.str(lp.Name)
	h.bool(lp.Incomplete)
	h.bool(dep.HasC)
	h.bool(dep.Failed)
	h.bool(dep.Types != nil)
	// The errors that the importers report at their imports
	// (reportImportErrors).
	for _, d := range packageErrors(lp.Errors) {
		h.str(d.Msg)
	}

	asts := make([]*ast.File, len(lp.Files))
	for i, f := range lp.Files {
		asts[i] = f.AST
	}
	var exported []string
	for name := range load.PackageLevelNames(asts) {
		if token.IsExported(name) {
			exported = append(exported, name)
		}
	}
	slices.Sort(exported)
	h.int(int64(len(exported)))
	for _, name := range exported {
		h.str(name)
	}

	if dep.Types != nil {
		writeAPI(h, dep.Types)
	}
	h.int(int64(len(opaque)))
	for _, e := range opaque {
		h.str(string(e.Path))
		h.str(e.Type)
	}

	deps = slices.DeleteFunc(slices.Clone(deps), func(s *summary) bool { return s == nil }) // nil: cyclic
	sort.Slice(deps, func(i, j int) bool { return deps[i].importPath < deps[j].importPath })
	h.int(int64(len(deps)))
	for _, d := range deps {
		h.str(d.importPath)
		h.hash(d.apiHash)
	}
	return h.sum()
}

// writeAPI writes the declarations of pkg that importers can see, without
// positions (see apiHash).
func writeAPI(h *hasher, pkg *types.Package) {
	qual := func(p *types.Package) string { return p.Path() }
	typ := func(t types.Type) string { return types.TypeString(t, qual) }
	tparams := func(list *types.TypeParamList) {
		h.int(int64(list.Len()))
		for i := range list.Len() {
			tp := list.At(i)
			h.str(tp.Obj().Name())
			h.str(typ(tp.Constraint()))
		}
	}
	scope := pkg.Scope()
	for _, name := range scope.Names() { // sorted
		switch obj := scope.Lookup(name).(type) {
		case *types.TypeName:
			h.str("type")
			h.str(name)
			switch t := obj.Type().(type) {
			case *types.Alias:
				h.str("=")
				tparams(t.TypeParams())
				h.str(typ(t.Rhs()))
			case *types.Named:
				tparams(t.TypeParams())
				h.str(typ(t.Underlying()))
				methods := make([]*types.Func, t.NumMethods())
				for i := range methods {
					methods[i] = t.Method(i)
				}
				sort.Slice(methods, func(i, j int) bool { return methods[i].Name() < methods[j].Name() })
				h.int(int64(len(methods)))
				for _, m := range methods {
					sig := m.Type().(*types.Signature)
					_, ptr := sig.Recv().Type().(*types.Pointer)
					h.str(m.Name())
					h.bool(ptr)
					h.str(typ(sig))
				}
			default:
				h.str(typ(t)) // an invalid type
			}
		case *types.Func:
			if obj.Exported() {
				h.str("func")
				h.str(name)
				sig := obj.Type().(*types.Signature)
				tparams(sig.TypeParams())
				h.str(typ(sig))
			}
		case *types.Var:
			if obj.Exported() {
				h.str("var")
				h.str(name)
				h.str(typ(obj.Type()))
			}
		case *types.Const:
			if obj.Exported() {
				h.str("const")
				h.str(name)
				h.str(typ(obj.Type()))
				h.str(obj.Val().ExactString())
			}
		}
	}
}
