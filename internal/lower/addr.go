package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Taking addresses: the owner table (Phase 03 README §2.2). Every pointer shares the owner of
// the block it points into (ptr.hpp's invariant), so it can't dangle:
//
//	Go                       owner                    C++
//	&x (x is boxed)          its box                  x: the box is the pointer
//	&global                  the static owner         gp::static_ptr(global)
//	&*p                      p's                      gp::ptr<T>(p, &*p): p, after its nil check
//	new(T)                   a new block              gp::new_<T>()
//
// The rest of the table comes with the types it needs, each a pointer that shares the owner of
// the object it points into (gp::ptr's aliasing constructor):
//
//	&p.f (p is *T)           p                        gp::ptr<F>(p, &p->f)            step 3.8
//	&x.f, &x.arr[i]          x's box                  gp::ptr<F>(x, &(*x).f)          steps 3.6, 3.8
//	&global.f, &garr[i]      the static owner         gp::static_ptr(global.f)        steps 3.6, 3.8
//	&s[i] (s is a slice)     the slice's array        gp::ptr<T>(s.gp_owner(), &s[i]) step 3.5b
//	&T{...}                  a new block              gp::new_<T>(T{...})             step 3.8
//
// The owner is evaluated once, so an owner that is not a variable (&f().x, &(*pp).f) must go
// into a temporary first. The boxing analysis (box.go) boxes every local variable whose address,
// or the address of a part of it, is taken, so &x never needs the address of a C++ stack
// variable.

// addr returns the C++ of &x.
func (fg *funcGen) addr(e *ast.UnaryExpr) cppir.Expr {
	switch x := ast.Unparen(e.X).(type) {
	case *ast.Ident:
		v, ok := fg.g.info.Uses[x].(*types.Var)
		switch {
		case !ok:
		case isPkgVar(v):
			return call("gp::static_ptr", cxxName(fg.cname(v, x.Pos())))
		case fg.isBoxed(v):
			return cxxName(fg.cname(v, x.Pos())) // the box
		}
		fg.g.errorf(e.Pos(), "internal error: the address of %s, which is not boxed", x.Name)
		return placeholder
	case *ast.StarExpr:
		// &*p is p, but Go's &*p panics for a nil p, as *p does: the aliasing constructor
		// gets p and &*p. p is evaluated twice, so it must be a variable.
		if id, ok := ast.Unparen(x.X).(*ast.Ident); ok {
			if _, ok := fg.g.info.Uses[id].(*types.Var); ok {
				p := fg.expr(x.X)
				typ := fg.g.cxxType(fg.g.info.TypeOf(e), e.Pos())
				if tn, ok := typ.(*cppir.TypeName); ok {
					return &cppir.Call{Fun: &cppir.Name{Name: tn.Name, Args: tn.Args}, Args: []cppir.Expr{p, &cppir.Unary{Op: "&", X: deref(p)}}}
				}
				return placeholder
			}
		}
		fg.g.unsupported(e.Pos(), "&*p where p is not a variable")
	case *ast.CompositeLit:
		fg.g.unsupported(e.Pos(), "&T{...} (Phase 03 step 3.8)")
	case *ast.SelectorExpr:
		fg.g.unsupported(e.Pos(), "the address of a field (Phase 03 step 3.8)")
	case *ast.IndexExpr:
		fg.g.unsupported(e.Pos(), "the address of an element (Phase 03 steps 3.5 and 3.6)")
	default:
		fg.g.unsupported(e.Pos(), "the address of "+nodeKind(x))
	}
	return placeholder
}
