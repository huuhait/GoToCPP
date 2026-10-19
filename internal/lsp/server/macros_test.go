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
