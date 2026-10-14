package cache

import (
	"context"
	"sort"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// A Snapshot is the state of the workspace at one moment: the open
// documents and the derived data (State). It never changes, so any number
// of goroutines may read it without a lock.
//
// A snapshot lives while it is current or acquired (Session.Snapshot; like
// gopls, by reference counting). When a newer snapshot replaces it, its
// BackgroundContext is cancelled, so background work on it (diagnostics,
// step 11.4) stops; a request that holds it goes on and answers from it.
// When it is neither current nor acquired, it is destroyed: its State is
// released. Using a destroyed snapshot is a bug.
type Snapshot struct {
	session  *Session
	id       uint64
	overlays map[protocol.DocumentURI]*Overlay // never changes
	state    State
	ctx      context.Context
	cancel   context.CancelFunc
	refs     int // under session.mu; the session's own reference counts
}

func newSnapshot(s *Session, id uint64, overlays map[protocol.DocumentURI]*Overlay, state State) *Snapshot {
	ctx, cancel := context.WithCancel(context.Background())
	return &Snapshot{session: s, id: id, overlays: overlays, state: state, ctx: ctx, cancel: cancel, refs: 1}
}

// decref drops a reference and destroys the snapshot after the last one.
func (s *Snapshot) decref() {
	s.session.mu.Lock()
	s.refs--
	if s.refs < 0 {
		s.session.mu.Unlock()
		panic("cache: a snapshot was released too often")
	}
	dead := s.refs == 0
	if dead {
		s.session.live--
	}
	s.session.mu.Unlock()
	if dead {
		s.cancel()
		if s.state != nil {
			s.state.Release()
		}
	}
}

// ID returns the sequence number of the snapshot: 0 for the first one of
// its session, and one more for each change after it. A newer snapshot has
// a greater ID.
func (s *Snapshot) ID() uint64 { return s.id }

// Overlay returns the open document uri, or nil.
func (s *Snapshot) Overlay(uri protocol.DocumentURI) *Overlay { return s.overlays[uri] }

// Overlays returns the open documents, sorted by URI.
func (s *Snapshot) Overlays() []*Overlay {
	all := make([]*Overlay, 0, len(s.overlays))
	for _, o := range s.overlays {
		all = append(all, o)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].uri < all[j].uri })
	return all
}

// State returns the derived data of the snapshot (nil when the session has
// none).
func (s *Snapshot) State() State { return s.state }

// BackgroundContext returns a context that is done when a newer snapshot
// replaces this one, the session shuts down, or the snapshot is destroyed:
// the context of the work that the snapshot starts on its own (background
// diagnostics), which is useless once the snapshot is old. A request uses
// its own context instead: it finishes on its snapshot.
func (s *Snapshot) BackgroundContext() context.Context { return s.ctx }
