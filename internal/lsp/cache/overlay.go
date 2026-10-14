package cache

import (
	"fmt"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// An Overlay is a document that is open in the editor: the text of its
// buffer, which may differ from the file on disk (it may not be saved yet).
//
// An Overlay is immutable: an edit makes a new one. So a reader can keep an
// Overlay, and read it from any goroutine, without a lock; the snapshots
// hold them.
type Overlay struct {
	uri        protocol.DocumentURI
	languageID protocol.LanguageKind
	version    int32
	mapper     *protocol.Mapper
	// broken: a change failed (Overlay.change), so the text differs from
	// the editor's. It stays broken until the document is opened again;
	// step 11.4 must not report diagnostics for it (they would be at the
	// wrong places).
	broken bool
}

// newOverlay returns the overlay of the document of a didOpen.
func newOverlay(item protocol.TextDocumentItem) *Overlay {
	return &Overlay{
		uri:        item.URI,
		languageID: item.LanguageID,
		version:    item.Version,
		mapper:     protocol.NewMapper(item.Text),
	}
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
// is not the editor's any more (Session.DidChange).
func (o *Overlay) Broken() bool { return o.broken }

// change returns the overlay after the changes of a didChange, applied in
// order, with the version. A change with a range replaces that range; a
// change without one replaces the whole text.
//
// It is an error when a range ends before it starts. Then the result has
// the old text and version, but it is marked Broken: the editor has applied
// a change that the server could not, so the two texts differ until the
// document is opened again.
func (o *Overlay) change(version int32, changes []protocol.TextDocumentContentChangeEvent) (*Overlay, error) {
	// Each change copies the text and makes a new line index. Measured on
	// a 1 MiB text on an idle machine (2026-10-14): one change about
	// 0.17 ms, 100 changes in one didChange about 16 ms
	// (BenchmarkChange1MiB, BenchmarkChange1MiBx100). Step 11.11 measures
	// it against the budgets; an incremental line index (or a rope) would
	// make it cheaper.
	m, err := applyChanges(o.mapper, changes)
	n := &Overlay{uri: o.uri, languageID: o.languageID, version: version, mapper: m, broken: o.broken}
	if err != nil {
		err = fmt.Errorf("%s: %v", o.uri, err)
		n.mapper, n.version, n.broken = o.mapper, o.version, true
	}
	return n, err
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
