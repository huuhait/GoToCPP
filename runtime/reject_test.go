package rtfiles_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// rejectCases are C++ uses of the runtime's types that must not compile with gocpp's flags for
// generated code (build.Flags and build.GeneratedFlags, without -Werror): each would be wrong
// at run time, and a requires-expression can't test some of them (g++ reports an ambiguous
// conversion in delete p as a hard error even there). The line marked "// bad" is where the
// compiler must report an error.
var rejectCases = []struct {
	name, src string
	want      string // also in the output, when not ""
}{
	// delete p: two conversions to object pointers (ptr.hpp), so neither is used. Otherwise
	// it compiles and frees the object that the gocpp pointer still owns.
	{"delete", "void f(gp::ptr<int64_t> p) {\n\tdelete p; // bad\n}\n", ""},
	{"delete-void", "void f(gp::ptr<void> p) {\n\tdelete p; // bad\n}\n", ""},
	// p[i]: raw pointer arithmetic without a bounds check.
	{"index", "int64_t f(gp::ptr<int64_t> p) {\n\treturn p[1]; // bad\n}\n", ""},
	// if (p): Go has no pointer conditions.
	{"condition", "bool f(gp::ptr<int64_t> p) {\n\tif (p) { // bad\n\t\treturn true;\n\t}\n\treturn false;\n}\n", ""},
	// A class object through a C function's `...`: the callee would read the address of a
	// copy as its pointer. clang rejects it by default; g++ with -Werror=conditionally-supported.
	{"varargs-ptr", "extern \"C\" void var(int, ...);\nvoid f(gp::ptr<int64_t> p) {\n\tvar(1, p); // bad\n}\n", ""},
	{"varargs-cstr", "extern \"C\" void var(int, ...);\nvoid f(const gp::string& s) {\n\tvar(1, gp::cstr(s)); // bad\n}\n", ""},
	// gp::range over a C array of C arrays: a clear message instead of an error deep inside.
	{"range-grid", "void f() {\n\tint g[2][3] = {};\n\tfor (auto&& [i, v] : gp::range(g)) { // bad\n\t\t(void)i;\n\t\t(void)v;\n\t}\n}\n",
		"gp::range: the elements are C arrays"},
	// gp::range over a C number or a value without begin() and end(): a message for users.
	{"range-number", "void f() {\n\tint n = 3;\n\tfor (auto&& i : gp::range_keys(n)) { // bad\n\t\t(void)i;\n\t}\n}\n",
		"gp::range: range over a C number"},
	{"range-object", "struct S {};\nvoid f() {\n\tS s;\n\tfor (auto&& [i, v] : gp::range(s)) { // bad\n\t\t(void)i;\n\t\t(void)v;\n\t}\n}\n",
		"gp::range: range over a C/C++ value without begin() and end()"},
	// if (isdigit(c)), if (p): Go has only bool conditions (gp::cgo::cond, Phase 04 §2.5).
	{"cond-int", "bool f(int c) {\n\treturn gp::cgo::cond(c); // bad\n}\n", "non-boolean condition"},
	{"cond-pointer", "bool f(const char* p) {\n\treturn gp::cgo::cond(p); // bad\n}\n", "non-boolean condition"},
	// std::optional converts to bool only explicitly: not a Go condition either.
	{"cond-optional", "#include <optional>\nbool f(std::optional<int> o) {\n\treturn gp::cgo::cond(o); // bad\n}\n", "non-boolean condition"},
	// string(p) of a C string: cgo's form is C.GoString(p) (Phase 04 step 4.9).
	{"string-of-pointer", "gp::string f(const char* p) {\n\treturn gp::cgo::as<gp::string>(p); // bad\n}\n", "use C.GoString(p)"},
	{"string-of-array", "gp::string f() {\n\tchar a[4] = \"abc\";\n\treturn gp::cgo::as<gp::string>(a); // bad\n}\n", "use C.GoString(p)"},
	{"string-of-class", "struct V {};\ngp::string f() {\n\treturn gp::cgo::as<gp::string>(V{}); // bad\n}\n", "string(x) needs a std::string"},
	// No implicit conversion makes a gp::string from a C string: a C char* has no owner.
	{"string-from-pointer", "void g(const gp::string&);\nvoid f(const char* p) {\n\tg(p); // bad\n}\n", ""},
	// A gocpp string still converts only to std::string_view: f(std::string) + f(const char*)
	// doesn't take it.
	{"string-to-overloads", "#include <string>\nvoid h(std::string);\nvoid h(const char*);\nvoid f(const gp::string& s) {\n\th(s); // bad\n}\n", ""},
}

// rejectControl is the same code written the way the emitter writes it: it must compile, so
// that the cases above fail for their own reason.
const rejectControl = `extern "C" void var(int, ...);
void f(gp::ptr<int64_t> p, gp::ptr<void> u, const gp::string& s) {
	var(1, p.gp_get(), gp::cstr(s).gp_ptr(), u.gp_get());
	if (p != nullptr) {
		(void)*p;
	}
	int g[2][3] = {};
	for (auto&& i : gp::range_keys(g)) {
		(void)g[i];
	}
	const char* c = "x";
	if (gp::cgo::cond(c != nullptr) && gp::cgo::cond(g[0][0] == 0) && gp::cgo::to_bool(c == nullptr)) {
		(void)c;
	}
	std::vector<bool> bs{true};
	if (gp::cgo::cond(bs[0])) { // a class that converts to bool implicitly
		(void)bs;
	}
	(void)gp::cgo::as<gp::string>(std::string("x"));
	(void)(s == "x");
}
`

// TestRejected compiles each of rejectCases with each compiler and checks that it fails at
// the marked line, and that rejectControl compiles.
func TestRejected(t *testing.T) {
	ccs := compilers(t)
	rt := extractRuntime(t)
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			dir := t.TempDir()
			compile := func(name, body string) (string, int) {
				t.Helper()
				src := filepath.Join(dir, name+".cpp")
				if err := os.WriteFile(src, []byte("#include <gocpp/rt.hpp>\n"+body), 0o644); err != nil {
					t.Fatal(err)
				}
				args := slices.Concat(build.Flags(c.cc, false), build.GeneratedFlags(c.cc),
					[]string{"-I", filepath.Join(rt, "include"), "-fsyntax-only", src})
				out, _, code, err := command(dir, build.CompilerEnv(), true, c.cc.Path, args...)
				if err != nil {
					t.Fatal(err)
				}
				return string(out), code
			}
			if out, code := compile("control", rejectControl); code != 0 || out != "" {
				t.Fatalf("the control doesn't compile cleanly: exit code %d\n%s", code, out)
			}
			for _, tc := range rejectCases {
				out, code := compile(tc.name, tc.src)
				// The marked line, counted with the #include line before the case.
				line := 2 + strings.Count(tc.src[:strings.Index(tc.src, "// bad")], "\n")
				at := fmt.Sprintf("%s.cpp:%d:", tc.name, line)
				want := "an error at " + at
				if tc.want != "" {
					want += " with " + tc.want
				}
				if code == 0 || !strings.Contains(out, at) || !strings.Contains(out, "error") ||
					!strings.Contains(out, tc.want) {
					t.Errorf("%s: exit code %d, want %s; output:\n%s", tc.name, code, want, out)
				}
			}
		})
	}
}
