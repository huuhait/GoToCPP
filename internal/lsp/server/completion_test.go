package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// The tests of completion (completion.go, completer.go, autoimport.go)
// talk to a server over pipes, with module trees on disk, like the tests
// of the navigation features. A source with a | has the cursor there.

// compFiles is a module with the package app (two files and a header),
// geom, which app imports, and shapes, which it doesn't.
var compFiles = map[string]string{
	"gocpp.mod": "module example.com/app\n",
	"app/main.gp": `package main

import (
	"fmt"

	"example.com/app/geom"
	"./engine.hpp"
)

// Answer is the answer.
const Answer = 42

// Count counts the calls.
var Count int

type Point struct {
	X, Y  int
	label string
}

type Named struct {
	Point
	Name string
}

// Dist returns the distance.
func (p Point) Dist(q Point) int { return 0 }

// Move moves the point.
func (p *Point) Move(dx int) {}

type Shape interface {
	Area() int
	Perimeter() int
}

// Join joins the parts with sep.
func Join(sep string, parts ...string) string { return "" }

func Pick[T any]() T {
	var t T
	return t
}

func Same[T any](x T) T { return x }

func origin() Point { return Point{} }

func main() {
	count := 1
	p := Point{X: 1}
	var sh Shape
	_, _, _ = count, p, sh
	fmt.Println(geom.Area(geom.Rect{}))
}
`,
	"app/util.gp": `package main

func helper(n int) string { return "" }
`,
	"app/engine.hpp": "struct Engine {};\n",
	"geom/geom.gp": `// Package geom does geometry.
package geom

// Rect is a rectangle.
type Rect struct {
	W, H int
	area int
}

// Area returns the area of r.
func Area(r Rect) int { return r.W * r.H }

func scale(r Rect) Rect { return r }
`,
	"shapes/shapes.gp": `// Package shapes has shapes.
package shapes

// Circle is a circle.
type Circle struct{ R int }

// NewCircle returns a circle.
func NewCircle(r int) Circle { return Circle{r} }
`,
	"cmd/tool/main.gp": "package main\n\nfunc main() {}\n",
}

// Capabilities of the clients of the completion tests.
const (
	snippetInit   = `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"completion":{"completionItem":{"snippetSupport":true}}}}}`
	plainCompInit = `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"completion":{"completionItem":{"documentationFormat":["plaintext"]}}}}}`
)

// A compClient is a client of a server for the completion tests.
type compClient struct {
	*dclient
	versions map[string]int
}

// startComp writes the files into a new folder and starts a server with
// the test pipeline, initialized with init; requests wait for the index of
// the packages to import as long as it takes.
func startComp(t *testing.T, files map[string]string, init string) *compClient {
	t.Helper()
	root := writeTree(t, files)
	s := newServer(nil, testConfig(), debounce.RealClock{})
	c := &compClient{dclient: &dclient{client: startServer(t, s, func(s *Server) { s.comp.index.wait = time.Minute }), root: root},
		versions: make(map[string]int)}
	c.call("initialize", init)
	c.notify("initialized", "{}")
	return c
}

// set gives the document name the text, with | at the cursor (opened or
// changed), and returns the text without the | and the params of a request
// at the cursor.
func (c *compClient) set(name, text string) (string, string) {
	c.t.Helper()
	i := strings.Index(text, "|")
	if i < 0 {
		c.t.Fatalf("no | in %q", text)
	}
	src := text[:i] + text[i+1:]
	if v := c.versions[name]; v == 0 {
		c.open(name, src)
	} else {
		c.change(name, v+1, src)
	}
	c.versions[name]++
	p := protocol.NewMapper(src).Position(i)
	return src, fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, c.uri(name), p.Line, p.Character)
}

// complete returns the items of a completion at the | of text in the
// document name.
func (c *compClient) complete(name, text string) []protocol.CompletionItem {
	c.t.Helper()
	_, params := c.set(name, text)
	resp := c.call("textDocument/completion", params)
	if resp.Error != nil {
		c.t.Fatalf("completion at %q: %v", text, resp.Error)
	}
	var list protocol.CompletionList
	if err := json.Unmarshal(resp.Result, &list); err != nil {
		c.t.Fatalf("completion: %v: %s", err, resp.Result)
	}
	if !list.IsIncomplete {
		c.t.Errorf("the list is complete")
	}
	return list.Items
}

// labels returns the labels of the items, in order.
func labels(items []protocol.CompletionItem) []string {
	var l []string
	for _, it := range items {
		l = append(l, it.Label)
	}
	return l
}

// item returns the item with the label, or nil.
func item(items []protocol.CompletionItem, label string) *protocol.CompletionItem {
	for i := range items {
		if items[i].Label == label {
			return &items[i]
		}
	}
	return nil
}

// newText returns the text that the item inserts.
func newText(it *protocol.CompletionItem) string {
	switch e := it.TextEdit.Value.(type) {
	case protocol.TextEdit:
		return e.NewText
	case protocol.InsertReplaceEdit:
		return e.NewText
	}
	return ""
}

// inMain returns the main.gp of compFiles with the statements stmts at the
// end of main.
func inMain(stmts string) string {
	src := compFiles["app/main.gp"]
	return strings.Replace(src, "\tfmt.Println(geom.Area(geom.Rect{}))\n", "\tfmt.Println(geom.Area(geom.Rect{}))\n"+stmts, 1)
}

// A compCase is a completion and what it must give: the labels in want (in
// that order, among the others), none of the labels in not, and, if first
// is set, the first label.
type compCase struct {
	name  string
	text  string // the document app/main.gp, with the cursor
	want  []string
	not   []string
	first string
	// details are the details of some labels.
	details map[string]string
}

// run runs the cases as subtests of t.
func (c *compClient) run(t *testing.T, tests []compCase) {
	defer func() { c.t = t }()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c.t = t
			items := c.complete("app/main.gp", tt.text)
			got := labels(items)
			i := 0
			for _, l := range got {
				if i < len(tt.want) && l == tt.want[i] {
					i++
				}
			}
			if i < len(tt.want) {
				t.Errorf("want %q in this order, got %q", tt.want, got)
			}
			for _, l := range tt.not {
				if slices.Contains(got, l) {
					t.Errorf("%q is offered: %q", l, got)
				}
			}
			if tt.first != "" && (len(got) == 0 || got[0] != tt.first) {
				t.Errorf("first %q, got %q", tt.first, got)
			}
			for l, d := range tt.details {
				if it := item(items, l); it == nil || it.Detail != d {
					t.Errorf("the detail of %q: %+v, want %q", l, it, d)
				}
			}
		})
	}
}

// TestCompletionScope: the names in scope at the cursor, the innermost
// first and shadowing the outer ones, filtered by the prefix in any case.
func TestCompletionScope(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	c.run(t, []compCase{
		{name: "locals and package names", text: inMain("\tco|\n"),
			want: []string{"count", "const", "complex", "copy"}, not: []string{"Count2"},
			details: map[string]string{"count": "int", "Count": "int"}},
		{name: "case-insensitive", text: inMain("\tCOU|\n"), want: []string{"count", "Count"}},
		{name: "shadowing", text: inMain("\t{\n\t\tcount := \"inner\"\n\t\t_ = count\n\t\tcou|\n\t}\n"),
			details: map[string]string{"count": "string"}},
		{name: "declared after the cursor", text: inMain("\tlat|\n\tlater := 1\n\t_ = later\n"), not: []string{"later"}},
		{name: "params", text: strings.Replace(inMain(""), "func (p *Point) Move(dx int) {}", "func (p *Point) Move(dx int) {\n\td|\n}", 1),
			want: []string{"dx"}, details: map[string]string{"dx": "int"}},
		{name: "package level and other files", text: inMain("\the|\n"), want: []string{"helper"},
			details: map[string]string{"helper": "func(n int) string"}},
		{name: "imports", text: inMain("\tge|\n"), want: []string{"geom"}, details: map[string]string{"geom": `"example.com/app/geom"`}},
		{name: "header package", text: inMain("\teng|\n"), want: []string{"engine"}},
		{name: "builtins", text: inMain("\tx := le|\n"), want: []string{"len"}, details: map[string]string{"len": "func(v Type) int"}},
		{name: "types", text: inMain("\tvar x in|\n"), want: []string{"int", "int64"}},
		{name: "constants", text: inMain("\tx := tr|\n"), want: []string{"true"}},
		{name: "nil", text: inMain("\tvar e error = ni|\n"), want: []string{"nil"}},
		{name: "no iota outside const", text: inMain("\tx := io|\n"), not: []string{"iota"}},
		{name: "iota in const", text: inMain("\tconst k = io|\n"), want: []string{"iota"}},
		{name: "no C", text: inMain("\tx := C|\n"), not: []string{"C"}},
		{name: "a name being declared", text: inMain("\tvar cou| int\n"), not: []string{"count", "Count"}},
	})
}

// TestCompletionSelector: the fields and methods after x., also the
// promoted ones, the methods of *T for an addressable value only, the
// methods of an interface, and the exported members of an imported
// package.
func TestCompletionSelector(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	c.run(t, []compCase{
		{name: "fields first, then methods", text: inMain("\tp.|\n"), want: []string{"label", "X", "Y", "Dist", "Move"},
			details: map[string]string{"X": "int", "Dist": "func(q Point) int", "Move": "func(dx int)"}},
		{name: "promoted", text: inMain("\tn := Named{}\n\tn.|\n"), want: []string{"Name", "Point", "X", "Y", "Dist", "Move"}},
		{name: "prefix", text: inMain("\tp.Di|\n"), want: []string{"Dist"}, not: []string{"X", "Move"}},
		{name: "not addressable", text: inMain("\torigin().|\n"), want: []string{"X", "Dist"}, not: []string{"Move"}},
		{name: "pointer", text: inMain("\tpp := &p\n\tpp.|\n"), want: []string{"X", "Dist", "Move"}},
		{name: "interface", text: inMain("\tsh.|\n"), want: []string{"Area", "Perimeter"}},
		{name: "package members", text: inMain("\tgeom.|\n"), want: []string{"Area", "Rect"}, not: []string{"scale"},
			details: map[string]string{"Area": "func(r geom.Rect) int", "Rect": "struct{...}"}},
		{name: "unexported fields of another package", text: inMain("\tr := geom.Rect{}\n\tr.|\n"), want: []string{"H", "W"}, not: []string{"area"}},
		{name: "standard package", text: inMain("\tfmt.Spr|\n"), want: []string{"Sprint", "Sprintf", "Sprintln"}},
		{name: "in the middle of a name", text: inMain("\tp.Di|st(p)\n"), want: []string{"Dist"}},
		{name: "a header package", text: inMain("\tengine.|\n"), not: []string{"Engine"}},
		{name: "C", text: inMain("\tC.|\n"), not: []string{"int"}},
	})
}

// TestCompletionRanking: what fits the expected type first: the right side
// of an assignment, a variable's value, an argument, a result, a field of a
// composite literal; a function whose result fits is called.
func TestCompletionRanking(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	c.run(t, []compCase{
		{name: "assignment", text: inMain("\tvar s string\n\ts = |\n"), first: "s"},
		{name: "variable", text: inMain("\tvar q Point = |\n"), first: "p"},
		{name: "a call that fits", text: inMain("\tvar q Point = or|\n"), first: "origin"},
		{name: "argument", text: inMain("\tvar name string\n\tJoin(|)\n"), first: "name"},
		{name: "variadic argument", text: inMain("\tvar name string\n\tJoin(\",\", |)\n"), first: "name"},
		{name: "struct field value", text: inMain("\tvar w int\n\t_ = geom.Rect{W: |}\n"), want: []string{"count", "w"}},
		{name: "struct field names", text: inMain("\t_ = geom.Rect{|}\n"), want: []string{"W", "H"}, not: []string{"area"}},
		{name: "struct field names, one set", text: inMain("\t_ = geom.Rect{W: 1, |}\n"), want: []string{"H"}, not: []string{"W", "count"}},
		{name: "return", text: strings.Replace(inMain(""), "func origin() Point { return Point{} }", "func origin() Point {\n\tvar q Point\n\treturn |\n}", 1), first: "q"},
		{name: "comparison", text: inMain("\tif count == | {\n\t}\n"), first: "count"},
		// The members after x. that fit come first: helper wants an int.
		{name: "selector argument", text: inMain("\thelper(p.|)\n"), want: []string{"X", "Y", "Dist", "label", "Move"}},
		// An untyped constant that converts comes after a value of the
		// type itself (gopls's 0.75).
		{name: "untyped constant", text: inMain("\tvar m int = |\n\t_ = m\n"), want: []string{"count", "Count", "Answer"}},
	})
}

// TestCompletionKeywords: the statement keywords where a statement starts,
// the declaration keywords at the top level, none in an expression.
func TestCompletionKeywords(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	c.run(t, []compCase{
		{name: "statement", text: inMain("\tre|\n"), want: []string{"return"}},
		{name: "blank statement", text: inMain("\t|\n"), want: []string{"defer", "for", "go", "if", "return", "select", "switch", "var"}},
		{name: "loop", text: inMain("\tfor {\n\t\tbr|\n\t}\n"), want: []string{"break"}},
		{name: "no break outside a loop", text: inMain("\tbr|\n"), not: []string{"break"}},
		{name: "switch", text: inMain("\tswitch count {\n\tcase 1:\n\t\tfa|\n\t}\n"), want: []string{"fallthrough"}},
		{name: "expression", text: inMain("\tx := re|\n"), not: []string{"return"}},
		{name: "top level", text: inMain("") + "\nf|\n", want: []string{"func"}, not: []string{"for", "fmt", "false"}},
		{name: "range", text: inMain("\tfor i := r|\n"), want: []string{"range"}},
		{name: "top level, blank", text: inMain("") + "\n|\n", want: []string{"const", "func", "import", "type", "var"}},
	})
	// A new file: package, also alone in a new folder.
	for _, text := range []string{"|", "pa|", "// A new file.\n|\n"} {
		for _, name := range []string{"app/new.gp", "newdir/new.gp"} {
			if got := labels(c.complete(name, text)); !slices.Equal(got, []string{"package"}) {
				t.Errorf("in the new file %s %q: %q", name, text, got)
			}
		}
	}
	// return with results: a space after it, unless a result has a name
	// (but _).
	for sig, want := range map[string]string{"Point": "return ", "(_ Point)": "return ", "(q Point)": "return", "()": "return"} {
		items := c.complete("app/main.gp", strings.Replace(inMain(""), "func origin() Point { return Point{} }", "func origin() "+sig+" {\n\tret|\n}", 1))
		if it := item(items, "return"); it == nil || newText(it) != want {
			t.Errorf("return in func origin() %s: %+v, want %q", sig, it, want)
		}
	}
}

// TestCompletionSnippets: with snippets, a function comes with its call
// (no placeholders, like gopls), and a field of a struct literal with its
// colon; without, only the name. The text edit replaces the whole name.
func TestCompletionSnippets(t *testing.T) {
	for _, snippets := range []bool{true, false} {
		t.Run(fmt.Sprint("snippets=", snippets), func(t *testing.T) {
			init := plainCompInit
			if snippets {
				init = snippetInit
			}
			c := startComp(t, compFiles, init)
			for _, tt := range []struct {
				text, label, snippet, plain string
			}{
				{inMain("\tJo|\n"), "Join", "Join(${1:})", "Join"},
				{inMain("\tor|\n"), "origin", "origin()", "origin"},
				{inMain("\tp.Mo|\n"), "Move", "Move(${1:})", "Move"},
				{inMain("\tx := le|\n"), "len", "len(${1:})", "len"},
				{inMain("\tvar x int = Pi|\n"), "Pick", "Pick[${1:}]()", "Pick"},
				{inMain("\tvar x int = Sa|\n"), "Same", "Same(${1:})", "Same"},
				{inMain("\tJo|(\",\")\n"), "Join", "Join", "Join"},
				{inMain("\t_ = geom.Rect{|}\n"), "W", "W: ${1:}", "W"},
				{inMain("\t_ = geom.Rect{\n\t\t|\n\t}\n"), "W", "W: ${1:},", "W"},
				{inMain("\tvar f func() = or|\n"), "origin", "origin()", "origin"},
				{inMain("\tCou|\n"), "Count", "Count", "Count"},
				// any wants anything: a function is still called.
				{inMain("\tfmt.Println(Jo|)\n"), "Join", "Join(${1:})", "Join"},
				{inMain("\tvar a any = he|\n\t_ = a\n"), "helper", "helper(${1:})", "helper"},
			} {
				items := c.complete("app/main.gp", tt.text)
				it := item(items, tt.label)
				if it == nil {
					t.Errorf("at %q: no %s in %q", tt.text, tt.label, labels(items))
					continue
				}
				want := tt.plain
				format := protocol.PlainTextTextFormat
				if snippets {
					want, format = tt.snippet, protocol.SnippetTextFormat
				}
				if got := newText(it); got != want || it.InsertTextFormat == nil || *it.InsertTextFormat != format {
					t.Errorf("at %q: %s inserts %q (format %v), want %q", tt.text, tt.label, got, it.InsertTextFormat, want)
				}
			}
			// The edit replaces the name around the cursor.
			src, _ := c.set("app/main.gp", inMain("\tp.Di|st(p)\n"))
			items := c.complete("app/main.gp", inMain("\tp.Di|st(p)\n"))
			e, ok := item(items, "Dist").TextEdit.Value.(protocol.TextEdit)
			start, end, err := protocol.NewMapper(src).RangeOffsets(e.Range)
			if !ok || err != nil || src[start:end] != "Dist" {
				t.Errorf("the edit of Dist: %+v (%v)", e, err)
			}
		})
	}
	// A client with insertReplaceSupport gets both ranges.
	c := startComp(t, compFiles, `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"completion":{"completionItem":{"insertReplaceSupport":true}}}}}`)
	src, _ := c.set("app/main.gp", inMain("\tp.Di|st(p)\n"))
	items := c.complete("app/main.gp", inMain("\tp.Di|st(p)\n"))
	e, ok := item(items, "Dist").TextEdit.Value.(protocol.InsertReplaceEdit)
	m := protocol.NewMapper(src)
	is, ie, _ := m.RangeOffsets(e.Insert)
	rs, re, _ := m.RangeOffsets(e.Replace)
	if !ok || src[rs:re] != "Dist" || src[is:ie] != "Dist" {
		// "st" is in "Dist": the insert range replaces it too (gopls).
		t.Errorf("insert/replace edit: %+v", e)
	}
	items = c.complete("app/main.gp", inMain("\tp.X|Dist\n"))
	e, _ = item(items, "X").TextEdit.Value.(protocol.InsertReplaceEdit)
	is, ie, _ = m.RangeOffsets(e.Insert)
	if src2 := strings.Replace(src, "\tp.Dist(p)\n", "\tp.XDist\n", 1); src2[is:ie] != "X" {
		t.Errorf("the insert range of X in XDist: %q", src2[is:ie])
	}
}

// importEditOf returns the document after the additional edits of the item.
func importEditOf(t *testing.T, src string, it *protocol.CompletionItem) string {
	t.Helper()
	if it == nil {
		t.Fatal("no item")
	}
	m := protocol.NewMapper(src)
	out := src
	for _, e := range slices.Backward(it.AdditionalTextEdits) {
		start, end, err := m.RangeOffsets(e.Range)
		if err != nil {
			t.Fatal(err)
		}
		out = out[:start] + e.NewText + out[end:]
	}
	return out
}

// TestCompletionAutoImport: the members of a package that the file doesn't
// import, of the standard library and of the module, with the edit that
// adds the import: in its group, before the header imports; an import that
// is there already is never added again.
func TestCompletionAutoImport(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	for _, tt := range []struct {
		name, text, label, imports string
	}{
		{"standard", inMain("\tstrings.Jo|\n"), "Join",
			"import (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"example.com/app/geom\"\n\n\t\"./engine.hpp\"\n)\n"},
		{"standard, sorted first", inMain("\tbytes.Equ|\n"), "Equal",
			"import (\n\t\"bytes\"\n\t\"fmt\"\n\n\t\"example.com/app/geom\"\n\n\t\"./engine.hpp\"\n)\n"},
		{"module", inMain("\tshapes.|\n"), "NewCircle",
			"import (\n\t\"fmt\"\n\n\t\"example.com/app/geom\"\n\t\"example.com/app/shapes\"\n\n\t\"./engine.hpp\"\n)\n"},
		{"package name", inMain("\tshap|\n"), "shapes",
			"import (\n\t\"fmt\"\n\n\t\"example.com/app/geom\"\n\t\"example.com/app/shapes\"\n\n\t\"./engine.hpp\"\n)\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c.t = t
			src, _ := c.set("app/main.gp", tt.text)
			items := c.complete("app/main.gp", tt.text)
			it := item(items, tt.label)
			got := importEditOf(t, src, it)
			if !strings.Contains(got, tt.imports) {
				t.Errorf("after the edits:\n%s\nwant the imports\n%s", got[:strings.Index(got, "// Answer")], tt.imports)
			}
			if want := tt.label != "shapes"; strings.Contains(it.Detail, "(from \"") != want {
				t.Errorf("the detail: %q", it.Detail)
			}
		})
	}
	c.t = t
	// The documentation of a member of a package to import.
	items := c.complete("app/main.gp", inMain("\tshapes.NewCi|\n"))
	if doc := c.resolve(item(items, "NewCircle")); doc != "markdown:NewCircle returns a circle." {
		t.Errorf("the doc of shapes.NewCircle: %q", doc)
	}
	// Never main packages, never the package itself, nor an import of the
	// file under another name.
	if items := c.complete("app/main.gp", inMain("\ttool.|\n")); len(items) != 0 {
		t.Errorf("a main package: %q", labels(items))
	}
	if items := c.complete("app/main.gp", strings.Replace(inMain("\tstrings.|\n"), "\"fmt\"\n", "\"fmt\"\n\tstr \"strings\"\n", 1)); len(items) != 0 {
		t.Errorf("strings imported as str: %q", labels(items))
	}
	// Other shapes of files: only headers, no imports, single import
	// declarations; a CRLF file, a block comment on the package line,
	// imports on one line, a comment above the first import. The result
	// is what Organize Imports (step 11.7) prints.
	for _, tt := range []struct{ text, want string }{
		{"package main\n\nimport \"./engine.hpp\"\n\nfunc f() {\n\tstrings.|\n}\n",
			"package main\n\nimport (\n\t\"strings\"\n\n\t\"./engine.hpp\"\n)\n\nfunc f() {"},
		{"package main\n\nfunc f() {\n\tstrings.|\n}\n", "package main\n\nimport \"strings\"\n\nfunc f() {"},
		{"package main\n\nimport \"fmt\" // the format\n\nfunc f() {\n\tfmt.Println()\n\tstrings.|\n}\n",
			"package main\n\nimport (\n\t\"fmt\" // the format\n\t\"strings\"\n)\n\nfunc f() {"},
		{"package main\n\nimport \"strings\"\n\nfunc f() {\n\tshapes.|\n}\n",
			"package main\n\nimport (\n\t\"strings\"\n\n\t\"example.com/app/shapes\"\n)\n\nfunc f() {"},
		{"package main\r\n\r\nimport \"fmt\"\r\n\r\nfunc f() {\r\n\tfmt.Println()\r\n\tstrings.|\r\n}\r\n",
			"package main\r\n\r\nimport (\r\n\t\"fmt\"\r\n\t\"strings\"\r\n)\r\n\r\nfunc f() {"},
		{"package main /* a\nb */\n\nfunc f() {\n\tstrings.|\n}\n", "package main /* a\nb */\n\nimport \"strings\"\n\nfunc f() {"},
		{"package main\n\nimport (\"fmt\"; \"os\")\n\nfunc f() {\n\tfmt.Println(os.Args)\n\tstrings.|\n}\n",
			"package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"strings\"\n)\n\nfunc f() {"},
		{"package main\n\nimport (\n\t// fmt formats.\n\t\"fmt\"\n)\n\nfunc f() {\n\tfmt.Println()\n\tbytes.|\n}\n",
			"package main\n\nimport (\n\t// fmt formats.\n\t\"bytes\"\n\t\"fmt\"\n)\n\nfunc f() {"},
		// Code on the line of an import: no edit.
		{"package main; import \"fmt\"\n\nfunc f() {\n\tfmt.Println()\n\tstrings.|\n}\n", "package main; import \"fmt\"\n\nfunc f() {"},
	} {
		src, _ := c.set("app/util.gp", tt.text)
		items := c.complete("app/util.gp", tt.text)
		label := "Join"
		if strings.Contains(tt.text, "shapes.") {
			label = "NewCircle"
		}
		if got := importEditOf(t, src, item(items, label)); !strings.HasPrefix(got, tt.want) {
			t.Errorf("in\n%s\nafter the edits:\n%s\nwant the start\n%s", tt.text, got, tt.want)
		}
	}
}

// resolve returns the documentation of the item from completionItem/resolve.
func (c *compClient) resolve(it *protocol.CompletionItem) string {
	c.t.Helper()
	if it == nil {
		c.t.Fatal("no item")
		return ""
	}
	data, err := json.Marshal(it)
	if err != nil {
		c.t.Fatal(err)
	}
	resp := c.call("completionItem/resolve", string(data))
	if resp.Error != nil {
		c.t.Fatalf("resolve: %v", resp.Error)
	}
	var got protocol.CompletionItem
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		c.t.Fatal(err)
	}
	if got.Label != it.Label || newText(&got) != newText(it) {
		c.t.Errorf("resolve changed the item: %s", resp.Result)
	}
	if got.Documentation == nil {
		return ""
	}
	switch v := got.Documentation.Value.(type) {
	case protocol.MarkupContent:
		return string(v.Kind) + ":" + v.Value
	case string:
		return v
	}
	return ""
}

// TestCompletionDocumentation: completionItem/resolve gives the doc comment,
// in Markdown by default and in plain text when the client wants it.
func TestCompletionDocumentation(t *testing.T) {
	for _, tt := range []struct {
		init, kind string
	}{
		{snippetInit, "markdown"},
		{plainCompInit, "plaintext"},
		{`{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"hover":{"contentFormat":["plaintext"]}}}}`, "plaintext"},
	} {
		c := startComp(t, compFiles, tt.init)
		for _, d := range []struct{ text, label, doc string }{
			{inMain("\tJo|\n"), "Join", "Join joins the parts with sep."},
			{inMain("\tAns|\n"), "Answer", "Answer is the answer."},
			{inMain("\tp.Mo|\n"), "Move", "Move moves the point."},
			{inMain("\tgeom.Ar|\n"), "Area", "Area returns the area of r."},
			{inMain("\tfmt.Sprin|\n"), "Sprint", "Sprint formats using the default formats for its operands and returns the resulting string.\nSpaces are added between operands when neither is a string."},
			{inMain("\tgeo|\n"), "geom", "Package geom does geometry."},
			{inMain("\tapp|\n"), "append", builtinDoc(t, "append")},
			{inMain("\tcou|\n"), "count", ""},
			{inMain("\tre|\n"), "return", ""},
		} {
			items := c.complete("app/main.gp", d.text)
			want := d.doc
			if tt.kind == "markdown" {
				want = strings.TrimSuffix(docMarkdown(d.doc), "\n")
			}
			if want != "" {
				want = tt.kind + ":" + want
			}
			if got := c.resolve(item(items, d.label)); got != want {
				t.Errorf("%s: the doc of %s:\n%q\nwant\n%q", tt.kind, d.label, got, want)
			}
		}
	}
}

// TestCompletionBroken: in a file that doesn't parse, the completion still
// works: x. at the end of a line (before a statement, a } or a declaration),
// an incomplete statement, and a broken function before the cursor.
func TestCompletionBroken(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	c.run(t, []compCase{
		{name: "x. before }", text: inMain("\tp.|\n"), want: []string{"X", "Dist"}},
		{name: "x. before a statement", text: inMain("\tp.|\n\tfmt.Println(count)\n"), want: []string{"X", "Dist"}},
		{name: "x. before a keyword", text: inMain("\tp.|\n\tvar later int\n\t_ = later\n"), want: []string{"X", "Dist"}},
		{name: "x. at the end of the file", text: strings.TrimSuffix(inMain(""), "}\n") + "\tp.|", want: []string{"X", "Dist"}},
		{name: "pkg. before a statement", text: inMain("\tgeom.|\n\tcount++\n"), want: []string{"Area", "Rect"}},
		{name: "x.Na before }", text: inMain("\tn := Named{}\n\tn.Na|\n"), want: []string{"Name"}, not: []string{"X"}},
		{name: "incomplete statement", text: inMain("\tx := \n\tcou|\n"), want: []string{"count"}},
		{name: "incomplete call", text: inMain("\tJoin(cou|\n"), want: []string{"count"}},
		{name: "broken function before", text: strings.Replace(inMain("\tcou|\n"), "func origin() Point { return Point{} }", "func origin() Point {\n\tx :=\n\treturn Point{}\n}", 1), want: []string{"count"}},
		{name: "chain", text: inMain("\tn := Named{}\n\tn.Point.|\n"), want: []string{"X", "Dist"}, not: []string{"Name"}},
		// x. before a line that the parser takes into the selector: the
		// text is fixed (x._) for completion.
		{name: "after x. before var", text: inMain("\tp.\n\tvar yyy = 1\n\t_ = yyy\n\tyy|\n"), want: []string{"yyy"}},
		{name: "after x. before if", text: inMain("\tp.\n\tif count > 0 {\n\t\tzzz := 2\n\t\t_ = zzz\n\t\tzz|\n\t}\n"), want: []string{"zzz"}},
		{name: "x. before var", text: inMain("\tp.|\n\tvar yyy = 1\n\t_ = yyy\n"), want: []string{"X", "Dist"}},
		{name: "x. before var, CRLF", text: strings.ReplaceAll(inMain("\tp.|\n\tvar yyy = 1\n\t_ = yyy\n\tyy\n"), "\n", "\r\n"), want: []string{"X", "Dist"}},
	})
	// Nothing in a comment or a literal.
	for _, text := range []string{inMain("\t// p.|\n"), inMain("\t_ = \"p.|\"\n"), inMain("\t_ = 1.|5\n"), inMain("\t/* co| */\n")} {
		if items := c.complete("app/main.gp", text); len(items) != 0 {
			c.t.Errorf("at %q: %q", text, labels(items))
		}
	}
}

// TestCompletionOpaqueImports: the C/C++ types of an imported package show
// as written in the details, like in hover (Part A of job 11.6).
func TestCompletionOpaqueImports(t *testing.T) {
	c := startComp(t, opaqueFiles, snippetInit)
	c.t = t
	items := c.complete("main.gp", strings.Replace(opaqueFiles["main.gp"], "\ta.Run(nil)\n", "\ta.Run(nil)\n\tlib.|\n", 1))
	for label, want := range map[string]string{
		"Open":    "func(w *SDL.SDL_Window) *SDL.SDL_Renderer",
		"Default": "*SDL.SDL_Window",
		"Handle":  "SDL.Uint32",
	} {
		if it := item(items, label); it == nil || it.Detail != want {
			t.Errorf("%s: %+v, want the detail %q", label, it, want)
		}
	}
}

// TestCompletionCancel: a completion whose context is done answers with
// its error (TestNavigationCancel has all the requests too).
func TestCompletionCancel(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	c.t = t
	_, params := c.set("app/main.gp", inMain("\tco|\n"))
	c.call("textDocument/completion", params)
	snap, release := c.s.session.Snapshot()
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, name := range []string{"textDocument/completion", "completionItem/resolve"} {
		if _, err := c.s.requests[name].run(ctx, env{snapshot: snap}, json.RawMessage(params)); !errors.Is(err, context.Canceled) {
			t.Errorf("%s: %v, want context.Canceled", name, err)
		}
	}
}

// TestCompletionIndex: the index of the packages to import is made in the
// background when a document opens, and again after a change on disk; a
// request that comes before it is done waits only a little.
func TestCompletionIndex(t *testing.T) {
	c := startComp(t, compFiles, snippetInit)
	c.t = t
	c.complete("app/main.gp", inMain("\tshapes.|\n"))
	// A new package on disk: found after the watched-files notification.
	writeFile(t, c.root, "newpkg/newpkg.gp", "package newpkg\n\nfunc Hello() {}\n")
	if items := c.complete("app/main.gp", inMain("\tnewpkg.|\n")); len(items) != 0 {
		t.Errorf("before the notification: %q", labels(items))
	}
	// A saved file (a change, no new file) leaves the index as it is.
	gen := func() uint64 {
		c.s.comp.index.mu.Lock()
		defer c.s.comp.index.mu.Unlock()
		return c.s.comp.index.gen
	}
	before := gen()
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, c.uri("geom/geom.gp")))
	c.complete("app/main.gp", inMain("\tnewpkg.|\n")) // after the notification
	if gen() != before {
		t.Error("a changed file made the index old")
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":1}]}`, c.uri("newpkg/newpkg.gp")))
	// The request after the change gets the old index at once, and starts
	// a new one; wait for it.
	deadline := time.Now().Add(10 * time.Second)
	for {
		items := c.complete("app/main.gp", inMain("\tnewpkg.|\n"))
		if item(items, "Hello") != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the new package is never offered")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// An unsaved file of a new folder counts too.
	c.open("draft/draft.gp", "package draft\n\nfunc Sketch() {}\n")
	if items := c.complete("app/main.gp", inMain("\tdraft.|\n")); item(items, "Sketch") == nil {
		t.Errorf("an unsaved package: %q", labels(items))
	}

	// A request that comes while the index of a module is being made
	// waits for it for importIndex.wait at most, then goes on without it.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ix := newImportIndex(ctx, c.s.pipeline)
	ix.wait = 0
	got := ix.module(&load.Module{Path: "example.com/app", Dir: c.root}, true)
	if got != nil {
		t.Logf("the walk ended within no time: %d packages", len(got))
	}
	<-ix.mods[c.root].done
	if got := ix.module(&load.Module{Path: "example.com/app", Dir: c.root}, true); len(got) != 3 {
		t.Errorf("the packages of the module (geom, newpkg, shapes): %+v", got)
	}
}

// TestCompletionSameName: two packages to import with the same name and
// the same member: two items, each resolving to its own doc.
func TestCompletionSameName(t *testing.T) {
	files := maps.Clone(compFiles)
	files["a/util/util.gp"] = "package util\n\n// Foo is a's.\nfunc Foo() {}\n"
	files["b/util/util.gp"] = "package util\n\n// Foo is b's.\nfunc Foo() {}\n"
	c := startComp(t, files, plainCompInit)
	c.t = t
	items := c.complete("app/main.gp", inMain("\tutil.Fo|\n"))
	var docs []string
	for _, it := range items {
		if it.Label == "Foo" {
			docs = append(docs, it.Detail+": "+c.resolve(&it))
		}
	}
	slices.Sort(docs)
	want := []string{`func() (from "example.com/app/a/util"): plaintext:Foo is a's.`, `func() (from "example.com/app/b/util"): plaintext:Foo is b's.`}
	if !slices.Equal(docs, want) {
		t.Errorf("the two Foo: %q, want %q", docs, want)
	}
}

// TestCompletionGoroutines: no goroutine of completion (the index's walks)
// outlives the server.
func TestCompletionGoroutines(t *testing.T) {
	checkGoroutines(t)
	c := startComp(t, compFiles, snippetInit)
	c.t = t
	if items := c.complete("app/main.gp", inMain("\tshapes.|\n")); item(items, "NewCircle") == nil {
		t.Errorf("shapes.: %q", labels(items))
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":1}]}`, c.uri("new/new.gp")))
	c.complete("app/main.gp", inMain("\tshapes.|\n")) // starts a walk
	wantResult(t, c.call("shutdown", ""), "null")
	c.notify("exit", "")
	if code := c.wait(); code != 0 {
		t.Errorf("exit code %d", code)
	}
}

// TestCompletionSweep completes at every byte of the example programs and
// of the test modules: no error and no panic, whatever the place. The
// requests run in the test, on one snapshot (no pipes, no JSON), with a
// resolve of one item in 50. With -short, at every 7th byte.
func TestCompletionSweep(t *testing.T) {
	step := 1
	if testing.Short() {
		step = 7
	}
	trees := map[string]map[string]string{"comp": compFiles, "nav": navFiles, "opaque": opaqueFiles}
	examples, err := filepath.Glob(filepath.Join("..", "..", "..", "examples", "*", ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range examples {
		if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
			continue
		}
		files := make(map[string]string)
		filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && (strings.HasSuffix(path, ".gp") || filepath.Base(path) == "gocpp.mod") {
				data, _ := os.ReadFile(path)
				rel, _ := filepath.Rel(dir, path)
				files[filepath.ToSlash(rel)] = string(data)
			}
			return nil
		})
		if _, ok := files["gocpp.mod"]; !ok {
			files["gocpp.mod"] = "module example.com/" + filepath.Base(dir) + "\n"
		}
		trees[filepath.Base(dir)] = files
	}
	for _, tree := range slices.Sorted(maps.Keys(trees)) {
		files := trees[tree]
		t.Run(tree, func(t *testing.T) {
			c := startComp(t, files, snippetInit)
			var names []string
			for _, name := range slices.Sorted(maps.Keys(files)) {
				if strings.HasSuffix(name, ".gp") {
					c.open(name, files[name])
					names = append(names, name)
				}
			}
			c.call("textDocument/hover", c.posParams(names[0], files[names[0]], "|"+files[names[0]][:1])) // after the opens
			snap, release := c.s.session.Snapshot()
			defer release()
			ctx, e := context.Background(), env{snapshot: snap}
			n, offered := 0, 0
			for _, name := range names {
				src := files[name]
				m := protocol.NewMapper(src)
				for off := 0; off <= len(src); off += step {
					p := m.Position(off)
					params := fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, c.uri(name), p.Line, p.Character)
					res, err := c.s.completion(ctx, e, json.RawMessage(params))
					if err != nil {
						t.Fatalf("%s at offset %d: %v", name, off, err)
					}
					list := res.(*protocol.CompletionList)
					if len(list.Items) > 0 {
						offered++
					}
					if len(list.Items) > 0 && n%50 == 0 {
						data, _ := json.Marshal(list.Items[0])
						if _, err := c.s.resolveCompletion(ctx, e, data); err != nil {
							t.Fatalf("resolve in %s at offset %d: %v", name, off, err)
						}
					}
					n++
				}
			}
			// (win32 is for Windows only: the files that the build
			// constraints leave out have no package.)
			if offered < n/10 && tree != "win32" {
				t.Errorf("candidates at %d of %d places only", offered, n)
			}
			t.Logf("candidates at %d of %d places", offered, n)
		})
	}
}

// TestFixDangling: the fixed text of x. at the end of a line keeps the
// length (the _ takes the place of a space or tab), and the check of the
// fixed text is for completion only: the diagnostics are those of the
// user's text, with its syntax error.
func TestFixDangling(t *testing.T) {
	for _, tt := range []struct{ src, want string }{
		{"package p\n\nfunc f() {\n\tp.\n\tvar y = 1\n}\n", "package p\n\nfunc f() {\n\tp._\nvar y = 1\n}\n"},
		{"package p\n\nfunc f() {\n\tp.\r\n\t\tif x {\r\n\t\t}\r\n}\n", "package p\n\nfunc f() {\n\tp._\r\n\tif x {\r\n\t\t}\r\n}\n"},
		{"package p\n\nfunc f() {\n\tp.\nvar y = 1\n}\n", ""},        // no room
		{"package p\n\nfunc f() {\n\tp.\n}\n", ""},                   // before }
		{"package p\n\nfunc f() {\n\tp.\n\tq()\n}\n", ""},            // no phantom: p.q
		{"package p\n\nfunc f() {\n\tp. // c\n\tvar y = 1\n}\n", ""}, // a comment
		{"package p\n\nfunc f() {\n\tp.x\n}\n", ""},                  // nothing wrong
	} {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "p.gp", tt.src, parser.ParseComments|parser.AllErrors)
		pf := &cache.ParsedFile{Path: "p.gp", Src: []byte(tt.src), Fset: fset, Tok: fset.File(f.FileStart), File: f, Err: err}
		got := string(fixDangling(pf))
		if got != tt.want || got != "" && len(got) != len(tt.src) {
			t.Errorf("fixDangling(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}

	c := startComp(t, compFiles, snippetInit)
	c.t = t
	text := inMain("\tp.\n\tvar yyy = 1\n\t_ = yyy\n\tyy|\n")
	if items := c.complete("app/main.gp", text); item(items, "yyy") == nil {
		t.Fatalf("no yyy: %q", labels(items))
	}
	if p := c.next("app/main.gp"); !strings.Contains(format(p), "expected selector or type assertion") {
		t.Errorf("the diagnostics of the user's text:\n%s", format(p))
	}
}
