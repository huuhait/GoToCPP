package diag

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unicode"
)

func TestShortName(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"sub", "two"} {
		if err := os.Mkdir(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	join := filepath.Join
	// From here, a relative path to root is longer than root itself.
	deep := root + strings.Repeat(string(filepath.Separator)+"d", len(root))
	// The root folder: "/", or "C:\" on Windows.
	top := filepath.VolumeName(root) + string(filepath.Separator)

	tests := []struct {
		name string
		cwd  string
		file string
		want string
	}{
		{"current folder", root, join(root, "a.gp"), "./a.gp"},
		{"sub-folder", root, join(root, "sub", "a.gp"), "sub/a.gp"},
		{"folder that doesn't exist", root, join(root, "sub", "new", "a.gp"), "sub/new/a.gp"},
		{"parent folder", join(root, "sub"), join(root, "a.gp"), "../a.gp"},
		{"other folder", join(root, "sub"), join(root, "two", "a.gp"), "../two/a.gp"},
		{"absolute is shorter", deep, join(root, "a.gp"), join(root, "a.gp")},
		{"relative name", root, "a.gp", "a.gp"},
		{"relative name with a folder", root, join("sub", "a.gp"), join("sub", "a.gp")},
		{"no name", root, "", ""},
		{"file in the root folder", root, top + "a.gp", top + "a.gp"},
		{"current folder is the root", top, top + "a.gp", top + "a.gp"},
		{"no current folder", "", join(root, "a.gp"), join(root, "a.gp")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &printState{cwd: tt.cwd, dirs: make(map[string]string)}
			if got := s.shortName(tt.file); got != filepath.FromSlash(tt.want) {
				t.Errorf("shortName(%q) from %q = %q, want %q", tt.file, tt.cwd, got, tt.want)
			}
			if got := ShortFileName(tt.cwd, tt.file); got != filepath.FromSlash(tt.want) {
				t.Errorf("ShortFileName(%q, %q) = %q, want %q", tt.cwd, tt.file, got, tt.want)
			}
		})
	}
}

func TestShortPath(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ cwd, path, want string }{
		{root, root, "."},
		{root, sub, "sub"},
		{sub, root, ".."},
		{root, filepath.Join(root, "sub", "a.gp"), filepath.Join("sub", "a.gp")},
		{root, "rel", "rel"},
	} {
		if got := ShortPath(tt.cwd, tt.path); got != tt.want {
			t.Errorf("ShortPath(%q, %q) = %q, want %q", tt.cwd, tt.path, got, tt.want)
		}
	}
}

// TestShortNameSymlink checks the check of the go command that the relative
// path is really the same folder. The current folder is root/link, a
// symbolic link to root/real/sub. Seen from there, ".." is root/real, not
// root, so a file in root keeps its absolute name. Windows is different:
// it resolves ".." by name, without following the link. So there the check
// passes, and go build shows ..\a.go too.
func TestShortNameSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "real", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(filepath.Join(root, "real", "sub"), link); err != nil {
		t.Skip("no symbolic links:", err)
	}
	s := &printState{cwd: link, dirs: make(map[string]string)}
	want := filepath.Join(root, "a.gp")
	if runtime.GOOS == "windows" {
		want = `..\a.gp`
	}
	if got := s.shortName(filepath.Join(root, "a.gp")); got != want {
		t.Errorf("file in root: got %q, want %q", got, want)
	}
	if got, want := s.shortName(filepath.Join(link, "a.gp")), filepath.FromSlash("./a.gp"); got != want {
		t.Errorf("file in the current folder: got %q, want %q", got, want)
	}
}

func TestPrintPaths(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := filepath.Join(root, "a.gp")
	var l List
	l.Add(Diagnostic{
		Pos:     pos(a, 4, 5),
		Msg:     "x redeclared in this block",
		Related: []Related{{Pos: pos(a, 3, 5), Msg: "other declaration of x"}},
	})
	for i := 1; i <= 9; i++ {
		l.Errorf(pos(a, 10+i, 2), "e%d", i)
	}
	for _, tt := range []struct{ dir, short string }{
		{root, "./a.gp"},
		{filepath.Join(root, "sub"), "../a.gp"},
	} {
		// The related place and the "too many errors" line use the same
		// file name as the diagnostics.
		tt.short = filepath.FromSlash(tt.short)
		var want strings.Builder
		fmt.Fprintf(&want, "%s:4:5: x redeclared in this block\n\t%s:3:5: other declaration of x\n", tt.short, tt.short)
		for i := 1; i <= 9; i++ {
			fmt.Fprintf(&want, "%s:%d:2: e%d\n", tt.short, 10+i, i)
		}
		fmt.Fprintf(&want, "%s:19:2: too many errors\n", tt.short)

		var b strings.Builder
		if err := (&Printer{Dir: tt.dir}).Print(&b, &l); err != nil {
			t.Fatal(err)
		}
		if got := b.String(); got != want.String() {
			t.Errorf("from %s, got:\n%s\nwant:\n%s", tt.dir, got, want.String())
		}
	}
}

func TestPrintDefaultDir(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	var l List
	l.Errorf(pos(filepath.Join(root, "a.gp"), 1, 1), "m")
	var b strings.Builder
	if err := (&Printer{}).Print(&b, &l); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), filepath.FromSlash("./a.gp")+":1:1: m\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestSource(t *testing.T) {
	tests := []struct {
		name      string
		src       string // the file a.gp
		line, col int
		want      string // what Print writes after the diagnostic line
	}{
		{"first column", "x := y\n", 1, 1, "x := y\n^\n"},
		{"in the line", "x := y\n", 1, 6, "x := y\n     ^\n"},
		{"other line", "package p\n\nx := y\n", 3, 6, "x := y\n     ^\n"},
		{"tab", "\tprintln(y)\n", 1, 10, "\tprintln(y)\n\t        ^\n"},
		{"tabs between", "\t\tx :=\ty\n", 1, 8, "\t\tx :=\ty\n\t\t    \t^\n"},
		// "é" is 2 bytes and 1 character: the column 13 is the 12th character.
		{"UTF-8 before the column", "s := \"é\" + y\n", 1, 13, "s := \"é\" + y\n           ^\n"},
		// "e" and U+0301 (a combining accent) are 3 bytes, 1 terminal cell.
		{"combining mark", "s := \"e\u0301\" + y\n", 1, 14, "s := \"e\u0301\" + y\n           ^\n"},
		// Each of 世 and 界 is 3 bytes and 2 terminal cells.
		{"wide characters", "s := \"世界\" + y\n", 1, 17, "s := \"世界\" + y\n              ^\n"},
		{"emoji", "s := \"🙂\" + y\n", 1, 15, "s := \"🙂\" + y\n            ^\n"},
		// 🚀 is 4 bytes, ✅ and ⭐ are 3 bytes; all three are 2 cells.
		{"more emoji", "s := \"🚀✅⭐\" + y\n", 1, 21, "s := \"🚀✅⭐\" + y\n                ^\n"},
		// A file can start with a byte order mark (3 bytes, no width).
		{"byte order mark", "\ufeffpackage p\n", 1, 4, "\ufeffpackage p\n^\n"},
		{"just after the end", "x :=\n", 1, 5, "x :=\n    ^\n"},
		{"past the end", "x\n", 1, 3, ""},
		{"no column", "x\n", 1, 0, ""},
		{"no line", "x\n", 0, 1, ""},
		{"line after the end", "x\n", 3, 1, ""},
		// The position of the end of the file, after the last newline.
		{"end of file", "x\n", 2, 1, "\n^\n"},
		{"no newline at the end", "x := y", 1, 6, "x := y\n     ^\n"},
		{"empty file", "", 1, 1, "\n^\n"},
		{"CRLF", "x := y\r\nz\r\n", 1, 6, "x := y\n     ^\n"},
		// Control characters and bad UTF-8 are shown as U+FFFD.
		{"escape character", "s := \"\x1b[31m\" + y\n", 1, 16, "s := \"\uFFFD[31m\" + y\n               ^\n"},
		{"bad UTF-8", "s := \"\xff\" + y\n", 1, 12, "s := \"\uFFFD\" + y\n           ^\n"},
		{"carriage return in the line", "a\rb := y\n", 1, 8, "a\uFFFDb := y\n       ^\n"},
		// Bidi controls too: they could change the order on the screen.
		{"bidi override", "s := \"\u202e\" + y\n", 1, 14, "s := \"\uFFFD\" + y\n           ^\n"},
		{"bidi isolate", "\u2067s := y\n", 1, 9, "\uFFFDs := y\n      ^\n"},
		// A column in the middle of a character does no harm.
		{"column inside a character", "é\n", 1, 2, "é\n ^\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var l List
			l.Errorf(pos("a.gp", tt.line, tt.col), "m")
			p := Printer{Source: true, ReadFile: func(name string) ([]byte, error) {
				if name != "a.gp" {
					t.Errorf("ReadFile(%q), want a.gp", name)
				}
				return []byte(tt.src), nil
			}}
			var b strings.Builder
			if err := p.Print(&b, &l); err != nil {
				t.Fatal(err)
			}
			got, ok := strings.CutPrefix(b.String(), pos("a.gp", tt.line, tt.col).String()+": m\n")
			if !ok {
				t.Fatalf("output %q doesn't start with the diagnostic", b.String())
			}
			if got != tt.want {
				t.Errorf("got:\n%q\nwant:\n%q", got, tt.want)
			}
		})
	}
}

func TestSourceOff(t *testing.T) {
	var l List
	l.Errorf(pos("a.gp", 1, 1), "m")
	p := Printer{ReadFile: func(string) ([]byte, error) {
		t.Error("ReadFile called without Source")
		return nil, nil
	}}
	var b strings.Builder
	if err := p.Print(&b, &l); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "a.gp:1:1: m\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// TestSourceFiles checks the order of the lines with related places and
// several files, that each file is read once, and that a file that can't
// be read only loses its source lines.
func TestSourceFiles(t *testing.T) {
	files := map[string]string{
		"a.gp": "package p\nvar x int\nvar x string\n",
		"b.gp": "package p\nfunc f() { g(1, 2) }\n",
	}
	reads := make(map[string]int)
	p := Printer{Source: true, ReadFile: func(name string) ([]byte, error) {
		reads[name]++
		src, ok := files[name]
		if !ok {
			return nil, os.ErrNotExist
		}
		return []byte(src), nil
	}}
	var l List
	l.Add(Diagnostic{
		Pos:     pos("a.gp", 3, 5),
		Msg:     "x redeclared in this block",
		Related: []Related{{Pos: pos("a.gp", 2, 5), Msg: "other declaration of x"}},
	})
	l.Errorf(pos("b.gp", 2, 17), "too many arguments in call to g\n\thave (number, number)\n\twant (int)")
	l.Errorf(pos("missing.gp", 1, 1), "first")
	l.Errorf(pos("a.gp", 1, 1), "start")
	l.Errorf(pos("missing.gp", 2, 1), "second")
	l.Errorf(pos("b.gp", 1, 9), "name")

	want := `a.gp:1:1: start
package p
^
a.gp:3:5: x redeclared in this block
var x string
    ^
	a.gp:2:5: other declaration of x
b.gp:1:9: name
package p
        ^
b.gp:2:17: too many arguments in call to g
	have (number, number)
	want (int)
func f() { g(1, 2) }
                ^
missing.gp:1:1: first
missing.gp:2:1: second
`
	var b strings.Builder
	if err := p.Print(&b, &l); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	for name, n := range reads {
		if n != 1 {
			t.Errorf("%s was read %d times, want 1", name, n)
		}
	}
	if len(reads) != 3 {
		t.Errorf("read %v, want a.gp, b.gp and missing.gp", reads)
	}
}

// TestSourceOS reads the files with os.ReadFile: a real file, a file that
// doesn't exist and a folder.
func TestSourceOS(t *testing.T) {
	root := t.TempDir()
	good := filepath.Join(root, "a.gp")
	if err := os.WriteFile(good, []byte("package p\n\tx := y\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var l List
	l.Errorf(pos(good, 2, 7), "undefined: y")
	l.Errorf(pos(filepath.Join(root, "missing.gp"), 1, 1), "missing")
	l.Errorf(pos(root, 1, 1), "folder")
	var b strings.Builder
	if err := (&Printer{Source: true, Dir: root}).Print(&b, &l); err != nil {
		t.Fatal(err)
	}
	// root itself is shown relative to the current folder root: "../<name>".
	dot := "." + string(filepath.Separator)
	folder := ".." + string(filepath.Separator) + filepath.Base(root)
	want := dot + "a.gp:2:7: undefined: y\n\tx := y\n\t     ^\n" +
		dot + "missing.gp:1:1: missing\n" +
		folder + ":1:1: folder\n"
	if got := b.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWidth(t *testing.T) {
	tests := []struct {
		r    rune
		want int
	}{
		{'a', 1}, {'é', 1}, {'ư', 1}, {'Ω', 1}, {'ж', 1},
		{'\u0301', 0}, // combining acute accent
		{'\u200b', 0}, // zero width space
		{'\ufeff', 0}, // byte order mark
		{'世', 2}, {'ア', 2}, {'한', 2}, {'Ａ', 2}, {'🙂', 2}, {'\U00020000', 2},
		// Emoji with the East Asian Width W, in and after the BMP.
		{'🚀', 2}, {'✅', 2}, {'⭐', 2}, {'⏰', 2}, {'☕', 2}, {'✨', 2},
		{'🫠', 2}, {'🟢', 2}, {'🈚', 2}, {'🀄', 2}, {'🃏', 2}, {'🆎', 2},
		// Emoji with the East Asian Width N: one cell.
		{'🌡', 1}, {'🌶', 1}, {'🐿', 1},
		// A combining mark is 0, even where the table says wide.
		{'\u3099', 0},
	}
	for _, tt := range tests {
		if got := width(tt.r); got != tt.want {
			t.Errorf("width(%U) = %d, want %d", tt.r, got, tt.want)
		}
	}
}

// TestWideTable checks the generated table: sorted, without overlaps, and
// with the LatinOffset that unicode.Is needs.
func TestWideTable(t *testing.T) {
	last := -1 // the end of the range before
	check := func(lo, hi, stride uint32) {
		if stride != 1 || lo > hi || int(lo) <= last {
			t.Errorf("bad range %#x-%#x (stride %d) after %#x", lo, hi, stride, last)
		}
		last = int(hi)
	}
	latin := 0
	for _, r := range wide.R16 {
		check(uint32(r.Lo), uint32(r.Hi), uint32(r.Stride))
		if r.Hi <= unicode.MaxLatin1 {
			latin++
		}
	}
	for _, r := range wide.R32 {
		check(r.Lo, r.Hi, r.Stride)
	}
	if wide.LatinOffset != latin {
		t.Errorf("LatinOffset = %d, want %d", wide.LatinOffset, latin)
	}
	if len(wide.R16) == 0 || len(wide.R32) == 0 {
		t.Errorf("the table has %d and %d ranges, want some of both", len(wide.R16), len(wide.R32))
	}
}

// TestPrintLongColumns pins the first known difference from go build (see
// Print): Print shows the real column, also when it is 255 or more. The go
// compiler keeps a column in 8 bits (cmd/internal/src/pos.go), so for this
// source, real go build (Go 1.25.14) prints:
//
//	col/a.go:4:254: undefined: u254
//	col/a.go:5: undefined: u255
//	col/a.go:6: undefined: u256
//	col/a.go:7: undefined: u300
//
// go build also sorts such positions as if they had the same column.
// Print sorts them by the real column.
func TestPrintLongColumns(t *testing.T) {
	dir := t.TempDir()
	var src strings.Builder
	src.WriteString("package col\n\nfunc f() {\n")
	for _, col := range []int{254, 255, 256, 300} {
		// The undefined name u<col> starts at the column col.
		pad := strings.Repeat("x", col-1-len("\tprintln(\"")-len("\", "))
		fmt.Fprintf(&src, "\tprintln(%q, u%d)\n", pad, col)
	}
	src.WriteString("}\n")
	a := filepath.Join(dir, "a.go")
	if err := os.WriteFile(a, []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	var l List
	typeCheck(t, dir, &l)
	// Two more errors on one line, the one at column 300 added first.
	l.Errorf(pos(a, 9, 300), "second")
	l.Errorf(pos(a, 9, 256), "first")

	dot := "." + string(filepath.Separator)
	want := dot + "a.go:4:254: undefined: u254\n" +
		dot + "a.go:5:255: undefined: u255\n" +
		dot + "a.go:6:256: undefined: u256\n" +
		dot + "a.go:7:300: undefined: u300\n" +
		dot + "a.go:9:256: first\n" +
		dot + "a.go:9:300: second\n"
	var b strings.Builder
	if err := (&Printer{Dir: dir}).Print(&b, &l); err != nil {
		t.Fatal(err)
	}
	if got := b.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("disk full") }

func TestPrintWriteError(t *testing.T) {
	var l List
	l.Errorf(pos("a.gp", 1, 1), "m")
	if err := (&Printer{}).Print(failWriter{}, &l); err == nil || err.Error() != "disk full" {
		t.Errorf("Print to a failing writer: err = %v, want disk full", err)
	}
	// Nothing to print: nothing is written, so no error.
	if err := (&Printer{}).Print(failWriter{}, &List{}); err != nil {
		t.Errorf("Print of an empty list: err = %v", err)
	}
	if err := (&Printer{}).Print(failWriter{}, nil); err != nil {
		t.Errorf("Print of a nil list: err = %v", err)
	}
}

// TestPrintConcurrent prints one List from several goroutines at the same
// time. Print and Diagnostics only read the list, so go test -race finds
// nothing.
func TestPrintConcurrent(t *testing.T) {
	var l List
	for i := range 30 {
		l.Add(Diagnostic{
			Pos:     pos("a.gp", 30-i, 1),
			Msg:     fmt.Sprintf("e%d", i),
			Related: []Related{{Pos: pos("a.gp", 40, 1), Msg: "r"}},
		})
	}
	printAll := func() string {
		var b strings.Builder
		if err := (&Printer{All: true}).Print(&b, &l); err != nil {
			t.Error(err)
		}
		return b.String()
	}
	want := printAll()
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if got := printAll(); got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
			if n := len(l.Diagnostics()); n != 30 {
				t.Errorf("Diagnostics() has %d, want 30", n)
			}
		})
	}
	wg.Wait()
}
