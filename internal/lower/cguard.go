package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// The guard of the C/C++ values (Phase 04): before anything is lowered, checkCUses walks the
// package and reports every use of an opaque expression (check.Opaque) that lowering doesn't
// translate yet as "not supported yet", with the step that brings it. Lowering then never sees
// such a use: the rest of the lowerer reads the Go types of what it lowers, and an opaque
// expression has none, so without the guard it could write broken C++ or crash.
//
// What lowering translates today (cvalues.go, cnames.go):
//   - the expressions: a variable of a C/C++ type, a C/C++ name (a path, a member of a value
//     path through gp::deref, a template's instance), a gocpp field of a C/C++ type, a call of a
//     C/C++ function or of a function of the package, in parentheses or not;
//   - their places: a statement (a call), the value of a variable, an assignment with =, a
//     return value, an argument of a C/C++ function, of a function of the package or of a
//     conversion to a gocpp type (int(x)), the tag and the cases of a switch, a deferred call.
//
// A func literal that uses an opaque local of the function around it is not supported either
// until step 4.8: it captures it by copy, and a change through a C++ reference (std.swap) would
// be lost.

// checkCUses reports the first use of a C/C++ value that lowering doesn't translate yet.
func (g *gen) checkCUses() {
	if g.pkg.Opaque == nil {
		return
	}
	for _, f := range g.pkg.Load.Files {
		w := &cWalk{g: g}
		ast.Inspect(f.AST, w.visit)
	}
}

// A cWalk is checkCUses' walk of one file.
type cWalk struct {
	g     *gen
	stack []ast.Node         // the nodes above the current one
	lits  []*ast.FuncLit     // the func literals above it, innermost last
	sigs  []*types.Signature // the signatures of the functions above it, innermost last
	funcs []ast.Node         // their *ast.FuncDecl or *ast.FuncLit
}

// visit is the ast.Inspect function of the walk: it checks n and goes into it if it should.
func (w *cWalk) visit(n ast.Node) bool {
	if n == nil {
		top := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
		if lit, ok := top.(*ast.FuncLit); ok && len(w.lits) > 0 && w.lits[len(w.lits)-1] == lit {
			w.lits = w.lits[:len(w.lits)-1]
		}
		if len(w.funcs) > 0 && w.funcs[len(w.funcs)-1] == top {
			w.funcs, w.sigs = w.funcs[:len(w.funcs)-1], w.sigs[:len(w.sigs)-1]
		}
		return true
	}
	if w.g.err != nil {
		return false
	}
	switch x := n.(type) {
	case *ast.FieldList, *ast.TypeSpec, *ast.ImportSpec:
		return false // types: the type translator's (ctypes.go)
	case *ast.ValueSpec:
		// Its type is a type expression: only the values are checked.
		w.stack = append(w.stack, n)
		for _, v := range x.Values {
			ast.Inspect(v, w.visit)
		}
		w.stack = w.stack[:len(w.stack)-1]
		return false
	case *ast.FuncDecl:
		if fn, ok := w.g.info.Defs[x.Name].(*types.Func); ok {
			w.funcs, w.sigs = append(w.funcs, x), append(w.sigs, fn.Signature())
		}
	case *ast.FuncLit:
		if w.g.pkg.Opaque.IsOpaque(x) {
			w.g.unsupported(x.Pos(), "func literals with C/C++ parameters or results (step 4.11)")
			return false
		}
		w.lits = append(w.lits, x)
		if sig, ok := w.g.info.TypeOf(x).(*types.Signature); ok {
			w.funcs, w.sigs = append(w.funcs, x), append(w.sigs, sig)
		}
	case ast.Expr:
		if !w.check(x) {
			return false
		}
	}
	w.stack = append(w.stack, n)
	return true
}

// check checks the expression e and reports whether the walk goes into it.
func (w *cWalk) check(e ast.Expr) bool {
	g := w.g
	if tv, ok := g.info.Types[e]; ok && tv.IsType() {
		return false // a type expression (ctypes.go)
	}
	var parent ast.Node
	for i := len(w.stack) - 1; i >= 0; i-- {
		if _, ok := w.stack[i].(*ast.ParenExpr); !ok {
			parent = w.stack[i]
			break
		}
	}
	if id, ok := e.(*ast.Ident); ok && !w.capturedStructOK(id, parent) {
		return false
	}
	if !g.pkg.Opaque.IsOpaque(e) {
		if _, ok := e.(*ast.ParenExpr); !ok && !w.gocppTargetOK(e, parent) {
			return false
		}
		return true
	}
	if _, ok := e.(*ast.ParenExpr); ok {
		return true // its parent decides: the parentheses are transparent
	}
	if !g.cExprOK(e, w.lits) || !g.cPlaceOK(e, parent) || !w.targetOK(e, parent) {
		return false
	}
	// A C/C++ name's parts and template arguments are the name's (cnames.go, ctypes.go).
	switch e.(type) {
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		if _, ok := g.cRefOf(e); ok {
			return false
		}
	}
	return true
}

// cExprOK reports whether lowering translates the opaque expression e, and reports why not if
// it doesn't. lits are the func literals that e is in, innermost last.
func (g *gen) cExprOK(e ast.Expr, lits []*ast.FuncLit) bool {
	switch x := e.(type) {
	case *ast.Ident:
		if v, ok := g.info.Uses[x].(*types.Var); ok {
			return g.capturedOK(x, v, lits)
		}
		return true
	case *ast.SelectorExpr:
		if _, ok := g.cRefOf(x); ok {
			return true
		}
		if sel := g.info.Selections[x]; sel != nil && sel.Kind() != types.FieldVal {
			return true // a method value or expression of a gocpp type (closures.go, methods.go)
		}
		if sel := g.info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
			if root := rootIdent(x); root != nil {
				if v, ok := g.info.Uses[root].(*types.Var); ok {
					return g.capturedOK(root, v, lits)
				}
			}
			return true
		}
		g.unsupported(x.Sel.Pos(), "members of C/C++ values (gp::deref, step 4.4)")
		return false
	case *ast.CallExpr:
		if _, ok := g.cRefOf(x.Fun); ok {
			return true
		}
		if g.calledSig(x) != nil {
			return true // a function or a method of the package
		}
		if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok {
			if _, ok := g.info.Uses[id].(*types.Builtin); ok {
				return true // the place of its argument says why (cPlaceOK)
			}
		}
		switch f := ast.Unparen(x.Fun).(type) {
		case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.FuncType, *ast.InterfaceType, *ast.ChanType:
			g.unsupported(x.Pos(), "conversions to this C/C++ type (steps 4.4 and 4.10)")
		case *ast.SelectorExpr:
			g.unsupported(f.Sel.Pos(), "member calls on C/C++ values (gp::deref, step 4.4)")
		default:
			g.unsupported(x.Pos(), "this call with C/C++ values (step 4.4)")
		}
		return false
	case *ast.IndexExpr, *ast.IndexListExpr:
		if _, ok := g.cRefOf(x); ok {
			return true
		}
		g.unsupported(x.Pos(), "indexing C/C++ values (step 4.5)")
		return false
	case *ast.BinaryExpr:
		g.unsupported(x.OpPos, "the operator "+x.Op.String()+" on C/C++ values (step 4.4)")
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			g.unsupported(x.OpPos, "& of C/C++ values (step 4.10)")
		} else {
			g.unsupported(x.OpPos, "the operator "+x.Op.String()+" on C/C++ values (step 4.4)")
		}
	case *ast.StarExpr:
		g.unsupported(x.Star, "* of C/C++ pointers (step 4.4)")
	case *ast.CompositeLit:
		g.unsupported(x.Lbrace, "composite literals of C/C++ types (step 4.6)")
	case *ast.SliceExpr:
		g.unsupported(x.Lbrack, "slicing C/C++ values (step 4.4)")
	case *ast.TypeAssertExpr:
		g.unsupported(x.Pos(), "type assertions on C/C++ values (Phase 05)")
	default:
		g.unsupported(e.Pos(), "this use of C/C++ values (step 4.4)")
	}
	return false
}

// capturedOK reports whether the use id of the variable v is not inside a func literal that
// captures it (the innermost of lits declares it, or it is a package variable), and reports the
// use if it is.
func (g *gen) capturedOK(id *ast.Ident, v *types.Var, lits []*ast.FuncLit) bool {
	if len(lits) == 0 || v.Parent() == g.pkg.Types.Scope() || v.Parent() == nil {
		return true
	}
	lit := lits[len(lits)-1]
	if lit.Pos() <= v.Pos() && v.Pos() < lit.End() {
		return true
	}
	g.unsupported(id.Pos(), "C/C++ values in closures (a func literal that uses "+id.Name+": step 4.8)")
	return false
}

// rootIdent returns the variable that the field chain x is in: v in v.a.b, or nil.
func rootIdent(x *ast.SelectorExpr) *ast.Ident {
	var e ast.Expr = x
	for {
		switch y := ast.Unparen(e).(type) {
		case *ast.SelectorExpr:
			e = y.X
		case *ast.Ident:
			return y
		default:
			return nil
		}
	}
}

// calledSig returns the signature of the function or the method of the package that c calls
// (also a method of a gocpp type with C/C++ parameters: q.Put(n)), or of the func value in a
// variable that it calls (size := q.Size; size()), or nil.
func (g *gen) calledSig(c *ast.CallExpr) *types.Signature {
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		switch obj := g.info.Uses[f].(type) {
		case *types.Func:
			if obj.Pkg() == g.pkg.Types {
				return obj.Signature()
			}
		case *types.Var:
			sig, _ := obj.Type().Underlying().(*types.Signature) // a func value (closures.go)
			return sig
		}
	case *ast.SelectorExpr:
		if sel := g.info.Selections[f]; sel != nil && (sel.Kind() == types.MethodVal || sel.Kind() == types.MethodExpr) {
			if fn, ok := sel.Obj().(*types.Func); ok && fn.Pkg() == g.pkg.Types {
				sig, _ := g.info.TypeOf(f).(*types.Signature)
				return sig
			}
		}
	}
	return nil
}

// cPlaceOK reports whether lowering translates the opaque expression e in its place, the node
// parent (the first one above it that is not a parenthesis), and reports why not if it doesn't.
func (g *gen) cPlaceOK(e ast.Expr, parent ast.Node) bool {
	switch p := parent.(type) {
	case *ast.ExprStmt:
		if _, ok := ast.Unparen(e).(*ast.CallExpr); ok {
			return true
		}
	case *ast.ValueSpec, *ast.ReturnStmt, *ast.DeferStmt, *ast.GoStmt, *ast.SwitchStmt, *ast.CaseClause:
		return true
	case *ast.AssignStmt:
		if p.Tok == token.ASSIGN || p.Tok == token.DEFINE {
			if len(p.Lhs) > 1 && len(p.Rhs) == 1 {
				if c, ok := ast.Unparen(p.Rhs[0]).(*ast.CallExpr); ok {
					if _, ok := g.cRefOf(c.Fun); ok {
						g.unsupported(c.Pos(), "several values from one C/C++ value (gp::values, step 4.7)")
						return false
					}
				}
			}
			return true
		}
		g.unsupported(p.TokPos, "the operator "+p.Tok.String()+" on C/C++ values (step 4.4)")
		return false
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		// The parent is a C/C++ name of its own, or a gocpp field, which is checked as an
		// opaque expression too (a member or an index of a C/C++ value isn't one: cExprOK
		// reports it there).
		return true
	case *ast.CallExpr:
		if containsExpr(p.Fun, e) {
			return true // checked as the call (cExprOK)
		}
		if r, ok := g.cRefOf(p.Fun); ok {
			if n, ok := r.x.(*cppir.Name); ok && r.kind == interop.Type && isBuiltinC(n.Name) {
				return g.cConvOK(e, nil, g.srcText(p.Fun)) // C.int(x)
			}
			return true // an argument of a C/C++ function
		}
		if conv := g.pkg.Conversion(p); conv != nil && conv.Kind == check.AsCall {
			to := g.info.TypeOf(p)
			return g.cConvOK(e, to, types.TypeString(to, types.RelativeTo(g.pkg.Types))) // int(x): cvalues.go
		}
		if g.calledSig(p) != nil {
			return true // targetOK checks the parameter's type
		}
		if id, ok := ast.Unparen(p.Fun).(*ast.Ident); ok {
			if b, ok := g.info.Uses[id].(*types.Builtin); ok {
				if b.Name() == "print" || b.Name() == "println" {
					g.unsupported(e.Pos(), "printing C/C++ values (step 4.12): convert the value to a gocpp type first, int(x)")
				} else {
					g.unsupported(e.Pos(), b.Name()+" of C/C++ values (step 4.4)")
				}
				return false
			}
		}
		if sel, ok := ast.Unparen(p.Fun).(*ast.SelectorExpr); ok {
			if fn, ok := g.info.Uses[sel.Sel].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" {
				g.unsupported(e.Pos(), "printing C/C++ values (step 4.12): convert the value to a gocpp type first, int(x)")
				return false
			}
		}
		g.unsupported(e.Pos(), "passing C/C++ values to this function (step 4.4)")
		return false
	case *ast.KeyValueExpr, *ast.CompositeLit:
		g.unsupported(e.Pos(), "C/C++ values in composite literals (step 4.4)")
		return false
	case *ast.IfStmt, *ast.ForStmt:
		g.unsupported(e.Pos(), "C/C++ values as conditions (step 4.4)")
		return false
	case *ast.RangeStmt:
		g.unsupported(e.Pos(), "range over C/C++ values (gp::range, step 4.7)")
		return false
	case *ast.IncDecStmt:
		g.unsupported(p.TokPos, "the operator "+p.Tok.String()+" on C/C++ values (step 4.4)")
		return false
	case *ast.BinaryExpr, *ast.UnaryExpr, *ast.StarExpr:
		return true // the operator is opaque too, and reported (cExprOK)
	}
	g.unsupported(e.Pos(), "this use of C/C++ values (step 4.4)")
	return false
}

// containsExpr reports whether the expression x is e or holds it.
func containsExpr(x, e ast.Expr) bool {
	return x.Pos() <= e.Pos() && e.End() <= x.End()
}

// capturedStructOK reports whether the use id of a variable is not a func literal's use of an
// outer local whose type holds a C/C++ part by value (a struct with a C++ field): the literal
// captures a copy, so a change made through a C++ reference later (std.swap(q.v, w)) would be
// lost (step 4.8 boxes such locals). A method value or call on it (q.Get) copies the receiver
// in Go too, so it is fine. It reports the use if it isn't.
func (w *cWalk) capturedStructOK(id *ast.Ident, parent ast.Node) bool {
	g := w.g
	v, ok := g.info.Uses[id].(*types.Var)
	if !ok || len(w.lits) == 0 || g.pkg.Opaque.IsOpaque(id) || !complete(v.Type()) || !cmpHasC(v.Type()) {
		return true // (an opaque variable: capturedOK)
	}
	if sel, ok := parent.(*ast.SelectorExpr); ok && sel.X == id {
		if s := g.info.Selections[sel]; s != nil && s.Kind() == types.MethodVal {
			return true
		}
	}
	return g.capturedOK(id, v, w.lits)
}

// A valueTarget is where a value goes: the type of the variable, parameter or result that it gives
// a value to, that variable (nil if there is none, as for a map entry), and the words of Go's
// messages for the place ("variable declaration", "argument to f").
type valueTarget struct {
	to      types.Type
	v       *types.Var
	context string
}

// targetOf returns where the value e goes, a direct operand of parent (the first node above
// it that is not a parenthesis), or the zero valueTarget if e is no value with a target there (x :=
// e, the function of a call, a C/C++ function's argument).
func (w *cWalk) targetOf(e ast.Expr, parent ast.Node) valueTarget {
	g := w.g
	var t valueTarget
	switch p := parent.(type) {
	case *ast.ValueSpec:
		if p.Type == nil || len(p.Names) != len(p.Values) {
			return valueTarget{}
		}
		for i, v := range p.Values {
			if containsExpr(v, e) {
				if obj, ok := g.info.Defs[p.Names[i]].(*types.Var); ok {
					t.to, t.v = obj.Type(), obj
				}
			}
		}
		t.context = "variable declaration"
	case *ast.AssignStmt:
		if p.Tok != token.ASSIGN || len(p.Lhs) != len(p.Rhs) {
			return valueTarget{}
		}
		for i, v := range p.Rhs {
			if containsExpr(v, e) && !isBlank(p.Lhs[i]) {
				t.to, t.v = g.info.TypeOf(p.Lhs[i]), g.varOf(p.Lhs[i])
			}
		}
		t.context = "assignment"
	case *ast.ReturnStmt:
		if len(w.sigs) == 0 {
			return valueTarget{}
		}
		res := w.sigs[len(w.sigs)-1].Results()
		for i, v := range p.Results {
			if containsExpr(v, e) && len(p.Results) == res.Len() {
				t.to, t.v = res.At(i).Type(), res.At(i)
			}
		}
		t.context = "return statement"
	case *ast.CallExpr:
		sig := g.calledSig(p)
		if sig == nil || containsExpr(p.Fun, e) {
			return valueTarget{}
		}
		params := sig.Params()
		for i, a := range p.Args {
			if !containsExpr(a, e) {
				continue
			}
			switch {
			case sig.Variadic() && i >= params.Len()-1 && !p.Ellipsis.IsValid():
				if sl, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok {
					t.to = sl.Elem()
				}
			case i < params.Len():
				t.to, t.v = params.At(i).Type(), params.At(i)
			}
		}
		t.context = "argument to " + g.srcText(p.Fun)
	}
	return t
}

// varOf returns the variable that the target of an assignment x is (a variable or a field), or
// nil.
func (g *gen) varOf(x ast.Expr) *types.Var {
	switch y := ast.Unparen(x).(type) {
	case *ast.Ident:
		v, _ := g.info.Uses[y].(*types.Var)
		return v
	case *ast.SelectorExpr:
		if s := g.info.Selections[y]; s != nil && s.Kind() == types.FieldVal {
			v, _ := s.Obj().(*types.Var)
			return v
		}
	}
	return nil
}

// targetOK reports whether the C/C++ value e goes, in its place parent, to a C/C++ target or to
// none (x := lib.f()), and reports a user error, as Go's for a value of another type, if it goes
// to a gocpp type: C++ would convert it implicitly, with C++'s rules (undefined behavior for a
// float out of an int's range), as cgo doesn't allow either.
func (w *cWalk) targetOK(e ast.Expr, parent ast.Node) bool {
	g := w.g
	t := w.targetOf(e, parent)
	if t.to == nil || !complete(t.to) {
		return true // a C/C++ target, or none
	}
	ts := types.TypeString(t.to, types.RelativeTo(g.pkg.Types))
	hint := ""
	switch b := underBasic(t.to); {
	case b != nil && b.Info()&types.IsString != 0:
		hint = ": convert it, string(x) (step 4.9)"
	case b != nil:
		hint = ": convert it, " + ts + "(x)"
	}
	g.userError(e.Pos(), "cannot use %s (C/C++ value) as %s value in %s%s", g.srcText(e), ts, t.context, hint)
	return false
}

// gocppTargetOK reports whether the gocpp value e (not opaque) goes, in its place parent, to a
// gocpp target or to none, and reports a user error, as cgo's, if it goes to a C/C++ variable,
// parameter or result (var i C.int = f, q.Put(f) with Put(n C.int)): C++ would convert it
// implicitly, with C++'s rules (a float out of an int's range is undefined behavior). An untyped
// constant is a C literal there (cvalues.go), and nil is nullptr. (An argument of a C/C++
// function has a parameter type that gocpp doesn't know: C++'s rules apply, Phase 04 §2.5.)
func (w *cWalk) gocppTargetOK(e ast.Expr, parent ast.Node) bool {
	g := w.g
	tv, ok := g.info.Types[e]
	if !ok || !tv.IsValue() || tv.Type == nil || !complete(tv.Type) || tv.IsNil() {
		return true
	}
	if b, ok := tv.Type.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
		return true // an untyped constant: a C literal
	}
	t := w.targetOf(e, parent)
	if t.to == nil || complete(t.to) {
		return true // a gocpp target, or none
	}
	what := "value"
	switch {
	case tv.Value != nil:
		what = "constant " + tv.Value.String()
	case g.varOf(e) != nil:
		what = "variable"
	}
	ts, hint := "a C/C++ type", ""
	if t.v != nil {
		if te := g.typeExprOf(t.v); te != nil {
			ts = g.srcText(te)
			if _, ok := ast.Unparen(te).(*ast.StarExpr); ok {
				hint = ": convert it, (" + ts + ")(x)"
			} else {
				hint = ": convert it, " + ts + "(x)"
			}
		}
	}
	g.userError(e.Pos(), "cannot use %s (%s of type %s) as %s value in %s%s", g.srcText(e), what,
		types.TypeString(tv.Type, types.RelativeTo(g.pkg.Types)), ts, t.context, hint)
	return false
}

// typeExprOf returns the type expression that the variable v (also a parameter, a result or a
// field) of a C/C++ type was declared with, or nil.
func (g *gen) typeExprOf(v *types.Var) ast.Expr {
	if te := g.pkg.Opaque.TypeExpr(v); te != nil {
		return te
	}
	return g.sigTypes()[v]
}

// cConvOK reports whether the conversion to the gocpp or C type to (int(p), C.int(p)) of the
// C/C++ value e is one that Go allows, as far as the declared type of e tells, and reports a
// user error, as Go's, if it isn't: a C pointer converts only through unsafe.Pointer, and only a
// bool converts to bool. (A value whose type gocpp doesn't know is checked by the C++ compiler:
// gp::cgo::as.)
func (g *gen) cConvOK(e ast.Expr, to types.Type, toName string) bool {
	kind, typ := g.cKind(e)
	if kind == "" {
		return true
	}
	b := underBasic(to)
	switch {
	case kind == "pointer" && b != nil && b.Info()&types.IsBoolean != 0:
		g.userError(e.Pos(), "cannot convert %s (C pointer) to type bool: compare it: %s != nil", g.srcText(e), g.srcText(e))
		return false
	case kind == "pointer" && (b != nil || to == nil):
		g.userError(e.Pos(), "cannot convert %s (C pointer) to type %s: use unsafe.Pointer (step 4.10)", g.srcText(e), toName)
		return false
	case kind != "pointer" && b != nil && b.Info()&types.IsBoolean != 0:
		g.userError(e.Pos(), "cannot convert %s (C/C++ value of type %s) to type bool", g.srcText(e), typ)
		return false
	}
	return true
}

// cKind returns what the C/C++ value e is, when its declared type tells (a variable, a
// parameter or a field of a C/C++ type): "pointer" (*cstdio.FILE), "number" (C.int, C.double),
// or "" if gocpp doesn't know; and the type as the user wrote it.
func (g *gen) cKind(e ast.Expr) (string, string) {
	var obj types.Object
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		obj = g.info.Uses[x]
	case *ast.SelectorExpr:
		if s := g.info.Selections[x]; s != nil && s.Kind() == types.FieldVal {
			obj = s.Obj()
		}
	}
	v, ok := obj.(*types.Var)
	if !ok {
		return "", ""
	}
	te := g.typeExprOf(v)
	if te == nil {
		return "", ""
	}
	switch t := ast.Unparen(te).(type) {
	case *ast.StarExpr:
		return "pointer", g.srcText(te)
	case *ast.SelectorExpr:
		if g.isC(t.X) && cBuiltins[t.Sel.Name] != "" {
			return "number", g.srcText(te)
		}
	}
	return "", ""
}

// srcText returns e as the user wrote it: std.vector, cstdio.printf, not the C.std.vector and
// C.printf of the loader's rewrite (the uses of header packages and namespaces get their names
// back while it prints).
func (g *gen) srcText(e ast.Expr) string {
	g.cUse(nil) // the uses, by their selectors
	var restore []func()
	for sel, u := range g.cuses {
		if e.Pos() <= sel.Pos() && sel.End() <= e.End() {
			x := sel.X
			sel.X = u.Ident
			restore = append(restore, func() { sel.X = x })
		}
	}
	s := types.ExprString(e)
	for _, r := range restore {
		r()
	}
	return s
}
