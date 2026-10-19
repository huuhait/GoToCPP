package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The server on a system whose file names are case-insensitive (Phase 14 step 14.5, ADR-096), run here with the
// CaseFold flavor of fspath over a real folder: the editor spells the names of the files in another case than the
// disk has (the folder was opened as `c:\users\me\proj`, a file was typed in another case), and the server must
// treat all the spellings as one file. The tests change only the spelling of the names of the tree that the test
// made, so the listing that fspath.Canon reads is the real folder's.

// spelled returns the URI of the file name with every element in upper case: a spelling that is not the disk's.
func (c *dclient) spelled(name string) protocol.DocumentURI {
	return protocol.URIFromPath(filepath.Join(c.root, strings.ToUpper(filepath.FromSlash(name))))
}

// mixed is another spelling: the first letter of every element in upper case.
func (c *dclient) mixed(name string) protocol.DocumentURI {
	parts := strings.Split(filepath.FromSlash(name), string(filepath.Separator))
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return protocol.URIFromPath(filepath.Join(append([]string{c.root}, parts...)...))
}

const (
	cfMain = "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() {\n\t_ = lib.F(1)\n}\n"
	cfLib  = "package lib\n\nfunc F(x int) int { return x }\n"
)

// caseFoldFlavor makes the current flavor one that folds case: the system's own on a real PC, else CaseFold.
func caseFoldFlavor() (restore func()) {
	if runtime.GOOS == "windows" {
		return fspath.Use(fspath.Windows)
	}
	return fspath.Use(fspath.CaseFold)
}

func startCaseFold(t *testing.T) *dclient {
	t.Helper()
	t.Cleanup(caseFoldFlavor())
	c := startDiag(t, testConfig(), map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"main.gp":     cfMain,
		"lib/util.gp": cfLib,
	}, nil)
	return c
}

// TestCaseFoldOneDocument: the spellings of one file are one document. A document opened with one spelling is
// changed and closed with others, and the diagnostics come under the spelling the editor opened it with.
func TestCaseFoldOneDocument(t *testing.T) {
	checkGoroutines(t)
	c := startCaseFold(t)
	mainURI, utilOpen, utilChange := c.spelled("main.gp"), c.mixed("lib/util.gp"), c.uri("lib/util.gp")
	c.notify("textDocument/didOpen", didOpen(mainURI, 1, cfMain))
	want(t, c.nextPublish(mainURI), 1, "")

	// The library is open under the mixed spelling, with a second parameter: main is wrong now.
	two := strings.Replace(cfLib, "(x int)", "(x, y int)", 1)
	c.notify("textDocument/didOpen", didOpen(utilOpen, 1, two))
	p := c.nextPublish(mainURI)
	if got := format(p); !strings.Contains(got, "not enough arguments in call to lib.F") {
		t.Fatalf("main.gp with the open library:\n%s", got)
	}
	want(t, c.nextPublish(utilOpen), 1, "")

	// A change under the disk's own spelling reaches the same document, and the answer is under the first spelling.
	c.notify("textDocument/didChange", fullChange(utilChange, 2, cfLib))
	want(t, c.nextPublish(mainURI), 1, "")
	want(t, c.nextPublish(utilOpen), 2, "")
	c.waitOpen(2)
	if o := c.overlay(c.spelled("lib/util.gp")); o == nil || o.Version() != 2 || o.URI() != utilOpen {
		t.Errorf("the library by a third spelling: %+v", o)
	}

	// Opened again under another spelling: the same document, replaced.
	c.notify("textDocument/didOpen", didOpen(utilChange, 3, two))
	c.waitVersion(utilOpen, 3)
	c.waitOpen(2)

	// Closed under a third spelling: gone, and its diagnostics are cleared under the spelling it was published with.
	c.notify("textDocument/didClose", docParams(c.spelled("lib/util.gp")))
	c.waitOpen(1)
	if c.overlay(utilOpen) != nil {
		t.Errorf("the library is still open after its close")
	}
}

// waitVersion waits until the open document uri has the version v.
func (c *dclient) waitVersion(uri protocol.DocumentURI, v int32) {
	c.t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(time.Millisecond) {
		if o := c.overlay(uri); o != nil && o.Version() == v {
			return
		}
		if time.Now().After(deadline) {
			c.t.Fatalf("%s is not open at version %d", uri, v)
		}
	}
}

// waitOpen waits until n documents are open: a notification is handled after the call that sent it returns.
func (c *dclient) waitOpen(n int) {
	c.t.Helper()
	for deadline := time.Now().Add(10 * time.Second); len(c.overlays()) != n; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			c.t.Fatalf("%d documents are open, want %d", len(c.overlays()), n)
		}
	}
}

// TestCaseFoldLocations: an answer about an open document uses the editor's spelling of it, any other file the
// case of the disk.
func TestCaseFoldLocations(t *testing.T) {
	checkGoroutines(t)
	c := startCaseFold(t)
	mainURI, utilOpen := c.spelled("main.gp"), c.mixed("lib/util.gp")
	c.notify("textDocument/didOpen", didOpen(mainURI, 1, cfMain))
	want(t, c.nextPublish(mainURI), 1, "")

	at := func(uri protocol.DocumentURI, src, mark string) string {
		p := at(t, src, mark)
		return fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, uri, p.Line, p.Character)
	}
	locations := func(method string, params string) []protocol.Location {
		t.Helper()
		resp := c.call(method, params)
		if resp.Error != nil {
			t.Fatalf("%s: %v", method, resp.Error)
		}
		var locs []protocol.Location
		if err := json.Unmarshal(resp.Result, &locs); err != nil {
			t.Fatalf("%s: %s: %v", method, resp.Result, err)
		}
		return locs
	}
	// The library is not open: the definition is in the case of the disk.
	locs := locations("textDocument/definition", at(mainURI, cfMain, "lib.|F(1)"))
	if len(locs) != 1 || locs[0].URI != c.uri("lib/util.gp") {
		t.Fatalf("definition of lib.F, library closed: %+v, want %s", locs, c.uri("lib/util.gp"))
	}
	// The library is open under a spelling of the editor's: the same answer names that spelling.
	c.notify("textDocument/didOpen", didOpen(utilOpen, 1, cfLib))
	want(t, c.nextPublish(utilOpen), 1, "")
	locs = locations("textDocument/definition", at(mainURI, cfMain, "lib.|F(1)"))
	if len(locs) != 1 || locs[0].URI != utilOpen || locs[0].Range.Start.Line != 2 {
		t.Fatalf("definition of lib.F, library open: %+v, want %s", locs, utilOpen)
	}
	// References come from both files, each under its own open spelling.
	locs = locations("textDocument/references", strings.TrimSuffix(at(utilOpen, cfLib, "func |F(x"), "}")+`,"context":{"includeDeclaration":true}}`)
	got := map[protocol.DocumentURI]int{}
	for _, l := range locs {
		got[l.URI]++
	}
	if len(locs) != 2 || got[utilOpen] != 1 || got[mainURI] != 1 {
		t.Errorf("references of F: %+v, want one in %s and one in %s", locs, utilOpen, mainURI)
	}
	// A rename edits both open documents under their spellings.
	resp := c.call("textDocument/rename", strings.TrimSuffix(at(mainURI, cfMain, "lib.|F(1)"), "}")+`,"newName":"Renamed"}`)
	if resp.Error != nil {
		t.Fatalf("rename: %v", resp.Error)
	}
	var we protocol.WorkspaceEdit
	if err := json.Unmarshal(resp.Result, &we); err != nil {
		t.Fatal(err)
	}
	if len(we.Changes) != 2 || len(we.Changes[utilOpen]) != 1 || len(we.Changes[mainURI]) != 1 {
		t.Errorf("rename edits: %s, want one edit in %s and one in %s", resp.Result, utilOpen, mainURI)
	}
}

// TestCaseFoldWatchedFiles: the events of the editor's file watcher name files in any spelling; a change on disk is
// found, and a file that was made or removed makes the server forget the case it knew.
func TestCaseFoldWatchedFiles(t *testing.T) {
	checkGoroutines(t)
	c := startCaseFold(t)
	mainURI := c.spelled("main.gp")
	c.notify("textDocument/didOpen", didOpen(mainURI, 1, cfMain))
	want(t, c.nextPublish(mainURI), 1, "")

	// The library on disk gets a parameter; the event names it in a spelling that the disk does not have.
	writeFile(t, c.root, "lib/util.gp", strings.Replace(cfLib, "(x int)", "(x, y int)", 1))
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, c.mixed("lib/util.gp")))
	if got := format(c.nextPublish(mainURI)); !strings.Contains(got, "not enough arguments in call to lib.F") {
		t.Fatalf("main.gp after the library changed on disk:\n%s", got)
	}
	writeFile(t, c.root, "lib/util.gp", cfLib)
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":2}]}`, c.spelled("lib/util.gp")))
	want(t, c.nextPublish(mainURI), 1, "")

	// A file is made with another case than the one the server has asked about: the name it knows is old.
	extra := filepath.Join(c.root, "lib", "extra.gp")
	asked := fspath.Canon(filepath.Join(c.root, "LIB", "EXTRA.GP")) // does not exist yet: as given, under the case of the folder
	if want := filepath.Join(c.root, "lib", "EXTRA.GP"); asked != want {
		t.Fatalf("Canon of a missing file = %q, want %q", asked, want)
	}
	if err := os.WriteFile(extra, []byte("package lib\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":1}]}`, c.spelled("lib/extra.gp")))
	// The notification is handled in order: a request after it sees its effect.
	c.call("workspace/symbol", `{"query":"zzz"}`)
	if got := fspath.Canon(filepath.Join(c.root, "LIB", "EXTRA.GP")); got != extra {
		t.Errorf("Canon after a created file = %q, want %q", got, extra)
	}
}

// TestCaseFoldContentModified: a request that names a document in one spelling is stale after a change that names it in another.
func TestCaseFoldContentModified(t *testing.T) {
	checkGoroutines(t)
	if runtime.GOOS == "windows" {
		t.Cleanup(fspath.Use(fspath.Windows))
	} else {
		t.Cleanup(fspath.Use(fspath.CaseFold))
	}
	root := writeTree(t, map[string]string{"a.gp": "old"})
	disk := protocol.URIFromPath(filepath.Join(root, "a.gp"))
	spelled := protocol.URIFromPath(filepath.Join(root, "A.GP"))
	other := protocol.URIFromPath(filepath.Join(root, "A.gp")) // a third spelling: neither the disk's nor the request's
	m := newTestMethods()
	c := startWith(t, m.setup)
	c.call("initialize", initParams)
	c.notify("textDocument/didOpen", didOpen(disk, 1, "old"))
	stale := c.request("test/stale", docParams(spelled))
	wait(t, m.started)
	c.notify("textDocument/didChange", fullChange(other, 2, "new"))
	wantError(t, c.response(stale), protocol.ContentModified, "changed")
	wait(t, m.returned)
	// The request found the document under its other spelling too.
	if r := textOf(t, c.call("test/text", docParams(spelled))); r.Text != "new" || r.Version != 2 {
		t.Errorf("the document under the other spelling: %+v", r)
	}
	c.settled()
}

// TestCaseFoldFolderDeleted: the event of a deleted folder names it in another spelling than the disk's: the open importer sees it go.
func TestCaseFoldFolderDeleted(t *testing.T) {
	checkGoroutines(t)
	c := startCaseFold(t)
	mainURI := c.spelled("main.gp")
	c.notify("textDocument/didOpen", didOpen(mainURI, 1, cfMain))
	want(t, c.nextPublish(mainURI), 1, "")
	if err := os.RemoveAll(filepath.Join(c.root, "lib")); err != nil {
		t.Fatal(err)
	}
	// The event names only the folder, in a spelling that the disk does not have.
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":3}]}`, c.mixed("lib")))
	if got := format(c.nextPublish(mainURI)); !strings.Contains(got, "lib") {
		t.Errorf("main.gp after the folder lib was deleted:\n%s", got)
	}
}

func TestIsHeaderFold(t *testing.T) {
	t.Run("case-sensitive", func(t *testing.T) {
		t.Cleanup(fspath.Use(fspath.Posix))
		if isHeader("/p/X.H") || !isHeader("/p/x.h") || !isHeader("/p/x.hpp") || isHeader("/p/x.gp") {
			t.Error("isHeader")
		}
	})
	t.Run("case-insensitive", func(t *testing.T) {
		t.Cleanup(fspath.Use(fspath.CaseFold))
		if !isHeader("/p/X.H") || !isHeader("/p/x.HPP") || isHeader("/p/x.GO") {
			t.Error("isHeader")
		}
	})
}

// TestCaseFoldNestedFolderDeleted: a folder with a package below it is deleted, and the event names the folder in another spelling:
// the package below it is gone too (the files and folders under a changed folder are matched by the file system's rule).
func TestCaseFoldNestedFolderDeleted(t *testing.T) {
	checkGoroutines(t)
	t.Cleanup(caseFoldFlavor())
	main := "package main\n\nimport \"example.com/m/lib/inner\"\n\nfunc main() { inner.G() }\n"
	c := startDiag(t, testConfig(), map[string]string{
		"gocpp.mod":          "module example.com/m\n",
		"main.gp":            main,
		"lib/inner/in.gp":    "package inner\n\nfunc G() {}\n",
		"lib/other/other.gp": "package other\n",
	}, nil)
	mainURI := c.spelled("main.gp")
	c.notify("textDocument/didOpen", didOpen(mainURI, 1, main))
	want(t, c.nextPublish(mainURI), 1, "")
	if err := os.RemoveAll(filepath.Join(c.root, "lib")); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":3}]}`, c.spelled("lib")))
	if got := format(c.nextPublish(mainURI)); !strings.Contains(got, "inner") {
		t.Errorf("main.gp after the folder lib, with the package inner below it, was deleted:\n%s", got)
	}
}

// TestCaseFoldModuleDeleted: gocpp.mod is deleted, and the event names it in another spelling (it cannot be found on disk
// any more, so its name is the editor's): every package is read again.
func TestCaseFoldModuleDeleted(t *testing.T) {
	checkGoroutines(t)
	c := startCaseFold(t)
	mainURI := c.spelled("main.gp")
	c.notify("textDocument/didOpen", didOpen(mainURI, 1, cfMain))
	want(t, c.nextPublish(mainURI), 1, "")
	if err := os.Remove(filepath.Join(c.root, "gocpp.mod")); err != nil {
		t.Fatal(err)
	}
	c.notify("workspace/didChangeWatchedFiles", fmt.Sprintf(`{"changes":[{"uri":%q,"type":3}]}`, c.spelled("gocpp.mod")))
	if got := format(c.nextPublish(mainURI)); got == "" {
		t.Errorf("main.gp has no diagnostics after gocpp.mod was deleted")
	}
}

// TestCaseFoldRelatedInformation: the related place of a diagnostic in an open document is under the editor's spelling of it.
func TestCaseFoldRelatedInformation(t *testing.T) {
	checkGoroutines(t)
	t.Cleanup(caseFoldFlavor())
	src := "package x\n\nfunc F() {\n\tx := 1\n\tvar x int\n\t_ = x\n}\n"
	c := startDiag(t, testConfig(), map[string]string{"gocpp.mod": "module example.com/m\n", "x/x.gp": src}, nil)
	uri := c.spelled("x/x.gp")
	c.notify("textDocument/didOpen", didOpen(uri, 1, src))
	p := c.nextPublish(uri)
	if len(p.Diagnostics) != 1 || len(p.Diagnostics[0].RelatedInformation) != 1 {
		t.Fatalf("diagnostics: %s", p.raw)
	}
	if got := p.Diagnostics[0].RelatedInformation[0].Location.URI; got != uri {
		t.Errorf("the related place is under %s, want the editor's %s", got, uri)
	}
}
