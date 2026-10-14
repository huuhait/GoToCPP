package cppir

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

// indentUnit is one level of indentation in the generated C++.
const indentUnit = "    "

// A writer writes the C++ line by line, with indentation and #line directives.
//
// Each line is either mapped to a .gp place (line) or not (plain). Before a mapped line, the
// writer writes `#line N "/abs/file.gp"` unless the line that C++ would give it anyway is
// already N in that file: C++ counts on from the last #line, so a directive is needed exactly
// when the mapping is not "the previous line + 1". So the three C++ lines of `s, big :=
// add(7, 5)` each get a #line 11, and a breakpoint on .gp line 12 can't stop between them
// (Phase 02 README §2.1).
//
// A plain line (a prototype, the main wrapper) gets no directive of its own: it takes the next
// line number. After mapped lines, that number belongs to the .gp file, so a debugger would
// show generated-only code (gp_init_funcs, the main wrapper) at .gp lines that have nothing to
// do with it. When the writer knows the generated file's own path (out), a plain line that
// isn't blank switches back to that file first: `#line 57 "/work/main.gp.cpp"`, its real line
// number. (A blank line has no code, so it doesn't matter where it maps to.)
type writer struct {
	buf   bytes.Buffer
	depth int

	// out is the absolute path of the generated file, or "" if it isn't known.
	out string

	// lines is the number of lines written so far.
	lines int

	// file and lineNo are the presumed place of the next line: the file and line number
	// that the C++ compiler gives it. file is "" while the compiler counts the lines of the
	// generated file itself: before the first #line, or after a switch back to out.
	file   string
	lineNo int
}

// indent and dedent change the indentation of the lines that follow.
func (w *writer) indent() { w.depth++ }
func (w *writer) dedent() { w.depth-- }

// line writes the line l mapped to pos, with a #line directive before it where needed. If
// pos is not valid, the line is plain.
func (w *writer) line(pos Pos, l string) {
	if !pos.IsValid() {
		w.plain(l)
		return
	}
	if w.file != pos.File || w.lineNo != pos.Line {
		w.directive(pos.Line, pos.File)
		w.file, w.lineNo = pos.File, pos.Line
	}
	w.write(l)
}

// plain writes the line l, which is not mapped to a .gp place.
func (w *writer) plain(l string) {
	if l != "" && w.file != "" && w.out != "" {
		// The directive is line w.lines+1, so the next line is w.lines+2.
		w.directive(w.lines+2, w.out)
		w.file = ""
	}
	w.write(l)
}

// blank writes an empty line.
func (w *writer) blank() {
	w.write("")
}

// directive writes `#line n "file"`.
func (w *writer) directive(n int, file string) {
	w.buf.WriteString("#line ")
	w.buf.WriteString(strconv.Itoa(n))
	w.buf.WriteByte(' ')
	w.buf.WriteString(CString(file))
	w.buf.WriteByte('\n')
	w.lines++
}

// write writes one line with the current indentation (an empty line stays empty). A newline
// in the text of a node would break the #line count: it is a bug in the tree, so it panics.
func (w *writer) write(l string) {
	if strings.Contains(l, "\n") {
		panic(fmt.Sprintf("cppir: a newline in the text of a line: %q", l))
	}
	if l != "" {
		for range w.depth {
			w.buf.WriteString(indentUnit)
		}
		w.buf.WriteString(l)
	}
	w.buf.WriteByte('\n')
	w.lines++
	if w.file != "" {
		w.lineNo++
	}
}

// CString returns s as a C++ string literal (without a suffix such as _gs): printable ASCII
// stays as it is, except `"` and `\`, which get a backslash, and a ? after a ? (\?), because
// GCC and Clang warn about a trigraph such as ??= even in C++20, which has none. The control
// characters that C++ has a named escape for are written with it (\n, \t, \r, \a, \b, \f,
// \v), which never takes a character that follows; every other byte is a 3-digit octal escape
// (\303\251 for "é"). Never \x: in C++ a hex escape takes every hex digit that follows
// ("\x41BC" is one character), and an octal escape takes at most 3. The #line file names use
// it too, so a Windows path keeps its backslashes ("C:\\work\\hello.gp"); GCC and Clang read
// the escapes of a #line file name like those of a string literal.
func CString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"' || c == '\\' || c == '?' && i > 0 && s[i-1] == '?':
			b.WriteByte('\\')
			b.WriteByte(c)
		case c >= 0x20 && c < 0x7f:
			b.WriteByte(c)
		case namedEscapes[c] != 0:
			b.WriteByte('\\')
			b.WriteByte(namedEscapes[c])
		default:
			b.WriteByte('\\')
			b.WriteByte('0' + c>>6)
			b.WriteByte('0' + c>>3&7)
			b.WriteByte('0' + c&7)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// namedEscapes are the letters of the C++ escapes for control characters: \n for '\n'.
var namedEscapes = [256]byte{'\a': 'a', '\b': 'b', '\f': 'f', '\n': 'n', '\r': 'r', '\t': 't', '\v': 'v'}
