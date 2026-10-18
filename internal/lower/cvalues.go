package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// C/C++ values in gocpp code (Phase 04 README §2.5, steps 4.4 and 4.5):
//
//	cstdio.printf("%d\n", 5)            ::printf("%d\n", 5);       (a call as a statement)
//	var n C.int = cstdio.puts("x")      int n = ::puts("x");       (an initializer of a C type)
//	var v std.vector[int]               ::std::vector<int64_t> v{}; (the zero value: value-initialized)
//	x := cmath.sqrt(2.0)                auto x = ...; (preserves ordinary prvalue elision and owns
//	                                    pair/tuple reference members: crange.go's ownedValue)
//	C.int(n), C.ulonglong(n)            gp::cgo::as<int>(n), gp::cgo::as<unsigned long long>(n)
//	std.string("hi")                    ::std::string("hi")        (a constructor)
//	int(cmath.floor(x))                 gp::cgo::as<int64_t>(::floor(x)) (a conversion to a gocpp
//	                                    type, gp_cgoh.As[int](...) after the check's rewrite)
//	return n, f(v) (C types)            return n; ::f(v)
//	x.m, x.m(1), x.m.n (x opaque)       gp::deref(x).m, gp::deref(x).m(1), gp::deref(gp::deref(x).m).n
//	f()(), v[0](x) (C++ callables)      f()(), v[0](x)
//	v[i], m["k"] (not a template)       v[i], m["k"]               (C++'s operator[]: no bounds check)
//	a + b, a == b, a << n, a &^ b       a + b, a == b, a << n, a & ~b (C++'s operators and rules)
//	p == nil                            p == nullptr
//	-x, +x, ^x, !x                      -x, +x, ~x, !x
//	*p                                  gp::cgo::star(p)           (Go's nil check for a raw pointer)
//	&x (x a C/C++ variable)             &x                         (a raw C++ address, §2.7)
//	if ok, for x.valid() (opaque bool)  if (gp::cgo::cond(ok)), for (; gp::cgo::cond(gp::deref(x).valid()); )
//	s += "!", it++, d--                 s += "!", ++it, --d        (stmt.go)
//	a, b := std.minmax(x, y)            auto gp_t0 = gp::values(::std::minmax(x, y)); (stmt.go)
//	for i, v := range c                 for (auto&& [i, v] : gp::range(c)) { (crange.go)
//
// An untyped constant where a C/C++ value takes it (an argument of a C/C++ function, the value of
// a variable or a result of a C/C++ type, a template argument, an operand or an index next to a
// C/C++ value) is a plain C literal (cConstant): "x" is a string literal there, not a gp::string,
// and 5 an int. A typed gocpp number, bool or string next to a C/C++ value in an operator is a
// user error, as in cgo (cguard.go: mismatched types): C++ would convert one of them silently.
//
// Every opaque expression goes through opaqueExpr: what it doesn't translate yet (slicing,
// &x of anything but a variable) is reported by the guard (cguard.go) as "not supported yet",
// never written as broken C++. Composite literals are clits.go's.

// varType returns the C++ type of the variable v (also a parameter, a result or a struct field):
// its gocpp type, or, for a type with C/C++ parts, the C++ of the type expression it was declared
// with, or of the composite literal that a package variable without a type is declared from;
// auto for a local declared from a C/C++ value without a type (x := lib.f()).
func (g *gen) varType(v *types.Var, pos token.Pos) cppir.Type {
	if complete(v.Type()) {
		return g.cxxType(v.Type(), pos)
	}
	if e := g.pkg.Opaque.TypeExpr(v); e != nil {
		return g.typeExpr(e)
	}
	if e := g.sigTypes()[v]; e != nil {
		return g.typeExpr(e)
	}
	if lit := g.pkgVarLit(v); lit != nil {
		return g.typeWith(v.Type(), lit.Type, pos) // the literal's type, with an inferred array's length (clits.go)
	}
	if v.IsField() || v.Parent() == nil || v.Parent() == g.pkg.Types.Scope() {
		g.unsupported(pos, "package variables of C/C++ values without a type ("+v.Name()+"): write its type, var "+v.Name()+" T = ...")
		return typeName("gp_unsupported")
	}
	return typeName("auto")
}

// sigTypes returns the type expressions of the parameters and results of the package's
// functions and func literals, also those without a name (which have no object of their own for
// Opaque.TypeExpr), for the ones whose types have C/C++ parts.
func (g *gen) sigTypes() map[*types.Var]ast.Expr {
	if g.sigExprs != nil {
		return g.sigExprs
	}
	g.sigExprs = make(map[*types.Var]ast.Expr)
	add := func(list *ast.FieldList, vars *types.Tuple) {
		if list == nil {
			return
		}
		i := 0
		for _, f := range list.List {
			n := max(len(f.Names), 1)
			for range n {
				if i < vars.Len() && !complete(vars.At(i).Type()) {
					g.sigExprs[vars.At(i)] = f.Type
				}
				i++
			}
		}
	}
	for _, f := range g.pkg.Load.Files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			var ft *ast.FuncType
			var sig *types.Signature
			switch x := n.(type) {
			case *ast.FuncDecl:
				if fn, ok := g.info.Defs[x.Name].(*types.Func); ok {
					ft, sig = x.Type, fn.Signature()
				}
			case *ast.FuncLit:
				sig, _ = g.info.TypeOf(x).(*types.Signature)
				ft = x.Type
			}
			if sig != nil {
				add(ft.Params, sig.Params())
				add(ft.Results, sig.Results())
			}
			return true
		})
	}
	return g.sigExprs
}

// opaqueExpr returns the C++ of the opaque expression e (check.Opaque): a C/C++ value whose type
// go/types doesn't know.
func (fg *funcGen) opaqueExpr(e ast.Expr) cppir.Expr {
	switch x := e.(type) {
	case *ast.ParenExpr:
		return fg.expr(x.X)
	case *ast.Ident:
		return fg.ident(x) // (a boxed local is (*x): cbox.go)
	case *ast.CallExpr:
		return fg.call(x)
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		if r, ok := fg.g.cRefOf(x); ok {
			return fg.g.cValue(r, x)
		}
		switch y := x.(type) {
		case *ast.SelectorExpr:
			if fg.g.info.Selections[y] != nil {
				// A field of a C/C++ type of a gocpp struct, or a method value or expression of a
				// gocpp type with C/C++ parameters or results (structs.go, methods.go).
				return fg.selector(y)
			}
			x := fg.expr(y.X)
			if path := fg.g.pkg.PromotedPath(y); path != nil {
				// A member of the one C/C++ type that a struct embeds (check/promoted.go):
				// through the embedded fields, o.vector.push_back for o.push_back.
				x, _ = fg.fieldSteps(x, fg.g.info.TypeOf(y.X), path, y.Pos())
			}
			return fg.g.cMemberOf(x, y.Sel)
		case *ast.IndexExpr:
			if gocppContainer(fg.g.info.TypeOf(y.X)) {
				return fg.index(y) // of a gocpp slice, array or map of C/C++ values: Go's index
			}
			// C++'s operator[] (Phase 04 README §2.5): no gocpp bounds check, and a std::map
			// inserts the key.
			return &cppir.Index{X: fg.expr(y.X), Index: fg.cValueOf(y.Index)}
		}
	case *ast.BinaryExpr:
		return fg.cBinary(x.Op, x.X, x.Y)
	case *ast.UnaryExpr:
		return fg.cUnary(x)
	case *ast.SliceExpr:
		if gocppContainer(fg.g.info.TypeOf(x.X)) {
			return fg.slice(x) // of a gocpp slice or array of C/C++ values: Go's (ops.go)
		}
		return fg.cSlice(x) // of a C array (cunsafe.go)
	case *ast.CompositeLit:
		return fg.compLit(x) // of a C/C++ type, or a gocpp one with C/C++ parts (clits.go)
	case *ast.FuncLit:
		return fg.funcValue(x) // with C/C++ parameters or results: a callback (closures.go)
	case *ast.StarExpr:
		// C++'s *x, but with Go's nil check for a raw pointer (and a gocpp pointer that comes
		// back from C++), gocpp/interop.hpp.
		return call("gp::cgo::star", fg.expr(x.X))
	case *ast.TypeAssertExpr:
		return fg.typeAssert(x) // x.(std.string) of an interface value (iconv.go)
	}
	fg.g.errorf(e.Pos(), "internal error: an opaque %s that the guard let through (cguard.go)", nodeKind(e))
	return placeholder
}

// cMemberOf returns the C++ of the member name of the C/C++ value x: gp::deref(x).name, which
// dereferences a raw pointer (and a gocpp pointer) one level with Go's nil check and passes an
// object through, like Go's x.m (gocpp/interop.hpp). The name is a C/C++ name (macros.go): a
// member can be a macro (glibc's st_mtime).
func (g *gen) cMemberOf(x cppir.Expr, name *ast.Ident) cppir.Expr {
	g.noteCName(name.Name, name.Pos())
	return g.cNode(&cppir.Member{X: call("gp::deref", x), Name: name.Name}).(*cppir.Member)
}

// opaqueCallee reports whether c calls a C/C++ value that is not a C/C++ name: a member of a
// C/C++ value (v.push_back), or a C++ callable (f()(), v[0](x), a function pointer in a
// variable). Its arguments are C/C++ arguments (cArgs), and C++ evaluates the callee first.
func (g *gen) opaqueCallee(c *ast.CallExpr) bool {
	if !g.pkg.Opaque.IsOpaque(c.Fun) || g.info.Types[c.Fun].IsType() || g.pkg.Conversion(c) != nil || g.calledSig(c) != nil ||
		g.cgoHelper(c) != "" || g.cPointerType(c) != nil || g.unsafeFunc(c) != "" || g.pkg.OpaqueSlice(c) != nil || g.pkg.UnsafeWrapper(c) != nil {
		return false
	}
	if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok && g.info.Selections[sel] != nil {
		return false // a method of a gocpp type with C/C++ parameters (methods.go)
	}
	_, ok := g.cRefOf(c.Fun)
	return !ok
}

// opaqueCall returns the C++ of the call c of a C/C++ value (opaqueCallee).
func (fg *funcGen) opaqueCall(c *ast.CallExpr) cppir.Expr {
	if c.Ellipsis.IsValid() {
		fg.g.unsupported(c.Ellipsis, "... in a call of a C/C++ function")
		return placeholder
	}
	return &cppir.Call{Fun: fg.expr(c.Fun), Args: fg.cArgs(c.Args)}
}

// cBinary returns the C++ of x op y where x or y is a C/C++ value: C++'s operator, with C++'s
// rules (Phase 04 README §2.5), its operands evaluated from left to right like Go's (order.go
// moved what needs it into temporaries). An untyped constant is a C literal, nil is nullptr
// (cValueOf). The operands of && and || are conditions (cBool). The parentheses are those of
// the gocpp operators (ops.go), which C++ doesn't need but g++ -Wall and the reader do.
func (fg *funcGen) cBinary(op token.Token, x, y ast.Expr) cppir.Expr {
	switch op {
	case token.LAND, token.LOR:
		a, b := fg.cBool(x), fg.cBool(y)
		if op == token.LOR {
			return groupExprs(a, "||", b, precLogOr, precLogAnd)
		}
		return &cppir.Binary{Op: "&&", X: a, Y: b}
	}
	a, b := fg.cValueOf(x), fg.cValueOf(y)
	switch op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		return &cppir.Binary{Op: op.String(), X: at(a, precShift), Y: at(b, precShift)}
	case token.SHL, token.SHR:
		// clang warns about a + b << 3 (-Wshift-op-parentheses).
		return &cppir.Binary{Op: op.String(), X: at(a, precUnary), Y: b}
	case token.AND:
		return groupExprs(a, "&", b, precBitAnd, precUnary-1)
	case token.OR:
		return groupExprs(a, "|", b, precBitOr, precUnary-1)
	case token.XOR:
		return groupExprs(a, "^", b, precBitXor, precUnary-1)
	case token.AND_NOT:
		if cppir.Prec(a) != precBitAnd {
			a = at(a, precUnary)
		}
		return &cppir.Binary{Op: "&", X: a, Y: &cppir.Unary{Op: "~", X: b}}
	}
	return &cppir.Binary{Op: op.String(), X: a, Y: b}
}

// cAssignOp returns the C++ operator of the assignment operation tok (+=, &^=) on a C/C++
// target, and whether its value is complemented first (x &^= y is x &= ~y).
func cAssignOp(tok token.Token) (string, bool) {
	if tok == token.AND_NOT_ASSIGN {
		return "&=", true
	}
	return tok.String(), false
}

// cUnary returns the C++ of a unary operator on a C/C++ value: -x, +x, !x and ^x (C++'s ~x),
// and &x of a C/C++ variable, a raw C++ address (Phase 04 README §2.7: C++'s lifetime rules).
func (fg *funcGen) cUnary(e *ast.UnaryExpr) cppir.Expr {
	switch e.Op {
	case token.NOT:
		return &cppir.Unary{Op: "!", X: fg.cBool(e.X)}
	case token.SUB, token.ADD:
		return &cppir.Unary{Op: e.Op.String(), X: fg.expr(e.X)}
	case token.XOR:
		return &cppir.Unary{Op: "~", X: fg.expr(e.X)}
	case token.AND:
		if lit, ok := ast.Unparen(e.X).(*ast.CompositeLit); ok {
			if fg.g.cxxLit(lit) {
				return fg.cNew(lit) // &T{...} of a C/C++ type (cunsafe.go)
			}
			return fg.addr(e) // &[]C.int{1}: a gocpp literal in a new block (addr.go)
		}
		switch ast.Unparen(e.X).(type) {
		case *ast.SelectorExpr, *ast.IndexExpr:
			if owned, _ := ownedPath(fg.g.info, e.X); owned {
				return fg.cOwnedAddr(e) // &s.f, &xs[i] of a gocpp value: a gp::cptr (cunsafe.go)
			}
		}
		return &cppir.Unary{Op: "&", X: fg.expr(e.X)}
	}
	fg.g.errorf(e.OpPos, "internal error: the operator %s on a C/C++ value that the guard let through (cguard.go)", e.Op)
	return placeholder
}

// cBool returns the C++ of e as a condition (of an if, a for, a case of a switch without a tag)
// or an operand of &&, || and !: a C/C++ value that is not a comparison or a logical operator
// goes through gp::cgo::cond, which accepts only a bool, as Go does (C++ would take a C int or a
// pointer). A gocpp value is a bool already (go/types checked it).
func (fg *funcGen) cBool(e ast.Expr) cppir.Expr {
	if c, ok := ast.Unparen(e).(*ast.CallExpr); ok && fg.temps[ast.Unparen(e)] == "" {
		if conv := fg.g.pkg.Conversion(c); conv != nil && conv.Kind == check.Compare {
			// A comparison of C/C++ values as a condition: C++ makes it a bool there.
			return fg.expr(c.Args[0])
		}
	}
	x := fg.expr(e)
	if !fg.g.pkg.Opaque.IsOpaque(e) {
		return x
	}
	switch u := ast.Unparen(e).(type) {
	case *ast.BinaryExpr:
		switch u.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ, token.LAND, token.LOR:
			return x
		}
	case *ast.UnaryExpr:
		if u.Op == token.NOT {
			return x
		}
	}
	return call("gp::cgo::cond", x)
}

// cValue returns the C++ of the C/C++ name r, written as e, used as a value.
func (g *gen) cValue(r cRef, e ast.Expr) cppir.Expr {
	switch r.kind {
	case interop.Namespace:
		g.userError(e.Pos(), "%s is a C++ namespace, not a value", r.src)
		return placeholder
	case interop.Type, interop.Template:
		g.userError(e.Pos(), "%s is a C/C++ type, not a value", r.src)
		return placeholder
	}
	return r.x
}

// cCall returns the C++ of the call c if it calls a C/C++ name (a function, a function
// template's instance, a macro, a constructor or a C type's conversion), and false if it
// doesn't.
func (fg *funcGen) cCall(c *ast.CallExpr) (cppir.Expr, bool) {
	r, ok := fg.g.cRefOf(c.Fun)
	if !ok {
		return nil, false
	}
	if r.x == placeholder {
		return placeholder, true // reported
	}
	args := fg.cArgs(c.Args)
	if r.kind == interop.Value {
		fg.rawPointerArgs(c.Args, args) // a gocpp pointer through a C function's ... (cunsafe.go)
	}
	switch r.kind {
	case interop.Namespace:
		fg.g.userError(c.Fun.Pos(), "%s is a C++ namespace, not a function", r.src)
		return placeholder, true
	case interop.Type:
		if len(args) == 1 && isUnsafePointer(fg.g.info.TypeOf(c.Args[0])) {
			// C.callback_t(p) of an unsafe.Pointer: a conversion to a C type, in the runtime
			// (a function pointer needs std::bit_cast, not a cast: cunsafe.go).
			return templateCall("gp::cgo::as", []cppir.Node{fg.g.cType(r, c.Fun)}, args[0]), true
		}
		if n, ok := r.x.(*cppir.Name); ok && n.Args == nil && isBuiltinC(n.Name) && len(args) == 1 {
			// C.int(n): a conversion, with Go's results where a C++ cast is undefined
			// behavior (a float out of range: gp::cgo::as, gocpp/interop.hpp).
			return templateCall("gp::cgo::as", []cppir.Node{typeName(n.Name)}, args[0]), true
		}
	}
	if c.Ellipsis.IsValid() {
		fg.g.unsupported(c.Ellipsis, "... in a call of a C/C++ function")
		return placeholder, true
	}
	return &cppir.Call{Fun: r.x, Args: args}, true
}

// isBuiltinC reports whether the C++ name s is a C type of cBuiltins.
func isBuiltinC(s string) bool {
	for _, b := range cBuiltins {
		if b == s {
			return true
		}
	}
	return false
}

// cArgs returns the C++ of the arguments of a call of a C/C++ function: an untyped constant is a
// plain C literal (cConstant), and so is a typed string constant (const S string = "x";
// cstdio.puts(S): its value is known, and a gp::string would not convert to const char*), nil
// is nullptr, a holder of C.cstr its const char* (holder.gp_ptr(), also through a C `...`:
// cstrings.go), and a gocpp value has its gocpp type.
func (fg *funcGen) cArgs(list []ast.Expr) []cppir.Expr {
	xs := make([]cppir.Expr, len(list))
	for i, a := range list {
		if tv := fg.g.info.Types[a]; tv.Value != nil && tv.Value.Kind() == constant.String && fg.temps[ast.Unparen(a)] == "" {
			xs[i] = lit(cppir.CString(constant.StringVal(tv.Value)))
			continue
		}
		if fg.g.isHolder(a) {
			xs[i] = &cppir.Call{Fun: &cppir.Member{X: fg.cValueOf(a), Name: "gp_ptr"}}
			continue
		}
		if t := fg.outPtrTarget(a); t != nil {
			xs[i] = call("gp::cgo::out_ptr", fg.expr(t)) // &db of a *C.T variable, for a T** parameter
			continue
		}
		if fg.g.isPtrToUnsafePointer(a) {
			// &p of an unsafe.Pointer variable p, for a void** parameter: the argument is a
			// void* that the function writes, and that p gets when the statement ends.
			xs[i] = call("gp::cgo::out_ptr", fg.expr(a))
			continue
		}
		xs[i] = fg.cMoved(a, fg.cValueOf(a))
	}
	return xs
}

// opaqueMember returns fun as a member x.m of a C/C++ value (not a C/C++ name, nor a field or a
// method of a gocpp type), or nil.
func (g *gen) opaqueMember(fun ast.Expr) *ast.SelectorExpr {
	sel, ok := ast.Unparen(fun).(*ast.SelectorExpr)
	if !ok || !g.pkg.Opaque.IsOpaque(sel) || g.info.Selections[sel] != nil {
		return nil
	}
	if _, ok := g.cRefOf(sel); ok {
		return nil
	}
	return sel
}

// cValueOf returns the C++ of e where a C/C++ value takes it (cArgs).
func (fg *funcGen) cValueOf(e ast.Expr) cppir.Expr {
	if _, ok := fg.temps[ast.Unparen(e)]; ok {
		return fg.expr(ast.Unparen(e)) // order.go named it (nameOperands)
	}
	tv := fg.g.info.Types[e]
	switch {
	case tv.Value != nil:
		return fg.g.cConstantOf(e)
	case tv.IsNil():
		return lit("nullptr")
	case tv.Type != nil && isFunc(tv.Type):
		return fg.callbackValue(e) // a gocpp func for C/C++ code (ccallback.go)
	}
	return fg.exactExpr(e)
}

// asConversion returns the C++ of the conversion conv of a C/C++ value to a gocpp type
// (gp_cgoh.As[T](x) after the check's rewrite): gp::cgo::as<T>(x) for a number, a bool or a
// string, which is Go's conversion also where a static_cast is undefined behavior (a float out
// of range); string(x) copies the bytes of a std::string or std::string_view (and is Go's
// string(r) of a C integer).
func (fg *funcGen) asConversion(c *ast.CallExpr, conv *check.Conversion) cppir.Expr {
	if conv.Kind == check.Compare {
		// x == y of C/C++ values, gp_cgoh.Cmp(x == y): a Go bool, as in cgo.
		return call("gp::cgo::to_bool", fg.expr(c.Args[0]))
	}
	if conv.Kind != check.AsCall || len(c.Args) != 1 {
		fg.g.unsupported(c.Pos(), "this conversion of a C/C++ value")
		return placeholder
	}
	to := fg.g.info.TypeOf(c)
	if x, ok := fg.cUnsafeConversion(c, to); ok {
		return x // unsafe.Pointer(p), (*byte)(p) of a C void* (cunsafe.go)
	}
	if isInterface(to) {
		// any(x) of a C/C++ value: the interface holds it as it is (iconv.go).
		return fg.g.toInterface(types.Typ[types.Invalid], to, fg.expr(c.Args[0]), c.Pos())
	}
	b := basicOf(to)
	if b == nil || b.Info()&(types.IsNumeric|types.IsBoolean|types.IsString) == 0 || b.Info()&types.IsComplex != 0 || isNewtype(to) {
		// A named type, a complex number, a pointer, a slice, ...
		fg.g.unsupported(c.Pos(), "conversions of C/C++ values to "+types.TypeString(to, types.RelativeTo(fg.g.pkg.Types)))
		return placeholder
	}
	return templateCall("gp::cgo::as", []cppir.Node{fg.g.cxxType(to, c.Pos())}, fg.expr(c.Args[0]))
}

// markAllCValues runs markCValues over the package's functions and package variables.
func (g *gen) markAllCValues() {
	for _, f := range g.pkg.Load.Files {
		for _, d := range f.AST.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if fn, ok := g.info.Defs[d.Name].(*types.Func); ok && d.Body != nil {
					g.markCValues(d.Body, fn.Signature())
				}
			case *ast.GenDecl:
				g.markCValues(d, nil)
			}
		}
	}
}

// markCallArgs marks the func arguments of the call c that go to C/C++ types: the fixed
// parameters of a gocpp function that have such a type, and the elements that append adds to a
// slice of such a type (ccallback.go). mark is markCValues's.
func (g *gen) markCallArgs(c *ast.CallExpr, mark func(ast.Expr, bool, types.Type)) {
	if id, ok := ast.Unparen(c.Fun).(*ast.Ident); ok {
		if b, ok := g.info.Uses[id].(*types.Builtin); ok {
			if b.Name() == "append" && len(c.Args) > 1 && !c.Ellipsis.IsValid() {
				if sl, ok := under(g.info.TypeOf(c.Args[0])).(*types.Slice); ok {
					for _, a := range c.Args[1:] {
						if t := g.info.TypeOf(a); t != nil && isFunc(t) { // (only funcs: constants stay as they were)
							mark(a, !complete(sl.Elem()), sl.Elem())
						}
					}
				}
			}
			return
		}
	}
	if tv, ok := g.info.Types[c.Fun]; !ok || tv.IsType() {
		return
	}
	sig, _ := under(g.info.TypeOf(c.Fun)).(*types.Signature)
	if sig == nil {
		return
	}
	n := sig.Params().Len()
	for i, a := range c.Args {
		if i >= n || sig.Variadic() && i >= n-1 {
			break
		}
		pt := sig.Params().At(i).Type()
		if t := g.info.TypeOf(a); t != nil && isFunc(t) {
			mark(a, !complete(pt), pt)
		}
	}
}

// markCValues finds the untyped constants in n that are the values of C/C++ variables, results
// and elements (var n C.int = 5, n = 5, return 5 from a func with a C/C++ result, []C.int{5}),
// which are C literals there (cConstant): go/types doesn't convert them, as it has no type for
// the target. (nil is nullptr there anyway.) sig is the signature of the function that n is the
// body of (nil for a package variable's value).
func (g *gen) markCValues(n ast.Node, sig *types.Signature) {
	// mark marks e, the value for a target of type to (nil when go/types has none): a func goes
	// to a C/C++ place only if the place is not a gocpp func variable (to is no signature).
	mark := func(e ast.Expr, cTarget bool, to types.Type) {
		tv, ok := g.info.Types[e]
		switch {
		case !ok || !cTarget:
		case tv.Value != nil:
			if g.cConsts == nil {
				g.cConsts = make(map[ast.Expr]bool)
			}
			g.cConsts[ast.Unparen(e)] = true
		case tv.Type != nil && !tv.IsNil() && isFunc(tv.Type) && (to == nil || !isFunc(to)):
			// A gocpp func in a C/C++ variable, member or element: a callback (ccallback.go).
			if g.cFuncs == nil {
				g.cFuncs = make(map[ast.Expr]bool)
			}
			g.cFuncs[ast.Unparen(e)] = true
		}
	}
	isC := func(e ast.Expr) bool {
		if g.pkg.Opaque.IsOpaque(e) {
			return true
		}
		if id, ok := e.(*ast.Ident); ok {
			if v, ok := g.info.ObjectOf(id).(*types.Var); ok {
				return !complete(v.Type())
			}
		}
		return false
	}
	ast.Inspect(n, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			if len(s.Lhs) == len(s.Rhs) {
				for i, v := range s.Rhs {
					mark(v, isC(s.Lhs[i]), g.info.TypeOf(s.Lhs[i]))
				}
			}
		case *ast.ValueSpec:
			for i, v := range s.Values {
				if i < len(s.Names) && len(s.Names) == len(s.Values) {
					mark(v, isC(s.Names[i]), g.info.TypeOf(s.Names[i]))
				}
			}
		case *ast.ReturnStmt:
			if sig != nil && sig.Results().Len() == len(s.Results) {
				for i, v := range s.Results {
					mark(v, !complete(sig.Results().At(i).Type()), sig.Results().At(i).Type())
				}
			}
		case *ast.CallExpr:
			// A func passed to a gocpp function whose parameter has a C/C++ type (a C function
			// pointer type: func run(f lib.cb_t)), or appended to a slice of such elements.
			g.markCallArgs(s, mark)
		case *ast.CompositeLit:
			// The elements of a gocpp literal that go to C/C++ types ([]C.int{1}, P{n: 1} with a
			// field n C.int; clits.go). A literal of a C/C++ type writes its own (cValueOf).
			for _, elt := range s.Elts {
				parts := []ast.Expr{elt}
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					parts = []ast.Expr{kv.Key, kv.Value}
				}
				for _, x := range parts {
					if t, ok := g.litTarget(s, x); ok {
						mark(x, !complete(t.to), t.to)
					}
				}
			}
		case *ast.FuncLit:
			lit, _ := g.info.TypeOf(s).(*types.Signature)
			if lit != nil {
				g.markCValues(s.Body, lit)
			}
			return false
		}
		return true
	})
}
