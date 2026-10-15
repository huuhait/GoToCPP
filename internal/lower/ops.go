package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
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
//	s + t + u (3 or more)       gp::concat(s, t, u): one allocation (concat)
//	s[i], len(s)                s[i] (gp::string's operator[], which checks the bounds), gp::len(s)
//	s[a:b], s[a:], s[:b], s[:]  gp::sub(s, a, b), gp::sub(s, a), gp::sub(s, 0, b), s (slice)
//	s[a:b:c] (a slice)          gp::sub(s, a, b, c)
//	a[i], a[a:b], a == b        arrays (step 3.6): arrays.go
//	s == nil (a slice)          s == nullptr
//	T(x)                        T(x), or x if T and x have the same C++ type; gp::f2i<T>(x)
//	                            for a float x and an integer type T
//	string(r)                   gp::rune_to_string(r), for an integer r
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
func narrow(b *types.Basic, x cppir.Expr) cppir.Expr {
	if !isSmallInt(b) {
		return x
	}
	return call(basicTypes[b.Kind()], x)
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

// operandType returns the basic type of the operand x (under its name, for a named type: step
// 3.9), or nil (with an error) if it has another type.
func (fg *funcGen) operandType(pos token.Pos, op token.Token, x ast.Expr) *types.Basic {
	t := fg.typeOf(x)
	b := underBasic(t)
	if b == nil {
		fg.g.unsupported(pos, "the operator "+op.String()+" on "+types.TypeString(t, types.RelativeTo(fg.g.pkg.Types)))
	}
	return b
}

// typeOf returns the Go type of the expression e. go/types leaves the type of a shift count
// that is not a constant but is made of untyped constants untyped (x >> (1<<n - 2)); Go's
// compiler computes it as a uint, so inside such a count (markCounts) an untyped integer is a
// uint.
func (fg *funcGen) typeOf(e ast.Expr) types.Type {
	return fg.g.untypedCount(e, fg.g.info.TypeOf(e))
}

// untypedCount returns t, the type of e, or uint for an untyped integer t inside an untyped
// shift count (typeOf). It depends only on e, not on where the translation is: an operand of
// the count may be translated into a temporary in front of the statement (order.go).
func (g *gen) untypedCount(e ast.Expr, t types.Type) types.Type {
	if b, ok := t.(*types.Basic); ok && g.uintCounts[e] && (b.Kind() == types.UntypedInt || b.Kind() == types.UntypedRune) {
		return types.Typ[types.Uint]
	}
	return t
}

// markCounts finds the shift counts in n that are untyped but not constants (x >> (1<<k - 2),
// x <<= 1<<k) and marks every expression in them (uintCounts, typeOf).
func (g *gen) markCounts(n ast.Node) {
	mark := func(count ast.Expr) {
		if g.info.Types[count].Value != nil {
			return
		}
		if b := basicOf(g.info.TypeOf(count)); b == nil || b.Info()&types.IsUntyped == 0 {
			return
		}
		ast.Inspect(count, func(m ast.Node) bool {
			if e, ok := m.(ast.Expr); ok {
				g.uintCounts[e] = true
			}
			return true
		})
	}
	ast.Inspect(n, func(m ast.Node) bool {
		switch m := m.(type) {
		case *ast.BinaryExpr:
			if m.Op == token.SHL || m.Op == token.SHR {
				mark(m.Y)
			}
		case *ast.AssignStmt:
			if (m.Tok == token.SHL_ASSIGN || m.Tok == token.SHR_ASSIGN) && len(m.Rhs) == 1 {
				mark(m.Rhs[0])
			}
		}
		return true
	})
}

// constValue returns the value of the expression e if it is a constant, or nil.
func (fg *funcGen) constValue(e ast.Expr) constant.Value {
	return fg.g.info.Types[e].Value
}

func (fg *funcGen) binary(e *ast.BinaryExpr) cppir.Expr {
	if t := fg.g.info.TypeOf(e.X); (e.Op == token.EQL || e.Op == token.NEQ) && t != nil && cmpHasC(t) {
		// A struct or an array with a C/C++ part: its == is the C++ compiler's (typedecl.go).
		fg.g.unsupported(e.OpPos, "== on values with C/C++ parts (step 4.4)")
		return placeholder
	}
	if (e.Op == token.SHL || e.Op == token.SHR) && fg.constValue(e.X) != nil {
		// A constant shifted by a count that is not a constant: the constant has the type
		// of the shift (the spec, "Operators"), which go/types may not record for the
		// constant itself (x >> (1<<n - 2): the inner shift is a count, a uint).
		t := fg.operandType(e.OpPos, e.Op, e)
		if t == nil {
			return placeholder
		}
		return fg.shift(t, fg.typeOf(e), e.Op, e.X, e.Y)
	}
	x := fg.binaryOp(e.OpPos, e.Op, e.X, e.Y)
	switch e.Op {
	case token.LAND, token.LOR, token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		// A bool of a named type (type ok bool, step 3.9): b && c and x < y give C++'s bool.
		return fg.g.wrapNamed(fg.typeOf(e), x, e.Pos())
	}
	return x
}

// binaryOp returns the C++ of x op y, a Go binary expression whose operator is at pos. (Step
// 2.4's x op= y uses it too.) Both operands are translated from left to right.
func (fg *funcGen) binaryOp(pos token.Pos, op token.Token, x, y ast.Expr) cppir.Expr {
	switch op {
	case token.LOR:
		// a || (b && c): C++ doesn't need the parentheses, but they are clearer, and g++
		// -Wall asks for them.
		return fg.grouped(x, "||", y, precLogOr, precLogAnd)
	case token.LAND:
		return fg.infix(x, "&&", y)
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		a, b := fg.expr(x), fg.expr(y) // first, so that an operand's own error comes first
		if (op == token.EQL || op == token.NEQ) && (isPointer(fg.typeOf(x)) || isPointer(fg.typeOf(y)) ||
			isSlice(fg.typeOf(x)) || isSlice(fg.typeOf(y)) || isArray(fg.typeOf(x)) ||
			isMap(fg.typeOf(x)) || isMap(fg.typeOf(y)) || isStruct(fg.typeOf(x)) ||
			isFunc(fg.typeOf(x)) || isFunc(fg.typeOf(y)) || isWeakPointer(fg.typeOf(x))) {
			// p == q compares the addresses, p == nil is p == nullptr (gp::ptr's ==; C++20
			// writes != and nil == p from it). A slice or a map compares only with nil:
			// s == nullptr, and so does a func (step 3.11). Arrays compare their elements (gp::array's ==), structs their
			// fields (the generated operator==, typedecl.go), weak pointers the addresses
			// they were made from (gp::weak_ptr's ==, weak.go).
			return &cppir.Binary{Op: op.String(), X: at(a, precShift), Y: at(b, precShift)}
		}
		if fg.operandType(pos, op, x) == nil {
			return placeholder
		}
		// A comparison as an operand of a comparison gets parentheses, which C++ doesn't
		// need: (a < b) == c, not a < b == c, which reads like a chain.
		return &cppir.Binary{Op: op.String(), X: at(a, precShift), Y: at(b, precShift)}
	}

	// The other operators: the result has the type of x (a shift too, whose count may have
	// another type).
	t := fg.operandType(pos, op, x)
	if t == nil {
		return placeholder
	}
	xt := fg.typeOf(x)
	narrow := func(b *types.Basic, x cppir.Expr) cppir.Expr {
		if isNewtype(xt) {
			return x // gp::newtype's operators convert back to the type (step 3.9)
		}
		return narrow(b, x)
	}
	switch op {
	case token.ADD, token.SUB:
		if op == token.ADD && t.Info()&types.IsString != 0 {
			return fg.g.wrapNamed(xt, fg.concat(x, y), pos) // gp::string's + gives a string
		}
		return narrow(t, fg.infix(x, op.String(), y))
	case token.MUL:
		return narrow(t, fg.infix(x, "*", y))
	case token.QUO, token.REM:
		if t.Info()&types.IsInteger == 0 {
			return fg.infix(x, op.String(), y) // floats: IEEE division, x / 0 is ±Inf or NaN
		}
		if v := fg.constValue(y); v != nil && constant.Sign(v) != 0 && constant.Compare(v, token.NEQ, constant.MakeInt64(-1)) {
			// Neither a division by zero nor MIN / -1 can happen.
			return narrow(t, fg.infix(x, op.String(), y))
		}
		helper := "gp::div"
		if op == token.REM {
			helper = "gp::rem"
		}
		// gp::div deduces its type from x (y converts to it), so a constant x needs its type.
		return call(helper, fg.exactExpr(x), fg.expr(y))
	case token.SHL, token.SHR:
		return fg.shift(t, xt, op, x, y)
	case token.AND:
		return narrow(t, fg.grouped(x, "&", y, precBitAnd, precUnary-1))
	case token.OR:
		return narrow(t, fg.grouped(x, "|", y, precBitOr, precUnary-1))
	case token.XOR:
		return narrow(t, fg.grouped(x, "^", y, precBitXor, precUnary-1))
	case token.AND_NOT:
		a := fg.expr(x)
		if cppir.Prec(a) != precBitAnd {
			a = at(a, precUnary)
		}
		notY := &cppir.Unary{Op: "~", X: fg.expr(y)}
		return narrow(t, &cppir.Binary{Op: "&", X: a, Y: notY})
	}
	fg.g.unsupported(pos, "the operator "+op.String())
	return placeholder
}

// concat returns the C++ of the string concatenation x + y. A chain of + (a + b + c, also
// with parentheses: a + (b + c), and the s + t + u of s += t + u) is one call of gp::concat
// with all the operands, which allocates the result once, where C++'s + would make a string
// for each +: Go's compiler does the same (concatstrings). The operands are translated from
// left to right, and an operand that order.go put into a temporary, or a constant, is one
// operand. Two operands stay a + b.
func (fg *funcGen) concat(x, y ast.Expr) cppir.Expr {
	var parts []ast.Expr
	var walk func(e ast.Expr)
	walk = func(e ast.Expr) {
		u := ast.Unparen(e)
		_, temp := fg.temps[u]
		if b, ok := u.(*ast.BinaryExpr); ok && b.Op == token.ADD && !temp && fg.constValue(u) == nil {
			if t := underBasic(fg.typeOf(u)); t != nil && t.Info()&types.IsString != 0 {
				walk(b.X)
				walk(b.Y)
				return
			}
		}
		parts = append(parts, e)
	}
	walk(x)
	walk(y)
	if len(parts) == 2 {
		return fg.infix(parts[0], "+", parts[1])
	}
	return call("gp::concat", fg.exprs(parts, false)...)
}

// infix returns x op y for a C++ operator that groups from left to right, like all of Go's.
// The printer adds the parentheses that C++ needs: a - (b - c), (a + b) * c.
func (fg *funcGen) infix(x ast.Expr, op string, y ast.Expr) cppir.Expr {
	a := fg.expr(x)
	return &cppir.Binary{Op: op, X: a, Y: fg.expr(y)}
}

// grouped is infix for the C++ operators whose operands read better with parentheses: an
// operand whose operator has a precedence of at most loose gets them, unless it is the same
// operator on the left (a | b | c). So a bit operator's operand is a name, a call, a unary
// expression or in parentheses, (a + b) | 1 and (a & b) | c (C++ would group both the same
// without them, but they are easy to misread, and g++ -Wall warns), and so is an && inside
// ||.
func (fg *funcGen) grouped(x ast.Expr, op string, y ast.Expr, prec, loose int) cppir.Expr {
	a, b := fg.expr(x), fg.expr(y)
	if p := cppir.Prec(a); p < prec || p <= loose && p != prec {
		a = &cppir.Paren{X: a}
	}
	return &cppir.Binary{Op: op, X: a, Y: at(b, max(prec, loose)+1)}
}

// shift returns the C++ of x << y or x >> y, where x has the integer type t (under the named
// type nt, or nt itself). Go defines every count: one of the width of t or more gives 0 (or -1
// for >> of a negative signed x), and a negative one panics. C++ defines only counts below the
// width, so the plain operator is used only for a constant count below it; the runtime's
// gp::shl and gp::shr do the rest (with overloads for the named types, gocpp/newtype.hpp).
func (fg *funcGen) shift(t *types.Basic, nt types.Type, op token.Token, x, y ast.Expr) cppir.Expr {
	if v := fg.constValue(y); v != nil {
		n, exact := constant.Uint64Val(constant.ToInt(v))
		if exact && n < bitSize(t) {
			// The left operand of a plain shift gets parentheses even where C++ doesn't
			// need them: clang warns about a + b << 3 (-Wshift-op-parentheses).
			a := at(fg.expr(x), precUnary)
			x := &cppir.Binary{Op: op.String(), X: a, Y: lit(strconv.FormatUint(n, 10))}
			if isNewtype(nt) {
				return x // gp::newtype's << converts back to the type
			}
			return narrow(t, x)
		}
	}
	helper := "gp::shl"
	if op == token.SHR {
		helper = "gp::shr"
	}
	a := fg.exactExpr(x)
	if v := fg.constValue(x); v != nil {
		var st types.Type = t // the type of the shift
		if isNewtype(nt) {
			st = nt
		}
		a = fg.g.constant(v, st, x.Pos(), true)
	}
	return call(helper, a, fg.shiftCount(y))
}

// shiftCount returns the C++ of the count of a shift for gp::shl or gp::shr, which take a
// count of any integer type. A constant count (one of the width or more, here) is written as
// an unsigned value: go/types may leave it untyped, and Go allows any count that a uint holds.
// So is a count that is not a constant but untyped (1<<n - 2, typeOf).
func (fg *funcGen) shiftCount(y ast.Expr) cppir.Expr {
	v := fg.constValue(y)
	if v == nil {
		return valueOf(fg.typeOf(y), fg.expr(y)) // a named count: its value (step 3.9)
	}
	n, _ := constant.Uint64Val(constant.ToInt(v))
	if n <= math.MaxInt32 {
		return lit(strconv.FormatUint(n, 10))
	}
	return call("UINT64_C", lit(strconv.FormatUint(n, 10)))
}

func (fg *funcGen) unary(e *ast.UnaryExpr) cppir.Expr {
	switch e.Op {
	case token.AND:
		return fg.addr(e)
	case token.NOT:
		return &cppir.Unary{Op: "!", X: fg.expr(e.X)}
	case token.ADD, token.SUB, token.XOR:
		t := fg.operandType(e.OpPos, e.Op, e.X)
		if t == nil {
			return placeholder
		}
		x := fg.expr(e.X)
		narrow := narrow
		if isNewtype(fg.typeOf(e.X)) {
			// gp::newtype's operators convert back to the type (step 3.9).
			narrow = func(_ *types.Basic, x cppir.Expr) cppir.Expr { return x }
		}
		switch e.Op {
		case token.ADD:
			return x // +x is x (C++'s +x would promote a small integer to int)
		case token.XOR:
			return narrow(t, &cppir.Unary{Op: "~", X: x})
		}
		// The printer writes - -x as -(-x), not as the decrement --x.
		return narrow(t, &cppir.Unary{Op: "-", X: x})
	}
	fg.g.unsupported(e.OpPos, "the operator "+e.Op.String())
	return placeholder
}

// index returns the C++ of s[i]: for a string, the byte at i, as a uint8_t, after
// gp::string's bounds check (Go's "index out of range" panic); for a slice (step 3.5), the
// element itself, a T& that can be assigned, after gp::slice's bounds check.
func (fg *funcGen) index(e *ast.IndexExpr) cppir.Expr {
	t := fg.g.info.TypeOf(e.X)
	if isMap(t) {
		return fg.mapRead(e) // step 3.7, maps.go
	}
	if arrayOf(t) != nil {
		return fg.arrayIndex(e) // arrays.go
	}
	if !isString(t) && !isSlice(t) {
		fg.g.unsupported(e.Lbrack, "index expressions on "+types.TypeString(t, types.RelativeTo(fg.g.pkg.Types))+" (Phase 03)")
		return placeholder
	}
	return &cppir.Index{X: fg.expr(e.X), Index: fg.intExpr(e.Index)}
}

// slice returns the C++ of the slice expression s[a:b], s[a:], s[:b], s[:] or s[a:b:c]. Phase
// 03 step 3.4 has it for strings: the substring shares the bytes of s, after Go's bounds
// checks (gp::sub, with Go's "slice bounds out of range" panics); step 3.5 for slices, whose
// result shares the array (gp::sub's checks against the capacity, also with a third bound).
// A missing low bound is 0, and s[:] is s itself. The bounds may have any integer types
// (gp::sub is a template), so a constant bound is a plain C++ int literal (intExpr). In
// s = s[a:b] of a local s, s is moved into gp::sub when nothing else reads it (markMoves,
// slices.go).
func (fg *funcGen) slice(e *ast.SliceExpr) cppir.Expr {
	t := fg.g.info.TypeOf(e.X)
	if arrayOf(t) != nil {
		return fg.arraySlice(e) // arrays.go
	}
	if !isString(t) && !isSlice(t) {
		fg.g.unsupported(e.Lbrack, "slice expressions on "+types.TypeString(t, types.RelativeTo(fg.g.pkg.Types))+" (Phase 03 step 3.6)")
		return placeholder
	}
	x := fg.movedExpr(e.X)
	switch {
	case e.Low == nil && e.High == nil && e.Max == nil:
		return x
	case e.High == nil:
		x = call("gp::sub", x, fg.intExpr(e.Low))
	case e.Max != nil:
		x = call("gp::sub", x, fg.sliceLow(e), fg.intExpr(e.High), fg.intExpr(e.Max))
	default:
		x = call("gp::sub", x, fg.sliceLow(e), fg.intExpr(e.High))
	}
	// s[a:b] of a named string or slice type has that type (step 3.9): gp::sub gives the
	// underlying one.
	return fg.g.wrapNamed(fg.g.info.TypeOf(e), x, e.Pos())
}

// sliceLow returns the C++ of the low bound of the slice expression e: 0 if it has none.
func (fg *funcGen) sliceLow(e *ast.SliceExpr) cppir.Expr {
	if e.Low == nil {
		return lit("0")
	}
	return fg.intExpr(e.Low)
}

// builtin returns the C++ of a call of the builtin len, cap, panic, new, make, append, copy,
// clear or delete. (A constant len(s) was folded before.) new(T) is gp::new_<T>(), a pointer to
// a new zero T; len and cap are gp::len and gp::cap; the slice builtins are slices.go's, the
// map builtins maps.go's.
func (fg *funcGen) builtin(c *ast.CallExpr, name string) cppir.Expr {
	arg := c.Args[0]
	if name == "new" {
		t := fg.g.info.TypeOf(arg)
		return newBox(fg.g.cxxType(t, arg.Pos()), nil)
	}
	t := fg.g.info.TypeOf(arg)
	b := basicOf(t)
	tname := types.TypeString(t, types.RelativeTo(fg.g.pkg.Types))
	switch name {
	case "len", "cap":
		// len and cap of an array, or of a pointer to one, are N: a constant unless the
		// argument has calls (gp::len(f()) evaluates f, and doesn't dereference a pointer).
		if s := underBasic(t); !isSlice(t) && arrayOf(t) == nil && !(name == "len" && isMap(t)) && (name == "cap" || s == nil || s.Info()&types.IsString == 0) {
			fg.g.unsupported(c.Pos(), name+" of "+tname+" (Phase 03)")
			return placeholder
		}
		return call("gp::"+name, arrayPtrValue(t, fg.expr(arg)))
	case "make", "append", "copy", "clear", "delete":
		var x cppir.Expr
		if isMap(t) {
			x = fg.mapBuiltin(c, name) // step 3.7, maps.go
		} else {
			x = fg.sliceBuiltin(c, name)
		}
		// make(IDs, n) and append(ids, x) of a named type have that type (step 3.9): the
		// runtime gives the underlying one.
		return fg.g.wrapNamed(fg.g.info.TypeOf(c), x, c.Pos())
	default: // panic
		if b == nil {
			fg.g.unsupported(c.Pos(), "panic with a value of type "+tname+" (Phase 05)")
			return placeholder
		}
		// gp::panic is a template: the value needs its Go type, which decides how it prints
		// (panic(1.5) prints +1.500000e+000). A constant is gp::panic_constant, and int64,
		// uint64 and uintptr (int64_t and uint64_t in C++, like int and uint) name their kind:
		// Go prints a chain of two panics with the same value once (gocpp/panic.hpp,
		// panic_identity), and gc gives a constant and a value different data pointers.
		// gc's constants include a local variable that only its declaration gives a constant
		// (panicid.go).
		fun := &cppir.Name{Name: "gp::panic"}
		value := func() cppir.Expr { return fg.exactExpr(arg) }
		if v := fg.staticConst(arg); v != nil {
			fun.Name = "gp::panic_constant"
			value = func() cppir.Expr { return fg.g.constant(v, fg.g.untypedCount(arg, t), arg.Pos(), true) }
		}
		switch b.Kind() {
		case types.Int64, types.Uint64, types.Uintptr:
			fun.Args = []cppir.Node{lit(strconv.Itoa(int(b.Kind())))}
		}
		return &cppir.Call{Fun: fun, Args: []cppir.Expr{value()}}
	}
}

// conversion returns the C++ of the conversion T(x), where x is not a constant (expr folds a
// conversion of a constant). Phase 02 has the conversions between the number types and the
// identities (bool(b), string(s)); Phase 03 adds string(r) for an integer r, the UTF-8 of the
// rune r (gp::rune_to_string, which gives "\uFFFD" for a value that is not a code point, as
// Go does). The conversions with slices ([]byte(s), string(b), []rune(s), string(runes),
// []T(nil)) are step 3.5's (sliceConversion, slices.go).
//
// Between integer types, T(x) is a C++ functional cast, which C++20 defines like Go: the
// value modulo 2^width of T, sign-extended from a signed x. Between floats and from integers to
// floats, it rounds to the nearest value, as in Go. From a float to an integer type,
// gp::f2i<T> truncates toward zero and gives amd64's results where C++ has undefined
// behavior (out of range, NaN).
func (fg *funcGen) conversion(c *ast.CallExpr, to types.Type) cppir.Expr {
	if len(c.Args) != 1 {
		fg.g.errorf(c.Pos(), "internal error: a conversion with %d arguments", len(c.Args))
		return placeholder
	}
	arg := c.Args[0]
	from := fg.g.info.TypeOf(arg)
	if isNewtype(to) || isNewtype(from) {
		return fg.namedConversion(c, to, from) // step 3.9, named.go
	}
	return fg.convertValue(c, to, from, fg.exactExpr(arg))
}

// convertValue returns the C++ of the conversion c to the type to of a value of type from (not
// named types that aren't structs: namedConversion), whose C++ is x.
func (fg *funcGen) convertValue(c *ast.CallExpr, to, from types.Type, x cppir.Expr) cppir.Expr {
	arg := c.Args[0]
	if y := fg.arrayConversion(c, to, from, x); y != nil {
		return y // arrays.go
	}
	if isStruct(to) || isStruct(from) {
		return fg.structConversion(c, to, from, x) // structs.go
	}
	if isPointer(to) {
		// (*int)(p) between pointers of the same C++ type, or (*int)(nil); between pointers
		// whose base types have identical underlying types ((*B)(pa) for type B A),
		// gp::ptr_cast<B>(pa), which shares pa's target.
		typ := fg.g.cxxType(to, c.Pos())
		if fg.g.info.Types[arg].IsNil() {
			return construct(typ, lit("nullptr"))
		}
		if isPointer(from) && cppir.TypeString(typ) == cppir.TypeString(fg.g.cxxType(from, c.Pos())) {
			return x
		}
		if isPointer(from) {
			elem := under(to).(*types.Pointer).Elem()
			return templateCall("gp::ptr_cast", []cppir.Node{fg.g.cxxType(elem, c.Pos())}, x)
		}
		qual := types.RelativeTo(fg.g.pkg.Types)
		fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual))
		return placeholder
	}
	if isFunc(to) {
		// Between func types with identical underlying types (type Handler func(int), step
		// 3.11): the same C++ type, or nil.
		if fg.g.info.Types[arg].IsNil() {
			return construct(fg.g.cxxType(to, c.Pos()))
		}
		return x
	}
	if isSlice(to) || isSlice(from) {
		return fg.sliceConversion(c, to, from, x)
	}
	if isMap(to) {
		return fg.mapConversion(c, to, from, x) // maps.go
	}
	if isWeakPointer(to) {
		// weak.Pointer[T](w), A(w) for an alias A: only identity conversions reach here, since a
		// type defined over weak.Pointer is gated (weak.go).
		return x
	}
	tb, fb := basicOf(to), basicOf(from)
	if tb == nil || fb == nil {
		qual := types.RelativeTo(fg.g.pkg.Types)
		fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual))
		return placeholder
	}
	ti, fi := tb.Info(), fb.Info()
	if ti&types.IsString != 0 || fi&types.IsString != 0 {
		if ti&fi&types.IsString != 0 {
			return x // string(s)
		}
		if ti&types.IsString != 0 && fi&types.IsInteger != 0 {
			// The template takes the value with its Go type, which decides what is valid
			// (a uint64 above 2^63 is not a negative number).
			return call("gp::rune_to_string", x)
		}
		qual := types.RelativeTo(fg.g.pkg.Types)
		fg.g.unsupported(c.Pos(), "the conversion from "+types.TypeString(from, qual)+" to "+types.TypeString(to, qual))
		return placeholder
	}
	ct := basicTypes[tb.Kind()]
	switch {
	case ct == basicTypes[fb.Kind()]:
		return x // int(i) of an int64, bool(b): the same C++ type
	case ti&types.IsInteger != 0 && fi&types.IsFloat != 0:
		f2i := &cppir.Name{Name: "gp::f2i", Args: []cppir.Node{typeName(ct)}}
		return &cppir.Call{Fun: f2i, Args: []cppir.Expr{x}}
	}
	return call(ct, x)
}
