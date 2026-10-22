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
//     but <-, *p) and &x of a C/C++ variable, a composite literal of a C/C++ type or a gocpp one
//     with C/C++ parts (clits.go: cLitOK), in parentheses or not;
//   - their places: a statement (a call), the value of a variable, an assignment with = or op=,
//     ++ and --, a return value, an argument of a C/C++ function, of a function of the package,
//     of a C++ callable or of a conversion to a gocpp type (int(x)), an operand, an index, a
//     condition, the tag and the cases of a switch, a deferred call of a C/C++ function, an
//     element of a composite literal, the range expression and the variables of a range
//     (crange.go; a range that assigns needs C/C++ targets: rangeTargetOK).
//
// A func literal that uses an opaque local of the function around it, and a deferred call of a
// member of one (defer v.clear()), work through a box (cbox.go): the closure shares the object, and
// it outlives the C++ block of the function that declared it. What stays unsupported there is
// listed in deferRecvOK (a receiver that is an element, a pointer's target or another place that
// a deferred call can't keep) and in addrOK (& of a loop variable).
//
// The rules of cgo where gocpp values meet C/C++ values (the mismatched types of cOperandsOK,
// the targets of targetOK and gocppTargetOK, also the elements and fields of gocpp literals) are
// user errors here too: go/types has no types for the C/C++ side, so it can't report them.

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
	sigs  []*types.Signature // the signatures of the functions above it, innermost last
	funcs []ast.Node         // their *ast.FuncDecl or *ast.FuncLit
}

// visit is the ast.Inspect function of the walk: it checks n and goes into it if it should.
func (w *cWalk) visit(n ast.Node) bool {
	if n == nil {
		top := w.stack[len(w.stack)-1]
		w.stack = w.stack[:len(w.stack)-1]
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
	case *ast.KeyValueExpr:
		if len(w.stack) > 0 {
			if lit, ok := w.stack[len(w.stack)-1].(*ast.CompositeLit); ok && w.g.cxxLit(lit) && cLitForm(lit) == litFields {
				// A field name of a literal of a C/C++ type (clits.go): only the value is checked.
				w.stack = append(w.stack, n)
				ast.Inspect(x.Value, w.visit)
				w.stack = w.stack[:len(w.stack)-1]
				return false
			}
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
	case *ast.CallExpr:
		if w.g.cgoHelper(x) != "" {
			// A cgo helper (cstrings.go): its function is no C/C++ value, its arguments are.
			if !w.check(x) {
				return false
			}
			w.stack = append(w.stack, n)
			for _, a := range x.Args {
				ast.Inspect(a, w.visit)
			}
			w.stack = w.stack[:len(w.stack)-1]
			return false
		}
		if !w.check(x) {
			return false
		}
	case *ast.FuncLit:
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
	if !g.pkg.Opaque.IsOpaque(e) {
		if _, ok := e.(*ast.ParenExpr); !ok && (!w.gocppTargetOK(e, parent) || !g.rangeTargetOK(e, parent)) {
			return false
		}
		return true
	}
	if _, ok := e.(*ast.ParenExpr); ok {
		return true // its parent decides: the parentheses are transparent
	}
	if !g.cExprOK(e) || !g.cPlaceOK(e, parent) || !w.targetOK(e, parent) || !w.addrOK(e, parent) {
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
// it if it isn't: as a result of the function, of a local (C++'s lifetime: it would dangle at once;
// Phase 13's vet looks further), of the variable of a 3-clause for loop (Go gives each iteration
// its own variable, which a pointer can keep; a C++ address can't, and the pointers of all the
// iterations would be one), and in a go statement. In a defer statement it is fine: the
// frame holds the box of the variable, which the deferred call's arguments use (cbox.go).
func (w *cWalk) addrOK(e ast.Expr, parent ast.Node) bool {
	g := w.g
	u, ok := e.(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return true
	}
	if _, bare := ast.Unparen(u.X).(*ast.Ident); !bare && !cAddr(g.info, u) {
		return true // the address of a part of a gocpp value: an owning pointer (cunsafe.go: cOwnedAddr)
	}
	id := rootIdentOf(u.X) // x in &x, &x[i], &x.f: a raw address into the C/C++ local
	if id == nil {
		return true
	}
	v, ok := g.info.Uses[id].(*types.Var)
	if !ok || isPkgVar(v) {
		return true
	}
	for _, n := range w.stack {
		if s, ok := n.(*ast.ForStmt); ok && s.Init != nil && s.Init.Pos() <= v.Pos() && v.Pos() < s.Init.End() {
			g.userError(u.OpPos, "& of the variable of a 3-clause for loop (%s): Go gives each iteration its own variable, and a C/C++ address can't keep it alive: copy it into a variable of the body, j := %s, and take &j", id.Name, id.Name)
			return false
		}
	}
	if _, ok := parent.(*ast.ReturnStmt); ok {
		g.userError(u.OpPos, "&%s of a C/C++ local can't be a result: C/C++ values follow C++'s lifetime rules, and %s ends with the function", g.srcText(u.X), id.Name)
		return false
	}
	return true
}

// cExprOK reports whether lowering translates the opaque expression e, and reports why not if
// it doesn't.
func (g *gen) cExprOK(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident:
		return true // (a local that a func literal uses is boxed: cbox.go)
	case *ast.SelectorExpr:
		return true // a C/C++ name, a method or a field of a gocpp type, or a member of a C/C++ value: gp::deref(x).m
	case *ast.CallExpr:
		if _, ok := g.cRefOf(x.Fun); ok {
			return true
		}
		if g.cgoHelper(x) != "" {
			return true // C.CString(s), C.cstr(s): a C pointer, a holder (cstrings.go)
		}
		if name := g.unsafeFunc(x); name != "" {
			return g.unsafeCallOK(x, name) // unsafe.Slice(p, n) and the others (cunsafe.go)
		}
		if g.pkg.UnsafeWrapper(x) != nil {
			return true // gp_cgoh.UnsafeSlice(unsafe.Slice(p, n)): the call inside is checked in its place
		}
		if s := g.pkg.OpaqueSlice(x); s != nil {
			return g.cSliceOK(s) // a[lo:hi] of a C array (cunsafe.go)
		}
		if g.cPointerType(x) != nil {
			return true // (*C.char)(p): the operand is checked in its place (cPlaceOK)
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
			g.unsupported(x.Pos(), "conversions to this C/C++ type (a later step of Phase 04)")
			return false
		}
		return true // a member of a C/C++ value, or a C++ callable (opaqueCallee)
	case *ast.IndexExpr:
		if _, ok := g.cRefOf(x); ok || g.genericRef(x.X) {
			return true
		}
		return g.unknownKindOK(x.X, x.Lbrack) // an index of a C/C++ value: C++'s operator[]
	case *ast.IndexListExpr:
		if _, ok := g.cRefOf(x); ok || g.genericRef(x.X) {
			return true // (F[A, B] of a generic function: generics.go)
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
			if _, ok := ast.Unparen(x.X).(*ast.CompositeLit); ok {
				// &[]C.int{1}: a gocpp literal, in a new block (gp::ptr), as in Go; &T{...} of a
				// C/C++ type: a gocpp pointer that owns the new object (cunsafe.go: cNew).
				return true
			}
			switch ast.Unparen(x.X).(type) {
			case *ast.SelectorExpr, *ast.IndexExpr, *ast.StarExpr:
				return true // &v[i], &s.f, &*p: a raw C++ address, as &x (cunsafe.go)
			}
			if v := g.tvalOf(ast.Unparen(x.X)); v != nil && v.Lvalue {
				return true // typed mode: &v.at(0), the address of a T& result (typed.go)
			}
			g.userError(x.OpPos, "invalid operation: cannot take address of %s (C/C++ value): it must be a variable, a field, an element or *p", g.srcText(x.X))
			return false
		case token.ARROW:
			return true // <-ch of a channel of C/C++ values: gp::recv (chans.go)
		}
		return true
	case *ast.StarExpr:
		return true // gp::cgo::star(x)
	case *ast.CompositeLit:
		return g.cLitOK(x) // clits.go
	case *ast.SliceExpr:
		if gocppContainer(g.info.TypeOf(x.X)) {
			return true // ss[1:] of a gocpp slice or array of C/C++ values: Go's slicing
		}
		return g.cSliceOK(x) // a C array: gp::cgo::slice_of (cunsafe.go)
	case *ast.TypeAssertExpr:
		if x.Type != nil && isInterface(g.info.TypeOf(x.X)) {
			return true // x.(std.string): the interface's value is a C/C++ one (iconv.go)
		}
		g.userError(x.Pos(), "invalid operation: %s (C/C++ value) is not an interface: a type assertion or type switch needs a Go interface value", g.srcText(x.X))
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
	if g.cIdentical(g.cBuiltinName(cv), t) {
		return true // C.int is int32 (cidentity.go)
	}
	if g.typedOperandsOK(op, typed, cv) {
		return true // typed mode: C++'s usual arithmetic conversions (typed.go)
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

// calledSig returns the signature of the function or the method of the package that c calls
// (also a method of a gocpp type with C/C++ parameters: q.Put(n)), or of the func value in a
// variable that it calls (size := q.Size; size()), or nil.
func (g *gen) calledSig(c *ast.CallExpr) *types.Signature {
	fun := ast.Unparen(c.Fun)
	explicit := false // F[int](...): the function is generic (generics.go)
	for {
		switch x := fun.(type) {
		case *ast.IndexExpr:
			fun, explicit = ast.Unparen(x.X), true
			continue
		case *ast.IndexListExpr:
			fun, explicit = ast.Unparen(x.X), true
			continue
		}
		break
	}
	switch f := fun.(type) {
	case *ast.Ident:
		switch obj := g.info.Uses[f].(type) {
		case *types.Func:
			if obj.Pkg() == g.pkg.Types || g.isGeneric(obj) {
				return obj.Signature()
			}
		case *types.Var:
			if explicit {
				return nil
			}
			sig, _ := obj.Type().Underlying().(*types.Signature) // a func value (closures.go)
			return sig
		}
	case *ast.SelectorExpr:
		if fn := g.qualifiedFunc(f); fn != nil && g.isGeneric(fn) {
			return fn.Signature() // pkg.F(...), F a generic function of another package of the program
		}
		if explicit {
			return nil
		}
		if sel := g.info.Selections[f]; sel != nil && (sel.Kind() == types.MethodVal || sel.Kind() == types.MethodExpr) {
			if fn, ok := sel.Obj().(*types.Func); ok && fn.Pkg() == g.pkg.Types {
				sig, _ := g.info.TypeOf(f).(*types.Signature)
				return sig
			}
		}
	}
	return nil
}

// isGeneric reports whether fn is a generic function of a package of the program (generics.go).
func (g *gen) isGeneric(fn *types.Func) bool {
	if fn.Signature().TypeParams().Len() == 0 {
		return false
	}
	o := g.other(fn.Pkg())
	return o != nil && o.generics[fn.Origin()] != nil
}

// cPlaceOK reports whether lowering translates the opaque expression e in its place, the node
// parent (the first one above it that is not a parenthesis), and reports why not if it doesn't.
func (g *gen) cPlaceOK(e ast.Expr, parent ast.Node) bool {
	if g.isHolder(e) && !g.cstrPlaceOK(e, parent) {
		return false
	}
	switch p := parent.(type) {
	case *ast.TypeAssertExpr:
		if e == ast.Unparen(p.X) && p.Type == nil {
			// switch v.(type): the same user error as for x.(T) (the C/C++ value is no interface).
			g.userError(e.Pos(), "invalid operation: %s (C/C++ value) is not an interface: a type assertion or type switch needs a Go interface value", g.srcText(e))
			return false
		}
	case *ast.ExprStmt:
		if _, ok := ast.Unparen(e).(*ast.CallExpr); ok {
			return true
		}
		if isRecv(e) != nil {
			return true // <-ch as a statement
		}
		g.userError(e.Pos(), "%s (C/C++ value) is not used", g.srcText(e))
		return false
	case *ast.ValueSpec:
		return g.oneValueOK(e, len(p.Names), p.Values, p.Names[0].Pos())
	case *ast.ReturnStmt, *ast.SwitchStmt, *ast.CaseClause, *ast.IfStmt, *ast.ForStmt:
		return true // (an if's or a for's condition: gp::cgo::cond checks that it is a bool)
	case *ast.GoStmt:
		if c, ok := ast.Unparen(e).(*ast.CallExpr); ok {
			return g.deferRecvOK(c, "go") // go v.clear(): the goroutine keeps the object like a deferred call (cbox.go)
		}
		return true
	case *ast.DeferStmt:
		if c, ok := ast.Unparen(e).(*ast.CallExpr); ok {
			return g.deferRecvOK(c, "defer") // defer v.clear(): cbox.go
		}
		return true
	case *ast.SendStmt:
		return true // ch <- v: a C/C++ value is copied or moved into the channel (chans.go)
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
		if g.cgoHelper(p) != "" {
			return true // an argument of a cgo helper, C.GoString(p) (cstrings.go)
		}
		if g.unsafeFunc(p) != "" || g.pkg.OpaqueSlice(p) != nil || g.pkg.UnsafeWrapper(p) != nil {
			return true // unsafe.Slice(p, n) and the others, a[lo:hi]: cunsafe.go checks the call
		}
		if g.cPointerType(p) != nil {
			return g.cPtrConvOK(e, p) // (*C.char)(p) (cunsafe.go)
		}
		if tv := g.info.Types[p.Fun]; tv.IsType() && isUnsafePointer(tv.Type) {
			return true // unsafe.Pointer(&cvar): a conversion of a C pointer (cunsafe.go)
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
		if tv := g.info.Types[p.Fun]; tv.IsType() && isInterface(tv.Type) {
			return true // any(&cvar): the interface holds the C/C++ value (iconv.go)
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
					case "len", "cap", "append", "copy", "clear", "delete", "close":
						// A gocpp slice, array, map or channel of C/C++ values (var xs []C.int):
						// lowered as for any other one.
						return true
					}
				}
				switch b.Name() {
				case "print", "println":
					return true // gp::cgo::print_arg (fmtcall.go)
				case "make":
					if len(p.Args) > 1 && !containsExpr(p.Args[0], e) {
						return true // make([]byte, n) with n a C number (C.ulong), as in cgo: intExpr converts it
					}
					g.unsupported(e.Pos(), "make of C/C++ values")
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
		g.unsupported(e.Pos(), "passing C/C++ values to this function")
		return false
	case *ast.SliceExpr:
		// ss[1:] of a gocpp slice or array of C/C++ values (or a C/C++ bound), or a[lo:hi] of
		// a C array (gp::cgo::slice_of; cSliceOK checks the slice expression itself).
		return true
	case *ast.KeyValueExpr, *ast.CompositeLit:
		return true // an element of a literal: targetOK checks where it goes (clits.go: litTarget)
	case *ast.RangeStmt:
		if p.Tok == token.ASSIGN && (e == ast.Unparen(p.Key) || e == ast.Unparen(p.Value)) {
			return g.assignableOK(e)
		}
		// A gocpp slice, array or map of C/C++ values (Go's range: the elements are copies), a
		// C++ container or a C array (gp::range: crange.go), or a range variable of a C/C++ type.
		return true
	case *ast.BinaryExpr, *ast.UnaryExpr, *ast.StarExpr:
		return true // the operator is opaque too, and checked (cExprOK)
	}
	g.unsupported(e.Pos(), "this use of C/C++ values")
	return false
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
	if isRecv(e) != nil {
		return true // v, ok := <-ch of a channel of C/C++ values (chans.go)
	}
	if g.mapCommaOK(e) {
		return true // v, ok := m[k] of a gocpp map of C/C++ values (maps.go)
	}
	if ta, ok := ast.Unparen(e).(*ast.TypeAssertExpr); ok && ta.Type != nil && isInterface(g.info.TypeOf(ta.X)) {
		return true // v, ok := x.(std.string): the comma-ok form (iconv.go)
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
	case *types.Slice, *types.Array, *types.Map, *types.Chan:
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
		if v := g.tvalOf(c); v != nil && v.Assignable {
			return true // typed mode: a T& result is an lvalue, v.at(0) = 5 (C++'s rule, typed.go)
		}
		g.userError(c.Pos(), "cannot assign to %s (neither addressable nor a map index expression)", g.srcText(c))
		return false
	}
	return true
}

// containsExpr reports whether the expression x is e or holds it.
func containsExpr(x, e ast.Expr) bool {
	return x.Pos() <= e.Pos() && e.End() <= x.End()
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
				t.texpr = g.valueTypeExpr(p.Lhs[i])
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
					if at, ok := ast.Unparen(g.valueTypeExpr(p.Args[0])).(*ast.ArrayType); ok {
						t.texpr = at.Elt
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
	case *ast.CompositeLit, *ast.KeyValueExpr:
		lit, ok := p.(*ast.CompositeLit)
		for i := len(w.stack) - 1; !ok && i > 0; i-- {
			if w.stack[i] == p {
				lit, ok = w.stack[i-1].(*ast.CompositeLit)
				break
			}
		}
		if ok {
			t, _ = g.litTarget(lit, e) // clits.go
		}
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
	if g.isCommaOKFlag(e) {
		return true // the ok of v, ok := x.(std.string) is a Go bool (iconv.go)
	}
	if p, ok := parent.(*ast.AssignStmt); ok && len(p.Lhs) == 2 && len(p.Rhs) == 1 && isRecv(p.Rhs[0]) != nil &&
		(ast.Unparen(p.Lhs[1]) == ast.Unparen(e) || ast.Unparen(p.Rhs[0]) == ast.Unparen(e)) {
		return true // v, ok = <-ch of C/C++ values: the receive gives the value and a Go bool (chans.go)
	}
	t := w.targetOf(e, parent)
	if t.to == nil || !complete(t.to) {
		return true // a C/C++ target, or none
	}
	if isInterface(t.to) {
		return true // an interface holds the C/C++ value as it is (iconv.go: foreign values)
	}
	if hasTypeParam(t.to) {
		return true // a type parameter: the C++ compiler deduces the type argument (generics.go)
	}
	if !t.multi && g.cIdentical(g.cBuiltinName(e), t.to) {
		return true // C.int is int32 (cidentity.go)
	}
	if !t.multi && g.typedScalarOK(e, t.to) {
		return true // typed mode: C++'s conversion into a Go number (typed.go)
	}
	ts := types.TypeString(t.to, types.RelativeTo(g.pkg.Types))
	hint := ""
	switch b := underBasic(t.to); {
	case t.multi:
		hint = ": declare new variables for its values (a, b := x), and convert those"
	case b != nil && b.Info()&types.IsString != 0:
		hint = ": convert it, string(" + g.srcText(e) + ")"
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
// constant is a C literal there (cvalues.go), if Go allows its value there (cConstOK), and nil is
// nullptr. (An argument of a C/C++ function has a parameter type that gocpp doesn't know: C++'s
// rules apply, Phase 04 §2.5.)
func (w *cWalk) gocppTargetOK(e ast.Expr, parent ast.Node) bool {
	g := w.g
	tv, ok := g.info.Types[e]
	if !ok || !tv.IsValue() || tv.Type == nil || !complete(tv.Type) || tv.IsNil() {
		return true
	}
	if tv.Value != nil {
		if handled, valid := g.cConstConversion(e, tv, parent); handled {
			return valid // explicit C conversions check typed constants too
		}
	}
	if b, ok := tv.Type.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
		if tv.Value != nil {
			return w.cConstOK(e, tv, parent) // a C literal, if Go allows it there (cconsts.go)
		}
		return true
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
	if g.cIdentical(g.cBuiltinOfType(te), tv.Type) {
		return true // C.int is int32 (cidentity.go)
	}
	if g.typedNumberToC(e, te) {
		return true // typed mode: C++'s conversion into a C number (typed.go)
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

// rangeTargetOK reports whether e, if it is a target of a range over a C/C++ value that assigns
// (for i, v = range c), is not a gocpp-typed one, and reports a user error if it is: the index,
// the key and the element are C/C++ values (crange.go), which don't go into gocpp variables
// implicitly (gocppTargetOK). A new variable (:=) can be converted in the body.
func (g *gen) rangeTargetOK(e ast.Expr, parent ast.Node) bool {
	r, ok := parent.(*ast.RangeStmt)
	if !ok || r.Tok != token.ASSIGN || e != ast.Unparen(r.Key) && e != ast.Unparen(r.Value) || isBlank(e) || !g.cRange(r) {
		return true
	}
	t := g.info.TypeOf(e)
	if t == nil || !complete(t) {
		return true
	}
	what := "the element"
	if e == ast.Unparen(r.Key) {
		what = "the index or key"
	}
	g.userError(e.Pos(), "cannot assign %s of range over %s (a C/C++ value) to %s of type %s: declare a new variable with :=, and convert it, %s(x)",
		what, g.srcText(r.X), g.srcText(e), types.TypeString(t, types.RelativeTo(g.pkg.Types)), types.TypeString(t, types.RelativeTo(g.pkg.Types)))
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
	case kind == "pointer" && b != nil && b.Info()&types.IsString != 0:
		g.userError(e.Pos(), "cannot convert %s (C pointer) to type %s: use C.GoString(%s)", g.srcText(e), toName, g.srcText(e))
		return false
	case kind == "pointer" && b != nil && b.Info()&types.IsBoolean != 0:
		g.userError(e.Pos(), "cannot convert %s (C pointer) to type bool: compare it: %s != nil", g.srcText(e), g.srcText(e))
		return false
	case kind == "pointer" && (b != nil || to == nil):
		g.userError(e.Pos(), "cannot convert %s (C pointer) to type %s: convert it through unsafe.Pointer, as in uintptr(unsafe.Pointer(%s))", g.srcText(e), toName, g.srcText(e))
		return false
	case kind == "pointer" && to != nil && isPointer(to):
		g.userError(e.Pos(), "cannot convert %s (C pointer) to type %s: convert it through unsafe.Pointer, (%s)(unsafe.Pointer(%s))", g.srcText(e), toName, toName, g.srcText(e))
		return false
	case kind != "pointer" && b != nil && b.Info()&types.IsBoolean != 0 &&
		g.cBuiltinName(e) != "bool": // (C.bool is bool: the conversion is the identity)
		g.userError(e.Pos(), "cannot convert %s (C/C++ value of type %s) to type bool", g.srcText(e), typ)
		return false
	}
	return true
}

// cKind returns what the C/C++ value e is, when its declared type tells (a variable, a
// parameter or a field of a C/C++ type): "pointer" (*cstdio.FILE), "number" (C.int, C.double),
// or "" if gocpp doesn't know; and the type as the user wrote it.
func (g *gen) cKind(e ast.Expr) (string, string) {
	te := g.valueTypeExpr(e)
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

// srcText returns e as the user wrote it: std.vector, cstdio.printf, C.cstr, not the
// C.std.vector and C.printf of the loader's rewrite or check's gp_cgoh.Cstr (the uses of header
// packages and namespaces and the cgo helpers get their names back while it prints).
func (g *gen) srcText(e ast.Expr) string {
	return g.pkg.SourceText(e)
}

// isCommaOKFlag reports whether e is the variable ok of v, ok := x.(T) (or var v, ok = x.(T)) with a
// C/C++ type T: go/types has no tuple for such an assertion, so the check calls the variable a
// C/C++ value, but it is a bool.
func (g *gen) isCommaOKFlag(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	if g.commaOKFlags == nil {
		g.commaOKFlags = make(map[types.Object]bool)
		for _, f := range g.pkg.Load.Files {
			ast.Inspect(f.AST, func(n ast.Node) bool {
				var names, values []ast.Expr
				switch x := n.(type) {
				case *ast.AssignStmt:
					if x.Tok == token.DEFINE {
						names, values = x.Lhs, x.Rhs
					}
				case *ast.ValueSpec:
					for _, id := range x.Names {
						names = append(names, id)
					}
					values = x.Values
				}
				if len(names) == 2 && len(values) == 1 {
					if ta, ok := ast.Unparen(values[0]).(*ast.TypeAssertExpr); ok && ta.Type != nil {
						if second, ok := names[1].(*ast.Ident); ok {
							if obj := g.info.Defs[second]; obj != nil {
								g.commaOKFlags[obj] = true
							}
						}
					}
				}
				return true
			})
		}
	}
	return g.commaOKFlags[g.info.Uses[id]]
}
