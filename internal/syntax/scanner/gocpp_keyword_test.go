// gocpp: the tests of patch 1 (../FORK.md §4): a Go keyword right after a
// '.' on the same line is a name, for C++ names such as std::map and
// node.type(). This file is new in the patch.

package scanner_test

import (
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// kwKeywords are the 25 keywords of the Go spec.
var kwKeywords = []string{
	"break", "case", "chan", "const", "continue",
	"default", "defer", "else", "fallthrough", "for",
	"func", "go", "goto", "if", "import",
	"interface", "map", "package", "range", "return",
	"select", "struct", "switch", "type", "var",
}

// kwToken is one token that Scan returned.
type kwToken struct {
	tok token.Token
	lit string
	pos token.Position
}

// kwScan scans src in the given mode and returns its tokens, without the
// EOF. A scan error fails the test.
func kwScan(t *testing.T, src string, mode scanner.Mode) []kwToken {
	t.Helper()
	fset := token.NewFileSet()
	file := fset.AddFile("test.gp", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		t.Errorf("scanning %q: %s: %s", src, pos, msg)
	}, mode)
	var toks []kwToken
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return toks
		}
		toks = append(toks, kwToken{tok, lit, fset.Position(pos)})
	}
}

// kwDescribe writes toks like "IDENT(x) . IDENT(map) ;": a name or a literal
// with its text, a keyword or an operator as itself.
func kwDescribe(toks []kwToken) string {
	var b strings.Builder
	for i, t := range toks {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(t.tok.String())
		if t.tok.IsLiteral() {
			b.WriteString("(" + t.lit + ")")
		}
	}
	return b.String()
}

// kwCheck scans src twice, with and without scanner.ScanComments, and
// compares the tokens with want (written for ScanComments; without it,
// the COMMENT tokens are left out).
func kwCheck(t *testing.T, src, want string) {
	t.Helper()
	if got := kwDescribe(kwScan(t, src, scanner.ScanComments)); got != want {
		t.Errorf("scanning %q with comments:\ngot  %s\nwant %s", src, got, want)
	}
	var fields []string
	for _, f := range strings.Fields(want) {
		if f != "COMMENT" {
			fields = append(fields, f)
		}
	}
	want = strings.Join(fields, " ")
	if got := kwDescribe(kwScan(t, src, 0)); got != want {
		t.Errorf("scanning %q without comments:\ngot  %s\nwant %s", src, got, want)
	}
}

// TestKeywordList checks kwKeywords against go/token, so that the tests
// below cover every keyword.
func TestKeywordList(t *testing.T) {
	n := 0
	for tok := token.ILLEGAL; tok <= token.TILDE; tok++ { // TILDE is the last token of go/token
		if tok.IsKeyword() {
			n++
		}
	}
	if n != len(kwKeywords) {
		t.Errorf("go/token has %d keywords, kwKeywords has %d", n, len(kwKeywords))
	}
	for _, kw := range kwKeywords {
		if !token.IsKeyword(kw) {
			t.Errorf("%q is not a keyword in go/token", kw)
		}
	}
}

func TestKeywordAfterDot(t *testing.T) {
	for _, test := range []struct{ src, want string }{
		// The examples of the plan (phase 01 README §2.2).
		{"m := std.map[std.string, int]{}", "IDENT(m) := IDENT(std) . IDENT(map) [ IDENT(std) . IDENT(string) , IDENT(int) ] { } ;"},
		{"t := node.type()", "IDENT(t) := IDENT(node) . IDENT(type) ( ) ;"},
		{"n := posix.select(fd+1, &rd, nil, nil, nil)", "IDENT(n) := IDENT(posix) . IDENT(select) ( IDENT(fd) + INT(1) , & IDENT(rd) , IDENT(nil) , IDENT(nil) , IDENT(nil) ) ;"},

		// Space and comments on the same line don't matter.
		{"x . map", "IDENT(x) . IDENT(map) ;"},
		{"x.\t\tmap", "IDENT(x) . IDENT(map) ;"},
		{"x.\rmap", "IDENT(x) . IDENT(map) ;"}, // a lone \r doesn't end a line in Go
		{"x./* c */map", "IDENT(x) . COMMENT IDENT(map) ;"},
		{"x. /* a */ /* b */ map", "IDENT(x) . COMMENT COMMENT IDENT(map) ;"},

		// Chains, and a '.' and its keyword on a later line.
		{"x.map.range.type()", "IDENT(x) . IDENT(map) . IDENT(range) . IDENT(type) ( ) ;"},
		{"a.b\nx.func", "IDENT(a) . IDENT(b) ; IDENT(x) . IDENT(func) ;"},

		// The rule looks at the lines of the source, not at the
		// positions that a line directive gives.
		{"x./*line other.gp:9:1*/map", "IDENT(x) . COMMENT IDENT(map) ;"},

		// A keyword on the next line stays a keyword: an editor often
		// has an unfinished "fmt." there (TestIncompleteSelection).
		{"x.\nmap", "IDENT(x) . map"},
		{"x.\r\nmap", "IDENT(x) . map"},
		{"x. // c\nmap", "IDENT(x) . COMMENT map"},
		{"x./*\n*/map", "IDENT(x) . COMMENT map"},
		{"fmt.\ntype X int", "IDENT(fmt) . type IDENT(X) IDENT(int) ;"},

		// Only right after the '.'.
		{"x.y map", "IDENT(x) . IDENT(y) map"},
		{"x.y(map[int]int{})", "IDENT(x) . IDENT(y) ( map [ IDENT(int) ] IDENT(int) { } ) ;"},

		// Type assertions and type switches: there, '(' follows the '.'.
		{"x.(type)", "IDENT(x) . ( type ) ;"},
		{"x. (type)", "IDENT(x) . ( type ) ;"},
		{"x.(map[string]int)", "IDENT(x) . ( map [ IDENT(string) ] IDENT(int) ) ;"},
		{"x.(func())", "IDENT(x) . ( func ( ) ) ;"},
		{"x.(chan int)", "IDENT(x) . ( chan IDENT(int) ) ;"},
		{"x.(interface{})", "IDENT(x) . ( interface { } ) ;"},
		{"x.(struct{})", "IDENT(x) . ( struct { } ) ;"},

		// "...", and the '.' of a number, are not a '.' token. A '.'
		// right after them is one.
		{"func f(a ...map[string]int)", "func IDENT(f) ( IDENT(a) ... map [ IDENT(string) ] IDENT(int) ) ;"},
		{"[...]struct{}{}", "[ ... ] struct { } { } ;"},
		{"x := 1. + .5", "IDENT(x) := FLOAT(1.) + FLOAT(.5) ;"},
		{"1.map", "FLOAT(1.) map"},
		{"x.5", "IDENT(x) FLOAT(.5) ;"},
		{"1.e5.map", "FLOAT(1.e5) . IDENT(map) ;"},
		{"....map", "... . IDENT(map) ;"},

		// A dot import: a string follows the '.'.
		{`import . "fmt"`, `import . STRING("fmt") ;`},
		{"import (\n\t. \"fmt\"\n)", `import ( . STRING("fmt") ; ) ;`},
	} {
		kwCheck(t, test.src, test.want)
	}
}

func TestEveryKeywordAfterDot(t *testing.T) {
	for _, kw := range kwKeywords {
		// On the same line, the keyword is a name, so at the end of the
		// source it gets the automatic semicolon of a name.
		name := "IDENT(" + kw + ")"
		kwCheck(t, "x."+kw, "IDENT(x) . "+name+" ;")
		kwCheck(t, "x . "+kw, "IDENT(x) . "+name+" ;")
		kwCheck(t, "x./* c */"+kw, "IDENT(x) . COMMENT "+name+" ;")
		kwCheck(t, "x."+kw+"()", "IDENT(x) . "+name+" ( ) ;")

		// The name keeps the word as its literal, and its position.
		if toks := kwScan(t, "x . "+kw, 0); len(toks) == 4 {
			if got := toks[2]; got.tok != token.IDENT || got.lit != kw || got.pos.Offset != 4 || got.pos.Line != 1 || got.pos.Column != 5 {
				t.Errorf(`scanning "x . %s": got %s %q at %s (offset %d), want IDENT %q at test.gp:1:5 (offset 4)`,
					kw, got.tok, got.lit, got.pos, got.pos.Offset, kw)
			}
		}

		// On the next line, it stays a keyword. Four keywords get an
		// automatic semicolon at the end, as in upstream.
		want := "IDENT(x) . " + kw
		switch kw {
		case "break", "continue", "fallthrough", "return":
			want += " ;"
		}
		kwCheck(t, "x.\n"+kw, want)
		kwCheck(t, "x. // c\n"+kw, strings.Replace(want, ". ", ". COMMENT ", 1))
		kwCheck(t, "x./*\n*/"+kw, strings.Replace(want, ". ", ". COMMENT ", 1))
	}
}

// TestKeywordAfterDotSemicolon checks that a keyword that became a name gets
// the automatic semicolon of a name at the end of a line.
func TestKeywordAfterDotSemicolon(t *testing.T) {
	for _, test := range []struct {
		src  string
		want string
		semi int // the offset of the automatic semicolon
	}{
		{"v := x.range\n", "IDENT(v) := IDENT(x) . IDENT(range) ;", 12},
		{"v := x.range", "IDENT(v) := IDENT(x) . IDENT(range) ;", 12},
		{"v := x.range // c\n", "IDENT(v) := IDENT(x) . IDENT(range) COMMENT ;", 17},
		{"v := x.range /* c */\n", "IDENT(v) := IDENT(x) . IDENT(range) COMMENT ;", 20},
		{"v := x.range /*\n*/", "IDENT(v) := IDENT(x) . IDENT(range) COMMENT ;", 15},
	} {
		kwCheck(t, test.src, test.want)
		for _, mode := range []scanner.Mode{0, scanner.ScanComments} {
			toks := kwScan(t, test.src, mode)
			last := toks[len(toks)-1]
			if last.tok != token.SEMICOLON || last.lit != "\n" || last.pos.Offset != test.semi {
				t.Errorf("scanning %q (mode %d): the last token is %s %q at offset %d, want an automatic semicolon at offset %d",
					test.src, mode, last.tok, last.lit, last.pos.Offset, test.semi)
			}
		}
	}
	kwCheck(t, "v := x.range\nw := y.map\n", "IDENT(v) := IDENT(x) . IDENT(range) ; IDENT(w) := IDENT(y) . IDENT(map) ;")
}

func TestKeywordAfterDotPositions(t *testing.T) {
	const src = "a.b\nstd . /* c */ map[k]\n\tx.\n\ttype"
	pos := func(offset, line, column int) token.Position {
		return token.Position{Filename: "test.gp", Offset: offset, Line: line, Column: column}
	}
	want := []kwToken{
		{token.IDENT, "a", pos(0, 1, 1)},
		{token.PERIOD, "", pos(1, 1, 2)},
		{token.IDENT, "b", pos(2, 1, 3)},
		{token.SEMICOLON, "\n", pos(3, 1, 4)},
		{token.IDENT, "std", pos(4, 2, 1)},
		{token.PERIOD, "", pos(8, 2, 5)},
		{token.COMMENT, "/* c */", pos(10, 2, 7)},
		{token.IDENT, "map", pos(18, 2, 15)},
		{token.LBRACK, "", pos(21, 2, 18)},
		{token.IDENT, "k", pos(22, 2, 19)},
		{token.RBRACK, "", pos(23, 2, 20)},
		{token.SEMICOLON, "\n", pos(24, 2, 21)},
		{token.IDENT, "x", pos(26, 3, 2)},
		{token.PERIOD, "", pos(27, 3, 3)},
		{token.TYPE, "type", pos(30, 4, 2)},
	}
	got := kwScan(t, src, scanner.ScanComments)
	if len(got) != len(want) {
		t.Fatalf("scanning %q: got %d tokens, want %d:\n%s", src, len(got), len(want), kwDescribe(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d: got %s %q at %s (offset %d), want %s %q at %s (offset %d)", i,
				got[i].tok, got[i].lit, got[i].pos, got[i].pos.Offset,
				want[i].tok, want[i].lit, want[i].pos, want[i].pos.Offset)
		}
	}
}

// TestKeywordAfterDotInit checks that Init forgets a '.' of the source
// before.
func TestKeywordAfterDotInit(t *testing.T) {
	fset := token.NewFileSet()
	var s scanner.Scanner
	src := []byte("x.")
	s.Init(fset.AddFile("a.gp", fset.Base(), len(src)), src, nil, 0)
	s.Scan()
	if _, tok, _ := s.Scan(); tok != token.PERIOD {
		t.Fatalf("scanning %q: got %s, want .", src, tok)
	}
	src = []byte("map")
	s.Init(fset.AddFile("b.gp", fset.Base(), len(src)), src, nil, 0)
	if _, tok, lit := s.Scan(); tok != token.MAP {
		t.Errorf("scanning %q after Init: got %s %q, want the keyword map", src, tok, lit)
	}
}
