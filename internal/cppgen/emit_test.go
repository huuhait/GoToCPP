package cppgen

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/version"
	"github.com/huuhait/gocpp/stdlib"
)

var update = flag.Bool("update", false, "rewrite testdata/emit/*.cpp.golden with the emitted C++, and add new macros to cxx_macros.txt")

// emitPatterns loads and checks the package of the patterns (in the folder dir) and returns
// its C++. The check runs without the phase gates (Phase: check.LastPhase): they are tested in
// internal/check, and the emitter already handles some features before their phase (package
// variables and init functions, Phase 03), and its own "not supported yet" errors are tested
// here.
func emitPatterns(t *testing.T, cfg Config, dir string, patterns ...string) (string, error) {
	t.Helper()
	fset, pkg := checkPatterns(t, dir, patterns...)
	var out bytes.Buffer
	err := EmitPackage(&out, fset, pkg, cfg)
	if err != nil && out.Len() != 0 {
		t.Errorf("EmitPackage failed (%v) but wrote %d bytes", err, out.Len())
	}
	return out.String(), err
}

// checkPatterns loads and checks the package of the patterns (in the folder dir), as
// emitPatterns does, and fails the test if it has errors.
func checkPatterns(t *testing.T, dir string, patterns ...string) (*token.FileSet, *check.Package) {
	t.Helper()
	lcfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
	prog := load.Load(lcfg, patterns...)
	r := check.Check(prog, check.Config{Phase: check.LastPhase})
	if len(prog.Roots) != 1 {
		t.Fatalf("%d root packages", len(prog.Roots))
	}
	pkg := r.Package(prog.Roots[0])
	if pkg.Errors.HasErrors() || prog.Errors.HasErrors() {
		var b bytes.Buffer
		for _, d := range append(prog.Errors.Diagnostics(), pkg.Errors.Diagnostics()...) {
			b.WriteString(d.Pos.String() + ": " + d.Msg + "\n")
		}
		t.Fatalf("the program has errors:\n%s", b.String())
	}
	return r.Fset, pkg
}

// An emitFunc returns the C++ of a test program, written with cfg.
type emitFunc func(t *testing.T, cfg Config) string

// emitter returns the emitFunc of the program in path: a .gp file, or a folder with the .gp
// files of one package.
func emitter(path string) emitFunc {
	return func(t *testing.T, cfg Config) string {
		t.Helper()
		abs, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		var cpp string
		if strings.HasSuffix(abs, ".gp") {
			cpp, err = emitPatterns(t, cfg, filepath.Dir(abs), abs)
		} else {
			cpp, err = emitPatterns(t, cfg, abs, ".")
		}
		if err != nil {
			t.Fatal(err)
		}
		return cpp
	}
}

// withDir replaces the folder dir in the #line directives of cpp with $DIR, so that the
// golden files don't depend on where the repository is.
func withDir(cpp, dir string) string {
	inner := cppir.CString(dir + string(filepath.Separator))
	inner = inner[1 : len(inner)-1]
	return strings.ReplaceAll(cpp, `"`+inner, `"$DIR/`)
}

// TestEmit emits each program of testdata/emit (a .gp file, or a folder with a package of
// several files) and compares the C++ with its .cpp.golden file (go test -update rewrites
// them). Then it builds the C++ with each compiler and checks that the program prints what
// the same program prints with real Go.
func TestEmit(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "emit"))
	if err != nil {
		t.Fatal(err)
	}
	progs, err := filepath.Glob(filepath.Join(dir, "*.gp"))
	if err != nil || len(progs) == 0 {
		t.Fatalf("no test programs: %v", err)
	}
	more, err := filepath.Glob(filepath.Join(dir, "*", "*.gp"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range more {
		if d := filepath.Dir(f); !slices.Contains(progs, d) {
			progs = append(progs, d)
		}
	}
	for _, prog := range progs {
		t.Run(strings.TrimSuffix(filepath.Base(prog), ".gp"), func(t *testing.T) {
			t.Parallel()
			emit := emitter(prog)
			golden := strings.TrimSuffix(prog, ".gp") + ".cpp.golden"
			got := withDir(emit(t, Config{}), dir)
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v (go test -update writes it)", err)
			}
			if got != string(want) {
				t.Errorf("the C++ differs from %s (go test -update rewrites it):\n%s", golden, got)
			}
			gps := []string{prog}
			if !strings.HasSuffix(prog, ".gp") {
				gps, _ = filepath.Glob(filepath.Join(prog, "*.gp"))
			}
			goWant := goResult(t, gps...)
			if arch := goarchOnly(t, gps[0]); arch != "" && arch != runtime.GOARCH {
				t.Logf("the output is compared with Go's only on %s (Phase 02 README §2.3)", arch)
				goWant = nil
			}
			checkRunsEnv(t, emit, goWant, nil, cycleEnv(t, gps[0]))
		})
	}
}

// goarchOnly returns the GOARCH of a "// goarch: amd64" line in the .gp file, or "". Such a
// program prints what Go prints only on that architecture: Go's float-to-integer conversions
// of out-of-range values depend on the CPU, and gocpp copies amd64's (gp::f2i).
func goarchOnly(t *testing.T, gp string) string {
	t.Helper()
	src, err := os.ReadFile(gp)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(src), "\n") {
		if arch, ok := strings.CutPrefix(l, "// goarch: "); ok {
			return strings.TrimSpace(arch)
		}
	}
	return ""
}

// cycleEnv returns the environment change for the program in the .gp file gp if it has a
// "// lsan: cycle" line in its header (the comment lines before the package clause), as in
// tests/README.md: it leaks a reference cycle on purpose (D1), so a sanitizer run
// (GOCPP_TEST_CXXFLAGS with -fsanitize=address) turns LeakSanitizer off for it with
// ASAN_OPTIONS=detect_leaks=0, after the options of the environment. Else nil.
func cycleEnv(t *testing.T, gp string) []string {
	t.Helper()
	src, err := os.ReadFile(gp)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(src), "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "//") {
			break // the package clause: the header ends
		}
		if l == "// lsan: cycle" {
			if o := os.Getenv("ASAN_OPTIONS"); o != "" {
				return []string{"ASAN_OPTIONS=" + o + ":detect_leaks=0"}
			}
			return []string{"ASAN_OPTIONS=detect_leaks=0"}
		}
	}
	return nil
}

// checkRuns writes the C++ of a program to a file (with Config.OutputPath set to it), builds
// it with each compiler, runs it, and compares what it prints and its exit code with want
// (nil: nothing to compare with, only build and run).
func checkRuns(t *testing.T, emit emitFunc, want *result) {
	t.Helper()
	checkRunsNorm(t, emit, want, nil)
}

// checkRunsNorm is checkRuns, where norm (if not nil) rewrites both results before they are
// compared.
func checkRunsNorm(t *testing.T, emit emitFunc, want *result, norm func(result) result) {
	t.Helper()
	checkRunsMode(t, emit, want, norm, false)
}

// checkRunsMode is checkRunsNorm, with nochecks for a -nochecks build (cxx.buildMode).
func checkRunsMode(t *testing.T, emit emitFunc, want *result, norm func(result) result, nochecks bool) {
	t.Helper()
	checkRunsWith(t, emit, want, norm, nochecks, nil)
}

// checkRunsEnv is checkRunsNorm, where the program runs with the environment changes env.
func checkRunsEnv(t *testing.T, emit emitFunc, want *result, norm func(result) result, env []string) {
	t.Helper()
	checkRunsWith(t, emit, want, norm, false, env)
}

// checkRunsWith is checkRunsNorm with nochecks (checkRunsMode) and env (checkRunsEnv).
func checkRunsWith(t *testing.T, emit emitFunc, want *result, norm func(result) result, nochecks bool, env []string) {
	t.Helper()
	if want != nil && norm != nil {
		w := norm(*want)
		want = &w
	}
	work := t.TempDir()
	src := filepath.Join(work, "main.gp.cpp")
	cpp := emit(t, Config{OutputPath: src})
	checkOwnLines(t, cpp, src)
	if err := os.WriteFile(src, []byte(cpp), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			got := runEnv(t, work, env, c.buildMode(t, src, nochecks))
			if norm != nil {
				got = norm(got)
			}
			if want != nil && got != *want {
				t.Errorf("the program prints\nstdout:\n%s\nstderr:\n%s\nexit code %d\nbut with Go\nstdout:\n%s\nstderr:\n%s\nexit code %d",
					got.stdout, got.stderr, got.code, want.stdout, want.stderr, want.code)
			}
		})
	}
}

// checkOwnLines checks the #line directives that point at the generated file out itself: each
// one gives the next line its real line number, and the main wrapper comes after one.
func checkOwnLines(t *testing.T, cpp, out string) {
	t.Helper()
	own := `"` + strings.Trim(cppir.CString(out), `"`) + `"`
	lines := strings.Split(cpp, "\n")
	last := -1
	for i, l := range lines {
		if n, ok := strings.CutSuffix(l, " "+own); ok {
			if want := fmt.Sprintf("#line %d", i+2); n != want {
				t.Errorf("line %d is %q, want %s %s", i+1, l, want, own)
			}
			last = i
		}
		if strings.HasPrefix(l, "#line ") && !strings.HasSuffix(l, own) {
			last = -1
		}
	}
	wrapper := slices.Index(lines, "int main(int argc, char** argv) {")
	if wrapper < 0 || last < 0 || last > wrapper {
		t.Errorf("the main wrapper (line %d) doesn't come after a #line of the generated file:\n%s", wrapper+1, cpp)
	}
}

var (
	goOnce sync.Once
	goCmd  string
)

// goResult runs the .gp files, one package, as a Go program with real Go (the go command in
// PATH) and returns what it prints, or nil (and logs why) without a go command.
func goResult(t *testing.T, gps ...string) *result {
	t.Helper()
	goOnce.Do(func() { goCmd, _ = exec.LookPath("go") })
	if goCmd == "" {
		t.Log("no go command: the output is not compared with Go's")
		return nil
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "prog")
	args := []string{"build", "-o", bin}
	for _, gp := range gps {
		src, err := os.ReadFile(gp)
		if err != nil {
			t.Fatal(err)
		}
		name := strings.TrimSuffix(filepath.Base(gp), ".gp") + ".go"
		if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
			t.Fatal(err)
		}
		args = append(args, name)
	}
	out, _, code, err := goCommand(dir, args...)
	if err != nil || code != 0 {
		t.Fatalf("go build %s: exit code %d %v\n%s", strings.Join(gps, " "), code, err, out)
	}
	r := run(t, dir, bin) // its stderr ends at the stack trace, as gocpp's does
	return &r
}

// goStackMark is the line of a panic message after which Go and gocpp print the stack trace.
const goStackMark = "goroutine 1 [running]:\n"

// pcRx matches the pc in the signal line of a nil pointer panic, "[signal SIGSEGV:
// segmentation violation code=0x1 addr=0x0 pc=0x4a2b1c]": an address in the program, which
// differs between Go and gocpp.
var pcRx = regexp.MustCompile(` pc=0x[0-9a-f]+\]`)

// maskPC replaces the pc of the signal line in the stderr of r with PC (checkRunsNorm).
func maskPC(r result) result {
	r.stderr = pcRx.ReplaceAllString(r.stderr, " pc=PC]")
	return r
}

// TestExampleHello builds examples/hello, the milestone M1 program, and checks that it
// prints its output.txt.
func TestExampleHello(t *testing.T) {
	checkExample(t, "hello")
}

// TestExampleFizzbuzz and TestExamplePrimes build the other Phase 02 examples (switch, all
// the for forms, labels) and check that they print their output.txt.
func TestExampleFizzbuzz(t *testing.T) {
	checkExample(t, "fizzbuzz")
}

func TestExamplePrimes(t *testing.T) {
	checkExample(t, "primes")
}

// checkExample builds examples/<name> and checks that it prints its output.txt.
func checkExample(t *testing.T, name string) {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "examples", name))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join(dir, "output.txt"))
	if err != nil {
		t.Fatal(err)
	}
	checkRuns(t, emitter(dir), &result{stdout: string(want)})
}

// TestUnsupported checks that what the emitter can't translate yet gives an *Error at the
// right place, never a panic or C++ that doesn't compile.
func TestUnsupported(t *testing.T) {
	tests := []struct {
		name, src, want string
	}{
		{"makechan", "func main() {\n\t_ = make(chan int)\n}", "main.gp:4:6: cppgen: not supported yet: make of chan int (Phase 08)"},
		{"bytes-named-elem", "type MyByte byte\nfunc main() {\n\t_ = []MyByte(\"ab\")\n}", "main.gp:5:6: cppgen: not supported yet: the conversion from string to []MyByte"},
		{"runes-named-elem", "type R rune\nfunc main() {\n\t_ = string([]R{'a'})\n}", "main.gp:5:6: cppgen: not supported yet: the conversion from []R to string"},
		{"slice-tags", "func main() {\n\t_ = []struct{ Q int }([]struct {\n\t\tQ int `t:\"q\"`\n\t}{})\n}", `main.gp:4:6: cppgen: not supported yet: the conversion from []struct{Q int "t:\"q\""} to []struct{Q int}`},
		{"struct-slice-tags", "type W struct{ S []struct{ Q int `t:\"q\"` } }\ntype V struct{ S []struct{ Q int } }\nfunc main() {\n\t_ = V(W{})\n}", "main.gp:6:6: cppgen: not supported yet: the conversion from W to V, whose fields differ in struct tags inside a slice or a map"},
		{"struct-map-tags", "type W struct{ M map[int]struct{ Q int `t:\"q\"` } }\ntype V struct{ M map[int]struct{ Q int } }\nfunc main() {\n\t_ = V(W{})\n}", "main.gp:6:6: cppgen: not supported yet: the conversion from W to V, whose fields differ in struct tags inside a slice or a map"},
		{"printfunc", "import \"fmt\"\nfunc main() {\n\tfmt.Println(1, main)\n}", "main.gp:5:17: cppgen: not supported yet: printing a func value"},
		{"printlnfunc", "func g() {}\nfunc main() {\n\tprintln(g)\n}", "main.gp:5:10: cppgen: not supported yet: printing a func value with println"},
		{"printfuncfield", "import \"fmt\"\ntype T struct{ f func() }\nfunc main() {\n\tfmt.Println(T{})\n}", "main.gp:6:14: cppgen: not supported yet: printing a func value"},
		{"printf", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%#v\\n\", 1)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf directive %#v (Phase 09)`},
		{"printf-star", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%-*d\\n\", 1, 2)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf directive %-*d (Phase 09)`},
		{"printf-index", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%5.1[1]f\\n\", 1.5)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf directive %5.1[1]f (Phase 09)`},
		{"sprintf-goverb", "import \"fmt\"\nfunc main() {\n\tprintln(fmt.Sprintf(\"%-5q %T\", \"a\", 1))\n}", `main.gp:5:22: cppgen: not supported yet: the fmt.Sprintf directive %T (Phase 05)`},
		{"errorf", "import \"fmt\"\nfunc main() {\n\t_ = fmt.Errorf(\"x\")\n}", "main.gp:5:6: cppgen: not supported yet: the call of fmt.Errorf (gocpp lowers only fmt.Print, Println, Printf, Sprint, Sprintln and Sprintf)"},
		{"printf-goverb", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%z %x\\n\", 1, 2)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf directive %x (Phase 09)`},
		{"errorresult", "import \"fmt\"\nfunc main() {\n\tn, err := fmt.Println()\n\tprintln(n, err)\n}", "main.gp:5:5: cppgen: not supported yet: type error"},
		{"rangechan", "func main() {\n\tfor range make(chan int) {\n\t}\n}", "main.gp:4:12: cppgen: not supported yet: range over channels (Phase 08)"},
		{"go", "func main() {\n\tgo println()\n}", "main.gp:4:2: cppgen: not supported yet: go statements (Phase 08)"},
		{"insideif", "func main() {\n\tif x := 1; x > 0 {\n\t\tgo println()\n\t}\n}", "main.gp:5:3: cppgen: not supported yet: go statements (Phase 08)"},
		{"nomain", "func f() {}", "main.gp:2:9: cppgen: function main is undeclared in the main package"},
		{"addrderefcall", "func f() *int { return nil }\nfunc main() {\n\t_ = &*f()\n}", "main.gp:5:6: cppgen: not supported yet: &*p where p is not a variable"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			src := "// test\npackage main\n" + tt.src + "\n"
			if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := emitPatterns(t, Config{}, dir, filepath.Join(dir, "main.gp"))
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("got %v (%T), want an *Error", err, err)
			}
			if got := strings.TrimPrefix(err.Error(), dir+string(filepath.Separator)); got != tt.want {
				t.Errorf("got  %s\nwant %s", got, tt.want)
			}
		})
	}
}

// TestRuntimeFuncNames checks that a Go function named like any free function of namespace
// gp can be called with a gp::string argument (funcCallName qualifies the call, so C++'s
// argument-dependent lookup doesn't find the runtime's function too). The names come from
// clang's AST of gocpp/rt.hpp (-ast-dump-filter=gp::), so a function that the runtime adds
// is tested too.
func TestRuntimeFuncNames(t *testing.T) {
	var clang *cxx
	for _, c := range compilers(t) {
		if c.missing == "" && c.cc.Family == build.Clang {
			clang = c
		}
	}
	if clang == nil {
		t.Skip("needs clang++, whose -ast-dump lists the runtime's functions")
	}
	dir := t.TempDir()
	rt := filepath.Join(dir, "rt.cpp")
	if err := os.WriteFile(rt, []byte("#include <gocpp/rt.hpp>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	args := append(clang.flags(includeFlag(t)), "-fsyntax-only", "-Xclang", "-ast-dump-filter=gp::", "-Xclang", "-ast-dump", rt)
	out, _, code, err := command(dir, false, clang.cc.Path, args...)
	if err != nil || code != 0 {
		t.Fatalf("%s %s: exit code %d %v", clang.cc.Path, strings.Join(args, " "), code, err)
	}
	// The dump has a "Dumping gp::len:" line before each declaration, then its kind.
	var names []string
	lines := strings.Split(string(out), "\n")
	for i, l := range lines[:len(lines)-1] {
		name, ok := strings.CutPrefix(l, "Dumping gp::")
		name, _ = strings.CutSuffix(name, ":")
		kind, _, _ := strings.Cut(lines[i+1], " ")
		if ok && (kind == "FunctionDecl" || kind == "FunctionTemplateDecl") && token.IsIdentifier(name) &&
			!slices.Contains(names, name) && name != "main" && name != "init" && name != "fmt" {
			names = append(names, name)
		}
	}
	for _, want := range []string{"len", "panic", "print", "println", "div", "shl", "run_main"} {
		if !slices.Contains(names, want) {
			t.Fatalf("the runtime's functions %v lack %s: did clang's dump change?", names, want)
		}
	}
	var b strings.Builder
	b.WriteString("package main\n\nimport \"fmt\"\n\n")
	for i, n := range names {
		fmt.Fprintf(&b, "func %s(s string) int {\n\treturn %d\n}\n\n", n, i)
	}
	b.WriteString("func main() {\n\ts := \"x\"\n")
	for _, n := range names {
		fmt.Fprintf(&b, "\tfmt.Println(%s(s), %s(\"lit\"))\n", n, n)
	}
	b.WriteString("}\n")
	gp := filepath.Join(dir, "main.gp")
	if err := os.WriteFile(gp, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	checkRuns(t, emitter(gp), goResult(t, gp))
}

// TestPanics checks the run-time panics of expressions (division by zero, a negative shift
// count, a string index out of range; also in x /= y and x <<= n, in a case and in a loop
// condition), the builtin panic with values of basic types, the nil pointer dereferences
// (also through new(*int), &*p and a parameter), the panics of slices (an index out of
// range, also in a store, op=, ++ and &s[i]; the slice expressions with 2 and 3 indexes; make),
// of arrays (index, slicing, the conversions from slices, nil pointers to arrays), and of maps
// (a store into a nil map, and the order of the panics of a store): each program prints what it
// prints with Go, exits with code 2, and stderr is Go's up to the "goroutine 1 [running]:" line
// (goResult), with the signal line of a nil dereference (its pc masked: maskPC). The cases of
// noChecksStay also run as a -nochecks build, with the same result.
func TestPanics(t *testing.T) {
	tests := []struct{ name, body string }{
		{"div", "a, b := 7, 0\n\tfmt.Println(\"before\", a/2)\n\tfmt.Println(a / b)"},
		{"div8", "var a, b int8 = -128, 0\n\tfmt.Println(a / b)"},
		{"divu", "var a, b uint = 7, 0\n\tfmt.Println(a / b)"},
		{"rem", "a, b := 7, 0\n\tfmt.Println(a % b)"},
		{"div-assign", "a, b := 7, 0\n\ta /= b\n\tfmt.Println(a)"},
		{"rem-assign8", "var a, b int8 = 7, 0\n\ta %= b\n\tfmt.Println(a)"},
		{"shl-assign", "x, n := 1, -1\n\tx <<= n\n\tfmt.Println(x)"},
		{"shr-assign16", "var x uint16 = 8\n\tn := -3\n\tx >>= n\n\tfmt.Println(x)"},
		{"switch-case", "x, y := 1, 0\n\tswitch x {\n\tcase 2:\n\t\tfmt.Println(2)\n\tcase 1 / y:\n\t\tfmt.Println(1)\n\t}"},
		{"for-cond", "for i := 3; 10/i > 0; i-- {\n\t\tfmt.Println(i)\n\t}"},
		{"rem16", "var a, b uint16 = 7, 0\n\tfmt.Println(a % b)"},
		{"shl", "x, n := 1, -1\n\tfmt.Println(x << n)"},
		{"shr8", "var x int64 = 1\n\tvar n int8 = -128\n\tfmt.Println(x >> n)"},
		{"index", "s, i := \"abc\", 5\n\tfmt.Println(s[i])"},
		{"index-len", "s := \"abc\"\n\tfmt.Println(s[len(s)])"},
		{"index-neg", "s, i := \"abc\", -1\n\tfmt.Println(s[i])"},
		{"index-uint", "var i uint64 = 18446744073709551615\n\ts := \"abc\"\n\tfmt.Println(s[i])"},
		{"index-empty", "var s string\n\tfmt.Println(s[0])"},
		{"slice-alen", "s, i, j := \"abc\", 1, 5\n\tfmt.Println(s[i:j])"},
		{"slice-b", "s, i, j := \"abc\", 3, 2\n\tfmt.Println(s[i:j])"},
		{"slice-neg", "s, i := \"abc\", -1\n\tfmt.Println(s[i:])"},
		{"slice-neg-high", "s, j := \"abc\", -2\n\tfmt.Println(s[:j])"},
		{"slice-from", "s, i := \"abc\", uint8(4)\n\tfmt.Println(s[i:])"},
		{"slice-uint", "s, j := \"abc\", uint64(1<<64-1)\n\tfmt.Println(s[1:j])"},
		{"slice-const", "s := \"abc\"\n\tfmt.Println(s[1:5])"},
		{"slice-literal", "i := 4\n\tfmt.Println(\"abc\"[i:])"},
		{"slice-both-bad", "s, i, j := \"abc\", 9, 7\n\tfmt.Println(s[i:j])"},
		{"slice-range", "s, i := \"abc\", 4\n\tfor range s[i:] {\n\t}"},
		{"slice-concat", "s, i := \"abc\", 4\n\tt := s + s + s[i:]\n\tfmt.Println(t)"},
		{"panic-str", "s := \"bo\"\n\tpanic(s + \"om\")"},
		{"panic-const", "panic(\"multi\\nline\")"},
		{"panic-int", "panic(42)"},
		{"panic-int8", "var x int8 = -7\n\tpanic(x * 2)"},
		{"panic-uint", "var x uint64 = 1 << 63\n\tpanic(x)"},
		{"panic-float", "panic(1.5)"},
		{"panic-float32", "var f float32 = 0.1\n\tpanic(f)"},
		{"panic-bool", "x := 1\n\tpanic(x > 0)"},
		{"panic-rune", "panic('x')"},
		{"nil-deref", "var p *int\n\tfmt.Println(\"before\")\n\tfmt.Println(*p)"},
		{"nil-store", "var p *int\n\t*p = 1"},
		{"nil-inc", "var p *string\n\t*p += \"x\""},
		{"nil-new", "pp := new(*int)\n\t**pp = 1"},
		{"nil-addr", "var p *int\n\tq := &*p\n\tfmt.Println(q == nil)"},
		{"nil-param", "fmt.Println(deref(nil))"},
		// Slices (step 3.5): index, slice-expression and make panics.
		{"slice-index", "s, i := []int{1, 2, 3}, 5\n\tfmt.Println(s[i])"},
		{"slice-index-neg", "s, i := []int{1, 2, 3}, -1\n\tfmt.Println(s[i])"},
		{"slice-index-uint8", "s := make([]string, 3)\n\tvar i uint8 = 200\n\tfmt.Println(s[i])"},
		{"slice-store", "s, i := []int{1, 2, 3}, 3\n\ts[i] = 7"},
		{"slice-op-assign", "s, i := []float64{1}, 1\n\ts[i] *= 2"},
		{"slice-inc", "s, i := []uint8{1}, 4\n\ts[i]++"},
		{"slice-nil", "var s []int\n\tfmt.Println(\"before\")\n\ts[0] = 1"},
		{"slice-empty", "s := []bool{}\n\tfmt.Println(s[0])"},
		{"slice-nested", "s := [][]int{{1}, nil}\n\tfmt.Println(s[0][0])\n\tfmt.Println(s[1][0])"},
		{"slice-addr", "s, i := []int{1, 2, 3}, 3\n\tp := &s[i]\n\tfmt.Println(*p)"},
		{"slice-addr-call", "i := 1\n\tp := &mk()[i]\n\tfmt.Println(*p)"},
		{"slice-runes", "r, i := []rune(\"h\u00e9llo\"), 5\n\tfmt.Println(len(r), r[i])"},
		{"slice-append-index", "s, i := []int{1}, 2\n\tfmt.Println(append(s, 2)[i])"},
		{"slice-acap", "s, j := make([]int, 2, 3), 4\n\tfmt.Println(len(s[:j]))"},
		{"slice-acap-from", "s, i := make([]int, 2, 3), 4\n\tfmt.Println(len(s[i:]))"},
		{"slice-b", "s, i, j := []int{1, 2, 3}, 2, 1\n\tfmt.Println(len(s[i:j]))"},
		{"slice-neg-low", "s, i := []int{1, 2, 3}, -1\n\tfmt.Println(len(s[i:]))"},
		{"slice-uint-high", "s, j := []int{1, 2, 3}, uint64(1<<64-1)\n\tfmt.Println(len(s[:j]))"},
		{"slice-nil-sub", "var s []int\n\tj := 1\n\tfmt.Println(len(s[:0]), len(s[:j]))"},
		{"slice3-max", "s, k := []int{1, 2, 3}, 5\n\tfmt.Println(len(s[0:1:k]))"},
		{"slice3-b", "s, j, k := []int{1, 2, 3}, 3, 2\n\tfmt.Println(len(s[0:j:k]))"},
		{"slice3-c", "s, i, j := []int{1, 2, 3}, 2, 1\n\tfmt.Println(len(s[i:j:3]))"},
		{"slice-range", "s, i := []int{1, 2, 3}, 4\n\tfor range s[i:] {\n\t}"},
		{"slice-append-sub", "s, j := []int{1, 2, 3}, 9\n\ts = append(s[:j], 4)"},
		{"makeslice-len", "n := -1\n\tfmt.Println(len(make([]int, n)))"},
		{"makeslice-cap", "n, c := 3, 2\n\tfmt.Println(len(make([]int, n, c)))"},
		{"makeslice-huge", "n := 1 << 62\n\tfmt.Println(len(make([]int, n)))"},
		{"makeslice-uint", "var n uint64 = 1 << 63\n\tfmt.Println(len(make([]byte, n)))"},
		{"makeslice-cap-huge", "n, c := 1, 1<<60\n\tfmt.Println(len(make([]int, n, c)))"},
		// Arrays (step 3.6): index, slice-expression and conversion panics, and the nil
		// dereference of a nil pointer to an array (p[i], p[:], &p[i], range with a value,
		// range over *p with two variables), but not where Go doesn't evaluate it (len(p) and
		// for i := range *p).
		{"array-index", "a, i := [3]int{1, 2, 3}, 5\n\tfmt.Println(a[i])"},
		{"array-index-neg", "a, i := [3]string{}, -1\n\tfmt.Println(\"before\")\n\ta[i] = \"x\""},
		{"array-index-uint8", "var a [3]bool\n\tvar i uint8 = 200\n\ta[i] = true"},
		{"array-op-assign", "a, i := [2]float64{1}, 2\n\ta[i] *= 2"},
		{"array-inc", "a, i := [2][2]uint8{}, 3\n\ta[1][i]++"},
		{"array-empty", "var a [0]int\n\ti := 0\n\tfmt.Println(a[i])"},
		{"array-call", "i := 4\n\tfmt.Println(two()[i])"},
		{"array-addr", "a, i := [3]int{}, 3\n\tp := &a[i]\n\tfmt.Println(*p)"},
		{"array-ptr-index", "var p *[3]int\n\tfmt.Println(len(p), cap(p))\n\tfmt.Println(p[0])"},
		{"array-ptr-index-bad", "var p *[3]int\n\ti := 7\n\tp[i] = 1"},
		{"array-ptr-addr", "var p *[3]int\n\tq := &p[0]\n\tfmt.Println(q == nil)"},
		{"array-ptr-slice", "var p *[3]int\n\tfor i := range *p {\n\t\tfmt.Print(i)\n\t}\n\tfmt.Println(len(p[1:]))"},
		{"array-ptr-range", "var p *[3]int\n\tfor i, _ := range p {\n\t\tfmt.Print(i)\n\t}\n\tfor i, v := range p {\n\t\tfmt.Println(i, v)\n\t}"},
		{"array-deref-range", "var p *[3]int\n\tfor i, _ := range *p {\n\t\tfmt.Println(i)\n\t}"},
		{"array-slice", "a, j := [3]int{}, 4\n\tp := &a\n\tfmt.Println(len(p[:j]))"},
		{"array-slice-boxed", "a, i, j := [3]int{}, 2, 1\n\tfmt.Println(len(a[i:j]))"},
		{"array-slice3", "a, k := [3]string{}, 4\n\tfmt.Println(len(a[0:1:k]))"},
		{"array-slice-empty", "var a [0]int\n\tj := 1\n\tfmt.Println(len(a[:0]), len(a[:j]))"},
		{"to-array", "s := []int{1, 2}\n\ta := [3]int(s)\n\tfmt.Println(a[0])"},
		{"to-array-ptr", "s := make([]string, 2, 10)\n\tp := (*[3]string)(s)\n\tfmt.Println(p[0])"},
		{"to-array-ptr-nil", "var s []int\n\tfmt.Println((*[0]int)(s) == nil, len([0]int(s)))\n\tfmt.Println((*[1]int)(s) == nil)"},
		// Maps (step 3.7): a store into a nil map panics (reads don't), also through op=, ++
		// and append; the value of a store is evaluated before the entry is taken, and so is the
		// divisor of /= and %= (gc rewrites m[k] /= y to m[k] = m[k] / y), while <<= takes the
		// entry before it checks the count.
		{"map-nil-store", "var m map[string]int\n\tfmt.Println(len(m), m[\"a\"])\n\tm[\"a\"] = 1"},
		{"map-nil-inc", "var m map[int]float64\n\tm[1]++"},
		{"map-nil-op-assign", "var m map[string]string\n\tm[\"a\"] += \"x\""},
		{"map-nil-append", "var m map[int][]int\n\tm[1] = append(m[1], 2)"},
		{"map-nil-inner", "m := map[string]map[string]int{\"a\": nil}\n\tfmt.Println(len(m[\"a\"]), len(m[\"b\"]))\n\tm[\"a\"][\"b\"] = 1"},
		{"map-nil-shl", "var m map[string]int\n\tn := -1\n\tm[\"a\"] <<= n"},
		{"map-shl", "m := map[string]int{}\n\tn := -1\n\tm[\"a\"] <<= n"},
		{"map-nil-div", "var m map[string]int\n\tz := 0\n\tm[\"a\"] /= z"},
		{"map-nil-rem8", "var m map[string]int8\n\tvar z int8\n\tm[\"a\"] %= z"},
		{"map-nil-value-first", "var m map[string]int\n\tz := 0\n\tm[\"a\"] = 7 / z"},
		{"map-nil-op-value-first", "var m map[string]int\n\tz := 0\n\tm[\"a\"] += 7 / z"},
		{"map-nil-append-arg-first", "var m map[int][]int\n\tz := 0\n\tm[1] = append(m[1], 7/z)"},
		{"map-slice-index", "m := map[string][]int{}\n\tfmt.Println(m[\"x\"][0])"},
		{"map-nil-swap", "var m map[int]int\n\tm[1], m[2] = m[2], m[1]"},
		// Structs (step 3.8): a field through a nil pointer (read, store, op=, ++, &p.f, a
		// promoted field through a nil embedded pointer, a nested pointer), and an index out of
		// range in a field.
		{"struct-nil-read", "var p *Pt\n\tfmt.Println(\"before\")\n\tfmt.Println(p.a)"},
		{"struct-nil-second", "var p *Pt\n\tfmt.Println(p.b)"},
		{"struct-nil-store", "var p *Pt\n\tp.b = 1"},
		{"struct-nil-op-assign", "var p *Pt\n\tp.a += 2"},
		{"struct-nil-inc", "var p *Pt\n\tp.b++"},
		{"struct-nil-addr", "var p *Pt\n\tq := &p.b\n\tfmt.Println(q == nil)"},
		{"struct-nil-deref", "var p *Pt\n\tv := *p\n\tfmt.Println(v)"},
		{"struct-nil-embedded", "var w Wrap\n\tfmt.Println(w.Pt == nil)\n\tfmt.Println(w.a)"},
		{"struct-nil-nested", "l := &List{}\n\tfmt.Println(l.next == nil)\n\tl.next.v = 3"},
		{"struct-nil-call", "fmt.Println(nilPt().b)"},
		{"struct-index", "var s Arrs\n\ti := 3\n\ts.a[i] = 1"},
		// defer (step 3.13): a panic runs the deferred calls, then goes on; a deferred call that
		// panics adds its panic to the chain ("panic: A" then "\tpanic: B", also on a normal
		// return), and the others still run; a nil dereference's signal line comes once, after
		// the chain; a panic while a deferred call's arguments are evaluated.
		{"defer-runs", "defer fmt.Println(\"deferred\")\n\tfmt.Println(\"body\")\n\tpanic(\"boom\")"},
		{"defer-chain", "defer fmt.Println(\"last\")\n\tdefer boom(\"second\")\n\tpanic(\"first\")"},
		{"defer-chain3", "defer boom(\"third\")\n\tdefer fmt.Println(\"between\")\n\tdefer boom(\"second\")\n\tpanic(\"first\")"},
		{"defer-return", "defer fmt.Println(\"still runs\")\n\tdefer boom(\"on return\")\n\tfmt.Println(\"body\")"},
		{"defer-return-chain", "defer boom(\"B\")\n\tdefer boom(\"A\")"},
		{"defer-int", "defer boom(\"after\")\n\tpanic(42)"},
		{"defer-multiline", "defer boom(\"x\\ny\")\n\tpanic(\"a\\nb\")"},
		{"defer-nil-first", "var p *int\n\tdefer boom(\"after nil\")\n\tfmt.Println(*p)"},
		{"defer-nil-later", "defer fmt.Println(deref(nil))\n\tpanic(\"first\")"},
		{"defer-nil-in-defer", "defer deref(nil)\n\tpanic(\"first\")"},
		{"defer-arg-panic", "a, b := 1, 0\n\tdefer fmt.Println(\"first defer\")\n\tdefer fmt.Println(a / b)\n\tfmt.Println(\"not reached\")"},
		{"defer-index", "s, i := []int{1}, 3\n\tdefer fmt.Println(\"deferred\", s[0])\n\ts[0] = 9\n\tfmt.Println(s[i])"},
		{"defer-loop", "for i := range 3 {\n\t\tdefer fmt.Println(\"loop\", i)\n\t}\n\tpanic(\"after the loop\")"},
		// Go prints two neighbouring panics of the chain whose values have the same interface
		// words once, as "[recovered, repanicked]" (gocpp/panic.hpp, panic_identity): the same
		// constant, a bool or 1-byte value, a small integer (< 256), 0.0, "", and the runtime's
		// global errors are; a larger or negative integer, a string that is not a constant, a
		// constant next to a variable, two types, and an index error are not.
		{"rp-const", "defer panic(\"x\")\n\tpanic(\"x\")"},
		{"rp-const-int", "defer panic(5)\n\tpanic(5)"},
		{"rp-small", "n := 7\n\tdefer panic(n)\n\tpanic(n)"},
		{"rp-bool", "b := true\n\tdefer panic(b)\n\tpanic(b)"},
		{"rp-empty", "e := \"\"\n\tdefer panic(e)\n\tpanic(e)"},
		{"rp-int8", "var i8 int8 = -1\n\tdefer panic(i8)\n\tpanic(i8)"},
		{"rp-byte-const", "var c uint8 = 7\n\tdefer panic(c)\n\tpanic(uint8(7))"},
		{"rp-zero", "f := 0.0\n\tdefer panic(f)\n\tpanic(f)"},
		{"rp-nil", "defer deref(nil)\n\tvar p *int\n\tfmt.Println(*p)"},
		{"rp-div", "z := 0\n\tdefer div(1, z)\n\tfmt.Println(1 / z)"},
		{"rp-nil-map", "var m map[int]int\n\tdefer setm(m)\n\tm[2] = 2"},
		{"rp-three", "defer panic(5)\n\tdefer panic(5)\n\tpanic(5)"},
		{"rp-aab", "defer panic(\"B\")\n\tdefer panic(\"A\")\n\tpanic(\"A\")"},
		{"rp-abb", "defer panic(\"B\")\n\tdefer panic(\"B\")\n\tpanic(\"A\")"},
		{"rp-big", "big := 100000\n\tdefer panic(big)\n\tpanic(big)"},
		{"rp-str", "s := \"x\"\n\tdefer panic(s)\n\tpanic(s)"},
		{"rp-neg", "m1 := -1\n\tdefer panic(m1)\n\tpanic(m1)"},
		{"rp-int16", "var h int16 = -1\n\tdefer panic(h)\n\tpanic(h)"},
		{"rp-const-var", "x := \"x\"\n\tdefer panic(x)\n\tpanic(\"x\")"},
		{"rp-const-var-int", "n := 5\n\tdefer panic(n)\n\tpanic(5)"},
		{"rp-types", "a, b := 5, int64(5)\n\tdefer panic(b)\n\tpanic(a)"},
		{"rp-uintptr", "var u uintptr = 5\n\tdefer panic(u)\n\tpanic(uint(5))"},
		{"rp-index", "s, i := []int{1}, 5\n\tdefer at(s, i)\n\tfmt.Println(s[i])"},
		{"rp-float-bits", "f, g := 1.0, 1.0\n\tdefer panic(g)\n\tpanic(f)"},
		// gc's escape analysis makes a local variable that only its declaration gives a
		// constant that constant (panicid.go); a reassigned or address-taken one, a parameter
		// and a package variable stay values.
		{"rp-reassigned", "n := -1\n\tn--\n\tdefer panic(n)\n\tpanic(n)"},
		{"rp-reassigned-small", "n := 5\n\tn++\n\tdefer panic(n)\n\tpanic(n)"},
		{"rp-addr", "n := -1\n\tp := &n\n\t_ = p\n\tdefer panic(n)\n\tpanic(n)"},
		{"rp-param", "fmt.Println(deferPanic(-1))"},
		{"rp-pkgvar", "defer panic(gbig)\n\tpanic(gbig)"},
		{"rp-chain-var", "a := 500\n\tb := a\n\tdefer panic(b)\n\tpanic(500)"},
		{"rp-redeclared", "a := 500\n\ta, c := 600, 1\n\t_ = c\n\tdefer panic(a)\n\tpanic(a)"},
		{"rp-int64-conv", "n := 100000\n\tdefer panic(int64(n))\n\tpanic(int64(n))"},
		{"rp-uintptr-conv", "var u uint = 100000\n\tdefer panic(uintptr(u))\n\tpanic(uintptr(u))"},
		{"rp-int32-conv", "n := 100000\n\tdefer panic(int32(n))\n\tpanic(int32(n))"},
		{"rp-float-conv", "f := 1.5\n\tdefer panic(float64(f))\n\tpanic(float64(f))"},
		{"rp-var-decl", "var s = \"long string\"\n\tdefer panic(s)\n\tpanic(\"long string\")"},
		// recover (step 5.10): a panic that a deferred call recovers stays in the chain as
		// "[recovered]" when that call panics again, and panicking again with the recovered value is
		// the same panic ("[recovered, repanicked]", also for values that gc doesn't keep in static
		// data and runtime errors), panic(nil) is a *runtime.PanicNilError, and a recovered nil
		// dereference's signal line stays until the recovering call returns.
		{"rec-repanic", "defer func() {\n\t\tr := recover()\n\t\tpanic(r)\n\t}()\n\tpanic(\"x\")"},
		{"rec-repanic-big", "n := 100000\n\tdefer func() { panic(recover()) }()\n\tpanic(n)"},
		{"rec-repanic-str", "s := fmt.Sprint(\"a\", 1)\n\tdefer func() { panic(recover()) }()\n\tpanic(s)"},
		{"rec-new-panic", "defer func() {\n\t\trecover()\n\t\tpanic(\"second\")\n\t}()\n\tpanic(\"first\")"},
		{"rec-new-same-text", "n := 100000\n\tdefer func() {\n\t\trecover()\n\t\tpanic(n)\n\t}()\n\tpanic(n)"},
		{"rec-chain", "defer boom(\"third\")\n\tdefer func() {\n\t\trecover()\n\t\tpanic(\"second\")\n\t}()\n\tpanic(\"first\")"},
		{"rec-chain-recovered-late", "defer func() {\n\t\tr := recover()\n\t\tfmt.Println(\"recovered\", r)\n\t\tpanic(\"final\")\n\t}()\n\tdefer boom(\"second\")\n\tpanic(\"first\")"},
		{"rec-nil", "fmt.Println(\"before\")\n\tpanic(nil)"},
		{"rec-nil-repanic", "defer func() { panic(recover()) }()\n\tpanic(nil)"},
		{"rec-nil-new", "defer func() {\n\t\tfmt.Println(recover())\n\t\tpanic(\"after\")\n\t}()\n\tpanic(nil)"},
		{"rec-runtime-repanic", "defer func() { panic(recover()) }()\n\tvar m map[int]int\n\tm[1] = 1"},
		{"rec-index-repanic", "defer func() { panic(recover()) }()\n\ts, i := []int{1}, 5\n\tfmt.Println(s[i])"},
		{"rec-div-repanic", "z := 0\n\tdefer func() { panic(recover()) }()\n\tfmt.Println(1 / z)"},
		{"rec-nil-deref-repanic", "defer func() { panic(recover()) }()\n\tvar p *int\n\tfmt.Println(*p)"},
		{"rec-nil-deref-new", "defer func() {\n\t\trecover()\n\t\tpanic(\"new\")\n\t}()\n\tvar p *int\n\tfmt.Println(*p)"},
		{"rec-nil-deref-later", "func() {\n\t\tdefer func() { recover() }()\n\t\tvar p *int\n\t\tfmt.Println(*p)\n\t}()\n\tpanic(\"later\")"},
		{"rec-nil-deref-outer", "defer func() {\n\t\tfunc() {\n\t\t\tdefer func() { recover() }()\n\t\t\tvar q *int\n\t\t\tfmt.Println(*q)\n\t\t}()\n\t}()\n\tpanic(\"outer\")"},
		{"rec-named-result-then-panic", "defer func() { panic(\"in defer\") }()\n\tfunc() (n int) {\n\t\tdefer func() { n = 2; recover() }()\n\t\tpanic(1)\n\t}()"},
		{"rec-deferred-recover-not-direct", "defer func() {\n\t\tfunc() { recover() }()\n\t}()\n\tpanic(\"not recovered\")"},
		{"rec-defer-recover-builtin", "defer recover()\n\tpanic(\"still panics\")"},
		{"rec-helper-not-deferred", "defer func() { helperRecover() }()\n\tpanic(\"helper\")"},
		// Complex numbers (step 3.15c): printed like println; gc converts a value with a new
		// allocation (a chain prints both), a constant from read-only data (printed once).
		{"panic-complex", "c := complex(1.5, -2)\n\tpanic(c * c)"},
		{"panic-complex64", "var c complex64 = 1i\n\tc /= 0\n\tpanic(c)"},
		{"panic-complex-const", "panic(1 + 2i)"},
		{"rp-complex-const", "defer panic(1i)\n\tpanic(1i)"},
		{"rp-complex-const64", "defer panic(complex64(2))\n\tpanic(complex64(2))"},
		{"rp-complex-consts", "defer panic(1 + 1i)\n\tpanic(1 + 2i)"},
		{"rp-complex-var", "c := 1i\n\tc++\n\tdefer panic(c)\n\tpanic(c)"},
		{"rp-complex-single-def", "c := 2i\n\tdefer panic(c)\n\tpanic(c)"},
		// Named types (step 3.9): division and shifts of named integers (also op= and a named
		// count), indexes and bounds of named slices, strings and arrays (also a named index),
		// a nil named map, a nil named pointer, and a short slice converted to a named array.
		{"named-div", "a, b := N(7), N(0)\n\tfmt.Println(a / b)"},
		{"named-rem-small", "var a, b U8 = 7, 0\n\tfmt.Println(a % b)"},
		{"named-op-assign", "x, z := N(5), N(0)\n\tx /= z\n\tfmt.Println(x)"},
		{"named-shift", "x, n := N(1), -1\n\tfmt.Println(x << n)"},
		{"named-shift-count", "var k N = -2\n\tfmt.Println(1 << k)"},
		{"named-slice-index", "ids := IDs{1, 2}\n\ti := 2\n\tfmt.Println(ids[i])"},
		{"named-slice-named-index", "ids := IDs{1, 2}\n\tvar i N = 5\n\tids[i] = 1"},
		{"named-slice-bounds", "ids, j := IDs{1, 2}, 3\n\tfmt.Println(len(ids[:j]))"},
		{"named-append-index", "ids := append(IDs{}, 1)\n\tfmt.Println(ids[1])"},
		{"named-string-index", "n, i := Name(\"ab\"), 2\n\tfmt.Println(n[i])"},
		{"named-array-index", "var g Grid\n\ti := 3\n\tg[i]++"},
		{"named-map-nil", "var m Ages\n\tm[\"a\"] = 1"},
		{"named-map-div", "m := Ages{\"a\": 1}\n\tz := 0\n\tm[\"a\"] /= z"},
		{"named-nil-pointer", "var l Link\n\tfmt.Println(\"before\")\n\tfmt.Println(l.a)"},
		{"named-to-array", "ids := IDs{1}\n\tg := Grid(ids)\n\tfmt.Println(g[0])"},
		// Methods (step 3.10): a value method through a nil pointer panics at the call (also a
		// deferred one, at the defer statement, and one promoted through a nil embedded
		// pointer), a pointer method gets the nil pointer and panics where it dereferences it;
		// the receiver is read after the calls of the arguments (gnil is set to nil by clear),
		// and an index of the receiver panics after them.
		{"method-nil-value", "var p *Pt\n\tfmt.Println(\"before\")\n\tfmt.Println(p.sum())"},
		{"method-nil-pointer", "var p *Pt\n\tfmt.Println(p.isNil())\n\tp.set(1)"},
		{"method-nil-defer", "var p *Pt\n\tdefer fmt.Println(\"deferred\")\n\tdefer p.sum()\n\tfmt.Println(\"not reached\")"},
		{"method-nil-embedded", "var w Wrap\n\tfmt.Println(w.isNil())\n\tfmt.Println(w.sum())"},
		{"method-nil-after-args", "gnil = &Pt{1, 2}\n\tfmt.Println(gnil.add(clear()))"},
		{"method-index-after-args", "s := []Pt{{1, 2}}\n\ti := 1\n\ts[i].set(clear())"},
		{"method-div", "p := Pt{1, 0}\n\tfmt.Println(p.div())"},
		{"method-div-pointer", "p := &Pt{1, 0}\n\tp.divSet()"},
		// Method values and expressions (step 3.10b): a value method's value through a nil
		// pointer panics where it is made, after the calls of its scope; a pointer method's value
		// binds the nil pointer and panics in the method; (*T).M of a value method panics with
		// Go's wrapper message; a panic in a method called through a method value.
		{"method-value-nil", "var p *Pt\n\tfmt.Println(\"before\")\n\tf := p.sum\n\tfmt.Println(\"not reached\", f())"},
		{"method-value-nil-after-args", "gnil = &Pt{1, 2}\n\tfmt.Println(call(gnil.sum, clear()))"},
		{"method-value-nil-pointer", "var p *Pt\n\tf := p.set\n\tfmt.Println(\"bound\")\n\tf(1)"},
		{"method-expr-nil", "f := (*Pt).sum\n\tfmt.Println(\"made\")\n\tfmt.Println(f(nil))"},
		{"method-expr-nil-call", "var p *Pt\n\tfmt.Println((*Pt).sum(p))"},
		{"method-value-div", "p := Pt{1, 0}\n\tf := p.div\n\tdefer fmt.Println(\"deferred\")\n\tfmt.Println(f())"},
		// Func values (step 3.11): calling nil panics like a nil dereference (with the signal
		// line), after the arguments; also a nil field, map value, element and deferred func (when
		// the deferred call runs); a panic in a closure, in a deferred closure, and in a closure
		// in a package variable.
		{"func-nil", "var fn func()\n\tfmt.Println(\"before\")\n\tfn()"},
		{"func-nil-args-first", "var fn func(int)\n\targ := func() int { fmt.Println(\"arg\"); return 1 }\n\tfn(arg())"},
		{"func-nil-field", "type H struct{ f func() }\n\tvar h H\n\th.f()"},
		{"func-nil-map", "m := map[string]func(){}\n\tfmt.Println(len(m))\n\tm[\"x\"]()"},
		{"func-nil-elem", "fs := make([]func(int) int, 2)\n\tfmt.Println(fs[1](3))"},
		{"func-nil-deferred", "var fn func()\n\tdefer fn()\n\tfmt.Println(\"body\")"},
		{"func-nil-result", "mkf := func() func() { return nil }\n\tmkf()()"},
		{"func-closure-panic", "n := 0\n\tinc := func() {\n\t\tn++\n\t\tif n == 2 {\n\t\t\tpanic(n)\n\t\t}\n\t}\n\tinc()\n\tinc()"},
		{"func-deferred-closure", "defer func() { panic(\"in defer\") }()\n\tpanic(\"body\")"},
		{"func-pkg-closure", "fmt.Println(gfn(2))\n\tfmt.Println(gfn(0))"},
		// The audit of step 3.14b: the integer sizes of division, modulo and shift counts that
		// the cases above leave out (an unsigned count never panics), the 3-index forms of
		// arrays and pointers to arrays, a nil pointer reached through a pointer to a pointer, to
		// an array (copied) and to a struct's array field, make with a bad capacity, a small
		// index type and an element too big for the length, and make of a map with a negative
		// hint, which doesn't panic, and nor do delete, clear, a read and a range of a nil map.
		{"div16", "var a, b int16 = 7, 0\n\tfmt.Println(a / b)"},
		{"div32", "var a, b int32 = 7, 0\n\tfmt.Println(a / b)"},
		{"rem64", "var a, b int64 = 7, 0\n\tfmt.Println(a % b)"},
		{"div-u8", "var a, b uint8 = 7, 0\n\tfmt.Println(a / b)"},
		{"rem-u32", "var a, b uint32 = 7, 0\n\tfmt.Println(a % b)"},
		{"div-u64", "var a, b uint64 = 7, 0\n\tfmt.Println(a / b)"},
		{"rem-uintptr", "var a, b uintptr = 7, 0\n\tfmt.Println(a % b)"},
		{"div-min", "var a int64 = -1 << 63\n\tb, z := int64(-1), int64(0)\n\tfmt.Println(a/b, a%b)\n\tfmt.Println(a / z)"},
		{"shl-count16", "var n int16 = -1\n\tfmt.Println(1 << n)"},
		{"shr-count32", "x := -8\n\tvar n int32 = -5\n\tfmt.Println(x >> 70, x >> n)"},
		{"shl-count64", "var x uint8 = 1\n\tvar n int64 = -1 << 63\n\tfmt.Println(x<<200, x << n)"},
		{"shl-unsigned-count", "var n uint64 = 1 << 63\n\tx := 3\n\tfmt.Println(x<<n, x>>n, -x>>n)\n\tm := -1\n\tfmt.Println(x << m)"},
		{"array-index-int8", "var a [3]int\n\tvar i int8 = -1\n\tfmt.Println(a[i])"},
		{"array-slice3-b", "a, j, k := [3]int{}, 3, 2\n\tfmt.Println(len(a[0:j:k]))"},
		{"array-slice3-c", "a, i, j := [3]int{}, 2, 1\n\tfmt.Println(len(a[i:j:3]))"},
		{"array-ptr-slice3", "a, k := [3]int{}, 4\n\tp := &a\n\tfmt.Println(len(p[0:1:k]))"},
		{"array-ptr-slice3-b", "a, j, k := [3]int{}, 2, 1\n\tp := &a\n\tfmt.Println(len(p[:j:k]))"},
		{"array-ptr-nil-slice3", "var p *[3]int\n\tfmt.Println(len(p[0:1:2]))"},
		{"slice3-uint", "s, k := []int{1}, uint64(1<<64-1)\n\tfmt.Println(len(s[0:0:k]))"},
		{"string-slice-uint8", "s, i, j := \"abc\", uint8(2), uint8(1)\n\tfmt.Println(s[i:j])"},
		{"nil-ptr-ptr", "var q *int\n\tpp := &q\n\tfmt.Println(*pp == nil)\n\t**pp = 1"},
		{"array-nil-copy", "var p *[3]int\n\ta := *p\n\tfmt.Println(a[0])"},
		{"struct-nil-array-field", "var p *Arrs\n\tp.a[0] = 1"},
		{"makeslice-cap-neg", "c := -1\n\tfmt.Println(len(make([]int, 0, c)))"},
		{"makeslice-int8", "var n int8 = -1\n\tfmt.Println(len(make([]string, n)))"},
		{"makeslice-big-elem", "n := 1 << 30\n\tfmt.Println(len(make([]Big, n)))"},
		{"map-nil-delete", "var m map[string]int\n\tdelete(m, \"a\")\n\tclear(m)\n\tv, ok := m[\"a\"]\n\tfor range m {\n\t}\n\tfmt.Println(len(m), v, ok)\n\tm[\"a\"] = 1"},
		{"makemap-neg", "n := -1\n\tm := make(map[int]int, n)\n\tvar h int64 = -1 << 63\n\tfmt.Println(len(m), len(make(map[string]bool, h)))\n\tpanic(\"after\")"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			src := "package main\n\n"
			if strings.Contains(tt.body, "fmt.") {
				src += "import \"fmt\"\n\n"
			}
			src += "func main() {\n\t" + tt.body + "\n}\n"
			if strings.Contains(tt.body, "deref(") {
				src += "\nfunc deref(p *int) int {\n\treturn *p\n}\n"
			}
			if strings.Contains(tt.body, "mk(") {
				src += "\nfunc mk() []int {\n\treturn []int{1}\n}\n"
			}
			if strings.Contains(tt.body, "deferPanic(") {
				src += "\nfunc deferPanic(n int) int {\n\tdefer panic(n)\n\tpanic(n)\n}\n"
			}
			if strings.Contains(tt.body, "gbig") {
				src += "\nvar gbig = 100000\n"
			}
			if strings.Contains(tt.body, "gfn(") {
				src += "\nvar gfn = func(n int) int {\n\treturn 10 / n\n}\n"
			}
			if strings.Contains(tt.body, "div(") {
				src += "\nfunc div(a, b int) int {\n\treturn a / b\n}\n"
			}
			if strings.Contains(tt.body, "setm(") {
				src += "\nfunc setm(m map[int]int) {\n\tm[1] = 1\n}\n"
			}
			if strings.Contains(tt.body, "at(s") {
				src += "\nfunc at(s []int, i int) int {\n\treturn s[i]\n}\n"
			}
			if strings.Contains(tt.body, "boom(") {
				src += "\nfunc boom(s string) {\n\tpanic(s)\n}\n"
			}
			if strings.Contains(tt.body, "helperRecover(") {
				src += "\nfunc helperRecover() {\n\trecover()\n}\n"
			}
			if strings.Contains(tt.body, "two(") {
				src += "\nfunc two() [2]int {\n\treturn [2]int{1, 2}\n}\n"
			}
			if strings.Contains(tt.body, "Big") {
				src += "\ntype Big struct{ a [1 << 20]byte }\n"
			}
			if strings.HasPrefix(tt.name, "named-") {
				src += "\ntype N int\n\ntype U8 uint8\n\ntype IDs []int\n\ntype Name string\n\ntype Grid [3]int\n\n" +
					"type Ages map[string]int\n\ntype Pt struct{ a int }\n\ntype Link *Pt\n"
			}
			if strings.HasPrefix(tt.name, "method-") {
				src += "\ntype Pt struct{ a, b int }\n\ntype Wrap struct{ *Pt }\n\nvar gnil *Pt\n\n" +
					"func (p Pt) sum() int {\n\treturn p.a + p.b\n}\n\nfunc (p Pt) add(n int) int {\n\treturn p.a + n\n}\n\n" +
					"func (p Pt) div() int {\n\treturn p.a / p.b\n}\n\nfunc (p *Pt) divSet() {\n\tp.a /= p.b\n}\n\n" +
					"func (p *Pt) set(v int) {\n\tp.a = v\n}\n\nfunc (p *Pt) isNil() bool {\n\treturn p == nil\n}\n\n" +
					"func clear() int {\n\tprintln(\"clear\")\n\tgnil = nil\n\treturn 1\n}\n\n" +
					"func call(f func() int, n int) int {\n\treturn f() + n\n}\n"
			}
			if strings.HasPrefix(tt.name, "struct-") {
				src += "\ntype Pt struct{ a, b int }\n\ntype Wrap struct{ *Pt }\n\ntype List struct {\n\tv    int\n\tnext *List\n}\n\n" +
					"type Arrs struct{ a [3]int }\n\nfunc nilPt() *Pt {\n\treturn nil\n}\n"
			}
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goResult(t, gp)
			if want != nil && want.code != 2 {
				t.Fatalf("with Go, the program exits with %d, not with a panic:\n%s", want.code, want.stderr)
			}
			checkRunsNorm(t, emitter(gp), want, maskPCAddr)
			if noChecksStay[tt.name] {
				// A -nochecks build panics the same way: these checks stay (§2.9).
				// (An index of a [0]T stays, though its message is an index error.)
				if want != nil && tt.name != "array-empty" && removedRx.MatchString(want.stderr) {
					t.Fatalf("noChecksStay has %s, but Go's panic is one that -nochecks removes:\n%s", tt.name, want.stderr)
				}
				t.Run("nochecks", func(t *testing.T) {
					checkRunsMode(t, emitter(gp), want, maskPCAddr, true)
				})
			}
		})
	}
}

// noChecksStay are the TestPanics cases that also run as a -nochecks build, which must print
// what Go prints: at least one for each check that -nochecks keeps (Phase 03 §2.9): division
// and modulo by zero (also of named types and in a method), MIN / -1 (Go's result), a
// negative shift count and Go's results for counts of the width or more, a store into a nil
// map (also through op= and append), make with a bad length or capacity, an index of a [0]T,
// the builtin panic, a deferred call that panics, and a panic in a closure. The cases that
// -nochecks changes (an index, a slice expression or a conversion out of range, a nil
// dereference or call, (*T).M with nil) are undefined behavior there, so they don't run.
var noChecksStay = map[string]bool{
	"div": true, "div8": true, "divu": true, "rem16": true, "div-assign": true, "rem-assign8": true,
	"div16": true, "div32": true, "rem64": true, "div-u8": true, "rem-u32": true, "div-u64": true,
	"rem-uintptr": true, "div-min": true, "named-div": true, "named-op-assign": true, "method-div": true,
	"method-value-div": true, "map-nil-div": true, "rp-div": true,
	"shl": true, "shr8": true, "shl-assign": true, "shr-assign16": true, "shl-count16": true,
	"shr-count32": true, "shl-count64": true, "shl-unsigned-count": true, "named-shift-count": true,
	"map-shl": true, "map-nil-shl": true,
	"map-nil-store": true, "map-nil-inc": true, "map-nil-op-assign": true, "map-nil-append": true,
	"map-nil-inner": true, "named-map-nil": true, "rp-nil-map": true, "map-nil-delete": true,
	"makeslice-len": true, "makeslice-cap": true, "makeslice-huge": true, "makeslice-uint": true,
	"makeslice-cap-huge": true, "makeslice-cap-neg": true, "makeslice-int8": true,
	"makeslice-big-elem": true, "makemap-neg": true, "array-empty": true,
	"panic-str": true, "panic-int": true, "defer-chain": true, "defer-return": true,
	"func-closure-panic": true, "func-pkg-closure": true,
}

// removedRx matches the panics whose checks -nochecks removes.
var removedRx = regexp.MustCompile(`panic: (runtime error: (index out of range|slice bounds|invalid memory address|cannot convert slice)|value method)`)

// TestNoChecksCalls checks that a -nochecks build keeps no bounds or nil check (Phase 03 §2.9,
// step 3.14b): the emitter emits no check of its own, it calls the runtime, whose checks
// GOCPP_NOCHECKS removes. So the C++ of each run test (tests/run) that exits normally, of the
// phases up to the one being built, compiled with -nochecks into assembly, calls none of the
// panics of those checks (panicCallRx), and so does noChecksFmtProg, which prints every kind of
// composite value with fmt. A run test that the emitter doesn't take yet is skipped. One
// compiler is enough: both see the same preprocessed C++. The runtime's own object files,
// compiled for -nochecks, use none of them either (nm -u).
func TestNoChecksCalls(t *testing.T) {
	progs, err := filepath.Glob(filepath.Join("..", "..", "tests", "run", "*.gp"))
	if err != nil || len(progs) == 0 {
		t.Fatalf("no run tests: %v", err)
	}
	// And the fmt shim with every kind of composite value (step 3.15b), which must add no
	// check of its own either.
	fmtProg := filepath.Join(t.TempDir(), "fmt_composites.gp")
	if err := os.WriteFile(fmtProg, []byte(noChecksFmtProg), 0o644); err != nil {
		t.Fatal(err)
	}
	progs = append(progs, fmtProg)
	var c *cxx
	for _, cc := range compilers(t) {
		if cc.missing == "" {
			c = cc
			break
		}
	}
	if c == nil {
		t.Skip("no C++ compiler")
	}
	phaseRx := regexp.MustCompile(`^// phase: (\d\d)\n`)
	checked := 0
	for _, prog := range progs {
		src, err := os.ReadFile(prog)
		if err != nil {
			t.Fatal(err)
		}
		m := phaseRx.FindSubmatch(src)
		if m == nil {
			t.Fatalf("%s: no phase line", prog)
		}
		if phase, _ := strconv.Atoi(string(m[1])); phase > version.Phase+1 || bytes.Contains(src, []byte("\n// ExitCode:")) {
			continue
		}
		checked++
		t.Run(strings.TrimSuffix(filepath.Base(prog), ".gp"), func(t *testing.T) {
			t.Parallel()
			abs, err := filepath.Abs(prog)
			if err != nil {
				t.Fatal(err)
			}
			cpp, err := emitPatterns(t, Config{}, filepath.Dir(abs), abs)
			if err != nil && prog == fmtProg {
				t.Fatal(err)
			}
			if err != nil {
				t.Skipf("not emitted yet: %v", err)
			}
			work := t.TempDir()
			file := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(file, []byte(cpp), 0o644); err != nil {
				t.Fatal(err)
			}
			asm := filepath.Join(work, "main.s")
			args := append(c.flags(append([]string{includeFlag(t)}, c.noChecksFlags()...)...), "-S", file, "-o", asm)
			out, _, code, err := command(work, true, c.cc.Path, args...)
			if err != nil || code != 0 {
				t.Fatalf("%s %s: exit code %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
			}
			text, err := os.ReadFile(asm)
			if err != nil {
				t.Fatal(err)
			}
			if calls := panicCallRx.FindAllString(string(text), -1); len(calls) != 0 {
				t.Errorf("the -nochecks build still calls %q", slices.Compact(slices.Sorted(slices.Values(calls))))
			}
		})
	}
	if checked == 0 {
		t.Fatal("no run test checked")
	}
	t.Run("runtime", func(t *testing.T) {
		// The runtime's own files, compiled for -nochecks: none of them refers to these panics
		// of another file (nm -u lists what an object uses from elsewhere).
		nm, err := exec.LookPath("nm")
		if err != nil {
			t.Skip("no nm")
		}
		for _, obj := range c.runtimeObjectsMode(t, true) {
			out, _, code, err := command(filepath.Dir(obj), false, nm, "-u", obj)
			if err != nil || code != 0 {
				t.Fatalf("nm -u %s: exit code %d %v", obj, code, err)
			}
			if calls := panicCallRx.FindAllString(string(out), -1); len(calls) != 0 {
				t.Errorf("%s (-nochecks) calls %q", obj, slices.Compact(slices.Sorted(slices.Values(calls))))
			}
		}
	})
}

// noChecksFmtProg prints composite values with fmt (TestNoChecksCalls).
const noChecksFmtProg = `// phase: 03

package main

import "fmt"

type P struct {
	X, Y int
	Name string
	Tags []string
}

type S struct{ n int }

func (s S) String() string { return fmt.Sprint("S", s.n) }

type E struct{ msg string }

func (e *E) Error() string { return e.msg }

func main() {
	p := &P{1, 2, "p", []string{"a", "b"}}
	var np *P
	grid := [2][3]int{{1, 2, 3}, {4, 5, 6}}
	m := map[string][]int{"b": {2}, "a": {1, 1}}
	ss := []S{{1}, {2}}
	fmt.Println(*p, p, np, grid, m, ss, S{3}, &E{"boom"}, []*P{p, nil})
	fmt.Printf("%v %+v %d %s %q %v\n", p, *p, grid, ss, p.Tags, m)
	s := fmt.Sprintf("%v|%v|%5.2f|%-4d|%t", m, grid[1][:2], 3.14159, 7, true)
	fmt.Println(s, fmt.Sprint(ss, 1, "x"), min(3, 1, 2), max(2.5, 1.0))
	var nilMap map[int]bool
	var nilSlice []int
	fmt.Println(nilMap, nilSlice, len(s))
}
`

// panicCallRx matches the mangled names of the panics of the checks that -nochecks removes
// (gocpp/checks.hpp, bounds.hpp): gp::panic_index, panic_bounds, panic_nil (not panic_nil_map,
// which stays) and panic_wrap.
var panicCallRx = regexp.MustCompile(`_ZN2gp(11panic_index|12panic_bounds|9panic_nil|10panic_wrap)E`)

// TestFatal checks Go's fatal errors (Phase 03 step 3.14b), which no recover stops and which
// run no deferred calls: running out of memory (make, new and a large local variable, each
// asking for 32 TiB or more, and a make in a deferred call that a panic runs) prints what Go
// prints up to its "fatal error:" line, then the goroutine's trace without Go's runtime frames
// (normFatal; for the deferred call, its frames, a panic frame and the panicking function at
// the panic's line), and exits with code 2; a
// recursion that overflows the stack (8 MiB here) prints Go's lines (want: Go 1.25's, checked
// by hand, since Go needs 1 GB of stack for it) without a trace. Each also as a -nochecks
// build. The out-of-memory cases need an allocation that big to fail: they are skipped where
// the kernel never refuses one (vm.overcommit_memory = 1). With a sanitizer
// (GOCPP_TEST_CXXFLAGS), which reports these itself, the test is skipped; in an optimized
// build, the traces aren't compared.
func TestFatal(t *testing.T) {
	if strings.Contains(os.Getenv("GOCPP_TEST_CXXFLAGS"), "-fsanitize=") {
		t.Skip("a sanitizer reports running out of memory and stack overflows itself")
	}
	const prelude = "package main\n\nimport \"fmt\"\n\n"
	tests := []struct {
		name, src string
		oom       bool
		want      *result // nil: Go's result
	}{
		{"oom-make", prelude + "func grow(n int) []string {\n\treturn make([]string, n/2, n)\n}\n\n" +
			"func main() {\n\tdefer fmt.Println(\"deferred\")\n\tfmt.Println(\"start\")\n\ts := grow(1 << 41)\n\tfmt.Println(len(s))\n}\n", true, nil},
		{"oom-new", prelude + "type Huge [1 << 45]byte\n\n" +
			"func main() {\n\tdefer fmt.Println(\"deferred\")\n\tfmt.Println(\"start\")\n\tp := new(Huge)\n\tfmt.Println(len(p))\n}\n", true, nil},
		{"oom-local", prelude + "func main() {\n\tdefer fmt.Println(\"deferred\")\n\tfmt.Println(\"start\")\n" +
			"\tvar a [1 << 45]byte\n\ta[7] = 1\n\tfmt.Println(a[7])\n}\n", true, nil},
		{"oom-defer", prelude + "func main() {\n\tfmt.Println(\"start\")\n\tdefer fmt.Println(\"not run\")\n\tdefer func() {\n" +
			"\t\t_ = make([]byte, 1<<45)\n\t}()\n\tn := 1\n\tpanic(fmt.Sprint(\"boom\", n))\n}\n", true, nil},
		// Fatal errors are not panics (step 5.10): a deferred recover doesn't stop them, and the
		// deferred calls don't run.
		{"oom-recover", prelude + "func main() {\n\tdefer func() { fmt.Println(\"recovered\", recover()) }()\n\tfmt.Println(\"start\")\n" +
			"\tn := 1 << 45\n\ts := make([]byte, n)\n\tfmt.Println(len(s))\n}\n", true, nil},
		{"stack-overflow-recover", prelude + "var depth int\n\nfunc f(n int) int {\n\tvar a [100]int\n\ta[n%100] = n\n\tdepth++\n\treturn f(n+1) + a[n%7]\n}\n\n" +
			"func main() {\n\tdefer func() { fmt.Println(\"recovered\", recover()) }()\n\tfmt.Println(\"start\")\n\tfmt.Println(f(0))\n}\n", false,
			&result{"start\n", "runtime: goroutine stack exceeds 1000000000-byte limit\nruntime: sp=0xX stack=[0xX, 0xX]\nfatal error: stack overflow\n", 2}},
		{"stack-overflow", prelude + "var depth int\n\nfunc f(n int) int {\n\tvar a [100]int\n\ta[n%100] = n\n\tdepth++\n\treturn f(n+1) + a[n%7]\n}\n\n" +
			"func main() {\n\tdefer fmt.Println(\"deferred\")\n\tfmt.Println(\"start\")\n\tfmt.Println(f(0))\n}\n", false,
			&result{"start\n", "runtime: goroutine stack exceeds 1000000000-byte limit\nruntime: sp=0xX stack=[0xX, 0xX]\nfatal error: stack overflow\n", 2}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.oom {
				if mode, err := os.ReadFile("/proc/sys/vm/overcommit_memory"); err != nil || strings.TrimSpace(string(mode)) == "1" {
					t.Skip("the kernel may give any allocation (vm.overcommit_memory = 1, or not Linux)")
				}
			} else if runtime.GOOS != "linux" {
				t.Skip("the stack overflow handler is Linux's (runtime/src/os/linux)")
			}
			dir := t.TempDir()
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(tt.src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := tt.want
			if want == nil {
				if want = goResult(t, gp); want == nil {
					return
				}
				want.stderr = normFatal(want.stderr)
			}
			// An optimized build (GOCPP_TEST_CXXFLAGS=-O2) can show a runtime header's line for
			// an inlined frame (cpp-mapping.md, "Where the trace differs"): only the lines up
			// to the goroutine's are compared then.
			optimized := slices.ContainsFunc(testFlags(), func(f string) bool { return strings.HasPrefix(f, "-O") && f != "-O0" })
			if optimized {
				w := *want
				w.stderr, _, _ = strings.Cut(w.stderr, goStackMark)
				want = &w
			}
			work := t.TempDir()
			src := filepath.Join(work, "main.gp.cpp")
			if err := os.WriteFile(src, []byte(emitter(gp)(t, Config{OutputPath: src})), 0o644); err != nil {
				t.Fatal(err)
			}
			for _, c := range compilers(t) {
				for _, nochecks := range []bool{false, true} {
					name := c.name
					if nochecks {
						name += "-nochecks"
					}
					t.Run(name, func(t *testing.T) {
						if c.missing != "" {
							t.Skip(c.missing)
						}
						// The stack has a limit of 8 MiB (ulimit -s), as by default, also where
						// the tests run without one (the recursion would fill the memory).
						bin := c.buildMode(t, src, nochecks)
						stdout, stderr, code, err := command(work, false, "/bin/sh", "-c", `ulimit -s 8192 2>/dev/null; exec "$0"`, bin)
						if err != nil {
							t.Fatal(err)
						}
						got := result{string(stdout), normFatal(string(stderr)), code}
						if optimized {
							got.stderr, _, _ = strings.Cut(got.stderr, goStackMark)
						}
						if got != *want {
							t.Errorf("the program prints\nstdout:\n%s\nstderr (normalized):\n%s\nexit code %d\nwant\nstdout:\n%s\nstderr:\n%s\nexit code %d\nwhole stderr:\n%s",
								got.stdout, got.stderr, got.code, want.stdout, want.stderr, want.code, stderr)
						}
					})
				}
			}
		})
	}
}

// hexRx matches a hexadecimal number.
var hexRx = regexp.MustCompile(`0x[0-9a-f]+`)

// normFatal rewrites the stderr of a fatal error so that Go's and gocpp's compare: the lines up
// to "fatal error: ..." with each hexadecimal number as 0xX, then, if there is a trace, the
// goroutine's (Go's after its "runtime stack:" section) as "goroutine 1 [running]:" and its
// frames without Go's runtime frames, normalized as normTrace does, with the panic frame as
// "panic()" alone (panicFrame). Go's other goroutines, after a blank line, are left out.
func normFatal(stderr string) string {
	lines := strings.Split(stderr, "\n")
	var b strings.Builder
	i := 0
	for ; i < len(lines); i++ {
		b.WriteString(hexRx.ReplaceAllString(lines[i], "0xX") + "\n")
		if strings.HasPrefix(lines[i], "fatal error: ") {
			i++
			break
		}
	}
	for i < len(lines) && !strings.HasPrefix(lines[i], "goroutine 1 ") {
		i++
	}
	if i == len(lines) || i+1 == len(lines) || lines[i+1] == "" {
		// No frames: gocpp prints Go's "goroutine N [running]:" line of a fatal error, without a
		// trace (runtime/thread_test.go); Go's own output has the frames.
		return b.String()
	}
	trace := "\n" + goStackMark
	for k := i + 1; k+1 < len(lines) && lines[k] != ""; k += 2 {
		if !strings.HasPrefix(lines[k], "runtime.") {
			trace += lines[k] + "\n" + lines[k+1] + "\n"
		}
	}
	return b.String() + panicFrame.ReplaceAllString(normTrace(trace), "panic()\n")
}

// addrRx matches the address in the signal line of a nil pointer panic, addr=0x8: Go prints the
// offset of a field that is read through a nil pointer, gocpp 0x0 (Phase 03 STATE, 3.2).
var addrRx = regexp.MustCompile(` addr=0x[0-9a-f]+ `)

// maskPCAddr is maskPC that also replaces the address of the signal line with ADDR.
func maskPCAddr(r result) result {
	r = maskPC(r)
	r.stderr = addrRx.ReplaceAllString(r.stderr, " addr=ADDR ")
	return r
}

// TestAnonNames checks the names of the anonymous structs that hold types declared in functions
// (typedecl.go: anonKey): two functions' types of the same name give two structs, and the names
// don't depend on the lines and columns of the declarations.
func TestAnonNames(t *testing.T) {
	const prog = "package main\n\nimport \"fmt\"\n\nfunc f() {\n%s\ttype T struct{ a int }\n\tfmt.Println(struct{ t T }{T{1}})\n}\n\n" +
		"func g() {\n\ttype T struct{ b string }\n\tfmt.Println(struct{ t T }{T{\"s\"}})\n}\n\nfunc main() {\n\tf()\n\tg()\n}\n"
	anonRx := regexp.MustCompile(`struct (gp_anon_[0-9a-f]+) \{`)
	names := func(src string) []string {
		dir := t.TempDir()
		gp := filepath.Join(dir, "main.gp")
		if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		cpp, err := emitPatterns(t, Config{}, dir, gp)
		if err != nil {
			t.Fatal(err)
		}
		var list []string
		for _, m := range anonRx.FindAllStringSubmatch(cpp, -1) {
			list = append(list, m[1])
		}
		return list
	}
	a := names(fmt.Sprintf(prog, ""))
	b := names(fmt.Sprintf(prog, "\t// a line above the type\n\n"))
	if len(a) != 2 || a[0] == a[1] {
		t.Errorf("the anonymous structs are %v, want two different ones", a)
	}
	if !slices.Equal(a, b) {
		t.Errorf("the names changed with the lines: %v, then %v", a, b)
	}
}
