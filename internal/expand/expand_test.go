package expand

import (
	"strings"
	"testing"
)

// TestApplyEditsSpans: the declarations that File prints from the tree (decl edits) have a Span that says
// where their text stands in the result, also when the edits before them change the length of the text.
func TestApplyEditsSpans(t *testing.T) {
	src := []byte("0123456789\nabcdefghij\nABCDEFGHIJ\n")
	edits := []edit{
		{start: 2, end: 4, text: "XYZ", decl: true},     // grows by 1
		{start: 11, end: 21, text: "-", decl: true},     // shrinks by 9
		{start: 22, end: 22, text: "ins", insert: true}, // not a declaration of the source: no span
		{start: 27, end: 29, text: "QQQQ", decl: true},  // after the insertion
		{start: 0, end: 0, text: "head", decl: false},   // plain edit: no span
	}
	out, spans, err := applyEditsSpans(src, edits)
	if err != nil {
		t.Fatal(err)
	}
	if len(spans) != 3 {
		t.Fatalf("%d spans: %+v", len(spans), spans)
	}
	wantText := []string{"XYZ", "-", "QQQQ"}
	for i, sp := range spans {
		if got := string(out[sp.OutStart:sp.OutEnd]); got != wantText[i] {
			t.Errorf("span %d: %q, want %q", i, got, wantText[i])
		}
	}
	if spans[0].SrcStart != 2 || spans[0].SrcEnd != 4 || spans[1].SrcStart != 11 || spans[2].SrcStart != 27 {
		t.Errorf("source places: %+v", spans)
	}
	// The text between the edits is the source, byte for byte.
	if !strings.HasPrefix(string(out), "head01XYZ4567") {
		t.Errorf("out = %q", out)
	}
}

// TestApplyEditsAssociatedSpans: attaching generated methods changes metadata only, not a byte of the output.
// Relative insertion spans follow CRLF normalization/EOF padding, and methods of another type stay outside the selection.
func TestApplyEditsAssociatedSpans(t *testing.T) {
	for _, src := range []string{"type P int\n", "type P int\r\n", "type P int"} {
		t.Run(strings.ReplaceAll(src, "\n", "LF"), func(t *testing.T) {
			text := "Equal\nbody\n\nClone\nbody\n\nOther\nbody"
			clone, other := strings.Index(text, "Clone"), strings.Index(text, "Other")
			e := edit{start: len(src), end: len(src), text: text, insert: true}
			want, _, err := applyEditsSpans([]byte(src), []edit{e})
			if err != nil {
				t.Fatal(err)
			}
			e.spans = []Span{
				{SrcStart: 0, SrcEnd: 1, OutStart: 0, OutEnd: clone - 2},
				{SrcStart: 0, SrcEnd: 1, OutStart: clone, OutEnd: other - 2},
				{SrcStart: 2, SrcEnd: 3, OutStart: other, OutEnd: len(text)},
			}
			out, spans, err := applyEditsSpans([]byte(src), []edit{e})
			if err != nil || string(out) != string(want) || len(spans) != 2 {
				t.Fatalf("metadata changed text or lost spans: %q vs %q; %+v (%v)", out, want, spans, err)
			}
			nl := lineBreak([]byte(src), 0)
			for i, sp := range spans {
				selection := string(out[sp.OutStart:sp.OutEnd])
				selected := text[:other-2]
				if i == 1 {
					selected = text[other:]
				}
				if selection != strings.ReplaceAll(selected, "\n", nl) {
					t.Errorf("span %d selects %q, want %q", i, selection, selected)
				}
			}
		})
	}
}

// TestApplyEditsOverlap: two edits of the same bytes are an error, not text that is left out.
func TestApplyEditsOverlap(t *testing.T) {
	_, err := ApplyEdits([]byte("0123456789"), []Edit{{Start: 2, End: 6, Text: "a"}, {Start: 4, End: 8, Text: "b"}})
	if err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Errorf("err = %v", err)
	}
	out, err := ApplyEdits([]byte("0123456789\n"), []Edit{{Start: 2, End: 4, Text: "ab"}, {Start: 6, End: 8, Text: "cd"}})
	if err != nil || string(out) != "01ab45cd89\n" {
		t.Errorf("out = %q, %v", out, err)
	}
}
