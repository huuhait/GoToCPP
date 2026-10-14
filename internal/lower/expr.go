package lower

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Expressions (Phase 02 README §2.3): constants (folded by go/types), variables, calls of
// user functions, of the builtins len, print, println and panic and of fmt.Print, Println and
// Printf (gp::fmtshim), the byte s[i] of a string, the substring s[a:b], conversions between
// basic types (also string(r)), every operator of the Phase 02 subset with string
// concatenation (ops.go), pointers (step 3.3): *p, &x (addr.go), new(T), nil, p == q, and
// a boxed variable x (box.go), which is (*x), slices (step 3.5, slices.go) and arrays (step
// 3.6, arrays.go). The rest is
// reported as "not supported yet": the phase gates of internal/check reject it before, with a
// message for the user.
//
// Every expression that is not a constant has exactly its Go type in C++ (int8 is int8_t, not
// the int that C++ arithmetic gives: ops.go casts the small integer types back), so that a
// runtime template that deduces its argument types (gp::println, gp::div) sees the Go type.
// A constant gets its Go type only where exactExpr asks for it.
//
// Go's order of evaluation is order.go's: before the expressions of a statement are
// translated, fg.order moves the calls that need it into gp_ temporaries, and fg.expr then
// writes a temporary's name for its expression (fg.temps).
//
// An expression is a cppir.Expr. Where an operand needs parentheses that C++'s precedence
// doesn't (for the reader, or because g++ -Wall asks for them), lowering adds a cppir.Paren
// (at); the printer adds the ones that C++ needs.

// The C++ operator precedences (cppir.Prec), higher binds tighter.
const (
	precLogOr  = cppir.PrecLogOr
	precLogAnd = cppir.PrecLogAnd
	precBitOr  = cppir.PrecBitOr
	precBitXor = cppir.PrecBitXor
	precBitAnd = cppir.PrecBitAnd
	precShift  = cppir.PrecShift
	precUnary  = cppir.PrecUnary
)

// placeholder stands for an expression that couldn't be translated (an error was recorded).
var placeholder = cxxName("gp_unsupported")

// at returns x as an operand that needs at least the precedence prec: in parentheses if x
// binds less tightly.
func at(x cppir.Expr, prec int) cppir.Expr {
	if cppir.Prec(x) < prec {
		return &cppir.Paren{X: x}
	}
	return x
}

// A funcGen lowers the body of one function (or of gp_var_init).
type funcGen struct {
	g     *gen
	names *funcNames
	fn    *funcInfo // nil in gp_var_init

	flow       *flow               // where break and continue go (nil in gp_var_init)
	contLabels map[ast.Stmt]string // the label at the end of a loop's body, for continue

	temps   map[ast.Expr]string // the expressions that order.go put into temporaries
	ordered map[ast.Expr]bool   // the calls that order.go ordered
	force   map[ast.Expr]bool   // the calls that order.go must put into temporaries
	moves   map[ast.Expr]bool   // the slices that s = append(s, ...) and s = s[a:b] move (slices.go)

	// boxed are the variables of the function that live in a box (box.go): nil in
	// gp_var_init, which has none.
	boxed map[*types.Var]boxReason

	// out are the open statement lists, innermost last (stmt.go: add, push, pop).
	out []frame

	// shadows counts the Scopes that declarations of shadowing variables opened and that
	// are still open (declLines); fresh reports that nothing is added yet to the C++ block
	// that was opened last.
	shadows int
	fresh   bool
}

// newFuncGen returns the funcGen of a function (fn and flow are nil in gp_var_init).
func newFuncGen(g *gen, names *funcNames, fn *funcInfo) *funcGen {
	fg := &funcGen{
		g: g, names: names, fn: fn,
		contLabels: make(map[ast.Stmt]string),
		temps:      make(map[ast.Expr]string),
		ordered:    make(map[ast.Expr]bool),
		force:      make(map[ast.Expr]bool),
		moves:      make(map[ast.Expr]bool),
	}
	if fn != nil {
		fg.boxed = fn.boxed
	}
	return fg
}

// isBoxed reports whether v is a variable of the function that lives in a box (box.go).
func (fg *funcGen) isBoxed(v *types.Var) bool {
	return fg.boxed[v] != 0
}

// deref returns *x, the variable that the pointer x points to.
func deref(x cppir.Expr) cppir.Expr {
	return &cppir.Unary{Op: "*", X: x}
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
	return placeholder.Name
}

// expr returns the C++ of the expression e, where C++ converts it to the type that it needs
// (an operand, an initializer, the argument of a user function). Use exactExpr where C++
// deduces the type from the expression.
func (fg *funcGen) expr(e ast.Expr) cppir.Expr {
	if t, ok := fg.temps[e]; ok {
		return cxxName(t)
	}
	// A constant is folded: go/types computed its value and the type it converts to.
	if tv, ok := fg.g.info.Types[e]; ok && tv.Value != nil {
		return fg.g.constant(tv.Value, fg.g.untypedCount(e, tv.Type), e.Pos(), false)
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
	case *ast.SliceExpr:
		return fg.slice(e)
	case *ast.StarExpr:
		// *p: gp::ptr's operator* panics like Go for a nil p.
		return deref(fg.expr(e.X))
	case *ast.CompositeLit:
		if isArray(fg.g.info.TypeOf(e)) {
			return fg.arrayLit(e) // arrays.go
		}
		return fg.compositeLit(e)
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
func (fg *funcGen) exactExpr(e ast.Expr) cppir.Expr {
	tv, ok := fg.g.info.Types[e]
	switch {
	case ok && tv.Value != nil:
		return fg.g.constant(tv.Value, fg.g.untypedCount(e, tv.Type), e.Pos(), true)
	case ok && tv.IsNil() && isSlice(tv.Type):
		// nullptr would be a std::nullptr_t: gp::slice<T>(), a nil slice.
		return construct(fg.g.cxxType(tv.Type, e.Pos()))
	}
	return fg.expr(e)
}

// intExpr returns the C++ of e, an index, a bound of a slice expression or a length (make): a
// constant is a literal of type int, which every runtime template takes (go/types may record
// an untyped float constant, s[2.0], or a byte, s['a']); the rest is fg.expr's.
func (fg *funcGen) intExpr(e ast.Expr) cppir.Expr {
	if v := fg.constValue(e); v != nil {
		return fg.g.constant(v, types.Typ[types.Int], e.Pos(), false)
	}
	return fg.expr(e)
}

// exprs returns the C++ of each expression. With exact, constants get their exact type
// (exactExpr).
func (fg *funcGen) exprs(list []ast.Expr, exact bool) []cppir.Expr {
	xs := make([]cppir.Expr, len(list))
	for i, e := range list {
		if exact {
			xs[i] = fg.exactExpr(e)
		} else {
			xs[i] = fg.expr(e)
		}
	}
	return xs
}

func (fg *funcGen) ident(id *ast.Ident) cppir.Expr {
	switch obj := fg.g.info.Uses[id].(type) {
	case *types.Var:
		if fg.isBoxed(obj) {
			// The variable is in its box: (*x).
			return &cppir.Paren{X: deref(cxxName(fg.cname(obj, id.Pos())))}
		}
		return cxxName(fg.cname(obj, id.Pos()))
	case *types.Func:
		fg.g.unsupported(id.Pos(), "function values (Phase 03)")
	case *types.Nil:
		// A nil pointer or slice: C++ converts nullptr to gp::ptr<T> and gp::slice<T> (where
		// C++ deduces the type, exactExpr writes gp::slice<T>()). The other nil values (maps,
		// funcs, interfaces) come with their types.
		if t := fg.g.info.TypeOf(id); t != nil && (isPointer(t) || isSlice(t) || t == types.Typ[types.UntypedNil]) {
			return lit("nullptr")
		}
		fg.g.unsupported(id.Pos(), "nil of type "+types.TypeString(fg.g.info.TypeOf(id), types.RelativeTo(fg.g.pkg.Types)))
	default:
		fg.g.unsupported(id.Pos(), "the identifier "+id.Name)
	}
	return placeholder
}

// call returns the C++ of a function call.
func (fg *funcGen) call(c *ast.CallExpr) cppir.Expr {
	if tv := fg.g.info.Types[c.Fun]; tv.IsType() {
		return fg.conversion(c, tv.Type)
	}
	if !fg.ordered[c] {
		fg.g.errorf(c.Pos(), "internal error: a call whose evaluation order was not decided")
		return placeholder
	}
	if c.Ellipsis.IsValid() && !fg.isBuiltin(c, "append") {
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
				for _, a := range c.Args {
					if t := fg.g.info.TypeOf(a); t != nil && (isSlice(t) || isArray(t)) {
						// Go prints [len/cap]0xc000012345 for a slice, with an address of
						// Go's heap (D23), and rejects an array: internal/check rejects
						// both, so this is only a safety net.
						fg.g.unsupported(a.Pos(), "printing a slice or an array with "+obj.Name())
						return placeholder
					}
				}
			case "len", "cap", "panic", "new", "make", "append", "copy", "clear":
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

	if template {
		for _, a := range c.Args {
			if t := fg.g.info.TypeOf(a); t != nil && isPointer(t) {
				fg.g.unsupported(a.Pos(), "printing a pointer (Phase 03 step 3.15)")
				return placeholder
			}
			if t := fg.g.info.TypeOf(a); t != nil && isSlice(t) {
				fg.g.unsupported(a.Pos(), "printing a slice (Phase 03 step 3.15)")
				return placeholder
			}
			if t := fg.g.info.TypeOf(a); t != nil && isArray(t) {
				fg.g.unsupported(a.Pos(), "printing an array (Phase 03 step 3.15)")
				return placeholder
			}
		}
	}

	// f(g()), where g has several results: the arguments are g's results.
	// (std::apply names the function without calling it, so ADL doesn't apply.)
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		arg := fg.expr(c.Args[0])
		var fn cppir.Expr = cxxName(callee)
		if template {
			// std::apply needs a callable object. The elements of the tuple have their
			// Go types: [](const auto&... gp_a) { gp::println(gp_a...); }
			body := &cppir.ExprStmt{X: call(callee, &cppir.Expand{X: cxxName("gp_a")})}
			fn = &cppir.Lambda{
				Params: []cppir.Param{{Type: &cppir.Ref{Elem: &cppir.Const{Elem: typeName("auto")}}, Pack: true, Name: "gp_a"}},
				Body:   &cppir.Block{Stmts: []cppir.Stmt{body}, Inline: true},
			}
		}
		return &cppir.Call{Fun: cxxName("std::apply"), Args: []cppir.Expr{fn, arg}}
	}
	return &cppir.Call{Fun: cxxName(callee), Args: fg.exprs(c.Args, template)}
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
// verbs %v %d %s %t %f and %%, without flags, width, precision or argument indexes, and any
// character that is no verb of Go's fmt (%z, %!, %é), for which fmtshim writes Go's error form
// %!z(int=5), as Go does. If not, it returns the first directive that it doesn't know. The
// format is parsed like Go's fmt.(*pp).doPrintf and the phase gate (internal/check,
// checkFormat and verbPhase), so the emitter accepts exactly what the gate lets through in
// Phase 02. (A % at the end is Go's %!(NOVERB), which fmtshim prints too.)
func printfVerbs(format string) (string, bool) {
	for i := 0; i < len(format); {
		if format[i] != '%' {
			i++
			continue
		}
		start := i
		i++
		if i < len(format) && (strings.IndexByte("#0+- [*", format[i]) >= 0 || '1' <= format[i] && format[i] <= '9' ||
			format[i] == '.' && i+1 < len(format)) {
			// Flags, an argument index, a width or a precision (a . at the end is the verb).
			end := i + 1
			for end < len(format) && !isLetter(format[end-1]) {
				end++
			}
			return format[start:end], false
		}
		if i == len(format) {
			break
		}
		verb, size := utf8.DecodeRuneInString(format[i:])
		i += size
		if strings.ContainsRune(goVerbs, verb) && !strings.ContainsRune("vdstf%", verb) {
			return format[start:i], false
		}
	}
	return "", true
}

// goVerbs are the verbs of Go's fmt (and %w of fmt.Errorf), as internal/check has them.
const goVerbs = "vTtbcdoOqxXUeEfFgGspw%"

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
//   - Strings: "..."_gs, a gp::string over the literal's bytes (cppir.CString's escapes).
//   - Booleans: true and false.
func (g *gen) constant(v constant.Value, t types.Type, pos token.Pos, exact bool) cppir.Expr {
	b := basicOf(t)
	if b == nil {
		g.cxxType(t, pos) // reports it
		return placeholder
	}
	info := b.Info()
	switch {
	case info&types.IsBoolean != 0:
		return lit(strconv.FormatBool(constant.BoolVal(v)))
	case info&types.IsString != 0:
		return lit(cppir.CString(constant.StringVal(v)) + "_gs")
	case info&types.IsInteger != 0:
		x, plain := intLiteral(v, b.Kind(), info&types.IsUnsigned != 0)
		if exact && plain {
			// A plain literal is a C++ int: int64_t(5).
			return call(basicTypes[b.Kind()], x)
		}
		return x
	case info&types.IsFloat != 0:
		return floatLiteral(v, b.Kind() == types.Float32)
	}
	g.unsupported(pos, "constants of type "+b.Name())
	return placeholder
}

// intLiteral returns the C++ of the integer constant v of the kind, and whether it is a
// plain decimal literal (a C++ int, maybe negated) rather than one of the exact type.
func intLiteral(v constant.Value, kind types.BasicKind, unsigned bool) (cppir.Expr, bool) {
	v = constant.ToInt(v)
	if unsigned {
		u, _ := constant.Uint64Val(v)
		switch {
		case u <= math.MaxInt32:
			return lit(strconv.FormatUint(u, 10)), true
		case kind == types.Uint32:
			return call("UINT32_C", lit(strconv.FormatUint(u, 10))), false
		}
		return call("UINT64_C", lit(strconv.FormatUint(u, 10))), false
	}
	i, _ := constant.Int64Val(v)
	switch {
	case -math.MaxInt32 <= i && i <= math.MaxInt32:
		if i < 0 {
			return &cppir.Unary{Op: "-", X: lit(strconv.FormatInt(-i, 10))}, true
		}
		return lit(strconv.FormatInt(i, 10)), true
	case kind == types.Int32: // only math.MinInt32 is left
		return lit("INT32_MIN"), false
	case i == math.MinInt64:
		return lit("INT64_MIN"), false
	case i < 0:
		return &cppir.Unary{Op: "-", X: call("INT64_C", lit(strconv.FormatInt(-i, 10)))}, false
	}
	return call("INT64_C", lit(strconv.FormatInt(i, 10))), false
}

func floatLiteral(v constant.Value, float32 bool) cppir.Expr {
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
	if neg, ok := strings.CutPrefix(s, "-"); ok {
		return &cppir.Unary{Op: "-", X: lit(neg)}
	}
	return lit(s)
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
	"StarExpr":       "pointer types",
	"KeyValueExpr":   "key: value",
	"TypeSwitchStmt": "type switches (Phase 05)",
	"SelectStmt":     "select statements (Phase 08)",
	"GoStmt":         "go statements (Phase 08)",
	"DeferStmt":      "defer statements (Phase 03)",
	"SendStmt":       "send statements (Phase 08)",
}

// nodeKind names the kind of the AST node n for an error message.
func nodeKind(n ast.Node) string {
	name := strings.TrimPrefix(fmt.Sprintf("%T", n), "*ast.")
	if s, ok := nodeKinds[name]; ok {
		return s
	}
	return name
}
