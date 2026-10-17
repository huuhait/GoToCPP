package format

import (
	"bytes"
	goformat "go/format"
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/printer"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// The tests in this file check what is gocpp's own: the import groups with
// headers, gocpp syntax, //gocpp: directives, and the errors. The tests in
// gofmt_test.go and stdlib_test.go check that everything else is gofmt's.

// errorText returns the text that gocpp fmt prints for err, one error per
// line, like gofmt prints its errors.
func errorText(err error) string {
	var b strings.Builder
	scanner.PrintError(&b, err)
	return b.String()
}

// A formatCase is a source file and what Source makes of it.
type formatCase struct {
	name string
	src  string
	want string // the formatted source; "" means that src is already formatted
}

func runFormatCases(t *testing.T, cases []formatCase) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if want == "" {
				want = tc.src
			}
			got, err := Source("x.gp", []byte(tc.src))
			if err != nil {
				t.Fatalf("Source: %v", err)
			}
			if string(got) != want {
				t.Fatalf("Source gives\n%s\nwant\n%s", got, want)
			}
			// gofmt's output doesn't change when gofmt formats it again,
			// and the same must hold here.
			again, err := Source("x.gp", got)
			if err != nil || !bytes.Equal(again, got) {
				t.Errorf("formatting the result again gives\n%s\n(error %v), want no change", again, err)
			}
		})
	}
}

// TestImportGroups checks gocpp's rule for import groups: a group with a
// header or a namespace import stays as it is written (order and
// duplicates), and every other group is sorted like gofmt sorts it.
func TestImportGroups(t *testing.T) {
	runFormatCases(t, []formatCase{{
		// gofmt would sort <windows.h> first, which breaks the build on
		// Windows: <winsock2.h> must come before it.
		name: "headers keep their order",
		src: `package p

import (
	"<winsock2.h>"
	"<windows.h>"

	"<GL/glew.h>"
	"<GL/gl.h>"
)
`,
	}, {
		name: "a group of gocpp packages is sorted and deduplicated",
		src: `package p

import (
	"os"
	"fmt"
	"fmt"
)
`,
		want: `package p

import (
	"fmt"
	"os"
)
`,
	}, {
		// The style of the spec: the gocpp imports, then the headers in a
		// group of their own.
		name: "a gocpp group is sorted, the header group after it is not",
		src: `package p

import (
	"strings"
	"fmt"

	"<vector>"
	"<algorithm>"
	"./physics.h"
)
`,
		want: `package p

import (
	"fmt"
	"strings"

	"<vector>"
	"<algorithm>"
	"./physics.h"
)
`,
	}, {
		name: "a mixed group stays as written, also its duplicates",
		src: `package p

import (
	"os"
	"<vector>"
	"fmt"
	"fmt"
)
`,
	}, {
		name: "namespace imports keep their order",
		src: `package p

import (
	fs "::std::filesystem"
	cfmt "::fmt"
)
`,
	}, {
		name: "a namespace import keeps a gocpp group as written",
		src: `package p

import (
	"strings"
	chrono "::std::chrono"
	"fmt"
)
`,
	}, {
		name: "local headers keep their order",
		src: `package p

import (
	"./physics.h"
	"./audio.h"
	"native/x.hpp"
	"../shared/b.h"
)
`,
	}, {
		name: "renamed and blank headers keep their order",
		src: `package p

import (
	rl "<raylib.h>"
	_ "<GL/gl.h>"
	cstring "<string.h>"
)
`,
	}, {
		// The path only needs the shape of a header. gocpp check reports
		// the error.
		name: "a broken header path keeps the group as written",
		src: `package p

import (
	"strings"
	"<raylib.h"
	"fmt"
)
`,
	}, {
		// "./physics" is an error (a local path must be a header), but it
		// is not a header, so the group is sorted like any other.
		name: "a ./ path that is not a header is sorted",
		src: `package p

import (
	"strings"
	"./physics"
)
`,
		want: `package p

import (
	"./physics"
	"strings"
)
`,
	}, {
		// The group keeps its order, but the printer still does the rest
		// of gofmt's work: the space after a name, the alignment of the
		// comments, and the empty line before ")" that gofmt removes.
		name: "a header group gets gofmt's other formatting",
		src: `package p

import (
	"fmt"

	rl    "<raylib.h>" // raylib
	"<cstdio>" // printf
	/* c */ "<cmath>"

)
`,
		want: `package p

import (
	"fmt"

	rl "<raylib.h>" // raylib
	"<cstdio>"      // printf
	/* c */ "<cmath>"
)
`,
	}, {
		// A comment line ends a group, as in gofmt: the header after it
		// is in a group of its own, and the group before it is sorted.
		name: "a comment line ends a group",
		src: `package p

import (
	"strings"
	"fmt"
	// The headers.
	"<windows.h>"
	"<winsock2.h>"
)
`,
		want: `package p

import (
	"fmt"
	"strings"
	// The headers.
	"<windows.h>"
	"<winsock2.h>"
)
`,
	}, {
		// gofmt removes a duplicate by merging its line in the file's line
		// table, which moves the lines after it. The header group between
		// two such groups keeps its order and its empty lines.
		name: "duplicates go away around a header group",
		src: `package p

import (
	"os"
	"fmt"
	"fmt"

	"<b.h>"
	"<a.h>"

	"z"
	"y"
	"y" // y
)
`,
		want: `package p

import (
	"fmt"
	"os"

	"<b.h>"
	"<a.h>"

	"y" // y
	"z"
)
`,
	}, {
		// gofmt sorts only inside one import declaration.
		name: "import declarations without parentheses stay as they are",
		src: `package p

import "<winsock2.h>"
import "<windows.h>"
import "os"
import "fmt"
`,
	}, {
		name: "every group of the file and every declaration is handled on its own",
		src: `package p

import (
	"strings"
	"fmt"

	"<b.h>"
	"<a.h>"
)

import (
	"os"
	"io"
)

import (
	"<d.h>"
	"<c.h>"
)
`,
		want: `package p

import (
	"fmt"
	"strings"

	"<b.h>"
	"<a.h>"
)

import (
	"io"
	"os"
)

import (
	"<d.h>"
	"<c.h>"
)
`,
	}})
}

// TestGocppSyntax checks that gocpp code that the stock go/parser rejects
// is formatted: a Go keyword after "." is a name (parser patch 1).
func TestGocppSyntax(t *testing.T) {
	const src = `package p

import (
	"<map>"
	posix "<sys/select.h>"
)

func f(node *Node, fd int) {
	m:=std.map[std.string,int]{}
	t:=node.type( )
	n:=posix.select(fd+1,nil,nil,nil,nil)
	node.range(func(){ m.clear() })
	_,_,_=m,t,n
}
`
	runFormatCases(t, []formatCase{{
		name: "keywords after a dot",
		src:  src,
		want: `package p

import (
	"<map>"
	posix "<sys/select.h>"
)

func f(node *Node, fd int) {
	m := std.map[std.string, int]{}
	t := node.type()
	n := posix.select(fd+1, nil, nil, nil, nil)
	node.range(func() { m.clear() })
	_, _, _ = m, t, n
}
`,
	}})

	// This is why gocpp fmt can't use go/format: it parses with the
	// stock go/parser.
	if _, err := goformat.Source([]byte(src)); err == nil {
		t.Error("go/format formats std.map; then gocpp fmt could use it (phase 01 README §2.11)")
	}
}

// TestDirectiveErrors checks that a file with errors in its //gocpp:
// directives is not formatted: Source returns the errors and no source.
// go/printer would turn the misspelled directive in the doc comment into
// an ordinary comment ("// gocpp:Link m").
func TestDirectiveErrors(t *testing.T) {
	for _, tc := range []struct {
		name, src, errs string
	}{{
		name: "a misspelled directive in the doc comment",
		src:  "// Package p uses libm.\n//gocpp:Link m\npackage  p\n",
		errs: `x.gp:2:1: unknown //gocpp: directive "Link"; did you mean //gocpp:link?` + "\n",
	}, {
		name: "a directive after the package clause",
		src:  "package  p\n\n//gocpp:link m\nfunc f() {}\n",
		errs: "x.gp:3:1: //gocpp: directives must come before the package clause\n",
	}, {
		name: "two errors, in the order of the file",
		src:  "//gocpp:link -lm\n//gocpp:pkgconfig sdl2\n\npackage  p\n",
		errs: `x.gp:1:14: invalid library name "-lm": write m, without -l` + "\n" +
			`x.gp:2:1: unknown //gocpp: directive "pkgconfig"; did you mean //gocpp:pkg-config?` + "\n",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Source("x.gp", []byte(tc.src))
			if got != nil {
				t.Errorf("Source returned source:\n%s\nwant none", got)
			}
			if _, ok := err.(scanner.ErrorList); !ok {
				t.Fatalf("Source returned the error %#v, want a scanner.ErrorList", err)
			}
			if text := errorText(err); text != tc.errs {
				t.Errorf("errors:\n%s\nwant:\n%s", text, tc.errs)
			}
		})
	}
}

// TestDirectivesFormatted checks files whose directives have no errors:
// they are formatted like gofmt formats //go: directives, which it moves
// to the end of a doc comment. A comment that only looks like a broken
// directive gets a warning from gocpp check, but it is an ordinary
// comment, and the file is formatted.
func TestDirectivesFormatted(t *testing.T) {
	runFormatCases(t, []formatCase{{
		name: "directives in the doc comment",
		src:  "// Package p uses libm.\n//gocpp:link m\n//More text.\npackage  p\n",
		want: "// Package p uses libm.\n// More text.\n//\n//gocpp:link m\npackage p\n",
	}, {
		name: "directives in a comment of their own",
		src:  "//gocpp:pkg-config sdl2\n//gocpp:cflags -DX=1 // comment\n\npackage  p\n",
		want: "//gocpp:pkg-config sdl2\n//gocpp:cflags -DX=1 // comment\n\npackage p\n",
	}, {
		name: "a comment that looks like a broken directive",
		src:  "// gocpp:link m\npackage  p\n",
		want: "// gocpp:link m\npackage p\n",
	}})
}

// TestSyntaxErrors checks that a file with syntax errors is not formatted,
// like in gofmt: Source returns the parser's errors (the same ones as
// go/parser's) and no source. The errors are the ones that gofmt prints
// for the same file.
func TestSyntaxErrors(t *testing.T) {
	got, err := Source("x.gp", []byte("package p\n\nfunc f( {\n}\nvar = 1\n"))
	if got != nil {
		t.Errorf("Source returned source:\n%s\nwant none", got)
	}
	const want = "x.gp:3:9: expected ')', found '{'\n" +
		"x.gp:4:1: missing ',' in parameter list\n" +
		"x.gp:5:1: expected ')', found 'var'\n"
	if text := errorText(err); text != want {
		t.Errorf("errors:\n%s\nwant:\n%s", text, want)
	}
	// The syntax errors come first: a directive error is not reported
	// with them.
	_, err = Source("x.gp", []byte("//gocpp:lnk m\npackage p\nfunc {\n"))
	if text, want := errorText(err), "x.gp:3:6: expected 'IDENT', found '{'\n"; text != want {
		t.Errorf("errors:\n%s\nwant:\n%s", text, want)
	}
}

// TestFragment checks Fragment, which formats what gocpp fmt reads from
// standard input: a whole file, or a list of declarations or statements,
// like gofmt. gofmt_test.go runs gofmt's own fragment tests (stdin*.input)
// too.
func TestFragment(t *testing.T) {
	for _, tc := range []struct {
		name, src, want string
	}{{
		name: "a complete file",
		src:  "package  p\nvar x=1\n",
		want: "package p\n\nvar x = 1\n",
	}, {
		name: "declarations",
		src:  "func f(){}\nvar x=1\n",
		want: "func f() {}\n\nvar x = 1\n",
	}, {
		// The space around the code and the indentation of the first line
		// stay.
		name: "statements",
		src:  "\n\n\tx:=1\n\tif x>0{ y()}\n\n",
		want: "\n\n\tx := 1\n\tif x > 0 {\n\t\ty()\n\t}\n\n",
	}, {
		name: "an expression",
		src:  "a+b *c",
		want: "a + b*c",
	}, {
		name: "gocpp syntax",
		src:  "m:=std.map[std.string,int]{}",
		want: "m := std.map[std.string, int]{}",
	}, {
		// A fragment can't have directives, so a //gocpp: line is not
		// checked: it is only a comment here. (gofmt prints the two spaces
		// before "{" in the same way.)
		name: "a //gocpp: line in a fragment",
		src:  "//gocpp:Link m\nfunc f(){}\n",
		want: "//gocpp:Link m\nfunc f()  {}\n",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Fragment("<standard input>", []byte(tc.src))
			if err != nil {
				t.Fatalf("Fragment: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("Fragment gives %q, want %q", got, tc.want)
			}
		})
	}

	// A complete file gets the checks of Source.
	_, err := Fragment("<standard input>", []byte("//gocpp:Link m\npackage p\n"))
	const want = `<standard input>:1:1: unknown //gocpp: directive "Link"; did you mean //gocpp:link?` + "\n"
	if text := errorText(err); text != want {
		t.Errorf("errors:\n%s\nwant:\n%s", text, want)
	}
	// Source doesn't take fragments.
	if _, err := Source("x.gp", []byte("var x = 1\n")); err == nil {
		t.Error("Source formats a fragment, want an error")
	}
}

// TestNormalizeNumbers pins the value of normalizeNumbers, the unexported
// mode of go/printer that gofmt uses: with it, the number literals get
// their canonical form, and without it they stay as they are written. If
// a new Go changes the value, the first check fails.
func TestNormalizeNumbers(t *testing.T) {
	for _, tc := range []struct{ lit, want string }{
		{"0XFF", "0xFF"}, {"0xabc", "0xabc"}, {"1E5", "1e5"}, {"1E+5", "1e+5"}, {"1.5E-3", "1.5e-3"},
		{"0X1P-2", "0x1p-2"}, {"0x1P3", "0x1p3"}, {"0B101", "0b101"}, {"0O17", "0o17"}, {"017", "017"},
		{"1_000", "1_000"}, {"0X_FF", "0x_FF"},
		// Imaginary literals: an integer one also loses its leading zeros.
		{"0123i", "123i"}, {"00i", "0i"}, {"0_1i", "1i"}, {"1E2i", "1e2i"}, {"0X1P3i", "0x1p3i"},
		{"0B1i", "0b1i"}, {"0O7i", "0o7i"}, {"00.5i", "00.5i"},
	} {
		src := "package p\n\nconst c = " + tc.lit + "\n"
		got, err := Source("x.gp", []byte(src))
		if err != nil {
			t.Fatalf("%s: %v", tc.lit, err)
		}
		if want := "package p\n\nconst c = " + tc.want + "\n"; string(got) != want {
			t.Errorf("Source prints %s as %q, want %q", tc.lit, strings.TrimPrefix(string(got), "package p\n\nconst c = "), tc.want+"\n")
		}

		// Without the mode, the printer keeps the literal.
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, "x.gp", src, ParserMode)
		if err != nil {
			t.Fatal(err)
		}
		cfg := config
		cfg.Mode &^= normalizeNumbers
		var b bytes.Buffer
		if err := cfg.Fprint(&b, fset, f); err != nil {
			t.Fatal(err)
		}
		if b.String() != src {
			t.Errorf("without normalizeNumbers, %s prints as %q, want it unchanged", tc.lit, b.String())
		}
	}
	// The mode is none of the exported ones.
	if normalizeNumbers&(printer.RawFormat|printer.TabIndent|printer.UseSpaces|printer.SourcePos) != 0 {
		t.Errorf("normalizeNumbers (%#x) is an exported printer mode", uint(normalizeNumbers))
	}
}
