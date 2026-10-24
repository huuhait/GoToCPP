package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/diag"
)

// genericCMsg starts the error of an instance of a generic type with a C/C++ type argument, and
// genericCTail ends it (the words of lower's).
const (
	genericCMsg  = "an instance of a generic type with a C/C++ type argument ("
	genericCTail = ") is not supported yet: the type arguments of a generic type are Go types"
)

// checkGenericCArgs reports each instance of a generic type that the files write with a C/C++ type
// argument: Box[std.string], iter.Seq[std.string], Box[G] with type G struct{ s std.string }, in
// a declaration of a package variable or a local one, a field, a parameter, a result, a composite
// literal, a conversion or a type declaration (also of a generic type declared in a function). The
// lowering cannot make such an instance (ADR-030 point 5: the struct, the equality, the descriptor and
// the method tables of an instance are made from Go types), so the build refuses it; this pass gives
// check, vet and the editor the same error, at the same place, before the build gets that far.
//
// Only the outermost instance is reported (Box[Box[std.string]] once), and only what the lowering
// treats as an instance: a name of a generic named type (or of an alias of one, type A[T any] =
// Box[T]); an alias of an unnamed type (type Vec[T any] = []T) is that unnamed type. A type argument
// counts when its type has a part that go/types left invalid (a C/C++ type), also inside a named type
// of the program (a struct with a C/C++ field, type H C.int), as the lowering's hasC does.
//
// An instance written inside a generic declaration (a generic function, a method of a generic type, a
// generic type) is not reported there: the lowering meets it only when it makes an instance of the
// declaration, and a declaration that is never instantiated is never lowered. The summaries report it
// at the uses of the declaration (genericcinst.go), and so does this pass for an instance of a generic
// type whose declaration has one (W[int] with type W[T any] struct{ b Box[std.string] }).
//
// Check runs it after the opaque pass (reportGenericCArgs), whose sanity rule an error turns off: the
// expressions of such a program are still each typed or opaque.
func checkGenericCArgs(files []*ast.File, info *types.Info, sums *genericSums, report func(pos token.Pos, msg string)) {
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			if genericDecl(n) {
				return false
			}
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			named := genericTypeOrigin(e, info)
			if named == nil || constraintOnly(named) {
				return true
			}
			for _, a := range typeArgExprs(e) {
				tv, ok := info.Types[a]
				if !ok || !tv.IsType() || tv.Type == nil || !typeHasC(tv.Type, nil) {
					continue
				}
				report(e.Pos(), genericCMsg+types.ExprString(e)+genericCTail)
				return false
			}
			if named != nil && sums != nil {
				if r := sums.summary(named.Origin().Obj()).always; r != (genericReason{}) {
					report(e.Pos(), genericTypeMsg(types.ExprString(e), r))
					return false
				}
			}
			return true
		})
	}
}

// genericTypeMsg is the error of the instance inst of a generic type whose declaration has an instance
// of a generic type with a C/C++ type argument (r).
func genericTypeMsg(inst string, r genericReason) string {
	return "an instance of the generic type " + r.typ + " with a C/C++ type argument is not supported yet (" + inst + " " + r.desc +
		"): the type arguments of a generic type are Go types"
}

// typeArgExprs returns the indices of the index expression e (F[A] or F[A, B]), or nil.
func typeArgExprs(e ast.Expr) []ast.Expr {
	switch x := e.(type) {
	case *ast.IndexExpr:
		return []ast.Expr{x.Index}
	case *ast.IndexListExpr:
		return x.Indices
	}
	return nil
}

// genericTypeExpr reports whether e is an instance of a generic type written out, Box[T] or
// pkg.Box[T]: an index expression whose base names a type whose type (after aliases) is a named type
// with type parameters (lower's isGenericTypeExpr).
func genericTypeExpr(e ast.Expr, info *types.Info) bool {
	return genericTypeOrigin(e, info) != nil
}

// genericTypeOrigin returns the generic named type of the instance e written out (genericTypeExpr), or
// nil. (go/types may have no type for the instance itself when a type argument is a C/C++ type.)
func genericTypeOrigin(e ast.Expr, info *types.Info) *types.Named {
	var base ast.Expr
	switch x := e.(type) {
	case *ast.IndexExpr:
		base = x.X
	case *ast.IndexListExpr:
		base = x.X
	default:
		return nil
	}
	var id *ast.Ident
	switch b := ast.Unparen(base).(type) {
	case *ast.Ident:
		id = b
	case *ast.SelectorExpr:
		id = b.Sel
	}
	if id == nil {
		return nil
	}
	tn, _ := info.Uses[id].(*types.TypeName)
	if tn == nil {
		return nil
	}
	n, _ := types.Unalias(tn.Type()).(*types.Named)
	if n == nil || n.TypeParams().Len() == 0 {
		return nil
	}
	return n.Origin()
}

// typeHasC reports whether the type t has a part that go/types left invalid (a C/C++ type), also in
// the underlying types of the named types in it (through pointers, slices, maps, channels, struct
// fields and signatures), as lower's hasC. seen guards recursive types (nil at the top).
func typeHasC(t types.Type, seen map[*types.Named]bool) bool {
	switch t := types.Unalias(t).(type) {
	case *types.Basic:
		return t.Kind() == types.Invalid
	case *types.Named:
		if seen[t] {
			return false
		}
		if seen == nil {
			seen = make(map[*types.Named]bool)
		}
		seen[t] = true
		for a := range t.TypeArgs().Types() {
			if typeHasC(a, seen) {
				return true
			}
		}
		return typeHasC(t.Underlying(), seen)
	case *types.Pointer:
		return typeHasC(t.Elem(), seen)
	case *types.Slice:
		return typeHasC(t.Elem(), seen)
	case *types.Array:
		return typeHasC(t.Elem(), seen)
	case *types.Chan:
		return typeHasC(t.Elem(), seen)
	case *types.Map:
		return typeHasC(t.Key(), seen) || typeHasC(t.Elem(), seen)
	case *types.Struct:
		for i := range t.NumFields() {
			if typeHasC(t.Field(i).Type(), seen) {
				return true
			}
		}
	case *types.Signature:
		return typeHasC(t.Params(), seen) || typeHasC(t.Results(), seen)
	case *types.Tuple:
		for i := range t.Len() {
			if typeHasC(t.At(i).Type(), seen) {
				return true
			}
		}
	}
	return false
}

// reportGenericCArgs adds the errors of checkGenericCArgs to the errors of the checked package p,
// unless go/types reported a type error in p (Package.typeErrors): an invalid type argument may then
// be the program's error (Box[Undefined]), and a limit of C/C++ types is not reported next to a type
// error of the program, as the elided literal's is not (mapvalue.go).
//
// It also reports the uses of generic functions with C/C++ type arguments whose instances would make
// such an instance (checkGenericInstances), with the generic functions known so far (sums).
func (p *Package) reportGenericCArgs(fset *token.FileSet, sums *genericSums) {
	if p.Info == nil || p.Errors == nil || p.typeErrors {
		return
	}
	report := func(pos token.Pos, msg string) {
		p.Errors.Add(diag.Diagnostic{Pos: fset.Position(pos), Severity: diag.Error, Msg: msg})
	}
	checkGenericCArgs(p.astFiles(), p.Info, sums, report)
	checkGenericInstances(p.astFiles(), p.Info, sums, report)
	checkDeducedMembers(p.astFiles(), p.Info, p.SourceText, report)
}

// checkDeducedMembers reports a member of the result of a generic call whose type arguments C++
// deduces (slices.Insert(c, 1, cv), with c a []std.string) when the generic result type is a slice or
// a map (S ~[]E, []E, map[K]V): go/types gives the call no type, the lowering gives it that gocpp
// container type (lower/deducedres.go), which has no members, so r.size() is the build's error at the
// same place with the same words. Also through a variable declared from the call (r := ..., var r = ...).
func checkDeducedMembers(files []*ast.File, info *types.Info, text func(ast.Expr) string, report func(pos token.Pos, msg string)) {
	kind := func(e ast.Expr) string {
		call, ok := ast.Unparen(e).(*ast.CallExpr)
		if !ok {
			return ""
		}
		id := instanceIdent(call.Fun)
		if id == nil {
			return ""
		}
		fn, ok := info.Uses[id].(*types.Func)
		if !ok || fn.Signature().TypeParams().Len() == 0 || fn.Signature().Results().Len() != 1 {
			return ""
		}
		if tv, ok := info.Types[call]; ok && tv.Type != nil && !hasInvalid(tv.Type) {
			return "" // typed by go/types
		}
		if len(typeArgExprs(ast.Unparen(call.Fun))) >= fn.Signature().TypeParams().Len() {
			return "" // every type argument written: no type argument that C++ deduces
		}
		t := fn.Signature().Results().At(0).Type()
		if tp, ok := types.Unalias(t).(*types.TypeParam); ok {
			t = CoreType(tp.Constraint())
		}
		switch types.Unalias(t).(type) {
		case *types.Slice:
			return "slice"
		case *types.Map:
			return "map"
		}
		return ""
	}
	for _, f := range files {
		vars := make(map[*types.Var]string)
		note := func(lhs []ast.Expr, rhs []ast.Expr) {
			if len(lhs) != len(rhs) {
				return
			}
			for i, l := range lhs {
				if id, ok := l.(*ast.Ident); ok {
					if v, ok := info.Defs[id].(*types.Var); ok && hasInvalid(v.Type()) {
						if k := kind(rhs[i]); k != "" {
							vars[v] = k
						}
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.AssignStmt:
				if n.Tok == token.DEFINE {
					note(n.Lhs, n.Rhs)
				}
			case *ast.ValueSpec:
				if n.Type == nil {
					lhs := make([]ast.Expr, len(n.Names))
					for i, id := range n.Names {
						lhs[i] = id
					}
					note(lhs, n.Values)
				}
			case *ast.SelectorExpr:
				x := ast.Unparen(n.X)
				k := kind(x)
				if id, ok := x.(*ast.Ident); ok {
					if v, ok := info.Uses[id].(*types.Var); ok && vars[v] != "" {
						k = vars[v]
					}
				}
				if k != "" {
					report(n.Sel.Pos(), fmt.Sprintf("%s undefined (%s is a gocpp %s of C/C++ values, which has no field or method %s)",
						text(n), text(n.X), k, n.Sel.Name))
				}
			}
			return true
		})
	}
}

// reportForeignCxxGenerics adds the build's limit for generic functions of other packages that use
// C/C++ names (genericcinst.go: checkForeignCxxGenerics), also in a package without C/C++ names of its
// own.
func (p *Package) reportForeignCxxGenerics(fset *token.FileSet, sums *genericSums) {
	if p.Info == nil || p.Errors == nil || p.typeErrors {
		return
	}
	checkForeignCxxGenerics(p.astFiles(), p.Types, p.Info, sums, func(pos token.Pos, msg string) {
		p.Errors.Add(diag.Diagnostic{Pos: fset.Position(pos), Severity: diag.Error, Msg: msg})
	})
}
