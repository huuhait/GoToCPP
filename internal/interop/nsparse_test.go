package interop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests in this file need no compiler: they check the probe file, the
// parsing of the compiler's output (with outputs as gcc and clang print
// them) and the dependency file, and they run fake compilers for what the
// real ones don't do on purpose.

func TestMakeProbe(t *testing.T) {
	pf := makeProbe([]string{"<vector>", `"./x.h"`}, [][]string{{"std"}, {"std", "filesystem"}}, true)
	want := `#include <vector>
; template <class> struct gp_inc_0; using gp_inc_0_c = ::gp_inc_0<int>;
#include "./x.h"
; template <class> struct gp_inc_1; using gp_inc_1_c = ::gp_inc_1<int>;
` + helperLines + `#if defined(std)
#error gp_kind_macro
#else
namespace gp_kn_0 = ::std;
using gp_kt_0 = ::std;
constexpr bool gp_km_0 = gp_ktmpl<::std>(gp_kp5{});
using gp_kv_0 = decltype(::std);
template <class... gp_kT> auto gp_kw_0() -> decltype(::std<gp_kT...>);
template <auto... gp_kV> auto gp_kx_0() -> decltype(::std<gp_kV...>);
template <class... gp_kT> auto gp_kd_0(gp_kT... gp_ka) -> decltype(::std{gp_ka...});
template <class... gp_kT> auto gp_kc_0(gp_kT... gp_ka) -> decltype(::std(gp_ka...));
#endif
#if defined(std) || defined(filesystem)
#error gp_kind_macro
#else
namespace gp_kn_1 = ::std::filesystem;
using gp_kt_1 = ::std::filesystem;
constexpr bool gp_km_1 = gp_ktmpl<::std::filesystem>(gp_kp5{});
using gp_kv_1 = decltype(::std::filesystem);
template <class... gp_kT> auto gp_kw_1() -> decltype(::std::filesystem<gp_kT...>);
template <auto... gp_kV> auto gp_kx_1() -> decltype(::std::filesystem<gp_kV...>);
template <class... gp_kT> auto gp_kd_1(gp_kT... gp_ka) -> decltype(::std::filesystem{gp_ka...});
template <class... gp_kT> auto gp_kc_1(gp_kT... gp_ka) -> decltype(::std::filesystem(gp_ka...));
#endif
namespace gp_probe_end = ::gp_probe_no_such_namespace;
namespace gp_probe_eof {}
`
	if got := string(pf.src); got != want {
		t.Errorf("makeProbe:\n%s\nwant:\n%s", got, want)
	}
	lines := strings.Split(want, "\n")
	if pf.firstBlock != 5+nHelperLines || nHelperLines != 7 || !strings.HasPrefix(lines[pf.firstBlock-1], "#if defined(std)") {
		t.Errorf("firstBlock = %d, %d helper lines", pf.firstBlock, nHelperLines)
	}
	for k, prefix := range map[int]string{blockError: "#error", blockElse: "#else", lineNamespace: "namespace gp_kn_1", lineType: "using gp_kt_1",
		lineTemplate: "constexpr bool gp_km_1", lineValue: "using gp_kv_1", lineValueTmplT: "template <class... gp_kT> auto gp_kw_1",
		lineValueTmplV: "template <auto... gp_kV> auto gp_kx_1", lineBraced: "template <class... gp_kT> auto gp_kd_1",
		lineCall: "template <class... gp_kT> auto gp_kc_1", blockEndif: "#endif"} {
		if n := pf.firstBlock + blockLines + k; !strings.HasPrefix(lines[n-1], prefix) {
			t.Errorf("line %d of the second block (%d) is %q, want %s...", k, n, lines[n-1], prefix)
		}
	}
	if pf.canary() != 5+nHelperLines+2*blockLines || !strings.HasPrefix(lines[pf.canary()-1], "namespace gp_probe_end") {
		t.Errorf("canary() = %d", pf.canary())
	}
	for n := 1; n <= len(lines); n++ {
		if got, want := pf.helpers(n), n >= 5 && n < 5+nHelperLines; got != want {
			t.Errorf("helpers(%d) = %v", n, got)
		}
	}

	// Without paths there are no helper lines: the file is the namespace
	// probe's of Phase 01 without candidates.
	for _, kinds := range []bool{true, false} {
		pf = makeProbe([]string{"<vector>"}, nil, kinds)
		want = "#include <vector>\n; template <class> struct gp_inc_0; using gp_inc_0_c = ::gp_inc_0<int>;\n" +
			"namespace gp_probe_end = ::gp_probe_no_such_namespace;\nnamespace gp_probe_eof {}\n"
		if got := string(pf.src); got != want || pf.canary() != 3 || pf.helpers(3) {
			t.Errorf("makeProbe without paths:\n%s\nwant:\n%s", got, want)
		}
	}

	// The namespace mode: the namespace probe of Phase 01.
	pf = makeProbe([]string{"<vector>"}, [][]string{{"std"}, {"std", "filesystem"}}, false)
	want = `#include <vector>
; template <class> struct gp_inc_0; using gp_inc_0_c = ::gp_inc_0<int>;
#if defined(std)
#error gp_kind_macro
#else
namespace gp_kn_0 = ::std;
#endif
#if defined(std) || defined(filesystem)
#error gp_kind_macro
#else
namespace gp_kn_1 = ::std::filesystem;
#endif
namespace gp_probe_end = ::gp_probe_no_such_namespace;
namespace gp_probe_eof {}
`
	if got := string(pf.src); got != want || pf.firstBlock != 3 || pf.canary() != 13 || pf.helpers(3) {
		t.Errorf("makeProbe for namespaces:\n%s\nwant:\n%s", got, want)
	}
	if path, offset, ok := pf.block(12); !ok || path != 1 || offset != nsBlockEndif {
		t.Errorf("block(12) = %d, %d, %v", path, offset, ok)
	}
	for _, n := range []int{2, 13} {
		if _, _, ok := pf.block(n); ok {
			t.Errorf("block(%d) is in a block", n)
		}
	}
}

func TestClassify(t *testing.T) {
	// The bits of the C++ lines of a block: the lines with errors.
	all := uint(0)
	for k := lineNamespace; k <= lineCall; k++ {
		all |= 1 << k
	}
	not := func(lines ...int) uint { // errors on every C++ line but these
		f := all
		for _, k := range lines {
			f &^= 1 << k
		}
		return f
	}
	for _, c := range []struct {
		failed uint
		want   Kind
	}{
		{0, Namespace}, // nothing failed: what the first line says
		{not(lineNamespace), Namespace},
		{not(lineType, lineCall), Type},                                // std::string
		{not(lineTemplate, lineCall), Template},                        // std::vector
		{not(lineValue, lineCall), Value},                              // std::cout
		{not(lineValue), Value},                                        // a variable of a type without operator()
		{not(lineValueTmplT, lineValueTmplV, lineCall), ValueTemplate}, // std::max
		{not(lineValueTmplT), ValueTemplate},                           // std::is_same_v
		{not(lineValueTmplV), ValueTemplate},                           // std::in_place_index
		{not(lineBraced, lineCall), Template},                          // std::ranges::subrange
		{not(lineType, lineBraced, lineCall), Type},                    // an aggregate struct
		{not(lineCall), Value},                                         // overloaded functions
		{all, NotFound},
		{1 << blockError, Macro},
		{1<<blockError | all, Macro},
		{1 << blockIf, NotFound},
		{1<<blockIf | not(lineNamespace), NotFound},
		{1 << blockElse, NotFound},
		{1 << blockEndif, NotFound},
	} {
		if got := classify(c.failed); got != c.want {
			t.Errorf("classify(%b) = %v, want %v", c.failed, got, c.want)
		}
	}
	for _, c := range []struct {
		failed uint
		want   Kind
	}{
		{0, Namespace},
		{1 << lineNamespace, NotNamespace},
		{1 << blockError, Macro},
		{1 << blockIf, NotNamespace},
		{1 << nsBlockEndif, NotNamespace},
	} {
		if got := classifyNS(c.failed); got != c.want {
			t.Errorf("classifyNS(%b) = %v, want %v", c.failed, got, c.want)
		}
	}
}

// The probe file of the parser tests: two includes (lines 1 to 4), the
// helper lines, the paths std and stdd, the canary and the last line.
const testSrc = "/tmp/p/gp_probe.cpp"

var (
	testIncludes = []string{"<vector>", `"./bad.h"`}
	testProbe    = makeProbe(testIncludes, [][]string{{"std"}, {"stdd"}}, true)
	canaryGCC    = errAt(testProbe.canary(), 28, "'gp_probe_no_such_namespace' is not a namespace-name")
)

// line returns the line of the probe file testProbe that is line k of the
// block of path i.
func line(i, k int) int { return testProbe.firstBlock + i*blockLines + k }

// errAt returns a gcc error at the line n of testSrc.
func errAt(n, col int, msg string) string {
	return fmt.Sprintf("%s:%d:%d: error: %s\n", testSrc, n, col, msg)
}

// allFail returns gcc errors on every C++ line of the block of path i but
// the lines but.
func allFail(i int, but ...int) string {
	var b strings.Builder
	for k := lineNamespace; k <= lineCall; k++ {
		if !slices.Contains(but, k) {
			b.WriteString(errAt(line(i, k), 5, "nope"))
		}
	}
	return b.String()
}

func TestReadOutput(t *testing.T) {
	stddNS := line(1, lineNamespace)
	for _, c := range []struct {
		name    string
		out     string
		kinds   []Kind
		incErrs []IncludeError
		err     string // a part of the error, if it must fail
	}{
		{name: "gcc",
			out:   errAt(stddNS, 23, "'stdd' is not a namespace-name; did you mean 'std'?") + allFail(1) + canaryGCC,
			kinds: []Kind{Namespace, NotFound}},
		{name: "kinds",
			out:   allFail(0, lineValue, lineCall) + allFail(1, lineTemplate) + canaryGCC,
			kinds: []Kind{Value, Template}},
		{name: "clang, notes and errors without a column",
			out: fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:23: error: expected namespace name\n", line(0, lineNamespace)) +
				"In file included from /tmp/p/gp_probe.cpp:1:\n" +
				"/usr/include/c++/15/vector:111:11: note: namespace 'std' defined here\n" +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d: error: expected namespace name\n", stddNS) +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d: error: expected a type\n", line(1, lineType)) +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:28: error: expected namespace name\n", testProbe.canary()) +
				"3 errors generated.\n",
			kinds: []Kind{Type, Template}},
		{name: "macro",
			out:   fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:2: error: #error gp_kind_macro\n", line(0, blockError)) + canaryGCC,
			kinds: []Kind{Macro, Namespace}},
		{name: "an error on the #if line",
			out:   fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:5: error: \"defined\" cannot be used as a macro name\n", line(1, blockIf)) + canaryGCC,
			kinds: []Kind{Namespace, NotFound}},
		{name: "CRLF",
			out:   strings.ReplaceAll(allFail(1)+canaryGCC, "\n", "\r\n"),
			kinds: []Kind{Namespace, NotFound}},
		{name: "clang include stack, outermost first",
			out: "In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"In file included from /tmp/p/./inc_bad.h:2:\n" +
				"/tmp/p/./bad.h:2:19: error: expected ';' at end of declaration list\n" +
				"/tmp/p/./bad.h:2:21: error: expected ';' after struct\n" +
				allFail(1) + canaryGCC,
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 2, Column: 19, Msg: "expected ';' at end of declaration list"}}},
		{name: "gcc include stack, innermost first",
			out: "In file included from /tmp/p/inc_bad.h:2,\n" +
				"                 from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h: In function 'void f()':\n" +
				"/tmp/p/bad.h:2:18: error: expected ';' at end of member declaration\n" +
				canaryGCC,
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 2, Column: 18, Msg: "expected ';' at end of member declaration"}}},
		{name: "gcc: missing header first, then nothing",
			out: "/tmp/p/gp_probe.cpp:3:10: fatal error: ./bad.h: No such file or directory\n" +
				"compilation terminated.\n",
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, Kind: HeaderNotFound, Msg: "./bad.h: No such file or directory"}}},
		{name: "first error of each include, in include order",
			out: "/tmp/p/gp_probe.cpp:3:10: fatal error: './bad.h' file not found\n" +
				"In file included from /tmp/p/gp_probe.cpp:1:\n" +
				"/usr/include/c++/15/vector:1:1: error: first\n" +
				"/usr/include/c++/15/vector:2:1: error: second\n",
			incErrs: []IncludeError{
				{Index: 0, Include: "<vector>", File: "/usr/include/c++/15/vector", Line: 1, Column: 1, Msg: "first"},
				{Index: 1, Include: `"./bad.h"`, Kind: HeaderNotFound, Msg: "'./bad.h' file not found"}}},
		{name: "unfinished declaration",
			out: "/tmp/p/gp_probe.cpp:4:1: error: expected primary-expression before ';' token\n" +
				errAt(line(0, lineNamespace), 1, "expected primary-expression before 'namespace'") + canaryGCC,
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, Kind: HeaderUnclosed, Msg: "the header ends inside a declaration or a block that it doesn't close: expected primary-expression before ';' token"}}},
		{name: "open brace",
			out: canaryGCC +
				errAt(testProbe.canary()+1, 26, "expected '}' at end of input") +
				"In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h:2:12: note: to match this '{'\n",
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 2, Column: 12, Msg: "expected '}' at end of input (to match this '{')"}}},
		// gcc prints the include stack of an #include only once: the error
		// at the end of the file is in a header whose stack came with a note.
		{name: "gcc: no include stack the second time",
			out: errAt(stddNS, 23, "reference to 'stdd' is ambiguous") +
				"In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h:2:29: note: candidates are: 'namespace a::stdd { }'\n" +
				canaryGCC +
				"/tmp/p/bad.h: In instantiation of 'int late_f() [with T = int]':\n" +
				"/tmp/p/bad.h:7:41:   required from here\n" +
				"/tmp/p/bad.h:6:45: error: 'nope' is not a member of 'int'\n",
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 6, Column: 45, Msg: "'nope' is not a member of 'int'"}}},
		// An error in an instantiation that a path's line asked for belongs
		// to that line, also the next errors in the same context; a context
		// that a header started is the header's again.
		{name: "gcc: instantiation from a path",
			out: "In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h: In instantiation of 'struct B<int>':\n" +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:23:   required from here\n", stddNS) +
				"/tmp/p/bad.h:2:56: error: static assertion failed: one\n" +
				"/tmp/p/bad.h:2:56: note: the comparison reduces to '(4 == 0)'\n" +
				"/tmp/p/bad.h:2:94: error: static assertion failed: two\n" +
				allFail(1, lineNamespace, lineCall) +
				"/tmp/p/bad.h: In instantiation of 'struct B<char>':\n" +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:19:   required from here\n", line(1, lineCall)) +
				"/tmp/p/bad.h:2:56: error: static assertion failed: one\n" +
				canaryGCC,
			kinds: []Kind{Namespace, NotFound}},
		{name: "gcc: instantiation from a path, then from a header",
			out: "In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h: In instantiation of 'struct B<int>':\n" +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:23:   required from here\n", stddNS) +
				"/tmp/p/bad.h:2:56: error: static assertion failed: one\n" +
				canaryGCC +
				"/tmp/p/bad.h: In instantiation of 'int late_f() [with T = int]':\n" +
				"/tmp/p/bad.h:7:41:   required from here\n" +
				"/tmp/p/bad.h:6:45: error: 'nope' is not a member of 'int'\n",
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 6, Column: 45, Msg: "'nope' is not a member of 'int'"}}},
		{name: "clang: instantiation from a path",
			out: allFail(1, lineType) +
				"In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/./bad.h:3:17: note: 'B2I' declared here\n" +
				"/tmp/p/./bad.h:2:46: error: static assertion failed due to requirement 'sizeof(int) == 0': one\n" +
				"/tmp/p/./bad.h:4:10: note: in instantiation of template class 'B2<int>' requested here\n" +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:16: note: in instantiation of template class 'B3<int>' requested here\n", line(1, lineType)) +
				"In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/./bad.h:2:56: note: expression evaluates to '4 == 0'\n" +
				canaryGCC,
			kinds: []Kind{Namespace, NotFound}},
		{name: "clang: instantiation from a header",
			out: "In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/./bad.h:6:45: error: type 'int' cannot be used prior to '::' because it has no members\n" +
				"/tmp/p/./bad.h:7:41: note: in instantiation of function template specialization 'late_f<int>' requested here\n" +
				canaryGCC,
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 6, Column: 45, Msg: "type 'int' cannot be used prior to '::' because it has no members"}}},
		{name: "an error in the helper lines",
			out: errAt(testProbe.firstBlock-1, 5, "what") + canaryGCC,
			err: fmt.Sprintf("unexpected error: gp_probe.cpp:%d:5: error: what", testProbe.firstBlock-1)},
		{name: "end of the file without a note",
			out: canaryGCC + errAt(testProbe.canary()+1, 26, "expected '}' at end of input"),
			err: fmt.Sprintf("unexpected error: gp_probe.cpp:%d:26: error: expected '}' at end of input", testProbe.canary()+1)},
		{name: "a header that no include brought in",
			out: "/tmp/p/other.h:1:1: error: what\n" + canaryGCC,
			err: "unexpected error: /tmp/p/other.h:1:1: error: what"},
		{name: "driver error",
			out: "g++: error: unrecognized command-line option '-Qunused-arguments'\n",
			err: "unexpected error: g++: error: unrecognized command-line option '-Qunused-arguments'"},
		{name: "gcc's sorry is an error",
			out:     "In file included from /tmp/p/gp_probe.cpp:1:\n/usr/include/c++/15/vector:9:5: sorry, unimplemented: this\n" + canaryGCC,
			incErrs: []IncludeError{{Index: 0, Include: "<vector>", File: "/usr/include/c++/15/vector", Line: 9, Column: 5, Msg: "this"}}},
		{name: "warnings don't count",
			out:   fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:1: warning: hmm\n", line(0, lineNamespace)) + canaryGCC,
			kinds: []Kind{Namespace, Namespace}},
		{name: "crash",
			out: fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:1: internal compiler error: Segmentation fault\n", line(0, lineNamespace)) + canaryGCC,
			err: fmt.Sprintf("unexpected error: gp_probe.cpp:%d:1: internal compiler error: Segmentation fault", line(0, lineNamespace))},
		{name: "stopped early",
			out: errAt(stddNS, 23, "'stdd' is not a namespace-name"),
			err: fmt.Sprintf("it didn't check the whole probe file (no error for its line %d); its output: /tmp/p/gp_probe.cpp:%d:23: error: 'stdd' is not a namespace-name", testProbe.canary(), stddNS)},
		{name: "no output",
			err: fmt.Sprintf("it didn't check the whole probe file (no error for its line %d); it printed nothing", testProbe.canary())},
	} {
		t.Run(c.name, func(t *testing.T) {
			run, err := readOutput(testProbe, []byte(c.out), testSrc, testIncludes)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("readOutput = %+v, %v; want an error with %q", run, err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for i := range c.incErrs { // the probe cleans the paths of headers
				if c.incErrs[i].File != "" {
					c.incErrs[i].File = filepath.Clean(c.incErrs[i].File)
				}
			}
			if !slices.Equal(run.kinds, c.kinds) || !reflect.DeepEqual(run.incErrs, c.incErrs) {
				t.Errorf("readOutput:\nkinds   %v\nincErrs %+v\nwant:\nkinds   %v\nincErrs %+v", run.kinds, run.incErrs, c.kinds, c.incErrs)
			}
		})
	}
}

func TestReadOutputStopped(t *testing.T) {
	stddNS := line(1, lineNamespace)
	for _, c := range []struct {
		name string
		out  string
		path int // the path that stopped the compiler, or -1 for another error
	}{
		{name: "gcc",
			out: "In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h: In instantiation of 'struct Deep<int*>':\n" +
				"/tmp/p/bad.h:16:40:   required from 'struct Deep<int>'\n" +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:23:   required from here\n", stddNS) +
				"/tmp/p/bad.h:16:40: fatal error: template instantiation depth exceeds maximum of 900\n" +
				"compilation terminated.\n",
			path: 1},
		{name: "clang",
			out: allFail(0) +
				"In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/./bad.h:16:56: fatal error: recursive template instantiation exceeded maximum depth of 1024\n" +
				"/tmp/p/./bad.h:16:56: note: in instantiation of template class 'Deep<int **>' requested here\n" +
				"/tmp/p/./bad.h:16:56: note: (skipping 1015 contexts in backtrace; use -ftemplate-backtrace-limit=0 to see all)\n" +
				fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:16: note: in instantiation of template class 'Deep<int>' requested here\n", line(0, lineCall)),
			path: 0},
		// Not a fatal error, or not on a path's line: the probe fails (in
		// the helper lines, with an unexpected error).
		{name: "not fatal",
			out:  errAt(stddNS, 23, "'stdd' is not a namespace-name"),
			path: -1},
		{name: "fatal in the helper lines",
			out:  fmt.Sprintf("/tmp/p/gp_probe.cpp:%d:1: fatal error: what\n", testProbe.firstBlock-1),
			path: -1},
	} {
		t.Run(c.name, func(t *testing.T) {
			run, err := readOutput(testProbe, []byte(c.out), testSrc, testIncludes)
			var stop *errStopped
			switch {
			case err == nil:
				t.Fatalf("readOutput = %+v, want an error", run)
			case c.path >= 0 && !strings.Contains(err.Error(), "it didn't check the whole probe file"):
				t.Errorf("error %q", err)
			case c.path < 0 && errors.As(err, &stop):
				t.Errorf("readOutput: %v, stopped at path %d; want another error", err, stop.path)
			case c.path >= 0 && (!errors.As(err, &stop) || stop.path != c.path):
				t.Errorf("readOutput: %v (%#v); want stopped at path %d", err, stop, c.path)
			}
		})
	}
}

func TestReadOutputNamespaces(t *testing.T) {
	pf := makeProbe(testIncludes, [][]string{{"std"}, {"stdd"}, {"EOF"}}, false)
	stdd := pf.firstBlock + nsBlockLines + lineNamespace
	eof := pf.firstBlock + 2*nsBlockLines + blockError
	out := errAt(stdd, 23, "'stdd' is not a namespace-name") + errAt(eof, 2, "#error gp_kind_macro") + errAt(pf.canary(), 28, "x")
	run, err := readOutput(pf, []byte(out), testSrc, testIncludes)
	if err != nil || !slices.Equal(run.kinds, []Kind{Namespace, NotNamespace, Macro}) {
		t.Errorf("readOutput = %+v, %v; want [Namespace NotNamespace Macro]", run, err)
	}
}

func TestParseDiagnosticsWindowsPaths(t *testing.T) {
	src := `C:\Users\me\AppData\Local\Temp\gocpp-probe-1\gp_probe.cpp`
	out := "In file included from " + src + ":3:\r\n" +
		`C:\proj\my lib\bad.h:2:19: error: expected ';'` + "\r\n" +
		src + ":13:23: error: expected namespace name\r\n"
	got := parseDiagnostics([]byte(out), src)
	want := []diagnostic{
		{severity: "error", file: `C:\proj\my lib\bad.h`, line: 2, col: 19, msg: "expected ';'", text: `C:\proj\my lib\bad.h:2:19: error: expected ';'`, probeLine: 3},
		{severity: "error", file: src, line: 13, col: 23, msg: "expected namespace name", text: src + ":13:23: error: expected namespace name", probeLine: 13},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseDiagnostics:\n%+v\nwant:\n%+v", got, want)
	}
}

func TestParseDeps(t *testing.T) {
	for _, c := range []struct {
		name, data, dir string
		want            []string
	}{
		{name: "gcc",
			data: "gp_probe: /tmp/p/gp_probe.cpp \\\n /tmp/p/./physics.h \\\n /tmp/p/vec.h\n",
			want: []string{"/tmp/p/physics.h", "/tmp/p/vec.h"}},
		{name: "escapes",
			data: "gp_probe: /tmp/p/gp_probe.cpp /tmp/p/my\\ dir/x.h /tmp/p/a\\#b.h /tmp/p/c$$d.h\n",
			want: []string{"/tmp/p/my dir/x.h", "/tmp/p/a#b.h", "/tmp/p/c$d.h"}},
		{name: "CRLF and tabs",
			data: "gp_probe: /tmp/p/gp_probe.cpp \\\r\n\t/tmp/p/x.h\r\n",
			want: []string{"/tmp/p/x.h"}},
		{name: "relative", dir: "/src/game",
			data: "gp_probe: /tmp/p/gp_probe.cpp native/x.hpp\n",
			want: []string{filepath.Join("/src/game", "native/x.hpp")}},
		{name: "only the probe", data: "gp_probe: /tmp/p/gp_probe.cpp\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("Unix paths")
			}
			if got := parseDeps([]byte(c.data), testSrc, c.dir); !slices.Equal(got, c.want) {
				t.Errorf("parseDeps = %q, want %q", got, c.want)
			}
		})
	}
}

func TestProbeBadArguments(t *testing.T) {
	c := newCache()
	cxx := build.CXX{Path: filepath.Join(t.TempDir(), "no-such-c++"), Family: build.GCC, Version: "none"}
	for _, inc := range []string{"raylib.h", "", "<>", `""`, "<a", `"a`, "<a\nb>", "<a>b>", `"a"b"`, "<a\x00>", `<a.h>"`} {
		if _, err := c.probe(context.Background(), cxx, "", nil, []string{"<vector>", inc}, []string{"std"}); err == nil || !strings.Contains(err.Error(), "bad include") {
			t.Errorf("include %q: %v, want a bad include error", inc, err)
		}
	}
	for _, cand := range []string{"", "::", "a::", "::a::::b", "std.vector", "1abc", "a b", "a;b", "a::1b", "a\nb", "a-b", ":a"} {
		if _, err := c.probe(context.Background(), cxx, "", nil, []string{"<vector>"}, []string{"std", cand}); err == nil || !strings.Contains(err.Error(), "bad path") {
			t.Errorf("path %q: %v, want a bad path error", cand, err)
		}
	}
	for _, cand := range []string{"std", "::std", "std::filesystem", "::std::filesystem", "_x", "x1", "ñandú", "日本"} {
		if _, err := splitPath(cand); err != nil {
			t.Errorf("splitPath(%q): %v", cand, err)
		}
	}
	if n := c.runs.Load(); n != 0 {
		t.Errorf("%d compiler runs, want 0", n)
	}
	if _, err := c.probe(context.Background(), build.CXX{Path: cxx.Path}, "", nil, []string{"<vector>"}, []string{"std"}); err == nil || !strings.Contains(err.Error(), "unknown family") {
		t.Errorf("a compiler of no family: %v, want an unknown family error", err)
	}
}

func TestProbeNoIncludes(t *testing.T) {
	// A .gp file with a namespace rename and no header import: nothing
	// runs (the compiler here doesn't exist), every path is Unknown, and
	// reserved names are NotFound.
	c := newCache()
	cxx := build.CXX{Path: filepath.Join(t.TempDir(), "no-such-c++"), Family: build.GCC, Version: "none"}
	res, err := c.probe(context.Background(), cxx, t.TempDir(), nil, nil, []string{"::std::filesystem", "std", "gp_ns_0"})
	want := map[string]Kind{"::std::filesystem": Unknown, "std": Unknown, "gp_ns_0": NotFound}
	if err != nil || !reflect.DeepEqual(res.Kinds, want) || res.IncludeErrors != nil {
		t.Errorf("probe without includes = %+v, %v; want Kinds %v", res, err, want)
	}
	// No candidates either. (With includes, the probe runs also without
	// candidates or with only reserved ones, to check the includes:
	// TestProbeNoCandidates.)
	res, err = c.probe(context.Background(), cxx, t.TempDir(), nil, nil, nil)
	if err != nil || res.Kinds == nil || len(res.Kinds) != 0 || res.IncludeErrors != nil {
		t.Errorf("probe without includes and candidates = %+v, %v; want empty Kinds", res, err)
	}
	if n := c.runs.Load(); n != 0 {
		t.Errorf("%d compiler runs, want 0", n)
	}
}

func TestFileStateUnchanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.h")
	write := func(content string, mod time.Time) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-time.Hour)
	write("namespace a {}\n", old)
	f, _, err := stateOf(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if !f.unchanged(now) {
		t.Error("unchanged file: changed")
	}
	// Touched, same content: unchanged, and the new stat is kept.
	write("namespace a {}\n", old.Add(time.Minute))
	before := f.stat
	if !f.unchanged(now) || f.stat == before {
		t.Errorf("touched file: unchanged %v, stat %+v (before %+v); want unchanged with the new stat", f.unchanged(now), f.stat, before)
	}
	// New content of the same size, with the same modification time: on
	// Linux the change time gives it away.
	write("namespace b {}\n", old.Add(time.Minute))
	if runtime.GOOS == "linux" && f.unchanged(now) {
		t.Error("new content with the old size and modification time: unchanged")
	}
	write("namespace bb {}\n", old.Add(2*time.Minute))
	if f.unchanged(now) {
		t.Error("new content: unchanged")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if f.unchanged(now) {
		t.Error("removed file: unchanged")
	}
	gone := fileState{path: path}
	if !gone.unchanged(now) {
		t.Error("a file that still isn't there: changed")
	}
}

// rewriteDuringRead makes stateOf write content into the file at path
// once, between its first stat and its read, with the modification time
// mod, as a save in an editor at that moment would.
func rewriteDuringRead(t *testing.T, path, content string, mod time.Time) {
	t.Helper()
	once := false
	hook := func(p string) {
		if p != path || once {
			return
		}
		once = true
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Error(err)
		}
		if err := os.Chtimes(path, mod, mod); err != nil {
			t.Error(err)
		}
	}
	testHookRead.Store(&hook)
	t.Cleanup(func() { testHookRead.Store(nil) })
}

func TestStateOfFileChangedWhileRead(t *testing.T) {
	// Not parallel: it sets testHookRead.
	old := time.Now().Add(-time.Hour)
	for _, c := range []struct{ name, content string }{
		{"other content", "namespace vecns_v2 {}\n"},
		// Written again with the same content, size and modification time:
		// on Linux the change time still changes, so it counts as changed.
		{"same content", "namespace vecns {}\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.name == "same content" && runtime.GOOS != "linux" {
				t.Skip("needs the change time (Linux)")
			}
			path := filepath.Join(t.TempDir(), "vec.h")
			write := func(content string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, old, old); err != nil {
					t.Fatal(err)
				}
			}
			write("namespace vecns {}\n")
			recorded, _, err := stateOf(path)
			if err != nil {
				t.Fatal(err)
			}

			// dependsOn: the result can't be cached.
			rewriteDuringRead(t, path, c.content+"// longer\n", old)
			if files, ok := dependsOn([]string{path}, true, false, time.Now(), "", nil); ok {
				t.Errorf("dependsOn = %+v, true; want not cacheable for a file that changed while it was read", files)
			}

			// unchanged: a check whose stat misses (the file was touched)
			// and then reads a file that changes counts it as changed.
			write("namespace vecns {}\n")
			recorded.stat = fileStat{} // the stat shortcut misses
			rewriteDuringRead(t, path, c.content, old)
			if recorded.unchanged(time.Now()) {
				t.Error("unchanged = true for a file that changed while it was read")
			}
		})
	}
}

func TestKindString(t *testing.T) {
	for k, want := range map[Kind]string{Unknown: "Unknown", Namespace: "Namespace", Type: "Type", Template: "Template", Value: "Value",
		ValueTemplate: "ValueTemplate", Macro: "Macro", NotFound: "NotFound", NotNamespace: "NotNamespace", 9: "Kind(9)"} {
		if got := k.String(); got != want {
			t.Errorf("Kind(%d).String() = %q, want %q", int(k), got, want)
		}
	}
}

func TestIncludeErrorError(t *testing.T) {
	for _, c := range []struct {
		e    IncludeError
		want string
	}{
		{IncludeError{Include: "<raylibb.h>", Msg: "'raylibb.h' file not found"}, "<raylibb.h>: 'raylibb.h' file not found"},
		{IncludeError{Include: `"./bad.h"`, File: "/p/bad.h", Line: 2, Column: 19, Msg: "expected ';'"}, `"./bad.h": /p/bad.h:2:19: expected ';'`},
		{IncludeError{Include: `"./bad.h"`, File: "/p/bad.h", Line: 2, Msg: "expected ';'"}, `"./bad.h": /p/bad.h:2: expected ';'`},
	} {
		if got := c.e.Error(); got != c.want {
			t.Errorf("Error() = %q, want %q", got, c.want)
		}
	}
}

// fakeCompiler writes a fake compiler, the shell script body, and returns
// it as a compiler of the family family. The script can write next to
// itself ($0.args, $0.env).
func fakeCompiler(t *testing.T, family build.Family, body string) build.CXX {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	path := filepath.Join(t.TempDir(), "fake++")
	writeScript(t, path, "#!/bin/sh\n"+body+"\n")
	return build.CXX{Path: path, Family: family, Version: "fake 1.0"}
}

// writeScript writes the shell script src (which starts with #!/bin/sh) to
// path, executable, and runs it once, so that a test can run it without
// "text file busy". A test that forks a process (a compiler) while another
// one writes a script can give that child the script's write fd for a
// moment (between fork and exec, golang/go#22315), and exec fails with
// ETXTBSY while it has it. The run retries that; once one exec succeeded,
// no process has the fd any more. For the run, the line after #!/bin/sh
// exits at once when GOCPP_TEST_WARMUP is 1.
func writeScript(t testing.TB, path, src string) {
	t.Helper()
	body, ok := strings.CutPrefix(src, "#!/bin/sh\n")
	if !ok {
		t.Fatalf("script %s doesn't start with #!/bin/sh", path)
	}
	src = "#!/bin/sh\n[ \"$GOCPP_TEST_WARMUP\" = 1 ] && exit 0\n" + body
	if err := os.WriteFile(path, []byte(src), 0o755); err != nil {
		t.Fatal(err)
	}
	for try := 0; ; try++ {
		cmd := exec.Command(path)
		cmd.Env = append(os.Environ(), "GOCPP_TEST_WARMUP=1")
		err := cmd.Run()
		if err == nil {
			return
		}
		if !errors.Is(err, syscall.ETXTBSY) || try == 50 {
			t.Fatalf("running %s: %v", path, err)
		}
		time.Sleep(time.Duration(try+1) * 10 * time.Millisecond)
	}
}

func TestProbeFakeCompilers(t *testing.T) {
	for _, c := range []struct {
		name, body string
		err        string
	}{
		// A compiler that prints nothing can't make every candidate a
		// namespace.
		{"silent", "exit 0", fmt.Sprintf("kind probe: fake++: it didn't check the whole probe file (no error for its line %d); it printed nothing", 3+nHelperLines+blockLines)},
		{"driver error", `echo "fake++: error: unrecognized command-line option '-Qunused-arguments'" >&2; exit 1`,
			"kind probe: fake++: unexpected error: fake++: error: unrecognized command-line option '-Qunused-arguments'"},
		{"too much output", `printf '%9000000s\n' x`, "kind probe: fake++ printed more than 8 MiB"},
		{"crash", crashBody(), "kind probe: fake++: signal: segmentation fault"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cxx := fakeCompiler(t, build.GCC, c.body)
			res, err := newCache().probe(context.Background(), cxx, t.TempDir(), nil, []string{"<vector>"}, []string{"std"})
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Fatalf("probe = %+v, %v; want an error with %q", res, err, c.err)
			}
			if res.Kinds != nil || res.IncludeErrors != nil {
				t.Errorf("probe with an error = %+v, want the zero Result", res)
			}
		})
	}
}

func TestProbeOutputLimitStops(t *testing.T) {
	// A compiler that prints forever is stopped when its output passes the
	// limit, not at the time limit. A shell loop is slow under load (and with -race):
	// yes(1) prints as fast as the pipe takes it, and the time limit is long, so
	// that "stopped by the limit" is told by the error, not by the clock.
	const long = `exec yes "$(printf '%1000s' x)"`
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cxx := fakeCompiler(t, build.GCC, long)
	start := time.Now()
	_, err := newCache().probe(ctx, cxx, t.TempDir(), nil, []string{"<vector>"}, []string{"std"})
	if err == nil || !strings.Contains(err.Error(), "printed more than 8 MiB") {
		t.Errorf("probe = %v, want an output limit error", err)
	}
	if d := time.Since(start); d > time.Minute { // well before the 2 minutes of ctx
		t.Errorf("the probe took %v", d)
	}

	// When the output had include errors before the limit, they are the
	// result (the candidates are Unknown then anyway).
	cxx = fakeCompiler(t, build.GCC, `for a in "$@"; do f=$a; done
echo "$f:1:10: fatal error: x.h: No such file or directory"
`+long)
	res, err := newCache().probe(ctx, cxx, t.TempDir(), nil, []string{"<x.h>"}, []string{"std"})
	want := []IncludeError{{Index: 0, Include: "<x.h>", Kind: HeaderNotFound, Msg: "x.h: No such file or directory"}}
	if err != nil || !reflect.DeepEqual(res.IncludeErrors, want) || res.Kinds["std"] != Unknown {
		t.Errorf("probe = %+v, %v; want the include error %+v", res, err, want)
	}
}

func TestProbeTimeLimit(t *testing.T) {
	// The compiler hangs, and so does a process that it started, with the
	// output open: the probe stops both, and leaves no files behind.
	cxx := fakeCompiler(t, build.GCC, "(while :; do :; done) &\nwhile :; do :; done")
	tmp := t.TempDir()
	dir := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := newCache().probe(ctx, cxx, dir, nil, []string{"<vector>"}, []string{"std"})
	if err == nil || !strings.Contains(err.Error(), "fake++ was stopped: context deadline exceeded") {
		t.Errorf("probe = %v, want a time limit error", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Errorf("the probe took %v after its time limit of 300ms", d)
	}
	if files, err := os.ReadDir(tmp); err != nil || len(files) > 0 {
		t.Errorf("TMPDIR after the probe: %v, %v; want it empty", files, err)
	}
}

func TestProbeCommand(t *testing.T) {
	// What the compiler gets: the flags of its family, the probe file, its
	// folder, and its environment.
	body := `for a in "$@"; do printf '%s\n' "$a"; done > "$0.args"
printf '%s\n' "LC_ALL=$LC_ALL" "TMPDIR=$TMPDIR" "DEPENDENCIES_OUTPUT=${DEPENDENCIES_OUTPUT-unset}" "PWD=$(pwd)" > "$0.env"
exit 0`
	for _, c := range []struct {
		family    build.Family
		want, not []string
	}{
		{build.Clang, []string{"-w", "-ferror-limit=0", "-Qunused-arguments", "-fno-caret-diagnostics", "-fdiagnostics-show-note-include-stack"}, []string{"-fmax-errors=0"}},
		// g++: no -w, which would turn off the build's -Werror=narrowing (quietFlags).
		{build.GCC, []string{"-Wno-error", "-Werror=narrowing", "-Werror=conditionally-supported", "-fmax-errors=0", "-fno-diagnostics-show-caret",
			"-ftrack-macro-expansion=0"}, []string{"-w", "-Qunused-arguments", "-ferror-limit=0"}},
	} {
		t.Run(c.family.String(), func(t *testing.T) {
			// The folders first: t.TempDir uses TMPDIR too.
			cxx := fakeCompiler(t, c.family, body)
			tmp := t.TempDir()
			dir := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			t.Setenv("DEPENDENCIES_OUTPUT", filepath.Join(dir, "deps.d"))
			t.Setenv("LC_ALL", "de_DE.UTF-8")
			flags := []string{"-DGOCPP_FLAG", "-std=gnu++23", "-fmax-errors=1", "-Wfatal-errors", "-fdiagnostics-format=json",
				"-fno-diagnostics-show-caret", "-Werror", "-fmessage-length=20", "-ferror-limit=1"}
			kept := []string{"-DGOCPP_FLAG", "-std=gnu++23", "-Werror"} // the others only change the messages
			if _, err := newCache().probe(context.Background(), cxx, dir, flags, []string{"<vector>"}, []string{"std"}); err == nil {
				t.Fatal("probe with a silent compiler: no error")
			}
			data, err := os.ReadFile(cxx.Path + ".args")
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			for _, w := range append([]string{"-fsyntax-only", "-std=c++20", "-iquote", "-MD"}, c.want...) {
				if !slices.Contains(args, w) {
					t.Errorf("args %q: no %s", args, w)
				}
			}
			// The flags come after -std=c++20 and -iquote, which they can
			// change, and before the probe's own flags, without those that
			// change only the messages.
			if i := slices.Index(args, "-DGOCPP_FLAG"); i < 0 || i+len(kept) > len(args) || !slices.Equal(args[i:i+len(kept)], kept) ||
				i < slices.Index(args, "-std=c++20") || i < slices.Index(args, "-iquote") || i > slices.Index(args, "-fsyntax-only") {
				t.Errorf("args %q: want the flags %q after -std=c++20 and -iquote and before -fsyntax-only", args, kept)
			}
			for _, f := range []string{"-fmax-errors=1", "-Wfatal-errors", "-fdiagnostics-format=json", "-fmessage-length=20", "-ferror-limit=1"} {
				if slices.Contains(args, f) {
					t.Errorf("args %q: %s is there", args, f)
				}
			}
			for _, n := range c.not {
				if slices.Contains(args, n) {
					t.Errorf("args %q: %s is for the other family", args, n)
				}
			}
			if i := slices.Index(args, "-iquote"); i < 0 || i+1 >= len(args) || args[i+1] != dir {
				t.Errorf("args %q: want -iquote %s", args, dir)
			}
			src := args[len(args)-1]
			if filepath.Base(src) != "gp_probe.cpp" || !strings.HasPrefix(src, tmp+string(filepath.Separator)) {
				t.Errorf("the probe file is %s, want gp_probe.cpp in a folder in %s", src, tmp)
			}
			data, err = os.ReadFile(cxx.Path + ".env")
			if err != nil {
				t.Fatal(err)
			}
			env := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			wantEnv := []string{"LC_ALL=C", "TMPDIR=" + filepath.Dir(src), "DEPENDENCIES_OUTPUT=unset", "PWD=" + dir}
			if !slices.Equal(env, wantEnv) {
				t.Errorf("environment %q, want %q", env, wantEnv)
			}
			if files, err := os.ReadDir(tmp); err != nil || len(files) > 0 {
				t.Errorf("TMPDIR after the probe: %v, %v; want it empty", files, err)
			}

			// The macro probe: the same, with -E -dM.
			if _, err := newCache().probeMacros(context.Background(), cxx, dir, flags, []string{"<vector>"}); err == nil || !strings.Contains(err.Error(), "macro probe: fake++: it printed no macros") {
				t.Fatalf("macro probe with a silent compiler: %v", err)
			}
			if data, err = os.ReadFile(cxx.Path + ".args"); err != nil {
				t.Fatal(err)
			}
			margs := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			for i, a := range margs {
				if a == "-E" {
					margs[i] = "-fsyntax-only"
				}
			}
			margs = slices.DeleteFunc(margs, func(a string) bool { return a == "-dM" })
			// Only the folder of the source differs.
			margs[len(margs)-1], margs[slices.Index(margs, "-MF")+1] = args[len(args)-1], args[slices.Index(args, "-MF")+1]
			if !slices.Equal(margs, args) {
				t.Errorf("macro probe args %q, want those of the kind probe with -E -dM: %q", margs, args)
			}
			if files, err := os.ReadDir(tmp); err != nil || len(files) > 0 {
				t.Errorf("TMPDIR after the macro probe: %v, %v; want it empty", files, err)
			}
		})
	}
}

func TestParseMacros(t *testing.T) {
	out := "#define A 1\n#define F(x) x\n#define E\n#define G (1)\r\n#define V(...) __VA_ARGS__\n#define ñ 2\n\n"
	ms, err := parseMacros([]byte(out))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]MacroKind{"A": ObjectLike, "F": FunctionLike, "E": ObjectLike, "G": ObjectLike, "V": FunctionLike, "ñ": ObjectLike}
	for name, k := range want {
		if got := ms.Kind(name); got != k {
			t.Errorf("Kind(%q) = %v, want %v", name, got, k)
		}
	}
	if ms.Len() != len(want) || !slices.Equal(ms.Names(), []string{"A", "E", "F", "G", "V", "ñ"}) {
		t.Errorf("Names() = %q", ms.Names())
	}
	for _, bad := range []string{"", "\n", "int x;\n", "#define\n", "#define 1x 2\n", "#define (x) x\n", "# define A 1\n"} {
		if _, err := parseMacros([]byte(bad)); err == nil {
			t.Errorf("parseMacros(%q): no error", bad)
		}
	}
	var zero Macros
	if zero.Kind("A") != NotMacro || zero.Len() != 0 || len(zero.Names()) != 0 {
		t.Error("the zero Macros has macros")
	}
	for k, want := range map[MacroKind]string{NotMacro: "NotMacro", ObjectLike: "ObjectLike", FunctionLike: "FunctionLike", 3: "MacroKind(3)"} {
		if got := k.String(); got != want {
			t.Errorf("MacroKind(%d).String() = %q, want %q", int(k), got, want)
		}
	}
}

func TestMacroProbeFakeCompilers(t *testing.T) {
	for _, c := range []struct {
		name, body string
		err        string
	}{
		{"silent", "exit 0", "macro probe: fake++: it printed no macros"},
		{"not a macro list", "echo 'int x;'", `macro probe: fake++: unexpected line in the macro list: "int x;"`},
		{"failed", `echo "gp_macros.cpp:1:10: fatal error: x.h: No such file or directory" >&2; exit 1`,
			"macro probe: fake++ failed (exit status 1); its output: gp_macros.cpp:1:10: fatal error: x.h: No such file or directory"},
		{"crash", crashBody(), "macro probe: fake++: signal: segmentation fault"},
		{"too many macros", `head -c 70000000 /dev/zero`, "macro probe: fake++ printed more than 64 MiB of macros"},
		{"too many messages", `printf '%9000000s\n' x >&2`, "macro probe: fake++ printed more than 8 MiB of messages"},
		{"no dependency file", "echo '#define A 1'", "macro probe: fake++ wrote no dependency file"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cxx := fakeCompiler(t, build.GCC, c.body)
			ms, err := newCache().probeMacros(context.Background(), cxx, t.TempDir(), nil, []string{"<vector>"})
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Fatalf("macro probe = %d macros, %v; want an error with %q", ms.Len(), err, c.err)
			}
			if ms.Len() != 0 {
				t.Errorf("macro probe with an error = %d macros, want none", ms.Len())
			}
		})
	}
	// A failure is not cached.
	cxx := fakeCompiler(t, build.GCC, "exit 0")
	c := newCache()
	for range 2 {
		if _, err := c.probeMacros(context.Background(), cxx, "", nil, []string{"<vector>"}); err == nil {
			t.Fatal("no error")
		}
	}
	if n := c.macroRuns.Load(); n != 2 {
		t.Errorf("%d compiler runs, want 2", n)
	}
	if _, err := c.probeMacros(context.Background(), build.CXX{Path: cxx.Path}, "", nil, nil); err == nil || !strings.Contains(err.Error(), "macro probe: unknown family") {
		t.Errorf("a compiler of no family: %v", err)
	}
}

func TestProbeFlags(t *testing.T) {
	in := []string{"-I/x", "-fdiagnostics-color=always", "-fno-diagnostics-show-caret", "-Wfatal-errors", "-Wfatal-errors=x",
		"-fmax-errors=3", "-ferror-limit=3", "-fmessage-length=40", "-fcolor-diagnostics", "-fno-show-column",
		"-fdiagnostics-format=json", "-fdiagnostics-print-source-range-info", "-ftemplate-backtrace-limit=0",
		"-Wall", "-Werror", "-Werror=unused", "-DX=-fmax-errors=1", "-fshow-columnx", "-std=gnu++23", "-fno-exceptions"}
	want := []string{"-I/x", "-Wall", "-Werror", "-Werror=unused", "-DX=-fmax-errors=1", "-fshow-columnx", "-std=gnu++23", "-fno-exceptions"}
	if got := probeFlags(in); !slices.Equal(got, want) {
		t.Errorf("probeFlags = %q, want %q", got, want)
	}
	if in[1] != "-fdiagnostics-color=always" {
		t.Error("probeFlags changed its argument")
	}
}
