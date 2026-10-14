package protocol

import (
	"math/rand"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

func TestMapperOffset(t *testing.T) {
	for _, test := range []struct {
		text      string
		line, chr uint32
		want      int
	}{
		{"", 0, 0, 0},
		{"", 0, 5, 0},
		{"", 3, 0, 0},
		{"abc", 0, 2, 2},
		{"abc", 0, 3, 3},
		{"abc", 0, 99, 3},     // past the end of the line
		{"abc", 1, 0, 3},      // past the last line
		{"ab\ncd", 1, 1, 4},   //
		{"ab\ncd", 0, 7, 2},   // the end of the line is before its \n
		{"ab\n", 1, 0, 3},     // the empty last line
		{"ab\n", 2, 0, 3},     // past it
		{"ab\r\ncd", 0, 3, 2}, // the end of the line is before its \r\n
		{"ab\r\ncd", 1, 0, 4}, //
		{"ab\rcd", 0, 3, 3},   // a lone \r is a character of the line
		{"ab\rcd", 1, 0, 5},   // so there is one line
		{"ab\rcd", 0, 9, 5},   //
		{"ab\r", 0, 9, 3},     // also at the end of the text
		{"a\r\r\nb", 0, 9, 2}, // a \r, then the line end \r\n: two lines
		{"a\r\r\nb", 1, 0, 4}, //
		{"a\n\rb", 1, 1, 3},   // \n, then a \r of the second line
		{"a\n\rb", 2, 0, 4},   //
		{"é中x", 0, 2, 5},      // é is 2 bytes, 中 3; each is one UTF-16 unit
		{"😀x", 0, 2, 4},       // 😀 is 4 bytes and two UTF-16 units
		{"😀x", 0, 3, 5},       //
		{"😀x", 0, 1, 0},       // in the middle of 😀: its start
		{"a😀😀", 0, 4, 5},      // in the middle of the second 😀
		{"x\n😀\r\n", 1, 2, 6}, //
		{"x\n😀\r\n", 1, 9, 6}, //
		{"a\xffb", 0, 2, 2},   // a bad byte is one unit
	} {
		m := NewMapper(test.text)
		if got := m.Offset(Position{Line: test.line, Character: test.chr}); got != test.want {
			t.Errorf("Offset(%q, %d:%d) = %d, want %d", test.text, test.line, test.chr, got, test.want)
		}
	}
}

func TestMapperPosition(t *testing.T) {
	for _, test := range []struct {
		text      string
		off       int
		line, chr uint32
	}{
		{"", 0, 0, 0},
		{"", 5, 0, 0},
		{"abc", -1, 0, 0},
		{"abc", 2, 0, 2},
		{"abc", 10, 0, 3},
		{"ab\ncd", 2, 0, 2},
		{"ab\ncd", 3, 1, 0},
		{"ab\r\ncd", 3, 0, 2}, // between \r and \n: the end of the line
		{"ab\r\ncd", 4, 1, 0},
		{"ab\rcd", 3, 0, 3},
		{"é中x", 5, 0, 2},
		{"é中x", 3, 0, 1}, // inside 中: its start
		{"😀x", 4, 0, 2},
		{"😀x", 2, 0, 0}, // inside 😀
		{"ab\n", 3, 1, 0},
	} {
		m := NewMapper(test.text)
		want := Position{Line: test.line, Character: test.chr}
		if got := m.Position(test.off); got != want {
			t.Errorf("Position(%q, %d) = %d:%d, want %d:%d", test.text, test.off, got.Line, got.Character, want.Line, want.Character)
		}
	}
}

func TestMapperLineCount(t *testing.T) {
	for text, want := range map[string]int{"": 1, "a": 1, "a\n": 2, "a\r\nb\rc\nd": 3, "\r\r\n\n": 3, "\r": 1} {
		if got := NewMapper(text).LineCount(); got != want {
			t.Errorf("LineCount(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestRangeOffsets(t *testing.T) {
	m := NewMapper("ab\ncd")
	start, end, err := m.RangeOffsets(Range{Start: Position{0, 1}, End: Position{1, 1}})
	if start != 1 || end != 4 || err != nil {
		t.Errorf("RangeOffsets = %d, %d, %v; want 1, 4", start, end, err)
	}
	if _, _, err := m.RangeOffsets(Range{Start: Position{1, 0}, End: Position{0, 1}}); err == nil {
		t.Error("RangeOffsets of a backward range: no error")
	}
	// Both clamp to the same end of the line: an empty range.
	start, end, err = m.RangeOffsets(Range{Start: Position{0, 9}, End: Position{0, 5}})
	if start != 2 || end != 2 || err != nil {
		t.Errorf("RangeOffsets = %d, %d, %v; want 2, 2", start, end, err)
	}
}

// pieces are what the random texts are made of: one-, two-, three- and
// four-byte runes, \n, \r\n and a lone \r (a character, not a line end).
var pieces = []string{"a", "b", " ", "é", "中", "😀", "🎉", "\n", "\r\n", "\r", "\t"}

func randomText(rnd *rand.Rand, n int) string {
	var b strings.Builder
	for range n {
		b.WriteString(pieces[rnd.Intn(len(pieces))])
	}
	return b.String()
}

// A refText is the reference implementation of the position rules: the
// text as UTF-16 units, cut into lines by simple slicing.
type refText struct {
	units []uint16
	lines [][2]int // the start and the end (without the line end) of each line, in units
}

func newRefText(text string) *refText {
	r := &refText{units: utf16.Encode([]rune(text))}
	start := 0
	for i, u := range r.units {
		if u == '\n' {
			end := i
			if end > start && r.units[end-1] == '\r' {
				end-- // \r\n
			}
			r.lines = append(r.lines, [2]int{start, end})
			start = i + 1
		}
	}
	r.lines = append(r.lines, [2]int{start, len(r.units)})
	return r
}

// index returns the index in units of the position p.
func (r *refText) index(p Position) int {
	if int(p.Line) >= len(r.lines) {
		return len(r.units)
	}
	l := r.lines[p.Line]
	i := min(l[0]+int(p.Character), l[1])
	if i > 0 && i < len(r.units) && utf16.IsSurrogate(rune(r.units[i])) && r.units[i] >= 0xDC00 {
		i-- // the second half of a pair: back to its first half
	}
	return i
}

// byteOffset returns the length in UTF-8 bytes of the first i units.
func (r *refText) byteOffset(i int) int {
	return len(string(utf16.Decode(r.units[:i])))
}

// TestMapperRandom compares Mapper.Offset with the reference on random
// texts and positions, and checks that Position undoes Offset.
func TestMapperRandom(t *testing.T) {
	rnd := rand.New(rand.NewSource(11))
	for range 2000 {
		text := randomText(rnd, rnd.Intn(40))
		m, ref := NewMapper(text), newRefText(text)
		if m.LineCount() != len(ref.lines) {
			t.Fatalf("%q: LineCount = %d, want %d", text, m.LineCount(), len(ref.lines))
		}
		for range 20 {
			p := Position{Line: uint32(rnd.Intn(len(ref.lines) + 2)), Character: uint32(rnd.Intn(12))}
			want := ref.byteOffset(ref.index(p))
			got := m.Offset(p)
			if got != want {
				t.Fatalf("Offset(%q, %d:%d) = %d, want %d", text, p.Line, p.Character, got, want)
			}
			if back := m.Offset(m.Position(got)); back != got {
				t.Fatalf("%q: Offset(Position(%d)) = %d", text, got, back)
			}
		}
		// Every rune boundary that is not inside a \r\n is a position.
		for off := 0; off <= len(text); off++ {
			if off < len(text) && !utf8.RuneStart(text[off]) || off > 0 && off < len(text) && text[off-1] == '\r' && text[off] == '\n' {
				continue
			}
			if back := m.Offset(m.Position(off)); back != off {
				p := m.Position(off)
				t.Fatalf("%q: Position(%d) = %d:%d, and its Offset is %d", text, off, p.Line, p.Character, back)
			}
		}
	}
}

func BenchmarkNewMapper(b *testing.B) {
	text := strings.Repeat("\tfmt.Println(\"hello, world\", x+y) // a comment\n", 20000) // 1 MB
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		NewMapper(text)
	}
}
