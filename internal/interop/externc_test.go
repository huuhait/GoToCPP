package interop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests of the extern "C" includes in the probes and of the C header
// check (step 4.14).

func TestSplitExternC(t *testing.T) {
	if got := ExternC(`"./x.h"`); got != `extern "C" "./x.h"` {
		t.Errorf("ExternC = %q", got)
	}
	for _, c := range []struct {
		in, out string
		marked  bool
	}{
		{`extern "C" "./x.h"`, `"./x.h"`, true},
		{`extern "C" <lua.h>`, "<lua.h>", true},
		{"<lua.h>", "<lua.h>", false},
	} {
		if out, marked := SplitExternC(c.in); out != c.out || marked != c.marked {
			t.Errorf("SplitExternC(%q) = %q, %v", c.in, out, marked)
		}
	}
	if err := checkArgs(nil, []string{ExternC("<lua.h>"), ExternC(`"x.h"`)}); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{ExternC("lua.h"), `extern "C"`, ExternC(ExternC("<x.h>"))} {
		if err := checkArgs(nil, []string{bad}); err == nil {
			t.Errorf("checkArgs(%q): no error", bad)
		}
	}
}

// TestMakeProbeExternC checks the lines of includes in extern "C" blocks
// and which include each line belongs to.
func TestMakeProbeExternC(t *testing.T) {
	pf := makeProbe([]string{"<vector>", ExternC(`"./c.h"`), "<cstdio>", ExternC("<lua.h>")}, [][]string{{"std"}}, false)
	want := `#include <vector>
; template <class> struct gp_inc_0; using gp_inc_0_c = ::gp_inc_0<int>;
extern "C" {
#include "./c.h"
}
; template <class> struct gp_inc_1; using gp_inc_1_c = ::gp_inc_1<int>;
#include <cstdio>
; template <class> struct gp_inc_2; using gp_inc_2_c = ::gp_inc_2<int>;
extern "C" {
#include <lua.h>
}
; template <class> struct gp_inc_3; using gp_inc_3_c = ::gp_inc_3<int>;
#if defined(std)
#error gp_kind_macro
#else
namespace gp_kn_0 = ::std;
#endif
namespace gp_probe_end = ::gp_probe_no_such_namespace;
namespace gp_probe_eof {}
`
	if got := string(pf.src); got != want {
		t.Errorf("makeProbe:\n%s\nwant:\n%s", got, want)
	}
	if pf.firstBlock != 13 || pf.canary() != 18 || pf.helpers(13) {
		t.Errorf("firstBlock %d, canary %d", pf.firstBlock, pf.canary())
	}
	type place struct {
		index        int
		check, isInc bool
	}
	wantPlaces := map[int]place{
		1: {0, false, true}, 2: {0, true, true},
		3: {1, false, true}, 4: {1, false, true}, 5: {1, true, true}, 6: {1, true, true},
		7: {2, false, true}, 8: {2, true, true},
		9: {3, false, true}, 10: {3, false, true}, 11: {3, true, true}, 12: {3, true, true},
	}
	for n := 0; n <= pf.canary()+1; n++ {
		index, check, ok := pf.include(n)
		if got := (place{index, check, ok}); got != wantPlaces[n] {
			t.Errorf("include(%d) = %+v, want %+v", n, got, wantPlaces[n])
		}
	}
}

// externCHeaders are the headers of the tests of extern "C" includes.
var externCHeaders = map[string]string{
	// A C header without a guard. As C++ without extern "C", it declares
	// puts with C++ linkage, and then <stdio.h> declares it with C
	// linkage: an error. Inside extern "C" { } it is fine.
	"c.h": "int puts(const char *);\n#include <stdio.h>\n#include <math.h>\nint c_add(int, int);\n#define C_ANSWER 42\n",
	// Headers that end inside a block or a declaration.
	"open.h":  "extern \"C\" {\nint f(void);\n",
	"ns.h":    "namespace a {\n",
	"class.h": "struct S {\n",
	"decl.h":  "int g(\n",
	"bad.h":   "int x = ;\n",
	// C: with a guard, without one, and C++.
	"guarded.h": "#ifdef __cplusplus\nextern \"C\" {\n#endif\nint g_add(int, int);\n#ifdef __cplusplus\n}\n#endif\n",
	"cpp.h":     "#pragma once\nclass Widget { int x; };\n",
	"flag.h":    "#ifndef NEED_FLAG\n#error needs -DNEED_FLAG\n#endif\nint flagged(void);\n",
	"word.h":    "int __cplusplus_like(void); /* no guard: another word */\n",
	"comment.h": "/* not for __cplusplus */\nint commented(void);\n",
	"inc.h":     "#include \"inner.h\"\n",
	"inner.h":   "int inner(void);\n",
}

// TestProbeKindsExternC checks that the kind probe and the macro probe
// include a header marked with ExternC inside extern "C" { }: a C header
// that is an error in C++ without it works with it, and the scope checks
// still find a header that leaves a block open, for the right include.
func TestProbeKindsExternC(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		writeFiles(t, dir, externCHeaders)
		ctx := context.Background()
		paths := []string{"c_add", "puts", "C_ANSWER", "std", "std::vector"}

		res, err := ProbeKinds(ctx, cxx, dir, nil, []string{"<vector>", ExternC(`"./c.h"`)}, paths)
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"c_add", Value}, {"puts", Value}, {"C_ANSWER", Macro}, {"std", Namespace}, {"std::vector", Template}})
		ns, err := ProbeNamespaces(ctx, cxx, dir, nil, []string{"<vector>", ExternC(`"./c.h"`)}, []string{"std", "c_add"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, ns, []kindCase{{"std", Namespace}, {"c_add", NotNamespace}})
		ms, err := ProbeMacros(ctx, cxx, dir, nil, []string{"<vector>", ExternC(`"./c.h"`)})
		if err != nil {
			t.Fatal(err)
		}
		if ms.Kind("C_ANSWER") != ObjectLike {
			t.Errorf("C_ANSWER is %v", ms.Kind("C_ANSWER"))
		}

		// Without the block, the header is an error in C++.
		res, err = ProbeKinds(ctx, cxx, dir, nil, []string{"<vector>", `"./c.h"`}, paths)
		if err != nil {
			t.Fatal(err)
		}
		if len(res.IncludeErrors) != 1 || res.IncludeErrors[0].Index != 1 || res.IncludeErrors[0].Kind != HeaderErrors {
			t.Errorf("without extern \"C\": include errors %v, want one for ./c.h", res.IncludeErrors)
		}
		if _, err := ProbeMacros(ctx, cxx, dir, nil, []string{"<vector>", `"./c.h"`}); err != nil {
			t.Errorf("macro probe without extern \"C\": %v (it only preprocesses)", err)
		}

		// Include errors of marked and unmarked includes, each after a
		// marked include that works, and before one that works.
		for _, c := range []struct {
			header string
			kind   IncludeErrorKind
		}{
			{"open.h", HeaderUnclosed},
			{"ns.h", HeaderUnclosed},
			{"class.h", HeaderUnclosed},
			{"decl.h", HeaderUnclosed},
			{"bad.h", HeaderErrors},
			{"missing.h", HeaderNotFound},
		} {
			for _, marked := range []bool{true, false} {
				inc := `"./` + c.header + `"`
				if marked {
					inc = ExternC(inc)
				}
				includes := []string{ExternC(`"./c.h"`), inc, "<cstdio>"}
				res, err := ProbeKinds(ctx, cxx, dir, nil, includes, []string{"std"})
				if err != nil {
					t.Fatal(err)
				}
				// The first error is the header's own. A block that it leaves
				// open can break the includes after it too.
				if len(res.IncludeErrors) == 0 {
					t.Errorf("%s (extern \"C\" %v): no include errors", c.header, marked)
					continue
				}
				ie := res.IncludeErrors[0]
				if ie.Index != 1 || ie.Include != inc || ie.Kind != c.kind {
					t.Errorf("%s (extern \"C\" %v): include error %+v, want index 1, %s", c.header, marked, ie, c.kind)
				}
				if c.kind == HeaderErrors && ie.File != filepath.Join(dir, c.header) {
					t.Errorf("%s (extern \"C\" %v): the error is in %q", c.header, marked, ie.File)
				}
			}
		}
	})
}

// testCC returns the C compiler that goes with the C++ compiler cxx.
func testCC(t *testing.T, cxx build.CXX) build.CXX {
	t.Helper()
	t.Setenv("GOCPP_CC", "")
	cc, err := build.FindCC(context.Background(), cxx)
	if err != nil {
		t.Fatal(err)
	}
	return cc
}

func TestCheckCHeader(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		writeFiles(t, dir, externCHeaders)
		ctx := context.Background()
		for _, c := range []struct {
			include string
			flags   []string
			want    CHeader
		}{
			{`"./c.h"`, nil, CHeader{C: true}},
			{`"c.h"`, nil, CHeader{C: true}},
			{`"./guarded.h"`, nil, CHeader{C: true, Guard: true}},
			{`"./cpp.h"`, nil, CHeader{}},
			{`"./open.h"`, nil, CHeader{}},
			{`"./missing.h"`, nil, CHeader{}},
			{`"./flag.h"`, nil, CHeader{}},
			{`"./flag.h"`, []string{"-DNEED_FLAG"}, CHeader{C: true}},
			{`"./word.h"`, nil, CHeader{C: true}},
			// The limits of the guard test: a word in a comment counts.
			{`"./comment.h"`, nil, CHeader{C: true, Guard: true}},
		} {
			got, err := CheckCHeader(ctx, cc, dir, c.flags, c.include)
			if err != nil {
				t.Fatalf("%s: %v", c.include, err)
			}
			if got != c.want {
				t.Errorf("CheckCHeader(%s, %q) = %+v, want %+v", c.include, c.flags, got, c.want)
			}
		}
		for _, bad := range []string{"<stdio.h>", ExternC(`"./c.h"`), `"./c.h`} {
			if _, err := CheckCHeader(ctx, cc, dir, nil, bad); err == nil {
				t.Errorf("CheckCHeader(%s): no error", bad)
			}
		}
		if _, err := CheckCHeader(ctx, build.CXX{Path: "x"}, dir, nil, `"./c.h"`); err == nil || !strings.Contains(err.Error(), "unknown family") {
			t.Errorf("a compiler of no known family: %v", err)
		}
	})
}

// TestCheckCHeaderCache checks when the C header check runs the compiler
// again: for other flags, another folder, and a changed header or a
// header that it includes; not for the same header again.
func TestCheckCHeaderCache(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := testCC(t, cxx)
		dir := t.TempDir()
		writeFiles(t, dir, externCHeaders)
		writeFiles(t, dir, map[string]string{"vec.h": "#include <vector>\n#include \"inner.h\"\n"})
		c := newCache()
		// Every answer here has a list of the files it depends on, also
		// when the header isn't C: none of them expires.
		c.errorTTL = time.Nanosecond
		ctx := context.Background()
		var runs int64
		check := func(why, include string, flags []string, wantRun bool, want CHeader) {
			t.Helper()
			got, err := c.checkCHeader(ctx, cc, dir, flags, include)
			if err != nil {
				t.Fatalf("%s: %v", why, err)
			}
			if got != want {
				t.Errorf("%s: %+v, want %+v", why, got, want)
			}
			if wantRun {
				runs++
			}
			if n := c.cRuns.Load(); n != runs {
				t.Errorf("%s: %d compiler runs so far, want %d", why, n, runs)
				runs = n
			}
		}
		check("first", `"./inc.h"`, nil, true, CHeader{C: true})
		check("again", `"./inc.h"`, nil, false, CHeader{C: true})
		check("other flags", `"./inc.h"`, []string{"-DX"}, true, CHeader{C: true})
		check("other flags, again", `"./inc.h"`, []string{"-DX"}, false, CHeader{C: true})
		check("a C++ header", `"./cpp.h"`, nil, true, CHeader{})
		check("a C++ header, again", `"./cpp.h"`, nil, false, CHeader{})
		// A C++ standard header isn't found in C mode.
		check("<vector>", `"./vec.h"`, nil, true, CHeader{})
		check("<vector>, again", `"./vec.h"`, nil, false, CHeader{})
		writeFiles(t, dir, map[string]string{"inner.h": "int inner2(void);\n"})
		check("<vector>, inner.h changed", `"./vec.h"`, nil, true, CHeader{})
		writeFiles(t, dir, map[string]string{"inner.h": "int inner(void);\n"})

		writeFiles(t, dir, map[string]string{"inner.h": "class Inner {};\n"})
		check("inner.h changed", `"./inc.h"`, nil, true, CHeader{})
		check("inner.h changed, again", `"./inc.h"`, nil, false, CHeader{})
		writeFiles(t, dir, map[string]string{"inc.h": "#ifdef __cplusplus\n#endif\n"})
		check("inc.h changed", `"./inc.h"`, nil, true, CHeader{C: true, Guard: true})
		check("inc.h changed, again", `"./inc.h"`, nil, false, CHeader{C: true, Guard: true})

		// A header that isn't found, included by a header in a sub-folder,
		// or found through -I: checked again when it appears there.
		writeFiles(t, dir, map[string]string{"sub/s.h": "#include \"gen.h\"\n", "c2.h": "#include <genlib.h>\n"})
		check("sub/s.h, gen.h missing", `"./sub/s.h"`, nil, true, CHeader{})
		check("sub/s.h, again", `"./sub/s.h"`, nil, false, CHeader{})
		writeFiles(t, dir, map[string]string{"sub/gen.h": "int gen(void);\n"})
		check("sub/gen.h appeared", `"./sub/s.h"`, nil, true, CHeader{C: true})
		check("sub/gen.h appeared, again", `"./sub/s.h"`, nil, false, CHeader{C: true})
		inc := []string{"-I" + filepath.Join(dir, "inc")}
		check("c2.h, genlib.h missing", `"./c2.h"`, inc, true, CHeader{})
		check("c2.h, again", `"./c2.h"`, inc, false, CHeader{})
		writeFiles(t, dir, map[string]string{"inc/genlib.h": "int genlib(void);\n"})
		check("inc/genlib.h appeared", `"./c2.h"`, inc, true, CHeader{C: true})
		check("inc/genlib.h appeared, again", `"./c2.h"`, inc, false, CHeader{C: true})

		// A header that isn't found: checked again when it appears.
		check("missing", `"./new.h"`, nil, true, CHeader{})
		check("missing, again", `"./new.h"`, nil, false, CHeader{})
		writeFiles(t, dir, map[string]string{"new.h": "int fresh(void);\n"})
		check("new.h appeared", `"./new.h"`, nil, true, CHeader{C: true})
		check("new.h, again", `"./new.h"`, nil, false, CHeader{C: true})

		// A header that changed in the last seconds is not cached.
		path := filepath.Join(dir, "new.h")
		if err := os.WriteFile(path, []byte("int fresher(void);\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		soon := time.Now().Add(time.Hour)
		if err := os.Chtimes(path, soon, soon); err != nil {
			t.Fatal(err)
		}
		check("new.h just changed", `"./new.h"`, nil, true, CHeader{C: true})
		check("new.h just changed, not cached", `"./new.h"`, nil, true, CHeader{C: true})

		// Another folder is another header.
		other := t.TempDir()
		writeFiles(t, other, map[string]string{"inc.h": "class X {};\n"})
		got, err := c.checkCHeader(ctx, cc, other, nil, `"./inc.h"`)
		if err != nil || got != (CHeader{}) {
			t.Errorf("another folder: %+v, %v", got, err)
		}
	})
}

// TestCCompilerFallback checks the C compiler of the C header check without
// a C compiler next to the C++ compiler: the C++ compiler itself in C mode,
// unless GOCPP_CC names a compiler, or the C++ compiler doesn't compile C.
func TestCCompilerFallback(t *testing.T) {
	old := findCC
	findCC = func(context.Context, build.CXX) (build.CXX, error) { return build.CXX{}, errors.New("no C compiler") }
	t.Cleanup(func() { findCC = old })
	reset := func() {
		ccLookups.Lock()
		ccLookups.m = nil
		ccLookups.Unlock()
	}
	ctx := context.Background()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		reset()
		t.Setenv("GOCPP_CC", "")
		cc, err := CCompiler(ctx, cxx)
		if err != nil || cc.Path != cxx.Path {
			t.Fatalf("CCompiler = %+v, %v; want the C++ compiler", cc, err)
		}
		dir := t.TempDir()
		writeFiles(t, dir, externCHeaders)
		for include, want := range map[string]CHeader{`"./c.h"`: {C: true}, `"./cpp.h"`: {}, `"./guarded.h"`: {C: true, Guard: true}} {
			if got, err := CheckCHeaderFor(ctx, cxx, dir, nil, include); err != nil || got != want {
				t.Errorf("CheckCHeaderFor(%s) with %s = %+v, %v; want %+v", include, cxx.Name(), got, err, want)
			}
		}
		// GOCPP_CC names a compiler that isn't there: no fallback.
		reset()
		t.Setenv("GOCPP_CC", "no-such-cc")
		if _, err := CCompiler(ctx, cxx); err == nil || err.Error() != "no C compiler" {
			t.Errorf("with GOCPP_CC: %v", err)
		}
	})
	// A C++ compiler without the C front end.
	reset()
	t.Setenv("GOCPP_CC", "")
	cxx := fakeCompiler(t, build.GCC, `echo "fake++: fatal error: cannot execute 'cc1'" >&2; exit 1`)
	if _, err := CheckCHeaderFor(ctx, cxx, t.TempDir(), nil, `"./c.h"`); err == nil ||
		!strings.Contains(err.Error(), "no C compiler; the C++ compiler doesn't compile C either: fake++ -x c failed") || !strings.Contains(err.Error(), "cannot execute 'cc1'") {
		t.Errorf("without a C front end: %v", err)
	}
}
