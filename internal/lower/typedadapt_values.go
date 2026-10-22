package lower

import (
	"go/constant"
	"go/types"
	"math/big"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// ---------------------------------------------------------------- constants for enums

func enumParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	m := g.mapParam(p, scope)
	if m.Kind == ftype.Enum && (m.Ref == "" || m.Const) {
		if canon := g.interop.typed.rep.Mapper.Canonical(m.Type); canon != "" {
			return adaptKey{row: "enum", name: canon}, vAdapt
		}
	}
	return adaptKey{}, vOther
}

func lowerEnumConst(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	g := fg.g
	canon := st.key.name
	tv := g.info.Types[st.arg]
	v := constant.ToInt(tv.Value)
	if v.Kind() != constant.Int {
		return nil, false
	}
	val := constant.Val(v)
	n := new(big.Int)
	switch x := val.(type) {
	case int64:
		n.SetInt64(x)
	case *big.Int:
		n.Set(x)
	default:
		return nil, false
	}
	lo, hi, ok := g.enumRange(canon)
	if !ok {
		return nil, false
	}
	if n.Cmp(lo) < 0 || n.Cmp(hi) > 0 {
		g.userError(st.arg.Pos(), "%s: the constant %s is outside the values that C++ defines for "+
			"the enum %s (%s to %s): pass an enumerator by name", st.name, n, canon, lo, hi)
		return nil, false
	}
	return &cppir.Cast{Kind: "static_cast", Type: typeName("::" + canon), X: st.x}, true
}

// enumRange returns the values that C++ defines for the enum canon: the range of its underlying
// type for an enum class, else the range of the smallest bit-field that holds every enumerator
// ([dcl.enum]; a value outside it is undefined behavior for an enum with no fixed type).
func (g *gen) enumRange(canon string) (lo, hi *big.Int, ok bool) {
	rep := g.interop.typed.rep
	e := rep.Decls.Enums[canon]
	if e == nil {
		return nil, nil, false
	}
	target := rep.Mapper.Target()
	if e.Scoped {
		if e.Underlying.T == nil {
			return nil, nil, false
		}
		um := rep.Mapper.MapIn(e.Underlying.T, "")
		b, isBasic := um.Type.Underlying().(*types.Basic)
		if um.Type == nil || !isBasic {
			return nil, nil, false
		}
		nu, good := numericOf(b.Kind(), target)
		if !good {
			return nil, nil, false
		}
		return intRange(nu)
	}
	min, max := new(big.Int), new(big.Int)
	for _, ev := range e.Values {
		if ev.Value == nil {
			return nil, nil, false
		}
		if ev.Value.Cmp(min) < 0 {
			min.Set(ev.Value)
		}
		if ev.Value.Cmp(max) > 0 {
			max.Set(ev.Value)
		}
	}
	if min.Sign() >= 0 {
		bits := max.BitLen()
		hi = new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(bits)), big.NewInt(1))
		return big.NewInt(0), hi, true
	}
	for bits := 1; bits < 64; bits++ {
		half := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
		lo = new(big.Int).Neg(half)
		hi = new(big.Int).Sub(half, big.NewInt(1))
		if min.Cmp(lo) >= 0 && max.Cmp(hi) <= 0 {
			return lo, hi, true
		}
	}
	return nil, nil, false
}

// intRange returns the range of the integer type n.
func intRange(n numeric) (lo, hi *big.Int, ok bool) {
	if n.float {
		return nil, nil, false
	}
	one := big.NewInt(1)
	if n.signed {
		half := new(big.Int).Lsh(one, uint(n.bits-1))
		return new(big.Int).Neg(half), new(big.Int).Sub(half, one), true
	}
	return big.NewInt(0), new(big.Int).Sub(new(big.Int).Lsh(one, uint(n.bits)), one), true
}

// ---------------------------------------------------------------- numbers by value

// scalarKind returns the Go basic kind of the mapped by-value C number type m (not a bool, not a
// reference).
func scalarKind(m ftype.Mapped) (types.BasicKind, bool) {
	if m.Kind != ftype.Scalar || m.Ref != "" || m.Type == nil {
		return 0, false
	}
	b, ok := m.Type.Underlying().(*types.Basic)
	if !ok || b.Info()&types.IsNumeric == 0 {
		return 0, false
	}
	return b.Kind(), true
}

// numConstParam: an untyped constant goes to a number parameter if Go would allow it there: the
// value must be representable in the C type (300 for a char is an error, as cgo says, where C++
// would wrap it). The argument is not changed. A parameter of another kind (a pointer takes 0, a
// bool, a class) is not this row's.
func numConstParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	m := g.mapParam(p, scope)
	if kind, ok := scalarKind(m); ok {
		spelled := goCName(kind, g.interop.typed.rep.Mapper.Target())
		if c := m.C; c != nil && c.Kind == ctype.Builtin {
			spelled = c.Name
		}
		return adaptKey{row: "numconst", kind: kind, name: spelled}, vAdapt
	}
	return adaptKey{}, vOther
}

func lowerNumConst(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	g := fg.g
	kind, spelled := st.key.kind, st.key.name
	tv := g.info.Types[st.arg]
	why, ok := representable(tv, kind)
	if ok {
		return nil, false
	}
	to := "C." + cName(spelled)
	desc := constDesc(g.srcText(st.arg), tv, tv.Value)
	g.userError(st.arg.Pos(), "cannot use %s as %s value in argument to %s%s", desc, to, st.name, why)
	return nil, false
}

// floatConvParam: a Go float goes to an integer parameter (or a float64 to a float one) with Go's
// conversion, gp::cgo::as<T>, which is defined for every value (C++ makes a float out of the range
// of an integer undefined behavior). A float32 or float64 to a parameter of a float type that holds
// it is C++'s.
func floatConvParam(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict) {
	m := g.mapParam(p, scope)
	kind, ok := scalarKind(m)
	if !ok {
		return adaptKey{}, vOther
	}
	info := types.Typ[kind].Info()
	switch {
	case info&types.IsInteger != 0:
		name := goCName(kind, g.interop.typed.rep.Mapper.Target())
		return adaptKey{row: "floatconv", name: name}, vAdapt
	case kind == types.Float32 && ac == acFloat64:
		return adaptKey{row: "floatconv", name: "float"}, vAdapt
	}
	return adaptKey{}, vOther
}

func lowerFloatConv(fg *funcGen, st *adaptSite) (cppir.Expr, bool) {
	return templateCall("gp::cgo::as", []cppir.Node{typeName(st.key.name)}, st.x), true
}
