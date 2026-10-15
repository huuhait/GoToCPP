package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Weak pointers (Phase 03 README §2.2, step 3.18): Go 1.24's package weak is a compiler
// intrinsic until generics come (Phase 07). Its stub (stdlib/weak) gives go/types the generic
// declarations, and lowering maps them to the runtime's gp::weak_ptr (gocpp/ptr.hpp):
//
//	weak.Pointer[T]      gp::weak_ptr<T> (24 bytes, gocppSizes)
//	weak.Pointer[T]{}    gp::weak_ptr<T>{}, the zero weak pointer
//	weak.Make(p)         gp::weak_make<T>(p) (nil p: the zero weak pointer)
//	w.Value()            gp::weak_value(w): nil once the last pointer to the target is gone
//	pw.Value()           gp::weak_value(*pw), through a pointer (nil panics)
//	w == v               w == v (the addresses they were made from, as in Go)
//
// A weak.Pointer is no struct for lowering (isStruct and isNewtype are false for it): it has
// no fields that a program can name (they are unexported), and the runtime type is a class of
// its own. Method values (f := w.Value) and method expressions (weak.Pointer[T].Value) go
// through methods.go, whose callMethod calls gp::weak_value too. fmt prints it as Go prints the
// struct that Go declares, {[] 0xc000012345} or {[] <nil>} (gocpp/fmtshim.hpp).

// isWeakPointer reports whether t is weak.Pointer[T] of the standard package weak (through
// aliases), an instance (not the generic type itself).
func isWeakPointer(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.TypeArgs().Len() != 1 {
		return false
	}
	obj := n.Origin().Obj()
	return obj.Name() == "Pointer" && obj.Pkg() != nil && obj.Pkg().Path() == "weak"
}

// weakStruct reports whether t's underlying type is the struct that package weak declares for
// Pointer (type WP weak.Pointer[T] is such a type): its fields belong to package weak.
func weakStruct(t types.Type) bool {
	st, ok := under(t).(*types.Struct)
	if !ok || st.NumFields() == 0 {
		return false
	}
	pkg := st.Field(0).Pkg()
	return pkg != nil && pkg.Path() == "weak"
}

// weakElem returns the T of weak.Pointer[T].
func weakElem(t types.Type) types.Type {
	return types.Unalias(t).(*types.Named).TypeArgs().At(0)
}

// weakType returns gp::weak_ptr<elem>, the C++ type of weak.Pointer[T] where elem is T's.
func weakType(elem cppir.Type) cppir.Type {
	return &cppir.TypeName{Name: "gp::weak_ptr", Args: []cppir.Node{elem}}
}

// weakFunc reports whether fn is the function or the method of package weak named name:
// "Make", or "Value" for the method Pointer[T].Value.
func weakFunc(fn *types.Func, name string) bool {
	return fn != nil && fn.Name() == name && fn.Pkg() != nil && fn.Pkg().Path() == "weak"
}

// isWeakMake reports whether the call c calls weak.Make (also weak.Make[T]): no func value,
// though its callee has a func type (funcValueCall).
func (fg *funcGen) isWeakMake(c *ast.CallExpr) bool {
	fun := ast.Unparen(c.Fun)
	switch x := fun.(type) {
	case *ast.IndexExpr:
		fun = ast.Unparen(x.X)
	case *ast.IndexListExpr:
		fun = ast.Unparen(x.X)
	}
	sel, ok := fun.(*ast.SelectorExpr)
	if !ok || fg.g.info.Selections[sel] != nil {
		return false
	}
	fn, _ := fg.g.info.Uses[sel.Sel].(*types.Func)
	return weakFunc(fn, "Make") && len(c.Args) == 1
}

// weakMake returns the C++ of the call c if it calls weak.Make (isWeakMake), else nil. The type
// argument is written out, so that a nil argument (weak.Make[T](nil)) converts to gp::ptr<T> as
// any other argument does.
func (fg *funcGen) weakMake(c *ast.CallExpr) cppir.Expr {
	if !fg.isWeakMake(c) {
		return nil
	}
	t := fg.g.info.TypeOf(c)
	if !isWeakPointer(t) {
		fg.g.errorf(c.Pos(), "internal error: weak.Make gives %s", t)
		return placeholder
	}
	elem := fg.g.cxxType(weakElem(t), c.Pos())
	return templateCall("gp::weak_make", []cppir.Node{elem}, fg.expr(c.Args[0]))
}

// weakValue returns the C++ of w.Value() for the receiver recv: gp::weak_value(recv), or
// gp::weak_value(*recv) when recv is a pointer to the weak pointer (ptr), which panics for nil
// as Go's call does.
func weakValue(recv cppir.Expr, ptr bool) cppir.Expr {
	if ptr {
		recv = deref(recv)
	}
	return call("gp::weak_value", recv)
}
