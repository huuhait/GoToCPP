package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// docSymbols asks for the symbols of the file; the result is hierarchical
// for a client that supports it.
func (c *refClient) docSymbols(name string) json.RawMessage {
	c.t.Helper()
	resp := c.call("textDocument/documentSymbol", docParams(c.uri(name)))
	if resp.Error != nil {
		c.t.Fatalf("documentSymbol %s: %v", name, resp.Error)
	}
	return resp.Result
}

// outline writes the symbols as an indented list: "kind name detail".
func outline(list []protocol.DocumentSymbol, indent string, out *[]string) {
	for _, s := range list {
		line := fmt.Sprintf("%s%d %s", indent, s.Kind, s.Name)
		if s.Detail != "" {
			line += " " + s.Detail
		}
		*out = append(*out, line)
		outline(s.Children, indent+"  ", out)
	}
}

func TestDocumentSymbolHierarchical(t *testing.T) {
	c, _ := startRefsInit(t, wsFiles, refInit)
	c.open("lib/lib.gp", wsFiles["lib/lib.gp"])
	var syms []protocol.DocumentSymbol
	if err := json.Unmarshal(c.docSymbols("lib/lib.gp"), &syms); err != nil {
		t.Fatal(err)
	}
	var got []string
	outline(syms, "", &got)
	want := []string{
		"11 Shape interface",
		"  6 Area func() int",
		"  6 Name func() string",
		"23 Rect struct",
		"  8 W int",
		"  8 H int",
		"6 (Rect).Area func() int",
		"6 (Rect).Name func() string",
		"23 Circle struct",
		"  8 R int",
		"6 (*Circle).Area func() int",
		"6 (*Circle).Name func() string",
		"12 New func(w, h int) Rect",
		"14 Max",
		"13 Default",
		"23 Base struct",
		"  8 ID int",
		"6 (Base).Describe func() string",
		"23 Derived struct",
		"  8 Base Base",
		"  8 Label string",
		"12 total func(shapes []Shape) int",
		"12 Total func(shapes ...Shape) int",
	}
	sameList(t, "the symbols of lib.gp", got, want)
	// The ranges: the whole declaration, and the name inside it.
	for _, s := range syms {
		if s.Name == "New" {
			if s.SelectionRange.Start.Line != 21 || s.SelectionRange.Start.Character != 5 || s.SelectionRange.End.Character != 8 ||
				s.Range.Start.Line != 21 || s.Range.Start.Character != 0 {
				t.Errorf("New: range %+v, selection %+v", s.Range, s.SelectionRange)
			}
		}
	}
}

func TestDocumentSymbolFlat(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	var syms []protocol.SymbolInformation
	if err := json.Unmarshal(c.docSymbols("util/util.gp"), &syms); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range syms {
		got = append(got, fmt.Sprintf("%d %s in %s", s.Kind, s.Name, s.ContainerName))
	}
	sameList(t, "the flat symbols of util.gp", got, []string{
		"12 Double in util", "23 Holder in util", "8 Rect in Holder", "8 Tag in Holder"})
	if len(syms) > 0 && syms[0].Location.URI != c.uri("util/util.gp") {
		t.Errorf("location %v", syms[0].Location)
	}
}

// TestDocumentSymbolBroken: a file with a syntax error still has the symbols
// the parser got to; a file that is not there is an error.
func TestDocumentSymbolBroken(t *testing.T) {
	c, _ := startRefsInit(t, wsFiles, refInit)
	src := "package lib\n\nfunc Good() {}\n\nfunc Bad( {\n\nfunc Later() {}\n"
	c.open("lib/new.gp", src)
	var syms []protocol.DocumentSymbol
	if err := json.Unmarshal(c.docSymbols("lib/new.gp"), &syms); err != nil {
		t.Fatal(err)
	}
	var got []string
	outline(syms, "", &got)
	if len(got) == 0 || got[0] != "12 Good func()" {
		t.Errorf("the symbols of a broken file: %q", got)
	}
	resp := c.call("textDocument/documentSymbol", docParams(c.uri("lib/missing.gp")))
	if resp.Error == nil {
		t.Errorf("a missing file: %s, want an error", resp.Result)
	}
}

// startRefsRoot starts a server whose client has the folder as its workspace
// folder, no document open.
func startRefsRoot(t *testing.T, files map[string]string) *refClient {
	t.Helper()
	root := writeTree(t, files)
	c := &dclient{client: startServer(t, newServer(nil, testConfig(), debounce.RealClock{}), nil), root: root}
	init := fmt.Sprintf(`{"processId":1,"rootUri":%q,"workspaceFolders":[{"uri":%q,"name":"ws"}],"capabilities":{}}`, protocol.URIFromPath(root), protocol.URIFromPath(root))
	c.call("initialize", init)
	c.notify("initialized", "{}")
	c.skipShown = true
	return &refClient{dclient: c, src: files}
}

// workspaceSymbols asks for the symbols matching query and formats them as
// "name (container) file:line".
func (c *refClient) workspaceSymbols(query string) []string {
	c.t.Helper()
	q, _ := json.Marshal(query)
	resp := c.call("workspace/symbol", fmt.Sprintf(`{"query":%s}`, q))
	if resp.Error != nil {
		c.t.Fatalf("workspace/symbol %q: %v", query, resp.Error)
	}
	var syms []protocol.SymbolInformation
	if err := json.Unmarshal(resp.Result, &syms); err != nil {
		c.t.Fatalf("%s: %v", resp.Result, err)
	}
	out := []string{}
	for _, s := range syms {
		rel, _ := filepath.Rel(c.root, s.Location.URI.Path())
		out = append(out, fmt.Sprintf("%s (%s) %s:%d", s.Name, s.ContainerName, filepath.ToSlash(rel), s.Location.Range.Start.Line+1))
	}
	return out
}

func TestWorkspaceSymbol(t *testing.T) {
	c := startRefsRoot(t, wsFiles)
	// An exact match first, then prefixes, substrings and the rest; the
	// members of types are Type.Member; test files and main packages count.
	sameList(t, "rect", c.workspaceSymbols("rect"), []string{
		"Rect (example.com/ws/lib) lib/lib.gp:9",
		"Rect.H (example.com/ws/lib) lib/lib.gp:10",
		"Rect.W (example.com/ws/lib) lib/lib.gp:10",
		"Rect.Area (example.com/ws/lib) lib/lib.gp:13",
		"Rect.Name (example.com/ws/lib) lib/lib.gp:14",
		"Holder.Rect (example.com/ws/util) util/util.gp:9",
	})
	sameList(t, "square", c.workspaceSymbols("Square"), []string{
		"Square (example.com/ws/app) app/main.gp:10",
		"Square.Area (example.com/ws/app) app/main.gp:12",
		"Square.Name (example.com/ws/app) app/main.gp:13",
		"Square.Side (example.com/ws/app) app/main.gp:10",
	})
	// A subsequence, in order.
	sameList(t, "dbl", c.workspaceSymbols("dbl"), []string{
		"Double (example.com/ws/util) util/util.gp:6",
		"Derived.Label (example.com/ws/lib) lib/lib.gp:34", // D, b, l in order
	})
	sameList(t, "helper", c.workspaceSymbols("helperf"), []string{"helperForTest (example.com/ws/lib) lib/lib_test.gp:3"})
	sameList(t, "no match", c.workspaceSymbols("zzzz"), []string{})
	if got := c.workspaceSymbols(""); len(got) < 30 {
		t.Errorf("an empty query: %d symbols, want them all", len(got))
	}
	// The exact name first, then substrings at the start of a word and early
	// ones, then the subsequences.
	sameList(t, "area", c.workspaceSymbols("area"), []string{
		"Rect.Area (example.com/ws/lib) lib/lib.gp:13",
		"Shape.Area (example.com/ws/lib) lib/lib.gp:5",
		"Circle.Area (example.com/ws/lib) lib/lib.gp:18",
		"Square.Area (example.com/ws/app) app/main.gp:12",
		"Square.Name (example.com/ws/app) app/main.gp:13",
	})
}

// TestWorkspaceSymbolLimit: at most maxWorkspaceSymbols come back.
func TestWorkspaceSymbolLimit(t *testing.T) {
	c := startRefsRoot(t, bigPackage())
	if got := c.workspaceSymbols(""); len(got) != maxWorkspaceSymbols {
		t.Errorf("an empty query: %d symbols, want the limit %d", len(got), maxWorkspaceSymbols)
	}
	got := c.workspaceSymbols("F25_3")
	if len(got) != 1 || !strings.HasPrefix(got[0], "F25_3 (example.com/bench/big) big/f25.gp:") {
		t.Errorf("F25_3: %q", got)
	}
}

// TestWorkspaceSymbolOpenDocuments: with no workspace folder, the modules of
// the open documents are the workspace; an unsaved edit is in the symbols; a
// folder without gocpp.mod gives its own files.
func TestWorkspaceSymbolOpenDocuments(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	sameList(t, "before the open", c.workspaceSymbols("Double"), []string{})
	c.open("util/util.gp", wsFiles["util/util.gp"])
	sameList(t, "Double", c.workspaceSymbols("Double"), []string{"Double (example.com/ws/util) util/util.gp:6"})
	c.change("util/util.gp", 2, wsFiles["util/util.gp"]+"\nfunc Triple(s lib.Shape) int { return 3 * s.Area() }\n")
	sameList(t, "Triple", c.workspaceSymbols("Triple"), []string{"Triple (example.com/ws/util) util/util.gp:13"})
	c.change("util/util.gp", 3, wsFiles["util/util.gp"])
	sameList(t, "Triple after the edit is undone", c.workspaceSymbols("Triple"), []string{})

	loose := startRefs(t, map[string]string{"a.gp": "package main\n\nfunc alpha() {}\n\nfunc main() { alpha() }\n"}, plainInit)
	loose.open("a.gp", "package main\n\nfunc alpha() {}\n\nfunc main() { alpha() }\n")
	sameList(t, "a folder without a module", loose.workspaceSymbols("alpha"), []string{"alpha () a.gp:3"})
}

func TestMatchSymbol(t *testing.T) {
	for _, tc := range []struct {
		query, name string
		ok          bool
	}{
		{"", "Anything", true},
		{"rect", "Rect", true},
		{"RECT", "Rect.Area", true},
		{"ra", "Rect.Area", true}, // R..A
		{"area", "Rect", false},
		{"tr", "Rect", false}, // in order only
		{"ünï", "Ünïcode", true},
		{"x", "", false},
	} {
		if _, ok := matchSymbol(tc.query, tc.name); ok != tc.ok {
			t.Errorf("matchSymbol(%q, %q) = %v, want %v", tc.query, tc.name, ok, tc.ok)
		}
	}
	exact, _ := matchSymbol("area", "Area")
	prefix, _ := matchSymbol("area", "AreaOf")
	sub, _ := matchSymbol("area", "TotalArea")
	fuzzy, _ := matchSymbol("area", "AxRxExA")
	if !(exact > prefix && prefix > sub && sub > fuzzy) {
		t.Errorf("scores: exact %d, prefix %d, substring %d, subsequence %d: want them in this order", exact, prefix, sub, fuzzy)
	}
}

// TestSymQueryMatchesMatchSymbol: the prepared query (bytes, lower case names
// made once) gives the score and the answer of matchSymbol for every pair of
// a set of names and queries, ASCII or not.
func TestSymQueryMatchesMatchSymbol(t *testing.T) {
	names := []string{"", "a", "Rect", "Rect.Area", "AreaOf", "TotalArea", "AxRxExA", "area", "Area", "Ünïcode", "Ünï.Çode", "get_value", "GetValue",
		"HTTPServer.ServeHTTP", "x.y.z", "F25_3", "pkg.Type.Method", "ABC", "aBc", "a.b", "_x", "Z"}
	queries := []string{"", "a", "ar", "area", "AREA", "ra", "rect", "Rect", "gv", "GV", "get", "Value", "http", "xyz", "x.y", "f25", "F25_3", "ünï", "Ü", "_", ".", "zz", "bc", "tm"}
	for _, q := range queries {
		sq := newSymQuery(q)
		for _, n := range names {
			lower := strings.ToLower(n)
			sym := wsSymbol{name: n, lower: lower, ascii: isASCII(n) && isASCII(lower)}
			gs, gok := sq.match(&sym)
			ws, wok := matchSymbol(q, n)
			if gs != ws || gok != wok {
				t.Errorf("query %q, name %q: got (%d, %v), matchSymbol says (%d, %v)", q, n, gs, gok, ws, wok)
			}
		}
	}
}

func TestDocumentHighlight(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	src := wsFiles["app/main.gp"]
	c.open("app/main.gp", src)
	resp := c.call("textDocument/documentHighlight", c.posParams("app/main.gp", src, "co|unt++"))
	if resp.Error != nil {
		t.Fatal(resp.Error)
	}
	var hl []protocol.DocumentHighlight
	if err := json.Unmarshal(resp.Result, &hl); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, h := range hl {
		got = append(got, fmt.Sprintf("%d:%d kind %d", h.Range.Start.Line+1, h.Range.Start.Character+1, h.Kind))
	}
	// The declaration and the increment write; the Println reads.
	sameList(t, "highlights of count", got, []string{"43:2 kind 3", "44:18 kind 3", "46:14 kind 2"})
	// A name of another package: only the places in this file.
	resp = c.call("textDocument/documentHighlight", c.posParams("app/main.gp", src, "lib.Ma|x"))
	json.Unmarshal(resp.Result, &hl)
	if len(hl) != 1 || hl[0].Range.Start.Line != 18 {
		t.Errorf("highlights of lib.Max: %+v", hl)
	}
	// A C/C++ name: none.
	nav := startRefs(t, navFiles, plainInit)
	resp = nav.call("textDocument/documentHighlight", nav.posParams("app/main.gp", navFiles["app/main.gp"], "engine.sta|rt(e)"))
	if resp.Error != nil || string(resp.Result) != "[]" {
		t.Errorf("highlights of a C++ name: %s, %v", resp.Result, resp.Error)
	}
}

func fileURI(path string) protocol.DocumentURI { return protocol.URIFromPath(path) }

func newServerForBench() *Server { return newServer(nil, testConfig(), debounce.RealClock{}) }

// TestSymbolTableForgetsDeleted: the symbols of a file that is deleted on disk
// leave the table (and the workspace symbols).
func TestSymbolTableForgetsDeleted(t *testing.T) {
	c := startRefsRoot(t, wsFiles)
	sameList(t, "before", c.workspaceSymbols("Double"), []string{"Double (example.com/ws/util) util/util.gp:6"})
	c.s.symbols.filesMu.Lock()
	n := len(c.s.symbols.files)
	c.s.symbols.filesMu.Unlock()
	if n == 0 {
		t.Fatal("the symbol table is empty after a query")
	}
	if err := os.Remove(filepath.Join(c.root, "util", "util.gp")); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":3}]}`, c.uri("util/util.gp")))
	c.call("workspace/symbol", `{"query":"nothing"}`) // the notification is applied
	c.s.symbols.filesMu.Lock()
	_, has := c.s.symbols.files[filepath.Join(c.root, "util", "util.gp")]
	c.s.symbols.filesMu.Unlock()
	if has {
		t.Error("the symbols of the deleted file are still in the table")
	}
	sameList(t, "after", c.workspaceSymbols("Double"), []string{})
}

// TestIndexNameOfSecondFile: a folder whose first file has a broken package
// clause is a package by its next file's (the walk of the module index does
// that), so the fix of the second file, reported as a changed file, brings
// the folder into the index: the symbols have their package path.
func TestIndexNameOfSecondFile(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/idx\n",
		"p/a.gp":    "package\n\nfunc Asym() {}\n",
		"p/b.gp":    "package\n\nfunc Bsym() {}\n",
	}
	c := startRefsRoot(t, files)
	sameList(t, "before the fix", c.workspaceSymbols("Bsym"), []string{})
	if err := os.WriteFile(filepath.Join(c.root, "p", "b.gp"), []byte("package p\n\nfunc Bsym() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, c.uri("p/b.gp")))
	sameList(t, "after the fix", c.workspaceSymbols("Bsym"), []string{"Bsym (example.com/idx/p) p/b.gp:3"})
}
