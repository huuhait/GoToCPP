package cache

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// workspaceFiles is a module with a package that has an internal and an
// external test file.
var workspaceFiles = map[string]string{
	"gocpp.mod":        "module example.com/w\n",
	"lib/lib.gp":       "package lib\n\nimport \"strings\"\n\nfunc F() string { return strings.ToUpper(\"a\") }\n\nfunc helper() int { return 1 }\n",
	"lib/lib_test.gp":  "package lib\n\nfunc testOnly() int { return helper() }\n",
	"lib/ext_test.gp":  "package lib_test\n\nimport \"example.com/w/lib\"\n\nfunc use() string { return lib.F() }\n",
	"app/main.gp":      "package main\n\nimport \"example.com/w/lib\"\n\nfunc main() { _ = lib.F() }\n",
	"empty/readme.txt": "no gocpp files\n",
}

// TestImportsOf: the imports of a package come from its metadata without a
// check, in the order the loader asked for them.
func TestImportsOf(t *testing.T) {
	w := newWorld(t, testConfig(), workspaceFiles)
	snap, release := w.s.Snapshot()
	defer release()
	refs, err := snap.ImportsOf(context.Background(), filepath.Join(w.root, "app"))
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].ImportPath != "example.com/w/lib" || refs[0].Std || refs[0].Dir != filepath.Join(w.root, "lib") {
		t.Errorf("imports of app: %+v", refs)
	}
	refs, err = snap.ImportsOf(context.Background(), filepath.Join(w.root, "lib"))
	if err != nil || len(refs) != 1 || refs[0].ImportPath != "strings" || !refs[0].Std {
		t.Errorf("imports of lib: %+v, %v", refs, err)
	}
	if _, _, checks := w.p.Stats(); checks != 0 {
		t.Errorf("%d checks for the imports, want none", checks)
	}
	// A folder that is no package: an error is not a panic; no session
	// state is an error too.
	if refs, err := snap.ImportsOf(context.Background(), filepath.Join(w.root, "empty")); err != nil || len(refs) != 0 {
		t.Errorf("imports of a folder without gocpp files: %+v, %v", refs, err)
	}
	bare, rel := NewSession(nil).Snapshot()
	defer rel()
	if _, err := bare.ImportsOf(context.Background(), w.root); err == nil {
		t.Error("ImportsOf without a pipeline: no error")
	}
}

// TestPackageLookup: the types of a check know the package itself and the
// packages it read from export data, and an object of another check is found
// in them by its objectpath.
func TestPackageLookup(t *testing.T) {
	w := newWorld(t, testConfig(), workspaceFiles)
	pkg := w.pkg("app")
	if tp := pkg.Lookup("main"); tp != nil && tp.Path() != "main" {
		t.Errorf("Lookup of the package itself: %v", tp)
	}
	own := pkg.Check.Types.Path()
	if pkg.Lookup(own) != pkg.Check.Types {
		t.Errorf("Lookup(%q) is not the package's own types", own)
	}
	lib := pkg.Lookup("example.com/w/lib")
	if lib == nil || lib.Scope().Lookup("F") == nil {
		t.Fatalf("Lookup of lib: %v", lib)
	}
	if pkg.Lookup("example.com/w/nothing") != nil {
		t.Error("Lookup of a package the check did not read: not nil")
	}
	paths := pkg.ImportedPaths()
	if len(paths) == 0 || paths[0] > paths[len(paths)-1] {
		t.Errorf("ImportedPaths %q: want them sorted", paths)
	}
	found := false
	for _, p := range paths {
		found = found || p == "example.com/w/lib"
	}
	if !found {
		t.Errorf("ImportedPaths %q has no lib", paths)
	}
}

// TestCheckTests: a package with its test files, from source, once per
// snapshot; a folder without a package is an error.
func TestCheckTests(t *testing.T) {
	w := newWorld(t, testConfig(), workspaceFiles)
	snap, release := w.s.Snapshot()
	defer release()
	dir := filepath.Join(w.root, "lib")
	tc, err := snap.CheckTests(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	// The package with its internal tests, and the external test package.
	if len(tc.Program.Roots) != 2 {
		t.Fatalf("%d roots, want the package and its external tests", len(tc.Program.Roots))
	}
	var tests []string
	for _, root := range tc.Program.Roots {
		cp := tc.Result.Package(root)
		if cp == nil || cp.Types == nil {
			t.Fatalf("root %s has no types", root.ImportPath)
		}
		for _, f := range root.Files {
			if f.Test {
				tests = append(tests, filepath.Base(f.Path))
			}
		}
	}
	if strings.Join(tests, " ") != "lib_test.gp ext_test.gp" && strings.Join(tests, " ") != "ext_test.gp lib_test.gp" {
		t.Errorf("test files %q", tests)
	}
	// The internal test sees the unexported name of the package.
	internal := tc.Result.Package(tc.Program.Roots[0])
	if internal.Types.Scope().Lookup("testOnly") == nil || internal.Types.Scope().Lookup("helper") == nil {
		t.Errorf("the internal test variant: %v", internal.Types.Scope().Names())
	}
	// Once per snapshot: the second call gives the same check.
	before := w.p.stats.testChecks.Load()
	again, err := snap.CheckTests(context.Background(), dir)
	if err != nil || again != tc {
		t.Errorf("a second CheckTests: %v, %v, want the same check", again, err)
	}
	if n := w.p.stats.testChecks.Load(); n != before {
		t.Errorf("%d test checks for the second call", n-before)
	}
	// A change makes a new snapshot: a new check, with the unsaved text.
	w.open("lib/lib_test.gp", "package lib\n\nfunc testOnly() int { return helper() + 2 }\n\nfunc moreTest() {}\n")
	snap2, release2 := w.s.Snapshot()
	defer release2()
	tc2, err := snap2.CheckTests(context.Background(), dir)
	if err != nil || tc2 == tc {
		t.Fatalf("after a change: %v, %v", tc2, err)
	}
	if tc2.Result.Package(tc2.Program.Roots[0]).Types.Scope().Lookup("moreTest") == nil {
		t.Error("the check with tests does not have the unsaved test file")
	}
	// Errors: a folder that is no package, and a cancelled request.
	if _, err := snap.CheckTests(context.Background(), filepath.Join(w.root, "empty")); err == nil {
		t.Error("CheckTests of a folder without gocpp files: no error")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := snap.CheckTests(ctx, filepath.Join(w.root, "app")); err == nil {
		t.Error("CheckTests with a cancelled context: no error")
	}
	bare, rel := NewSession(nil).Snapshot()
	defer rel()
	if _, err := bare.CheckTests(context.Background(), w.root); err == nil {
		t.Error("CheckTests without a pipeline: no error")
	}
}

// TestEdited: a snapshot of what the workspace would be after an edit: the
// open documents of the original with other texts, the original unchanged.
func TestEdited(t *testing.T) {
	w := newWorld(t, testConfig(), workspaceFiles)
	w.open("app/main.gp", workspaceFiles["app/main.gp"])
	snap, release := w.s.Snapshot()
	defer release()
	libUri := w.uri("lib/lib.gp")
	newLib := "package lib\n\nfunc F() string { return \"changed\" }\n\nfunc G() int { return 1 }\n"
	edited, rel, err := snap.Edited(map[protocol.DocumentURI]string{libUri: newLib})
	if err != nil {
		t.Fatal(err)
	}
	if o := edited.Overlay(libUri); o == nil || o.Text() != newLib {
		t.Errorf("the edited overlay of lib.gp: %v", o)
	}
	if o := edited.Overlay(w.uri("app/main.gp")); o == nil || o.Text() != workspaceFiles["app/main.gp"] {
		t.Errorf("the open document that was not edited: %v", o)
	}
	if snap.Overlay(libUri) != nil {
		t.Error("the original snapshot has the edit")
	}
	ctx := context.Background()
	pkg, err := edited.Package(ctx, filepath.Join(w.root, "lib"))
	if err != nil || pkg.Check.Types.Scope().Lookup("G") == nil {
		t.Errorf("the edited package: %v, %v", pkg, err)
	}
	orig, err := snap.Package(ctx, filepath.Join(w.root, "lib"))
	if err != nil || orig.Check.Types.Scope().Lookup("G") != nil {
		t.Errorf("the original package: %v, %v", orig, err)
	}
	// The edit of an open document replaces its text and keeps its version.
	edited2, rel2, err := snap.Edited(map[protocol.DocumentURI]string{w.uri("app/main.gp"): "package main\n\nfunc main() {}\n"})
	if err != nil {
		t.Fatal(err)
	}
	if o := edited2.Overlay(w.uri("app/main.gp")); o == nil || o.Version() != 1 || !strings.Contains(o.Text(), "func main() {}") {
		t.Errorf("the replaced overlay: %v", o)
	}
	rel()
	rel() // twice: nothing happens
	rel2()
	if _, _, err := (&Snapshot{}).Edited(nil); err == nil {
		t.Error("Edited without a pipeline: no error")
	}
}
