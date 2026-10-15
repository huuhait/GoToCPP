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

	"github.com/huuhait/gocpp/internal/cppir"
)

// Expressions (Phase 02 README §2.3): constants (folded by go/types), variables, calls of
// user functions, of the builtins len, print, println, panic, min and max (builtins.go) and of
// package fmt (gp::fmtshim, fmtcall.go), the byte s[i] of a string, the substring s[a:b],
// conversions between basic types (also string(r)), every operator of the Phase 02 subset
// with string concatenation (ops.go), pointers (step 3.3): *p, &x (addr.go), new(T), nil,
// p == q, and a boxed variable x (box.go), which is (*x), slices (step 3.5, slices.go), arrays
// (step 3.6, arrays.go) and structs (step 3.8, structs.go). The rest is reported as "not
// supported yet": the phase gates of internal/check reject it before, with a message for the
// user.
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
	// top is the outermost node of the code: the function declaration, or the package
	// variable's initializer in gp_var_init (funcInfo.top).
	top ast.Node

	flow       *flow               // where break and continue go (nil in gp_var_init)
	contLabels map[ast.Stmt]string // the label at the end of a loop's body, for continue

	temps   map[ast.Expr]string // the expressions that order.go put into temporaries
	ordered map[ast.Expr]bool   // the calls that order.go ordered
	force   map[ast.Expr]bool   // the calls that order.go must put into temporaries
	moves   map[ast.Expr]bool   // the slices that s = append(s, ...) and s = s[a:b] move (slices.go)
	stores  map[ast.Expr]bool   // the map entries m[k] that op= or ++ changes (maps.go: markStores)

	// recvs are the receivers that a deferred method call stored (methods.go: storeRecv).
	recvs map[*ast.SelectorExpr]cppir.Expr

	// boxed are the variables of the function that live in a box (box.go): nil in
	// gp_var_init, which has none.
	boxed map[*types.Var]boxReason
	// singleDefs are the local variables that only their declaration gives a value
	// (panicid.go), found on first use.
	singleDefs map[*types.Var]ast.Expr
	// inPlace are the boxed locals that an argument of fmt copies in place (inplace.go),
	// found on first use, for the code inPlaceTop.
	inPlace       map[*types.Var]bool
	inPlaceParams map[*types.Var]bool // the parameters, receivers and results in inPlaceTop
	inPlaceTop    ast.Node
	// commaOKs are the reads v, ok := m[k] of maps of C/C++ values (maps.go: commaOK).
	commaOKs map[*ast.IndexExpr]bool
	// autoTemps are the temporaries of the order that are copies of C/C++ values (cMoved).
	autoTemps map[string]bool
	// places are the expressions of the code placesTop that are used as places (order.go:
	// place), found on first use.
	places    map[ast.Expr]bool
	placesTop ast.Node

	// out are the open statement lists, innermost last (stmt.go: add, push, pop).
	out []frame

	// shadows counts the Scopes that declarations of shadowing variables opened and that
	// are still open (declLines); fresh reports that nothing is added yet to the C++ block
	// that was opened last.
	shadows int
	fresh   bool
	// loopInit: the statement being lowered is the init of a 3-clause for (bind).
	loopInit bool
	// deferCall is the call of the defer statement being ordered (order.go: gc evaluates its
	// func value and receiver first, from left to right), and opTarget the target of the x op=
	// y being ordered (emitPanic binds it once).
	deferCall *ast.CallExpr
	opTarget  ast.Expr
	// cTag: the switch being lowered has a C/C++ value as its tag (control.go: caseCond).
	cTag bool
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
		stores:     make(map[ast.Expr]bool),
	}
	if fn != nil {
		fg.boxed, fg.top = fn.boxed, fn.top
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
		if fg.g.tdecls().members[n] {
			// In a method, a member of its struct of that name would hide it (methods.go).
			return fg.g.ns + "::" + n
		}
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
	// Go may convert the value implicitly to another type there (step 3.9, named.go).
	return fg.implicitConv(e, fg.value(e))
}

// value returns the C++ of the expression e, which is not in a temporary, with its own type
// (expr).
func (fg *funcGen) value(e ast.Expr) cppir.Expr {
	// A constant is folded: go/types computed its value and the type it converts to.
	if tv, ok := fg.g.info.Types[e]; ok && tv.Value != nil {
		if fg.g.cConsts[e] {
			return fg.g.cConstant(tv.Value, tv.Type, e.Pos()) // a C/C++ value (cvalues.go)
		}
		return fg.g.constant(tv.Value, fg.g.untypedCount(e, tv.Type), e.Pos(), false)
	}
	if fg.g.pkg.Opaque.IsOpaque(e) {
		return fg.opaqueExpr(e) // a C/C++ value (cvalues.go)
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
		return fg.compLit(e) // structs.go
	case *ast.SelectorExpr:
		return fg.selector(e) // structs.go
	case *ast.FuncLit:
		return fg.funcValue(e) // closures.go
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
	case ok && tv.IsNil() && (isSlice(tv.Type) || isMap(tv.Type) || isFunc(tv.Type)):
		// nullptr would be a std::nullptr_t: gp::slice<T>(), a nil slice (gp::map<K, V>(), a
		// nil map; gp::func<Sig>(), a nil func).
		return construct(fg.g.cxxType(tv.Type, e.Pos()))
	}
	return fg.expr(e)
}

// intExpr returns the C++ of e, an index, a bound of a slice expression or a length (make): a
// constant is a literal of type int, which every runtime template takes (go/types may record
// an untyped float constant, s[2.0], or a byte, s['a']), and a value of a named integer type
// is its integer (x.gp_value()); the rest is fg.expr's.
func (fg *funcGen) intExpr(e ast.Expr) cppir.Expr {
	if v := fg.constValue(e); v != nil {
		return fg.g.constant(v, types.Typ[types.Int], e.Pos(), false)
	}
	return valueOf(fg.g.info.TypeOf(e), fg.expr(e)) // a named integer: its value (step 3.9)
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
		return fg.funcRef(obj, id.Pos()) // a function of the package as a value (closures.go)
	case *types.Nil:
		// A nil pointer, slice, map or func: C++ converts nullptr to gp::ptr<T>, gp::slice<T>,
		// gp::map<K, V> and gp::func<Sig> (where C++ deduces the type, exactExpr writes
		// gp::slice<T>()). The other nil values (interfaces) come with their types.
		if t := fg.g.info.TypeOf(id); t != nil && (isPointer(t) || isSlice(t) || isMap(t) || isFunc(t) || t == types.Typ[types.UntypedNil]) {
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
	for _, name := range []string{"complex", "real", "imag"} {
		if fg.isBuiltin(c, name) {
			return fg.complexBuiltin(c, name) // complex.go
		}
	}
	if conv := fg.g.pkg.Conversion(c); conv != nil {
		return fg.asConversion(c, conv) // of a C/C++ value (cvalues.go), not a call
	}
	if !fg.ordered[c] {
		fg.g.errorf(c.Pos(), "internal error: a call whose evaluation order was not decided")
		return placeholder
	}
	if x, ok := fg.cCall(c); ok {
		return x // of a C/C++ function (cvalues.go)
	}
	if fg.g.opaqueCallee(c) {
		return fg.opaqueCall(c) // of a member of a C/C++ value, or a C++ callable (cvalues.go)
	}
	if x := fg.weakMake(c); x != nil {
		return x // weak.Make(p), weak.Make[T](p) (weak.go)
	}
	if fg.funcValueCall(c) {
		return fg.callValue(c) // closures.go
	}
	var callee string
	var sig *types.Signature // of a function of the package
	template := false        // the callee is a runtime template, which deduces its argument types
	switch fun := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		switch obj := fg.g.info.Uses[fun].(type) {
		case *types.Builtin:
			switch obj.Name() {
			case "print", "println":
				callee, template = "gp::"+obj.Name(), true
				for _, a := range c.Args {
					if t := fg.g.info.TypeOf(a); t != nil && isFunc(t) {
						// Go prints the func's code address.
						fg.g.unsupported(a.Pos(), "printing a func value with "+obj.Name())
						return placeholder
					}
					if t := fg.g.info.TypeOf(a); t != nil && (isSlice(t) || isArray(t) || isMap(t)) {
						// Go prints [len/cap]0xc000012345 for a slice and 0xc000012345 for a
						// map, with an address of Go's heap (D23), and rejects an array:
						// internal/check rejects them, so this is only a safety net.
						what := "a slice or an array"
						if isMap(t) {
							what = "a map"
						}
						fg.g.unsupported(a.Pos(), "printing "+what+" with "+obj.Name())
						return placeholder
					}
				}
			case "len", "cap", "panic", "new", "make", "append", "copy", "clear", "delete":
				return fg.builtin(c, obj.Name())
			case "min", "max":
				return fg.minMax(c, obj.Name()) // builtins.go
			default:
				fg.g.unsupported(c.Pos(), "the builtin "+obj.Name())
				return placeholder
			}
		case *types.Func:
			sig = obj.Signature()
			callee = fg.cname(obj, fun.Pos())
			switch {
			case sig.Variadic():
				callee = fg.g.ns + "::" + callee // the last argument is a gp::slice (ADL)
			case len(c.Args) != 1 || !isTuple(fg.g.info.TypeOf(c.Args[0])):
				callee = fg.g.funcCallName(callee, fg.argTypes(c.Args))
			}
		default:
			fg.g.errorf(c.Pos(), "internal error: the call of %s, which is no function", fun.Name)
			return placeholder
		}
	case *ast.SelectorExpr:
		if sel := fg.g.info.Selections[fun]; sel != nil && sel.Kind() == types.MethodVal {
			return fg.methodCall(c, fun, sel) // methods.go
		}
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
			if t := fg.g.info.TypeOf(a); t != nil && hasFunc(t) {
				// Go prints a func's code address (closures.go).
				fg.g.unsupported(a.Pos(), "printing a func value")
				return placeholder
			}
		}
	}
	if fn := fg.fmtFunc(c); fn != nil {
		return fg.fmtCall(c, fn, callee) // fmtcall.go
	}

	// f(g()), where g has several results: the arguments are g's results. A function of the
	// package is named qualified, as funcCallName would (std::apply names it without calling
	// it, so ADL doesn't apply, but the qualified name is the one the other calls use).
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		arg := fg.expr(c.Args[0])
		var fn cppir.Expr = cxxName(callee)
		if sig != nil && !strings.HasPrefix(callee, fg.g.ns+"::") {
			fn = cxxName(fg.g.ns + "::" + callee)
		}
		if template {
			// std::apply needs a callable object. The elements of the tuple have their
			// Go types: [](const auto&... gp_a) { gp::println(gp_a...); }, and results with
			// C/C++ parts go through gp::cgo::print_arg (fmtcall.go).
			var elem cppir.Expr = cxxName("gp_a")
			if hasC(fg.g.info.TypeOf(c.Args[0])) {
				elem = call("gp::cgo::print_arg", elem)
			}
			body := &cppir.ExprStmt{X: call(callee, &cppir.Expand{X: elem})}
			fn = &cppir.Lambda{
				Params: []cppir.Param{{Type: &cppir.Ref{Elem: &cppir.Const{Elem: typeName("auto")}}, Pack: true, Name: "gp_a"}},
				Body:   &cppir.Block{Stmts: []cppir.Stmt{body}, Inline: true},
			}
		} else if x := fg.tupleArgs(c, sig, fn, false); x != nil {
			fn = x // results that Go converts to the parameters' types (step 3.9, named.go), or variadic
		}
		return &cppir.Call{Fun: cxxName("std::apply"), Args: []cppir.Expr{fn, arg}}
	}
	if sig != nil {
		return &cppir.Call{Fun: cxxName(callee), Args: fg.callArgs(c, sig)} // closures.go: variadics
	}
	if template {
		return &cppir.Call{Fun: cxxName(callee), Args: fg.printArgs(c.Args)} // print, println: fmtcall.go
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
// (bool, the integer and the float types; not complex numbers, which are gp::complex128), the
// plain name is safe, and it stays, because it is easier to read. Every call by the name of a
// function must go through here: of the package's functions now, and from Phase 03 on of
// methods, other packages' functions and func values.
func (g *gen) funcCallName(name string, args []types.Type) string {
	if strings.HasPrefix(name, g.ns+"::") {
		return name // qualified already (cname, in a method)
	}
	for _, t := range args {
		if b := basicOf(t); b == nil || b.Info()&(types.IsString|types.IsComplex) != 0 {
			return g.ns + "::" + name // gp::complex128 is a class too (gp::real)
		}
	}
	return name
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
//   - Complex numbers (step 3.15c): gp::complex128(1.0, 2.0), each part a float literal, and
//     gp::complex64(1.0f, 2.0f) (complex.go).
//   - Strings: "..."_gs, a gp::string over the literal's bytes (cppir.CString's escapes).
//   - Booleans: true and false.
//   - A named type over one of them (step 3.9): T(literal), Celsius(1.5).
func (g *gen) constant(v constant.Value, t types.Type, pos token.Pos, exact bool) cppir.Expr {
	if isNewtype(t) {
		// A constant of a named type (step 3.9) is the constant of its underlying type as a
		// value of the named type: Celsius(1.5), Name("x"_gs).
		return g.wrapNamed(t, g.constant(v, t.Underlying(), pos, false), pos)
	}
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
	case info&types.IsComplex != 0:
		kind := b.Kind()
		if kind == types.UntypedComplex {
			kind = types.Complex128
		}
		return complexConstant(v, kind) // complex.go
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
	"CompositeLit":   "composite literals",
	"FuncLit":        "function literals",
	"SelectorExpr":   "selectors",
	"IndexExpr":      "index expressions",
	"IndexListExpr":  "instantiations (Phase 07)",
	"SliceExpr":      "slice expressions",
	"TypeAssertExpr": "type assertions (Phase 05)",
	"StarExpr":       "pointer types",
	"KeyValueExpr":   "key: value",
	"TypeSwitchStmt": "type switches (Phase 05)",
	"SelectStmt":     "select statements (Phase 08)",
	"GoStmt":         "go statements (Phase 08)",
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
