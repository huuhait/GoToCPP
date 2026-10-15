package interop

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests in this file run the real compilers, g++ and clang++, by the
// rules of the test harness (tests/README.md, "Compilers"): GOCPP_CXX
// tests with that compiler only, a compiler that isn't installed is a
// skip, and a compiler that GOCPP_REQUIRE_CXX names must be tested ("make
// ci" sets g++,clang++), so there a missing one fails.

// compilerMatrix returns the compiler programs to test with: GOCPP_CXX, or
// g++ and clang++.
func compilerMatrix() []string {
	if cxx := os.Getenv("GOCPP_CXX"); cxx != "" {
		return []string{cxx}
	}
	return []string{"g++", "clang++"}
}

// requiredCXX returns the compilers of GOCPP_REQUIRE_CXX, and fails the
// test if one of them is not in the matrix (the harness's checkRequired).
func requiredCXX(tb testing.TB, matrix []string) []string {
	tb.Helper()
	env := os.Getenv("GOCPP_REQUIRE_CXX")
	var required []string
	for want := range strings.SplitSeq(env, ",") {
		if want = strings.TrimSpace(want); want == "" {
			continue
		}
		if !slices.ContainsFunc(matrix, func(prog string) bool { return prog == want || filepath.Base(prog) == want }) {
			if cxx := os.Getenv("GOCPP_CXX"); cxx != "" {
				tb.Fatalf("GOCPP_REQUIRE_CXX=%s needs %s, but GOCPP_CXX=%s tests only that compiler", env, want, cxx)
			}
			tb.Fatalf("GOCPP_REQUIRE_CXX=%s needs %s, which the tests don't use: they use g++ and clang++, or the compiler in GOCPP_CXX", env, want)
		}
		required = append(required, want)
	}
	return required
}

// forEachCompiler runs fn once per compiler of the matrix, as subtests
// named after the compilers.
func forEachCompiler(t *testing.T, fn func(t *testing.T, cxx build.CXX)) {
	t.Helper()
	matrix := compilerMatrix()
	required := requiredCXX(t, matrix)
	for _, prog := range matrix {
		name := filepath.Base(prog)
		t.Run(name, func(t *testing.T) {
			cxx := lookTestCompiler(t, prog, slices.Contains(required, prog) || slices.Contains(required, name))
			fn(t, cxx)
		})
	}
}

// lookTestCompiler returns the compiler prog. One that isn't installed is
// a skip, or a failure if it is required. For g++ and clang++ it also
// checks the family that LookCXX found.
func lookTestCompiler(tb testing.TB, prog string, required bool) build.CXX {
	tb.Helper()
	if _, err := exec.LookPath(prog); err != nil {
		if required {
			tb.Fatalf("GOCPP_REQUIRE_CXX needs %s, but it is not installed: %v", prog, err)
		}
		tb.Skipf("%s is not installed: %v", prog, err)
	}
	cxx, err := build.LookCXX(context.Background(), prog)
	if err != nil {
		tb.Fatal(err)
	}
	want := map[string]build.Family{"g++": build.GCC, "clang++": build.Clang}[filepath.Base(prog)]
	if want != 0 && cxx.Family != want {
		tb.Fatalf("LookCXX(%q).Family = %v, want %v", prog, cxx.Family, want)
	}
	return cxx
}

// writeFiles writes the files (path relative to dir: content) and makes
// them an hour old: the cache doesn't keep results that depend on a file
// that changed in the last seconds (racyWindow).
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
}

// The folder of a .gp file in the tests, with its local headers.
var testHeaders = map[string]string{
	"physics.h": `#pragma once
#include "vec.h"
namespace physics_ns {
inline int step() { return 1; }
}
struct Point { int x, y; };
`,
	"vec.h": `#pragma once
namespace vecns { struct V {}; }
`,
	"native/x.hpp": `#pragma once
namespace native_ns { inline namespace v1 { int f(); } }
namespace alias_ns = native_ns;
inline namespace gp_test_inline { namespace inner_ns { int g(); } }
namespace ñandú { int h(); }
`,
	"macros.h": `#pragma once
#define OPEN_BRACE ns_x {
#define NS_MACRO physics_ns
`,
	// -iquote, unlike -I, doesn't make <vector> find this file.
	"vector": `#error "the local file named vector was included instead of <vector>"
`,
	"bad.h": `#pragma once
struct Bad { int x }
`,
	"bad2.h": `#pragma once
int bad2 = ;
`,
	"inc_bad.h": `#pragma once
#include "bad.h"
`,
	"unclosed.h": `#pragma once
extern "C" {
int unclosed_fn(void);
`,
	// A C header that forgot its closing #ifdef __cplusplus } #endif.
	"mylib.h": `#ifdef __cplusplus
extern "C" {
#endif
int mylib_init(void);
`,
	// The scope check can't see an open extern "C++" block; the end of
	// the file does.
	"opencxx.h": `#pragma once
extern "C++" {
int opencxx(int);
`,
	// A header that ends in the middle of a declaration.
	"unfinished.h": `#pragma once
int unfinished(
`,
	// A header that leaves a class open: the headers after it are inside
	// the class.
	"openclass.h": `#pragma once
struct OpenClass { int a;
`,
	// A missing ; after a struct is harmless here: the ; after the
	// #include ends the declaration.
	"nosemi.h": `#pragma once
struct NoSemi {}
`,
	// An error early in the header, and one at the end of the file, when
	// the template is instantiated: gcc prints the include stack only for
	// the first one.
	"late.h": `#pragma once
int late_a = undeclared_a;
template <class T> int late_f() { return T::nope; }
inline int late_g() { return late_f<int>(); }
`,
	// The candidate amb is ambiguous: its error has notes in this header,
	// so gcc prints the header's include stack for them, and not again for
	// the error at the end of the file.
	"ambig.h": `#pragma once
namespace amb_a { namespace amb {} }
namespace amb_b { namespace amb {} }
using namespace amb_a;
using namespace amb_b;
template <class T> int late_f() { return T::nope; }
inline int late_g() { return late_f<int>(); }
`,
}

// testDir returns a new folder with testHeaders.
func testDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, testHeaders)
	return dir
}

// A kindCase is a candidate and the kind it must get.
type kindCase struct {
	cand string
	want Kind
}

// checkKinds checks that res has the kinds of cases and no include errors.
func checkKinds(t *testing.T, res Result, cases []kindCase) {
	t.Helper()
	if len(res.IncludeErrors) > 0 {
		t.Errorf("include errors: %v", res.IncludeErrors)
	}
	for _, c := range cases {
		if got := res.Kinds[c.cand]; got != c.want {
			t.Errorf("%s: %v, want %v", c.cand, got, c.want)
		}
	}
	if len(res.Kinds) != len(cases) {
		t.Errorf("got %d kinds, want %d: %v", len(res.Kinds), len(cases), res.Kinds)
	}
}

func candsOf(cases []kindCase) []string {
	var cands []string
	for _, c := range cases {
		cands = append(cands, c.cand)
	}
	return cands
}

var probeIncludes = []string{"<vector>", "<filesystem>", "<chrono>", "<cstdio>", "<string>",
	`"./physics.h"`, `"native/x.hpp"`, `"macros.h"`}

var probeCases = []kindCase{
	// A macro that opens a brace comes first: the lines after it still count.
	{"OPEN_BRACE", NotNamespace},
	{"std", Namespace},
	{"chrono", NotNamespace}, // only std::chrono is
	{"stdd", NotNamespace},
	{"fmtt", NotNamespace},
	{"::std::filesystem", Namespace}, // a renamed namespace path
	{"std::chrono", Namespace},       // the leading :: is optional
	{"::std::vector", NotNamespace},  // a class template
	{"::std::string", NotNamespace},  // a typedef
	{"physics_ns", Namespace},        // from "./physics.h" next to the .gp file
	{"vecns", Namespace},             // from vec.h, which physics.h includes
	{"alias_ns", Namespace},          // a namespace alias
	{"::native_ns::v1", Namespace},   // an inline namespace
	{"inner_ns", Namespace},          // found through an inline namespace
	{"Point", NotNamespace},          // a struct
	{"printf", NotNamespace},         // a function
	{"stdin", NotNamespace},          // a macro
	{"NS_MACRO", NotNamespace},       // a macro, also when it names a namespace
	{"class", NotNamespace},          // a C++ keyword
	{"and", NotNamespace},            // an alternative token (&&)
	{"ñandú", Namespace},             // non-ASCII names work
	{"日本", NotNamespace},
}

func TestProbeNamespaces(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := testDir(t)
		c := newCache()
		res, err := c.probe(context.Background(), cxx, dir, probeIncludes, candsOf(probeCases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, probeCases)
		if n := c.runs.Load(); n != 1 {
			t.Errorf("%d compiler runs, want 1", n)
		}

		// The public function, and a quoted include without "./".
		res, err = ProbeNamespaces(context.Background(), cxx, dir, []string{`"physics.h"`}, []string{"physics_ns", "stdd"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"physics_ns", Namespace}, {"stdd", NotNamespace}})

		// A struct without its ; at the end of a header doesn't hide the
		// next line.
		res, err = c.probe(context.Background(), cxx, dir, []string{`"nosemi.h"`, "<vector>"}, []string{"std", "stdd"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"std", Namespace}, {"stdd", NotNamespace}})
	})
}

func TestProbeReservedNames(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		// gp_ns_0 and gp_inc_0 are the probe's own aliases: they must not
		// be found.
		res, err := newCache().probe(context.Background(), cxx, t.TempDir(), []string{"<vector>"}, []string{"gp_ns_0", "std", "gp_inc_0", "::gp_inc_0_c"})
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, []kindCase{{"gp_ns_0", NotNamespace}, {"std", Namespace}, {"gp_inc_0", NotNamespace}, {"::gp_inc_0_c", NotNamespace}})
		// Also with include errors, where the others are Unknown.
		res, err = newCache().probe(context.Background(), cxx, t.TempDir(), []string{"<gocpp_no_such_header.h>"}, []string{"gp_ns_0", "std"})
		if err != nil || len(res.IncludeErrors) != 1 || res.Kinds["gp_ns_0"] != NotNamespace || res.Kinds["std"] != Unknown {
			t.Errorf("reserved name with include errors: %+v, %v", res, err)
		}
	})
}

func TestProbeStdWithCHeaders(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		// g++ declares namespace std even when no header does; clang++
		// doesn't. Both are the compilers' own answers (ProbeNamespaces'
		// doc says so). <math.h> brings libstdc++'s wrapper, which
		// declares std.
		want := map[build.Family]Kind{build.GCC: Namespace, build.Clang: NotNamespace}[cxx.Family]
		for _, c := range []struct {
			include string
			want    Kind
		}{{"<stdio.h>", want}, {"<math.h>", Namespace}} {
			res, err := newCache().probe(context.Background(), cxx, t.TempDir(), []string{c.include}, []string{"std"})
			if err != nil {
				t.Fatal(err)
			}
			checkKinds(t, res, []kindCase{{"std", c.want}})
		}
	})
}

func TestProbeCacheReadsNoUnchangedHeaders(t *testing.T) {
	// Not parallel: it counts the reads of all the caches (fileReads).
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		// A cache hit only stats the headers; it reads one again only
		// when its stat changed.
		dir := testDir(t)
		c := newCache()
		includes := []string{"<vector>", `"./physics.h"`}
		cases := []kindCase{{"physics_ns", Namespace}, {"vecns", Namespace}}
		probe := func(why string, wantReads int64) {
			t.Helper()
			reads := fileReads.Load()
			res, err := c.probe(context.Background(), cxx, dir, includes, candsOf(cases))
			if err != nil {
				t.Fatal(err)
			}
			checkKinds(t, res, cases)
			if n := fileReads.Load() - reads; wantReads >= 0 && n != wantReads {
				t.Errorf("%s: %d files read, want %d", why, n, wantReads)
			}
		}
		probe("first call", -1)
		probe("cache hit", 0)
		old := time.Now().Add(-30 * time.Minute)
		if err := os.Chtimes(filepath.Join(dir, "vec.h"), old, old); err != nil {
			t.Fatal(err)
		}
		probe("vec.h touched", 1)
		probe("vec.h touched, its new stat kept", 0)
		if n := c.runs.Load(); n != 1 {
			t.Errorf("%d compiler runs, want 1", n)
		}
	})
}

func TestProbeHeaderSavedDuringCheck(t *testing.T) {
	// Not parallel: it sets testHookRead. vec.h is saved after the
	// compiler read it, while the cache takes its state (between the stat
	// and the read): the result is not cached, and the next call sees the
	// new namespace.
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		dir := testDir(t)
		c := newCache()
		includes := []string{"<vector>", `"./physics.h"`}
		cases := []kindCase{{"vecns", Namespace}, {"vecns_v2", NotNamespace}}
		rewriteDuringRead(t, filepath.Join(dir, "vec.h"), "#pragma once\nnamespace vecns_v2 {}\n", time.Now().Add(-time.Hour))
		res, err := c.probe(context.Background(), cxx, dir, includes, candsOf(cases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, cases) // what the compiler read
		cases = []kindCase{{"vecns", NotNamespace}, {"vecns_v2", Namespace}}
		res, err = c.probe(context.Background(), cxx, dir, includes, candsOf(cases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, cases)
		if n := c.runs.Load(); n != 2 {
			t.Errorf("%d compiler runs, want 2", n)
		}
	})
}

func TestProbeIncludeErrors(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := testDir(t)
		for _, c := range []struct {
			name     string
			includes []string
			cands    []string           // std and stdd if nil
			want     []includeErrorCase // the include errors that must be there
			// more are includes that may have an error too: clang reports
			// the syntax error above a missing header, gcc stops at the
			// missing header first.
			more []int
		}{
			{name: "missing system header",
				includes: []string{"<vector>", "<gocpp_no_such_header.h>", `"./physics.h"`},
				want:     []includeErrorCase{{1, "", 0, "gocpp_no_such_header.h"}}},
			// physics.h is next to the .gp file, but <> doesn't look
			// there: gcc doesn't find it, clang says to use quotes.
			{name: "local header in <>",
				includes: []string{"<vector>", "<physics.h>"},
				want:     []includeErrorCase{{1, "", 0, "physics.h"}}},
			{name: "missing local header",
				includes: []string{`"./missing.h"`, "<vector>"},
				want:     []includeErrorCase{{0, "", 0, "missing.h"}}},
			{name: "syntax error",
				includes: []string{"<vector>", `"./bad.h"`},
				want:     []includeErrorCase{{1, "bad.h", 2, ""}}},
			{name: "syntax error in a header it includes",
				includes: []string{`"./inc_bad.h"`, "<vector>"},
				want:     []includeErrorCase{{0, "bad.h", 2, ""}}},
			{name: "two headers with errors",
				includes: []string{`"bad.h"`, "<vector>", `"bad2.h"`},
				want:     []includeErrorCase{{0, "bad.h", 2, ""}, {2, "bad2.h", 2, ""}}},
			{name: "missing header after a syntax error",
				includes: []string{`"bad.h"`, "<gocpp_no_such_header.h>"},
				want:     []includeErrorCase{{1, "", 0, "gocpp_no_such_header.h"}},
				more:     []int{0}},
			// The error is at the scope check after the #include, so it has
			// no place in the header.
			{name: "unfinished declaration",
				includes: []string{"<vector>", `"./unfinished.h"`, "<cstdio>"},
				want:     []includeErrorCase{{1, "", 0, "the header ends inside a declaration or a block that it doesn't close: "}}},
			{name: "open extern C",
				includes: []string{"<vector>", `"./unclosed.h"`},
				want:     []includeErrorCase{{1, "", 0, "the header ends inside a declaration or a block that it doesn't close: "}}},
			// Before a C++ header, which must not use up the open brace.
			{name: "open extern C before a C++ header",
				includes: []string{`"./mylib.h"`, "<vector>"},
				want:     []includeErrorCase{{0, "", 0, "the header ends inside a declaration or a block that it doesn't close: "}},
				more:     []int{1}},
			// The place is the brace that is left open.
			{name: "open extern C++",
				includes: []string{"<vector>", `"./opencxx.h"`},
				want:     []includeErrorCase{{1, "opencxx.h", 2, "(to match this '{')"}}},
			// <cstdio> fails inside the class too. Its first error comes from
			// a macro of glibc's <stdio.h> (__BEGIN_DECLS, defined in a header
			// that <vector> brought in); it belongs to <cstdio>, not to
			// <vector> (gcc needs -ftrack-macro-expansion=0 for that).
			{name: "open class",
				includes: []string{"<vector>", `"./openclass.h"`, "<cstdio>"},
				want:     []includeErrorCase{{1, "", 0, "the header ends inside a declaration or a block that it doesn't close: "}},
				more:     []int{2}},
			// The error at the end of the file belongs to late.h, also when
			// gcc prints no include stack for it.
			{name: "late error in a header",
				includes: []string{`"./late.h"`, "<vector>", `"bad.h"`},
				want:     []includeErrorCase{{0, "late.h", 2, ""}, {2, "bad.h", 2, ""}}},
			{name: "late error after notes in the header",
				includes: []string{`"./ambig.h"`},
				cands:    []string{"amb"},
				want:     []includeErrorCase{{0, "ambig.h", 6, ""}}},
		} {
			t.Run(c.name, func(t *testing.T) {
				cands := c.cands
				if cands == nil {
					cands = []string{"std", "stdd"}
				}
				res, err := newCache().probe(context.Background(), cxx, dir, c.includes, cands)
				if err != nil {
					t.Fatal(err)
				}
				for _, cand := range cands {
					if k, ok := res.Kinds[cand]; !ok || k != Unknown {
						t.Errorf("%s: %v (in the map: %v), want Unknown", cand, k, ok)
					}
				}
				checkIncludeErrors(t, res.IncludeErrors, c.includes, dir, c.want, c.more)
			})
		}
	})
}

// An includeErrorCase is an include error that must be there: the index of
// the include, the file of the error relative to the test folder and its
// line, or "" and 0 for an error at the #include line, and a part of its
// message.
type includeErrorCase struct {
	index int
	file  string
	line  int
	msg   string
}

// checkIncludeErrors checks that got has the include errors want, in
// include order, and no others than those and errors for the includes
// more.
func checkIncludeErrors(t *testing.T, got []IncludeError, includes []string, dir string, want []includeErrorCase, more []int) {
	t.Helper()
	for _, w := range want {
		i := slices.IndexFunc(got, func(e IncludeError) bool { return e.Index == w.index })
		if i < 0 {
			t.Errorf("no include error for %s; got %v", includes[w.index], got)
			continue
		}
		e := got[i]
		wantFile := ""
		if w.file != "" {
			wantFile = filepath.Join(dir, w.file)
		}
		file := e.File
		if file != "" {
			file = filepath.Clean(file) // gcc and clang print "dir/./bad.h" for "./bad.h"
		}
		// The cases without a file are a missing header or a header that
		// the scope check caught.
		wantKind := HeaderErrors
		switch {
		case w.file == "" && strings.HasPrefix(w.msg, "the header ends inside"):
			wantKind = HeaderUnclosed
		case w.file == "":
			wantKind = HeaderNotFound
		}
		if e.Include != includes[w.index] || file != wantFile || e.Line != w.line || e.Msg == "" || !strings.Contains(e.Msg, w.msg) || e.Kind != wantKind {
			t.Errorf("include error %+v, want include %s, file %q, line %d, kind %v, a message with %q", e, includes[w.index], wantFile, w.line, wantKind, w.msg)
		}
	}
	for i, e := range got {
		if i > 0 && e.Index <= got[i-1].Index {
			t.Errorf("include errors not in include order, or two for one include: %v", got)
		}
		if !slices.ContainsFunc(want, func(w includeErrorCase) bool { return w.index == e.Index }) && !slices.Contains(more, e.Index) {
			t.Errorf("unexpected include error %v", e)
		}
	}
}

func TestProbeLeavesNoFiles(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		dir := testDir(t)
		tmp := t.TempDir()
		t.Setenv("TMPDIR", tmp)
		for _, includes := range [][]string{{"<vector>", `"./physics.h"`}, {`"./bad.h"`}, {"<gocpp_no_such_header.h>"}} {
			if _, err := newCache().probe(context.Background(), cxx, dir, includes, []string{"std"}); err != nil {
				t.Fatal(err)
			}
		}
		if files, err := os.ReadDir(tmp); err != nil || len(files) > 0 {
			t.Errorf("TMPDIR after the probes: %v, %v; want it empty", files, err)
		}
		var want, got []string
		for name := range testHeaders {
			if top, _, _ := strings.Cut(name, "/"); !slices.Contains(want, top) {
				want = append(want, top)
			}
		}
		files, err := os.ReadDir(dir)
		for _, f := range files {
			got = append(got, f.Name())
		}
		slices.Sort(want)
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("the .gp folder after the probes: %q, %v; want only the test headers %q", got, err, want)
		}
	})
}

func TestProbeNoCandidates(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		// Without candidates (or with only reserved ones), the probe
		// still checks the includes: once per include set.
		dir := testDir(t)
		c := newCache()
		for _, cands := range [][]string{nil, {}, {"gp_x", "::gp_x::y"}} {
			res, err := c.probe(context.Background(), cxx, dir, []string{"<vector>", `"./physics.h"`}, cands)
			if err != nil || len(res.Kinds) != len(cands) || res.IncludeErrors != nil {
				t.Errorf("probe with candidates %#v = %+v, %v; want no include errors", cands, res, err)
			}
			for _, cand := range cands {
				if res.Kinds[cand] != NotNamespace {
					t.Errorf("%s: %v, want NotNamespace", cand, res.Kinds[cand])
				}
			}
			res, err = c.probe(context.Background(), cxx, dir, []string{"<vector>", "<gocpp_no_such_header.h>"}, cands)
			if err != nil || len(res.IncludeErrors) != 1 || res.IncludeErrors[0].Index != 1 || res.IncludeErrors[0].Kind != HeaderNotFound {
				t.Errorf("probe of a missing header with candidates %#v = %+v, %v; want its include error", cands, res, err)
			}
		}
		if n := c.runs.Load(); n != 2 {
			t.Errorf("%d compiler runs, want 2 (one per include set)", n)
		}
	})
}

func TestProbeManyCandidates(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := t.TempDir()
		var hdr strings.Builder
		var cases []kindCase
		for i := range 100 {
			fmt.Fprintf(&hdr, "namespace ns_%d {}\n", i)
			cases = append(cases, kindCase{fmt.Sprintf("ns_%d", i), Namespace}, kindCase{fmt.Sprintf("nx_%d", i), NotNamespace})
		}
		writeFiles(t, dir, map[string]string{"many.h": hdr.String()})
		c := newCache()
		res, err := c.probe(context.Background(), cxx, dir, []string{"<vector>", `"./many.h"`}, candsOf(cases))
		if err != nil {
			t.Fatal(err)
		}
		checkKinds(t, res, cases)
		if n := c.runs.Load(); n != 1 {
			t.Errorf("%d compiler runs for %d candidates, want 1", n, len(cases))
		}
	})
}

func TestProbeCache(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := testDir(t)
		c := newCache()
		ctx := context.Background()
		includes := []string{"<vector>", `"./physics.h"`}
		var runs int64
		probe := func(why string, wantRun bool, cases ...kindCase) {
			t.Helper()
			res, err := c.probe(ctx, cxx, dir, includes, candsOf(cases))
			if err != nil {
				t.Fatalf("%s: %v", why, err)
			}
			checkKinds(t, res, cases)
			if wantRun {
				runs++
			}
			if n := c.runs.Load(); n != runs {
				t.Errorf("%s: %d compiler runs so far, want %d", why, n, runs)
				runs = n
			}
		}
		probe("first call", true, kindCase{"std", Namespace}, kindCase{"stdd", NotNamespace}, kindCase{"physics_ns", Namespace})
		probe("the same again", false, kindCase{"std", Namespace}, kindCase{"stdd", NotNamespace}, kindCase{"physics_ns", Namespace})
		probe("fewer candidates, another order, another spelling", false, kindCase{"::physics_ns", Namespace}, kindCase{"std", Namespace})
		probe("a new candidate", true, kindCase{"std", Namespace}, kindCase{"vecns", Namespace})
		probe("the new one is cached too", false, kindCase{"vecns", Namespace}, kindCase{"stdd", NotNamespace})

		// Another folder or other includes are another include set.
		includes = []string{"<vector>", `"physics.h"`}
		probe("other includes", true, kindCase{"std", Namespace})
		includes = []string{"<vector>", `"./physics.h"`}
		probe("back to the first includes", false, kindCase{"std", Namespace})

		// A changed local header, and one that it includes.
		writeFiles(t, dir, map[string]string{"physics.h": "#pragma once\n#include \"vec.h\"\nnamespace physics_v2 {}\n"})
		probe("physics.h changed", true, kindCase{"physics_ns", NotNamespace}, kindCase{"physics_v2", Namespace})
		probe("physics.h changed, cached", false, kindCase{"physics_v2", Namespace})
		writeFiles(t, dir, map[string]string{"vec.h": "#pragma once\nnamespace vecns_v2 {}\n"})
		probe("vec.h changed", true, kindCase{"vecns", NotNamespace}, kindCase{"vecns_v2", Namespace})

		// A header that changed in the last seconds may have changed
		// while the compiler ran: the result is not cached. Its time is set
		// an hour ahead (as if it changed after the compiler started), so
		// that it stays within racyWindow of every probe below, however
		// long they take: with the time of the write, a probe that starts
		// more than racyWindow later (a slow compiler under load) caches it.
		vec := filepath.Join(dir, "vec.h")
		if err := os.WriteFile(vec, []byte("#pragma once\nnamespace vecns_v3 {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		soon := time.Now().Add(time.Hour)
		if err := os.Chtimes(vec, soon, soon); err != nil {
			t.Fatal(err)
		}
		probe("vec.h just changed", true, kindCase{"vecns_v3", Namespace})
		probe("vec.h just changed, not cached", true, kindCase{"vecns_v3", Namespace})
		writeFiles(t, dir, map[string]string{"vec.h": "#pragma once\nnamespace vecns_v3 {}\n"})
		probe("vec.h is old now", true, kindCase{"vecns_v3", Namespace})
		probe("vec.h is old now, cached", false, kindCase{"vecns_v3", Namespace})

		// A quoted include that was found in the system folders is hidden
		// by a file of that name that appears next to the .gp file.
		includes = []string{`"stdio.h"`}
		probe("quoted system header", true, kindCase{"shadow_ns", NotNamespace})
		writeFiles(t, dir, map[string]string{"stdio.h": "#pragma once\nnamespace shadow_ns {}\n"})
		probe("a local stdio.h appeared", true, kindCase{"shadow_ns", Namespace})
	})
}

func TestProbeCacheIncludeErrors(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := testDir(t)
		ctx := context.Background()
		c := newCache()
		c.errorTTL = time.Hour
		check := func(why string, includes []string, wantRuns int64, wantErr bool) {
			t.Helper()
			res, err := c.probe(ctx, cxx, dir, includes, []string{"std", "missing_ns"})
			if err != nil {
				t.Fatalf("%s: %v", why, err)
			}
			if got := len(res.IncludeErrors) > 0; got != wantErr {
				t.Errorf("%s: include errors %v, want some: %v", why, res.IncludeErrors, wantErr)
			}
			if n := c.runs.Load(); n != wantRuns {
				t.Errorf("%s: %d compiler runs so far, want %d", why, n, wantRuns)
			}
		}
		check("missing local header", []string{"<vector>", `"./missing.h"`}, 1, true)
		check("missing local header, cached", []string{"<vector>", `"./missing.h"`}, 1, true)
		writeFiles(t, dir, map[string]string{"missing.h": "namespace missing_ns {}\n"})
		check("the header appeared", []string{"<vector>", `"./missing.h"`}, 2, false)

		check("syntax error", []string{`"./bad.h"`}, 3, true)
		check("syntax error, cached", []string{`"./bad.h"`}, 3, true)
		writeFiles(t, dir, map[string]string{"bad.h": "#pragma once\nstruct Bad { int x; };\n"})
		check("syntax error fixed", []string{`"./bad.h"`}, 4, false)

		// A missing system header can appear where the cache doesn't look
		// (a library is installed), so the result expires.
		check("missing system header", []string{"<gocpp_no_such_header.h>"}, 5, true)
		check("missing system header, cached", []string{"<gocpp_no_such_header.h>"}, 5, true)
		c.errorTTL = 0
		check("expires at once", []string{"<gocpp_no_such_header_2.h>"}, 6, true)
		check("expired", []string{"<gocpp_no_such_header_2.h>"}, 7, true)
	})
}

func TestProbeConcurrent(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		dir := testDir(t)
		c := newCache()
		ctx := context.Background()
		setA := []string{"<vector>", `"./physics.h"`}
		setB := []string{"<filesystem>", `"native/x.hpp"`}
		casesA := []kindCase{{"std", Namespace}, {"physics_ns", Namespace}, {"stdd", NotNamespace}}
		casesB := []kindCase{{"::std::filesystem", Namespace}, {"alias_ns", Namespace}, {"fmtt", NotNamespace}}
		var wg sync.WaitGroup
		for i := range 24 {
			includes, cases := setA, casesA
			if i%3 == 2 {
				includes, cases = setB, casesB
			}
			// Each call asks for a part of its set's candidates, and
			// together they ask for all of them.
			cases = cases[:1+(i/3)%len(cases)]
			wg.Go(func() {
				res, err := c.probe(ctx, cxx, dir, includes, candsOf(cases))
				if err != nil {
					t.Error(err)
					return
				}
				for _, kc := range cases {
					if got := res.Kinds[kc.cand]; got != kc.want {
						t.Errorf("%s: %v, want %v", kc.cand, got, kc.want)
					}
				}
			})
		}
		wg.Wait()
		// The calls for one set wait for each other, so each set needs a
		// run for each new candidate it gets, in the worst case.
		if n := c.runs.Load(); n < 2 || n > 6 {
			t.Errorf("%d compiler runs for two include sets with three candidates each, want 2 to 6", n)
		}
		runs := c.runs.Load()
		for range 8 {
			wg.Go(func() {
				res, err := c.probe(ctx, cxx, dir, setA, candsOf(casesA))
				if err != nil || res.Kinds["physics_ns"] != Namespace {
					t.Errorf("cached probe: %+v, %v", res, err)
				}
			})
		}
		wg.Wait()
		if n := c.runs.Load(); n != runs {
			t.Errorf("%d compiler runs for cached candidates, want none", n-runs)
		}
	})
}

func TestProbeCanceled(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		// A canceled call waits for neither the compiler nor the lock.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c := newCache()
		if res, err := c.probe(ctx, cxx, t.TempDir(), []string{"<vector>"}, []string{"std"}); err == nil {
			t.Errorf("probe with a canceled context = %+v, want an error", res)
		}
		set, err := c.lock(context.Background(), setKey(cxx, "", []string{"<vector>"}))
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		if res, err := c.probe(ctx, cxx, "", []string{"<vector>"}, []string{"std"}); err == nil {
			t.Errorf("probe while another call holds the set = %+v, want a context error", res)
		}
		<-set.lock
	})
}

// BenchmarkProbe measures one probe run with <vector>, <filesystem> and
// <raylib.h> and 10 candidates, and the same probe from the cache. The
// language server runs the probe in the background, so the run's time
// shows how soon the namespace errors appear, and the cached time is what
// every check of an unchanged file pays.
func BenchmarkProbe(b *testing.B) {
	includes := []string{"<vector>", "<filesystem>", "<raylib.h>"}
	cands := []string{"std", "chrono", "stdd", "fmtt", "::std::filesystem", "rl", "glm", "ImGui", "::std::chrono", "foo"}
	matrix := compilerMatrix()
	required := requiredCXX(b, matrix)
	for _, prog := range matrix {
		name := filepath.Base(prog)
		b.Run(name, func(b *testing.B) {
			cxx := lookTestCompiler(b, prog, slices.Contains(required, prog) || slices.Contains(required, name))
			dir := b.TempDir()
			ctx := context.Background()
			if res, err := newCache().probe(ctx, cxx, dir, includes, cands); err != nil || len(res.IncludeErrors) > 0 {
				b.Skipf("the probe doesn't work here: %v %v", err, res.IncludeErrors)
			}
			b.Run("run", func(b *testing.B) {
				for b.Loop() {
					if _, err := newCache().probe(ctx, cxx, dir, includes, cands); err != nil {
						b.Fatal(err)
					}
				}
			})
			b.Run("cached", func(b *testing.B) {
				c := newCache()
				if _, err := c.probe(ctx, cxx, dir, includes, cands); err != nil {
					b.Fatal(err)
				}
				for b.Loop() {
					if _, err := c.probe(ctx, cxx, dir, includes, cands); err != nil {
						b.Fatal(err)
					}
				}
				if n := c.runs.Load(); n != 1 {
					b.Errorf("%d compiler runs, want 1", n)
				}
			})
			// A cache hit stats the local headers, to see whether they
			// changed; it reads them only when their stat changed. big.h is
			// 4 MiB, like a single-file library (miniaudio.h).
			b.Run("cached-local-headers", func(b *testing.B) {
				dir := b.TempDir()
				writeBenchHeaders(b, dir)
				local := append(slices.Clip(includes), `"./physics.h"`, `"./big.h"`)
				c := newCache()
				if _, err := c.probe(ctx, cxx, dir, local, cands); err != nil {
					b.Fatal(err)
				}
				for b.Loop() {
					if _, err := c.probe(ctx, cxx, dir, local, cands); err != nil {
						b.Fatal(err)
					}
				}
				if n := c.runs.Load(); n != 1 {
					b.Errorf("%d compiler runs, want 1", n)
				}
			})
		})
	}
}

// writeBenchHeaders writes physics.h and vec.h of testHeaders and a big.h
// of 4 MiB into dir, an hour old.
func writeBenchHeaders(b *testing.B, dir string) {
	old := time.Now().Add(-time.Hour)
	files := map[string]string{"physics.h": testHeaders["physics.h"], "vec.h": testHeaders["vec.h"],
		"big.h": strings.Repeat("// a comment line of a big single-file library, 64 bytes long..\n", 4<<20/64)}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			b.Fatal(err)
		}
		if err := os.Chtimes(path, old, old); err != nil {
			b.Fatal(err)
		}
	}
}
