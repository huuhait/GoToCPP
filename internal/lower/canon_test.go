package lower

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// canonDecls is the package of the canonical-spelling tests: the named types, the aliases and
// the local types that canonCorpus uses.
const canonDecls = `package main

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
)

type MyInt = int
type MyByte = byte
type Anything = any
type Rect struct{ W, H float64 }
type Celsius float64
type IDs []int
type Names map[string][]string
type Fn func(int, ...string) (bool, error)
type Sh interface{ Area() float64 }
type Shape interface {
	Sh
	Scale(k float64)
}
type RW interface {
	io.Reader
	io.Writer
}
type Exp interface{ M(MyInt, MyByte) Anything }
type Node struct {
	next *Node
	kids []Node
}
type Pair struct {
	A int
	B string ` + "`json:\"b\"`" + `
}
type Emb struct {
	Rect
	*Node
	X int
}
type Ch chan int

func scope() {
	_ = 0
}

func local() (any, any) {
	type L struct{ N int }
	return L{}, nil
}

func local2() any {
	type L struct{ N int }
	return L{}
}

var _ = errors.New
var _ = fmt.Sprint
var _ = os.Args
var _ = strings.ToUpper
var _ url.Values
`

// canonCorpus are Go types (as source text in canonDecls's package) that %T names.
var canonCorpus = []string{
	"int", "int64", "uint8", "byte", "rune", "int32", "uintptr", "float32", "complex128", "string", "bool",
	"MyInt", "MyByte", "any", "interface{}", "Anything", "error",
	"Rect", "Celsius", "IDs", "Names", "Fn", "Sh", "Shape", "RW", "Exp", "Node", "Pair", "Emb", "Ch",
	"[]int", "[]byte", "[]MyInt", "[3]string", "[0]int", "[2][3]int", "map[string]int", "map[MyInt][]MyByte",
	"*Rect", "**Rect", "*int", "[]*Rect", "map[Rect]*Node",
	"chan int", "<-chan int", "chan<- int", "chan (<-chan int)", "chan<- chan int", "<-chan <-chan int", "chan<- <-chan int",
	"func()", "func(int) string", "func(a, b int) (int, error)", "func(...int)", "func(x int, rest ...string) (n int, err error)",
	"func(func(int) bool) func() error", "func(MyInt, MyByte) (MyByte, MyInt)",
	"struct{}", "struct{ X int }", "struct{ X, Y int; Z string }", "struct{ Rect }", "struct{ *Rect; N int }",
	"struct{ A int `json:\"a\"` }", "struct{ x int }",
	"interface{ M() }", "interface{ M(); N(int) string }", "interface{ N(int) string; M() }",
	"interface{ io.Reader; Close() error }", "interface{ Close() error; Read(p []byte) (int, error) }",
	"interface{ M(MyInt) }", "interface{ M(int) }", "interface{ m() }", "interface{ M(); m(); a() }",
	"interface{ M(interface{ N() }) }", "interface{ M() any }", "interface{ Sh; Rect2() }",
	"io.Reader", "io.ReadWriter", "io.Writer", "fmt.Stringer", "url.Values", "*url.URL", "func(url.Values, ...string) (int, error)",
	"os.Signal", "strings.Builder", "[]fmt.Stringer", "map[string]io.Reader",
	"interface{ Close() error }", "interface{ F(io.Reader) url.Values }", "func(io.Reader)",
	"struct{ R io.Reader; U *url.URL }",
}

// canonPairs are pairs of types that are identical (true) or not (false): their identity texts
// must be equal exactly when types.Identical says so.
var canonPairs = []struct {
	a, b string
	same bool
}{
	{"MyInt", "int", true},
	{"MyByte", "uint8", true},
	{"byte", "uint8", true},
	{"rune", "int32", true},
	{"any", "interface{}", true},
	{"Anything", "any", true},
	{"func(a, b int) (x int, err error)", "func(int, int) (int, error)", true},
	{"func(MyInt, MyByte)", "func(int, uint8)", true},
	{"interface{ M(MyInt) }", "interface{ M(int) }", true},
	{"interface{ io.Reader; Close() error }", "interface{ Close() error; Read(p []byte) (int, error) }", true},
	{"interface{ M(); N() }", "interface{ N(); M() }", true},
	{"[]MyInt", "[]int", true},
	{"map[string]MyByte", "map[string]uint8", true},
	{"chan int", "<-chan int", false},
	{"<-chan int", "chan<- int", false},
	{"int", "int64", false},
	{"uint", "uint64", false},
	{"uint64", "uintptr", false},
	{"Rect", "struct{ W, H float64 }", false},
	{"Celsius", "float64", false},
	{"IDs", "[]int", false},
	{"func(int)", "func(int64)", false},
	{"func(...int)", "func([]int)", false},
	{"func(int) (int, error)", "func(int) (error, int)", false},
	{"interface{ M() }", "interface{ N() }", false},
	{"interface{ M() }", "interface{ M(int) }", false},
	{"interface{ M() }", "interface{ m() }", false},
	{"struct{ X int }", "struct{ Y int }", false},
	{"struct{ X int }", "struct{ X int `a:\"b\"` }", false},
	{"struct{ Rect }", "struct{ R Rect }", false},
	{"io.Reader", "interface{ Read(p []byte) (n int, err error) }", false}, // named vs unnamed
	{"*Rect", "*Node", false},
	{"[2]int", "[3]int", false},
	{"map[string]int", "map[string]int64", false},
}

// fileOf are the files of the fset of typeCheckCorpus.
var fileOf = map[*token.FileSet][]*ast.File{}

// typeCheckCorpus type-checks canonDecls with the standard library from source.
func typeCheckCorpus(t *testing.T) (*types.Package, *token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", canonDecls, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	pkg, err := conf.Check("main", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	fileOf[fset] = []*ast.File{f}
	return pkg, fset, f
}

// evalType returns the type that the source text expr denotes in the scope of pkg (the
// package-level declarations).
func evalType(t *testing.T, fset *token.FileSet, pkg *types.Package, expr string) types.Type {
	t.Helper()
	// Inside the body of scope(), where the file's imports are visible.
	pos := token.NoPos
	for _, f := range fileOf[fset] {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "scope" {
				pos = fd.Body.Lbrace + 1
			}
		}
	}
	tv, err := types.Eval(fset, pkg, pos, "(*"+expr+")(nil)")
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	return tv.Type.(*types.Pointer).Elem()
}

// goTypeNames runs real Go on a program that prints %T of a nil pointer to each type and
// returns the names, without the *.
func goTypeNames(t *testing.T, exprs []string, extra string) []string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go is not installed")
	}
	var src bytes.Buffer
	src.WriteString(canonDecls)
	src.WriteString(extra)
	src.WriteString("\nfunc main() {\n")
	for _, e := range exprs {
		fmt.Fprintf(&src, "\tfmt.Printf(\"%%T\\n\", (*%s)(nil))\n", e)
	}
	src.WriteString("}\n")
	dir := t.TempDir()
	file := filepath.Join(dir, "main.go")
	if err := os.WriteFile(file, src.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("go", "run", file)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOFLAGS=", "GO111MODULE=off", "GOTOOLCHAIN=local")
	cmd.WaitDelay = 5 * time.Second
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run: %v\n%s\n%s", err, out, src.String())
	}
	names := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(names) != len(exprs) {
		t.Fatalf("go printed %d lines for %d types:\n%s", len(names), len(exprs), out)
	}
	for i, n := range names {
		names[i] = strings.TrimPrefix(n, "*")
	}
	return names
}

// TestCanonName compares the names of canon (the name of a descriptor) with %T of real Go.
func TestCanonName(t *testing.T) {
	pkg, fset, _ := typeCheckCorpus(t)
	want := goTypeNames(t, canonCorpus, "")
	c := &canon{}
	for i, expr := range canonCorpus {
		typ := evalType(t, fset, pkg, expr)
		if got := c.str(typ); got != want[i] {
			t.Errorf("%s: name %q, Go's %%T is %q", expr, got, want[i])
		}
	}
}

// TestCanonLocal: two types of the same name declared in two functions have one %T name and
// two identity texts.
func TestCanonLocal(t *testing.T) {
	pkg, _, _ := typeCheckCorpus(t)
	// The scopes of the functions hold the local types.
	var found []*types.TypeName
	var walk func(s *types.Scope)
	walk = func(s *types.Scope) {
		for _, n := range s.Names() {
			if tn, ok := s.Lookup(n).(*types.TypeName); ok && s != pkg.Scope() {
				found = append(found, tn)
			}
		}
		for i := range s.NumChildren() {
			walk(s.Child(i))
		}
	}
	walk(pkg.Scope())
	var ls []*types.TypeName
	for _, tn := range found {
		if tn.Name() == "L" {
			ls = append(ls, tn)
		}
	}
	if len(ls) != 2 {
		t.Fatalf("found %d local types L, want 2", len(ls))
	}
	name := (&canon{}).str(ls[0].Type())
	if name != "main.L" || name != (&canon{}).str(ls[1].Type()) {
		t.Errorf("the name of a local type is %q, want main.L (twice)", name)
	}
	c := &canon{ident: true, local: func(tn *types.TypeName) string {
		if tn.Parent() == tn.Pkg().Scope() {
			return ""
		}
		return strconv.Itoa(int(tn.Pos()))
	}}
	a, b := c.str(ls[0].Type()), c.str(ls[1].Type())
	if a == b {
		t.Errorf("two local types L have the same identity text %q", a)
	}
	if !strings.HasPrefix(a, "main.L·") {
		t.Errorf("identity text %q", a)
	}
}

// TestCanonIdentity: the identity texts of two types are equal exactly when the types are
// identical.
func TestCanonIdentity(t *testing.T) {
	pkg, fset, _ := typeCheckCorpus(t)
	c := &canon{ident: true}
	for _, p := range canonPairs {
		a, b := evalType(t, fset, pkg, p.a), evalType(t, fset, pkg, p.b)
		if got := types.Identical(a, b); got != p.same {
			t.Errorf("%s, %s: types.Identical = %v, the table says %v", p.a, p.b, got, p.same)
		}
		if got := c.str(a) == c.str(b); got != p.same {
			t.Errorf("%s (%q), %s (%q): equal texts = %v, want %v", p.a, c.str(a), p.b, c.str(b), got, p.same)
		}
	}
}

// TestCanonSigText: spot checks of the identity text with import paths.
func TestCanonSigText(t *testing.T) {
	pkg, fset, _ := typeCheckCorpus(t)
	c := &canon{ident: true}
	for _, tt := range []struct{ expr, want string }{
		{"func(url.Values, ...string) (int, error)", "func(net/url.Values, ...string) (int, error)"},
		{"func(a, b MyInt) (n MyByte, err error)", "func(int, int) (uint8, error)"},
		{"func(io.Reader) any", "func(io.Reader) interface {}"},
		{"func(interface{ Close() error; Read(p []byte) (int, error) })", "func(interface { Close() error; Read([]uint8) (int, error) })"},
		{"func(struct{ x int })", "func(struct { main.x int })"},
		{"func(interface{ m() })", "func(interface { main.m() })"},
		{"func(chan<- <-chan int)", "func(chan<- <-chan int)"},
	} {
		if got := c.str(evalType(t, fset, pkg, tt.expr)); got != tt.want {
			t.Errorf("%s: %q, want %q", tt.expr, got, tt.want)
		}
	}
}

// TestSortedMethods: the slots of an interface follow the runtime's order (exported names by name,
// then unexported ones by name and package path), whatever the source order, and an embedded
// interface's methods are in it.
func TestSortedMethods(t *testing.T) {
	src := `package p
type E interface{ Beta(); alpha() }
type I interface {
	zed()
	E
	Alpha()
	Ünder()
	beta()
	Zulu()
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "p.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := (&types.Config{}).Check("p", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}
	it := pkg.Scope().Lookup("I").Type().Underlying().(*types.Interface)
	var got []string
	for _, m := range sortedMethods(it) {
		got = append(got, m.Name())
	}
	want := "Alpha Beta Zulu Ünder alpha beta zed" // (Ü is an upper case letter: exported)
	if s := strings.Join(got, " "); s != want {
		t.Errorf("sortedMethods = %q, want %q", s, want)
	}
}
