package cache

import (
	"fmt"
	"math/rand"
	"testing"
)

// goneByIndex is how Clone found the handles to drop before cloneHandles: an index of importers
// and a walk from the stale handles.
func goneByIndex(handles []*handle, stale func(*handle) bool) map[pkgID]bool {
	importers := make(map[pkgID][]pkgID)
	var seeds []pkgID
	for _, h := range handles {
		for _, dep := range h.deps {
			importers[dep.id] = append(importers[dep.id], h.id)
		}
		for _, dep := range h.cut {
			importers[dep] = append(importers[dep], h.id)
		}
		if stale(h) {
			seeds = append(seeds, h.id)
		}
	}
	gone := make(map[pkgID]bool)
	for len(seeds) > 0 {
		id := seeds[len(seeds)-1]
		seeds = seeds[:len(seeds)-1]
		if !gone[id] {
			gone[id] = true
			seeds = append(seeds, importers[id]...)
		}
	}
	return gone
}

// graphState returns a state whose handles are n packages p0 ... p(n-1), stored in order, each
// importing some earlier ones; with cycles, some also have a cut import of a later package.
func graphState(rng *rand.Rand, n int, cycles bool) (*state, []*handle) {
	s := &state{files: &layered[string, *fileHandle]{}, handles: &layered[pkgID, *handle]{}}
	var all []*handle
	for i := 0; i < n; i++ {
		h := &handle{id: pkgID{dir: fmt.Sprintf("/m/p%d", i)}}
		for j := 0; j < i; j++ {
			if rng.Intn(6) == 0 {
				h.deps = append(h.deps, all[j])
			}
		}
		if cycles && i < n-1 && rng.Intn(5) == 0 {
			h.cut = append(h.cut, pkgID{dir: fmt.Sprintf("/m/p%d", i+1+rng.Intn(n-i-1))})
		}
		s.storeHandle(h.id, h)
		all = append(all, h)
	}
	return s, all
}

func TestCloneHandlesFindsTheImportersOfStaleHandles(t *testing.T) {
	for _, cycles := range []bool{false, true} {
		for seed := int64(0); seed < 200; seed++ {
			rng := rand.New(rand.NewSource(seed))
			n := 2 + rng.Intn(40)
			s, all := graphState(rng, n, cycles)
			staleIDs := map[pkgID]bool{}
			for i := 0; i < 1+rng.Intn(3); i++ {
				staleIDs[all[rng.Intn(n)].id] = true
			}
			stale := func(h *handle) bool { return staleIDs[h.id] }

			want := goneByIndex(all, stale)
			handles, order := s.cloneHandles(stale)

			for _, h := range all {
				_, kept := handles.get(h.id)
				if kept == want[h.id] {
					t.Fatalf("cycles %v seed %d: %s kept %v, but gone by the index %v", cycles, seed, h.id.dir, kept, want[h.id])
				}
			}
			if handles.size() != n-len(want) || len(order) != n-len(want) {
				t.Fatalf("cycles %v seed %d: %d handles and %d in order, want %d", cycles, seed, handles.size(), len(order), n-len(want))
			}
		}
	}
}

// What the clone keeps stays in the order it was stored in, which the next clone relies on.
func TestCloneHandlesKeepsTheOrder(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	s, all := graphState(rng, 30, false)
	victim := all[10].id
	_, order := s.cloneHandles(func(h *handle) bool { return h.id == victim })
	last := -1
	for _, h := range order {
		var index int
		fmt.Sscanf(h.id.dir, "/m/p%d", &index)
		if index <= last {
			t.Fatalf("%s comes after p%d", h.id.dir, last)
		}
		last = index
	}
}

// A package that is stored again after it was dropped is last in the order and is dropped with
// the next change of what it imports.
func TestStoreHandleKeepsFirstAndOrders(t *testing.T) {
	s := &state{handles: &layered[pkgID, *handle]{}}
	first := &handle{id: pkgID{dir: "/m/a"}}
	second := &handle{id: pkgID{dir: "/m/a"}}
	if got := s.storeHandle(first.id, first); got != first {
		t.Fatal("the first handle was not stored")
	}
	if got := s.storeHandle(second.id, second); got != first {
		t.Fatal("a second handle replaced the first")
	}
	if len(s.order) != 1 {
		t.Fatalf("%d handles in order, want 1", len(s.order))
	}
}
