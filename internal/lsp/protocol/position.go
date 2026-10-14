// gocpp's own code (not copied from gopls, see COPIED.md): conversion
// between LSP positions and byte offsets.

package protocol

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// A Mapper converts between byte offsets in a UTF-8 text and LSP positions,
// whose Character counts UTF-16 code units (the "utf-16" position
// encoding, the only one the server offers). It is immutable, so it is safe
// for concurrent use.
//
// The rules, those of gopls's Mapper (go/token and Neovim agree, and VS
// Code sends its own line ends):
//   - Only "\n" ends a line. A "\r" right before it belongs to the line end
//     ("\r\n"); any other "\r" is a character of the line. (LSP 3.17 also
//     lists a lone "\r" as a line end, but go/token doesn't, so a position
//     from the front-end would not match.)
//   - The line end is not part of the line: a Character past the end of
//     its line means the end of the line, before "\n" or "\r\n".
//   - A Line past the last line means the end of the text. (The spec doesn't
//     say; clients send it to mean "the end", for example to replace the
//     whole text.)
//   - A Character in the middle of a character that takes two UTF-16 units
//     (a rune above U+FFFF, such as most emoji) means the start of that
//     character, so an edit never splits a character.
//   - A byte that is not valid UTF-8 counts as one UTF-16 unit (as U+FFFD).
//
// Step 11.4 must convert the front-end's positions through byte offsets
// (token.Position.Offset, then Position), never use token.Position.Column
// as a Character: Column counts UTF-8 bytes, Character UTF-16 units.
type Mapper struct {
	text  string
	lines []int // the byte offset of the start of each line; lines[0] == 0
}

// NewMapper returns a Mapper for text.
func NewMapper(text string) *Mapper {
	return &Mapper{text: text, lines: lineStarts(text)}
}

// lineStarts returns the byte offset of the start of each line of text.
func lineStarts(text string) []int {
	lines := make([]int, 1, 1+strings.Count(text, "\n"))
	for i := 0; ; {
		j := strings.IndexByte(text[i:], '\n')
		if j < 0 {
			return lines
		}
		i += j + 1
		lines = append(lines, i)
	}
}

// Text returns the text.
func (m *Mapper) Text() string { return m.text }

// LineCount returns the number of lines: one more than the number of line
// ends.
func (m *Mapper) LineCount() int { return len(m.lines) }

// line returns the byte offsets of the start and the end of line i, without
// its line end.
func (m *Mapper) line(i int) (start, end int) {
	start = m.lines[i]
	if i+1 == len(m.lines) {
		return start, len(m.text)
	}
	end = m.lines[i+1] - 1 // the \n
	if end > start && m.text[end-1] == '\r' {
		end--
	}
	return start, end
}

// Offset returns the byte offset of the position p, by the rules of Mapper.
func (m *Mapper) Offset(p Position) int {
	if int64(p.Line) >= int64(len(m.lines)) {
		return len(m.text)
	}
	start, end := m.line(int(p.Line))
	return start + utf16Prefix(m.text[start:end], int(p.Character))
}

// utf16Prefix returns the length in bytes of the longest prefix of s that
// is at most n UTF-16 units long and doesn't end in the middle of a rune.
func utf16Prefix(s string, n int) int {
	units := 0
	for i := 0; i < len(s); {
		if c := s[i]; c < utf8.RuneSelf {
			if units == n {
				return i
			}
			units++
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		w := 1
		if r > 0xFFFF {
			w = 2
		}
		if units+w > n {
			return i
		}
		units += w
		i += size
	}
	return len(s)
}

// Position returns the position of the byte offset off. An offset before
// the text is its start and one after it is its end; an offset in a line
// end is the end of the line, and one inside a rune is the start of the
// rune.
func (m *Mapper) Position(off int) Position {
	off = max(0, min(off, len(m.text)))
	line := sort.Search(len(m.lines), func(i int) bool { return m.lines[i] > off }) - 1
	start, end := m.line(line)
	off = min(off, end)
	units := 0
	for i := start; i < off; {
		r, size := utf8.DecodeRuneInString(m.text[i:])
		if i+size > off {
			break // off is inside this rune
		}
		units++
		if r > 0xFFFF {
			units++
		}
		i += size
	}
	return Position{Line: uint32(line), Character: uint32(units)}
}

// RangeOffsets returns the byte offsets of the start and the end of r. It is
// an error when the start comes after the end.
func (m *Mapper) RangeOffsets(r Range) (start, end int, err error) {
	start, end = m.Offset(r.Start), m.Offset(r.End)
	if start > end {
		return 0, 0, fmt.Errorf("the range %d:%d-%d:%d ends before it starts",
			r.Start.Line, r.Start.Character, r.End.Line, r.End.Character)
	}
	return start, end, nil
}
