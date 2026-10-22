package lower

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"slices"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/typedinterop/ctype"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// The table of argument adaptations of typed interop (ADR-130).
//
// Typed mode knows the C++ type of every parameter of a function it calls (clang says it), so
// the arguments that C++ would refuse but whose meaning is certain from the two types alone are
// converted by the compiler, in the one table below. A row is: the class of the Go argument, the
// class of the C++ parameter that the row adapts, and what the lowering writes. The rows:
//
//	string  (runtime)  const char*                 ::f(gp::cstr(s).gp_ptr())
//	string  (runtime)  std::string                 ::f(::std::string(s))
//	string  (any)      const wchar_t* char16_t* char32_t*
//	                                               ::f(gp::wide_cstr<wchar_t>(s).gp_ptr())
//	&x, x a Go number  T* (T another number type)  gp::cgo::call_out<T>(&x, "x", lambda(T* p))
//	                   const T*                    ::f(gp::cgo::scalar_in<T>(&x, "x"))
//	&p, p a *C.T       void**                      gp::cgo::call_void_out(p, lambda(void** q))
//
// (a lambda that makes the call, see outWrap)
//	untyped constant   an enum E                   ::f(static_cast<E>(3))
//
// An argument is adapted only when EVERY overload of the callee that the call can reach has the
// same row at that place (adaptationFor): then the call without the adaptation was ill-formed for
// each of them (the conversion does not exist in C++), and the adapted argument has exactly the
// parameter's type, so the C++ overload resolution can pick no other function than the one the
// user meant. A mixed set (f(const char*) next to f(std::string_view)) is left to C++ as before.
// What every row promises about lifetimes and values is in the ADR; what is NOT adapted (a char*
// that the callee writes, a []byte, a pointer to an element, a conversion that would narrow) stays
// the C++ compiler's error, or the lowering's own message when the argument looks like a row's but
// breaks its rule (scalarPtrRow).

// An argClass is the kind of Go argument that a row adapts.
type argClass int

const (
	acNone        argClass = iota
	acString               // a Go string that is not a constant
	acStringConst          // a Go string constant (already a C literal)
	acAddrScalar           // &x, x a variable (or field) of a Go number type
	acAddrCPtr             // &p, p a variable (or field) of type *C.T
	acNumConst             // an untyped number constant (an integer, a rune or a float)
	acFloat32              // a Go float32 value that is not a constant
	acFloat64              // a Go float64 value that is not a constant
)

// A verdict says what a parameter makes of an argument class.
type verdict int

const (
	vOther  verdict = iota // C++'s own rules decide, or it is not known: leave the call alone
	vIgnore                // can never take it: the overload is no candidate for this argument
	vAdapt                 // the row adapts it
)

// An adaptKey says which row and with what detail. The overloads of a call must all have the same
// key for an argument to be adapted (adaptationFor).
type adaptKey struct {
	row string
	// name is the C++ type that the row works with: the character type of a wide string, the
	// pointee of a scalar pointer, the enum, the integer or float of a conversion.
	name string
	// isConst: the pointee of a scalar pointer is const (the callee only reads it).
	isConst bool
	// kind is the Go kind of the parameter of a constant (numconst); name is then how the C type of
	// the parameter is spelled in the messages.
	kind types.BasicKind
}

// An adaptRow is one row of the table.
type adaptRow struct {
	name string
	args []argClass
	// param says what the C++ parameter p (written in the namespace scope) makes of an argument of
	// class ac.
	param func(g *gen, ac argClass, p *ctype.Type, scope string) (adaptKey, verdict)
	// lower returns the C++ of the adapted argument; false leaves the argument as it is.
	lower func(fg *funcGen, st *adaptSite) (cppir.Expr, bool)
}

// An adaptSite is the argument number i of the call that a row lowers.
type adaptSite struct {
	call *ast.CallExpr
	i    int
	arg  ast.Expr
	x    cppir.Expr // the argument as cArgs lowered it
	key  adaptKey
	name string // the callee's name
	sigs []calleeSig
	// outs collects the out-parameters of the call: a row that the callee writes through
	// (scalarptr, voidout) appends one and puts its parameter name in the call (outWrap).
	outs *[]outWrap
}

// An outWrap is an argument that the callee writes through. The call is made inside a function
// that holds the temporary, so that what the callee stored goes back to the variable when the call
// has returned, before the rest of the expression is evaluated, as in Go:
//
//	gp::cgo::call_out<int>(&x, "x", [&](int* gp_out0) -> decltype(auto) {
//		return ::f(a, gp_out0);
//	})
type outWrap struct {
	fun   cppir.Expr   // the helper, gp::cgo::call_out<int>
	args  []cppir.Expr // before the lambda: the variable (and its name for the messages)
	param cppir.Param  // the lambda's parameter: the pointer that the callee gets
}

// An outCall is what a call needs besides its arguments when an argument is written through: the
// wrappers, and the arguments that are evaluated before the variable is read.
type outCall struct {
	wraps []outWrap
	// early are the other arguments that contain a call. Go evaluates all the arguments, then makes
	// the call, and the callee reads the variable then: a call among the arguments that changes the
	// variable is seen. The value of an adapted &x is copied when the helper runs, so these are
	// evaluated first, in order, in locals of a function around the helper:
	//
	//	[&]() -> decltype(auto) {
	//		auto&& gp_in0 = g(&x);
	//		return gp::cgo::call_out<int>(&x, "x", [&](int* gp_out0) -> decltype(auto) {
	//			return ::f(gp_out0, std::forward<decltype(gp_in0)>(gp_in0));
	//		});
	//	}()
	early []earlyArg
}

// An earlyArg is an argument that is evaluated into a local before the call.
type earlyArg struct {
	name string
	x    cppir.Expr
}

// wrapOuts wraps the call in the lambdas of the out-parameters, the first out outermost (so the copies
// go back in the opposite order of the arguments), and in the function that evaluates the early
// arguments.
func wrapOuts(callExpr cppir.Expr, oc outCall) cppir.Expr {
	for i := len(oc.wraps) - 1; i >= 0; i-- {
		o := oc.wraps[i]
		lambda := &cppir.Lambda{
			Default: "&",
			Params:  []cppir.Param{o.param},
			Result:  typeName("decltype(auto)"),
			Body:    &cppir.Block{Stmts: []cppir.Stmt{&cppir.Return{X: callExpr}}, Inline: true},
		}
		callExpr = &cppir.Call{Fun: o.fun, Args: append(slices.Clone(o.args), lambda)}
	}
	if len(oc.early) == 0 {
		return callExpr
	}
	var stmts []cppir.Stmt
	for _, e := range oc.early {
		stmts = append(stmts, varDecl(cppir.Pos{}, typeName("auto&&"), e.name, e.x))
	}
	stmts = append(stmts, &cppir.Return{X: callExpr})
	return &cppir.Call{Fun: &cppir.Lambda{
		Default: "&",
		Result:  typeName("decltype(auto)"),
		Body:    &cppir.Block{Stmts: stmts, Inline: true},
	}}
}

// evaluateEarly moves the arguments of the call c that contain a call, and are not out-parameters,
// into locals that are evaluated before the helpers run (outCall.early), and puts the local in
// their place in args.
func (fg *funcGen) evaluateEarly(c *ast.CallExpr, args []cppir.Expr, oc *outCall) {
	for i, a := range c.Args {
		if i >= len(args) || isOutParamName(args[i], oc.wraps) || !containsCall(a) ||
			pointsIntoTemporary(args[i]) {
			continue
		}
		name := fg.inputName()
		oc.early = append(oc.early, earlyArg{name: name, x: args[i]})
		forward := []cppir.Node{call("decltype", cxxName(name))}
		args[i] = templateCall("std::forward", forward, cxxName(name))
	}
}

// isOutParamName reports whether x is the parameter of the lambda of one of the out-parameters.
func isOutParamName(x cppir.Expr, wraps []outWrap) bool {
	n, ok := x.(*cppir.Name)
	return ok && slices.ContainsFunc(wraps, func(o outWrap) bool { return o.param.Name == n.Name })
}

// pointsIntoTemporary reports whether x is the pointer of a holder that lives until the end of the
// full expression (gp::cstr(s).gp_ptr()): a local would end its life too early.
func pointsIntoTemporary(x cppir.Expr) bool {
	c, ok := x.(*cppir.Call)
	if !ok {
		return false
	}
	m, ok := c.Fun.(*cppir.Member)
	return ok && m.Name == "gp_ptr"
}

// containsCall reports whether the Go expression has a call in it.
func containsCall(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if _, ok := n.(*ast.CallExpr); ok {
			found = true
		}
		return !found
	})
	return found
}

// inputName returns the name of the next local of the early arguments of the function.
func (fg *funcGen) inputName() string {
	fg.outSeq++
	return "gp_in" + strconv.Itoa(fg.outSeq-1)
}

// outParam returns the parameter name of the lambda of the next out-parameter of the function.
func (fg *funcGen) outParam(pointee cppir.Type) cppir.Param {
	fg.outSeq++
	return cppir.Param{Type: &cppir.Ptr{Elem: pointee}, Name: "gp_out" + strconv.Itoa(fg.outSeq-1)}
}

var adaptTable []*adaptRow

// The table is set in init: the rows' lowerings call back into the lowering, which reads the table.
func init() {
	adaptTable = []*adaptRow{
		{name: "cstr", args: []argClass{acString}, param: cstrParam, lower: lowerCStr},
		{
			name: "stdstring", args: []argClass{acString},
			param: stdStringParam, lower: lowerStdString,
		},
		{
			name: "wide", args: []argClass{acString, acStringConst},
			param: wideParam, lower: lowerWide,
		},
		{
			name: "scalarptr", args: []argClass{acAddrScalar},
			param: scalarPtrParam, lower: lowerScalarPtr,
		},
		{
			name: "voidout", args: []argClass{acAddrCPtr},
			param: voidOutParam, lower: lowerVoidOut,
		},
		{
			name: "enum", args: []argClass{acNumConst},
			param: enumParam, lower: lowerEnumConst,
		},
		{
			name: "numconst", args: []argClass{acNumConst},
			param: numConstParam, lower: lowerNumConst,
		},
		{
			name: "floatconv", args: []argClass{acFloat32, acFloat64},
			param: floatConvParam, lower: lowerFloatConv,
		},
	}
}

// typedAdaptArgs returns the C++ of the arguments args of the call c of a C/C++ function with the
// rows of the table applied, and the out-parameters that the call must be wrapped in (wrapOuts);
// args is the result of cArgs for c.Args (after rawPointerArgs). It returns args unchanged when
// typed mode is off or nothing is adapted.
func (fg *funcGen) typedAdaptArgs(c *ast.CallExpr, args []cppir.Expr) ([]cppir.Expr, outCall) {
	g := fg.g
	if !g.typedOn() || c.Ellipsis.IsValid() {
		return args, outCall{}
	}
	var sigs []calleeSig
	var name string
	var known bool
	var asked bool
	var outs outCall
	for i, a := range c.Args {
		if i >= len(args) {
			break
		}
		ac := g.argClassOf(a)
		if ac == acNone {
			continue
		}
		if !asked {
			sigs, name, known = g.calleeSigs(c)
			asked = true
		}
		if !known || len(sigs) == 0 {
			return args, outCall{}
		}
		row, key, ok := g.adaptationFor(ac, sigs, i)
		if !ok {
			continue
		}
		site := &adaptSite{
			call: c, i: i, arg: a, x: args[i], key: key, name: name, sigs: sigs, outs: &outs.wraps,
		}
		if x, ok := row.lower(fg, site); ok {
			args[i] = x
		}
	}
	if len(outs.wraps) > 0 {
		fg.evaluateEarly(c, args, &outs)
	}
	return args, outs
}

// argClassOf returns the class of the Go argument a, or acNone.
func (g *gen) argClassOf(a ast.Expr) argClass {
	tv := g.info.Types[a]
	switch {
	case tv.Type == nil || tv.Type == types.Typ[types.Invalid]:
		// &p of a pointer to a C type has no Go type (the C part is invalid): only the form
		// decides.
	case tv.Value != nil && tv.Value.Kind() == constant.String:
		if g.isHolder(a) {
			return acNone
		}
		return acStringConst
	case tv.Value != nil && (tv.Value.Kind() == constant.Int || tv.Value.Kind() == constant.Float):
		if b, ok := tv.Type.(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
			return acNumConst
		}
		return acNone
	case tv.Value != nil:
		return acNone
	case g.goStringValue(a):
		return acString
	case g.goFloatValue(a) == types.Float32:
		return acFloat32
	case g.goFloatValue(a) == types.Float64:
		return acFloat64
	}
	u, ok := ast.Unparen(a).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return acNone
	}
	if g.isVariableName(u.X) {
		if _, ok := g.goNumberKind(u.X); ok {
			return acAddrScalar
		}
	}
	if g.isVariableName(u.X) && g.isCPtrVariable(u.X) {
		return acAddrCPtr
	}
	return acNone
}

// goFloatValue returns types.Float32 or types.Float64 if e is a value of exactly that Go type (not
// a constant, not a named type, not a C/C++ value), else types.Invalid.
func (g *gen) goFloatValue(e ast.Expr) types.BasicKind {
	tv, ok := g.info.Types[e]
	if !ok || tv.Value != nil || tv.Type == nil || !complete(tv.Type) || !tv.IsValue() {
		return types.Invalid
	}
	if g.pkg.Opaque.IsOpaque(e) {
		return types.Invalid
	}
	b, ok := tv.Type.(*types.Basic)
	if !ok || (b.Kind() != types.Float32 && b.Kind() != types.Float64) {
		return types.Invalid
	}
	return b.Kind()
}

// isVariableName reports whether e names a variable or a field of a struct: the forms of &x whose
// pointee is exactly one value (never an element of an array, which a C array parameter would take
// several of).
func (g *gen) isVariableName(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		v, ok := g.info.Uses[x].(*types.Var)
		return ok && !v.IsField()
	case *ast.SelectorExpr:
		if s := g.info.Selections[x]; s != nil {
			return s.Kind() == types.FieldVal
		}
		_, ok := g.info.Uses[x.Sel].(*types.Var) // pkg.v
		return ok
	}
	return false
}

// goNumberKind returns the basic kind of e if it is a variable of a Go number type (int, uint8,
// float32, ...: not a named type, not a C type, not a constant).
func (g *gen) goNumberKind(e ast.Expr) (types.BasicKind, bool) {
	tv, ok := g.info.Types[e]
	if !ok || tv.Value != nil || tv.Type == nil || !complete(tv.Type) || g.pkg.Opaque.IsOpaque(e) {
		return 0, false
	}
	b, ok := tv.Type.(*types.Basic)
	if !ok || b.Info()&types.IsNumeric == 0 || b.Info()&(types.IsComplex|types.IsUntyped) != 0 {
		return 0, false
	}
	return b.Kind(), true
}

// isCPtrVariable reports whether e is a variable declared with a pointer to a C/C++ type
// (var p *lib.T: a gp::cptr<T>; outPtrTarget's rule).
func (g *gen) isCPtrVariable(e ast.Expr) bool {
	star, ok := ast.Unparen(g.valueTypeExpr(e)).(*ast.StarExpr)
	if !ok || !g.isCType(star.X) {
		return false
	}
	_, nested := ast.Unparen(star.X).(*ast.StarExpr)
	return !nested
}

// adaptationFor returns the row that adapts the argument number i of class ac in a call of one of
// the overloads sigs, or false: every overload that the call can reach (enough parameters, or
// variadic) must either never take the argument (vIgnore) or have the same row with the same key.
func (g *gen) adaptationFor(ac argClass, sigs []calleeSig, i int) (*adaptRow, adaptKey, bool) {
	var found *adaptRow
	var key adaptKey
	for _, s := range sigs {
		choice, viable := g.sigChoice(ac, s, i)
		if !viable {
			continue // fewer parameters: not viable for this call
		}
		switch choice.verdict {
		case vIgnore:
			continue
		case vOther:
			return nil, key, false
		}
		if found != nil && (choice.row != found || choice.key != key) {
			return nil, key, false
		}
		found = choice.row
		key = choice.key
	}
	return found, key, found != nil
}

// A choice is what one overload makes of an argument: the row and key, and the verdict.
type choice struct {
	row     *adaptRow
	key     adaptKey
	verdict verdict
}

// sigChoice returns what the overload s makes of the argument number i of class ac; viable is false
// when the overload has too few parameters for the call.
func (g *gen) sigChoice(ac argClass, s calleeSig, i int) (c choice, viable bool) {
	switch {
	case i < len(s.t.Params):
		c.row, c.key, c.verdict = g.classifyParam(ac, s.t.Params[i], s.scope)
		if c.verdict == vAdapt && (ac == acAddrScalar || ac == acAddrCPtr) && s.arrayAt(i) {
			c.verdict = vOther // float col[3]: the callee takes several elements, &x is one
		}
		return c, true
	case s.t.Variadic && ac == acString:
		return choice{adaptTable[0], adaptKey{row: "cstr"}, vAdapt}, true // a char* for %s
	case s.t.Variadic:
		return choice{verdict: vOther}, true
	}
	return choice{}, false
}

// classifyParam combines the rows of the class ac for the parameter p: the row that adapts it, else
// vIgnore when some row knows that the argument can never convert (every verdict of vIgnore is a
// fact about the parameter's type), else vOther.
func (g *gen) classifyParam(
	ac argClass, p *ctype.Type, scope string,
) (*adaptRow, adaptKey, verdict) {
	all := vOther
	for _, r := range adaptTable {
		if !hasClass(r.args, ac) {
			continue
		}
		k, v := r.param(g, ac, p, scope)
		switch v {
		case vAdapt:
			return r, k, vAdapt
		case vIgnore:
			all = vIgnore
		}
	}
	return nil, adaptKey{}, all
}

func hasClass(list []argClass, ac argClass) bool {
	for _, c := range list {
		if c == ac {
			return true
		}
	}
	return false
}

// mapParam maps the parameter type p (written in the namespace scope) to the foreign view.
func (g *gen) mapParam(p *ctype.Type, scope string) ftype.Mapped {
	return g.interop.typed.rep.Mapper.MapIn(p, scope)
}
