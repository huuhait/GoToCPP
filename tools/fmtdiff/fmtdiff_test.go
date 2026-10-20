package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"strings"
	"testing"
)

func TestMain(m *testing.M) {
	// nothing here uses the module cache or the network
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	os.Exit(m.Run())
}

func TestDeterministic(t *testing.T) {
	g := NewGen()
	for i := 0; i < 5; i++ {
		a := g.Source(g.Program(7, i, 80), nil)
		b := g.Source(NewGen().Program(7, i, 80), nil)
		if a != b {
			t.Fatalf("program %d differs between two generations", i)
		}
	}
	a := g.Source(g.Program(7, 1, 80), nil)
	if a == g.Source(g.Program(8, 1, 80), nil) || a == g.Source(g.Program(7, 2, 80), nil) {
		t.Error("seed and index must change the program")
	}
}

// check type-checks the source with the Go compiler's front end.
func check(t *testing.T, name, src string) *ast.File {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, name, src, 0)
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("main", fset, []*ast.File{f}, nil); err != nil {
		t.Fatalf("%s: %v\n%s", name, err, src)
	}
	return f
}

// The programs are valid Go (so real Go runs them; gocpp checks them in the gated test).
func TestGeneratedProgramsTypecheck(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the standard library from source")
	}
	g := NewGen()
	for i := 0; i < 20; i++ {
		check(t, "gen.go", g.Source(g.Program(3, i, 250), nil))
	}
}

// Every value of the catalog is used by some case over enough programs, and the cases refer to
// values that exist.
func TestCatalogUsed(t *testing.T) {
	g := NewGen()
	used := map[int]bool{}
	for i := 0; i < 60; i++ {
		p := g.Program(11, i, 250)
		for _, c := range p.Cases {
			for _, v := range c.Vals {
				if v < 0 || v >= len(p.Vals) {
					t.Fatalf("case %d refers to value %d", c.ID, v)
				}
				used[v] = true
			}
		}
	}
	missing := 0
	for i, v := range g.Vals {
		if !used[i] && v.Decl != "" && !v.TOnly {
			missing++
			if missing < 5 {
				t.Errorf("value %s never used", v.Describe)
			}
		}
	}
}

func TestSourceOfSubset(t *testing.T) {
	g := NewGen()
	p := g.Program(5, 0, 60)
	full := g.Source(p, nil)
	sub := g.Source(p, map[int]bool{3: true, 10: true})
	if strings.Count(sub, "\tc(") != 2 || strings.Count(full, "\tc(") != 60 {
		t.Errorf("cases: full %d, subset %d", strings.Count(full, "\tc("), strings.Count(sub, "\tc("))
	}
	if len(sub) >= len(full) {
		t.Error("the subset declares as many values as the whole")
	}
}

func TestPrune(t *testing.T) {
	g := NewGen()
	p := g.Program(5, 0, 60)
	src := g.Source(p, map[int]bool{3: true, 10: true})
	pruned, err := Prune(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(pruned) >= len(src)/2 {
		t.Errorf("pruned %d bytes of %d", len(pruned), len(src))
	}
	if testing.Short() {
		return
	}
	check(t, "pruned.go", pruned)
	if strings.Contains(pruned, "type Fmtr") && !strings.Contains(src[strings.Index(src, "func main"):], "Fmtr") {
		t.Error("a type that no case uses stayed")
	}
}

func TestPruneKeepsMethods(t *testing.T) {
	src := `package main

import (
	"errors"
	"fmt"
	"math"
)

type A struct{ n int }

func (a A) String() string { return helper(a.n) }

func helper(n int) string { return fmt.Sprint("A", n) }

type B struct{}

func (B) String() string { return "B" }

var _ = errors.New
var _ = math.Pi

var v1 A = A{1}
var v2 B

func c(id int, s string) { fmt.Printf("%d %q\n", id, s) }

func main() {
	c(1, fmt.Sprintf("%v", v1))
}
`
	got, err := Prune(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"type A struct", "func (a A) String()", "func helper", "var v1 A", `import "fmt"`} {
		if !strings.Contains(got, want) {
			t.Errorf("pruned source lacks %q:\n%s", want, got)
		}
	}
	for _, bad := range []string{"type B", "v2", "errors", "math"} {
		if strings.Contains(got, bad) {
			t.Errorf("pruned source still has %q:\n%s", bad, got)
		}
	}
}

func TestParseOut(t *testing.T) {
	m := parseOut("1 \"a b\"\n2 \"\"\nnot a case\n10 \"x\\ny\"\n")
	if len(m) != 3 || m[1] != `"a b"` || m[10] != `"x\ny"` {
		t.Errorf("parseOut = %v", m)
	}
}

func TestTestFile(t *testing.T) {
	txt, err := testFile("package main\n\nfunc main() {}\n", "1 \"a\"\n\n2 \"b\"\n", "", 3, 4, "1,2")
	if err != nil {
		t.Fatal(err)
	}
	want := "// Output:\n// 1 \"a\"\n//\n// 2 \"b\"\n"
	if !strings.HasPrefix(txt, "// phase: 05\n") || !strings.HasSuffix(txt, want) {
		t.Errorf("test file:\n%s", txt)
	}
	if _, err := testFile("package main\n", "1 \"a\" \n", "", 1, 1, "1"); err == nil {
		t.Error("an output line that ends in a space was accepted")
	}
}

func TestIsolate(t *testing.T) {
	r := &Runner{}
	budget := 100
	oc := &Outcome{}
	bad := map[int]bool{7: true, 30: true}
	set := make([]int, 40)
	for i := range set {
		set[i] = i + 1
	}
	got := r.isolate(&budget, oc, set, func(sub map[int]bool) bool {
		for id := range sub {
			if bad[id] {
				return true
			}
		}
		return false
	})
	if len(got) != 2 || !bad[got[0]] || !bad[got[1]] {
		t.Errorf("isolate = %v", got)
	}
	// a failure that needs two cases
	budget = 100
	got = r.isolate(&budget, oc, set[:8], func(sub map[int]bool) bool { return sub[2] && sub[7] })
	if len(got) != 8 {
		t.Errorf("isolate of an interaction = %v", got)
	}
}

// Every value of the catalog is valid Go.
func TestAllValuesTypecheck(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the standard library from source")
	}
	g := NewGen()
	p := &Program{Seed: 1, Vals: g.Vals}
	for i, v := range g.Vals {
		p.Cases = append(p.Cases, Case{ID: i + 1, Expr: "fmt.Sprint(" + v.Op + ")", Vals: []int{i}})
	}
	check(t, "all.go", g.Source(p, nil))
}
