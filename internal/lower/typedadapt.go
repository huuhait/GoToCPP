package lower

import (
	"go/ast"
	"go/types"
	"slices"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/typedinterop/adapt"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
)

// The argument adaptations of typed interop (ADR-130, ADR-134): the C++ that each row of the table
// writes.
//
// Typed mode knows the C++ type of every parameter of a function it calls (clang says it), so
// the arguments that C++ would refuse but whose meaning is certain from the two types alone are
// converted by the compiler. The table, which row applies to which argument and the errors of the
// rows are internal/typedinterop/adapt's (the interop checker asks the same function); this file
// writes what a row converts to. A row is: the class of the Go argument, the class of the C++
// parameter that the row adapts, and what the lowering writes. The rows:
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
// same row at that place (adapt.Decide): then the call without the adaptation was ill-formed for
// each of them (the conversion does not exist in C++), and the adapted argument has exactly the
// parameter's type, so the C++ overload resolution can pick no other function than the one the
// user meant. A mixed set (f(const char*) next to f(std::string_view)) is left to C++ as before.
// What every row promises about lifetimes and values is in the ADR; what is NOT adapted (a char*
// that the callee writes, a []byte, a pointer to an element, a conversion that would narrow) stays
// the C++ compiler's error, or adapt's own message when the argument looks like a row's but breaks
// its rule.

// An adaptSite is the argument number i of the call that a row lowers.
type adaptSite struct {
	call *ast.CallExpr
	i    int
	arg  ast.Expr
	x    cppir.Expr // the argument as cArgs lowered it
	key  adapt.Key
	name string // the callee's name
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

// A lowerer writes the C++ of an argument that the row adapts (adapt.Decide said Convert); false
// leaves the argument as it is.
type lowerer func(fg *funcGen, st *adaptSite) (cppir.Expr, bool)

// lowerers has the C++ of each row of adapt.Table. The decision (which row, and the errors of the
// row) is adapt.Decide's; this is only the code that a row writes.
var lowerers map[string]lowerer

// The map is set in init: the rows' lowerings call back into the lowering, which reads it.
func init() {
	lowerers = map[string]lowerer{
		adapt.RowCStr:      lowerCStr,
		adapt.RowStdString: lowerStdString,
		adapt.RowWide:      lowerWide,
		adapt.RowScalarPtr: lowerScalarPtr,
		adapt.RowVoidOut:   lowerVoidOut,
		adapt.RowEnum:      lowerEnumConst,
		adapt.RowFloatConv: lowerFloatConv,
	}
}

// typedAdaptArgs returns the C++ of the arguments args of the call c of a C/C++ function with the
// rows of the table applied, and the out-parameters that the call must be wrapped in (wrapOuts);
// args is the result of cArgs for c.Args (after rawPointerArgs). It returns args unchanged when
// typed mode is off or nothing is adapted. What is adapted, and what is an error, is decided by
// adapt.Decide (the interop checker asks the same function); this writes the result.
func (fg *funcGen) typedAdaptArgs(c *ast.CallExpr, args []cppir.Expr) ([]cppir.Expr, outCall) {
	g := fg.g
	if !g.typedOn() || c.Ellipsis.IsValid() {
		return args, outCall{}
	}
	var call *adapt.Call
	var outs outCall
	for i, a := range c.Args {
		if i >= len(args) {
			break
		}
		if g.argClass(a) == adapt.ClassNone {
			continue
		}
		if call == nil {
			if call = fg.adaptCall(c); call == nil {
				return args, outCall{}
			}
		}
		d := adapt.Decide(g.interop.typed.rep.Mapper, call, i)
		switch d.Outcome {
		case adapt.Refuse:
			pos := c.Pos()
			if d.At != adapt.AtCall {
				pos = c.Args[d.At].Pos()
			}
			g.userError(pos, "%s", d.Msg)
		case adapt.Convert:
			site := &adaptSite{
				call: c, i: i, arg: a, x: args[i], key: d.Key, name: call.Name, outs: &outs.wraps,
			}
			if x, ok := lowerers[d.Key.Row](fg, site); ok {
				args[i] = x
			}
		}
	}
	if len(outs.wraps) > 0 {
		fg.evaluateEarly(c, args, &outs)
	}
	return args, outs
}

// adaptCall returns the call c as adapt.Decide sees it, or nil when the callee is not known (a
// template, a macro, a function value).
func (fg *funcGen) adaptCall(c *ast.CallExpr) *adapt.Call {
	g := fg.g
	sigs, name, known := g.calleeSigs(c)
	if !known || len(sigs) == 0 {
		return nil
	}
	call := &adapt.Call{
		Name: name,
		Sigs: sigs,
		Args: make([]adapt.Arg, len(c.Args)),
		Source: func(i int) string {
			if i == adapt.AtCall {
				return g.srcText(c.Fun)
			}
			return g.srcText(c.Args[i])
		},
		Deferred:           fg.thunk > 0,
		MethodKeepsCharPtr: func(cs adapt.CharSet) (string, bool) { return g.methodKeepsCharPtr(c, cs) },
		AliasUseOK:         func() bool { return g.aliasUseOK(c) },
	}
	hasString := false
	for i, a := range c.Args {
		call.Args[i] = g.describeArg(a)
		switch call.Args[i].Class {
		case adapt.ClassString, adapt.ClassStringConst:
			hasString = true
		}
	}
	if hasString {
		// The other arguments that are the address of a char pointer: the callee can store a
		// pointer into the string there.
		for i, a := range c.Args {
			call.Args[i].CharPtrAddr = g.charPtrAddr(a)
		}
	}
	return call
}

// goFacts are the answers about the Go program that adapt.Describe asks, from the lowering's
// state.
type goFacts struct{ g *gen }

func (f goFacts) Operand(e ast.Expr) types.TypeAndValue { return f.g.info.Types[e] }

func (f goFacts) Opaque(e ast.Expr) bool { return f.g.pkg.Opaque.IsOpaque(e) }

func (f goFacts) Holder(e ast.Expr) bool { return f.g.isHolder(e) }

func (f goFacts) Variable(e ast.Expr) bool { return f.g.isVariableName(e) }

func (f goFacts) CPtrVariable(e ast.Expr) bool { return f.g.isCPtrVariable(e) }

// describeArg returns the argument a as adapt.Decide sees it.
func (g *gen) describeArg(a ast.Expr) adapt.Arg {
	return adapt.Describe(goFacts{g}, a, g.srcText)
}

// argClass returns the class of the Go argument a.
func (g *gen) argClass(a ast.Expr) adapt.Class { return g.describeArg(a).Class }

// goStringValue reports whether e is a gocpp string that is not a constant (a constant is a C
// literal already, cArgs): a variable, a call, an operator's result of type string.
func (g *gen) goStringValue(e ast.Expr) bool { return adapt.GoStringValue(goFacts{g}, e) }

// isVariableName reports whether e names a variable or a field of a struct (adapt.IsVariableName).
func (g *gen) isVariableName(e ast.Expr) bool { return adapt.IsVariableName(g.info.Info, e) }

// isCPtrVariable reports whether e is a variable declared with a pointer to a C/C++ type
// (var p *lib.T: a gp::cptr<T>; outPtrTarget's rule).
func (g *gen) isCPtrVariable(e ast.Expr) bool {
	return adapt.CPtrDeclared(g.valueTypeExpr(e), g.isCType)
}

// charPtrAddr returns the C++ name of the character type of the pointer whose address the argument a
// is (&end of a *C.char variable: char **), or "" (adapt.CharPtrAddr).
func (g *gen) charPtrAddr(a ast.Expr) string {
	var typed *ftype.Value
	if v := g.tvalOf(a); v != nil {
		typed = &v.Value
	}
	return adapt.CharPtrAddr(typed, a, g.valueTypeExpr, g.isC)
}
