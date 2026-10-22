package lower

import (
	"go/ast"
	"go/types"
)

// The basic C number types are the Go fixed-size types (ADR-130, "type identity"):
//
//	C.char, C.schar  int8        C.uchar    uint8      C.short   int16     C.ushort  uint16
//	C.int            int32       C.uint     uint32     C.float   float32   C.double  float64
//	C.longlong       int64       C.ulonglong uint64    C.bool    bool
//	C.long           int64       C.ulong    uint64     (Windows, LLP64: int32 and uint32)
//
// A value of one is a value of the other, so `var x C.int = n32`, `x + n32` and passing an int32
// where a C.int is wanted need no conversion, and the explicit conversions between them stay legal
// (they are conversions between identical types). Go's int, uint and uintptr are not aliases (they
// are 64 bits and spell no C type), and neither is a named type over a fixed-size type (type Count
// int32): those are the mismatches that cgo's rules report. C.size_t and C.wchar_t are not in the
// list (their width is the target's business).
//
// This is the checker's rule and it needs no clang: it is applied wherever the declared type of the
// C value is a C builtin that the program wrote (cKind). The C++ side is the same type already
// (int32_t is int, int64_t is long on Linux and long long on Windows), or one of the same width.

// cIdentityNames are the C builtins that are aliases of Go's fixed-size types.
var cIdentityNames = map[string]bool{
	"char": true, "schar": true, "uchar": true, "short": true, "ushort": true, "int": true,
	"uint": true, "long": true, "ulong": true, "longlong": true, "ulonglong": true, "float": true,
	"double": true, "bool": true,
}

// cIdentical reports whether the C builtin named name (without the C.: "int") is the same type as
// the Go type t on the target of the package: t must be the predeclared type, not a named type.
func (g *gen) cIdentical(name string, t types.Type) bool {
	if !cIdentityNames[name] || t == nil {
		return false
	}
	b, ok := t.(*types.Basic)
	if !ok {
		return false
	}
	kind, ok := g.cConstType(name)
	return ok && b.Kind() == kind
}

// cBuiltinName returns the C builtin that the declared type of the C value e is ("int" for a
// variable of type C.int), or "".
func (g *gen) cBuiltinName(e ast.Expr) string {
	return g.cBuiltinOfType(g.valueTypeExpr(e))
}

// cBuiltinOfType returns the C builtin that the type expression te is, or "".
func (g *gen) cBuiltinOfType(te ast.Expr) string {
	if te == nil {
		return ""
	}
	sel, ok := ast.Unparen(te).(*ast.SelectorExpr)
	if !ok || !g.isC(sel.X) || cBuiltins[sel.Sel.Name] == "" {
		return ""
	}
	return sel.Sel.Name
}
