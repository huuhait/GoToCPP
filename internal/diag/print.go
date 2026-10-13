package diag

import (
	"bytes"
	"go/token"
	"io"
	"os"
	"path/filepath"
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
}

// diagnostic writes d, its source line if s.Source is set, and its related
// places.
func (s *printState) diagnostic(b *bytes.Buffer, d *Diagnostic) {
	b.WriteString(s.position(d.Pos))
	if d.Severity != Error {
		b.WriteString(d.Severity.String())
		b.WriteString(": ")
	}
	b.WriteString(d.Msg)
	b.WriteByte('\n')
	if s.Source {
		s.source(b, d.Pos)
	}
	for _, r := range d.Related {
		b.WriteByte('\t')
		b.WriteString(s.position(r.Pos))
		b.WriteString(r.Msg)
		b.WriteByte('\n')
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

// source writes the source line of pos, and under it a line with a caret
// (^) under the column of pos. It writes nothing if the file can't be read,
// the line doesn't exist, or the column is not on the line. The column
// just after the last character is on the line: an error at the end of a
// line points there.
func (s *printState) source(b *bytes.Buffer, pos token.Position) {
	if pos.Filename == "" || pos.Line < 1 || pos.Column < 1 {
		return
	}
	line, ok := s.line(pos.Filename, pos.Line)
	if !ok || pos.Column-1 > len(line) {
		return
	}
	// The caret line copies each tab of the source line, so that a tab
	// moves both lines to the same place, and puts spaces for the other
	// characters before the column: as many as the terminal cells they
	// take.
	var caret []byte
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRune(line[i:])
		before := i < pos.Column-1
		switch {
		case r == '\t':
			b.WriteByte('\t')
			if before {
				caret = append(caret, '\t')
			}
		case r == utf8.RuneError && size == 1, unicode.IsControl(r), unicode.Is(unicode.Bidi_Control, r):
			// Show bad UTF-8, control characters (like the start of an
			// escape sequence) and bidi controls as U+FFFD. As they are,
			// they could do strange things in a terminal. A bidi control
			// changes the order in which the terminal shows the
			// characters after it ("Trojan Source"), so the caret could
			// point at the wrong one.
			b.WriteRune(utf8.RuneError)
			if before {
				caret = append(caret, ' ')
			}
		default:
			b.Write(line[i : i+size])
			if before {
				for range width(r) {
					caret = append(caret, ' ')
				}
			}
		}
		i += size
	}
	b.WriteByte('\n')
	b.Write(caret)
	b.WriteString("^\n")
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
