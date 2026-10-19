package server

import (
	"strings"
	"testing"
)

// TestDiagnosticsBuiltinMacros: the built-in macros (step 10.8) expand in the loader and keep the
// positions of the text, so the diagnostics of a document with them point at what the user wrote,
// also on a line where an emoji comes before the use (UTF-16 columns).
func TestDiagnosticsBuiltinMacros(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{
		"gocpp.mod": "module example.com/m\n",
		// The loader's errors stop the type check, as in gocpp check: two packages.
		"main.gp": "package main\n\nimport _ \"embed\"\n\n//go:embed a.txt\nvar a string\n\nfunc main() {\n" +
			"\tvar n int = #file()\n" +
			"\t_ = \"😀\" + #text(x) + undefinedName\n" +
			"\t_, _ = n, a\n" +
			"}\n",
		"bad/bad.gp": "package bad\n\nfunc F() {\n" +
			"\t_ = \"😀\" + #line(1)\n" +
			"\t_ = #embed(\"\")\n" +
			"\t_ = #text()\n" +
			"}\n",
		"a.txt": "x",
	}
	c := startDiag(t, testConfig(), files, nil)
	c.open("main.gp", files["main.gp"])
	// The embedded file is not read by the server: no error for a.txt.
	want(t, c.next("main.gp"), 1,
		"8:13-8:20 error: cannot use (\"main.gp\") (untyped string constant \"main.gp\") as int value in variable declaration\n"+
			"9:23-9:36 error: undefined: undefinedName\n")
	c.open("bad/bad.gp", files["bad/bad.gp"])
	want(t, c.next("bad/bad.gp"), 1,
		"3:18-3:19 error: #line takes no arguments\n"+
			"4:12-4:14 error: #embed : invalid file name\n"+
			"5:5-5:6 error: #text takes one argument: #text(x)\n")
}

const macroNavSrc = "package main\n\nimport (\n\t_ \"embed\"\n\t\"fmt\"\n)\n\n//go:embed a.txt\nvar blob string\n\n" +
	"func where() string { return #func() }\n\n" +
	"func main() {\n\tcount := 3\n\tfmt.Println(#line(), count, #text(count + 1), blob, where(), #embed(\"a.txt\"))\n\tcount++\n}\n"

// TestNavigationAroundBuiltinMacros: hover and rename work on the names around the built-in macros (step
// 10.8), whose expansions keep the span of the use; a hover on the macro itself finds nothing.
func TestNavigationAroundBuiltinMacros(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": macroNavSrc, "a.txt": "x"}
	c := startRefs(t, files, plainInit)
	c.open("main.gp", macroNavSrc)
	for _, tc := range []hoverCase{
		{"#line(), cou|nt,", "var count int", "count"},
		{"blob, wh|ere()", "func where() string", "where"},
		{"va|r blob", "", ""},
		{"var bl|ob string", "var blob string", "blob"},
		{"func wh|ere", "func where() string", "where"},
		{"cou|nt++", "var count int", "count"},
	} {
		text, rng := c.hover("main.gp", macroNavSrc, tc.mark)
		if text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q: %q (range %q), want %q (range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
	// The macro uses are not names: no hover, no rename.
	for _, mark := range []string{"#li|ne()", "#fu|nc()", "#te|xt(count"} {
		if text, _ := c.hover("main.gp", macroNavSrc, mark); strings.Contains(text, "var count") {
			t.Errorf("hover at %q gave %q", mark, text)
		}
	}
	got, err := c.rename("main.gp", "cou|nt := 3", "n")
	if err != nil {
		t.Fatal(err)
	}
	lines := diffLinesAgainst(t, macroNavSrc, got["main.gp"])
	// #text(count + 1) is text, not a use: it keeps its words.
	want := []string{"14: n := 3", "15: fmt.Println(#line(), n, #text(count + 1), blob, where(), #embed(\"a.txt\"))", "16: n++"}
	sameList(t, "the changed lines of main.gp", lines, want)
}

// TestDiagnosticsEmbedInConstants: the server reads no embedded file, so #embed gives a placeholder that is long
// enough for the constant expressions that gocpp check accepts: a bound, an index and a divisor.
func TestDiagnosticsEmbedInConstants(t *testing.T) {
	checkGoroutines(t)
	src := "package main\n\nconst n = len(#embed(\"a.txt\"))\n\nvar arr [n - 1]int\n\nvar c = #embed(\"a.txt\")[2]\n\nconst d = 100 / len(#embed(\"a.txt\"))\n\nfunc main() { _, _, _ = arr, c, d }\n"
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src, "a.txt": "abcdef"}
	c := startDiag(t, testConfig(), files, nil)
	c.open("main.gp", src)
	want(t, c.next("main.gp"), 1, "")
}

// macroFiles is a module with macros in a package and uses in main (step 10.9). The loader turns a macro
// into a function of the same name, with hidden parameters, and a use into a call of it; the features
// must show the macro as the user wrote it and the use at the text of the use.
var macroFiles = map[string]string{
	"gocpp.mod": "module example.com/m\n",
	"util/util.gp": `package util

import "fmt"

// Assert panics when cond is false.
macro Assert(cond bool, msg lazy string) {
	if !cond {
		panic(fmt.Sprintf("%s:%d: %s: %s", #file(), #line(), #text(cond), msg()))
	}
}

macro Dbg[T any](v T) T {
	fmt.Println(#text(v), v)
	return v
}

macro Trace() func() {
	fmt.Println("enter", #func())
	return func() {}
}
`,
	"main.gp": `package main

import "example.com/m/util"

#util.Trace()
func main() {
	x := #util.Dbg(3)
	#util.Assert(x > 1, "small")
	println(x)
}
`,
}

func TestMacroNavigation(t *testing.T) {
	checkGoroutines(t)
	c := startRefs(t, macroFiles, plainInit)
	src, usrc := macroFiles["main.gp"], macroFiles["util/util.gp"]
	c.open("main.gp", src)

	// hover: the macro as declared, not the function with its hidden parameters, at the use (also on
	// top of a func) and at the declaration; the range is the name.
	for _, tc := range []struct{ file, src, mark, want, rng string }{
		{"main.gp", src, "#util.Db|g(3)", "macro Dbg[T any](v T) T", "Dbg"},
		{"main.gp", src, "#util.Tr|ace()", "macro Trace() func()", "Trace"},
		{"main.gp", src, "#util.As|sert(x", "macro Assert(cond bool, msg lazy string)", "Assert"},
		{"util/util.gp", usrc, "macro As|sert", "macro Assert(cond bool, msg lazy string)\nAssert panics when cond is false.", "Assert"},
		{"main.gp", src, "#util.Assert(|x > 1", "var x int", "x"},
	} {
		text, rng := c.hover(tc.file, tc.src, tc.mark)
		if !strings.HasPrefix(text, tc.want) || strings.Contains(text, "gp_") || rng != tc.rng {
			t.Errorf("hover at %q: %q (range %q), want %q (range %q)", tc.mark, text, rng, tc.want, tc.rng)
		}
	}

	// definition: the declaration of the macro, in the other package, from a use inside code and on top
	sameList(t, "definition of #util.Dbg", c.definitions("textDocument/definition", "main.gp", src, "#util.Db|g(3)"),
		[]string{"util/util.gp:12:7-12:10 Dbg"})
	sameList(t, "definition of #util.Trace", c.definitions("textDocument/definition", "main.gp", src, "#util.Tr|ace()"),
		[]string{"util/util.gp:17:7-17:12 Trace"})

	// references and rename: the declaration and the uses, as text (the # stays)
	sameList(t, "references of Dbg", c.references("main.gp", "#util.Db|g(3)", true), []string{"main.gp:7:13 Dbg", "util/util.gp:12:7 Dbg"})
	edited, err := c.rename("main.gp", "#util.Db|g(3)", "Dump")
	if err != nil {
		t.Fatal(err)
	}
	sameList(t, "the changed lines of main.gp", diffLinesAgainst(t, src, edited["main.gp"]), []string{"7: x := #util.Dump(3)"})
	sameList(t, "the changed lines of util.gp", diffLinesAgainst(t, usrc, edited["util/util.gp"]), []string{"12: macro Dump[T any](v T) T {"})

	// A variable and the names of the lowering: no hint, no token for the hidden parameters.
	for _, h := range c.hints("main.gp", nil) {
		if strings.Contains(h, "gp_") {
			t.Errorf("a hint for a hidden name: %q", h)
		}
	}
	for _, tok := range c.semTokens("util/util.gp", nil) {
		if strings.Contains(tok, " gp_") {
			t.Errorf("a token for a hidden name: %q", tok)
		}
	}
	if syms := string(c.docSymbols("util/util.gp")); !strings.Contains(syms, `"name":"#Assert"`) {
		t.Errorf("symbols of util.gp: %s", syms)
	}
}

// TestDiagnosticsMacros: the loader's errors about macros and the errors of go/types in the words of the
// macro, at the text of the use.
func TestDiagnosticsMacros(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{
		"gocpp.mod":    "module example.com/m\n",
		"util/util.gp": macroFiles["util/util.gp"],
		// The loader's errors stop the type check, as in gocpp check: two packages.
		"a/a.gp": "package a\n\nimport \"example.com/m/util\"\n\nfunc F() {\n" +
			"\t#util.Assert(true)\n" +
			"\t#util.Trace()\n" +
			"\t#util.Nope()\n" +
			"}\n",
		"b/b.gp": "package b\n\nimport \"example.com/m/util\"\n\nfunc F() {\n" +
			"\tutil.Assert(true, \"x\")\n" +
			"\t#util.Assert(1, \"x\")\n" +
			"}\n",
	}
	c := startDiag(t, testConfig(), files, nil)
	c.open("a/a.gp", files["a/a.gp"])
	want(t, c.next("a/a.gp"), 1,
		"5:18-5:19 error: #Assert: not enough arguments (have 1, want 2)\n\tmacro Assert is declared at util/util.gp:6\n"+
			"6:1-6:2 error: #Trace returns a func(): as a statement it would drop the func that ends its work. Use it on top of a func, or call the result: f := #Trace(...)\n"+
			"7:7-7:11 error: undefined macro #util.Nope: package util declares no macro Nope\n")
	c.open("b/b.gp", files["b/b.gp"])
	want(t, c.next("b/b.gp"), 1,
		"5:1-5:23 error: util.Assert is a macro: write #util.Assert(...)\n"+
			"6:14-6:15 error: cannot use 1 (untyped int constant) as bool value in argument to #util.Assert\n\tmacro Assert is declared at util/util.gp:6\n")
}

const macroBodySrc = "package main\n\nimport \"example.com/m/util\"\n\n" +
	"#util.Trace()\n#Wrap()\n#util.Trace()\nfunc work(n int) (total int) {\n\tx := n * 2\n\ttotal = x + 1\n\treturn total\n}\n\n" +
	"macro Wrap() {\n\t#body()\n}\n\nfunc main() { println(work(1)) }\n"

// TestMacroWrappedBody: the code of a function that has macros on top is found by its place, also
// when a wrap macro moved it into a func literal (the rewrite makes the nodes nest by position). (A
// start-style macro written after a wrap macro has its variable declared inside the literal: its
// line is not found by hover. Known.)
func TestMacroWrappedBody(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "util/util.gp": macroFiles["util/util.gp"], "main.gp": macroBodySrc}
	c := startRefs(t, files, plainInit)
	c.open("main.gp", macroBodySrc)
	for _, tc := range []hoverCase{
		{"x := n * 2|", "", ""},
		{"total = x| + 1", "var x int", "x"},
		{"x := |n * 2", "var n int", "n"},
		{"return to|tal", "var total int", "total"},
		{"#util.Tr|ace()\n#Wrap", "macro Trace() func()", "Trace"},
		{"#Wr|ap()\n#util", "macro Wrap()", "Wrap"},
	} {
		text, rng := c.hover("main.gp", macroBodySrc, tc.mark)
		if !strings.HasPrefix(text, tc.text) || rng != tc.rng {
			t.Errorf("hover at %q: %q (range %q), want %q (range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
	sameList(t, "definition of x", c.definitions("textDocument/definition", "main.gp", macroBodySrc, "total = x| + 1"), []string{"main.gp:9:2-9:3 x"})
}

// TestCompletionMacroDetail: the completion of `util.` after a # (or without) shows a macro as declared,
// not with the hidden parameters of its function.
func TestCompletionMacroDetail(t *testing.T) {
	checkGoroutines(t)
	c := startComp(t, macroFiles, plainCompInit)
	items := c.complete("main.gp", "package main\n\nimport \"example.com/m/util\"\n\nfunc main() {\n\t_ = util.Db|\n}\n")
	it := item(items, "Dbg")
	if it == nil {
		t.Fatalf("no Dbg in %v", labels(items))
	}
	if it.Detail != "macro Dbg[T any](v T) T" {
		t.Errorf("detail = %q", it.Detail)
	}
}

// TestMacroManyWraps: with 20 wrap macros on one func the code is still found by its place, and an error in it
// is reported at its place (the nesting does not depend on the layout of the source).
func TestMacroManyWraps(t *testing.T) {
	checkGoroutines(t)
	head := "package main\n\nmacro Wrap() {\n\t#body()\n}\n\n"
	body := strings.Repeat("#Wrap()\n", 20) + "func many(a int) (total int, err error) {\n\tx := a * 2\n\ttotal = x + 1\n\treturn total, nil\n}\n\nfunc main() { println(many(1)) }\n"
	src := head + body
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("main.gp", src)
	for _, tc := range []hoverCase{
		{"total = x| + 1", "var x int", "x"},
		{"x := |a * 2", "var a int", "a"},
		{"return to|tal, nil", "var total int", "total"},
		{"#Wr|ap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\n#Wrap()\nfunc many", "macro Wrap()", "Wrap"},
	} {
		text, rng := c.hover("main.gp", src, tc.mark)
		if !strings.HasPrefix(text, tc.text) || rng != tc.rng {
			t.Errorf("hover at %q: %q (range %q), want %q (range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
	bad := head + strings.Repeat("#Wrap()\n", 20) + "func many(a int) (total int) {\n\ttotal = \"x\"\n\treturn\n}\n\nfunc main() { println(many(1)) }\n"
	files2 := map[string]string{"gocpp.mod": "module example.com/m\n", "b/b.gp": strings.Replace(bad, "package main", "package b", 1)}
	d := startDiag(t, testConfig(), files2, nil)
	d.open("b/b.gp", files2["b/b.gp"])
	want(t, d.next("b/b.gp"), 1, "27:9-27:12 error: cannot use \"x\" (untyped string constant) as int value in assignment\n")
}
