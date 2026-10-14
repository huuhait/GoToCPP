// Package cache is the state of the language server, built like gopls's
// (gopls/internal/cache): a Session owns a sequence of immutable Snapshots,
// and every change to the open documents (didOpen, didChange, didClose)
// makes a new one. A request takes the snapshot that is current when it
// comes and works on it to the end, while newer edits make newer
// snapshots; it never sees a document change under it.
//
// A Snapshot holds the open documents (Overlays) and a State: the derived
// data of the memoized pipeline (step 11.3: parsed files, checked
// packages; Pipeline.NewState), which each new snapshot clones from the
// one before.
package cache

import (
	"errors"
	"fmt"
	"sync"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A Session is the state of one client: the current snapshot, replaced by
// a new one on every change. Its methods are safe for concurrent use, but
// the changes (DidOpen, DidChange, DidClose) must come one at a time, in
// the order of the client's notifications: the server's read loop makes
// them (protocol.Handler).
type Session struct {
	mu      sync.Mutex
	current *Snapshot
	live    int  // the snapshots that are not destroyed yet
	closed  bool // Shutdown was called
}

// NewSession returns a session whose first snapshot (ID 0) has no open
// documents and the state state. state may be nil: then every snapshot's
// State is nil.
func NewSession(state State) *Session {
	s := &Session{live: 1}
	s.current = newSnapshot(s, 0, map[protocol.DocumentURI]*Overlay{}, state)
	return s
}

// Snapshot returns the current snapshot, acquired: it is not destroyed
// until release is called. The caller must call release once, when it is
// done with the snapshot; release may be called from any goroutine, and a
// second call does nothing.
func (s *Session) Snapshot() (snap *Snapshot, release func()) {
	s.mu.Lock()
	snap = s.current
	snap.refs++
	s.mu.Unlock()
	var once sync.Once
	return snap, func() { once.Do(snap.decref) }
}

// Live returns the number of snapshots that are not destroyed yet: the
// current one and the older ones that are still acquired.
func (s *Session) Live() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.live
}

// errShutdown is the error of a change after Shutdown.
var errShutdown = errors.New("the session is shut down")

// DidOpen opens the document of a didOpen in a new snapshot. A document that
// is open already is replaced; replaced reports that.
func (s *Session) DidOpen(item protocol.TextDocumentItem) (replaced bool, err error) {
	snap, release := s.Snapshot()
	defer release()
	_, replaced = snap.overlays[item.URI]
	return replaced, s.update(snap, item.URI, newOverlay(item))
}

// DidChange applies the changes of a didChange to the open document uri in a
// new snapshot (Overlay.change says how). It is an error when the document
// is not open, or a range ends before it starts; in the second case the new
// snapshot has the overlay marked Broken.
func (s *Session) DidChange(uri protocol.DocumentURI, version int32, changes []protocol.TextDocumentContentChangeEvent) error {
	snap, release := s.Snapshot()
	defer release()
	old := snap.overlays[uri]
	if old == nil {
		return fmt.Errorf("%s is not open", uri)
	}
	// The new text is made without the lock, so readers never wait for it.
	o, err := old.change(version, changes)
	if uerr := s.update(snap, uri, o); uerr != nil {
		return uerr
	}
	return err
}

// DidClose removes the document of a didClose in a new snapshot. It is an
// error when the document is not open.
func (s *Session) DidClose(uri protocol.DocumentURI) error {
	snap, release := s.Snapshot()
	defer release()
	if snap.overlays[uri] == nil {
		return fmt.Errorf("%s is not open", uri)
	}
	return s.update(snap, uri, nil)
}

// update makes the snapshot after prev, in which the document uri has the
// overlay o (nil: closed), and makes it current. prev must still be the
// current snapshot.
func (s *Session) update(prev *Snapshot, uri protocol.DocumentURI, o *Overlay) error {
	// Copy on write: the map of the new snapshot is a copy (a few dozen
	// open documents cost about a microsecond to copy; a persistent map
	// like gopls's would make it O(log n) if that ever matters).
	overlays := make(map[protocol.DocumentURI]*Overlay, len(prev.overlays)+1)
	for k, v := range prev.overlays {
		overlays[k] = v
	}
	if o == nil {
		delete(overlays, uri)
	} else {
		overlays[uri] = o
	}
	// The next snapshot is made first, so that its state can know it.
	next := newSnapshot(s, prev.id+1, overlays, nil)
	if prev.state != nil {
		next.state = prev.state.Clone(next, map[protocol.DocumentURI]bool{uri: true})
	}
	s.mu.Lock()
	var err error
	switch {
	case s.closed:
		err = errShutdown
	case s.current != prev:
		// Another change came meanwhile: the callers broke the rule of
		// one change at a time.
		err = fmt.Errorf("%s: the snapshot changed during the change", uri)
	}
	if err != nil {
		s.mu.Unlock()
		next.cancel()
		if next.state != nil {
			next.state.Release()
		}
		return err
	}
	s.current = next
	s.live++
	s.mu.Unlock()
	prev.cancel() // work for prev in the background stops
	prev.decref() // the session's reference
	return nil
}

// Shutdown ends the session: the background context of the current
// snapshot is cancelled, so no background work goes on, and later changes
// are errors. The current snapshot stays current, so Snapshot still works
// (a request that comes after Shutdown gets the last snapshot); so it is
// never destroyed, and its State's Release is not called (the process
// ends after shutdown anyway).
func (s *Session) Shutdown() {
	s.mu.Lock()
	s.closed = true
	cur := s.current
	s.mu.Unlock()
	cur.cancel()
}

// A State is the derived data of one snapshot: what step 11.3's memoized
// pipeline keeps per file and per package (parsed files, checked packages,
// their diagnostics). The session gives each new snapshot the state that
// Clone returns from the state of the snapshot before.
//
// A State belongs to its snapshot: what it has computed must depend only
// on the snapshot's inputs (its overlays and, from step 11.3, the files on
// disk), so that the result of a request is the same whenever it runs. It
// may compute lazily, behind its own lock, since requests on one snapshot
// run at the same time. The content-hash caches that outlive snapshots
// (parsed files by the hash of their text) are shared by the states, not
// owned by one.
type State interface {
	// Clone returns the state of next, the snapshot after the receiver's;
	// changed holds the documents that were opened, edited or closed.
	// next is complete but for its State: a lazy state may keep it, to
	// read its overlays when it computes. Clone keeps what does not
	// depend on the changed documents and must not change the receiver,
	// which requests may still use. It runs on the read loop, so it must
	// be quick: copy on write, compute later.
	Clone(next *Snapshot, changed map[protocol.DocumentURI]bool) State

	// Release is called once, when the snapshot is destroyed: it is not
	// current any more and no request holds it. The state lets go of
	// what only it holds (step 11.3's memory cap counts on it).
	Release()
}
