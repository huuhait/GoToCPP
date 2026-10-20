package lower

import (
	"go/ast"
	"go/token"
	"go/types"
)

// Element references (step 15.2, ADR-114): the copies out of a slice element.
//
// `v := xs[i]` and `for _, v := range xs` copy the element into a new variable. For an element
// that owns something (an interface value, a pointer, a string, a slice, a map) the copy costs a
// reference count, up and down. The variable can be a const reference to the element instead,
// when nothing can overwrite or free the element while the variable lives:
//
//   - The variable lives to the end of its block (for a range variable, of the iteration), and
//     the code in that region is pure in the sense of borrow.go: it stores no owner anywhere but
//     into its own local variables, and runs no unknown code. A call through an interface is
//     known by class hierarchy analysis (chapure.go): it is pure when every method it can reach
//     is. A pure region cannot drop the element's owner, so the slot keeps its value.
//   - Arrays that are local variables are written by private stores, which a pure region may do,
//     and a slice can alias one: a region that stores into an array of owners is not pure for
//     this purpose.
//   - The slice that holds the element is not reassigned in the region (for `v := xs[i]`: its
//     variable, and the variable at the root of its path), or its array would be freed under
//     the reference. A range loop holds a copy of the slice, which keeps the array alive.
//   - The variable itself is never assigned, its address is never taken, no closure captures it,
//     and it is not returned (a return copies it either way).
//   - In a program that can run a second thread, nothing stores into elements of that type
//     anywhere (hotLoad), since another thread could overwrite the slot at any time.
//
// Anything else keeps the copy. The variables that qualify are in escapeInfo.refs; the lowering
// declares them const T& (stmt.go newTargets, control.go bindVars).

// elemRefs finds the element variables of the function that can be references.
func (w *escWalker) elemRefs() {
	g := w.g
	returned := returnedVars(g.info, w.d.Body)
	ast.Inspect(w.d.Body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.BlockStmt:
			w.elemRefsIn(n.List, returned)
		case *ast.CaseClause:
			w.elemRefsIn(n.Body, returned)
		case *ast.CommClause:
			w.elemRefsIn(n.Body, returned)
		case *ast.RangeStmt:
			w.rangeRef(n, returned)
		}
		return true
	})
}

// elemRefsIn handles the declarations `v := xs[i]` among the statements list.
func (w *escWalker) elemRefsIn(list []ast.Stmt, returned map[*types.Var]bool) {
	g := w.g
	for k, s := range list {
		as, ok := s.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			continue
		}
		id, ok := as.Lhs[0].(*ast.Ident)
		if !ok || id.Name == "_" {
			continue
		}
		v, _ := g.info.Defs[id].(*types.Var)
		ix, ok := ast.Unparen(as.Rhs[0]).(*ast.IndexExpr)
		if v == nil || !ok {
			continue
		}
		if _, isSlice := under(g.info.TypeOf(ix.X)).(*types.Slice); !isSlice {
			continue
		}
		what := types.ExprString(ix)
		if why := w.refVarOK(v, returned); why != "" {
			w.refNote(id.Pos(), v, what, why)
			continue
		}
		if !w.callFree(ix) {
			w.refNote(id.Pos(), v, what, "the element is found by a call")
			continue
		}
		root := w.rootVar(ix.X)
		if root == nil {
			w.refNote(id.Pos(), v, what, "the slice is not a variable or a part of one")
			continue
		}
		if w.bi.captured[root] {
			w.refNote(id.Pos(), v, what, "the slice is captured by a closure")
			continue
		}
		region := list[k+1:]
		if w.assignsVar(region, root) {
			w.refNote(id.Pos(), v, what, "the region assigns "+root.Name())
			continue
		}
		if why := w.regionWritesElems(region, v.Type()); why != "" {
			w.refNote(id.Pos(), v, what, why)
			continue
		}
		if g.hotLoad(ix) {
			w.refNote(id.Pos(), v, what, "another thread may store the element")
			continue
		}
		w.out.refs[v] = true
		w.refNote(id.Pos(), v, what, "")
	}
}

// rangeRef handles the value variable of `for _, v := range xs`.
func (w *escWalker) rangeRef(s *ast.RangeStmt, returned map[*types.Var]bool) {
	g := w.g
	if s.Tok != token.DEFINE || s.Value == nil {
		return
	}
	id, ok := s.Value.(*ast.Ident)
	if !ok || id.Name == "_" {
		return
	}
	v, _ := g.info.Defs[id].(*types.Var)
	sl, isSlice := under(g.info.TypeOf(s.X)).(*types.Slice)
	if v == nil || !isSlice {
		return
	}
	what := "range " + types.ExprString(s.X)
	if why := w.refVarOK(v, returned); why != "" {
		w.refNote(id.Pos(), v, what, why)
		return
	}
	if why := w.regionWritesElems(s.Body.List, v.Type()); why != "" {
		w.refNote(id.Pos(), v, what, why)
		return
	}
	if g.hotElems(sl.Elem()) {
		w.refNote(id.Pos(), v, what, "another thread may store the elements")
		return
	}
	w.out.refs[v] = true
	w.refNote(id.Pos(), v, what, "")
}

// refVarOK says why the variable v cannot be a reference, or "".
func (w *escWalker) refVarOK(v *types.Var, returned map[*types.Var]bool) string {
	switch {
	case w.assign[v]:
		return "it is assigned or its address is taken"
	case w.bi.first[v] != 0 || w.bi.captured[v]:
		return "it is boxed or captured"
	case returned[v]:
		return "it is returned"
	case !w.g.refType(v.Type()):
		return "its type is copied"
	}
	return ""
}

// refNote records the decision for the variable v: copied with the reason, or borrowed ("").
func (w *escWalker) refNote(pos token.Pos, v *types.Var, what, why string) {
	if why == "" {
		w.out.notes = append(w.out.notes, escapeNote{pos: pos, msg: v.Name() + " borrows " + what})
		return
	}
	note := escapeNote{pos: pos, msg: v.Name() + " copies " + what, detail: why, only2: true}
	w.out.notes = append(w.out.notes, note)
}

// callFree reports whether e has no call, receive or function literal (a conversion and the
// builtins len and cap do not count): the C++ of e has no temporaries.
func (w *escWalker) callFree(e ast.Expr) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			ok = false
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				ok = false
			}
		case *ast.CallExpr:
			isLenCap := w.g.isBuiltinCall(x, "len") || w.g.isBuiltinCall(x, "cap")
			if w.g.info.Types[x.Fun].IsType() || isLenCap {
				return true
			}
			ok = false
		}
		return ok
	})
	return ok
}

// rootVar returns the variable at the root of the path e (x, x.f, x[i], *x, (x)), or nil.
func (w *escWalker) rootVar(e ast.Expr) *types.Var {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			v, _ := w.g.info.Uses[x].(*types.Var)
			return v
		case *ast.SelectorExpr:
			if v, ok := w.g.info.Uses[x.Sel].(*types.Var); ok && isPkgVar(v) {
				return v // pkg.V
			}
			e = x.X
		case *ast.IndexExpr:
			e = x.X
		case *ast.StarExpr:
			e = x.X
		default:
			return nil
		}
	}
}

// assignsVar reports whether the statements assign the variable v as a whole (or a part of it
// that a local path reaches), or take its address.
func (w *escWalker) assignsVar(list []ast.Stmt, v *types.Var) bool {
	found := false
	hit := func(e ast.Expr) {
		if w.rootVar(e) == v {
			found = true
		}
	}
	for _, s := range list {
		ast.Inspect(s, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					hit(l)
				}
			case *ast.IncDecStmt:
				hit(x.X)
			case *ast.RangeStmt:
				if x.Tok == token.ASSIGN {
					if x.Key != nil {
						hit(x.Key)
					}
					if x.Value != nil {
						hit(x.Value)
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					hit(x.X)
				}
			}
			return !found
		})
		if found {
			return true
		}
	}
	return false
}

// regionWritesElems says why the statements may overwrite or free an element of type elem that
// a reference points to, or "" when they cannot: they are pure (borrow.go, with class hierarchy
// analysis for the calls through an interface) and store into no array that holds owners.
func (w *escWalker) regionWritesElems(list []ast.Stmt, elem types.Type) string {
	g := w.g
	a := &borrowWalk{g: g, cha: true, pure: true, pureV: true, deps: make(map[*types.Func]bool)}
	why := ""
	for _, s := range list {
		ast.Inspect(s, func(n ast.Node) bool {
			if n == nil || why != "" {
				return false
			}
			a.purity(n)
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					if w.arrayOfOwners(l) {
						why = "the region stores into an array of owners"
					}
				}
			case *ast.IncDecStmt:
				if w.arrayOfOwners(x.X) {
					why = "the region stores into an array of owners"
				}
			case *ast.RangeStmt:
				if x.Tok == token.ASSIGN {
					for _, l := range []ast.Expr{x.Key, x.Value} {
						if l != nil && w.arrayOfOwners(l) {
							why = "the region stores into an array of owners"
						}
					}
				}
			}
			return why == ""
		})
		if why != "" {
			return why
		}
	}
	ok := a.pure
	if holdsString(elem) {
		ok = a.pureV
	}
	if !ok {
		return "the region may store an owner or run unknown code"
	}
	return ""
}

// arrayOfOwners reports whether storing to l writes memory of an array whose elements a reference
// can point to: l is such an array, or an element of one, or has one inside (a struct). That is an
// array of owners, and also an array of strings, which mayOwn does not count as owners (a string
// store is private to a local variable for ADR-086) but which a slice of the array can share with
// a reference to its element: `xs := arr[:]; v := xs[1]; arr[1] = "Q"`.
func (w *escWalker) arrayOfOwners(l ast.Expr) bool {
	g := w.g
	holds := func(t types.Type) bool {
		return t != nil && containsOwnerArray(t)
	}
	for {
		l = ast.Unparen(l)
		if holds(g.info.TypeOf(l)) {
			return true
		}
		switch x := l.(type) {
		case *ast.SelectorExpr:
			l = x.X
		case *ast.IndexExpr:
			if a, ok := under(g.info.TypeOf(x.X)).(*types.Array); ok && referable(a.Elem()) {
				return true
			}
			l = x.X
		default:
			return false
		}
	}
}

// referable reports whether a slot of type t can be the target of an element reference: it may own
// something, or it holds a string.
func referable(t types.Type) bool {
	return mayOwn(t) || holdsString(t)
}

// containsOwnerArray reports whether a value of type t is or holds an array whose elements are
// referable (referable).
func containsOwnerArray(t types.Type) bool {
	switch u := under(t).(type) {
	case *types.Array:
		return referable(u.Elem())
	case *types.Struct:
		for i := range u.NumFields() {
			if containsOwnerArray(u.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

// hotElems reports whether the elements of a slice, map or array of type elem are stored by the
// program in a program that can have another thread (hotLoad).
func (g *gen) hotElems(elem types.Type) bool {
	t := g.threads()
	return t.threaded && (t.library || t.mutElems[elemKey(elem)])
}
