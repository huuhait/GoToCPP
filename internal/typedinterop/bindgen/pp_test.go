package bindgen

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseMarker(t *testing.T) {
	cases := []struct {
		line  string
		file  string
		flags [5]bool
		ok    bool
	}{
		{`# 1 "/p/main.cpp"`, "/p/main.cpp", [5]bool{}, true},
		{`# 1 "/usr/include/stdio.h" 1 3 4`, "/usr/include/stdio.h", [5]bool{false, true, false, true, true}, true},
		{`# 40 "/p/a.h" 2`, "/p/a.h", [5]bool{false, false, true}, true},
		{`# 1 "<built-in>" 1`, "<built-in>", [5]bool{false, true}, true},
		{`# 7 "a \"quoted\" name.h"`, `a "quoted" name.h`, [5]bool{}, true},
		{`# pragma foo`, "", [5]bool{}, false},
		{`# 12`, "", [5]bool{}, false},
		{`# 12 file.h`, "", [5]bool{}, false},
		{`# 12 "unterminated`, "", [5]bool{}, false},
	}
	for _, c := range cases {
		file, flags, ok := parseMarker(c.line)
		if ok != c.ok || file != c.file || flags != c.flags {
			t.Errorf("parseMarker(%q) = %q, %v, %v; want %q, %v, %v", c.line, file, flags, ok, c.file, c.flags, c.ok)
		}
	}
}

func TestParseDefine(t *testing.T) {
	cases := []struct {
		in   string
		want macro
		ok   bool
	}{
		{"A 1", macro{Name: "A", Body: "1"}, true},
		{"A", macro{Name: "A"}, true},
		{"A   (1 + 2)  ", macro{Name: "A", Body: "(1 + 2)"}, true},
		{"F(x, y) ((x) + (y))", macro{Name: "F", Params: "(x, y)", FuncLike: true, Body: "((x) + (y))"}, true},
		{"G() 1", macro{Name: "G", Params: "()", FuncLike: true, Body: "1"}, true},
		{"H (x) x", macro{Name: "H", Body: "(x) x"}, true}, // a space before ( makes it object-like
		{"V(...) f(__VA_ARGS__)", macro{Name: "V", Params: "(...)", FuncLike: true, Body: "f(__VA_ARGS__)"}, true},
		{"(1) 2", macro{}, false},
		{"F(x", macro{}, false},
	}
	for _, c := range cases {
		m := parseDefine(c.in)
		if (m != nil) != c.ok {
			t.Errorf("parseDefine(%q) = %v", c.in, m)
			continue
		}
		if m != nil && *m != c.want {
			t.Errorf("parseDefine(%q) = %+v, want %+v", c.in, *m, c.want)
		}
	}
}

// ppText is the output of "clang -E -dD" for a main file that includes an
// umbrella header (which includes a sibling, a header in a folder below, and
// a system header), with two macros that are undefined or redefined.
const ppText = `# 1 "/p/main.cpp"
# 1 "<built-in>" 1
#define __GNUC__ 4
# 1 "<built-in>" 2
# 1 "<command line>" 1
#define FROM_COMMAND_LINE 1
# 1 "<command line>" 2
# 1 "/p/main.cpp" 2
# 1 "/p/inc/umb.h" 1
#define UMB 1
# 1 "/p/inc/a.h" 1
#define A 1
#define GONE 2
# 2 "/p/inc/umb.h" 2
# 1 "/p/inc/sub/b.h" 1
#define B 1
# 3 "/p/inc/umb.h" 2
# 1 "/usr/include/stdio.h" 1 3
#define EOF (-1)
# 4 "/p/inc/umb.h" 2
#undef GONE
#define F(x) x
#define EMPTY
#define UMB 5
# 2 "/p/main.cpp" 2
`

func macroNames(ms []*macro) []string {
	var out []string
	for _, m := range ms {
		out = append(out, m.Name)
	}
	return out
}

func TestParsePPScopes(t *testing.T) {
	cases := []struct {
		scope  Scope
		roots  []string
		files  []string
		macros []string
	}{
		{ScopeHeader, nil, []string{"/p/inc/umb.h"}, []string{"F", "EMPTY", "UMB"}},
		{ScopeDir, nil, []string{"/p/inc/umb.h", "/p/inc/a.h"}, []string{"A", "F", "EMPTY", "UMB"}},
		{ScopeTree, nil, []string{"/p/inc/umb.h", "/p/inc/a.h", "/p/inc/sub/b.h"}, []string{"A", "B", "F", "EMPTY", "UMB"}},
		{ScopeAuto, nil, []string{"/p/inc/umb.h", "/p/inc/a.h"}, []string{"A", "F", "EMPTY", "UMB"}},
		// a header that lies in a folder of the include path is taken alone
		{ScopeAuto, []string{"/p/inc"}, []string{"/p/inc/umb.h"}, []string{"F", "EMPTY", "UMB"}},
		{ScopeDir, []string{"/p/inc"}, []string{"/p/inc/umb.h", "/p/inc/a.h"}, []string{"A", "F", "EMPTY", "UMB"}},
	}
	for _, c := range cases {
		res, err := parsePP(strings.NewReader(ppText), "/p/main.cpp", c.scope, "/p", c.roots)
		if err != nil {
			t.Fatal(err)
		}
		if res.Header != "/p/inc/umb.h" {
			t.Errorf("%s %v: header = %q", c.scope, c.roots, res.Header)
		}
		if !reflect.DeepEqual(res.Files, c.files) {
			t.Errorf("%s %v: files = %v, want %v", c.scope, c.roots, res.Files, c.files)
		}
		// UMB was defined again after the sibling's macros, so it comes last
		got := macroNames(res.Macros)
		if !reflect.DeepEqual(got, c.macros) {
			t.Errorf("%s %v: macros = %v, want %v", c.scope, c.roots, got, c.macros)
		}
		for _, m := range res.Macros {
			if m.Name == "UMB" && m.Body != "5" {
				t.Errorf("UMB = %q: the last definition counts", m.Body)
			}
			if m.Name == "F" && !m.FuncLike {
				t.Error("F is function-like")
			}
		}
	}
}

func TestParsePPRelativeNames(t *testing.T) {
	text := "# 1 \"main.cpp\"\n# 1 \"sub/x.h\" 1\n#define X 1\n# 2 \"main.cpp\" 2\n"
	res, err := parsePP(strings.NewReader(text), "/w/main.cpp", ScopeAuto, "/w", nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.Header != "/w/sub/x.h" || len(res.Macros) != 1 {
		t.Errorf("header %q, macros %v", res.Header, macroNames(res.Macros))
	}
}

func TestSameHeaderAndBits(t *testing.T) {
	for _, c := range []struct {
		file, parent string
		want         bool
	}{
		{"/glibc/stdlib.h", "/gcc/stdlib.h", true},
		{"/gcc/cstdlib", "/gcc/stdlib.h", true},
		{"/glibc/stdlib.h", "/gcc/cstdlib", true},
		{"/gcc/cmath", "/gcc/math.h", true},
		{"/gcc/x.h", "/gcc/y.h", false},
		{"/gcc/cstdio", "/gcc/stdlib.h", false},
		{"/gcc/c", "/gcc/.h", false},
		{"/gcc/cfoo.h", "/gcc/foo.h", false}, // only extension-less wrappers
	} {
		if got := sameHeader(c.file, c.parent); got != c.want {
			t.Errorf("sameHeader(%q, %q) = %v", c.file, c.parent, got)
		}
	}
	if !isBits("/usr/include/bits/types.h") || isBits("/usr/include/bitsy.h") || isBits("/usr/include/types.h") {
		t.Error("isBits")
	}
}

func TestParsePPWrapper(t *testing.T) {
	// <stdlib.h> in C++ is a wrapper that includes <cstdlib>, which includes the C library's file,
	// which includes a part of itself in bits/
	text := `# 1 "/p/main.cpp"
# 1 "/gcc/stdlib.h" 1
#define WRAPPER 1
# 1 "/gcc/cstdlib" 1
# 1 "/glibc/stdlib.h" 1
#define EXIT_FAILURE 1
# 1 "/glibc/bits/waitflags.h" 1
#define WNOHANG 1
#define _PRIVATE_FROM_BITS 1
# 2 "/glibc/stdlib.h" 2
# 1 "/glibc/asm-generic/errno.h" 1
#define EBIG 7
# 3 "/glibc/stdlib.h" 2
# 1 "/glibc/other.h" 1
#define OTHER 1
# 3 "/glibc/stdlib.h" 2
# 2 "/gcc/stdlib.h" 2
`
	res, err := parsePP(strings.NewReader(text), "/p/main.cpp", ScopeAuto, "/p", []string{"/gcc", "/glibc"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/gcc/stdlib.h", "/gcc/cstdlib", "/glibc/stdlib.h", "/glibc/bits/waitflags.h", "/glibc/asm-generic/errno.h"}
	if !reflect.DeepEqual(res.Files, want) {
		t.Errorf("files = %v, want %v", res.Files, want)
	}
	if got := macroNames(res.Macros); !reflect.DeepEqual(got, []string{"WRAPPER", "EXIT_FAILURE", "WNOHANG", "_PRIVATE_FROM_BITS", "EBIG"}) {
		t.Errorf("macros = %v", got)
	}
}
