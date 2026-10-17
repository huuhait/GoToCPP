package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The tests of the navigation features (hover, definition, signature help)
// talk to a server over pipes, with module trees on disk, like the tests of
// the diagnostics.

// navFiles is a module with two packages: app (two files, and a local
// header) and geom, which app imports (from export data, as every import of
// the pipeline).
var navFiles = map[string]string{
	"gocpp.mod": "module example.com/app\n",
	"app/main.gp": `// Package main is the app.
package main

import (
	"fmt"
	"strings"

	"example.com/app/geom"
	"./engine.hpp"
)

// Answer is the answer.
const Answer = 6 * 7

const Name = "gocpp"

// Count counts the calls.
var Count int

// Point is a point in the plane.
type Point struct {
	X, Y  int // the coordinates
	Label string
}

// Named has a name.
type Named struct {
	Point
	Name string
}

// Dist returns the distance from p to q.
func (p Point) Dist(q Point) (int, error) {
	return p.X - q.X + p.Y - q.Y, nil
}

// Join joins the parts with sep.
func Join(sep string, parts ...string) string {
	return strings.Join(parts, sep)
}

var e *engine.Engine

func main() {
	p := Point{X: 1}
	n := Named{Point: p}
	fmt.Println(n.X, Join(",", "a", "b"), geom.Area(geom.Rect{W: 2, H: 3}), helper())
	s := []int{}
	s = append(s, len(s))
	var err error
	_ = err
	d, _ := p.Dist(Point{})
	Count += d
	engine.start(e)
	h := engine.make()
	h.run()
}

// Draw draws on w.
func Draw(w *engine.Engine, more ...engine.Engine) {}
`,
	"app/util.gp": `package main

// helper returns the answer.
func helper() int { return Answer }
`,
	"app/engine.hpp": "struct Engine {};\nvoid start(Engine*);\n",
	"geom/geom.gp": `// Package geom does geometry.
package geom

// Rect is a rectangle.
type Rect struct {
	W, H int
}

// Area returns the area of r.
func Area(r Rect) int { return r.W * r.H }

// Unit is the unit square.
var Unit = Rect{1, 1}
`,
}

// noHeaderFiles returns navFiles without the header import, for the tests
// that count checks: the background work for a header import (the C++
// compiler lookup) checks the package again when it ends.
func noHeaderFiles() map[string]string {
	files := maps.Clone(navFiles)
	src := files["app/main.gp"]
	src = src[:strings.Index(src, "\n// Draw draws")]
	for _, s := range []string{"\t\"./engine.hpp\"\n", "var e *engine.Engine\n", "\tengine.start(e)\n", "\th := engine.make()\n", "\th.run()\n"} {
		src = strings.Replace(src, s, "", 1)
	}
	files["app/main.gp"] = src
	return files
}

// The params of initialize of the navigation tests: a client that wants
// plain text, or Markdown.
const (
	plainInit    = `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"hover":{"contentFormat":["plaintext","markdown"]},"signatureHelp":{"signatureInformation":{"documentationFormat":["plaintext"]}}}}}`
	markdownInit = `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"hover":{"contentFormat":["markdown","plaintext"]}}}}`
)

// startNav writes the files into a new folder and starts a server with the
// test pipeline (cfg), initialized with init.
func startNav(t *testing.T, cfg cache.Config, files map[string]string, init string) *dclient {
	t.Helper()
	root := writeTree(t, files)
	c := &dclient{client: startServer(t, newServer(nil, cfg, debounce.RealClock{}), nil), root: root}
	c.call("initialize", init)
	c.notify("initialized", "{}")
	return c
}

// at returns the position of the | in mark, which (without the |) must be
// in src.
func at(t testing.TB, src, mark string) protocol.Position {
	t.Helper()
	i := strings.Index(mark, "|")
	text := mark[:i] + mark[i+1:]
	j := strings.Index(src, text)
	if j < 0 {
		t.Fatalf("%q is not in the file", text)
	}
	return protocol.NewMapper(src).Position(j + i)
}

// posParams returns the params of a request at the | of mark in the file
// name with the text src.
func (c *dclient) posParams(name, src, mark string) string {
	c.t.Helper()
	p := at(c.t, src, mark)
	return fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, c.uri(name), p.Line, p.Character)
}

// hover returns the hover text at the | of mark in the file name, and the
// text of its range; "" when there is none.
func (c *dclient) hover(name, src, mark string) (string, string) {
	c.t.Helper()
	resp := c.call("textDocument/hover", c.posParams(name, src, mark))
	if resp.Error != nil {
		c.t.Fatalf("hover %q: %v", mark, resp.Error)
	}
	if string(resp.Result) == "null" {
		return "", ""
	}
	var h protocol.Hover
	if err := json.Unmarshal(resp.Result, &h); err != nil {
		c.t.Fatal(err)
	}
	m := protocol.NewMapper(src)
	start, end, err := m.RangeOffsets(h.Range)
	if err != nil {
		c.t.Fatal(err)
	}
	return h.Contents.Value, src[start:end]
}

// A hoverCase is a hover at the | of mark in app/main.gp: its text and the
// text of its range.
type hoverCase struct {
	mark, text, rng string
}

// TestHover shows each kind of name in plain text. The texts are those of
// gopls for the same code as .go, with two differences: gopls adds a link to
// pkg.go.dev for the names it can link, and after the declaration of a
// struct type with embedded fields (Named) the "// Embedded fields:" table
// of the fields they promote, which gocpp doesn't print. The C/C++ names,
// header imports and C are gocpp's own.
func TestHover(t *testing.T) {
	c := startNav(t, testConfig(), navFiles, plainInit)
	src := navFiles["app/main.gp"]
	c.open("app/main.gp", src)
	for _, tc := range []hoverCase{
		// A function, with a variadic parameter.
		{"Jo|in(\",", "func Join(sep string, parts ...string) string\nJoin joins the parts with sep.", "Join"},
		// Its declaration.
		{"func Jo|in", "func Join(sep string, parts ...string) string\nJoin joins the parts with sep.", "Join"},
		// A method: with its receiver name.
		{"p.Di|st(", "func (p Point) Dist(q Point) (int, error)\nDist returns the distance from p to q.", "Dist"},
		// A parameter, a local variable.
		{"func (p Point) Dist(|q Point)", "var q Point", "q"},
		{"d, _ := p.|Dist", "func (p Point) Dist(q Point) (int, error)\nDist returns the distance from p to q.", "Dist"},
		{"Count += |d", "var d int", "d"},
		// A package-level variable.
		{"var Cou|nt", "var Count int\nCount counts the calls.", "Count"},
		// A field, with the comment at the end of its line as its doc.
		{"Point{X|: 1}", "field X int\nthe coordinates", "X"},
		// A promoted field: through the embedded field.
		{"n.|X,", "field X int // through Point\nthe coordinates", "X"},
		// An embedded field.
		{"\tPoi|nt\n\tName", "field Point Point", "Point"},
		// Constants: the expression and the value, or a string's value.
		{"|Answer = ", "const Answer untyped int = 6 * 7 // 42\nAnswer is the answer.", "Answer"},
		{"Na|me = ", "const Name untyped string = \"gocpp\"", "Name"},
		// A type: its declaration, its doc and its methods.
		{"Na|med{Point", "type Named struct {\n\tPoint\n\tName string\n}\nNamed has a name.\nfunc (p Point) Dist(q Point) (int, error)", "Named"},
		{"type Po|int", "type Point struct {\n\tX, Y  int // the coordinates\n\tLabel string\n}\nPoint is a point in the plane.\nfunc (p Point) Dist(q Point) (int, error)", "Point"},
		// In another file of the package.
		{"hel|per()", "func helper() int\nhelper returns the answer.", "helper"},
		// From another package of the workspace (export data).
		{"geom.Ar|ea", "func geom.Area(r geom.Rect) int\nArea returns the area of r.", "Area"},
		{"geom.Re|ct{", "type Rect struct {\n\tW, H int\n}\nRect is a rectangle.", "Rect"},
		{"Rect{|W: 2", "field W int", "W"},
		// From the standard library.
		{"fmt.Print|ln", "func fmt.Println(a ...any) (n int, err error)\nPrintln formats using the default formats for its operands and writes to standard output.\nSpaces are always added between operands and a newline is appended.\nIt returns the number of bytes written and any write error encountered.", "Println"},
		// A package: its name in the code, its import path, the package
		// clause.
		{"ge|om.Area", "package geom\nPackage geom does geometry.\n - Package path: example.com/app/geom", "geom"},
		{"\"example.com/app/ge|om\"", "package geom\nPackage geom does geometry.\n - Package path: example.com/app/geom", "\"example.com/app/geom\""},
		{"package ma|in", "package main\nPackage main is the app.\n - Package path: example.com/app/app\n - Module: example.com/app", "main"},
		// Predeclared names, from Go's builtin.go.
		{"var err er|ror", "type error interface {\n\tError() string\n}\nThe error built-in interface type is the conventional interface for\nrepresenting an error condition, with the nil value representing no error.", "error"},
		{"s = app|end", "func append(slice []Type, elems ...Type) []Type\n" + builtinDoc(t, "append"), "append"},
		// C/C++: a header import, a header package, a name from it, and a
		// variable of a C/C++ type, as the user wrote it.
		{"\"./engine|.hpp\"", "import \"./engine.hpp\" // C/C++ header\nIts C-level names are `engine.X` in this file. Their types are opaque to gocpp: the C++ compiler checks them.\n\nFile: `" + filepath.Join(c.root, "app", "engine.hpp") + "`", "\"./engine.hpp\""},
		{"eng|ine.start", "import engine \"./engine.hpp\" // C/C++ header\nThe header package of `./engine.hpp`: its C-level names are `engine.X`. Their types are opaque to gocpp: the C++ compiler checks them.", "engine"},
		{"engine.st|art", "engine.start // C/C++\nA C/C++ name from `./engine.hpp`. Its type is opaque to gocpp: the C++ compiler checks it.", "start"},
		{"var |e *", "var e *engine.Engine", "e"},
		{"func Dr|aw", "func Draw(w *engine.Engine, more ...engine.Engine)\nDraw draws on w.", "Draw"},
		{"func Draw(|w *", "var w *engine.Engine", "w"},
		// A variable whose value comes from C/C++, and a member of it.
		{"|h := engine", "var h // C/C++\nIts type comes from C/C++ and is opaque to gocpp: the C++ compiler checks it.", "h"},
		{"h.r|un()", "h.run // C/C++\nA member of a C/C++ value. Its type is opaque to gocpp: the C++ compiler checks it.", "run"},
		// Nothing: a keyword, a comment, a literal.
		{"fu|nc main", "", ""},
		{"// Count co|unts", "", ""},
		{"\"a|\", \"b\"", "", ""},
	} {
		text, rng := c.hover("app/main.gp", src, tc.mark)
		if text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q:\n%s\n(range %q), want\n%s\n(range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
}

// TestHoverCAndTypeSwitch: C (C.int) is no package the file imports: a
// line on C's built-in types, and no definition (the import "C" that the
// loader adds has no import line). The name of a type switch has the type
// of the value where it is declared, and the clause's type in a clause.
func TestHoverCAndTypeSwitch(t *testing.T) {
	src := "package main\n\nimport \"fmt\"\n\nvar x C.int\n\nfunc main() {\n\tvar v any = 1\n\tswitch y := v.(type) {\n\tcase int:\n\t\tfmt.Println(y, x)\n\t}\n}\n"
	files := map[string]string{"gocpp.mod": "module example.com/c\n", "main.gp": src}
	c := startNav(t, testConfig(), files, plainInit)
	c.open("main.gp", src)
	for _, tc := range []hoverCase{
		{"var x |C.int", "package C\nC's built-in types and helpers (`C.int`, `C.size_t`, `C.CString`), which need no import in gocpp. Their types are opaque to gocpp: the C++ compiler checks them.", "C"},
		{"switch |y", "var y any", "y"},
		{"Println(|y", "var y int", "y"},
	} {
		text, rng := c.hover("main.gp", src, tc.mark)
		if text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q:\n%s\n(range %q), want\n%s\n(range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
	if got := c.definitions("textDocument/definition", "main.gp", src, "var x |C.int"); len(got) != 0 {
		t.Errorf("the definition of C: %q, want none", got)
	}
}

// TestHoverCgoHelpers: check rewrote C.GoString(p) to a function of its hidden package with a
// real declaration (Phase 04 step 4.9): the result has its type (s is a string), the helper
// shows cgo's declaration, C shows C's line, and the names inside the call are found where the
// user wrote them. A conversion of a C value, int(x), is rewritten too (gp_cgoh.As[int](x)).
func TestHoverCgoHelpers(t *testing.T) {
	src := "package main\n\nfunc main() {\n\tvar p *C.char\n\ts := C.GoString(p)\n\tc := C.CString(s)\n\tC.free(c)\n\tn := int(C.int(len(s)))\n\t_ = n\n}\n"
	files := map[string]string{"gocpp.mod": "module example.com/c\n", "main.gp": src}
	c := startNav(t, testConfig(), files, plainInit)
	c.open("main.gp", src)
	cHover := "package C\nC's built-in types and helpers (`C.int`, `C.size_t`, `C.CString`), which need no import in gocpp. Their types are opaque to gocpp: the C++ compiler checks them."
	for _, tc := range []hoverCase{
		{"|s := C", "var s string", "s"},
		{"s := C.GoStr|ing", "func C.GoString(p *C.char) string\nC.GoString copies the C string at p, up to its NUL, into a gocpp string. A nil p gives \"\". p may be any char pointer (const char*, unsigned char*).", "GoString"},
		{"s := |C.GoString", cHover, "C"},
		{"GoString(|p)", "var p *C.char", "p"},
		{"C.CString(|s)", "var s string", "s"},
		{"C.fr|ee(c)", "func C.free(p unsafe.Pointer)\nC.free frees memory of C's heap, as C's free (C.CString's copies). A nil p does nothing.", "free"},
		{"|c := C", "var c // C/C++\nIts type comes from C/C++ and is opaque to gocpp: the C++ compiler checks it.", "c"},
		{"|n := int", "var n int", "n"},
		{"int(C.int(len(|s", "var s string", "s"},
	} {
		text, rng := c.hover("main.gp", src, tc.mark)
		if text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q:\n%s\n(range %q), want\n%s\n(range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
}

// builtinDoc returns the doc comment of the predeclared name in Go's
// builtin.go.
func builtinDoc(t *testing.T, name string) string {
	_, doc := builtinDecl(name)
	if doc == nil {
		t.Fatalf("no doc for %s", name)
	}
	return strings.TrimSuffix(doc.Text(), "\n")
}

// TestHoverMarkdown: a client that prefers Markdown gets Markdown, like
// gopls's: the declaration fenced as Go, then a line, then the doc in
// Markdown and the methods.
func TestHoverMarkdown(t *testing.T) {
	c := startNav(t, testConfig(), navFiles, markdownInit)
	src := navFiles["app/main.gp"]
	c.open("app/main.gp", src)
	for _, tc := range []hoverCase{
		{"Jo|in(\",", "```go\nfunc Join(sep string, parts ...string) string\n```\n\n---\n\nJoin joins the parts with sep.", "Join"},
		{"Na|med{Point", "```go\ntype Named struct {\n\tPoint\n\tName string\n}\n```\n\n---\n\nNamed has a name.\n\n```go\nfunc (p Point) Dist(q Point) (int, error)\n```", "Named"},
		{"ge|om.Area", "```go\npackage geom\n```\n\n---\n\nPackage geom does geometry.\n\n---\n\n - Package path: example.com/app/geom", "geom"},
		{"var |e *", "```go\nvar e *engine.Engine\n```", "e"},
		// The doc comment syntax becomes Markdown.
		{"fmt.Print|ln", "```go\nfunc fmt.Println(a ...any) (n int, err error)\n```\n\n---\n\nPrintln formats using the default formats for its operands and writes to standard output. Spaces are always added between operands and a newline is appended. It returns the number of bytes written and any write error encountered.", "Println"},
	} {
		text, rng := c.hover("app/main.gp", src, tc.mark)
		if text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q:\n%s\n(range %q), want\n%s\n(range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
	// No format in the capabilities: Markdown, like gopls.
	c2 := startNav(t, testConfig(), navFiles, `{"processId":1,"rootUri":null,"capabilities":{}}`)
	c2.open("app/main.gp", src)
	if text, _ := c2.hover("app/main.gp", src, "var |e *"); text != "```go\nvar e *engine.Engine\n```" {
		t.Errorf("without a content format: %q", text)
	}
}

// TestHoverOverlay: the hover sees the unsaved text of the document, in
// the package and in an import.
func TestHoverOverlay(t *testing.T) {
	c := startNav(t, testConfig(), navFiles, plainInit)
	src := navFiles["app/main.gp"]
	c.open("app/main.gp", src)
	src2 := strings.Replace(src, "// Count counts the calls.\nvar Count int", "// Count counts the edits.\nvar Count int64", 1)
	c.change("app/main.gp", 2, src2)
	if text, _ := c.hover("app/main.gp", src2, "var Cou|nt"); text != "var Count int64\nCount counts the edits." {
		t.Errorf("after the change: %q", text)
	}
	geom := strings.Replace(navFiles["geom/geom.gp"], "// Area returns the area of r.", "// Area returns the area of the rectangle r.", 1)
	c.open("geom/geom.gp", geom)
	if text, _ := c.hover("app/main.gp", src2, "geom.Ar|ea"); text != "func geom.Area(r geom.Rect) int\nArea returns the area of the rectangle r." {
		t.Errorf("after the change of geom: %q", text)
	}
}

// TestHoverBroken: in a file that doesn't parse, the names that go/types
// found anyway still have a hover (check.Package.Partial), and so do the
// names of another file of the package.
func TestHoverBroken(t *testing.T) {
	c := startNav(t, testConfig(), navFiles, plainInit)
	// A statement that is being typed, and a broken function at the end.
	src := strings.Replace(navFiles["app/main.gp"], "Count += d", "Count += ", 1) + "\nfunc broken() { x := }\n"
	c.open("app/main.gp", src)
	// didOpen is a notification: a request after it is read after it, so
	// the session has the document once the hover answers.
	c.hover("app/main.gp", src, "hel|per()")
	snap, release := c.s.session.Snapshot()
	pkg, err := snap.PackageOf(context.Background(), c.uri("app/main.gp"))
	release()
	if err != nil || pkg.Check.Checked() || pkg.Check.Partial == nil {
		t.Fatalf("the package: %v, checked %v, partial %v; want a package with load errors and partial types", err, pkg.Check.Checked(), pkg.Check.Partial != nil)
	}
	for _, tc := range []hoverCase{
		{"Jo|in(\",", "func Join(sep string, parts ...string) string\nJoin joins the parts with sep.", "Join"},
		{"geom.Ar|ea", "func geom.Area(r geom.Rect) int\nArea returns the area of r.", "Area"},
		{"hel|per()", "func helper() int\nhelper returns the answer.", "helper"},
	} {
		if text, rng := c.hover("app/main.gp", src, tc.mark); text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q:\n%s\n(range %q), want\n%s", tc.mark, text, rng, tc.text)
		}
	}
	// A broken import: the rest still answers.
	src = strings.Replace(navFiles["app/main.gp"], "\"strings\"", "\"strings\"\n\t\"example.com/app/nosuch\"", 1)
	c.change("app/main.gp", 2, src)
	if text, _ := c.hover("app/main.gp", src, "p.Di|st("); !strings.HasPrefix(text, "func (p Point) Dist") {
		t.Errorf("with a broken import: %q", text)
	}
}

// TestNavigationCached: the requests use the cached check of the package:
// none checks it again while nothing changes.
func TestNavigationCached(t *testing.T) {
	files := noHeaderFiles()
	src := files["app/main.gp"]
	c := startNav(t, testConfig(), files, plainInit)
	c.open("app/main.gp", src)
	c.hover("app/main.gp", src, "Jo|in(\",")
	_, _, checks := c.s.pipeline.Stats()
	for range 3 {
		c.hover("app/main.gp", src, "geom.Ar|ea")
		c.call("textDocument/definition", c.posParams("app/main.gp", src, "hel|per()"))
		c.call("textDocument/signatureHelp", c.posParams("app/main.gp", src, "Join(\",\", |\"a\""))
	}
	if _, _, n := c.s.pipeline.Stats(); n != checks {
		t.Errorf("%d checks after the first hover, want none", n-checks)
	}
}

// TestNavigationCancel: a request whose context is done answers with its
// error, also when its package is in the cache.
func TestNavigationCancel(t *testing.T) {
	checkGoroutines(t)
	c := startNav(t, testConfig(), navFiles, plainInit)
	src := navFiles["app/main.gp"]
	c.open("app/main.gp", src)
	c.hover("app/main.gp", src, "Jo|in(\",") // checked now
	snap, release := c.s.session.Snapshot()
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	params := json.RawMessage(c.posParams("app/main.gp", src, "Jo|in(\","))
	for name, m := range c.s.requests {
		if _, err := m.run(ctx, env{snapshot: snap}, params); !errors.Is(err, context.Canceled) {
			t.Errorf("%s: got the error %v, want context.Canceled", name, err)
		}
	}
	c.call("shutdown", "")
	c.notify("exit", "")
	c.wait()
}
