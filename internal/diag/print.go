package diag

import (
	"bytes"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"sort"
	"unicode"
	"unicode/utf8"
)

// A Printer prints the diagnostics of one package in the way the go
// command prints compile errors. The zero Printer prints like go build,
// with two small differences (see Print).
type Printer struct {
	// All prints every diagnostic. Without it, Print stops after 10
	// errors with a "too many errors" line, like go build. The -e flag of
	// gocpp sets it (go build -gcflags=-e does the same).
	All bool

	// Source prints the source line under each diagnostic, and a caret (^)
	// under its column. go build has no such option.
	Source bool

	// Indent starts each line of a source excerpt (Source). The go test
	// runner and many editors read a line that starts with white space as
	// the continuation of the diagnostic before it, so gocpp indents the
	// excerpts with a tab.
	Indent string

	// MaxWidth is the most terminal cells that a source line of an
	// excerpt takes. A longer line is cut on both sides of the column and
	// marked with "…" (its tabs become single spaces, so that the caret
	// stays under the column). 0 shows every line whole.
	MaxWidth int

	// WholeLine makes an excerpt of a place without a column (only the
	// line is known, as with the C++ compiler's errors) underline the code
	// of the line with ~, with no caret. Without it such a place has no
	// excerpt.
	WholeLine bool

	// Related shows an excerpt under each related place that has a
	// position too, if Source is set.
	Related bool

	// Color writes ANSI escape sequences: the position in bold, an error
	// message in bold red, a warning in yellow, the related places
	// (notes) in magenta and the carets in green. See ColorEnabled for
	// the usual rules to turn it on.
	Color bool

	// Dir is the current folder: Print shows file names relative to it.
	// It must be an absolute path. If it is empty, Print uses os.Getwd.
	Dir string

	// ReadFile reads a source file for Source. It gets the file name of
	// the diagnostic, unchanged. If it is nil, Print uses os.ReadFile.
	ReadFile func(name string) ([]byte, error)
}

// Print writes the diagnostics of l to w, one per line, like go build:
//
//	./main.gp:4:10: undefined: count
//	./main.gp:7:2: warning: <message>
//
// A diagnostic without a position prints only its message. A related place
// prints on its own line, after a tab. go build prints no warnings; a
// warning gets "warning: " before its message, so that it does not look
// like an error.
//
// The order is the order of the go compiler. Files come in the order of
// their first diagnostic, not by name. Inside a file, the order is by
// line, then column, and diagnostics at the same place keep the order in
// which they were added. Diagnostics without a file name come first.
//
// Some errors are dropped, like the go compiler does: an error with the
// same file, line and message as the error added just before it (warnings
// don't count), and exact duplicates. Without All, Print shows only the
// first 10 errors in the order they were added, then a line
// "<position of the 10th error>: too many errors". go build prints that
// line after exactly 10 errors too.
//
// File names are shown like the go command shows them. The go command
// gives the compiler absolute file names. In the compiler's output, it
// replaces the folder of the package with its path relative to the
// current folder, if that is shorter (see cmd/go/internal/work/shell.go).
// Print does the same with the folder of each file. So a file in the
// current folder is shown as "./main.gp", a file in a sub-folder as
// "sub/a.gp", a file in the parent folder as "../a.gp", and a file far
// away keeps its absolute name. For this, give the token.FileSet absolute
// file names. A relative name is shown as it is.
//
// Print differs from go build in two known ways:
//   - The column. The go compiler keeps a column in 8 bits, so go build
//     shows no column when it is 255 or more ("a.go:5: ..."), and it sorts
//     and compares such positions as if they had the same column. Print
//     keeps the real column, so that editors and terminals can jump to
//     it. (The compiler also caps the line at 1,048,574, which no real
//     file reaches.)
//   - A file outside the package folder. Print chooses the short name for
//     the folder of each file, go build for the package folder. So from
//     the folder m/, a file m/other/x.go of a package in m/p is shown as
//     "other/x.go" by Print and as "./other/x.go" by go build. This
//     happens with a //line directive, with a related place in another
//     package, and later with errors in C++ headers.
func (p *Printer) Print(w io.Writer, l *List) error {
	if l == nil {
		return nil
	}
	limit := maxErrors
	if p.All {
		limit = 0
	}
	ds, stopPos, stop := l.view(limit)

	s := &printState{
		Printer: p,
		cwd:     p.Dir,
		dirs:    make(map[string]string),
		files:   make(map[string][][]byte),
		cells:   make(map[cellKey]*lineCells),
	}
	if s.cwd == "" {
		s.cwd, _ = os.Getwd() // if it fails, cwd is "" and names stay as they are
	}
	var b bytes.Buffer
	for i := range ds {
		s.diagnostic(&b, &ds[i])
	}
	if stop {
		b.WriteString(s.position(stopPos))
		b.WriteString("too many errors\n")
	}
	if b.Len() == 0 {
		return nil
	}
	_, err := w.Write(b.Bytes())
	return err
}

// printState holds what one call of Print needs.
type printState struct {
	*Printer
	cwd   string              // the current folder, or "" to show names as they are
	dirs  map[string]string   // a folder and how it is shown
	files map[string][][]byte // the lines of each file; nil if it can't be read
	cells map[cellKey]*lineCells
}

// A cellKey names a source line; its cells are made once however many errors point into it (a
// generated line of megabytes with hundreds of errors).
type cellKey struct {
	file string
	line int
}

// lineCells are the cells of a line and their terminal width.
type lineCells struct {
	cs    []cell
	width int
}

// diagnostic writes d, its source line if s.Source is set, and its related
// places.
func (s *printState) diagnostic(b *bytes.Buffer, d *Diagnostic) {
	b.WriteString(s.paint(styleBold, s.position(d.Pos)))
	msgStyle := styleErrorMsg
	if d.Severity != Error {
		b.WriteString(s.paint(styleWarning, d.Severity.String()+":"))
		b.WriteByte(' ')
		msgStyle = ""
	}
	b.WriteString(s.paint(msgStyle, d.Msg))
	b.WriteByte('\n')
	if s.Source {
		s.source(b, "", d.Pos, d.End)
	}
	for _, r := range d.Related {
		b.WriteString(s.paint(styleNote, "\t"+s.position(r.Pos)+r.Msg))
		b.WriteByte('\n')
		if s.Source && s.Related {
			s.source(b, "\t", r.Pos, token.Position{})
		}
	}
}

// position returns "file:line:col: " for pos, with the file name shown like
// the go command shows it. It returns "" if pos has no file name and no
// line, because the go compiler prints no position then.
func (s *printState) position(pos token.Position) string {
	if pos.Filename == "" && !pos.IsValid() {
		return ""
	}
	pos.Filename = s.shortName(pos.Filename)
	return pos.String() + ": "
}

// shortName returns the file name the way the go command shows it (see
// Print).
func (s *printState) shortName(name string) string {
	if s.cwd == "" || !filepath.IsAbs(name) {
		return name
	}
	i := len(name) - 1
	for i >= 0 && !os.IsPathSeparator(name[i]) {
		i--
	}
	dir := name[:i] // the folder, without the separator at its end
	short, ok := s.dirs[dir]
	if !ok {
		short = shortDir(s.cwd, dir)
		s.dirs[dir] = short
	}
	return short + name[i:]
}

// ShortFileName returns the file name name the way Print shows it, with
// cwd as the current folder (Printer.Dir): its folder relative to cwd when
// that is shorter ("./main.gp", "sub/a.gp", "../a.gp"), else as it is. A
// relative name, or an empty cwd, leaves name as it is. gocpp check
// -types names files with it, so that they look like the ones in errors.
func ShortFileName(cwd, name string) string {
	s := printState{cwd: cwd, dirs: make(map[string]string)}
	return s.shortName(name)
}

// ShortPath returns the path p relative to the current folder cwd, if that
// is shorter and is the same file or folder. Else it returns p. It is
// base.ShortPath of the go command, which shows folders this way in its
// messages ("directory prefix sub does not contain main module").
func ShortPath(cwd, p string) string {
	return shortDir(cwd, p)
}

// shortDir returns the folder dir relative to the current folder cwd, if
// that is shorter and is the same folder. Else it returns dir. This is
// base.ShortPath of the go command (cmd/go/internal/base/path.go).
func shortDir(cwd, dir string) string {
	if !filepath.IsAbs(dir) {
		return dir // "" for the root folder, or a volume like "C:"
	}
	rel, err := filepath.Rel(cwd, dir)
	if err != nil || len(rel) >= len(dir) || !sameDir(cwd, rel, dir) {
		return dir
	}
	return rel
}

// sameDir reports whether the relative path rel, seen from cwd, is the
// folder dir. Like the go command, it does not clean rel, because with
// symbolic links "x/.." may not be the folder that holds x. Two paths that
// both don't exist count as the same, like in the go command.
func sameDir(cwd, rel, dir string) bool {
	if !os.IsPathSeparator(cwd[len(cwd)-1]) {
		cwd += string(filepath.Separator)
	}
	fi1, err1 := os.Stat(cwd + rel)
	fi2, err2 := os.Stat(dir)
	if err1 != nil || err2 != nil {
		return os.IsNotExist(err1) && os.IsNotExist(err2)
	}
	return os.SameFile(fi1, fi2)
}

// A cell is one character of a source line as the excerpt shows it.
type cell struct {
	text  string // what is written (U+FFFD for a character that is not safe)
	width int    // terminal cells; a tab is 1 here, and written as a tab
	tab   bool
	start int // the byte offset in the line
}

// cells splits line into cells. Bad UTF-8, control characters (like the
// start of an escape sequence) and bidi controls are shown as U+FFFD: as
// they are, they could do strange things in a terminal. A bidi control
// changes the order in which the terminal shows the characters after it
// ("Trojan Source"), so the caret could point at the wrong one.
func cells(line []byte) []cell {
	cs := make([]cell, 0, len(line))
	text := string(line) // the cells are pieces of it
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRune(line[i:])
		c := cell{start: i}
		switch {
		case r == '\t':
			c.text, c.width, c.tab = "\t", 1, true
		case r == utf8.RuneError && size == 1, unicode.IsControl(r), unicode.Is(unicode.Bidi_Control, r):
			c.text, c.width = string(utf8.RuneError), 1
		default:
			c.text, c.width = text[i:i+size], width(r)
		}
		cs = append(cs, c)
		i += size
	}
	return cs
}

// source writes the source line of pos, and under it a line with a caret
// (^) under the column of pos, and ~ under the rest of the range up to end
// if end is on the same line (on a later line: up to the end of the line).
// Every line starts with s.Indent and indent. It writes nothing if the file
// can't be read, the line doesn't exist, or the column is not on the line.
// The column just after the last character is on the line: an error at the
// end of a line points there. Without a column, the code of the line is
// underlined with ~ if s.WholeLine is set, else nothing is written.
func (s *printState) source(b *bytes.Buffer, indent string, pos, end token.Position) {
	if pos.Filename == "" || pos.Line < 1 || pos.Column < 0 || pos.Column < 1 && !s.WholeLine {
		return
	}
	line, ok := s.line(pos.Filename, pos.Line)
	if ok && pos.Column-1 > len(line) && s.lastLine(pos.Filename, pos.Line) {
		// The end of the file ("expected '}', found 'EOF'") is after the last line's end: the caret
		// goes just after the last character.
		pos.Column = len(line) + 1
		end = token.Position{}
	}
	if !ok || pos.Column-1 > len(line) {
		return
	}
	lc := s.cells[cellKey{pos.Filename, pos.Line}]
	if lc == nil {
		cs := cells(line)
		lc = &lineCells{cs, cellsWidth(cs)}
		s.cells[cellKey{pos.Filename, pos.Line}] = lc
	}
	cs := lc.cs // shared: not changed below
	// from and to are the cell numbers of the range: from is the caret
	// (len(cs) just after the line), to the end of the range (exclusive).
	var from, to int
	caret := pos.Column > 0
	if caret {
		// A column in the middle of a character puts the caret after it.
		from = cellAt(cs, pos.Column-1)
		to = from + 1
		switch {
		case end.Filename == pos.Filename && end.Line > pos.Line:
			to = len(cs)
		case end.Filename == pos.Filename && end.Line == pos.Line && end.Column > pos.Column:
			to = max(cellAt(cs, end.Column-1), from)
		}
	} else {
		from = 0
		for from < len(cs) && (cs[from].text == " " || cs[from].tab) {
			from++
		}
		if from == len(cs) {
			return // a blank line has nothing to underline
		}
		to = len(cs)
		for to > from && (cs[to-1].text == " " || cs[to-1].tab) {
			to--
		}
	}
	to = max(to, from)

	// The part of the line to show. A long line is cut around the caret.
	lo, hi := 0, len(cs)
	var before, after string
	cut := false // the line is cut: a tab shows as a space (one cell)
	if s.MaxWidth > 0 && lc.width > max(s.MaxWidth, minMaxWidth) {
		cut = true
		lo, hi = window(cs, from, max(s.MaxWidth, minMaxWidth))
		if lo > 0 {
			before = "…"
		}
		if hi < len(cs) {
			after = "…"
		}
	}
	to = min(to, hi)

	b.WriteString(indent)
	b.WriteString(s.Indent)
	b.WriteString(before)
	for _, c := range cs[lo:hi] {
		if cut && c.tab {
			b.WriteByte(' ')
			continue
		}
		b.WriteString(c.text)
	}
	b.WriteString(after)
	b.WriteByte('\n')

	// The caret line copies each tab of the source line before the range,
	// so that a tab moves both lines to the same place, and puts spaces for
	// the other characters: as many as the terminal cells they take.
	var cl []byte
	if before != "" {
		cl = append(cl, ' ')
	}
	for _, c := range cs[lo:min(from, hi)] {
		if c.tab && !cut {
			cl = append(cl, '\t')
			continue
		}
		for range c.width {
			cl = append(cl, ' ')
		}
	}
	n := 0 // the cells of the range
	for _, c := range cs[min(from, hi):max(to, min(from, hi))] {
		n += c.width
	}
	var mark []byte
	if caret {
		mark = append(mark, '^')
		for range max(n, 1) - 1 {
			mark = append(mark, '~')
		}
	} else {
		for range n {
			mark = append(mark, '~')
		}
	}
	b.WriteString(indent)
	b.WriteString(s.Indent)
	b.Write(cl)
	b.WriteString(s.paint(styleCaret, string(mark)))
	b.WriteByte('\n')
}

// cellAt returns the number of cells of cs that start before the byte offset off: the index of
// the first cell at or after it (a binary search: a line of megabytes can have many errors).
func cellAt(cs []cell, off int) int {
	return sort.Search(len(cs), func(i int) bool { return cs[i].start >= off })
}

// minMaxWidth is the least Printer.MaxWidth that is used: a narrower window
// has no room for the code around the column.
const minMaxWidth = 20

// cellsWidth returns the terminal cells of cs.
func cellsWidth(cs []cell) int {
	n := 0
	for _, c := range cs {
		n += c.width
	}
	return n
}

// window returns the cells lo:hi of cs to show for a line that is wider than
// maxw cells (a tab is a space then): the caret cell from is about a third
// of the way into the window, which takes at most maxw cells with the
// ellipsis (one cell) at each end that is cut.
func window(cs []cell, from, maxw int) (lo, hi int) {
	w := 0
	for lo = min(from, len(cs)); lo > 0 && w+cs[lo-1].width <= maxw/3; lo-- {
		w += cs[lo-1].width
	}
	avail := maxw
	if lo > 0 {
		avail--
	}
	w = 0
	for hi = lo; hi < len(cs) && w+cs[hi].width <= avail; hi++ {
		w += cs[hi].width
	}
	if hi < len(cs) { // the ellipsis on the right takes a cell
		for hi > lo && w+1 > avail {
			hi--
			w -= cs[hi].width
		}
		return lo, hi
	}
	// The window reaches the end of the line: use the room on the left.
	for lo > 0 && cellsWidth(cs[lo-1:hi])+boolInt(lo-1 > 0) <= maxw {
		lo--
	}
	return lo, hi
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// lastLine reports whether n is the last line of the file name (already read by line): the
// last one with text, as a final line end makes an empty line after it.
func (s *printState) lastLine(name string, n int) bool {
	lines := s.files[name]
	if k := len(lines); k > 1 && len(lines[k-1]) == 0 {
		return n == k-1
	}
	return n == len(lines)
}

// line returns line n (n >= 1) of the file, without its line end. It reads
// each file at most once per Print.
func (s *printState) line(name string, n int) ([]byte, bool) {
	lines, ok := s.files[name]
	if !ok {
		read := s.ReadFile
		if read == nil {
			read = os.ReadFile
		}
		if data, err := read(name); err == nil {
			lines = bytes.Split(data, []byte("\n"))
		}
		s.files[name] = lines // nil if the file can't be read: don't try again
	}
	if n > len(lines) {
		return nil, false
	}
	return bytes.TrimSuffix(lines[n-1], []byte("\r")), true
}

//go:generate python3 mkwidth.py

// width returns the number of terminal cells that r takes: 0 for a
// combining mark and other characters without width, 2 for a wide East
// Asian character or an emoji (the table wide in widthtable.go, made by
// mkwidth.py), else 1.
func width(r rune) int {
	switch {
	case unicode.In(r, unicode.Mn, unicode.Me, unicode.Cf):
		return 0
	case unicode.Is(wide, r):
		return 2
	}
	return 1
}
