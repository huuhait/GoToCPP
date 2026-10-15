package main

import (
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The tests of the extern "C" rule in gocpp build, run and emit (Phase 04
// §2.9, step 4.14): C headers whose functions a .c file of the package
// defines, called from .gp code.

// externCFiles is a package whose main.gp calls C functions through local
// headers and a "system" header (found with -I), all defined in calc.c:
//   - calc.h is C without a __cplusplus guard: wrapped by default;
//   - guarded.h has the guard: not wrapped (it wraps itself);
//   - legacy.hpp is C with a C++ name: wrapped only by //gocpp:extern-c;
//   - sys/fakesys.h is C, found as <fakesys.h>: wrapped only by
//     //gocpp:extern-c;
//   - conflict.h declares puts before <stdio.h> does: as C++ that is an
//     error (C++ linkage, then C linkage), so the probes too must see it
//     inside extern "C" { }, or the check reports the header;
//   - cpp.h is C++ (a template and overloads), which extern "C" { } would
//     break: not wrapped.
var externCFiles = map[string]string{
	"main.gp": `//gocpp:cflags -Isys
//gocpp:extern-c ./legacy.hpp
//gocpp:extern-c <fakesys.h>
package main

import (
	"./calc.h"
	"./conflict.h"
	"./cpp.h"
	"./guarded.h"
	"./legacy.hpp"
	"<fakesys.h>"
)

func main() {
	println(int(calc.calc_add(2, 3)), int(guarded.g_mul(2, 3)), int(legacy.old_sub(9, 2)), int(fakesys.fakesys_answer()))
	println(int(conflict.conflict_ok()), int(cpp.twice(C.int(21))), int(cpp.over(C.int(1))))
}
`,
	"calc.h":        "/* C, no guard. */\n#include <stddef.h>\nint calc_add(int a, int b);\n",
	"guarded.h":     "#pragma once\n#ifdef __cplusplus\nextern \"C\" {\n#endif\nint g_mul(int, int);\n#ifdef __cplusplus\n}\n#endif\n",
	"legacy.hpp":    "int old_sub(int, int);\n",
	"conflict.h":    "int puts(const char *);\n#include <stdio.h>\nint conflict_ok(void);\n",
	"cpp.h":         "#pragma once\ntemplate <class T> T twice(T x) { return x + x; }\ninline int over(int x) { return x + 1; }\ninline double over(double x) { return x; }\n",
	"sys/fakesys.h": "int fakesys_answer(void);\n",
	"calc.c": `#include "calc.h"
#include "guarded.h"
#include "legacy.hpp"
#include "conflict.h"
#include <fakesys.h>

int calc_add(int a, int b) { return a + b; }
int g_mul(int a, int b) { return a * b; }
int old_sub(int a, int b) { return a - b; }
int conflict_ok(void) { return puts("") >= 0; }
int fakesys_answer(void) { return 42; }
`,
}

func TestExternC(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04") // directives
		inDir(t, externCFiles)
		if r := gocppRun("", false, "run", "."); r != (result{0, "\n", "5 6 7 42\n1 42 2\n"}) {
			t.Fatalf("gocpp run . = %+v", r)
		}
		r := gocppRun("", false, "emit", ".")
		if r.code != 0 {
			t.Fatalf("gocpp emit . = %+v", r)
		}
		// Each #include at the import's line, and the braces of the wrapped
		// ones too.
		lines := mappedLines(r.stdout)
		for _, inc := range []struct {
			line int
			text string
			wrap bool
		}{
			{7, `"./calc.h"`, true},
			{8, `"./conflict.h"`, true},
			{9, `"./cpp.h"`, false},
			{10, `"./guarded.h"`, false},
			{11, `"./legacy.hpp"`, true},
			{12, `<fakesys.h>`, true},
		} {
			i := slices.IndexFunc(lines, func(l mappedLine) bool { return l.text == "#include "+inc.text })
			if i < 1 || i+1 >= len(lines) || lines[i].line != inc.line {
				t.Errorf("#include %s is not at main.gp:%d in:\n%s", inc.text, inc.line, r.stdout)
				continue
			}
			before, after := lines[i-1], lines[i+1]
			wrapped := before.text == `extern "C" {` && after.text == "}"
			if wrapped != inc.wrap || wrapped && (before.line != inc.line || after.line != inc.line) {
				t.Errorf("#include %s: the lines around it are %+v and %+v, want extern \"C\" %v at main.gp:%d", inc.text, before, after, inc.wrap, inc.line)
			}
		}
	})
}

// A mappedLine is a line of generated C++ (not a #line directive) and the
// line of the .gp file that it is mapped to, or 0.
type mappedLine struct {
	text string
	line int
}

// mappedLines returns the lines of the C++ cpp, with the .gp lines that
// the #line directives map them to, as the compiler counts them.
func mappedLines(cpp string) []mappedLine {
	var lines []mappedLine
	next := 0 // the .gp line of the next line, or 0
	for l := range strings.SplitSeq(cpp, "\n") {
		if rest, ok := strings.CutPrefix(l, "#line "); ok {
			n, file, _ := strings.Cut(rest, " ")
			next, _ = strconv.Atoi(n)
			if !strings.HasSuffix(file, `.gp"`) {
				next = 0
			}
			continue
		}
		lines = append(lines, mappedLine{l, next})
		if next != 0 {
			next++
		}
	}
	return lines
}

// TestExternCOff checks that the wrapping is what makes the C functions
// link: with //gocpp:no-extern-c, or without //gocpp:extern-c for a header
// that the default rules don't wrap, the generated C++ calls the
// C++-mangled names, which calc.c doesn't define.
func TestExternCOff(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04") // directives
		for _, tc := range []struct {
			what, from, to, missing string
		}{
			{"no-extern-c", "//gocpp:cflags -Isys\n", "//gocpp:cflags -Isys\n//gocpp:no-extern-c ./calc.h\n", `calc_add\(int, int\)`},
			{"no extern-c for legacy.hpp", "//gocpp:extern-c ./legacy.hpp\n", "", `old_sub\(int, int\)`},
			{"no extern-c for <fakesys.h>", "//gocpp:extern-c <fakesys.h>\n", "", `fakesys_answer\(\)`},
		} {
			files := maps.Clone(externCFiles)
			files["main.gp"] = strings.Replace(files["main.gp"], tc.from, tc.to, 1)
			inDir(t, files)
			r := gocppRun("", false, "run", ".")
			if r.code == 0 || !regexp.MustCompile(`undefined reference to .*`+tc.missing).MatchString(r.stderr) {
				t.Errorf("%s: gocpp run . = %+v, want a link error for the C++ name %s", tc.what, r, tc.missing)
			}
		}
	})
}

// TestExternCErrors checks the errors of the extern-c and no-extern-c
// directives in gocpp build: at the directives, before anything is built.
func TestExternCErrors(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("GOCPP_GATE_PHASE", "04") // directives
	inDir(t, map[string]string{
		"main.gp": "//gocpp:extern-c ./calc.h\n//gocpp:no-extern-c calc.h\npackage main\n\nimport \"./calc.h\"\n\nfunc main() {}\n",
		"b.gp":    "//gocpp:extern-c ./typo.h\npackage main\n",
		"calc.h":  "int calc_add(int, int);\n",
	})
	want := `# command-line-arguments
./b.gp:1:1: //gocpp:extern-c names header ./typo.h, which no file of the package imports
./main.gp:2:1: header calc.h has both //gocpp:extern-c and //gocpp:no-extern-c (the other is at main.gp:1)
`
	if r := gocppRun("", false, "build", "."); r.code != 1 || r.stderr != want {
		t.Errorf("gocpp build . = %+v, want exit 1 and\n%s", r, want)
	}
}
