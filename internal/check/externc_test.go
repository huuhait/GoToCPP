package check

import (
	"maps"
	"slices"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// TestExternC checks which header imports the build wraps in extern "C"
// (Package.ExternC), that the namespace probe gets them so marked, and the
// errors of the extern-c and no-extern-c directives.
func TestExternC(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"main.gp": `//gocpp:extern-c <zlib.h>
//gocpp:no-extern-c ./plain.h
//gocpp:extern-c ./odd.hpp
//gocpp:extern-c ./nothere.h
//gocpp:extern-c <stdio.h>
//gocpp:no-extern-c <stdio.h>
//gocpp:extern-c ./sub/../guarded.h
package main

import (
	"<zlib.h>"
	"<stdio.h>"
	"./plain.h"
	"./c.h"
	"./guarded.h"
	"./cpp.h"
	"./odd.hpp"
	"./native.hpp"
	"./notfound.h"
)

func main() {}
`,
		// The same header with another spelling, and a directive in another
		// file.
		"b.gp":       "//gocpp:no-extern-c ./b.h\npackage main\n\nimport (\n\t\"c.h\"\n\t\"./b.h\"\n)\n",
		"plain.h":    "int plain(void);\n",
		"c.h":        "#pragma once\nint c_add(int, int);\n",
		"b.h":        "int b(void);\n",
		"guarded.h":  "#ifdef __cplusplus\nextern \"C\" {\n#endif\nint g(void);\n#ifdef __cplusplus\n}\n#endif\n",
		"cpp.h":      "#pragma once\nnamespace cpp { int f(); }\n",
		"odd.hpp":    "int odd(void);\n",
		"native.hpp": "int native(void);\n",
	}
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		rec := new(probeRecorder)
		cfg := probeConfig(cxx, 4)
		cfg.Probe = rec.probe
		prog, root := loadTree(t, maps.Clone(files))
		r := Check(prog, cfg)
		want := `# example.com/m
./main.gp:4:1: //gocpp:extern-c names header ./nothere.h, which no file of the package imports
./main.gp:6:1: header <stdio.h> has both //gocpp:extern-c and //gocpp:no-extern-c (the other is at main.gp:5)
./main.gp:19:2: cannot find header "./notfound.h"
`
		if got := errorText(t, prog, r, root, true); got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
		p := mainPkg(t, prog, r)
		wantWrap := map[string]bool{
			"<zlib.h>":       true,  // extern-c
			`"./plain.h"`:    false, // no-extern-c, though it is C
			`"./c.h"`:        true,  // C, no guard
			`"c.h"`:          true,
			`"./b.h"`:        false, // no-extern-c in b.gp
			`"./guarded.h"`:  true,  // extern-c, though it has a guard
			`"./cpp.h"`:      false, // C++
			`"./odd.hpp"`:    true,  // extern-c
			`"./notfound.h"`: false, // not found: the probe says so
		}
		// <stdio.h> (both directives) and ./native.hpp get the default
		// rules: not wrapped, and not decided by the C check either.
		for k, v := range wantWrap {
			if p.ExternC[k] != v {
				t.Errorf("ExternC[%s] = %v, want %v", k, p.ExternC[k], v)
			}
		}
		if p.ExternC["<stdio.h>"] || p.ExternC[`"./native.hpp"`] {
			t.Errorf("ExternC = %v", p.ExternC)
		}
		var calls [][]string
		for _, c := range rec.calls {
			calls = append(calls, c.includes)
		}
		wantCalls := [][]string{
			{interop.ExternC("<zlib.h>"), "<stdio.h>", `"./plain.h"`, interop.ExternC(`"./c.h"`), interop.ExternC(`"./guarded.h"`),
				`"./cpp.h"`, interop.ExternC(`"./odd.hpp"`), `"./native.hpp"`, `"./notfound.h"`},
			{interop.ExternC(`"c.h"`), `"./b.h"`},
		}
		slices.SortFunc(calls, func(a, b []string) int { return len(b) - len(a) })
		if !slices.EqualFunc(calls, wantCalls, slices.Equal) {
			t.Errorf("probe calls with includes\n%q\nwant\n%q", calls, wantCalls)
		}
	})
}

// TestExternCNoCompiler checks that a package whose C++ compiler isn't
// found gets no extern "C" decisions and no more errors than the probe's.
func TestExternCNoCompiler(t *testing.T) {
	t.Setenv("GOCPP_CXX", "no-such-c++")
	got := probeErrors(t, Config{Phase: 4}, map[string]string{
		"main.gp": "package main\n\nimport \"./c.h\"\n\nfunc main() {}\n",
		"c.h":     "int c(void);\n",
	})
	want := "# example.com/m\n./main.gp:3:8: cannot check the header imports: GOCPP_CXX: the C++ compiler \"no-such-c++\" was not found: exec: \"no-such-c++\": executable file not found in $PATH\n"
	if got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestExternCNoCCompiler checks a C++ compiler without a C compiler next
// to it: an error at the import of each local .h header that the C header
// check would decide about.
func TestExternCNoCCompiler(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Setenv("GOCPP_CC", "no-such-cc")
		got := probeErrors(t, probeConfig(cxx, 4), map[string]string{
			"main.gp": "package main\n\nimport (\n\t\"./c.h\"\n\t\"./d.hpp\"\n)\n\nfunc main() {}\n",
			"c.h":     "int c(void);\n",
			"d.hpp":   "int d(void);\n",
		})
		want := "# example.com/m\n./main.gp:4:2: cannot check whether header \"./c.h\" is a C header: GOCPP_CC: the C compiler \"no-such-cc\" was not found: exec: \"no-such-cc\": executable file not found in $PATH\n"
		if got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
	})
}
