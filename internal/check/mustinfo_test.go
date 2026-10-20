package check

import (
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/syntax/gpast"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// TestMustInfo: Package.Must says where the `!` forms were and what each does on an error, although
// the trees are plain Go after the rewrite.
func TestMustInfo(t *testing.T) {
	const src = `package main

import (
	"os"
	"strconv"
	"testing"
)

var port = strconv.Atoi!("80")

func ret(s string) (int, error) {
	n := strconv.Atoi!(s)
	defer os.Remove!("x")
	return n, nil
}

func must!(s string) int {
	f := func() error {
		os.Remove!(s)
		return nil
	}
	g := func() {
		os.Remove!(s)
	}
	f!()
	g()
	return strconv.Atoi!(s)
}

func plain(s string) int {
	return strconv.Atoi!(s)
}

func inTest(t *testing.T, s string) int {
	h := func() int { return strconv.Atoi!(s) }
	return h()
}

func main() {
	os.Remove!("y")
}

func noBang() { os.Remove("z") }

func dropped!(s string) { os.Remove(s) }
`
	dir := writeTree(t, map[string]string{"main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, dir, "main.gp")
	if txt := errorText(t, prog, r, dir, true); txt != "" {
		t.Fatalf("errors:\n%s", txt)
	}
	p := mainPkg(t, prog, r)
	if p.Must == nil {
		t.Fatal("no Must info")
	}
	var funcs []string
	for _, f := range p.Must.Funcs {
		funcs = append(funcs, fmt.Sprintf("%s exit=%v", f.Decl.Name.Name, f.Exit))
	}
	if got, want := strings.Join(funcs, ","), "must exit=false,dropped exit=false"; got != want {
		t.Errorf("Funcs = %q, want %q", got, want)
	}
	// Each call by the line of its text and the function that holds it.
	var got []string
	for _, c := range p.Must.Calls {
		pos := prog.Fset.Position(c.Pos)
		fn := "<var>"
		switch f := c.Func.(type) {
		case *ast.FuncDecl:
			fn = f.Name.Name
		case *ast.FuncLit:
			fn = "lit@" + fmt.Sprint(prog.Fset.Position(f.Pos()).Line)
		}
		got = append(got, fmt.Sprintf("%d:%s:%s:defer=%v", pos.Line, fn, c.Kind, c.Deferred))
	}
	want := []string{
		"9:<var>:panic:defer=false",
		"12:ret:return:defer=false",
		"13:ret:return:defer=true",
		"19:lit@18:return:defer=false",
		"23:lit@22:panic:defer=false",
		"25:must:return:defer=false",
		"27:must:return:defer=false",
		"31:plain:panic:defer=false",
		"35:lit@35:fatal:defer=false",
		"40:must_main:exit:defer=false", // a main with ! is renamed, and the real main calls it
	}
	// The order is the order of the lowering; compare as sets by line.
	if len(got) != len(want) {
		t.Fatalf("calls:\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	seen := map[string]bool{}
	for _, g := range got {
		seen[g] = true
	}
	for _, w := range want {
		if !seen[w] {
			t.Errorf("missing %s; got:\n%s", w, strings.Join(got, "\n"))
		}
	}
}

// A package without `!` has no Must info, and neither has one with errors.
func TestMustInfoNone(t *testing.T) {
	dir := writeTree(t, map[string]string{"main.gp": "package main\n\nfunc main() {}\n"})
	prog, r := loadAndCheck(t, load.Config{}, dir, "main.gp")
	if p := mainPkg(t, prog, r); p.Must != nil {
		t.Errorf("Must = %+v", p.Must)
	}
}

// main! and init! end the program: Exit, and their calls are MustExit.
func TestMustInfoExit(t *testing.T) {
	const src = `package main

import "os"

func init!() {
	os.Remove!("a")
}

func main!() {
	os.Remove!("b")
}
`
	dir := writeTree(t, map[string]string{"main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, dir, "main.gp")
	if txt := errorText(t, prog, r, dir, true); txt != "" {
		t.Fatalf("errors:\n%s", txt)
	}
	p := mainPkg(t, prog, r)
	if len(p.Must.Funcs) != 2 || !p.Must.Funcs[0].Exit || !p.Must.Funcs[1].Exit || p.Must.Funcs[0].Name != "init" || p.Must.Funcs[1].Name != "main" {
		t.Errorf("Funcs = %+v", p.Must.Funcs)
	}
	if len(p.Must.Calls) != 2 || p.Must.Calls[0].Kind != MustExit || p.Must.Calls[1].Kind != MustExit {
		t.Errorf("Calls = %+v", p.Must.Calls)
	}
}

// TestMustInfoCountsEveryCall: Package.Must has exactly one entry for each F!(...) that the user
// wrote, in every form the lowering handles (statements, var declarations of every shape, returns,
// conditions, defers, ...). The count of the source is the number of calls whose callee name ends
// with ! in the parsed file; the programs are the language tests of Phase 10.
func TestMustInfoCountsEveryCall(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("..", "..", "tests", "run", "must_*.gp"))
	if err != nil || len(files) < 100 {
		t.Fatalf("must corpus: %d files, %v", len(files), err)
	}
	checked := 0
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fset := token.NewFileSet()
		tree, err := parser.ParseFile(fset, f, src, parser.Phase10Syntax)
		if err != nil {
			continue
		}
		want := 0
		ast.Inspect(tree, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id := calleeIdent(call.Fun); id != nil && gpast.IsMust(id.Name) {
					want++
				}
			}
			return true
		})
		prog, r := loadAndCheck(t, load.Config{Dir: filepath.Dir(f)}, filepath.Dir(f), f)
		if errorText(t, prog, r, filepath.Dir(f), true) != "" {
			continue // an error test or a program of a later phase
		}
		p := mainPkg(t, prog, r)
		got := 0
		if p.Must != nil {
			got = len(p.Must.Calls)
		}
		if got != want {
			t.Errorf("%s: %d explicit calls in the source, %d in Must.Calls", filepath.Base(f), want, got)
		}
		checked++
	}
	if checked < 100 {
		t.Errorf("only %d programs were checked", checked)
	}
}

// Every shape of a var declaration with a ! call is recorded.
func TestMustInfoVarForms(t *testing.T) {
	const src = `package main

import "strconv"

func two() (int, int, error) { return 1, 2, nil }

func main() {
	var a = strconv.Atoi!("1")
	var b int = strconv.Atoi!("2")
	var c, d = two!()
	var (
		e = strconv.Atoi!("3")
	)
	_, _, _, _, _ = a, b, c, d, e
}

var g = strconv.Atoi!("4")
var h, i = two!()
`
	dir := writeTree(t, map[string]string{"main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, dir, "main.gp")
	if txt := errorText(t, prog, r, dir, true); txt != "" {
		t.Fatalf("errors:\n%s", txt)
	}
	if got := len(mainPkg(t, prog, r).Must.Calls); got != 6 {
		t.Errorf("Calls = %d, want 6", got)
	}
}

// TestMustInfoEditorData: what the language server reads from Package.Must (step 11.13): the place of the
// ! and of the callee's name of each written call, the calls that a must function handles without a !, and
// for each statement that the lowering rewrote the code that stands for it.
func TestMustInfoEditorData(t *testing.T) {
	const src = `package main

import (
	"os"
	"strconv"
)

func ret(s string) (int, error) {
	n := strconv.Atoi!(s)
	defer os.Remove!("x")
	return n, nil
}

func dropped!(s string) {
	os.Remove(s)
	n := strconv.Atoi(s)
	_ = n
	defer os.Remove("y")
}

func main() {
	os.Remove!("y")
}
`
	dir := writeTree(t, map[string]string{"main.gp": src})
	prog, r := loadAndCheck(t, load.Config{}, dir, "main.gp")
	if txt := errorText(t, prog, r, dir, true); txt != "" {
		t.Fatalf("errors:\n%s", txt)
	}
	p := mainPkg(t, prog, r)
	text := func(pos token.Pos, n int) string {
		off := prog.Fset.File(pos).Offset(pos)
		return src[off : off+n]
	}
	// the written calls: the ! and the name
	var names []string
	for _, c := range p.Must.Calls {
		if !c.Bang.IsValid() || text(c.Bang, 1) != "!" {
			t.Errorf("a call at %v: Bang %v", prog.Fset.Position(c.Pos), c.Bang)
			continue
		}
		off, bang := prog.Fset.File(c.Name).Offset(c.Name), prog.Fset.File(c.Bang).Offset(c.Bang)
		names = append(names, src[off:bang])
	}
	if got, want := strings.Join(names, ","), "Atoi,Remove,Remove"; got != want {
		t.Errorf("names of the written calls = %q, want %q", got, want)
	}
	// the must function
	if len(p.Must.Funcs) != 1 || text(p.Must.Funcs[0].Bang, 1) != "!" || prog.Fset.Position(p.Must.Funcs[0].Bang).Line != 14 {
		t.Errorf("Funcs = %+v", p.Must.Funcs)
	}
	// the calls that dropped! handles: lines 15, 16 and 18 (the deferred one)
	var handled []string
	for _, c := range p.Must.Handled {
		if c.Bang.IsValid() {
			t.Errorf("a handled call has a !: %+v", c)
		}
		handled = append(handled, fmt.Sprintf("%d:%s:%s:defer=%v:%s", prog.Fset.Position(c.Pos).Line, c.Kind, text(c.Name, 6), c.Deferred, text(c.Pos, 6)))
	}
	slices.Sort(handled)
	want := []string{"15:return:Remove:defer=false:os.Rem", "16:return:Atoi(s:defer=false:strcon", "18:return:Remove:defer=true:os.Rem"}
	if strings.Join(handled, "|") != strings.Join(want, "|") {
		t.Errorf("Handled = %q, want %q", handled, want)
	}
	// the statements: every call is in one, and the lowered code of `n := strconv.Atoi!(s)` is the call and the check
	for _, c := range append(append([]MustCall{}, p.Must.Calls...), p.Must.Handled...) {
		in := 0
		for _, s := range p.Must.Stmts {
			if s.Pos <= c.Pos && c.End <= s.End && len(s.Lowered) > 0 {
				in++
			}
		}
		if in == 0 {
			t.Errorf("the call at line %d is in no statement", prog.Fset.Position(c.Pos).Line)
		}
	}
	found := false
	for _, s := range p.Must.Stmts {
		if prog.Fset.Position(s.Pos).Line == 9 {
			found = true
			if len(s.Lowered) != 2 {
				t.Errorf("the code for line 9 has %d statements, want 2 (the call, the check)", len(s.Lowered))
			}
		}
	}
	if !found {
		t.Errorf("no statement for line 9: %+v", p.Must.Stmts)
	}
}
