package interop

import (
	"context"
	"errors"
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
	pf := makeProbe([]string{"<vector>", `"./x.h"`}, [][]string{{"std"}, {"std", "filesystem"}})
	want := `#include <vector>
; template <class> struct gp_inc_0; using gp_inc_0_c = ::gp_inc_0<int>;
#include "./x.h"
; template <class> struct gp_inc_1; using gp_inc_1_c = ::gp_inc_1<int>;
#if defined(std)
#error gp_ns_macro
#else
namespace gp_ns_0 = ::std;
#endif
#if defined(std) || defined(filesystem)
#error gp_ns_macro
#else
namespace gp_ns_1 = ::std::filesystem;
#endif
namespace gp_probe_end = ::gp_probe_no_such_namespace;
namespace gp_probe_eof {}
`
	if got := string(pf.src); got != want {
		t.Errorf("makeProbe:\n%s\nwant:\n%s", got, want)
	}
	lines := strings.Split(want, "\n")
	if pf.firstBlock() != 5 || !strings.HasPrefix(lines[pf.firstBlock()-1], "#if defined(std)") {
		t.Errorf("firstBlock() = %d", pf.firstBlock())
	}
	if pf.canary() != 15 || !strings.HasPrefix(lines[pf.canary()-1], "namespace gp_probe_end") {
		t.Errorf("canary() = %d", pf.canary())
	}
}

// The probe file of the parser tests: two includes (lines 1 to 4), the
// candidates std (lines 5 to 9) and stdd (10 to 14), the canary (15) and
// the last line (16).
const testSrc = "/tmp/p/gp_probe.cpp"

var (
	testIncludes = []string{"<vector>", `"./bad.h"`}
	testProbe    = makeProbe(testIncludes, [][]string{{"std"}, {"stdd"}})
)

const canaryGCC = "/tmp/p/gp_probe.cpp:15:28: error: 'gp_probe_no_such_namespace' is not a namespace-name\n"

func TestReadOutput(t *testing.T) {
	for _, c := range []struct {
		name    string
		out     string
		kinds   []Kind
		incErrs []IncludeError
		err     string // a part of the error, if it must fail
	}{
		{name: "gcc",
			out:   "/tmp/p/gp_probe.cpp:13:23: error: 'stdd' is not a namespace-name; did you mean 'std'?\n" + canaryGCC,
			kinds: []Kind{Namespace, NotNamespace}},
		{name: "clang, notes and errors without a column",
			out: "/tmp/p/gp_probe.cpp:8:23: error: expected namespace name\n" +
				"In file included from /tmp/p/gp_probe.cpp:1:\n" +
				"/usr/include/c++/15/vector:111:11: note: namespace 'std' defined here\n" +
				"/tmp/p/gp_probe.cpp:13: error: expected namespace name\n" +
				"/tmp/p/gp_probe.cpp:15:28: error: expected namespace name\n" +
				"3 errors generated.\n",
			kinds: []Kind{NotNamespace, NotNamespace}},
		{name: "macro",
			out:   "/tmp/p/gp_probe.cpp:6:2: error: #error gp_ns_macro\n" + canaryGCC,
			kinds: []Kind{NotNamespace, Namespace}},
		{name: "CRLF",
			out:   strings.ReplaceAll("/tmp/p/gp_probe.cpp:13:23: error: x\n"+canaryGCC, "\n", "\r\n"),
			kinds: []Kind{Namespace, NotNamespace}},
		{name: "clang include stack, outermost first",
			out: "In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"In file included from /tmp/p/./inc_bad.h:2:\n" +
				"/tmp/p/./bad.h:2:19: error: expected ';' at end of declaration list\n" +
				"/tmp/p/./bad.h:2:21: error: expected ';' after struct\n" +
				"/tmp/p/gp_probe.cpp:13:23: error: expected namespace name\n" +
				"/tmp/p/gp_probe.cpp:15:28: error: expected namespace name\n",
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
				"/tmp/p/gp_probe.cpp:8:1: error: expected primary-expression before 'namespace'\n" + canaryGCC,
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, Kind: HeaderUnclosed, Msg: "the header ends inside a declaration or a block that it doesn't close: expected primary-expression before ';' token"}}},
		{name: "open brace",
			out: canaryGCC +
				"/tmp/p/gp_probe.cpp:16:26: error: expected '}' at end of input\n" +
				"In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h:2:12: note: to match this '{'\n",
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 2, Column: 12, Msg: "expected '}' at end of input (to match this '{')"}}},
		// gcc prints the include stack of an #include only once: the error
		// at the end of the file is in a header whose stack came with a note.
		{name: "gcc: no include stack the second time",
			out: "/tmp/p/gp_probe.cpp:13:23: error: reference to 'stdd' is ambiguous\n" +
				"In file included from /tmp/p/gp_probe.cpp:3:\n" +
				"/tmp/p/bad.h:2:29: note: candidates are: 'namespace a::stdd { }'\n" +
				canaryGCC +
				"/tmp/p/bad.h: In instantiation of 'int late_f() [with T = int]':\n" +
				"/tmp/p/bad.h:7:41:   required from here\n" +
				"/tmp/p/bad.h:6:45: error: 'nope' is not a member of 'int'\n",
			incErrs: []IncludeError{{Index: 1, Include: `"./bad.h"`, File: "/tmp/p/bad.h", Line: 6, Column: 45, Msg: "'nope' is not a member of 'int'"}}},
		{name: "end of the file without a note",
			out: canaryGCC + "/tmp/p/gp_probe.cpp:16:26: error: expected '}' at end of input\n",
			err: "unexpected error: gp_probe.cpp:16:26: error: expected '}' at end of input"},
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
			out:   "/tmp/p/gp_probe.cpp:8:1: warning: hmm\n" + canaryGCC,
			kinds: []Kind{Namespace, Namespace}},
		{name: "crash",
			out: "/tmp/p/gp_probe.cpp:8:1: internal compiler error: Segmentation fault\n" + canaryGCC,
			err: "unexpected error: gp_probe.cpp:8:1: internal compiler error: Segmentation fault"},
		{name: "stopped early",
			out: "/tmp/p/gp_probe.cpp:13:23: error: 'stdd' is not a namespace-name\n",
			err: "it didn't check the whole probe file (no error for its line 15); its output: /tmp/p/gp_probe.cpp:13:23: error: 'stdd' is not a namespace-name"},
		{name: "no output",
			err: "it didn't check the whole probe file (no error for its line 15); it printed nothing"},
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

func TestParseDiagnosticsWindowsPaths(t *testing.T) {
	src := `C:\Users\me\AppData\Local\Temp\gocpp-nsprobe-1\gp_probe.cpp`
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
		if _, err := c.probe(context.Background(), cxx, "", []string{"<vector>", inc}, []string{"std"}); err == nil || !strings.Contains(err.Error(), "bad include") {
			t.Errorf("include %q: %v, want a bad include error", inc, err)
		}
	}
	for _, cand := range []string{"", "::", "a::", "::a::::b", "std.vector", "1abc", "a b", "a;b", "a::1b", "a\nb", "a-b", ":a"} {
		if _, err := c.probe(context.Background(), cxx, "", []string{"<vector>"}, []string{"std", cand}); err == nil || !strings.Contains(err.Error(), "bad candidate") {
			t.Errorf("candidate %q: %v, want a bad candidate error", cand, err)
		}
	}
	for _, cand := range []string{"std", "::std", "std::filesystem", "::std::filesystem", "_x", "x1", "ñandú", "日本"} {
		if _, err := splitCandidate(cand); err != nil {
			t.Errorf("splitCandidate(%q): %v", cand, err)
		}
	}
	if n := c.runs.Load(); n != 0 {
		t.Errorf("%d compiler runs, want 0", n)
	}
	if _, err := c.probe(context.Background(), build.CXX{Path: cxx.Path}, "", []string{"<vector>"}, []string{"std"}); err == nil || !strings.Contains(err.Error(), "unknown family") {
		t.Errorf("a compiler of no family: %v, want an unknown family error", err)
	}
}

func TestProbeNoIncludes(t *testing.T) {
	// A .gp file with a namespace rename and no header import: nothing
	// runs (the compiler here doesn't exist), every candidate is Unknown,
	// and reserved names are NotNamespace.
	c := newCache()
	cxx := build.CXX{Path: filepath.Join(t.TempDir(), "no-such-c++"), Family: build.GCC, Version: "none"}
	res, err := c.probe(context.Background(), cxx, t.TempDir(), nil, []string{"::std::filesystem", "std", "gp_ns_0"})
	want := map[string]Kind{"::std::filesystem": Unknown, "std": Unknown, "gp_ns_0": NotNamespace}
	if err != nil || !reflect.DeepEqual(res.Kinds, want) || res.IncludeErrors != nil {
		t.Errorf("probe without includes = %+v, %v; want Kinds %v", res, err, want)
	}
	// No candidates either. (With includes, the probe runs also without
	// candidates or with only reserved ones, to check the includes:
	// TestProbeNoCandidates.)
	res, err = c.probe(context.Background(), cxx, t.TempDir(), nil, nil)
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
			run := &probeRun{deps: []string{path}, depsOK: true, start: time.Now()}
			if files, ok := dependsOn(run, "", nil); ok {
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
	for k, want := range map[Kind]string{Unknown: "Unknown", Namespace: "Namespace", NotNamespace: "NotNamespace", 7: "Kind(7)"} {
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
		{"silent", "exit 0", "fake++: it didn't check the whole probe file (no error for its line 8); it printed nothing"},
		{"driver error", `echo "fake++: error: unrecognized command-line option '-Qunused-arguments'" >&2; exit 1`,
			"namespace probe: fake++: unexpected error: fake++: error: unrecognized command-line option '-Qunused-arguments'"},
		{"too much output", `printf '%9000000s\n' x`, "namespace probe: fake++ printed more than 8 MiB"},
		{"crash", `kill -SEGV $$`, "namespace probe: fake++: signal: segmentation fault"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cxx := fakeCompiler(t, build.GCC, c.body)
			res, err := newCache().probe(context.Background(), cxx, t.TempDir(), []string{"<vector>"}, []string{"std"})
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
	// limit, not at the time limit.
	cxx := fakeCompiler(t, build.GCC, `while :; do printf '%1000s\n' x; done`)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	start := time.Now()
	_, err := newCache().probe(ctx, cxx, t.TempDir(), []string{"<vector>"}, []string{"std"})
	if err == nil || !strings.Contains(err.Error(), "printed more than 8 MiB") {
		t.Errorf("probe = %v, want an output limit error", err)
	}
	if d := time.Since(start); d > 25*time.Second { // well before the 30 s of ctx, also with -race
		t.Errorf("the probe took %v", d)
	}

	// When the output had include errors before the limit, they are the
	// result (the candidates are Unknown then anyway).
	cxx = fakeCompiler(t, build.GCC, `for a in "$@"; do f=$a; done
echo "$f:1:10: fatal error: x.h: No such file or directory"
while :; do printf '%1000s\n' x; done`)
	res, err := newCache().probe(ctx, cxx, t.TempDir(), []string{"<x.h>"}, []string{"std"})
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
	_, err := newCache().probe(ctx, cxx, dir, []string{"<vector>"}, []string{"std"})
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
		{build.Clang, []string{"-ferror-limit=0", "-Qunused-arguments", "-fno-caret-diagnostics", "-fdiagnostics-show-note-include-stack"}, []string{"-fmax-errors=0"}},
		{build.GCC, []string{"-fmax-errors=0", "-fno-diagnostics-show-caret", "-ftrack-macro-expansion=0"}, []string{"-Qunused-arguments", "-ferror-limit=0"}},
	} {
		t.Run(c.family.String(), func(t *testing.T) {
			// The folders first: t.TempDir uses TMPDIR too.
			cxx := fakeCompiler(t, c.family, body)
			tmp := t.TempDir()
			dir := t.TempDir()
			t.Setenv("TMPDIR", tmp)
			t.Setenv("DEPENDENCIES_OUTPUT", filepath.Join(dir, "deps.d"))
			t.Setenv("LC_ALL", "de_DE.UTF-8")
			if _, err := newCache().probe(context.Background(), cxx, dir, []string{"<vector>"}, []string{"std"}); err == nil {
				t.Fatal("probe with a silent compiler: no error")
			}
			data, err := os.ReadFile(cxx.Path + ".args")
			if err != nil {
				t.Fatal(err)
			}
			args := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
			for _, w := range append([]string{"-fsyntax-only", "-std=c++20", "-w", "-iquote", "-MMD"}, c.want...) {
				if !slices.Contains(args, w) {
					t.Errorf("args %q: no %s", args, w)
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
		})
	}
}
