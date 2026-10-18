package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Methods (Phase 03 README §2.5, step 3.10).
//
//	Go                              C++
//	func (v T) M(a int) int         int64_t M(int64_t a) const;  (a member of struct T)
//	                                int64_t T::M(int64_t a) const { T v = *this; ... }
//	func (p *T) PM(a int)           static void PM(gp::ptr<T> p, int64_t a);
//	                                void T::PM(gp::ptr<T> p, int64_t a) { ... }
//	x.M(1), f().M(1)                x.M(1), f().M(1)
//	p.M(1) (p is *T)                p->M(1)  (gp::ptr's -> panics like Go for a nil p)
//	g.M(1) (g a package *T)         gp::ptr<T>(g)->M(1)  (a copy of the pointer holds the object)
//	p.PM(1)                         T::PM(p, 1)
//	x.PM(1) (x addressable)         T::PM(<&x>, 1)  (the owner table: addr.go; x is boxed, box.go)
//	o.M() (promoted)                o.Inner.M(), o.Inner->M(), Inner::PM(gp::field_ptr(...))
//	defer x.M(a)                    gp_df.gp_push_call([](auto&& gp_a0, auto&& gp_a1) {
//	                                    gp_a0.M(gp_a1); }, x, a);  (the receiver copied now)
//	f := x.M, x.PM, T.M, (*T).PM    gp::func<Sig>(<a generic lambda>)  (methodValue)
//
// A value receiver is the method's own copy, as in Go: the const member function starts with T v
// = *this; (a box, gp::ptr<T> v = gp::new_<T>(*this);, when the boxing analysis boxes v, or when T
// is larger than 128 KiB, as for a large parameter: D24). A const T& would see the writes that
// the method makes through other pointers, and could dangle (the audit), so it is only for a body
// that provably can't change or free the object (readOnly): const T& v = *this;. A receiver
// without a name (or _) is not copied: nothing reads it. A pointer receiver is the first
// parameter of a static member function, an owning gp::ptr<T> that the method can keep, and a
// boxed one is copied into its box like any parameter (decl.go: paramBoxes).
//
// The members are declared in the struct (typedecl.go: structDef, named.go: newtypeDef) and
// defined out of line, after all the structs and the prototypes, in source order with the
// functions, so the order of the declarations doesn't matter (a method may use a type declared
// after it) and methods may call each other. A method's C++ name goes through MethodName like a
// field's, so it never clashes with a field, its struct's name or a C++ word. Inside a member
// function the members hide the package's names: a package variable, function or type whose
// C++ name is a member's is written qualified there (gpkg_main::X: expr.go's cname, typedecl.go's
// structType), and so are the names of the base classes of a named type (named.go: baseNames).
// Go has no methods on local types (a method is declared at package level, on a package-level
// type), so a hoisted local type has none of its own (only promoted ones, Phase 05).
//
// Go's order (order.go): the receiver is evaluated with the arguments, as gc does
// (typecheck.FixMethodCall makes it the first argument): its calls before the arguments' calls,
// in their places, and its reads after all the calls of the call, like any operand. So
// g.M(f()), where f changes the package variable g, calls M on the new g (and panics for a nil
// g after f ran): analyzeMethod. The pointer of a value method called through a pointer that is
// not a local of the caller (a package or boxed variable, a field, an element) is copied first,
// gp::ptr<T>(g)->M(...), so the object stays alive until the method has made its copy, even if
// the arguments' evaluation dropped the last other pointer to it.
//
// The fmt shim (gocpp/fmtshim.hpp) calls String() and Error() through hooks that the method
// sets give each struct (hookDefs): gp::string gp_String() const for a value method set,
// static gp::string gp_String_ptr(const gp::ptr<T>&) for a pointer-only one, promoted methods
// included; the same for Error().
//
// Method values (f := x.M) and method expressions (T.M, (*T).PM) are func values (step 3.10b,
// on step 3.11's gp::func): methodValue.

// A methodInfo is what a method's funcInfo has more than a function's.
type methodInfo struct {
	recv   *types.Var      // the receiver (its name may be "" or _)
	ptr    bool            // a pointer receiver: a static member function
	si     *structInfo     // the receiver's struct
	member string          // the member's C++ name: M (funcInfo.name is the qualified T::M)
	named  *types.Named    // the receiver's base type
	hidden map[string]bool // the struct's members (memberNames)
	// ref: the value receiver is a const T& to *this, since the body can't change it (readOnly).
	ref bool
}

// collectMethod adds the method declaration d to the package's functions (file.go: collect).
// Its C++ names come with prepare, when all the types are known.
func (g *gen) collectMethod(d *ast.FuncDecl) {
	switch {
	case d.Body == nil:
		g.unsupported(d.Pos(), "a method without a body")
		return
	case d.Name.Name == "_":
		return // can't be called: nothing to write
	}
	fn, _ := g.info.Defs[d.Name].(*types.Func)
	if fn == nil {
		g.errorf(d.Pos(), "internal error: no object for method %s", d.Name.Name)
		return
	}
	recv := fn.Signature().Recv()
	_, ptr := types.Unalias(recv.Type()).(*types.Pointer)
	base := recv.Type()
	if ptr {
		base = types.Unalias(base).(*types.Pointer).Elem()
	}
	named, _ := types.Unalias(base).(*types.Named)
	if named == nil {
		g.errorf(d.Pos(), "internal error: the receiver of %s is not a named type", d.Name.Name)
		return
	}
	if named.TypeParams() != nil {
		g.unsupported(d.Pos(), "methods of generic types (Phase 07)")
		return
	}
	m := &methodInfo{recv: recv, ptr: ptr, named: named}
	g.funcs = append(g.funcs, &funcInfo{decl: d, obj: fn, method: m})
}

// prepareRecv is prepare's part for a method (decl.go): the C++ names of the member and of the
// receiver, and the receiver's parameter for a pointer method, which it returns (nil for a
// function or a value method).
func (f *funcInfo) prepareRecv(g *gen) []cppir.Param {
	m := f.method
	if m == nil {
		return nil
	}
	m.si = g.structOf(m.named, f.decl.Pos())
	if m.si == nil {
		return nil
	}
	m.member = g.methodName(m.si.name, f.obj.Name())
	m.hidden = g.memberNames(m.si)
	f.name = m.si.name + "::" + m.member
	v := m.recv
	named := v.Name() != "" && v.Name() != "_"
	if !m.ptr {
		if named {
			f.names.declare(v)
			m.ref = f.boxed[v] == 0 && readOnly(g.info, f.decl.Body, v)
			if !m.ref && f.boxed[v] == 0 && largeVar(v.Type()) {
				f.boxed[v] = boxLarge // a copy larger than 128 KiB: in a box (recvCopy)
			}
		}
		return nil
	}
	defer g.hideMembers(m.hidden)()
	p := cppir.Param{Type: g.cxxType(v.Type(), f.decl.Pos())}
	if named {
		p.Name = f.names.declare(v)
		if f.boxed[v] != 0 {
			f.args = append(f.args, v) // copied into its box at the entry (paramBoxes)
		}
	}
	return []cppir.Param{p}
}

// members returns the C++ names of the members of the method's struct, which hide the package's
// names in its body (hideMembers), or nil for a function.
func (f *funcInfo) members() map[string]bool {
	if f.method == nil {
		return nil
	}
	return f.method.hidden
}

// quals returns the qualifiers of the function's definition: const for a value method.
func (f *funcInfo) quals() []string {
	if f.method != nil && !f.method.ptr {
		return []string{"const"}
	}
	return nil
}

// hideMembers makes the names members of the struct whose method is being lowered (typedecl.go:
// typeDecls.members) until the function it returns is called.
func (g *gen) hideMembers(names map[string]bool) func() {
	td := g.tdecls()
	old := td.members
	td.members = names
	return func() { td.members = old }
}

// methodNames returns the C++ names of the methods that the named type of si declares (none for
// an anonymous struct), in the order of go/types.
func (g *gen) methodNames(si *structInfo) []string {
	named, ok := si.t.(*types.Named)
	if !ok {
		return nil
	}
	var names []string
	for i := range named.NumMethods() {
		if m := named.Method(i); m.Name() != "_" {
			names = append(names, g.methodName(si.name, m.Name()))
		}
	}
	return names
}

// memberNames returns the C++ names that the members of the struct si (and its base classes)
// bring into the body of its member functions: its fields, its methods and, for a named type
// that is not a struct, the injected class names of its bases (named.go: baseNames).
func (g *gen) memberNames(si *structInfo) map[string]bool {
	names := make(map[string]bool)
	for _, f := range si.fields {
		names[f] = true
	}
	for _, m := range g.methodNames(si) {
		names[m] = true
	}
	for _, m := range g.forwarderNames(si) { // promote.go
		names[m] = true
	}
	if si.under != nil {
		for n := range baseNames {
			names[n] = true
		}
	}
	return names
}

// methodsOf returns the methods of the struct si (prepared), in source order.
func (g *gen) methodsOf(si *structInfo) []*funcInfo {
	var ms []*funcInfo
	for _, f := range g.funcs {
		if f.method != nil && f.method.si == si {
			ms = append(ms, f)
		}
	}
	return ms
}

// methodDecls returns the declarations of the members that si's methods and the fmt shim's
// hooks (hookDefs) add to its struct.
func (g *gen) methodDecls(si *structInfo) []cppir.Decl {
	var decls []cppir.Decl
	for _, f := range g.methodsOf(si) {
		d := &cppir.Func{Result: f.result, Name: f.method.member, Params: f.params}
		if f.method.ptr {
			d.Specs = []string{"static"}
		} else {
			d.Quals = []string{"const"}
		}
		decls = append(decls, d)
	}
	decls = append(decls, g.fwdDecls(si)...) // promote.go
	for _, h := range g.hooks(si) {
		d := &cppir.Func{Result: typeName("gp::string"), Name: h.name}
		if h.ptr {
			d.Specs = []string{"static"}
			d.Params = []cppir.Param{h.param(si)}
		} else {
			d.Quals = []string{"const"}
		}
		decls = append(decls, d)
	}
	return decls
}

// recvCopy adds a value receiver's copy at the method's entry, on the line of the func: T v =
// *this;, or its box (see the top). It does nothing for a function, a pointer method or a
// receiver without a name.
func (fg *funcGen) recvCopy() {
	m := fg.fn.method
	if m == nil || m.ptr || m.si == nil {
		return
	}
	v := m.recv
	name, ok := fg.names.names[v]
	if !ok {
		return // no name, or _: nothing reads it
	}
	p := fg.g.pos(fg.fn.decl.Pos())
	typ := fg.g.cxxType(v.Type(), v.Pos())
	this := deref(lit("this"))
	if m.ref {
		fg.add(varDecl(p, &cppir.Ref{Elem: &cppir.Const{Elem: typ}}, name, this))
		return
	}
	if fg.isBoxed(v) {
		fg.add(varDecl(p, ptrType(typ), name, newBox(typ, this)))
		return
	}
	fg.add(varDecl(p, typ, name, this))
}

// readOnly reports whether the body of a value method with the receiver v provably can't
// change v's object while it runs, so that v can be a const reference to it (Phase 03 README
// §2.5): it calls nothing but the builtins that write no memory and run no Go code (len, cap,
// min, max, new, make, print, println, panic) and conversions, takes no address, has no defer,
// go or func literal, ranges over no func or channel, and assigns (=, op=, ++, range with =)
// only variables of its own other than v, or parts of them (box.go: root). Then nothing writes
// memory that v's object is in (no write through a pointer, a slice, a map or a package
// variable, and no call that could), and nothing frees it: the caller holds it for the call (a
// copy of a pointer that is not a local of the caller: methodCall). With goroutines (Phase 08),
// a const T& receiver can see the writes of another goroutine while the method runs, but only
// in a program that has a data race in Go too.
func readOnly(info *types.Info, body *ast.BlockStmt, v *types.Var) bool {
	b := &boxer{info: info}
	own := func(e ast.Expr) bool {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok && id.Name == "_" {
			return true
		}
		w := b.root(e) // the local variable that e is or is in (not through a pointer)
		return w != nil && w != v
	}
	ok := true
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if info.Types[x.Fun].IsType() {
				break
			}
			id, _ := ast.Unparen(x.Fun).(*ast.Ident)
			b, isBuiltin := info.Uses[id].(*types.Builtin)
			if !isBuiltin {
				ok = false
				break
			}
			switch b.Name() {
			case "len", "cap", "min", "max", "new", "make", "print", "println", "panic":
			default:
				ok = false
			}
		case *ast.UnaryExpr:
			if x.Op == token.AND || x.Op == token.ARROW {
				ok = false
			}
		case *ast.FuncLit, *ast.DeferStmt, *ast.GoStmt, *ast.SendStmt, *ast.SelectStmt:
			ok = false
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				ok = ok && own(l)
			}
		case *ast.IncDecStmt:
			ok = ok && own(x.X)
		case *ast.RangeStmt:
			switch under(info.TypeOf(x.X)).(type) {
			case *types.Signature, *types.Chan:
				// A range over a func calls user code without a CallExpr (Phase 07), one over a
				// channel blocks and lets other goroutines run (Phase 08).
				ok = false
			}
			if x.Tok == token.ASSIGN {
				for _, e := range []ast.Expr{x.Key, x.Value} {
					ok = ok && (e == nil || own(e))
				}
			}
		}
		return ok
	})
	return ok
}

// methodSel returns the selector of the call c if c calls a method (x.M(...)), and its
// selection, or nil.
func (fg *funcGen) methodSel(c *ast.CallExpr) (*ast.SelectorExpr, *types.Selection) {
	x, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	if s := fg.g.info.Selections[x]; s != nil && s.Kind() == types.MethodVal {
		return x, s
	}
	return nil, nil
}

// A recvPath is the way from the operand x of x.M to the receiver of the method M: the fields
// of the embedded types that M is promoted through, if any.
type recvPath struct {
	t       types.Type // the type of x
	path    []int      // the indexes of the embedded fields
	end     types.Type // the type at the end of the path: the receiver's base type, or a pointer to it
	ptrRecv bool       // M has a pointer receiver
	deref   bool       // a pointer is dereferenced on the way (an embedded pointer, or x itself)
	last    int        // the last step of path whose type is a pointer (-1 if none)
}

// pathOf returns the way to the receiver of the method selection s.
func pathOf(s *types.Selection) recvPath {
	idx := s.Index()
	r := recvPath{t: s.Recv(), path: idx[:len(idx)-1], last: -1}
	sig := s.Obj().Type().(*types.Signature)
	_, r.ptrRecv = types.Unalias(sig.Recv().Type()).(*types.Pointer)
	t := r.t
	for k, i := range r.path {
		if p, ok := under(t).(*types.Pointer); ok {
			r.deref, r.last = true, k
			t = p.Elem()
		}
		t = under(t).(*types.Struct).Field(i).Type()
	}
	r.end = t
	return r
}

// endPtr reports whether the end of the path is a pointer.
func (r recvPath) endPtr() bool { return isPointer(r.end) }

// analyzeMethod returns the order scope of the call c of a method (order.go): the receiver,
// then the arguments, in a scope that C++ evaluates in any order (the receiver of a member call
// is evaluated first in C++17, the one of a static call is an argument). Taking the receiver's
// address has the items of its address (collectAddr); a value method through a pointer, and a
// promotion through an embedded pointer, read memory through the pointer, which a call can
// change, and panic for nil: after the calls of the scope, as gc does.
func (fg *funcGen) analyzeMethod(c *ast.CallExpr, x *ast.SelectorExpr, s *types.Selection) *orderScope {
	// A value method is a member call, recv.M(args) or recv->M(args), whose object C++
	// evaluates first; a pointer method is a static call, T::PM(recv, args).
	// A deferred call's receiver is an argument of gp_push_call (deferThunk).
	sc := &orderScope{objFirst: !pathOf(s).ptrRecv && c != fg.deferCall}
	if _, stored := fg.recvs[x]; !stored {
		fg.collectRecv(sc, x, s, 0, nil)
		for _, it := range sc.items {
			it.recv = it.e == x // emitPanic checks *p for it, not the method value x
		}
	}
	for i, a := range c.Args {
		fg.collect(sc, a, i+1, nil)
	}
	sc.plan()
	return sc
}

// collectRecv adds the items of the receiver of x.M, a method call's or a method value's, to the
// scope (order.go), below the operators up: taking the receiver's address has the items of its
// address (collectAddr); a value method through a pointer, and a promotion through an embedded
// pointer, read memory through the pointer, which a call can change, and panic for nil.
func (fg *funcGen) collectRecv(sc *orderScope, x *ast.SelectorExpr, s *types.Selection, root int, up *orderStep) {
	r := pathOf(s)
	below := &orderStep{node: x, up: up}
	if r.ptrRecv && !r.endPtr() && !r.deref {
		fg.collectAddr(sc, x.X, root, below) // &x, &x.Inner: the address of an addressable x
		return
	}
	fg.collect(sc, x.X, root, below)
	if r.deref || !r.ptrRecv && r.endPtr() {
		sc.items = append(sc.items, &orderItem{e: x, root: root, up: up, eff: effect{read: true, panics: true}})
	}
}

// methodCall returns the C++ of the call c of a method, x.M(args) (see the top).
func (fg *funcGen) methodCall(c *ast.CallExpr, x *ast.SelectorExpr, s *types.Selection) cppir.Expr {
	fn := s.Obj().(*types.Func)
	r := pathOf(s)
	recv, ptr := fg.recvs[x], false // a deferred call's stored receiver (storeRecv)
	if recv == nil {
		recv, ptr = fg.recvExpr(x, r, true)
	}
	tuple := len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0]))
	var args []cppir.Expr
	var params []cppir.Param
	if tuple {
		// x.M(g()), where g has several results: std::apply calls a lambda with them, which
		// converts them to M's parameter types where Go does (step 3.9) and puts those of a
		// variadic parameter in a slice (step 3.11), as tupleArgs.
		params, args = fg.spreadTuple(c, fn.Signature())
	} else {
		args = fg.callArgs(c, fn.Signature()) // closures.go: variadics
	}
	call := fg.g.callMethod(recv, ptr, fn, r, args, c.Pos())
	if !tuple {
		return call
	}
	lambda := &cppir.Lambda{Default: "&", Params: params, Body: &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: call}}, Inline: true}}
	return &cppir.Call{Fun: cxxName("std::apply"), Args: []cppir.Expr{lambda, fg.expr(c.Args[0])}}
}

// callMethod returns the call of the method fn on recv, the receiver at the end of the path r:
// a pointer to it when ptr is set, else a value. A value method through a pointer is a member
// call through ->, which panics for nil; a pointer method is a static call with the pointer,
// T::PM(p, args).
func (g *gen) callMethod(recv cppir.Expr, ptr bool, fn *types.Func, r recvPath, args []cppir.Expr, pos token.Pos) cppir.Expr {
	if weakFunc(fn, "Value") {
		return weakValue(recv, ptr) // weak.go
	}
	base := r.end
	if p, ok := under(base).(*types.Pointer); ok {
		base = p.Elem()
	}
	if isInterface(base) {
		// A method of an interface value: the wrapper of its interface struct (ifaces.go).
		return &cppir.Call{Fun: &cppir.Member{X: recv, Name: g.ifaceMethodName(base, fn, pos)}, Args: args}
	}
	si := g.structOf(base, pos)
	if si == nil {
		return placeholder
	}
	member := g.methodName(si.name, fn.Name())
	if r.ptrRecv {
		owner := cppir.TypeString(g.structType(base, pos))
		return &cppir.Call{Fun: cxxName(owner + "::" + member), Args: append([]cppir.Expr{recv}, args...)}
	}
	return &cppir.Call{Fun: &cppir.Member{X: recv, Arrow: ptr, Name: member}, Args: args}
}

// recvExpr returns the C++ of the receiver of the method call x.M(...) at the end of the path r,
// and whether it is a pointer: for a pointer method always a pointer (the address of an
// addressable x, recvAddr), for a value method the value, or the pointer that -> dereferences
// (held by a copy, with hold, when it is not a local of the caller: see the top).
func (fg *funcGen) recvExpr(x *ast.SelectorExpr, r recvPath, hold bool) (cppir.Expr, bool) {
	if r.ptrRecv && !r.endPtr() {
		return fg.recvAddr(x, r), true
	}
	recv, _ := fg.fieldSteps(fg.expr(x.X), r.t, r.path, x.Pos())
	if !r.endPtr() {
		return recv, false
	}
	if hold && !r.ptrRecv && (len(r.path) > 0 || !fg.heldPtr(x.X)) {
		recv = construct(fg.g.cxxType(r.end, x.Pos()), recv)
	}
	return recv, true
}

// heldPtr reports whether the pointer e, the receiver of a value method, holds its object while
// the call runs without a copy: a local variable that is not boxed (nothing but an assignment
// changes it), a temporary of the order, or a value that only the call's full expression has (a
// call's result, &T{...}).
func (fg *funcGen) heldPtr(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok || fg.isLocal(e) {
		return true
	}
	switch x := e.(type) {
	case *ast.CallExpr:
		return !fg.g.info.Types[x.Fun].IsType()
	case *ast.UnaryExpr:
		_, lit := ast.Unparen(x.X).(*ast.CompositeLit)
		return x.Op == token.AND && lit
	}
	return false
}

// recvAddr returns the address of the receiver of a pointer method that x.M reaches by value:
// &x (the owner table, addr.go), or the address of the embedded field it is promoted from, one
// gp::field_ptr for each field after the last pointer on the path (as structs.go's fieldPtr).
func (fg *funcGen) recvAddr(x *ast.SelectorExpr, r recvPath) cppir.Expr {
	var p cppir.Expr
	t, start := r.t, 0
	if r.last < 0 {
		if len(r.path) == 0 || !isPointer(r.t) {
			p = fg.addrOf(x.X)
		} else {
			p = fg.expr(x.X)
		}
	} else {
		p, t = fg.fieldSteps(fg.expr(x.X), r.t, r.path[:r.last], x.Pos())
		start = r.last
	}
	return fg.g.fieldPtrs(p, t, r.path[start:], x.Pos())
}

// fieldPtrs returns gp::field_ptr(...gp::field_ptr(p, &T::f)...), the pointer to the fields at
// path from p, a pointer to a value of type t (or the type that t points to).
func (g *gen) fieldPtrs(p cppir.Expr, t types.Type, path []int, pos token.Pos) cppir.Expr {
	for _, i := range path {
		if pt, ok := under(t).(*types.Pointer); ok {
			t = pt.Elem()
		}
		member := &cppir.Unary{Op: "&", X: cxxName(cppir.TypeString(g.cxxType(t, pos)) + "::" + g.fieldName(t, i, pos))}
		p = call("gp::field_ptr", p, member)
		t = under(t).(*types.Struct).Field(i).Type()
	}
	return p
}

// storeRecv is deferThunk's part for a deferred method call (defer.go): it adds the receiver to
// the values that the defer statement evaluates now, as the lambda's parameter gp_a0 (a copy of
// the receiver for a value method, *p for one through a pointer, which panics there for a nil
// p; the pointer for a pointer method), and makes the call use it. The function it returns
// forgets it again.
func (fg *funcGen) storeRecv(c *ast.CallExpr, params *[]cppir.Param, values *[]cppir.Expr) func() {
	x, s := fg.methodSel(c)
	if x == nil {
		return func() {}
	}
	recv, ptr := fg.recvExpr(x, pathOf(s), false)
	if ptr && !pathOf(s).ptrRecv {
		recv = deref(recv) // the value, copied when the frame stores it
	}
	if isInterface(pathOf(s).end) {
		// defer i.M(): Go evaluates the method value there, which panics for a nil interface (not
		// when the deferred call runs).
		recv = call("gp::method_recv", recv)
	}
	*values = append(*values, recv)
	name := "gp_a" + strconv.Itoa(len(*params))
	*params = append(*params, cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Name: name})
	if fg.recvs == nil {
		fg.recvs = make(map[*ast.SelectorExpr]cppir.Expr)
	}
	fg.recvs[x] = cxxName(name)
	return func() { delete(fg.recvs, x) }
}

// methodValue returns the C++ of a method value x.M (not called) or a method expression T.M, a
// func value (step 3.11's gp::func, closures.go) made from a generic lambda, which the stack trace
// leaves out as Go leaves out its wrappers (main.T.M-fm, and main.(*T).M for (*T).M):
//
//	x.M (value method)    gp::func<Sig>([gp_r = x](auto&&... gp_a) { return gp_r.M(gp_a...); })
//	p.M (value method)    the same with gp_r = *p: a copy now, which panics now for a nil p
//	x.PM, p.PM            [gp_r = <&x>, p](auto&&... gp_a) { return T::PM(gp_r, gp_a...); }
//	T.M, (*T).PM          [](auto&& gp_r, auto&&... gp_a) { return gp_r.M(gp_a...); }, T::PM(...)
//	(*T).M                the same with gp_r->M(...), after Go's check: a nil gp_r panics with
//	                      "value method main.T.M called using nil *T pointer"
//
// The receiver of a method value is evaluated where the value is made, like a method call's
// receiver (order.go: collectRecv): its reads after the calls of its scope. A value receiver
// larger than 128 KiB is captured in a box (D24). A call of a method expression, T.M(x, a), is a
// call of its lambda, without a gp::func (closures.go: callValue). Promoted methods work the same
// way, through the embedded fields (recvFrom).
func (fg *funcGen) methodValue(x *ast.SelectorExpr, s *types.Selection) cppir.Expr {
	sig, _ := fg.g.info.TypeOf(x).(*types.Signature)
	if sig == nil {
		fg.g.errorf(x.Pos(), "internal error: a method value without a signature")
		return placeholder
	}
	lam := fg.methodLambda(x, s)
	if fn, _ := s.Obj().(*types.Func); fg.g.awareFunc(fn) || isIfaceMethod(fn) {
		// The method calls recover, or it is an interface's (its code is known when it runs): a
		// deferred call of this func value gives the ticket, which gp::icall takes away again
		// unless the method that runs is aware (recover.go).
		return construct(fg.g.funcType(sig, x.Pos()), call("gp::aware", lam))
	}
	return construct(fg.g.funcType(sig, x.Pos()), lam)
}

// methodLambda returns the generic lambda of the method value or method expression x
// (methodValue).
func (fg *funcGen) methodLambda(x *ast.SelectorExpr, s *types.Selection) *cppir.Lambda {
	fn := s.Obj().(*types.Func)
	r := pathOf(s)
	auto := &cppir.Ref{Elem: typeName("auto"), RValue: true}
	args := []cppir.Expr{&cppir.Expand{X: cxxName("gp_a")}}
	lam := &cppir.Lambda{Params: []cppir.Param{{Type: auto, Pack: true, Name: "gp_a"}}, Body: &cppir.Block{Inline: true}}
	recv := cppir.Expr(cxxName("gp_r"))
	ptr := false
	if s.Kind() == types.MethodExpr {
		// The receiver is the first parameter, of the type before the dot.
		lam.Params = append([]cppir.Param{{Type: auto, Name: "gp_r"}}, lam.Params...)
		if isPointer(r.t) && !r.ptrRecv && len(r.path) == 0 {
			// (*T).M of a value method: Go's wrapper checks the pointer (runtime.panicwrap). A
			// runtime call, so that -nochecks removes it with the other nil checks.
			base := r.end.(*types.Pointer).Elem().(*types.Named)
			msg := "value method " + wrapperName(base, true) + "." + fn.Name() +
				" called using nil *" + wrapperName(base, false) + " pointer"
			lam.Body.Stmts = append(lam.Body.Stmts, &cppir.ExprStmt{X: call("gp::check_wrap", recv, lit(cppir.CString(msg)))})
		}
		var ok bool
		recv, ptr, ok = fg.recvFrom(recv, r)
		if !ok {
			fg.g.errorf(x.Pos(), "internal error: the method expression %s has no pointer for its receiver", types.ExprString(x))
		}
	} else {
		// A method value: the receiver now, a copy of a value (*p for a value method through a
		// pointer: nil panics here), or the pointer for a pointer method.
		value, isPtr := fg.recvExpr(x, r, false)
		if isPtr && !r.ptrRecv {
			value = deref(value)
		}
		if isInterface(r.end) {
			// f := i.M binds the interface value now: Go's method value of a nil interface
			// panics here, not where f is called (step 5.9).
			value = call("gp::method_recv", value)
		}
		if !r.ptrRecv {
			if base := recvBase(r); largeVar(base) {
				typ := fg.g.cxxType(base, x.Pos())
				value = newBox(typ, value) // a large copy goes into a box (D24)
				recv = &cppir.Paren{X: deref(recv)}
			}
		}
		lam.Captures = []cppir.Capture{{Name: "gp_r", Init: value}}
		ptr = r.ptrRecv
	}
	lam.Body.Stmts = append(lam.Body.Stmts, &cppir.Return{X: fg.g.callMethod(recv, ptr, fn, r, args, x.Pos())})
	return lam
}

// wrapperName returns the name of the named type n as Go's wrapper for (*T).M writes it in its
// nil panic (runtime.panicwrap): main.T (with the package when pkg is set), or for an
// instantiated generic type its type arguments as [...]: weak.Pointer[...], Pointer[...].
func wrapperName(n *types.Named, pkg bool) string {
	name := n.Obj().Name()
	if pkg && n.Obj().Pkg() != nil {
		name = n.Obj().Pkg().Name() + "." + name // as goTypeName names it
	}
	if n.TypeArgs().Len() > 0 {
		name += "[...]"
	}
	return name
}

// methodExpr returns e as a method expression (T.M, (*T).PM) and its selection, or nil.
func (fg *funcGen) methodExpr(e ast.Expr) (*ast.SelectorExpr, *types.Selection) {
	x, ok := ast.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	if s := fg.g.info.Selections[x]; s != nil && s.Kind() == types.MethodExpr {
		return x, s
	}
	return nil, nil
}

// recvBase returns the receiver's base type at the end of the path r: T for T or *T.
func recvBase(r recvPath) types.Type {
	if p, ok := under(r.end).(*types.Pointer); ok {
		return p.Elem()
	}
	return r.end
}

// recvFrom returns the receiver of the method at the end of the path r from base, the C++ of a
// value of type r.t (a struct value or a pointer to one), and whether it is a pointer: for a
// pointer method of a value at the end of the path, its address, from the last pointer on the
// path (base itself, or an embedded pointer) with one gp::field_ptr for each field after it; else
// the fields of the path. It reports false if the address needs a pointer that the path doesn't
// have (an addressable variable is fg.recvAddr's).
func (fg *funcGen) recvFrom(base cppir.Expr, r recvPath) (cppir.Expr, bool, bool) {
	if r.ptrRecv && !r.endPtr() {
		p, t, start := base, r.t, 0
		switch {
		case r.last >= 0:
			p, t = fg.fieldSteps(base, r.t, r.path[:r.last], token.NoPos)
			start = r.last
		case !isPointer(r.t):
			return placeholder, true, false
		}
		return fg.g.fieldPtrs(p, t, r.path[start:], token.NoPos), true, true
	}
	recv, _ := fg.fieldSteps(base, r.t, r.path, token.NoPos)
	return recv, r.endPtr(), true
}

// A hook is one of the fmt shim's members for a method of the method set of a struct
// (gocpp/fmtshim.hpp): gp_String or gp_Error, or for a method that only the pointer's method
// set has, the static gp_String_ptr or gp_Error_ptr.
type hook struct {
	name string // gp_String, gp_String_ptr, ...
	ptr  bool   // static, with the pointer
	sel  *types.Selection
}

// param returns the parameter of a pointer hook of si: const gp::ptr<T>& gp_p.
func (h hook) param(si *structInfo) cppir.Param {
	return cppir.Param{Type: &cppir.Ref{Elem: &cppir.Const{Elem: ptrType(typeName(si.name))}}, Name: "gp_p"}
}

// hooks returns the hooks of the struct si: for Error() string, then String() string, the one of
// T's method set, else the one of *T's (promoted methods too). A method of another signature,
// or a field of that name, is none (fmt doesn't call it either).
func (g *gen) hooks(si *structInfo) []hook {
	var hs []hook
	vset := types.NewMethodSet(si.t)
	pset := types.NewMethodSet(types.NewPointer(si.t))
	for _, name := range []string{"Error", "String"} {
		if s := vset.Lookup(g.pkg.Types, name); s != nil && stringer(s) {
			hs = append(hs, hook{name: "gp_" + name, sel: s})
		} else if s := pset.Lookup(g.pkg.Types, name); s != nil && stringer(s) {
			hs = append(hs, hook{name: "gp_" + name + "_ptr", ptr: true, sel: s})
		}
	}
	return hs
}

// stringer reports whether the method of the selection s is func() string, as fmt's Stringer
// and error interfaces have it.
func stringer(s *types.Selection) bool {
	sig, ok := s.Obj().Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	return types.Identical(sig.Results().At(0).Type(), types.Typ[types.String])
}

// hookDefs returns the definitions of the hooks of si (see the top), out of line after all the
// structs: a promoted method may be reached through an embedded pointer to a struct that is
// defined after si. Each calls the method as Go's fmt would through the interface:
//
//	gp::string Celsius::gp_String() const { return String(); }
//	gp::string T::gp_Error_ptr(const gp::ptr<T>& gp_p) { return T::Error(gp_p); }
func (g *gen) hookDefs(si *structInfo) []cppir.Decl {
	var decls []cppir.Decl
	defer g.hideMembers(g.memberNames(si))() // they are member functions
	for _, h := range g.hooks(si) {
		fn := h.sel.Obj().(*types.Func)
		r := pathOf(h.sel)
		var recv cppir.Expr
		ptr := false
		d := &cppir.Func{Result: typeName("gp::string"), Name: si.name + "::" + h.name}
		fg := newFuncGen(g, g.newFuncNames(), nil)
		var base cppir.Expr // where the path starts: the pointer gp_p (to *T), or *this (a T)
		if h.ptr {
			d.Params = []cppir.Param{h.param(si)}
			base = cxxName("gp_p")
		} else {
			d.Quals = []string{"const"}
			if len(r.path) == 0 && !r.ptrRecv {
				// The struct's own value method: String().
				member := g.methodName(si.name, fn.Name())
				d.Body = &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: call(member)}}}
				decls = append(decls, d, &cppir.Blank{})
				continue
			}
			base = deref(lit("this")) // promoted through the embedded fields of *this
		}
		// A pointer method of a value at the end of the path takes its address from the last
		// pointer on the path: gp_p itself, or an embedded pointer, which T's value method set
		// needs for a pointer method (Out{*Mid} with Mid{In} and (*In).String).
		var ok bool
		recv, ptr, ok = fg.recvFrom(base, r)
		if !ok {
			g.errorf(token.NoPos, "internal error: the hook %s of %s has no pointer for its receiver", h.name, si.name)
			return nil
		}
		x := g.callMethod(recv, ptr, fn, r, nil, token.NoPos)
		d.Body = &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: x}}}
		decls = append(decls, d, &cppir.Blank{})
	}
	return decls
}
