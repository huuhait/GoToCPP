package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Conversions to interface types, type assertions and type switches (Phase 05 steps 5.4 and 5.5;
// the runtime is gocpp/iface.hpp and gocpp/ifaceconv.hpp):
//
//	Go                              C++
//	var s Shape = Rect{2, 3}        Shape s = gp::to_iface<Shape>(Rect{2, 3}, &gp_td_Rect);
//	var s Shape = &Circle{1}        gp::to_iface<Shape>(gp::new_<Circle>(...), gp_tdp_Circle)
//	var a any = 1, any(1.5)         gp::to_iface<gp::any>(int64_t(1), &gp::td_int), (1.5, &gp::td_float64)
//	var e error = &MyErr{}          gp::to_iface<gp::error>(..., gp_tdp_MyErr)
//	var s Shape = nil, return nil   Shape()  (nil of an interface type: the nullptr constructor is explicit)
//	var r Reader = rw, any(s)       gp::iface_cast<Reader>(rw), gp::iface_cast<gp::any>(s)
//	x == nil                        x == nullptr
//	s.Area()                        s.Area()  (the wrapper of the interface struct; no copy of the
//	                                interface is held: a thunk reads the box once, at its entry)
//	x.(Rect)                        gp::assert_type<Rect>(x, &gp_td_Rect)  (a const Rect&)
//	x.(*Circle)                     gp::assert_type<gp::ptr<Circle>>(x, gp_tdp_Circle)
//	v, ok := x.(T)                  auto gp_t0 = gp::assert_type2<T>(x, desc); T v = std::get<0>(gp_t0); ...
//	x.(Shape), v, ok := x.(Shape)   gp::assert_iface<Shape>(x), gp::assert_iface2<Shape>(x)
//
//	switch v := x.(type) {          { const Shape& gp_x0 = x;
//	case int, string:                   if (gp::type_is(gp_x0, &gp::td_int) || gp::type_is(gp_x0, &gp::td_string)) {
//	    ...                                 Shape v = gp_x0; ...
//	case Rect:                          } else if (gp::type_is(gp_x0, &gp_td_Rect)) {
//	    ...                                 Rect v = gp::unbox<Rect>(gp_x0); ...
//	case Namer:                         } else if (gp::type_has<Namer>(gp_x0)) {
//	    ...                                 Namer v = gp::iface_cast<Namer>(gp_x0); ...
//	case nil: ...                       } else if (gp_x0 == nullptr) { ...
//	default: ... }                      } else { ... } }
//
// Go converts a value to an interface where it is assigned (assignment, declaration, return,
// argument, composite literal element, map key and value, append, conversion): noteConversions
// finds those places (named.go), and the value is converted where it is written (convertTo). A
// constant has the exact Go type of its default (any(1) is an int: int64_t(1), &gp::td_int), a
// typed nil pointer in an interface is not nil (the descriptor is *T's), and a value of a
// struct, array, slice, map or func type is copied into the interface's box, as Go does.
//
// A failed assertion panics with Go's text (gocpp/ifaceconv.hpp). The tests of a type switch
// run in the order of the cases (default last, wherever it is), each case's types in order,
// until one matches (on a const reference to the operand); v has the type of the case when it lists one type (and a copy of the
// operand's interface otherwise), and is declared only in the clauses that use it. A break in
// a clause turns the chain into a C++ switch on the number of the clause, as switchStmt does.

// toInterface returns x, the C++ of a value of type from, as a value of the interface type to:
// gp::to_iface for a concrete value, gp::iface_cast for another interface, to() for nil.
func (g *gen) toInterface(from, to types.Type, x cppir.Expr, pos token.Pos) cppir.Expr {
	typ := g.cxxType(to, pos)
	switch {
	case from == nil || isNilType(from):
		return construct(typ)
	case isInterface(from):
		if cppir.TypeString(typ) == cppir.TypeString(g.cxxType(from, pos)) {
			return x
		}
		return templateCall("gp::iface_cast", []cppir.Node{typ}, x)
	}
	if !complete(from) {
		// A C/C++ value: its descriptor is made from its C++ type (gp::foreign_td). It has no
		// method table (the checker doesn't know its methods), so only an interface without
		// methods can hold it.
		if isErrorType(to) || !isAnyType(to) && g.ifaceMethods(to) > 0 {
			g.unsupported(pos, "a C/C++ value in an interface with methods ("+g.canonName(to)+"): only interfaces without methods hold C/C++ values until Phase 12 (typed interop)")
			return placeholder
		}
		return templateCall("gp::to_iface_foreign", []cppir.Node{typ}, x)
	}
	return templateCall("gp::to_iface", []cppir.Node{typ}, x, g.desc(from, pos))
}

// noConvArgs reports whether the call c takes its arguments as they are, without Go's
// conversion to the parameters' types: the calls of package fmt (the runtime's fmt takes
// every value, step 5.11 brings the real fmt) and the builtins print, println and panic.
func (g *gen) noConvArgs(c *ast.CallExpr) bool {
	fun := ast.Unparen(c.Fun)
	switch f := fun.(type) {
	case *ast.IndexExpr:
		fun = ast.Unparen(f.X) // gp_cgoh.As[T](x)
	case *ast.IndexListExpr:
		fun = ast.Unparen(f.X)
	}
	switch fun := fun.(type) {
	case *ast.Ident:
		if b, ok := g.info.Uses[fun].(*types.Builtin); ok {
			switch b.Name() {
			case "print", "println", "panic":
				return true
			}
		}
	case *ast.SelectorExpr:
		if g.info.Selections[fun] == nil {
			fn, ok := g.info.Uses[fun.Sel].(*types.Func)
			// (the helpers of gocpp/cgoh are the check's rewrite of C conversions: their arguments are C/C++ values)
			return ok && fn.Pkg() != nil && (fn.Pkg().Path() == "fmt" || fn.Pkg().Path() == "gocpp/cgoh")
		}
	}
	return false
}

// interfaceConversion returns the C++ of the conversion c, T(x), to the interface type to.
func (fg *funcGen) interfaceConversion(c *ast.CallExpr, to, from types.Type) cppir.Expr {
	arg := c.Args[0]
	if tv := fg.g.info.Types[arg]; tv.IsNil() {
		return construct(fg.g.cxxType(to, c.Pos()))
	}
	if types.Identical(from, to) {
		return fg.expr(arg)
	}
	var x cppir.Expr
	if tv := fg.g.info.Types[arg]; tv.Value != nil && !isInterface(from) {
		if b, ok := from.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
			from = types.Default(from)
		}
		x = fg.g.constant(tv.Value, from, arg.Pos(), true)
	} else {
		x = fg.exactExpr(arg)
	}
	return fg.g.toInterface(from, to, x, c.Pos())
}

// typeAssert returns the C++ of the type assertion e, x.(T), and of its comma-ok form (a
// std::tuple<T, bool>).
func (fg *funcGen) typeAssert(e *ast.TypeAssertExpr) cppir.Expr {
	g := fg.g
	if e.Type == nil {
		g.errorf(e.Pos(), "internal error: x.(type) outside a type switch")
		return placeholder
	}
	t := g.info.TypeOf(e.Type)
	x := fg.expr(e.X)
	commaOK := isTuple(g.info.TypeOf(e)) || fg.commaAsserts[e]
	if !complete(t) {
		// A C/C++ type: the value's descriptor is the foreign one of its C++ type.
		ct := g.typeExpr(e.Type)
		name := "gp::assert_type"
		if commaOK {
			name = "gp::assert_type2"
		}
		return templateCall(name, []cppir.Node{ct}, x, templateCall("gp::foreign_td", []cppir.Node{ct}))
	}
	if isInterface(t) {
		name := "gp::assert_iface"
		if commaOK {
			name = "gp::assert_iface2"
		}
		return templateCall(name, []cppir.Node{g.cxxType(t, e.Pos())}, x)
	}
	name := "gp::assert_type"
	if commaOK {
		name = "gp::assert_type2"
	}
	return templateCall(name, []cppir.Node{g.cxxType(t, e.Pos())}, x, g.desc(t, e.Pos()))
}

// typeSwitchGuard returns the assertion x.(type) of the type switch s, and the name it declares
// (nil for none).
func typeSwitchGuard(s *ast.TypeSwitchStmt) (*ast.TypeAssertExpr, *ast.Ident) {
	switch a := s.Assign.(type) {
	case *ast.ExprStmt:
		return a.X.(*ast.TypeAssertExpr), nil
	case *ast.AssignStmt:
		id, _ := a.Lhs[0].(*ast.Ident)
		return a.Rhs[0].(*ast.TypeAssertExpr), id
	}
	return nil, nil
}

// usedIn reports whether the variable v is used in the statements list.
func (g *gen) usedIn(list []ast.Stmt, v *types.Var) bool {
	used := false
	for _, s := range list {
		ast.Inspect(s, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && g.info.Uses[id] == v {
				used = true
			}
			return !used
		})
	}
	return used
}

// typeSwitchStmt lowers a type switch (see the top).
func (fg *funcGen) typeSwitchStmt(s *ast.TypeSwitchStmt) {
	g := fg.g
	end := s.Body.Rbrace
	wrapped := false
	wrap := func() {
		if !wrapped {
			fg.open(s.Switch)
			wrapped = true
		}
	}
	shadows := 0
	if s.Init != nil {
		wrap()
		mark := fg.shadows
		fg.stmt(s.Init)
		shadows = fg.initShadows(mark)
	}
	guard, _ := typeSwitchGuard(s)
	xe := guard.X
	xt := g.info.TypeOf(xe)
	xtyp := g.cxxType(xt, xe.Pos())
	pre := fg.order(false, xe)
	wrap()
	// The operand is read once, as a const reference (a temporary's lifetime is extended): no code
	// of the program runs between the tests and the declaration of v, which copies the value, so
	// nothing can change or free what the reference is to.
	tmp := fg.names.temp("x")
	tp := g.pos(xe.Pos())
	fg.add(withLead(tp, pre, varDecl(tp, &cppir.Ref{Elem: &cppir.Const{Elem: xtyp}}, tmp, fg.expr(xe))))
	operand := cxxName(tmp)

	clauses := make([]*ast.CaseClause, len(s.Body.List))
	for i, c := range s.Body.List {
		clauses[i] = c.(*ast.CaseClause)
	}
	// cond is the test of a case: its types, one after the other.
	cond := func(c *ast.CaseClause) cppir.Expr {
		var parts []cppir.Expr
		for _, te := range c.List {
			t := g.info.TypeOf(te)
			switch {
			case g.info.Types[te].IsNil():
				parts = append(parts, &cppir.Binary{Op: "==", X: operand, Y: lit("nullptr")})
			case !complete(t):
				ct := g.typeExpr(te) // a C/C++ type: the foreign descriptor of its C++ type
				parts = append(parts, call("gp::type_is", operand, templateCall("gp::foreign_td", []cppir.Node{ct})))
			case isInterface(t):
				parts = append(parts, templateCall("gp::type_has", []cppir.Node{g.cxxType(t, te.Pos())}, operand))
			default:
				parts = append(parts, call("gp::type_is", operand, g.desc(t, te.Pos())))
			}
		}
		if len(parts) > 1 {
			for i := range parts {
				parts[i] = at(parts[i], precBitOr)
			}
		}
		return orList(parts)
	}
	// declare adds the declaration of v at the start of the clause c, if the clause uses it.
	declare := func(c *ast.CaseClause) {
		v, _ := g.info.Implicits[c].(*types.Var)
		if v == nil || !g.usedIn(c.Body, v) {
			return
		}
		typ := g.varType(v, c.Pos()) // also a C/C++ type (cvalues.go)
		var init cppir.Expr = operand
		if len(c.List) == 1 && !g.info.Types[c.List[0]].IsNil() {
			t := g.info.TypeOf(c.List[0])
			if isInterface(t) && complete(t) {
				init = g.toInterface(xt, t, operand, c.Pos())
			} else {
				init = templateCall("gp::unbox", []cppir.Node{typ}, operand)
			}
		}
		p := g.pos(c.Case)
		if fg.isBoxed(v) {
			fg.add(varDecl(p, ptrType(typ), fg.names.declare(v), newBox(typ, init)))
			return
		}
		fg.add(varDecl(p, typ, fg.names.declare(v), init))
	}
	body := func(c *ast.CaseClause) func() {
		return func() {
			declare(c)
			fg.stmts(c.Body, clauseEnd(c))
		}
	}

	var arms []arm
	var def *ast.CaseClause
	if !fg.flow.breakTo[s] {
		for _, c := range clauses {
			if c.List == nil {
				def = c
				continue
			}
			c := c
			arms = append(arms, arm{pos: c.Case, cond: func() ([]*cppir.Line, cppir.Expr) { return nil, cond(c) }, body: body(c)})
		}
		if def != nil {
			arms = append(arms, arm{pos: def.Case, body: body(def)})
		}
		if arms != nil {
			fg.chain(arms, end)
		}
	} else {
		// A break in a clause: the number of the clause that matches, then a C++ switch.
		index := fg.names.temp("case")
		p := g.pos(s.Switch)
		fg.add(varDecl(p, typeName("int"), index, &cppir.Unary{Op: "-", X: lit("1")}))
		n := 0
		values := make([][]cppir.Expr, len(clauses))
		for i, c := range clauses {
			if c.List == nil {
				continue // default
			}
			c := c
			k := strconv.Itoa(n)
			n++
			values[i] = []cppir.Expr{lit(k)}
			arms = append(arms, arm{pos: c.Case, cond: func() ([]*cppir.Line, cppir.Expr) { return nil, cond(c) }, body: func() {
				fg.add(&cppir.ExprStmt{Pos: g.pos(c.Case), X: &cppir.Assign{Op: "=", X: cxxName(index), Y: lit(k)}})
			}})
		}
		if arms != nil {
			fg.chain(arms, arms[len(arms)-1].pos)
		}
		sw := &cppir.Switch{Pos: p, Tag: cxxName(index)}
		fg.add(sw)
		for i, c := range clauses {
			cs := &cppir.Case{Pos: g.pos(c.Case), Values: values[i], Break: !endsWithJump(c)}
			sw.Cases = append(sw.Cases, cs)
			fg.fresh = false
			c := c
			fg.inner(&cs.Body, &cs.End, clauseEnd(c), body(c))
		}
		sw.End = g.pos(end)
	}
	fg.shadows += shadows
	fg.closeShadows(fg.shadows-shadows, end)
	fg.close(end)
	fg.breakLabel(s, end)
}
