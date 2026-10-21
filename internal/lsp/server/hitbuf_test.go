package server

import (
	"context"
	"go/ast"
	"testing"
)

// filledHits returns a list of hits that hold pointers.
func filledHits(room, n int) []hit {
	list := takeHits(room)
	for i := 0; i < n; i++ {
		list = append(list, hit{path: "a.gp", off: i, id: &ast.Ident{Name: "x"}, v: &view{}})
	}
	return list
}

// TestRecycleHitsEmptiesTheResult: the result has no list after it gave the list back.
func TestRecycleHitsEmptiesTheResult(t *testing.T) {
	list := filledHits(100, 40)
	res := &result{hits: list[:30], hitBuf: list}
	res.recycleHits()
	if res.hits != nil || res.hitBuf != nil {
		t.Fatal("the result still has the list after recycleHits")
	}
}

// TestRecycledHitsAreCleared: a list that went back holds no pointers, whatever the next search
// gets from the pool.
func TestRecycledHitsAreCleared(t *testing.T) {
	list := filledHits(100, 40)
	(&result{hits: list[:30], hitBuf: list}).recycleHits()
	for i := 0; i < 8; i++ {
		got := takeHits(50)
		for j, h := range got[:cap(got)] {
			if h != (hit{}) {
				t.Fatalf("hit %d of a pooled list is not zero: %+v", j, h)
			}
		}
	}
}

// TestTakenHitsAreEmptyAndRoomy: the list that takeHits gives is empty and has the room asked for.
func TestTakenHitsAreEmptyAndRoomy(t *testing.T) {
	list := filledHits(100, 40)
	(&result{hitBuf: list}).recycleHits()
	for i := 0; i < 8; i++ {
		got := takeHits(50)
		if len(got) != 0 || cap(got) < 50 {
			t.Fatalf("takeHits(50): len %d, cap %d", len(got), cap(got))
		}
	}
}

// TestHugeHitListsAreNotKept: a list over maxPooledHits is left to the collector.
func TestHugeHitListsAreNotKept(t *testing.T) {
	big := make([]hit, 0, maxPooledHits+1)
	(&result{hitBuf: big}).recycleHits()
	for i := 0; i < 8; i++ {
		if got := takeHits(1); cap(got) > maxPooledHits {
			t.Fatalf("takeHits returned the huge list (cap %d)", cap(got))
		}
	}
}

// TestHitCapacityAsked: a list for more hits than the pool's lists hold is a new one.
func TestHitCapacityAsked(t *testing.T) {
	(&result{hitBuf: make([]hit, 0, 4)}).recycleHits()
	if got := takeHits(1000); cap(got) < 1000 {
		t.Fatalf("takeHits(1000) has cap %d", cap(got))
	}
}

// TestRecycledHitsAreReused: the next search gets the list that the last one gave back, so a
// popular name's list is made once (no other test makes a list this long).
func TestRecycledHitsAreReused(t *testing.T) {
	const size = 12345
	first := takeHits(size)
	first = append(first, hit{path: "a.gp"})
	(&result{hitBuf: first}).recycleHits()
	again := takeHits(size)
	if cap(again) != size || &again[:1][0] != &first[:1][0] {
		t.Fatalf("takeHits(%d) made a new list (cap %d)", size, cap(again))
	}
}

// TestReferencesRecyclesItsHitList: a references request takes its list from the pool (the first
// list with room: A) and gives the list that it searched in back (A again, at the end). The pool
// is seeded with two lists, A then B, so after the request it holds B then A. A request that
// makes its own list, or does not give it back, or gives back a list other than the one that
// the search made, leaves another order.
func TestReferencesRecyclesItsHitList(t *testing.T) {
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
	c.references("lib/lib.gp", "func |Count", true)
	spareHits.Lock()
	got := append([][]hit(nil), spareHits.lists...)
	spareHits.Unlock()
	if len(got) != 2 || cap(got[0]) != 15000 || cap(got[1]) != 16000 {
		t.Fatalf("the pool after the request has %d lists, want B (15000) then A (16000)", len(got))
	}
	if &got[0][:1][0] != &listB[:1][0] || &got[1][:1][0] != &listA[:1][0] {
		t.Fatal("the lists of the pool are not the seeded ones")
	}
}

// TestViewOfIsMadeOncePerPackage: every request that searches a package gets the view that the
// first one made (a view is the files and the check of a package, and nothing changes it).
func TestViewOfIsMadeOncePerPackage(t *testing.T) {
	rn := bindingProofRenamer(t)
	pkg := rn.sym.q.view().pkg
	first := viewOf(pkg)
	if first == nil {
		t.Fatal("no view of a checked package")
	}
	if again := viewOf(pkg); again != first {
		t.Fatal("a second viewOf made another view")
	}
}

// TestViewOfHoldsWhatAFreshViewHolds: the shared view has the types, the info and the files that a
// view made for the request would have.
func TestViewOfHoldsWhatAFreshViewHolds(t *testing.T) {
	rn := bindingProofRenamer(t)
	pkg := rn.sym.q.view().pkg
	first := viewOf(pkg)
	fresh := makeView(pkg)
	if first.dir != fresh.dir || first.types != fresh.types || first.info != fresh.info {
		t.Fatalf("the view differs from a new one: %q %v %v", first.dir, first.types, first.info)
	}
	if len(first.files) == 0 || len(first.files) != len(fresh.files) {
		t.Fatalf("the view has %d files, a new one %d", len(first.files), len(fresh.files))
	}
	for path, file := range fresh.files {
		if first.files[path] != file {
			t.Fatalf("the file %s of the view is not the package's", path)
		}
	}
}

// TestViewOfPatchedPackageIsItsOwn: a package made for patched text (a rename's check) has its
// own view, not the one of the package it was patched from.
func TestViewOfPatchedPackageIsItsOwn(t *testing.T) {
	rn := bindingProofRenamer(t)
	normal := rn.sym.q.view()
	src, err := rn.e.snapshot.ReadFile(rn.sym.q.file.Path)
	if err != nil {
		t.Fatal(err)
	}
	text := append(append([]byte(nil), src...), []byte("\n// patched input\n")...)
	variant, err := rn.e.snapshot.PackageWithFile(context.Background(), rn.sym.q.file.Path, text)
	if err != nil {
		t.Fatal(err)
	}
	if v := viewOf(variant); v == nil || v == viewOf(normal.pkg) || v.pkg != variant {
		t.Fatal("the patched package shares the view of the ordinary one")
	}
}
