package cppgen

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"strconv"
	"strings"
)

// Operators, conversions, s[i] and the builtins len and panic (Phase 02 README §2.3):
//
//	Go                          C++
//	a + b, a - b, a * b         the same operator
//	a / b, a % b (integers)     gp::div(a, b), gp::rem(a, b); a / 3 and a % 3 for a constant
//	                            divisor other than 0 and -1
//	a / b (floats)              a / b
//	a << n, a >> n              gp::shl(a, n), gp::shr(a, n); a << 3 for a constant n below
//	                            the width of a
//	a & b, a | b, a ^ b         the same operator
//	a &^ b                      a & ~b
//	-x, ^x, !x, +x              -x, ~x, !x, x
//	== != < <= > >=, && ||      the same operator
//	s + t, s == t, s < t, …     gp::string's operators
//	s[i], len(s)                s[i] (gp::string's operator[], which checks the bounds), gp::len(s)
//	T(x)                        T(x), or x if T and x have the same C++ type; gp::f2i<T>(x)
//	                            for a float x and an integer type T
//	panic(v)                    gp::panic(v)
//
// The small-integer rule: C++ computes int8, int16, uint8 and uint16 operands in int, so a
// result that should wrap doesn't (uint8 200 + 100 is 300 in C++, 44 in Go). The result of
// every + - * / % << >> & | ^ &^ and unary - and ^ on these four types is cast back to its
// type, uint8_t(a + b), unless a runtime helper (gp::div, gp::shl, …) already returns it.
// Comparisons need no cast: every operand already has a value of its Go type (the cast rule
// makes sure of it), and C++ compares those values exactly in int.
//
// Signed overflow wraps like Go's because the C++ is compiled with -fwrapv, and shifts of
// negative values are defined since C++20 (two's complement).

// isSmallInt reports whether C++ promotes values of the basic type b to int in arithmetic.
func isSmallInt(b *types.Basic) bool {
	switch b.Kind() {
	case types.Int8, types.Int16, types.Uint8, types.Uint16:
		return true
	}
	return false
}

// narrow casts x, the result of an operator on operands of the type b, back to b if C++
// computes it in int (the small-integer rule).
func narrow(b *types.Basic, x cexpr) cexpr {
	if !isSmallInt(b) {
		return x
	}
	return cexpr{basicTypes[b.Kind()] + "(" + x.text + ")", precPostfix}
}

// bitSize returns the width of the integer type b in bits (int, uint and uintptr have 64:
// gocpp has only 64-bit targets).
func bitSize(b *types.Basic) uint64 {
	switch b.Kind() {
	case types.Int8, types.Uint8:
		return 8
	case types.Int16, types.Uint16:
		return 16
	case types.Int32, types.Uint32:
		return 32
	}
	return 64
}

// operandType returns the basic type of the operand x, or nil (with an error) if it has
// another type.
func (fg *funcGen) operandType(pos token.Pos, op token.Token, x ast.Expr) *types.Basic {
	t := fg.g.info.TypeOf(x)
	b := basicOf(t)
	if b == nil {
		fg.g.unsupported(pos, "the operator "+op.String()+" on "+types.TypeString(t, types.RelativeTo(fg.g.pkg.Types)))
	}
	return b
}

// constValue returns the value of the expression e if it is a constant, or nil.
func (fg *funcGen) constValue(e ast.Expr) constant.Value {
	return fg.g.info.Types[e].Value
}

func (fg *funcGen) binary(e *ast.BinaryExpr) cexpr {
	return fg.binaryOp(e.OpPos, e.Op, e.X, e.Y)
}

// binaryOp returns the C++ of x op y, a Go binary expression whose operator is at pos. (Step
// 2.4's x op= y uses it too.) Both operands are translated from left to right.
func (fg *funcGen) binaryOp(pos token.Pos, op token.Token, x, y ast.Expr) cexpr {
	switch op {
	case token.LOR:
		// a || (b && c): C++ doesn't need the parentheses, but they are clearer, and g++
		// -Wall asks for them.
		return fg.grouped(x, "||", y, precLogOr, precLogAnd)
	case token.LAND:
		return fg.infix(x, "&&", y, precLogAnd)
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		a, b := fg.expr(x), fg.expr(y) // first, so that an operand's own error comes first
		if fg.operandType(pos, op, x) == nil {
			return placeholder
		}
		prec := precRel
		if op == token.EQL || op == token.NEQ {
			prec = precEq
		}
		// A comparison as an operand of a comparison gets parentheses, which C++ doesn't
		// need: (a < b) == c, not a < b == c, which reads like a chain.
		return cexpr{a.at(precShift) + " " + op.String() + " " + b.at(precShift), prec}
	}

	// The other operators: the result has the type of x (a shift too, whose count may have
	// another type).
	t := fg.operandType(pos, op, x)
	if t == nil {
		return placeholder
	}
	switch op {
	case token.ADD, token.SUB:
		return narrow(t, fg.infix(x, op.String(), y, precAdd))
	case token.MUL:
		return narrow(t, fg.infix(x, "*", y, precMul))
	case token.QUO, token.REM:
		if t.Info()&types.IsInteger == 0 {
			return fg.infix(x, op.String(), y, precMul) // floats: IEEE division, x / 0 is ±Inf or NaN
		}
		if v := fg.constValue(y); v != nil && constant.Sign(v) != 0 && constant.Compare(v, token.NEQ, constant.MakeInt64(-1)) {
			// Neither a division by zero nor MIN / -1 can happen.
			return narrow(t, fg.infix(x, op.String(), y, precMul))
		}
		helper := "gp::div("
		if op == token.REM {
			helper = "gp::rem("
		}
		// gp::div deduces its type from x (y converts to it), so a constant x needs its type.
		return cexpr{helper + fg.exactExpr(x).text + ", " + fg.expr(y).text + ")", precPostfix}
	case token.SHL, token.SHR:
		return fg.shift(t, op, x, y)
	case token.AND:
		return narrow(t, fg.grouped(x, "&", y, precBitAnd, precUnary-1))
	case token.OR:
		return narrow(t, fg.grouped(x, "|", y, precBitOr, precUnary-1))
	case token.XOR:
		return narrow(t, fg.grouped(x, "^", y, precBitXor, precUnary-1))
	case token.AND_NOT:
		a := fg.expr(x)
		if a.prec != precBitAnd {
			a.text = a.at(precUnary)
		}
		return narrow(t, cexpr{a.text + " & ~" + fg.expr(y).at(precUnary), precBitAnd})
	}
	fg.g.unsupported(pos, "the operator "+op.String())
	return placeholder
}

// infix returns x op y for a C++ operator of the precedence prec that groups from left to
// right, like all of Go's: a right operand of the same precedence needs parentheses,
// a - (b - c).
func (fg *funcGen) infix(x ast.Expr, op string, y ast.Expr, prec int) cexpr {
	a := fg.expr(x).at(prec)
	return cexpr{a + " " + op + " " + fg.expr(y).at(prec+1), prec}
}

// grouped is infix for the C++ operators whose operands read better with parentheses: an
// operand whose operator has a precedence of at most loose gets them, unless it is the same
// operator on the left (a | b | c). So a bit operator's operand is a name, a call, a unary
// expression or in parentheses, (a + b) | 1 and (a & b) | c (C++ would group both the same
// without them, but they are easy to misread, and g++ -Wall warns), and so is an && inside
// ||.
func (fg *funcGen) grouped(x ast.Expr, op string, y ast.Expr, prec, loose int) cexpr {
	a, b := fg.expr(x), fg.expr(y)
	if a.prec < prec || a.prec <= loose && a.prec != prec {
		a.text = "(" + a.text + ")"
	}
	return cexpr{a.text + " " + op + " " + b.at(max(prec, loose)+1), prec}
}

// shift returns the C++ of x << y or x >> y, where x has the integer type t. Go defines every
// count: one of the width of t or more gives 0 (or -1 for >> of a negative signed x), and a
// negative one panics. C++ defines only counts below the width, so the plain operator is used
// only for a constant count below it; the runtime's gp::shl and gp::shr do the rest.
func (fg *funcGen) shift(t *types.Basic, op token.Token, x, y ast.Expr) cexpr {
	if v := fg.constValue(y); v != nil {
		n, exact := constant.Uint64Val(constant.ToInt(v))
		if exact && n < bitSize(t) {
			// The left operand of a plain shift gets parentheses even where C++ doesn't
			// need them: clang warns about a + b << 3 (-Wshift-op-parentheses).
			a := fg.expr(x).at(precUnary)
			return narrow(t, cexpr{a + " " + op.String() + " " + strconv.FormatUint(n, 10), precShift})
		}
	}
	helper := "gp::shl("
	if op == token.SHR {
		helper = "gp::shr("
	}
	return cexpr{helper + fg.exactExpr(x).text + ", " + fg.shiftCount(y).text + ")", precPostfix}
}

// shiftCount returns the C++ of the count of a shift for gp::shl or gp::shr, which take a
// count of any integer type. A constant count (one of the width or more, here) is written as
// an unsigned value: go/types may leave it untyped, and Go allows any count that a uint holds.
func (fg *funcGen) shiftCount(y ast.Expr) cexpr {
	v := fg.constValue(y)
	if v == nil {
		return fg.expr(y)
	}
	n, _ := constant.Uint64Val(constant.ToInt(v))
	if n <= math.MaxInt32 {
		return cexpr{strconv.FormatUint(n, 10), precPrimary}
	}
	return cexpr{"UINT64_C(" + strconv.FormatUint(n, 10) + ")", precPrimary}
}

func (fg *funcGen) unary(e *ast.UnaryExpr) cexpr {
	switch e.Op {
	case token.NOT:
		return cexpr{"!" + fg.expr(e.X).at(precUnary), precUnary}
	case token.ADD, token.SUB, token.XOR:
		t := fg.operandType(e.OpPos, e.Op, e.X)
		if t == nil {
			return placeholder
		}
		x := fg.expr(e.X)
		switch e.Op {
		case token.ADD:
			return x // +x is x (C++'s +x would promote a small integer to int)
		case token.XOR:
			return narrow(t, cexpr{"~" + x.at(precUnary), precUnary})
		}
		s := x.at(precUnary)
		if strings.HasPrefix(s, "-") {
			s = "(" + s + ")" // - -x, not the decrement --x
		}
		return narrow(t, cexpr{"-" + s, precUnary})
	}
	fg.g.unsupported(e.OpPos, "the operator "+e.Op.String())
	return placeholder
}

// index returns the C++ of s[i]. Phase 02 has it only for strings: the byte at i, as a
// uint8_t, after gp::string's bounds check (Go's "index out of range" panic).
func (fg *funcGen) index(e *ast.IndexExpr) cexpr {
	t := fg.g.info.TypeOf(e.X)
	if b := basicOf(t); b == nil || b.Info()&types.IsString == 0 {
		fg.g.unsupported(e.Lbrack, "index expressions on "+types.TypeString(t, types.RelativeTo(fg.g.pkg.Types))+" (Phase 03)")
		return placeholder
	}
	s := fg.expr(e.X).at(precPostfix)
	return cexpr{s + "[" + fg.expr(e.Index).text + "]", precPostfix}
}

// builtin returns the C++ of a call of the builtin len or panic. (A constant len(s) was
// folded before.)
func (fg *funcGen) builtin(c *ast.CallExpr, name string) cexpr {
	arg := c.Args[0]
	t := fg.g.info.TypeOf(arg)
	b := basicOf(t)
	tname := types.TypeString(t, types.RelativeTo(fg.g.pkg.Types))
	switch name {
	case "len":
		if b == nil || b.Info()&types.IsString == 0 {
			fg.g.unsupported(c.Pos(), "len of "+tname+" (Phase 03)")
			return placeholder
		}
		return cexpr{"gp::len(" + fg.expr(arg).text + ")", precPostfix}
	default: // panic
		if b == nil {
			fg.g.unsupported(c.Pos(), "panic with a value of type "+tname+" (Phase 05)")
			return placeholder
		}
		// gp::panic is a template: the value needs its Go type, which decides how it prints
		// (panic(1.5) prints +1.500000e+000).
		return cexpr{"gp::panic(" + fg.exactExpr(arg).text + ")", precPostfix}
	}
}

// conversion returns the C++ of the conversion T(x), where x is not a constant (expr folds a
// conversion of a constant). Phase 02 has the conversions between the number types and the
// identities (bool(b), string(s)); the conversions between strings and bytes, runes or
// integers are Phase 03's.
//
// Between integer types, T(x) is a C++ functional cast, which C++20 defines like Go: the
// value modulo 2^width of T, sign-extended from a signed x. Between floats and from integers to
// floats, it rounds to the nearest value, as in Go. From a float to an integer type,
// gp::f2i<T> truncates toward zero and gives amd64's results where C++ has undefined
// behavior (out of range, NaN).
func (fg *funcGen) conversion(c *ast.CallExpr, to types.Type) cexpr {
	if len(c.Args) != 1 {
		fg.g.errorf(c.Pos(), "internal error: a conversion with %d arguments", len(c.Args))
		return placeholder
	}
	arg := c.Args[0]
	from := fg.g.info.TypeOf(arg)
	tb, fb := basicOf(to), basicOf(from)
	if tb == nil || fb == nil {
		qual := types.RelativeTo(fg.g.pkg.Types)
		fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual))
		return placeholder
	}
	ti, fi := tb.Info(), fb.Info()
	if ti&types.IsString != 0 || fi&types.IsString != 0 {
		if ti&fi&types.IsString == 0 {
			fg.g.unsupported(c.Pos(), "conversions between strings and bytes, runes or integers (Phase 03)")
			return placeholder
		}
		return fg.expr(arg) // string(s)
	}
	x := fg.expr(arg)
	ct := basicTypes[tb.Kind()]
	switch {
	case ct == basicTypes[fb.Kind()]:
		return x // int(i) of an int64, bool(b): the same C++ type
	case ti&types.IsInteger != 0 && fi&types.IsFloat != 0:
		return cexpr{"gp::f2i<" + ct + ">(" + x.text + ")", precPostfix}
	}
	return cexpr{ct + "(" + x.text + ")", precPostfix}
}
