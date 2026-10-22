package adapt

import (
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"math/big"

	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// Numbers: the widths of the Go and C number types, which values a type holds, and whether a
// constant fits one. The rows of the table and the messages share them.

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

// GoCName returns the C++ builtin type that gocpp's number type of kind k is (int64_t is long on
// Linux and long long on Windows): the spelling that a parameter's builtin has when it is the same
// type.
func GoCName(k types.BasicKind, t ftype.Target) string {
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

// cgoNames maps the C++ spelling of a C builtin to the name cgo gives it (C.uint for unsigned int).
var cgoNames = map[string]string{
	"char":               "char",
	"signed char":        "schar",
	"unsigned char":      "uchar",
	"short":              "short",
	"unsigned short":     "ushort",
	"int":                "int",
	"unsigned int":       "uint",
	"long":               "long",
	"unsigned long":      "ulong",
	"long long":          "longlong",
	"unsigned long long": "ulonglong",
	"float":              "float",
	"double":             "double",
	"bool":               "bool",
	"size_t":             "size_t",
	"wchar_t":            "wchar_t",
}

// CgoName returns the name that gocpp gives the C builtin type name (C.int, C.ulonglong), without
// the C.
func CgoName(name string) string {
	if n, ok := cgoNames[name]; ok {
		return n
	}
	return name
}

// underBasic returns the basic type under t, or nil.
func underBasic(t types.Type) *types.Basic {
	if t == nil {
		return nil
	}
	b, _ := t.Underlying().(*types.Basic)
	return b
}

// Representable reports whether the constant tv is representable in a value of the basic type
// kind, and why not as go/types says it: " (overflows)" or " (truncated)" for a number (truncated:
// a float or complex constant for an integer type, or a complex one with an imaginary part), or ""
// for a constant of another kind (a string, a bool).
func Representable(tv types.TypeAndValue, kind types.BasicKind) (string, bool) {
	v := tv.Value
	src := underBasic(tv.Type)
	info := types.Typ[kind].Info()
	if info&types.IsBoolean != 0 {
		return "", v.Kind() == constant.Bool // C.bool takes true and false
	}
	if src == nil || src.Info()&types.IsNumeric == 0 {
		return "", false
	}
	why := " (overflows)"
	if src.Info()&types.IsInteger == 0 && info&types.IsInteger != 0 {
		why = " (truncated)"
	}
	if v.Kind() == constant.Complex {
		if constant.Sign(constant.Imag(v)) != 0 {
			return " (truncated)", false
		}
		v = constant.Real(v) // 1 + 0i
	}
	switch {
	case info&types.IsInteger != 0:
		i := constant.ToInt(v)
		if i.Kind() != constant.Int {
			return " (truncated)", false
		}
		size := map[types.BasicKind]uint{types.Int8: 8, types.Uint8: 8, types.Int16: 16, types.Uint16: 16,
			types.Int32: 32, types.Uint32: 32, types.Int64: 64, types.Uint64: 64}[kind]
		lo, hi := constant.MakeInt64(0), constant.Shift(constant.MakeInt64(1), token.SHL, size)
		if info&types.IsUnsigned == 0 {
			hi = constant.Shift(constant.MakeInt64(1), token.SHL, size-1)
			lo = constant.UnaryOp(token.SUB, hi, 0)
		}
		if constant.Compare(i, token.LSS, lo) || constant.Compare(i, token.GEQ, hi) {
			return why, false
		}
	default:
		f := constant.ToFloat(v)
		if f.Kind() != constant.Float && f.Kind() != constant.Int {
			return why, false
		}
		if kind == types.Float32 {
			if x, _ := constant.Float32Val(f); math.IsInf(float64(x), 0) {
				return why, false
			}
		} else if x, _ := constant.Float64Val(f); math.IsInf(x, 0) {
			return why, false
		}
	}
	return "", true
}

// ConstDesc returns the untyped constant src (tv is its type, v its value) as go/types describes
// an operand: 200 (untyped int constant), 1e300 (untyped float constant 1e+300).
func ConstDesc(src string, tv types.TypeAndValue, v constant.Value) string {
	if b := underBasic(tv.Type); b != nil && b.Info()&types.IsUntyped == 0 {
		return fmt.Sprintf("%s (constant %s of type %s)", src, v.String(), tv.Type.String())
	}
	name := "untyped constant"
	if b, ok := tv.Type.(*types.Basic); ok {
		name = b.Name() + " constant" // untyped int
	}
	if s := v.String(); s != src {
		name += " " + s
	}
	return fmt.Sprintf("%s (%s)", src, name)
}
