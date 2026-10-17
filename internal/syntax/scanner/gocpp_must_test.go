// gocpp: the tests of patch 4 (../FORK.md §4): a name directly followed by
// !( or ![ is one name, F!, in Phase10Syntax mode, for must calls and must
// functions. This file is new in the patch.

package scanner_test

import (
	"go/token"
	"strconv"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// muScan scans src in the mode and returns the tokens as "IDENT(x) ! (":
// a name or a literal with its text, a keyword or an operator as itself,
// and the offset of each IDENT with a !. A scan error fails the test.
func muScan(t *testing.T, src string, mode scanner.Mode) string {
	t.Helper()
	fset := token.NewFileSet()
	file := fset.AddFile("test.gp", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		t.Errorf("scanning %q: %s: %s", src, pos, msg)
	}, mode)
	var out []string
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return strings.Join(out, " ")
		}
		d := tok.String()
		if tok.IsLiteral() || tok == token.COMMENT {
			d += "(" + lit + ")"
		}
		if tok == token.IDENT && strings.HasSuffix(lit, "!") {
			// The name and its position: the ! is part of the token.
			off := fset.Position(pos).Offset
			if src[off:off+len(lit)] != lit {
				t.Errorf("scanning %q: IDENT(%s) at offset %d, but the source there is %q", src, lit, off, src[off:off+len(lit)])
			}
		}
		out = append(out, d)
	}
}

// TestMustNames checks the tokens of must names in both modes. Without
// Phase10Syntax, the scanner scans Go: a name, !, and the rest.
func TestMustNames(t *testing.T) {
	for _, tc := range []struct {
		src       string
		goTokens  string // Phase10Syntax off
		gocppToks string // Phase10Syntax on
	}{
		// The forms of the plan.
		{"F!(x)", "IDENT(F) ! ( IDENT(x) ) ;", "IDENT(F!) ( IDENT(x) ) ;"},
		{"os.ReadFile!(p)", "IDENT(os) . IDENT(ReadFile) ! ( IDENT(p) ) ;", "IDENT(os) . IDENT(ReadFile!) ( IDENT(p) ) ;"},
		{"parse![int](s)", "IDENT(parse) ! [ IDENT(int) ] ( IDENT(s) ) ;", "IDENT(parse!) [ IDENT(int) ] ( IDENT(s) ) ;"},
		{"f!()", "IDENT(f) ! ( ) ;", "IDENT(f!) ( ) ;"},
		{"x := f!()", "IDENT(x) := IDENT(f) ! ( ) ;", "IDENT(x) := IDENT(f!) ( ) ;"},
		{"func f!() T {", "func IDENT(f) ! ( ) IDENT(T) {", "func IDENT(f!) ( ) IDENT(T) {"},
		{"func (s *S) Start!() {", "func ( IDENT(s) * IDENT(S) ) IDENT(Start) ! ( ) {", "func ( IDENT(s) * IDENT(S) ) IDENT(Start!) ( ) {"},
		{"func parse![T any](s string) T {", "func IDENT(parse) ! [ IDENT(T) IDENT(any) ] ( IDENT(s) IDENT(string) ) IDENT(T) {", "func IDENT(parse!) [ IDENT(T) IDENT(any) ] ( IDENT(s) IDENT(string) ) IDENT(T) {"},
		{"defer f.Close!()", "defer IDENT(f) . IDENT(Close) ! ( ) ;", "defer IDENT(f) . IDENT(Close!) ( ) ;"},
		{"é!(x)", "IDENT(é) ! ( IDENT(x) ) ;", "IDENT(é!) ( IDENT(x) ) ;"},
		{"_!(x)", "IDENT(_) ! ( IDENT(x) ) ;", "IDENT(_!) ( IDENT(x) ) ;"},
		{"a!(b)\nc", "IDENT(a) ! ( IDENT(b) ) ; IDENT(c) ;", "IDENT(a!) ( IDENT(b) ) ; IDENT(c) ;"},

		// Not a must name: another token after the !, or a space.
		{"a!=b", "IDENT(a) != IDENT(b) ;", "IDENT(a) != IDENT(b) ;"},
		{"a != (b)", "IDENT(a) != ( IDENT(b) ) ;", "IDENT(a) != ( IDENT(b) ) ;"},
		{"a !(b)", "IDENT(a) ! ( IDENT(b) ) ;", "IDENT(a) ! ( IDENT(b) ) ;"},
		{"a! (b)", "IDENT(a) ! ( IDENT(b) ) ;", "IDENT(a) ! ( IDENT(b) ) ;"},
		{"a!\n(b)", "IDENT(a) ! ( IDENT(b) ) ;", "IDENT(a) ! ( IDENT(b) ) ;"},
		{"a!/* c */(b)", "IDENT(a) ! COMMENT(/* c */) ( IDENT(b) ) ;", "IDENT(a) ! COMMENT(/* c */) ( IDENT(b) ) ;"},
		{"a!b", "IDENT(a) ! IDENT(b) ;", "IDENT(a) ! IDENT(b) ;"},
		{"a!", "IDENT(a) ! ", "IDENT(a) ! "},
		{"a!{", "IDENT(a) ! {", "IDENT(a) ! {"},
		{"a!!(b)", "IDENT(a) ! ! ( IDENT(b) ) ;", "IDENT(a) ! ! ( IDENT(b) ) ;"},
		{"a[i]!(b)", "IDENT(a) [ IDENT(i) ] ! ( IDENT(b) ) ;", "IDENT(a) [ IDENT(i) ] ! ( IDENT(b) ) ;"},
		{"f(x)!(b)", "IDENT(f) ( IDENT(x) ) ! ( IDENT(b) ) ;", "IDENT(f) ( IDENT(x) ) ! ( IDENT(b) ) ;"},
		{"1!(b)", "INT(1) ! ( IDENT(b) ) ;", "INT(1) ! ( IDENT(b) ) ;"},
		{"f(!b)", "IDENT(f) ( ! IDENT(b) ) ;", "IDENT(f) ( ! IDENT(b) ) ;"},
		{"x = !(a)", "IDENT(x) = ! ( IDENT(a) ) ;", "IDENT(x) = ! ( IDENT(a) ) ;"},

		// A keyword is not a name: these are valid Go.
		{"if!(a) {", "if ! ( IDENT(a) ) {", "if ! ( IDENT(a) ) {"},
		{"return!(x)", "return ! ( IDENT(x) ) ;", "return ! ( IDENT(x) ) ;"},
		{"case!(b):", "case ! ( IDENT(b) ) :", "case ! ( IDENT(b) ) :"},
		{"for!(a) {", "for ! ( IDENT(a) ) {", "for ! ( IDENT(a) ) {"},
		{"switch!(a) {", "switch ! ( IDENT(a) ) {", "switch ! ( IDENT(a) ) {"},
		{"go!(a)", "go ! ( IDENT(a) ) ;", "go ! ( IDENT(a) ) ;"},
		{"defer!(a)", "defer ! ( IDENT(a) ) ;", "defer ! ( IDENT(a) ) ;"},
		{"range!(a)", "range ! ( IDENT(a) ) ;", "range ! ( IDENT(a) ) ;"},
		{"else!(a)", "else ! ( IDENT(a) ) ;", "else ! ( IDENT(a) ) ;"},
		{"select!(a)", "select ! ( IDENT(a) ) ;", "select ! ( IDENT(a) ) ;"},
		{"if!x {", "if ! IDENT(x) {", "if ! IDENT(x) {"},
		{"if !x!=y {", "if ! IDENT(x) != IDENT(y) {", "if ! IDENT(x) != IDENT(y) {"},

		// A keyword after a dot is a name (patch 1), but not a must name:
		// the C++ names of patch 1 are never Go functions with errors.
		{"x.type!(a)", "IDENT(x) . IDENT(type) ! ( IDENT(a) ) ;", "IDENT(x) . IDENT(type) ! ( IDENT(a) ) ;"},
		{"x.select![a]", "IDENT(x) . IDENT(select) ! [ IDENT(a) ] ;", "IDENT(x) . IDENT(select) ! [ IDENT(a) ] ;"},

		// Comments, strings and numbers.
		{"// a!(b)\nc", "COMMENT(// a!(b)) IDENT(c) ;", "COMMENT(// a!(b)) IDENT(c) ;"},
		{"/* a!(b) */ c", "COMMENT(/* a!(b) */) IDENT(c) ;", "COMMENT(/* a!(b) */) IDENT(c) ;"},
		{`"a!(b)"`, `STRING("a!(b)") ;`, `STRING("a!(b)") ;`},
		{"a!(b) // c!(d)", "IDENT(a) ! ( IDENT(b) ) COMMENT(// c!(d)) ;", "IDENT(a!) ( IDENT(b) ) COMMENT(// c!(d)) ;"},
	} {
		// Without comments in the token list, the tests above would need two
		// versions: the scanner here always returns them.
		if got := muScan(t, tc.src, scanner.ScanComments); got != strings.TrimSpace(tc.goTokens) {
			t.Errorf("Go mode: scanning %q:\ngot  %s\nwant %s", tc.src, got, tc.goTokens)
		}
		if got := muScan(t, tc.src, scanner.ScanComments|scanner.Phase10Syntax); got != strings.TrimSpace(tc.gocppToks) {
			t.Errorf("Phase10Syntax: scanning %q:\ngot  %s\nwant %s", tc.src, got, tc.gocppToks)
		}
	}
}

// TestMustNamePositions checks the position and the end of a must name:
// the ! is part of the token, so a name that starts at offset n with k
// bytes in its literal ends at n+k.
func TestMustNamePositions(t *testing.T) {
	src := "x := os.ReadFile!(p)\nfunc f!() {}"
	fset := token.NewFileSet()
	file := fset.AddFile("test.gp", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), func(pos token.Position, msg string) { t.Errorf("%s: %s", pos, msg) }, scanner.Phase10Syntax)
	var got []string
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		if strings.HasSuffix(lit, "!") {
			p := fset.Position(pos)
			got = append(got, lit+"@"+strconv.Itoa(p.Line)+":"+strconv.Itoa(p.Column))
		}
	}
	if want := "ReadFile!@1:9 f!@2:6"; strings.Join(got, " ") != want {
		t.Errorf("got %v, want %s", got, want)
	}
}

// TestMustNamesAreOptIn checks that without Phase10Syntax, nothing of
// patch 4 shows: the tokens of a Go file are the same.
func TestMustNamesAreOptIn(t *testing.T) {
	src := "package p\nfunc f() { if!(a) {}; x := a!=b; y := !c }\n"
	if a, b := muScan(t, src, scanner.ScanComments), muScan(t, src, scanner.ScanComments|scanner.Phase10Syntax); a != b {
		t.Errorf("the tokens of Go code change with Phase10Syntax:\n%s\n%s", a, b)
	}
}
