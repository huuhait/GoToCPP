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
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
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
	var out bytes.Buffer
	err := EmitPackage(&out, r.Fset, pkg, cfg)
	if err != nil && out.Len() != 0 {
		t.Errorf("EmitPackage failed (%v) but wrote %d bytes", err, out.Len())
	}
	return out.String(), err
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
	inner := cString(dir + string(filepath.Separator))
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
			checkRuns(t, emit, goWant)
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
			got := run(t, work, c.build(t, src))
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
	own := `"` + strings.Trim(cString(out), `"`) + `"`
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
	out, _, code, err := command(dir, true, goCmd, args...)
	if err != nil || code != 0 {
		t.Fatalf("go build %s: exit code %d %v\n%s", strings.Join(gps, " "), code, err, out)
	}
	r := run(t, dir, bin)
	// After this line Go prints the stack trace of a panic, which gocpp doesn't have yet
	// (Phase 03): the output is compared up to it.
	if i := strings.Index(r.stderr, goStackMark); i >= 0 {
		r.stderr = r.stderr[:i+len(goStackMark)]
	}
	return &r
}

// goStackMark is the line of a panic message after which Go prints the stack trace.
const goStackMark = "goroutine 1 [running]:\n"

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
		{"stringconv", "func main() {\n\tr := 'x'\n\tprintln(string(r))\n}", "main.gp:5:10: cppgen: not supported yet: conversions between strings and bytes, runes or integers (Phase 03)"},
		{"method", "type T int\nfunc (T) M() {}\nfunc main() {}", "main.gp:3:1: cppgen: not supported yet: type declarations (Phase 03)"},
		{"funcvalue", "func g() {}\nfunc main() {\n\tf := g\n\tf()\n}", "main.gp:5:2: cppgen: not supported yet: type func()"},
		{"funcvalue2", "func g() {}\nfunc main() {\n\tprintln(g == nil)\n}", "main.gp:5:10: cppgen: not supported yet: function values (Phase 03)"},
		{"complex", "func main() {\n\tc := complex(1, 2)\n\tprintln(real(c))\n}", "main.gp:4:2: cppgen: not supported yet: type complex128"},
		{"printf", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%5d\\n\", 1)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %5d (Phase 03)`},
		{"printf-goverb", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%z %x\\n\", 1, 2)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %x (Phase 03)`},
		{"printf-precision", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%.2f\\n\", 1.5)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %.2f (Phase 03)`},
		{"printf-flag", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%-z\\n\", 1)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %-z (Phase 03)`},
		{"sprint", "import \"fmt\"\nfunc main() {\n\tprintln(fmt.Sprint(1))\n}", "main.gp:5:10: cppgen: not supported yet: fmt.Sprint: Phase 02 has only fmt.Print, fmt.Println and fmt.Printf"},
		{"errorresult", "import \"fmt\"\nfunc main() {\n\tn, err := fmt.Println()\n\tprintln(n, err)\n}", "main.gp:5:5: cppgen: not supported yet: type error"},
		{"rangestring", "func main() {\n\tfor i := range \"abc\" {\n\t\tprintln(i)\n\t}\n}", "main.gp:4:17: cppgen: not supported yet: range over strings (Phase 03)"},
		{"rangechan", "func main() {\n\tfor range make(chan int) {\n\t}\n}", "main.gp:4:12: cppgen: not supported yet: range over channels (Phase 08)"},
		{"defer", "func main() {\n\tdefer println()\n}", "main.gp:4:2: cppgen: not supported yet: defer statements (Phase 03)"},
		{"go", "func main() {\n\tgo println()\n}", "main.gp:4:2: cppgen: not supported yet: go statements (Phase 08)"},
		{"insideif", "func main() {\n\tif x := 1; x > 0 {\n\t\tdefer println()\n\t}\n}", "main.gp:5:3: cppgen: not supported yet: defer statements (Phase 03)"},
		{"nomain", "func f() {}", "main.gp:2:9: cppgen: function main is undeclared in the main package"},
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
// condition) and the builtin panic with values of basic types: each
// program prints what it prints with Go, exits with code 2, and stderr is Go's up to the
// "goroutine 1 [running]:" line (goResult).
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
		{"panic-str", "s := \"bo\"\n\tpanic(s + \"om\")"},
		{"panic-const", "panic(\"multi\\nline\")"},
		{"panic-int", "panic(42)"},
		{"panic-int8", "var x int8 = -7\n\tpanic(x * 2)"},
		{"panic-uint", "var x uint64 = 1 << 63\n\tpanic(x)"},
		{"panic-float", "panic(1.5)"},
		{"panic-float32", "var f float32 = 0.1\n\tpanic(f)"},
		{"panic-bool", "x := 1\n\tpanic(x > 0)"},
		{"panic-rune", "panic('x')"},
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
			gp := filepath.Join(dir, "main.gp")
			if err := os.WriteFile(gp, []byte(src), 0o644); err != nil {
				t.Fatal(err)
			}
			want := goResult(t, gp)
			if want != nil && want.code != 2 {
				t.Fatalf("with Go, the program exits with %d, not with a panic:\n%s", want.code, want.stderr)
			}
			checkRuns(t, emitter(gp), want)
		})
	}
}
