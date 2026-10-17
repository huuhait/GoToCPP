package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The tests of step 11.9: semantic tokens, inlay hints, document links and
// code actions.

// semFiles is a module for the tokens: a struct, an interface, a constant,
// a generic function, a package that the file imports, and a header.
var semFiles = map[string]string{
	"gocpp.mod": "module example.com/sem\n",
	"sem/sem.gp": `package sem

import (
	"fmt"

	"example.com/sem/other"
	"./engine.hpp"
)

type S struct{ F int }

type I interface{ M() }

const K = 1

var V = other.X

func (s *S) Do(a int) {
	v := a + K
	s.F = v
	fmt.Println(v, other.X)
	engine.start(nil)
	var e *engine.Engine
	_ = e
}

func G[T any](x T) {}
`,
	"sem/engine.hpp": "struct Engine {};\nvoid start(void*);\n",
	"other/other.gp": "package other\n\nvar X = 1\n",
}

// semTokens asks for the semantic tokens of the file name (the whole file,
// or the range) and decodes them as "line:col text type modifiers" (1-based
// lines and UTF-16 columns), in the order of the answer.
func (c *refClient) semTokens(name string, rng *protocol.Range) []string {
	c.t.Helper()
	var resp *protocol.Response
	if rng == nil {
		resp = c.call("textDocument/semanticTokens/full", docParams(c.uri(name)))
	} else {
		resp = c.call("textDocument/semanticTokens/range", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}}}`,
			c.uri(name), rng.Start.Line, rng.Start.Character, rng.End.Line, rng.End.Character))
	}
	if resp.Error != nil {
		c.t.Fatalf("semanticTokens %s: %v", name, resp.Error)
	}
	if string(resp.Result) == "null" {
		return nil
	}
	var toks protocol.SemanticTokens
	if err := json.Unmarshal(resp.Result, &toks); err != nil {
		c.t.Fatal(err)
	}
	return decodeTokens(c.t, c.text(name), toks.Data)
}

// decodeTokens decodes the data of semantic tokens against the text.
func decodeTokens(t testing.TB, text string, data []uint32) []string {
	t.Helper()
	if len(data)%5 != 0 {
		t.Fatalf("%d numbers are no tokens", len(data))
	}
	m := protocol.NewMapper(text)
	var out []string
	line, col := uint32(0), uint32(0)
	for i := 0; i < len(data); i += 5 {
		line += data[i]
		if data[i] == 0 {
			col += data[i+1]
		} else {
			col = data[i+1]
		}
		start := m.Offset(protocol.Position{Line: line, Character: col})
		end := m.Offset(protocol.Position{Line: line, Character: col + data[i+2]})
		var mods []string
		for b, name := range semanticModifiers {
			if data[i+4]&(1<<b) != 0 {
				mods = append(mods, name)
			}
		}
		out = append(out, fmt.Sprintf("%d:%d %s %s %s", line+1, col+1, text[start:end], semanticTypes[data[i+3]], strings.Join(mods, ",")))
	}
	return out
}

func TestSemanticTokens(t *testing.T) {
	c := startRefs(t, semFiles, plainInit)
	got := c.semTokens("sem/sem.gp", nil)
	want := []string{
		"10:6 S struct declaration",
		"10:16 F property declaration",
		"10:18 int type defaultLibrary",
		"12:6 I interface declaration",
		"12:19 M method declaration",
		"14:7 K variable declaration,readonly",
		"16:5 V variable declaration",
		"16:9 other namespace ",
		"16:15 X variable ",
		"18:7 s parameter declaration",
		"18:10 S struct ",
		"18:13 Do method declaration",
		"18:16 a parameter declaration",
		"18:18 int type defaultLibrary",
		"19:2 v variable declaration",
		"19:7 a parameter ",
		"19:11 K variable readonly",
		"20:2 s parameter ",
		"20:4 F property modification",
		"20:8 v variable ",
		"21:2 fmt namespace ",
		"21:6 Println function defaultLibrary",
		"21:14 v variable ",
		"21:17 other namespace ",
		"21:23 X variable ",
		"22:2 engine cppHeader ",
		"22:9 start cppName ",
		"22:15 nil variable readonly,defaultLibrary",
		"23:6 e variable declaration",
		"23:9 engine cppHeader ",
		"23:16 Engine cppName ",
		"24:6 e variable ",
		"27:6 G function declaration",
		"27:8 T typeParameter declaration",
		"27:10 any interface defaultLibrary",
		"27:15 x parameter declaration",
		"27:17 T typeParameter ",
	}
	sameList(t, "the tokens of sem.gp", got, want)

	// A range: the tokens inside it, as the whole file has them.
	rng := &protocol.Range{Start: protocol.Position{Line: 18, Character: 0}, End: protocol.Position{Line: 20, Character: 0}}
	sameList(t, "the tokens of lines 19 and 20", c.semTokens("sem/sem.gp", rng), want[14:20])
}

// TestSemanticTokensLegend: the legend in the capabilities is the one the
// tokens use, and the requests of a document that changes are cancelled.
func TestSemanticTokensLegend(t *testing.T) {
	_, res := startRefsInit(t, semFiles, plainInit)
	opts, ok := res.Capabilities.SemanticTokensProvider.(map[string]any)
	if !ok {
		t.Fatalf("semanticTokensProvider %#v", res.Capabilities.SemanticTokensProvider)
	}
	legend := opts["legend"].(map[string]any)
	var types, mods []string
	for _, x := range legend["tokenTypes"].([]any) {
		types = append(types, x.(string))
	}
	for _, x := range legend["tokenModifiers"].([]any) {
		mods = append(mods, x.(string))
	}
	sameList(t, "token types", types, semanticTypes)
	sameList(t, "token modifiers", mods, semanticModifiers)
	if opts["full"] != true || opts["range"] != true {
		t.Errorf("full %v, range %v", opts["full"], opts["range"])
	}
	s := newServerForBench()
	for _, name := range []string{"textDocument/semanticTokens/full", "textDocument/semanticTokens/range", "textDocument/inlayHint"} {
		if !s.requests[name].contentModified {
			t.Errorf("%s is not cancelled by a change of its document", name)
		}
	}
}

// TestSemanticTokensText: the tokens are those of the text of the editor (an
// unsaved edit), with UTF-16 columns after a character outside the BMP, and a
// document that is not a file of its package has none.
func TestSemanticTokensText(t *testing.T) {
	c := startRefs(t, semFiles, plainInit)
	src := semFiles["sem/sem.gp"]
	c.open("sem/sem.gp", src)
	text := src + "\nfunc Added() int { s := \"\U0001F600\"; _ = s; return K }\n"
	c.change("sem/sem.gp", 2, text)
	c.src["sem/sem.gp"] = text
	got := c.semTokens("sem/sem.gp", nil)
	// The columns count UTF-16 units: the emoji is two.
	sameList(t, "the new function", got[37:], []string{
		"29:6 Added function declaration",
		"29:14 int type defaultLibrary",
		"29:20 s variable declaration",
		"29:35 s variable ",
		"29:45 K variable readonly",
	})
	// A test file is not in its package's check: no tokens.
	c2 := startRefs(t, wsFiles, plainInit)
	if toks := c2.semTokens("lib/lib_test.gp", nil); toks != nil {
		t.Errorf("tokens of a test file: %q", toks)
	}
}

// hintFiles is a package for the inlay hints.
var hintFiles = map[string]string{
	"gocpp.mod": "module example.com/h\n",
	"h/h.gp": `package h

import "fmt"

func add(a, b int) int { return a + b }

func vari(prefix string, rest ...int) {}

type Pt struct{ X, Y int }

func (p Pt) Shift(dx, dy int) Pt { return Pt{p.X + dx, p.Y + dy} }

func use() {
	x := add(1, 2)
	y := 2
	z := add(x, y)
	vari("p", 1, 2, 3)
	for i, v := range []string{"a"} {
		_, _ = i, v
	}
	var w = add(y, 3)
	_ = z + w
	fn := func(n int) {}
	fn(7)
	s := string(rune(65))
	p := Pt{1, 2}.Shift(3, y)
	fmt.Println(s, p, len(s))
	a, b := 1, "two"
	_, _ = a, b
	b, c := "three", 3.5
	_, _ = b, c
}
`,
}

// hints asks for the inlay hints of the file name, in the range or in all of
// it, and formats them as "line:col label kind padding".
func (c *refClient) hints(name string, rng *protocol.Range) []string {
	c.t.Helper()
	src := c.text(name)
	if rng == nil {
		end := protocol.NewMapper(src).Position(len(src))
		rng = &protocol.Range{End: end}
	}
	resp := c.call("textDocument/inlayHint", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}}}`,
		c.uri(name), rng.Start.Line, rng.Start.Character, rng.End.Line, rng.End.Character))
	if resp.Error != nil {
		c.t.Fatalf("inlayHint %s: %v", name, resp.Error)
	}
	var hints []protocol.InlayHint
	if err := json.Unmarshal(resp.Result, &hints); err != nil || hints == nil {
		c.t.Fatalf("inlayHint %s: %s: %v", name, resp.Result, err)
	}
	var out []string
	for _, h := range hints {
		var label string
		for _, part := range h.Label {
			label += part.Value
		}
		pad := ""
		if h.PaddingLeft {
			pad += " left"
		}
		if h.PaddingRight {
			pad += " right"
		}
		out = append(out, fmt.Sprintf("%d:%d %s kind %d%s", h.Position.Line+1, h.Position.Character+1, label, h.Kind, pad))
	}
	return out
}

func TestInlayHints(t *testing.T) {
	c := startRefs(t, hintFiles, plainInit)
	got := c.hints("h/h.gp", nil)
	want := []string{
		"14:3 int kind 1 left",
		"14:11 a: kind 2 right",
		"14:14 b: kind 2 right",
		"15:3 int kind 1 left",
		"16:3 int kind 1 left",
		"16:11 a: kind 2 right",
		"16:14 b: kind 2 right",
		"17:7 prefix: kind 2 right",
		"17:12 rest: kind 2 right",
		"18:7 int kind 1 left",
		"18:10 string kind 1 left",
		"21:7 int kind 1 left",
		"21:14 a: kind 2 right",
		"21:17 b: kind 2 right",
		"23:4 func(n int) kind 1 left",
		"24:5 n: kind 2 right",
		"25:3 string kind 1 left",
		"26:3 Pt kind 1 left",
		"26:22 dx: kind 2 right",
		"26:25 dy: kind 2 right",
		"27:14 a: kind 2 right",
		"28:3 int kind 1 left",
		"28:6 string kind 1 left",
		"30:6 float64 kind 1 left",
	}
	sameList(t, "the hints of h.gp", got, want)
	// The range: only the hints inside it. Line 17 (index 16) is vari(...).
	rng := &protocol.Range{Start: protocol.Position{Line: 16, Character: 0}, End: protocol.Position{Line: 17, Character: 0}}
	sameList(t, "the hints of line 17", c.hints("h/h.gp", rng), want[7:9])
}

// TestInlayHintsOpaque: no hint where the types are C/C++: a call of a C/C++
// function, a variable that holds a C/C++ value; the Go calls around them
// get theirs.
func TestInlayHintsOpaque(t *testing.T) {
	c := startRefs(t, navFiles, plainInit)
	got := c.hints("app/main.gp", nil)
	for _, h := range got {
		// Lines 53 to 56 of main.gp are engine.start(e), h := engine.make(), h.run().
		for _, line := range []string{"53:", "54:", "55:"} {
			if strings.HasPrefix(h, line) {
				t.Errorf("a hint at a C/C++ call: %s (all: %q)", h, got)
			}
		}
	}
	found := false
	for _, h := range got {
		if strings.Contains(h, "sep:") || strings.Contains(h, "int kind 1") {
			found = true
		}
	}
	if !found {
		t.Errorf("no hints for the Go code: %q", got)
	}
}

// TestDocumentLinks: a header import is a link to the header file; an address
// in a comment is a link; a header that is not found has none.
func TestDocumentLinks(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/l\n",
		"p/p.gp":    "// See https://example.com/docs/page.html, and (https://example.org/x?y=1).\npackage p\n\nimport \"./local.h\"\nimport \"<missing_system.h>\"\n\nfunc f() { local.f() }\n",
		"p/local.h": "void f(void);\n",
		"p/q.gp":    "package p\n\nfunc g() {}\n",
	}
	c := startRefs(t, files, plainInit)
	links := func(name string) []protocol.DocumentLink {
		resp := c.call("textDocument/documentLink", docParams(c.uri(name)))
		if resp.Error != nil {
			t.Fatalf("documentLink %s: %v", name, resp.Error)
		}
		var links []protocol.DocumentLink
		if err := json.Unmarshal(resp.Result, &links); err != nil || links == nil {
			t.Fatalf("documentLink %s: %s: %v", name, resp.Result, err)
		}
		return links
	}
	got := links("p/p.gp")
	var list []string
	for _, l := range got {
		list = append(list, fmt.Sprintf("%d:%d-%d:%d %s", l.Range.Start.Line+1, l.Range.Start.Character+1, l.Range.End.Line+1, l.Range.End.Character+1, strings.ReplaceAll(string(*l.Target), string(protocol.URIFromPath(c.root)), "ROOT")))
	}
	sameList(t, "the links of p.gp", list, []string{
		"1:8-1:42 https://example.com/docs/page.html",
		"1:49-1:74 https://example.org/x?y=1",
		"4:8-4:19 ROOT/p/local.h",
	})
	if got[2].Tooltip != "Open the header ./local.h" {
		t.Errorf("tooltip %q", got[2].Tooltip)
	}
	if got := links("p/q.gp"); len(got) != 0 {
		t.Errorf("links of a file without any: %+v", got)
	}
	// A link for each use, in the unsaved text.
	c.open("p/q.gp", "package p\n\nimport \"./local.h\"\n\n// http://example.net/a\nfunc g() {}\n")
	if got := links("p/q.gp"); len(got) != 2 {
		t.Errorf("links of the open q.gp: %+v", got)
	}
}

// importFiles is a module for the quick fix "Add import".
var importFiles = map[string]string{
	"gocpp.mod":    "module example.com/ws2\n",
	"a/a.gp":       "package a\n\nfunc F() string { return strings.ToUpper(\"x\") }\n",
	"b/b.gp":       "package b\n\nimport \"fmt\"\n\nfunc G() { fmt.Println(strings.TrimSpace(\" \")) }\n",
	"c/c.gp":       "package c\n\nfunc H() int { return uniq.Helper() }\n",
	"uniq/u.gp":    "package uniq\n\nfunc Helper() int { return 1 }\n",
	"d/d.gp":       "package d\n\nfunc I() int { return util.Helper() }\n",
	"util/u.gp":    "package util\n\nfunc Helper() int { return 1 }\n",
	"x/util/u2.gp": "package util\n\nfunc Helper() int { return 2 }\n",
}

// fixes asks for the code actions of the file name for the diagnostic
// "undefined: name" at the | of mark, with the extra context (JSON members).
func (c *refClient) fixes(file, mark, name, extra string) []protocol.CodeAction {
	c.t.Helper()
	p := at(c.t, c.text(file), mark)
	context := fmt.Sprintf(`{"diagnostics":[{"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"message":"undefined: %s","source":"gocpp"}]%s}`,
		p.Line, p.Character, p.Line, p.Character+uint32(len(name)), name, extra)
	return c.codeActions(file, context)
}

// applyAction applies the edits of the one document edit of the action to the
// text of the file and returns the result.
func (c *refClient) applyAction(file string, a protocol.CodeAction) string {
	c.t.Helper()
	if a.Edit == nil || len(a.Edit.DocumentChanges) != 1 || a.Edit.DocumentChanges[0].TextDocumentEdit == nil {
		c.t.Fatalf("the action %q has the edit %+v, want one document edit", a.Title, a.Edit)
	}
	te := a.Edit.DocumentChanges[0].TextDocumentEdit
	if te.TextDocument.URI != c.uri(file) {
		c.t.Fatalf("the edit is of %s, want %s", te.TextDocument.URI, c.uri(file))
	}
	var edits []protocol.TextEdit
	for _, e := range te.Edits {
		switch v := e.Value.(type) {
		case protocol.TextEdit:
			edits = append(edits, v)
		case protocol.AnnotatedTextEdit:
			edits = append(edits, v.TextEdit)
		}
	}
	return applyEdits(c.t, c.text(file), edits)
}

func TestCodeActionAddImport(t *testing.T) {
	c := startRefs(t, importFiles, plainInit)
	titles := func(actions []protocol.CodeAction) []string {
		var out []string
		for _, a := range actions {
			out = append(out, fmt.Sprintf("%s [%s]", a.Title, a.Kind))
		}
		return out
	}
	// A standard package, into a file without imports.
	acts := c.fixes("a/a.gp", "return |strings", "strings", "")
	sameList(t, "actions for strings in a.gp", titles(acts), []string{`Add import "strings" [quickfix]`})
	if !acts[0].IsPreferred || len(acts[0].Diagnostics) != 1 {
		t.Errorf("the action %+v: want it preferred, with its diagnostic", acts[0])
	}
	if got, want := c.applyAction("a/a.gp", acts[0]), "package a\n\nimport \"strings\"\n\nfunc F() string { return strings.ToUpper(\"x\") }\n"; got != want {
		t.Errorf("a.gp after the fix:\n%s\nwant:\n%s", got, want)
	}
	// Into the group of the imports the file has.
	acts = c.fixes("b/b.gp", "Println(|strings", "strings", "")
	if len(acts) != 1 {
		t.Fatalf("actions for strings in b.gp: %v", titles(acts))
	}
	if got, want := c.applyAction("b/b.gp", acts[0]), "package b\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc G() { fmt.Println(strings.TrimSpace(\" \")) }\n"; got != want {
		t.Errorf("b.gp after the fix:\n%s\nwant:\n%s", got, want)
	}
	// A package of the module.
	acts = c.fixes("c/c.gp", "return |uniq", "uniq", "")
	sameList(t, "actions for uniq in c.gp", titles(acts), []string{`Add import "example.com/ws2/uniq" [quickfix]`})
	if got, want := c.applyAction("c/c.gp", acts[0]), "package c\n\nimport \"example.com/ws2/uniq\"\n\nfunc H() int { return uniq.Helper() }\n"; got != want {
		t.Errorf("c.gp after the fix:\n%s\nwant:\n%s", got, want)
	}
	// Two packages with the name: by path, none preferred.
	acts = c.fixes("d/d.gp", "return |util", "util", "")
	sameList(t, "actions for util in d.gp", titles(acts), []string{`Add import "example.com/ws2/util" [quickfix]`, `Add import "example.com/ws2/x/util" [quickfix]`})
	if acts[0].IsPreferred || acts[1].IsPreferred {
		t.Errorf("preferred actions among two: %+v", acts)
	}
	// The kinds that are asked for.
	only := func(kinds string) []string {
		return titles(c.fixes("a/a.gp", "return |strings", "strings", `,"only":`+kinds))
	}
	sameList(t, "only quickfix", only(`["quickfix"]`), []string{`Add import "strings" [quickfix]`})
	sameList(t, "only the empty kind (everything)", only(`[""]`), []string{`Add import "strings" [quickfix]`})
	sameList(t, "only source.organizeImports", only(`["source.organizeImports"]`), nil)
	sameList(t, "only refactor", only(`["refactor"]`), nil)
	// Not a missing import: another message, a name that is not at the place
	// of the diagnostic (the text changed since), a name that nothing offers.
	if acts := c.codeActions("a/a.gp", `{"diagnostics":[{"range":{"start":{"line":2,"character":22},"end":{"line":2,"character":29}},"message":"declared and not used: strings"}]}`); len(acts) != 0 {
		t.Errorf("actions for another message: %v", titles(acts))
	}
	if acts := c.fixes("a/a.gp", "return st|rings", "strings", ""); len(acts) != 0 {
		t.Errorf("actions for a diagnostic that is not at the name: %v", titles(acts))
	}
	if acts := c.fixes("a/a.gp", "return |strings", "nosuchpackage", ""); len(acts) != 0 {
		t.Errorf("actions for a name that is not at its place: %v", titles(acts))
	}
	// The unsaved text counts: the diagnostic is of the text of the editor.
	text := "package a\n\n// edited\nfunc F() string { return strings.ToUpper(\"x\") }\n"
	c.open("a/a.gp", text)
	c.src["a/a.gp"] = text
	acts = c.fixes("a/a.gp", "return |strings", "strings", "")
	if len(acts) != 1 || acts[0].Edit.DocumentChanges[0].TextDocumentEdit.TextDocument.Version != 1 {
		t.Fatalf("actions in the open a.gp: %+v", acts)
	}
	if got, want := c.applyAction("a/a.gp", acts[0]), "package a\n\nimport \"strings\"\n\n// edited\nfunc F() string { return strings.ToUpper(\"x\") }\n"; got != want {
		t.Errorf("the open a.gp after the fix:\n%s\nwant:\n%s", got, want)
	}
}

// TestCodeActionCapabilities: the quick fix is among the kinds that a client
// with code action literals is told about.
func TestCodeActionCapabilities(t *testing.T) {
	init := `{"processId":1,"rootUri":null,"capabilities":{"textDocument":{"codeAction":{"codeActionLiteralSupport":{"codeActionKind":{"valueSet":["quickfix","source.organizeImports"]}}}}}}`
	_, res := startRefsInit(t, importFiles, init)
	opts, ok := res.Capabilities.CodeActionProvider.(map[string]any)
	if !ok {
		t.Fatalf("codeActionProvider %#v", res.Capabilities.CodeActionProvider)
	}
	var kinds []string
	for _, k := range opts["codeActionKinds"].([]any) {
		kinds = append(kinds, k.(string))
	}
	sameList(t, "code action kinds", kinds, []string{"quickfix", "source.organizeImports"})
}

// TestEditorFeaturesSweep: the semantic tokens, hints and links of every file
// of the test modules and the example programs come without an error, and the
// tokens are in order, do not overlap and are identifiers of the text.
func TestEditorFeaturesSweep(t *testing.T) {
	trees := map[string]map[string]string{"ws": wsFiles, "nav": navFiles, "sem": semFiles, "hint": hintFiles, "opaque": opaqueFiles}
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
		trees["example-"+filepath.Base(dir)] = files
	}
	for tree, files := range trees {
		t.Run(tree, func(t *testing.T) {
			c := startRefs(t, files, plainInit)
			n := 0
			for name, src := range files {
				if !strings.HasSuffix(name, ".gp") {
					continue
				}
				c.open(name, src)
				for _, l := range c.semTokens(name, nil) {
					n++
					var line, col int
					var text string
					if _, err := fmt.Sscanf(l, "%d:%d %s", &line, &col, &text); err != nil || text == "" {
						t.Fatalf("%s: the token %q", name, l)
					}
				}
				c.hints(name, nil)
				resp := c.call("textDocument/documentLink", docParams(c.uri(name)))
				if resp.Error != nil {
					t.Fatalf("documentLink %s: %v", name, resp.Error)
				}
				checkTokenOrder(t, c, name, src)
			}
			t.Logf("%d tokens", n)
		})
	}
}

// checkTokenOrder checks the raw tokens of the file: sorted, one line each,
// inside the text.
func checkTokenOrder(t *testing.T, c *refClient, name, src string) {
	t.Helper()
	resp := c.call("textDocument/semanticTokens/full", docParams(c.uri(name)))
	if resp.Error != nil || string(resp.Result) == "null" {
		return
	}
	var toks protocol.SemanticTokens
	if err := json.Unmarshal(resp.Result, &toks); err != nil {
		t.Fatal(err)
	}
	m := protocol.NewMapper(src)
	prevEnd := -1
	line, col := uint32(0), uint32(0)
	for i := 0; i < len(toks.Data); i += 5 {
		line += toks.Data[i]
		if toks.Data[i] == 0 {
			col += toks.Data[i+1]
		} else {
			col = toks.Data[i+1]
		}
		start := m.Offset(protocol.Position{Line: line, Character: col})
		end := m.Offset(protocol.Position{Line: line, Character: col + toks.Data[i+2]})
		if start < prevEnd || end <= start || end > len(src) || strings.Contains(src[start:end], "\n") {
			t.Fatalf("%s: token %d at %d:%d-%d overlaps the one before (ends at %d) or is empty or multiline", name, i/5, line+1, col+1, end, prevEnd)
		}
		prevEnd = end
	}
}
