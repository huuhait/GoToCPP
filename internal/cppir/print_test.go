package cppir

import (
	"strings"
	"testing"
)

// Short constructors for the tests.

func n(s string) *Name              { return &Name{Name: s} }
func l(s string) *Lit               { return &Lit{Text: s} }
func tn(s string) *TypeName         { return &TypeName{Name: s} }
func es(p Pos, x Expr) *ExprStmt    { return &ExprStmt{Pos: p, X: x} }
func bin(op string, x, y Expr) Expr { return &Binary{Op: op, X: x, Y: y} }
func cl(f string, args ...Expr) Expr {
	return &Call{Fun: n(f), Args: args}
}
func vd(p Pos, typ, name string, init Expr) *VarDecl {
	return &VarDecl{Pos: p, Type: tn(typ), Vars: []Declarator{{Name: name, Init: init}}}
}

// gp returns the place of line in /a.gp.
func gp(line int) Pos { return Pos{File: "/a.gp", Line: line} }

// fn returns a definition void f() with the statements, at line 1 of /a.gp, and its closing
// brace at end.
func fn(end int, stmts ...Stmt) *Func {
	return &Func{Pos: gp(1), Result: tn("void"), Name: "f", Body: &Block{Stmts: stmts, End: gp(end)}}
}

// checkPrint checks that Print writes want for the declarations, without the file's own path.
func checkPrint(t *testing.T, want string, decls ...Decl) {
	t.Helper()
	got := string(Print(&File{Decls: decls}, ""))
	want = strings.TrimPrefix(want, "\n")
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestPrintLines checks the #line rule on whole statements: a directive before a line whose
// place is not the line before + 1, none before the zero Pos (plain lines, which count), and
// every line of a node at its own place: the closing brace at End.
func TestPrintLines(t *testing.T) {
	checkPrint(t, `
// header
#include <gocpp/rt.hpp>

namespace gpkg_main {
void f();
#line 1 "/a.gp"
void f() {
    x();
#line 2 "/a.gp"
    y();
    z();
#line 7 "/a.gp"
    {
        w();
        v();
#line 4 "/a.gp"
    }
    u();
    generated();
    t();
#line 12 "/b.gp"
    s();
#line 9 "/a.gp"
}
} // namespace gpkg_main
`,
		&Comment{Text: "header"},
		&Include{Path: "gocpp/rt.hpp", System: true},
		&Blank{},
		&Namespace{Name: "gpkg_main", Decls: []Decl{
			&Func{Result: tn("void"), Name: "f"},
			fn(9,
				es(gp(2), cl("x")), // follows the header's line: none
				es(gp(2), cl("y")), // a second C++ line for line 2: again
				es(gp(3), cl("z")),
				&Block{Pos: gp(7), Stmts: []Stmt{es(gp(8), cl("w")), es(gp(9), cl("v"))}, End: gp(4)},
				es(gp(5), cl("u")),
				es(Pos{}, cl("generated")), // plain: it takes line 6
				es(gp(7), cl("t")),
				es(Pos{File: "/b.gp", Line: 12}, cl("s")),
			),
		}},
	)
}

// TestPrintOwnLines checks that with the file's own path (Config.OutputPath), a plain line
// after mapped lines maps back to the generated file, at its real line, and a blank one
// doesn't.
func TestPrintOwnLines(t *testing.T) {
	f := &File{Decls: []Decl{
		&Comment{Text: "header"},
		fn(3, es(gp(2), cl("x"))),
		&Blank{},
		&Func{Result: tn("int"), Name: "main", Body: &Block{Stmts: []Stmt{&Return{X: l("0")}}}},
	}}
	want := `// header
#line 1 "/a.gp"
void f() {
    x();
}

#line 8 "/work/main.gp.cpp"
int main() {
    return 0;
}
`
	if got := string(Print(f, "/work/main.gp.cpp")); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestPrintJoined checks the nodes that share a line: an inline Block, a Line, a Line that
// ends with a Scope, and nested Scopes, which close together on one line, } } }, also through
// a Line. A Scope that ends with anything else closes on its own.
func TestPrintJoined(t *testing.T) {
	t0 := vd(Pos{}, "int64_t", "gp_t0", bin("+", n("v"), l("1")))
	inner := &Scope{Pos: gp(5), Head: []Stmt{vd(Pos{}, "int64_t", "w", n("gp_t1"))}, Stmts: []Stmt{
		es(gp(6), cl("g", n("v"), n("w"))),
		&Line{Pos: gp(7), Stmts: []Stmt{
			vd(Pos{}, "int64_t", "gp_t2", cl("f")),
			&Scope{Pos: gp(7), Head: []Stmt{vd(Pos{}, "int64_t", "f", n("gp_t2"))}, Stmts: []Stmt{
				es(gp(8), cl("h", n("f"))),
			}, End: gp(10)},
		}},
	}, End: gp(10)}
	outer := &Scope{Pos: gp(4), Head: []Stmt{vd(Pos{}, "int64_t", "v", n("gp_t0"))}, Stmts: []Stmt{
		&Line{Pos: gp(5), Stmts: []Stmt{vd(Pos{}, "int64_t", "gp_t1", n("w")), inner}},
	}, End: gp(10)}
	lone := &Scope{Pos: gp(11), Head: []Stmt{vd(Pos{}, "int64_t", "k", l("2"))}, Stmts: []Stmt{
		&If{Pos: gp(12), Cond: n("k"), Then: &Block{Stmts: []Stmt{&Return{Pos: gp(13)}}, End: gp(14)}},
	}, End: gp(15)}
	checkPrint(t, `
#line 1 "/a.gp"
void f() {
    { auto gp_t3 = f(); g(x, gp_t3); }
    a = b; b = c;
    int64_t gp_t0 = v + 1; { int64_t v = gp_t0;
        int64_t gp_t1 = w; { int64_t w = gp_t1;
            g(v, w);
            int64_t gp_t2 = f(); { int64_t f = gp_t2;
                h(f);
#line 10 "/a.gp"
    } } }
    { int64_t k = 2;
        if (k) {
            return;
        }
    }
    {}
    {}
}
`,
		fn(18,
			&Block{Pos: gp(2), Stmts: []Stmt{vd(Pos{}, "auto", "gp_t3", cl("f")), es(Pos{}, cl("g", n("x"), n("gp_t3")))}, Inline: true},
			&Line{Pos: gp(3), Stmts: []Stmt{
				es(Pos{}, &Assign{Op: "=", X: n("a"), Y: n("b")}),
				es(Pos{}, &Assign{Op: "=", X: n("b"), Y: n("c")}),
			}},
			&Line{Pos: gp(4), Stmts: []Stmt{t0, outer}},
			lone,
			&Block{Pos: gp(16), Inline: true},
			&Line{Pos: gp(17), Stmts: []Stmt{&Block{Inline: true}}}, // a Line of one
		),
	)
}

// TestPrintControl checks if-else chains, the loops, a switch with } break; and labels.
func TestPrintControl(t *testing.T) {
	chain := &If{Pos: gp(2), Cond: n("a"), Then: &Block{Stmts: []Stmt{es(gp(3), cl("x"))}},
		Else: &If{Pos: gp(4), Cond: n("b"), Then: &Block{Stmts: []Stmt{es(gp(5), cl("y"))}},
			Else: &Block{Pos: gp(6), Stmts: []Stmt{es(gp(7), cl("z"))}, End: gp(8)}}}
	stop := &Line{Pos: gp(10), Stmts: []Stmt{
		vd(Pos{}, "auto", "gp_t0", cl("g")),
		&If{Cond: &Unary{Op: "!", X: cl("f", n("gp_t0"))}, Then: &Break{}},
	}}
	forever := &For{Pos: gp(9), Body: &Block{Stmts: []Stmt{stop, &Continue{Pos: gp(11)}}, End: gp(12)}}
	counted := &For{Pos: gp(13),
		Init: &VarDecl{Type: tn("int64_t"), Vars: []Declarator{{Name: "i", Init: l("0")}, {Name: "gp_n0", Init: n("n")}}},
		Cond: bin("<", n("i"), n("gp_n0")),
		Post: &Postfix{X: n("i"), Op: "++"},
		Body: &Block{Stmts: []Stmt{&Goto{Pos: gp(14), Label: "gp_L_continue"}, &Label{Pos: gp(15), Name: "gp_L_continue"}}, End: gp(15)}}
	noInit := &For{Pos: gp(16), Cond: n("c"), Post: &Assign{Op: "+=", X: n("i"), Y: l("2")}, Body: &Block{End: gp(16), Inline: true}}
	loop := &While{Pos: gp(17), Cond: bin("<", n("i"), l("3")), Body: &Block{Stmts: []Stmt{es(gp(18), &Postfix{X: n("i"), Op: "++"})}, End: gp(19)}}
	sw := &Switch{Pos: gp(20), Tag: n("x"), Cases: []*Case{
		{Pos: gp(21), Values: []Expr{l("1"), l("2")}, Body: []Stmt{&Attr{Pos: gp(22), Name: "fallthrough"}}, End: gp(22)},
		{Pos: gp(23), Body: []Stmt{es(gp(24), cl("d"))}, End: gp(24), Break: true},
	}, End: gp(25)}
	tagged := &Line{Pos: gp(26), Stmts: []Stmt{vd(Pos{}, "auto", "gp_t1", cl("f")), &Switch{Pos: gp(26), Tag: n("gp_t1"), End: gp(27)}}}
	checkPrint(t, `
#line 1 "/a.gp"
void f() {
    if (a) {
        x();
    } else if (b) {
        y();
    } else {
        z();
    }
    for (;;) {
        auto gp_t0 = g(); if (!f(gp_t0)) break;
        continue;
    }
    for (int64_t i = 0, gp_n0 = n; i < gp_n0; i++) {
        goto gp_L_continue;
        gp_L_continue:;
#line 15 "/a.gp"
    }
    for (; c; i += 2) {}
    while (i < 3) {
        i++;
    }
    switch (x) {
    case 1: case 2: {
        [[fallthrough]];
#line 22 "/a.gp"
    }
    default: {
        d();
#line 24 "/a.gp"
    } break;
    }
    auto gp_t1 = f(); switch (gp_t1) {
    }
    return;
#line 28 "/a.gp"
}
`,
		fn(28, chain, forever, counted, noInit, loop, sw, tagged, &Return{Pos: gp(28)}),
	)
}

// TestPrintDecls checks the declarations that Phase 03 adds: structs with fields, access
// labels and members, templates, specifiers and qualifiers, namespace variables, try/catch
// and lambdas. Everything is plain (the zero Pos), so there is no #line.
func TestPrintDecls(t *testing.T) {
	point := &Struct{Pos: Pos{}, Name: "Point", Members: []Decl{
		&Access{Pos: Pos{}, Name: "public"},
		vd(Pos{}, "int64_t", "X", l("0")),
		&VarDecl{Pos: Pos{}, Type: tn("int64_t"), Vars: []Declarator{{Name: "Y", Init: &InitList{}}}},
		&Func{Pos: Pos{}, Result: tn("bool"), Name: "operator==", Params: []Param{{Type: &Ref{Elem: &Const{Elem: tn("Point")}}}}, Quals: []string{"const", "= default"}},
		&Func{Pos: Pos{}, Specs: []string{"static"}, Result: tn("void"), Name: "Inc", Params: []Param{{Type: &TypeName{Name: "gp::ptr", Args: []Node{tn("Point")}}, Name: "p"}}},
		&Using{Pos: Pos{}, Name: "newtype::newtype"},
	}, End: Pos{}}
	box := &Struct{Pos: Pos{}, Template: &Template{Params: []Param{{Type: tn("typename"), Name: "T"}, {Type: tn("int"), Name: "N"}}},
		Name: "Box", Bases: []Type{&TypeName{Name: "gp::newtype", Args: []Node{tn("Box"), tn("double")}}}, End: Pos{}}
	counter := &Func{Pos: Pos{}, Result: &TypeName{Name: "gp::func", Args: []Node{&FuncType{Result: tn("int64_t")}}}, Name: "counter", Body: &Block{Stmts: []Stmt{
		vd(Pos{}, "gp::ptr<int64_t>", "n", &Call{Fun: &Name{Name: "gp::new_", Args: []Node{tn("int64_t")}}, Args: []Expr{l("0")}}),
		&Return{Pos: Pos{}, X: &Lambda{Captures: []Capture{{Name: "n"}}, Result: tn("int64_t"), Body: &Block{Stmts: []Stmt{
			es(Pos{}, &Postfix{X: &Paren{X: &Unary{Op: "*", X: n("n")}}, Op: "++"}),
			&Return{X: &Paren{X: &Unary{Op: "*", X: n("n")}}},
		}}}},
	}, End: Pos{}}}
	deferred := fn(0,
		&Try{Pos: Pos{}, Body: &Block{Stmts: []Stmt{&Goto{Pos: Pos{}, Label: "gp_exit"}}}, Catches: []*Catch{
			{Pos: Pos{}, Body: &Block{Stmts: []Stmt{es(Pos{}, &Call{Fun: &Member{X: n("gp_df"), Name: "on_panic"}, Args: []Expr{cl("std::current_exception")}})}, End: Pos{}}},
		}},
		&Label{Pos: Pos{}, Name: "gp_exit"},
	)
	deferred.Pos, deferred.Body.End = Pos{}, Pos{}
	constinit := &VarDecl{Pos: Pos{}, Specs: []string{"constinit"}, Type: tn("gp::string"), Vars: []Declarator{
		{Name: "s", Init: &InitList{Elems: []Expr{n("gp::static_bytes"), l(`"go"`), l("2")}}},
	}}
	checkPrint(t, `
struct Point {
public:
    int64_t X = 0;
    int64_t Y{};
    bool operator==(const Point&) const = default;
    static void Inc(gp::ptr<Point> p);
    using newtype::newtype;
};
template <typename T, int N>
struct Box : gp::newtype<Box, double> {
};
struct Box;
gp::func<int64_t()> counter() {
    gp::ptr<int64_t> n = gp::new_<int64_t>(0);
    return [n]() -> int64_t { (*n)++; return (*n); };
}
void f() {
    try {
        goto gp_exit;
    } catch (...) {
        gp_df.on_panic(std::current_exception());
    }
    gp_exit:;
}
constinit gp::string s{gp::static_bytes, "go", 2};
`,
		point, box, &Struct{Pos: Pos{}, Name: "Box", Forward: true}, counter, deferred, constinit)
}

// TestPrintExprs checks the expressions and types, and the parentheses that the printer adds
// where C++'s precedence needs them (and only there).
func TestPrintExprs(t *testing.T) {
	a, b, c := n("a"), n("b"), n("c")
	tests := []struct {
		x    Expr
		want string
	}{
		{bin("-", a, bin("-", b, c)), "a - (b - c)"},
		{bin("-", bin("-", a, b), c), "a - b - c"},
		{bin("*", bin("+", a, b), c), "(a + b) * c"},
		{bin("+", a, bin("*", b, c)), "a + b * c"},
		{bin("||", bin("&&", a, b), c), "a && b || c"}, // readability parens are lowering's Paren
		{bin("||", &Paren{X: bin("&&", a, b)}, c), "(a && b) || c"},
		{bin("==", bin("<", a, b), c), "a < b == c"},
		{bin("<<", bin("+", a, b), l("3")), "a + b << 3"},
		{&Unary{Op: "-", X: &Unary{Op: "-", X: a}}, "-(-a)"},
		{&Unary{Op: "-", X: l("-5")}, "-(-5)"},
		{&Unary{Op: "+", X: &Unary{Op: "+", X: a}}, "+(+a)"},
		{&Unary{Op: "&", X: &Unary{Op: "&", X: a}}, "&(&a)"},
		{&Unary{Op: "!", X: &Unary{Op: "!", X: a}}, "!!a"},
		{&Unary{Op: "~", X: bin("+", a, b)}, "~(a + b)"},
		{&Unary{Op: "-", X: cl("INT64_C", l("5"))}, "-INT64_C(5)"},
		{&Call{Fun: bin("+", a, b), Args: []Expr{c}}, "(a + b)(c)"},
		{cl("f", bin(",", a, b), c), "f((a, b), c)"},
		{&Index{X: &Unary{Op: "*", X: a}, Index: bin("+", b, l("1"))}, "(*a)[b + 1]"},
		{&Member{X: &Unary{Op: "*", X: a}, Name: "f"}, "(*a).f"},
		{&Member{X: a, Arrow: true, Name: "f"}, "a->f"},
		{&Postfix{X: &Index{X: a, Index: l("0")}, Op: "++"}, "a[0]++"},
		{&Assign{Op: "=", X: a, Y: &Assign{Op: "=", X: b, Y: c}}, "a = b = c"},
		{&Assign{Op: "+=", X: a, Y: &Cond{Cond: b, X: l("1"), Y: l("2")}}, "a += b ? 1 : 2"},
		{&Cond{Cond: &Assign{Op: "=", X: a, Y: b}, X: c, Y: &Assign{Op: "=", X: a, Y: c}}, "(a = b) ? c : a = c"},
		{&Cast{Kind: "static_cast", Type: tn("void"), X: bin("+", a, b)}, "static_cast<void>(a + b)"},
		{&InitList{Type: tn("Point"), Elems: []Expr{&Designated{Name: "X", X: l("1")}, &Designated{Name: "Y", X: l("2")}}}, "Point{.X = 1, .Y = 2}"},
		{&InitList{Elems: []Expr{a, b}}, "{a, b}"},
		{&Member{X: &InitList{Type: tn("T")}, Name: "x"}, "T{}.x"},
		{&Call{Fun: &Name{Name: "std::get", Args: []Node{l("0")}}, Args: []Expr{a}}, "std::get<0>(a)"},
		{&Name{Name: "gp::array", Args: []Node{tn("int64_t"), bin(">", a, b)}}, "gp::array<int64_t, (a > b)>"},
		{&Lambda{Params: []Param{{Type: &Ref{Elem: &Const{Elem: tn("auto")}}, Pack: true, Name: "gp_a"}}, Body: &Block{Stmts: []Stmt{es(Pos{}, cl("gp::println", &Expand{X: n("gp_a")}))}}}, "[](const auto&... gp_a) { gp::println(gp_a...); }"},
		{&Lambda{Default: "&", Body: &Block{}}, "[&]() {}"},
		{&Lambda{Default: "=", Captures: []Capture{{Name: "this"}, {Name: "m", ByRef: true}}, Body: &Block{}}, "[=, this, &m]() {}"},
		{&Lambda{
			Captures: []Capture{{Name: "n"}, {Name: "r", ByRef: true, Init: n("x")}, {Name: "c", Init: bin(",", a, b)}},
			Params:   []Param{{Type: tn("int64_t"), Name: "v"}},
			Specs:    []string{"mutable", "noexcept"},
			Result:   tn("bool"),
			Body:     &Block{Stmts: []Stmt{&Return{X: bin("<", n("n"), n("v"))}}},
		}, "[n, &r = x, c = (a, b)](int64_t v) mutable noexcept -> bool { return n < v; }"},
	}
	for _, tt := range tests {
		if got := ExprString(tt.x); got != tt.want {
			t.Errorf("got %s, want %s", got, tt.want)
		}
	}
	types := []struct {
		t    Type
		want string
	}{
		{&TypeName{Name: "std::tuple", Args: []Node{tn("int64_t"), tn("bool")}}, "std::tuple<int64_t, bool>"},
		{&Ref{Elem: &Const{Elem: tn("gp::string")}}, "const gp::string&"},
		{&Ref{Elem: tn("T"), RValue: true}, "T&&"},
		{&Const{Elem: &Ptr{Elem: tn("char")}}, "char* const"},
		{&Ptr{Elem: &Const{Elem: tn("char")}}, "const char*"},
		{&Ptr{Elem: &Ptr{Elem: tn("char")}}, "char**"},
		{&TypeName{Name: "gp::func", Args: []Node{&FuncType{Result: tn("gp::string"), Params: []Type{tn("int64_t"), tn("bool")}}}}, "gp::func<gp::string(int64_t, bool)>"},
	}
	for _, tt := range types {
		if got := TypeString(tt.t); got != tt.want {
			t.Errorf("got %s, want %s", got, tt.want)
		}
	}
}

// TestPrec checks the precedence of each kind of expression.
func TestPrec(t *testing.T) {
	tests := []struct {
		x    Expr
		want int
	}{
		{n("x"), PrecPrimary},
		{l("1"), PrecPrimary},
		{&Paren{X: bin(",", n("a"), n("b"))}, PrecPrimary},
		{&InitList{}, PrecPrimary},
		{&Lambda{Body: &Block{}}, PrecPrimary},
		{&InitList{Type: tn("T")}, PrecPostfix},
		{cl("f"), PrecPostfix},
		{&Index{X: n("a"), Index: l("0")}, PrecPostfix},
		{&Member{X: n("a"), Name: "b"}, PrecPostfix},
		{&Postfix{X: n("a"), Op: "++"}, PrecPostfix},
		{&Cast{Kind: "static_cast", Type: tn("int"), X: n("a")}, PrecPostfix},
		{&Unary{Op: "-", X: n("a")}, PrecUnary},
		{bin("*", n("a"), n("b")), PrecMul},
		{bin("-", n("a"), n("b")), PrecAdd},
		{bin(">>", n("a"), n("b")), PrecShift},
		{bin("<=>", n("a"), n("b")), PrecCmp3},
		{bin(">=", n("a"), n("b")), PrecRel},
		{bin("!=", n("a"), n("b")), PrecEq},
		{bin("&", n("a"), n("b")), PrecBitAnd},
		{bin("^", n("a"), n("b")), PrecBitXor},
		{bin("|", n("a"), n("b")), PrecBitOr},
		{bin("&&", n("a"), n("b")), PrecLogAnd},
		{bin("||", n("a"), n("b")), PrecLogOr},
		{&Assign{Op: "=", X: n("a"), Y: n("b")}, PrecAssign},
		{&Cond{Cond: n("a"), X: n("b"), Y: n("c")}, PrecAssign},
		{bin(",", n("a"), n("b")), PrecComma},
	}
	for _, tt := range tests {
		if got := Prec(tt.x); got != tt.want {
			t.Errorf("Prec(%s) = %d, want %d", ExprString(tt.x), got, tt.want)
		}
	}
}

// TestPrintInline checks the one-line forms of the statements, which a Line or an inline
// Block uses whatever the statement is.
func TestPrintInline(t *testing.T) {
	ifElse := &If{Cond: n("c"), Then: &Block{Stmts: []Stmt{es(Pos{}, cl("a"))}}, Else: &Block{Stmts: []Stmt{es(Pos{}, cl("b"))}}}
	sw := &Switch{Tag: n("x"), Cases: []*Case{{Values: []Expr{l("1")}, Body: []Stmt{&Return{}}}, {Body: []Stmt{&Break{}}, Break: true}}}
	checkPrint(t, `
#line 1 "/a.gp"
void f() {
    if (c) { a(); } else { b(); } x = 1; for (;;) {} while (c) { g(); } switch (x) { case 1: { return; } default: { break; } break; } try { g(); } catch (const E& e) { h(); } { int64_t v = 1; g(v); } /* note */
#line 1 "/a.gp"
}
`,
		fn(1, &Line{Pos: gp(2), Stmts: []Stmt{
			ifElse,
			es(Pos{}, &Assign{Op: "=", X: n("x"), Y: l("1")}),
			&For{Body: &Block{}},
			&While{Cond: n("c"), Body: &Block{Stmts: []Stmt{es(Pos{}, cl("g"))}}},
			sw,
			&Try{Body: &Block{Stmts: []Stmt{es(Pos{}, cl("g"))}}, Catches: []*Catch{{Param: &Param{Type: &Ref{Elem: &Const{Elem: tn("E")}}, Name: "e"}, Body: &Block{Stmts: []Stmt{es(Pos{}, cl("h"))}}}}},
			&Scope{Head: []Stmt{vd(Pos{}, "int64_t", "v", l("1"))}, Stmts: []Stmt{es(Pos{}, cl("g", n("v")))}},
			&Comment{Text: "note"},
		}}),
	)
}

// TestPrintMore checks a comment on its own line, an if that goes on over more lines only
// because of its else (with an inline Then, and with a Then that is not a block), and an
// anonymous namespace.
func TestPrintMore(t *testing.T) {
	// A Then or Else that the If writes over several lines uses its statements' own places.
	elseBlock := &Block{Pos: gp(5), Stmts: []Stmt{es(gp(6), cl("b"))}, End: gp(7)}
	inlineThen := &If{Pos: gp(3), Cond: n("c"), Then: &Block{Stmts: []Stmt{es(gp(4), cl("a"))}, Inline: true}, Else: elseBlock}
	elseIf := &If{Pos: gp(10), Cond: n("d"), Then: &Break{Pos: gp(11)}, Else: &Block{Pos: gp(12), Stmts: []Stmt{es(gp(13), cl("e"))}, End: gp(14)}}
	breakThen := &If{Pos: gp(8), Cond: n("c"), Then: &Break{Pos: gp(9)}, Else: elseIf}
	checkPrint(t, `
namespace {
#line 1 "/a.gp"
void f() {
    // a note
    if (c) {
        a();
    } else {
        b();
    }
    if (c) {
        break;
    } else if (d) {
        break;
    } else {
        e();
    }
    if (c) { a(); } else { b(); }
#line 1 "/a.gp"
}
} // namespace
`,
		&Namespace{Decls: []Decl{fn(1,
			&Comment{Pos: gp(2), Text: "a note"},
			inlineThen,
			breakThen,
			&If{Pos: gp(15), Cond: n("c"), Then: &Block{Stmts: []Stmt{es(Pos{}, cl("a"))}, Inline: true}, Else: &Block{Stmts: []Stmt{es(Pos{}, cl("b"))}, Inline: true}},
		)}},
	)
}

// TestPrintBadText checks that text that would break the output is a panic (a bug in the
// tree): */ in a comment that shares its line, a newline in a name.
func TestPrintBadText(t *testing.T) {
	bad := []Stmt{
		&Line{Stmts: []Stmt{&Comment{Text: "a */ b"}, es(Pos{}, cl("f"))}},
		es(gp(1), n("a\nb")),
	}
	for _, s := range bad {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("no panic for %#v", s)
				}
			}()
			Print(&File{Decls: []Decl{fn(1, s)}}, "")
		}()
	}
}
