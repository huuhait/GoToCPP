package cache

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

// TestVersionsKept: the cache of checks keeps the last keptVersions checks
// of a package, however often it is edited; an edit and its undo (two
// texts that alternate) find their checks in it.
func TestVersionsKept(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	src := app["shapes/shapes.gp"]
	w.pkg("shapes")
	w.open("shapes/shapes.gp", src)
	entries := func() int { n, _ := w.p.checks.Stats(); return n }
	base := entries()
	for i := range 12 {
		w.edit("shapes/shapes.gp", src+fmt.Sprintf("\nvar V%d = %d\n", i, i))
		w.pkg("shapes")
	}
	if got := entries(); got > base+keptVersions {
		t.Errorf("%d checks in the cache after 12 edits of one package (%d before), want at most %d more", got, base, keptVersions)
	}
	// Two texts that alternate: after the first round, no check is made.
	a, b := src+"\nvar A = 1\n", src+"\nvar B = 1\n"
	for _, text := range []string{a, b, a, b} {
		w.edit("shapes/shapes.gp", text)
		w.pkg("shapes")
	}
	before := w.checks()
	for _, text := range []string{a, b, a, b, a} {
		w.edit("shapes/shapes.gp", text)
		w.pkg("shapes")
	}
	if n := w.checks() - before; n != 0 {
		t.Errorf("%d checks for two texts that alternate, want 0", n)
	}
}

// Imported API changes invalidate the decoded universe even when the importer
// source is unchanged. A revert restores answers; a new importer text then
// exercises decoding instead of merely hitting a previous package check.
func TestImportSetAPIChangeAndRevert(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	base := app["geom/geom.gp"]
	w.open("geom/geom.gp", base)
	first := w.pkg("geom")
	if errs := w.errs(first); errs != "" {
		t.Fatal(errs)
	}
	id := pkgID{dir: filepath.Join(w.root, "geom")}
	set := w.p.importSets.sets[id]
	if set == nil {
		t.Fatal("no decoded imports")
	}
	w.open("shapes/shapes.gp", "package shapes\n\ntype Square struct{ Edge int }\n")
	changed := w.pkg("geom")
	if errs := w.errs(changed); !strings.Contains(errs, "Side") {
		t.Fatalf("unchanged importer missed imported API change: %s", errs)
	}
	newSet := w.p.importSets.sets[id]
	if newSet == set {
		t.Fatal("reused import set after imported API change")
	}
	w.edit("shapes/shapes.gp", app["shapes/shapes.gp"])
	if errs := w.errs(w.pkg("geom")); errs != "" {
		t.Fatalf("revert retained stale error: %s", errs)
	}
	w.edit("geom/geom.gp", base+"\n// unique text after the revert\n")
	if errs := w.errs(w.pkg("geom")); errs != "" {
		t.Fatalf("new check after revert used stale imports: %s", errs)
	}
	if w.p.importSets.sets[id] == newSet {
		t.Fatal("reused changed imports after revert")
	}
	// An old snapshot's imported objects must not have been mutated.
	if errs := w.errs(first); errs != "" {
		t.Fatalf("old package changed: %s", errs)
	}
}

func TestImportSetDependencyIdentity(t *testing.T) {
	var sets importSets
	id := pkgID{dir: "/p"}
	a, b := new(summary), new(summary)
	set := &importSet{deps: []*summary{a, nil}}
	sets.put(id, set)
	if sets.get(id, []*summary{a, nil}) != set {
		t.Fatal("missed identical summaries")
	}
	for _, deps := range [][]*summary{{b, nil}, {a}, {nil, a}, {a, b}} {
		if sets.get(id, deps) != nil {
			t.Fatal("reused a different dependency universe")
		}
	}
	if sets.get(pkgID{dir: "/q"}, []*summary{a, nil}) != nil {
		t.Fatal("shared a universe across packages")
	}
}

func TestVersionKeeperNote(t *testing.T) {
	p := NewPipeline(testConfig())
	var v versionKeeper
	id := pkgID{dir: "/x"}
	for i := range 10 {
		v.note(p, id, Hash{byte(i)})
	}
	if got := len(v.keys[id]); got != keptVersions {
		t.Errorf("%d keys kept, want %d", got, keptVersions)
	}
	// A key seen again moves to the end and does not push another out.
	last := append([]Hash(nil), v.keys[id]...)
	v.note(p, id, last[0])
	if v.keys[id][keptVersions-1] != last[0] || len(v.keys[id]) != keptVersions {
		t.Errorf("keys %v after seeing %v again", v.keys[id], last[0])
	}
}

// TestImportSetPositions: the second check of a package takes the decoded
// imports of the first (importset.go), and the positions of their objects
// still read, in the new file set of the check.
func TestImportSetPositions(t *testing.T) {
	w := newWorld(t, testConfig(), app)
	src := app["geom/geom.gp"]
	w.open("geom/geom.gp", src)
	first := w.pkg("geom")
	w.edit("geom/geom.gp", src+"\nvar Extra = 1\n")
	second := w.pkg("geom")
	if first == second {
		t.Fatal("the edit made no new check")
	}
	if first.Check.Types.Imports()[0] != second.Check.Types.Imports()[0] {
		t.Fatal("the body-only edit did not reuse decoded imports")
	}
	if len(w.p.importSets.sets) == 0 {
		t.Fatal("no import set kept")
	}
	for i, pkg := range []*Package{first, second} {
		found := 0
		for _, imp := range pkg.Check.Types.Imports() {
			for _, name := range imp.Scope().Names() {
				pos := pkg.Fset().Position(imp.Scope().Lookup(name).Pos())
				// (Without the files of the decode, the positions of the imports fall into the
				// first file of the package: a name, a line, the wrong file.)
				if pos.Filename == "" || pos.Line == 0 || strings.HasSuffix(imp.Path(), "shapes") && !strings.Contains(pos.Filename, "shapes") {
					t.Errorf("check %d: %s.%s is at %v", i, imp.Path(), name, pos)
				}
				found++
			}
		}
		if found == 0 {
			t.Errorf("check %d: no imported object", i)
		}
	}
}
