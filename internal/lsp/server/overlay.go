package server

import (
	"fmt"
	"sort"
	"sync"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// An Overlay is a document that is open in the editor: the text of its
// buffer, which may differ from the file on disk (it may not be saved yet).
//
// An Overlay is immutable: an edit makes a new one. So a reader can keep an
// Overlay, and read it from any goroutine, without a lock; the snapshots of
// step 11.2 will hold them.
type Overlay struct {
	uri        protocol.DocumentURI
	languageID protocol.LanguageKind
	version    int32
	mapper     *protocol.Mapper
	// broken: a change failed (Overlays.Change), so the text differs from
	// the editor's. It stays broken until the document is opened again;
	// step 11.4 must not report diagnostics for it (they would be at the
	// wrong places).
	broken bool
}

// URI returns the URI of the document.
func (o *Overlay) URI() protocol.DocumentURI { return o.uri }

// LanguageID returns the language of the document, as the editor named it
// in didOpen.
func (o *Overlay) LanguageID() protocol.LanguageKind { return o.languageID }

// Version returns the editor's version of the text: the version of the
// last didOpen or didChange. (The spec wants it to go up with every
// change; the server keeps whatever the editor sends.)
func (o *Overlay) Version() int32 { return o.version }

// Text returns the text of the document.
func (o *Overlay) Text() string { return o.mapper.Text() }

// Mapper returns the position mapper of the text: its line index.
func (o *Overlay) Mapper() *protocol.Mapper { return o.mapper }

// Broken reports whether a change of the document failed, so that the text
// is not the editor's any more (Overlays.Change).
func (o *Overlay) Broken() bool { return o.broken }

// Overlays is the set of the open documents, by URI. It is safe for
// concurrent use. Its methods replace an Overlay by a new one and never
// change an Overlay that a reader may hold.
type Overlays struct {
	mu    sync.RWMutex
	files map[protocol.DocumentURI]*Overlay
}

// NewOverlays returns an empty set.
func NewOverlays() *Overlays {
	return &Overlays{files: make(map[protocol.DocumentURI]*Overlay)}
}

// Open adds the document of a didOpen. A document that is open already is
// replaced; replaced reports that.
func (s *Overlays) Open(item protocol.TextDocumentItem) (o *Overlay, replaced bool) {
	o = &Overlay{
		uri:        item.URI,
		languageID: item.LanguageID,
		version:    item.Version,
		mapper:     protocol.NewMapper(item.Text),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, replaced = s.files[item.URI]
	s.files[item.URI] = o
	return o, replaced
}

// Change applies the changes of a didChange to the open document uri, in
// order, and gives the result the version. A change with a range replaces
// that range; a change without one replaces the whole text.
//
// It is an error when the document is not open, or a range ends before it
// starts. Then the text doesn't change, but the overlay is marked Broken:
// the editor has applied a change that the server could not, so the two
// texts differ until the document is opened again.
//
// The changes of one document must come one call at a time, in order (the
// server handles notifications in order); different documents may change
// at the same time.
func (s *Overlays) Change(uri protocol.DocumentURI, version int32, changes []protocol.TextDocumentContentChangeEvent) (*Overlay, error) {
	old := s.Get(uri)
	if old == nil {
		return nil, fmt.Errorf("%s is not open", uri)
	}
	// The new text is made without the lock, so readers never wait for
	// it. Each change copies the text and makes a new line index. Measured
	// on a 1 MiB text on an idle machine (2026-10-14): one change about
	// 0.17 ms, 100 changes in one didChange about 16 ms
	// (BenchmarkChange1MiB, BenchmarkChange1MiBx100). Step 11.11 measures
	// it against the budgets; an incremental line index (or a rope) would
	// make it cheaper.
	m, err := applyChanges(old.mapper, changes)
	o := &Overlay{uri: uri, languageID: old.languageID, version: version, mapper: m, broken: old.broken}
	if err != nil {
		err = fmt.Errorf("%s: %v", uri, err)
		o.mapper, o.version, o.broken = old.mapper, old.version, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.files[uri] != old {
		// Another call changed, closed or opened the document meanwhile:
		// the callers broke the rule above.
		return nil, fmt.Errorf("%s changed during the change", uri)
	}
	s.files[uri] = o
	return o, err
}

// applyChanges applies the content changes to the text of m and returns a
// mapper of the new text. Each change is relative to the text after the
// changes before it (LSP 3.17 "DidChangeTextDocumentParams").
func applyChanges(m *protocol.Mapper, changes []protocol.TextDocumentContentChangeEvent) (*protocol.Mapper, error) {
	for i, c := range changes {
		if c.Range == nil {
			m = protocol.NewMapper(c.Text)
			continue
		}
		start, end, err := m.RangeOffsets(*c.Range)
		if err != nil {
			return nil, fmt.Errorf("change %d: %v", i, err)
		}
		text := m.Text()
		m = protocol.NewMapper(text[:start] + c.Text + text[end:])
	}
	return m, nil
}

// Close removes the document of a didClose. It is an error when the
// document is not open.
func (s *Overlays) Close(uri protocol.DocumentURI) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.files[uri]; !ok {
		return fmt.Errorf("%s is not open", uri)
	}
	delete(s.files, uri)
	return nil
}

// Get returns the open document uri, or nil.
func (s *Overlays) Get(uri protocol.DocumentURI) *Overlay {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.files[uri]
}

// All returns the open documents, sorted by URI.
func (s *Overlays) All() []*Overlay {
	s.mu.RLock()
	all := make([]*Overlay, 0, len(s.files))
	for _, o := range s.files {
		all = append(all, o)
	}
	s.mu.RUnlock()
	sort.Slice(all, func(i, j int) bool { return all[i].uri < all[j].uri })
	return all
}
