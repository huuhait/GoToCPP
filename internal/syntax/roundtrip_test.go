package syntax_test

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/format"
	fork "github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// TestStdlibRoundTrip parses every Go file of the Go standard library with
// the fork: all .go files in $(go env GOROOT)/src, outside testdata
// folders. For each file it checks two things:
//
//  1. The fork gives the same AST as the stock go/parser. The positions must
//     be the same too, and so must the legacy object resolution
//     (ast.Ident.Obj, ast.File.Scope and ast.File.Unresolved), because gocpp
//     uses it (phase 01 README §2.5). So the gocpp patches never change how
//     Go code parses.
//  2. The round trip: format.Source (internal/format) formats the file as
//     gocpp fmt does: it parses it with the fork, sorts the imports and
//     prints it with go/printer and gofmt's settings. The fork parses the
//     printed source to an equal AST again. (The imports of the standard
//     library are sorted already, so sorting them changes nothing.) Here a
//     position only has to be set on both sides or on neither, because
//     some positions mean something: CallExpr.Ellipsis is the "..." of
//     f(xs...), TypeSpec.Assign the "=" of an alias. The object resolution
//     is not compared here; check 1 does that.
//
// Check 1 also runs with the syntax of Phase 10 on (parser.Phase10Syntax,
// the mode of gocpp fmt): Go code parses in the same way.
//
// In both checks a nil slice and an empty one differ: a CaseClause with a
// nil List is "default". Both parsers keep the comments (ParseComments),
// and the comments must have the same text. A file that the stock go/parser
// rejects is skipped; the test logs it. Each folder is a subtest, and the
// folders run in parallel: the whole test takes about 2 seconds on 16 cores
// and 12 on one, so it has no -short mode.
func TestStdlibRoundTrip(t *testing.T) {
	src := filepath.Join(goroot(t), "src")
	dirs, nfiles := stdlibFiles(t, src)
	// A wrong folder or a broken walk would make this test pass without
	// checking anything. Go 1.25 has 6240 such files.
	if nfiles < 1000 {
		t.Fatalf("found only %d Go files in %s; the Go standard library has thousands", nfiles, src)
	}

	var sum summary
	t.Cleanup(func() { sum.log(t, src) })
	for _, dir := range slices.Sorted(maps.Keys(dirs)) {
		t.Run(dir, func(t *testing.T) {
			t.Parallel()
			for _, name := range dirs[dir] {
				checkFile(t, src, path.Join(dir, name), &sum)
			}
		})
	}
}

// goroot returns the GOROOT of the go command in PATH, like the fork's
// BenchmarkScanFiles (FORK.md §3): go test puts $GOROOT/bin first in PATH,
// so this is the Go that runs the test, and its go/parser is the stock one
// that the test uses.
func goroot(t *testing.T) string {
	out, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && len(exit.Stderr) > 0 {
			t.Fatalf("go env GOROOT: %v\n%s", err, exit.Stderr)
		}
		t.Fatalf("go env GOROOT: %v", err)
	}
	dir := strings.TrimSpace(string(out))
	if dir == "" {
		t.Fatal("go env GOROOT printed nothing")
	}
	return dir
}

// stdlibFiles returns the .go files under src, outside testdata folders, by
// folder. A folder is relative to src, with slashes ("net/http"). It also
// returns the number of files.
func stdlibFiles(t *testing.T, src string) (map[string][]string, int) {
	dirs := map[string][]string{}
	n := 0
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if !d.Type().IsRegular() || !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}
		rel, err := filepath.Rel(src, filepath.Dir(p))
		if err != nil {
			return err
		}
		dir := filepath.ToSlash(rel)
		dirs[dir] = append(dirs[dir], d.Name())
		n++
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return dirs, n
}

// maxShown is the number of failures that TestStdlibRoundTrip explains. A
// broken fork can make thousands of files fail, and the first ones say
// enough. The others are only counted, like "too many errors" in go build.
const maxShown = 10

// A summary collects what the subtests found: the counts and the failures.
// The subtests run in parallel, so mu guards it.
type summary struct {
	mu       sync.Mutex
	files    int
	bytes    int
	failures []failure
	rejected []string // the files that go/parser rejects, with the error
}

// A failure is the message of one failed check.
type failure struct {
	name string // the file
	msg  string
}

// fail marks t as failed and keeps the message. At the end, log shows the
// first maxShown messages by file name, so that two runs show the same
// ones, whatever order the folders ran in.
func (s *summary) fail(t *testing.T, name, format string, args ...any) {
	t.Fail()
	msg := fmt.Sprintf(format, args...)
	s.mu.Lock()
	s.failures = append(s.failures, failure{name, msg})
	s.mu.Unlock()
}

func (s *summary) checked(n int) {
	s.mu.Lock()
	s.files++
	s.bytes += n
	s.mu.Unlock()
}

func (s *summary) reject(name string, err error) {
	msg, _, _ := strings.Cut(err.Error(), "\n")
	s.mu.Lock()
	s.rejected = append(s.rejected, name+": "+msg)
	s.mu.Unlock()
}

func (s *summary) log(t *testing.T, src string) {
	t.Logf("%s: checked %d files (%.1f MB) with %s", src, s.files, float64(s.bytes)/1e6, runtime.Version())
	slices.Sort(s.rejected)
	t.Logf("skipped %d files that go/parser rejects", len(s.rejected))
	for _, r := range s.rejected {
		t.Logf("\t%s", r)
	}
	// The checks of one file run in order, and the sort keeps that order.
	slices.SortStableFunc(s.failures, func(a, b failure) int { return strings.Compare(a.name, b.name) })
	for _, f := range s.failures[:min(len(s.failures), maxShown)] {
		t.Error(f.msg)
	}
	if len(s.failures) > maxShown {
		t.Errorf("too many failures: %d in all, the first %d by file name are shown "+
			"(-run 'TestStdlibRoundTrip/<folder>' runs one folder)", len(s.failures), maxShown)
	}
}

// checkFile does the two checks of TestStdlibRoundTrip on one file. name is
// the path of the file in src, with slashes.
func checkFile(t *testing.T, src, name string, sum *summary) {
	// A panic in the fork, in internal/format or in the comparer is reported
	// with the name of the file, and the other files are still checked.
	defer func() {
		if r := recover(); r != nil {
			sum.fail(t, name, "%s: panic: %v\n%s", name, r, debug.Stack())
		}
	}()
	data, err := os.ReadFile(filepath.Join(src, filepath.FromSlash(name)))
	if err != nil {
		sum.fail(t, name, "%v", err)
		return
	}

	// 1. The fork parses the file like go/parser. Each parser gets its own
	// FileSet, so the same offset gives the same token.Pos in both.
	stockFset := token.NewFileSet()
	stock, err := parser.ParseFile(stockFset, name, data, parser.ParseComments)
	if err != nil {
		sum.reject(name, err)
		return
	}
	forkFset := token.NewFileSet()
	file, err := fork.ParseFile(forkFset, name, data, fork.ParseComments)
	if err != nil {
		sum.fail(t, name, "%s: go/parser accepts this file, but the fork does not:\n%v%s", name, err, errorLine(err, data))
		return
	}
	sum.checked(len(data))
	same := comparer{exact: true, fsets: [2]*token.FileSet{forkFset, stockFset}}
	if d := same.files(file, stock); d != nil {
		sum.fail(t, name, "%s: the fork and go/parser give different ASTs\n%s",
			same.where(name, d), same.explain(d, [2]string{"fork", "go/parser"}, [2][]byte{data, data}))
	}

	// 1b. The same with the syntax of Phase 10 on (fork.Phase10Syntax, which
	// gocpp fmt uses): no Go file changes. The standard library has no name
	// directly before !( or ![, and no #.
	p10Fset := token.NewFileSet()
	p10, err := fork.ParseFile(p10Fset, name, data, fork.ParseComments|fork.Phase10Syntax)
	if err != nil {
		sum.fail(t, name, "%s: go/parser accepts this file, but the fork with Phase10Syntax does not:\n%v%s", name, err, errorLine(err, data))
		return
	}
	same10 := comparer{exact: true, fsets: [2]*token.FileSet{p10Fset, stockFset}}
	if d := same10.files(p10, stock); d != nil {
		sum.fail(t, name, "%s: the fork with Phase10Syntax and go/parser give different ASTs\n%s",
			same10.where(name, d), same10.explain(d, [2]string{"fork (Phase10Syntax)", "go/parser"}, [2][]byte{data, data}))
	}

	// 2. The round trip: format the file as gocpp fmt does, and parse the
	// result again.
	printed, err := format.Source(name, data)
	if err != nil {
		sum.fail(t, name, "%s: format.Source: %v", name, err)
		return
	}
	againFset := token.NewFileSet()
	again, err := fork.ParseFile(againFset, name+" (printed)", printed, fork.ParseComments)
	if err != nil {
		sum.fail(t, name, "%s: the fork cannot parse what format.Source printed:\n%v%s", name, err, errorLine(err, printed))
		return
	}
	round := comparer{fsets: [2]*token.FileSet{forkFset, againFset}}
	if d := round.files(file, again); d != nil {
		sum.fail(t, name, "%s: format.Source printed source that the fork parses to a different AST\n%s",
			round.where(name, d), round.explain(d, [2]string{"source", "printed"}, [2][]byte{data, printed}))
	}
}

// A comparer compares two ASTs, a and b, field by field with reflect. It
// stops at the first difference. Side 0 is a, side 1 is b.
type comparer struct {
	// exact: positions must be equal (each file has its own FileSet, so the
	// same offset gives the same position), and so must the legacy object
	// resolution. Otherwise a position only has to be set on both sides or
	// on neither, and the object resolution is not compared.
	exact bool
	fsets [2]*token.FileSet
}

// A difference is the first place where two ASTs differ.
type difference struct {
	path  []string    // the fields and indexes, from the inside out: "[3]", ".List", ".Body"
	what  string      // how they differ: `"x" vs "y"`
	nodes [2]ast.Node // the innermost nodes around the difference, on each side
}

// String returns the path from the file to the difference, and what
// differs: `File.Decls[2].Body.List[0].Tok: := vs =`.
func (d *difference) String() string {
	var b strings.Builder
	b.WriteString("File")
	for _, p := range slices.Backward(d.path) {
		b.WriteString(p)
	}
	return b.String() + ": " + d.what
}

var (
	posType    = reflect.TypeFor[token.Pos]()
	objectType = reflect.TypeFor[*ast.Object]()
	scopeType  = reflect.TypeFor[*ast.Scope]()
	fileType   = reflect.TypeFor[ast.File]()
	unresolved = fieldIndex(fileType, "Unresolved")
)

func fieldIndex(t reflect.Type, name string) int {
	f, ok := t.FieldByName(name)
	if !ok {
		panic(t.String() + " has no field " + name)
	}
	return f.Index[0]
}

func (c *comparer) files(a, b *ast.File) *difference {
	return c.value(reflect.ValueOf(a), reflect.ValueOf(b))
}

func (c *comparer) value(a, b reflect.Value) *difference {
	switch a.Type() {
	case posType:
		return c.pos(token.Pos(a.Int()), token.Pos(b.Int()))
	case objectType:
		if !c.exact {
			return nil
		}
		return c.object(a.Interface().(*ast.Object), b.Interface().(*ast.Object))
	case scopeType:
		if !c.exact {
			return nil
		}
		return c.scope(a.Interface().(*ast.Scope), b.Interface().(*ast.Scope))
	}
	switch a.Kind() {
	case reflect.Pointer, reflect.Interface:
		if a.IsNil() || b.IsNil() {
			if a.IsNil() && b.IsNil() {
				return nil
			}
			return &difference{what: typeOrNil(a) + " vs " + typeOrNil(b)}
		}
		if a.Kind() == reflect.Interface {
			if a.Elem().Type() != b.Elem().Type() {
				return &difference{what: typeOrNil(a) + " vs " + typeOrNil(b)}
			}
			return c.value(a.Elem(), b.Elem())
		}
		d := c.value(a.Elem(), b.Elem())
		if d != nil && d.nodes[0] == nil {
			if n, ok := a.Interface().(ast.Node); ok {
				d.nodes = [2]ast.Node{n, b.Interface().(ast.Node)}
			}
		}
		return d
	case reflect.Struct:
		t := a.Type()
		for i := range a.NumField() {
			if t == fileType && i == unresolved && !c.exact {
				continue
			}
			if d := c.value(a.Field(i), b.Field(i)); d != nil {
				d.path = append(d.path, "."+t.Field(i).Name)
				return d
			}
		}
		return nil
	case reflect.Slice:
		n := min(a.Len(), b.Len())
		for i := range n {
			if d := c.value(a.Index(i), b.Index(i)); d != nil {
				d.path = append(d.path, fmt.Sprintf("[%d]", i))
				return d
			}
		}
		if a.Len() == b.Len() {
			// In go/ast, nil and empty mean different things: a CaseClause
			// with a nil List is "default", and go/printer prints "case :"
			// for an empty one.
			if a.IsNil() != b.IsNil() {
				return &difference{what: nilOrEmpty(a) + " vs " + nilOrEmpty(b)}
			}
			return nil
		}
		what := fmt.Sprintf("%d vs %d elements", a.Len(), b.Len())
		side, long := 0, a
		if b.Len() > n {
			side, long = 1, b
		}
		if node, ok := long.Index(n).Interface().(ast.Node); ok {
			what += ", first extra one at " + c.showPos(side, node.Pos())
		}
		return &difference{what: what}
	case reflect.String:
		if a.String() != b.String() {
			return &difference{what: fmt.Sprintf("%q vs %q", a.String(), b.String())}
		}
		return nil
	case reflect.Bool:
		if a.Bool() != b.Bool() {
			return &difference{what: fmt.Sprintf("%v vs %v", a.Bool(), b.Bool())}
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// token.Token, ast.ChanDir, ast.ObjKind, ...: %v prints their names.
		if a.Int() != b.Int() {
			return &difference{what: fmt.Sprintf("%v vs %v", a.Interface(), b.Interface())}
		}
		return nil
	}
	// go/ast has no other kinds of fields today. A new one must not be
	// skipped without anybody noticing.
	panic("comparer: a field of type " + a.Type().String() + " is not handled")
}

func typeOrNil(v reflect.Value) string {
	if v.IsNil() {
		return "nil"
	}
	if v.Kind() == reflect.Interface {
		return v.Elem().Type().String()
	}
	return v.Type().String()
}

// nilOrEmpty describes a slice of length 0.
func nilOrEmpty(v reflect.Value) string {
	if v.IsNil() {
		return "nil"
	}
	return "empty slice"
}

func (c *comparer) pos(a, b token.Pos) *difference {
	if c.exact && a == b || !c.exact && a.IsValid() == b.IsValid() {
		return nil
	}
	return &difference{what: c.showPos(0, a) + " vs " + c.showPos(1, b)}
}

// showPos returns p as line:column. It ignores //line directives, so the
// line is the real one.
func (c *comparer) showPos(side int, p token.Pos) string {
	if !p.IsValid() {
		return "token.NoPos"
	}
	pos := c.fsets[side].PositionFor(p, false)
	return fmt.Sprintf("%d:%d", pos.Line, pos.Column)
}

// object compares two objects of the legacy object resolution (exact mode
// only). An object points to its declaration, and the declaration back to
// the object, so the declarations are compared only by their type and
// position, which also says which declaration it is.
func (c *comparer) object(a, b *ast.Object) *difference {
	if a == nil && b == nil {
		return nil
	}
	diff := func(what string) *difference {
		return &difference{what: c.showObject(0, a) + " vs " + c.showObject(1, b) + what}
	}
	if a == nil || b == nil || a.Kind != b.Kind || a.Name != b.Name {
		return diff("")
	}
	if a.Data != b.Data {
		return diff(fmt.Sprintf(" (Data %v vs %v)", a.Data, b.Data))
	}
	if (a.Type == nil) != (b.Type == nil) {
		return diff(fmt.Sprintf(" (Type %v vs %v)", a.Type, b.Type))
	}
	da, _ := a.Decl.(ast.Node)
	db, _ := b.Decl.(ast.Node)
	if reflect.TypeOf(a.Decl) != reflect.TypeOf(b.Decl) || da != nil && da.Pos() != db.Pos() {
		return diff(fmt.Sprintf(" (declared by %T vs %T)", a.Decl, b.Decl))
	}
	return nil
}

func (c *comparer) showObject(side int, o *ast.Object) string {
	if o == nil {
		return "no object"
	}
	if n, ok := o.Decl.(ast.Node); ok {
		return fmt.Sprintf("%s %s declared at %s", o.Kind, o.Name, c.showPos(side, n.Pos()))
	}
	return fmt.Sprintf("%s %s", o.Kind, o.Name)
}

func (c *comparer) scope(a, b *ast.Scope) *difference {
	if a == nil || b == nil {
		if a == nil && b == nil {
			return nil
		}
		return &difference{what: typeOrNil(reflect.ValueOf(a)) + " vs " + typeOrNil(reflect.ValueOf(b))}
	}
	if d := c.scope(a.Outer, b.Outer); d != nil {
		d.path = append(d.path, ".Outer")
		return d
	}
	names := slices.Collect(maps.Keys(a.Objects))
	for name := range b.Objects {
		if a.Objects[name] == nil {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	for _, name := range names {
		if d := c.object(a.Objects[name], b.Objects[name]); d != nil {
			d.path = append(d.path, fmt.Sprintf(".Objects[%q]", name))
			return d
		}
	}
	return nil
}

// where returns the file name and the line and column of the difference on
// side a, if it has one.
func (c *comparer) where(name string, d *difference) string {
	if n := d.nodes[0]; n != nil && n.Pos().IsValid() {
		if _, ok := n.(*ast.File); !ok {
			return name + ":" + c.showPos(0, n.Pos())
		}
	}
	return name
}

// explain says where the difference is and what differs, with the source
// line on each side. names name the sides, and srcs are their sources.
func (c *comparer) explain(d *difference, names [2]string, srcs [2][]byte) string {
	s := fmt.Sprintf("\t%s (%s vs %s)", d, names[0], names[1])
	var lines [2]int // 0: no line to show
	var texts [2]string
	for side, n := range d.nodes {
		if n == nil || !n.Pos().IsValid() {
			continue
		}
		if _, ok := n.(*ast.File); ok {
			continue // the line of the package clause would not help
		}
		lines[side], texts[side] = sourceLine(srcs[side], c.fsets[side].PositionFor(n.Pos(), false).Offset)
	}
	if c.exact && lines[0] == lines[1] {
		// The same line of the same source: show it once.
		if lines[0] != 0 {
			s += fmt.Sprintf("\n\tline %d: %s", lines[0], texts[0])
		}
		return s
	}
	for side, line := range lines {
		if line != 0 {
			s += fmt.Sprintf("\n\t%s line %d: %s", names[side], line, texts[side])
		}
	}
	return s
}

// errorLine returns the line of src with the first syntax error of err, to
// show it under the error.
func errorLine(err error, src []byte) string {
	var list scanner.ErrorList
	if !errors.As(err, &list) || len(list) == 0 {
		return ""
	}
	line, text := sourceLine(src, list[0].Pos.Offset)
	return fmt.Sprintf("\n\tline %d: %s", line, text)
}

// sourceLine returns the number and the text of the line of src at offset.
// The number is counted in src, so a //line directive doesn't change it.
// The text is without the space around it, and shortened if it is long.
func sourceLine(src []byte, offset int) (int, string) {
	start := bytes.LastIndexByte(src[:offset], '\n') + 1
	end := bytes.IndexByte(src[offset:], '\n')
	if end < 0 {
		end = len(src)
	} else {
		end += offset
	}
	text := []rune(strings.TrimSpace(string(src[start:end])))
	if len(text) > 120 {
		text = append(text[:120:120], []rune("...")...)
	}
	return bytes.Count(src[:start], []byte("\n")) + 1, string(text)
}

// TestCompare checks the comparer of TestStdlibRoundTrip itself: if it
// missed differences, that test would pass without checking anything.
func TestCompare(t *testing.T) {
	// use returns the x of "x + y" in xy.
	use := func(f *ast.File) *ast.Ident {
		return f.Decls[1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0].(*ast.BinaryExpr).X.(*ast.Ident)
	}
	const xy = "package p\n\nvar x = 1\nvar _ = x + y\n"
	for _, tt := range []struct {
		a, b   string
		exact  bool
		change func(b *ast.File) // changes the AST of b before the comparison
		want   string            // the difference, or "" if there is none
	}{
		{a: "package p; var _ = f(a, b)", b: "package p; var _ = f(a, b, c)", exact: true,
			want: "File.Decls[0].Specs[0].Values[0].Args: 2 vs 3 elements, first extra one at 1:28"},
		{a: "package p; func f() { x := 1 }", b: "package p; func f() { x = 1 }",
			want: "File.Decls[0].Body.List[0].Tok: := vs ="},
		{a: "package p; var _ = a", b: "package p; var _ = a()", exact: true,
			want: "File.Decls[0].Specs[0].Values[0]: *ast.Ident vs *ast.CallExpr"},
		{a: "package p; func f()", b: "package p; func f() {}",
			want: "File.Decls[0].Body: nil vs *ast.BlockStmt"},
		{a: "// a\npackage p", b: "// b\npackage p",
			want: `File.Doc.List[0].Text: "// a" vs "// b"`},
		// The label's statement is the empty statement before "}", which has
		// no ";" in the source (Implicit), or an explicit ";".
		{a: "package p; func f() { L: }", b: "package p; func f() { L: ; }",
			want: "File.Decls[0].Body.List[0].Stmt.Implicit: true vs false"},
		// "default:" has a nil List; an empty List would print as "case :".
		{a: "package p; func f() { switch { default: } }", b: "package p; func f() { switch { default: } }",
			change: func(b *ast.File) {
				b.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.SwitchStmt).Body.List[0].(*ast.CaseClause).List = []ast.Expr{}
			},
			want: "File.Decls[0].Body.List[0].Body.List[0].List: nil vs empty slice"},
		{a: "package p; var x = 1", b: "package p; var  x = 1", exact: true,
			want: "File.Decls[0].Specs[0].Names[0].NamePos: 1:16 vs 1:17"},
		// Without exact, only the set positions must match.
		{a: "package p; var x = 1", b: "package p; var  x = 1"},
		{a: "package p; type A = int", b: "package p; type A int",
			want: "File.Decls[0].Specs[0].Assign: 1:19 vs token.NoPos"},
		{a: "package p; var _ = f(xs...)", b: "package p; var _ = f(xs)",
			want: "File.Decls[0].Specs[0].Values[0].Ellipsis: 1:24 vs token.NoPos"},
		// The object resolution, with exact only.
		{a: xy, b: xy, exact: true, change: func(b *ast.File) { use(b).Obj = nil },
			want: "File.Decls[1].Specs[0].Values[0].X.Obj: var x declared at 3:5 vs no object"},
		{a: xy, b: xy, change: func(b *ast.File) { use(b).Obj = nil }},
		{a: xy, b: xy, exact: true, change: func(b *ast.File) { b.Scope.Objects["x"].Kind = ast.Con },
			want: "File.Decls[0].Specs[0].Names[0].Obj: var x declared at 3:5 vs const x declared at 3:5"},
		{a: xy, b: xy, exact: true, change: func(b *ast.File) { delete(b.Scope.Objects, "x") },
			want: `File.Scope.Objects["x"]: var x declared at 3:5 vs no object`},
		{a: xy, b: xy, exact: true, change: func(b *ast.File) { b.Unresolved = nil },
			want: "File.Unresolved: 1 vs 0 elements, first extra one at 4:13"},
		{a: xy, b: xy, change: func(b *ast.File) { b.Unresolved = nil }},
	} {
		// The stock go/parser, so that a broken fork can't make this test
		// fail: it checks only the comparer.
		var fsets [2]*token.FileSet
		var files [2]*ast.File
		for i, src := range []string{tt.a, tt.b} {
			fsets[i] = token.NewFileSet()
			f, err := parser.ParseFile(fsets[i], "x.go", src, parser.ParseComments)
			if err != nil {
				t.Fatal(err)
			}
			files[i] = f
		}
		if tt.change != nil {
			tt.change(files[1])
		}
		c := comparer{exact: tt.exact, fsets: fsets}
		got := ""
		if d := c.files(files[0], files[1]); d != nil {
			got = d.String()
		}
		if got != tt.want {
			t.Errorf("compare %q and %q (exact %v):\ngot  %s\nwant %s", tt.a, tt.b, tt.exact, got, tt.want)
		}
	}
}
