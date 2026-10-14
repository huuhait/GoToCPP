package cppir

// C++ operator precedences, higher binds tighter.
const (
	PrecComma   = 1 + iota // ,
	PrecAssign             // = op= ?: (right to left)
	PrecLogOr              // ||
	PrecLogAnd             // &&
	PrecBitOr              // |
	PrecBitXor             // ^
	PrecBitAnd             // &
	PrecEq                 // == !=
	PrecRel                // < <= > >=
	PrecCmp3               // <=>
	PrecShift              // << >>
	PrecAdd                // + -
	PrecMul                // * / %
	PrecUnary              // -x !x ~x *p &x ++x
	PrecPostfix            // f(x) a[i] a.b a->b x++ T(x) T{x} static_cast<T>(x)
	PrecPrimary            // names, literals, (e), lambdas, {a, b}
)

// binaryPrecs are the precedences of the binary operators.
var binaryPrecs = map[string]int{
	",":  PrecComma,
	"||": PrecLogOr,
	"&&": PrecLogAnd,
	"|":  PrecBitOr,
	"^":  PrecBitXor,
	"&":  PrecBitAnd,
	"==": PrecEq, "!=": PrecEq,
	"<": PrecRel, "<=": PrecRel, ">": PrecRel, ">=": PrecRel,
	"<=>": PrecCmp3,
	"<<":  PrecShift, ">>": PrecShift,
	"+": PrecAdd, "-": PrecAdd,
	"*": PrecMul, "/": PrecMul, "%": PrecMul,
}

// Prec returns the C++ precedence of the outermost operator of e (PrecPrimary for a name or
// a literal): an operand of a place that needs more gets parentheses.
func Prec(e Expr) int {
	switch e := e.(type) {
	case *Binary:
		if p, ok := binaryPrecs[e.Op]; ok {
			return p
		}
		return PrecComma // not an operator that Print knows: parenthesize it everywhere
	case *Assign, *Cond, *Designated:
		return PrecAssign
	case *Unary:
		return PrecUnary
	case *Call, *Index, *Member, *Postfix, *Cast:
		return PrecPostfix
	case *InitList:
		if e.Type != nil {
			return PrecPostfix
		}
	}
	return PrecPrimary
}
