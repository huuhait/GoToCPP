package diag

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"strings"
	"testing"
)

func pos(file string, line, col int) token.Position {
	return token.Position{Filename: file, Line: line, Column: col}
}

// e and w make an error and a warning.
func e(file string, line, col int, msg string) Diagnostic {
	return Diagnostic{Pos: pos(file, line, col), Msg: msg}
}

func w(file string, line, col int, msg string) Diagnostic {
	return Diagnostic{Pos: pos(file, line, col), Msg: msg, Severity: Warning}
}

// errorLines returns n errors "undefined: u<i>" on lines 1 to n of a.gp,
// and the lines that Print shows for them.
func errorLines(n int) ([]Diagnostic, []string) {
	var ds []Diagnostic
	var lines []string
	for i := 1; i <= n; i++ {
		ds = append(ds, e("a.gp", i, 2, fmt.Sprintf("undefined: u%d", i)))
		lines = append(lines, fmt.Sprintf("a.gp:%d:2: undefined: u%d\n", i, i))
	}
	return ds, lines
}

// lines joins the lines and adds "too many errors" at a.gp:<stop>:2 if
// stop > 0.
func lines(ls []string, stop int) string {
	s := strings.Join(ls, "")
	if stop > 0 {
		s += fmt.Sprintf("a.gp:%d:2: too many errors\n", stop)
	}
	return s
}

func TestPrint(t *testing.T) {
	ten, tenLines := errorLines(10)
	eleven, elevenLines := errorLines(11)

	// The 11 errors, added from the last line to the first: the 10 that
	// were added first are shown, and the 10th of them is on line 2.
	var reversed []Diagnostic
	for i := len(eleven) - 1; i >= 0; i-- {
		reversed = append(reversed, eleven[i])
	}

	// 10 errors with warnings between them: the warnings don't count.
	var withWarnings []Diagnostic
	for i, d := range ten {
		withWarnings = append(withWarnings, d, w("a.gp", i+1, 9, "look"))
	}
	var withWarningsLines []string
	for i, l := range tenLines {
		withWarningsLines = append(withWarningsLines, l)
		if i < 9 { // the warning after the 10th error is not shown
			withWarningsLines = append(withWarningsLines, fmt.Sprintf("a.gp:%d:9: warning: look\n", i+1))
		}
	}

	// An error dropped as a duplicate (u1 again on line 1) does not count.
	withDup := append([]Diagnostic{eleven[0], e("a.gp", 1, 5, "undefined: u1")}, eleven[1:]...)

	related := Diagnostic{
		Pos: pos("a.gp", 4, 5),
		Msg: "x redeclared in this block",
		Related: []Related{
			{Pos: pos("a.gp", 3, 5), Msg: "other declaration of x"},
			{Msg: "no place"},
		},
	}
	relatedOther := related
	relatedOther.Pos.Column = 9
	relatedOther.Related = []Related{{Pos: pos("a.gp", 2, 5), Msg: "other declaration of x"}}
	relatedSame := related
	relatedSame.Pos.Column = 9

	tests := []struct {
		name  string
		diags []Diagnostic
		all   bool
		want  string
	}{
		{name: "empty"},
		{
			name:  "error",
			diags: []Diagnostic{e("a.gp", 3, 5, "undefined: x")},
			want:  "a.gp:3:5: undefined: x\n",
		},
		{
			name:  "warning",
			diags: []Diagnostic{w("a.gp", 3, 5, "look")},
			want:  "a.gp:3:5: warning: look\n",
		},
		{
			name:  "no position",
			diags: []Diagnostic{{Msg: "no .gp files"}},
			want:  "no .gp files\n",
		},
		{
			name:  "only a file name",
			diags: []Diagnostic{{Pos: token.Position{Filename: "a.gp"}, Msg: "can't read"}},
			want:  "a.gp: can't read\n",
		},
		{
			name:  "no column",
			diags: []Diagnostic{{Pos: token.Position{Filename: "a.gp", Line: 3}, Msg: "m"}},
			want:  "a.gp:3: m\n",
		},
		{
			name:  "by line, then column",
			diags: []Diagnostic{e("a.gp", 9, 1, "c"), e("a.gp", 2, 7, "b"), e("a.gp", 2, 3, "a")},
			want:  "a.gp:2:3: a\na.gp:2:7: b\na.gp:9:1: c\n",
		},
		{
			name:  "files in the order of their first diagnostic",
			diags: []Diagnostic{e("b.gp", 5, 1, "b5"), e("a.gp", 1, 1, "a1"), e("b.gp", 1, 1, "b1")},
			want:  "b.gp:1:1: b1\nb.gp:5:1: b5\na.gp:1:1: a1\n",
		},
		{
			name:  "no file name first",
			diags: []Diagnostic{e("a.gp", 1, 1, "a"), {Msg: "none"}},
			want:  "none\na.gp:1:1: a\n",
		},
		{
			name:  "same place: the order they were added",
			diags: []Diagnostic{e("a.gp", 1, 1, "zz"), e("a.gp", 1, 1, "aa")},
			want:  "a.gp:1:1: zz\na.gp:1:1: aa\n",
		},
		{
			name:  "same line and message: dropped",
			diags: []Diagnostic{e("a.gp", 4, 10, "undefined: x"), e("a.gp", 4, 14, "undefined: x")},
			want:  "a.gp:4:10: undefined: x\n",
		},
		{
			name:  "same line and message, not in a row: kept",
			diags: []Diagnostic{e("a.gp", 4, 10, "undefined: x"), e("a.gp", 4, 13, "undefined: y"), e("a.gp", 4, 16, "undefined: x")},
			want:  "a.gp:4:10: undefined: x\na.gp:4:13: undefined: y\na.gp:4:16: undefined: x\n",
		},
		{
			name:  "same message on another line: kept",
			diags: []Diagnostic{e("a.gp", 4, 10, "undefined: x"), e("a.gp", 5, 10, "undefined: x")},
			want:  "a.gp:4:10: undefined: x\na.gp:5:10: undefined: x\n",
		},
		{
			name:  "same line and message in another file: kept",
			diags: []Diagnostic{e("a.gp", 4, 10, "undefined: x"), e("b.gp", 4, 10, "undefined: x")},
			want:  "a.gp:4:10: undefined: x\nb.gp:4:10: undefined: x\n",
		},
		{
			name:  "same line, other message: kept",
			diags: []Diagnostic{e("a.gp", 4, 10, "undefined: x"), e("a.gp", 4, 14, "undefined: y")},
			want:  "a.gp:4:10: undefined: x\na.gp:4:14: undefined: y\n",
		},
		{
			name:  "exact duplicate, not in a row: dropped",
			diags: []Diagnostic{e("a.gp", 1, 1, "A"), e("a.gp", 2, 1, "B"), e("a.gp", 1, 1, "A")},
			want:  "a.gp:1:1: A\na.gp:2:1: B\n",
		},
		{
			name:  "a warning between: the error is still dropped",
			diags: []Diagnostic{e("a.gp", 4, 10, "undefined: x"), w("a.gp", 4, 12, "look"), e("a.gp", 4, 14, "undefined: x")},
			want:  "a.gp:4:10: undefined: x\na.gp:4:12: warning: look\n",
		},
		{
			name:  "warnings on the same line: kept",
			diags: []Diagnostic{w("a.gp", 4, 1, "look"), w("a.gp", 4, 5, "look")},
			want:  "a.gp:4:1: warning: look\na.gp:4:5: warning: look\n",
		},
		{
			name:  "exact duplicate warnings: dropped",
			diags: []Diagnostic{w("a.gp", 4, 1, "look"), w("a.gp", 4, 1, "look")},
			want:  "a.gp:4:1: warning: look\n",
		},
		{
			name:  "error and warning with the same text: kept",
			diags: []Diagnostic{e("a.gp", 4, 1, "m"), w("a.gp", 4, 1, "m")},
			want:  "a.gp:4:1: m\na.gp:4:1: warning: m\n",
		},
		{
			name:  "related places",
			diags: []Diagnostic{related},
			want:  "a.gp:4:5: x redeclared in this block\n\ta.gp:3:5: other declaration of x\n\tno place\n",
		},
		{
			name:  "same line and message, other related places: kept",
			diags: []Diagnostic{related, relatedOther},
			want: "a.gp:4:5: x redeclared in this block\n\ta.gp:3:5: other declaration of x\n\tno place\n" +
				"a.gp:4:9: x redeclared in this block\n\ta.gp:2:5: other declaration of x\n",
		},
		{
			name:  "same line, message and related places: dropped",
			diags: []Diagnostic{related, relatedSame},
			want:  "a.gp:4:5: x redeclared in this block\n\ta.gp:3:5: other declaration of x\n\tno place\n",
		},
		{
			name:  "9 errors",
			diags: ten[:9],
			want:  lines(tenLines[:9], 0),
		},
		{
			name:  "exactly 10 errors: too many errors",
			diags: ten,
			want:  lines(tenLines, 10),
		},
		{
			name:  "11 errors: the first 10",
			diags: eleven,
			want:  lines(tenLines, 10),
		},
		{
			name:  "11 errors with All",
			diags: eleven,
			all:   true,
			want:  lines(elevenLines, 0),
		},
		{
			name:  "10 errors with All",
			diags: ten,
			all:   true,
			want:  lines(tenLines, 0),
		},
		{
			name:  "the first 10 that were added",
			diags: reversed,
			want:  lines(elevenLines[1:], 2),
		},
		{
			name:  "warnings don't count",
			diags: withWarnings,
			want:  lines(withWarningsLines, 10),
		},
		{
			name:  "a dropped duplicate doesn't count",
			diags: withDup,
			want:  lines(tenLines, 10),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l List
			for _, d := range tt.diags {
				l.Add(d)
			}
			var b strings.Builder
			p := Printer{All: tt.all}
			if err := p.Print(&b, &l); err != nil {
				t.Fatal(err)
			}
			if got := b.String(); got != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestErrorf(t *testing.T) {
	var l List
	l.Errorf(pos("a.gp", 2, 3), "unknown directive %q", "pkgconfig")
	l.Errorf(pos("a.gp", 3, 1), "%s", "100% plain")
	want := []Diagnostic{
		e("a.gp", 2, 3, `unknown directive "pkgconfig"`),
		e("a.gp", 3, 1, "100% plain"),
	}
	if got := l.Diagnostics(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
}

func TestCollect(t *testing.T) {
	const src = `package p

var x int
var x string

func f() {
	a, b := 1
	_ = undefinedName
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "/w/a.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var l List
	conf := types.Config{Error: l.Collect}
	conf.Check("p", fset, []*ast.File{f}, nil)

	p := func(line, col int) token.Position {
		return fset.File(f.Pos()).Position(fset.File(f.Pos()).LineStart(line) + token.Pos(col-1))
	}
	// Code and End come from unexported fields of types.Error. If this
	// fails because they are empty, go/types has renamed the fields: fix
	// typeErrorData. The codes are DuplicateDecl (10), UnusedVar (101),
	// WrongAssignCount (17) and UndeclaredName (75).
	want := []Diagnostic{
		{
			Pos:  p(4, 5),
			Msg:  "x redeclared in this block",
			Code: 10,
			// go/types reports it as a second error, "\tother declaration of x".
			Related: []Related{{Pos: p(3, 5), Msg: "other declaration of x"}},
		},
		{Pos: p(7, 2), Msg: "declared and not used: a", Soft: true, Code: 101},
		{Pos: p(7, 5), Msg: "declared and not used: b", Soft: true, Code: 101},
		{Pos: p(7, 10), End: p(7, 11), Msg: "assignment mismatch: 2 variables but 1 value", Code: 17},
		{Pos: p(8, 6), End: p(8, 19), Msg: "undefined: undefinedName", Code: 75},
	}
	if got := l.Diagnostics(); !reflect.DeepEqual(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", dump(got), dump(want))
	}
	if !l.HasErrors() {
		t.Error("HasErrors() = false, want true")
	}
}

// importerFunc is a types.Importer made from a function.
type importerFunc func(path string) (*types.Package, error)

func (f importerFunc) Import(path string) (*types.Package, error) { return f(path) }

// TestCollectUnusedImport checks that a real unused import gets the code
// UnusedImport, from ErrorCode and in Diagnostic.Code. Step 1.10 finds
// those errors by their code, with a filter in front of Collect.
func TestCollectUnusedImport(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "a.go", "package p\n\nimport \"example.com/fake\"\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	var l List
	var codes []int // what a filter in front of Collect sees
	conf := types.Config{
		// An empty package is enough: the import is not used.
		Importer: importerFunc(func(path string) (*types.Package, error) {
			pkg := types.NewPackage(path, "fake")
			pkg.MarkComplete()
			return pkg, nil
		}),
		Error: func(err error) {
			codes = append(codes, ErrorCode(err.(types.Error)))
			l.Collect(err)
		},
	}
	conf.Check("p", fset, []*ast.File{f}, nil)
	if !reflect.DeepEqual(codes, []int{UnusedImport}) {
		t.Errorf("ErrorCode gave %v, want [%d]", codes, UnusedImport)
	}
	// An error that go/types didn't make has no code.
	if code := ErrorCode(types.Error{Msg: "made by hand"}); code != 0 {
		t.Errorf("ErrorCode of a hand-made error = %d, want 0", code)
	}
	// go/types gives this error no range, so End is empty.
	want := []Diagnostic{{
		Pos:  token.Position{Filename: "a.go", Offset: 18, Line: 3, Column: 8},
		Msg:  `"example.com/fake" imported and not used`,
		Soft: true,
		Code: UnusedImport,
	}}
	if got := l.Diagnostics(); !reflect.DeepEqual(got, want) {
		t.Errorf("got:\n%s\nwant:\n%s", dump(got), dump(want))
	}
}

// TestCollectUnusedVar checks that a real unused variable gets the code
// UnusedVar, from ErrorCode and in Diagnostic.Code. Step 1.10 holds those
// errors back too, with the filter in front of Collect.
func TestCollectUnusedVar(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "a.go", "package p\n\nfunc f() {\n\tx := 1\n}\n", 0)
	if err != nil {
		t.Fatal(err)
	}
	var l List
	var codes []int
	conf := types.Config{Error: func(err error) {
		codes = append(codes, ErrorCode(err.(types.Error)))
		l.Collect(err)
	}}
	conf.Check("p", fset, []*ast.File{f}, nil)
	if !reflect.DeepEqual(codes, []int{UnusedVar}) {
		t.Errorf("ErrorCode gave %v, want [%d]", codes, UnusedVar)
	}
	if ds := l.Diagnostics(); len(ds) != 1 || ds[0].Code != UnusedVar || ds[0].Msg != "declared and not used: x" {
		t.Errorf("got:\n%s", dump(ds))
	}
}

func dump(ds []Diagnostic) string {
	var b strings.Builder
	for _, d := range ds {
		fmt.Fprintf(&b, "\t%+v\n", d)
	}
	return b.String()
}

func TestCollectOther(t *testing.T) {
	fset := token.NewFileSet()
	file := fset.AddFile("a.gp", -1, 100)
	file.SetLines([]int{0, 10, 20})
	at := file.Pos(12) // line 2, column 3

	var l List
	// An error that is not a types.Error: no position.
	l.Collect(errors.New("boom"))
	// A types.Error without a FileSet: no position either.
	l.Collect(types.Error{Msg: "no file set"})
	// A tab-indented error with nothing before it: added on its own, without the tab.
	var l2 List
	l2.Collect(types.Error{Fset: fset, Pos: at, Msg: "\tother declaration of x"})

	want := []Diagnostic{{Msg: "boom"}, {Msg: "no file set"}}
	if got := l.Diagnostics(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v\nwant %+v", got, want)
	}
	want2 := []Diagnostic{{Pos: token.Position{Filename: "a.gp", Offset: 12, Line: 2, Column: 3}, Msg: "other declaration of x"}}
	if got := l2.Diagnostics(); !reflect.DeepEqual(got, want2) {
		t.Errorf("got %+v\nwant %+v", got, want2)
	}
}

func TestHasErrors(t *testing.T) {
	var l List
	if l.HasErrors() {
		t.Error("empty list: HasErrors() = true")
	}
	l.Add(w("a.gp", 1, 1, "look"))
	if l.HasErrors() {
		t.Error("only a warning: HasErrors() = true")
	}
	l.Errorf(pos("a.gp", 2, 1), "bad")
	if !l.HasErrors() {
		t.Error("with an error: HasErrors() = false")
	}
}

func TestDiagnostics(t *testing.T) {
	var l List
	if got := l.Diagnostics(); len(got) != 0 {
		t.Errorf("empty list: got %v", got)
	}
	twelve, _ := errorLines(12)
	for i := len(twelve) - 1; i >= 0; i-- {
		l.Add(twelve[i])
	}
	l.Add(Diagnostic{Pos: pos("a.gp", 20, 1), Msg: "r", Related: []Related{{Msg: "first"}}})

	// All of them, sorted, without the limit.
	got := l.Diagnostics()
	if len(got) != 13 {
		t.Fatalf("got %d diagnostics, want 13", len(got))
	}
	for i, d := range got[:12] {
		if d.Pos.Line != i+1 {
			t.Errorf("diagnostic %d is on line %d, want %d", i, d.Pos.Line, i+1)
		}
	}

	// The result is a copy, also its Related places.
	got[0].Msg = "changed"
	got[12].Related[0].Msg = "changed"
	again := l.Diagnostics()
	if again[0].Msg != "undefined: u1" || again[12].Related[0].Msg != "first" {
		t.Errorf("changing the result changed the list: %+v, %+v", again[0], again[12])
	}
}

func TestSeverityString(t *testing.T) {
	for s, want := range map[Severity]string{Error: "error", Warning: "warning", 7: "Severity(7)"} {
		if got := s.String(); got != want {
			t.Errorf("Severity(%d).String() = %q, want %q", int(s), got, want)
		}
	}
}
