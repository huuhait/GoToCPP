package lower

import (
	"fmt"
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
// What lowering translates today (cvalues.go, cnames.go, stmt.go):
//   - the expressions: a variable of a C/C++ type, a C/C++ name (a path, a member of a value
//     path through gp::deref, a template's instance), a gocpp field of a C/C++ type, a call of a
//     C/C++ function, of a function of the package, of a member of a C/C++ value or of a C++
//     callable, a member (gp::deref) and an index of a C/C++ value, the operators (binary, unary
//     but <-, *p) and &x of a C/C++ variable, in parentheses or not;
//   - their places: a statement (a call), the value of a variable, an assignment with = or op=,
//     ++ and --, a return value, an argument of a C/C++ function, of a function of the package,
//     of a C++ callable or of a conversion to a gocpp type (int(x)), an operand, an index, a
//     condition, the tag and the cases of a switch, a deferred call of a C/C++ function.
//
// A func literal that uses an opaque local of the function around it is not supported either
// until step 4.8: it captures it by copy, and a change through a C++ reference (std.swap) would
// be lost. So is a deferred call of a member of a C/C++ value (defer v.clear()): the C++ object
// is gone when the defers run, until step 4.8 boxes it.
//
// The rules of cgo where gocpp values meet C/C++ values (the mismatched types of cOperandsOK,
// the targets of targetOK and gocppTargetOK) are user errors here too: go/types has no types
// for the C/C++ side, so it can't report them.

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
	case *ast.GenDecl:
		if x.Tok == token.CONST && !w.constsOK(x) {
			return false
		}
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
	if !g.cExprOK(e, w.lits) || !g.cPlaceOK(e, parent) || !w.targetOK(e, parent) || !w.addrOK(e, parent) {
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

// constsOK reports whether the values of the const declaration d are constants as far as their
// C/C++ parts go, and reports Go's error if one isn't: a C/C++ variable or a call of a C/C++
// function in it (const d = x + 1 with var x C.int), which go/types doesn't report for a value
// it has no type for. (C macros in constants are a gate of the check, Phase 12.)
func (w *cWalk) constsOK(d *ast.GenDecl) bool {
	g := w.g
	for _, spec := range d.Specs {
		for _, v := range spec.(*ast.ValueSpec).Values {
			if !g.pkg.Opaque.IsOpaque(v) {
				continue
			}
			bad := false
			ast.Inspect(v, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.Ident:
					if obj, ok := g.info.Uses[x].(*types.Var); ok && !complete(obj.Type()) {
						bad = true
					}
				case *ast.CallExpr:
					if r, ok := g.cRefOf(x.Fun); !ok || r.kind != interop.Type {
						bad = true // (C.int(1) is a conversion)
					}
				}
				return !bad
			})
			if bad {
				g.userError(v.Pos(), "%s (C/C++ value) is not constant", g.srcText(v))
				return false
			}
		}
	}
	return true
}

// addrOK reports whether e, if it is &x of a C/C++ variable (a raw C++ address, Phase 04
// README §2.7), is where the variable outlives the pointer as far as gocpp can tell, and reports
// it if it isn't: in a defer or go statement, which runs after the C++ block of a local has
// ended (step 4.8 boxes such a local), and as a result of the function, of a local (C++'s
// lifetime: it would dangle at once; Phase 13's vet looks further).
func (w *cWalk) addrOK(e ast.Expr, parent ast.Node) bool {
	g := w.g
	u, ok := e.(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return true
	}
	id, ok := ast.Unparen(u.X).(*ast.Ident)
	if !ok {
		return true
	}
	v, ok := g.info.Uses[id].(*types.Var)
	if !ok || isPkgVar(v) {
		return true
	}
	for _, n := range w.stack {
		loopVar := false
		switch s := n.(type) {
		case *ast.ForStmt:
			loopVar = s.Init != nil && s.Init.Pos() <= v.Pos() && v.Pos() < s.Init.End()
		case *ast.RangeStmt:
			loopVar = s.Tok == token.DEFINE && s.Pos() <= v.Pos() && v.Pos() < s.X.Pos()
		}
		if loopVar {
			// Go 1.22 gives each iteration its own variable, which a pointer can keep;
			// gocpp's C/C++ loop variable is one, so the pointers would share it and dangle.
			g.unsupported(u.OpPos, "& of a C/C++ loop variable ("+id.Name+": each iteration has its own in Go; step 4.8)")
			return false
		}
		switch n.(type) {
		case *ast.DeferStmt, *ast.GoStmt:
			g.unsupported(u.OpPos, "& of a C/C++ local in a defer or go statement (the local must outlive the function: step 4.8)")
			return false
		}
	}
	if _, ok := parent.(*ast.ReturnStmt); ok {
		g.userError(u.OpPos, "&%s of a C/C++ local can't be a result: C/C++ values follow C++'s lifetime rules, and %s ends with the function", id.Name, id.Name)
		return false
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
		return true // a member of a C/C++ value: gp::deref(x).m
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
		switch ast.Unparen(x.Fun).(type) {
		case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.FuncType, *ast.InterfaceType, *ast.ChanType:
			g.unsupported(x.Pos(), "conversions to this C/C++ type (steps 4.9 and 4.10)")
			return false
		}
		return true // a member of a C/C++ value, or a C++ callable (opaqueCallee)
	case *ast.IndexExpr:
		if _, ok := g.cRefOf(x); ok {
			return true
		}
		return g.unknownKindOK(x.X, x.Lbrack) // an index of a C/C++ value: C++'s operator[]
	case *ast.IndexListExpr:
		if _, ok := g.cRefOf(x); ok {
			return true
		}
		g.userError(x.Lbrack, "invalid operation: %s (a C/C++ value takes one index)", g.srcText(x))
		return false
	case *ast.BinaryExpr:
		return g.cOperandsOK(x.Op, x.X, x.Y, g.srcText(x), x.Pos())
	case *ast.UnaryExpr:
		switch x.Op {
		case token.AND:
			if id, ok := ast.Unparen(x.X).(*ast.Ident); ok {
				if v, ok := g.info.Uses[id].(*types.Var); ok && !complete(v.Type()) {
					return true // &x of a C/C++ variable: a raw C++ address (Phase 04 README §2.7)
				}
			}
			if r, ok := g.cRefOf(ast.Unparen(x.X)); ok && r.kind == interop.Value {
				if _, ok := r.x.(*cppir.Name); ok {
					return true // &lib.global: a C/C++ global, which lives as long as the program
				}
			}
			g.unsupported(x.OpPos, "& of C/C++ values other than variables (step 4.10)")
			return false
		case token.ARROW:
			g.unsupported(x.OpPos, "the operator <- on C/C++ values (Phase 08)")
			return false
		}
		return true
	case *ast.StarExpr:
		return true // gp::cgo::star(x)
	case *ast.CompositeLit:
		g.unsupported(x.Lbrace, "composite literals of C/C++ types (step 4.6)")
	case *ast.SliceExpr:
		if gocppContainer(g.info.TypeOf(x.X)) {
			return true // ss[1:] of a gocpp slice or array of C/C++ values: Go's slicing
		}
		g.unsupported(x.Lbrack, "slicing C/C++ values (step 4.10)")
	case *ast.TypeAssertExpr:
		g.unsupported(x.Pos(), "type assertions on C/C++ values (Phase 05)")
	default:
		g.unsupported(e.Pos(), "this use of C/C++ values")
	}
	return false
}

// unknownKindOK reports whether x, the operand of an index x[i], is not a C/C++ name whose kind
// the kind probe can't tell (std.vector[int].npos: a nested type or a static member), and reports
// it with Phase 04 §6's hint if it is: a parenthesized C/C++ value is always a value, so (x)[i] is
// an index. (A member x.m of such a name is cMember's, cnames.go.)
func (g *gen) unknownKindOK(x ast.Expr, at token.Pos) bool {
	r, ok := g.cRefOf(x)
	if !ok || r.kind != interop.Unknown || r.x == placeholder {
		return true
	}
	g.userError(at, "the compiler probe can't tell whether %s is a C/C++ type or a value: if it is a value, write (%s)", r.src, r.src)
	return false
}

// cOperandsOK reports whether the operands x and y of the operator op (also of x op= y; the
// expression is expr, which starts at pos, where Go reports it), one of them a C/C++ value, may
// meet, and reports a user error like cgo's if they may not: a typed gocpp number, bool or string
// next to a C/C++ value would be converted by C++'s rules without a word, where cgo says
// "mismatched types C.int and int". An untyped constant is a C literal there and nil is nullptr
// (cvalues.go); the other gocpp values (pointers, structs, slices) meet C++'s operators as they
// are (a gocpp pointer that a C++ container holds compares with a gocpp pointer). The operands
// of && and || are bools anyway (cBool checks the C/C++ side). The count of a shift may be any
// gocpp integer, as in Go; a gocpp value shifted by a C/C++ count is reported, so that the shift
// keeps its gocpp type.
func (g *gen) cOperandsOK(op token.Token, x, y ast.Expr, expr string, pos token.Pos) bool {
	if op == token.LAND || op == token.LOR {
		return true
	}
	ox, oy := g.pkg.Opaque.IsOpaque(x), g.pkg.Opaque.IsOpaque(y)
	if ox == oy {
		return true
	}
	if op == token.SHL || op == token.SHR {
		if ox {
			return true
		}
		if t := g.typedBasic(x); t != nil {
			g.userError(pos, "invalid operation: %s (shift count %s is a C/C++ value): convert it, uint(%s)", expr, g.srcText(y), g.srcText(y))
			return false
		}
		return true
	}
	typed, cv := y, x
	if !ox {
		typed, cv = x, y
	}
	t := g.typedBasic(typed)
	if t == nil {
		return true
	}
	ts := types.TypeString(t, types.RelativeTo(g.pkg.Types))
	kind, cs := g.cKind(cv)
	cname := cs
	if cs == "" {
		cname = "C/C++ value"
	}
	left, right := cname, ts
	if !ox {
		left, right = ts, cname
	}
	var hint string
	switch b := underBasic(t); {
	case b.Info()&types.IsString != 0:
		hint = "convert the string: std.string(" + g.srcText(typed) + ")"
	case kind == "number":
		hint = "convert one of them: " + ts + "(" + g.srcText(cv) + ") or " + cs + "(" + g.srcText(typed) + ")"
	default:
		// gocpp doesn't know the C/C++ side's type, or it is no number (a pointer, an
		// iterator: it + C.long(n)): the gocpp side converts to a C type.
		n := g.srcText(typed)
		hint = "convert " + n + " to a C type, C.long(" + n + ") or C.size_t(" + n + ")"
		if kind != "pointer" {
			hint += ", or a C/C++ number to " + ts + ", " + ts + "(" + g.srcText(cv) + ")"
		}
	}
	g.userError(pos, "invalid operation: %s (mismatched types %s and %s): %s", expr, left, right, hint)
	return false
}

// typedBasic returns the type of e if e is a typed gocpp value of a basic type (a number, a bool
// or a string, also of a named type over one: also a typed constant), or nil.
func (g *gen) typedBasic(e ast.Expr) types.Type {
	tv, ok := g.info.Types[e]
	if !ok || !tv.IsValue() || tv.Type == nil || tv.IsNil() || !complete(tv.Type) {
		return nil
	}
	if b := underBasic(tv.Type); b != nil && b.Info()&types.IsUntyped == 0 {
		return tv.Type
	}
	return nil
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
		g.userError(e.Pos(), "%s (C/C++ value) is not used", g.srcText(e))
		return false
	case *ast.ValueSpec:
		return g.oneValueOK(e, len(p.Names), p.Values, p.Names[0].Pos())
	case *ast.ReturnStmt, *ast.SwitchStmt, *ast.CaseClause, *ast.IfStmt, *ast.ForStmt:
		return true // (an if's or a for's condition: gp::cgo::cond checks that it is a bool)
	case *ast.DeferStmt, *ast.GoStmt:
		if c, ok := ast.Unparen(e).(*ast.CallExpr); ok && g.opaqueCallee(c) {
			g.unsupported(c.Pos(), "deferred calls of members of C/C++ values (the C++ object must outlive the function: step 4.8)")
			return false
		}
		return true
	case *ast.AssignStmt:
		if inList(p.Lhs, e) && !g.assignableOK(e) {
			return false
		}
		if p.Tok == token.ASSIGN || p.Tok == token.DEFINE {
			return inList(p.Lhs, e) || g.oneValueOK(e, len(p.Lhs), p.Rhs, p.Lhs[0].Pos())
		}
		op := p.Tok + (token.ADD - token.ADD_ASSIGN)
		if p.Tok == token.AND_NOT_ASSIGN {
			op = token.AND_NOT
		}
		return g.cOperandsOK(op, p.Lhs[0], p.Rhs[0], g.srcText(p.Lhs[0])+" "+p.Tok.String()+" "+g.srcText(p.Rhs[0]), p.Pos())
	case *ast.IncDecStmt:
		return g.assignableOK(e)
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		// The parent is a C/C++ name of its own, a gocpp field, a member or an index of a
		// C/C++ value, which is checked as an opaque expression too (cExprOK); or a gocpp
		// index with a C/C++ value as its index (xs[n]), which Go allows for any integer.
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
		if conv := g.pkg.Conversion(p); conv != nil && conv.Kind == check.Compare {
			return true // x == y of C/C++ values: a bool (cvalues.go)
		}
		if conv := g.pkg.Conversion(p); conv != nil && conv.Kind == check.AsCall {
			to := g.info.TypeOf(p)
			return g.cConvOK(e, to, types.TypeString(to, types.RelativeTo(g.pkg.Types))) // int(x): cvalues.go
		}
		if g.calledSig(p) != nil {
			return true // targetOK checks the parameter's type
		}
		if g.opaqueCallee(p) {
			return true // an argument of a member of a C/C++ value or of a C++ callable
		}
		if id, ok := ast.Unparen(p.Fun).(*ast.Ident); ok {
			if b, ok := g.info.Uses[id].(*types.Builtin); ok {
				// e is the container itself, an element appended to it, a key to delete, or
				// a container of append(xs, ys...) and copy (not a C++ vector).
				arg := len(p.Args) > 0 && (containsExpr(p.Args[0], e) || gocppContainer(g.info.TypeOf(e)) ||
					b.Name() == "append" && !p.Ellipsis.IsValid() || b.Name() == "delete")
				if arg && gocppContainer(g.info.TypeOf(p.Args[0])) {
					switch b.Name() {
					case "len", "cap", "append", "copy", "clear", "delete":
						// A gocpp slice, array or map of C/C++ values (var xs []C.int):
						// lowered as for any other one.
						return true
					}
				}
				switch b.Name() {
				case "print", "println":
					return true // gp::cgo::print_arg (fmtcall.go)
				case "len", "cap":
					g.userError(e.Pos(), "invalid argument: %s (C/C++ value) for built-in %s: call its C++ function, %s.size() or std.size(%s)",
						g.srcText(e), b.Name(), g.srcText(e), g.srcText(e))
				case "min", "max":
					g.userError(e.Pos(), "invalid argument: %s (C/C++ value) for built-in %s: use std.%s, or convert it first", g.srcText(e), b.Name(), b.Name())
				default:
					g.unsupported(e.Pos(), b.Name()+" of C/C++ values")
				}
				return false
			}
		}
		if sel, ok := ast.Unparen(p.Fun).(*ast.SelectorExpr); ok {
			if fn, ok := g.info.Uses[sel.Sel].(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" {
				return true // gp::cgo::fmt_arg (fmtcall.go)
			}
		}
		if sel, ok := ast.Unparen(p.Fun).(*ast.SelectorExpr); ok {
			if isUnsafe(g, sel) {
				g.unsupported(e.Pos(), "unsafe."+sel.Sel.Name+" of C/C++ values (step 4.10)")
				return false
			}
		}
		g.unsupported(e.Pos(), "passing C/C++ values to this function")
		return false
	case *ast.SliceExpr:
		if gocppContainer(g.info.TypeOf(p.X)) {
			return true // ss[1:] of a gocpp slice or array of C/C++ values (or a C/C++ bound)
		}
	case *ast.KeyValueExpr, *ast.CompositeLit:
		g.unsupported(e.Pos(), "C/C++ values in composite literals (step 4.6)")
		return false
	case *ast.RangeStmt:
		if p.X == e && gocppContainer(g.info.TypeOf(e)) || p.X != e {
			// A gocpp slice, array or map of C/C++ values (Go's range: the elements are
			// copies), or a range variable of a C/C++ type.
			return true
		}
		g.unsupported(e.Pos(), "range over C/C++ values (gp::range, step 4.7)")
		return false
	case *ast.BinaryExpr, *ast.UnaryExpr, *ast.StarExpr:
		return true // the operator is opaque too, and checked (cExprOK)
	}
	g.unsupported(e.Pos(), "this use of C/C++ values")
	return false
}

// isUnsafe reports whether sel is a name of package unsafe (unsafe.Slice).
func isUnsafe(g *gen, sel *ast.SelectorExpr) bool {
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	pn, ok := g.info.Uses[id].(*types.PkgName)
	return ok && pn.Imported() == types.Unsafe
}

// oneValueOK reports whether the C/C++ value e, one of values that go to n targets, is not the
// only value for several targets unless it is a call (a C++ pair or tuple: gp::values), and
// reports Go's error, at pos (the first target), if it is: a C++ map has no comma-ok form (v, ok := m[k]), and a C/C++ value
// that is no call is one value.
func (g *gen) oneValueOK(e ast.Expr, n int, values []ast.Expr, pos token.Pos) bool {
	if n < 2 || len(values) != 1 {
		return true
	}
	if _, ok := ast.Unparen(e).(*ast.CallExpr); ok {
		return true
	}
	if g.mapCommaOK(e) {
		return true // v, ok := m[k] of a gocpp map of C/C++ values (maps.go)
	}
	hint := ""
	if ix, ok := ast.Unparen(e).(*ast.IndexExpr); ok {
		x := g.srcText(ix.X)
		hint = fmt.Sprintf(": a C++ map has no comma-ok form, use %s.find(k) != %s.end() or %s.count(k) == 1", x, x, x)
	}
	g.userError(pos, "assignment mismatch: %d variables but %s is 1 value%s", n, g.srcText(e), hint)
	return false
}

// gocppContainer reports whether t is a gocpp slice, array, map or pointer to an array, also of
// C/C++ elements ([]C.int).
func gocppContainer(t types.Type) bool {
	if t == nil {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Slice, *types.Array, *types.Map:
		return true
	case *types.Pointer:
		_, ok := u.Elem().Underlying().(*types.Array)
		return ok
	}
	return false
}

// mapCommaOK reports whether e is m[k] of a gocpp map (also one of C/C++ values), which gives
// two values to two targets (v, ok := m[k]).
func (g *gen) mapCommaOK(e ast.Expr) bool {
	ix, ok := ast.Unparen(e).(*ast.IndexExpr)
	return ok && isMap(g.info.TypeOf(ix.X))
}

// inList reports whether e is one of list or in one of them.
func inList(list []ast.Expr, e ast.Expr) bool {
	for _, x := range list {
		if containsExpr(x, e) {
			return true
		}
	}
	return false
}

// assignableOK reports whether the target e of an assignment, of op= or of ++ and -- is a place,
// if it is a C/C++ value, and reports Go's error if it isn't: a call's result is no place in Go,
// also where C++ returns a reference (v.at(0) = 5, v.back()++). (go/types doesn't check a
// target that it has no type for.) Only e itself is looked at: e is a direct target, or a part of
// one, which is checked as its own expression.
func (g *gen) assignableOK(e ast.Expr) bool {
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok && g.pkg.Opaque.IsOpaque(c) {
		g.userError(c.Pos(), "cannot assign to %s (neither addressable nor a map index expression)", g.srcText(c))
		return false
	}
	return true
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
	multi   bool     // one value for several targets (a, b = std.minmax(x, y))
	texpr   ast.Expr // the type expression of to, if v is nil (an element of a []C.int)
}

// targetOf returns where the value e goes, a direct operand of parent (the first node above
// it that is not a parenthesis), or the zero valueTarget if e is no value with a target there (x :=
// e, the function of a call, a C/C++ function's argument).
func (w *cWalk) targetOf(e ast.Expr, parent ast.Node) valueTarget {
	g := w.g
	var t valueTarget
	switch p := parent.(type) {
	case *ast.ValueSpec:
		if p.Type != nil && len(p.Names) > 1 && len(p.Values) == 1 && containsExpr(p.Values[0], e) && !g.mapCommaOK(p.Values[0]) {
			// var a, b int = std.minmax(x, y): every target has the type.
			if obj, ok := g.info.Defs[p.Names[0]].(*types.Var); ok {
				return valueTarget{to: obj.Type(), v: obj, context: "variable declaration", multi: true}
			}
		}
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
		if len(p.Lhs) > 1 && len(p.Rhs) == 1 && (p.Tok == token.ASSIGN || p.Tok == token.DEFINE) {
			if !containsExpr(p.Rhs[0], e) || g.mapCommaOK(p.Rhs[0]) {
				return valueTarget{} // (a target; v, ok := m[k] of a gocpp map: Go's types)
			}
			// a, b = std.minmax(x, y): the first target that is a gocpp variable (with :=, one
			// that exists already) says it.
			for _, l := range p.Lhs {
				if id, ok := l.(*ast.Ident); isBlank(l) || ok && g.info.Defs[id] != nil {
					continue
				}
				if to := g.info.TypeOf(l); to != nil && complete(to) {
					return valueTarget{to: to, v: g.varOf(l), context: "assignment", multi: true}
				}
			}
			return valueTarget{}
		}
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
		if id, ok := ast.Unparen(p.Fun).(*ast.Ident); ok && len(p.Args) > 1 && !p.Ellipsis.IsValid() && !containsExpr(p.Args[0], e) {
			if b, ok := g.info.Uses[id].(*types.Builtin); ok && b.Name() == "append" {
				// append(xs, n) of a []C.int xs: the elements go to C.int.
				if sl, ok := under(g.info.TypeOf(p.Args[0])).(*types.Slice); ok {
					t := valueTarget{to: sl.Elem(), context: "argument to append"}
					if v := g.varOf(p.Args[0]); v != nil {
						if at, ok := g.typeExprOf(v).(*ast.ArrayType); ok {
							t.texpr = at.Elt
						}
					}
					return t
				}
			}
		}
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
	case t.multi:
		hint = ": declare new variables for its values (a, b := x), and convert those"
	case b != nil && b.Info()&types.IsString != 0:
		hint = ": convert it, string(" + g.srcText(e) + ") (step 4.9)"
	case b != nil:
		hint = ": convert it, " + ts + "(" + g.srcText(e) + ")"
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
	te := t.texpr
	if t.v != nil {
		te = g.typeExprOf(t.v)
	}
	if te != nil {
		ts = g.srcText(te)
		if _, ok := ast.Unparen(te).(*ast.StarExpr); ok {
			hint = ": convert it, (" + ts + ")(" + g.srcText(e) + ")"
		} else {
			hint = ": convert it, " + ts + "(" + g.srcText(e) + ")"
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
