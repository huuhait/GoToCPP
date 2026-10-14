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
//	&s[i] (s is a slice)     the slice's array        gp::ptr<T>(s.gp_owner(), &s[i]) (elemAddr)
//	&f()[i] (not a variable) the slice's array        gp::elem_ptr(f(), i)
//
// Arrays (step 3.6, arrays.go: addrOf), also nested (&a[i][j]) and for a[lo:hi], which slices
// the array that a pointer to it points into:
//
//	&a[i] (a is boxed)       a's box                  gp::ptr<T>(a, &(*a)[i])
//	&garr[i]                 the static owner         gp::static_ptr(garr[i])
//	&p[i] (p is *[N]T)       p                        gp::ptr<T>(p, &(*p)[i])
//	&s[j][i] (s is [][N]T)   the slice's array        gp::ptr<T>(s.gp_owner(), &s[j][i])
//	&f()[i] (f() is *[N]T)   the array's block        gp::elem_ptr(f(), i)
//	&[N]T{...}               a new block              gp::new_<A>(A{...})
//
// Structs (step 3.8, structs.go), with the same addrOf, each a pointer that shares the owner of
// the object it points into (gp::ptr's aliasing constructor):
//
//	&p.f (p is *T)           p                        gp::ptr<F>(p, &p->f)
//	&x.f, &x.arr[i].g        x's box                  gp::ptr<F>(x, &(*x).f)
//	&global.f                the static owner         gp::static_ptr(global.f)
//	&s[i].f                  the slice's array        gp::ptr<F>(s.gp_owner(), &s[i].f)
//	&f().f (f() is *T)       f()'s block              gp::field_ptr(f(), &T::f)
//	&T{...}                  a new block              gp::new_<T>(T{...})
//
// The aliasing form reads the owner twice, so it is for an owner that is a variable (or in a
// temporary); another one goes through gp::elem_ptr or gp::field_ptr, which evaluate it once. The boxing analysis (box.go) boxes every local variable whose address,
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
				return construct(typ, p, &cppir.Unary{Op: "&", X: deref(p)})
			}
		}
		fg.g.unsupported(e.Pos(), "&*p where p is not a variable")
	case *ast.CompositeLit:
		if t := fg.g.info.TypeOf(x); isArray(t) || isStruct(t) {
			// &[N]T{...}, &T{...}: a new block that holds the literal, gp::new_<A>(A{...}); one
			// larger than 128 KiB is built in the block itself (largeBox, arrays.go).
			if largeVar(t) {
				return largeBox(fg.g.cxxType(t, x.Pos()), fg.expr(x))
			}
			return newBox(fg.g.cxxType(t, x.Pos()), fg.expr(x))
		}
		fg.g.unsupported(e.Pos(), "&T{...} of a slice or map literal")
	case *ast.SelectorExpr:
		return fg.addrOf(x) // a field: arrays.go and structs.go
	case *ast.IndexExpr:
		if isSlice(fg.g.info.TypeOf(x.X)) {
			return fg.elemAddr(e, x)
		}
		return fg.addrOf(x) // an element of an array (arrays.go)
	default:
		fg.g.unsupported(e.Pos(), "the address of "+nodeKind(x))
	}
	return placeholder
}

// elemAddr returns the C++ of &s[i], for a slice s: a pointer that shares the owner of the
// slice's array, gp::ptr<T>(s.gp_owner(), &s[i]). That evaluates s twice, so it is only for an
// s that is a variable (or in a temporary, order.go): reading it twice is reading it once,
// since nothing between the two reads can change it. Any other s, &f()[i] or &ss[j][i], is
// gp::elem_ptr(s, i), which evaluates it once. An index out of range panics with Go's message
// (s[i]'s check).
func (fg *funcGen) elemAddr(e *ast.UnaryExpr, x *ast.IndexExpr) cppir.Expr {
	s := ast.Unparen(x.X)
	_, variable := fg.temps[s]
	if id, ok := s.(*ast.Ident); ok {
		_, variable = fg.g.info.Uses[id].(*types.Var)
	}
	if !variable {
		return call("gp::elem_ptr", fg.expr(s), fg.intExpr(x.Index))
	}
	sx := fg.expr(s)
	owner := &cppir.Call{Fun: &cppir.Member{X: sx, Name: "gp_owner"}}
	typ := fg.g.cxxType(fg.g.info.TypeOf(e), e.Pos())
	return construct(typ, owner, &cppir.Unary{Op: "&", X: &cppir.Index{X: sx, Index: fg.intExpr(x.Index)}})
}
