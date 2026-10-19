package check

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The tests load programs with load.Load and the real standard library
// stubs (stdlib.FS), then type-check them with Check.

// writeTree writes the files (slash-separated names relative to the
// folder, and their contents) into a new temporary folder and returns it,
// cleaned as the loader cleans the folders in its messages.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// loadAndCheck loads the patterns in the folder dir with the standard
// library stubs (for linux/amd64 unless cfg says otherwise) and checks the
// program with Check, without the phase gates (gates_test.go tests them).
func loadAndCheck(t *testing.T, cfg load.Config, dir string, patterns ...string) (*load.Program, *Result) {
	t.Helper()
	cfg.Dir, cfg.Stdlib, cfg.GoStd = dir, stdlib.FS(), stdlib.IsStd
	if cfg.GOOS == "" {
		cfg.GOOS, cfg.GOARCH = "linux", "amd64"
	}
	prog := load.Load(cfg, patterns...)
	return prog, Check(prog, fakeProbeConfig(LastPhase))
}

// fakeProbeConfig returns the Config for the phase with a namespace probe
// that runs no compiler: every name is a namespace, and every header is
// fine. Most tests use headers that don't exist (<lib.h>); the tests of the
// probe (nsprobe_test.go) run the real compilers.
func fakeProbeConfig(phase int) Config {
	return Config{
		Phase: phase,
		CXX:   func(context.Context) (build.CXX, error) { return build.CXX{}, nil },
		Probe: func(_ context.Context, _ build.CXX, _ string, _, _, candidates []string) (interop.Result, error) {
			kinds := make(map[string]interop.Kind)
			for _, c := range candidates {
				kinds[c] = interop.Namespace
			}
			return interop.Result{Kinds: kinds}, nil
		},
	}
}

// errorText returns what gocpp check would print for the program, like go
// build: the program's errors, then a "# path" line and the diagnostics of
// each package that has some, with file names relative to dir. all prints
// every error; without it, a package shows at most 10.
func errorText(t *testing.T, prog *load.Program, r *Result, dir string, all bool) string {
	t.Helper()
	var b bytes.Buffer
	pr := diag.Printer{All: all, Dir: dir}
	print := func(header string, l *diag.List) {
		if len(l.Diagnostics()) == 0 {
			return
		}
		if header != "" {
			fmt.Fprintf(&b, "# %s\n", header)
		}
		if err := pr.Print(&b, l); err != nil {
			t.Fatal(err)
		}
	}
	print("", prog.Errors)
	for _, p := range r.Packages {
		name := p.Load.ImportPath
		if p.Load.ForTest != "" {
			name += " [" + p.Load.ForTest + ".test]"
		}
		print(name, p.Errors)
	}
	return b.String()
}

// mainPkg returns the checked root package of the program.
func mainPkg(t *testing.T, prog *load.Program, r *Result) *Package {
	t.Helper()
	if len(prog.Roots) == 0 {
		t.Fatal("no root package")
	}
	p := r.Package(prog.Roots[0])
	if p == nil {
		t.Fatal("the root has no checked package")
	}
	return p
}

// TestExamples checks the 19 target programs (the two with the syntax of Phase 10 too, since
// step 10.12 made it always available), for
// linux and windows (win32 only for windows: its files say so), each as a
// program and with its tests: 0 errors, and every package is checked.
func TestExamples(t *testing.T) {
	mains, err := filepath.Glob("../../examples/*/main.gp")
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, mainFile := range mains {
		dir, err := filepath.Abs(filepath.Dir(mainFile))
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(dir)
		n++
		t.Run(name, func(t *testing.T) {
			systems := 0
			for _, goos := range []string{"linux", "windows"} {
				for _, tests := range []bool{false, true} {
					prog, r := loadAndCheck(t, load.Config{GOOS: goos, GOARCH: "amd64", Tests: tests}, dir, ".")
					if ds := prog.Roots[0].Errors.Diagnostics(); len(ds) == 1 &&
						strings.Contains(ds[0].Msg, "build constraints exclude all .gp files") {
						continue
					}
					systems++
					msg, masked := maskedExamples[name] // an error until a step lands (gates_test.go)
					want := ""
					if masked {
						want = errorLines([]string{msg})
					}
					if errs := errorText(t, prog, r, dir, true); errs != want {
						t.Errorf("%s, Tests %v:\n%s\nwant:\n%s", goos, tests, errs, want)
					}
					for _, p := range r.Packages {
						if !p.Checked() && !masked {
							t.Errorf("%s, Tests %v: %s is not checked", goos, tests, p.Load.ImportPath)
						}
					}
				}
			}
			if systems == 0 {
				t.Errorf("the build constraints exclude every file on linux and windows")
			}
		})
	}
	if n != 19 {
		t.Errorf("checked %d examples, want 19", n)
	}
}

// TestDiffPrograms checks the Go programs of tests/diff (Go code only, no
// C or C++): 0 errors.
func TestDiffPrograms(t *testing.T) {
	files, err := filepath.Glob("../../tests/diff/*.gp")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no programs in tests/diff")
	}
	for _, f := range files {
		abs, err := filepath.Abs(f)
		if err != nil {
			t.Fatal(err)
		}
		prog, r := loadAndCheck(t, load.Config{}, filepath.Dir(abs), abs)
		if errs := errorText(t, prog, r, filepath.Dir(abs), true); errs != "" {
			t.Errorf("%s:\n%s", f, errs)
		}
		if !mainPkg(t, prog, r).Checked() {
			t.Errorf("%s is not checked", f)
		}
	}
}

// TestGenericPackage checks a program that uses a generic package of its
// module: generic functions, a generic type with methods, constraints with
// type sets, and instances across packages.
func TestGenericPackage(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"gen/gen.gp": `package gen

import "cmp"

type Number interface {
	~int | ~int64 | ~float64
}

func Sum[T Number](xs ...T) T {
	var s T
	for _, x := range xs {
		s += x
	}
	return s
}

func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

type Stack[T any] struct{ items []T }

func (s *Stack[T]) Push(x T) { s.items = append(s.items, x) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T
	if len(s.items) == 0 {
		return zero, false
	}
	x := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return x, true
}
`,
		"main.gp": `package main

import (
	"fmt"

	"example.com/m/gen"
)

type celsius float64

func main() {
	var s gen.Stack[string]
	s.Push("a")
	x, ok := s.Pop()
	fmt.Println(gen.Sum(1, 2, 3), gen.Sum[celsius](1.5, 2), gen.Max("a", "b"), x, ok)
}
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	p := mainPkg(t, prog, r)
	var got []string
	for id, inst := range p.Info.Instances {
		got = append(got, id.Name+" "+inst.Type.String())
	}
	for _, want := range []string{
		"Stack example.com/m/gen.Stack[string]",
		"Sum func(xs ...int) int",
		"Sum func(xs ...example.com/m.celsius) example.com/m.celsius",
		"Max func(a string, b string) string",
	} {
		if !contains(got, want) {
			t.Errorf("no instance %q in %q", want, got)
		}
	}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// TestConfig checks the settings of types.Config: the pinned Go version,
// the per-file version of a //go:build go1.N line, gocpp's sizes
// (gocppSizes: a string is 32 bytes) and FakeImportC.
func TestConfig(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp": `package main

import (
	"unsafe"

	"<stdio.h>"
)

const (
	intSize  = unsafe.Sizeof(int(0))
	pairSize = unsafe.Sizeof(struct {
		a int8
		b int64
	}{})
	stringSize = unsafe.Sizeof("")
)

func main() {
	stdio.puts(C.CString("hi"))
	for i := range 3 {
		_ = i
	}
}
`,
		"old.gp": "//go:build go1.21\n\npackage main\n\nfunc old() {\n\tfor i := range 3 {\n\t\t_ = i\n\t}\n}\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n" +
		"./old.gp:6:17: cannot range over 3 (untyped int constant): requires go1.22 or later\n"
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	p := mainPkg(t, prog, r)
	if v := p.Types.GoVersion(); v != GoVersion {
		t.Errorf("package GoVersion %q, want %q", v, GoVersion)
	}
	versions := make(map[string]string)
	for f, v := range p.Info.FileVersions {
		versions[filepath.Base(prog.Fset.File(f.Pos()).Name())] = v
	}
	if versions["a.gp"] != "go1.25" || versions["old.gp"] != "go1.21" {
		t.Errorf("FileVersions %v, want a.gp go1.25 and old.gp go1.21", versions)
	}
	for name, want := range map[string]int64{"intSize": 8, "pairSize": 16, "stringSize": 32} {
		c, _ := p.Types.Scope().Lookup(name).(*types.Const)
		if c == nil {
			t.Fatalf("no constant %s", name)
		}
		if got, _ := constant.Int64Val(c.Val()); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
}

// findExpr returns the expression of main.gp that types.ExprString prints
// as src and that starts on the given line where the text at starts (the
// first time on the line).
func findExpr(t *testing.T, prog *load.Program, p *Package, line int, at, src string) ast.Expr {
	t.Helper()
	for _, f := range p.Load.Files {
		if filepath.Base(f.Path) != "main.gp" {
			continue
		}
		text, err := os.ReadFile(f.Path)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(text), "\n")
		col := strings.Index(lines[line-1], at) + 1
		if col == 0 {
			t.Fatalf("line %d has no %q", line, at)
		}
		var found ast.Expr
		ast.Inspect(f.AST, func(n ast.Node) bool {
			e, ok := n.(ast.Expr)
			if ok && found == nil && types.ExprString(e) == src {
				if pos := prog.Fset.Position(e.Pos()); pos.Line == line && pos.Column == col {
					found = e
				}
			}
			return found == nil
		})
		if found != nil {
			return found
		}
	}
	t.Fatalf("no expression %s at main.gp:%d, at %q", src, line, at)
	return nil
}

// TestOpaqueTypeArgs checks the type arguments of opaque instantiations:
// they get types, a typo is reported, and an import used only in them is
// used.
func TestOpaqueTypeArgs(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": `package main

import (
	"strings"
	str "strings"
	. "unicode/utf8"

	"<map>"
	"<string>"
	"<vector>"
)

type myT struct{ n int }

type Box[T any] struct {
	items std.vector[T]
}

var a std.vector[strings.Builder]
var m std.map[std.string, std.vector[myT]]
var s = std.vector[[]str.Builder]{}
var n std.array[int, UTFMax]

func f[T any](v std.vector[T]) {
	type local struct{ x int }
	var l std.vector[local]
	_ = l
}

func main() {
	_, _, _, _ = a, m, s, n
	f[int](nil)
	_ = Box[int]{}
}
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	p := mainPkg(t, prog, r)
	for _, c := range []struct {
		line         int
		at, src, typ string
	}{
		{19, "strings.", "strings.Builder", "strings.Builder"},
		{20, "myT]", "myT", "example.com/m.myT"},
		{21, "[]str", "[]str.Builder", "[]strings.Builder"},
		{22, "UTFMax", "UTFMax", "untyped int"},
		{16, "T]", "T", "T"},
		{24, "T]", "T", "T"},
		{26, "local]", "local", "example.com/m.local"},
		// An opaque argument stays without a type (step 1.11 marks it).
		{20, "std.string", "C.std.string", ""},
	} {
		e := findExpr(t, prog, p, c.line, c.at, c.src)
		tv, ok := p.Info.Types[e]
		got := ""
		if ok {
			got = tv.Type.String()
		}
		if got != c.typ {
			t.Errorf("%d: %s has type %q, want %q", c.line, c.src, got, c.typ)
		}
	}
	// The type parameter of Box is T of Box, not of f.
	e := findExpr(t, prog, p, 16, "T]", "T")
	if obj := p.Info.Uses[e.(*ast.Ident)]; obj == nil || obj.Pos() != p.Types.Scope().Lookup("Box").Type().(*types.Named).TypeParams().At(0).Obj().Pos() {
		t.Errorf("T in Box's field is %v, want Box's type parameter", obj)
	}
}

// TestOpaqueTypeArgErrors checks the errors in type arguments of opaque
// instantiations: typos are reported once, also when go/types checked the
// argument itself (in a value), nested arguments are checked, and an
// import that is not used is still reported, after the type arguments.
func TestOpaqueTypeArgErrors(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": `package main

import (
	"os"
	str "strings"

	"<map>"
	"<memory>"
	"<string>"
	"<vector>"
)

type Point struct{ x int }

var a std.vector[Pointt]
var b std.map[std.string, std.vector[Pointt]]
var c = std.vector[std.vector[*Pointt]]{}

func main() {
	_, _, _ = a, b, c
	p := std.make_unique[Pointt]()
	_ = p
	type later int
	var d std.vector[later2]
	_ = d
}

type later2 int
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := `# example.com/m
./main.gp:4:2: "os" imported and not used
./main.gp:5:2: "strings" imported as str and not used
./main.gp:15:18: undefined: Pointt
./main.gp:16:38: undefined: Pointt
./main.gp:17:32: undefined: Pointt
./main.gp:21:23: undefined: Pointt
`
	if errs := errorText(t, prog, r, root, true); errs != want {
		t.Errorf("errors:\n%s\nwant:\n%s", errs, want)
	}
	if ds := mainPkg(t, prog, r).Errors.Diagnostics(); len(ds) > 0 && ds[0].Code != diag.UnusedImport {
		t.Errorf("the first error has code %d, want UnusedImport", ds[0].Code)
	}
}

// TestOpaqueTypeArgOrder checks that the errors keep the order in which
// they were found, which decides what Print shows: it stops after 10
// errors found. go/types finds the unused import "os" last, and the errors
// of the type arguments (CheckExpr) come after it. It also checks that a
// typo in an argument that go/types checked itself (in a value) counts
// once, though CheckExpr finds it again.
func TestOpaqueTypeArgOrder(t *testing.T) {
	var src strings.Builder
	src.WriteString("package main\n\nimport (\n\t\"os\"\n\n\t\"<memory>\"\n\t\"<vector>\"\n)\n\nfunc main() {\n")
	for i := range 6 {
		fmt.Fprintf(&src, "\t_ = std.make_unique[Q%d]()\n", i) // lines 11 to 16
	}
	src.WriteString("}\n\n")
	for i := range 6 {
		fmt.Fprintf(&src, "var v%d std.vector[P%d]\n", i, i) // lines 19 to 24
	}
	root := writeTree(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src.String()})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	got := errorText(t, prog, r, root, false)
	// 13 errors, found in this order: Q0..Q5 and "os" by go/types, then
	// Q0..Q5 again by CheckExpr (dropped) and P0..P5.
	var want strings.Builder
	want.WriteString("# example.com/m\n./main.gp:4:2: \"os\" imported and not used\n")
	for i := range 6 {
		fmt.Fprintf(&want, "./main.gp:%d:22: undefined: Q%d\n", 11+i, i)
	}
	for i := range 3 {
		fmt.Fprintf(&want, "./main.gp:%d:19: undefined: P%d\n", 19+i, i)
	}
	want.WriteString("./main.gp:21:19: too many errors\n")
	if got != want.String() {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want.String())
	}
}

// unusedThenErrors returns a program with an unused variable in f0 and a
// type error in each of f1 to f10, plus the lines in extra.
func unusedThenErrors(imports, extra string) string {
	src := "package main\n\n" + imports + "func f0() {\n\tunused := 1\n}\n"
	for i := 1; i <= 10; i++ {
		src += fmt.Sprintf("func f%d() { var x int = \"s\"; _ = x }\n", i)
	}
	return src + "\nfunc main() {}\n" + extra
}

// goBuildUnusedThenErrors is what go build 1.25.14 prints for
// unusedThenErrors("", "") as main.go (with .gp here): go/types reports
// the unused variable at the end of f0's body, so it is the first error
// found, and the limit of 10 cuts f10.
const goBuildUnusedThenErrors = `# example.com/m
./main.gp:4:2: declared and not used: unused
./main.gp:6:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:7:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:8:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:9:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:10:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:11:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:12:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:13:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:14:25: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:14:25: too many errors
`

// TestUnusedVarOrder checks that an unused variable keeps its place among
// the errors, like go build: in a Go program, and in a program with an
// opaque argument, whose error comes after all of go/types' errors (so,
// after the 10th, it is not shown).
func TestUnusedVarOrder(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   unusedThenErrors("", ""),
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if got := errorText(t, prog, r, root, false); got != goBuildUnusedThenErrors {
		t.Errorf("Go program, errors:\n%s\nwant (go build):\n%s", got, goBuildUnusedThenErrors)
	}

	// The same with import "C" (from a header): the lines move by 3.
	root = writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp":   unusedThenErrors("import \"<vector>\"\n\n", "\nvar v std.vector[Pointt]\n"),
	})
	prog, r = loadAndCheck(t, load.Config{}, root, ".")
	want := "# example.com/m\n./main.gp:6:2: declared and not used: unused\n"
	for line := 8; line <= 16; line++ {
		want += fmt.Sprintf("./main.gp:%d:25: cannot use \"s\" (untyped string constant) as int value in variable declaration\n", line)
	}
	want += "./main.gp:16:25: too many errors\n"
	if got := errorText(t, prog, r, root, false); got != want {
		t.Errorf("program with C, errors:\n%s\nwant:\n%s", got, want)
	}
	// With -e, the opaque argument's error is there, after the others.
	if got := errorText(t, prog, r, root, true); !strings.HasSuffix(got, "./main.gp:21:18: undefined: Pointt\n") {
		t.Errorf("program with C, all errors:\n%s", got)
	}
}

// TestTestVariants checks that an external test package that imports q,
// which imports p, sees one p: the test variant of p, also through q.
func TestTestVariants(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/m\n",
		"p/p.gp":       "package p\n\ntype T struct{ N int }\n",
		"p/in_test.gp": "package p\n\nfunc helper() T { return T{N: 1} }\n",
		"p/x_test.gp": `package p_test

import (
	"example.com/m/p"
	"example.com/m/q"
)

var _ p.T = q.Get()
var _ = q.Get().N
`,
		"q/q.gp": "package q\n\nimport \"example.com/m/p\"\n\nfunc Get() p.T { return p.T{} }\n",
		"main.gp": "package main\n\nimport (\n\t\"example.com/m/p\"\n\t\"example.com/m/q\"\n)\n\n" +
			"func main() { var _ p.T = q.Get() }\n",
	})
	prog, r := loadAndCheck(t, load.Config{Tests: true}, root, "./p", ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	// The packages example.com/m/p: the test variant and the plain one.
	var ps []*Package
	for _, cp := range r.Packages {
		if cp.Load.ImportPath == "example.com/m/p" {
			ps = append(ps, cp)
		}
	}
	if len(ps) != 2 || ps[0].Types == ps[1].Types {
		t.Fatalf("%d checked packages example.com/m/p, want 2 different ones", len(ps))
	}
	// Each importer gets the p of its graph: x_test and q' get the test
	// variant, main and q the plain p.
	imported := func(cp *Package, path string) *types.Package {
		for _, imp := range cp.Types.Imports() {
			if imp.Path() == path {
				return imp
			}
		}
		t.Fatalf("%s doesn't import %s", cp.Load.ImportPath, path)
		return nil
	}
	for _, cp := range r.Packages {
		if cp.Load.ImportPath == "example.com/m/p" {
			continue
		}
		want := r.Package(cp.Load.Imports["example.com/m/p"]).Types
		if got := imported(cp, "example.com/m/p"); got != want {
			t.Errorf("%s [%s] imports another p than the loader gave it", cp.Load.ImportPath, cp.Load.ForTest)
		}
	}
	xt := r.Package(prog.Roots[1])
	if xt.Load.ImportPath != "example.com/m/p_test" || imported(xt, "example.com/m/p").Scope().Lookup("helper") == nil {
		t.Errorf("the external test doesn't see the test variant of p")
	}
}

// TestTestVariantsDiamond checks test variants with a diamond: x_test
// imports p, q, r and s; s imports q and r; q and r import p. Every package
// reaches exactly one p, directly or not: the one the loader gave it (the
// test variant in the test's graph, the plain p for main).
func TestTestVariantsDiamond(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/m\n",
		"p/p.gp":       "package p\n\ntype T struct{ N int }\n",
		"p/in_test.gp": "package p\n\nfunc helper() T { return T{N: 1} }\n",
		"p/x_test.gp": `package p_test

import (
	"example.com/m/p"
	"example.com/m/q"
	"example.com/m/r"
	"example.com/m/s"
)

var _ p.T = q.Q()
var _ p.T = r.R()
var _ = s.S()
`,
		"q/q.gp": "package q\n\nimport \"example.com/m/p\"\n\nfunc Q() p.T { return p.T{} }\n",
		"r/r.gp": "package r\n\nimport \"example.com/m/p\"\n\nfunc R() p.T { return p.T{} }\n",
		"s/s.gp": "package s\n\nimport (\n\t\"example.com/m/q\"\n\t\"example.com/m/r\"\n)\n\n" +
			"func S() bool { return q.Q() == r.R() }\n",
		"main.gp": "package main\n\nimport (\n\t\"example.com/m/p\"\n\t\"example.com/m/s\"\n)\n\n" +
			"func main() { var _ p.T; _ = s.S() }\n",
	})
	prog, r := loadAndCheck(t, load.Config{Tests: true}, root, "./p", ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	const pPath = "example.com/m/p"
	// reached returns the packages with the path pPath that lp (or tp)
	// imports, directly or not.
	loadReached := func(lp *load.Package) map[*load.Package]bool {
		found := make(map[*load.Package]bool)
		seen := make(map[*load.Package]bool)
		var walk func(*load.Package)
		walk = func(lp *load.Package) {
			for path, dep := range lp.Imports {
				if path == pPath {
					found[dep] = true
				}
				if !seen[dep] {
					seen[dep] = true
					walk(dep)
				}
			}
		}
		walk(lp)
		return found
	}
	typesReached := func(tp *types.Package) map[*types.Package]bool {
		found := make(map[*types.Package]bool)
		seen := make(map[*types.Package]bool)
		var walk func(*types.Package)
		walk = func(tp *types.Package) {
			for _, dep := range tp.Imports() {
				if dep.Path() == pPath {
					found[dep] = true
				}
				if !seen[dep] {
					seen[dep] = true
					walk(dep)
				}
			}
		}
		walk(tp)
		return found
	}
	checkedP := 0
	for _, cp := range r.Packages {
		if cp.Load.ImportPath == pPath {
			checkedP++
			continue
		}
		lps := loadReached(cp.Load)
		if len(lps) == 0 {
			continue // imports no p
		}
		if len(lps) != 1 {
			t.Fatalf("%s [%s]: the loader gave it %d packages p", cp.Load.ImportPath, cp.Load.ForTest, len(lps))
		}
		var lp *load.Package
		for dep := range lps {
			lp = dep
		}
		tps := typesReached(cp.Types)
		if len(tps) != 1 || !tps[r.Package(lp).Types] {
			t.Errorf("%s [%s] reaches %d packages p, want exactly the loader's", cp.Load.ImportPath, cp.Load.ForTest, len(tps))
		}
		if cp.Load.ImportPath == "example.com/m" && lp.ForTest != "" {
			t.Errorf("main gets the test variant of p")
		}
		if cp.Load.ForTest == pPath && lp.ForTest != pPath {
			t.Errorf("%s [%s.test] gets the plain p", cp.Load.ImportPath, pPath)
		}
	}
	if checkedP != 2 {
		t.Errorf("%d packages p, want 2 (the test variant and the plain one)", checkedP)
	}
}

// TestLoadErrors checks that a package with a load error is not checked,
// and neither are the packages that import it; only the load errors are
// shown for them. A package that doesn't depend on them is checked.
func TestLoadErrors(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\n\nimport \"example.com/m/nonexist\"\n\nvar X = nonexist.X\nvar Y int = \"s\"\n",
		"b/b.gp":    "package b\n\nimport \"example.com/m/a\"\n\nvar Z int = a.X + \"s\"\n",
		"c/c.gp":    "package c\n\nvar W int = \"s\"\n",
		"d/d.gp":    "package d\n\nfunc f() { x := }\n",
		"main.gp": "package main\n\nimport (\n\t\"example.com/m/b\"\n\t\"example.com/m/c\"\n\t\"example.com/m/d\"\n)\n\n" +
			"func main() { _, _ = b.Z, c.W; d.F() }\n",
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	want := `# example.com/m/a
a/a.gp:3:8: package example.com/m/nonexist is not in module example.com/m ($ROOT/nonexist)
# example.com/m/c
c/c.gp:3:13: cannot use "s" (untyped string constant) as int value in variable declaration
# example.com/m/d
d/d.gp:3:17: expected operand, found '}'
`
	if got := strings.ReplaceAll(errorText(t, prog, r, root, true), root, "$ROOT"); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	for _, cp := range r.Packages {
		wantChecked := cp.Load.ImportPath == "example.com/m/c" || cp.Load.Std
		if cp.Checked() != wantChecked {
			t.Errorf("%s: Checked %v, want %v", cp.Load.ImportPath, cp.Checked(), wantChecked)
		}
		if !cp.Checked() && (cp.Info != nil || cp.Errors != cp.Load.Errors) {
			t.Errorf("%s: not checked, but has Info or its own Errors", cp.Load.ImportPath)
		}
	}
}

// TestCouldNotImport checks that go/types' "could not import X" is not
// added to the load error of X, when a package with load errors is checked
// after all (Check doesn't, but the language server will want its types).
func TestCouldNotImport(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp":    "package a\n\nimport \"example.com/m/nonexist\"\n\nvar X = nonexist.X\nvar Y int = \"s\"\n",
		"main.gp":   "package main\n\nimport \"example.com/m/a\"\n\nvar Z int = a.X\nvar W int = \"s\"\n\nfunc main() {}\n",
	})
	prog, _ := loadAndCheck(t, load.Config{}, root, ".")
	checked := make(map[*load.Package]*types.Package)
	var a, main *Package
	for _, lp := range prog.Packages {
		cp := checkPackage(prog.Fset, lp, checked, newCgohPackage(), false, nil, false)
		checked[lp] = cp.Types
		switch lp.ImportPath {
		case "example.com/m/a":
			a = cp
		case "example.com/m":
			main = cp
		}
	}
	var b bytes.Buffer
	pr := diag.Printer{All: true, Dir: root}
	for _, cp := range []*Package{a, main} {
		if err := pr.Print(&b, cp.Errors); err != nil {
			t.Fatal(err)
		}
	}
	want := `a/a.gp:3:8: package example.com/m/nonexist is not in module example.com/m ($ROOT/nonexist)
a/a.gp:6:13: cannot use "s" (untyped string constant) as int value in variable declaration
./main.gp:6:13: cannot use "s" (untyped string constant) as int value in variable declaration
`
	if got := strings.ReplaceAll(b.String(), root, "$ROOT"); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}

	// In a package without load errors, a refused import is an internal
	// problem, so its error stays.
	c := &collector{fset: prog.Fset, list: new(diag.List), seen: make(map[errorKey]bool)}
	conf := types.Config{
		Importer: &importer{from: prog.Roots[0], checked: map[*load.Package]*types.Package{}, refused: map[string]bool{}},
		Error:    c.collect,
	}
	f := prog.Roots[0].Files[0].AST
	conf.Check("example.com/m", prog.Fset, []*ast.File{f}, nil)
	ds := c.list.Diagnostics()
	if len(ds) == 0 || !strings.HasPrefix(ds[0].Msg, "could not import example.com/m/a") {
		t.Errorf("errors %v, want could not import first", ds)
	}
	if len(ds) > 0 && ds[0].Code != brokenImport {
		t.Errorf("could not import has code %d, want %d (brokenImport)", ds[0].Code, brokenImport)
	}
}

// TestInfo spot-checks each map of types.Info, and InitOrder, on a small
// program.
func TestInfo(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": `package main

import "fmt"

type S struct{ n int }

func (s S) Get() int { return s.n }

func id[T any](x T) T { return x }

var a = b + 1
var b = id(2)

func main() {
	var v any = S{n: a}
	switch x := v.(type) {
	case S:
		fmt.Println(x.Get())
	}
}
`,
	})
	prog, r := loadAndCheck(t, load.Config{}, root, ".")
	if errs := errorText(t, prog, r, root, true); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	p := mainPkg(t, prog, r)
	info := p.Info
	byName := func(m map[*ast.Ident]types.Object, name string) types.Object {
		for id, obj := range m {
			if id.Name == name {
				return obj
			}
		}
		return nil
	}
	if obj := byName(info.Defs, "S"); obj == nil || obj.Type().String() != "example.com/m.S" {
		t.Errorf("Defs: S is %v", obj)
	}
	if obj := byName(info.Uses, "Println"); obj == nil || obj.Pkg().Path() != "fmt" {
		t.Errorf("Uses: Println is %v", obj)
	}
	foundSel := false
	for e, sel := range info.Selections {
		if e.Sel.Name == "Get" && sel.Kind() == types.MethodVal {
			foundSel = true
		}
	}
	if !foundSel {
		t.Errorf("Selections: no method value x.Get")
	}
	foundImplicit := false
	for n, obj := range info.Implicits {
		if _, ok := n.(*ast.CaseClause); ok && obj.Name() == "x" && obj.Type().String() == "example.com/m.S" {
			foundImplicit = true
		}
	}
	if !foundImplicit {
		t.Errorf("Implicits: no x of type S for the case clause")
	}
	if inst, ok := info.Instances[findExpr(t, prog, p, 12, "id", "id").(*ast.Ident)]; !ok ||
		inst.Type.String() != "func(x int) int" {
		t.Errorf("Instances: id(2) is %v", inst)
	}
	if tv := info.Types[findExpr(t, prog, p, 11, "b + 1", "b + 1")]; tv.Type == nil || tv.Type.String() != "int" {
		t.Errorf("Types: b + 1 is %v", tv.Type)
	}
	foundScope := false
	for n := range info.Scopes {
		if fn, ok := n.(*ast.FuncType); ok && fn.Pos().IsValid() {
			foundScope = true
		}
	}
	if !foundScope || len(info.Scopes) < 5 {
		t.Errorf("Scopes: %d scopes, want function scopes", len(info.Scopes))
	}
	var order []string
	for _, init := range info.InitOrder {
		order = append(order, init.String())
	}
	if got := strings.Join(order, "; "); got != "b = id(2); a = b + 1" {
		t.Errorf("InitOrder: %s", got)
	}
	if len(info.FileVersions) != 1 {
		t.Errorf("FileVersions: %v", info.FileVersions)
	}
}
