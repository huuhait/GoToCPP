package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/stdlib"
)

// The tests of the memoized pipeline (step 11.3) load small module trees
// with the real standard library stubs.

// testConfig is the Config of the tests: the standard library stubs, for
// linux/amd64, without the phase gates, and no C++ compiler (the trees
// have no header imports).
func testConfig() Config {
	return Config{
		Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64", Phase: check.LastPhase,
		CXX: func(context.Context) (build.CXX, error) {
			return build.CXX{}, errors.New("no C++ compiler in the tests")
		},
	}
}

// writeTree writes the files (slash-separated names relative to the
// folder) into a new temporary folder and returns it.
func writeTree(t testing.TB, files map[string]string) string {
	t.Helper()
	root := filepath.Clean(t.TempDir())
	for name, src := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A world is a session with a pipeline over a module tree.
type world struct {
	t    testing.TB
	root string
	p    *Pipeline
	s    *Session
}

func newWorld(t testing.TB, cfg Config, files map[string]string) *world {
	p := NewPipeline(cfg)
	return &world{t: t, root: writeTree(t, files), p: p, s: NewSession(p.NewState())}
}

func (w *world) uri(name string) protocol.DocumentURI {
	return protocol.URIFromPath(filepath.Join(w.root, filepath.FromSlash(name)))
}

// open opens the file name (relative to the root) with the text.
func (w *world) open(name, text string) {
	w.t.Helper()
	if _, err := w.s.DidOpen(protocol.TextDocumentItem{URI: w.uri(name), LanguageID: "gocpp", Version: 1, Text: text}); err != nil {
		w.t.Fatal(err)
	}
}

// edit replaces the text of the open file name.
func (w *world) edit(name, text string) {
	w.t.Helper()
	if err := w.s.DidChange(w.uri(name), 2, []protocol.TextDocumentContentChangeEvent{{Text: text}}); err != nil {
		w.t.Fatal(err)
	}
}

func (w *world) close(name string) {
	w.t.Helper()
	if err := w.s.DidClose(w.uri(name)); err != nil {
		w.t.Fatal(err)
	}
}

// pkg returns the checked package of the folder dir (relative to the root)
// in the current snapshot.
func (w *world) pkg(dir string) *Package {
	w.t.Helper()
	snap, release := w.s.Snapshot()
	defer release()
	pkg, err := snap.Package(context.Background(), filepath.Join(w.root, filepath.FromSlash(dir)))
	if err != nil {
		w.t.Fatal(err)
	}
	return pkg
}

// checks returns the number of package checks so far.
func (w *world) checks() int64 {
	_, _, n := w.p.Stats()
	return n
}

// errs returns the diagnostics of pkg as gocpp check prints them, with
// file names relative to the root.
func (w *world) errs(pkg *Package) string {
	w.t.Helper()
	var b strings.Builder
	pr := diag.Printer{All: true, Dir: w.root}
	for _, l := range []*diag.List{pkg.Program.Errors, pkg.Check.Errors} {
		if err := pr.Print(&b, l); err != nil {
			w.t.Fatal(err)
		}
	}
	return b.String()
}

// app is a module of three packages: main imports geom and fmt, geom
// imports shapes, and main sees a type of shapes only through geom.
var app = map[string]string{
	"gocpp.mod": "module example.com/app\n",
	"main.gp": `package main

import (
	"fmt"

	"example.com/app/geom"
)

func main() {
	r := geom.Rect{W: 2, H: 3}
	fmt.Println(r.Area(), geom.Unit().Side)
}
`,
	"geom/geom.gp": `package geom

import "example.com/app/shapes"

type Rect struct{ W, H int }

func (r Rect) Area() int { return r.W * r.H }

func Unit() shapes.Square { return shapes.Square{Side: 1} }
`,
	"shapes/shapes.gp": "package shapes\n\ntype Square struct{ Side int }\n",
}

// TestPackageCache checks the hits and misses of the check cache: a
// package is checked once for its content, also in later snapshots and
// when a document is opened with the content it has on disk.
func TestPackageCache(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	main := w.pkg(".")
	if errs := w.errs(main); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	if main.Check == nil || !main.Check.Checked() || main.Check.Types.Name() != "main" {
		t.Fatalf("main is not checked: %+v", main.Check)
	}
	if len(main.Program.Packages) != 1 {
		t.Errorf("the load of main has %d packages, want 1 (the imports are summaries)", len(main.Program.Packages))
	}
	first := w.checks()
	// main, geom, shapes, fmt and the standard packages that fmt imports.
	if first < 4 {
		t.Fatalf("%d checks, want main, geom, shapes, fmt and more", first)
	}
	if again := w.pkg("."); again != main || w.checks() != first {
		t.Errorf("the same snapshot checked again: %d checks", w.checks()-first)
	}
	// A new snapshot whose main.gp has the text on disk: the same key.
	w.open("main.gp", app["main.gp"])
	if again := w.pkg("."); again != main || w.checks() != first {
		t.Errorf("an unchanged document made %d checks", w.checks()-first)
	}
	// An edit checks main again, and only main.
	w.edit("main.gp", strings.Replace(app["main.gp"], "W: 2", "W: 5", 1))
	edited := w.pkg(".")
	if edited == main || w.checks() != first+1 {
		t.Errorf("an edit of main made %d checks, want 1", w.checks()-first)
	}
	// Back to the old text: the old check, from the cache.
	w.edit("main.gp", app["main.gp"])
	if again := w.pkg("."); again != main || w.checks() != first+1 {
		t.Errorf("the old text again: %d more checks, same package %v", w.checks()-first-1, again == main)
	}
}

// TestAPIHash checks the API-hash short-circuit: an edit in a function body
// of a leaf package checks only it; an edit of its API checks its importers
// too, also those that see the API only through another package.
func TestAPIHash(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	main, geom, shapes := w.pkg("."), w.pkg("geom"), w.pkg("shapes")
	n := w.checks()

	// A body edit and a line added above the declarations: geom is
	// checked again, with the same API hash, and main is not.
	w.open("geom/geom.gp", strings.Replace(app["geom/geom.gp"], "return r.W * r.H", "return r.H * r.W", 1))
	w.edit("geom/geom.gp", strings.Replace(app["geom/geom.gp"], "type Rect", "// A Rect.\n\n\ntype Rect", 1))
	geom2 := w.pkg("geom")
	if geom2 == geom || geom2.APIHash != geom.APIHash {
		t.Errorf("geom after a body edit: new check %v, same API %v", geom2 != geom, geom2.APIHash == geom.APIHash)
	}
	if w.pkg(".") != main || w.checks() != n+1 {
		t.Errorf("a body edit of geom made %d checks, want 1 (geom only)", w.checks()-n)
	}

	// An API edit of geom: main is checked again, and sees it.
	n = w.checks()
	w.edit("geom/geom.gp", strings.Replace(app["geom/geom.gp"], "func (r Rect) Area", "func (r Rect) Size", 1))
	main2 := w.pkg(".")
	if main2 == main || w.checks() != n+2 {
		t.Errorf("an API edit of geom made %d checks, want 2 (geom and main)", w.checks()-n)
	}
	if errs := w.errs(main2); !strings.Contains(errs, "r.Area undefined") {
		t.Errorf("main after Area became Size: errors\n%s", errs)
	}
	w.close("geom/geom.gp")
	if w.pkg(".") != main {
		t.Error("main after geom was closed: not the first check")
	}

	// An API edit of shapes, which main sees only through geom.Unit:
	// geom's declarations stay the same, but main is checked again.
	n = w.checks()
	w.open("shapes/shapes.gp", "package shapes\n\ntype Square struct{ Edge int }\n")
	if shapes2 := w.pkg("shapes"); shapes2.APIHash == shapes.APIHash {
		t.Error("shapes: the same API hash after a field was renamed")
	}
	main3 := w.pkg(".")
	if errs := w.errs(main3); !strings.Contains(errs, "Side undefined") {
		t.Errorf("main after Side became Edge: errors\n%s", errs)
	}
	if w.checks() != n+3 {
		t.Errorf("an API edit of shapes made %d checks, want 3 (shapes, geom, main)", w.checks()-n)
	}
}

// TestOverlayShadowsDisk checks that the open documents are read instead
// of the files on disk: an edited file, and a new file that is not saved.
func TestOverlayShadowsDisk(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	w.open("geom/geom.gp", strings.Replace(app["geom/geom.gp"], "type Rect struct{ W, H int }", "type Rect struct{ W, H, D int }", 1))
	w.open("geom/extra.gp", "package geom\n\nfunc Extra() int { return 1 }\n")
	geom := w.pkg("geom")
	rect := geom.Check.Types.Scope().Lookup("Rect")
	if rect == nil || !strings.Contains(rect.Type().Underlying().String(), "D int") {
		t.Errorf("Rect is %v, want the overlay's", rect)
	}
	if geom.Check.Types.Scope().Lookup("Extra") == nil {
		t.Error("the unsaved file extra.gp is not in geom")
	}
	if len(geom.Program.Roots[0].Files) != 2 {
		t.Errorf("geom has %d files, want 2", len(geom.Program.Roots[0].Files))
	}
	w.close("geom/extra.gp")
	w.close("geom/geom.gp")
	geom = w.pkg("geom")
	if geom.Check.Types.Scope().Lookup("Extra") != nil || len(geom.Program.Roots[0].Files) != 1 {
		t.Error("geom after the documents were closed: not the files on disk")
	}

	// An open gocpp.mod changes the module path of every package.
	main := w.pkg(".")
	w.open("gocpp.mod", "module example.com/other\n")
	if errs := w.errs(w.pkg(".")); !strings.Contains(errs, "no required module provides package example.com/app/geom") {
		t.Errorf("main in the module example.com/other: errors\n%s", errs)
	}
	w.close("gocpp.mod")
	if w.pkg(".") != main {
		t.Error("main after gocpp.mod was closed: not the first check")
	}
}

// TestParseCache checks the parse cache: a file is parsed once for its
// content and mode, also after an edit is undone.
func TestParseCache(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	parse := func() *ParsedFile {
		t.Helper()
		snap, release := w.s.Snapshot()
		defer release()
		pf, err := snap.ParseFile(context.Background(), w.uri("main.gp"), ParseFull)
		if err != nil {
			t.Fatal(err)
		}
		return pf
	}
	parses := func() int64 { n, _, _ := w.p.Stats(); return n }
	disk := parse()
	if disk.File.Name.Name != "main" || disk.Err != nil || disk.Tok.Name() != filepath.Join(w.root, "main.gp") {
		t.Fatalf("main.gp: %s, %v, %s", disk.File.Name.Name, disk.Err, disk.Tok.Name())
	}
	n := parses()
	if parse() != disk || parses() != n {
		t.Error("parsed again in the same snapshot")
	}
	w.open("main.gp", app["main.gp"]+"\nfunc f() {}\n")
	edited := parse()
	if edited == disk || parses() != n+1 || len(edited.File.Decls) != len(disk.File.Decls)+1 {
		t.Errorf("after an edit: %d parses, want 1", parses()-n)
	}
	w.edit("main.gp", app["main.gp"])
	if parse() != disk || parses() != n+1 {
		t.Error("the text of the disk again: parsed again")
	}
	w.edit("main.gp", "package main\n\nfunc {\n")
	if broken := parse(); broken.Err == nil {
		t.Error("no syntax error")
	}
}

// TestConcurrent asks for the same package from many goroutines at once, on
// one snapshot and on the snapshots of edits made meanwhile: each package
// content is checked once.
func TestConcurrent(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	snap, release := w.s.Snapshot()
	defer release()
	var wg sync.WaitGroup
	got := make([]*Package, 32)
	for i := range got {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pkg, err := snap.Package(context.Background(), w.root)
			if err != nil {
				t.Error(err)
			}
			got[i] = pkg
		}()
	}
	wg.Wait()
	for _, pkg := range got {
		if pkg != got[0] {
			t.Fatal("two goroutines got different checks of one package")
		}
	}
	n := w.checks()
	ref := newWorld(t, testConfig(), app)
	ref.pkg(".")
	if n != ref.checks() {
		t.Errorf("%d checks at the same time, want %d (one per package)", n, ref.checks())
	}

	// Edits while requests run.
	w.open("geom/geom.gp", app["geom/geom.gp"])
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			snap, release := w.s.Snapshot()
			defer release()
			if _, err := snap.Package(context.Background(), w.root); err != nil {
				t.Error(err)
			}
		}()
		w.edit("geom/geom.gp", strings.Replace(app["geom/geom.gp"], "r.W * r.H", "r.W * r.H * "+string(rune('1'+i%3)), 1))
	}
	wg.Wait()
	// geom had 3 contents (and the first), main one: at most 4 more checks.
	if extra := w.checks() - n; extra > 4 {
		t.Errorf("%d checks for 4 contents of geom", extra)
	}
}

// TestCancel checks that a request that gives up stops the work nobody
// else waits for, and that a later request gets the result.
func TestCancel(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	snap, release := w.s.Snapshot()
	defer release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := snap.Package(ctx, w.root); !errors.Is(err, context.Canceled) {
		t.Errorf("a cancelled request: %v", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	snap.Package(ctx, w.root) // may finish or not
	pkg, err := snap.Package(context.Background(), w.root)
	if err != nil || !pkg.Check.Checked() {
		t.Fatalf("after the cancelled requests: %v", err)
	}
}

// TestLRU checks the memory cap of the checked packages: the cache keeps
// no more than MaxCheckBytes of source (and the newest package), a package
// that was dropped is checked again when asked for, and its importers still
// get its summary without a check.
func TestLRU(t *testing.T) {
	files := map[string]string{"gocpp.mod": "module example.com/app\n", "main.gp": "package main\n\nimport (\n"}
	var imports, uses string
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		files[name+"/"+name+".gp"] = "package " + name + "\n\nfunc F() int { return 1 }\n" + strings.Repeat("// padding\n", 20)
		imports += "\t\"example.com/app/" + name + "\"\n"
		uses += "\t_ = " + name + ".F()\n"
	}
	files["main.gp"] += imports + ")\n\nfunc main() {\n" + uses + "}\n"
	cfg := testConfig()
	cfg.MaxCheckBytes = 35000 // about three of the small packages (cost: sourceFactor × 250 bytes)
	w := newWorld(t, cfg, files)
	if errs := w.errs(w.pkg(".")); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	if count, cost := w.p.checks.Stats(); cost > cfg.MaxCheckBytes || count < 1 {
		t.Errorf("the check cache keeps %d packages of cost %d, over %d", count, cost, cfg.MaxCheckBytes)
	}
	// At most about three of the seven checked packages are kept: the
	// others are checked again when asked for.
	n := w.checks()
	for _, name := range []string{"a", "b", "c", "d", "e", "f"} {
		w.pkg(name)
		if _, cost := w.p.checks.Stats(); cost > cfg.MaxCheckBytes {
			t.Errorf("the check cache costs %d, over %d", cost, cfg.MaxCheckBytes)
		}
	}
	if again := w.checks() - n; again < 3 {
		t.Errorf("%d of a to f checked again, want 3 or more", again)
	}
	// An edit of main needs the summaries of a to f, which are kept.
	w.open("main.gp", files["main.gp"]+"\n// more\n")
	n = w.checks()
	w.pkg(".")
	if w.checks() != n+1 {
		t.Errorf("an edit of main with dropped imports: %d checks, want 1", w.checks()-n)
	}
	if count, cost := w.p.checks.Stats(); cost > cfg.MaxCheckBytes && count > 1 {
		t.Errorf("the check cache keeps %d packages of cost %d, over %d", count, cost, cfg.MaxCheckBytes)
	}
}

// TestImportCycle checks that a cycle doesn't deadlock: the loader reports
// it.
func TestImportCycle(t *testing.T) {
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod": "module m\n",
		"a/a.gp":    "package a\n\nimport \"m/b\"\n\nvar A = b.B\n",
		"b/b.gp":    "package b\n\nimport \"m/a\"\n\nvar B = a.A\n",
	})
	for _, dir := range []string{"a", "b"} {
		done := make(chan string)
		go func() { done <- w.errs(w.pkg(dir)) }()
		select {
		case errs := <-done:
			if !strings.Contains(errs, "import cycle not allowed") {
				t.Errorf("%s: errors:\n%s", dir, errs)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("deadlock")
		}
	}
	// An edit of b breaks the cycle, which a sees: the cut import counts
	// for Clone too.
	w.open("b/b.gp", "package b\n\nvar B = 1\n")
	if errs := w.errs(w.pkg("a")); errs != "" {
		t.Errorf("a after the cycle was broken: errors:\n%s", errs)
	}
}

// TestClone checks that Clone keeps the handles of the packages that an
// edit doesn't touch, and drops those of the edited package and its
// importers.
func TestClone(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	w.pkg(".")
	handles := func() map[pkgID]*handle {
		snap, release := w.s.Snapshot()
		defer release()
		st := snap.State().(*state)
		st.mu.Lock()
		defer st.mu.Unlock()
		m := make(map[pkgID]*handle, st.handles.size())
		st.handles.each(func(k pkgID, v *handle) { m[k] = v })
		return m
	}
	before := handles()
	w.open("geom/geom.gp", app["geom/geom.gp"])
	after := handles()
	for id, h := range before {
		_, kept := after[id]
		touched := id.dir == w.root || id.dir == filepath.Join(w.root, "geom")
		if kept == touched {
			t.Errorf("%s: kept %v", id.dir, kept)
		}
		if kept && after[id] != h {
			t.Errorf("%s: a new handle", id.dir)
		}
	}
}

// TestCXXOnce checks that the pipeline looks for the C++ compiler of the
// namespace probe once (build.FindCXX runs the compiler), not on every
// check of a package with header imports.
func TestCXXOnce(t *testing.T) {
	cfg := testConfig()
	calls := 0
	var mu sync.Mutex
	found := make(chan struct{}) // the first lookup ends when it is closed
	cfg.CXX = func(context.Context) (build.CXX, error) {
		<-found
		mu.Lock()
		calls++
		mu.Unlock()
		return build.CXX{}, errors.New("no C++ compiler in the tests")
	}
	src := "package main\n\nimport \"<vector>\"\n\nfunc main() { var v std.vector[int]; _ = v }\n"
	w := newWorld(t, cfg, map[string]string{"gocpp.mod": "module m\n", "main.gp": src})
	// The first check doesn't wait for the lookup (probe.go): it is
	// provisional, without the error, and stale once the lookup is done.
	if pkg := w.pkg("."); !pkg.Provisional() || w.errs(pkg) != "" {
		t.Errorf("first check: provisional %v, errors:\n%s", pkg.Provisional(), w.errs(pkg))
	}
	close(found)
	w.p.prober.wait()
	if pkg := w.pkg("."); pkg.Provisional() || !strings.Contains(w.errs(pkg), "cannot check the header imports") {
		t.Errorf("after the lookup: provisional %v, errors:\n%s", pkg.Provisional(), w.errs(pkg))
	}
	w.open("main.gp", strings.Replace(src, "_ = v", "_ = &v", 1))
	w.pkg(".")
	w.p.prober.wait()
	if n := w.checks(); n != 3 || calls != 1 {
		t.Errorf("%d checks, %d compiler lookups; want 3 and 1", n, calls)
	}
}

// TestResolution checks that the imports resolve with what the snapshot
// sees, and that a check depends on it through its key: (a) an unsaved file
// in a new folder makes an import resolve; (b) a folder that appears on
// disk is found by the next handle of its importer, which then sees the
// API of the new package; (c) the first text of main again doesn't bring
// back the old check.
func TestResolution(t *testing.T) {
	mainSrc := "package main\n\nimport \"example.com/app/util\"\n\nfunc main() { _ = util.F() }\n"
	files := map[string]string{"gocpp.mod": "module example.com/app\n", "main.gp": mainSrc}
	notIn := "is not in module example.com/app"

	t.Run("overlay", func(t *testing.T) {
		w := newWorld(t, testConfig(), files)
		if errs := w.errs(w.pkg(".")); !strings.Contains(errs, notIn) {
			t.Fatalf("main without util: errors\n%s", errs)
		}
		w.open("util/util.gp", "package util\n\nfunc F() int { return 1 }\n")
		if errs := w.errs(w.pkg(".")); errs != "" {
			t.Errorf("main with the unsaved util: errors\n%s", errs)
		}
		w.edit("util/util.gp", "package util\n\nfunc G() int { return 1 }\n")
		if errs := w.errs(w.pkg(".")); !strings.Contains(errs, "undefined: util.F") {
			t.Errorf("main after util.F went: errors\n%s", errs)
		}
		w.close("util/util.gp")
		if errs := w.errs(w.pkg(".")); !strings.Contains(errs, notIn) {
			t.Errorf("main after util was closed: errors\n%s", errs)
		}
	})

	t.Run("disk", func(t *testing.T) {
		w := newWorld(t, testConfig(), files)
		if errs := w.errs(w.pkg(".")); !strings.Contains(errs, notIn) {
			t.Fatalf("main without util: errors\n%s", errs)
		}
		dir := filepath.Join(w.root, "util")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "util.gp"), []byte("package util\n\nfunc F() int { return 1 }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		w.open("main.gp", strings.Replace(mainSrc, "_ = util.F()", "_ = util.F() + 1", 1))
		if errs := w.errs(w.pkg(".")); errs != "" {
			t.Errorf("main after util was written: errors\n%s", errs)
		}
		// util is in main's key: an edit of its API reaches main.
		w.open("util/util.gp", "package util\n\nfunc G() int { return 1 }\n")
		if errs := w.errs(w.pkg(".")); !strings.Contains(errs, "undefined: util.F") {
			t.Errorf("main after util.F went: errors\n%s", errs)
		}
		w.close("util/util.gp")
		// The first text of main: not the first check, which had no util.
		w.edit("main.gp", mainSrc)
		if errs := w.errs(w.pkg(".")); errs != "" {
			t.Errorf("main with its first text: errors\n%s", errs)
		}
	})
}

// TestOpenSurvivesLRU checks that the checked package of an open document
// stays while its snapshot holds it, whatever the cap of the check cache.
func TestOpenSurvivesLRU(t *testing.T) {
	files := map[string]string{"gocpp.mod": "module example.com/app\n", "main.gp": app["main.gp"]}
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		files[name+"/"+name+".gp"] = "package " + name + "\n\nfunc F() int { return 1 }\n"
	}
	maps.Copy(files, map[string]string{"geom/geom.gp": app["geom/geom.gp"], "shapes/shapes.gp": app["shapes/shapes.gp"]})
	cfg := testConfig()
	cfg.MaxCheckBytes = 1 // only the newest package
	w := newWorld(t, cfg, files)
	w.open("geom/geom.gp", app["geom/geom.gp"])
	geom := w.pkg("geom")
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		w.pkg(name)
	}
	n := w.checks()
	if w.pkg("geom") != geom || w.checks() != n {
		t.Error("the package of the open document was dropped")
	}
	w.close("geom/geom.gp")
	w.pkg("shapes") // the cache now holds only shapes
	if w.pkg("geom"); w.checks() == n {
		t.Error("the package of a closed document stayed over the cap")
	}
}

// TestManyImporters checks the memory cap with 200 small packages that
// import a big one: each check holds the types of the big package (from its
// export data), which the cost counts, so the cache stays under its cap and
// the heap with it.
func TestManyImporters(t *testing.T) {
	if testing.Short() {
		t.Skip("checks 200 packages")
	}
	files := bigPackage()
	for i := range 200 {
		files[fmt.Sprintf("u%03d/u.gp", i)] = fmt.Sprintf("package u%03d\n\nimport \"example.com/bench/big\"\n\nvar X = big.F0_0(%d)\n", i, i)
	}
	cfg := testConfig()
	cfg.MaxCheckBytes = 20 << 20
	w := newWorld(t, cfg, files)
	w.pkg("big")
	before := heapAfterGC()
	for i := range 200 {
		w.pkg(fmt.Sprintf("u%03d", i))
		if _, cost := w.p.checks.Stats(); cost > cfg.MaxCheckBytes {
			t.Fatalf("the check cache costs %d, over %d", cost, cfg.MaxCheckBytes)
		}
	}
	grown := heapAfterGC() - before
	n, cost := w.p.checks.Stats()
	t.Logf("%d packages kept, cost %d MB, heap grew %d MB", n, cost>>20, grown>>20)
	if grown > 2*cfg.MaxCheckBytes {
		t.Errorf("the heap grew %d MB for a cap of %d MB", grown>>20, cfg.MaxCheckBytes>>20)
	}
}

// TestUnreadableFolder checks that a folder that exists but can't be read
// is reported as such at its import, not as missing ("not in module").
func TestUnreadableFolder(t *testing.T) {
	if os.Getuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("needs a folder without permissions")
	}
	w := newWorld(t, testConfig(), map[string]string{
		"gocpp.mod":    "module example.com/app\n",
		"main.gp":      "package main\n\nimport \"example.com/app/util\"\n\nfunc main() { util.F() }\n",
		"util/util.gp": "package util\n\nfunc F() {}\n",
	})
	dir := filepath.Join(w.root, "util")
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	errs := w.errs(w.pkg("."))
	if !strings.Contains(errs, "permission denied") || strings.Contains(errs, "not in module") {
		t.Errorf("errors:\n%s", errs)
	}
}

// TestInvalidate checks Session.Invalidate (didChangeWatchedFiles): a file
// changed on disk is read again only in the snapshot after it, with its
// package and the importers whose API it changed; a deleted folder stands
// for everything in it.
func TestInvalidate(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	if errs := w.errs(w.pkg(".")); errs != "" {
		t.Fatalf("errors:\n%s", errs)
	}
	path := filepath.Join(w.root, "shapes", "shapes.gp")
	if err := os.WriteFile(path, []byte("package shapes\n\ntype Square struct{ Edge int }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if errs := w.errs(w.pkg(".")); errs != "" {
		t.Errorf("before Invalidate, the snapshot reads the disk again:\n%s", errs)
	}
	if err := w.s.Invalidate([]protocol.DocumentURI{w.uri("shapes/shapes.gp")}); err != nil {
		t.Fatal(err)
	}
	if errs := w.errs(w.pkg(".")); !strings.Contains(errs, "Side") {
		t.Errorf("after Invalidate of shapes.gp, errors:\n%s", errs)
	}
	if err := os.RemoveAll(filepath.Join(w.root, "geom")); err != nil {
		t.Fatal(err)
	}
	if err := w.s.Invalidate([]protocol.DocumentURI{w.uri("geom")}); err != nil {
		t.Fatal(err)
	}
	if errs := w.errs(w.pkg(".")); !strings.Contains(errs, "geom") {
		t.Errorf("after Invalidate of the deleted folder geom, errors:\n%s", errs)
	}
	snap, release := w.s.Snapshot()
	defer release()
	if _, err := snap.Package(context.Background(), filepath.Join(w.root, "geom")); err == nil {
		t.Error("the deleted package geom is still there")
	}
}

// TestRecheck checks Session.Recheck: the package of the folder and its
// importers get new handles, and nothing is read again (the files, folders
// and the other handles stay).
func TestRecheck(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	w.pkg(".")
	stateOf := func() *state {
		snap, release := w.s.Snapshot()
		defer release()
		return snap.State().(*state)
	}
	before := stateOf()
	if err := w.s.Recheck([]string{filepath.Join(w.root, "shapes")}); err != nil {
		t.Fatal(err)
	}
	after := stateOf()
	after.mu.Lock()
	defer after.mu.Unlock()
	// What exists stays (what is missing is looked for again in every
	// snapshot, see Clone).
	before.mu.Lock()
	before.files.each(func(path string, f *fileHandle) {
		if got, _ := after.files.get(path); f.err == nil && got != f {
			t.Errorf("the file %s is read again", path)
		}
	})
	before.dirs.each(func(dir string, d *dirHandle) {
		if got, _ := after.dirs.get(dir); len(d.names) > 0 && got != d {
			t.Errorf("the folder %s is read again", dir)
		}
	})
	before.mu.Unlock()
	if after.mods.size() != before.mods.size() {
		t.Errorf("mods %d→%d: want all kept", before.mods.size(), after.mods.size())
	}
	before.handles.each(func(id pkgID, _ *handle) {
		_, kept := after.handles.get(id)
		rechecked := id.dir == w.root || id.dir == filepath.Join(w.root, "geom") || id.dir == filepath.Join(w.root, "shapes")
		if kept == rechecked {
			t.Errorf("%s: kept %v", id.dir, kept)
		}
	})
}

// TestStdlibNames: a name of a file of the standard library, as the loader
// and the pipeline make it (filepath.Join, so $GOCPP\stdlib\fmt\fmt.gp on
// Windows), is in Config.StdlibDir, also when StdlibDir is not clean or is
// the default with its slash: the snapshot reads it from the embedded
// files, never from the disk.
func TestStdlibNames(t *testing.T) {
	for _, dir := range []string{"", "$GOCPP/./stdlib/", filepath.Join(t.TempDir(), "std") + "//"} {
		cfg := testConfig()
		cfg.StdlibDir = dir
		w := newWorld(t, cfg, map[string]string{"gocpp.mod": "module example.com/m\n"})
		if dir == "" {
			dir = "$GOCPP/stdlib"
		}
		name := filepath.Join(dir, "fmt", "fmt.gp")
		snap, release := w.s.Snapshot()
		if !snap.IsStdFile(name) {
			t.Errorf("StdlibDir %q: %q is not a file of the standard library", dir, name)
		}
		src, err := snap.ReadFile(name)
		if want, _ := fs.ReadFile(stdlib.FS(), "fmt/fmt.gp"); err != nil || !bytes.Equal(src, want) {
			t.Errorf("StdlibDir %q: ReadFile(%q): %v, not the embedded file", dir, name, err)
		}
		if _, err := snap.ParsePath(context.Background(), name, ParseFull); err != nil {
			t.Errorf("StdlibDir %q: ParsePath(%q): %v", dir, name, err)
		}
		release()
	}
}

// TestLateImport: a file whose import comes after another declaration (or
// after a syntax error) has the same imports in its metadata as in its check,
// so the check resolves them (no "internal error: the import of fmt was not
// resolved with the metadata"), whatever the history of the file was.
func TestLateImport(t *testing.T) {
	late := "package main\n\ndefer x()\n\nimport \"fmt\"\nimport \"os\"\n\nfunc main() { fmt.Println(os.Args) }\n"
	late2 := "package main\n\nfunc g() {}\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n"
	for name, text := range map[string]string{"after a syntax error": late, "after a declaration": late2} {
		for _, history := range []bool{false, true} {
			w := newWorld(t, testConfig(), map[string]string{
				"gocpp.mod": "module example.com/late\n",
				"main.gp":   "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n",
			})
			if history {
				w.open("main.gp", "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println(1) }\n")
				w.pkg(".")
				w.edit("main.gp", text)
			} else {
				w.open("main.gp", text)
			}
			errs := w.errs(w.pkg("."))
			if strings.Contains(errs, "internal error") || strings.Contains(errs, "undefined: fmt") {
				t.Errorf("%s (history %v): bogus errors:\n%s", name, history, errs)
			}
			if !strings.Contains(errs, "imports must appear before other declarations") && name == "after a declaration" {
				t.Errorf("%s: the real error is missing:\n%s", name, errs)
			}
		}
	}
}

// TestMemoPanic: a function that panics in Package.Memo leaves nothing kept
// (a sync.Once would be spent, and the later callers would get nil): the next
// call computes the value, and it is kept after that.
func TestMemoPanic(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	pkg := w.pkg(".")
	type key struct{}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic of f did not reach the caller")
			}
		}()
		pkg.Memo(key{}, func() any { panic("boom") })
	}()
	n := 0
	for range 3 {
		if got := pkg.Memo(key{}, func() any { n++; return "value" }); got != "value" {
			t.Fatalf("Memo after a panic: %v", got)
		}
	}
	if n != 1 {
		t.Errorf("f ran %d times after the panic, want 1", n)
	}
}

// TestCanceledHandleKeepsNothing: a request whose context is done while the
// handle of a package is made gets the context's error, and leaves nothing
// behind. The key of the metadata used to take the error for a header ("context
// canceled" instead of the file's text), so every later cancelled request for
// the package shared one metadata, made for whatever text the file had then:
// the imports of another version of the file (a bogus "package ... is not in
// module", the importers missing from a search), until the package changed.
func TestCanceledHandleKeepsNothing(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/cc\n",
		"a/a.gp":    "package a\n",
		"b/b.gp":    "package b\n",
		"main.gp":   "package main\n\nimport \"example.com/cc/a\"\n\nfunc main() { _ = a.X }\n",
	}
	w := newWorld(t, testConfig(), files)
	imports := func(ctx context.Context) ([]string, error) {
		snap, release := w.s.Snapshot()
		defer release()
		refs, err := snap.ImportsOf(ctx, w.root)
		var out []string
		for _, r := range refs {
			out = append(out, r.ImportPath)
		}
		return out, err
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	w.open("main.gp", "package main\n\nimport \"example.com/cc/a\"\n\nfunc main() {}\n")
	if got, err := imports(canceled); err == nil {
		t.Errorf("a cancelled request made a handle: %v", got)
	}
	w.edit("main.gp", "package main\n\nimport \"example.com/cc/b\"\n\nfunc main() {}\n")
	if got, err := imports(canceled); err == nil {
		t.Errorf("a cancelled request made a handle: %v", got)
	}
	got, err := imports(context.Background())
	if err != nil || len(got) != 1 || got[0] != "example.com/cc/b" {
		t.Errorf("the imports after the cancelled requests: %v, %v; want [example.com/cc/b]", got, err)
	}
}
