package cache

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/fspath"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The cache on a system whose file names are case-insensitive (Phase 14 step 14.5, ADR-096): the same folder or file
// under two spellings is one package, one document and one file handle.

func caseFold(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Cleanup(fspath.Use(fspath.Windows))
	} else {
		t.Cleanup(fspath.Use(fspath.CaseFold))
	}
}

func (w *world) spelled(name string) protocol.DocumentURI {
	return protocol.URIFromPath(filepath.Join(w.root, strings.ToUpper(filepath.FromSlash(name))))
}

// TestCaseFoldOnePackage: a package asked for under two spellings of its folder is checked once (one handle), and an
// open document under a third spelling is the text that the package sees.
func TestCaseFoldOnePackage(t *testing.T) {
	caseFold(t)
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"main.gp":     "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.F(1) }\n",
		"lib/util.gp": "package lib\n\nfunc F(x int) int { return x }\n",
	})
	snap, release := w.s.Snapshot()
	defer release()
	a, err := snap.Package(context.Background(), filepath.Join(w.root, "lib"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := snap.Package(context.Background(), filepath.Join(w.root, "LIB"))
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Errorf("lib and LIB are two packages (%p, %p)", a, b)
	}
	st := snap.State().(*state)
	st.mu.Lock()
	n := st.handles.size()
	st.mu.Unlock()
	if n != 1 {
		t.Errorf("%d package handles after two spellings of one folder, want 1", n)
	}
	// An open document under another spelling replaces the file in the package.
	w.open("main.gp", "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.F(1) }\n")
	w.s.DidOpen(protocol.TextDocumentItem{URI: w.spelled("lib/util.gp"), LanguageID: "gocpp", Version: 1, Text: "package lib\n\nfunc F(x, y int) int { return x }\n"})
	if got := w.errs(w.pkg(".")); !strings.Contains(got, "not enough arguments") {
		t.Errorf("main does not see the library as opened under LIB/UTIL.GP:\n%s", got)
	}
}

// TestCaseFoldOverlayKeys: opening, changing and closing a document under different spellings is one document.
func TestCaseFoldOverlayKeys(t *testing.T) {
	caseFold(t)
	w := newWorld(t, testConfig(), map[string]string{"gocpp.mod": "module example.com/m\n", "a.gp": "package main\n"})
	item := func(uri protocol.DocumentURI, v int32) protocol.TextDocumentItem {
		return protocol.TextDocumentItem{URI: uri, LanguageID: "gocpp", Version: v, Text: "package main\n"}
	}
	if replaced, err := w.s.DidOpen(item(w.uri("a.gp"), 1)); err != nil || replaced {
		t.Fatalf("first open: %v %v", replaced, err)
	}
	if replaced, err := w.s.DidOpen(item(w.spelled("a.gp"), 2)); err != nil || !replaced {
		t.Errorf("second open under another spelling: replaced %v (%v), want true", replaced, err)
	}
	snap, release := w.s.Snapshot()
	if n := len(snap.Overlays()); n != 1 {
		t.Errorf("%d open documents, want 1", n)
	}
	if o := snap.Overlay(w.uri("a.gp")); o == nil || o.Version() != 2 || o.URI() != w.spelled("a.gp") {
		t.Errorf("the document: %+v", o)
	}
	other := protocol.URIFromPath(filepath.Join(w.root, "A.gp")) // a third spelling
	if o := snap.Overlay(other); o == nil || o.Version() != 2 {
		t.Errorf("the document under a third spelling: %+v", o)
	}
	release()
	if err := w.s.DidChange(w.uri("a.gp"), 3, []protocol.TextDocumentContentChangeEvent{{Text: "package x\n"}}); err != nil {
		t.Errorf("change under the first spelling: %v", err)
	}
	if err := w.s.DidClose(w.spelled("a.gp")); err != nil {
		t.Errorf("close under the second: %v", err)
	}
	snap, release = w.s.Snapshot()
	defer release()
	if n := len(snap.Overlays()); n != 0 {
		t.Errorf("%d documents open after the close", n)
	}
}

// TestDiskFileLosesBOM: a file on disk with a UTF-8 byte order mark reads without it, so that the columns of line 0 are the editor's.
func TestDiskFileLosesBOM(t *testing.T) {
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp":      "\xef\xbb\xbfpackage main; func F() {}\n",
	})
	snap, release := w.s.Snapshot()
	defer release()
	src, err := snap.ReadFile(filepath.Join(w.root, "a.gp"))
	if err != nil || string(src) != "package main; func F() {}\n" {
		t.Errorf("ReadFile = %q, %v", src, err)
	}
	pf, err := snap.ParseFile(context.Background(), w.uri("a.gp"), ParseFull)
	if err != nil || pf.File.Name.Name != "main" {
		t.Fatalf("ParseFile: %v", err)
	}
	if off := pf.Tok.Offset(pf.File.Name.Pos()); off != len("package ") {
		t.Errorf("the package name is at offset %d, want %d (no BOM)", off, len("package "))
	}
}

func TestContainsName(t *testing.T) {
	names := []string{"Main.gp", "util.gp"}
	t.Run("case-sensitive", func(t *testing.T) {
		t.Cleanup(fspath.Use(fspath.Posix))
		if containsName(names, "main.gp") || !containsName(names, "util.gp") {
			t.Error("a case-sensitive folder")
		}
	})
	t.Run("case-insensitive", func(t *testing.T) {
		caseFold(t)
		if !containsName(names, "main.gp") || !containsName(names, "UTIL.GP") || containsName(names, "other.gp") {
			t.Error("a case-insensitive folder")
		}
	})
}

// TestCaseFoldNames: the functions that take a name from outside (the reads of the loader, the files of a folder) find the
// file or folder under a spelling that is not the disk's, and record the canonical name, so that a replay finds it too.
func TestCaseFoldNames(t *testing.T) {
	caseFold(t)
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"main.gp":     "package main\n\nfunc main() {}\n",
		"lib/util.gp": "package lib\n",
	})
	snap, release := w.s.Snapshot()
	defer release()
	upper := func(rel string) string { return filepath.Join(w.root, strings.ToUpper(filepath.FromSlash(rel))) }
	disk := func(rel string) string { return filepath.Join(w.root, filepath.FromSlash(rel)) }

	if src, err := snap.ReadFile(upper("lib/util.gp")); err != nil || string(src) != "package lib\n" {
		t.Errorf("ReadFile under another spelling: %q, %v", src, err)
	}
	if files, err := snap.SourceFiles(upper("lib")); err != nil || len(files) != 1 || files[0] != disk("lib/util.gp") {
		t.Errorf("SourceFiles under another spelling: %q, %v; want %q", files, err, disk("lib/util.gp"))
	}

	// The reads of a load: the recorded name is the canonical one, and a replay of them finds a read by any spelling.
	var reads []*read
	fsys := stateFS{s: snap.State().(*state), log: &reads}
	if _, err := fsys.ReadFile(upper("lib/util.gp")); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.ReadDir(upper("lib")); err != nil {
		t.Fatal(err)
	}
	if _, err := fsys.Stat(upper("main.gp")); err != nil {
		t.Fatal(err)
	}
	for _, r := range reads {
		if r.path != disk("lib/util.gp") && r.path != disk("lib") && r.path != disk("main.gp") {
			t.Errorf("a read was recorded under %q, not the name of the disk", r.path)
		}
	}
	replay := newReplayFS(reads)
	if _, err := replay.ReadFile(upper("lib/util.gp")); err != nil {
		t.Errorf("replay of ReadFile under another spelling: %v", err)
	}
	if _, err := replay.ReadDir(upper("lib")); err != nil {
		t.Errorf("replay of ReadDir under another spelling: %v", err)
	}
}

// TestCaseFoldModuleChangedClone: a changed gocpp.mod makes the packages and module handles start again. The event names it in another
// case than the folder lists (the file is gone, so Canon cannot say), and State.Clone must still know it is the module file.
func TestCaseFoldModuleChangedClone(t *testing.T) {
	caseFold(t)
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod":   "module example.com/m\n",
		"main.gp":     "package main\n\nimport \"example.com/m/lib\"\n\nfunc main() { lib.F() }\n",
		"lib/util.gp": "package lib\n\nfunc F() {}\n",
	})
	w.pkg(".")
	snap, release := w.s.Snapshot()
	st := snap.State().(*state)
	st.mu.Lock()
	before := st.mods.size()
	st.mu.Unlock()
	release()
	if before == 0 {
		t.Fatal("no module handles after a check")
	}
	if err := os.Remove(filepath.Join(w.root, "gocpp.mod")); err != nil {
		t.Fatal(err)
	}
	fspath.Invalidate()
	next := newSnapshot(w.s, 99, map[protocol.DocumentURI]*Overlay{}, nil)
	n := st.Clone(next, Change{Files: map[protocol.DocumentURI]bool{w.spelled("gocpp.mod"): true}}).(*state)
	if n.mods.size() != 0 || n.handles.size() != 0 {
		t.Errorf("after the module file changed (named GOCPP.MOD): %d module handles and %d package handles are kept, want none", n.mods.size(), n.handles.size())
	}
}
