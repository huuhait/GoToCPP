package server

import (
	"encoding/json"
	"fmt"
	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// famFiles has an interface in package a, and types that implement it in the
// package, in its internal test file, in its external test package, and in
// another package: the method is one family wherever it is asked for.
var famFiles = map[string]string{
	"gocpp.mod": "module example.com/fam\n",
	"a/a.gp": `package a

type Shape interface{ Area() int }

type Tri struct{}

func (Tri) Area() int { return 1 }

func Total(s Shape) int { return s.Area() }
`,
	"a/a_test.gp": `package a

type testShape struct{}

func (testShape) Area() int { return 2 }

func useTest() int { return Total(testShape{}) }
`,
	"a/ext_test.gp": `package a_test

import "example.com/fam/a"

type xShape struct{}

func (xShape) Area() int { return 3 }

func useExt() int { return a.Total(xShape{}) + xShape{}.Area() }
`,
	"b/b.gp": `package b

import "example.com/fam/a"

type Sq struct{}

func (Sq) Area() int { return 4 }

func Use() int { return a.Total(a.Tri{}) + a.Total(Sq{}) }
`,
}

// famStarts are the places of the family of Area: every declaration of the
// method, and a call through the interface.
var famStarts = []struct{ file, mark string }{
	{"a/a.gp", "Shape interface{ Ar|ea"},
	{"a/a.gp", "(Tri) Ar|ea"},
	{"a/a.gp", "s.Ar|ea()"},
	{"a/a_test.gp", "(testShape) Ar|ea"},
	{"a/ext_test.gp", "(xShape) Ar|ea"},
	{"a/ext_test.gp", "xShape{}.Ar|ea()"},
	{"b/b.gp", "(Sq) Ar|ea"},
}

// TestMethodFamilyTests: the family of an interface method has the methods of
// the types that test files declare, internal and external, and the answer is
// the same from every start; a rename changes all of them, and the check of
// the tests after it passes.
func TestMethodFamilyTests(t *testing.T) {
	c := startRefs(t, famFiles, plainInit)
	want := []string{
		"a/a.gp:3:23 Area", "a/a.gp:7:12 Area", "a/a.gp:9:36 Area",
		"a/a_test.gp:5:18 Area", "a/ext_test.gp:7:15 Area", "a/ext_test.gp:9:57 Area", "b/b.gp:7:11 Area",
	}
	for _, st := range famStarts {
		sameList(t, fmt.Sprintf("references from %s %q", st.file, st.mark), c.references(st.file, st.mark, true), want)
	}
	wantEdit := []string{
		"a/a.gp:3: type Shape interface{ Zq() int }",
		"a/a.gp:7: func (Tri) Zq() int { return 1 }",
		"a/a.gp:9: func Total(s Shape) int { return s.Zq() }",
		"a/a_test.gp:5: func (testShape) Zq() int { return 2 }",
		"a/ext_test.gp:7: func (xShape) Zq() int { return 3 }",
		"a/ext_test.gp:9: func useExt() int { return a.Total(xShape{}) + xShape{}.Zq() }",
		"b/b.gp:7: func (Sq) Zq() int { return 4 }",
	}
	var cases []renameCase
	for _, st := range famStarts {
		cases = append(cases, renameCase{st.file, st.mark, "Zq", wantEdit, ""})
	}
	c.runRenames(t, cases)
}

// brokenFiles: the package p, which q imports, has an error.
func brokenFiles(extra string) map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/br\n",
		"p/p.gp":    "package p\n\ntype Namer interface{ Name() string }\n" + extra,
		"q/q.gp":    "package q\n\nimport \"example.com/br/p\"\n\nfunc F(n p.Namer) string { return n.Name() }\n",
	}
}

// TestRenameBrokenDeclaringPackage: when the declaring package has an error
// that leaves its importers without its exports (a syntax error, an import
// that fails), a rename is refused, never half done; the references say that
// they are not all of them.
func TestRenameBrokenDeclaringPackage(t *testing.T) {
	for name, extra := range map[string]string{
		"syntax": "\nfunc broken3( {\n",
		"import": "\nimport \"example.com/nothing/here\"\n",
		"stdlib": "\nimport \"sort\"\n\nvar _ = sort.Ints\n", // a standard package gocpp does not have yet
	} {
		t.Run(name, func(t *testing.T) {
			c := startRefs(t, brokenFiles(extra), plainInit)
			c.runRenames(t, []renameCase{
				{"p/p.gp", "type Na|mer", "Renamed", nil, "package example.com/br/q imports example.com/br/p, but its check has no types for it"},
				{"p/p.gp", "type Na|mer", "Renamed", nil, "fix the errors first"},
				// From the importer: p.Namer is unavailable there.
				{"q/q.gp", "p.Na|mer", "Renamed", nil, "the package example.com/br/p has errors that make its exports unavailable"},
			})
			// References: what is known, and a warning.
			src := c.text("p/p.gp")
			p := at(t, src, "type Na|mer")
			c.id++
			c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/references","params":{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"context":{"includeDeclaration":true}}}`,
				c.id, c.uri("p/p.gp"), p.Line, p.Character))
			var msg protocol.ShowMessageParams
			if err := json.Unmarshal([]byte(c.readNotification("window/showMessage")), &msg); err != nil {
				t.Fatal(err)
			}
			if msg.Type != protocol.Warning || !strings.Contains(msg.Message, "the results are incomplete") || !strings.Contains(msg.Message, "example.com/br/q") {
				t.Errorf("the warning %+v", msg)
			}
			resp, ok := c.read().(*protocol.Response)
			if !ok || resp.Error != nil {
				t.Fatalf("the response %+v", resp)
			}
			var locs []protocol.Location
			if err := json.Unmarshal(resp.Result, &locs); err != nil || len(locs) != 1 {
				t.Errorf("references: %s, want the declaration (%v)", resp.Result, err)
			}
			// From q: no references, and the same warning.
			q := at(t, c.text("q/q.gp"), "p.Na|mer")
			c.id++
			c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/references","params":{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"context":{"includeDeclaration":true}}}`,
				c.id, c.uri("q/q.gp"), q.Line, q.Character))
			c.readNotification("window/showMessage")
			if resp, ok := c.read().(*protocol.Response); !ok || string(resp.Result) != "[]" {
				t.Errorf("references from q: %+v", resp)
			}
		})
	}
}

// TestRefsInCppComparison: a Go variable in a C++ comparison or conversion
// (check wraps the operand in gp_cgoh.Cmp(...) and gp_cgoh.As[T](...), whose
// names take the operand's place) is found from that use, with the same
// results as from its declaration.
func TestRefsInCppComparison(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "cpp-std", "main.gp"))
	if err != nil {
		t.Skip("no examples/cpp-std:", err)
	}
	files := map[string]string{"gocpp.mod": "module example.com/cppstd\n", "main.gp": string(src)}
	c := startRefs(t, files, plainInit)
	for _, tc := range []struct{ decl, use string }{
		{"|at := s.find", "; |at != std.string.npos"},
		{"|cs := std.string(gs)", "fmt.Println(int(cs.length()), |cs == \"from gocpp\")"},
		{"|ages := std.map", "\"has eve:\", |ages.count(\"eve\") == 1)"},
	} {
		want := c.references("main.gp", tc.decl, true)
		if len(want) < 2 {
			t.Errorf("references from %q: %q", tc.decl, want)
		}
		sameList(t, "references from "+tc.use, c.references("main.gp", tc.use, true), want)
		// A highlight too, and a rename.
		resp := c.call("textDocument/documentHighlight", c.posParams("main.gp", files["main.gp"], tc.use))
		var hl []protocol.DocumentHighlight
		if err := json.Unmarshal(resp.Result, &hl); err != nil || len(hl) != len(want) {
			t.Errorf("highlights from %q: %s, want %d", tc.use, resp.Result, len(want))
		}
		a, errA := c.rename("main.gp", tc.decl, "renamedVar")
		b, errB := c.rename("main.gp", tc.use, "renamedVar")
		if errA != nil || errB != nil || fmt.Sprint(a) != fmt.Sprint(b) {
			t.Errorf("rename from %q: %v %v; from the declaration: %v", tc.use, errB, b, errA)
		}
	}
}

// lenFiles: an interface method, a type that a use needs to be that
// interface, and one that nothing converts to it.
var lenFiles = map[string]string{
	"gocpp.mod": "module example.com/len\n",
	"a/a.gp": `package a

type Lener interface{ Len() int }

type Real struct{}

func (Real) Len() int { return 1 }

type Counterer struct{}

func (Counterer) Len() int { return 2 }

func Use(l Lener) int { return l.Len() }

func f() int { return Use(Real{}) + Counterer{}.Len() }
`,
}

// TestRenameMethodFamilyMessage: a rename of a method changes the methods
// that a use ties to the interface (Real: Use(Real{}) needs a Lener), says so,
// and leaves the one that nothing converts to it (Counterer) alone, as gopls
// does.
func TestRenameMethodFamilyMessage(t *testing.T) {
	c := startRefs(t, lenFiles, plainInit)
	got, rerr := c.rename("a/a.gp", "Lener interface{ L|en", "Size")
	if rerr != nil {
		t.Fatal(rerr)
	}
	sameList(t, "the changed lines", diffLines(t, c, got), []string{
		"a/a.gp:3: type Lener interface{ Size() int }",
		"a/a.gp:7: func (Real) Size() int { return 1 }",
		"a/a.gp:13: func Use(l Lener) int { return l.Size() }",
	})
	shown := c.takeShown()
	if len(shown) != 1 || shown[0].Type != protocol.Info {
		t.Fatalf("the messages: %+v", shown)
	}
	m := shown[0].Message
	for _, want := range []string{"Len was also renamed in 1 other method declaration(s)", "a.Real.Len (a.gp:7)", "ties to the same interface", "Undo the rename"} {
		if !strings.Contains(m, want) {
			t.Errorf("the message lacks %q:\n%s", want, m)
		}
	}
	if strings.Contains(m, "Counterer") {
		t.Errorf("the message names a type that no use ties to the interface:\n%s", m)
	}
	// A rename of a name that has no family says nothing of the kind.
	if _, rerr := c.rename("a/a.gp", "func |Use(", "Used"); rerr != nil {
		t.Fatal(rerr)
	}
	if shown := c.takeShown(); len(shown) != 0 {
		t.Errorf("messages for a function: %+v", shown)
	}
}

// TestRenameWideFamily: with Options.WideFamily (gocpp lsp -wide-family) a
// rename follows the method through every type that has the name and the
// shape of an interface method, and says which of them no use needs.
func TestRenameWideFamily(t *testing.T) {
	c := startRefs(t, lenFiles, plainInit)
	c.s.wideFamily = true
	got, rerr := c.rename("a/a.gp", "Lener interface{ L|en", "Size")
	if rerr != nil {
		t.Fatal(rerr)
	}
	sameList(t, "the changed lines", diffLines(t, c, got), []string{
		"a/a.gp:3: type Lener interface{ Size() int }",
		"a/a.gp:7: func (Real) Size() int { return 1 }",
		"a/a.gp:11: func (Counterer) Size() int { return 2 }",
		"a/a.gp:13: func Use(l Lener) int { return l.Size() }",
		"a/a.gp:15: func f() int { return Use(Real{}) + Counterer{}.Size() }",
	})
	shown := c.takeShown()
	if len(shown) != 1 || shown[0].Type != protocol.Info {
		t.Fatalf("the messages: %+v", shown)
	}
	m := shown[0].Message
	for _, want := range []string{"Len was also renamed in 2 other method declaration(s)", "a.Real.Len (a.gp:7)", "a.Counterer.Len (a.gp:11)", "wider than gopls", "Not needed by any use that gocpp found", "Undo the rename"} {
		if !strings.Contains(m, want) {
			t.Errorf("the message lacks %q:\n%s", want, m)
		}
	}
	unused := m[strings.Index(m, "Not needed by any use"):]
	if strings.Contains(unused, "Real.Len") || !strings.Contains(unused, "Counterer.Len") {
		t.Errorf("the types not needed:\n%s", unused)
	}
}

// TestWarnOtherModules: the workspace has two modules; the search covers the
// one of the document, and the user is told, once for references and every
// time for a rename. A name that stays in its package says nothing.
func TestWarnOtherModules(t *testing.T) {
	files := map[string]string{
		"m1/gocpp.mod": "module example.com/m1\n",
		"m1/x.gp":      "package m1\n\nfunc Exported() int { return local() }\n\nfunc local() int { return 1 }\n",
		"m2/gocpp.mod": "module example.com/m2\n",
		"m2/y.gp":      "package m2\n\nfunc Y() {}\n",
	}
	root := writeTree(t, files)
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	c.skipShown = true
	c.call("initialize", fmt.Sprintf(`{"processId":1,"rootUri":%q,"workspaceFolders":[{"uri":%q,"name":"ws"}],"capabilities":{}}`, fileURI(root), fileURI(root)))
	c.notify("initialized", "{}")
	rc := &refClient{dclient: c, src: files}
	rc.references("m1/x.gp", "func Exp|orted", true)
	shown := c.takeShown()
	if len(shown) != 1 || shown[0].Type != protocol.Warning || !strings.Contains(shown[0].Message, "only the module in "+filepath.Join(root, "m1")) || !strings.Contains(shown[0].Message, filepath.Join(root, "m2")) {
		t.Fatalf("the messages after references: %+v", shown)
	}
	rc.references("m1/x.gp", "func Exp|orted", true)
	if shown := c.takeShown(); len(shown) != 0 {
		t.Errorf("the warning came again for references: %+v", shown)
	}
	rc.references("m1/x.gp", "func lo|cal", true) // unexported: it stays in its package
	if shown := c.takeShown(); len(shown) != 0 {
		t.Errorf("a warning for a local name: %+v", shown)
	}
	for range 2 {
		if _, rerr := rc.rename("m1/x.gp", "func Exp|orted", "Renamed"); rerr != nil {
			t.Fatal(rerr)
		}
		if shown := c.takeShown(); len(shown) != 1 || !strings.Contains(shown[0].Message, "gocpp rename: only the module") {
			t.Errorf("the messages after a rename: %+v", shown)
		}
	}
}

// TestRenameMain: the entry point of a program is not renamed (the build
// would fail), and the messages of a rename at a place with no name say when the
// file has a syntax error.
func TestRenameMain(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"m/m.gp":    "package main\n\nfunc main() { helper() }\n\nfunc helper() {}\n\nfunc bad( {\n}\n",
	}
	c := startRefs(t, files, plainInit)
	c.runRenames(t, []renameCase{
		{"m/m.gp", "func ma|in()", "start", nil, "it is the entry point of the program"},
		{"m/m.gp", "func hel|per", "other", []string{"m/m.gp:3: func main() { other() }", "m/m.gp:5: func other() {}"}, ""},
		{"m/m.gp", "func bad( {\n|}\n", "x", nil, "the file has an error at line 7, and the code after it may not be understood"},
	})
	if resp := c.call("textDocument/prepareRename", c.posParams("m/m.gp", files["m/m.gp"], "func bad( {\n|}\n")); resp.Error == nil || !strings.Contains(resp.Error.Message, "error at line 7") {
		t.Errorf("prepareRename after a syntax error: %v", resp.Error)
	}
}

// promoFiles: Sq has Name from Base, and so satisfies Shape; Base alone does
// not (it has no Area). The family of Name is one from every start.
var promoFiles = map[string]string{
	"gocpp.mod": "module example.com/promo\n",
	"a/a.gp": `package a

type Shape interface {
	Area() int
	Name() string
}

type Base struct{}

func (Base) Name() string { return "base" }

type Sq struct{ Base }

func (Sq) Area() int { return 1 }

var _ Shape = Sq{}

func Label(s Shape) string { return s.Name() + Base{}.Name() }
`,
}

func TestMethodFamilyPromoted(t *testing.T) {
	c := startRefs(t, promoFiles, plainInit)
	starts := []string{"Area() int\n\tNa|me() string", "func (Base) Na|me()", "return s.Na|me()", "Base{}.Na|me()"}
	var cases []renameCase
	for _, m := range starts {
		cases = append(cases, renameCase{"a/a.gp", m, "Title", []string{
			"a/a.gp:5: Title() string",
			"a/a.gp:10: func (Base) Title() string { return \"base\" }",
			"a/a.gp:18: func Label(s Shape) string { return s.Title() + Base{}.Title() }",
		}, ""})
	}
	c.runRenames(t, cases)
}

// TestRenameChecksUneditedTests: with the closure blind to the test files (a
// test hook), a rename of an interface method leaves the types of the test
// files without the method, and edits no test file. The check of the result
// re-checks every test check that the search loaded, so it is still refused.
func TestRenameChecksUneditedTests(t *testing.T) {
	skipTestClosure = true
	defer func() { skipTestClosure = false }()
	c := startRefs(t, famFiles, plainInit)
	c.runRenames(t, []renameCase{
		{"a/a.gp", "Shape interface{ Ar|ea", "Zq", nil, "it would make an error in the tests of a"},
	})
}

// TestRenameNewImplements: a method renamed to the name of a method of a
// well-known standard interface, with its signature, makes its type satisfy
// it: the rename is applied, with a warning that the behavior changes.
func TestRenameNewImplements(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/ni\n",
		"a/a.gp": `package a

type T struct{}

func (T) Show() string { return "t" }

func (T) Other() int { return 1 }

type S []int

func (S) Size() int           { return 0 }
func (S) Less(i, j int) bool  { return false }
func (S) Swap(i, j int)       {}
func (S) Weird(x string) bool { return false }
`,
	}
	c := startRefs(t, files, plainInit)
	for _, tc := range []struct {
		mark, name, want string // want "" for no warning
	}{
		{"Sh|ow", "String", "a.T.Show is now called String and now implements fmt.Stringer"},
		{"Sh|ow", "Error", "now implements error"},
		{"Sh|ow", "Close", ""},   // wrong signature: func() error
		{"Oth|er", "String", ""}, // wrong signature: func() int
		{"Sh|ow", "Fresh", ""},   // not a standard name
		{"Si|ze", "Len", "now implements sort.Interface"},
		{"Wei|rd", "Len", ""}, // wrong signature
	} {
		if _, rerr := c.rename("a/a.gp", tc.mark, tc.name); rerr != nil {
			t.Errorf("rename %q to %s: %v", tc.mark, tc.name, rerr)
			continue
		}
		var warns []string
		for _, m := range c.takeShown() {
			if m.Type == protocol.Warning && strings.Contains(m.Message, "now implements") {
				warns = append(warns, m.Message)
			}
		}
		switch {
		case tc.want == "" && len(warns) != 0:
			t.Errorf("rename %q to %s: unexpected warning %q", tc.mark, tc.name, warns)
		case tc.want != "" && (len(warns) != 1 || !strings.Contains(warns[0], tc.want)):
			t.Errorf("rename %q to %s: warnings %q, want %q", tc.mark, tc.name, warns, tc.want)
		}
	}
}

// TestRenameBrokenTransitive: a imports b imports p; p has a syntax error. The
// rename of a name of p is refused (b imports p and has no types for it), so
// a, which only reaches p through b, is never left behind.
func TestRenameBrokenTransitive(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/tr\n",
		"p/p.gp":    "package p\n\ntype T struct{ X int }\n\nfunc broken( {\n",
		"b/b.gp":    "package b\n\nimport \"example.com/tr/p\"\n\ntype Alias = p.T\n\ntype Emb struct{ p.T }\n",
		"a/a.gp":    "package a\n\nimport \"example.com/tr/b\"\n\nfunc F(x b.Alias, e b.Emb) int { return x.X + e.X }\n",
	}
	c := startRefs(t, files, plainInit)
	c.runRenames(t, []renameCase{
		{"p/p.gp", "type |T struct", "U", nil, "package example.com/tr/b imports example.com/tr/p, but its check has no types for it"},
		{"p/p.gp", "T struct{ |X", "Y", nil, "fix the errors first"},
	})
}
