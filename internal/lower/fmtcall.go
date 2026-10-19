package lower

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Calls of package fmt (Phase 05 step 5.11, ADR 028): Print, Println, Printf, Sprint, Sprintln,
// Sprintf, Fprint, Fprintln, Fprintf, Append, Appendln, Appendf, Errorf and FormatString are the
// runtime's (gocpp/fmt.hpp, runtime/src/fmt.cpp: a port of Go's fmt that walks values by their type
// descriptors). The Sprint functions give a gp::string, Errorf a gp::error, the Print and Fprint
// functions Go's (n int, err error), a std::tuple.
//
// Every operand is a value of its exact C++ type (exactExpr), taken by value, so it prints the
// value at the call, as Go does (Go copies it into an interface there); which values gc copies
// earlier, in their places among the calls, is order.go's rule. An operand that is an interface
// value (any, error, fmt.Stringer, a Go interface) is passed as it is: fmt uses its dynamic
// type. The others pass the descriptor of their Go type, gp::fmt::as(d, x), unless the C++ type
// says the Go type, which saves the descriptor (and its code) in the usual calls:
//
//	fmt.Println(n, s, ok, f)          gp::fmt::Println(n, s, ok, f);            // int, string, bool, float64
//	fmt.Println(p, xs)                gp::fmt::Println(gp::fmt::as(&gp_td_Point, p), gp::fmt::as(gp_tdc_0, xs));
//	fmt.Println(n64)                  gp::fmt::Println(gp::fmt::as(&gp::td_int64, n64));   // C++ can't tell int64 from int
//	fmt.Println(err, nil)             gp::fmt::Println(err, gp::any());
//	fmt.Fprintln(os.Stderr, x)        gp::fmt::Fprintln(gp::fmt::stderr_, x);
//	fmt.Errorf("read %s: %w", n, err) gp::fmt::Errorf("read %s: %w"_gs, n, err);
//	fmt.Printf(f, args...)            gp::fmt::Printf(f, gp::fmt::spread(args));   // the elements of []any
//
// A value of a type that has an Error method and is no pointer goes into an interface first
// (gp::to_iface<gp::any>), so that %w can wrap it: Errorf needs the value itself, not a view.
//
// An argument with a C/C++ part (a C/C++ value, or a gocpp value with one inside: hasC) goes
// through gp::cgo::fmt_arg (gocpp/interop.hpp), which checks that fmt can print every part of it
// at the .gp line, as the user's error, and registers the text of its C++ classes; a value whose
// Go type is invalid (a C/C++ value) has its descriptor deduced from its C++ type there:
//
//	fmt.Println(v, n)     // v is a std.vector[int], n a C.int
//	gp::fmt::Println(gp::cgo::fmt_arg(v), gp::cgo::fmt_arg(n));
//	println(cstdio.EOF)
//	gp::println(gp::cgo::print_arg(EOF));
//
// Fprint, Fprintln and Fprintf take os.Stdout and os.Stderr and a fmt.State (the argument of a
// Format method); the other io.Writers come with package io (Phase 09).

// fmtFuncs are the functions of package fmt that the runtime has.
var fmtFuncs = map[string]bool{
	"Print": true, "Println": true, "Printf": true,
	"Sprint": true, "Sprintln": true, "Sprintf": true,
	"Fprint": true, "Fprintln": true, "Fprintf": true,
	"Append": true, "Appendln": true, "Appendf": true,
	"Errorf": true, "FormatString": true,
}

// fmtFirst returns the index of the first operand of the fmt function name: after the writer or
// the slice, and the format.
func fmtFirst(name string) int {
	switch name {
	case "Print", "Println", "Sprint", "Sprintln":
		return 0
	case "Printf", "Sprintf", "Errorf", "Fprint", "Fprintln", "Append", "Appendln":
		return 1
	case "Fprintf", "Appendf":
		return 2
	}
	return 0
}

// fmtFormatted reports whether the function takes a format before its operands.
func fmtFormatted(name string) bool {
	switch name {
	case "Printf", "Sprintf", "Errorf", "Fprintf", "Appendf":
		return true
	}
	return false
}

// fmtFunc returns the fmt function that c calls, or nil if c calls no function of package fmt.
func (fg *funcGen) fmtFunc(c *ast.CallExpr) *types.Func {
	sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok || fg.g.info.Selections[sel] != nil {
		return nil
	}
	fn, ok := fg.g.info.Uses[sel.Sel].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != "fmt" {
		return nil
	}
	return fn
}

// pkgFunc returns the C++ name of the function of another package that c calls, pkg.F, or ""
// (with an error) if there is none yet. (A method call, a call of a func value and T.M(x) are
// lowered before: methods.go, closures.go.)
func (fg *funcGen) pkgFunc(c *ast.CallExpr, sel *ast.SelectorExpr) string {
	fn, _ := fg.g.info.Uses[sel.Sel].(*types.Func)
	if fn == nil || fn.Pkg() == nil || fg.g.info.Selections[sel] != nil {
		fg.g.unsupported(c.Pos(), "the call of "+types.ExprString(c.Fun))
		return ""
	}
	if fn.Pkg().Path() != "fmt" || !fmtFuncs[fn.Name()] {
		fg.g.unsupported(c.Pos(), "the call of "+fn.Pkg().Name()+"."+fn.Name()+" (gocpp lowers only the Print, Sprint, Fprint, Append and Errorf functions of fmt)")
		return ""
	}
	return "gp::fmt::" + fn.Name()
}

// fmtCall returns the C++ of the call c of the fmt function fn, whose C++ name is callee.
func (fg *funcGen) fmtCall(c *ast.CallExpr, fn *types.Func, callee string) cppir.Expr {
	name := fn.Name()
	fg.g.noteFmt(c.Pos(), name) // why.go
	if name == "FormatString" {
		args := fg.exprs(c.Args, false)
		if len(c.Args) > 0 && isNilType(fg.g.info.TypeOf(c.Args[0])) {
			args[0] = construct(typeName("gp::fmt::State")) // a nil interface
		}
		return &cppir.Call{Fun: cxxName(callee), Args: args}
	}
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) && fmtFirst(name) == 0 {
		return fg.fmtTupleCall(c, callee)
	}
	first := fmtFirst(name)
	args := make([]cppir.Expr, len(c.Args))
	for i, a := range c.Args {
		switch {
		case i == 0 && (name == "Fprint" || name == "Fprintln" || name == "Fprintf"):
			args[i] = fg.fmtWriter(a)
		case i == 0 && (name == "Append" || name == "Appendln" || name == "Appendf"):
			args[i] = fg.exactExpr(a) // the []byte
		case i == first-1 && fmtFormatted(name):
			args[i] = fg.exactExpr(a) // the format, a string
		case i == len(c.Args)-1 && c.Ellipsis.IsValid():
			// fmt.Println(a...): the elements of the []any are the operands.
			args[i] = call("gp::fmt::spread", fg.exactExpr(a))
		default:
			args[i] = fg.fmtArg(a)
		}
	}
	return &cppir.Call{Fun: cxxName(callee), Args: args}
}

// fmtWriter returns the C++ of the writer of Fprint, Fprintln or Fprintf: os.Stdout, os.Stderr or
// a fmt.State.
func (fg *funcGen) fmtWriter(e ast.Expr) cppir.Expr {
	if sel, ok := ast.Unparen(e).(*ast.SelectorExpr); ok && fg.g.info.Selections[sel] == nil {
		if v, ok := fg.g.info.Uses[sel.Sel].(*types.Var); ok && v.Pkg() != nil && v.Pkg().Path() == "os" {
			switch v.Name() {
			case "Stdout":
				return cxxName("gp::fmt::stdout_")
			case "Stderr":
				return cxxName("gp::fmt::stderr_")
			}
		}
	}
	if t := fg.g.info.TypeOf(e); t != nil && isFmtIface(t, "State") {
		return fg.exactExpr(e)
	} else if t != nil && isNilType(t) {
		return construct(typeName("gp::fmt::State")) // a nil writer: the call panics
	}
	fg.g.notYet(e.Pos(), "fmt.Fprint to a writer other than os.Stdout, os.Stderr or a fmt.State is not available yet (package io: Phase 09)")
	return placeholder
}

// plainFmtType reports whether the C++ type of a value of the Go type t says the Go type, so that
// the value is a plain operand of fmt (gocpp/fmt.hpp plain_desc): the unnamed bool, int8, int16,
// int32, uint8, uint16, uint32, int, uint, float32, float64, complex64, complex128 and string.
func plainFmtType(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	if !ok {
		return false
	}
	switch b.Kind() {
	case types.Bool, types.Int8, types.Int16, types.Int32, types.Uint8, types.Uint16, types.Uint32,
		types.Int, types.Uint, types.Float32, types.Float64, types.Complex64, types.Complex128, types.String:
		return true
	}
	return false
}

// hasErrorMethod reports whether the method set of t has Error() string.
func hasErrorMethod(t types.Type) bool {
	s := types.NewMethodSet(t).Lookup(nil, "Error")
	return s != nil && stringerSig(wrapSelection(s))
}

// stringerSig reports whether the method of the selection s is func() string.
func stringerSig(s *selection) bool {
	sig, ok := s.Obj().Type().(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		return false
	}
	return types.Identical(sig.Results().At(0).Type(), types.Typ[types.String])
}

// fmtArg returns the C++ of e as an operand of fmt's ...any (see the top).
func (fg *funcGen) fmtArg(e ast.Expr) cppir.Expr {
	return fg.fmtArgOf(fg.g.info.TypeOf(e), fg.exactExpr(e), e.Pos())
}

// fmtArgOf is fmtArg for the value x of the Go type t.
func (fg *funcGen) fmtArgOf(t types.Type, x cppir.Expr, pos token.Pos) cppir.Expr {
	switch {
	case t != nil && isNilType(t):
		return construct(typeName("gp::any")) // an untyped nil is a nil interface
	case t == nil || fg.g.foreignVal(t):
		return call("gp::cgo::fmt_arg", x) // a C/C++ value: its descriptor comes from its C++ type
	case isInterface(t):
		return x // its dynamic type
	}
	if b, ok := types.Unalias(t).(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
		t = types.Default(t)
	}
	if plainFmtType(t) && !hasC(t) {
		return x
	}
	if hasErrorMethod(t) && !isPointer(t) {
		// Errorf wraps the value: it must be in an interface (a box it owns).
		any := types.Universe.Lookup("any").Type()
		return fg.g.toInterface(t, any, x, pos)
	}
	if (isArray(t) || isStruct(t)) && largeVar(t) {
		x = call("gp::fmt::heap", largeBox(fg.g.cxxType(t, pos), x)) // arrays.go
	}
	x = call("gp::fmt::as", fg.g.desc(t, pos), x)
	if hasC(t) {
		x = call("gp::cgo::fmt_arg", x)
	}
	return x
}

// printArgs returns the C++ of the arguments of the builtin print or println: each with its
// exact type (exactExpr), a C/C++ value through gp::cgo::print_arg.
func (fg *funcGen) printArgs(args []ast.Expr) []cppir.Expr {
	for _, a := range args {
		if t := fg.g.info.TypeOf(a); t != nil && isInterface(t) {
			fg.g.unsupported(a.Pos(), "printing an interface value with the builtin print (Go prints its words)")
			return nil
		}
	}
	xs := fg.exprs(args, true)
	for i, a := range args {
		if t := fg.g.info.TypeOf(a); t == nil || hasC(t) {
			xs[i] = call("gp::cgo::print_arg", xs[i])
		}
	}
	return xs
}

// fmtTupleCall returns the C++ of fmt.Print(g()), Println, Sprint or Sprintln, where g has several
// results: the call goes through std::apply with a lambda that takes the results, each with its
// descriptor:
//
//	std::apply([](auto&& gp_r0, auto&& gp_r1) { return gp::fmt::Println(gp_r0, gp::fmt::as(&gp::td_int64, gp_r1)); }, g())
func (fg *funcGen) fmtTupleCall(c *ast.CallExpr, callee string) cppir.Expr {
	tuple := fg.g.info.TypeOf(c.Args[0]).(*types.Tuple)
	var params []cppir.Param
	var args []cppir.Expr
	for i := range tuple.Len() {
		name := "gp_r" + strconv.Itoa(i)
		params = append(params, cppir.Param{Type: &cppir.Ref{Elem: typeName("auto"), RValue: true}, Name: name})
		args = append(args, fg.fmtArgOf(tuple.At(i).Type(), cxxName(name), c.Pos()))
	}
	body := &cppir.Return{X: &cppir.Call{Fun: cxxName(callee), Args: args}}
	fn := &cppir.Lambda{Params: params, Body: &cppir.Block{Stmts: []cppir.Stmt{body}, Inline: true}}
	return &cppir.Call{Fun: cxxName("std::apply"), Args: []cppir.Expr{fn, fg.expr(c.Args[0])}}
}

// isFmtIface reports whether t is the interface type fmt.name.
func isFmtIface(t types.Type, name string) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "fmt" && n.Obj().Name() == name
}

// stdPkgMember returns the object of the qualified name x, pkg.Name, if pkg is a standard package
// that the program imports and gocpp has no code for beyond fmt's calls (fmt, os, ...), or nil.
func (fg *funcGen) stdPkgMember(x *ast.SelectorExpr) types.Object {
	id, ok := x.X.(*ast.Ident)
	if !ok {
		return nil
	}
	pn, ok := fg.g.info.Uses[id].(*types.PkgName)
	if !ok || fg.g.foreign(pn.Imported()) != nil {
		return nil
	}
	return fg.g.info.Uses[x.Sel]
}

// stdMemberValue returns the C++ of the use of obj, pkg.Name, as a value (not called): a function
// of fmt becomes a func value that spreads its final ...any into the runtime function,
//
//	h := fmt.Println     gp::func<std::tuple<int64_t, gp::error>(gp::slice<gp::any>)>(
//	                         [](gp::slice<gp::any> gp_a0) { return gp::fmt::Println(gp::fmt::spread(gp_a0)); })
//
// Fprint, Fprintln, Fprintf (they need an io.Writer) and FormatString, os.Stdout and os.Stderr
// outside the writer argument of a Fprint call (they need the type os.File) and the other
// members of the standard packages are not available yet (Phase 09).
func (fg *funcGen) stdMemberValue(x *ast.SelectorExpr, obj types.Object) cppir.Expr {
	name := types.ExprString(x)
	fn, ok := obj.(*types.Func)
	pkg := obj.Pkg().Path()
	if !ok || pkg != "fmt" || !fmtFuncs[fn.Name()] || fn.Name() == "FormatString" ||
		(fmtWriterFunc(fn.Name())) {
		switch {
		case ok && pkg == "fmt":
			fg.g.notYet(x.Pos(), name+" as a value is not available yet (it needs package io: Phase 09)")
		case !ok && pkg == "os":
			fg.g.notYet(x.Pos(), name+" used as a value, outside the writer argument of fmt.Fprint, is not available yet (it needs the type os.File: Phase 09)")
		default:
			fg.g.notYet(x.Pos(), name+" is not available yet (Phase 09)")
		}
		return placeholder
	}
	sig := fn.Type().(*types.Signature)
	ft := fg.g.funcType(sig, x.Pos())
	ftype, _ := ft.(*cppir.TypeName)
	if ftype == nil || len(ftype.Args) != 1 {
		return placeholder
	}
	sigT, _ := ftype.Args[0].(*cppir.FuncType)
	if sigT == nil {
		return placeholder
	}
	var params []cppir.Param
	var args []cppir.Expr
	for i, p := range sigT.Params {
		pn := "gp_a" + strconv.Itoa(i)
		params = append(params, cppir.Param{Type: p, Name: pn})
		if i == len(sigT.Params)-1 && sig.Variadic() {
			args = append(args, call("gp::fmt::spread", cxxName(pn)))
		} else {
			args = append(args, cxxName(pn))
		}
	}
	body := &cppir.Return{X: &cppir.Call{Fun: cxxName("gp::fmt::" + fn.Name()), Args: args}}
	lambda := &cppir.Lambda{Params: params, Body: &cppir.Block{Stmts: []cppir.Stmt{body}, Inline: true}}
	return construct(ft, lambda)
}

// fmtWriterFunc reports whether the fmt function name takes an io.Writer.
func fmtWriterFunc(name string) bool {
	return name == "Fprint" || name == "Fprintln" || name == "Fprintf"
}
