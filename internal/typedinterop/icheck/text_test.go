package icheck

import (
	"reflect"
	"testing"
)

func TestSplitCall(t *testing.T) {
	tests := []struct {
		in     string
		callee string
		args   []string
		ok     bool
	}{
		{"C.abs(1)", "C.abs", []string{"1"}, true},
		{"C.area(p, 1, 2)", "C.area", []string{"p", "1", "2"}, true},
		{"C.nothing()", "C.nothing", nil, true},
		{"C.gcounter().inc()", "C.gcounter().inc", nil, true},
		{`C.printf("a,b)", f(x, y), z)`, "C.printf", []string{`"a,b)"`, "f(x, y)", "z"}, true},
		{"v.push_back('(')", "v.push_back", []string{"'('"}, true},
		{`f("\")", 2)`, "f", []string{`"\")"`, "2"}, true},
		{"a[1](x)", "a[1]", []string{"x"}, true},
		{"f(g(1), h{2, 3})", "f", []string{"g(1)", "h{2, 3}"}, true},
		{"C.SIZE", "", nil, false},
		{"(1 + 2)", "", []string{"1 + 2"}, true}, // a parenthesized expression is a call of nothing
		{"f(1", "", nil, false},
		{"f(1))", "", nil, false},
	}
	for _, tc := range tests {
		callee, args, ok := splitCall(tc.in)
		if ok != tc.ok || ok && (callee != tc.callee || !reflect.DeepEqual(args, tc.args)) {
			t.Errorf("splitCall(%q) = %q, %q, %v; want %q, %q, %v", tc.in, callee, args, ok, tc.callee, tc.args, tc.ok)
		}
	}
}

func TestUserText(t *testing.T) {
	hdrs := map[string]string{"area": "shapes", "printf": "cstdio"}
	tests := []struct{ in, want string }{
		{"C.area(p, 1)", "shapes.area(p, 1)"},
		{"C.std.vector[int]", "std.vector[int]"},
		{"C.std.chrono.steady_clock.now()", "std.chrono.steady_clock.now()"},
		{"C.printf(\"%d\", C.area(x))", "cstdio.printf(\"%d\", shapes.area(x))"},
		{"C.unknown(1)", "unknown(1)"},
		{"C.int(goI)", "C.int(goI)"},
		{"C.puts(C.cstr(s))", "puts(C.cstr(s))"},
		{"C.As[unsafe.Pointer](&gi)", "unsafe.Pointer(&gi)"},
		{"C.As[*C.char](p)", "(*C.char)(p)"},
		{"C.struct_stat", "C.struct_stat"},
		{"abc.C.x", "abc.x"},
		{"xC.area", "xC.area"},
		{"no prefix", "no prefix"},
	}
	for _, tc := range tests {
		if got := userText(tc.in, hdrs); got != tc.want {
			t.Errorf("userText(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestDescribeLit(t *testing.T) {
	tests := []struct{ in, want string }{
		{"5", "untyped int constant"},
		{"(-3)", "untyped int constant"},
		{"0x1F", "untyped int constant"},
		{"2.0", "untyped float constant"},
		{"1e9", "untyped float constant"},
		{`"x"`, "untyped string constant"},
		{"'b'", "untyped rune constant"},
		{"true", "untyped bool constant"},
		{"nullptr", "untyped nil"},
	}
	for _, tc := range tests {
		if got := describeLit(tc.in); got != tc.want {
			t.Errorf("describeLit(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPlainType(t *testing.T) {
	tests := []struct{ in, want string }{
		{"std::vector<long>", "std.vector<long>"},
		{"gocpp_probe_internal_pr::string", "string"},
		{"std::type_identity_t<unsigned char>", "unsigned char"},
		{"gocpp_probe_internal_pr::slice<int>", "[]int"},
		{"const char *", "const char *"},
		{"FILE * (aka '_IO_FILE *')", "FILE *"},
	}
	for _, tc := range tests {
		if got := plainType(tc.in); got != tc.want {
			t.Errorf("plainType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestClosest(t *testing.T) {
	names := []string{"size", "push_back", "operator==", "~vector", "front", "Size"}
	tests := []struct{ name, want string }{
		{"sizee", "size"},
		{"Push_back", "push_back"},
		{"frnt", "front"},
		{"size", "Size"},      // an exact match is not a suggestion; a case difference is one
		{"nothingcloser", ""}, // too far
	}
	for _, tc := range tests {
		if got := closest(names, tc.name); got != tc.want {
			t.Errorf("closest(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
