package lower

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// The C++ of lending: how a call passes its pointer arguments and its receiver to the borrowed
// parameters of the callee (borrow.go has the analysis of the callee, ADR-086).
//
// The callee's parameter is a gp::bptr<T>, a raw pointer, so the object must stay alive until the
// call returns. The call site makes sure of that, in one of three ways, from the cheapest:
//
//   - the argument is a temporary (a call's result, &T{...}), or a variable that nothing can
//     change while the call runs (a local that is not boxed, a borrowed parameter, an order
//     temporary): gp::lend(x), or the temporary itself. A boxed variable, a field, an element, a
//     package variable and anything else that a call can change is not one of these;
//   - the address of a part of an object that is alive for the same reason (&p.f, the receiver
//     p.Embedded.M()): gp::borrow(&p->f), a raw address, without the aliasing pointer that
//     gp::field_ptr would make;
//   - the callee is pure (borrow.go): while it runs, nothing can drop an owner, so a pointer
//     that the caller loads from a field, an element or a global stays valid. The other
//     arguments must not run code first (a call in them could drop it), see noCalls.
//
// Otherwise the argument is copied (gp::ptr<T>(x), converting to the temporary the callee
// borrows), which is what a call did before.
//
// A parameter that is a const reference (borrow.go: ref) is passed the same way with the weaker
// rule of refStable, because a reference does not need an owner of its own: the argument as it
// is when it is a temporary, a constant or a local that no call can change, or when the callee is
// pure for values; else a copy.

// borrowedType returns gp::bptr<X> for the owning pointer type gp::ptr<X>.
func borrowedType(t cppir.Type) cppir.Type {
	if tn, ok := t.(*cppir.TypeName); ok && tn.Name == "gp::ptr" {
		return &cppir.TypeName{Name: "gp::bptr", Args: tn.Args}
	}
	return t
}

// lendCall returns gp::lend(x).
func lendCall(x cppir.Expr) cppir.Expr { return call("gp::lend", x) }

// borrowAddr returns gp::borrow(&x): the address of the lvalue x as a borrowed pointer.
func borrowAddr(x cppir.Expr) cppir.Expr {
	return call("gp::borrow", &cppir.Unary{Op: "&", X: x})
}

// valueStable reports whether the pointer (or slice) value of e is the same, and its object
// alive, until the call that e is an argument of returns: a temporary, or a variable that no
// call can change.
func (fg *funcGen) valueStable(e ast.Expr) bool {
	e = ast.Unparen(e)
	if _, ok := fg.temps[e]; ok {
		return true
	}
	switch x := e.(type) {
	case *ast.CallExpr:
		return !fg.g.info.Types[x.Fun].IsType()
	case *ast.Ident:
		v, ok := fg.g.info.Uses[x].(*types.Var)
		if !ok {
			return false
		}
		if _, ok := fg.heldPtrs[v]; ok {
			return true // a deferred call's own copy
		}
		return fg.isLocal(x)
	case *ast.UnaryExpr:
		if x.Op != token.AND {
			return false
		}
		inner := ast.Unparen(x.X)
		if _, lit := inner.(*ast.CompositeLit); lit {
			return true
		}
		return fg.storageStable(inner)
	}
	return false
}

// storageStable reports whether the storage of the addressable expression e stays alive until
// the call returns: a variable (a boxed one is a box that the frame holds, a package variable
// never goes), or a field or an element reached from stable storage or through a pointer or a
// slice whose value is stable. A pointer loaded from a field on the way is not stable.
func (fg *funcGen) storageStable(e ast.Expr) bool {
	e = ast.Unparen(e)
	switch x := e.(type) {
	case *ast.Ident:
		_, ok := fg.g.info.Uses[x].(*types.Var)
		return ok
	case *ast.StarExpr:
		return fg.valueStable(x.X)
	case *ast.SelectorExpr:
		sel := fg.fieldSel(x)
		if sel == nil {
			return false
		}
		return fg.pathStable(x.X, sel.Recv(), sel.Index())
	case *ast.IndexExpr:
		xt := fg.g.info.TypeOf(x.X)
		switch {
		case isArray(xt):
			return fg.storageStable(x.X)
		case isSlice(xt) || arrayPtr(xt) != nil:
			return fg.valueStable(x.X)
		}
	}
	return false
}

// pathStable reports whether the fields at index from x (of the type t), a struct or a pointer
// to one, are alive until the call returns: x is stable storage (a struct) or a stable pointer,
// and the path goes through no pointer after its first step (an embedded pointer is loaded from
// memory).
func (fg *funcGen) pathStable(x ast.Expr, t types.Type, index []int) bool {
	for k, i := range index {
		if p, ok := under(t).(*types.Pointer); ok {
			if k > 0 {
				return false
			}
			t = p.Elem()
		}
		st, ok := under(t).(*types.Struct)
		if !ok {
			return false
		}
		t = st.Field(i).Type()
	}
	if isPointer(fg.g.info.TypeOf(x)) {
		return fg.valueStable(x)
	}
	return fg.storageStable(x)
}

// foreignWork reports whether evaluating or copying any expression can run implicit C++ code.
// A scalar result can hide such work in its operands, as in len([]Foreign{foreign}).
func (fg *funcGen) foreignWork(es ...ast.Expr) bool {
	foreign := false
	for _, e := range es {
		if e == nil {
			continue
		}
		ast.Inspect(e, func(n ast.Node) bool {
			x, ok := n.(ast.Expr)
			if !ok {
				return !foreign
			}
			if _, hoisted := fg.temps[x]; hoisted {
				return false
			}
			if id, ok := x.(*ast.Ident); ok {
				switch fg.g.info.ObjectOf(id).(type) {
				case *types.PkgName, *types.Builtin, *types.Label:
					return false
				}
			}
			foreign = foreign || fg.g.borrowHasC(fg.g.info.TypeOf(x))
			return !foreign
		})
		if foreign {
			return true
		}
	}
	return false
}

// noCalls reports whether nothing can run other code while the arguments are evaluated.
// Besides explicit calls, implicit foreign copies, constructors and conversions can drop owners.
func (fg *funcGen) noCalls(es ...ast.Expr) bool {
	if fg.foreignWork(es...) {
		return false
	}
	ok := true
	for _, e := range es {
		if e == nil {
			continue
		}
		ast.Inspect(e, func(n ast.Node) bool {
			if x, isExpr := n.(ast.Expr); isExpr {
				if _, hoisted := fg.temps[x]; hoisted {
					return false // evaluated before the call, into a temporary (order.go)
				}
			}
			switch x := n.(type) {
			case *ast.CallExpr:
				if fg.g.info.Types[x.Fun].IsType() {
					return true
				}
				for _, name := range []string{"len", "cap", "min", "max"} {
					if fg.isBuiltin(x, name) {
						return true
					}
				}
				ok = false
			case *ast.FuncLit:
				ok = false
			case *ast.UnaryExpr:
				if x.Op == token.ARROW {
					ok = false
				}
			}
			return ok
		})
	}
	return ok
}

// lendArgs adapts the C++ arguments of the call c of the function or method fn to its borrowed
// parameters: the argument at a borrowed position is lent (lendArg). recv is the receiver
// expression of a method call (it is evaluated with the arguments). args are the C++ of c.Args,
// as callArgs made them.
func (fg *funcGen) lendArgs(c *ast.CallExpr, fn *types.Func, recv ast.Expr, args []cppir.Expr) []cppir.Expr {
	s := fg.g.borrowOf(fn)
	if s == nil || !s.hasBorrowed() && !s.hasRef() || len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		return args
	}
	nfixed := len(args) // the arguments that stand for one Go argument each
	if len(args) != len(c.Args) {
		// A variadic call packs its trailing arguments in one slice.
		if !fn.Signature().Variadic() || len(args) != fn.Signature().Params().Len() {
			return args
		}
		nfixed = len(args) - 1
	}
	noCalls := fg.noCalls(append([]ast.Expr{recv}, c.Args...)...)
	safe := s.pure && noCalls
	safeV := s.pureV && noCalls
	sig := fn.Signature()
	macro := fg.g.macroSig(fn)
	for i := range args {
		if i >= sig.Params().Len() || i >= nfixed || viewIndex(macro, i) {
			continue // (the hidden string arguments of a macro are gp::static_view values: viewArgs)
		}
		// An argument loaded from a package variable that the program assigns is copied for the
		// call even for a pure callee: another goroutine may swap it (hotLoad).
		safe, safeV := safe && !fg.hotLoad(c.Args[i]), safeV && !fg.hotLoad(c.Args[i])
		var how lendHow
		switch {
		case s.borrowed(i):
			args[i], how = fg.lendArg(c.Args[i], args[i], safe, sig.Params().At(i).Type())
		case s.isRef(i):
			args[i], how = fg.refArg(c.Args[i], args[i], safeV, sig.Params().At(i).Type())
		default:
			continue
		}
		fg.g.noteLend(c.Args[i].Pos(), fn, how)
	}
	return args
}

// refArg returns the C++ of the argument a for a const reference parameter of the Go type pt: the
// default C++ def itself when nothing can change the argument while the call runs (a temporary, a
// constant, a local that is not boxed), or the callee is pure for values and nothing evaluated
// before the call runs code (safe); else a copy, which the reference binds to and which lives
// until the end of the call.
func (fg *funcGen) refArg(a ast.Expr, def cppir.Expr, safe bool, pt types.Type) (cppir.Expr, lendHow) {
	if fg.refStable(a) {
		return def, lendTemp
	}
	if safe {
		return def, lendPure
	}
	return fg.keep(pt, def), lendCopy
}

// refStable reports whether the value of e is a temporary, a constant or a variable that no call
// can change: a const reference to it is valid and unchanged until the call returns.
func (fg *funcGen) refStable(e ast.Expr) bool {
	e = ast.Unparen(e)
	if tv, ok := fg.g.info.Types[e]; ok && (tv.Value != nil || tv.IsNil()) {
		return true
	}
	if _, ok := fg.temps[e]; ok {
		return true
	}
	switch x := e.(type) {
	case *ast.CompositeLit, *ast.FuncLit, *ast.BinaryExpr, *ast.SliceExpr, *ast.BasicLit:
		return true // new values
	case *ast.UnaryExpr:
		return x.Op != token.ARROW // &x and the operators make a new value; a receive blocks
	case *ast.CallExpr:
		if fg.g.info.Types[x.Fun].IsType() {
			return len(x.Args) == 1 && fg.refStable(x.Args[0]) // (a conversion may be its operand)
		}
		return true
	case *ast.Ident:
		if v, ok := fg.g.info.Uses[x].(*types.Var); ok {
			if _, held := fg.heldPtrs[v]; held {
				return true
			}
			return fg.isLocal(x)
		}
	}
	return false
}

// How a call lends an argument or a receiver, for the reason log.
type lendHow int

const (
	lendTemp lendHow = iota // a temporary or a variable no call can change
	lendRaw                 // the raw address of a part of an object that stays alive
	lendPure                // a pointer loaded from memory, to a callee that drops no owner
	lendCopy                // the call keeps its own reference
	lendBox                 // a box that the frame holds
	lendNil                 // nil
)

// lendArg returns the C++ of the argument a of a call, whose default C++ is def (an owning
// pointer), for a borrowed parameter of the Go type pt (see the top). safe says that the callee
// is pure and nothing evaluated before the call can drop an owner.
func (fg *funcGen) lendArg(a ast.Expr, def cppir.Expr, safe bool, pt types.Type) (cppir.Expr, lendHow) {
	a = ast.Unparen(a)
	if tv := fg.g.info.Types[a]; tv.IsNil() {
		return def, lendNil // nullptr
	}
	if _, ok := fg.temps[a]; ok {
		return lendCall(def), lendTemp
	}
	if u, ok := a.(*ast.UnaryExpr); ok && u.Op == token.AND {
		x := ast.Unparen(u.X)
		if _, lit := x.(*ast.CompositeLit); lit {
			return def, lendTemp // a new object: the temporary lives to the end of the call
		}
		if id, ok := x.(*ast.Ident); ok {
			if v, ok := fg.g.info.Uses[id].(*types.Var); ok && !isPkgVar(v) && fg.isBoxed(v) {
				return lendCall(def), lendBox // &x of a boxed variable is its box, which the frame holds
			}
		}
		if fg.storageStable(x) || safe && fg.addressable(x) {
			if raw, ok := fg.rawAddr(x); ok {
				if fg.storageStable(x) {
					return raw, lendRaw
				}
				return raw, lendPure
			}
		}
		return fg.keep(pt, def), lendCopy
	}
	if _, ok := a.(*ast.CallExpr); ok && fg.valueStable(a) {
		return def, lendTemp // a temporary
	}
	if fg.valueStable(a) {
		return lendCall(def), lendTemp
	}
	if safe {
		return lendCall(def), lendPure
	}
	return fg.keep(pt, def), lendCopy
}

// keep returns a copy of the owning pointer def, of the Go pointer type pt, that lives until the
// end of the call: a temporary that the callee borrows.
func (fg *funcGen) keep(pt types.Type, def cppir.Expr) cppir.Expr {
	if c, ok := def.(*cppir.Call); ok {
		if n, ok := c.Fun.(*cppir.Name); ok && (n.Name == "gp::ptr" || n.Name == "gp::field_ptr") {
			return def // already an owning pointer made here (an aliasing gp::ptr<T>(owner, &part), gp::field_ptr)
		}
	}
	return construct(fg.g.cxxType(pt, token.NoPos), def)
}

// rawAddr returns the C++ of &x as a borrowed pointer, for the addressable expression x: the
// address of the C++ lvalue, where a nil pointer on the way panics as the access does.
func (fg *funcGen) rawAddr(x ast.Expr) (cppir.Expr, bool) {
	if !fg.addressable(x) || cObject(fg.g.info.TypeOf(x)) || !complete(fg.g.info.TypeOf(x)) {
		return nil, false
	}
	return borrowAddr(fg.expr(x)), true
}

// lendRecv returns the receiver of the call x.M(...) of a pointer method that borrows its
// receiver. def is the receiver as methodCall made it (an owning pointer, or its address), r the
// path to it. The receiver is the pointer at the end of the path, or the address of the value
// there. safe says that the callee is pure and nothing evaluated before the call can drop an
// owner (lendArg).
func (fg *funcGen) lendRecv(x *ast.SelectorExpr, r recvPath, def cppir.Expr, safe bool) (cppir.Expr, lendHow) {
	pt := types.NewPointer(recvBase(r))
	if r.endPtr() {
		// The pointer at the end of the path: x.X itself, or loaded from an embedded field.
		if len(r.path) == 0 {
			return fg.lendArg(x.X, def, safe, pt)
		}
		if safe {
			return lendCall(def), lendPure
		}
		return fg.keep(pt, def), lendCopy
	}
	// The address of the value at the end of the path.
	if id, ok := ast.Unparen(x.X).(*ast.Ident); ok && len(r.path) == 0 {
		if v, ok := fg.g.info.Uses[id].(*types.Var); ok && !isPkgVar(v) && fg.isBoxed(v) {
			return lendCall(def), lendBox // the box of the variable, which the frame holds
		}
	}
	if stable := fg.pathStable(x.X, r.t, r.path); safe || stable {
		inner, _ := fg.fieldSteps(fg.expr(x.X), r.t, r.path, x.Pos())
		if stable {
			return borrowAddr(inner), lendRaw
		}
		return borrowAddr(inner), lendPure
	}
	return fg.keep(pt, def), lendCopy
}

// lendFrom is lendRecv for a receiver that is computed from base, the C++ of a value of type r.t
// (a struct value or a pointer to one): in a forwarder (base is its own parameter or *this) or
// in the lambda of a method value or expression. stable says that base, and what it points
// into, stays alive until the call returns. It reports false when the receiver can only be
// made as an owning pointer (the caller falls back to that).
func (fg *funcGen) lendFrom(base cppir.Expr, r recvPath, stable, safe bool, pt types.Type) (cppir.Expr, bool) {
	safe = safe && !fg.g.threads().hotFields(r.t, r.path)
	hops := false // a pointer is loaded on the way (after the first step)
	t := r.t
	for k, i := range r.path {
		if p, ok := under(t).(*types.Pointer); ok {
			if k > 0 {
				hops = true
			}
			t = p.Elem()
		}
		st, ok := under(t).(*types.Struct)
		if !ok {
			return nil, false
		}
		t = st.Field(i).Type()
	}
	inner, _ := fg.fieldSteps(base, r.t, r.path, token.NoPos)
	if r.endPtr() {
		switch {
		case len(r.path) == 0 && stable, safe:
			return lendCall(inner), true
		}
		return fg.keep(pt, inner), true
	}
	if stable && !hops || safe {
		return borrowAddr(inner), true
	}
	return nil, false
}

// refRecv returns the receiver of the call x.M(...) of a pointer method whose receiver is a const
// reference (borrow.go): the pointer def as it is when it is a new pointer (an address) or a
// variable that no call can change, else a copy unless the callee is pure for values and
// nothing evaluated before the call runs code (safe).
func (fg *funcGen) refRecv(x *ast.SelectorExpr, r recvPath, def cppir.Expr, safe bool) (cppir.Expr, lendHow) {
	pt := types.NewPointer(recvBase(r))
	if !r.endPtr() {
		return def, lendTemp // &x...: a new pointer, or the box of a variable that the frame holds
	}
	if len(r.path) == 0 {
		return fg.refArg(x.X, def, safe, pt)
	}
	return fg.refFrom(r, def, safe)
}

// refFrom is refRecv for a receiver whose base is already held by an adapter or forwarder.
// Holding the outer object does not freeze an embedded pointer: the callee can replace that
// field before reading its const-reference receiver, so snapshot it unless the callee is pure.
func (fg *funcGen) refFrom(r recvPath, def cppir.Expr, safe bool) (cppir.Expr, lendHow) {
	if !r.endPtr() || len(r.path) == 0 {
		return def, lendTemp // a new address or the adapter's own receiver parameter
	}
	if safe && !fg.g.threads().hotFields(r.t, r.path) {
		return def, lendPure
	}
	return fg.keep(types.NewPointer(recvBase(r)), def), lendCopy
}
