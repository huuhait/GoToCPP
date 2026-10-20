package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/lsp/bench"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The `!` and macro features of the editor (step 11.13, ADR-099): hover, inlay hints, semantic tokens,
// completion after a #, rename of f! and #Name, and "Show expansion".

// bangFiles has the `!` forms in every place where the error goes somewhere else: a function that returns
// the error, a closure, a deferred call, a function without an error result (panic), a function with a
// *testing.T (t.Fatal), main (exit 1), a must function that handles dropped errors, and a package variable.
var bangFiles = map[string]string{
	"gocpp.mod": "module example.com/m\n",
	"b/b.gp": `package main

import (
	"os"
	"strconv"
	"testing"
)

var table = []int{strconv.Atoi!("1")}

func ret(path string) (int, error) {
	f := os.Open!(path)
	defer f.Close!()
	n := strconv.Atoi!(path)
	fn := func() (int, error) {
		return strconv.Atoi!("2"), nil
	}
	_ = fn
	return n, nil
}

func pan(path string) int {
	return strconv.Atoi!(path)
}

func fatal(t *testing.T, path string) {
	n := strconv.Atoi!(path)
	_ = n
}

func must!(path string) int {
	os.Remove(path)
	n := strconv.Atoi(path)
	return n
}

func main() {
	n := strconv.Atoi!("3")
	println(n, must!("4"))
}

func loop(paths []string) (int, error) {
	total := 0
	for _, p := range paths {
		n := strconv.Atoi!(p)
		total += n
	}
	return total, nil
}
`}

func TestBangHover(t *testing.T) {
	c := startRefs(t, bangFiles, plainInit)
	src := bangFiles["b/b.gp"]
	c.open("b/b.gp", src)
	for _, tc := range []struct {
		mark, rng string
		want      []string // substrings of the hover
		not       []string
	}{
		{"f := os.Op|en!(path)", "Open", []string{
			"func os.Open(name string) (*os.File, error)",
			"`ret` returns it: the other results are zero values, and the defers run",
			"The call gives one value: `*os.File`",
			"f, must_e2 := os.Open(path)\nif must_e2 != nil {\n\treturn 0, must_e2\n}"}, nil},
		// the cursor on the ! itself, and right after the name
		{"f := os.Open|!(path)", "Open", []string{"`ret` returns it"}, nil},
		{"n := strconv.Atoi!(path)\n\tfn := func() (int, error)", "", nil, nil},
		{"return strconv.At|oi!(\"2\")", "Atoi", []string{
			"the closure returns it through its own results (the outer function is not left)",
			"must_t3, must_e5 := strconv.Atoi(\"2\")"}, []string{"`ret` returns"}},
		{"return strconv.At|oi!(path)", "Atoi", []string{
			"`pan` panics with it: it has no error result (like `regexp.MustCompile`)",
			"panic(must_e"}, nil},
		{"n := strconv.A|toi!(path)\n\t_ = n", "Atoi", []string{"the test fails and stops: `t.Fatal(err)`", "t.Fatal(must_e"}, nil},
		{"n := strconv.A|toi!(\"3\")", "Atoi", []string{"`main`'s defers run, then `<program>: <error>` is printed to stderr and the program exits with code 1"}, nil},
		// the deferred call
		{"defer f.Cl|ose!()", "Close", []string{
			"If this call fails while `ret` is about to return a nil error, `ret` returns this error instead",
			"defer func() {"}, nil},
		// a call that a must function handles for the user: no ! in the text
		{"os.Rem|ove(path)", "Remove", []string{
			"No `!` is written here: `must` is a must function, so the call that drops its error is handled for you",
			"The call gives no value besides the error",
			"must_e"}, nil},
		// the must function: its real signature, and how it was written
		{"println(n, mu|st!(\"4\"))", "must", []string{
			"func must(path string) (int, error)",
			"A must function, declared `func must!(path string) int`",
			"`main`'s defers run"}, nil},
		{"func mu|st!(path string) int", "must", []string{
			"func must(path string) (int, error)",
			"A must function, declared `func must!(path string) int`"}, []string{"defers run", "No `!`"}},
		// the initializer of a package variable, and a deferred call of a function of another package
		{"[]int{strconv.At|oi!(\"1\")}", "Atoi", []string{"the initializer of the package variable panics with it"}, nil},
		// a call inside a loop: the code of its own statement, not of the loop
		{"n := strconv.A|toi!(p)", "Atoi", []string{"n, must_e", "return 0, must_e"}, []string{"for _, p := range paths", "total += n"}},
		// a plain function has none of it
		{"func re|t(path string)", "ret", []string{"func ret(path string) (int, error)"}, []string{"must function", "`!`"}},
	} {
		if tc.rng == "" {
			continue
		}
		text, rng := c.hover("b/b.gp", src, tc.mark)
		if rng != tc.rng {
			t.Errorf("hover at %q: range %q, want %q (text %q)", tc.mark, rng, tc.rng, text)
		}
		for _, w := range tc.want {
			if !strings.Contains(text, w) {
				t.Errorf("hover at %q: %q does not contain %q", tc.mark, text, w)
			}
		}
		for _, n := range tc.not {
			if strings.Contains(text, n) {
				t.Errorf("hover at %q: %q contains %q", tc.mark, text, n)
			}
		}
	}
	// Markdown: the code is fenced, in a section of its own.
	m := startRefs(t, bangFiles, markdownInit)
	m.open("b/b.gp", src)
	text, _ := m.hover("b/b.gp", src, "f := os.Op|en!(path)")
	if !strings.Contains(text, "---\n\n```go\n// the code that stands for this statement") {
		t.Errorf("markdown hover: %q", text)
	}
}

func TestBangHints(t *testing.T) {
	c := startRefs(t, bangFiles, plainInit)
	src := bangFiles["b/b.gp"]
	c.open("b/b.gp", src)
	var got []string
	for _, h := range c.hints("b/b.gp", nil) {
		if strings.Contains(h, "→") {
			got = append(got, h)
		}
	}
	sameList(t, "the hints of the ! forms", got, []string{
		"9:37 → panic kind 0 left",
		"12:21 → return err kind 0 left",
		"13:18 → deferred: return err kind 0 left",
		"14:26 → return err kind 0 left",
		"16:28 → return err kind 0 left",
		"23:28 → panic kind 0 left",
		"27:26 → t.Fatal kind 0 left",
		"32:17 !→ return err kind 0 left",
		"33:25 !→ return err kind 0 left",
		"38:25 → exit 1 kind 0 left",
		"39:23 → exit 1 kind 0 left",
		"45:24 → return err kind 0 left",
	})
	// the hints of the lowering's own code are not there: no name: for t.Fatal(must_e1), no type for the temporaries
	for _, h := range c.hints("b/b.gp", nil) {
		if strings.Contains(h, "args:") || strings.Contains(h, "err:") || strings.Contains(h, "must_") {
			t.Errorf("a hint for the code of a ! form: %q", h)
		}
	}
	// a range: only the calls whose end is in it
	rng := &protocol.Range{Start: protocol.Position{Line: 11, Character: 0}, End: protocol.Position{Line: 12, Character: 30}}
	var in []string
	for _, h := range c.hints("b/b.gp", rng) {
		if strings.Contains(h, "→") {
			in = append(in, h)
		}
	}
	sameList(t, "the hints in lines 12 and 13", in, []string{"12:21 → return err kind 0 left", "13:18 → deferred: return err kind 0 left"})
	// the tooltip says what happens
	resp := c.call("textDocument/inlayHint", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":31,"character":0},"end":{"line":31,"character":40}}}`, c.uri("b/b.gp")))
	if !strings.Contains(string(resp.Result), "Handled for you: the call drops its error, and `must` is a must function") {
		t.Errorf("the tooltip of an implicit hint: %s", resp.Result)
	}
}

func TestBangSemanticTokens(t *testing.T) {
	c := startRefs(t, bangFiles, plainInit)
	src := bangFiles["b/b.gp"]
	c.open("b/b.gp", src)
	c.hover("b/b.gp", src, "f := os.Op|en!(path)")
	got := c.semTokens("b/b.gp", nil)
	for _, want := range []string{
		"9:31 ! operator ", // the ! of a call in a package variable
		"12:10 Open function defaultLibrary",
		"12:14 ! operator ",
		"13:15 ! operator ",
		"31:10 ! operator declaration", // func must!(
		"39:17 ! operator ",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no token %q in\n%s", want, strings.Join(got, "\n"))
		}
	}
	// no token for a ! that is not in the text, and none on top of another
	seen := map[string]bool{}
	for _, tok := range got {
		place := strings.Fields(tok)[0]
		if seen[place] {
			t.Errorf("two tokens at %s", place)
		}
		seen[place] = true
	}
}

// macroUseFiles is a package with macros of every shape, used from main.
var macroUseFiles = map[string]string{
	"gocpp.mod": "module example.com/m\n",
	"util/util.gp": `package util

import (
	"fmt"
	"sync"
)

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

macro Twice(cond bool) {
	#Assert(cond, "twice")
}

macro Count() {
	#body()
}

macro Locked(mu *sync.Mutex) func() {
	mu.Lock()
	return mu.Unlock
}

// Lz has a lazy parameter first.
macro Lz(f lazy int) int {
	return f()
}

macro internalOnly() {
}

macro Tb(v	lazy	string) {
	_ = v
}
`,
	"main.gp": `package main

import "example.com/m/util"

#util.Count()
func work(n int) int {
	x := #util.Dbg(n * 2)
	#util.Assert(x > 1, "small")
	#util.Twice(x != 3)
	return x
}

#Derive(Equal, Clone)
type Point struct{ X, Y int }

func main() { println(work(1), Point{}.Clone()) }

func lzUse() int { return #util.Lz(3) }
`,
}

func TestMacroUseHover(t *testing.T) {
	c := startRefs(t, macroUseFiles, plainInit)
	src := macroUseFiles["main.gp"]
	c.open("main.gp", src)
	for _, tc := range []struct {
		mark, rng string
		want      []string
	}{
		{"x := #util.Db|g(n * 2)", "Dbg", []string{
			"macro Dbg[T any](v T) T",
			"Site arguments the compiler passes (the macro reads them with the built-ins): `#file()` = `\"main.gp\"`, `#line()` = `7`, `#func()` = `\"main.work\"`, `#text(v)` = `\"n * 2\"`.",
			"util.Dbg(\"main.gp\", 7, \"main.work\", \"n * 2\", n * 2)"}},
		{"#util.As|sert(x > 1", "Assert", []string{
			"macro Assert(cond bool, msg lazy string)",
			"`#line()` = `8`",
			"`#text(cond)` = `\"x > 1\"`",
			"`msg` is lazy: the argument is not evaluated here; the macro calls `msg()` when, and if, it needs the value.",
			"Assert panics when cond is false.",
			"util.Assert(\"main.gp\", 8, \"main.work\", \"x > 1\", x > 1, func() string { ... })"}},
		{"#util.Co|unt()", "Count", []string{
			"macro Count()",
			"A wrap macro: the code of the function it stands on is passed as `#body()`.",
			"util.Count(\"main.gp\", 5, \"main.work\", func() { ... })"}},
		{"#util.Tw|ice(x != 3)", "Twice", []string{"macro Twice(cond bool)", "`#func()` = `\"main.work\"`"}},
		{"#Der|ive(Equal, Clone)", "#Derive", []string{
			"#Derive(Equal, Clone)",
			"Writes methods for the type `Point`",
			"`func (p Point) Equal(other Point) bool`",
			"`func (p Point) Clone() Point`"}},
	} {
		text, rng := c.hover("main.gp", src, tc.mark)
		if rng != tc.rng {
			t.Errorf("hover at %q: range %q, want %q (%q)", tc.mark, rng, tc.rng, text)
		}
		for _, w := range tc.want {
			if !strings.Contains(text, w) {
				t.Errorf("hover at %q: %q does not contain %q", tc.mark, text, w)
			}
		}
		if strings.Contains(text, "gp_L") || strings.Contains(text, "gp_text") {
			t.Errorf("hover at %q shows a hidden name: %q", tc.mark, text)
		}
	}
	// The hover of #Derive ends with the word: the generators after it are methods, found by their own names.
	if text, _ := c.hover("main.gp", src, "#Derive(|Equal, Clone)"); !strings.HasPrefix(text, "func (p Point) Equal(other Point) bool") {
		t.Errorf("hover on the generator Equal: %q", text)
	}
	if text, _ := c.hover("main.gp", src, "#Derive|(Equal, Clone)"); !strings.HasPrefix(text, "#Derive(Equal, Clone)") {
		t.Errorf("hover right after #Derive: %q", text)
	}
	// A macro used in a macro passes the place of the outermost use on: the hover in the macro's file says so.
	usrc := macroUseFiles["util/util.gp"]
	c.open("util/util.gp", usrc)
	text, _ := c.hover("util/util.gp", usrc, "#As|sert(cond, \"twice\")")
	if !strings.Contains(text, "`#file()` = `(of the outer use)`") {
		t.Errorf("a macro in a macro: %q", text)
	}
	// The declaration has no site arguments: it is not a use.
	text, _ = c.hover("util/util.gp", usrc, "macro As|sert")
	if strings.Contains(text, "Site arguments") || strings.Contains(text, "the call that the use becomes") {
		t.Errorf("the declaration: %q", text)
	}
}

func TestMacroHints(t *testing.T) {
	c := startRefs(t, macroUseFiles, plainInit)
	src := macroUseFiles["main.gp"]
	c.open("main.gp", src)
	got := c.hints("main.gp", nil)
	lzLine := strings.Count(src[:strings.Index(src, "#util.Lz(")], "\n") + 1
	lzCol := len("func lzUse() int { return #util.Lz(") + 1
	if want := fmt.Sprintf("%d:%d lazy f: kind 2 right", lzLine, lzCol); !slices.Contains(got, want) {
		t.Errorf("no hint %q (a lazy parameter that is the first) in %q", want, got)
	}
	for _, want := range []string{
		"7:17 v: kind 2 right",
		"8:15 cond: kind 2 right",
		"8:22 lazy msg: kind 2 right", // the lazy argument says so
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no hint %q in %q", want, got)
		}
	}
	for _, h := range got {
		if strings.Contains(h, "gp_") {
			t.Errorf("a hint for a hidden name: %q", h)
		}
	}
	resp := c.call("textDocument/inlayHint", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":7,"character":0},"end":{"line":7,"character":40}}}`, c.uri("main.gp")))
	if !strings.Contains(string(resp.Result), "`msg` is lazy: this argument is not evaluated here. The macro calls `msg()` when, and if, it needs the value.") {
		t.Errorf("the tooltip of a lazy argument: %s", resp.Result)
	}
}

func TestMacroSemanticTokens(t *testing.T) {
	c := startRefs(t, macroUseFiles, plainInit)
	src := macroUseFiles["main.gp"]
	c.open("main.gp", src)
	c.hover("main.gp", src, "x := #util.Db|g(n * 2)")
	got := c.semTokens("main.gp", nil)
	for _, want := range []string{
		"5:1 # macro ", // the # of a qualified use has a token of its own
		"5:2 util namespace ",
		"5:7 Count macro ",
		"7:7 # macro ",
		"7:8 util namespace ",
		"7:13 Dbg macro ",
		"13:1 #Derive macro defaultLibrary",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no token %q in\n%s", want, strings.Join(got, "\n"))
		}
	}
	// declaration, a use in the package, lazy
	usrc := macroUseFiles["util/util.gp"]
	c.open("util/util.gp", usrc)
	c.hover("util/util.gp", usrc, "macro As|sert")
	got = c.semTokens("util/util.gp", nil)
	tbLine := strings.Count(usrc[:strings.Index(usrc, "macro Tb")], "\n") + 1
	if want := fmt.Sprintf("%d:%d lazy keyword ", tbLine, len("macro Tb(v\t")+1); !slices.Contains(got, want) {
		t.Errorf("no token %q (lazy after a tab) in\n%s", want, strings.Join(got, "\n"))
	}
	for _, want := range []string{
		"9:7 Assert macro declaration",
		"9:29 lazy keyword ",
		"15:7 Dbg macro declaration",
		"21:2 #Assert macro ", // an unqualified use is one token with its #
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no token %q in\n%s", want, strings.Join(got, "\n"))
		}
	}
}

func TestMacroCompletion(t *testing.T) {
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "util/util.gp": macroUseFiles["util/util.gp"], "main.gp": "package main\n\nimport \"example.com/m/util\"\n\nmacro Local(a int, b string) {\n}\n\nfunc main() {\n}\n"}
	c := startComp(t, files, plainCompInit)
	const head = "package main\n\nimport \"example.com/m/util\"\n\nmacro Local(a int, b string) {\n}\n\n"
	for _, tc := range []struct {
		name, text string
		want       []string // labels, in order; nil: no item at all
		not        []string
	}{
		{"after #", head + "func main() {\n\t#|\n}\n",
			[]string{"Local", "Derive", "file", "line", "func", "text", "embed", "body", "util"}, nil},
		{"a prefix", head + "func main() {\n\t#Lo|\n}\n", []string{"Local"}, []string{"Derive"}},
		{"a prefix in any case", head + "func main() {\n\t#de|\n}\n", []string{"Derive"}, nil},
		{"after #util.", head + "func main() {\n\t#util.|\n}\n", []string{"Assert", "Count", "Dbg", "Locked", "Lz", "Tb", "Twice"}, []string{"Local", "Derive", "internalOnly"}},
		{"inside a name", head + "func main() {\n\tx := #util.D|bg(1)\n}\n", []string{"Dbg"}, nil},
		{"on top of a func", head + "#|\nfunc main() {\n}\n", []string{"Local", "Derive", "file", "line", "func", "text", "embed", "body", "util"}, nil},
		{"a comment", head + "// #|\nfunc main() {\n}\n", nil, nil},
		{"a string", head + "func main() {\n\t_ = \"#|\"\n}\n", nil, nil},
		{"a backquote in a string", head + "func main() {\n\t_ = \"a`b\" + #Lo|\n}\n", []string{"Local"}, nil},
		{"a string with a hash after a quote", head + "func main() {\n\t_ = \"a\\\"#|\"\n}\n", nil, nil},
		{"a raw string of several lines", head + "func main() {\n\t_ = `one\ntwo #|\nthree`\n}\n", nil, nil},
		{"after a raw string", head + "func main() {\n\t_ = `one\ntwo` + #Lo|\n}\n", []string{"Local"}, nil},
		{"a block comment", head + "/* one\n #| */\nfunc main() {\n}\n", nil, nil},
		{"after a comment", head + "func main() { /* c */ _ = #Lo|\n}\n", []string{"Local"}, nil},
		{"an unknown package", head + "func main() {\n\t#nope.|\n}\n", []string{}, nil},
	} {
		items := c.complete("main.gp", tc.text)
		got := labels(items)
		if tc.want == nil {
			if len(got) != 0 {
				t.Errorf("%s: got %v, want none", tc.name, got)
			}
			continue
		}
		if !slices.Equal(got, tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
		for _, n := range tc.not {
			if slices.Contains(got, n) {
				t.Errorf("%s: %s is offered: %v", tc.name, n, got)
			}
		}
	}
	// A raw string with CRLF line ends: the tree's literal has lost the carriage returns and ends early, the scanner does not
	// (the ordinary completion may answer there; the macros do not).
	for _, it := range c.complete("main.gp", head+"func main() {\r\n\t_ = `one\r\ntwo\r\nthree\r\nfour #|`\r\n}\r\n") {
		if it.Label == "Derive" || it.Label == "embed" || it.Label == "body" {
			t.Errorf("a macro in a raw string: %q", it.Label)
		}
	}
	// the detail, the snippet and the text edit
	items := c.complete("main.gp", head+"func main() {\n\t#Lo|\n}\n")
	it := item(items, "Local")
	if it == nil || it.Detail != "macro Local(a int, b string)" {
		t.Fatalf("Local: %+v", it)
	}
	edit, ok := it.TextEdit.Value.(protocol.TextEdit)
	if !ok || edit.NewText != "Local(" || edit.Range.Start.Character != 2 || edit.Range.End.Character != 4 {
		t.Errorf("the edit of Local: %+v", it.TextEdit.Value)
	}
	cs := startComp(t, files, snippetInit)
	it = item(cs.complete("main.gp", head+"func main() {\n\t#Lo|\n}\n"), "Local")
	if edit, ok := it.TextEdit.Value.(protocol.TextEdit); !ok || edit.NewText != "Local(${1:a}, ${2:b})" {
		t.Errorf("the snippet of Local: %+v", it.TextEdit.Value)
	}
	// the name under the cursor is replaced whole, and a parenthesis that is there is not inserted again
	items = cs.complete("main.gp", head+"func main() {\n\t#Der|ive(Equal)\n}\n")
	it = item(items, "Derive")
	if it == nil {
		t.Fatalf("#Der|ive: %v", labels(items))
	}
	if edit, ok := it.TextEdit.Value.(protocol.TextEdit); !ok || edit.NewText != "Derive" || edit.Range.Start.Character != 2 || edit.Range.End.Character != 8 {
		t.Errorf("the edit of Derive before a parenthesis: %+v", it.TextEdit.Value)
	}
	items = cs.complete("main.gp", head+"func main() {\n\t#Der|ive\n}\n")
	if it := item(items, "Derive"); it == nil || it.TextEdit.Value.(protocol.TextEdit).NewText != "Derive(${1:Equal})" {
		t.Errorf("the edit of Derive with none: %+v", it)
	}
	items = c.complete("main.gp", head+"func main() {\n\t#util.|\n}\n")
	if it := item(items, "Assert"); it == nil || it.Detail != "macro Assert(cond bool, msg lazy string)" {
		t.Errorf("Assert: %+v", it)
	}
	// Ordinary completion is untouched by the #: the same list as before for a name.
	items = c.complete("main.gp", head+"func main() {\n\tutil.|\n}\n")
	if item(items, "Dbg") == nil {
		t.Errorf("util.: %v", labels(items))
	}
}

func TestBangRename(t *testing.T) {
	c := startRefs(t, bangFiles, plainInit)
	src := bangFiles["b/b.gp"]
	c.open("b/b.gp", src)
	// From a use, from the declaration and from the ! itself: the declaration and the uses change, and every ! stays.
	for _, mark := range []string{"println(n, mu|st!(\"4\"))", "func mu|st!(path", "println(n, must|!(\"4\")"} {
		for _, newName := range []string{"fetch", "fetch!"} {
			edited, err := c.rename("b/b.gp", mark, newName)
			if err != nil {
				t.Fatalf("rename at %q to %q: %v", mark, newName, err)
			}
			sameList(t, "the changed lines ("+mark+" to "+newName+")", diffLinesAgainst(t, src, edited["b/b.gp"]), []string{
				"31: func fetch!(path string) int {",
				"39: println(n, fetch!(\"4\"))",
			})
		}
	}
	// the message names what the user typed
	if _, err := c.rename("b/b.gp", "func mu|st!(path", "x!!"); err == nil || !strings.Contains(err.Message, `"x!!" is not a valid name`) {
		t.Errorf("rename to x!!: %v", err)
	}
	// A function that is not a must function: the ! is not a name.
	_, err := c.rename("b/b.gp", "func re|t(path", "again!")
	if err == nil || !strings.Contains(err.Message, "A ! after a name marks a must function or a must call") {
		t.Errorf("rename of ret to again!: %v", err)
	}
	// The name of a callee in a ! call is a name like any: library functions can't be renamed here, ret can.
	edited, rerr := c.rename("b/b.gp", "func re|t(path", "again")
	if rerr != nil {
		t.Fatal(rerr)
	}
	sameList(t, "the changed lines (ret)", diffLinesAgainst(t, src, edited["b/b.gp"]), []string{"11: func again(path string) (int, error) {"})
	// highlights and references see the callee of a ! call as a use, and not the names of the lowering
	refs := c.references("b/b.gp", "func mu|st!(path", true)
	sameList(t, "references of must", refs, []string{"b/b.gp:31:6 must", "b/b.gp:39:13 must"})
	// prepareRename takes the name without the !
	resp := c.call("textDocument/prepareRename", c.posParams("b/b.gp", src, "println(n, must|!(\"4\")"))
	if !strings.Contains(string(resp.Result), `"placeholder":"must"`) || resp.Error != nil {
		t.Errorf("prepareRename: %s %v", resp.Result, resp.Error)
	}
}

func TestMacroRename(t *testing.T) {
	c := startRefs(t, macroUseFiles, plainInit)
	src, usrc := macroUseFiles["main.gp"], macroUseFiles["util/util.gp"]
	c.open("main.gp", src)
	for _, newName := range []string{"Dump", "#Dump"} {
		edited, err := c.rename("main.gp", "x := #util.Db|g(n * 2)", newName)
		if err != nil {
			t.Fatalf("rename to %q: %v", newName, err)
		}
		sameList(t, "main.gp ("+newName+")", diffLinesAgainst(t, src, edited["main.gp"]), []string{"7: x := #util.Dump(n * 2)"})
		sameList(t, "util.gp ("+newName+")", diffLinesAgainst(t, usrc, edited["util/util.gp"]), []string{"15: macro Dump[T any](v T) T {"})
	}
	// the cursor on the # of a use is on the macro
	for _, mark := range []string{"x := |#util.Dbg(n * 2)", "x := #util.Dbg|(n * 2)"} {
		edited, err := c.rename("main.gp", mark, "#Renamed")
		if err != nil {
			t.Fatalf("rename at %q: %v", mark, err)
		}
		sameList(t, "main.gp ("+mark+")", diffLinesAgainst(t, src, edited["main.gp"]), []string{"7: x := #util.Renamed(n * 2)"})
	}
	if text, rng := c.hover("main.gp", src, "x := |#util.Dbg(n * 2)"); !strings.HasPrefix(text, "macro Dbg[T any](v T) T") || rng != "Dbg" {
		t.Errorf("hover on the # of a use: %q (range %q)", text, rng)
	}
	_, err := c.rename("main.gp", "func wo|rk(n int)", "#again")
	if err == nil || !strings.Contains(err.Message, "A # before a name marks a macro use") {
		t.Errorf("rename of work to #again: %v", err)
	}
	// definition and references of a macro used in a macro
	sameList(t, "definition of Assert in Twice", c.definitions("textDocument/definition", "main.gp", src, "#util.As|sert(x > 1"),
		[]string{"util/util.gp:9:7-9:13 Assert"})
	refs := c.references("main.gp", "#util.As|sert(x > 1", true)
	sameList(t, "references of Assert", refs, []string{"main.gp:8:8 Assert", "util/util.gp:9:7 Assert", "util/util.gp:21:3 Assert"})
}

func TestShowExpansion(t *testing.T) {
	c := startRefs(t, mustFiles, showDocInit)
	src := mustFiles["m/m.gp"]
	c.open("m/m.gp", src)

	actionsAt := func(mark string) []protocol.CodeAction {
		t.Helper()
		p := at(t, src, mark)
		resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[]}}`,
			c.uri("m/m.gp"), p.Line, p.Character, p.Line, p.Character))
		if resp.Error != nil {
			t.Fatal(resp.Error)
		}
		var as []protocol.CodeAction
		if err := json.Unmarshal(resp.Result, &as); err != nil {
			t.Fatal(err)
		}
		return as
	}
	// Offered on a declaration with a ! in it (a call, or the declaration of a must function, dropped errors included),
	// and not on one without.
	for mark, title := range map[string]string{
		"q := lo|ad!(name)":          "Show expansion of use",
		"n = po|rt(name)":            "Show expansion of load",
		"func lo|ad!(name":           "Show expansion of load",
		"return len(na|me), nil":     "",
		"func repo|rt(a, b int)":     "",
		"fmt.Println(use(\"ap|p\"))": "",
	} {
		as := actionsAt(mark)
		switch {
		case title == "" && len(as) != 0:
			t.Errorf("at %q: actions %+v", mark, as)
		case title != "" && (len(as) != 1 || as[0].Title != title || as[0].Kind != protocol.RefactorInline || as[0].Command == nil || as[0].Command.Command != ExpandCommand):
			t.Errorf("at %q: actions %+v, want %q", mark, as, title)
		}
	}
	// Not for a request that wants other kinds.
	p := at(t, src, "q := lo|ad!(name)")
	resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[],"only":["quickfix"]}}`,
		c.uri("m/m.gp"), p.Line, p.Character, p.Line, p.Character))
	if string(resp.Result) != "[]" {
		t.Errorf("only quickfix: %s", resp.Result)
	}

	// The command: the expansion of the file as gocpp expand prints it, the declaration selected, the document shown.
	as := actionsAt("q := lo|ad!(name)")
	arg := string(as[0].Command.Arguments[0])
	c.id++
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"workspace/executeCommand","params":{"command":%q,"arguments":[%s]}}`, c.id, ExpandCommand, arg))
	var shown protocol.ShowDocumentParams
	var res *protocol.Response
	for res == nil {
		switch m := c.read().(type) {
		case *protocol.Request:
			if m.Method != "window/showDocument" {
				t.Fatalf("a request %s", m.Method)
			}
			if err := json.Unmarshal(m.Params, &shown); err != nil {
				t.Fatal(err)
			}
		case *protocol.Response:
			res = m
		}
	}
	if res.Error != nil {
		t.Fatal(res.Error)
	}
	var er ExpandResult
	if err := json.Unmarshal(res.Result, &er); err != nil {
		t.Fatal(err)
	}
	if er.Text != mustExpansion {
		t.Errorf("the expansion:\n%s\nwant:\n%s", er.Text, mustExpansion)
	}
	if er.Selection == nil || strings.Split(er.Text, "\n")[er.Selection.Start.Line] != "func use(name string) (int, error) {" || er.Selection.End.Line != 43 {
		t.Errorf("the selection: %+v", er.Selection)
	}
	if shown.URI == "" || string(shown.URI) != string(er.URI) || !shown.TakeFocus || shown.Selection == nil || *shown.Selection != *er.Selection {
		t.Errorf("window/showDocument: %+v for %s", shown, er.URI)
	}
	if !strings.HasSuffix(er.URI.Path(), "m"+ExpandExt) {
		t.Errorf("the file: %s", er.URI)
	}
	data, err := os.ReadFile(er.URI.Path())
	if err != nil || string(data) != er.Text {
		t.Errorf("the file shown: %v", err)
	}

	// The text is the editor's: an unsaved edit is expanded, and a file with an error is not.
	changed := strings.Replace(src, "p := port!(name)", "p := port!(name + \"x\")", 1)
	c.change("m/m.gp", 2, changed)
	c.id++
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"workspace/executeCommand","params":{"command":%q,"arguments":[%s]}}`, c.id, ExpandCommand, arg))
	for res = nil; res == nil; {
		if m, ok := c.read().(*protocol.Response); ok {
			res = m
		}
	}
	if err := json.Unmarshal(res.Result, &er); err != nil || !strings.Contains(er.Text, `port(name + "x")`) {
		t.Errorf("an unsaved edit: %s", res.Result)
	}
	broken := strings.Replace(changed, "p := port!(name + \"x\")", "p := port!(undefinedName)", 1)
	c.change("m/m.gp", 3, broken)
	c.id++
	c.send(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"workspace/executeCommand","params":{"command":%q,"arguments":[%s]}}`, c.id, ExpandCommand, arg))
	for res = nil; res == nil; {
		if m, ok := c.read().(*protocol.Response); ok {
			res = m
		}
	}
	if res.Error == nil || !strings.Contains(res.Error.Message, "cannot expand: the package has errors, the first: ") || !strings.Contains(res.Error.Message, "undefined: undefinedName") {
		t.Errorf("a package with an error: %s %v", res.Result, res.Error)
	}
	// a bad argument
	resp = c.call("workspace/executeCommand", `{"command":"gocpp.showExpansion","arguments":[]}`)
	if resp.Error == nil {
		t.Errorf("no argument: %s", resp.Result)
	}

	// The file is removed when the server ends.
	file := er.URI.Path()
	c.call("shutdown", "")
	c.notify("exit", "")
	if c.wait() != 0 {
		t.Errorf("exit code")
	}
	if _, err := os.Stat(file); err == nil {
		t.Errorf("the file of the expansion is still there after the end of the server")
	}
}

// TestShowExpansionDeriveAssociation: a directive precedes its type, which is copied unchanged into the expansion.
// Both places must offer the action and select the methods associated by the loader, not a nearby declaration.
func TestShowExpansionDeriveAssociation(t *testing.T) {
	for _, tc := range []struct {
		name, src, typ         string
		marks, methods, absent []string
	}{
		{"wrap only", "package p\nmacro Wrap() { #body() }\n#Wrap()\nfunc wrapped() int { return 1 }\n", "wrapped",
			[]string{"#Wr|ap()", "func wra|pped()"}, []string{"func wrapped()"}, nil},
		{"CRLF derive", "package p\r\n#Derive(Equal)\r\ntype Point struct { X int }\r\n", "Point",
			[]string{"#Der|ive(Equal)", "type Po|int"}, []string{"func (p Point) Equal("}, nil},
		{"derive only", "package p\n#Derive(Equal)\ntype Point struct { X int }\n", "Point",
			[]string{"#Der|ive(Equal)", "type Po|int"}, []string{"func (p Point) Equal("}, nil},
		{"comments and stacked directives", "package p\n// Point docs.\n#Derive(Equal) // equal\n// Between directives.\n#Derive(Clone)\n/* Before the type. */\ntype Point struct { X int }\n", "Point",
			[]string{"#Der|ive(Equal)", "#Der|ive(Clone)", "// Bet|ween directives.", "type Po|int"}, []string{"func (p Point) Equal(", "func (p Point) Clone("}, nil},
		{"single spec group", "package p\n#Derive(Equal)\ntype (\n// Point docs.\nPoint struct { X int }\n)\n", "Point",
			[]string{"#Der|ive(Equal)", "ty|pe (", "Po|int struct"}, []string{"func (p Point) Equal("}, nil},
		{"nearby types", "package p\n#Derive(Equal)\ntype Point struct { X int }; type Plain int\n#Derive(Clone)\ntype Other struct { X int }\n", "Point",
			[]string{"#Der|ive(Equal)", "type Po|int"}, []string{"func (p Point) Equal("}, []string{"func (o Other) Clone("}},
		{"other type", "package p\n#Derive(Equal)\ntype Point struct { X int }\n#Derive(Clone)\ntype Other struct { X int }\n", "Other",
			[]string{"#Der|ive(Clone)", "type Ot|her"}, []string{"func (o Other) Clone("}, []string{"func (p Point) Equal("}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := startRefs(t, map[string]string{"gocpp.mod": "module example.com/p\n", "p.gp": tc.src}, plainInit)
			c.open("p.gp", tc.src)
			var selected string
			for _, mark := range tc.marks {
				p := at(t, tc.src, mark)
				resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[],"only":["refactor.inline"]}}`, c.uri("p.gp"), p.Line, p.Character, p.Line, p.Character))
				if resp.Error != nil {
					t.Fatal(resp.Error)
				}
				var actions []protocol.CodeAction
				if err := json.Unmarshal(resp.Result, &actions); err != nil {
					t.Fatal(err)
				}
				if len(actions) != 1 || actions[0].Title != "Show expansion of "+tc.typ || actions[0].Command == nil {
					t.Fatalf("at %q: actions %s", mark, resp.Result)
				}
				cmd := actions[0].Command
				params, err := json.Marshal(map[string]any{"command": cmd.Command, "arguments": cmd.Arguments})
				if err != nil {
					t.Fatal(err)
				}
				resp = c.call("workspace/executeCommand", string(params))
				if resp.Error != nil {
					t.Fatal(resp.Error)
				}
				var er ExpandResult
				if err := json.Unmarshal(resp.Result, &er); err != nil || er.Selection == nil {
					t.Fatalf("at %q: expansion selection missing: %s (%v)", mark, resp.Result, err)
				}
				m := protocol.NewMapper(er.Text)
				a, b, err := m.RangeOffsets(*er.Selection)
				if err != nil || b <= a {
					t.Fatalf("selection %+v: %v", er.Selection, err)
				}
				text := er.Text[a:b]
				for _, want := range tc.methods {
					if !strings.Contains(text, want) {
						t.Errorf("at %q: selection %q lacks %q", mark, text, want)
					}
				}
				for _, unwanted := range tc.absent {
					if strings.Contains(text, unwanted) {
						t.Errorf("at %q: selection jumps to unrelated method %q", mark, unwanted)
					}
				}
				if selected != "" && text != selected {
					t.Errorf("directive and type select different methods: %q vs %q", selected, text)
				}
				selected = text
			}
		})
	}
}

func TestShowExpansionDeriveRejectsMultiSpec(t *testing.T) {
	src := "package p\n#Derive(Equal)\ntype (\nA struct { X int }\nB struct { X int }\n)\n#Derive(Clone)\ntype Other struct { X int }\ntype (\nPlain int\nOtherPlain int\n)\n"
	c := startRefs(t, map[string]string{"gocpp.mod": "module example.com/p\n", "p.gp": src}, plainInit)
	c.open("p.gp", src)
	for _, mark := range []string{"#Der|ive(Equal)", "A stru|ct", "B stru|ct", "Pla|in int", "OtherPla|in int"} {
		p := at(t, src, mark)
		resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[],"only":["refactor.inline"]}}`, c.uri("p.gp"), p.Line, p.Character, p.Line, p.Character))
		if resp.Error != nil || string(resp.Result) != "[]" {
			t.Errorf("at unsupported/unmarked %q: %s (%v)", mark, resp.Result, resp.Error)
		}
	}
}

// showDocInit is plainInit for a client that can show documents.
const showDocInit = `{"processId":1,"rootUri":null,"capabilities":{"window":{"showDocument":{"support":true}},"textDocument":{"hover":{"contentFormat":["plaintext","markdown"]}}}}`

// mustExpansion is what gocpp expand prints for mustFiles (the golden file of the guide's example in miniature).
const mustExpansion = `package main

import (
	"errors"
	"fmt"
)

func port(name string) (int, error) {
	if name == "" {
		return 0, errors.New("no name")
	}
	return len(name), nil
}

func load(name string) (n int, err error) {
	defer errors.Wrapf(&err, "load %s", name)
	{
		must_t1, must_e1 := port(name)
		if must_e1 != nil {
			return 0, must_e1
		}
		n = must_t1
	}
	return n, nil
}

func use(name string) (int, error) {
	p, must_e2 := port(name)
	if must_e2 != nil {
		return 0, must_e2
	}
	q, must_e3 := load(name)
	if must_e3 != nil {
		return 0, must_e3
	}
	{
		must_t2, must_e4 := port(name)
		if must_e4 != nil {
			return 0, must_e4
		}
		report(p, must_t2)
	}
	return p + q, nil
}

func report(a, b int) { fmt.Println(a, b) }

func main() {
	fmt.Println(use("app"))
}
`

// TestBangPartialMainPolicy: a library function called main is not the executable entry point. An unrelated
// type error must not change its ! policy; the actual main package's entry point still exits.
func TestBangPartialMainPolicy(t *testing.T) {
	for _, tc := range []struct{ name, pkg, decl, want string }{
		{"library return", "lib", "func main() error { fail!(); return nil }", "returns it"},
		{"library panic", "lib", "func main() { fail!() }", "panics with it"},
		{"library must", "lib", "func main!() { fail!() }", "returns it"},
		{"library method", "lib", "type T struct{}\nfunc (T) main() error { fail!(); return nil }", "returns it"},
		{"entry point", "main", "func main() { fail!() }", "program exits with code 1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package " + tc.pkg + "\nfunc fail() error { return nil }\n" + tc.decl + "\n"
			c := startRefs(t, map[string]string{"gocpp.mod": "module example.com/m\n", "a.gp": src}, plainInit)
			c.open("a.gp", src)
			clean, _ := c.hover("a.gp", src, "{ fa|il!()")
			broken := src + "var bad = undefinedName\n"
			c.change("a.gp", 2, broken)
			partial, _ := c.hover("a.gp", broken, "{ fa|il!()")
			if !strings.Contains(clean, tc.want) || !strings.Contains(partial, tc.want) {
				t.Errorf("policy %q lost: clean %q, partial %q", tc.want, clean, partial)
			}
			if tc.pkg == "lib" && strings.Contains(partial, "program exits") {
				t.Errorf("a library main was treated as an executable entry point: %q", partial)
			}
		})
	}
}

// TestBangPartialTestingPolicy: partial information must use the compiler's typed testing contexts, including
// testing.TB and inherited closure contexts. An unrelated error must not change return/Fatal/panic priority.
func TestBangPartialTestingPolicy(t *testing.T) {
	for _, tc := range []struct{ name, imports, decl, want string }{
		{"TB", `import "testing"`, "func helper(tb testing.TB) { fail!() }", "test fails and stops"},
		{"T closure", `import "testing"`, "func helper(t *testing.T) { f := func() { fail!() }; f() }", "test fails and stops"},
		{"TB closure", `import "testing"`, "func helper(tb testing.TB) { f := func() { fail!() }; f() }", "test fails and stops"},
		{"nested closure", `import "testing"`, "func helper(t *testing.T) { f := func() { g := func() { fail!() }; g() }; f() }", "test fails and stops"},
		{"renamed import", `import test "testing"`, "func helper(tb test.TB) { fail!() }", "test fails and stops"},
		{"closure parameter", `import "testing"`, "func helper() { f := func(tb testing.TB) { fail!() }; _ = f }", "test fails and stops"},
		{"return takes priority", `import "testing"`, "func helper(tb testing.TB) error { fail!(); return nil }", "returns it"},
		{"closure return takes priority", `import "testing"`, "func helper(t *testing.T) { f := func() error { fail!(); return nil }; _ = f }", "returns it"},
		{"return context is not inherited", `import "testing"`, "func helper(t *testing.T) error { f := func() { fail!() }; f(); return nil }", "panics with it"},
		{"unnamed parameter", `import "testing"`, "func helper(testing.TB) { fail!() }", "panics with it"},
		{"blank parameter", `import "testing"`, "func helper(_ testing.TB) { fail!() }", "panics with it"},
		{"other package T", `import testing "example.com/m/impostor"`, "func helper(t *testing.T) { fail!() }", "panics with it"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "package lib\n" + tc.imports + "\nfunc fail() error { return nil }\n" + tc.decl + "\n"
			files := map[string]string{
				"gocpp.mod":            "module example.com/m\n",
				"lib/lib.gp":           src,
				"impostor/impostor.gp": "package impostor\ntype T struct{}\nfunc (*T) Fatal(args ...any) {}\n",
			}
			c := startRefs(t, files, plainInit)
			c.open("lib/lib.gp", src)
			clean, _ := c.hover("lib/lib.gp", src, "{ fa|il!()")
			broken := src + "var bad = undefinedName\n"
			c.change("lib/lib.gp", 2, broken)
			partial, _ := c.hover("lib/lib.gp", broken, "{ fa|il!()")
			if !strings.Contains(clean, tc.want) || !strings.Contains(partial, tc.want) {
				t.Errorf("policy %q lost: clean %q, partial %q", tc.want, clean, partial)
			}
		})
	}
}

// TestBangSyntheticNamedResultOrigin: a named must function gets a synthetic error result with NoPos before
// lowering. Partial hover must hide that proven synthetic name, without treating unknown imported names as synthetic.
func TestBangSyntheticNamedResultOrigin(t *testing.T) {
	src := "package a\nfunc value(s string) (int, error) { return 1, nil }\nfunc two!(a, b string) (x, y int) { x = value(a); y = value(b); return }\n"
	c := startRefs(t, map[string]string{"gocpp.mod": "module example.com/m\n", "a.gp": src}, plainInit)
	c.open("a.gp", src)
	clean, _ := c.hover("a.gp", src, "func tw|o!")
	broken := src + "var bad = undefinedName\n"
	c.change("a.gp", 2, broken)
	partial, _ := c.hover("a.gp", broken, "func tw|o!")
	if !strings.HasPrefix(clean, "func two(a string, b string) (x int, y int, error)") || partial != clean {
		t.Errorf("synthetic result origin changed the signature: clean %q, partial %q", clean, partial)
	}
}

// TestBangUserPrefixNames: gp_/must_ spellings are legal ordinary user names, not evidence of generated code.
// Export-data positions are inexact, so imported names must not be erased by a spelling heuristic either.
func TestBangUserPrefixNames(t *testing.T) {
	src := "package main\nimport \"example.com/m/lib\"\nfunc F(gp_value int) {}\nfunc main() { F(1); lib.H(1); _ = lib.F(1); _ = lib.G(1); _ = lib.T{}.Method(1) }\n"
	lib := "package lib\nfunc F(must_r int) (must_err int) { return must_r }\nfunc G[T any](must_r T) (must_err T) { return must_r }\ntype T struct{}\nfunc (T) Method(must_r int) (must_err int) { return must_r }\nfunc H(gp_value int) {}\n"
	c := startRefs(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": src, "lib/lib.gp": lib}, plainInit)
	c.open("main.gp", src)
	for _, mark := range []string{"lib.F|(1)", "lib.G|(1)", ".Meth|od(1)"} {
		text, _ := c.hover("main.gp", src, mark)
		if !strings.Contains(text, "must_r ") || !strings.Contains(text, "must_err ") {
			t.Errorf("at %q: imported user parameter/result names lost: %q", mark, text)
		}
	}
	count := 0
	for _, h := range c.hints("main.gp", nil) {
		if strings.Contains(h, "gp_value:") {
			count++
		}
	}
	if count != 2 {
		t.Errorf("%d gp_value parameter hints, want local and imported calls", count)
	}
}

// TestBangWithErrors: a package with an error (here an unused variable) is not lowered, but the hover,
// the hints and the tokens of its ! forms stay, with the kind guessed from the signature of the function;
// only the code that stands for the statement is missing.
func TestBangWithErrors(t *testing.T) {
	c := startRefs(t, bangFiles, plainInit)
	src := strings.Replace(bangFiles["b/b.gp"], "n := strconv.Atoi(path)\n\treturn n", "n := strconv.Atoi(path)\n\tz := 1\n\treturn n", 1)
	c.open("b/b.gp", src)
	for _, tc := range []struct{ mark, want string }{
		{"f := os.Op|en!(path)", "`ret` returns it"},
		{"return strconv.At|oi!(path)", "`pan` panics with it"},
		{"n := strconv.A|toi!(path)\n\t_ = n", "t.Fatal(err)"},
		{"os.Rem|ove(path)", "`must` is a must function"},
	} {
		text, _ := c.hover("b/b.gp", src, tc.mark)
		if !strings.Contains(text, tc.want) || strings.Contains(text, "the code that stands for") {
			t.Errorf("hover at %q: %q", tc.mark, text)
		}
	}
	var got []string
	for _, h := range c.hints("b/b.gp", nil) {
		if strings.Contains(h, "→") {
			got = append(got, h)
		}
	}
	if len(got) != 12 || !slices.Contains(got, "12:21 → return err kind 0 left") || !slices.Contains(got, "27:26 → t.Fatal kind 0 left") || !slices.Contains(got, "32:17 !→ return err kind 0 left") {
		t.Errorf("the hints: %q", got)
	}
	toks := c.semTokens("b/b.gp", nil)
	if !slices.Contains(toks, "12:14 ! operator ") || !slices.Contains(toks, "31:10 ! operator declaration") {
		t.Errorf("the tokens: %q", toks)
	}
}

// TestBangSymbols: the outline shows a must function as f! and a macro as #Name, as the user wrote them
// (they come from the syntax tree of the file, not from the lowered one), and workspace symbols find them.
func TestBangSymbols(t *testing.T) {
	c := startRefs(t, bangFiles, plainInit)
	c.open("b/b.gp", bangFiles["b/b.gp"])
	syms := string(c.docSymbols("b/b.gp"))
	for _, want := range []string{`"name":"must!"`, `"name":"ret"`} {
		if !strings.Contains(syms, want) {
			t.Errorf("symbols of b.gp: %s lacks %s", syms, want)
		}
	}
	m := startRefs(t, macroUseFiles, plainInit)
	m.open("util/util.gp", macroUseFiles["util/util.gp"])
	syms = string(m.docSymbols("util/util.gp"))
	for _, want := range []string{`"name":"#Assert"`, `"name":"#Dbg"`, `"name":"#Count"`} {
		if !strings.Contains(syms, want) {
			t.Errorf("symbols of util.gp: %s lacks %s", syms, want)
		}
	}
}

// TestBangQuickAction: the quick action "Handle the error of f with !" puts a ! after the name of a call that
// drops its error, where the checker accepts one, and nowhere else.
func TestBangQuickAction(t *testing.T) {
	src := `package main

import (
	"fmt"
	"os"
	"strconv"
)

func a(path string) {
	os.Remove(path)
	defer os.Remove(path)
	n := strconv.Atoi(path)
	var m = strconv.Atoi(path)
	_, _ = n, m
	x, y := two(path)
	_, _ = x, y
	typed()
	u, v, e := two(path)
	_, _, _ = u, v, e
}

type MyErr struct{}

func (*MyErr) Error() string { return "" }

func d(path string) {
	defer fmt.Println(path)
}

func typed() *MyErr { return nil }

func two(path string) (int, int, error) { return 0, 0, nil }

func b(path string) error {
	os.Remove!(path)
	_ = os.Remove(path)
	fmt.Println(path)
	if os.Remove(path) != nil {
		return nil
	}
	return os.Remove(path)
}

func c!(path string) {
	os.Remove(path)
}
`
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	actions := func(mark string) []protocol.CodeAction {
		t.Helper()
		p := at(t, src, mark)
		resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[]}}`,
			c.uri("a/a.gp"), p.Line, p.Character, p.Line, p.Character))
		if resp.Error != nil {
			t.Fatal(resp.Error)
		}
		var all, as []protocol.CodeAction
		if err := json.Unmarshal(resp.Result, &all); err != nil {
			t.Fatal(err)
		}
		for _, a := range all {
			if a.Kind == protocol.RefactorRewrite { // ("Show expansion" is another action's business)
				as = append(as, a)
			}
		}
		return as
	}
	for _, tc := range []struct{ mark, title, line string }{
		{"\tos.Rem|ove(path)\n\tdefer", "Handle the error of Remove with !", "10: os.Remove!(path)"},
		{"defer os.Rem|ove(path)", "Handle the error of Remove with !", "11: defer os.Remove!(path)"},
		{"n := strconv.At|oi(path)", "Handle the error of Atoi with !", "12: n := strconv.Atoi!(path)"},
		{"var m = strconv.At|oi(path)", "Handle the error of Atoi with !", "13: var m = strconv.Atoi!(path)"},
		{"os.Remove|(path)\n\tdefer", "Handle the error of Remove with !", "10: os.Remove!(path)"},
		{"x, y := tw|o(path)", "Handle the error of two with !", "15: x, y := two!(path)"},
	} {
		as := actions(tc.mark)
		if len(as) != 1 || as[0].Title != tc.title || as[0].Kind != protocol.RefactorRewrite {
			t.Errorf("at %q: actions %+v", tc.mark, as)
			continue
		}
		te := as[0].Edit.DocumentChanges[0].TextDocumentEdit
		var edits []protocol.TextEdit
		for _, e := range te.Edits {
			switch v := e.Value.(type) {
			case protocol.TextEdit:
				edits = append(edits, v)
			case protocol.AnnotatedTextEdit:
				edits = append(edits, v.TextEdit)
			default:
				t.Fatalf("an edit of type %T", e.Value)
			}
		}
		got := diffLinesAgainst(t, src, applyEdits(t, src, edits))
		sameList(t, "the edit at "+tc.mark, got, []string{tc.line})
	}
	// Where a ! is already, or not allowed, or not needed: no action.
	for _, mark := range []string{
		"\tos.Rem|ove!(path)",        // has one
		"_ = os.Rem|ove(path)",       // the error is dropped on purpose
		"fmt.Prin|tln(path)",         // two results: a statement can't take the value
		"if os.Rem|ove(path) != nil", // the error is used
		"return os.Rem|ove(path)",    // returned
		"\tos.Rem|ove(path)\n}\n",    // handled by the must function c!
		"func| a(path",               // not a call
		"defer fmt.Prin|tln(path)",   // two results: a deferred call can't take the value either
		"ty|ped()",                   // the last result is *MyErr, not error
		"u, v, e := tw|o(path)",      // the error is taken
		"os.Remove(pa|th)",           // the argument
	} {
		if as := actions(mark); len(as) != 0 {
			t.Errorf("at %q: actions %+v", mark, as)
		}
	}
	// a selection of lines is not a place on a name
	p0, p1 := at(t, src, "\tos.Rem|ove(path)\n\tdefer"), at(t, src, "defer os.Rem|ove(path)")
	resp0 := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[]}}`,
		c.uri("a/a.gp"), p0.Line, p0.Character, p1.Line, p1.Character))
	var multi []protocol.CodeAction
	if err := json.Unmarshal(resp0.Result, &multi); err != nil {
		t.Fatal(err)
	}
	for _, a := range multi {
		if a.Kind == protocol.RefactorRewrite {
			t.Errorf("an action for a selection of two lines: %+v", a)
		}
	}
	// and a request that wants other kinds does not get it
	p := at(t, src, "\tos.Rem|ove(path)\n\tdefer")
	resp := c.call("textDocument/codeAction", fmt.Sprintf(`{"textDocument":{"uri":%q},"range":{"start":{"line":%d,"character":%d},"end":{"line":%d,"character":%d}},"context":{"diagnostics":[],"only":["source.organizeImports"]}}`,
		c.uri("a/a.gp"), p.Line, p.Character, p.Line, p.Character))
	if string(resp.Result) != "[]" {
		t.Errorf("only organize imports: %s", resp.Result)
	}
}

// macroProject returns the files of the generated macro project (internal/lsp/bench) and its file full of
// `!` and macro forms.
func macroProject() (files map[string]string, rel string) {
	p := bench.Generate(bench.Macro)
	files = map[string]string{"gocpp.mod": "module " + p.Module + "\n"}
	for _, f := range p.Files {
		if f.Kind == bench.Source {
			files[f.Rel+".gp"] = f.Content
		}
	}
	return files, p.MacroFile + ".gp"
}

// TestBangSyntaxErrors: a syntax error in one place does not take the `!` features of the rest of the file (the
// state of a file while it is typed). Each token of the macro project's file in turn (a stride of them) is deleted;
// the hints of the `!` calls and the tokens `!` that are not in the paragraph of the deletion, or the one after
// it (where the parser may have lost its way), are those of the file without the error; nothing crashes, and a
// hover on a call outside still says what happens.
func TestBangSyntaxErrors(t *testing.T) {
	files, rel := macroProject()
	c := startRefs(t, files, plainInit)
	src := files[rel]
	c.open(rel, src)
	feature := func() (hints, ops map[string]bool) {
		hints, ops = map[string]bool{}, map[string]bool{}
		for _, h := range c.hints(rel, nil) {
			if strings.Contains(h, "→") {
				hints[h] = true
			}
		}
		for _, tk := range c.semTokens(rel, nil) {
			if strings.Contains(tk, " ! operator") {
				ops[tk] = true
			}
		}
		return
	}
	baseHints, baseOps := feature()
	if len(baseHints) < 10 || len(baseOps) < 6 {
		t.Fatalf("the file has %d hints and %d tokens", len(baseHints), len(baseOps))
	}
	// paragraphs: [start, end) offsets of the blocks between blank lines
	var paras [][2]int
	start := 0
	for _, loc := range regexp.MustCompile(`\n\n+`).FindAllStringIndex(src, -1) {
		paras = append(paras, [2]int{start, loc[1]})
		start = loc[1]
	}
	paras = append(paras, [2]int{start, len(src) + 1})
	lineOf := func(off int) int { return strings.Count(src[:min(off, len(src))], "\n") + 1 }
	toks := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*!?|[0-9]+|"[^"\n]*"|[^\sA-Za-z0-9_]`).FindAllStringIndex(src, -1)
	tried, kept := 0, 0
	header := strings.Index(src, "\nvar (") // an error in the package clause or the imports cannot be repaired
	for k := 3; k < len(toks); k += 11 {
		a, b := toks[k][0], toks[k][1]
		if a < header {
			continue
		}
		bad := src[:a] + src[b:]
		c.change(rel, 2+k, bad)
		gotHints, gotOps := feature()
		checkTokenOrder(t, c, rel, bad) // what the repaired paths answer is valid: in order, in the text
		tried++
		// the paragraphs that may be damaged: the one of the deletion and the next
		lo, hi := 0, 0
		for i, p := range paras {
			if p[0] <= a && a < p[1] {
				lo, hi = lineOf(p[0]), lineOf(paras[min(i+1, len(paras)-1)][1])
			}
		}
		outside := func(item string) bool { // "line:col ..." with 1-based lines
			var line int
			fmt.Sscanf(item, "%d:", &line)
			return line < lo || line > hi
		}
		for h := range baseHints {
			if outside(h) && !gotHints[h] {
				t.Errorf("deleting %q at line %d: the hint %q is gone", src[a:b], lineOf(a), h)
			}
		}
		for o := range baseOps {
			if outside(o) && !gotOps[o] {
				t.Errorf("deleting %q at line %d: the token %q is gone", src[a:b], lineOf(a), o)
			}
		}
		if len(gotHints) >= len(baseHints) {
			kept++
		}
	}
	t.Logf("%d deletions, %d kept every hint", tried, kept)
	if tried < 20 {
		t.Fatalf("%d deletions, %d kept every hint", tried, kept)
	}
	// and a hover outside the broken place
	bad := strings.Replace(src, "func Parse", "func ((Parse", 1)
	c.change(rel, 9999, bad)
	h, _ := c.hover(rel, bad, "os.Open|!(path)")
	if !strings.Contains(h, "returns it") {
		t.Errorf("hover on os.Open! in a file with a syntax error: %q", h)
	}
}

// TestBangUserNamesWithMustPrefix: the names that the rewrite of the `!` forms gives to results are dropped from a
// hover, but names that the user wrote and that start with must_ are not reserved (ADR-037) and stay.
func TestBangUserNamesWithMustPrefix(t *testing.T) {
	src := `package a

func Named(must_count int, gp_x string) (must_r int, err error) { return }

func Other(must_e1 int, must_t2 string) (must_err int, must_r_1 error) { return }

func Mixed(path string) (n int, must_r2 string) { return }
`
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	for _, tc := range []struct{ mark, want string }{
		{"func Na|med", "func Named(must_count int, gp_x string) (must_r int, err error)"},
		{"func Ot|her", "func Other(must_e1 int, must_t2 string) (must_err int, must_r_1 error)"},
		{"func Mi|xed", "func Mixed(path string) (n int, must_r2 string)"},
	} {
		if text, _ := c.hover("a/a.gp", src, tc.mark); text != tc.want {
			t.Errorf("hover at %q: %q, want %q", tc.mark, text, tc.want)
		}
	}
	// and the one that the rewrite wrote is dropped (the function of bangFiles with a deferred ! call)
	b := startRefs(t, bangFiles, plainInit)
	b.open("b/b.gp", bangFiles["b/b.gp"])
	if text, _ := b.hover("b/b.gp", bangFiles["b/b.gp"], "func re|t(path"); text != "func ret(path string) (int, error)" {
		t.Errorf("hover of ret: %q", text)
	}
}

// TestExpansionFiles: the files that show expansions: a folder of their own for each source (two files with the
// same name in two packages do not overwrite each other), private to the user, only when the client can show
// documents, only for .gp files; a folder of a killed server is removed by the next one, and nothing else is.
func TestExpansionFiles(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	mk := func(pkg, extra string) string {
		return "package " + pkg + "\n\nimport \"strconv\"\n\nfunc F" + extra + "(s string) (int, error) {\n\tn := strconv.Atoi!(s)\n\treturn n, nil\n}\n"
	}
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/f.gp": mk("a", "A"), "b/f.gp": mk("b", "B")}
	// a client that cannot show documents: the answer comes with no request from the server
	c := startRefs(t, files, plainInit)
	run := func(name string) ExpandResult {
		t.Helper()
		c.open(name, files[name])
		resp := c.call("workspace/executeCommand", fmt.Sprintf(`{"command":%q,"arguments":[{"uri":%q,"range":{"start":{"line":5,"character":3},"end":{"line":5,"character":3}}}]}`, ExpandCommand, c.uri(name)))
		if resp.Error != nil {
			t.Fatalf("%s: %v", name, resp.Error)
		}
		var er ExpandResult
		if err := json.Unmarshal(resp.Result, &er); err != nil {
			t.Fatal(err)
		}
		return er
	}
	a, b := run("a/f.gp"), run("b/f.gp")
	if a.URI == b.URI || filepath.Base(a.URI.Path()) != "f"+ExpandExt || filepath.Base(b.URI.Path()) != "f"+ExpandExt {
		t.Fatalf("two files called f.gp: %s and %s", a.URI, b.URI)
	}
	for _, er := range []ExpandResult{a, b} {
		data, err := os.ReadFile(er.URI.Path())
		if err != nil || string(data) != er.Text {
			t.Errorf("%s: %v", er.URI, err)
		}
	}
	if da, db := mustRead(t, a.URI.Path()), mustRead(t, b.URI.Path()); !strings.Contains(da, "func FA(") || !strings.Contains(db, "func FB(") {
		t.Errorf("the files hold the expansion of the other package: %q, %q", da, db)
	}
	if runtime.GOOS != "windows" { // (Windows has no modes of files)
		for _, p := range []string{a.URI.Path(), filepath.Dir(a.URI.Path()), filepath.Dir(filepath.Dir(a.URI.Path()))} {
			fi, err := os.Stat(p)
			if err != nil || fi.Mode().Perm()&0o077 != 0 {
				t.Errorf("%s is not private: %v %v", p, fi, err)
			}
		}
	}
	// only .gp files
	resp := c.call("workspace/executeCommand", fmt.Sprintf(`{"command":%q,"arguments":[{"uri":%q,"range":{"start":{"line":0,"character":0},"end":{"line":0,"character":0}}}]}`, ExpandCommand, c.uri("a/notes.txt")))
	if resp.Error == nil || !strings.Contains(resp.Error.Message, "is not a .gp file") {
		t.Errorf("a .txt file: %s %v", resp.Result, resp.Error)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestExpansionsStale: the folders of killed servers go, only those; and after the end of the server nothing is
// written again.
func TestExpansionsStale(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	old := time.Now().Add(-48 * time.Hour)
	mk := func(name string, marker bool, mtime time.Time) string {
		dir := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o700); err != nil {
			t.Fatal(err)
		}
		if marker {
			m := filepath.Join(dir, expandMarker)
			if err := os.WriteFile(m, nil, 0o600); err != nil {
				t.Fatal(err)
			}
			os.Chtimes(m, mtime, mtime)
		}
		return dir
	}
	stale := mk(expandDirPrefix+"111", true, old)
	young := mk(expandDirPrefix+"222", true, time.Now().Add(-time.Hour))
	noMarker := mk(expandDirPrefix+"333", false, old)
	other := mk("somebody-elses", true, old)
	removeStale(ownedByUs)
	for dir, wantThere := range map[string]bool{stale: false, young: true, noMarker: true, other: true} {
		if _, err := os.Stat(dir); (err == nil) != wantThere {
			t.Errorf("%s: there = %v, want %v", filepath.Base(dir), err == nil, wantThere)
		}
	}
	var x expansions
	path, err := x.write("k", "n", "text")
	if err != nil || mustRead(t, path) != "text" {
		t.Fatalf("write: %v", err)
	}
	dir := filepath.Dir(filepath.Dir(path))
	x.close()
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("the folder is still there after close")
	}
	if p, err := x.write("k", "n", "text"); err == nil {
		t.Errorf("write after close made %s", p)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Errorf("write after close made the folder again")
	}
}

// TestBangHoverMovedNames: the lowering moves the name of some `!` calls to code of its own (the condition and the
// post statement of a for, the second operand of &&, a chain, a deferred call of a function of the package, a var
// group), where it has another place. The hover is still found, by the call's own record.
func TestBangHoverMovedNames(t *testing.T) {
	src := `package main

import (
	"os"
	"strconv"
)

type F struct{}

func (F) Read!(n int) int { return n }

func open!(n int) F { return F{} }

func lim!(i int) int { return 3 }

func chk!(i int) bool { return true }

func ok!(i int) bool { return true }

func step!(n int) {}

func parse!(s string) int { return len(s) }

func use(n int, s string) (int, error) {
	for i := 0; i < lim!(i); i = lim!(i) + 1 {
	}
	for i := lim!(0); i < 3; i++ {
	}
	if chk!(n) && chk!(n+1) {
	}
	switch {
	case ok!(n):
	}
	open!(n).Read!(n)
	defer step!(n)
	var (
		a = parse!(s)
		b = strconv.Atoi!(s)
	)
	println(a, b)
	_ = #text(parse!(s))
	return 0, nil
}

func init!() {
	os.Remove!("x")
}

func main!() {
	os.Remove!("y")
}
`
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	for _, tc := range []struct{ mark, rng, sig, behavior string }{
		{"i < li|m!(i)", "lim", "func lim(i int) (int, error)", "`use` returns it"},
		{"i = li|m!(i)", "lim", "func lim(i int) (int, error)", "`use` returns it"},
		{"i := li|m!(0)", "lim", "func lim(i int) (int, error)", "`use` returns it"},
		{"if ch|k!(n) &&", "chk", "func chk(i int) (bool, error)", "`use` returns it"},
		{"&& ch|k!(n+1)", "chk", "func chk(i int) (bool, error)", "`use` returns it"},
		{"case o|k!(n)", "ok", "func ok(i int) (bool, error)", "`use` returns it"},
		{"op|en!(n).Read!(n)", "open", "func open(n int) (F, error)", "`use` returns it"},
		{"Re|ad!(n)\n\tdefer", "Read", "func (F) Read(n int) (int, error)", "`use` returns it"},
		{"defer st|ep!(n)", "step", "func step(n int) error", "If this call fails while `use` is about to return a nil error"},
		{"a = pa|rse!(s)", "parse", "func parse(s string) (int, error)", "`use` returns it"},
		{"b = strconv.At|oi!(s)", "Atoi", "func strconv.Atoi(s string) (int, error)", "`use` returns it"},
		{"os.Re|move!(\"x\")", "Remove", "func os.Remove(name string) error", "it stops before `main` starts"},
		{"os.Re|move!(\"y\")", "Remove", "func os.Remove(name string) error", "`main`'s defers run, then"},
		{"func in|it!()", "init", "func init() error", "A must function, declared `func init!()`"},
		{"func ma|in!()", "main", "func main() error", "A must function, declared `func main!()`"},
	} {
		text, rng := c.hover("a/a.gp", src, tc.mark)
		if rng != tc.rng || !strings.HasPrefix(text, tc.sig) || !strings.Contains(text, tc.behavior) {
			t.Errorf("hover at %q: %q (range %q), want %q ... %q (range %q)", tc.mark, text, rng, tc.sig, tc.behavior, tc.rng)
		}
	}
	// main! and init! have a sentence of their own
	if text, _ := c.hover("a/a.gp", src, "func in|it!()"); !strings.Contains(text, "before `main` starts") {
		t.Errorf("hover on init!: %q", text)
	}
	if text, _ := c.hover("a/a.gp", src, "func ma|in!()"); !strings.Contains(text, "`main`'s defers run first") {
		t.Errorf("hover on main!: %q", text)
	}
	// #text(expr) is the text of the expression, not code: there is no call
	if text, _ := c.hover("a/a.gp", src, "#text(pa|rse!(s))"); text != "" {
		t.Errorf("hover in #text(): %q", text)
	}
}

// TestMacroStackedWraps: every use of a macro on top of a func resolves (hover, definition), also the second and later of
// several wrap macros that stand on one func (tests/run/umacro_stack.gp has 112): the rewrite nests the code of the
// function in one func literal for each, and the uses are found by their places.
func TestMacroStackedWraps(t *testing.T) {
	data, err := os.ReadFile("../../../tests/run/umacro_stack.gp")
	if err != nil {
		t.Fatal(err)
	}
	src := strings.Replace(string(data), "// phase: 05\n", "", 1)
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	m := protocol.NewMapper(src)
	uses := regexp.MustCompile(`(?m)^#Step\(`).FindAllStringIndex(src, -1)
	if len(uses) < 100 {
		t.Fatalf("%d uses", len(uses))
	}
	declLine := strings.Count(src[:strings.Index(src, "macro Step")], "\n") + 1
	bad := 0
	for _, u := range uses {
		pos := m.Position(u[0] + 3) // in the name
		params := fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, c.uri("a/a.gp"), pos.Line, pos.Character)
		resp := c.call("textDocument/hover", params)
		var h protocol.Hover
		if resp.Error != nil || string(resp.Result) == "null" || json.Unmarshal(resp.Result, &h) != nil || !strings.HasPrefix(h.Contents.Value, "macro Step(name string)") {
			bad++
			if bad < 4 {
				t.Errorf("hover at line %d: %s %v", pos.Line+1, resp.Result, resp.Error)
			}
			continue
		}
		resp = c.call("textDocument/definition", params)
		var locs []protocol.Location
		if json.Unmarshal(resp.Result, &locs); len(locs) != 1 || int(locs[0].Range.Start.Line)+1 != declLine {
			bad++
			if bad < 4 {
				t.Errorf("definition at line %d: %s", pos.Line+1, resp.Result)
			}
		}
	}
	if bad > 0 {
		t.Errorf("%d of %d uses do not resolve", bad, len(uses))
	}
}

// TestBangGuessedKinds: in a package with an error the kind of a `!` is guessed from the text of the signature of
// its function (check.mustPass.guessKind): *testing.T, B and F make it t.Fatal, a last result error makes it a
// return, also in a func literal, whose own signature counts, and the others panic.
func TestBangGuessedKinds(t *testing.T) {
	src := `package a

import (
	"strconv"
	"testing"
)

func bench(b *testing.B, s string) {
	n := strconv.Atoi!(s)
	_ = n
}

func fuzz(f *testing.F, s string) {
	n := strconv.Atoi!(s)
	_ = n
}

func test(t *testing.T, s string) {
	n := strconv.Atoi!(s)
	_ = n
}

func outer(s string) error {
	f := func() (int, error) { return strconv.Atoi!(s), nil }
	g := func() int { return strconv.Atoi!(s) }
	_, _ = f, g
	return nil
}

func plain(s string) int {
	return strconv.Atoi!(s)
}

var zzBad int = "s"
`
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	var got []string
	for _, h := range c.hints("a/a.gp", nil) {
		if strings.Contains(h, "→") {
			got = append(got, h)
		}
	}
	sameList(t, "the guessed kinds", got, []string{
		"9:23 → t.Fatal kind 0 left",
		"14:23 → t.Fatal kind 0 left",
		"19:23 → t.Fatal kind 0 left",
		"24:52 → return err kind 0 left",
		"25:43 → panic kind 0 left",
		"31:25 → panic kind 0 left",
	})
}

// TestBangPartialMatchesClean: a package with an error (here a type error added at the end) keeps what the `!`
// features say about the rest of a file: the hints of the `!` calls, the `!` tokens and the hovers of the calls (but for the
// code that stands for the statement, which needs the lowering) are those of the clean file, over the programs
// of the language tests of Phase 10.
func TestBangPartialMatchesClean(t *testing.T) {
	list, _ := filepath.Glob("../../../tests/run/must_*.gp")
	macros, _ := filepath.Glob("../../../tests/run/umacro_*.gp")
	list = append(list, macros...)
	if len(list) < 100 {
		t.Fatalf("%d programs", len(list))
	}
	collect := func(c *refClient, src string) map[string]string {
		out := map[string]string{}
		for _, h := range c.hints("a/a.gp", nil) {
			if strings.Contains(h, "→") {
				out["hint "+h] = ""
			}
		}
		for _, tk := range c.semTokens("a/a.gp", nil) {
			if strings.Contains(tk, " ! operator") {
				out["token "+tk] = ""
			}
		}
		for _, m := range regexp.MustCompile(`\w+!\(`).FindAllStringIndex(src, -1) {
			line := strings.Count(src[:m[0]], "\n")
			col := m[0] - (strings.LastIndex(src[:m[0]], "\n") + 1) + 1
			resp := c.call("textDocument/hover", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d}}`, c.uri("a/a.gp"), line, col))
			var h protocol.Hover
			json.Unmarshal(resp.Result, &h)
			text, _, _ := strings.Cut(h.Contents.Value, "\n// the code that stands")
			out[fmt.Sprintf("hover %d:%d", line+1, col)] = text
		}
		return out
	}
	checked, diffs := 0, 0
	for _, f := range list {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := strings.Replace(string(data), "// phase:", "// phase :", 1)
		files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
		c := startRefs(t, files, plainInit)
		c.open("a/a.gp", src)
		clean := collect(c, src)
		if len(clean) == 0 {
			continue
		}
		bad := src + "\nvar zzBad int = \"s\"\n"
		c.change("a/a.gp", 2, bad)
		part := collect(c, bad)
		checked++
		for k, v := range clean {
			if pv, ok := part[k]; !ok || pv != v {
				if diffs++; diffs < 10 {
					t.Errorf("%s: %s: clean %q, with an error %q", filepath.Base(f), k, v, pv)
				}
			}
		}
		for k := range part {
			if _, ok := clean[k]; !ok {
				if diffs++; diffs < 10 {
					t.Errorf("%s: %s: only with an error", filepath.Base(f), k)
				}
			}
		}
	}
	if checked < 100 {
		t.Errorf("only %d programs have forms to compare", checked)
	}
	t.Logf("%d programs, %d differences", checked, diffs)
}

// TestBangInsideBrokenDeclaration: the declaration with the syntax error is blanked for the repaired check, and what is in it
// comes from the check of the text as it is, as far as the parser's tree goes: the `!` calls before the error keep their hint, token
// and hover.
func TestBangInsideBrokenDeclaration(t *testing.T) {
	src := `package a

import (
	"os"
	"strconv"
)

func Broken(a string) (int, error) {
	x := strconv.Atoi!(a)
	f := os.Open!(a)
	println(x, f
	return 1, nil
}

func Fine(a string) (int, error) {
	y := strconv.Atoi!(a)
	return y, nil
}
`
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "a/a.gp": src}
	c := startRefs(t, files, plainInit)
	c.open("a/a.gp", src)
	hints := c.hints("a/a.gp", nil)
	for _, want := range []string{"9:23 → return err kind 0 left", "10:18 → return err kind 0 left", "16:23 → return err kind 0 left"} {
		if !slices.Contains(hints, want) {
			t.Errorf("no hint %q in %q", want, hints)
		}
	}
	toks := c.semTokens("a/a.gp", nil)
	for _, want := range []string{"9:19 ! operator ", "10:14 ! operator ", "16:19 ! operator "} {
		if !slices.Contains(toks, want) {
			t.Errorf("no token %q in %q", want, toks)
		}
	}
	// the names that the pass wrote for the `!` forms of the broken body are not the user's: no hint names a parameter of the helper
	for _, h := range hints {
		if strings.Contains(h, " v:") || strings.Contains(h, "err:") || strings.Contains(h, "results:") || strings.Contains(h, "gp_") {
			t.Errorf("a hint for the code of the pass: %q", h)
		}
	}
	for _, tok := range toks {
		if strings.Contains(tok, "gp_must") || strings.Contains(tok, " M1") || strings.Contains(tok, " M2") {
			t.Errorf("a token for the code of the pass: %q", tok)
		}
	}
	for _, mark := range []string{"x := strconv.At|oi!(a)", "f := os.Op|en!(a)", "y := strconv.At|oi!(a)\n\treturn y"} {
		if text, _ := c.hover("a/a.gp", src, mark); !strings.Contains(text, "returns it") {
			t.Errorf("hover at %q: %q", mark, text)
		}
	}
}
