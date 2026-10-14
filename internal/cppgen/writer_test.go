package cppgen

import (
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func at(file string, line int) token.Position {
	return token.Position{Filename: file, Line: line, Column: 1}
}

// TestWriterLines checks where the writer puts #line directives: before a mapped line whose
// line is not the previous one + 1, counting the plain lines too.
func TestWriterLines(t *testing.T) {
	var w writer
	w.plain("// header")
	w.blank()
	w.line(at("/a.gp", 6), "f() {") // the first mapped line always needs one
	w.indent()
	w.line(at("/a.gp", 7), "x;")       // 7 follows 6: none
	w.line(at("/a.gp", 7), "y;")       // a second line for .gp line 7: again
	w.line(at("/a.gp", 8), "z;\nzz;")  // two lines for 8: the second one again
	w.line(at("/a.gp", 3), "back;")    // backwards
	w.line(at("/b.gp", 4), "other;")   // another file, the next line number
	w.line(token.Position{}, "plain;") // no place: plain, and it takes line 5
	w.line(at("/b.gp", 6), "six;")     // 6 follows the plain line 5: none
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
	w.plain("}\nwrapper() {")        // 8: #line 9 "/work/main.gp.cpp", 9, 10
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
		{"\x00\x01\t\n\x7f\xff", `"\000\001\011\012\177\377"`},
		{"\x01" + "23", `"\00123"`}, // 3 digits: the 2 and the 3 stay characters
		{"??=", `"?\?="`},           // GCC and Clang warn about trigraphs even in C++20
		{"a???/b?", `"a?\?\?/b?"`},
	}
	for _, tt := range tests {
		if got := cString(tt.in); got != tt.want {
			t.Errorf("cString(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

// TestLineFileNames checks with each compiler that a #line file name that cString wrote
// reads back as the same bytes: a Windows path, spaces, quotes and UTF-8.
func TestLineFileNames(t *testing.T) {
	names := []string{
		`C:\work\hello.gp`,
		`C:\Users\Me\My Project\x"y.gp`,
		"/home/người dùng/app/main.gp",
		"/tmp/what??=/a??/b.gp",
	}
	dir := t.TempDir()
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			for i, name := range names {
				var w writer
				w.plain("#include <cstdio>")
				w.line(at(name, 42), "int main() { std::fputs(__FILE__, stdout); return __LINE__; }")
				src := filepath.Join(dir, fmt.Sprintf("%s-%d.cpp", c.name, i))
				if err := os.WriteFile(src, w.buf.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
				bin := strings.TrimSuffix(src, ".cpp")
				out, _, code, err := command(dir, true, c.cc.Path, append(c.flags(), src, "-o", bin)...)
				if err != nil || code != 0 || len(out) != 0 {
					t.Fatalf("compile %s: exit code %d %v\n%s", src, code, err, out)
				}
				r := run(t, dir, bin)
				if r.stdout != name || r.code != 42 {
					t.Errorf("__FILE__ is %q and __LINE__ %d, want %q and 42", r.stdout, r.code, name)
				}
			}
		})
	}
}
