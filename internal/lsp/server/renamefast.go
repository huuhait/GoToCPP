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

// seenByImporters reports whether the symbol is an exported package-level name
// that the packages importing its package see by a qualified name: a function,
// variable or constant (the scope of imports), or a type (the scope of the
// module: its embedded fields are seen wherever a value of their struct comes,
// and the proofs leave out a type that a field embeds).
func (sym *symbol) seenByImporters() bool {
	if sym.scope == scopeImports {
		return true
	}
	if sym.scope != scopeModule || len(sym.objs) != 1 {
		return false
	}
	_, isType := sym.objs[0].(*types.TypeName)
	return isType
}

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
// A type has the same rules when none of its hits is an embedded field of it
// (a field takes the name of its type, which can clash with another field, and
// is selected by it: those stay checked). A method or a field is left out (the
// method sets of the types change, which interfaces and promotion can see).
//
// Checking the importers one after the other was the cost of a rename: a
// package waits for the check of the package it imports (whose export data
// changed with the name), so a name that the packages of a chain all use was
// checked as many times as the chain is long.
func (rn *renamer) qualifiedOnly() map[string]bool {
	sym := rn.sym
	if recheckAllImporters || !sym.seenByImporters() || sym.declDir == "" || sym.declPath == "" ||
		!sym.packageLevel() {
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
	case *types.TypeName:
		// Every hit is the type's own name: an embedded field of the type takes the
		// name of its type, so its name changes with the type's (a clash with another
		// field, a selector that is not qualified), and is a hit of another object.
		for _, h := range rn.res.hits {
			if _, ok := h.obj.(*types.TypeName); !ok {
				return nil
			}
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
	case *types.Func, *types.Const, *types.Var, *types.TypeName:
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
// nongeneric ordinary Go function, or a nongeneric defined type that no field
// embeds, changing the package-scope name changes no type or method set (the
// types are the same objects under another name). Those guards establish
// unchanged binding at edited uses, no imported/file/package name collision and
// no capture of existing uses of the new name. Completeness below proves every
// own-package occurrence is an edit. Other packages must already satisfy
// qualifiedOnly's existing proof. Everything outside this boundary keeps the
// scratch-check validator.
func (rn *renamer) declaringScopeProvesResult(
	ctx context.Context, dirs, qualified map[string]bool,
) bool {
	sym := rn.sym
	if ctx.Err() != nil || recheckAllImporters || skipConflictChecks || !rn.bindingsChecked ||
		!rn.declFilesKnown || rn.declHasTests || len(rn.res.tests) != 0 ||
		!sym.seenByImporters() || !sym.packageLevel() || !dirs[sym.declDir] ||
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
	if !found || declaration.id == nil || !v.normalBindingFacts() {
		return false
	}
	target := declaration.obj
	if target == nil || target.Parent() != v.types.Scope() || v.types.Scope().Lookup(sym.name) != target {
		return false
	}
	file := v.files[declaration.path]
	if file == nil {
		return false
	}
	switch obj := target.(type) {
	case *types.Func:
		if !ordinaryFunction(obj, file, declaration.id) {
			return false
		}
	case *types.TypeName:
		if !ordinaryType(obj, file, declaration.id) || rn.embedsType(obj) {
			return false
		}
	default:
		return false
	}

	// Do not infer completeness from hit counts: source mapping or a repaired
	// view can omit an occurrence. Reuse the checked package's identifier index.
	seen := make(map[*ast.Ident]bool)
	for _, h := range rn.res.hits {
		if filepath.Dir(h.path) != sym.declDir {
			continue
		}
		if h.v != v || h.obj != target || h.id == nil || h.spec != nil ||
			h.id.Name != sym.name || v.info.ObjectOf(h.id) != target ||
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
			if entry.obj == target && !seen[entry.id] {
				return false
			}
		}
	}
	return ctx.Err() == nil
}

// ordinaryFunction reports whether fn is a nongeneric function with a body,
// declared by the declaration with the name id of the file, and not a method.
func ordinaryFunction(fn *types.Func, file *ast.File, id *ast.Ident) bool {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() != nil || sig.TypeParams().Len() != 0 {
		return false
	}
	for _, node := range file.Decls {
		if decl, ok := node.(*ast.FuncDecl); ok && decl.Name == id {
			return decl.Recv == nil && decl.Body != nil
		}
	}
	return false
}

// ordinaryType reports whether tn is a nongeneric defined type (not an alias),
// declared by the type declaration with the name id of the file.
func ordinaryType(tn *types.TypeName, file *ast.File, id *ast.Ident) bool {
	if tn.IsAlias() {
		return false
	}
	named, ok := tn.Type().(*types.Named)
	if !ok || named.TypeParams().Len() != 0 || named.Obj() != tn {
		return false
	}
	for _, node := range file.Decls {
		decl, ok := node.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range decl.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name == id {
				return ts.Assign == token.NoPos && ts.TypeParams == nil
			}
		}
	}
	return false
}

// embedsType reports whether a field of the packages that the search looked at
// is embedded by the name of the type tn (T, *T, pkg.T or a generic form of
// them): the field takes the type's name, so the rename changes the field's.
func (rn *renamer) embedsType(tn *types.TypeName) bool {
	for _, v := range rn.res.views {
		list := v.identIndex().byName[tn.Name()]
		if list == nil {
			continue
		}
		for _, entries := range [][]identObj{list.defs, list.uses} {
			for _, entry := range entries {
				if field, ok := entry.obj.(*types.Var); ok && field.IsField() && field.Embedded() {
					return true
				}
			}
		}
	}
	return false
}

// localProvesResult applies after the conflict guards succeeded. A local
// variable, parameter or result is seen only in the block that declares it,
// from its declaration to the end of that block, so the new name can
// make an error only inside that range, where
//
//   - the block declares nothing of the new name, and no name written there
//     is the new name (a declaration, a use of a variable, constant, type or
//     import: a name the renamed one could hide or be hidden by);
//   - every occurrence of the old name that is the variable is an edit (the
//     index of the checked package lists them all), and every edit is an
//     occurrence of the variable;
//   - no return without operands in the range is in a function that has a
//     result of the new name (it finds its results by lookup, and writes no
//     name that the first condition could see: bareReturnShadows);
//
// then each edited name finds the variable first (nothing between it and the
// declaration has the new name), and no other name changes its binding: names
// before the declaration do not see the variable, and the names outside the
// block do not see its scope. Types, method sets and the declarations of the
// package do not change, so no error is made and none is lost. A field or
// method of the new name is not a name of the block's scope: it is found by
// selection or key, never by lookup, and is left out of the second condition.
// Anything else keeps the scratch-check validator.
func (rn *renamer) localProvesResult(ctx context.Context, dirs map[string]bool) bool {
	sym := rn.sym
	if ctx.Err() != nil || recheckAllImporters || skipConflictChecks || !rn.bindingsChecked ||
		sym.scope != scopeLocal || len(sym.objs) != 1 || len(sym.extra) != 0 || len(dirs) != 1 {
		return false
	}
	obj, ok := sym.objs[0].(*types.Var)
	if !ok || obj.IsField() || obj.Pkg() == nil || obj.Parent() == nil ||
		obj.Parent() == obj.Pkg().Scope() || obj.Parent() == types.Universe {
		return false
	}
	var v *view
	seen := make(map[*ast.Ident]bool, len(rn.res.hits))
	for _, h := range rn.res.hits {
		if v == nil {
			v = h.v
		}
		if h.v != v || h.spec != nil || h.id == nil || h.obj != obj || h.id.Name != sym.name ||
			!dirs[filepath.Dir(h.path)] || v.info.ObjectOf(h.id) != obj {
			return false
		}
		seen[h.id] = true
	}
	if v == nil || !v.normalBindingFacts() || obj.Pkg() != v.types {
		return false
	}
	if obj.Parent().Lookup(rn.newName) != nil {
		return false
	}
	index := v.identIndex().byName
	if list := index[sym.name]; list != nil {
		for _, entries := range [][]identObj{list.defs, list.uses} {
			for _, entry := range entries {
				if entry.obj == obj && !seen[entry.id] {
					return false
				}
			}
		}
	}
	// The variable is seen from the end of its declaration; its name is a
	// little before (the right side of a declaration is out of its scope, and
	// is looked at too: that is only more careful).
	first, last := obj.Pos(), obj.Parent().End()
	if !first.IsValid() || !last.IsValid() {
		return false
	}
	if list := index[rn.newName]; list != nil {
		for _, entries := range [][]identObj{list.defs, list.uses} {
			for _, entry := range entries {
				if entry.id.Pos() >= first && entry.id.Pos() < last && !selectedName(entry.obj) {
					return false
				}
			}
		}
	}
	// A bare return writes no name, and finds the results of its function by lookup: a result
	// of the new name that the variable now hides is an error ("result parameter not in scope
	// at return"), however far the return is from the variable (a nested block, a loop).
	if file := v.files[v.fset.Position(obj.Pos()).Filename]; file == nil ||
		bareReturnShadows(file, first, last, rn.newName) {
		return false
	}
	return ctx.Err() == nil
}

// bareReturnShadows reports whether a return with no operands between first and last is in a
// function (declaration or literal) that has a result called name.
func bareReturnShadows(file *ast.File, first, last token.Pos, name string) bool {
	found := false
	check := func(signature *ast.FuncType, body *ast.BlockStmt) {
		if body == nil || signature.Results == nil {
			return
		}
		named := false
		for _, field := range signature.Results.List {
			for _, id := range field.Names {
				named = named || id.Name == name
			}
		}
		if !named {
			return
		}
		ast.Inspect(body, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.FuncLit:
				return false // its returns are its own
			case *ast.ReturnStmt:
				if len(node.Results) == 0 && node.Pos() >= first && node.Pos() < last {
					found = true
				}
			}
			return !found
		})
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.FuncDecl:
			check(node.Type, node.Body)
		case *ast.FuncLit:
			check(node.Type, node.Body)
		}
		return !found
	})
	return found
}

// selectedName reports whether obj is found by selection or by key, and never
// by the lookup of a name in a scope: a field, a method or a label.
func selectedName(obj types.Object) bool {
	switch obj := obj.(type) {
	case *types.Var:
		return obj.IsField()
	case *types.Func:
		sig, ok := obj.Type().(*types.Signature)
		return ok && sig.Recv() != nil
	case *types.Label:
		return true
	}
	return false
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
