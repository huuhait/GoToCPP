package lower

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"strings"
	"unicode/utf8"

	"github.com/huuhait/gocpp/internal/cppir"
)

// Calls of package fmt (Phase 03 README §2.10, step 3.15): fmt.Print, Println, Printf, Sprint,
// Sprintln and Sprintf are the runtime's stand-in, gp::fmtshim (gocpp/fmtshim.hpp), until
// Phase 05 and 09 bring the real fmt. The Sprint functions give a gp::string.
//
// Every value of Phase 03 is an argument: basic values, strings, slices, arrays, maps, structs,
// pointers and named types of them, also nil ones. The shim takes each argument by value, so it
// prints the value at the call, as Go does (Go copies it into an interface there); which values
// gc copies earlier, in their places among the calls, is order.go's rule (copied).
//
// C++ can't tell Go's int from int64, nor uint from uint64 and uintptr (all are int64_t or
// uint64_t), and the error forms (%!s(int64=5)) and %T name the Go type. So an argument whose
// type has one of them in a part that is not named (a named type and a struct carry their own
// tags: gp_value_tag, gp_fields) is passed with its type tag:
//
//	fmt.Printf("%s", x)   // x is an int64
//	gp::fmtshim::Printf("%s"_gs, gp::fmtshim::typed<gp::fmtshim::tag::int64>(x));
//	fmt.Println(m)        // m is a map[string]uint64
//	gp::fmtshim::Println(gp::fmtshim::typed<gp::fmtshim::tag::map<gp::fmtshim::tag::none, gp::fmtshim::tag::uint64>>(m));
//
// A nil value has its type: []int(nil) is gp::slice<int64_t>(), (*T)(nil) gp::ptr<T>(nullptr)
// (an untyped nil, fmt.Println(nil), is a nil interface: Phase 05).
//
// An argument with a C/C++ part (a C/C++ value, or a gocpp value with one inside: hasC) goes
// through gp::cgo::fmt_arg (gocpp/interop.hpp), and one of the builtin print and println
// through gp::cgo::print_arg (Phase 04 README §2.12, step 4.12). Lowering doesn't know the C++
// type of a C/C++ value; the runtime prints it by that type, and the helper's static_assert
// reports a type that can't be printed at the call's .gp line, as the user's error:
//
//	fmt.Println(v, n)     // v is a std.vector[int], n a C.int
//	gp::fmtshim::Println(gp::cgo::fmt_arg(v), gp::cgo::fmt_arg(n));
//	println(cstdio.EOF)
//	gp::println(gp::cgo::print_arg(EOF));

// fmtFuncs are the functions of package fmt that the runtime's stand-in has.
var fmtFuncs = map[string]bool{
	"Print": true, "Println": true, "Printf": true,
	"Sprint": true, "Sprintln": true, "Sprintf": true,
}

// fmtFormatted reports whether the function of fmt named name takes a format first.
func fmtFormatted(name string) bool {
	return name == "Printf" || name == "Sprintf"
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
		fg.g.unsupported(c.Pos(), "the call of "+fn.Pkg().Name()+"."+fn.Name()+" (gocpp lowers only fmt.Print, Println, Printf, Sprint, Sprintln and Sprintf)")
		return ""
	}
	if fmtFormatted(fn.Name()) && len(c.Args) > 0 {
		if tv := fg.g.info.Types[c.Args[0]]; tv.Value != nil && tv.Value.Kind() == constant.String {
			if verb, phase := printfVerbs(constant.StringVal(tv.Value)); verb != "" {
				fg.g.unsupported(c.Args[0].Pos(), fmt.Sprintf("the fmt.%s directive %s (Phase %02d)", fn.Name(), verb, phase))
				return ""
			}
		}
	}
	return "gp::fmtshim::" + fn.Name()
}

// fmtCall returns the C++ of the call c of the fmt function fn, whose C++ name is callee.
func (fg *funcGen) fmtCall(c *ast.CallExpr, fn *types.Func, callee string) cppir.Expr {
	if len(c.Args) == 1 && isTuple(fg.g.info.TypeOf(c.Args[0])) {
		return fg.fmtTupleCall(c, callee)
	}
	args := make([]cppir.Expr, len(c.Args))
	for i, a := range c.Args {
		if i == 0 && fmtFormatted(fn.Name()) {
			args[i] = fg.exactExpr(a) // the format, a string
			continue
		}
		args[i] = fg.fmtArg(a)
	}
	return &cppir.Call{Fun: cxxName(callee), Args: args}
}

// fmtArg returns the C++ of e as an argument of fmt's ...any: its value with its exact type
// (exactExpr), and its type tag where it needs one. An array or a struct larger than 128 KiB is
// copied to the heap, gp::fmtshim::heap(gp::new_<A>(a)), not by value on the C++ stack (D24).
func (fg *funcGen) fmtArg(e ast.Expr) cppir.Expr {
	t := fg.g.info.TypeOf(e)
	x := fg.exactExpr(e)
	if (isArray(t) || isStruct(t)) && largeVar(t) {
		x = call("gp::fmtshim::heap", largeBox(fg.g.cxxType(t, e.Pos()), x)) // arrays.go
	}
	x = fmtTyped(t, x)
	if t == nil || hasC(t) {
		x = call("gp::cgo::fmt_arg", x)
	}
	return x
}

// printArgs returns the C++ of the arguments of the builtin print or println: each with its
// exact type (exactExpr), a C/C++ value through gp::cgo::print_arg.
func (fg *funcGen) printArgs(args []ast.Expr) []cppir.Expr {
	xs := fg.exprs(args, true)
	for i, a := range args {
		if t := fg.g.info.TypeOf(a); t == nil || hasC(t) {
			xs[i] = call("gp::cgo::print_arg", xs[i])
		}
	}
	return xs
}

// fmtTyped returns x, a value of the Go type t, with its type tag, if t needs one:
// gp::fmtshim::typed<tag>(x).
func fmtTyped(t types.Type, x cppir.Expr) cppir.Expr {
	if tag := fmtTag(t); tag != "" {
		return templateCall("gp::fmtshim::typed", []cppir.Node{typeName(tag)}, x)
	}
	return x
}

// fmtTupleCall returns the C++ of fmt.F(g()), where g has several results: the call goes
// through std::apply with a generic lambda (std::apply needs a function object), which returns
// the string of the Sprint functions:
//
//	std::apply([](const auto&... gp_a) { gp::fmtshim::Println(gp_a...); }, g())
//
// The results get no type tags. Print, Println, Sprint and Sprintln never print a type's name,
// and Printf(g()) and Sprintf(g()), whose format is g's first result, are valid Go but need a
// format that is not a constant (gated to Phase 09): there, %!s(int64=5) needs the tags, so
// Phase 09's tuple path must give each result its tag.
func (fg *funcGen) fmtTupleCall(c *ast.CallExpr, callee string) cppir.Expr {
	params := []cppir.Param{{Type: &cppir.Ref{Elem: &cppir.Const{Elem: typeName("auto")}}, Pack: true, Name: "gp_a"}}
	var arg cppir.Expr = cxxName("gp_a")
	if hasC(fg.g.info.TypeOf(c.Args[0])) {
		arg = call("gp::cgo::fmt_arg", arg) // results with C/C++ parts
	}
	x := call(callee, &cppir.Expand{X: arg})
	var body cppir.Stmt = &cppir.ExprStmt{X: x}
	if strings.HasPrefix(callee, "gp::fmtshim::S") {
		body = &cppir.Return{X: x}
	}
	fn := &cppir.Lambda{Params: params, Body: &cppir.Block{Stmts: []cppir.Stmt{body}, Inline: true}}
	return &cppir.Call{Fun: cxxName("std::apply"), Args: []cppir.Expr{fn, fg.expr(c.Args[0])}}
}

// printfVerbs returns the first directive of a format of Printf or Sprintf that gp::fmtshim
// doesn't know in Phase 03, with the phase that brings it (the # flag, *, an argument index [n]:
// Phase 09; %T and %w: Phase 05, though the shim prints %T; the other verbs of Go's fmt: Phase
// 09), or "" and 0 if there is none. The shim knows the verbs %v %d %s %q %t %f and %%, the flags
// - + space and 0, a width and a precision, and any character that is no verb of Go's fmt (%z,
// %!, %é), for which it writes Go's error form %!z(int=5), as Go does. The format is parsed like
// Go's fmt.(*pp).doPrintf and the phase gate (internal/check, checkFormat and verbPhase), so the
// emitter accepts exactly what the gate lets through in Phase 03. (A % at the end is Go's
// %!(NOVERB), which the shim prints too.)
func printfVerbs(format string) (string, int) {
	for i := 0; i < len(format); {
		if format[i] != '%' {
			i++
			continue
		}
		start := i
		i++
		for i < len(format) && strings.IndexByte("#0+- ", format[i]) >= 0 {
			if format[i] == '#' {
				return directive(format, start), 9
			}
			i++
		}
		// The width and the precision, digits each (a . at the end is the verb, as in Go's
		// doPrintf); * and [n] are Phase 09.
		for part := 0; part < 2 && i < len(format); part++ {
			switch {
			case format[i] == '*' || format[i] == '[':
				return directive(format, start), 9
			case format[i] == '.' && i+1 < len(format) || '0' <= format[i] && format[i] <= '9':
				if format[i] == '.' {
					i++
				}
				for i < len(format) && '0' <= format[i] && format[i] <= '9' {
					i++
				}
			}
		}
		if i < len(format) && (format[i] == '*' || format[i] == '[') {
			return directive(format, start), 9
		}
		if i == len(format) {
			break
		}
		verb, size := utf8.DecodeRuneInString(format[i:])
		i += size
		if strings.ContainsRune(goVerbs, verb) && !strings.ContainsRune("vdsqtf%", verb) {
			if verb == 'T' || verb == 'w' {
				return format[start:i], 5
			}
			return format[start:i], 9
		}
	}
	return "", 0
}

// directive returns the directive of the format that starts at start, up to its verb (a
// letter), for an error message.
func directive(format string, start int) string {
	end := start + 1
	for end < len(format) && !isLetter(format[end-1]) {
		end++
	}
	return format[start:end]
}

// goVerbs are the verbs of Go's fmt (and %w of fmt.Errorf), as internal/check has them.
const goVerbs = "vTtbcdoOqxXUeEfFgGspw%"

func isLetter(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z'
}
