package lower

import (
	"go/ast"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// ---------------------------------------------------------------- &x of a number

// numeric describes a number type: its width, sign and kind.
type numeric struct {
	bits   int
	signed bool
	float  bool
}

// numericOf describes the basic kind k on the target t (Go's int, uint and uintptr have the
// pointer's width).
func numericOf(k types.BasicKind, t ftype.Target) (numeric, bool) {
	ptr := t.PtrBits
	if ptr == 0 {
		ptr = 64
	}
	switch k {
	case types.Int8:
		return numeric{8, true, false}, true
	case types.Uint8:
		return numeric{8, false, false}, true
	case types.Int16:
		return numeric{16, true, false}, true
	case types.Uint16:
		return numeric{16, false, false}, true
	case types.Int32:
		return numeric{32, true, false}, true
	case types.Uint32:
		return numeric{32, false, false}, true
	case types.Int64:
		return numeric{64, true, false}, true
	case types.Uint64:
		return numeric{64, false, false}, true
	case types.Int:
		return numeric{ptr, true, false}, true
	case types.Uint, types.Uintptr:
		return numeric{ptr, false, false}, true
	case types.Float32:
		return numeric{32, true, true}, true
	case types.Float64:
		return numeric{64, true, true}, true
	}
	return numeric{}, false
}

// goCName returns the C++ builtin type that gocpp's number type of kind k is (int64_t is long on
// Linux and long long on Windows): the spelling that a parameter's builtin has when it is the same
// type.
func goCName(k types.BasicKind, t ftype.Target) string {
	n, _ := numericOf(k, t)
	long := "long"
	ulong := "unsigned long"
	if t.LongBits == 32 {
		long = "long long"
		ulong = "unsigned long long"
	}
	switch {
	case n.float && n.bits == 32:
		return "float"
	case n.float:
		return "double"
	case n.signed:
		switch n.bits {
		case 8:
			return "signed char"
		case 16:
			return "short"
		case 32:
			return "int"
		}
		return long
	}
	switch n.bits {
	case 8:
		return "unsigned char"
	case 16:
		return "unsigned short"
	case 32:
		return "unsigned int"
	}
	return ulong
}

// fits reports whether every value of the number type a is a value of b (both integers or both
// floats).
func fits(a, b numeric) bool {
	if a.float != b.float {
		return false
	}
	if a.float {
		return a.bits <= b.bits
	}
	switch {
	case a.signed == b.signed:
		return a.bits <= b.bits
	case !a.signed && b.signed:
		return a.bits < b.bits
	}
	return false // a signed value in an unsigned type
}

// arithmeticCNames are the C number types that the scalarptr row takes as a pointee. The character
// types are left out on purpose: a char* is nearly always a buffer or a string, and a pointer to
// one variable would let the callee write past it.
var arithmeticCNames = map[string]bool{
	"short": true, "unsigned short": true, "int": true, "unsigned int": true,
	"long": true, "unsigned long": true, "long long": true, "unsigned long long": true,
	"float": true, "double": true,
}

func scalarPtrParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	m := g.mapParam(p, scope)
	switch {
	case pointsToCNumber(m):
		key := adaptKey{row: "scalarptr", name: m.Elem.C.Name}
		key.isConst = m.Elem.Const || m.Elem.C.Const
		return key, vAdapt
	case isNumberOrEnumValue(m):
		return adaptKey{}, vIgnore // a pointer does not convert to a number or an enum
	}
	return adaptKey{}, vOther
}

func lowerScalarPtr(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	g := fg.g
	u := ast.Unparen(st.arg).(*ast.UnaryExpr)
	kind, _ := g.goNumberKind(u.X)
	rep := g.interop.typed.rep
	target := rep.Mapper.Target()
	name := st.key.name
	isConst := st.key.isConst
	if goCName(kind, target) == name {
		return nil, false // the same C++ type: the pointer converts by itself
	}
	cm := rep.Mapper.MapIn(&ctype.Type{Kind: ctype.Builtin, Name: name}, "")
	cb, ok := cm.Type.Underlying().(*types.Basic)
	if cm.Type == nil || !ok {
		return nil, false
	}
	goNum, _ := numericOf(kind, target)
	cNum, ok := numericOf(cb.Kind(), target)
	if !ok {
		return nil, false
	}
	text := g.srcText(u.X)
	switch {
	case goNum.float != cNum.float:
		g.userError(st.arg.Pos(), "%s takes a pointer to %s, and &%s is a pointer to %s: "+
			"C converts neither way through a pointer; declare %s with the C type (C.%s)",
			st.name, name, text, types.Typ[kind].Name(), text, cName(name))
		return nil, false
	case isConst && !fits(goNum, cNum) && goNum.float:
		g.userError(st.arg.Pos(), "%s takes a pointer to const %s, and %s (%s) is wider than %s: "+
			"the copy would round it; declare %s with the C type (C.%s)",
			st.name, name, text, types.Typ[kind].Name(), name, text, cName(name))
		return nil, false
	case !isConst && !fits(cNum, goNum):
		g.userError(st.arg.Pos(), "%s takes a pointer to %s that it may write, and &%s points to %s "+
			"that cannot hold every %s (the copy's value would be cut when it is stored back): "+
			"declare %s with the C type (C.%s)",
			st.name, name, text, types.Typ[kind].Name(), name, text, cName(name))
		return nil, false
	}
	if isConst {
		quoted := lit(strconv.Quote(text))
		return templateCall("gp::cgo::scalar_in", []cppir.Node{typeName(name)}, st.x, quoted), true
	}
	param := fg.outParam(typeName(name))
	*st.outs = append(*st.outs, outWrap{
		fun:   &cppir.Name{Name: "gp::cgo::call_out", Args: []cppir.Node{typeName(name)}},
		args:  []cppir.Expr{st.x, lit(strconv.Quote(text))},
		param: param,
	})
	return cxxName(param.Name), true
}

// cName returns the name that gocpp gives the C builtin type name (C.int, C.ulonglong).
func cName(name string) string {
	for k, v := range cBuiltins {
		if v == name {
			return k
		}
	}
	return name
}

// ---------------------------------------------------------------- &p for void**

// pointsToCNumber reports whether m is a pointer to one of the C number types of the scalarptr row.
func pointsToCNumber(m ftype.Mapped) bool {
	if m.Kind != ftype.Pointer || m.Elem == nil || m.Elem.Kind != ftype.Scalar {
		return false
	}
	c := m.Elem.C
	return c != nil && c.Kind == ctype.Builtin && arithmeticCNames[c.Name]
}

// pointsToVoidPointer reports whether m is a void** that the callee may write (no const).
func pointsToVoidPointer(m ftype.Mapped) bool {
	if m.Kind != ftype.Pointer || m.Elem == nil || m.Elem.Kind != ftype.Pointer || m.Elem.Const {
		return false
	}
	inner := m.Elem.Elem
	return inner != nil && inner.Kind == ftype.Void && !inner.Const
}

// isNumberOrEnumValue reports whether m is a number (not a bool) or an enum taken by value: a
// pointer never converts to it.
func isNumberOrEnumValue(m ftype.Mapped) bool {
	if m.Kind == ftype.Scalar {
		return m.C == nil || m.C.Name != "bool"
	}
	return m.Kind == ftype.Enum && m.Ref == ""
}

func voidOutParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	m := g.mapParam(p, scope)
	switch {
	case pointsToVoidPointer(m):
		return adaptKey{row: "voidout"}, vAdapt
	case isNumberOrEnumValue(m):
		return adaptKey{}, vIgnore
	}
	return adaptKey{}, vOther
}

func lowerVoidOut(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	g := fg.g
	if fg.thunk > 0 {
		g.userError(st.arg.Pos(), "%s: the address of a pointer variable for a void** parameter "+
			"cannot be the argument of a deferred or spawned call (the variable gets the pointer "+
			"when the call ends): call it in a function and defer that", st.name)
		return nil, false
	}
	t := fg.outPtrTarget(st.arg)
	if t == nil {
		return nil, false
	}
	param := fg.outParam(&cppir.Ptr{Elem: typeName("void")})
	*st.outs = append(*st.outs, outWrap{
		fun:   cxxName("gp::cgo::call_void_out"),
		args:  []cppir.Expr{fg.expr(t)},
		param: param,
	})
	return cxxName(param.Name), true
}
