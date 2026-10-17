// gocpp: the tests of patch 5 (../FORK.md §4): a # directly followed by a
// letter is one name, #Name, in Phase10Syntax mode, for macros. This file
// is new in the patch.

package scanner_test

import (
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

// mnScan scans src in the mode and returns the tokens like muScan
// (gocpp_must_test.go) and the scan errors as "offset: message".
func mnScan(src string, mode scanner.Mode) (toks string, errs []string) {
	fset := token.NewFileSet()
	file := fset.AddFile("test.gp", fset.Base(), len(src))
	var s scanner.Scanner
	s.Init(file, []byte(src), func(pos token.Position, msg string) {
		errs = append(errs, pos.String()[len("test.gp:"):]+": "+msg)
	}, mode)
	var out []string
	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			return strings.Join(out, " "), errs
		}
		d := tok.String()
		if tok.IsLiteral() || tok == token.COMMENT || tok == token.ILLEGAL {
			d += "(" + lit + ")"
		}
		out = append(out, d)
	}
}

// TestMacroNames checks the tokens of macro names with Phase10Syntax, and
// that without it a # is an illegal character, as in Go.
func TestMacroNames(t *testing.T) {
	for _, tc := range []struct {
		src  string
		want string // with Phase10Syntax
		goGo string // without; "" if the same
		errs int    // the number of errors without Phase10Syntax
	}{
		{"#Name(x)", "IDENT(#Name) ( IDENT(x) ) ;", "ILLEGAL(#) IDENT(Name) ( IDENT(x) ) ;", 1},
		{"#util.Dbg(x)", "IDENT(#util) . IDENT(Dbg) ( IDENT(x) ) ;", "ILLEGAL(#) IDENT(util) . IDENT(Dbg) ( IDENT(x) ) ;", 1},
		{"#Dbg[int](x)", "IDENT(#Dbg) [ IDENT(int) ] ( IDENT(x) ) ;", "ILLEGAL(#) IDENT(Dbg) [ IDENT(int) ] ( IDENT(x) ) ;", 1},
		{"#file() #line()", "IDENT(#file) ( ) IDENT(#line) ( ) ;", "ILLEGAL(#) IDENT(file) ( ) ILLEGAL(#) IDENT(line) ( ) ;", 2},
		{"#_x1()", "IDENT(#_x1) ( ) ;", "ILLEGAL(#) IDENT(_x1) ( ) ;", 1},
		{"#é()", "IDENT(#é) ( ) ;", "ILLEGAL(#) IDENT(é) ( ) ;", 1},
		{"#a", "IDENT(#a) ;", "ILLEGAL(#) IDENT(a) ;", 1},
		{"#a\nb", "IDENT(#a) ; IDENT(b) ;", "ILLEGAL(#) IDENT(a) ; IDENT(b) ;", 1},
		// A keyword after the # is part of the name.
		{"#func()", "IDENT(#func) ( ) ;", "ILLEGAL(#) func ( ) ;", 1},
		{"#type #range #return", "IDENT(#type) IDENT(#range) IDENT(#return) ;", "ILLEGAL(#) type ILLEGAL(#) range ILLEGAL(#) return ;", 3},
		{"#body()", "IDENT(#body) ( ) ;", "ILLEGAL(#) IDENT(body) ( ) ;", 1},
		// Not a name: no letter after the #.
		{"# x", "ILLEGAL(#) IDENT(x) ;", "", 1},
		{"#", "ILLEGAL(#)", "", 1},
		{"#1", "ILLEGAL(#) INT(1) ;", "", 1},
		{"#(x)", "ILLEGAL(#) ( IDENT(x) ) ;", "", 1},
		{"##x", "ILLEGAL(#) IDENT(#x) ;", "ILLEGAL(#) ILLEGAL(#) IDENT(x) ;", 2},
		{"#!x", "ILLEGAL(#) ! IDENT(x) ;", "", 1},
		{"#\"s\"", "ILLEGAL(#) STRING(\"s\") ;", "", 1},
		{"#\u00a0x", "ILLEGAL(#) ILLEGAL(\u00a0) IDENT(x) ;", "", 2},
		// A # in a comment, a string or a rune is not a name.
		{"// #x\ny", "COMMENT(// #x) IDENT(y) ;", "COMMENT(// #x) IDENT(y) ;", 0},
		{"/* #x */ y", "COMMENT(/* #x */) IDENT(y) ;", "COMMENT(/* #x */) IDENT(y) ;", 0},
		{"\"#x\" `#y` '#'", "STRING(\"#x\") STRING(`#y`) CHAR('#') ;", "STRING(\"#x\") STRING(`#y`) CHAR('#') ;", 0},
		// Both new names together.
		{"#Trace(f!(x))", "IDENT(#Trace) ( IDENT(f!) ( IDENT(x) ) ) ;", "ILLEGAL(#) IDENT(Trace) ( IDENT(f) ! ( IDENT(x) ) ) ;", 1},
		{"x := #Dbg(a != b)", "IDENT(x) := IDENT(#Dbg) ( IDENT(a) != IDENT(b) ) ;", "IDENT(x) := ILLEGAL(#) IDENT(Dbg) ( IDENT(a) != IDENT(b) ) ;", 1},
	} {
		got, errs := mnScan(tc.src, scanner.ScanComments|scanner.Phase10Syntax)
		wantErrs := 0
		if tc.want != "" && strings.Contains(tc.want, "ILLEGAL") {
			wantErrs = strings.Count(tc.want, "ILLEGAL")
		}
		if got != tc.want || len(errs) != wantErrs {
			t.Errorf("Phase10Syntax: scanning %q:\ngot  %s (%d errors: %q)\nwant %s (%d errors)", tc.src, got, len(errs), errs, tc.want, wantErrs)
		}
		want := tc.goGo
		if want == "" {
			want = tc.want
		}
		got, errs = mnScan(tc.src, scanner.ScanComments)
		if got != want || len(errs) != tc.errs {
			t.Errorf("Go mode: scanning %q:\ngot  %s (%d errors: %q)\nwant %s (%d errors)", tc.src, got, len(errs), errs, want, tc.errs)
		}
	}
}

// TestMacroNameErrors checks the text and the place of the error of a #
// that is no name: it is Go's error.
func TestMacroNameErrors(t *testing.T) {
	_, errs := mnScan("x := #\ny := # z", scanner.Phase10Syntax)
	want := []string{"1:6: illegal character U+0023 '#'", "2:6: illegal character U+0023 '#'"}
	if strings.Join(errs, "|") != strings.Join(want, "|") {
		t.Errorf("got %q, want %q", errs, want)
	}
}

// TestMacroNamePositions checks that a macro name starts at its #, and that
// the semicolon after it comes at the end of the line.
func TestMacroNamePositions(t *testing.T) {
	src := "x := #util.Dbg(1)\n#Trace\n"
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
		if strings.HasPrefix(lit, "#") || tok == token.SEMICOLON {
			p := fset.Position(pos)
			got = append(got, strings.TrimSpace(lit)+"@"+p.String()[len("test.gp:"):])
		}
	}
	if want := "#util@1:6 @1:18 #Trace@2:1 @2:7"; strings.Join(got, " ") != want {
		t.Errorf("got %q, want %q", strings.Join(got, " "), want)
	}
}

// TestMacroNamesAreOptIn checks that without Phase10Syntax nothing of
// patch 5 shows: the tokens of a Go file with no # are the same.
func TestMacroNamesAreOptIn(t *testing.T) {
	src := "package p\n// #x\nfunc f() { s := \"#y\"; r := '#' /* #z */ }\n"
	a, _ := mnScan(src, scanner.ScanComments)
	b, _ := mnScan(src, scanner.ScanComments|scanner.Phase10Syntax)
	if a != b {
		t.Errorf("the tokens of Go code change with Phase10Syntax:\n%s\n%s", a, b)
	}
}
