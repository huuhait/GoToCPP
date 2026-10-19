package icheck

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
	"github.com/huuhait/gocpp/internal/typedinterop/ftype"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
)

// The mapping of the compiler's messages to Go's words, one case for each
// diagnostic that has a rule (the texts are what clang 17 to 21 print).

func testChecker() *checker {
	decls := &probe.DeclTable{Enums: map[string]*probe.Enum{"Color": {Name: "Color", Underlying: probe.TypeText{Text: "unsigned int"}}, "Mode": {Name: "Mode", Scoped: true}}}
	rep := &typedinterop.Report{Set: collect.NewSet(), Table: &probe.Table{}, Mapper: ftype.New(decls, ftype.LP64)}
	return &checker{rep: rep, hdrs: map[string]string{"area": "shapes", "paint": "shapes", "takes_cstr": "shapes", "nothing": "shapes", "Point": "shapes"}, lines: map[string][]string{}, warn: false, res: &Result{}}
}

func lit(s string) collect.Expr                { return &collect.Lit{Text: s} }
func goInt(lv bool) collect.Expr               { return &collect.Arg{T: &collect.GoType{Cxx: "int64_t"}, LValue: lv} }
func goStr() collect.Expr                      { return &collect.Arg{T: &collect.Stand{Name: "string"}, LValue: true} }
func path(h string, p ...string) *collect.Path { return &collect.Path{Parts: p, Hdr: h} }

func call(fun collect.Expr, args ...collect.Expr) *collect.Call {
	return &collect.Call{Fun: fun, Args: args}
}

func note(msg string) probe.Diag { return probe.Diag{Severity: "note", Msg: msg} }

type msgCase struct {
	name    string
	expr    collect.Expr
	text    string // Site.Text, as the collector writes it
	msg     string // the compiler's message
	notes   []probe.Diag
	want    string
	related []string // the texts of the related lines
	line    string   // the probe file's line, to find the argument (with errAt: the text where the error is)
	errAt   string
}

func TestMessageRules(t *testing.T) {
	sel := func(x collect.Expr, name string) collect.Expr { return &collect.Select{X: x, Name: name} }
	pt := &collect.Arg{T: path("", "Point"), LValue: true}
	cases := []msgCase{
		{name: "member", expr: sel(pt, "nosuch"), text: "p.nosuch",
			msg: "no member named 'nosuch' in 'Point'", want: "p.nosuch undefined (type shapes.Point has no field or method nosuch)"},
		{name: "member of a typedef'd struct is named as the program does", expr: sel(&collect.Arg{T: path("", "z_stream"), LValue: true}, "nosuch"), text: "strm.nosuch",
			msg: "no member named 'nosuch' in 'z_stream_s'", want: "strm.nosuch undefined (type z_stream has no field or method nosuch)"},
		{name: "member call", expr: call(sel(pt, "nosuch2")), text: "p.nosuch2()",
			msg: "no member named 'nosuch2' in 'Point'", want: "p.nosuch2 undefined (type shapes.Point has no field or method nosuch2)"},
		{name: "member with clang's suggestion", expr: call(sel(call(path("", "make_vec")), "sze")), text: "v.sze()",
			msg: "no member named 'sze' in 'std::vector<long>'; did you mean 'size'?", want: "v.sze undefined (type std.vector[C.long] has no field or method sze)", related: []string{"did you mean size?"}},
		{name: "global name", expr: call(path("shapes", "nosuchfn"), lit("1")), text: "C.nosuchfn(1)",
			msg: "no member named 'nosuchfn' in the global namespace", want: "undefined: nosuchfn (the imported headers declare no ::nosuchfn)"},
		{name: "namespace name", expr: call(path("", "chrono", "steady_clok", "now")), text: "C.std.chrono.steady_clok.now()",
			msg: "no member named 'steady_clok' in namespace 'std::chrono'", want: "undefined: std.chrono.steady_clok (the imported headers declare no ::std::chrono::steady_clok)"},
		{name: "undeclared", expr: call(path("", "foo")), text: "C.foo()",
			msg: "use of undeclared identifier 'foo'", want: "undefined: foo (the imported headers declare no ::foo)"},
		{name: "unknown type", expr: path("", "vectr"), text: "C.std.vectr",
			msg: "no template named 'vectr' in namespace 'std'", want: "undefined: std.vectr (the imported headers declare no ::std::vectr)"},
		{name: "unknown nested type", expr: path("", "chrono", "clok"), text: "C.std.chrono.clok",
			msg: "no type named 'clok' in namespace 'std::chrono'", want: "undefined: std.chrono.clok (the imported headers declare no ::std::chrono::clok)"},
		{name: "private", expr: sel(pt, "hidden"), text: "s.hidden",
			msg: "'hidden' is a private member of 'Secret'", want: "s.hidden undefined (hidden is a private member of type Secret)"},
		{name: "protected", expr: sel(pt, "prot"), text: "s.prot",
			msg: "'prot' is a protected member of 'Secret'", want: "s.prot undefined (prot is a protected member of type Secret)"},
		{name: "pointer member", expr: sel(pt, "x"), text: "pp.x",
			msg: "member reference type 'Point *' is a pointer; did you mean to use '->'?", want: "pp.x undefined (type **shapes.Point has no field or method x)"},
		{name: "no match", expr: call(path("shapes", "area"), lit("1")), text: "C.area(1)",
			msg:     "no matching function for call to 'area'",
			notes:   []probe.Diag{note("candidate function not viable: no known conversion from 'int' to 'Point' for 1st argument"), note("candidate function not viable: requires 2 arguments, but 1 was provided")},
			want:    "no overload of shapes.area matches the arguments (untyped int)",
			related: []string{"candidate: argument 1: cannot use C.int as shapes.Point", "candidate needs 2 arguments, got 1"}},
		{name: "no match, single argument candidate", expr: call(path("shapes", "area"), lit("1"), lit("2"), lit("3")), text: "C.area(1, 2, 3)",
			msg:     "no matching function for call to 'area'",
			notes:   []probe.Diag{note("candidate function not viable: requires single argument 'p', but 3 arguments were provided"), note("candidate function template not viable: requires at least 2 arguments, but 3 were provided")},
			want:    "no overload of shapes.area matches the arguments (untyped int, untyped int, untyped int)",
			related: []string{"candidate needs 1 argument, got 3", "candidate needs at least 2 arguments, got 3"}},
		{name: "no match, template ignored", expr: call(path("", "min"), lit("1"), lit(`"a"`)), text: "C.std.min(1, \"a\")",
			msg:     "no matching function for call to 'min'",
			notes:   []probe.Diag{note("candidate template ignored: deduced conflicting types for parameter '_Tp' ('int' vs. 'char[2]')")},
			want:    "no overload of std.min matches the arguments (untyped int, untyped string)",
			related: []string{"candidate template ignored: deduced conflicting types for parameter '_Tp' ('int' vs. 'char[2]')"}},
		{name: "no match member", expr: call(sel(pt, "push_back"), lit("1")), text: "v.push_back(1)",
			msg: "no matching member function for call to 'push_back'", want: "no overload of v.push_back matches the arguments (untyped int)"},
		{name: "no match constructor", expr: call(path("", "string"), lit("1")), text: "C.std.string(1)",
			msg: "no matching constructor for initialization of 'std::string'", want: "no overload of std.string matches the arguments (untyped int)"},
		{name: "ambiguous", expr: call(path("shapes", "ovl"), lit("1.5")), text: "C.ovl(1.5)",
			msg: "call to 'ovl' is ambiguous", notes: []probe.Diag{note("candidate function"), note("candidate function")},
			want: "call to ovl is ambiguous: several overloads match equally well", related: []string{"candidate declared here", "candidate declared here"}},
		{name: "parameter type", expr: call(path("shapes", "paint"), lit("2")), text: "C.paint(2)",
			line: "using gp_probe_0 = gocpp_probe_internal_c<decltype((::paint(2)))>;", errAt: "2)))",
			msg:     "cannot initialize a parameter of type 'Color' with an rvalue of type 'int'",
			want:    "cannot use 2 (untyped int constant) as Color value in argument to shapes.paint",
			related: []string{"C++ has no implicit conversion from an integer to an enum: write the enumerator, or an explicit conversion"}},
		{name: "parameter type with aka", expr: call(path("", "fclose"), lit("5")), text: "C.fclose(5)",
			msg:  "cannot initialize a parameter of type 'FILE *' (aka '_IO_FILE *') with an rvalue of type 'int'",
			want: "cannot use an argument (value of type C.int) as *FILE value in argument to fclose"},
		{name: "member typedef prefers the canonical type", expr: call(sel(pt, "push_back"), lit(`"x"`)), text: `v.push_back("x")`,
			msg:  "cannot initialize a parameter of type 'const value_type' (aka 'const long') with an lvalue of type 'const char[2]'",
			want: "cannot use an argument (value of type [2]C.char) as const C.long value in argument to v.push_back"},
		{name: "conversion string", expr: call(path("shapes", "takes_cstr"), goStr()), text: "C.takes_cstr(s)",
			msg:     "no viable conversion from 'gocpp_probe_internal_pr::string' to 'const char *'",
			want:    "cannot use an argument (value of type string) as *C.char value in argument to shapes.takes_cstr",
			related: []string{"a Go string has no conversion to a C pointer: write C.cstr(s) (or C.CString(s))"}},
		{name: "conversion record to pointer", expr: call(path("", "count"), goInt(true)), text: "C.count(p)",
			msg:     "no viable conversion from 'Point' to 'const Point *'",
			want:    "cannot use an argument (value of type shapes.Point) as *shapes.Point value in argument to count",
			related: []string{"the parameter is a pointer: pass the address of the value (&x)"}},
		{name: "reference binds temporary", expr: call(path("", "scale"), lit("1")), text: "C.scale(1)",
			msg:     "non-const lvalue reference to type 'Point' cannot bind to a temporary of type 'Point'",
			want:    "cannot use an argument (value of type shapes.Point) as shapes.Point value in argument to scale",
			related: []string{"the C++ parameter is a reference that needs a variable (an addressable value), not a temporary value or a const one"}},
		{name: "reference drops const", expr: call(path("", "scale"), lit("1")), text: "C.scale(1)",
			msg:     "binding reference of type 'Point' to value of type 'const Point' drops 'const' qualifier",
			want:    "cannot use an argument (value of type const shapes.Point) as shapes.Point value in argument to scale",
			related: []string{"the C++ parameter is a reference that needs a variable (an addressable value), not a temporary value or a const one"}},
		{name: "rvalue reference", expr: call(path("", "sink"), lit("1")), text: "C.sink(1)",
			msg:     "rvalue reference to type 'Point' cannot bind to lvalue of type 'Point'",
			want:    "cannot use an argument (value of type shapes.Point) as shapes.Point value in argument to sink",
			related: []string{"the C++ parameter is a reference that needs a variable (an addressable value), not a temporary value or a const one"}},
		{name: "reference could not bind", expr: call(path("", "take"), lit("1")), text: "C.take(1)",
			msg:     "reference to type 'const Vec' could not bind to an lvalue of type 'int'",
			want:    "cannot use an argument (value of type C.int) as const Vec value in argument to take",
			related: []string{"the C++ parameter is a reference that needs a variable (an addressable value), not a temporary value or a const one"}},
		{name: "method value", expr: sel(pt, "inc"), text: "c.inc",
			msg: "reference to non-static member function must be called; did you mean to call it with no arguments?", want: "cannot use c.inc as a value: a C++ member function must be called"},
		{name: "too many", expr: call(path("", "abs"), lit("1"), lit("2")), text: "C.abs(1, 2)",
			msg: "too many arguments to function call, expected 1, have 2", want: "too many arguments in call to abs (have 2, want 1)"},
		{name: "too many, single", expr: call(path("", "abs"), lit("1"), lit("2")), text: "C.abs(1, 2)",
			msg: "too many arguments to function call, expected single argument '__x', have 2 arguments", want: "too many arguments in call to abs (have 2, want 1)"},
		{name: "too many, at most", expr: call(path("", "f"), lit("1"), lit("2"), lit("3")), text: "C.f(1, 2, 3)",
			msg: "too many arguments to function call, expected at most 2, have 3", want: "too many arguments in call to f (have 3, want at most 2)"},
		{name: "too few", expr: call(path("", "f"), lit("1")), text: "C.f(1)",
			msg: "too few arguments to function call, expected 2, have 1", want: "not enough arguments in call to f (have 1, want 2)"},
		{name: "too few, at least", expr: call(path("", "f")), text: "C.f()",
			msg: "too few arguments to function call, expected at least 1, have 0", want: "not enough arguments in call to f (have 0, want at least 1)"},
		{name: "too few, single not specified", expr: call(path("", "abs")), text: "C.abs()",
			msg: "too few arguments to function call, single argument '__x' was not specified", want: "not enough arguments in call to abs (have 0, want 1)"},
		{name: "not a function", expr: call(path("shapes", "global_value")), text: "C.global_value()",
			msg: "called object type 'int' is not a function or function pointer", want: "invalid operation: cannot call non-function global_value (type C.int)"},
		{name: "binary", expr: &collect.Binary{Op: "+", X: pt, Y: lit("1")}, text: "p + 1",
			msg: "invalid operands to binary expression ('Point' and 'int')", want: "invalid operation: p + 1 (operator + is not defined on shapes.Point and C.int)"},
		{name: "unary", expr: &collect.Unary{Op: "-", X: pt}, text: "-p",
			msg: "invalid argument type 'Point' to unary expression", want: "invalid operation: operator - not defined on -p (type shapes.Point)"},
		{name: "const method", expr: call(sel(call(path("shapes", "gcounter")), "inc")), text: "C.gcounter().inc()",
			msg:  "'this' argument to member function 'inc' has type 'const Counter', but function is not marked const",
			want: "cannot call method inc on gcounter() (type const Counter): the method is not marked const"},
		{name: "non-static", expr: call(path("shapes", "Point", "sum"), lit("1")), text: "C.Point.sum(1)",
			msg: "call to non-static member function without an object argument", want: "cannot call shapes.Point.sum without a value: it is a method, not a static function"},
		{name: "address", expr: &collect.Addr{X: call(path("", "make"))}, text: "&C.make()",
			msg: "cannot take the address of an rvalue of type 'Counter'", want: "invalid operation: cannot take address of make() (value of type Counter)"},
		{name: "address of a temporary object", expr: &collect.Addr{X: call(path("", "make"))}, text: "&C.make()",
			msg: "taking the address of a temporary object of type 'Counter' [-Waddress-of-temporary]", want: "invalid operation: cannot take address of make() (value of type Counter)"},
		{name: "not assignable", expr: call(path("", "abs"), lit("1")), text: "C.abs(1)",
			msg: "expression is not assignable", want: "cannot assign to abs(1) (neither addressable nor a map index expression)"},
		{name: "incomplete", expr: sel(pt, "x"), text: "p.x",
			msg: "member access into incomplete type 'Opaque'", want: "invalid use of Opaque: the type is only declared in the headers, not defined"},
		{name: "incomplete template", expr: sel(pt, "x"), text: "p.x",
			msg: "implicit instantiation of undefined template 'Box<int>'", want: "invalid use of Box[C.int]: the type is only declared in the headers, not defined"},
		{name: "deleted", expr: call(path("shapes", "take"), goInt(true)), text: "C.take(nc)",
			msg: "call to deleted constructor of 'NoCopy'", want: "take(nc) uses a function that C++ has deleted (call to deleted constructor of 'NoCopy')"},
		{name: "implicitly deleted", expr: call(path("shapes", "take"), goInt(true)), text: "C.take(nc)",
			msg: "call to implicitly-deleted copy constructor of 'Holder'", want: "take(nc) uses a function that C++ has deleted (call to implicitly-deleted copy constructor of 'Holder')"},
		{name: "cast", expr: call(&collect.GoType{Cxx: "double"}, &collect.Arg{T: path("", "Point"), LValue: true}), text: "float64(p)",
			msg:  "cannot convert 'Point' to 'double' without a conversion operator",
			want: "cannot convert p (variable of type shapes.Point) to type float64"},
		{name: "functional cast", expr: call(&collect.GoType{Cxx: "int64_t"}, lit("1")), text: "int(x)",
			msg:  "no matching conversion for functional-style cast from 'Point' to 'int64_t' (aka 'long')",
			want: "cannot convert x (untyped int constant) to type int"},
		{name: "fallback", expr: call(path("", "f"), lit("1")), text: "C.f(1)",
			msg: "static assertion failed due to requirement 'sizeof(int) == 1'", want: "f(1) is not valid C++: static assertion failed due to requirement 'sizeof(int) == 1'"},
		{name: "fallback with types", expr: call(path("", "f"), lit("1")), text: "C.f(1)",
			msg: "no viable overloaded '=' for 'std::vector<long>' (aka 'vector<long>')", want: "f(1) is not valid C++: no viable overloaded '=' for 'std::vector<long>'"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := testChecker()
			ref := &collect.Ref{Key: tc.expr.String(), Expr: tc.expr}
			res := &probe.Result{Error: tc.msg, Notes: tc.notes, ErrorAt: &probe.Diag{File: "gp_probe.cpp", Line: 1, Col: 1, Severity: "error", Msg: tc.msg}}
			if tc.line != "" {
				c.rep.Table.Source = &probe.Source{Text: tc.line}
				res.ErrorAt.Col = strings.Index(tc.line, tc.errAt) + 1
			}
			st := collect.Site{File: "x.gp", Line: 3, Col: 5, Text: tc.text}
			fc := c.newFailCtx(ref, res, st, nil)
			got, related, _ := c.explainFailure(fc)
			if got != tc.want {
				t.Errorf("message:\n got %q\nwant %q", got, tc.want)
			}
			var rel []string
			for _, r := range related {
				rel = append(rel, r.Msg)
			}
			if strings.Join(rel, "\n") != strings.Join(tc.related, "\n") {
				t.Errorf("related:\n got %q\nwant %q", rel, tc.related)
			}
		})
	}
}

// Every message the rules can produce is one of the templates (so that it has an
// explanation), and the templates are what `gocpp explain` knows.
func TestTemplatesAreUsed(t *testing.T) {
	seen := map[string]bool{}
	for _, tmpl := range Templates {
		if seen[tmpl] {
			t.Errorf("template %q twice", tmpl)
		}
		seen[tmpl] = true
		if strings.Count(tmpl, "%") == 0 {
			t.Errorf("template %q has no verb", tmpl)
		}
	}
}
