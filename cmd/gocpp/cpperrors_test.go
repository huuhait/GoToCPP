package main

import (
	"bytes"
	"errors"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppgen"
	"github.com/huuhait/gocpp/internal/load"
)

// The tests of the C++ errors of a package that uses C or C++ (step 4.17).
// The interop error tests in tests/interop check the reports end to end at
// the .gp lines; these check the parsing of both compilers' and the
// linkers' output, -cpperrors=full, the errors that stay internal and the
// hint of gocpp run main.gp.

// The places in the canned outputs: the generated C++, the runtime's
// include folder, a system header and the .gp file.
const (
	tGen = "/work/main.gp.cpp"
	tRT  = "/cache/rt/include"
	tSys = "/usr/include/c++/15/bits/stl_algo.h"
	tGP  = "/src/main.gp"
)

func TestUserCXXErrors(t *testing.T) {
	// The lines of main.gp with C or C++ code; 40 and 41 have none.
	user := make(cppLineSet)
	for _, l := range []int{3, 4, 5, 6, 7, 8, 9, 12, 20, 30} {
		user.add(tGP, l)
	}
	c := &cxxClassifier{generated: tGen, include: tRT, user: user}
	type want struct {
		line int
		msg  string
		from []int
	}
	tests := []struct {
		name   string
		out    string
		want   []want // nil: an internal error
		others int    // the errors outside the user's code that are left out
	}{
		{
			name: "gcc: at a .gp line, with ‘’ quotes and colors",
			out: tGP + ": In function 'void gpkg_main::main()':\n" +
				"\x1b[01m\x1b[K" + tGP + ":7:34:\x1b[m\x1b[K \x1b[01;31m\x1b[Kerror: \x1b[m\x1b[Kcannot convert ‘gp::string’ to ‘const char*’\n" +
				"    7 |         cstdio.printf(gs(\"%d\\n\"), 1)\n" +
				"      |                                  |\n" +
				"In file included from /usr/include/c++/15/cstdio:47,\n" +
				"                 from " + tRT + "/gocpp/ptr.hpp:89,\n" +
				"                 from " + tGen + ":2:\n" +
				"/usr/include/stdio.h:366:43: note: initializing argument 1 of 'int printf(const char*, ...)'\n",
			want: []want{{7, "cannot convert 'gp::string' to 'const char*'" + cstrHint, nil}},
		},
		{
			name: "clang: at a .gp line, with a note in the runtime",
			out: tGP + ":7:14: error: no viable conversion from 'gp::string' to 'const char *'\n" +
				"    7 |     ::printf(gpkg_main::gs(\"%d\\n\"_gs), 1);\n" +
				tRT + "/gocpp/string.hpp:77:5: note: candidate function\n" +
				"/usr/include/stdio.h:366:43: note: passing argument to parameter '__format' here\n" +
				"1 error generated.\n",
			want: []want{{7, "no viable conversion from 'gp::string' to 'const char *'" + cstrHint, nil}},
		},
		{
			name: "gcc: a gocpp string for a char* parameter (C.cstr gives a const char*)",
			out:  tGP + ":7:14: error: cannot convert 'gp::string' to 'char*'\n",
			want: []want{{7, "cannot convert 'gp::string' to 'char*'" + charPtrHint, nil}},
		},
		{
			name: "clang: a gocpp string for a char* parameter",
			out:  tGP + ":7:14: error: no viable conversion from 'gp::string' to 'char *'\n1 error generated.\n",
			want: []want{{7, "no viable conversion from 'gp::string' to 'char *'" + charPtrHint, nil}},
		},
		{
			name: "gcc: deep in a system header, the context kept for the next errors",
			out: "In file included from /usr/include/c++/15/algorithm:63,\n" +
				"                 from " + tGP + ":3:\n" +
				tSys + ": In instantiation of 'constexpr void std::__insertion_sort(...) [with _RandomAccessIterator = int]':\n" +
				tSys + ":1817:25:   required from 'constexpr void std::__final_insertion_sort(...)'\n" +
				tSys + ":4841:18:   required from 'constexpr void std::sort(_RAIter, _RAIter) [with _RAIter = int]'\n" +
				tGP + ":6:52:   required from here\n" +
				"    6 |         std.sort(C.int(1), C.int(2))\n" +
				tSys + ":1780:17: error: no type named 'value_type' in 'struct std::iterator_traits<int>'\n" +
				"In file included from " + tRT + "/gocpp/rt.hpp:13,\n" +
				"                 from " + tGen + ":2:\n" +
				tSys + ":1780:25: error: invalid type argument of unary '*' (have 'int')\n",
			want: []want{{6, "no type named 'value_type' in 'struct std::iterator_traits<int>'", nil}},
		},
		{
			name: "clang: deep in a system header, follow-ons without notes dropped",
			out: "In file included from " + tGP + ":3:\n" +
				"In file included from /usr/include/c++/15/algorithm:63:\n" +
				tSys + ":343:64: error: no type named 'value_type' in 'std::iterator_traits<int>'\n" +
				tSys + ":1593:12: note: in instantiation of function template specialization 'std::__make_heap<int>' requested here\n" +
				tGP + ":6:48: note: in instantiation of function template specialization 'std::sort<int>' requested here\n" +
				"In file included from " + tGP + ":3:\n" +
				tSys + ":345:64: error: no type named 'difference_type' in 'std::iterator_traits<int>'\n" +
				"2 errors generated.\n",
			want: []want{{6, "no type named 'value_type' in 'std::iterator_traits<int>'", nil}},
		},
		{
			name: "gcc: static_assert of gocpp/interop.hpp, then a .gp line of its own",
			out: "In file included from " + tRT + "/gocpp/rt.hpp:30,\n" +
				"                 from " + tGen + ":2:\n" +
				tRT + "/gocpp/interop.hpp: In instantiation of 'constexpr bool gp::cgo::case_eq(const A&, const B&) [with A = point; B = point]':\n" +
				tGP + ":12:25:   required from here\n" +
				tRT + "/gocpp/interop.hpp:515:14: error: static assertion failed: gocpp: a switch on a C/C++ value needs a number\n" +
				tRT + "/gocpp/interop.hpp:515:14: note: 'std::is_scalar_v<point>' evaluates to false\n" +
				tRT + "/gocpp/interop.hpp:517:16: error: no match for 'operator==' (operand types are 'const point' and 'const point')\n" +
				tGP + ": In function 'void gpkg_main::main()':\n" +
				tGP + ":8:3: error: 'nope' was not declared in this scope\n",
			want: []want{{8, "'nope' was not declared in this scope", nil}, {12, "a switch on a C/C++ value needs a number", nil}},
		},
		{
			name: "clang: static_assert of gocpp/interop.hpp, with its condition",
			out: "In file included from " + tGen + ":2:\n" +
				tRT + "/gocpp/interop.hpp:515:9: error: static assertion failed due to requirement 'std::is_scalar_v<point>': gocpp: a switch on a C/C++ value needs a number\n" +
				tGP + ":8:18: note: in instantiation of function template specialization 'gp::cgo::case_eq<point, point>' requested here\n" +
				tRT + "/gocpp/interop.hpp:517:16: error: invalid operands to binary expression ('const point' and 'const point')\n",
			want: []want{{8, "a switch on a C/C++ value needs a number", nil}},
		},
		{
			name: "gcc: in a macro of a user header, at the macro's use",
			out: "In file included from " + tGP + ":3:\n" +
				tGP + ": In function 'void gpkg_main::main()':\n" +
				"/src/./mac.h:3:19: error: base operand of '->' is not a pointer\n" +
				"    3 | #define PX(p) ((p)->x)\n" +
				tGP + ":7:30: note: in expansion of macro 'PX'\n",
			want: []want{{7, "base operand of '->' is not a pointer", nil}},
		},
		{
			name: "clang: in a macro of a user header, at the macro's use",
			out: tGP + ":7:23: error: member reference type 'int' is not a pointer\n" +
				"/src/./mac.h:3:21: note: expanded from macro 'PX'\n",
			want: []want{{7, "member reference type 'int' is not a pointer", nil}},
		},
		{
			name: "two .gp frames: the inner one, and the outer one as from",
			out: tSys + ":10:3: error: bad\n" +
				tGP + ":20:5: note: in instantiation of function template specialization 'f<int>' requested here\n" +
				tGP + ":30:5: note: in instantiation of function template specialization 'g<int>' requested here\n",
			want: []want{{20, "bad", []int{30}}},
		},
		{
			name: "a user header that doesn't compile, at its import line",
			out: "In file included from " + tGP + ":5:\n" +
				"/src/bad.h:2:9: error: expected expression\n",
			want: []want{{5, "expected expression", nil}},
		},
		{
			name: "one error per .gp line, sorted by line",
			out: tGP + ":9:1: error: second\n" +
				tGP + ":4:1: error: first\n" +
				tGP + ":9:7: error: again on line 9\n",
			want: []want{{4, "first", nil}, {9, "second", nil}},
		},
		{
			name: "internal: in the generated lines",
			out:  tGen + ":1:12: error: 'gp_x' was not declared in this scope\n" + tGP + ":3:1: error: later\n",
		},
		{
			name: "internal: in the runtime outside gocpp/interop.hpp",
			out: tRT + "/gocpp/slice.hpp:40:5: error: no member named 'x'\n" +
				tGP + ":8:18: note: in instantiation of function template specialization 'gp::slice<int>::f' requested here\n",
		},
		{
			name: "internal: a system header instantiated by the runtime",
			out: tSys + ":40:5: error: bad\n" +
				tRT + "/gocpp/print.hpp:12:3: note: in instantiation of function template specialization 'std::sort<int>' requested here\n" +
				tGP + ":8:18: note: in instantiation of function template specialization 'gp::print<int>' requested here\n",
		},
		{
			name: "internal: an error without a place",
			out:  "cc1plus: error: unrecognized command-line option '-fbad'\n",
		},
		{
			name: "a header error after the first error is a follow-on, not an error at the import",
			out:  tGP + ":3:1: error: x\nIn file included from " + tGP + ":5:\n/src/bad.h:2:9: error: expected expression\n",
			want: []want{{3, "x", nil}},
		},
		{
			name:   "later errors in gocpp's code are left out and counted",
			out:    tGP + ":3:1: error: x\n" + tGP + ":40:1: error: y\n" + tGen + ":1:1: error: z\n" + tRT + "/gocpp/slice.hpp:4:1: error: w\n",
			want:   []want{{3, "x", nil}},
			others: 3,
		},
		{
			name: "internal, gcc: at a .gp line without C or C++ code",
			out: tGP + ": In function 'void gpkg_main::main()':\n" +
				tGP + ":40:20: error: cannot convert 'gp::slice<int>' to 'gp::slice<long int>' in initialization\n",
		},
		{
			name: "internal, clang: at a .gp line without C or C++ code",
			out:  tGP + ":40:20: error: no viable conversion from 'slice<int>' to 'slice<int64_t>'\n1 error generated.\n",
		},
		{
			name: "internal: a template instantiated by a .gp line without C or C++ code",
			out: tSys + ":10:3: error: bad\n" +
				tGP + ":41:5: note: in instantiation of function template specialization 'std::sort<int>' requested here\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs, others, ok := c.userCXXErrors([]byte(tt.out))
			if tt.want == nil {
				if ok {
					t.Fatalf("userCXXErrors = %+v, want an internal error", errs)
				}
				return
			}
			if !ok {
				t.Fatal("userCXXErrors says internal error, want user errors")
			}
			if others != tt.others {
				t.Errorf("userCXXErrors leaves out %d errors, want %d", others, tt.others)
			}
			// printCppErrors sorts; here the sort is done by hand.
			slices.SortStableFunc(errs, func(a, b cppError) int { return a.at.line - b.at.line })
			var got []want
			for _, e := range errs {
				if e.at.file != tGP {
					t.Errorf("error %q at %s, want %s", e.msg, e.at.file, tGP)
				}
				w := want{line: e.at.line, msg: e.msg}
				for _, f := range e.from {
					w.from = append(w.from, f.line)
				}
				got = append(got, w)
			}
			if !slices.EqualFunc(got, tt.want, func(a, b want) bool {
				return a.line == b.line && a.msg == b.msg && slices.Equal(a.from, b.from)
			}) {
				t.Errorf("userCXXErrors =\n\t%+v\nwant\n\t%+v", got, tt.want)
			}
		})
	}
}

func TestParseUndefined(t *testing.T) {
	tests := []struct {
		name string
		out  string
		want []undefinedRef
	}{
		{
			name: "GNU ld",
			out: "/usr/bin/ld: /work/main.gp.o: in function `gpkg_main::main()':\n" +
				"/src/main.gp:6:(.text+0xa): undefined reference to `gocpp_missing_fn(int)'\n" +
				"/src/main.gp:9:(.text+0x1e): undefined reference to `zlibVersion'\n" +
				"collect2: error: ld returned 1 exit status\n",
			want: []undefinedRef{{"gocpp_missing_fn(int)", cxxLoc{"/src/main.gp", 6}}, {"zlibVersion", cxxLoc{"/src/main.gp", 9}}},
		},
		{
			name: "GNU ld without debug information, and curly quotes",
			out:  "/usr/bin/ld: main.o:(.text+0x5): undefined reference to ‘sqlite3_open’\n",
			want: []undefinedRef{{"sqlite3_open", cxxLoc{}}},
		},
		{
			name: "lld",
			out: "ld.lld: error: undefined symbol: gocpp_missing_fn(int)\n" +
				">>> referenced by main.gp:6 (/src/main.gp:6)\n" +
				">>>               /work/main.gp.o:(gpkg_main::main())\n" +
				">>> referenced by main.gp:8 (/src/main.gp:8)\n" +
				">>>               /work/main.gp.o:(gpkg_main::main())\n" +
				"clang++: error: linker command failed with exit code 1 (use -v to see invocation)\n",
			want: []undefinedRef{{"gocpp_missing_fn(int)", cxxLoc{"/src/main.gp", 6}}, {"gocpp_missing_fn(int)", cxxLoc{"/src/main.gp", 8}}},
		},
		{
			name: "mold",
			out:  "mold: error: undefined symbol: f\n>>> referenced by main.gp:3\n>>>               /work/main.gp.o:(gpkg_main::main())\n",
			want: []undefinedRef{{"f", cxxLoc{"main.gp", 3}}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseUndefined([]byte(tt.out)); !slices.Equal(got, tt.want) {
				t.Errorf("parseUndefined =\n\t%+v\nwant\n\t%+v", got, tt.want)
			}
		})
	}
	for _, sym := range []string{"gp::rt_init()", "vtable for gpkg_main::T", "gpkg_main::f(int)"} {
		if !gocppSymbolRx.MatchString(sym) {
			t.Errorf("%q is not taken for gocpp's symbol", sym)
		}
	}
	for _, sym := range []string{"sqlite3_open", "mygp::x", "my_gpkg_x"} {
		if gocppSymbolRx.MatchString(sym) {
			t.Errorf("%q is taken for gocpp's symbol", sym)
		}
	}
}

// cppErrSrc is a program with a C++ error at line 9: a gocpp string where
// printf wants a const char*.
const cppErrSrc = `package main

import "<cstdio>"

func format() string { return "%d\n" }

func main() {
	cstdio.puts("ok")
	cstdio.printf(format(), 1)
}
`

func TestCppErrors(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		inDir(t, map[string]string{"main.gp": cppErrSrc})

		// The short report: exit 1, the error at the .gp line with the
		// source line, and no work folder kept.
		r := gocppRun("", false, "build", ".")
		if !regexp.MustCompile(`^# command-line-arguments\n\./main\.gp:9: C\+\+: (cannot convert|no viable conversion from) 'gp::string' to 'const char ?\*' \(a gocpp string is no C string: pass C\.cstr\(s\), or C\.CString\(s\) for a copy that C keeps\)\n\tcstdio\.printf\(format\(\), 1\)\n$`).MatchString(r.stderr) ||
			r.code != 1 {
			t.Errorf("gocpp build = %+v, want exit 1 and the error at ./main.gp:9", r)
		}

		// -cpperrors=full: the compiler's whole output, with its notes.
		r = gocppRun("", false, "run", "-cpperrors=full", ".")
		if r.code != 1 || !strings.HasPrefix(r.stderr, "# command-line-arguments\n") || !strings.Contains(r.stderr, "./main.gp:9:") ||
			!strings.Contains(r.stderr, "error: ") || !strings.Contains(r.stderr, "note: ") || strings.Contains(r.stderr, "internal error") {
			t.Errorf("gocpp run -cpperrors=full = %+v, want exit 1 and the compiler's output", r)
		}
		if r := gocppRun("", false, "build", "-cpperrors=long", "."); r.code != 2 || !strings.Contains(r.stderr, "want short or full") {
			t.Errorf("gocpp build -cpperrors=long = %+v, want exit 2 and a flag error", r)
		}

		// An error at a .gp line without C or C++ code is gocpp's: nearly
		// every generated line has the #line of its .gp line. The fault: the
		// call of fmt.Println on line 10 names a function that doesn't exist.
		writeFiles(t, ".", map[string]string{"main.gp": "package main\n\nimport (\n\t\"fmt\"\n\n\t\"<cstdio>\"\n)\n\nfunc main() {\n\tfmt.Println(\"pure go\")\n\tcstdio.puts(\"ok\")\n}\n"})
		testHookCPP = inAllFiles(func(cpp []byte) []byte {
			return bytes.Replace(cpp, []byte("gp::fmt::Println(\"pure go"), []byte("gp::fmt::Printlnx(\"pure go"), 1)
		})
		t.Cleanup(func() { testHookCPP = nil })
		r = gocppRun("", false, "build", ".")
		m := regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr)
		if m != nil {
			os.RemoveAll(m[1])
		}
		if r.code != 2 || !strings.Contains(r.stderr, "internal error: generated C++ did not compile\n") ||
			!regexp.MustCompile(`(?m)^\./main\.gp:10:\d+: error: .*Printlnx`).MatchString(r.stderr) || m == nil {
			t.Errorf("gocpp build with a fault on a pure Go line = %+v, want exit 2 and the internal error at ./main.gp:10", r)
		}

		// An error in the generated lines without a .gp line stays an
		// internal error, also in a package that uses C++.
		testHookCPP = inMainUnit(func(cpp []byte) []byte {
			return append([]byte("static int gp_test_broken = gp_test_no_such_name;\n"), cpp...)
		})
		t.Cleanup(func() { testHookCPP = nil })
		writeFiles(t, ".", map[string]string{"main.gp": strings.Replace(cppErrSrc, "cstdio.printf(format(), 1)", "cstdio.puts(\"x\")", 1)})
		r = gocppRun("", false, "build", ".")
		m = regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr)
		if m != nil {
			os.RemoveAll(m[1])
		}
		if r.code != 2 || !strings.Contains(r.stderr, "internal error: generated C++ did not compile\n") || !strings.Contains(r.stderr, "gp_test_no_such_name") || m == nil {
			t.Errorf("gocpp build of broken generated C++ = %+v, want exit 2 and the internal error", r)
		}
		// -cpperrors=full shows the whole output in an internal error too.
		r = gocppRun("", false, "build", "-cpperrors=full", ".")
		if m := regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr); m != nil {
			os.RemoveAll(m[1])
		}
		if r.code != 2 || !strings.Contains(r.stderr, "internal error: generated C++ did not compile\n") || !strings.Contains(r.stderr, "error: ") || !strings.Contains(r.stderr, "This is a bug in gocpp") {
			t.Errorf("gocpp build -cpperrors=full of broken generated C++ = %+v, want exit 2 and the internal error", r)
		}
	})
}

// TestCppLinkErrors checks the link errors of functions that a header
// declares and nothing defines.
func TestCppLinkErrors(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		files := map[string]string{
			"main.gp":  "package main\n\nimport \"./answer.h\"\n\nfunc main() {\n\tn := int(answer.c_answer())\n\tprintln(n)\n}\n",
			"answer.h": "#ifdef __cplusplus\nextern \"C\"\n#endif\nint c_answer(void);\n",
		}
		inDir(t, files)
		// No .c file: the hint names the directives.
		r := gocppRun("", false, "build", ".")
		if r.code != 1 || r.stderr != "# command-line-arguments\n./main.gp:6: C++: undefined reference to 'c_answer'\n\tn := int(answer.c_answer())\n"+
			"\t(the headers declare it, but the program links no definition: add the library with //gocpp:link or //gocpp:pkg-config, or the C or C++ file that defines it to the package)\n" {
			t.Errorf("gocpp build without the C file = %+v, want exit 1 and the link error at ./main.gp:6", r)
		}

		// With the .c file, gocpp run main.gp leaves it out, like go run
		// main.go: the hint says so.
		writeFiles(t, ".", map[string]string{"answer.c": "int c_answer(void) { return 42; }\n"})
		r = gocppRun("", false, "run", "main.gp")
		if r.code != 1 || !strings.Contains(r.stderr, "./main.gp:6: C++: undefined reference to 'c_answer'\n") ||
			!strings.Contains(r.stderr, "\t(the folder has C or C++ files, and gocpp run of named .gp files leaves them out: gocpp run . builds them too)\n") ||
			strings.Contains(r.stderr, "internal error") {
			t.Errorf("gocpp run main.gp = %+v, want exit 1 and the hint for gocpp run .", r)
		}
		if r := gocppRun("", true, "run", "."); r.code != 0 || r.stdout != "42\n" {
			t.Errorf("gocpp run . = %+v, want exit 0 and 42", r)
		}
	})
}

// TestExternCClashHints checks the hint for a .h header that gocpp wraps in
// extern "C" while a C++ file of the package may include it as C++ (step
// 4.16): it is printed only when an undefined symbol appears in that file.
func TestExternCClashHints(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"helpers.cpp":   "#include \"helpers.h\"\nint add(int a, int b) { return a + b; }\n",
		"impl.cpp":      "  # include \"sub/other.h\"\nint other_f() { return 1; }\n",
		"unrelated.cpp": "#include <vector>\n// #include \"helpers.h\" is only a comment\n",
		"deep.cpp":      "#include \"lib.h\"\nint add() { return 0; }\n", // sub/lib.h is not lib.h
		"lib.cpp":       "extern \"C\" {\n#include \"lib.h\"\n}\nint have(int x) { return x; }\n",
		"lib2.cpp":      "#include \"lib2.h\"\nint have2(int x) { return x; }\n",
		"ns.cpp":        "#include \"ns.h\"\nnamespace n { int f(int x) { return x; } }\n",
	})
	hint := func(h, f string) string {
		return `(` + h + ` parses as C, so gocpp includes it in extern "C" for the generated code; if ` + f +
			` includes it outside extern "C", what ` + f + ` defines has C++ linkage and the calls don't find it: add //gocpp:no-extern-c ` + h +
			`, or #ifdef __cplusplus extern "C" guards to the header)`
	}
	all := []string{"helpers.cpp", "impl.cpp", "unrelated.cpp", "deep.cpp", "lib.cpp", "lib2.cpp", "ns.cpp"}
	for _, tt := range []struct {
		name      string
		externC   map[string]bool
		files     []string
		undefined []string
		want      []string
	}{
		{"the pair: add is defined in helpers.cpp", map[string]bool{`"./helpers.h"`: true}, all, []string{"add"}, []string{hint("./helpers.h", "helpers.cpp")}},
		{"a mangled symbol", map[string]bool{`"./helpers.h"`: true}, all, []string{"add(int, int)"}, []string{hint("./helpers.h", "helpers.cpp")}},
		{"a qualified symbol", map[string]bool{`"./ns.h"`: true}, all, []string{"n::f(int)"}, []string{hint("./ns.h", "ns.cpp")}},
		{"a quoted include in a sub-folder", map[string]bool{`"./sub/other.h"`: true}, all, []string{"other_f"}, []string{hint("./sub/other.h", "impl.cpp")}},
		// The reviewer's repro: lib.cpp wraps its include and defines only
		// have; the forgotten function is missing, which lib.cpp doesn't
		// mention.
		{"include inside extern C, other function missing", map[string]bool{`"./lib.h"`: true}, all, []string{"missing"}, nil},
		{"include inside extern C, own function undefined", map[string]bool{`"./lib.h"`: true}, all, []string{"have"}, nil},
		{"the undefined symbol is not in the file", map[string]bool{`"./lib2.h"`: true}, all, []string{"missing"}, nil},
		{"a header of another folder is not the include", map[string]bool{`"./sub/lib.h"`: true}, []string{"deep.cpp"}, []string{"add"}, nil},
		{"a comment is no include", map[string]bool{`"./helpers.h"`: true}, []string{"unrelated.cpp"}, []string{"add"}, nil},
		{"not wrapped", map[string]bool{`"./helpers.h"`: false, "<lua.h>": true}, all, []string{"add"}, nil},
		{"no C++ file", map[string]bool{`"./helpers.h"`: true}, nil, []string{"add"}, nil},
		{"no undefined symbol", map[string]bool{`"./helpers.h"`: true}, all, nil, nil},
	} {
		if got := externCClashHints(dir, tt.externC, tt.files, tt.undefined); !slices.Equal(got, tt.want) {
			t.Errorf("%s:\n got %q\nwant %q", tt.name, got, tt.want)
		}
	}
	// A file with the stem of the header counts without an #include (it may
	// reach the header through another one).
	writeFiles(t, dir, map[string]string{"stem.cpp": "int stem_f() { return 1; }\n"})
	if got := externCClashHints(dir, map[string]bool{`"./stem.h"`: true}, []string{"stem.cpp"}, []string{"stem_f"}); len(got) != 1 {
		t.Errorf("same stem: got %q, want one hint", got)
	}
}

// TestEmitFailedPrefix checks that an internal error of the lowering whose
// message starts with "internal error: " gets the prefix once.
func TestEmitFailedPrefix(t *testing.T) {
	dir := t.TempDir()
	pkg := &check.Package{Load: &load.Package{ImportPath: "command-line-arguments"}}
	for _, tt := range []struct {
		err  error
		want string
	}{
		{&cppgen.Error{Pos: token.Position{Filename: filepath.Join(dir, "a.gp"), Line: 3, Column: 2}, Msg: "internal error: the address of x, which is not boxed"},
			"./a.gp:3:2: internal error: the address of x, which is not boxed\n"},
		{&cppgen.Error{Pos: token.Position{Filename: filepath.Join(dir, "a.gp"), Line: 3, Column: 2}, Msg: "not supported yet: x"},
			"./a.gp:3:2: internal error: not supported yet: x\n"},
		{&cppgen.Error{Msg: "internal error: no position"}, "\ninternal error: no position\n"},
		{errors.New("other"), "\ninternal error: other\n"},
	} {
		var stderr bytes.Buffer
		d := &driver{stderr: &stderr, check: &checkCommand{dir: dir}}
		if code := d.emitFailed(&buildSet{main: pkg, pkgs: []*check.Package{pkg}}, tt.err); code != exitInternal || !strings.Contains(stderr.String(), tt.want) ||
			strings.Count(stderr.String(), "internal error") != 1 {
			t.Errorf("emitFailed(%v) = %d, %q; want %d and %q", tt.err, code, stderr.String(), exitInternal, tt.want)
		}
	}
}
