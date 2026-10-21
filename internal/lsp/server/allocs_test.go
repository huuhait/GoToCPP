package server

import (
	"context"
	"strings"
	"testing"
)

// TestFileEditsNewText: the text of a file after its edits is the source with each range
// replaced, an import without a name getting one before its path.
func TestFileEditsNewText(t *testing.T) {
	src := "package p\n\nimport \"x/y\"\n\nvar a, b = y.Old, y.Old\n"
	doc := &document{src: []byte(src)}
	oldAt := strings.Index(src, "y.Old") + 2
	secondAt := strings.LastIndex(src, "y.Old") + 2
	importAt := strings.Index(src, "\"x/y\"")
	fe := &fileEdits{doc: doc, offs: []offsetEdit{
		{secondAt, secondAt + 3, "New"}, // out of order: the text sorts them
		{importAt, importAt, "z "},
		{oldAt, oldAt + 3, "New"},
	}}
	want := "package p\n\nimport z \"x/y\"\n\nvar a, b = y.New, y.New\n"
	if got := fe.newText(); got != want {
		t.Errorf("newText:\n%s\nwant:\n%s", got, want)
	}
	if string(doc.src) != src || fe.offs[0].start != secondAt {
		t.Error("newText changed the document or the order of the edits")
	}
	if got := (&fileEdits{doc: doc}).newText(); got != src {
		t.Errorf("newText without edits = %q, want the source", got)
	}
}

// TestRenameBuildsNoTextWhenAProofStandsIn: a rename that a proof covers is not checked on the
// edited text, so no file is copied and edited for the check.
func TestRenameBuildsNoTextWhenAProofStandsIn(t *testing.T) {
	var made int
	onNewText = func() { made++ }
	t.Cleanup(func() { onNewText = nil })
	files := bindingProofFiles()
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", files["a/a.gp"])
	c.next("a/a.gp")

	out, rerr := c.rename("a/a.gp", "func |F(", "G")

	if rerr != nil || !strings.Contains(out["a/a.gp"], "func G(") {
		t.Fatalf("rename of a function: %v, %v", out, rerr)
	}
	if made != 0 {
		t.Errorf("a proved rename made %d texts, want none", made)
	}
}

// TestRenameBuildsTheTextOfEachEditedFileForTheCheck: a rename that no proof covers (a field used
// from another package) is checked on the text of the files it edits, one text for each.
func TestRenameBuildsTheTextOfEachEditedFileForTheCheck(t *testing.T) {
	var made int
	onNewText = func() { made++ }
	t.Cleanup(func() { onNewText = nil })
	files := bindingProofFiles()
	files["a/a.gp"] += "type S struct{ Name string }\n"
	files["b/b.gp"] += "func Fld() string { return alias.S{}.Name }\n"
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", files["a/a.gp"])
	c.next("a/a.gp")

	out, rerr := c.rename("a/a.gp", "Name| string", "Title")

	if rerr != nil || len(out) != 2 || !strings.Contains(out["b/b.gp"], "S{}.Title") {
		t.Fatalf("rename of a field: %v, %v", out, rerr)
	}
	if made != 2 {
		t.Errorf("the checked rename made %d texts, want 2 (a file in each package)", made)
	}
}

// TestRenameRecyclesItsHitList: a rename takes its list of hits from the pool and gives the
// one it searched in back, as references does (see TestReferencesRecyclesItsHitList).
func TestRenameRecyclesItsHitList(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":  "module example.com/disk\n",
		"lib/lib.gp": "package lib\n\nfunc Count() int { return 1 }\n",
		"app/other.gp": "package app\n\nimport \"example.com/disk/lib\"\n\n" +
			"func Use() int { return lib.Count() }\n",
	}
	c := startRefs(t, files, plainInit)
	c.open("lib/lib.gp", files["lib/lib.gp"])
	c.next("lib/lib.gp")
	listA := make([]hit, 0, 16000)
	listB := make([]hit, 0, 15000)
	spareHits.Lock()
	saved := spareHits.lists
	spareHits.lists = [][]hit{listA, listB}
	spareHits.Unlock()
	t.Cleanup(func() {
		spareHits.Lock()
		spareHits.lists = saved
		spareHits.Unlock()
	})
	if _, rerr := c.rename("lib/lib.gp", "func |Count", "Total"); rerr != nil {
		t.Fatal(rerr)
	}
	spareHits.Lock()
	got := append([][]hit(nil), spareHits.lists...)
	spareHits.Unlock()
	if len(got) != 2 || cap(got[0]) != 15000 || cap(got[1]) != 16000 {
		t.Fatalf("the pool after the rename has %d lists, want B (15000) then A (16000)", len(got))
	}
}

// TestModuleImportsAreMadeOncePerSnapshot: every search on a snapshot gets the packages of the
// module and their imports that the first one made; the other direction of the imports is
// right.
func TestModuleImportsAreMadeOncePerSnapshot(t *testing.T) {
	rn := bindingProofRenamer(t)
	mod := rn.sym.q.lp.Module
	ctx := context.Background()
	first := &searcher{s: rn.s, snap: rn.e.snapshot}
	second := &searcher{s: rn.s, snap: rn.e.snapshot}
	one, err := first.moduleImports(ctx, mod)
	if err != nil {
		t.Fatal(err)
	}
	two, err := second.moduleImports(ctx, mod)
	if err != nil || one != two {
		t.Fatalf("two searches on one snapshot made two module imports (%p, %p, %v)", one, two, err)
	}
	if len(one.pkgs) == 0 || len(one.imports) == 0 {
		t.Fatalf("no packages (%d) or imports (%d)", len(one.pkgs), len(one.imports))
	}
	for _, p := range one.pkgs {
		if one.pathOfDir[p.dir] != p.path || one.dirOfPath[p.path] != p.dir {
			t.Errorf("package %s in %s: path of dir %q, dir of path %q", p.path, p.dir, one.pathOfDir[p.dir], one.dirOfPath[p.path])
		}
	}
	for dir, refs := range one.imports {
		for _, ref := range refs {
			if !containsString(one.reverse[ref.ImportPath], dir) {
				t.Errorf("%s imports %s, but the folders that import it are %v", dir, ref.ImportPath, one.reverse[ref.ImportPath])
			}
		}
	}
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// TestFileInfoSize: the table of the files of a search is not made for every file of the views
// when the names allow fewer hits.
func TestFileInfoSize(t *testing.T) {
	for _, tc := range []struct{ files, hits, want int }{
		{1500, 12, 12},
		{1500, 0, 0},
		{40, 3000, 40},
		{0, 5, 0},
	} {
		if got := fileInfoSize(tc.files, tc.hits); got != tc.want {
			t.Errorf("fileInfoSize(%d, %d) = %d, want %d", tc.files, tc.hits, got, tc.want)
		}
	}
}

// TestRenameWarnsOfAFileTheBuildLeavesOut: a file of the package that a build constraint leaves
// out is not renamed, and the rename says so when the file has the name.
func TestRenameWarnsOfAFileTheBuildLeavesOut(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":  "module example.com/m\n",
		"lib/lib.gp": "package lib\n\nfunc Count() int { return 1 }\n",
		"lib/only_windows.gp": "//go:build windows\n\npackage lib\n\n" +
			"func Other() int { return Count() }\n",
	}
	c := startRefs(t, files, plainInit)

	out, rerr := c.rename("lib/lib.gp", "func |Count", "Total")

	if rerr != nil || !strings.Contains(out["lib/lib.gp"], "func Total") {
		t.Fatalf("rename: %v, %v", out, rerr)
	}
	shown := c.takeShown()
	if len(shown) != 1 || !strings.Contains(shown[0].Message, "not renamed in only_windows.gp") {
		t.Fatalf("the messages: %+v", shown)
	}
}

// TestRenameDoesNotWarnWhenThePackageHasEveryFile: a package that has all the files of its folder
// leaves none out, so there is nothing to warn of.
func TestRenameDoesNotWarnWhenThePackageHasEveryFile(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":    "module example.com/m\n",
		"lib/lib.gp":   "package lib\n\nfunc Count() int { return 1 }\n",
		"lib/other.gp": "package lib\n\nfunc Other() int { return Count() }\n",
		"lib/lib_test.gp": "package lib\n\nimport \"testing\"\n\n" +
			"func TestCount(t *testing.T) { _ = Count() }\n",
	}
	c := startRefs(t, files, plainInit)

	out, rerr := c.rename("lib/lib.gp", "func |Count", "Total")

	if rerr != nil || !strings.Contains(out["lib/other.gp"], "Total()") {
		t.Fatalf("rename: %v, %v", out, rerr)
	}
	if shown := c.takeShown(); len(shown) != 0 {
		t.Fatalf("the messages: %+v", shown)
	}
}

// TestWorkspaceFileSymbolsAreMadeOncePerSnapshot: the list of the symbols of the files is made by
// the first query on a snapshot and given to the others as it is.
func TestWorkspaceFileSymbolsAreMadeOncePerSnapshot(t *testing.T) {
	c := warmedRoot(t)
	snap, release := c.s.session.Snapshot()
	defer release()

	first, err := c.s.workspaceFileSymbols(context.Background(), snap)
	if err != nil || len(first) == 0 {
		t.Fatalf("the first list: %d files, %v", len(first), err)
	}
	second, err := c.s.workspaceFileSymbols(context.Background(), snap)

	if err != nil || len(second) != len(first) || &second[0] != &first[0] {
		t.Fatalf("the second list is not the first: %d files, %v", len(second), err)
	}
}

// TestWorkspaceFileSymbolsLackingAFileAreNotKept: a list without the symbols of a file that did
// not parse is given to that query alone, and the next query on the snapshot makes it again.
func TestWorkspaceFileSymbolsLackingAFileAreNotKept(t *testing.T) {
	unparsableInSymbols = func(path string) bool { return strings.HasSuffix(path, "util.gp") }
	t.Cleanup(func() { unparsableInSymbols = nil })
	c := warmedRoot(t) // the background index has run, and no run is left to read the hook
	snap, release := c.s.session.Snapshot()
	defer release()
	count := func(files []fileSyms) int {
		n := 0
		for _, f := range files {
			n += len(f.syms)
		}
		return n
	}
	partial, err := c.s.workspaceFileSymbols(context.Background(), snap)
	if err != nil {
		t.Fatal(err)
	}
	unparsableInSymbols = nil

	whole, err := c.s.workspaceFileSymbols(context.Background(), snap)

	if err != nil || count(whole) <= count(partial) {
		t.Fatalf("symbols: %d without the file, %d after it parses again (%v)", count(partial), count(whole), err)
	}
}
