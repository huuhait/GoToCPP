package probe

import (
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// Constructors for hand-written references.
func pth(parts ...string) *collect.Path { return &collect.Path{Parts: parts} }
func nsp(ns string, parts ...string) *collect.Path {
	return &collect.Path{Ns: ns, Parts: parts}
}
func call(fun collect.Expr, args ...collect.Expr) *collect.Call {
	return &collect.Call{Fun: fun, Args: args}
}
func lit(s string) *collect.Lit                    { return &collect.Lit{Text: s} }
func goT(s string) *collect.GoType                 { return &collect.GoType{Cxx: s} }
func lv(t collect.Expr) *collect.Arg               { return &collect.Arg{T: t, LValue: true} }
func rv(t collect.Expr) *collect.Arg               { return &collect.Arg{T: t} }
func sel(x collect.Expr, n string) *collect.Select { return &collect.Select{X: x, Name: n} }
func str() *collect.Stand                          { return &collect.Stand{Name: "string"} }
func ref(e collect.Expr) *collect.Ref              { return &collect.Ref{Expr: e} }
func inst(x collect.Expr, args ...collect.Expr) *collect.Inst {
	return &collect.Inst{X: x, Args: args}
}
func index(x collect.Expr, args ...collect.Expr) *collect.IndexOrInst {
	return &collect.IndexOrInst{X: x, Args: args}
}

var stdKinds = map[string]interop.Kind{
	"strlen": interop.Value, "BUFSIZ": interop.Macro, "EOF": interop.Macro, "RAYWHITE": interop.Macro,
	"std": interop.Namespace, "std::vector": interop.Template, "std::string": interop.Type,
	"std::map": interop.Template, "std::minmax": interop.ValueTemplate, "std::chrono": interop.Namespace,
	"std::chrono::steady_clock": interop.Type, "std::chrono::steady_clock::now": interop.Value,
	"Color": interop.Type, "GREEN": interop.Value, "printf": interop.Value, "fopen": interop.Value, "FILE": interop.Type,
	"std::get": interop.ValueTemplate, "std::size_t": interop.Type, "table": interop.Value, "Vec": interop.Template,
}

// lines returns the probe lines of text (those that start with "using gp_probe_"
// or "constexpr auto gp_probe_"), without the numbers of the file.
func probeLines(src *Source) []string {
	var out []string
	for _, l := range strings.Split(src.Text, "\n") {
		if strings.HasPrefix(l, "using gp_probe_") || strings.HasPrefix(l, "constexpr auto gp_probe_") || strings.HasPrefix(l, "using gp_decl_") {
			out = append(out, l)
		}
	}
	return out
}

func genOne(t *testing.T, kinds map[string]interop.Kind, e collect.Expr, wantValue bool) *Source {
	t.Helper()
	r := ref(e)
	r.WantValue = wantValue
	return Generate(collect.NewSet(r), GenConfig{Kinds: kinds})
}

func TestGenerateForms(t *testing.T) {
	str := str()
	cases := []struct {
		name  string
		kinds map[string]interop.Kind
		e     collect.Expr
		value bool
		want  []string // lines, in order (after the numbers)
	}{
		{"call with a string variable", stdKinds, call(pth("strlen"), lv(str)),
			false, []string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((::strlen(std::declval<gocpp_probe_internal_pr::string&>())))>;"}},
		{"call with a temporary", stdKinds, call(pth("strlen"), rv(str)),
			false, []string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((::strlen(std::declval<gocpp_probe_internal_pr::string>())))>;"}},
		{"call with an untyped constant", stdKinds, call(pth("strlen"), lit(`"abc"`)),
			false, []string{`using gp_probe_0 = gocpp_probe_internal_c<decltype((::strlen("abc")))>;`}},
		{"macro has no ::", stdKinds, pth("BUFSIZ"), false, []string{
			"using gp_probe_0 = gocpp_probe_internal_c<decltype((BUFSIZ))>;",
			"using gp_probe_0_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((BUFSIZ))>;",
			"constexpr auto gp_probe_0_s = gocpp_probe_internal_pr::str_of((BUFSIZ));"}},
		{"macro with a value probe", stdKinds, pth("BUFSIZ"), true, []string{
			"using gp_probe_0 = gocpp_probe_internal_c<decltype((BUFSIZ))>;",
			"using gp_probe_0_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((BUFSIZ))>;",
			"constexpr auto gp_probe_0_s = gocpp_probe_internal_pr::str_of((BUFSIZ));"}},
		{"value in a constant context", stdKinds, call(pth("strlen"), lit(`"a"`)), true, []string{
			`using gp_probe_0 = gocpp_probe_internal_c<decltype((::strlen("a")))>;`,
			`using gp_probe_0_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((::strlen("a")))>;`,
			`constexpr auto gp_probe_0_s = gocpp_probe_internal_pr::str_of((::strlen("a")));`}},
		{"type through a template", stdKinds, index(nsp("::std", "vector"), goT("int64_t")), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<::std::vector<int64_t>>;"}},
		{"two template arguments", stdKinds, inst(nsp("::std", "map"), str, goT("int64_t")), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<::std::map<gocpp_probe_internal_pr::string, int64_t>>;"}},
		{"function template with arguments", stdKinds, call(inst(nsp("::std", "get"), lit("0"), lit("1")), lv(goT("int"))), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((::std::get<0, 1>(std::declval<int&>())))>;"}},
		{"index on a value", stdKinds, index(pth("table"), lit("3")), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype(((::table)[3]))>;"}},
		{"namespace and static member", stdKinds, call(nsp("::std", "chrono", "steady_clock", "now")), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((::std::chrono::steady_clock::now()))>;"}},
		{"member of a macro", stdKinds, pth("RAYWHITE", "r"), false, []string{
			"using gp_probe_0 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(RAYWHITE).r))>;",
			"using gp_probe_0_v = gocpp_probe_internal_val<gocpp_probe_internal_pr::enc((gocpp_probe_internal_pr::deref(RAYWHITE).r))>;",
			"constexpr auto gp_probe_0_s = gocpp_probe_internal_pr::str_of((gocpp_probe_internal_pr::deref(RAYWHITE).r));"}},
		{"member of a value", stdKinds, sel(lv(&collect.Decay{X: call(pth("fopen"), lit(`"x"`), lit(`"r"`))}), "_flags"), false,
			[]string{`using gp_probe_0 = gocpp_probe_internal_c<decltype((gocpp_probe_internal_pr::deref(std::declval<std::decay_t<decltype((::fopen("x", "r")))>&>())._flags))>;`}},
		{"constructor of a type", stdKinds, call(nsp("::std", "string"), lit(`"abc"`)), false,
			[]string{`using gp_probe_0 = gocpp_probe_internal_c<decltype((::std::string("abc")))>;`}},
		{"static member of a type after an instance", stdKinds, sel(inst(nsp("::std", "map"), goT("int"), goT("int")), "size_type"), false, []string{
			"using gp_probe_0 = gocpp_probe_internal_c<decltype((::std::map<int, int>::size_type))>;",
			"using gp_probe_0_alt = gocpp_probe_internal_c<typename ::std::map<int, int>::size_type>;"}},
		{"unary caret", stdKinds, &collect.Unary{Op: "^", X: pth("EOF")}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype(((~EOF)))>;"}},
		{"and not", stdKinds, &collect.Binary{Op: "&^", X: pth("EOF"), Y: lit("3")}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype(((EOF & ~3)))>;"}},
		{"binary", stdKinds, &collect.Binary{Op: "+", X: call(pth("strlen"), lv(str)), Y: rv(goT("int64_t"))}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype(((::strlen(std::declval<gocpp_probe_internal_pr::string&>()) + std::declval<int64_t>())))>;"}},
		{"deref and address", stdKinds, &collect.Star{X: &collect.Addr{X: lv(goT("int"))}}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype(((*(&std::declval<int&>()))))>;"}},
		{"pointer type", stdKinds, &collect.Star{X: pth("FILE")}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<::FILE*>;"}},
		{"literal with fields", stdKinds, &collect.Composite{Type: pth("Color"), Elts: []collect.Elt{{KeyName: "r", Val: lit("1")}, {KeyName: "g", Val: lit("2")}}}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((::Color{}))>;"}},
		{"literal with positions", stdKinds, &collect.Composite{Type: index(nsp("::std", "vector"), goT("int64_t")), Elts: []collect.Elt{{Val: lit("1")}, {Val: lit("2")}}}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((::std::vector<int64_t>{1, 2}))>;"}},
		{"literal with map keys", stdKinds, &collect.Composite{Type: inst(nsp("::std", "map"), str, goT("int")), Elts: []collect.Elt{{Key: lit(`"a"`), Val: lit("1")}}}, false,
			[]string{`using gp_probe_0 = gocpp_probe_internal_c<decltype((::std::map<gocpp_probe_internal_pr::string, int>{{"a", 1}}))>;`}},
		{"tuple element", stdKinds, &collect.Decay{X: &collect.TupleElem{X: call(nsp("::std", "minmax"), lit("3"), lit("4")), I: 1}}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<std::decay_t<std::tuple_element_t<1, std::remove_cvref_t<decltype((::std::minmax(3, 4)))>>>>;"}},
		{"range key", stdKinds, &collect.Decay{X: &collect.RangeElem{X: lv(goT("int")), Key: true}}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<int&>()))>>::key>>;"}},
		{"range value", stdKinds, &collect.Decay{X: &collect.RangeElem{X: lv(goT("int"))}}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<std::decay_t<typename gocpp_probe_internal_pr::range_t<std::remove_cvref_t<decltype((std::declval<int&>()))>>::val>>;"}},
		{"cgo types", stdKinds, call(nsp("C", "int"), lv(goT("int64_t"))), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((int(std::declval<int64_t&>())))>;"}},
		{"cgo struct", stdKinds, &collect.Star{X: nsp("C", "struct_stat")}, false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<struct stat*>;"}},
		{"cgo helper cstr", stdKinds, call(nsp("C", "cstr"), lv(str)), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((std::declval<const char*>()))>;"}},
		{"slice and array stand-ins", stdKinds, call(pth("strlen"), lv(&collect.Stand{Name: "slice", Args: []collect.Expr{goT("uint8_t")}}), lv(&collect.Stand{Name: "array", Args: []collect.Expr{goT("int"), lit("4")}}), lv(&collect.Stand{Name: "map", Args: []collect.Expr{str, goT("int")}})), false,
			[]string{"using gp_probe_0 = gocpp_probe_internal_c<decltype((::strlen(std::declval<gocpp_probe_internal_pr::slice<uint8_t>&>(), std::declval<gocpp_probe_internal_pr::array<int, 4>&>(), std::declval<gocpp_probe_internal_pr::map<gocpp_probe_internal_pr::string, int>&>())))>;"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := genOne(t, tc.kinds, tc.e, tc.value)
			got := probeLines(src)
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") {
				t.Errorf("lines:\n%s\nwant:\n%s\n(skipped %v)", strings.Join(got, "\n"), strings.Join(tc.want, "\n"), src.Skipped)
			}
		})
	}
}

func TestGenerateAmbiguous(t *testing.T) {
	// No kinds: std::vector may be a template or a value. The index reading
	// can't be written when the operand is a type (std.vector[int]), so only
	// the instance is probed.
	src := Generate(collect.NewSet(ref(index(nsp("::std", "vector"), goT("int64_t")))), GenConfig{})
	got := probeLines(src)
	if len(got) != 1 || got[0] != "using gp_probe_0_alt = gocpp_probe_internal_c<::std::vector<int64_t>>;" {
		t.Fatalf("lines %q", got)
	}
	if l := src.Lines[lineOf(src, "gp_probe_0_alt")]; l.Role != RoleAlt || l.Form != FormType {
		t.Errorf("alt line %+v", l)
	}
	// With a value operand both readings are written.
	src = Generate(collect.NewSet(ref(index(nsp("::std", "vector"), lit("3")))), GenConfig{})
	got = probeLines(src)
	if len(got) != 2 || !strings.HasPrefix(got[0], "using gp_probe_0 = gocpp_probe_internal_c<decltype(((::std::vector)[3]))>") ||
		got[1] != "using gp_probe_0_alt = gocpp_probe_internal_c<::std::vector<3>>;" {
		t.Fatalf("lines %q", got)
	}
}

func lineOf(src *Source, sub string) int {
	for n, l := range src.Lines {
		if strings.Contains(l.Text, sub) {
			return n
		}
	}
	return -1
}

func TestGenerateSkips(t *testing.T) {
	set := collect.NewSet(
		ref(call(pth("strlen"), &collect.Unknown{Why: "a Go struct"})),
		ref(pth("std")),
		ref(&collect.SliceX{X: lv(goT("int"))}),
		ref(&collect.Unknown{Why: "x"}),
		ref(&collect.Composite{Elts: nil}),
	)
	// A name that is only a callee is not probed.
	callee := ref(pth("strlen"))
	set2 := collect.NewSet(callee)
	set2.Refs[0].Standalone = false
	src := Generate(set, GenConfig{Kinds: stdKinds})
	if len(probeLines(src)) != 0 {
		t.Errorf("probed: %q", probeLines(src))
	}
	reasons := map[string]bool{}
	for _, r := range src.Skipped {
		reasons[strings.SplitN(r, ":", 2)[0]] = true
	}
	for _, w := range []string{"incomplete", "a namespace", "C++ has no slice expression"} {
		if !reasons[w] {
			t.Errorf("no skip reason %q in %v", w, src.Skipped)
		}
	}
	if len(src.Skipped) != 5 {
		t.Errorf("skipped %d: %v", len(src.Skipped), src.Skipped)
	}
	src2 := Generate(set2, GenConfig{Kinds: stdKinds})
	if !strings.HasPrefix(src2.Skipped[0], "name only") {
		t.Errorf("skip %v", src2.Skipped)
	}
}

func TestGenerateLocalAlias(t *testing.T) {
	def := &collect.Ref{Expr: &collect.Decay{X: call(nsp("::std", "string"), lit(`"abc"`))}}
	r1 := &collect.Ref{Expr: call(sel(lv(&collect.DeclRef{Def: nil}), "size"))}
	_ = r1
	// A DeclRef needs the Ref of its definition from a Set: build the set first.
	set := collect.NewSet(def)
	d := set.Refs[0]
	use := ref(call(sel(lv(&collect.DeclRef{Def: d}), "size")))
	set = collect.NewSet(def, use)
	src := Generate(set, GenConfig{Kinds: stdKinds})
	lines := probeLines(src)
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "using gp_probe_") || !strings.HasPrefix(lines[1], "using gp_decl_") {
		// the alias comes before the first line that uses it
		t.Logf("lines:\n%s", strings.Join(lines, "\n"))
	}
	aliasAt, useAt := -1, -1
	for i, l := range lines {
		if strings.HasPrefix(l, "using gp_decl_") {
			aliasAt = i
		}
		if strings.Contains(l, "gp_decl_") && strings.HasPrefix(l, "using gp_probe_") {
			useAt = i
		}
	}
	if aliasAt < 0 || useAt < 0 || aliasAt > useAt {
		t.Fatalf("alias at %d, use at %d:\n%s", aliasAt, useAt, strings.Join(lines, "\n"))
	}
	if strings.Count(src.Text, "using gp_decl_") != 1 {
		t.Errorf("alias declared more than once:\n%s", src.Text)
	}
	// The alias line is the definition's own: an error in it is its error.
	if l := src.Lines[lineOf(src, "using gp_decl_")]; l.Role != RoleAlias || l.Ref != d.ID {
		t.Errorf("alias line %+v, definition %d", l, d.ID)
	}
}

func TestGenerateIncludesAndHelpers(t *testing.T) {
	src := Generate(collect.NewSet(), GenConfig{Includes: []string{"<cstdio>", `"./physics.h"`, interop.ExternC(`"./legacy.h"`)}})
	idx := func(s string) int { return strings.Index(src.Text, s) }
	if !(idx("#include <cstddef>") < idx("#include <cstdio>") && idx("#include <cstdio>") < idx(`#include "./physics.h"`) &&
		idx(`extern "C" {`) < idx(`#include "./legacy.h"`) && idx(`#include "./legacy.h"`) < idx("namespace gocpp_probe_internal_pr")) {
		t.Errorf("order of the includes:\n%s", src.Text)
	}
	if strings.Contains(src.Text, "operator const char*") {
		t.Error("the stand-in of string converts to const char* by default")
	}
	src = Generate(collect.NewSet(), GenConfig{StringToCStr: true})
	if !strings.Contains(src.Text, "operator const char*") {
		t.Error("StringToCStr has no effect")
	}
}

func TestGenerateDeterministic(t *testing.T) {
	a := []*collect.Ref{ref(call(pth("strlen"), lit(`"a"`))), ref(pth("BUFSIZ")), ref(index(nsp("::std", "vector"), goT("int64_t")))}
	b := []*collect.Ref{a[2], a[0], a[1]}
	s1 := Generate(collect.NewSet(a...), GenConfig{Kinds: stdKinds})
	s2 := Generate(collect.NewSet(b...), GenConfig{Kinds: stdKinds})
	s3 := Generate(collect.NewSet(a...), GenConfig{Kinds: stdKinds})
	if s1.Text != s2.Text || s1.Text != s3.Text {
		t.Errorf("the probe file depends on the order or the run:\n%s\n---\n%s", s1.Text, s2.Text)
	}
}

func TestProbeLineMap(t *testing.T) {
	set := collect.NewSet(ref(call(pth("strlen"), lit(`"a"`))), ref(pth("BUFSIZ")))
	src := Generate(set, GenConfig{Kinds: stdKinds})
	lines := strings.Split(src.Text, "\n")
	if len(src.Lines) == 0 {
		t.Fatal("no lines")
	}
	for n, l := range src.Lines {
		if lines[n-1] != l.Text {
			t.Errorf("line %d is %q, the map says %q", n, lines[n-1], l.Text)
		}
		if l.Ref < 0 || l.Ref >= len(set.Refs) {
			t.Errorf("line %d: ref %d", n, l.Ref)
		}
	}
	if src.Probes != 2 || src.NRefs != 2 || len(src.Keys) != 2 {
		t.Errorf("probes %d refs %d keys %d", src.Probes, src.NRefs, len(src.Keys))
	}
}
