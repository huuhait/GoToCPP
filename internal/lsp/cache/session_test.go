package cache

import (
	"fmt"
	"maps"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A testState records what the session does with the states.
type testState struct {
	gen     int                           // 0 for the first state, +1 per clone
	changed map[protocol.DocumentURI]bool // the argument of the Clone that made it
	snap    *Snapshot                     // the snapshot Clone made it for
	log     *stateLog
}

type stateLog struct {
	mu       sync.Mutex
	clones   int
	released []int // the gens, in the order of Release
}

func (s *testState) Clone(next *Snapshot, ch Change) State {
	s.log.mu.Lock()
	s.log.clones++
	s.log.mu.Unlock()
	return &testState{gen: s.gen + 1, changed: maps.Clone(ch.Files), snap: next, log: s.log}
}

func (s *testState) Release() {
	s.log.mu.Lock()
	defer s.log.mu.Unlock()
	if slices.Contains(s.log.released, s.gen) {
		panic(fmt.Sprintf("state %d released twice", s.gen))
	}
	s.log.released = append(s.log.released, s.gen)
}

func (l *stateLog) get() (clones int, released []int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.clones, slices.Clone(l.released)
}

func TestSnapshots(t *testing.T) {
	log := &stateLog{}
	s := NewSession(&testState{log: log})
	snap0, release0 := s.Snapshot()
	if snap0.ID() != 0 || len(snap0.Overlays()) != 0 || snap0.State().(*testState).gen != 0 {
		t.Fatalf("the first snapshot: ID %d, %d overlays", snap0.ID(), len(snap0.Overlays()))
	}
	open(t, s, "one")
	if err := s.DidChange(testURI, 2, []protocol.TextDocumentContentChangeEvent{edit(nil, "two")}); err != nil {
		t.Fatal(err)
	}
	snap2, release2 := s.Snapshot()
	if snap2.ID() != 2 || snap2.Overlay(testURI).Text() != "two" {
		t.Errorf("after open and change: ID %d, text %q", snap2.ID(), snap2.Overlay(testURI).Text())
	}
	if st := snap2.State().(*testState); st.gen != 2 || st.snap != snap2 || !maps.Equal(st.changed, map[protocol.DocumentURI]bool{testURI: true}) {
		t.Errorf("the state of snapshot 2: gen %d, snapshot %p (want %p), changed %v", st.gen, st.snap, snap2, st.changed)
	}
	// Snapshot 0 is held: it is still there, and it never changed.
	if snap0.Overlay(testURI) != nil || s.Live() != 2 {
		t.Errorf("snapshot 0 changed, or %d snapshots live (want 2: 0 held, 2 held and current)", s.Live())
	}
	// Snapshot 1 was never held, so it is gone; 0 is not current, so its
	// background work stops.
	if _, released := log.get(); !slices.Equal(released, []int{1}) {
		t.Errorf("released the states %v, want [1]", released)
	}
	if snap0.BackgroundContext().Err() == nil || snap2.BackgroundContext().Err() != nil {
		t.Error("the background contexts: want the old one done, the current one not")
	}
	release0()
	release0() // a second call does nothing
	if _, released := log.get(); !slices.Equal(released, []int{1, 0}) || s.Live() != 1 {
		t.Errorf("after the release of snapshot 0: released %v, %d live", released, s.Live())
	}
	// Snapshot 2 is held by the request and the session.
	if err := s.DidClose(testURI); err != nil {
		t.Fatal(err)
	}
	if snap2.Overlay(testURI).Text() != "two" || s.Live() != 2 {
		t.Errorf("the held snapshot changed after didClose, or %d live", s.Live())
	}
	release2()
	clones, released := log.get()
	if clones != 3 || !slices.Equal(released, []int{1, 0, 2}) || s.Live() != 1 {
		t.Errorf("%d clones, released %v, %d live", clones, released, s.Live())
	}
	snap3, release3 := s.Snapshot()
	defer release3()
	if snap3.ID() != 3 || snap3.Overlay(testURI) != nil {
		t.Errorf("after didClose: ID %d, overlay %v", snap3.ID(), snap3.Overlay(testURI))
	}
}

// TestFailedChanges: a change that fails makes no snapshot, except the one
// of a broken overlay.
func TestFailedChanges(t *testing.T) {
	s := NewSession(nil)
	if s.DidChange(testURI, 1, nil) == nil || s.DidClose(testURI) == nil {
		t.Error("a change or close of a document that is not open: no error")
	}
	open(t, s, "ab")
	bad := []protocol.TextDocumentContentChangeEvent{edit(rng(0, 2, 0, 1), "x")}
	if err := s.DidChange(testURI, 2, bad); err == nil {
		t.Error("a backward range: no error")
	}
	snap, release := s.Snapshot()
	defer release()
	if snap.ID() != 2 || !snap.Overlay(testURI).Broken() {
		t.Errorf("after a failed change: ID %d, broken %v; want 2 and broken", snap.ID(), snap.Overlay(testURI).Broken())
	}
}

func TestShutdown(t *testing.T) {
	s := NewSession(nil)
	open(t, s, "x")
	snap, release := s.Snapshot()
	s.Shutdown()
	s.Shutdown()
	if snap.BackgroundContext().Err() == nil {
		t.Error("the background context is not done after Shutdown")
	}
	if _, err := s.DidOpen(protocol.TextDocumentItem{URI: "file:///w/b.gp"}); err == nil {
		t.Error("didOpen after Shutdown: no error")
	}
	if err := s.DidChange(testURI, 2, nil); err == nil {
		t.Error("didChange after Shutdown: no error")
	}
	release()
	// The last snapshot stays current.
	snap2, release2 := s.Snapshot()
	defer release2()
	if snap2 != snap || snap2.Overlay(testURI).Text() != "x" || s.Live() != 1 {
		t.Errorf("after Shutdown: snapshot %d, %d live", snap2.ID(), s.Live())
	}
}

// TestSnapshotsConcurrent takes and releases snapshots on many goroutines
// while the documents change (run it with -race): a held snapshot never
// changes, and every snapshot but the current one is destroyed at the end.
func TestSnapshotsConcurrent(t *testing.T) {
	log := &stateLog{}
	s := NewSession(&testState{log: log})
	open(t, s, "")
	var wg sync.WaitGroup
	var stop atomic.Bool
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for !stop.Load() {
				snap, release := s.Snapshot()
				o := snap.Overlay(testURI)
				// Snapshot n has the text of n-1 x's (open is 1).
				if n := int(snap.ID()); len(o.Text()) != n-1 || int(o.Version()) != n || snap.State().(*testState).gen != n {
					t.Errorf("snapshot %d: %d bytes, version %d", n, len(o.Text()), o.Version())
				}
				release()
			}
		}()
	}
	const changes = 500
	for v := int32(2); v <= changes+1; v++ {
		if err := s.DidChange(testURI, v, []protocol.TextDocumentContentChangeEvent{edit(rng(0, 0, 0, 0), "x")}); err != nil {
			t.Fatal(err)
		}
	}
	stop.Store(true)
	wg.Wait()
	clones, released := log.get()
	if s.Live() != 1 || clones != changes+1 || len(released) != changes+1 {
		t.Errorf("%d live, %d clones, %d released; want 1, %d, %d", s.Live(), clones, len(released), changes+1, changes+1)
	}
}
