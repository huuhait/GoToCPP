package cache

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"unicode/utf16"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

const testURI = protocol.DocumentURI("file:///w/main.gp")

// rng returns the range from l1:c1 to l2:c2.
func rng(l1, c1, l2, c2 uint32) *protocol.Range {
	return &protocol.Range{Start: protocol.Position{Line: l1, Character: c1}, End: protocol.Position{Line: l2, Character: c2}}
}

// edit returns a change that replaces the range r with text; a nil r
// replaces the whole text.
func edit(r *protocol.Range, text string) protocol.TextDocumentContentChangeEvent {
	return protocol.TextDocumentContentChangeEvent{Range: r, Text: text}
}

func open(t testing.TB, s *Session, text string) {
	t.Helper()
	if replaced, err := s.DidOpen(protocol.TextDocumentItem{URI: testURI, LanguageID: "gocpp", Version: 1, Text: text}); replaced || err != nil {
		t.Fatalf("the document was open already (%v)", err)
	}
}

// get returns the document uri of the current snapshot of s.
func get(s *Session, uri protocol.DocumentURI) *Overlay {
	snap, release := s.Snapshot()
	defer release()
	return snap.Overlay(uri)
}

// change applies the changes to the document uri and returns the document
// of the new snapshot.
func change(s *Session, uri protocol.DocumentURI, version int32, changes []protocol.TextDocumentContentChangeEvent) (*Overlay, error) {
	err := s.DidChange(uri, version, changes)
	return get(s, uri), err
}

// all returns the documents of the current snapshot of s.
func all(s *Session) []*Overlay {
	snap, release := s.Snapshot()
	defer release()
	return snap.Overlays()
}

func TestChange(t *testing.T) {
	for _, test := range []struct {
		name    string
		text    string
		changes []protocol.TextDocumentContentChangeEvent
		want    string
	}{
		{"insert", "hello\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 5, 0, 5), ", world")}, "hello, world\n"},
		{"replace", "a := 1\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 5, 0, 6), "42")}, "a := 42\n"},
		{"delete across lines", "one\ntwo\nthree\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 1, 2, 2), "")}, "oree\n"},
		{"join lines", "one\ntwo\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 3, 1, 0), " ")}, "one two\n"},
		{"at a line end", "ab\ncd", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 2, 0, 2), "X")}, "abX\ncd"},
		{"past a line end", "ab\ncd", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 10, 0, 20), "X")}, "abX\ncd"},
		{"past the end", "ab\ncd", []protocol.TextDocumentContentChangeEvent{edit(rng(5, 0, 9, 9), "!")}, "ab\ncd!"},
		{"the whole text", "ab\ncd\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 0, 999, 0), "new")}, "new"},
		{"at the end", "ab\n", []protocol.TextDocumentContentChangeEvent{edit(rng(1, 0, 1, 0), "c")}, "ab\nc"},
		{"empty text", "", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 0, 0, 0), "x\n")}, "x\n"},
		{"crlf line end", "ab\r\ncd\r\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 2, 0, 9), "!")}, "ab!\r\ncd\r\n"},
		{"crlf delete line end", "ab\r\ncd\r\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 2, 1, 0), "")}, "abcd\r\n"},
		{"crlf second line", "ab\r\ncd\r\nef", []protocol.TextDocumentContentChangeEvent{edit(rng(2, 1, 2, 2), "X")}, "ab\r\ncd\r\neX"},
		{"lone cr", "ab\rcd", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 3, 0, 4), "X")}, "ab\rXd"},
		{"lone cr is not a line end", "ab\rcd", []protocol.TextDocumentContentChangeEvent{edit(rng(1, 0, 1, 0), "X")}, "ab\rcdX"},
		{"cr before crlf", "a\r\r\nb", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 9, 0, 9), "X")}, "a\rX\r\nb"},
		{"emoji after", "😀x", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 2, 0, 3), "y")}, "😀y"},
		{"emoji delete", "a😀b", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 1, 0, 3), "")}, "ab"},
		{"emoji half", "a😀b", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 2, 0, 2), "X")}, "aX😀b"},
		{"emoji second line", "😀😀\n🎉x😀", []protocol.TextDocumentContentChangeEvent{edit(rng(1, 2, 1, 3), "é")}, "😀😀\n🎉é😀"},
		{"multi-byte", "é中x", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 1, 0, 2), "zz")}, "ézzx"},
		{"insert emoji", "ab", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 1, 0, 1), "🎉")}, "a🎉b"},
		{"several changes", "abc\ndef\n", []protocol.TextDocumentContentChangeEvent{
			edit(rng(1, 0, 1, 3), "XYZ"),
			edit(rng(0, 0, 0, 1), "12"), // on the text after the first change
			edit(rng(0, 4, 1, 1), ""),
		}, "12bcYZ\n"},
		{"full then incremental", "old", []protocol.TextDocumentContentChangeEvent{
			edit(nil, "new\ntext"),
			edit(rng(1, 0, 1, 1), "T"),
		}, "new\nText"},
		{"incremental then full", "old", []protocol.TextDocumentContentChangeEvent{
			edit(rng(0, 0, 0, 1), "x"),
			edit(nil, "all new"),
		}, "all new"},
		{"no changes", "same", nil, "same"},
		{"backward range clamped", "ab\n", []protocol.TextDocumentContentChangeEvent{edit(rng(0, 9, 0, 5), "!")}, "ab!\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := NewSession(nil)
			open(t, s, test.text)
			o, err := change(s, testURI, 2, test.changes)
			if err != nil {
				t.Fatal(err)
			}
			if o.Text() != test.want || get(s, testURI) != o {
				t.Errorf("the text is %q, want %q", o.Text(), test.want)
			}
			if o.Version() != 2 || o.LanguageID() != "gocpp" || o.URI() != testURI {
				t.Errorf("the overlay is %s %s version %d", o.URI(), o.LanguageID(), o.Version())
			}
		})
	}
}

func TestChangeErrors(t *testing.T) {
	s := NewSession(nil)
	if _, err := change(s, testURI, 2, []protocol.TextDocumentContentChangeEvent{edit(nil, "x")}); err == nil {
		t.Error("a change to a document that is not open: no error")
	}
	open(t, s, "ab\ncd\n")
	before := get(s, testURI)
	// The second change has a range that ends before it starts: nothing
	// changes, not even the first change.
	_, err := change(s, testURI, 2, []protocol.TextDocumentContentChangeEvent{
		edit(rng(0, 0, 0, 1), "X"),
		edit(rng(1, 1, 0, 1), "Y"),
	})
	if err == nil || !strings.Contains(err.Error(), "change 1: the range 1:1-0:1 ends before it starts") {
		t.Errorf("err = %v", err)
	}
	o := get(s, testURI)
	if o.Text() != "ab\ncd\n" || o.Version() != 1 || !o.Broken() {
		t.Errorf("after a failed change: %q, version %d, broken %v; want the old text and version, broken",
			o.Text(), o.Version(), o.Broken())
	}
	if before.Broken() || before.Text() != "ab\ncd\n" {
		t.Error("a failed change changed the overlay that a reader holds")
	}
	// It stays broken after a change that works, until it is opened again.
	if o, err := change(s, testURI, 3, []protocol.TextDocumentContentChangeEvent{edit(nil, "x")}); err != nil || !o.Broken() || o.Text() != "x" {
		t.Errorf("a change of a broken document: %v, broken %v", err, o.Broken())
	}
	s.DidOpen(protocol.TextDocumentItem{URI: testURI, Version: 4, Text: "y"})
	if get(s, testURI).Broken() {
		t.Error("broken after didOpen")
	}
}

func TestOpenClose(t *testing.T) {
	s := NewSession(nil)
	open(t, s, "one")
	held := get(s, testURI)
	if replaced, _ := s.DidOpen(protocol.TextDocumentItem{URI: testURI, Version: 7, Text: "two"}); !replaced {
		t.Error("opening an open document again: not replaced")
	}
	if o := get(s, testURI); o.Text() != "two" || o.Version() != 7 {
		t.Errorf("after the second open: %q version %d", o.Text(), o.Version())
	}
	if held.Text() != "one" {
		t.Errorf("an overlay that a reader holds changed to %q", held.Text())
	}
	other := protocol.DocumentURI("file:///w/a.gp")
	s.DidOpen(protocol.TextDocumentItem{URI: other, Text: "a"})
	if all := all(s); len(all) != 2 || all[0].URI() != other || all[1].URI() != testURI {
		t.Errorf("All = %v", all)
	}
	if err := s.DidClose(testURI); err != nil {
		t.Fatal(err)
	}
	if get(s, testURI) != nil || len(all(s)) != 1 {
		t.Error("the closed document is still there")
	}
	if err := s.DidClose(testURI); err == nil {
		t.Error("closing a document that is not open: no error")
	}
}

// A refDoc is the reference implementation of the edits: the text as an
// array of UTF-16 units, edited by slicing. It has the position rules of
// protocol.Mapper, written again in the simplest way.
type refDoc struct {
	units []uint16
}

func (d *refDoc) text() string { return string(utf16.Decode(d.units)) }

// index returns the index in units of the position p.
func (d *refDoc) index(p protocol.Position) int {
	line, start := 0, 0
	for i := 0; i <= len(d.units); i++ {
		end := i == len(d.units)
		if !end && d.units[i] != '\n' {
			continue
		}
		if line == int(p.Line) {
			lineEnd := i
			if !end && lineEnd > start && d.units[lineEnd-1] == '\r' {
				lineEnd-- // \r\n
			}
			j := min(start+int(p.Character), lineEnd)
			// Not between the two halves of a surrogate pair.
			if j > 0 && j < len(d.units) && d.units[j] >= 0xDC00 && d.units[j] <= 0xDFFF {
				j--
			}
			return j
		}
		if end {
			break
		}
		line++
		start = i + 1
	}
	return len(d.units)
}

func (d *refDoc) apply(c protocol.TextDocumentContentChangeEvent) bool {
	text := utf16.Encode([]rune(c.Text))
	if c.Range == nil {
		d.units = text
		return true
	}
	s, e := d.index(c.Range.Start), d.index(c.Range.End)
	if s > e {
		return false
	}
	d.units = append(append(append([]uint16(nil), d.units[:s]...), text...), d.units[e:]...)
	return true
}

var pieces = []string{"a", "b", " ", "x\ty", "é", "中", "😀", "🎉", "\n", "\n", "\r\n", "\r"}

func randomText(rnd *rand.Rand, max int) string {
	var b strings.Builder
	for range rnd.Intn(max + 1) {
		b.WriteString(pieces[rnd.Intn(len(pieces))])
	}
	return b.String()
}

// randomChange returns a random change to a text of lines lines: a range
// that may go past line ends and the last line, or now and then the whole
// text.
func randomChange(rnd *rand.Rand, lines int) protocol.TextDocumentContentChangeEvent {
	if rnd.Intn(30) == 0 {
		return edit(nil, randomText(rnd, 8))
	}
	pos := func() protocol.Position {
		return protocol.Position{Line: uint32(rnd.Intn(lines + 2)), Character: uint32(rnd.Intn(10))}
	}
	a, b := pos(), pos()
	if b.Line < a.Line || b.Line == a.Line && b.Character < a.Character {
		a, b = b, a
	}
	return edit(&protocol.Range{Start: a, End: b}, randomText(rnd, 4))
}

// TestChangeRandom applies random edits to the overlay and to the
// reference: the texts must always be the same.
func TestChangeRandom(t *testing.T) {
	rnd := rand.New(rand.NewSource(111))
	for doc := range 300 {
		s := NewSession(nil)
		start := randomText(rnd, 30)
		open(t, s, start)
		ref := &refDoc{units: utf16.Encode([]rune(start))}
		history := []string{fmt.Sprintf("open %q", start)}
		for v := int32(2); v < 60; v++ {
			lines := get(s, testURI).Mapper().LineCount()
			changes := []protocol.TextDocumentContentChangeEvent{randomChange(rnd, lines)}
			if rnd.Intn(5) == 0 {
				// Several changes in one didChange; a later one is on
				// the text after the ones before (and its lines are a
				// guess).
				changes = append(changes, randomChange(rnd, lines), randomChange(rnd, lines))
			}
			for _, c := range changes {
				if !ref.apply(c) {
					t.Fatalf("the reference refused a forward range: %+v", c)
				}
				if c.Range == nil {
					history = append(history, fmt.Sprintf("full %q", c.Text))
				} else {
					r := c.Range
					history = append(history, fmt.Sprintf("%d:%d-%d:%d %q", r.Start.Line, r.Start.Character, r.End.Line, r.End.Character, c.Text))
				}
			}
			o, err := change(s, testURI, v, changes)
			if err != nil {
				// Every range is forward, and the position rules keep
				// the order of positions, so no change fails.
				t.Fatalf("document %d: %v\n%s", doc, err, strings.Join(history, "\n"))
			}
			if got, want := o.Text(), ref.text(); got != want {
				t.Fatalf("document %d, version %d: the overlay is %q, the reference %q; the edits:\n%s",
					doc, v, got, want, strings.Join(history, "\n"))
			}
		}
	}
}

// FuzzChange is TestChangeRandom for go test -fuzz: one random edit of a
// text, applied to the overlay and to the reference.
func FuzzChange(f *testing.F) {
	f.Add("ab\r\ncd😀\n", uint32(0), uint32(1), uint32(1), uint32(3), "é\r")
	f.Add("😀😀", uint32(0), uint32(1), uint32(0), uint32(3), "")
	f.Add("a\rb\n\r\nc", uint32(1), uint32(0), uint32(9), uint32(0), "x")
	f.Fuzz(func(t *testing.T, text string, l1, c1, l2, c2 uint32, newText string) {
		// Text that comes in JSON is valid UTF-8, and the client counts
		// its UTF-16 units.
		text, newText = strings.ToValidUTF8(text, "�"), strings.ToValidUTF8(newText, "�")
		s := NewSession(nil)
		open(t, s, text)
		ref := &refDoc{units: utf16.Encode([]rune(text))}
		c := edit(rng(l1%8, c1%12, l2%8, c2%12), newText)
		refOK := ref.apply(c)
		o, err := change(s, testURI, 2, []protocol.TextDocumentContentChangeEvent{c})
		if (err == nil) != refOK {
			t.Fatalf("the overlay's error is %v, the reference accepted it: %v", err, refOK)
		}
		if err == nil && o.Text() != ref.text() {
			t.Fatalf("the overlay is %q, the reference %q", o.Text(), ref.text())
		}
	})
}

// TestOverlaysConcurrent reads the documents while they change (run it with
// -race).
func TestOverlaysConcurrent(t *testing.T) {
	s := NewSession(nil)
	open(t, s, "")
	var wg sync.WaitGroup
	done := make(chan struct{})
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				if o := get(s, testURI); o != nil {
					// The text of an overlay never changes: its
					// length is its version minus 1.
					if len(o.Text()) != int(o.Version())-1 {
						t.Errorf("version %d has %d bytes", o.Version(), len(o.Text()))
						return
					}
				}
				all(s)
			}
		}()
	}
	for v := int32(2); v <= 500; v++ {
		if _, err := change(s, testURI, v, []protocol.TextDocumentContentChangeEvent{edit(rng(0, 0, 0, 0), "x")}); err != nil {
			t.Fatal(err)
		}
	}
	close(done)
	wg.Wait()
}

// The cost of a change of a 1 MiB text: a copy of the text and a new line
// index (see Overlay.change).
func benchmarkChange(b *testing.B, n int) {
	text := strings.Repeat("\tfmt.Println(\"hello, world\", x+y) // a comment\n", 1<<20/48)
	changes := make([]protocol.TextDocumentContentChangeEvent, n)
	for i := range changes {
		changes[i] = edit(rng(uint32(i*100), 1, uint32(i*100), 1), "x")
	}
	s := NewSession(nil)
	s.DidOpen(protocol.TextDocumentItem{URI: testURI, Text: text})
	for b.Loop() {
		if _, err := change(s, testURI, 1, changes); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkChange1MiB(b *testing.B)     { benchmarkChange(b, 1) }
func BenchmarkChange1MiBx100(b *testing.B) { benchmarkChange(b, 100) }
