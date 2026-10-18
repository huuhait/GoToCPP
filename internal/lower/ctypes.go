package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/interop"
)

// The C type translator (Phase 04 README §2.4, step 4.2). go/types has no types for C/C++
// things: a variable declared as var p *cstdio.FILE has the invalid type. The opaque pass
// (check.Opaque.TypeExpr) keeps the type expression of every object declared with a C/C++
// type, and lowering prints the C++ type from that syntax:
//
//	C.char, C.schar, ... C.size_t        char, signed char, ... size_t (cgo's names, cBuiltins)
//	*cstdio.FILE, **C.char               FILE*, char** (pointers to C/C++ types are raw)
//	std.vector[int]                      ::std::vector<int64_t> (gocpp int is 64 bits)
//	std.array[C.int, 4], std.bitset[64]  ::std::array<int, 4>, ::std::bitset<64>
//	std.vector[Point], std.vector[*Node] ::std::vector<::gpkg_main::Point>,
//	                                     ::std::vector<gp::ptr<::gpkg_main::Node>>
//	std.vector[int].iterator             ::std::vector<int64_t>::iterator
//	[]C.int, map[string]C.int, [4]C.int  gp::slice<int>, gp::map<gp::string, int>, gp::array<int, 4>
//	*[]C.int                             gp::ptr<gp::slice<int>> (a gocpp pointer to a gocpp slice)
//
// A part that go/types has a complete type for is a gocpp type, printed as everywhere else
// (cxxType); a gocpp struct or named type in a C++ template's arguments is fully qualified
// (::gpkg_main::Point), like the C/C++ names. Not yet: the named types declared over C/C++ types
// (type Handle C.int) and func, struct and interface types with C/C++ parts.

// typeExpr returns the C++ of the type expression e, which may have C/C++ parts.
func (g *gen) typeExpr(e ast.Expr) cppir.Type {
	if tv, ok := g.info.Types[e]; ok && tv.IsType() && complete(tv.Type) {
		return g.cxxType(tv.Type, e.Pos())
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		return g.typeExpr(x.X)
	case *ast.StarExpr:
		if g.isCType(x.X) {
			if _, nested := ast.Unparen(x.X).(*ast.StarExpr); nested || g.inTemplate > 0 {
				// A pointer to a pointer of C's memory (char**: what C hands out and takes), or a
				// pointer in a C++ template's arguments: raw, C++ rules.
				g.inTemplate++
				defer func() { g.inTemplate-- }()
				return &cppir.Ptr{Elem: g.typeExpr(x.X)}
			}
			// A gocpp pointer to a C/C++ type: gp::cptr<T> (cunsafe.go), which owns or aliases what
			// it points to, as Go's pointer does, and converts to the raw T* of C.
			return cptrType(g.typeExpr(x.X))
		}
		return ptrType(g.typeExpr(x.X))
	case *ast.ArrayType:
		elem := g.typeExpr(x.Elt)
		if x.Len == nil {
			return sliceType(elem)
		}
		if tv, ok := g.info.Types[x.Len]; ok && tv.Value != nil {
			if n, ok := constant.Int64Val(constant.ToInt(tv.Value)); ok {
				return arrayType(elem, n)
			}
		}
	case *ast.MapType:
		if g.isCPtrType(x.Key) {
			// A pointer to a C/C++ type is a key: gp::cptr compares and hashes by address.
			return mapType(g.typeExpr(x.Key), g.typeExpr(x.Value))
		}
		if tv, ok := g.info.Types[x.Key]; !ok || !tv.IsType() || cmpHasC(tv.Type) {
			break // a key with a C/C++ part: its hash and == are the C++ compiler's
		}
		return mapType(g.typeExpr(x.Key), g.typeExpr(x.Value))
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr:
		if r, ok := g.cRefOf(x); ok {
			return g.cType(r, e)
		}
	case *ast.Ident:
		if tn, ok := g.info.Uses[x].(*types.TypeName); ok {
			if tn.IsAlias() {
				g.unsupported(x.Pos(), "aliases of C/C++ types ("+tn.Name()+"): write the C/C++ type (a later step of Phase 04)")
			} else {
				g.unsupported(x.Pos(), "named types declared over C/C++ types ("+tn.Name()+", a later step of Phase 04)")
			}
			return typeName("gp_unsupported")
		}
	}
	g.unsupported(e.Pos(), "this type with C/C++ parts: "+types.ExprString(e))
	return typeName("gp_unsupported")
}

// isCPtrType reports whether the type expression e is *T for a C/C++ type T that is no pointer: a
// gocpp pointer to a C/C++ type (gp::cptr), the type of a variable, a map key or an element.
func (g *gen) isCPtrType(e ast.Expr) bool {
	star, ok := ast.Unparen(e).(*ast.StarExpr)
	if !ok || !g.isCType(star.X) {
		return false
	}
	_, nested := ast.Unparen(star.X).(*ast.StarExpr)
	return !nested
}

// cType returns the C++ type of the C/C++ name r (written as e), which must name a type.
func (g *gen) cType(r cRef, e ast.Expr) cppir.Type {
	if r.x == placeholder {
		return typeName("gp_unsupported") // reported
	}
	switch r.kind {
	case interop.Value, interop.ValueTemplate:
		g.userError(e.Pos(), "%s is a C/C++ value, not a type", r.src)
		return typeName("gp_unsupported")
	case interop.Namespace:
		g.userError(e.Pos(), "%s is a C++ namespace, not a type", r.src)
		return typeName("gp_unsupported")
	}
	if n, ok := r.x.(*cppir.Name); ok {
		return g.cNode(&cppir.TypeName{Name: n.Name, Args: n.Args}).(*cppir.TypeName)
	}
	g.userError(e.Pos(), "%s is not a type", r.src)
	return typeName("gp_unsupported")
}

// isCType reports whether the type expression e is a C/C++ type (not a gocpp type with C/C++
// parts): a C/C++ name, an instance of a C++ template, or a raw pointer to one. A pointer to it
// is a raw C++ pointer.
func (g *gen) isCType(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.StarExpr:
		return g.isCType(x.X)
	case *ast.SelectorExpr:
		if g.isC(x.X) || g.cUse(x) != nil {
			return true
		}
		return g.isCType(x.X)
	case *ast.IndexExpr:
		return g.isCType(x.X)
	case *ast.IndexListExpr:
		return g.isCType(x.X)
	}
	return false
}

// templateArg returns the C++ of a template argument of a C++ template: a type (gocpp or C/C++),
// or a constant, as a C literal (cConstant), so that std.array[C.int, 4] is
// ::std::array<int, 4>.
func (g *gen) templateArg(e ast.Expr) cppir.Node {
	tv, ok := g.info.Types[e]
	if ok && tv.IsType() {
		switch types.Unalias(tv.Type).Underlying().(type) {
		case *types.Signature:
			g.unsupported(e.Pos(), "func types as C++ template arguments (step 4.11)")
			return placeholder
		case *types.Interface:
			g.unsupported(e.Pos(), "interface types as C++ template arguments (Phase 05)")
			return placeholder
		case *types.Chan:
			g.unsupported(e.Pos(), "channel types as C++ template arguments (Phase 08)")
			return placeholder
		}
	}
	switch {
	case ok && tv.IsType(), !ok && g.isCType(e):
		g.inTemplate++
		defer func() { g.inTemplate-- }()
		return g.typeExpr(e)
	case ok && tv.Value != nil:
		return g.cConstant(tv.Value, tv.Type, e.Pos())
	}
	g.unsupported(e.Pos(), "this template argument: "+types.ExprString(e))
	return placeholder
}

// complete reports whether t has no part that go/types left invalid (a C/C++ type), outside a
// named type, whose parts have their own declarations: a struct type with a field of a C/C++
// type (type Holder struct{ f *cstdio.FILE }) is complete, a named type declared over a C/C++
// type (type Handle C.int) is not. hasC looks inside named types too.
func complete(t types.Type) bool {
	switch t := t.(type) {
	case *types.Basic:
		return t.Kind() != types.Invalid
	case *types.Pointer:
		return complete(t.Elem())
	case *types.Slice:
		return complete(t.Elem())
	case *types.Array:
		return complete(t.Elem())
	case *types.Map:
		return complete(t.Key()) && complete(t.Elem())
	case *types.Chan:
		return complete(t.Elem())
	case *types.Struct:
		for i := range t.NumFields() {
			if !complete(t.Field(i).Type()) {
				return false
			}
		}
	case *types.Signature:
		return complete(t.Params()) && complete(t.Results())
	case *types.Tuple:
		for i := range t.Len() {
			if !complete(t.At(i).Type()) {
				return false
			}
		}
	case *types.Alias:
		return complete(types.Unalias(t))
	case *types.Named:
		b, ok := t.Underlying().(*types.Basic)
		return !ok || b.Kind() != types.Invalid
	}
	return true
}

// cmpHasC reports whether Go's == on values of type t needs the == of a C/C++ type, which the
// C++ structs of gocpp types with C/C++ fields don't have (typedecl.go): t is a C/C++ type, or a
// struct or an array that holds one by value. A pointer to such a struct compares as a pointer.
func cmpHasC(t types.Type) bool {
	switch u := t.Underlying().(type) {
	case *types.Struct:
		return hasCFields(u)
	case *types.Array:
		return cmpHasC(u.Elem())
	case *types.Slice, *types.Map, *types.Signature, *types.Pointer:
		return false // compared with nil (or as an address), whatever the elements are (xs == nil of a []C.int)
	}
	return !complete(t)
}

// hasC reports whether t has a C/C++ part, also in the fields of the named struct types in it
// (through pointers, slices and maps too).
func hasC(t types.Type) bool {
	return hasCSeen(t, make(map[*types.Named]bool))
}

// HasC is hasC for the driver, which reports a C++ error on a .gp line with such a value as the
// user's (cmd/gocpp/cpperrors.go).
func HasC(t types.Type) bool {
	return hasC(t)
}

func hasCSeen(t types.Type, seen map[*types.Named]bool) bool {
	switch t := types.Unalias(t).(type) {
	case *types.Named:
		if seen[t] {
			return false
		}
		seen[t] = true
		return !complete(t) || hasCSeen(t.Underlying(), seen)
	case *types.Pointer:
		return hasCSeen(t.Elem(), seen)
	case *types.Slice:
		return hasCSeen(t.Elem(), seen)
	case *types.Array:
		return hasCSeen(t.Elem(), seen)
	case *types.Map:
		return hasCSeen(t.Key(), seen) || hasCSeen(t.Elem(), seen)
	case *types.Chan:
		return hasCSeen(t.Elem(), seen)
	case *types.Struct:
		for i := range t.NumFields() {
			if hasCSeen(t.Field(i).Type(), seen) {
				return true
			}
		}
		return false
	case *types.Signature:
		return hasCSeen(t.Params(), seen) || hasCSeen(t.Results(), seen)
	case *types.Tuple:
		for i := range t.Len() {
			if hasCSeen(t.At(i).Type(), seen) {
				return true
			}
		}
		return false
	}
	return !complete(t)
}

// cConstantOf emits the constant e in a C context. At a known integer target the exact
// go/constant integer is preserved, including floating and zero-imaginary complex constants.
func (g *gen) cConstantOf(e ast.Expr) cppir.Expr {
	tv := g.info.Types[e]
	kind := g.cConstKinds[ast.Unparen(e)]
	if types.Typ[kind].Info()&types.IsInteger != 0 {
		return cInt(constant.ToInt(tv.Value), e.Pos(), g)
	}
	if (kind == types.Float32 || kind == types.Float64) && tv.Value.Kind() == constant.Complex {
		return floatLiteral(constant.ToFloat(constant.Real(tv.Value)), kind == types.Float32)
	}
	return g.cConstant(tv.Value, tv.Type, e.Pos())
}

// cConstant returns the C++ of the constant v of type t where a C/C++ value takes it: an
// argument of a C/C++ function, a template argument, the value of a variable of a C/C++ type
// (§2.5). An untyped constant is a plain C literal with C's default types: 5 is an int (a
// larger value the int type that C++ gives it: long, long long), 2.0 a double, "x" a string
// literal, 'a' a char, true a bool, nil nullptr. That is what C APIs expect (printf("%d", 5)).
// A typed constant keeps its gocpp type, so that a C++ template that deduces a type from it
// gets the gocpp type: int(0) is int64_t(0).
func (g *gen) cConstant(v constant.Value, t types.Type, pos token.Pos) cppir.Expr {
	b, _ := types.Unalias(t).(*types.Basic)
	if b == nil || b.Info()&types.IsUntyped == 0 {
		return g.constant(v, t, pos, true)
	}
	switch v.Kind() {
	case constant.Bool:
		return lit(strconv.FormatBool(constant.BoolVal(v)))
	case constant.String:
		return lit(cppir.CString(constant.StringVal(v)))
	case constant.Int:
		if b.Kind() == types.UntypedRune {
			if r, ok := constant.Int64Val(v); ok && r >= ' ' && r < 0x7f {
				return lit(charLiteral(rune(r)))
			}
		}
		return cInt(v, pos, g)
	case constant.Float:
		return floatLiteral(v, false)
	case constant.Complex:
		if constant.Sign(constant.Imag(v)) == 0 {
			// 1 + 0i for a C.double: Go makes it the real number (cconsts.go checks it fits).
			return floatLiteral(constant.ToFloat(constant.Real(v)), false)
		}
	}
	g.unsupported(pos, "the constant "+v.String()+" as a C/C++ value")
	return placeholder
}

// charLiteral returns the C++ character literal of the printable ASCII character r: 'a',
// '\”, '\\'.
func charLiteral(r rune) string {
	switch r {
	case '\'', '\\':
		return `'\` + string(r) + `'`
	}
	return "'" + string(r) + "'"
}

// cInt returns the C literal of the untyped integer constant v: a decimal literal, which C++
// gives the first of int, long and long long that holds it, with ULL above the long long
// values, and the smallest long long as (-9223372036854775807LL - 1) (9223372036854775808
// itself is no long long). A value outside the 64-bit integers is an error.
func cInt(v constant.Value, pos token.Pos, g *gen) cppir.Expr {
	if i, ok := constant.Int64Val(v); ok {
		switch {
		case i == math.MinInt64:
			return &cppir.Paren{X: &cppir.Binary{Op: "-", X: &cppir.Unary{Op: "-", X: lit("9223372036854775807LL")}, Y: lit("1")}}
		case i < 0:
			return &cppir.Unary{Op: "-", X: lit(strconv.FormatInt(-i, 10))}
		}
		return lit(strconv.FormatInt(i, 10))
	}
	if u, ok := constant.Uint64Val(v); ok {
		return lit(strconv.FormatUint(u, 10) + "ULL")
	}
	g.userError(pos, "the constant %s doesn't fit in a C/C++ integer type (64 bits)", v.String())
	return placeholder
}
