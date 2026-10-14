package main

import (
	"strings"
	"testing"
)

// TestGoHeader checks where goHeader ends the header: like go/build's
// readGoInfo, after the imports, without the byte that ended them, at the
// end of the file after a syntax error of the reader, or at a NUL byte.
func TestGoHeader(t *testing.T) {
	for _, c := range []struct{ src, header string }{
		{"package main\n\nfunc main() {}\n", "package main\n\n"},
		{"package main x\n", "package main "},
		{"package b\n\nimport \"fmt\" x\n", "package b\n\nimport \"fmt\" "},
		{"// doc\n//gocpp:link m\npackage p; import (\n\t\"a\"\n\tb \"b\"\n)\nvar x\n", "// doc\n//gocpp:link m\npackage p; import (\n\t\"a\"\n\tb \"b\"\n)\n"},
		{"package p\nimport . \"a\"\nimport `b`\n", "package p\nimport . \"a\"\nimport `b`\n"}, // ends at EOF
		// Syntax errors of the reader: the whole file.
		{"package main\n\nimport ( \"fmt\" 3 )\n\nfunc main() {}\n", ""},
		{"package \"x\"\n", ""},
		{"package p\nimport \"unterminated\n", ""},
		{"package p /* no end", ""},
		// A byte order mark stays in the header.
		{"\ufeffpackage p\nvar x\n", "\ufeffpackage p\n"},
	} {
		want := c.header
		if want == "" {
			want = c.src
		}
		if n, nul := goHeader([]byte(c.src)); c.src[:n] != want || nul {
			t.Errorf("goHeader(%q) = %q, %v; want %q, false", c.src, c.src[:n], nul, want)
		}
	}
	// A NUL byte: the header up to it.
	for _, src := range []string{"package p\x00\nimport \"a\"\n", "package p\nimport \"a\x00\"\n", "package p\n\x00"} {
		want := src[:strings.IndexByte(src, 0)+1]
		if n, nul := goHeader([]byte(src)); src[:n] != want || !nul {
			t.Errorf("goHeader(%q) = %q, %v; want %q, true", src, src[:n], nul, want)
		}
	}
	// After a syntax error of the reader, a NUL byte anywhere: the whole
	// file.
	src := "package main\n\nimport ( \"fmt\" 3 )\n\nvar x = \"a\x00b\"\n"
	if n, nul := goHeader([]byte(src)); n != len(src) || !nul {
		t.Errorf("goHeader(%q) = %d, %v; want %d, true", src, n, nul, len(src))
	}
	// No header at all.
	if n, _ := goHeader(nil); n != 0 {
		t.Errorf("goHeader(nil) = %d, want 0", n)
	}
	if src := strings.Repeat("x", 10); func() int { n, _ := goHeader([]byte(src)); return n }() != len(src) {
		t.Errorf("goHeader(%q) is not the whole file", src)
	}
}
