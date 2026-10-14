package cppgen

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"strconv"
	"strings"
)

// Expressions (Phase 02 README §2.3): constants (folded by go/types), variables, calls of
// user functions, of the builtins len, print, println and panic and of fmt.Print, Println and
// Printf (gp::fmtshim), the byte s[i] of a string, conversions between basic types, and every
// operator of the Phase 02 subset (ops.go). The rest is reported as "not supported yet": the
// phase gates of internal/check reject it before, with a message for the user.
//
// Every expression that is not a constant has exactly its Go type in C++ (int8 is int8_t, not
// the int that C++ arithmetic gives: ops.go casts the small integer types back), so that a
// runtime template that deduces its argument types (gp::println, gp::div) sees the Go type.
// A constant gets its Go type only where exactExpr asks for it.
//
// Not done yet: Go's left-to-right order of calls (step 2.5). Until then the C++ evaluates the
// arguments of a call and the operands of an operator in the order the C++ compiler picks,
// which is silently wrong for calls with side effects: fmt.Println(next(), next()) prints
// "2 1" with g++ (and "1 2" with clang++ and Go). The hook for step 2.5: every operand and
// every argument is translated by fg.expr or fg.exactExpr, from left to right, so the
// ordering pass can move a call into a gp_ temporary there (or rewrite the AST before).
//
// An expression is a cexpr: its C++ text and the C++ precedence of its outermost operator, so
// that an operand gets parentheses exactly where C++ would group it differently from the Go
// AST.

// C++ operator precedences, higher binds tighter.
const (
	precComma   = 1 + iota // ,
	precAssign             // = ?: (right to left)
	precLogOr              // ||
	precLogAnd             // &&
	precBitOr              // |
	precBitXor             // ^
	precBitAnd             // &
	precEq                 // == !=
	precRel                // < <= > >=
	precCmp3               // <=>
	precShift              // << >>
	precAdd                // + -
	precMul                // * / %
	precUnary              // -x !x ~x and casts
	precPostfix            // f(x) a[i] a.b
	precPrimary            // names, literals, (e)
)

// A cexpr is a C++ expression.
type cexpr struct {
	text string
	prec int // the precedence of its outermost operator (precPrimary for a name or literal)
}

// placeholder stands for an expression that couldn't be translated (an error was recorded).
var placeholder = cexpr{"gp_unsupported", precPrimary}

// at returns the text of x as an operand that needs at least the precedence prec: in
// parentheses if x binds less tightly.
func (x cexpr) at(prec int) string {
	if x.prec < prec {
		return "(" + x.text + ")"
	}
	return x.text
}

// A funcGen writes the body of one function (or of gp_var_init).
type funcGen struct {
	g     *gen
	names *funcNames
	fn    *funcInfo // nil in gp_var_init
}

// cname returns the C++ name of the variable or function obj.
func (fg *funcGen) cname(obj types.Object, pos token.Pos) string {
	if n, ok := fg.names.names[obj]; ok {
		return n
	}
	if n, ok := fg.g.names[obj]; ok {
		return n
	}
	fg.g.errorf(pos, "internal error: no C++ name for %s", obj.Name())
	return placeholder.text
}

// expr returns the C++ of the expression e, where C++ converts it to the type that it needs
// (an operand, an initializer, the argument of a user function). Use exactExpr where C++
// deduces the type from the expression.
func (fg *funcGen) expr(e ast.Expr) cexpr {
	// A constant is folded: go/types computed its value and the type it converts to.
	if tv, ok := fg.g.info.Types[e]; ok && tv.Value != nil {
		return fg.g.constant(tv.Value, tv.Type, e.Pos(), false)
	}
	switch e := e.(type) {
	case *ast.ParenExpr:
		return fg.expr(e.X) // the C++ gets parentheses where it needs them
	case *ast.Ident:
		return fg.ident(e)
	case *ast.CallExpr:
		return fg.call(e)
	case *ast.BinaryExpr:
		return fg.binary(e)
	case *ast.UnaryExpr:
		return fg.unary(e)
	case *ast.IndexExpr:
		return fg.index(e)
	}
	fg.g.unsupported(e.Pos(), nodeKind(e))
	return placeholder
}

// exactExpr returns the C++ of the expression e for a place where C++ deduces its type from
// the expression: an argument of a runtime template (gp::println, gp::fmtshim::Printf), or
// the initializer of an auto temporary. There a constant must have its Go type: a plain 5 is
// a C++ int, so Printf("%s", 5) would say int32 instead of Go's int. Every such place calls
// exactExpr; the other expressions already have their Go type in C++ (ops.go casts the
// small integer types back).
func (fg *funcGen) exactExpr(e ast.Expr) cexpr {
	if tv, ok := fg.g.info.Types[e]; ok && tv.Value != nil {
		return fg.g.constant(tv.Value, tv.Type, e.Pos(), true)
	}
	return fg.expr(e)
}

// exprs returns the C++ of each expression, joined with ", ". With exact, constants get their
// exact type (exactExpr).
func (fg *funcGen) exprs(list []ast.Expr, exact bool) string {
	parts := make([]string, len(list))
	for i, e := range list {
		if exact {
			parts[i] = fg.exactExpr(e).text
		} else {
			parts[i] = fg.expr(e).text
		}
	}
	return strings.Join(parts, ", ")
}

func (fg *funcGen) ident(id *ast.Ident) cexpr {
	switch obj := fg.g.info.Uses[id].(type) {
	case *types.Var:
		return cexpr{fg.cname(obj, id.Pos()), precPrimary}
	case *types.Func:
		fg.g.unsupported(id.Pos(), "function values (Phase 03)")
	case *types.Nil:
		fg.g.unsupported(id.Pos(), "nil")
	default:
		fg.g.unsupported(id.Pos(), "the identifier "+id.Name)
	}
	return placeholder
}

// call returns the C++ of a function call.
func (fg *funcGen) call(c *ast.CallExpr) cexpr {
	if tv := fg.g.info.Types[c.Fun]; tv.IsType() {
		return fg.conversion(c, tv.Type)
	}
	if c.Ellipsis.IsValid() {
		fg.g.unsupported(c.Ellipsis, "f(s...) (Phase 03)")
		return placeholder
	}
	var callee string
	template := false // the callee is a runtime template, which deduces its argument types
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		switch obj := fg.g.info.Uses[fun].(type) {
		case *types.Builtin:
			switch obj.Name() {
			case "print", "println":
				callee, template = "gp::"+obj.Name(), true
			case "len", "panic":
				return fg.builtin(c, obj.Name())
			default:
				fg.g.unsupported(c.Pos(), "the builtin "+obj.Name())
				return placeholder
			}
		case *types.Func:
			if obj.Signature().Variadic() {
				fg.g.unsupported(c.Pos(), "variadic functions (Phase 03)")
				return placeholder
			}
			callee = fg.cname(obj, fun.Pos())
			if len(c.Args) != 1 || !isTuple(fg.g.info.TypeOf(c.Args[0])) {
				callee = fg.g.funcCallName(callee, fg.argTypes(c.Args))
			}
		default:
			fg.g.unsupported(c.Pos(), "calls of function values (Phase 03)")
			return placeholder
		}
	case *ast.SelectorExpr:
		callee = fg.pkgFunc(c, fun)
		if callee == "" {
			return placeholder
		}
		template = true
	default:
		fg.g.unsupported(c.Pos(), "calls of "+nodeKind(fun))
		return placeholder
	}

	// f(g()), where g has several results: the arguments are g's results.
	// (std::apply names the function without calling it, so ADL doesn't apply.)
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		arg := fg.expr(c.Args[0]).text
		if template {
			// std::apply needs a callable object. The elements of the tuple have their
			// Go types.
			callee = "[](const auto&... gp_a) { " + callee + "(gp_a...); }"
		}
		return cexpr{"std::apply(" + callee + ", " + arg + ")", precPostfix}
	}
	return cexpr{callee + "(" + fg.exprs(c.Args, template) + ")", precPostfix}
}

// isTuple reports whether t is the result list of a call with several results.
func isTuple(t types.Type) bool {
	tuple, ok := t.(*types.Tuple)
	return ok && tuple.Len() > 1
}

// argTypes returns the Go types of the arguments.
func (fg *funcGen) argTypes(args []ast.Expr) []types.Type {
	ts := make([]types.Type, len(args))
	for i, a := range args {
		ts[i] = fg.g.info.TypeOf(a)
	}
	return ts
}

// funcCallName returns how a call names the package-level function whose C++ name is name,
// when its arguments have the Go types args.
//
// A call with an unqualified name, f(x), also finds functions through argument-dependent
// lookup (ADL): C++ looks for f in the namespaces of the arguments' class types too. So a
// Go function len(s string) called as len(s), with s a gp::string, finds gp::len as well, and
// the call is ambiguous; a gp::slice argument would find gp::append and gp::copy, a
// std::vector (Phase 04) std::sort. A qualified name, gpkg_main::len(s), turns ADL off. ADL
// looks only at class (and enum) types: when every argument has a fundamental C++ type
// (bool and the number types), the plain name is safe, and it stays, because it is easier to
// read. Every call by the name of a function must go through here: of the package's
// functions now, and from Phase 03 on of methods, other packages' functions and func values.
func (g *gen) funcCallName(name string, args []types.Type) string {
	for _, t := range args {
		if b := basicOf(t); b == nil || b.Info()&types.IsString != 0 {
			return g.ns + "::" + name
		}
	}
	return name
}

// fmtFuncs are the functions of package fmt that the runtime's stand-in has (gp::fmtshim, until
// Phase 05 and 09 bring the real fmt).
var fmtFuncs = map[string]bool{"Print": true, "Println": true, "Printf": true}

// pkgFunc returns the C++ name of the function of another package that c calls, pkg.F, or ""
// (with an error) if there is none yet.
func (fg *funcGen) pkgFunc(c *ast.CallExpr, sel *ast.SelectorExpr) string {
	fn, _ := fg.g.info.Uses[sel.Sel].(*types.Func)
	if fn == nil || fn.Pkg() == nil || fg.g.info.Selections[sel] != nil {
		fg.g.unsupported(c.Pos(), "method calls (Phase 03)")
		return ""
	}
	if fn.Pkg().Path() != "fmt" || !fmtFuncs[fn.Name()] {
		fg.g.unsupported(c.Pos(), fn.Pkg().Name()+"."+fn.Name()+": Phase 02 has only fmt.Print, fmt.Println and fmt.Printf")
		return ""
	}
	if fn.Name() == "Printf" && len(c.Args) > 0 {
		if tv := fg.g.info.Types[c.Args[0]]; tv.Value != nil && tv.Value.Kind() == constant.String {
			if verb, ok := printfVerbs(constant.StringVal(tv.Value)); !ok {
				fg.g.unsupported(c.Args[0].Pos(), "the fmt.Printf verb "+verb+" (Phase 03)")
				return ""
			}
		}
	}
	return "gp::fmtshim::" + fn.Name()
}

// printfVerbs reports whether the Printf format uses only what gp::fmtshim::Printf knows: the
// verbs %v %d %s %t %f and %%, without flags, width or precision. If not, it returns the
// first directive that it doesn't know. (A % at the end is Go's %!(NOVERB), which fmtshim
// prints too.)
func printfVerbs(format string) (string, bool) {
	for i := 0; i < len(format); i++ {
		if format[i] != '%' || i+1 == len(format) {
			continue
		}
		i++
		if !strings.ContainsRune("vdstf%", rune(format[i])) {
			end := i + 1
			for end < len(format) && !isLetter(format[end-1]) {
				end++
			}
			return format[i-1 : end], false
		}
	}
	return "", true
}

func isLetter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}

// constant returns the C++ literal of the constant v of type t (Phase 02 README §2.3): a
// literal of the C++ type of t, or one that C++ converts to it without a change of value
// wherever the generated code uses it.
//   - Integers: a small value (|v| < 2^31) is a plain decimal literal (7), a C++ int, which
//     C++ converts to the wider type exactly. A larger one is a literal of the exact type,
//     INT64_C(4611686018427387904), UINT32_C(4000000000), UINT64_C(...), so that its type
//     is right in arithmetic too; the smallest values are INT32_MIN and INT64_MIN (C++ has no
//     literal for them: -2147483648 is - applied to 2147483648, a long). With exact (where
//     C++ deduces the type from the literal: a runtime template such as gp::println, or
//     auto; see exactExpr), a plain literal gets its type too: int64_t(5), uint8_t(200).
//   - Floats: the shortest decimal that reads back as the same float64 (as a C++ double, with
//     .0 if it would read as an integer), and for float32 as the same float32, with an f
//     suffix: 0.1f. A float32 constant that is a double literal would print with the digits
//     of the double.
//   - Strings: "..."_gs, a gp::string over the literal's bytes (cString's escapes).
//   - Booleans: true and false.
func (g *gen) constant(v constant.Value, t types.Type, pos token.Pos, exact bool) cexpr {
	b := basicOf(t)
	if b == nil {
		g.cxxType(t, pos) // reports it
		return placeholder
	}
	info := b.Info()
	switch {
	case info&types.IsBoolean != 0:
		return cexpr{strconv.FormatBool(constant.BoolVal(v)), precPrimary}
	case info&types.IsString != 0:
		return cexpr{cString(constant.StringVal(v)) + "_gs", precPostfix}
	case info&types.IsInteger != 0:
		x := intLiteral(v, b.Kind(), info&types.IsUnsigned != 0)
		if exact && x.plain {
			// A plain literal is a C++ int: int64_t(5).
			return cexpr{basicTypes[b.Kind()] + "(" + x.text + ")", precPostfix}
		}
		return x.cexpr
	case info&types.IsFloat != 0:
		return floatLiteral(v, b.Kind() == types.Float32)
	}
	g.unsupported(pos, "constants of type "+b.Name())
	return placeholder
}

// An intLit is an integer literal: a cexpr, and whether it is a plain decimal literal (a C++
// int) rather than one of the exact type.
type intLit struct {
	cexpr
	plain bool
}

func intLiteral(v constant.Value, kind types.BasicKind, unsigned bool) intLit {
	v = constant.ToInt(v)
	if unsigned {
		u, _ := constant.Uint64Val(v)
		switch {
		case u <= math.MaxInt32:
			return intLit{cexpr{strconv.FormatUint(u, 10), precPrimary}, true}
		case kind == types.Uint32:
			return intLit{cexpr{"UINT32_C(" + strconv.FormatUint(u, 10) + ")", precPrimary}, false}
		}
		return intLit{cexpr{"UINT64_C(" + strconv.FormatUint(u, 10) + ")", precPrimary}, false}
	}
	i, _ := constant.Int64Val(v)
	switch {
	case -math.MaxInt32 <= i && i <= math.MaxInt32:
		if i < 0 {
			return intLit{cexpr{strconv.FormatInt(i, 10), precUnary}, true}
		}
		return intLit{cexpr{strconv.FormatInt(i, 10), precPrimary}, true}
	case kind == types.Int32: // only math.MinInt32 is left
		return intLit{cexpr{"INT32_MIN", precPrimary}, false}
	case i == math.MinInt64:
		return intLit{cexpr{"INT64_MIN", precPrimary}, false}
	case i < 0:
		return intLit{cexpr{"-INT64_C(" + strconv.FormatInt(-i, 10) + ")", precUnary}, false}
	}
	return intLit{cexpr{"INT64_C(" + strconv.FormatInt(i, 10) + ")", precPrimary}, false}
}

func floatLiteral(v constant.Value, float32 bool) cexpr {
	v = constant.ToFloat(v)
	var s string
	if float32 {
		f, _ := constant.Float32Val(v)
		s = strconv.FormatFloat(float64(f), 'g', -1, 32)
	} else {
		f, _ := constant.Float64Val(v)
		s = strconv.FormatFloat(f, 'g', -1, 64)
	}
	if !strings.ContainsAny(s, ".e") {
		s += ".0" // 3 would be an int
	}
	if float32 {
		s += "f"
	}
	if strings.HasPrefix(s, "-") {
		return cexpr{s, precUnary}
	}
	return cexpr{s, precPrimary}
}

// nodeKinds name the AST nodes in the "not supported yet" errors.
var nodeKinds = map[string]string{
	"BadExpr":        "a bad expression",
	"BasicLit":       "a literal",
	"CompositeLit":   "composite literals (Phase 03)",
	"FuncLit":        "function literals (Phase 03)",
	"SelectorExpr":   "selectors (Phase 03)",
	"IndexExpr":      "index expressions (Phase 03)",
	"IndexListExpr":  "instantiations (Phase 07)",
	"SliceExpr":      "slice expressions (Phase 03)",
	"TypeAssertExpr": "type assertions (Phase 05)",
	"StarExpr":       "pointers (Phase 03)",
	"KeyValueExpr":   "key: value",
	"IfStmt":         "if statements (step 2.4)",
	"ForStmt":        "for statements (step 2.4)",
	"RangeStmt":      "range loops (step 2.4)",
	"SwitchStmt":     "switch statements (step 2.4)",
	"TypeSwitchStmt": "type switches (Phase 05)",
	"SelectStmt":     "select statements (Phase 08)",
	"GoStmt":         "go statements (Phase 08)",
	"DeferStmt":      "defer statements (Phase 03)",
	"SendStmt":       "send statements (Phase 08)",
	"LabeledStmt":    "labels (step 2.4)",
	"BranchStmt":     "break, continue, goto and fallthrough (step 2.4)",
	"IncDecStmt":     "++ and -- (step 2.4)",
}

// nodeKind names the kind of the AST node n for an error message.
func nodeKind(n ast.Node) string {
	name := strings.TrimPrefix(fmt.Sprintf("%T", n), "*ast.")
	if s, ok := nodeKinds[name]; ok {
		return s
	}
	return name
}
