package vet

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
)

// illTyped decides whether analyzers that need complete types must stay
// away from a checked package, and says why (Phase 13 README §2.3). It
// returns "" for a package that go/types typed completely.
//
// go/types reports no error for code that uses C/C++ names: with FakeImportC
// it gives C.f no type, and every expression built from it has the invalid
// type. Analyzers that don't expect invalid types in a package without type
// errors can crash (go/ssa, which nilness builds on, does: spikes/2026-10-13/
// audit-checks/xtools-opaque), or guess wrong. So a package is ill-typed
// when
//   - a file imports "C" (the loader makes it of every header and
//     namespace use), or
//   - an expression or a declared name has a type that has an invalid part:
//     its own C/C++ names, and also the opaque types that reach it through
//     the API of a package that it imports (the C++ pointer in a field).
//
// Only the analyzers with RunDespiteErrors, which can deal with invalid
// types (x/tools declares them: copylocks, composites, structtag, ...),
// and gocpp's own analyzers, which check what they touch, run there.
func illTyped(info *types.Info, fset *token.FileSet) string {
	if info == nil {
		return "the package was not type-checked"
	}
	for id, obj := range info.Uses {
		if pn, ok := obj.(*types.PkgName); ok && pn.Imported().Path() == "C" {
			return fmt.Sprintf("it uses C/C++ names (line %d)", fset.Position(id.Pos()).Line)
		}
	}
	memo := make(map[types.Type]bool)
	// The first place in source order, so that the reason is stable.
	var at []token.Pos
	for e, tv := range info.Types {
		if tv.Type != nil && !tv.IsBuiltin() && hasInvalid(tv.Type, memo) {
			at = append(at, e.Pos())
		}
	}
	for id, obj := range info.Defs {
		switch obj.(type) {
		case *types.Var, *types.Const, *types.TypeName, *types.Func:
			// A label or a package name has the invalid type in go/types.
			if obj.Type() != nil && hasInvalid(obj.Type(), memo) {
				at = append(at, id.Pos())
			}
		}
	}
	if len(at) == 0 {
		return ""
	}
	sort.Slice(at, func(i, j int) bool { return at[i] < at[j] })
	return fmt.Sprintf("an expression has no complete type, because it comes from C/C++ (line %d)", fset.Position(at[0]).Line)
}

// hasInvalid reports whether t has an invalid part. A named type has the
// parts of its underlying type and its type arguments (type Handle C.int).
func hasInvalid(t types.Type, memo map[types.Type]bool) bool {
	if v, ok := memo[t]; ok {
		return v
	}
	memo[t] = false // a type that contains itself: the answer comes from the other parts
	bad := false
	switch t := t.(type) {
	case *types.Basic:
		bad = t.Kind() == types.Invalid
	case *types.Pointer:
		bad = hasInvalid(t.Elem(), memo)
	case *types.Slice:
		bad = hasInvalid(t.Elem(), memo)
	case *types.Array:
		bad = hasInvalid(t.Elem(), memo)
	case *types.Chan:
		bad = hasInvalid(t.Elem(), memo)
	case *types.Map:
		bad = hasInvalid(t.Key(), memo) || hasInvalid(t.Elem(), memo)
	case *types.Tuple:
		for v := range t.Variables() {
			if hasInvalid(v.Type(), memo) {
				bad = true
				break
			}
		}
	case *types.Signature:
		bad = hasInvalid(t.Params(), memo) || hasInvalid(t.Results(), memo)
	case *types.Struct:
		for f := range t.Fields() {
			if hasInvalid(f.Type(), memo) {
				bad = true
				break
			}
		}
	case *types.Interface:
		for m := range t.ExplicitMethods() {
			if hasInvalid(m.Type(), memo) {
				bad = true
				break
			}
		}
		for i := 0; !bad && i < t.NumEmbeddeds(); i++ {
			bad = hasInvalid(t.EmbeddedType(i), memo)
		}
	case *types.Union:
		for term := range t.Terms() {
			if hasInvalid(term.Type(), memo) {
				bad = true
				break
			}
		}
	case *types.Named:
		bad = hasInvalid(t.Underlying(), memo)
		for i := 0; !bad && i < t.TypeArgs().Len(); i++ {
			bad = hasInvalid(t.TypeArgs().At(i), memo)
		}
	case *types.Alias:
		bad = hasInvalid(types.Unalias(t), memo)
	case *types.TypeParam:
		bad = hasInvalid(t.Constraint(), memo)
	}
	memo[t] = bad
	return bad
}

// touchesInvalid reports whether the node n, or something inside it, has an
// invalid type: gocpp's own analyzers skip such code (the guard).
func touchesInvalid(info *types.Info, memo map[types.Type]bool, n ast.Node) bool {
	bad := false
	ast.Inspect(n, func(n ast.Node) bool {
		if bad {
			return false
		}
		e, ok := n.(ast.Expr)
		if !ok {
			return true
		}
		if tv, ok := info.Types[e]; ok && tv.Type != nil && !tv.IsBuiltin() && hasInvalid(tv.Type, memo) {
			bad = true
			return false
		}
		if call, ok := e.(*ast.CallExpr); ok && failedBuiltin(info, call) {
			bad = true
			return false
		}
		// A selector that go/types could not resolve (C.f, std.string): it
		// has no object, and the code that looks for one crashes.
		if sel, ok := e.(*ast.SelectorExpr); ok && info.Selections[sel] == nil && info.Uses[sel.Sel] == nil {
			bad = true
			return false
		}
		if id, ok := e.(*ast.Ident); ok {
			switch obj := info.ObjectOf(id).(type) {
			case *types.Var, *types.Const, *types.TypeName, *types.Func:
				if obj.Type() != nil && hasInvalid(obj.Type(), memo) {
					bad = true
				}
			}
		}
		return !bad
	})
	return bad
}

// failedBuiltin reports whether the call is one of a builtin that go/types
// could not check (its operand is a C/C++ value): it records no signature for
// the builtin, as for a call that it folds into a constant, and the call
// has no constant value then.
func failedBuiltin(info *types.Info, call *ast.CallExpr) bool {
	tf, ok := info.Types[call.Fun]
	if !ok || !tf.IsBuiltin() || tf.Type == nil || !hasInvalid(tf.Type, map[types.Type]bool{}) {
		return false
	}
	return info.Types[call].Value == nil
}
