package server

import (
	"bytes"
	"context"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"
)

// recheckAllImporters is set by a test to see what the check of the result
// finds without qualifiedOnly or the declaring-scope binding proof.
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

// declaringScopeProvesResult applies only after the existing declaration,
// capture, use and test-name conflict guards succeeded. For an exported,
// nongeneric ordinary Go function, changing the package-scope name changes no
// type or method set. Those guards establish unchanged binding at edited uses,
// no imported/file/package name collision and no capture of existing uses of
// the new name. Completeness below proves every own-package occurrence is an
// edit. Other packages must already satisfy qualifiedOnly's existing proof.
// Everything outside this boundary keeps the scratch-check validator.
func (rn *renamer) declaringScopeProvesResult(
	ctx context.Context, dirs, qualified map[string]bool,
) bool {
	sym := rn.sym
	if ctx.Err() != nil || recheckAllImporters || skipConflictChecks || !rn.bindingsChecked ||
		!rn.declFilesKnown || rn.declHasTests || len(rn.res.tests) != 0 ||
		sym.scope != scopeImports || !sym.packageLevel() || !dirs[sym.declDir] ||
		!token.IsExported(sym.name) || !token.IsExported(rn.newName) || rn.newName == "C" {
		return false
	}
	// Runtime/standard-library paths can give function spellings extra compiler
	// meaning (fmt's formatting gates, for example). Prove only domain-qualified
	// module paths; short and command-line package paths keep full validation.
	head, _, _ := strings.Cut(sym.declPath, "/")
	if !strings.Contains(head, ".") {
		return false
	}
	for dir := range dirs {
		if dir != sym.declDir && !qualified[dir] {
			return false
		}
	}
	for _, view := range rn.res.views {
		if !view.normalBindingFacts() {
			return false
		}
	}

	var declaration hit
	found := false
	for _, h := range rn.res.hits {
		if h.def && filepath.Dir(h.path) == sym.declDir {
			if found {
				return false
			}
			declaration = h
			found = true
		}
	}
	v := declaration.v
	fn, ok := declaration.obj.(*types.Func)
	if !found || !ok || declaration.id == nil || !v.normalBindingFacts() {
		return false
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() != nil || sig.TypeParams().Len() != 0 ||
		fn.Parent() != v.types.Scope() || v.types.Scope().Lookup(sym.name) != fn {
		return false
	}
	file := v.files[declaration.path]
	if file == nil {
		return false
	}
	ordinary := false
	for _, node := range file.Decls {
		if decl, ok := node.(*ast.FuncDecl); ok && decl.Name == declaration.id {
			ordinary = decl.Recv == nil && decl.Body != nil
		}
	}
	if !ordinary {
		return false
	}

	// Do not infer completeness from hit counts: source mapping or a repaired
	// view can omit an occurrence. Reuse the checked package's identifier index.
	seen := make(map[*ast.Ident]bool)
	for _, h := range rn.res.hits {
		if filepath.Dir(h.path) != sym.declDir {
			continue
		}
		if h.v != v || h.obj != fn || h.id == nil || h.spec != nil ||
			h.id.Name != sym.name || v.info.ObjectOf(h.id) != fn ||
			v.types.Scope().Innermost(h.id.Pos()) == nil {
			return false
		}
		seen[h.id] = true
	}
	list := v.identIndex().byName[sym.name]
	if list == nil {
		return false
	}
	for _, entries := range [][]identObj{list.defs, list.uses} {
		for _, entry := range entries {
			if entry.obj == fn && !seen[entry.id] {
				return false
			}
		}
	}
	return ctx.Err() == nil
}

type renameBindingFactsKey struct{}

// normalBindingFacts rejects partial, provisional, repaired or extension-bearing
// views. The memo derives only from the immutable checked package, never from a
// view-specific repair or mutable probe state. Like the full validator, these
// facts describe checked Go/GP files, not references in untracked C++ companions.
func (v *view) normalBindingFacts() bool {
	if v == nil || v.test || v.pkg == nil || v.pkg.Check == nil || v.pkg.Program == nil {
		return false
	}
	pkg := v.pkg
	cp := pkg.Check
	lp := cp.Load
	if lp == nil || len(pkg.Program.Roots) != 1 || pkg.Program.Roots[0] != lp ||
		v.info != cp.Info || v.types != cp.Types || v.fset != pkg.Fset() ||
		pkg.IsVariant() || pkg.Provisional() || pkg.Stale() || len(v.files) != len(lp.Files) {
		return false
	}
	for _, file := range lp.Files {
		if v.files[file.Path] != file.AST {
			return false
		}
	}
	return pkg.Memo(renameBindingFactsKey{}, func() any {
		if !cp.Checked() || cp.Info == nil || cp.Partial != nil || lp.Std || lp.Incomplete ||
			len(lp.Files) == 0 || pkg.Program.Errors == nil || cp.Errors == nil || lp.Errors == nil ||
			pkg.Program.Errors.HasErrors() || cp.Errors.HasErrors() || lp.Errors.HasErrors() ||
			cp.Must != nil ||
			len(lp.Macros)+len(lp.MacroUses)+len(lp.Derives)+len(lp.CFiles)+len(lp.CXXFiles) != 0 ||
			len(cp.Conversions)+len(cp.Consts) != 0 {
			return false
		}
		for range cp.Opaque.TypeExprs() {
			return false
		}
		for _, file := range lp.Files {
			if file.AST == nil || file.Info == nil || cp.Info.Scopes[file.AST] == nil ||
				len(file.Synthetic)+len(file.Directives)+len(file.Info.Includes)+
					len(file.Info.NamespaceRenames) != 0 || len(file.Src) == 0 ||
				bytes.Contains(file.Src, []byte("#")) ||
				bytes.Contains(file.Src, []byte("//go:")) ||
				bytes.Contains(file.Src, []byte("//gocpp:")) {
				return false
			}
			if rewrite := file.Rewrite; rewrite != nil &&
				(len(rewrite.Uses)+len(rewrite.Namespaces)+len(rewrite.Renames) != 0 ||
					rewrite.ImportC != nil) {
				return false
			}
			clean := true
			ast.Inspect(file.AST, func(node ast.Node) bool {
				if expr, ok := node.(ast.Expr); ok && cp.Opaque.IsOpaque(expr) {
					clean = false
				}
				return clean
			})
			if !clean {
				return false
			}
		}
		return true
	}).(bool)
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
