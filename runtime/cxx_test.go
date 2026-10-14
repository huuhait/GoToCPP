package rtfiles_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
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

// The C++ tests of the runtime: test/unit.cpp, test/slice_test.cpp and test/array_test.cpp
// (unit tests) and test/diff.cpp (the C++ side of a differential test against
// testdata/diff/main.go, run with real Go). Each is compiled with the embedded runtime
// (rtfiles.FS, extracted to a temporary folder, as the driver will do it), with g++ and with
// clang++, in each mode below.
//
// Like the test harness (tests/harness_test.go): GOCPP_CXX=<compiler> tests with that
// compiler only; a compiler that isn't installed is a skip, unless GOCPP_REQUIRE_CXX (a comma
// list, "g++,clang++" in make ci) names it, which makes it a failure.

// cmdTimeout bounds each compile and each run.
const cmdTimeout = 3 * time.Minute

// baseFlags are the driver's flags (Phase 02 §2.7), plus warnings as errors for the runtime's
// own code.
var baseFlags = []string{"-std=c++20", "-fwrapv", "-fno-strict-aliasing", "-ffp-contract=off", "-g",
	"-Wall", "-Wextra", "-Werror"}

// The modes: the driver's debug build, an optimized build, and the sanitizers. With
// -fno-sanitize-recover=all, a sanitizer report stops the program with a nonzero exit code.
var modes = []struct {
	name  string
	flags []string
}{
	{"O0", []string{"-O0"}},
	{"O2", []string{"-O2"}},
	{"asan", []string{"-O0", "-fsanitize=address,undefined", "-fno-omit-frame-pointer", "-fno-sanitize-recover=all"}},
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

// extractRuntime writes the embedded runtime into a temporary folder and returns the folder.
func extractRuntime(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	err := fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, filepath.FromSlash(p))
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		b, err := fs.ReadFile(rtfiles.FS(), p)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// command runs a command with a timeout, in the environment env (nil: the test's own). combined merges stdout and stderr into one stream
// (one pipe, so the order of the writes is kept); otherwise stdout and stderr are separate.
func command(dir string, env []string, combined bool, name string, args ...string) (stdout, stderr []byte, code int, err error) {
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

// compile builds the test program src (a file of runtime/test/) with the runtime in rt and
// returns the binary.
func (c cxx) compile(t *testing.T, rt, src, outDir string, flags []string, env []string) string {
	t.Helper()
	abs := sourcePath(t, src)
	bin := filepath.Join(outDir, strings.TrimSuffix(filepath.Base(src), ".cpp"))
	args := c.flags(flags)
	args = append(args, "-I", filepath.Join(rt, "include"), abs)
	for _, s := range rtfiles.Sources(runtime.GOOS) {
		args = append(args, filepath.Join(rt, filepath.FromSlash(s)))
	}
	args = append(args, "-o", bin)
	out, _, code, err := command(outDir, append(build.CompilerEnv(), env...), true, c.cc.Path, args...)
	if err != nil || code != 0 {
		t.Fatalf("%s %s: exit %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
	}
	if len(out) != 0 {
		t.Fatalf("%s %s printed warnings:\n%s", c.cc.Path, strings.Join(args, " "), out)
	}
	return bin
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
	"div", "rem", "shl", "shr", "index", "negindex", "uindex", "exit0", "big", "bigerr",
	"ptr", "nil-deref", "nil-store", "nil-field", "nil-method", "nil-addr",
	"substr", "utf8", "substr-alen", "substr-b", "substr-neg", "substr-from"}

// The slice cases (Phase 03 step 3.5a): the sliceCase function of testdata/diff/main.go.
var sliceDiffCases = []string{"slices", "slice-grow", "slice-free", "slice-index", "slice-index-neg",
	"slice-index-u8", "slice-index-nil", "slice-hi", "slice-hi-neg", "slice-hi-u", "slice-lo",
	"slice-lo-neg", "slice-from", "slice-nil-sub", "slice3-max", "slice3-max-neg", "slice3-max-u",
	"slice3-hi", "slice3-hi-neg", "slice3-lo", "slice3-lo-neg", "make-len", "make-len-big",
	"make-cap", "make-cap-big", "slice-convert"}

func init() { diffCases = append(diffCases, sliceDiffCases...) }

// The array cases (Phase 03 step 3.6a): the arrayCase function of testdata/diff/main.go.
var arrayDiffCases = []string{"arrays", "array-index", "array-index-neg", "array-index-u8",
	"array-index0", "array-convert", "array-convert-ptr", "array-convert-nil", "array-slice-hi",
	"array-slice-lo", "array-slice3-max", "array-slice-nil", "array-slice-nil0"}

func init() { diffCases = append(diffCases, arrayDiffCases...) }

// A result is what a program printed (stdout and stderr merged) and its exit code.
type result struct {
	out  []byte
	code int
}

// goStackMark is the line after which Go prints the stack trace of a panic. gocpp prints
// that line too, and the trace comes in Phase 03, so both outputs are compared up to it.
const goStackMark = "goroutine 1 [running]:\n"

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
	gocmd, err := exec.LookPath("go")
	if err != nil {
		t.Logf("no go command, so the differential tests are skipped: %v", err)
		return nil, ""
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "godiff")
	src := sourcePath(t, "testdata/diff/main.go")
	out, _, code, err := command(".", nil, true, gocmd, "build", "-o", bin, src)
	if err != nil || code != 0 {
		t.Fatalf("go build testdata/diff/main.go: exit %d %v\n%s", code, err, out)
	}
	// Go's runtime print (println) writes with one write call and drops what a short write
	// leaves. A blocking pipe write ends short when a signal arrives in the middle, and the
	// Go runtime sends itself SIGURG to preempt goroutines, so under load "bigerr" lost the
	// end of its 1 MiB line in real Go (the gocpp side loops until all is written). Without
	// asynchronous preemption there is no SIGURG, and Go's output is complete.
	env := append(os.Environ(), "GODEBUG=asyncpreemptoff=1")
	res := make(map[string]result)
	for _, name := range diffCases {
		out, _, code, err := command(dir, env, true, bin, name)
		if err != nil {
			t.Fatal(err)
		}
		if i := bytes.Index(out, []byte(goStackMark)); i >= 0 {
			out = out[:i+len(goStackMark)]
		}
		res[name] = result{maskPC(out), code}
	}
	return res, bin
}

// TestCXX compiles and runs the C++ unit tests and the differential test with each compiler
// in each mode.
func TestCXX(t *testing.T) {
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
					dir := t.TempDir()
					unit := c.compile(t, rt, "test/unit.cpp", dir, m.flags, nil)
					diff := c.compile(t, rt, "test/diff.cpp", dir, m.flags, nil)
					slice := c.compile(t, rt, "test/slice_test.cpp", dir, m.flags, nil)
					array := c.compile(t, rt, "test/array_test.cpp", dir, m.flags, nil)
					t.Run("unit", func(t *testing.T) { runUnit(t, unit) })
					t.Run("slice", func(t *testing.T) { runUnit(t, slice) })
					t.Run("array", func(t *testing.T) { runUnit(t, array) })
					t.Run("panic", func(t *testing.T) { runPanic(t, diff) })
					t.Run("leak", func(t *testing.T) { runLeak(t, diff, m.name == "asan") })
					t.Run("pipes", func(t *testing.T) { runPipes(t, diff, goBin) })
					t.Run("diff", func(t *testing.T) {
						if want == nil {
							t.Skip("no go command")
						}
						for _, name := range diffCases {
							t.Run(name, func(t *testing.T) {
								if name == "f2i" && runtime.GOARCH != "amd64" {
									t.Skipf("gocpp copies Go's float → int conversions on amd64; this Go is %s", runtime.GOARCH)
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
func runPanic(t *testing.T, bin string) {
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
		out = maskPC(out)
		if code != tc.code || string(out) != tc.out {
			t.Errorf("%s: exit code %d, output:\n%s\nwant exit code %d, output:\n%s", tc.name, code, out, tc.code, tc.out)
		}
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
	out = maskPC(out)
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
			compile := func(t *testing.T, env []string, flags ...string) {
				t.Helper()
				args := append(c.flags(flags), "-I", filepath.Join(rt, "include"), "-c")
				args = append(args, files...)
				out, _, code, err := command(t.TempDir(), append(build.CompilerEnv(), env...), true, c.cc.Path, args...)
				if err != nil || code != 0 || len(out) != 0 {
					t.Errorf("%s %s: exit %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
				}
			}
			t.Run("flag", func(t *testing.T) {
				compile(t, nil, "-O0", "-D_FORTIFY_SOURCE=3")
				compile(t, nil, "-O2", "-D_FORTIFY_SOURCE=2", "-DGOCPP_EXPECT_FORTIFY")
			})
			t.Run("nix-hardening", func(t *testing.T) {
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
			for _, s := range rtfiles.Sources(runtime.GOOS) {
				obj := filepath.Join(dir, strings.ReplaceAll(s, "/", "_")+".o")
				args := append(c.flags([]string{"-O0"}), "-I", filepath.Join(rt, "include"), "-c",
					filepath.Join(rt, filepath.FromSlash(s)), "-o", obj)
				out, _, code, err := command(dir, build.CompilerEnv(), true, c.cc.Path, args...)
				if err != nil || code != 0 || len(out) != 0 {
					t.Fatalf("%s %s: exit %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
				}
				out, _, code, err = command(dir, nil, true, nm, "-g", "--defined-only", "-C", obj)
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
