package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
)

// checkOpaqueArgs type-checks the type arguments of the opaque
// instantiations in files, after go/types checked the package pkg (Phase
// 01 README §2.7). An opaque instantiation is an index expression whose
// base is rooted in C: C.std.vector[strings.Builder], which the loader
// made out of std.vector[strings.Builder]. go/types gives the base no type
// (FakeImportC), and in a type (var v std.vector[T], a composite literal)
// it then stops without looking at the arguments. So nothing reports a
// typo (std.vector[Pointt]), codegen gets no type for the arguments, and
// an import or a local variable that only an argument uses is "not used".
//
// For each argument that has no type yet, checkOpaqueArgs runs
// types.CheckExpr in the scope of the argument (CheckExpr finds the
// innermost scope at the argument's position, and sees only the names
// declared before it, as go/types does), and it records the types in info.
// An argument that has a type already was checked by go/types (in a value,
// such as C.std.make_unique[T](), go/types checks the arguments itself).
// An argument that is an opaque instantiation itself is not checked as a
// whole (go/types would stop at its base again); its own arguments are,
// so std.map[std.string, std.vector[myT]] checks myT.
//
// Limits of CheckExpr, which takes no types.Config (it checks with
// go/types' defaults):
//   - It stops at the first error of an argument: in
//     std.vector[struct{ a Aa; b Bb }] only Aa is reported. report gets
//     that error, with the position of the argument.
//   - It uses go/types' default sizes (gc on amd64), not gocppSizes: today
//     they are the same, but once Phase 03 gives gocppSizes the real C++
//     layout, unsafe.Sizeof(s) in std.array[byte, unsafe.Sizeof(s)] would
//     be Go's size (TestOpaqueArgSizes fails then).
//   - It has no Go version, so every language feature is allowed inside
//     an argument, whatever the file's //go:build go1.N line says.
//
// An argument with a type must also be something that C++ can take as a
// template argument (badTemplateArg): a type that is instantiated, or a
// constant.
//
// checkOpaqueArgs returns the imports and the variables that the arguments
// use, so that the caller can drop their "imported and not used" and
// "declared and not used" errors.
func checkOpaqueArgs(fset *token.FileSet, pkg *types.Package, files []*ast.File, info *types.Info,
	report func(pos token.Pos, err error)) *argUses {
	used := &argUses{imports: make(map[*types.PkgName]bool), vars: make(map[token.Pos]bool)}
	for _, f := range files {
		if !importsC(f) {
			continue // nothing in f is rooted in C
		}
		dots := dotImports(f, info)
		ast.Inspect(f, func(n ast.Node) bool {
			for _, arg := range opaqueArgs(n, info) {
				if isOpaqueInstance(arg, info) {
					continue // ast.Inspect gets to its arguments
				}
				if _, ok := info.Types[arg]; !ok {
					err := types.CheckExpr(fset, pkg, arg.Pos(), arg, info)
					u := &useMarker{info: info, dots: dots, used: used,
						scope: innermost(pkg, arg.Pos()), pos: arg.Pos()}
					u.mark(arg)
					if err != nil {
						report(arg.Pos(), err)
						continue
					}
				}
				if msg := badTemplateArg(pkg, arg, info); msg != "" {
					report(arg.Pos(), types.Error{Fset: fset, Pos: arg.Pos(), Msg: msg})
				}
			}
			return true
		})
	}
	return used
}

// badTemplateArg returns the error of the template argument arg of an
// opaque instantiation, which go/types typed, or "": a generic type
// without type arguments (std.vector[Box] with type Box[T any] ...), with
// Go's message for it, or a value that is not a constant
// (std.array[int, n] with a variable n): C++ takes only types and
// constants. An argument without a valid type (a C/C++ name such as
// cstdio.BUFSIZ, or an error) is the C++ compiler's to check, and so is a
// value with a C/C++ part (int(C.N), with a macro N, is a constant in C++).
// A function (main, pkg.F) and nil are C++ template arguments too (a
// function pointer, nullptr); a variable that holds a function is not.
func badTemplateArg(pkg *types.Package, arg ast.Expr, info *types.Info) string {
	tv := info.Types[arg]
	if tv.Type == nil || tv.Type == types.Typ[types.Invalid] {
		return ""
	}
	qual := types.RelativeTo(pkg)
	switch {
	case tv.IsType():
		n, ok := types.Unalias(tv.Type).(*types.Named)
		if ok && n.TypeParams().Len() > 0 && n.TypeArgs().Len() == 0 {
			return fmt.Sprintf("cannot use generic type %s without instantiation",
				types.TypeString(n, qual))
		}
	case tv.IsValue() && tv.Value == nil && !tv.IsNil() && !namesFunc(arg, info) &&
		!hasCPart(arg, info):
		what := "value"
		if tv.Addressable() {
			what = "variable"
		}
		return fmt.Sprintf("template argument %s (%s of type %s) must be constant",
			types.ExprString(arg), what, types.TypeString(tv.Type, qual))
	}
	return ""
}

// namesFunc reports whether e names a function (f, pkg.F, in parentheses
// or not), not a method value (x.M).
func namesFunc(e ast.Expr, info *types.Info) bool {
	var id *ast.Ident
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		pkg, ok := x.X.(*ast.Ident)
		if !ok {
			return false
		}
		if _, ok := info.Uses[pkg].(*types.PkgName); !ok {
			return false
		}
		id = x.Sel
	default:
		return false
	}
	_, ok := info.Uses[id].(*types.Func)
	return ok
}

// hasCPart reports whether a part of e is rooted in C.
func hasCPart(e ast.Expr, info *types.Info) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if x, ok := n.(ast.Expr); ok && !found && rootedInC(x, info) {
			found = true
		}
		return !found
	})
	return found
}

// argUses are the imports and the variables that the checked arguments
// use.
type argUses struct {
	imports map[*types.PkgName]bool

	// vars holds the places of the variables: the place of go/types'
	// "declared and not used" error. (The variables of a type switch,
	// one in each case, all have the place of the name in the switch.)
	vars map[token.Pos]bool
}

// innermost returns the innermost scope of pkg at pos, as CheckExpr finds
// it, or nil.
func innermost(pkg *types.Package, pos token.Pos) *types.Scope {
	for i := range pkg.Scope().NumChildren() {
		if s := pkg.Scope().Child(i).Innermost(pos); s != nil {
			return s
		}
	}
	return nil
}

// opaqueArgs returns the type arguments of n if n is an opaque
// instantiation, else nil.
func opaqueArgs(n ast.Node, info *types.Info) []ast.Expr {
	switch n := n.(type) {
	case *ast.IndexExpr:
		if rootedInC(n.X, info) {
			return []ast.Expr{n.Index}
		}
	case *ast.IndexListExpr:
		if rootedInC(n.X, info) {
			return n.Indices
		}
	}
	return nil
}

// isOpaqueInstance reports whether e is an opaque instantiation.
func isOpaqueInstance(e ast.Expr, info *types.Info) bool {
	return opaqueArgs(e, info) != nil
}

// rootedInC reports whether e is a chain of selectors and index
// expressions that starts with the package name of import "C": C.std.vector,
// C.std.vector[int].iterator. The name has to be C's package name (in
// info.Uses), so a local variable C is not C.
func rootedInC(e ast.Expr, info *types.Info) bool {
	for {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		case *ast.Ident:
			pn, ok := info.Uses[x].(*types.PkgName)
			return ok && pn.Imported().Path() == "C"
		default:
			return false
		}
	}
}

// importsC reports whether f has import "C".
func importsC(f *ast.File) bool {
	for _, spec := range f.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == "C" {
			return true
		}
	}
	return false
}

// dotImports returns the package names of the dot imports of f, by the
// imported package.
func dotImports(f *ast.File, info *types.Info) map[*types.Package]*types.PkgName {
	var dots map[*types.Package]*types.PkgName
	for _, spec := range f.Imports {
		if spec.Name == nil || spec.Name.Name != "." {
			continue
		}
		if pn, ok := info.Defs[spec.Name].(*types.PkgName); ok {
			if dots == nil {
				dots = make(map[*types.Package]*types.PkgName)
			}
			dots[pn.Imported()] = pn
		}
	}
	return dots
}

// A useMarker finds the imports and the variables that a checked argument
// uses.
type useMarker struct {
	info *types.Info
	dots map[*types.Package]*types.PkgName // the dot imports of the file
	used *argUses

	// scope and pos are the scope and the place of the argument, to look
	// up a package name that CheckExpr didn't get to (after an error).
	scope *types.Scope
	pos   token.Pos
}

// mark adds to u.used what e uses, the way go/types counts a use: a
// package name as the X of a selector (strings in strings.Builder), a name
// that a dot import brought into the file, and a variable.
func (u *useMarker) mark(e ast.Expr) {
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := n.X.(*ast.Ident); ok {
				if pn := u.pkgName(id); pn != nil {
					u.used.imports[pn] = true
					return false // n.Sel is a name in pn's package
				}
			}
			u.mark(n.X)
			return false // n.Sel is a field or a method, never dot-imported
		case *ast.Ident:
			u.markIdent(n)
		}
		return true
	})
}

// pkgName returns the package name that id, the X of a selector, denotes,
// or nil. After an error in the argument, CheckExpr has not recorded
// id (std.vector[func(Pointt) strings.Builder]), so then pkgName looks it
// up in the argument's scope.
func (u *useMarker) pkgName(id *ast.Ident) *types.PkgName {
	if obj, ok := u.info.Uses[id]; ok {
		pn, _ := obj.(*types.PkgName)
		return pn
	}
	if u.scope == nil {
		return nil
	}
	_, obj := u.scope.LookupParent(id.Name, u.pos)
	pn, _ := obj.(*types.PkgName)
	return pn
}

// markIdent marks what the name id uses: a variable, or the dot import
// that brought it into the file.
func (u *useMarker) markIdent(id *ast.Ident) {
	obj := u.info.Uses[id]
	if obj == nil {
		return
	}
	if v, ok := obj.(*types.Var); ok {
		u.used.vars[v.Pos()] = true
	}
	if obj.Pkg() != nil && obj.Parent() == obj.Pkg().Scope() {
		if pn := u.dots[obj.Pkg()]; pn != nil {
			u.used.imports[pn] = true
		}
	}
}
