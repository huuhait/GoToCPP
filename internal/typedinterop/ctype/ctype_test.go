package ctype

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/cputime"
)

// roundTrip lists texts that clang prints and that Parse and String give back
// unchanged.
var roundTrip = []string{
	"int", "unsigned int", "long", "unsigned long", "long long", "unsigned long long", "short", "unsigned short",
	"char", "signed char", "unsigned char", "bool", "void", "float", "double", "long double", "wchar_t", "char8_t",
	"char16_t", "char32_t", "__int128", "unsigned __int128", "_Float16", "__fp16",
	"const int", "volatile int", "const volatile int", "const char *", "char *const", "const char *const",
	"char *const *", "char **", "char *const &", "int *&", "int *&&", "const std::string &", "long &", "long &&",
	"int[4]", "const char[2]", "int[]", "int *[3]", "int (*)[3]", "int (&)[4]", "int (&&)[4]", "int[2][3]", "int (*)[2][3]",
	"int (int, long)", "void ()", "int (*)(int)", "void (*)()", "void (&)(int)", "int *(int)", "int *(*)(int)",
	"void (*(*)(int))(char)", "void (*const)(int)", "int (const char *, ...)", "int (...)",
	"void () const", "void () const noexcept", "void () &", "void () const &&", "void (S::*)()", "void (S::*)() const",
	"int S::*", "int S::*const", "int (S::*&)()", "size_t (const char *) noexcept(true)",
	"std::vector<long>", "std::vector<std::vector<int>>", "std::map<std::string, int>", "std::pair<const int, long>",
	"std::basic_string<char>", "std::vector<int *>", "std::function<void (int, long)>", "std::function<int (&)[4]>",
	"std::array<int, 5>", "std::array<int, -1>", "std::array<char, true>", "std::integral_constant<unsigned long, 8UL>",
	"std::tuple<>", "S<>", "std::vector<long>::iterator", "std::vector<long>::const_iterator",
	"__gnu_cxx::__normal_iterator<long *, std::vector<long>>", "std::reverse_iterator<__gnu_cxx::__normal_iterator<const long *, std::vector<long>>>",
	"struct stat", "union U", "enum Color", "class std::string", "::geo::Vec", "typename T::type",
	"(unnamed struct)", "(unnamed union)", "(unnamed enum)", "(anonymous struct)", "(lambda)", "(anonymous namespace)::Foo",
	"<overloaded function type>", "<bound member function type>", "<dependent type>",
	"decltype(nullptr)", "decltype(__declval<std::vector<long> &>(0))", "std::nullptr_t", "auto",
	"gocpp_probe_internal_pr::slice<std::basic_string<char>>", "gocpp_probe_internal_pr::array<unsigned char, 32>",
	"std::tuple<int, std::pair<long, double>, void (*)(int)>", "std::tuple<Ts...>", "T...", "Args &&...", "void (Args &&...)", "auto (int) -> long", "auto (*)(int) const noexcept -> decltype(f(0))", "__remove_cv(char)", "void (const char *, ...) __attribute__((noreturn)) noexcept", "void (*)() __attribute__((noreturn))", "std::basic_string<char>::(unnamed enum)", "type-parameter-0-1", "X &(Y &&)", "X &(*)(const X &) noexcept", "std::vector<char>::size_type",
	"std::array<int, (1 << 3)>", "std::array<int, sizeof(int)>", "std::chrono::duration<long, std::ratio<1, 1000000000>>",
	"const std::vector<long> *", "std::unique_ptr<int, std::default_delete<int>>", "_Complex double",
	"int *__restrict", "const char *__restrict", "S<int>::T<char>", "std::function<void ()>",
}

func TestRoundTrip(t *testing.T) {
	for _, s := range roundTrip {
		ty, err := Parse(s)
		if err != nil {
			t.Errorf("%q: %v", s, err)
			continue
		}
		if got := ty.String(); got != s {
			t.Errorf("%q prints as %q", s, got)
		}
		again, err := Parse(ty.String())
		if err != nil || !Equal(ty, again) {
			t.Errorf("%q: Parse(String) differs (%v)", s, err)
		}
	}
}

// normal lists texts in another spelling and the normal form they get.
func TestNormal(t *testing.T) {
	for in, want := range map[string]string{
		"long unsigned int":                     "unsigned long",
		"unsigned":                              "unsigned int",
		"signed":                                "int",
		"short int":                             "short",
		"long long unsigned int":                "unsigned long long",
		"int unsigned":                          "unsigned int",
		"int const":                             "const int",
		"const  char   *  const":                "const char *const",
		"char const*":                           "const char *",
		"_Bool":                                 "bool",
		"std::vector< int >":                    "std::vector<int>",
		"std::vector<int > ":                    "std::vector<int>",
		"std::vector<std::vector<int> >":        "std::vector<std::vector<int>>",
		"void(*)(int,long)":                     "void (*)(int, long)",
		"int(void)":                             "int ()",
		"unsigned __int128":                     "unsigned __int128",
		"__int128 unsigned":                     "unsigned __int128",
		"(unnamed struct at /a/b.hpp:1:2)":      "(unnamed struct)",
		"enum class E":                          "enum E",
		"int [ 3 ]":                             "int[3]",
		"auto (*)() noexcept -> decltype(f(0))": "auto (*)() noexcept -> decltype(f(0))",
	} {
		ty, err := Parse(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		if got := ty.String(); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}

func TestStructure(t *testing.T) {
	ty := MustParse("const char *const *")
	if ty.Kind != Pointer || ty.Const || ty.Elem.Kind != Pointer || !ty.Elem.Const || ty.Elem.Elem.Kind != Builtin || !ty.Elem.Elem.Const || ty.Elem.Elem.Name != "char" {
		t.Errorf("%+v", ty)
	}
	if ty.Leaf().Name != "char" {
		t.Error("leaf")
	}

	f := MustParse("int (*)(const char *, ...)")
	if f.Kind != Pointer || f.Elem.Kind != Func || !f.Elem.Variadic || len(f.Elem.Params) != 1 || f.Elem.Elem.Name != "int" {
		t.Errorf("%+v", f.Elem)
	}

	a := MustParse("int (&)[4]")
	if a.Kind != LRef || a.Elem.Kind != Array || a.Elem.Len != 4 || a.Elem.Elem.Name != "int" {
		t.Errorf("%+v", a)
	}
	if a.StripRef().Kind != Array {
		t.Error("StripRef")
	}

	n := MustParse("std::map<std::basic_string<char>, std::vector<long>>::iterator")
	if n.Kind != Named || n.QualName() != "std::map::iterator" || n.Last() != "iterator" || len(n.Path[1].Args) != 2 {
		t.Errorf("%+v", n)
	}
	if n.Path[1].Args[0].Type.QualName() != "std::basic_string" {
		t.Errorf("%+v", n.Path[1].Args[0])
	}

	m := MustParse("void (Foo::*)(int) const &")
	if m.Kind != MemberPtr || m.Class.QualName() != "Foo" || m.Elem.Kind != Func || !m.Elem.ConstQ || m.Elem.RefQual != "&" {
		t.Errorf("%+v", m)
	}

	e := MustParse("std::array<int, 5>")
	if args := e.Args(); len(args) != 2 || args[0].Type == nil || args[1].Expr != "5" || args[1].Type != nil {
		t.Errorf("%+v", args)
	}
	pk := MustParse("std::tuple<Ts...>")
	if args := pk.Args(); len(args) != 1 || !args[0].Type.Pack || args[0].Type.QualName() != "Ts" {
		t.Errorf("%+v", args)
	}

	// Clone is deep and Equal sees differences.
	c := ty.Clone()
	if !Equal(ty, c) {
		t.Error("clone differs")
	}
	c.Elem.Elem.Const = false
	if Equal(ty, c) || !ty.Elem.Elem.Const {
		t.Error("clone shares")
	}
	if Equal(MustParse("int"), MustParse("unsigned int")) || Equal(MustParse("S<int>"), MustParse("S<long>")) ||
		Equal(MustParse("void ()"), MustParse("void () const")) || Equal(MustParse("int[3]"), MustParse("int[4]")) ||
		Equal(MustParse("void (*)()"), MustParse("void (&)()")) || Equal(MustParse("a::b"), MustParse("::a::b")) {
		t.Error("Equal says equal")
	}

	var names []string
	MustParse("std::map<K, V (*)(W)> *").Walk(func(t *Type) {
		if t.Kind == Named {
			names = append(names, t.QualName())
		}
	})
	if strings.Join(names, ",") != "std::map,K,V,W" {
		t.Errorf("walk: %v", names)
	}
	if MustParse("void ()").Params != nil || !MustParse("void (void)").Elem.Const == false {
		t.Error("(void)")
	}
}

// A nested function type in a template argument is read in linear time (isGroup used to parse
// the name twice at every level: 38 s for 24 levels). The time is the processor time of the
// parse (cputime.Of), so a loaded machine does not stretch it; a parse that does not end in a
// minute is an exponential one, and the test does not wait for it.
func TestNestedFunctionTypesAreLinear(t *testing.T) {
	type res struct {
		ty  *Type
		err error
		d   time.Duration
	}
	done := make(chan res, 1)
	parse := func(what, s string) res {
		t.Helper()
		go func() {
			var r res
			r.d, _ = cputime.Of(func() { r.ty, r.err = Parse(s) })
			done <- r
		}()
		select {
		case r := <-done:
			if r.d > time.Second {
				t.Errorf("%s took %v", what, r.d)
			}
			return r
		case <-time.After(time.Minute): // a regression is exponential: do not wait for it
			t.Fatalf("%s did not finish in a minute", what)
		}
		return res{}
	}
	s := "int"
	for i := 0; i < 40; i++ {
		s = "int (S<" + s + ">)"
	}
	r := parse("40 levels of nested function types", s)
	if r.err != nil {
		t.Fatal(r.err)
	}
	if r.ty.String() != s {
		t.Error("not the same text")
	}
	// Past the nesting limit the rejection is as fast: the failure is remembered too.
	for _, n := range []int{400, 2000} {
		s := "int"
		for i := 0; i < n; i++ {
			s = "int (S<" + s + ">)"
		}
		if r := parse(fmt.Sprintf("%d levels (past the limit)", n), s); r.err == nil {
			t.Errorf("%d levels of nesting parse", n)
		}
	}
	// The same with member pointers and pointers to functions, which take the other branch.
	s = "int"
	for i := 0; i < 40; i++ {
		s = "int (S<" + s + ">::*)(S<" + s + ">)"
		if len(s) > 100000 {
			break
		}
	}
	parse("nested member pointers", s)
}

// Types with no name print in a form that Parse reads (found by FuzzParse).
func TestUnnamedForms(t *testing.T) {
	for in, want := range map[string]string{
		"(anonymous at )": "(anonymous)", "(unnamed at x)": "(unnamed)", "(lambda)": "(lambda)", "(unnamed struct at f.h:1:2)": "(unnamed struct)",
		"S::(anonymous at )": "S::(anonymous)", "__remove_cvref const(A)": "const __remove_cvref (A)",
	} {
		ty, err := Parse(in)
		if err != nil {
			t.Errorf("%q: %v", in, err)
			continue
		}
		out := ty.String()
		again, err := Parse(out)
		if out != want || err != nil || !Equal(ty, again) {
			t.Errorf("%q: %q (want %q), reparse %v", in, out, want, err)
		}
	}
}

func TestErrors(t *testing.T) {
	for _, s := range []string{
		"", "   ", "*", "int (", "int )", "std::vector<", "std::vector<int", "int [", "int[3", "(unnamed struct", "<foo",
		"unsigned float", "long long long", "short long", "int __attribute__((vector_size(16)))",
		"int int2 float", "struct", "std::", "::", "int 3", "const", "int *3", "S<int,>", "S<,int>", "A& &", "int &*", "int &[3]", "void ()[3]", "void () ()", "A&& &",
	} {
		if ty, err := Parse(s); err == nil {
			t.Errorf("%q parses as %q", s, ty)
		} else if _, ok := err.(*ParseError); !ok {
			t.Errorf("%q: error %T", s, err)
		}
	}
}

// A text from a header is not trusted to be shallow: no crash, an error.
func TestDeep(t *testing.T) {
	deep := strings.Repeat("A<", 5000) + "int" + strings.Repeat(">", 5000)
	if _, err := Parse(deep); err == nil {
		t.Error("5000 levels of templates parse")
	}
	deepP := strings.Repeat("(*", 5000) + "int" + strings.Repeat(")", 5000)
	if _, err := Parse("int " + deepP); err == nil {
		t.Error("5000 levels of parentheses parse")
	}
	ok := strings.Repeat("A<", 100) + "int" + strings.Repeat(">", 100)
	if ty, err := Parse(ok); err != nil || ty.String() != ok {
		t.Errorf("100 levels: %v", err)
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range roundTrip {
		f.Add(s)
	}
	f.Add("(anonymous at )")
	f.Add("(unnamed at x)")
	f.Add("int (S<int (S<int (S<int>)>)>)")
	f.Add("__remove_cvref const(A)")
	f.Add(strings.Repeat("int (S<", 320) + "int" + strings.Repeat(">)", 320))
	f.Fuzz(func(t *testing.T, s string) {
		ty, err := Parse(s)
		if err != nil {
			return
		}
		out := ty.String()
		again, err := Parse(out)
		if err != nil {
			t.Fatalf("%q prints as %q which does not parse: %v", s, out, err)
		}
		if !Equal(ty, again) {
			t.Fatalf("%q -> %q -> %q", s, out, again)
		}
	})
}
