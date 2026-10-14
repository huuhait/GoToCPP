package cppir

import (
	"testing"
)

func at(file string, line int) Pos {
	return Pos{File: file, Line: line}
}

// TestWriterLines checks where the writer puts #line directives: before a mapped line whose
// line is not the previous one + 1, counting the plain lines too.
func TestWriterLines(t *testing.T) {
	var w writer
	w.plain("// header")
	w.blank()
	w.line(at("/a.gp", 6), "f() {") // the first mapped line always needs one
	w.indent()
	w.line(at("/a.gp", 7), "x;") // 7 follows 6: none
	w.line(at("/a.gp", 7), "y;") // a second line for .gp line 7: again
	w.line(at("/a.gp", 8), "z;") // two lines for 8: the second one again
	w.line(at("/a.gp", 8), "zz;")
	w.line(at("/a.gp", 3), "back;")  // backwards
	w.line(at("/b.gp", 4), "other;") // another file, the next line number
	w.line(Pos{}, "plain;")          // no place: plain, and it takes line 5
	w.line(at("/b.gp", 6), "six;")   // 6 follows the plain line 5: none
	w.dedent()
	w.blank()                   // takes 7
	w.line(at("/b.gp", 8), "}") // none
	want := `// header

#line 6 "/a.gp"
f() {
    x;
#line 7 "/a.gp"
    y;
    z;
#line 8 "/a.gp"
    zz;
#line 3 "/a.gp"
    back;
#line 4 "/b.gp"
    other;
    plain;
    six;

}
`
	if got := w.buf.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

// TestWriterOwnLines checks that with the generated file's path, a plain line after mapped
// lines switches back to the generated file, at its real line number, and a blank one doesn't.
func TestWriterOwnLines(t *testing.T) {
	w := writer{out: "/work/main.gp.cpp"}
	w.plain("// header")             // 1: the compiler counts the file's own lines here
	w.line(at("/a.gp", 6), "f() {")  // 2, 3
	w.line(at("/a.gp", 8), "}")      // 4, 5
	w.blank()                        // 6: blank, it stays at a.gp:9
	w.line(at("/a.gp", 10), "g() {") // 7
	w.plain("}")                     // 8: #line 9 "/work/main.gp.cpp", 9
	w.plain("wrapper() {")           // 10
	w.plain("}")                     // 11
	w.line(at("/a.gp", 3), "h();")   // 12, 13
	w.blank()                        // 14
	w.plain("x;")                    // 15: #line 16 ..., 16
	want := `// header
#line 6 "/a.gp"
f() {
#line 8 "/a.gp"
}

g() {
#line 9 "/work/main.gp.cpp"
}
wrapper() {
}
#line 3 "/a.gp"
h();

#line 16 "/work/main.gp.cpp"
x;
`
	if got := w.buf.String(); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestCString(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", `""`},
		{"hello, gocpp", `"hello, gocpp"`},
		{`C:\work\hello.gp`, `"C:\\work\\hello.gp"`},
		{"/home/me/my project/a.gp", `"/home/me/my project/a.gp"`},
		{`say "hi"`, `"say \"hi\""`},
		{"é", `"\303\251"`},
		{"\x00\x01\t\n\x7f\xff", `"\000\001\t\n\177\377"`},
		{"\a\b\f\n\r\t\v", `"\a\b\f\n\r\t\v"`},
		{"\n1\t7", `"\n1\t7"`},      // a named escape takes no digit that follows
		{"\x01" + "23", `"\00123"`}, // 3 digits: the 2 and the 3 stay characters
		{"??=", `"?\?="`},           // GCC and Clang warn about trigraphs even in C++20
		{"a???/b?", `"a?\?\?/b?"`},
	}
	for _, tt := range tests {
		if got := CString(tt.in); got != tt.want {
			t.Errorf("CString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

// TestWriterNewline checks that a newline in a line's text, which would break the count of
// the #line directives, is a panic (a bug in the tree), with and without a place.
func TestWriterNewline(t *testing.T) {
	for _, pos := range []Pos{at("/a.gp", 1), {}} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("no panic for a newline at %v", pos)
				}
			}()
			var w writer
			w.line(pos, "a;\nb;")
		}()
	}
}
