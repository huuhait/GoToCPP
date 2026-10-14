package cache

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/types/objectpath"

	"github.com/huuhait/gocpp/internal/check"
)

// The side table of opaque types (README §2.2 item 4, step 11.3b). go/types
// has no type for a part that comes from C or C++ (the parameter w of
// func Open(w *SDL.SDL_Window) has the type *invalid type), and neither has
// the export data. The opaque pass keeps the type expression that such an
// object was declared with (check.Opaque.TypeExpr), but only for the objects
// of the package it checked. So a summary keeps, beside the export data,
// the text of the type expressions of the objects that importers can see
// (by their objectpath), and a package checked with the summary of an
// import gets them back for that import's objects: Package.OpaqueType.
// The table is part of the API hash, so an edit that changes only such a
// type (C.int to C.long) checks the importers again.

// An opaqueEntry is the type of an object that importers can see, whose
// declared type comes from C or C++: a variable, a field, a constant or a
// type name with its type expression, or a function or method with C or
// C++ parts in its signature, with its signature as written
// ("func(w *SDL.SDL_Window) *SDL.SDL_Renderer"): go/types prints it with
// invalid types, and an unnamed result has no object of its own.
type opaqueEntry struct {
	Path objectpath.Path // the object in its package
	Type string          // the type expression as the source has it, on one line: *SDL.SDL_Window
}

// opaqueTableOf returns the side table of the checked package cp, sorted by
// path: the objects with a type expression that an importer can reach.
// src returns the content of a file of the package.
func opaqueTableOf(fset *token.FileSet, cp *check.Package, src func(name string) []byte) []opaqueEntry {
	if cp == nil || cp.Types == nil {
		return nil
	}
	var table []opaqueEntry
	var enc objectpath.Encoder
	scope := cp.Types.Scope()
	add := func(obj types.Object, text string) {
		if !reachable(obj, scope) {
			return
		}
		if path, err := enc.For(obj); err == nil { // else a local variable
			table = append(table, opaqueEntry{path, text})
		}
	}
	for obj, te := range cp.Opaque.TypeExprs() {
		if text, ok := exprText(fset, te, src); ok {
			add(obj, text)
		}
	}
	for obj, text := range funcSpellings(fset, cp, src) {
		add(obj, text)
	}
	slices.SortFunc(table, func(a, b opaqueEntry) int { return strings.Compare(string(a.Path), string(b.Path)) })
	return table
}

// reachable reports whether obj may have an objectpath: it is declared at
// the package level, or it is a field, or a parameter or result of a
// function declared at the package level (whose scope is in a file scope).
// It leaves out the variables of nested blocks without asking objectpath,
// which would look through the whole API for them.
func reachable(obj types.Object, pkgScope *types.Scope) bool {
	parent := obj.Parent()
	switch {
	case parent == pkgScope:
		return true
	case parent == nil:
		switch o := obj.(type) {
		case *types.Var:
			return o.IsField()
		case *types.Func:
			return true // a method
		}
		return false
	case parent.Parent() != nil && parent.Parent().Parent() == pkgScope:
		_, ok := obj.(*types.Var) // the function's own scope: also its top-level variables
		return ok
	}
	return false
}

// exprText returns the source text of the node e on one line, as
// gocpp check -types prints it.
func exprText(fset *token.FileSet, e ast.Node, src func(name string) []byte) (string, bool) {
	start, end := fset.Position(e.Pos()), fset.Position(e.End())
	data := src(start.Filename)
	if data == nil || start.Filename != end.Filename || start.Offset < 0 || end.Offset > len(data) || start.Offset > end.Offset {
		return "", false
	}
	return strings.Join(strings.Fields(string(data[start.Offset:end.Offset])), " "), true
}

// funcSpellings returns the signatures, as written, of the functions and
// methods of cp (also those of interfaces) that have C or C++ parts in their
// signature: "func", the type parameters, the parameters and the results,
// without the receiver and the name: "func[T any](x T) *C.X".
func funcSpellings(fset *token.FileSet, cp *check.Package, src func(name string) []byte) map[types.Object]string {
	if cp == nil || cp.Info == nil {
		return nil
	}
	m := make(map[types.Object]string)
	add := func(name *ast.Ident, ft *ast.FuncType) {
		obj, ok := cp.Info.Defs[name].(*types.Func)
		if !ok || !incomplete(obj.Type(), nil) {
			return
		}
		text, ok := exprText(fset, ft.Params, src)
		if !ok {
			return
		}
		if ft.TypeParams != nil {
			tparams, ok := exprText(fset, ft.TypeParams, src)
			if !ok {
				return
			}
			text = tparams + text
		}
		text = "func" + text
		if ft.Results != nil {
			res, ok := exprText(fset, ft.Results, src)
			if !ok {
				return
			}
			text += " " + res
		}
		m[obj] = text
	}
	for _, f := range cp.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				add(n.Name, n.Type)
			case *ast.InterfaceType:
				for _, field := range n.Methods.List {
					if ft, ok := field.Type.(*ast.FuncType); ok && len(field.Names) == 1 {
						add(field.Names[0], ft)
					}
				}
			}
			return true
		})
	}
	return m
}

// incomplete reports whether t has a part that go/types left invalid (a C
// or C++ type), as the opaque pass counts it: a named type is complete
// unless one of its type arguments is not. seen guards against cycles.
func incomplete(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}
	switch t := t.(type) {
	case *types.Basic:
		return t.Kind() == types.Invalid
	case *types.Alias:
		return incomplete(types.Unalias(t), seen)
	case *types.Named:
		for i := range t.TypeArgs().Len() {
			if incomplete(t.TypeArgs().At(i), seen) {
				return true
			}
		}
	case *types.Pointer:
		return incomplete(t.Elem(), seen)
	case *types.Slice:
		return incomplete(t.Elem(), seen)
	case *types.Array:
		return incomplete(t.Elem(), seen)
	case *types.Chan:
		return incomplete(t.Elem(), seen)
	case *types.Map:
		return incomplete(t.Key(), seen) || incomplete(t.Elem(), seen)
	case *types.Tuple:
		for i := range t.Len() {
			if incomplete(t.At(i).Type(), seen) {
				return true
			}
		}
	case *types.Signature:
		return incomplete(t.Params(), seen) || incomplete(t.Results(), seen)
	case *types.Struct:
		if seen == nil {
			seen = make(map[types.Type]bool)
		}
		seen[t] = true
		for i := range t.NumFields() {
			if incomplete(t.Field(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// mergeTables returns the side tables of a package and of the packages it
// imports, directly or not, by import path: own (if not empty) and the
// tables of the summaries deps (nil for a cyclic import). Only packages with
// C or C++ types have tables, so it is mostly empty.
func mergeTables(importPath string, own []opaqueEntry, deps []*summary) map[string][]opaqueEntry {
	var m map[string][]opaqueEntry
	add := func(path string, t []opaqueEntry) {
		if m == nil {
			m = make(map[string][]opaqueEntry)
		}
		m[path] = t
	}
	if len(own) > 0 {
		add(importPath, own)
	}
	for _, d := range deps {
		if d == nil {
			continue
		}
		for path, t := range d.tables {
			add(path, t)
		}
	}
	return m
}

// OpaqueType returns the type expression that the object obj was declared
// with, as its source has it on one line (*SDL.SDL_Window), when its type
// comes from C or C++: go/types has an invalid type for such a part. For a
// function or method with such parts in its signature, it returns the
// signature as written: "func(w *SDL.SDL_Window) *SDL.SDL_Renderer". It
// works for the objects of the package (from its opaque pass,
// check.Opaque.TypeExpr) and for the objects of the packages it imports,
// which come from export data (from the side table of their summaries),
// also when the summary was loaded from the disk cache. It returns false
// for every other object, and for one of an import cycle (loaded from
// source with the package). For a field or a method of an instance of a
// generic type, pass the object itself: OpaqueType takes its Origin.
func (pkg *Package) OpaqueType(obj types.Object) (string, bool) {
	switch o := obj.(type) {
	case nil:
		return "", false
	case *types.Var:
		obj = o.Origin()
	case *types.Func:
		obj = o.Origin()
	}
	if obj.Pkg() == nil {
		return "", false
	}
	if pkg.Check != nil && obj.Pkg() == pkg.Check.Types {
		src := func(name string) []byte { return pkg.src[name] }
		if _, ok := obj.(*types.Func); ok {
			pkg.funcOnce.Do(func() { pkg.funcTypes = funcSpellings(pkg.Fset(), pkg.Check, src) })
			t, ok := pkg.funcTypes[obj]
			return t, ok
		}
		te := pkg.Check.Opaque.TypeExpr(obj)
		if te == nil {
			return "", false
		}
		return exprText(pkg.Fset(), te, src)
	}
	pkg.depOnce.Do(pkg.decodeTables)
	t, ok := pkg.depTypes[obj]
	return t, ok
}

// decodeTables finds the objects of the side tables of the imports in the
// packages that the check read from their export data.
func (pkg *Package) decodeTables() {
	for path, table := range pkg.tables {
		tp := pkg.imported[path]
		if tp == nil {
			continue // the package itself, or not in the export data
		}
		for _, e := range table {
			obj, err := objectpath.Object(tp, e.Path)
			if err != nil {
				continue // not in the export data: no importer sees it
			}
			if pkg.depTypes == nil {
				pkg.depTypes = make(map[types.Object]string)
			}
			pkg.depTypes[obj] = e.Type
		}
	}
}
