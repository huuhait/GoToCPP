package server

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
)

// recheckAllImporters is set by a test to see what the check of the result
// finds without the shortcut of qualifiedOnly.
var recheckAllImporters bool

// qualifiedOnly returns the folders of the packages that need no check of the
// result, because they have no use of the name to edit, or because their edits are only
// qualified uses (pkg.Name) of the renamed name, in a package other than the one
// that declares it, for a rename that cannot make an error there: they need no
// check of the result. The rename is of an exported function, variable or
// constant of a package, to another exported name. Then
//
//   - a use in an importer is the selector pkg.Name, where pkg is the import
//     of the package that declares the name, whatever it is called (a use by
//     a dot import is a plain name, which the new name could clash with, and
//     stays checked); the edit changes the Sel of the selector only, and the
//     new name is looked up in the exports of the declaring package, which is
//     checked (a clash there is an error of that package: "redeclared"), so
//     the use resolves to the same object with the same type;
//   - the scopes of the importer take no part in the lookup of a qualified
//     name, so no declaration of the importer can capture or hide the new name;
//   - the new name is exported like the old, so the importer may still use it;
//   - nothing else of the importer changes: its imports, its declarations and
//     the types of its expressions are what they were, so it has no error that
//     it did not have.
//
// A type is left out (an embedded field takes the name of its type, which can
// clash with another field), and so is a method or a field (the method sets
// of the types change, which interfaces and promotion can see).
//
// Checking the importers one after the other was the cost of a rename: a
// package waits for the check of the package it imports (whose export data
// changed with the name), so a name that the packages of a chain all use was
// checked as many times as the chain is long.
func (rn *renamer) qualifiedOnly() map[string]bool {
	sym := rn.sym
	if recheckAllImporters || sym.scope != scopeImports || sym.declDir == "" || sym.declPath == "" || !sym.packageLevel() {
		return nil
	}
	if !token.IsExported(sym.name) || !token.IsExported(rn.newName) {
		return nil
	}
	switch obj := sym.objs[0].(type) {
	case *types.Func, *types.Const:
	case *types.Var:
		if obj.IsField() {
			return nil
		}
	default:
		return nil
	}
	hitsOf := make(map[string][]hit)
	for _, h := range rn.res.hits {
		dir := filepath.Dir(h.path)
		hitsOf[dir] = append(hitsOf[dir], h)
	}
	out := make(map[string]bool)
	// A package the search looked at and found no use in has no edit: the new
	// name is not in its files, and the old name was not used, so only a dot
	// import of the declaring package (which brings the new name into its scope,
	// where a declaration of its own can clash with it) can break it.
	for _, v := range rn.res.views {
		if v.test || v.dir == "" || v.dir == sym.declDir || v.pkg == nil || v.info == nil || len(hitsOf[v.dir]) > 0 {
			continue
		}
		if !dotImports(v, sym.declPath) {
			out[v.dir] = true
		}
	}
	for dir, hits := range hitsOf {
		if dir == sym.declDir {
			continue
		}
		// A qualified use in one file does not make a dot import in another
		// file harmless: the new export can clash with a package declaration
		// there even when that file never mentions the old name.
		if v := hits[0].v; v == nil || dotImports(v, sym.declPath) {
			continue
		}
		only := true
		for _, h := range hits {
			if !rn.qualifiedUse(h) {
				only = false
				break
			}
		}
		if only {
			out[dir] = true
		}
	}
	return out
}

// qualifiedUse reports whether the hit is the Sel of pkg.Name in a checked
// package (not a test view), where pkg is an import of the package that
// declares the name.
func (rn *renamer) qualifiedUse(h hit) bool {
	v := h.v
	if h.def || h.spec != nil || h.id == nil || h.obj == nil || v == nil || v.test || v.pkg == nil || v.info == nil {
		return false
	}
	if v.synthetic[h.id] {
		return false
	}
	switch h.obj.(type) {
	case *types.Func, *types.Const, *types.Var:
	default:
		return false
	}
	sel := v.selectors()[h.id]
	if sel == nil || sel.Sel != h.id {
		return false
	}
	x, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pn, ok := v.info.Uses[x].(*types.PkgName)
	return ok && pn.Imported() != nil && pn.Imported().Path() == rn.sym.declPath
}

// dotImports reports whether a file of the view imports the package path with a
// dot.
func dotImports(v *view, path string) bool {
	for _, f := range v.files {
		for _, spec := range f.Imports {
			if spec.Name != nil && spec.Name.Name == "." {
				if p, err := strconv.Unquote(spec.Path.Value); err != nil || p == path {
					return true
				}
			}
		}
	}
	return false
}
