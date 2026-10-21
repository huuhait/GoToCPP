package probe

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.MkdirAll(filepath.Join(real, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeHeader(t, real, "up.hpp", "")
	cases := []struct {
		name, spelled, want string
	}{
		{"relative to the working folder", "inc/api.hpp", filepath.Join(dir, "inc", "api.hpp")},
		{"dot element", "./inc/./api.hpp", filepath.Join(dir, "inc", "api.hpp")},
		{"absolute", filepath.Join(dir, "x.hpp"), filepath.Join(dir, "x.hpp")},
		{"doubled separators", dir + "//a///b.hpp", filepath.Join(dir, "a", "b.hpp")},
		{"empty", "", ""},
		{"a dot-dot that is not on disk", "gone/../x.hpp", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizePath(dir, c.spelled); got != c.want {
				t.Errorf("normalizePath(%q) = %q, want %q", c.spelled, got, c.want)
			}
		})
	}
	t.Run("a link before a dot-dot makes the parent the real parent", func(t *testing.T) {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(filepath.Join(real, "sub"), link); err != nil {
			t.Skip("no symbolic links:", err)
		}
		got := normalizePath(dir, "link/../up.hpp")
		if got != filepath.Join(real, "up.hpp") {
			t.Errorf("link/../up.hpp = %q, want %q (not the lexical parent)", got,
				filepath.Join(real, "up.hpp"))
		}
	})
}

func TestSourceSetPlace(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "a.hpp", "int é = 1; int target(int);\r\nint other;\n")
	path := filepath.Join(dir, "a.hpp")
	set := newSourceSet(dir, []string{"a.hpp"})
	loc := func(line, col int) srcLoc { return srcLoc{file: path, line: line, col: col, valid: true} }
	t.Run("the name stands there", func(t *testing.T) {
		// "int " 4 bytes, e-acute 2 bytes, " = 1; int " 10 bytes: byte column 17, UTF-16 column 15.
		line, col, length, ok := set.place(loc(1, 17), "target")
		if !ok || line != 0 || col != 15 || length != 6 {
			t.Errorf("place = %d %d %d %v, want 0 15 6 true", line, col, length, ok)
		}
	})
	t.Run("a line ending of Windows is not part of the line", func(t *testing.T) {
		if text, _ := set.file(path).line(1); strings.HasSuffix(text, "\r") {
			t.Errorf("line = %q", text)
		}
	})
	t.Run("the second line", func(t *testing.T) {
		if line, col, _, ok := set.place(loc(2, 5), "other"); !ok || line != 1 || col != 4 {
			t.Errorf("place = %d %d %v", line, col, ok)
		}
	})
	t.Run("another name there is refused", func(t *testing.T) {
		if _, _, _, ok := set.place(loc(1, 17), "targets"); ok {
			t.Error("accepted a name that does not stand at the place")
		}
		if _, _, _, ok := set.place(loc(1, 16), "target"); ok {
			t.Error("accepted a place in the middle of the text")
		}
	})
	t.Run("out of range", func(t *testing.T) {
		unplaced := []srcLoc{loc(9, 1), loc(1, 0), loc(1, 999), {valid: false},
			{file: "", line: 1, col: 1, valid: true}}
		for _, l := range unplaced {
			if _, _, _, ok := set.place(l, "int"); ok {
				t.Errorf("accepted %+v", l)
			}
		}
	})
	t.Run("only files of the run", func(t *testing.T) {
		if got := set.resolve("a.hpp"); got != path {
			t.Errorf("resolve(a.hpp) = %q", got)
		}
		if got := set.resolve("b.hpp"); got != "" {
			t.Errorf("resolve(b.hpp) = %q, want a refusal", got)
		}
	})
}

func TestSourceSetFileLimits(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "big.hpp", strings.Repeat("x", maxLocateFileBytes+1))
	set := newSourceSet(dir, []string{"big.hpp"})
	if _, ok := set.file(filepath.Join(dir, "big.hpp")).line(1); ok {
		t.Error("a header over the limit was read")
	}
}

func TestUTF16Units(t *testing.T) {
	for text, want := range map[string]int{"": 0, "abc": 3, "é": 1, "€": 1, "\U0001F600": 2, "a\xffb": 3} {
		if got := utf16Units(text); got != want {
			t.Errorf("utf16Units(%q) = %d, want %d", text, got, want)
		}
	}
}

func TestTidyDocBounds(t *testing.T) {
	long := make([]string, 0, maxDocLines+5)
	for range maxDocLines + 5 {
		long = append(long, " line")
	}
	got := tidyDoc(long)
	if strings.Count(got, "\n") != maxDocLines || !strings.HasSuffix(got, "\n...") {
		t.Errorf("lines = %d, tail %q", strings.Count(got, "\n"), got[max(0, len(got)-8):])
	}
	if got := tidyDoc([]string{strings.Repeat("y", maxDocBytes+100)}); len(got) > maxDocBytes+4 {
		t.Errorf("a long line is %d bytes", len(got))
	}
	if got := tidyDoc([]string{"", "  ", ""}); got != "" {
		t.Errorf("blank lines = %q", got)
	}
}

func TestTrailingCommentMarks(t *testing.T) {
	dir := t.TempDir()
	writeHeader(t, dir, "c.hpp", "int a; /**< block after */\nint b;  ///< line after\n"+
		"int c; /* plain */\nint d; // plain line  \nchar *e = \"//\";\n")
	path := filepath.Join(dir, "c.hpp")
	set := newSourceSet(dir, []string{"c.hpp"})
	for line, want := range []string{"block after", "line after", "plain", "plain line", ""} {
		at := srcLoc{file: path, line: line + 1, col: 5, valid: true}
		if got := set.trailingComment(at, at); got != want {
			t.Errorf("line %d: comment %q, want %q", line+1, got, want)
		}
	}
}

func TestPathKey(t *testing.T) {
	for _, c := range []struct {
		a, b string
		fold bool
		same bool
	}{
		{`C:\Inc\A.hpp`, `c:\inc\a.hpp`, true, true},
		{`C:\Inc\A.hpp`, `c:/inc/a.hpp`, true, true},
		{`\\Server\Share\A.hpp`, `\\server\share\a.hpp`, true, true},
		{`C:\Inc\A.hpp`, `C:\Inc\B.hpp`, true, false},
		{"/Inc/A.hpp", "/inc/a.hpp", false, false},
		{"/Inc/A.hpp", "/Inc/A.hpp", false, true},
	} {
		if got := pathKey(c.a, c.fold) == pathKey(c.b, c.fold); got != c.same {
			t.Errorf("pathKey(%q) == pathKey(%q) with fold %v: %v, want %v", c.a, c.b, c.fold, got, c.same)
		}
	}
}

func TestSourceSetResolveFoldsCaseWhereTheFileSystemDoes(t *testing.T) {
	dir := t.TempDir()
	dep := filepath.Join(dir, "Inc", "Api.hpp")
	spelled := filepath.Join(dir, "inc", "api.hpp")
	if got := newSourceSetFold(dir, []string{dep}, true).resolve(spelled); got != dep {
		t.Errorf("resolve with folding = %q, want the dependency's own spelling %q", got, dep)
	}
	if got := newSourceSetFold(dir, []string{dep}, false).resolve(spelled); got != "" {
		t.Errorf("resolve without folding = %q, want a refusal", got)
	}
}

// The rest of the paths of Windows can only be told by Windows' own filepath.
func TestWindowsPathsOfTheSourceSet(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows paths")
	}
	dir := t.TempDir() // C:\Users\...\TestWindowsPaths...\001
	if got := normalizePath(dir, `inc/api.hpp`); got != filepath.Join(dir, "inc", "api.hpp") {
		t.Errorf("a relative path with a slash = %q", got)
	}
	if got := normalizePath(dir, `inc\..\api.hpp`); got != "" {
		t.Errorf("a dot-dot through a folder that is not there = %q, want a refusal", got)
	}
	if got := normalizePath(`\\server\share\work`, `inc\api.hpp`); got != `\\server\share\work\inc\api.hpp` {
		t.Errorf("a path on a share = %q", got)
	}
	if got := normalizePath(dir, `\\server\share\a.hpp`); got != `\\server\share\a.hpp` {
		t.Errorf("an absolute path of a share = %q", got)
	}
	if got := normalizePath(dir, `C:/dir/./x.hpp`); got != `C:\dir\x.hpp` {
		t.Errorf("a drive path with slashes = %q", got)
	}
	writeHeader(t, dir, "A.hpp", "int x;\r\n")
	path := filepath.Join(dir, "A.hpp")
	set := newSourceSet(dir, []string{path})
	for _, spelled := range []string{path, strings.ToLower(path), strings.ReplaceAll(path, `\`, "/")} {
		if got := set.resolve(spelled); got != path {
			t.Errorf("resolve(%q) = %q, want %q", spelled, got, path)
		}
	}
	if _, col, _, ok := set.place(srcLoc{file: path, line: 1, col: 5, valid: true}, "x"); !ok || col != 4 {
		t.Errorf("place on a CRLF file = col %d ok %v", col, ok)
	}
}

func TestStripCommentMarksStarGutters(t *testing.T) {
	cases := map[string]string{
		"** gutter text": " gutter text",
		"   ***  deep":   "  deep",
		"**":             "",
		"**bold** text":  "*bold** text",
		" * one star":    " one star",
		"*/":             "/",
		"// line":        " line",
	}
	for input, want := range cases {
		if got := stripCommentMarks(input); got != want {
			t.Errorf("stripCommentMarks(%q) = %q, want %q", input, got, want)
		}
	}
}
