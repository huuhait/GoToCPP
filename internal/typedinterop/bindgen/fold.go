package bindgen

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

// A cval is the value of a constant expression of an initializer: an integer
// or a float. The generator folds the few operators that macros use inside
// braces ((1+2), -5, ~0, 1.0/3.0, 1 << 4): clang's AST keeps the operators and
// the conversions, and gives every node its type, so the folding follows C's
// rules for the type of each node (wrapping of unsigned and narrowing casts,
// truncation of a float cast to an integer, float32 rounding). What C leaves
// undefined (signed overflow, a shift by the width or more, division by zero, a
// float outside the range of the integer it is converted to) is not folded: the
// macro is listed as not translated. char is signed (the targets of this
// phase: x86-64 and aarch64 Windows/Linux/macOS agree on it except aarch64
// Linux, where a char literal above 127 would differ).
type cval struct {
	float  bool
	bool   bool // a bool: written true or false
	single bool // a float that has the precision of a float (kept for the shortest text)
	i      *big.Int
	f      float64
}

func (v cval) literal() (string, bool) {
	if v.bool {
		if v.i.Sign() != 0 {
			return "true", true
		}
		return "false", true
	}
	if v.float {
		if math.IsInf(v.f, 0) || math.IsNaN(v.f) || v.f == 0 && math.Signbit(v.f) {
			return "", false // not a Go constant, or a negative zero that a constant cannot keep
		}
		if v.f == math.Trunc(v.f) && math.Abs(v.f) < 1e15 {
			return strconv.FormatInt(int64(v.f), 10), true // a whole number: the field is a float anyway
		}
		if v.single {
			return floatLiteral32(float32(v.f)), true
		}
		return floatLiteral(v.f), true
	}
	return v.i.String(), true
}

// A numT says what kind of number a node's type is.
type numT struct {
	kind   numKind
	bits   int
	signed bool
}

type numKind uint8

const (
	notNum  numKind = iota // anything the folding does not read (pointers, enums, records, long double)
	intNum                 // an integer type, wchar_t and the char types
	boolNum                // bool
	fltNum                 // float and double
)

// intBits says how wide and how signed an integer type is, by its spelling
// (the desugared one: size_t is unsigned long).
func intBits(qual string) (bits int, signed bool, ok bool) {
	switch strings.TrimSpace(strings.TrimPrefix(qual, "const ")) {
	case "bool", "_Bool":
		return 1, false, true
	case "char", "signed char":
		return 8, true, true
	case "unsigned char", "char8_t":
		return 8, false, true
	case "short":
		return 16, true, true
	case "unsigned short", "char16_t":
		return 16, false, true
	case "int", "wchar_t":
		return 32, true, true
	case "unsigned int", "char32_t":
		return 32, false, true
	case "long", "long long":
		return 64, true, true
	case "unsigned long", "unsigned long long":
		return 64, false, true
	}
	return 0, false, false
}

// typeNum classifies a type, preferring the desugared spelling clang gives.
func typeNum(t *jtype) numT {
	if t == nil {
		return numT{}
	}
	s := t.DesugaredQualType
	if s == "" {
		s = t.QualType
	}
	s = strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(s, "const ", ""), "volatile ", ""))
	switch s {
	case "float":
		return numT{kind: fltNum, bits: 32}
	case "double":
		return numT{kind: fltNum, bits: 64}
	}
	bits, signed, ok := intBits(s)
	switch {
	case !ok:
		return numT{}
	case bits == 1:
		return numT{kind: boolNum, bits: 1}
	}
	return numT{kind: intNum, bits: bits, signed: signed}
}

// rng returns the smallest and largest value of an integer type.
func (t numT) rng() (lo, hi *big.Int) {
	one := big.NewInt(1)
	if t.signed {
		hi = new(big.Int).Sub(new(big.Int).Lsh(one, uint(t.bits-1)), one)
		lo = new(big.Int).Neg(new(big.Int).Lsh(one, uint(t.bits-1)))
		return
	}
	return big.NewInt(0), new(big.Int).Sub(new(big.Int).Lsh(one, uint(t.bits)), one)
}

func (t numT) fits(v *big.Int) bool {
	lo, hi := t.rng()
	return v.Cmp(lo) >= 0 && v.Cmp(hi) <= 0
}

// wrap brings an integer into the range of t (C's modular conversion).
func (t numT) wrap(v *big.Int) *big.Int {
	if t.kind != intNum || t.fits(v) {
		return v
	}
	mod := new(big.Int).Lsh(big.NewInt(1), uint(t.bits))
	r := new(big.Int).Mod(v, mod)
	if t.signed && r.Cmp(new(big.Int).Lsh(big.NewInt(1), uint(t.bits-1))) >= 0 {
		r.Sub(r, mod)
	}
	return r
}

// result checks an integer result of arithmetic: unsigned types wrap, a signed result
// that does not fit is undefined behaviour and is not folded.
func (t numT) result(v *big.Int) (cval, bool) {
	switch {
	case t.kind == boolNum:
		return cval{}, false
	case t.kind != intNum:
		return cval{}, false
	case !t.signed:
		return cval{i: t.wrap(v)}, true
	case !t.fits(v):
		return cval{}, false
	}
	return cval{i: v}, true
}

// roundFloat rounds f to a float of bits (32 or 64) precision.
func roundFloat(f float64, bits int) float64 {
	if bits == 32 {
		return float64(float32(f))
	}
	return f
}

// convert converts v to the type t, as a cast or an implicit conversion does.
func convert(v cval, t numT) (cval, bool) {
	switch t.kind {
	case boolNum:
		nz := v.float && v.f != 0 || !v.float && v.i.Sign() != 0
		if nz {
			return cval{i: big.NewInt(1), bool: true}, true
		}
		return cval{i: big.NewInt(0), bool: true}, true
	case intNum:
		if v.float {
			if math.IsInf(v.f, 0) || math.IsNaN(v.f) {
				return cval{}, false
			}
			i, _ := big.NewFloat(math.Trunc(v.f)).Int(nil)
			if !t.fits(i) {
				return cval{}, false // undefined: the float is out of the range of the integer
			}
			return cval{i: i}, true
		}
		return cval{i: t.wrap(v.i)}, true
	case fltNum:
		if v.float {
			return cval{float: true, single: t.bits == 32, f: roundFloat(v.f, t.bits)}, true
		}
		bf := new(big.Float).SetPrec(uint(24 + 29*(t.bits/64))).SetInt(v.i)
		f, _ := bf.Float64()
		return cval{float: true, single: t.bits == 32, f: f}, true
	}
	return cval{}, false
}

// fold evaluates the constant expression n. It reads literals, parentheses,
// casts between numbers, unary - + ~ !, and the binary operators + - * / % << >>
// & | ^ on numbers.
func fold(n *jnode) (cval, bool) {
	t := typeNum(n.Type)
	switch n.Kind {
	case "IntegerLiteral", "CharacterLiteral":
		s, _ := n.valueString()
		v, ok := new(big.Int).SetString(s, 0)
		if !ok || t.kind != intNum && t.kind != boolNum {
			return cval{}, false
		}
		return cval{i: t.wrap(v)}, true
	case "CXXBoolLiteralExpr":
		s, _ := n.valueString()
		if s == "true" {
			return cval{i: big.NewInt(1), bool: true}, true
		}
		return cval{i: big.NewInt(0), bool: true}, s == "false"
	case "FloatingLiteral":
		s, _ := n.valueString()
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || t.kind != fltNum {
			return cval{}, false
		}
		// the text clang prints is the value in the literal's own precision: a float
		// literal is rounded to float before it is widened
		return cval{float: true, single: t.bits == 32, f: roundFloat(f, t.bits)}, true
	case "UnaryExprOrTypeTraitExpr":
		// sizeof(type) and alignof(type): the value was asked of the compiler (structs.go: askTraits)
		if s, ok := n.valueString(); ok && t.kind == intNum {
			if v, ok := new(big.Int).SetString(s, 0); ok {
				return cval{i: t.wrap(v)}, true
			}
		}
		return cval{}, false
	case "ConstantExpr":
		if s, ok := n.valueString(); ok && (t.kind == intNum || t.kind == boolNum) {
			if v, ok := new(big.Int).SetString(s, 0); ok {
				return cval{i: t.wrap(v)}, true
			}
		}
		fallthrough
	case "ParenExpr", "ExprWithCleanups":
		if len(n.Inner) == 1 {
			return fold(&n.Inner[0])
		}
	case "ImplicitCastExpr", "CStyleCastExpr", "CXXFunctionalCastExpr", "CXXStaticCastExpr":
		if len(n.Inner) != 1 {
			return cval{}, false
		}
		v, ok := fold(&n.Inner[0])
		if !ok {
			return v, false
		}
		if t.kind == notNum {
			if n.CastKind == "NoOp" || n.CastKind == "LValueToRValue" {
				return v, true
			}
			return cval{}, false
		}
		return convert(v, t)
	case "UnaryOperator":
		if len(n.Inner) != 1 {
			return cval{}, false
		}
		v, ok := fold(&n.Inner[0])
		if !ok {
			return v, false
		}
		switch n.Opcode {
		case "+":
			return v, true
		case "-":
			if v.float {
				return cval{float: true, single: v.single, f: -v.f}, true
			}
			return t.result(new(big.Int).Neg(v.i))
		case "~":
			if v.float {
				return cval{}, false
			}
			return t.result(new(big.Int).Not(v.i))
		case "!":
			nz := v.float && v.f != 0 || !v.float && v.i.Sign() != 0
			if nz {
				return cval{i: big.NewInt(0), bool: true}, true
			}
			return cval{i: big.NewInt(1), bool: true}, true
		}
	case "BinaryOperator":
		if len(n.Inner) != 2 {
			return cval{}, false
		}
		a, ok1 := fold(&n.Inner[0])
		b, ok2 := fold(&n.Inner[1])
		if !ok1 || !ok2 {
			return cval{}, false
		}
		if t.kind == fltNum {
			if !a.float || !b.float {
				return cval{}, false
			}
			var r float64
			switch n.Opcode {
			case "+":
				r = a.f + b.f
			case "-":
				r = a.f - b.f
			case "*":
				r = a.f * b.f
			case "/":
				r = a.f / b.f
			default:
				return cval{}, false
			}
			return cval{float: true, single: t.bits == 32, f: roundFloat(r, t.bits)}, true
		}
		if a.float || b.float || t.kind != intNum {
			return cval{}, false
		}
		r := new(big.Int)
		switch n.Opcode {
		case "+":
			r.Add(a.i, b.i)
		case "-":
			r.Sub(a.i, b.i)
		case "*":
			r.Mul(a.i, b.i)
		case "/", "%":
			if b.i.Sign() == 0 {
				return cval{}, false
			}
			if t.signed && b.i.Cmp(big.NewInt(-1)) == 0 {
				if lo, _ := t.rng(); a.i.Cmp(lo) == 0 {
					return cval{}, false
				}
			}
			if n.Opcode == "/" {
				r.Quo(a.i, b.i) // C truncates toward zero
			} else {
				r.Rem(a.i, b.i)
			}
		case "<<", ">>":
			if b.i.Sign() < 0 || !b.i.IsUint64() || b.i.Uint64() >= uint64(t.bits) {
				return cval{}, false // undefined: the count is negative or not less than the width
			}
			if n.Opcode == "<<" {
				// C++20: the bits that do not fit are dropped; a signed result wraps
				r.Lsh(a.i, uint(b.i.Uint64()))
				return cval{i: t.wrap(r)}, true
			}
			r.Rsh(a.i, uint(b.i.Uint64()))
		case "&":
			r.And(a.i, b.i)
		case "|":
			r.Or(a.i, b.i)
		case "^":
			r.Xor(a.i, b.i)
		default:
			return cval{}, false
		}
		return t.result(r)
	}
	return cval{}, false
}
