package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Callbacks (Phase 04 README §2.8, step 4.11; the runtime is gocpp/callback.hpp). A gocpp func
// that goes to C/C++ code (an argument of a call of a C/C++ function, the value of a C/C++
// variable or member, an element of a C/C++ literal) is wrapped in a gp::cgo::callback, which
// converts to a C function pointer when the func has no state, and is a callable for C++
// templates, std::function and gp::func:
//
//	f (a function of the package)         gp::cgo::fn<&gpkg_main::f>()
//	func(x C.int) C.int { ... } (no       gp::cgo::fn_lit<gp::func<int(int)>>(lambda)
//	  captured variables)
//	func(...) { ... n ... } (captures),   gp::cgo::fn_val(gp::func<...>(lambda))
//	  or any other func value: g, a.f, h()
//
// nil stays nullptr (cValueOf). The places that are not call arguments (an assignment to a C/C++
// variable or member, a declaration with a value, a literal's element) are found before lowering
// (markCValues): g.cFuncs holds the func values that go there, and value writes them this way.

// callbackValue returns the C++ of the func value e where C/C++ code takes it (cValueOf).
//
// A function of this package (an *ast.Ident) and a function of another gocpp package of the program
// (pkg.F) are named directly: a plain function pointer, which C accepts too. An instance of a
// generic function (Phase 07) goes through fn_val: it is a C++ callable, and an error as a
// function pointer.
func (fg *funcGen) callbackValue(e ast.Expr) cppir.Expr {
	e = ast.Unparen(e)
	switch x := e.(type) {
	case *ast.Ident:
		if fn, ok := fg.g.info.Uses[x].(*types.Func); ok && fn.Signature().Recv() == nil {
			addr := &cppir.Unary{Op: "&", X: cxxName(fg.funcName(fn, x.Pos()))}
			return templateCall("gp::cgo::fn", []cppir.Node{addr})
		}
	case *ast.SelectorExpr:
		if fn := fg.g.qualifiedFunc(x); fn != nil && fn.Signature().Recv() == nil {
			addr := &cppir.Unary{Op: "&", X: cxxName(fg.funcName(fn, x.Pos()))}
			return templateCall("gp::cgo::fn", []cppir.Node{addr})
		}
	case *ast.FuncLit:
		sig, _ := fg.g.info.TypeOf(x).(*types.Signature)
		if sig == nil {
			break
		}
		lam := fg.lambda(x)
		ft := fg.g.funcType(sig, x.Pos())
		if len(lam.Captures) == 0 {
			return templateCall("gp::cgo::fn_lit", []cppir.Node{ft}, lam)
		}
		return call("gp::cgo::fn_val", construct(ft, lam))
	}
	// Any other func value: g, a.f, h(). fg.expr would find e marked and come back here.
	if fg.g.cFuncs[e] {
		fg.g.cFuncs[e] = false
		defer func() { fg.g.cFuncs[e] = true }()
	}
	return call("gp::cgo::fn_val", fg.expr(e))
}

// isCallback reports whether e is a gocpp func value (not nil).
func (fg *funcGen) isCallback(e ast.Expr) bool {
	tv := fg.g.info.Types[e]
	return tv.Type != nil && !tv.IsNil() && isFunc(tv.Type)
}

// cTarget reports whether the call c calls C/C++ code: a C/C++ function or constructor, a member
// of a C/C++ object, a C++ callable.
func (fg *funcGen) cTarget(c *ast.CallExpr) bool {
	if _, ok := fg.g.cRefOf(c.Fun); ok {
		return true
	}
	return fg.g.opaqueCallee(c) || fg.g.opaqueMember(c.Fun) != nil
}
