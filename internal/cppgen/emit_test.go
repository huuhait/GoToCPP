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
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/cppir"
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
	out, _, code, err := command(dir, true, goCmd, args...)
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
		{"makechan", "func main() {\n\t_ = make(chan int)\n}", "main.gp:4:6: cppgen: not supported yet: make of chan int (Phase 03)"},
		{"fmtmap", "import \"fmt\"\nfunc main() {\n\tfmt.Println(1, map[int]int{})\n}", "main.gp:5:17: cppgen: not supported yet: printing a map (Phase 03 step 3.15)"},
		{"bytes-named-elem", "type MyByte byte\nfunc main() {\n\t_ = []MyByte(\"ab\")\n}", "main.gp:5:6: cppgen: not supported yet: the conversion from string to []MyByte"},
		{"runes-named-elem", "type R rune\nfunc main() {\n\t_ = string([]R{'a'})\n}", "main.gp:5:6: cppgen: not supported yet: the conversion from []R to string"},
		{"slice-tags", "func main() {\n\t_ = []struct{ Q int }([]struct {\n\t\tQ int `t:\"q\"`\n\t}{})\n}", `main.gp:4:6: cppgen: not supported yet: the conversion from []struct{Q int "t:\"q\""} to []struct{Q int}`},
		{"struct-slice-tags", "type W struct{ S []struct{ Q int `t:\"q\"` } }\ntype V struct{ S []struct{ Q int } }\nfunc main() {\n\t_ = V(W{})\n}", "main.gp:6:6: cppgen: not supported yet: the conversion from W to V, whose fields differ in struct tags inside a slice or a map"},
		{"struct-map-tags", "type W struct{ M map[int]struct{ Q int `t:\"q\"` } }\ntype V struct{ M map[int]struct{ Q int } }\nfunc main() {\n\t_ = V(W{})\n}", "main.gp:6:6: cppgen: not supported yet: the conversion from W to V, whose fields differ in struct tags inside a slice or a map"},
		{"namedcomplex", "type Z complex128\nfunc main() {}", "main.gp:3:6: cppgen: not supported yet: type complex128"},
		{"methodvalue", "type T struct{}\nfunc (T) M() {}\nfunc main() {\n\tvar t T\n\tprintln(t.M == nil)\n}", "main.gp:7:10: cppgen: not supported yet: method values (Phase 03 step 3.10 after 3.11)"},
		{"fmtslice", "import \"fmt\"\nfunc main() {\n\tfmt.Println(1, [][]int{})\n}", "main.gp:5:17: cppgen: not supported yet: printing a slice (Phase 03 step 3.15)"},
		{"fmtarray", "import \"fmt\"\nfunc main() {\n\tfmt.Println(1, [2]int{})\n}", "main.gp:5:17: cppgen: not supported yet: printing an array (Phase 03 step 3.15)"},
		{"methodexpr", "type T int\nfunc (T) M() {}\nfunc main() {\n\tprintln(T.M == nil)\n}", "main.gp:6:10: cppgen: not supported yet: method expressions (Phase 03 step 3.10 after 3.11)"},
		{"printfunc", "import \"fmt\"\nfunc main() {\n\tfmt.Println(1, main)\n}", "main.gp:5:17: cppgen: not supported yet: printing a func value"},
		{"printlnfunc", "func g() {}\nfunc main() {\n\tprintln(g)\n}", "main.gp:5:10: cppgen: not supported yet: printing a func value with println"},
		{"printfuncfield", "import \"fmt\"\ntype T struct{ f func() }\nfunc main() {\n\tfmt.Println(T{})\n}", "main.gp:6:14: cppgen: not supported yet: printing a func value"},
		{"complex", "func main() {\n\tc := complex(1, 2)\n\tprintln(real(c))\n}", "main.gp:4:2: cppgen: not supported yet: type complex128"},
		{"printf", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%5d\\n\", 1)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %5d (Phase 03)`},
		{"printf-goverb", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%z %x\\n\", 1, 2)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %x (Phase 03)`},
		{"printf-precision", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%.2f\\n\", 1.5)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %.2f (Phase 03)`},
		{"printf-flag", "import \"fmt\"\nfunc main() {\n\tfmt.Printf(\"%-z\\n\", 1)\n}", `main.gp:5:13: cppgen: not supported yet: the fmt.Printf verb %-z (Phase 03)`},
		{"sprint", "import \"fmt\"\nfunc main() {\n\tprintln(fmt.Sprint(1))\n}", "main.gp:5:10: cppgen: not supported yet: fmt.Sprint: Phase 02 has only fmt.Print, fmt.Println and fmt.Printf"},
		{"errorresult", "import \"fmt\"\nfunc main() {\n\tn, err := fmt.Println()\n\tprintln(n, err)\n}", "main.gp:5:5: cppgen: not supported yet: type error"},
		{"rangechan", "func main() {\n\tfor range make(chan int) {\n\t}\n}", "main.gp:4:12: cppgen: not supported yet: range over channels (Phase 08)"},
		{"go", "func main() {\n\tgo println()\n}", "main.gp:4:2: cppgen: not supported yet: go statements (Phase 08)"},
		{"insideif", "func main() {\n\tif x := 1; x > 0 {\n\t\tgo println()\n\t}\n}", "main.gp:5:3: cppgen: not supported yet: go statements (Phase 08)"},
		{"nomain", "func f() {}", "main.gp:2:9: cppgen: function main is undeclared in the main package"},
		{"printptr", "func main() {\n\tx := 1\n\tprintln(&x)\n}", "main.gp:5:10: cppgen: not supported yet: printing a pointer (Phase 03 step 3.15)"},
		{"fmtptr", "import \"fmt\"\nfunc main() {\n\tfmt.Println(new(int))\n}", "main.gp:5:14: cppgen: not supported yet: printing a pointer (Phase 03 step 3.15)"},
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
// (goResult), with the signal line of a nil dereference (its pc masked: maskPC).
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
			if strings.Contains(tt.body, "two(") {
				src += "\nfunc two() [2]int {\n\treturn [2]int{1, 2}\n}\n"
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
					"func clear() int {\n\tprintln(\"clear\")\n\tgnil = nil\n\treturn 1\n}\n"
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
		})
	}
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
