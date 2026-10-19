package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The tests of references, highlights and rename (step 11.8) talk to a
// server over pipes, with module trees on disk, like the navigation tests.

// wsFiles is a module of three packages: lib (with an internal test file
// and an external one), util, and app, which imports both and has a test of
// its own.
var wsFiles = map[string]string{
	"gocpp.mod": "module example.com/ws\n",
	"lib/lib.gp": `package lib

// Shape has an area.
type Shape interface {
	Area() int
	Name() string
}

type Rect struct {
	W, H int
}

func (r Rect) Area() int    { return r.W * r.H }
func (r Rect) Name() string { return "rect" }

type Circle struct{ R int }

func (c *Circle) Area() int    { return 3 * c.R * c.R }
func (c *Circle) Name() string { return "circle" }

// New makes a rectangle.
func New(w, h int) Rect { return Rect{W: w, H: h} }

const Max = 10

var Default = New(1, 1)

type Base struct{ ID int }

func (b Base) Describe() string { return "base" }

type Derived struct {
	Base
	Label string
}

func total(shapes []Shape) int {
	n := 0
	for _, s := range shapes {
		n += s.Area()
	}
	return n
}

func Total(shapes ...Shape) int { return total(shapes) }

// A use that needs a Circle to be a Shape ties its methods to the interface's.
var _ Shape = (*Circle)(nil)
`,
	"lib/lib_test.gp": `package lib

func helperForTest() int { return total(nil) + Max }

func Helper2() int { return 2 }
`,
	"lib/ext_test.gp": `package lib_test

import "example.com/ws/lib"

func use() int { return lib.New(1, 2).W + lib.Max }
`,
	"util/util.gp": `package util

import "example.com/ws/lib"

// Double doubles the area of s.
func Double(s lib.Shape) int { return 2 * s.Area() }

type Holder struct {
	lib.Rect
	Tag string
}
`,
	"app/main.gp": `package main

import (
	"fmt"

	"example.com/ws/lib"
	"example.com/ws/util"
)

type Square struct{ Side int }

func (s Square) Area() int    { return s.Side * s.Side }
func (s Square) Name() string { return "square" }

func main() {
	r := lib.New(2, 3)
	var sh lib.Shape = r
	sq := Square{Side: 2}
	fmt.Println(r.Area(), sh.Name(), sq.Area(), lib.Max, lib.Default.W)
	fmt.Println(util.Double(sq), lib.Total(r, sq))
	d := lib.Derived{Base: lib.Base{ID: 1}, Label: "x"}
	fmt.Println(d.ID, d.Describe(), d.Base.ID)
	h := util.Holder{Rect: r}
	fmt.Println(h.W, h.Rect.H)
	var v any = r
	switch x := v.(type) {
	case lib.Rect:
		fmt.Println(x.W)
	case int:
		fmt.Println(x + 1)
	}
outer:
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if j == 2 {
				continue outer
			}
			if i == 2 {
				break outer
			}
		}
	}
	count := 0
	inc := func() { count++ }
	inc()
	fmt.Println(count)
}
`,
	"app/main_test.gp": `package main

import "example.com/ws/lib"

func testHelper() int { return lib.Max + Square{Side: 1}.Area() }
`,
}

// A refClient is a client of a workspace server.
type refClient struct {
	*dclient
	src map[string]string
}

func startRefs(t *testing.T, files map[string]string, init string) *refClient {
	t.Helper()
	c := startNav(t, testConfig(), files, init)
	c.skipShown = true
	return &refClient{dclient: c, src: files}
}

// locs formats locations as "file:line:col text", the file relative to the
// root, sorted by the server (by file and place).
func (c *dclient) locs(list []protocol.Location) []string {
	var out []string
	for _, l := range list {
		path := l.URI.Path()
		rel, err := filepath.Rel(c.root, path)
		if err != nil {
			rel = path
		}
		s := fmt.Sprintf("%s:%d:%d", filepath.ToSlash(rel), l.Range.Start.Line+1, l.Range.Start.Character+1)
		if text, err := c.textOf(path); err == nil {
			m := protocol.NewMapper(text)
			if start, end, err := m.RangeOffsets(l.Range); err == nil && end > start {
				s += " " + text[start:end]
			}
		}
		out = append(out, s)
	}
	return out
}

// references asks for the references at the | of mark in the file name.
func (c *refClient) references(name, mark string, decl bool) []string {
	c.t.Helper()
	p := at(c.t, c.text(name), mark)
	resp := c.call("textDocument/references", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"context":{"includeDeclaration":%v}}`, c.uri(name), p.Line, p.Character, decl))
	if resp.Error != nil {
		c.t.Fatalf("references %q: %v", mark, resp.Error)
	}
	var locs []protocol.Location
	if err := json.Unmarshal(resp.Result, &locs); err != nil || locs == nil {
		c.t.Fatalf("references %q: %s, want a list", mark, resp.Result)
	}
	return c.locs(locs)
}

// text returns the text of the file name: the open document's or the original.
func (c *refClient) text(name string) string {
	if o := c.overlay(c.uri(name)); o != nil {
		return o.Text()
	}
	if s, ok := c.src[name]; ok {
		return s
	}
	b, err := os.ReadFile(filepath.Join(c.root, filepath.FromSlash(name)))
	if err != nil {
		c.t.Fatal(err)
	}
	return string(b)
}

// rename asks for a rename at the | of mark in the file name and returns the
// text of every file that changes (by name relative to the root) or the error.
func (c *refClient) rename(name, mark, newName string) (map[string]string, *protocol.ResponseError) {
	c.t.Helper()
	p := at(c.t, c.text(name), mark)
	resp := c.call("textDocument/rename", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"newName":%q}`, c.uri(name), p.Line, p.Character, newName))
	if resp.Error != nil {
		return nil, resp.Error
	}
	var we protocol.WorkspaceEdit
	if err := json.Unmarshal(resp.Result, &we); err != nil {
		c.t.Fatalf("rename %q: %s: %v", mark, resp.Result, err)
	}
	edits := make(map[protocol.DocumentURI][]protocol.TextEdit)
	for uri, es := range we.Changes {
		edits[uri] = append(edits[uri], es...)
	}
	for _, dc := range we.DocumentChanges {
		te := dc.TextDocumentEdit
		if te == nil {
			c.t.Fatalf("rename %q: a document change that is no edit: %+v", mark, dc)
			continue
		}
		for _, e := range te.Edits {
			switch v := e.Value.(type) {
			case protocol.TextEdit:
				edits[te.TextDocument.URI] = append(edits[te.TextDocument.URI], v)
			case protocol.AnnotatedTextEdit:
				edits[te.TextDocument.URI] = append(edits[te.TextDocument.URI], v.TextEdit)
			default:
				c.t.Fatalf("rename %q: edit of type %T", mark, e.Value)
			}
		}
	}
	out := make(map[string]string)
	for uri, es := range edits {
		rel, err := filepath.Rel(c.root, uri.Path())
		if err != nil {
			c.t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		out[rel] = applyEdits(c.t, c.text(rel), es)
	}
	return out, nil
}

// diffLines returns the lines of after that are not the same line of before,
// as "file:line: text", for the files that changed.
func diffLines(t *testing.T, c *refClient, got map[string]string) []string {
	t.Helper()
	var out []string
	names := make([]string, 0, len(got))
	for n := range got {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		before := strings.Split(c.text(n), "\n")
		after := strings.Split(got[n], "\n")
		if len(before) != len(after) {
			t.Fatalf("%s: %d lines became %d", n, len(before), len(after))
		}
		for i := range after {
			if before[i] != after[i] {
				out = append(out, fmt.Sprintf("%s:%d: %s", n, i+1, strings.TrimSpace(after[i])))
			}
		}
	}
	return out
}

func sameList(t *testing.T, what string, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("%s:\n got %q\nwant %q", what, got, want)
	}
}

// A refCase is a references request at the | of mark in file.
type refCase struct {
	file, mark string
	decl       bool
	want       []string
}

func TestReferences(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	for _, tc := range []refCase{
		// A function, used in another package, in an external test and in its own file.
		{"lib/lib.gp", "func New|(w", true, []string{"app/main.gp:16:11 New", "lib/ext_test.gp:5:29 New", "lib/lib.gp:22:6 New", "lib/lib.gp:26:15 New"}},
		{"lib/lib.gp", "func New|(w", false, []string{"app/main.gp:16:11 New", "lib/ext_test.gp:5:29 New", "lib/lib.gp:26:15 New"}},
		// From a use in another package, and from the external test file.
		{"app/main.gp", "lib.Ne|w(2, 3)", true, []string{"app/main.gp:16:11 New", "lib/ext_test.gp:5:29 New", "lib/lib.gp:22:6 New", "lib/lib.gp:26:15 New"}},
		{"lib/ext_test.gp", "lib.Ma|x", true, []string{"app/main.gp:19:50 Max", "app/main_test.gp:5:36 Max", "lib/ext_test.gp:5:47 Max", "lib/lib.gp:24:7 Max", "lib/lib_test.gp:3:48 Max"}},
		// An unexported function, used in the package's own test file.
		{"lib/lib.gp", "func tot|al(", true, []string{"lib/lib.gp:37:6 total", "lib/lib.gp:45:42 total", "lib/lib_test.gp:3:35 total"}},
		{"lib/lib_test.gp", "total|(nil)", true, []string{"lib/lib.gp:37:6 total", "lib/lib.gp:45:42 total", "lib/lib_test.gp:3:35 total"}},
		{"lib/lib_test.gp", "func helperForTe|st", true, []string{"lib/lib_test.gp:3:6 helperForTest"}},
		// A method stands for the methods it implements and is implemented by:
		// the interface's, and the other packages' types'.
		{"lib/lib.gp", "(r Rect) Na|me()", true, []string{"app/main.gp:13:17 Name", "app/main.gp:19:27 Name", "lib/lib.gp:6:2 Name", "lib/lib.gp:14:15 Name", "lib/lib.gp:19:18 Name"}},
		{"app/main.gp", "sq.Ar|ea()", true, []string{"app/main.gp:12:17 Area", "app/main.gp:19:16 Area", "app/main.gp:19:38 Area", "app/main_test.gp:5:58 Area", "lib/lib.gp:5:2 Area", "lib/lib.gp:13:15 Area", "lib/lib.gp:18:18 Area", "lib/lib.gp:40:10 Area", "util/util.gp:6:45 Area"}},
		{"lib/lib.gp", "Area() int\n\tName|() string", true, []string{"app/main.gp:13:17 Name", "app/main.gp:19:27 Name", "lib/lib.gp:6:2 Name", "lib/lib.gp:14:15 Name", "lib/lib.gp:19:18 Name"}},
		// A type: its uses, as gopls has them. The embedded fields of that type
		// (Holder's Rect, h.Rect, the key of the literal) are other objects.
		{"lib/lib.gp", "type Rect| struct", true, []string{"app/main.gp:27:11 Rect", "lib/lib.gp:9:6 Rect", "lib/lib.gp:13:9 Rect", "lib/lib.gp:14:9 Rect", "lib/lib.gp:22:20 Rect", "lib/lib.gp:22:34 Rect", "util/util.gp:9:6 Rect"}},
		{"lib/lib.gp", "type Rect| struct", false, []string{"app/main.gp:27:11 Rect", "lib/lib.gp:13:9 Rect", "lib/lib.gp:14:9 Rect", "lib/lib.gp:22:20 Rect", "lib/lib.gp:22:34 Rect", "util/util.gp:9:6 Rect"}},
		// The embedded field: from its use (h.Rect, the key), and from its
		// declaration, which are the same, with the declaration.
		{"app/main.gp", "h.Re|ct.H", true, []string{"app/main.gp:23:19 Rect", "app/main.gp:24:21 Rect", "util/util.gp:9:6 Rect"}},
		{"app/main.gp", "Holder{Re|ct: r}", true, []string{"app/main.gp:23:19 Rect", "app/main.gp:24:21 Rect", "util/util.gp:9:6 Rect"}},
		{"util/util.gp", "lib.Re|ct\n", true, []string{"app/main.gp:23:19 Rect", "app/main.gp:24:21 Rect", "util/util.gp:9:6 Rect"}},
		{"util/util.gp", "lib.Re|ct\n", false, []string{"app/main.gp:23:19 Rect", "app/main.gp:24:21 Rect"}},
		// A field: in a literal, through a selector, promoted through an
		// embedded struct, in other packages and the external test.
		{"lib/lib.gp", "ID| int", true, []string{"app/main.gp:21:34 ID", "app/main.gp:22:16 ID", "app/main.gp:22:41 ID", "lib/lib.gp:28:19 ID"}},
		{"app/main.gp", "d.I|D, d.", true, []string{"app/main.gp:21:34 ID", "app/main.gp:22:16 ID", "app/main.gp:22:41 ID", "lib/lib.gp:28:19 ID"}},
		{"app/main.gp", "x.|W)", true, []string{"app/main.gp:19:67 W", "app/main.gp:24:16 W", "app/main.gp:28:17 W", "lib/ext_test.gp:5:39 W", "lib/lib.gp:10:2 W", "lib/lib.gp:13:40 W", "lib/lib.gp:22:39 W"}},
		{"lib/lib.gp", "Base) Descr|ibe", true, []string{"app/main.gp:22:22 Describe", "lib/lib.gp:30:15 Describe"}},
		// Locals: a variable captured by a closure, a label, a type switch name.
		{"app/main.gp", "count| := 0", true, []string{"app/main.gp:43:2 count", "app/main.gp:44:18 count", "app/main.gp:46:14 count"}},
		{"app/main.gp", "continue out|er", true, []string{"app/main.gp:32:1 outer", "app/main.gp:36:14 outer", "app/main.gp:39:11 outer"}},
		{"app/main.gp", "x| := v.(type)", true, []string{"app/main.gp:26:9 x", "app/main.gp:28:15 x", "app/main.gp:30:15 x"}},
		{"app/main.gp", "fmt.Println(x| + 1)", true, []string{"app/main.gp:26:9 x", "app/main.gp:28:15 x", "app/main.gp:30:15 x"}},
		{"app/main.gp", "Println(x.|W)", false, []string{"app/main.gp:19:67 W", "app/main.gp:24:16 W", "app/main.gp:28:17 W", "lib/ext_test.gp:5:39 W", "lib/lib.gp:13:40 W", "lib/lib.gp:22:39 W"}},
		// An import name: the import and its uses in the file.
		{"app/main.gp", "fm|t.Println(count)", true, []string{"app/main.gp:4:2 \"fmt\"", "app/main.gp:19:2 fmt", "app/main.gp:20:2 fmt", "app/main.gp:22:2 fmt", "app/main.gp:24:2 fmt", "app/main.gp:28:3 fmt", "app/main.gp:30:3 fmt", "app/main.gp:46:2 fmt"}},
		// A name of the standard library: its uses in the module.
		{"app/main.gp", "fmt.Print|ln(r", true, []string{"app/main.gp:19:6 Println", "app/main.gp:20:6 Println", "app/main.gp:22:6 Println", "app/main.gp:24:6 Println", "app/main.gp:28:7 Println", "app/main.gp:30:7 Println", "app/main.gp:46:6 Println"}},
		// A struct of the package main, whose field is used in its test file.
		{"app/main.gp", "Side|: 2", true, []string{"app/main.gp:10:21 Side", "app/main.gp:12:42 Side", "app/main.gp:12:51 Side", "app/main.gp:18:15 Side", "app/main_test.gp:5:49 Side"}},
	} {
		tc.mark = strings.ReplaceAll(tc.mark, "\\n", "\n")
		got := c.references(tc.file, tc.mark, tc.decl)
		sameList(t, fmt.Sprintf("references at %s %q (declaration %v)", tc.file, tc.mark, tc.decl), got, tc.want)
	}
}

// A renameCase is a rename at the | of mark in file: the lines that change
// ("file:line: new text") or, with err, the error message.
type renameCase struct {
	file, mark, name string
	want             []string
	err              string // part of the error message
}

func (c *refClient) runRenames(t *testing.T, cases []renameCase) {
	t.Helper()
	for _, tc := range cases {
		tc.mark = strings.ReplaceAll(tc.mark, "\\n", "\n")
		what := fmt.Sprintf("rename at %s %q to %s", tc.file, tc.mark, tc.name)
		got, rerr := c.rename(tc.file, tc.mark, tc.name)
		switch {
		case tc.err != "":
			if rerr == nil {
				t.Errorf("%s: got edits %q, want the error %q", what, diffLines(t, c, got), tc.err)
			} else if !strings.Contains(rerr.Message, tc.err) {
				t.Errorf("%s: the error %q, want %q in it", what, rerr.Message, tc.err)
			}
		case rerr != nil:
			t.Errorf("%s: the error %q", what, rerr.Message)
		default:
			sameList(t, what, diffLines(t, c, got), tc.want)
		}
	}
}

func TestRename(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	c.runRenames(t, []renameCase{
		// A function: its callers in other packages, in tests too.
		{"lib/lib.gp", "func New|(w", "Make", []string{
			"app/main.gp:16: r := lib.Make(2, 3)",
			"lib/ext_test.gp:5: func use() int { return lib.Make(1, 2).W + lib.Max }",
			"lib/lib.gp:22: func Make(w, h int) Rect { return Rect{W: w, H: h} }",
			"lib/lib.gp:26: var Default = Make(1, 1)"}, ""},
		// From a use in another package; the same edits.
		{"app/main.gp", "lib.Ne|w(2, 3)", "Make", []string{
			"app/main.gp:16: r := lib.Make(2, 3)",
			"lib/ext_test.gp:5: func use() int { return lib.Make(1, 2).W + lib.Max }",
			"lib/lib.gp:22: func Make(w, h int) Rect { return Rect{W: w, H: h} }",
			"lib/lib.gp:26: var Default = Make(1, 1)"}, ""},
		// A constant used in the tests of three packages.
		{"lib/lib.gp", "Max| = 10", "Limit", []string{
			"app/main.gp:19: fmt.Println(r.Area(), sh.Name(), sq.Area(), lib.Limit, lib.Default.W)",
			"app/main_test.gp:5: func testHelper() int { return lib.Limit + Square{Side: 1}.Area() }",
			"lib/ext_test.gp:5: func use() int { return lib.New(1, 2).W + lib.Limit }",
			"lib/lib.gp:24: const Limit = 10",
			"lib/lib_test.gp:3: func helperForTest() int { return total(nil) + Limit }"}, ""},
		// An unexported function, with its use in the package's test file.
		{"lib/lib.gp", "func tot|al(", "sum", []string{
			"lib/lib.gp:37: func sum(shapes []Shape) int {",
			"lib/lib.gp:45: func Total(shapes ...Shape) int { return sum(shapes) }",
			"lib/lib_test.gp:3: func helperForTest() int { return sum(nil) + Max }"}, ""},
		// From the test file.
		{"lib/lib_test.gp", "total|(nil)", "sum", []string{
			"lib/lib.gp:37: func sum(shapes []Shape) int {",
			"lib/lib.gp:45: func Total(shapes ...Shape) int { return sum(shapes) }",
			"lib/lib_test.gp:3: func helperForTest() int { return sum(nil) + Max }"}, ""},
		// A method renames the interface's method and all its implementations,
		// also in other packages, also from their side.
		{"lib/lib.gp", "(r Rect) Ar|ea()", "Surface", []string{
			"app/main.gp:12: func (s Square) Surface() int    { return s.Side * s.Side }",
			"app/main.gp:19: fmt.Println(r.Surface(), sh.Name(), sq.Surface(), lib.Max, lib.Default.W)",
			"app/main_test.gp:5: func testHelper() int { return lib.Max + Square{Side: 1}.Surface() }",
			"lib/lib.gp:5: Surface() int",
			"lib/lib.gp:13: func (r Rect) Surface() int    { return r.W * r.H }",
			"lib/lib.gp:18: func (c *Circle) Surface() int    { return 3 * c.R * c.R }",
			"lib/lib.gp:40: n += s.Surface()",
			"util/util.gp:6: func Double(s lib.Shape) int { return 2 * s.Surface() }"}, ""},
		{"app/main.gp", "func (s Square) Ar|ea()", "Surface", []string{
			"app/main.gp:12: func (s Square) Surface() int    { return s.Side * s.Side }",
			"app/main.gp:19: fmt.Println(r.Surface(), sh.Name(), sq.Surface(), lib.Max, lib.Default.W)",
			"app/main_test.gp:5: func testHelper() int { return lib.Max + Square{Side: 1}.Surface() }",
			"lib/lib.gp:5: Surface() int",
			"lib/lib.gp:13: func (r Rect) Surface() int    { return r.W * r.H }",
			"lib/lib.gp:18: func (c *Circle) Surface() int    { return 3 * c.R * c.R }",
			"lib/lib.gp:40: n += s.Surface()",
			"util/util.gp:6: func Double(s lib.Shape) int { return 2 * s.Surface() }"}, ""},
		// A type: its uses, and the embedded fields it names (Holder's Rect,
		// h.Rect.H, the key of the literal).
		{"lib/lib.gp", "type Rect| struct", "Box", []string{
			"app/main.gp:23: h := util.Holder{Box: r}",
			"app/main.gp:24: fmt.Println(h.W, h.Box.H)",
			"app/main.gp:27: case lib.Box:",
			"lib/lib.gp:9: type Box struct {",
			"lib/lib.gp:13: func (r Box) Area() int    { return r.W * r.H }",
			"lib/lib.gp:14: func (r Box) Name() string { return \"rect\" }",
			"lib/lib.gp:22: func New(w, h int) Box { return Box{W: w, H: h} }",
			"util/util.gp:9: lib.Box"}, ""},
		// A field: in the literal, through the selector, in other packages.
		{"lib/lib.gp", "W, H| int", "Height", []string{
			"app/main.gp:24: fmt.Println(h.W, h.Rect.Height)",
			"lib/lib.gp:10: W, Height int",
			"lib/lib.gp:13: func (r Rect) Area() int    { return r.W * r.Height }",
			"lib/lib.gp:22: func New(w, h int) Rect { return Rect{W: w, Height: h} }"}, ""},
		// A promoted field and method.
		{"app/main.gp", "d.I|D, d.", "Ident", []string{
			"app/main.gp:21: d := lib.Derived{Base: lib.Base{Ident: 1}, Label: \"x\"}",
			"app/main.gp:22: fmt.Println(d.Ident, d.Describe(), d.Base.Ident)",
			"lib/lib.gp:28: type Base struct{ Ident int }"}, ""},
		{"app/main.gp", "d.Descr|ibe", "Explain", []string{
			"app/main.gp:22: fmt.Println(d.ID, d.Explain(), d.Base.ID)",
			"lib/lib.gp:30: func (b Base) Explain() string { return \"base\" }"}, ""},
		// Locals: a variable captured by a closure, a label, a type switch name.
		{"app/main.gp", "count| := 0", "n", []string{
			"app/main.gp:43: n := 0",
			"app/main.gp:44: inc := func() { n++ }",
			"app/main.gp:46: fmt.Println(n)"}, ""},
		{"app/main.gp", "out|er:", "again", []string{
			"app/main.gp:32: again:",
			"app/main.gp:36: continue again",
			"app/main.gp:39: break again"}, ""},
		{"app/main.gp", "x| := v.(type)", "val", []string{
			"app/main.gp:26: switch val := v.(type) {",
			"app/main.gp:28: fmt.Println(val.W)",
			"app/main.gp:30: fmt.Println(val + 1)"}, ""},
		{"app/main.gp", "fmt.Println(x| + 1)", "val", []string{
			"app/main.gp:26: switch val := v.(type) {",
			"app/main.gp:28: fmt.Println(val.W)",
			"app/main.gp:30: fmt.Println(val + 1)"}, ""},
		// An import without a name gets one.
		{"app/main.gp", "fm|t.Println(count)", "out", []string{
			"app/main.gp:4: out \"fmt\"",
			"app/main.gp:19: out.Println(r.Area(), sh.Name(), sq.Area(), lib.Max, lib.Default.W)",
			"app/main.gp:20: out.Println(util.Double(sq), lib.Total(r, sq))",
			"app/main.gp:22: out.Println(d.ID, d.Describe(), d.Base.ID)",
			"app/main.gp:24: out.Println(h.W, h.Rect.H)",
			"app/main.gp:28: out.Println(x.W)",
			"app/main.gp:30: out.Println(x + 1)",
			"app/main.gp:46: out.Println(count)"}, ""},
		// The same name: nothing to do.
		{"lib/lib.gp", "func New|(w", "New", nil, ""},
		// What is refused.
		{"lib/lib.gp", "func New|(w", "Max", nil, "Max is already declared in this scope"},
		{"lib/lib.gp", "func New|(w", "func", nil, "not a valid name"},
		{"lib/lib.gp", "func New|(w", "a b", nil, "not a valid name"},
		{"lib/lib.gp", "func New|(w", "_", nil, "cannot rename to _"},
		{"lib/lib.gp", "func New|(w", "init", nil, "reserved"},
		{"lib/lib.gp", "Max| = 10", "max", nil, "the new name is not exported"},
		{"lib/lib.gp", "W, H| int", "W", nil, "the struct already has a field W"},
		{"lib/lib.gp", "W|, H int", "Area", nil, "already has a field or method Area"},
		{"lib/lib.gp", "package l|ib", "x", nil, "renaming a package is not supported"},
		{"app/main.gp", "\"example.com/ws/li|b\"", "x", nil, "renaming a package is not supported"},
		{"app/main.gp", "fmt.Print|ln(r", "Print", nil, "declared in the standard library"},
		{"app/main.gp", "fm|t.Println(count)", "lib", nil, "lib is already declared in this file"},
		// A rename that no scope rule sees, since a type switch name has no
		// declaration of go/types' own: the result check does.
		{"app/main.gp", "x| := v.(type)", "fmt", nil, "it would make an error"},
		{"lib/lib.gp", "Name() str|ing", "x", nil, "string is predeclared"},
		{"app/main.gp", "var v a|ny", "x", nil, "any is predeclared"},
		{"app/main.gp", "\tr := lib.New(2, 3)\n|\tvar sh", "x", nil, "no name at this position"},
	})
}

// confFiles is a package for the conflicts of rename.
var confFiles = map[string]string{
	"gocpp.mod": "module example.com/conf\n",
	"conf.gp": `package conf

import (
	"fmt"
	"strings"
)

var global = 1
var other = 2

type T struct{ A, B int }

func (t T) Get() int   { return t.A }
func (t T) Set(v int)  { t.A = v }
func (t *T) Inc()      { t.A++ }

func f(p int) int {
	x := p
	y := 2
	{
		z := x + y + global
		_ = z
	}
	return x + other
}

func g() {
	fmt.Println(global)
	for i := 0; i < 3; i++ {
		fmt.Println(i, strings.ToUpper("a"))
	}
}

func h() {
first:
	for {
		break first
	}
second:
	for {
		break second
	}
}

type Doer interface{ Do() }

type Impl struct{}

func (Impl) Do()                {}
func (i Impl) String() string   { return "impl" }

var _ Doer = Impl{}

func generic[T any](x T) T {
	var U int
	_ = U
	return x
}

func shadow() int {
	count := 1
	{
		count := 2
		_ = count
	}
	return count
}
`,
	"other.gp": `package conf

func inOther() int { return global + other }
`,
}

func TestRenameConflicts(t *testing.T) {
	c := startRefs(t, confFiles, plainInit)
	c.runRenames(t, []renameCase{
		// A declaration of the new name in the same scope.
		{"conf.gp", "x| := p", "y", nil, "y is already declared in this scope"},
		{"conf.gp", "func f(p| int)", "x", nil, "x is already declared in this scope"},
		{"conf.gp", "func f(p| int)", "q", []string{
			"conf.gp:17: func f(q int) int {",
			"conf.gp:18: x := q"}, ""},
		// The new name is a package-level name, in this or another file; an
		// import name of any file.
		{"conf.gp", "global| = 1", "other", nil, "other is already declared in this scope"},
		{"conf.gp", "global| = 1", "fmt", nil, "fmt is imported in conf.gp"},
		{"conf.gp", "func g|()", "strings", nil, "strings is imported in conf.gp"},
		{"other.gp", "func inOth|er", "global", nil, "global is already declared in this scope"},
		// The new name is captured by a nearer declaration: i is declared in
		// the loop, between fmt.Println and the import.
		{"conf.gp", "fm|t.Println(global)", "i", nil, "would refer to another declaration of i"},
		{"conf.gp", "fmt.Println(i, strings.To|Upper", "i", nil, "cannot rename"},
		// The renamed name would hide a name that its scope uses: x would hide
		// global and other (used in the function).
		{"conf.gp", "x| := p", "global", nil, "would hide the declaration of global"},
		{"conf.gp", "x| := p", "other", nil, "would hide the declaration of other"},
		{"conf.gp", "x| := p", "fmt", []string{
			"conf.gp:18: fmt := p",
			"conf.gp:21: z := fmt + y + global",
			"conf.gp:24: return fmt + other"}, ""},
		// ... but not a declaration in an inner scope, which hides it.
		{"conf.gp", "count| := 1", "total", []string{
			"conf.gp:61: total := 1",
			"conf.gp:66: return total"}, ""},
		{"conf.gp", "count| := 2", "total", []string{
			"conf.gp:63: total := 2",
			"conf.gp:64: _ = total"}, ""},
		// Fields and methods of one type.
		{"conf.gp", "A|, B int", "B", nil, "the struct already has a field B"},
		{"conf.gp", "A|, B int", "Get", nil, "already has a field or method Get"},
		{"conf.gp", "Get|() int", "A", nil, "already has a field or method A"},
		{"conf.gp", "Get|() int", "Set", nil, "already has a field or method Set"},
		{"conf.gp", "Get|() int", "Inc", nil, "already has a field or method Inc"},
		{"conf.gp", "Get|() int", "Fetch", []string{"conf.gp:13: func (t T) Fetch() int   { return t.A }"}, ""},
		// Labels of one function.
		{"conf.gp", "first|:", "second", nil, "the function already has a label second"},
		{"conf.gp", "first|:", "start", []string{
			"conf.gp:35: start:",
			"conf.gp:37: break start"}, ""},
		// A type parameter and a local in its function.
		{"conf.gp", "func generic[T| any]", "U", nil, "U is already declared in this scope"},
		// An interface method renames its implementations; a method that
		// implements an interface of the standard library does not.
		{"conf.gp", "Do|() }", "Run", []string{
			"conf.gp:45: type Doer interface{ Run() }",
			"conf.gp:49: func (Impl) Run()                {}"}, ""},
		{"conf.gp", "String|() string", "Text", nil, "implements"},
		// The same name is a conflict only at the declaration or a use.
		{"conf.gp", "global| = 1", "Global", []string{
			"conf.gp:8: var Global = 1",
			"conf.gp:21: z := x + y + Global",
			"conf.gp:28: fmt.Println(Global)",
			"other.gp:3: func inOther() int { return Global + other }"}, ""},
	})
}

// TestRenameResultCheck: what the conflict checks would miss, the check of the
// result catches. With the conflict checks off, the same refusals come from the
// errors that the edit would make.
func TestRenameResultCheck(t *testing.T) {
	skipConflictChecks = true
	defer func() { skipConflictChecks = false }()
	c := startRefs(t, confFiles, plainInit)
	c.runRenames(t, []renameCase{
		{"conf.gp", "x| := p", "y", nil, "it would make an error"},
		{"conf.gp", "global| = 1", "other", nil, "it would make an error"},
		{"conf.gp", "A|, B int", "B", nil, "it would make an error"},
		{"conf.gp", "first|:", "second", nil, "it would make an error"},
		{"conf.gp", "fm|t.Println(global)", "i", nil, "it would make an error"},
		// A rename that is fine stays fine.
		{"conf.gp", "Get|() int", "Fetch", []string{"conf.gp:13: func (t T) Fetch() int   { return t.A }"}, ""},
	})
}

// TestReferencesInChangedDiskFile: the places of the references in a file
// that is not open come from its current text: a change on disk that moves
// the use is seen by the next search (the line tables of files on disk are
// kept by content, never by name alone).
func TestReferencesInChangedDiskFile(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":    "module example.com/disk\n",
		"lib/lib.gp":   "package lib\n\nfunc Count() int { return 1 }\n",
		"app/other.gp": "package app\n\nimport \"example.com/disk/lib\"\n\nfunc Use() int { return lib.Count() }\n",
	}
	c := startRefs(t, files, plainInit)
	c.open("lib/lib.gp", files["lib/lib.gp"])
	c.next("lib/lib.gp")
	want := func(line int) {
		t.Helper()
		got := c.references("lib/lib.gp", "func |Count", true)
		found := fmt.Sprintf("app/other.gp:%d:", line)
		for _, g := range got {
			if strings.HasPrefix(g, found) {
				return
			}
		}
		t.Errorf("references %v: want one at line %d of app/other.gp", got, line)
	}
	want(5)
	moved := "package app\n\nimport \"example.com/disk/lib\"\n\n// a comment\n// and another\n\nfunc Use() int { return lib.Count() }\n"
	if err := os.WriteFile(filepath.Join(c.root, "app", "other.gp"), []byte(moved), 0o644); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, c.uri("app/other.gp")))
	want(8)
}
