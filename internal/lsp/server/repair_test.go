package server

import (
	"fmt"
	"go/token"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/syntax/parser"
	"github.com/huuhait/gocpp/internal/syntax/scanner"
)

func parsed(src string) *cache.ParsedFile {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "x.gp", []byte(src), cache.ParseFull)
	return &cache.ParsedFile{Path: "x.gp", Src: []byte(src), Fset: fset, Tok: fset.File(f.FileStart), File: f, Err: err}
}

const repairSrc = `package x

import "fmt"

// A is fine.
func A() int { return 1 }

#Trace()
#Wrap()
func B(n int) int {
	fmt.Println(n
	return n
}

type T struct{ X int }

var V = A()

func C() {
	if V > 0 {
		fmt.Println("c")
	}
}
`

// TestDeclStarts: the lines that start a declaration; a # line above a func belongs to it.
func TestDeclStarts(t *testing.T) {
	src := []byte(repairSrc)
	var got []string
	for _, off := range declStarts(src) {
		line, _, _ := strings.Cut(string(src[off:]), "\n")
		got = append(got, line)
	}
	sameList(t, "starts", got, []string{"func A() int { return 1 }", "#Trace()", "type T struct{ X int }", "var V = A()", "func C() {"})
}

// TestDeclStartsLexicalBoundaries: declaration-looking lines inside literals/comments are never recovery boundaries.
// A scanner error in an unterminated raw string/comment does not make its remaining text into declarations either.
func TestDeclStartsLexicalBoundaries(t *testing.T) {
	fake := "func fake() {}\ntype Fake int\nvar fakeValue int\nconst fakeConst = 1\nmacro FakeMacro() {}\n#Fake()"
	for _, tc := range []struct{ name, inside string }{
		{"raw string", "_ = `\n" + fake + "\n`\n"},
		{"block comment", "/*\n" + fake + "\n*/\n"},
		{"line comments", "// " + strings.ReplaceAll(fake, "\n", "\n// ") + "\n"},
		{"CRLF raw string", strings.ReplaceAll("_ = `\n"+fake+"\n`\n", "\n", "\r\n")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package p\nfunc broken() int {\n" + tc.inside + "return broken(\n}\nfunc caller() {}\n"
			starts := declStarts([]byte(src))
			if len(starts) != 2 || starts[0] != strings.Index(src, "func broken") || starts[1] != strings.Index(src, "func caller") {
				t.Fatalf("literal/comment produced declaration boundaries: %v", starts)
			}
		})
	}
	for _, open := range []string{"_ = `", "/*"} {
		t.Run("unterminated "+open, func(t *testing.T) {
			src := "package p\nfunc broken() {\n" + open + "\n" + fake + "\n"
			starts := declStarts([]byte(src))
			if len(starts) != 1 || starts[0] != strings.Index(src, "func broken") {
				t.Errorf("uncertain lexical tail produced declaration boundaries: %v", starts)
			}
		})
	}
}

// TestRepairLexicalCallerHover: a real syntax error after a closed literal/comment must blank only the broken
// function's body. Its intact signature and its healthy caller's inferred type survive regardless of literal text.
func TestRepairLexicalCallerHover(t *testing.T) {
	for _, tc := range []struct{ name, inside string }{
		{"raw keyword", "_ = `\nfunc fake() {}\n`\n"},
		{"raw control", "_ = `\nfaux fake() {}\n`\n"},
		{"comment keyword", "/*\nfunc fake() {}\n*/\n"},
		{"comment control", "/*\nfaux fake() {}\n*/\n"},
		{"line comment", "// func fake() {}\n"},
		{"raw CRLF", "_ = `\r\nfunc fake() {}\r\n`\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package lib\nfunc broken() int {\n" + tc.inside + "return 1\n}\nfunc caller() { value := broken(); _ = value }\n"
			c, _ := startStale(t, map[string]string{"gocpp.mod": "module example.com/m\n", "lib/lib.gp": src})
			c.open("lib/lib.gp", src)
			marks := []string{"value := bro|ken()", "_ = va|lue }"}
			clean := make([]string, len(marks))
			for i, mark := range marks {
				text, rng := c.hover("lib/lib.gp", src, mark)
				clean[i] = text + "|" + rng
			}
			bad := strings.Replace(src, "return 1", "return broken(", 1)
			c.change("lib/lib.gp", 2, bad)
			for i, mark := range marks {
				text, rng := c.hover("lib/lib.gp", bad, mark)
				if text == "" || text+"|"+rng != clean[i] {
					t.Errorf("at %q: repaired %q (%q), clean %q", mark, text, rng, clean[i])
				}
			}
			r := repairText(parsed(bad), nil)
			if r == nil || len(r.blanked) != 1 || !strings.Contains(string(r.text), "func broken() int {") {
				t.Fatalf("intact signature lost: %+v", r)
			}
			blanked := bad[r.blanked[0][0]:r.blanked[0][1]]
			if strings.Contains(blanked, "func broken") || strings.Contains(blanked, "func caller") {
				t.Errorf("repair blanked a signature or healthy caller: %q", blanked)
			}
		})
	}
}

// TestRepairText: the declaration with the first syntax error is blanked, nothing else, the line ends and the length stay, and
// the result parses; a file that cannot be repaired gives nothing.
func TestRepairText(t *testing.T) {
	pf := parsed(repairSrc)
	if pf.Err == nil {
		t.Fatal("the source has no error")
	}
	r := repairText(pf, nil)
	if r == nil {
		t.Fatal("no repair")
	}
	if len(r.text) != len(repairSrc) || strings.Count(string(r.text), "\n") != strings.Count(repairSrc, "\n") {
		t.Errorf("the repair changed the length or the lines")
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "x.gp", r.text, cache.ParseFull); err != nil {
		t.Errorf("the repair does not parse: %v", err)
	}
	if len(r.blanked) != 1 {
		t.Fatalf("blanked %v", r.blanked)
	}
	b := repairSrc[r.blanked[0][0]:r.blanked[0][1]]
	if !strings.Contains(b, "fmt.Println(n") || strings.Contains(b, "func B") || strings.Contains(b, "type T") {
		t.Errorf("the blanked body: %q", b)
	}
	// the signature of B and the macros on top of it stay, so that its callers check; the body is a panic
	if !strings.Contains(string(r.text), "#Trace()\n#Wrap()\nfunc B(n int) int {\npanic(\"\")") {
		t.Errorf("the repaired function:\n%s", r.text)
	}
	for _, want := range []string{"func A() int { return 1 }", "type T struct{ X int }", "func C() {"} {
		if !strings.Contains(string(r.text), want) {
			t.Errorf("the repair lost %q", want)
		}
	}
	if strings.TrimSpace(strings.Replace(string(r.text[r.blanked[0][0]:r.blanked[0][1]]), `panic("")`, "", 1)) != "" {
		t.Errorf("the blanked part is not white space")
	}
	if !r.within(r.blanked[0][0]) || !r.within(r.blanked[0][1]-1) || r.within(r.blanked[0][1]) || r.within(r.blanked[0][0]-1) {
		t.Errorf("within is wrong at the edges of %v", r.blanked)
	}

	// the signature is what is wrong: the whole declaration goes
	sig := strings.Replace(repairSrc, "func B(n int) int {", "func B(n int int) int {", 1)
	if r := repairText(parsed(sig), nil); r == nil || len(r.blanked) != 1 || !strings.Contains(sig[r.blanked[0][0]:r.blanked[0][1]], "func B(n int int)") {
		t.Errorf("a broken signature: %+v", r)
	}
	// a load error at an offset (a macro that is not there) in a file that parses: that declaration's body goes
	good := strings.Replace(repairSrc, "fmt.Println(n\n", "fmt.Println(n)\n", 1)
	off := strings.Index(good, "fmt.Println(\"c\")")
	if r := repairText(parsed(good), []int{off}); r == nil || len(r.blanked) != 1 || !strings.Contains(good[r.blanked[0][0]:r.blanked[0][1]], `Println("c")`) || strings.Contains(good[r.blanked[0][0]:r.blanked[0][1]], "func C") {
		t.Errorf("a load error: %+v", r)
	}
	if r := repairText(parsed(good), nil); r != nil {
		t.Errorf("a good file was repaired: %+v", r)
	}
	// an unclosed brace: the body of the function that has it goes
	brace := strings.Replace(repairSrc, "fmt.Println(n\n", "fmt.Println(n)\n\tif n > 0 {\n", 1)
	r = repairText(parsed(brace), nil)
	if r == nil || len(r.blanked) != 1 || !strings.Contains(brace[r.blanked[0][0]:r.blanked[0][1]], "if n > 0 {") || strings.Contains(brace[r.blanked[0][0]:r.blanked[0][1]], "func B") {
		t.Errorf("an unclosed brace: %+v", r)
	}
	// an error in the header cannot be repaired
	if r := repairText(parsed(strings.Replace(repairSrc, `import "fmt"`, `import fmt"`, 1)), nil); r != nil {
		t.Errorf("a broken import was repaired: %+v", r)
	}
	// too many broken places that the brackets do not show (one is repaired for each parse, up to maxRepairPasses)
	var many strings.Builder
	many.WriteString("package x\n\n")
	for range 2 * maxRepairPasses {
		many.WriteString("func F() {\n\tx := 1 +\n}\n\n")
	}
	if r := repairText(parsed(many.String()), nil); r != nil {
		t.Errorf("a file with %d broken functions was repaired", 2*maxRepairPasses)
	}
}

// TestUnbalanced: the declarations whose brackets do not match, by the scanner (brackets in strings and comments are
// nothing).
func TestUnbalanced(t *testing.T) {
	src := []byte("package x\n\nfunc I() { _ = a[1] + b[c[2]] }\n\nfunc A() { _ = \"(\" /* { */ }\n\nfunc B() {\n\tf(1\n}\n\nfunc G() { _ = a)(b }\n\nfunc C() {\n}\n}\n\nvar D = [3]int{1, 2\n\nfunc E() {}\n")
	starts := declStarts(src)
	var got []string
	for i, bad := range unbalanced(src, starts) {
		line, _, _ := strings.Cut(string(src[starts[i]:]), " ")
		got = append(got, fmt.Sprint(line, " ", bad))
	}
	sameList(t, "unbalanced", got, []string{"func false", "func false", "func true", "func true", "func true", "var true", "func false"})
}

// TestRepairTwoPlaces: two broken functions that the brackets do not show are two passes of the repair, and both
// are blanked (a repair of one pass would give up), the healthy one between them stays.
func TestRepairTwoPlaces(t *testing.T) {
	src := "package x\n\nfunc A() {\n\tx := 1 +\n}\n\nfunc Fine() int { return 1 }\n\nfunc B() {\n\ty := 2 +\n}\n"
	r := repairText(parsed(src), nil)
	if r == nil || len(r.blanked) != 2 {
		t.Fatalf("repair: %+v", r)
	}
	for _, sp := range r.blanked {
		if b := src[sp[0]:sp[1]]; strings.Contains(b, "Fine") || !strings.Contains(b, "+") {
			t.Errorf("blanked %q", b)
		}
	}
	if !strings.Contains(string(r.text), "func Fine() int { return 1 }") {
		t.Errorf("the healthy function is gone: %s", r.text)
	}
}

// TestRepairEarliestError: cascading errors on a later healthy declaration must not make it disappear. The error
// list need not be ordered: choose the earliest byte offset, not its first/last entry or the parser's furthest error.
func TestRepairEarliestError(t *testing.T) {
	src := "package x\n\nfunc Broken() {\n\tx := 1 +\n}\n\nfunc Fine() int { return 1 }\n"
	pf := parsed(src)
	early, late := strings.Index(src, "1 +"), strings.Index(src, "func Fine")
	for _, offsets := range [][]int{{early, late}, {late, early}} {
		list := scanner.ErrorList{}
		for _, off := range offsets {
			list.Add(token.Position{Filename: pf.Path, Offset: off, Line: 1, Column: 1}, "syntax error")
		}
		pf.Err = list
		r := repairText(pf, nil)
		if r == nil || !strings.Contains(string(r.text), "func Fine() int { return 1 }") {
			t.Errorf("errors at %v lost the healthy function: %+v", offsets, r)
		}
	}
}

// TestBodySpanTypeBraces: the braces of a struct or interface in the signature are not the body.
func TestBodySpanTypeBraces(t *testing.T) {
	for _, head := range []string{"func F() struct{ a int } ", "func F() (x struct{ a, b int }) ", "func F(i interface{ M() }) interface{} ", "func (r *R) F() (s struct{}, e error) "} {
		src := []byte(head + "{\n\tbody()\n}\n")
		lo, hi, ok := bodySpan(src, 0, len(src))
		if !ok || string(src[lo:hi]) != "\n\tbody()\n" {
			t.Errorf("%q: body %q (%v)", head, src[lo:hi], ok)
		}
	}
	// a macro, and macros on top
	src := []byte("#A()\n#B(func() {})\nmacro M(x int) {\n\tbody()\n}\n")
	if lo, hi, ok := bodySpan(src, 0, len(src)); !ok || string(src[lo:hi]) != "\n\tbody()\n" {
		t.Errorf("macro: %q (%v)", src[lo:hi], ok)
	}
	// not a function
	if _, _, ok := bodySpan([]byte("type T struct{ x int }\n"), 0, 23); ok {
		t.Errorf("a type has a body")
	}
}
