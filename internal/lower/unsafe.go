package lower

import (
	"go/ast"
	"go/types"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Package unsafe (Phase 04 README §2.7, step 4.10; the runtime is gocpp/unsafe.hpp):
//
//	var p unsafe.Pointer          gp::ptr<void> p{};  (an owning pointer: D16)
//	unsafe.Pointer(&x)            gp::unsafe_pointer(x)       shares x's owner
//	(*T)(p)                       gp::unsafe_cast<T>(p)       shares p's owner
//	uintptr(p), unsafe.Pointer(u) gp::unsafe_addr(p), gp::unsafe_from_addr(u)
//	unsafe.Add(p, n)              gp::unsafe_add(p, n)
//	unsafe.SliceData(s)           gp::unsafe_slice_data(s)
//	unsafe.Slice(p, n)            gp::unsafe_slice(p, n)
//	unsafe.StringData(s)          gp::unsafe_string_data(s)
//	unsafe.String(p, n)           gp::unsafe_string(p, n)
//
// The same functions take raw C pointers (the pointers of C values: unsafe.Slice(argv, argc),
// unsafe.String((*byte)(unsafe.Pointer(cp)), n)), which own nothing (gocpp/unsafe.hpp). The
// conversions of C values to and from unsafe.Pointer are cvalues.go's (asConversion).
// unsafe.Sizeof, Alignof and Offsetof are constants (go/types folds them).
//
// In gc, these builtins are expression nodes, not calls: gc evaluates them in their places like
// len, after the calls of their operands (walk/order.go), which order.go does the same
// (collectInPlace); unsafe.Slice and unsafe.String can panic (a bad length, a nil pointer).

// isUnsafePointer reports whether t is unsafe.Pointer (through names and aliases).
func isUnsafePointer(t types.Type) bool {
	b, ok := under(t).(*types.Basic)
	return ok && b.Kind() == types.UnsafePointer
}

// unsafeFunc returns the name of the function of package unsafe that c calls (Add, Slice,
// SliceData, String, StringData, Sizeof, Alignof, Offsetof), or "".
func (g *gen) unsafeFunc(c *ast.CallExpr) string {
	var id *ast.Ident
	switch f := ast.Unparen(c.Fun).(type) {
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.Ident:
		id = f // a dot import of unsafe
	default:
		return ""
	}
	b, ok := g.info.Uses[id].(*types.Builtin)
	if !ok || b.Pkg() != types.Unsafe {
		return ""
	}
	return b.Name()
}

// unsafeBuiltin returns the name of the builtin of package unsafe that c calls and that
// lowering writes as a call of the runtime (Add, Slice, SliceData, String, StringData), or "":
// the sizes are unsafeSize's, and the constants (Sizeof of a gocpp value) are never calls here.
func (g *gen) unsafeBuiltin(c *ast.CallExpr) string {
	switch name := g.unsafeFunc(c); name {
	case "Add", "Slice", "SliceData", "String", "StringData":
		return name
	}
	return ""
}

// unsafeSize returns the C++ of the call c of unsafe.Sizeof, Alignof or Offsetof of a C/C++ value or
// of a type with a C/C++ part, which check wrapped in gp_cgoh.UnsafeSize (a value, not a constant: the
// size is the C++ compiler's): gp::unsafe_sizeof<decltype(x)>(), gp::unsafe_alignof<decltype(x)>()
// and offsetof(T, f). The operand is not evaluated, as in Go.
func (fg *funcGen) unsafeSize(c *ast.CallExpr, name string) cppir.Expr {
	arg := ast.Unparen(c.Args[0])
	ast.Inspect(arg, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			fg.ordered[call] = true // not evaluated, so order.go didn't collect it (decltype)
		}
		return true
	})
	decl := func(e ast.Expr) cppir.Type {
		return typeName("decltype(" + cppir.ExprString(fg.expr(e)) + ")")
	}
	switch name {
	case "Sizeof":
		return templateCall("gp::unsafe_sizeof", []cppir.Node{decl(arg)})
	case "Alignof":
		return templateCall("gp::unsafe_alignof", []cppir.Node{decl(arg)})
	}
	// Offsetof(x.f): the offset of f in the struct that x is (x may be a pointer to it).
	sel, ok := arg.(*ast.SelectorExpr)
	if !ok {
		fg.g.errorf(c.Pos(), "internal error: unsafe.Offsetof of %s", nodeKind(arg))
		return placeholder
	}
	fg.g.noteCName(sel.Sel.Name, sel.Sel.Pos())
	st := typeName("std::remove_cvref_t<decltype(gp::deref(" + cppir.ExprString(fg.expr(sel.X)) + "))>")
	return &cppir.Call{Fun: cxxName("offsetof"), Args: []cppir.Expr{&cppir.Name{Name: cppir.TypeString(st) + ", " + sel.Sel.Name}}}
}

// unsafeCall returns the C++ of the call c of the unsafe builtin name.
func (fg *funcGen) unsafeCall(c *ast.CallExpr, name string) cppir.Expr {
	switch name {
	case "Add":
		return call("gp::unsafe_add", fg.expr(c.Args[0]), fg.intExpr(c.Args[1]))
	case "Slice":
		return call("gp::unsafe_slice", fg.unsafeOperand(c.Args[0]), fg.intExpr(c.Args[1]))
	case "SliceData":
		return call("gp::unsafe_slice_data", fg.expr(c.Args[0]))
	case "String":
		return call("gp::unsafe_string", fg.unsafeOperand(c.Args[0]), fg.intExpr(c.Args[1]))
	}
	return call("gp::unsafe_string_data", fg.expr(c.Args[0]))
}

// unsafeOperand returns the C++ of the pointer p of unsafe.Slice and unsafe.String: a gocpp
// pointer, or a raw C pointer (the runtime has both).
func (fg *funcGen) unsafeOperand(p ast.Expr) cppir.Expr {
	if fg.g.pkg.Opaque.IsOpaque(p) {
		return fg.cValueOf(p)
	}
	return fg.expr(p)
}

// unsafeConversion returns the C++ of the conversion c to the type to of a value of type from
// (x is its C++) when one of them is unsafe.Pointer: the conversions of Go's rules (the spec,
// "Package unsafe") between unsafe.Pointer and a pointer type or a uintptr, and false for any
// other.
func (fg *funcGen) unsafeConversion(c *ast.CallExpr, to, from types.Type, x cppir.Expr) (cppir.Expr, bool) {
	switch {
	case isUnsafePointer(to) && fg.g.pkg.Opaque.IsOpaque(c.Args[0]):
		// unsafe.Pointer(&cvar): go/types converts a pointer to the invalid type, so check
		// rewrote nothing; the operand is a C/C++ value (cunsafe.go).
		return templateCall("gp::cgo::as", []cppir.Node{fg.g.cxxType(to, c.Pos())}, x), true
	case isUnsafePointer(to) && fg.g.info.Types[c.Args[0]].IsNil():
		return construct(fg.g.cxxType(to, c.Pos()), lit("nullptr")), true
	case isUnsafePointer(to) && isUnsafePointer(from):
		return x, true
	case isUnsafePointer(to) && isPointer(from):
		return call("gp::unsafe_pointer", x), true // shares the owner
	case isUnsafePointer(to) && isUintptr(from):
		return call("gp::unsafe_from_addr", x), true
	case isUnsafePointer(from) && isPointer(to):
		elem := under(to).(*types.Pointer).Elem()
		return templateCall("gp::unsafe_cast", []cppir.Node{fg.g.cxxType(elem, c.Pos())}, x), true
	case isUnsafePointer(from) && isUintptr(to):
		return call("gp::unsafe_addr", x), true
	}
	return nil, false
}

// isUintptr reports whether t is uintptr (through names and aliases).
func isUintptr(t types.Type) bool {
	b, ok := under(t).(*types.Basic)
	return ok && b.Kind() == types.Uintptr
}

// isPtrToUnsafePointer reports whether e is a pointer to an unsafe.Pointer (the &p of a variable
// of type unsafe.Pointer): C's void** out-parameter (lib.alloc(&p)).
func (g *gen) isPtrToUnsafePointer(e ast.Expr) bool {
	tv, ok := g.info.Types[e]
	if !ok || tv.Type == nil || tv.Value != nil {
		return false
	}
	pt, ok := under(tv.Type).(*types.Pointer)
	return ok && isUnsafePointer(pt.Elem())
}
