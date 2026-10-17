package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// refInit is the initialize of a client that takes versioned document edits
// and has prepareRename.
const refInit = `{"processId":1,"rootUri":null,"capabilities":{"workspace":{"workspaceEdit":{"documentChanges":true}},"textDocument":{"rename":{"prepareSupport":true},"documentSymbol":{"hierarchicalDocumentSymbolSupport":true}}}}`

// startRefsInit is startRefs with the result of initialize.
func startRefsInit(t *testing.T, files map[string]string, init string) (*refClient, protocol.InitializeResult) {
	t.Helper()
	root := writeTree(t, files)
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	resp := c.call("initialize", init)
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	var res protocol.InitializeResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		t.Fatal(err)
	}
	c.notify("initialized", "{}")
	c.skipShown = true
	return &refClient{dclient: c, src: files}, res
}

// TestRefCapabilities: references, rename (with prepareRename for a client
// that has it), highlights and symbols are offered.
func TestRefCapabilities(t *testing.T) {
	_, res := startRefsInit(t, wsFiles, refInit)
	caps := res.Capabilities
	if caps.ReferencesProvider == nil || caps.DocumentHighlightProvider == nil || caps.DocumentSymbolProvider == nil || caps.WorkspaceSymbolProvider == nil {
		t.Errorf("capabilities %+v", caps)
	}
	if opts, ok := caps.RenameProvider.(map[string]any); !ok || opts["prepareProvider"] != true {
		t.Errorf("renameProvider %#v, want prepareProvider", caps.RenameProvider)
	}
	_, res = startRefsInit(t, wsFiles, plainInit)
	if res.Capabilities.RenameProvider != true {
		t.Errorf("renameProvider %#v for a client without prepareSupport, want true", res.Capabilities.RenameProvider)
	}
}

// TestPrepareRename: the range and the name of what can be renamed, and why
// something cannot.
func TestPrepareRename(t *testing.T) {
	c, _ := startRefsInit(t, wsFiles, refInit)
	prepare := func(file, mark string) (protocol.PrepareRenamePlaceholder, *protocol.ResponseError) {
		resp := c.call("textDocument/prepareRename", c.posParams(file, c.text(file), mark))
		if resp.Error != nil {
			return protocol.PrepareRenamePlaceholder{}, resp.Error
		}
		var p protocol.PrepareRenamePlaceholder
		if err := json.Unmarshal(resp.Result, &p); err != nil {
			t.Fatalf("%s: %v", resp.Result, err)
		}
		return p, nil
	}
	p, err := prepare("lib/lib.gp", "func Ne|w(w")
	if err != nil || p.Placeholder != "New" || p.Range.Start.Line != 21 || p.Range.Start.Character != 5 || p.Range.End.Character != 8 {
		t.Errorf("prepareRename of New: %+v, %v", p, err)
	}
	// A use, an embedded field (the type's name), a type switch name.
	for _, tc := range []struct{ file, mark, name string }{
		{"app/main.gp", "lib.Ne|w(2", "New"},
		{"app/main.gp", "h.Re|ct.H", "Rect"},
		{"app/main.gp", "fmt.Println(x| + 1)", "x"},
		{"app/main.gp", "co|unt++", "count"},
		{"app/main.gp", "fm|t.Println(count)", "fmt"},
	} {
		if p, err := prepare(tc.file, tc.mark); err != nil || p.Placeholder != tc.name {
			t.Errorf("prepareRename at %q: %+v, %v, want %s", tc.mark, p, err, tc.name)
		}
	}
	for _, tc := range []struct{ file, mark, msg string }{
		{"lib/lib.gp", "package l|ib", "renaming a package is not supported"},
		{"app/main.gp", "\"example.com/ws/li|b\"", "renaming a package is not supported"},
		{"app/main.gp", "fmt.Print|ln(r", "declared in the standard library"},
		{"lib/lib.gp", "Name() str|ing", "string is predeclared"},
		{"app/main.gp", "\tr := lib.New(2, 3)\n|\tvar sh", "no name at this position"},
	} {
		if _, err := prepare(tc.file, tc.mark); err == nil || !strings.Contains(err.Message, tc.msg) {
			t.Errorf("prepareRename at %q: %v, want %q", tc.mark, err, tc.msg)
		}
	}
}

// TestRenameDocumentChanges: a client with documentChanges gets versioned
// edits: the version of an open document, 0 for a file that is not open.
func TestRenameDocumentChanges(t *testing.T) {
	c, _ := startRefsInit(t, wsFiles, refInit)
	c.open("lib/lib.gp", wsFiles["lib/lib.gp"])
	c.change("lib/lib.gp", 7, wsFiles["lib/lib.gp"])
	p := at(t, wsFiles["lib/lib.gp"], "func New|(w")
	resp := c.call("textDocument/rename", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"newName":"Make"}`, c.uri("lib/lib.gp"), p.Line, p.Character))
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	var we protocol.WorkspaceEdit
	if err := json.Unmarshal(resp.Result, &we); err != nil {
		t.Fatal(err)
	}
	if len(we.Changes) != 0 {
		t.Errorf("changes %v with documentChanges", we.Changes)
	}
	versions := make(map[string]int32)
	for _, dc := range we.DocumentChanges {
		te := dc.TextDocumentEdit
		if te == nil {
			t.Fatalf("a document change that is no edit: %+v", dc)
		}
		versions[strings.TrimPrefix(string(te.TextDocument.URI), string(protocol.URIFromPath(c.root))+"/")] = te.TextDocument.Version
	}
	want := map[string]int32{"app/main.gp": 0, "lib/ext_test.gp": 0, "lib/lib.gp": 7}
	if fmt.Sprint(versions) != fmt.Sprint(want) {
		t.Errorf("versions %v, want %v", versions, want)
	}
}

// TestRenameUnsaved: a rename works on the text of the editor, after
// incremental changes, and not on what is on disk.
func TestRenameUnsaved(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	src := wsFiles["lib/lib.gp"]
	c.open("lib/lib.gp", src)
	// Add a use of New to the open document, as an incremental change.
	line := strings.Index(src, "func Total(")
	text := src[:line] + "var Again = New(3, 4)\n\n" + src[line:]
	c.notify("textDocument/didChange", fmt.Sprintf(`{"textDocument":{"uri":%q,"version":2},"contentChanges":[{"range":{"start":{"line":%d,"character":0},"end":{"line":%d,"character":0}},"text":"var Again = New(3, 4)\n\n"}]}`,
		c.uri("lib/lib.gp"), strings.Count(src[:line], "\n"), strings.Count(src[:line], "\n")))
	c.src["lib/lib.gp"] = text
	defer func() { c.src["lib/lib.gp"] = src }()
	got, err := c.rename("lib/lib.gp", "func New|(w", "Make")
	if err != nil {
		t.Fatal(err)
	}
	lines := diffLinesAgainst(t, text, got["lib/lib.gp"])
	want := []string{"22: func Make(w, h int) Rect { return Rect{W: w, H: h} }", "26: var Default = Make(1, 1)", "45: var Again = Make(3, 4)"}
	sameList(t, "the changed lines of lib.gp", lines, want)
}

// diffLinesAgainst returns the lines of after that differ from before's, as
// "line: text".
func diffLinesAgainst(t *testing.T, before, after string) []string {
	t.Helper()
	b, a := strings.Split(before, "\n"), strings.Split(after, "\n")
	if len(a) != len(b) {
		t.Fatalf("%d lines became %d", len(b), len(a))
	}
	var out []string
	for i := range a {
		if a[i] != b[i] {
			out = append(out, fmt.Sprintf("%d: %s", i+1, strings.TrimSpace(a[i])))
		}
	}
	return out
}

// TestRenameContentModified: the edits were made for a snapshot; when a file
// they touch changed meanwhile, the answer is ContentModified, never edits for
// the old text.
func TestRenameContentModified(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	src := wsFiles["app/main.gp"]
	c.open("app/main.gp", src)
	c.hover("app/main.gp", src, "lib.Ne|w(2") // the open is applied
	snap, release := c.s.session.Snapshot()
	defer release()
	c.change("app/main.gp", 2, strings.Replace(src, "Side: 2", "Side: 3", 1))
	c.hover("app/main.gp", src, "lib.Ne|w(2") // the change is applied
	params := json.RawMessage(strings.Replace(c.posParams("app/main.gp", src, "lib.Ne|w(2"), "}}", `},"newName":"Make"}`, 1))
	// The params of rename have the new name beside the position.
	var raw map[string]any
	if err := json.Unmarshal(params, &raw); err != nil {
		t.Fatal(err)
	}
	raw["newName"] = "Make"
	params, _ = json.Marshal(raw)
	_, err := c.s.requests["textDocument/rename"].run(context.Background(), env{snapshot: snap, log: c.s.log}, params)
	var rerr *protocol.ResponseError
	if !errors.As(err, &rerr) || rerr.Code != int64(protocol.ContentModified) {
		t.Errorf("rename on an old snapshot: %v, want ContentModified", err)
	}
}

// TestRenameTestOnlyImporter: a package that imports another one only in its
// test files is part of its references and renames.
func TestRenameTestOnlyImporter(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":         "module example.com/t\n",
		"lib/lib.gp":        "package lib\n\nfunc F() int { return 1 }\n\ntype R struct{ N int }\n\nfunc (r R) M() int { return r.N }\n",
		"user/user.gp":      "package user\n\nfunc Use() int { return 2 }\n",
		"user/user_test.gp": "package user\n\nimport \"example.com/t/lib\"\n\nfunc check() int { return lib.F() + lib.R{N: 1}.M() }\n",
		"user/ext_test.gp":  "package user_test\n\nimport (\n\t\"example.com/t/lib\"\n\t\"example.com/t/user\"\n)\n\nfunc check2() int { return lib.F() + user.Use() }\n",
	}
	c := startRefs(t, files, plainInit)
	sameList(t, "references of F", c.references("lib/lib.gp", "func F|()", true), []string{
		"lib/lib.gp:3:6 F", "user/ext_test.gp:8:32 F", "user/user_test.gp:5:31 F"})
	c.runRenames(t, []renameCase{
		{"lib/lib.gp", "func F|()", "G", []string{
			"lib/lib.gp:3: func G() int { return 1 }",
			"user/ext_test.gp:8: func check2() int { return lib.G() + user.Use() }",
			"user/user_test.gp:5: func check() int { return lib.G() + lib.R{N: 1}.M() }"}, ""},
		{"lib/lib.gp", "func (r R) M|()", "Count", []string{
			"lib/lib.gp:7: func (r R) Count() int { return r.N }",
			"user/user_test.gp:5: func check() int { return lib.F() + lib.R{N: 1}.Count() }"}, ""},
	})
}

// TestRenameNoModule: a folder without gocpp.mod is one package.
func TestRenameNoModule(t *testing.T) {
	files := map[string]string{
		"a.gp": "package main\n\nfunc f() int { return 1 }\n\nfunc main() { _ = f() }\n",
		"b.gp": "package main\n\nfunc g() int { return f() }\n",
	}
	c := startRefs(t, files, plainInit)
	sameList(t, "references of f", c.references("a.gp", "func f|()", true), []string{"a.gp:3:6 f", "a.gp:5:19 f", "b.gp:3:23 f"})
	c.runRenames(t, []renameCase{
		{"a.gp", "func f|()", "h", []string{"a.gp:3: func h() int { return 1 }", "a.gp:5: func main() { _ = h() }", "b.gp:3: func g() int { return h() }"}, ""},
	})
}

// TestReferencesCpp: a C/C++ name has no references and cannot be renamed;
// the Go names around it are ordinary.
func TestReferencesCpp(t *testing.T) {
	c := startRefs(t, navFiles, plainInit)
	for _, mark := range []string{"engine|.start(e)", "engine.sta|rt(e)", "h.ru|n()", "en|gine.make()"} {
		sameList(t, "references of "+mark, c.references("app/main.gp", mark, true), nil)
		resp := c.call("textDocument/prepareRename", c.posParams("app/main.gp", navFiles["app/main.gp"], mark))
		if resp.Error == nil || !strings.Contains(resp.Error.Message, "C/C++") {
			t.Errorf("prepareRename at %q: %v, want a C/C++ error", mark, resp.Error)
		}
		_, rerr := c.rename("app/main.gp", mark, "x")
		if rerr == nil || !strings.Contains(rerr.Message, "C/C++") {
			t.Errorf("rename at %q: %v, want a C/C++ error", mark, rerr)
		}
	}
	_, rerr := c.rename("app/main.gp", "\"./engine|.hpp\"", "x")
	if rerr == nil || !strings.Contains(rerr.Message, "header") {
		t.Errorf("rename of a header import: %v", rerr)
	}
	// Go names that hold C values rename like any other.
	c.runRenames(t, []renameCase{
		{"app/main.gp", "var e| *engine", "eng", []string{
			"app/main.gp:42: var eng *engine.Engine",
			"app/main.gp:54: engine.start(eng)"}, ""},
	})
}

// TestRenameTestFileDeclarations: the test files of the package declare names
// too: a rename to one of them is refused, though the package's own check
// doesn't see it.
func TestRenameTestFileDeclarations(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	c.runRenames(t, []renameCase{
		{"lib/lib.gp", "func New|(w", "Helper2", nil, "Helper2 is declared in the test file lib_test.gp"},
		{"lib/lib.gp", "func total|(", "helperForTest", nil, "another declaration of helperForTest"},
		{"lib/lib.gp", "func (r Rect) Na|me", "Helper2", []string{
			"app/main.gp:13: func (s Square) Helper2() string { return \"square\" }",
			"app/main.gp:19: fmt.Println(r.Area(), sh.Helper2(), sq.Area(), lib.Max, lib.Default.W)",
			"lib/lib.gp:6: Helper2() string",
			"lib/lib.gp:14: func (r Rect) Helper2() string { return \"rect\" }",
			"lib/lib.gp:19: func (c *Circle) Helper2() string { return \"circle\" }"}, ""},
		{"lib/lib.gp", "func New|(w", "Fresh", []string{
			"app/main.gp:16: r := lib.Fresh(2, 3)",
			"lib/ext_test.gp:5: func use() int { return lib.Fresh(1, 2).W + lib.Max }",
			"lib/lib.gp:22: func Fresh(w, h int) Rect { return Rect{W: w, H: h} }",
			"lib/lib.gp:26: var Default = Fresh(1, 1)"}, ""},
	})
}

// TestRenameExcludedFile: a file that the build leaves out (lib_windows.gp,
// on Linux) is not checked, so it is not renamed; when it mentions the name,
// the user gets a warning.
func TestRenameExcludedFile(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":          "module example.com/x\n",
		"lib/lib.gp":         "package lib\n\nfunc F() int { return 1 }\n",
		"lib/lib_windows.gp": "package lib\n\nfunc g() int { return F() }\n",
		"app/main.gp":        "package main\n\nimport \"example.com/x/lib\"\n\nfunc main() { _ = lib.F() }\n",
	}
	c := startRefs(t, files, plainInit)
	p := at(t, files["lib/lib.gp"], "func F|()")
	c.id++
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"textDocument/rename","params":{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"newName":"G"}}`,
		c.id, c.uri("lib/lib.gp"), p.Line, p.Character))
	var msg protocol.ShowMessageParams
	n := c.readNotification("window/showMessage")
	if err := json.Unmarshal([]byte(n), &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Type != protocol.Warning || !strings.Contains(msg.Message, "lib_windows.gp") || !strings.Contains(msg.Message, "F is not renamed") {
		t.Errorf("the message %+v", msg)
	}
	resp, ok := c.read().(*protocol.Response)
	if !ok || resp.Error != nil {
		t.Fatalf("the response %+v", resp)
	}
	var we protocol.WorkspaceEdit
	if err := json.Unmarshal(resp.Result, &we); err != nil {
		t.Fatal(err)
	}
	if len(we.Changes) != 2 {
		t.Errorf("edits in %d files, want lib.gp and main.gp: %v", len(we.Changes), we.Changes)
	}
}

// TestRefsConcurrent: the requests of this step run at the same time, on one
// snapshot and on newer ones, and answer what they answer alone (run it with
// -race: the caches and the symbol table are shared).
func TestRefsConcurrent(t *testing.T) {
	checkGoroutines(t)
	c := startRefs(t, wsFiles, plainInit)
	c.open("lib/lib.gp", wsFiles["lib/lib.gp"])
	c.open("app/main.gp", wsFiles["app/main.gp"])
	c.hover("app/main.gp", wsFiles["app/main.gp"], "lib.Ne|w(2") // opens applied
	snap, release := c.s.session.Snapshot()
	defer release()
	type req struct {
		method string
		params string
	}
	refs := func(file, mark string) req {
		p := at(t, wsFiles[file], mark)
		return req{"textDocument/references", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"context":{"includeDeclaration":true}}`, c.uri(file), p.Line, p.Character)}
	}
	ren := func(file, mark, name string) req {
		p := at(t, wsFiles[file], mark)
		return req{"textDocument/rename", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"newName":%q}`, c.uri(file), p.Line, p.Character, name)}
	}
	reqs := []req{
		refs("lib/lib.gp", "func New|(w"),
		refs("lib/lib.gp", "(r Rect) Na|me()"),
		refs("app/main.gp", "x.|W)"),
		ren("lib/lib.gp", "func New|(w", "Make"),
		ren("app/main.gp", "count| := 0", "n"),
		ren("lib/lib.gp", "(r Rect) Ar|ea()", "Surface"),
		{"textDocument/documentSymbol", docParams(c.uri("lib/lib.gp"))},
		{"workspace/symbol", `{"query":"area"}`},
		{"textDocument/documentHighlight", c.posParams("app/main.gp", wsFiles["app/main.gp"], "co|unt++")},
	}
	run := func(r req) string {
		res, err := c.s.requests[r.method].run(context.Background(), env{snapshot: snap, log: c.s.log}, json.RawMessage(r.params))
		var rerr *protocol.ResponseError
		if errors.As(err, &rerr) && rerr.Code == int64(protocol.ContentModified) {
			return "modified" // a rename for a file that changed meanwhile
		}
		if err != nil {
			t.Errorf("%s: %v", r.method, err)
			return ""
		}
		b, _ := json.Marshal(res)
		return string(b)
	}
	want := make([]string, len(reqs))
	for i, r := range reqs {
		want[i] = run(r)
	}
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range reqs {
				j := (i + g) % len(reqs)
				if got := run(reqs[j]); got != want[j] && got != "modified" {
					t.Errorf("%s: a concurrent answer differs from the first", reqs[j].method)
				}
			}
		}()
	}
	// Changes meanwhile make new snapshots.
	for v := 2; v < 6; v++ {
		c.change("app/main.gp", v, wsFiles["app/main.gp"]+strings.Repeat("\n", v))
	}
	wg.Wait()
}

// TestRefsBroken: a package with a syntax error still has references and
// renames (from the types that go/types made anyway), and the result check
// compares with the errors it had before.
func TestRefsBroken(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/b\n",
		"p/p.gp":    "package p\n\nfunc good() int {\n\tx := 1\n\treturn x + 1\n}\n\nfunc bad( {\n}\n",
	}
	c := startRefs(t, files, plainInit)
	sameList(t, "references of x", c.references("p/p.gp", "x| := 1", true), []string{"p/p.gp:4:2 x", "p/p.gp:5:9 x"})
	c.runRenames(t, []renameCase{
		{"p/p.gp", "x| := 1", "y", []string{"p/p.gp:4: y := 1", "p/p.gp:5: return y + 1"}, ""},
	})
}

// TestRefsErrors: the answers to bad requests.
func TestRefsErrors(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	for _, method := range []string{"textDocument/references", "textDocument/rename", "textDocument/prepareRename", "textDocument/documentHighlight", "textDocument/documentSymbol", "workspace/symbol"} {
		params := `{"textDocument":5}`
		if method == "workspace/symbol" {
			params = `{"query":5}`
		}
		resp := c.call(method, params)
		if resp.Error == nil || resp.Error.Code != int64(protocol.InvalidParams) {
			t.Errorf("%s with a number where a string or an object is: %v, want InvalidParams", method, resp.Error)
		}
	}
	// A document that is not a file of a package, or is not there: nothing
	// to find, and nothing to rename.
	missing := fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":0,"character":0},"context":{"includeDeclaration":true},"newName":"x"}`, c.uri("nothing/none.gp"))
	if resp := c.call("textDocument/references", missing); resp.Error != nil || string(resp.Result) != "[]" {
		t.Errorf("references in a missing file: %s, %v", resp.Result, resp.Error)
	}
	if resp := c.call("textDocument/documentHighlight", missing); resp.Error != nil || string(resp.Result) != "[]" {
		t.Errorf("highlights in a missing file: %s, %v", resp.Result, resp.Error)
	}
	for _, method := range []string{"textDocument/rename", "textDocument/prepareRename"} {
		if resp := c.call(method, missing); resp.Error == nil || !strings.Contains(resp.Error.Message, "cannot rename") {
			t.Errorf("%s in a missing file: %v", method, resp.Error)
		}
	}
	// A URI that is not a file.
	notFile := `{"textDocument":{"uri":"untitled:Untitled-1"},"position":{"line":0,"character":0},"context":{"includeDeclaration":true},"newName":"x"}`
	if resp := c.call("textDocument/references", notFile); resp.Error == nil || resp.Error.Code != int64(protocol.InvalidParams) {
		t.Errorf("references in an untitled document: %s, %v, want InvalidParams", resp.Result, resp.Error)
	}
	if resp := c.call("textDocument/documentSymbol", docParams("untitled:Untitled-1")); resp.Error == nil {
		t.Errorf("symbols of an untitled document: %s, want an error", resp.Result)
	}
}
