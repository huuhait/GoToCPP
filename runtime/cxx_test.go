package rtfiles_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The C++ tests of the runtime: test/unit.cpp, test/slice_test.cpp, test/array_test.cpp,
// test/map_test.cpp, test/fmt_test.cpp, test/trace_test.cpp, test/defer_test.cpp, test/func_test.cpp,
// test/thread_test.cpp, test/goroutine_test.cpp, test/chan_test.cpp, test/select_test.cpp, test/time_test.cpp and test/interop_test.cpp, test/cxxvalue_test.cpp, test/unsafe_test.cpp,
// test/type_test.cpp, test/iface_test.cpp and test/reflect_test.cpp (unit tests), test/nochecks_test.cpp
// (the unit tests of a -nochecks build, compiled with the runtime with -DGOCPP_NOCHECKS),
// and test/diff.cpp (the C++ side of a differential test against testdata/diff/main.go, run with
// real Go). (fmt is compared with real Go by the test programs tests/run/fmt_*.gp.)
// Each is compiled with the embedded runtime (rtfiles.FS, extracted to a temporary folder, as
// the driver will do it), with g++ and with clang++, in each mode below.
//
// Like the test harness (tests/harness_test.go): GOCPP_CXX=<compiler> tests with that
// compiler only; a compiler that isn't installed is a skip, unless GOCPP_REQUIRE_CXX (a comma
// list, "g++,clang++" in make ci) names it, which makes it a failure.

// cmdTimeout bounds each compile and each run. It catches a hung command, not a
// slow one: an -O2 compile of unit.cpp took over 3 minutes inside a loaded
// make ci (load 30), so the bound is generous; make test's -timeout 30m bounds
// the whole package.
const cmdTimeout = 10 * time.Minute

// baseFlags are the driver's flags (Phase 02 §2.7), plus warnings as errors for the runtime's
// own code.
var baseFlags = []string{"-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-fstack-clash-protection", "-g",
	"-Wall", "-Wextra", "-Werror"}

// The modes: the driver's debug build, an optimized build, and the sanitizers. With
// -fno-sanitize-recover=all, a sanitizer report stops the program with a nonzero exit code.
// The sanitizer mode also writes DWARF 4, so the stack trace's line table reader is tested with
// version 4 there and with the compilers' default, version 5, in the others.
var modes = []struct {
	name  string
	flags []string
}{
	{"O0", []string{"-O0"}},
	{"O2", []string{"-O2"}},
	{"asan", []string{"-O0", "-fsanitize=address,undefined", "-fno-omit-frame-pointer", "-fno-sanitize-recover=all", "-gdwarf-4"}},
}

// A cxx is one C++ compiler of the test matrix.
type cxx struct {
	name    string    // the subtest name: "g++", "clang++"
	cc      build.CXX // the compiler, as build.LookCXX found it
	missing string    // why it can't be used, or ""
}

var (
	cxxOnce sync.Once
	cxxList []cxx
	cxxErr  error
)

// compilers returns the compilers to test with, from GOCPP_CXX and GOCPP_REQUIRE_CXX.
func compilers(t *testing.T) []cxx {
	t.Helper()
	cxxOnce.Do(func() { cxxList, cxxErr = findCompilers(os.Getenv("GOCPP_CXX"), os.Getenv("GOCPP_REQUIRE_CXX")) })
	if cxxErr != nil {
		t.Fatal(cxxErr)
	}
	return cxxList
}

func findCompilers(env, required string) ([]cxx, error) {
	names := []string{"g++", "clang++"}
	if env != "" {
		names = []string{env}
	}
	var list []cxx
	for _, n := range names {
		c := cxx{name: filepath.Base(n)}
		cc, err := build.LookCXX(context.Background(), n)
		if err != nil {
			c.missing = err.Error()
		}
		c.cc = cc
		list = append(list, c)
	}
	for _, want := range strings.Split(required, ",") {
		want = strings.TrimSpace(want)
		if want == "" {
			continue
		}
		i := slices.IndexFunc(list, func(c cxx) bool { return c.name == want || (env != "" && env == want) })
		switch {
		case i < 0 && env != "":
			return nil, fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, but GOCPP_CXX=%s tests only that compiler", required, want, env)
		case i < 0:
			return nil, fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, which the tests don't use: they use g++ and clang++, or the compiler in GOCPP_CXX", required, want)
		case list[i].missing != "":
			return nil, fmt.Errorf("GOCPP_REQUIRE_CXX=%s needs %s, but %s", required, want, list[i].missing)
		}
	}
	return list, nil
}

// command runs a command with a timeout, in the environment env (nil: the test's own). combined merges stdout and stderr into one stream
// (one pipe, so the order of the writes is kept); otherwise stdout and stderr are separate.
func command(dir string, env []string, combined bool, name string, args ...string) (stdout, stderr []byte, code int, err error) {
	return commandPrio(prioRun, dir, env, combined, name, args...)
}

// commandPrio is command with a priority for the gate: at most jobs() commands run at the same
// time (parallel_test.go), and the highest priority goes first.
func commandPrio(prio int, dir string, env []string, combined bool, name string, args ...string) (stdout, stderr []byte, code int, err error) {
	gate.acquire(prio)
	defer gate.release()
	ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = env // nil: the test's own environment
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	if combined {
		cmd.Stderr = &out
	} else {
		cmd.Stderr = &errb
	}
	err = cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.Exited() {
		return out.Bytes(), errb.Bytes(), exit.ExitCode(), nil
	}
	if err != nil {
		return out.Bytes(), errb.Bytes(), -1, fmt.Errorf("%s: %v\n%s%s", name, err, out.Bytes(), errb.Bytes())
	}
	return out.Bytes(), errb.Bytes(), 0, nil
}

func (c cxx) flags(more []string) []string {
	args := slices.Concat(baseFlags, more)
	if c.cc.Family == build.Clang {
		args = append(args, "-Qunused-arguments")
	}
	return args
}

// sourcePath returns the absolute path of the file p of this folder. It reads the file
// first: go test caches test results and notices a changed file only when the test itself
// reads it, not when a compiler that the test runs does.
func sourcePath(t *testing.T, p string) string {
	t.Helper()
	if _, err := os.ReadFile(p); err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

// The cases of the differential test (the case names of testdata/diff/main.go).
var diffCases = []string{"println", "fmt", "floats", "arith", "f2i", "strings", "order",
	"panic-str", "panic-multi", "panic-empty", "panic-int", "panic-float", "panic-bool",
	"complex", "panic-complex", "panic-complex64",
	"div", "rem", "shl", "shr", "index", "negindex", "uindex", "exit0", "big", "bigerr",
	"ptr", "nil-deref", "nil-store", "nil-field", "nil-method", "nil-addr",
	"substr", "utf8", "substr-alen", "substr-b", "substr-neg", "substr-from"}

// The slice cases (Phase 03 step 3.5a): the sliceCase function of testdata/diff/main.go.
var sliceDiffCases = []string{"slices", "slice-grow", "slice-free", "slice-index", "slice-index-neg",
	"slice-index-u8", "slice-index-nil", "slice-hi", "slice-hi-neg", "slice-hi-u", "slice-lo",
	"slice-lo-neg", "slice-from", "slice-nil-sub", "slice3-max", "slice3-max-neg", "slice3-max-u",
	"slice3-hi", "slice3-hi-neg", "slice3-lo", "slice3-lo-neg", "make-len", "make-len-big",
	"make-cap", "make-cap-big", "slice-convert"}

// The array cases (Phase 03 step 3.6a): the arrayCase function of testdata/diff/main.go.
var arrayDiffCases = []string{"arrays", "array-index", "array-index-neg", "array-index-u8",
	"array-index0", "array-convert", "array-convert-ptr", "array-convert-nil", "array-slice-hi",
	"array-slice-lo", "array-slice3-max", "array-slice-nil", "array-slice-nil0"}

// The map cases (Phase 03 step 3.7a): the mapCase function of testdata/diff/main.go.
var mapDiffCases = []string{"maps", "map-floats", "map-nil", "map-nil-ref", "map-nil-inner"}

func init() {
	diffCases = append(diffCases, sliceDiffCases...)
	diffCases = append(diffCases, arrayDiffCases...)
	diffCases = append(diffCases, mapDiffCases...)
}

// A result is what a program printed (stdout and stderr merged) and its exit code.
type result struct {
	out  []byte
	code int
}

// goStackMark is the line after which Go prints the stack trace of a panic. gocpp prints
// that line too, then a trace of its own (gocpp/trace.hpp), which differs from Go's (other
// paths, offsets and runtime frames), so both outputs are compared up to it (cutTrace).
const goStackMark = "goroutine 1 [running]:\n"

// cutTrace returns out up to the end of the goStackMark line, or out when there is none.
func cutTrace(out []byte) []byte {
	if i := bytes.Index(out, []byte(goStackMark)); i >= 0 {
		return out[:i+len(goStackMark)]
	}
	return out
}

// pcRx matches the pc in the signal line of a nil pointer panic, "[signal SIGSEGV:
// segmentation violation code=0x1 addr=0x0 pc=0x4a2b1c]": an address in the program, which
// differs between Go and gocpp (and between builds). maskPC replaces it with "pc=PC".
var pcRx = regexp.MustCompile(`( pc=)0x[0-9a-f]+\]\n`)

func maskPC(out []byte) []byte {
	return pcRx.ReplaceAll(out, []byte("${1}PC]\n"))
}

// goResults runs testdata/diff/main.go with real Go, once for each case, and returns the
// results and the Go binary. It returns nil and "" (and logs why) when there is no go command.
func goResults(t *testing.T) (map[string]result, string) {
	t.Helper()
	return goProgramResults(t, "testdata/diff/main.go", diffCases)
}

// goProgramResults builds the Go program src, runs it once for each case, and returns the
// results and the binary, or nil and "" when there is no go command.
func goProgramResults(t *testing.T, src string, cases []string) (map[string]result, string) {
	t.Helper()
	gocmd, err := exec.LookPath("go")
	if err != nil {
		t.Logf("no go command, so the differential tests are skipped: %v", err)
		return nil, ""
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "godiff")
	abs := sourcePath(t, src)
	// GOAMD64=v1: gc fuses a*b + c from v3 on, and gocpp gives the results without fusing.
	goEnv := append(os.Environ(), "GOAMD64=v1")
	out, _, code, err := command(".", goEnv, true, gocmd, "build", "-o", bin, abs)
	if err != nil || code != 0 {
		t.Fatalf("go build %s: exit %d %v\n%s", src, code, err, out)
	}
	// Go's runtime print (println) writes with one write call and drops what a short write
	// leaves. A blocking pipe write ends short when a signal arrives in the middle, and the
	// Go runtime sends itself SIGURG to preempt goroutines, so under load "bigerr" lost the
	// end of its 1 MiB line in real Go (the gocpp side loops until all is written). Without
	// asynchronous preemption there is no SIGURG, and Go's output is complete.
	env := append(os.Environ(), "GODEBUG=asyncpreemptoff=1")
	res := make(map[string]result)
	for _, name := range cases {
		out, _, code, err := command(dir, env, true, bin, name)
		if err != nil {
			t.Fatal(err)
		}
		res[name] = result{maskPC(cutTrace(out)), code}
	}
	return res, bin
}

// TestCXX compiles and runs the C++ unit tests and the differential test with each compiler
// in each mode.
func TestCXX(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	want, goBin := goResults(t)
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			for _, m := range modes {
				t.Run(m.name, func(t *testing.T) {
					t.Parallel()
					// All the programs are built at the same time, from the runtime's objects, which
					// are built once for the mode (parallel_test.go). The checks run at the same time
					// too, each one as soon as its program is built; each runs its own processes.
					bins := c.startBuilds(t, rt, t.TempDir(), m.flags, []prog{
						{src: "test/map_test.cpp", weight: 24},
						{src: "test/diff.cpp", weight: 15},
						{src: "test/unit.cpp", weight: 50},
						{src: "test/fmt_test.cpp", weight: 14},
						{src: "test/interop_test.cpp", weight: 15},
						{src: "test/array_test.cpp", weight: 7},
						{src: "test/slice_test.cpp", weight: 7},
						{src: "test/cxxvalue_test.cpp", weight: 7},
						{src: "test/unsafe_test.cpp", weight: 7},
						{src: "test/trace_test.cpp", weight: 3},
						{src: "test/defer_test.cpp", weight: 3},
						{src: "test/func_test.cpp", weight: 3},
						{src: "test/type_test.cpp", weight: 4},
						{src: "test/iface_test.cpp", weight: 7},
						{src: "test/ifaceconv_test.cpp", weight: 4},
						{src: "test/errors_test.cpp", weight: 4},
						{src: "test/reflect_test.cpp", weight: 5},
						{src: "test/math_test.cpp", weight: 3},
						{src: "test/thread_test.cpp", weight: 3},
						{src: "test/goroutine_test.cpp", weight: 3},
						{src: "test/runtimepkg_test.cpp", weight: 3},
						{src: "test/chan_test.cpp", weight: 4},
						{src: "test/select_test.cpp", weight: 4},
						{src: "test/time_test.cpp", weight: 4},
						{src: "test/nochecks_test.cpp", flags: []string{"-DGOCPP_NOCHECKS"}, weight: 3},
					})

					t.Run("unit", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/unit.cpp"))
					})
					t.Run("slice", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/slice_test.cpp"))
					})
					t.Run("array", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/array_test.cpp"))
					})
					t.Run("map", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/map_test.cpp"))
					})
					t.Run("trace", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/trace_test.cpp"))
					})
					t.Run("defer", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/defer_test.cpp"))
					})
					t.Run("func", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/func_test.cpp"))
					})
					t.Run("type", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/type_test.cpp"))
					})
					t.Run("iface", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/iface_test.cpp"))
						runIface(t, bins.bin(t, "test/iface_test.cpp"))
					})
					t.Run("ifaceconv", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/ifaceconv_test.cpp"))
					})
					t.Run("errors", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/errors_test.cpp"))
					})
					t.Run("reflect", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/reflect_test.cpp"))
					})
					t.Run("math", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/math_test.cpp"))
					})
					t.Run("thread", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/thread_test.cpp")
						runUnit(t, bin)
						runThread(t, bin, m.name)
					})
					t.Run("goroutine", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/goroutine_test.cpp")
						runUnit(t, bin)
						runGoroutine(t, bin, m.name)
					})
					t.Run("runtimepkg", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/runtimepkg_test.cpp")
						runUnit(t, bin)
						runRuntimePkg(t, bin, m.name == "asan")
					})
					t.Run("chan", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/chan_test.cpp")
						runUnit(t, bin)
						runChan(t, bin)
					})
					t.Run("select", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/select_test.cpp")
						runUnit(t, bin)
						runSelect(t, bin)
					})
					t.Run("time", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/time_test.cpp"))
					})
					t.Run("interop", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/interop_test.cpp")
						runUnit(t, bin)
						runInterop(t, bin, m.name == "asan")
					})
					t.Run("cxxvalue", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/cxxvalue_test.cpp")
						runUnit(t, bin)
						runCxxValue(t, bin, m.name != "O2", m.name == "asan")
					})
					t.Run("nochecks", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/nochecks_test.cpp"))
					})
					t.Run("fmt", func(t *testing.T) {
						t.Parallel()
						runUnit(t, bins.bin(t, "test/fmt_test.cpp"))
					})
					t.Run("unsafe", func(t *testing.T) {
						t.Parallel()
						bin := bins.bin(t, "test/unsafe_test.cpp")
						runUnit(t, bin)
						runUnsafe(t, bin)
					})
					t.Run("panic", func(t *testing.T) {
						t.Parallel()
						runPanic(t, bins.bin(t, "test/diff.cpp"), m.name != "O2")
					})
					t.Run("leak", func(t *testing.T) {
						t.Parallel()
						runLeak(t, bins.bin(t, "test/diff.cpp"), m.name == "asan")
					})
					t.Run("pipes", func(t *testing.T) {
						t.Parallel()
						runPipes(t, bins.bin(t, "test/diff.cpp"), goBin)
					})
					t.Run("diff", func(t *testing.T) {
						if want == nil {
							t.Skip("no go command")
						}
						t.Parallel()
						diff := bins.bin(t, "test/diff.cpp")
						for _, name := range diffCases {
							t.Run(name, func(t *testing.T) {
								t.Parallel()
								if name == "f2i" && runtime.GOARCH != "amd64" {
									t.Skipf("gocpp copies Go's float → int conversions on amd64; this Go is %s", runtime.GOARCH)
								}
								if name == "complex" && runtime.GOARCH != "amd64" {
									t.Skipf("gocpp copies Go's complex * and / on amd64 (other Go ports fuse a*b + c); this Go is %s", runtime.GOARCH)
								}
								runDiff(t, diff, name, want[name])
							})
						}
					})
				})
			}
		})
	}
}

func runUnit(t *testing.T, bin string) {
	stdout, stderr, code, err := command(filepath.Dir(bin), append(os.Environ(), "GOCPP_RT_TEST=env-ok"), false, bin, "arg1", "arg two")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || len(stderr) != 0 || !bytes.HasPrefix(stdout, []byte("ok: ")) {
		t.Fatalf("test/unit.cpp: exit code %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
	t.Logf("%s", bytes.TrimSpace(stdout))
}

// nilSignal is the signal line of a nil pointer panic on this OS, after maskPC. Go's addr is
// the faulting address: the field's offset for an access (read or write) to a field at a
// small nonzero offset (below 4096), and 0x0 for offset 0, for 4096 or more and for a whole
// value. gocpp checks the pointer before any access and always prints 0x0, so the diff cases
// use only accesses where Go's is 0x0 too.
var nilSignal = func() string {
	if runtime.GOOS == "windows" {
		return "[signal 0xc0000005 code=0x0 addr=0x0 pc=PC]"
	}
	return "[signal SIGSEGV: segmentation violation code=0x1 addr=0x0 pc=PC]"
}()

// runPanic checks the end of a program without real Go: an unrecovered panic prints Go's
// message and exits with 2, after what the program printed before; a normal end exits with 0.
// The output is compared up to the goroutine line; after it, the stack trace must name the
// function of diff.cpp that panicked, with its line (exact only when frames says that the
// build keeps every frame, not at -O2).
func runPanic(t *testing.T, bin string, frames bool) {
	diffCPP := sourcePath(t, "test/diff.cpp")
	src, err := os.ReadFile(diffCPP)
	if err != nil {
		t.Fatal(err)
	}
	// lineOf is the line of diff.cpp that has text.
	lineOf := func(text string) int {
		t.Helper()
		i := strings.Index(string(src), text)
		if i < 0 {
			t.Fatalf("test/diff.cpp has no %q", text)
		}
		return strings.Count(string(src[:i]), "\n") + 1
	}
	for _, tc := range []struct {
		name, trace string
	}{
		// The panicking line is 3 lines below the case's own line.
		{"index", "main.main()\n\t" + diffCPP + ":" + fmt.Sprint(lineOf(`(c == "index")`)+3) + " +0x"},
		{"slice-index", "main.slice_case(...)\n\t" + diffCPP + ":" + fmt.Sprint(lineOf(`(c == "slice-index")`)+3) + " +0x"},
		{"panic-str", "main.main()\n\t" + diffCPP + ":" + fmt.Sprint(lineOf(`(c == "panic-str")`)+2) + " +0x"},
	} {
		out, _, code, err := command(filepath.Dir(bin), nil, true, bin, tc.name)
		if err != nil {
			t.Fatal(err)
		}
		trace := string(out[len(cutTrace(out)):])
		want := tc.trace
		if !frames {
			want = tc.trace[:strings.Index(tc.trace, "(")] // the function's name only
		}
		if code != 2 || !strings.HasPrefix(trace, want) || strings.Contains(trace, "gp::") {
			t.Errorf("%s: exit code %d, the trace:\n%s\nwant exit code 2 and a trace that starts with\n%s", tc.name, code, trace, want)
		}
	}
	for _, tc := range []struct {
		name, out string
		code      int
	}{
		{"panic-str", "before\npanic: boom\n\ngoroutine 1 [running]:\n", 2},
		{"panic-multi", "panic: a\n\tb\n\tc\n\ngoroutine 1 [running]:\n", 2},
		{"div", "panic: runtime error: integer divide by zero\n\ngoroutine 1 [running]:\n", 2},
		{"shl", "panic: runtime error: negative shift amount\n\ngoroutine 1 [running]:\n", 2},
		{"index", "panic: runtime error: index out of range [5] with length 3\n\ngoroutine 1 [running]:\n", 2},
		{"nil-field", "panic: runtime error: invalid memory address or nil pointer dereference\n" + nilSignal + "\n\ngoroutine 1 [running]:\n", 2},
		{"nil-store", "panic: runtime error: invalid memory address or nil pointer dereference\n" + nilSignal + "\n\ngoroutine 1 [running]:\n", 2},
		{"order", "a\nb\nc\nd\ne1\nno newline, then stderr\npanic: p\n\ngoroutine 1 [running]:\n", 2},
		{"exit0", "only stdout\n", 0},
		{"bad-owner", "fatal error: gocpp: a non-nil pointer without an owner (a bug in the runtime or in the generated code)\n", 2},
	} {
		out, _, code, err := command(filepath.Dir(bin), nil, true, bin, tc.name)
		if err != nil {
			t.Fatal(err)
		}
		out = maskPC(cutTrace(out))
		if code != tc.code || string(out) != tc.out {
			t.Errorf("%s: exit code %d, output:\n%s\nwant exit code %d, output:\n%s", tc.name, code, out, tc.code, tc.out)
		}
	}
}

// runInterop runs the cases of test/interop_test.cpp that end the program. The expected
// output is what a cgo program printed with Go 1.25 (C.GoStringN(p, -1) is a fatal error, C.GoBytes
// with a negative length a runtime error panic, a nil pointer with a length a nil dereference),
// up to the goroutine line. The case "dangle" (std::minmax's references without gp::values)
// runs only with ASan, which must report it: it shows that the gp::values test with
// temporaries would catch a dangling reference.
func runInterop(t *testing.T, bin string, asan bool) {
	const nilPanic = "panic: runtime error: invalid memory address or nil pointer dereference\n"
	for _, tc := range []struct {
		name, out string
	}{
		{"strn-neg", "fatal error: out of memory\n\ngoroutine 1 [running]:\n"},
		{"strn-nil", nilPanic + nilSignal + "\n\ngoroutine 1 [running]:\n"},
		{"bytes-neg", "panic: runtime error: gobytes: length out of range\n\ngoroutine 1 [running]:\n"},
		{"bytes-nil", nilPanic + nilSignal + "\n\ngoroutine 1 [running]:\n"},
		{"deref-nil", nilPanic + nilSignal + "\n\ngoroutine 1 [running]:\n"},
		{"slot-unset", "fatal error: a result of a type without a default constructor was never set\n\ngoroutine 1 [running]:\n"},
		// A callback that C calls (step 4.11): the panic, the C++ exception and the pointer to
		// gocpp memory end the program at the callback; the "C" frame is not unwound ("UNWOUND"
		// is what its destructor would print).
		{"callback-panic", "panic: stop\n\ngoroutine 1 [running]:\n"},
		{"callback-exception", "panic: C++ exception std::runtime_error: stop\n\ngoroutine 1 [running]:\n"},
		{"callback-owned", "fatal error: gocpp: a callback returned a pointer to gocpp memory to C: return a pointer from C (malloc), a package variable's, or the pointer that C passed\n\ngoroutine 1 [running]:\n"},
	} {
		out, _, code, err := command(filepath.Dir(bin), nil, true, bin, tc.name)
		if err != nil {
			t.Fatal(err)
		}
		out = maskPC(cutTrace(out))
		if code != 2 || string(out) != tc.out {
			t.Errorf("%s: exit code %d, output:\n%s\nwant exit code 2, output:\n%s", tc.name, code, out, tc.out)
		}
	}
	if !asan {
		return
	}
	out, _, code, err := command(filepath.Dir(bin), nil, true, bin, "dangle")
	if err != nil {
		t.Fatal(err)
	}
	if code == 0 || !bytes.Contains(out, []byte("ERROR: AddressSanitizer: stack-use-after-scope")) {
		t.Errorf("dangle: exit code %d, output:\n%s\nwant an AddressSanitizer report (stack-use-after-scope) and a nonzero exit code", code, out)
	}
}

// runIface runs the cases of test/iface_test.cpp that end the program (Phase 05 steps 5.2 and
// 5.3): the fatal errors of the runtime that only a bug in the emitter can reach, each with its
// message and exit code 2, up to the goroutine line.
func runIface(t *testing.T, bin string) {
	const sorted = " are not sorted (exported names first, then by name and package: gocpp/type.hpp)"
	for _, tc := range []struct{ name, msg string }{
		{"unsorted-type", "the methods of main.Unsorted" + sorted},
		{"unsorted-iface", "the methods of an interface type" + sorted},
		{"unsorted-iface-desc", "the methods of main.BadIface" + sorted},
		{"bad-conversion", "main.Rect does not implement main.Shape: missing method Scale"},
		{"bad-unbox", "unbox of the nil interface"},
		{"interface-name", "the interface type interface { Z1() } is also given the name interface { Z1(); } (every site must pass the same name)"},
		{"getitab-concrete", "getitab needs an interface type and a concrete type"},
	} {
		out, _, code, err := command(filepath.Dir(bin), nil, true, bin, tc.name)
		if err != nil {
			t.Fatal(err)
		}
		want := "fatal error: gocpp: internal error: " + tc.msg + "\n\n" + goStackMark
		if got := cutTrace(out); code != 2 || string(got) != want {
			t.Errorf("%s: exit code %d, output:\n%s\nwant exit code 2, output:\n%s", tc.name, code, got, want)
		}
	}
}

// runUnsafe runs the cases of test/unsafe_test.cpp that end the program (step 4.10): Go's panics
// of unsafe.Slice and unsafe.String, and the checkptr fatal error of a misaligned (*T)(p).
func runUnsafe(t *testing.T, bin string) {
	const trace = "\n\ngoroutine 1 [running]:\n"
	for _, tc := range []struct {
		name, out string
	}{
		{"misaligned", "fatal error: checkptr: misaligned pointer conversion" + trace},
		{"misaligned-c", "fatal error: checkptr: misaligned pointer conversion" + trace},
		{"slice-len", "panic: runtime error: unsafe.Slice: len out of range" + trace},
		{"string-nil", "panic: runtime error: unsafe.String: ptr is nil and len is not zero" + trace},
		{"store-keep-unowned", "fatal error: cannot keep the object alive: the pointer is stored into C memory (a struct that came from C), which can't hold a gocpp pointer: allocate the struct with &T{...}, or keep the object in a variable for as long as C uses it" + trace},
	} {
		out, _, code, err := command(filepath.Dir(bin), nil, true, bin, tc.name)
		if err != nil {
			t.Fatal(err)
		}
		if head := cutTrace(out); code != 2 || string(head) != tc.out {
			t.Errorf("%s: exit code %d, output:\n%s\nwant exit code 2, output:\n%s", tc.name, code, head, tc.out)
		}
	}
}

// runCxxValue checks the cases of test/cxxvalue_test.cpp that end the program (step 4.12): a
// C++ exception that no function with defers catches ends it like a Go panic (a std::bad_alloc
// like Go's fatal error "out of memory"), with the trace of the throw (the "-bare" cases), and
// one that a function with defers catches is a panic with a CppException value (step 5.10: the
// deferred calls run, and the trace starts in that function)
// (exact only when frames says that the build keeps every frame, not at -O2), and C's stdio
// comes out in the program's order next to fmt and println, through one pipe.
//
// bad-array is new int[-1]: g++ throws std::bad_array_new_length (on_terminate's fatal error),
// clang++ asks operator new for SIZE_MAX bytes (the new handler's): the same report. With ASan,
// whose allocator reports the clang++ request itself, it doesn't run.
func runCxxValue(t *testing.T, bin string, frames, asan bool) {
	for _, tc := range []struct {
		name, out string
		frame     string // a frame that the trace must show (with frames)
	}{
		{"throw-std-bare", "panic: C++ exception std::out_of_range: index 5\n\ngoroutine 1 [running]:\n", "(anonymous namespace)::throw_range(int)"},
		{"bad-alloc-bare", "fatal error: runtime: out of memory\n\ngoroutine 1 [running]:\n", "(anonymous namespace)::throw_bad_alloc()"},
		{"throw-std", "deferred\npanic: C++ exception std::out_of_range: index 5\n\ngoroutine 1 [running]:\n", "(anonymous namespace)::run_case()"},
		{"throw-int", "panic: C++ exception of type int\n\ngoroutine 1 [running]:\n", ""},
		{"throw-in-defer", "panic: first\n\tpanic: C++ exception std::logic_error: from a deferred call\n\ngoroutine 1 [running]:\n", "(anonymous namespace)::run_case()"},
		{"panic-noexcept", "panic: go panic\n\ngoroutine 1 [running]:\n", ""},
		{"bad-array", "fatal error: runtime: out of memory\n\ngoroutine 1 [running]:\n", "(anonymous namespace)::run_case()"},
		{"bad-alloc", "fatal error: runtime: out of memory\n\ngoroutine 1 [running]:\n", "(anonymous namespace)::run_case()"},
	} {
		if asan && tc.name == "bad-array" {
			continue
		}
		out, _, code, err := command(filepath.Dir(bin), nil, true, bin, tc.name)
		if err != nil {
			t.Fatal(err)
		}
		if head := cutTrace(out); code != 2 || string(head) != tc.out {
			t.Errorf("%s: exit code %d, output:\n%s\nwant exit code 2, output:\n%s", tc.name, code, out, tc.out)
		}
		if frames && tc.frame != "" && !bytes.Contains(out, []byte("\n"+tc.frame+"\n\t")) {
			t.Errorf("%s: the trace has no frame %s:\n%s", tc.name, tc.frame, out)
		}
	}
	// (std::terminate without an exception aborts, with the C++ library's message: not run
	// here, because an abort may leave a core dump.)
	out, _, code, err := command(filepath.Dir(bin), nil, true, bin, "stdio")
	if err != nil {
		t.Fatal(err)
	}
	const order = "1 printf\n2 fmt\n3 printf\n4 println\n5 fputs stderr\n6 fmt\n7 printf, flushed at the end\n"
	if code != 0 || string(out) != order {
		t.Errorf("stdio: exit code %d, output:\n%s\nwant exit code 0, output:\n%s", code, out, order)
	}
}

// runLeak checks that LeakSanitizer runs in the asan mode, although every program ends with
// _exit (gp::os::exit runs the check itself): a leak fails the program. Without ASan the
// leak is not found.
func runLeak(t *testing.T, bin string, asan bool) {
	out, _, code, err := command(filepath.Dir(bin), nil, true, bin, "leak")
	if err != nil {
		t.Fatal(err)
	}
	found := bytes.Contains(out, []byte("LeakSanitizer: detected memory leaks"))
	switch {
	case asan && (code == 0 || !found):
		t.Errorf("a leak with ASan: exit code %d, output:\n%s\nwant a LeakSanitizer report and a nonzero exit code", code, out)
	case !asan && (code != 0 || string(out) != "leaked\n"):
		t.Errorf("a leak without ASan: exit code %d, output:\n%s\nwant exit code 0 and %q", code, out, "leaked\n")
	}
}

func runDiff(t *testing.T, bin, name string, want result) {
	out, _, code, err := command(filepath.Dir(bin), nil, true, bin, name)
	if err != nil {
		t.Fatal(err)
	}
	out = maskPC(cutTrace(out))
	if code != want.code {
		t.Errorf("exit code %d, Go's is %d", code, want.code)
	}
	if !bytes.Equal(out, want.out) {
		t.Errorf("the output differs from Go's at line %d:\n%s", firstDiffLine(out, want.out), lineDiff(out, want.out))
	}
}

// firstDiffLine returns the number of the first line where a and b differ.
func firstDiffLine(a, b []byte) int {
	la, lb := strings.SplitAfter(string(a), "\n"), strings.SplitAfter(string(b), "\n")
	for i := range min(len(la), len(lb)) {
		if la[i] != lb[i] {
			return i + 1
		}
	}
	return min(len(la), len(lb)) + 1
}

// lineDiff shows the first few lines that differ.
func lineDiff(got, want []byte) string {
	lg, lw := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	var b strings.Builder
	shown := 0
	for i := 0; i < max(len(lg), len(lw)) && shown < 5; i++ {
		var g, w string
		if i < len(lg) {
			g = lg[i]
		}
		if i < len(lw) {
			w = lw[i]
		}
		if g != w {
			fmt.Fprintf(&b, "line %d:\n\tgocpp: %q\n\tGo:    %q\n", i+1, g, w)
			shown++
		}
	}
	return b.String()
}

// nixHardening is nixpkgs' default NIX_HARDENING_ENABLE (the setup hook of its compiler
// wrapper): what a user's Nix shell has unless it turns hardening off. The project's dev
// shell turns it off (hardeningDisable), so the test sets it itself.
const nixHardening = "bindnow format fortify fortify3 libcxxhardeningfast pic relro stackclashprotection stackprotector strictflexarrays1 strictoverflow zerocallusedregs"

const fortifyWarning = "_FORTIFY_SOURCE requires compiling with optimization"

// TestNoFortify checks gocpp/nofortify.h: a file that includes gocpp/rt.hpp, and the
// runtime's own .cpp files, compile at -O0 with -D_FORTIFY_SOURCE=3 -Werror, and also under a
// Nix compiler wrapper with hardening on, which adds -D_FORTIFY_SOURCE after the user's
// flags. At -O2, _FORTIFY_SOURCE stays.
func TestNoFortify(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	files := []string{sourcePath(t, "test/fortify.cpp")}
	for _, s := range rtfiles.Sources(runtime.GOOS) {
		files = append(files, filepath.Join(rt, filepath.FromSlash(s)))
	}
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			// compile builds each file in a command of its own, all at the same time.
			compile := func(t *testing.T, env []string, flags ...string) {
				t.Helper()
				dir := t.TempDir()
				err := parallel(len(files), func(i int) error {
					args := append(c.flags(flags), "-I", filepath.Join(rt, "include"), "-c", files[i],
						"-o", filepath.Join(dir, fmt.Sprintf("%d.o", i)))
					out, _, code, err := command(dir, append(build.CompilerEnv(), env...), true, c.cc.Path, args...)
					if err != nil || code != 0 || len(out) != 0 {
						return fmt.Errorf("%s %s: exit %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
					}
					return nil
				})
				if err != nil {
					t.Error(err)
				}
			}
			t.Run("flag", func(t *testing.T) {
				t.Parallel()
				t.Run("O0", func(t *testing.T) {
					t.Parallel()
					compile(t, nil, "-O0", "-D_FORTIFY_SOURCE=3")
				})
				t.Run("O2", func(t *testing.T) {
					t.Parallel()
					compile(t, nil, "-O2", "-D_FORTIFY_SOURCE=2", "-DGOCPP_EXPECT_FORTIFY")
				})
			})
			t.Run("nix-hardening", func(t *testing.T) {
				t.Parallel()
				env := []string{"NIX_HARDENING_ENABLE=" + nixHardening}
				// First without nofortify.h: if the compiler doesn't warn, it isn't a Nix
				// wrapper (or it ignores NIX_HARDENING_ENABLE), and there is nothing to check.
				dir := t.TempDir()
				plain := filepath.Join(dir, "plain.cpp")
				if err := os.WriteFile(plain, []byte("#include <cstdio>\nint main() { std::puts(\"x\"); }\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				out, _, _, err := command(dir, append(build.CompilerEnv(), env...), true, c.cc.Path, append(c.flags([]string{"-O0"}), "-Wno-error", "-c", plain)...)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(out, []byte(fortifyWarning)) {
					t.Skipf("%s with NIX_HARDENING_ENABLE doesn't warn about _FORTIFY_SOURCE at -O0: not a Nix compiler wrapper", c.cc.Path)
				}
				compile(t, env, "-O0")
			})
		})
	}
}

// TestSymbols checks the runtime's naming rule at link level: the runtime's object files
// define no strong global symbol outside namespace gp (or the gp_ prefix). An extern "C"
// helper, even in an unnamed namespace, would be a global C name in every gocpp program, and
// a user's C file with the same name (Phase 04) would not link. Weak symbols (W, V, u: the
// inline functions and template instances of the standard library, typeinfo) are left out:
// the linker merges them, so they can't clash. Skipped without nm.
func TestSymbols(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	nm, err := exec.LookPath("nm")
	if err != nil {
		t.Skipf("no nm: %v", err)
	}
	rt := extractRuntime(t)
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			dir := t.TempDir()
			// The runtime's objects at -O0 are those TestCXX links (parallel_test.go): the
			// flags are the same, and a compile that printed anything failed.
			objs, err := c.runtimeObjects(rt, []string{"-O0"})
			if err != nil {
				t.Fatal(err)
			}
			for i, s := range rtfiles.Sources(runtime.GOOS) {
				obj := objs[i]
				out, _, code, err := command(dir, nil, true, nm, "-g", "--defined-only", "-C", obj)
				if err != nil || code != 0 {
					t.Fatalf("nm %s: exit %d %v\n%s", obj, code, err, out)
				}
				n := 0
				for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
					f := strings.SplitN(line, " ", 3)
					if len(f) != 3 || strings.ContainsAny(f[1], "WwVvu") {
						continue
					}
					n++
					if !runtimeName(f[2]) {
						t.Errorf("%s defines the global symbol %q (nm type %s), outside namespace gp: give it internal linkage or put it in gp", s, f[2], f[1])
					}
				}
				if n == 0 {
					t.Errorf("nm found no global symbols in %s:\n%s", s, out)
				}
			}
		})
	}
}

// runtimeName reports whether a demangled symbol name belongs to the runtime: in namespace
// gp, or starting with gp_ (also the typeinfo, vtable and guard variables of such names).
func runtimeName(name string) bool {
	for _, p := range []string{"typeinfo for ", "typeinfo name for ", "vtable for ", "construction vtable for ", "guard variable for "} {
		name = strings.TrimPrefix(name, p)
	}
	return strings.HasPrefix(name, "gp::") || strings.HasPrefix(name, "gp_")
}
