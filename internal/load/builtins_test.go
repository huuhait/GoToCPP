package load

import (
	"bytes"
	"errors"
	"go/printer"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// The tests of the built-in macros (#file, #line, #func, #text, #embed, #body)
// and of //go:embed (step 10.8): each loads a small module with the syntax of
// Phase 10 on, prints the files of the main package as they are after the
// expansion, and compares the text, or the errors.

// embedStd is fakeStd with the package embed, which only declares FS here.
var embedStd = func() fstest.MapFS {
	m := maps.Clone(fakeStd)
	m["embed/embed.gp"] = &fstest.MapFile{Data: []byte("package embed\n\ntype FS struct{ files []string }\n")}
	return m
}()

func embedGoStd(path string) bool { return path == "embed" || goStd(path) }

// macroConfig is the loader configuration of these tests.
func macroConfig() Config {
	return Config{Phase10Syntax: true, Stdlib: embedStd, GoStd: embedGoStd}
}

// loadMacros loads the module in files (a gocpp.mod is added unless there is
// one) and returns the program, the package in the folder of the module, and
// the folder.
func loadMacros(t *testing.T, files map[string]string) (*Program, *Package, string) {
	t.Helper()
	files = maps.Clone(files)
	if _, ok := files["gocpp.mod"]; !ok {
		files["gocpp.mod"] = "module example.com/app\n\ngocpp 0.1\n"
	}
	root := writeTree(t, files)
	prog := loadIn(t, macroConfig(), root, ".")
	return prog, pkgOf(t, prog, "example.com/app"), root
}

// printedFile returns the file name of p as it is after the expansion.
func printedFile(t *testing.T, prog *Program, p *Package, name string) string {
	t.Helper()
	for _, f := range p.Files {
		if filepath.Base(f.Path) == name {
			var b bytes.Buffer
			if err := printer.Fprint(&b, prog.Fset, f.AST); err != nil {
				t.Fatal(err)
			}
			return b.String()
		}
	}
	t.Fatalf("no file %s in %s", name, p.ImportPath)
	return ""
}

// macroCase is one program and what the loader makes of it: the strings that
// the printed .gp files of the main package must have, and the errors of the
// program (all of them, as errorText prints them).
type macroCase struct {
	name  string
	files map[string]string
	want  []string
	errs  string
}

func runMacroCases(t *testing.T, cases []macroCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog, p, root := loadMacros(t, c.files)
			if got := errorText(t, prog, root); got != c.errs {
				t.Errorf("errors:\n%s\nwant:\n%s", got, c.errs)
			}
			var all string
			for _, f := range p.Files {
				all += printedFile(t, prog, p, filepath.Base(f.Path))
			}
			for _, w := range c.want {
				if !strings.Contains(all, w) {
					t.Errorf("missing %s in:\n%s", w, all)
				}
			}
		})
	}
}

func TestBuiltinMacros(t *testing.T) {
	const head = "package main\n\nimport \"fmt\"\n\n"
	one := func(src string) map[string]string { return map[string]string{"main.gp": src} }
	runMacroCases(t, []macroCase{
		{name: "file", files: one(head + "func main() { fmt.Println(#file()) }\n"),
			want: []string{`fmt.Println(("main.gp"))`}},
		{name: "line", files: one(head + "func main() {\n\n\tfmt.Println(#line())\n}\n"),
			want: []string{"fmt.Println((7))"}},
		{name: "two lines apart", files: one(head + "func main() {\n\tfmt.Println(#line(),\n\t\t#line())\n}\n"),
			want: []string{"fmt.Println((6),", "(7))"}},
		{name: "func main", files: one(head + "func main() { fmt.Println(#func()) }\n"),
			want: []string{`fmt.Println(("main.main"))`}},
		{name: "func plain", files: one(head + "func f() string { return #func() }\nfunc main() { fmt.Println(f()) }\n"),
			want: []string{`return ("main.f")`}},
		{name: "func method pointer", files: one(head + "type T struct{}\n\nfunc (t *T) M() string { return #func() }\nfunc main() { fmt.Println((&T{}).M()) }\n"),
			want: []string{`return ("main.(*T).M")`}},
		{name: "func method value receiver", files: one(head + "type T struct{}\n\nfunc (t T) M() string { return #func() }\nfunc main() { fmt.Println(T{}.M()) }\n"),
			want: []string{`return ("main.T.M")`}},
		{name: "func method over int", files: one(head + "type V int\n\nfunc (v V) W() string { return #func() }\nfunc main() { fmt.Println(V(1).W()) }\n"),
			want: []string{`return ("main.V.W")`}},
		{name: "func closure", files: one(head + "func main() {\n\tf := func() string { return #func() }\n\tfmt.Println(f())\n}\n"),
			want: []string{`return ("main.main.func1")`}},
		{name: "func closures numbered", files: one(head + "func main() {\n\ta := func() string { return #func() }\n\tb := func() string { return #func() }\n\tfmt.Println(a(), b(), #func())\n}\n"),
			want: []string{`return ("main.main.func1")`, `return ("main.main.func2")`, `fmt.Println(a(), b(), ("main.main"))`}},
		{name: "func closure in closure", files: one(head + "func main() {\n\tf := func() string {\n\t\tg := func() string { return #func() }\n\t\th := func() string { return #func() }\n\t\treturn g() + h() + #func()\n\t}\n\tfmt.Println(f())\n}\n"),
			want: []string{`return ("main.main.func1.1")`, `return ("main.main.func1.2")`, `("main.main.func1")`}},
		{name: "func closure after a closure with a closure", files: one(head + "func main() {\n\t_ = func() { _ = func() {} }\n\t_ = func() string { return #func() }\n\tfmt.Println()\n}\n"),
			want: []string{`return ("main.main.func2")`}},
		{name: "func closure in method", files: one(head + "type T struct{}\n\nfunc (t *T) M() func() string { return func() string { return #func() } }\nfunc main() { fmt.Println((&T{}).M()()) }\n"),
			want: []string{`return ("main.(*T).M.func1")`}},
		{name: "func package variable closures", files: one(head + "var a = func() string { return #func() }\nvar b = func() string { return #func() }\nfunc main() { fmt.Println(a(), b()) }\n"),
			want: []string{`return ("main.init.func1")`, `return ("main.init.func2")`}},
		{name: "func package variable", files: one(head + "var a = #func()\nfunc main() { fmt.Println(a) }\n"),
			want: []string{`var a = ("main.init")`}},
		{name: "func init functions", files: one(head + "func init() { fmt.Println(#func()) }\nfunc init() { fmt.Println(#func()) }\nfunc main() {}\n"),
			want: []string{`fmt.Println(("main.init.0"))`, `fmt.Println(("main.init.1"))`}},
		{name: "func init closure", files: one(head + "func init() { func() { fmt.Println(#func()) }() }\nfunc main() {}\n"),
			want: []string{`fmt.Println(("main.init.0.func1"))`}},
		{name: "func generic function", files: one(head + "func F[X any]() string { return #func() }\nfunc main() { fmt.Println(F[int]()) }\n"),
			want: []string{`return ("main.F[...]")`}},
		{name: "func generic method", files: one(head + "type G[X any] struct{}\n\nfunc (g *G[X]) M() string { return #func() }\nfunc (g G[X]) N() string { return #func() }\nfunc main() { fmt.Println((&G[int]{}).M()) }\n"),
			want: []string{`return ("main.(*G[...]).M")`, `return ("main.G[...].N")`}},
		{name: "func generic closure", files: one(head + "func F[X any]() string { return func() string { return #func() }() }\nfunc main() { fmt.Println(F[int]()) }\n"),
			want: []string{`return ("main.F[...].func1")`}},
		{name: "func of a must function", files: one(head + "func parse!() int { println(#func()); return 1 }\nfunc main() {}\n"),
			want: []string{`println(("main.parse"))`}},
		{name: "func in go and defer", files: one(head + "func main() {\n\tdefer func() { fmt.Println(#func()) }()\n\tgo func() { fmt.Println(#func()) }()\n}\n"),
			want: []string{`fmt.Println(("main.main.func1"))`, `fmt.Println(("main.main.func2"))`}},
		{name: "func in a struct literal", files: one(head + "type S struct{ F func() string }\n\nfunc main() {\n\ts := S{F: func() string { return #func() }}\n\tfmt.Println(s.F())\n}\n"),
			want: []string{`return ("main.main.func1")`}},
		{name: "text simple", files: one(head + "func main() { fmt.Println(#text(a + b)) }\n"),
			want: []string{`fmt.Println(("a + b"))`}},
		{name: "text keeps the spacing", files: one(head + "func main() { fmt.Println(#text(a   +b)) }\n"),
			want: []string{`fmt.Println(("a   +b"))`}},
		{name: "text multi line", files: one(head + "func main() {\n\tfmt.Println(#text(a +\n\t\t\tb))\n}\n"),
			want: []string{`fmt.Println(("a + b"))`}},
		{name: "text comment dropped", files: one(head + "func main() { fmt.Println(#text(a /* c */ + b)) }\n"),
			want: []string{`fmt.Println(("a + b"`}},
		{name: "text comment at the line end", files: one(head + "func main() {\n\tfmt.Println(#text(f(a, // first\n\t\tb)))\n}\n"),
			want: []string{`fmt.Println(("f(a, b)"))`}},
		{name: "text strings stay", files: one(head + "func main() { fmt.Println(#text(f(\"a  b\", `c\nd`))) }\n"),
			want: []string{"fmt.Println((\"f(\\\"a  b\\\", `c\\nd`)\"))"}},
		{name: "text unicode", files: one(head + "func main() { fmt.Println(#text(\"héllo, 世界\" + s)) }\n"),
			want: []string{`fmt.Println(("\"héllo, 世界\" + s"))`}},
		{name: "text of a call", files: one(head + "func main() { fmt.Println(#text(fmt.Sprint(1, 2))) }\n"),
			want: []string{`fmt.Println(("fmt.Sprint(1, 2)"))`}},
		{name: "text is not checked", files: one(head + "func main() { fmt.Println(#text(neverDeclared.X[3])) }\n"),
			want: []string{`fmt.Println(("neverDeclared.X[3]"))`}},
		{name: "text of a func literal", files: one(head + "func main() { fmt.Println(#text(func() int { return 1 })) }\n"),
			want: []string{`fmt.Println(("func() int { return 1 }"))`}},
		{name: "text is a constant", files: one(head + "const c = #text(x)\nfunc main() { fmt.Println(c) }\n"),
			want: []string{`const c = ("x")`}},
		{name: "line is a constant", files: one(head + "const l = #line()\nvar a [#line()]int\nfunc main() { fmt.Println(l, len(a)) }\n"),
			want: []string{`const l = (5)`, `var a [(6)]int`}},
		{name: "file is a constant", files: one(head + "const f = #file()\nfunc main() { fmt.Println(f, len(f)) }\n"),
			want: []string{`const f = ("main.gp")`}},
		{name: "nested in an argument", files: one(head + "func main() { fmt.Println(fmt.Sprint(#line(), #func())) }\n"),
			want: []string{`fmt.Sprint((5), ("main.main"))`}},
		{name: "in a switch and a case", files: one(head + "func main() {\n\tswitch #line() {\n\tcase 6:\n\t\tfmt.Println(#line())\n\t}\n}\n"),
			want: []string{"switch 6 {", "fmt.Println((8))"}},
		{name: "a struct tag is no use", files: one(head + "type S struct {\n\tA int `json:\"#line()\"`\n}\nfunc main() { fmt.Println(S{}) }\n"),
			want: []string{"`json:\"#line()\"`"}},
		{name: "a string is no use", files: one(head + "func main() { fmt.Println(\"#line()\", `#file()`) }\n"),
			want: []string{"`#file()`", `"#line()"`}},
		{name: "line directive changes the file and line", files: one(head + "func main() {\n//line other.gp:100\n\tfmt.Println(#file(), #line())\n}\n"),
			want: []string{`fmt.Println(("other.gp"), (100))`}},
		{name: "declaration group", files: one(head + "var (\n\ta = #line()\n\tb = #line()\n)\nfunc main() { fmt.Println(a, b) }\n"),
			want: []string{"= (6)", "= (7)"}},
		{name: "two files share the init counters", files: map[string]string{
			"a.gp": head + "func init() { fmt.Println(#func()) }\nvar a = func() string { return #func() }\nfunc main() {}\n",
			"b.gp": "package main\n\nfunc init() { println(#func()) }\nvar b = func() string { return #func() }\n",
		}, want: []string{`fmt.Println(("main.init.0"))`, `println(("main.init.1"))`, `return ("main.init.func1")`, `return ("main.init.func2")`}},
		{name: "go embed and macros side by side", files: map[string]string{
			"main.gp":  head + "import _ \"embed\"\n\n//go:embed data.txt\nvar d string\n\nfunc main() { fmt.Println(d, #line()) }\n",
			"data.txt": "hi\n",
		}, want: []string{`var d string = ("hi\n")`, `fmt.Println(d, (10))`}},
	})
}

// TestBuiltinMacroFileNames checks #file and #func of packages below the
// module's root, and of a program without gocpp.mod.
func TestBuiltinMacroFileNames(t *testing.T) {
	prog, _, _ := loadMacros(t, map[string]string{
		"main.gp":        "package main\n\nimport _ \"example.com/app/a/b\"\n\nfunc main() {}\n",
		"a/b/b.gp":       "package b\n\nimport _ \"example.com/app/a/b/c\"\n\nvar F = #file()\nvar G = #func()\nvar H = #line()\n",
		"a/b/other_x.gp": "package b\n\nvar I = #file()\n",
		"a/b/c/c.gp":     "package c\n\nvar K = #func()\n",
	})
	b := pkgOf(t, prog, "example.com/app/a/b")
	got := printedFile(t, prog, b, "b.gp") + printedFile(t, prog, b, "other_x.gp")
	for _, w := range []string{`var F = ("a/b/b.gp")`, `var G = ("example.com/app/a/b.init")`, `var H = (7)`, `var I = ("a/b/other_x.gp")`} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %s in:\n%s", w, got)
		}
	}
	if c := pkgOf(t, prog, "example.com/app/a/b/c"); !strings.Contains(printedFile(t, prog, c, "c.gp"), `("example.com/app/a/b/c.init")`) {
		t.Errorf("c: %s", printedFile(t, prog, c, "c.gp"))
	}

	// Without a module, the name of the file.
	root := writeTree(t, map[string]string{"prog.gp": "package main\n\nfunc main() { println(#file(), #line()) }\n"})
	p2 := loadIn(t, macroConfig(), root, filepath.Join(root, "prog.gp"))
	if got := printedFile(t, p2, p2.Roots[0], "prog.gp"); !strings.Contains(got, `println(("prog.gp"), (3))`) {
		t.Errorf("no module: %s", got)
	}
}

// TestBuiltinMacroErrors checks the errors of the built-in macros.
func TestBuiltinMacroErrors(t *testing.T) {
	const head = "package main\n\nimport \"fmt\"\n\n"
	cases := []struct{ name, src, errs string }{
		{"line with an argument", head + "func main() { fmt.Println(#line(1)) }\n",
			"# example.com/app\n./main.gp:5:33: #line takes no arguments\n"},
		{"file with an argument", head + "func main() { fmt.Println(#file(x)) }\n",
			"# example.com/app\n./main.gp:5:33: #file takes no arguments\n"},
		{"func with an argument", head + "func main() { fmt.Println(#func(\"a\")) }\n",
			"# example.com/app\n./main.gp:5:33: #func takes no arguments\n"},
		{"text without an argument", head + "func main() { fmt.Println(#text()) }\n",
			"# example.com/app\n./main.gp:5:27: #text takes one argument: #text(x)\n"},
		{"text with two arguments", head + "func main() { fmt.Println(#text(a, b)) }\n",
			"# example.com/app\n./main.gp:5:27: #text takes one argument: #text(x)\n"},
		{"body outside a macro", head + "func main() { #body() }\n",
			"# example.com/app\n./main.gp:5:15: #body() can only be used inside a macro declaration\n"},
		{"embed without an argument", head + "func main() { fmt.Println(#embed()) }\n",
			"# example.com/app\n./main.gp:5:27: #embed takes one argument, a string literal: #embed(\"file\")\n"},
		{"embed of a variable", head + "func main() { fmt.Println(#embed(name)) }\n",
			"# example.com/app\n./main.gp:5:27: #embed takes one argument, a string literal: #embed(\"file\")\n"},
		{"embed of a missing file", head + "func main() { fmt.Println(#embed(\"nope.txt\")) }\n",
			"# example.com/app\n./main.gp:5:34: #embed nope.txt: no matching files found\n"},
		{"embed with ..", head + "func main() { fmt.Println(#embed(\"../x\")) }\n",
			"# example.com/app\n./main.gp:5:34: #embed ../x: invalid file name\n"},
		{"embed with an absolute name", head + "func main() { fmt.Println(#embed(\"/etc/passwd\")) }\n",
			"# example.com/app\n./main.gp:5:34: #embed /etc/passwd: invalid file name\n"},
		{"embed of a folder", head + "func main() { fmt.Println(#embed(\"d\")) }\n",
			"# example.com/app\n./main.gp:5:34: #embed d: a folder can't be embedded as a string\n"},
		{"user macro is step 10.9", head + "func main() { #Debug(1) }\n",
			"# example.com/app\n./main.gp:5:15: macro #Debug is not supported yet (Phase 10, step 10.9)\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog, _, root := loadMacros(t, map[string]string{"main.gp": c.src, "d/x.txt": "x"})
			if got := errorText(t, prog, root); got != c.errs {
				t.Errorf("errors:\n%s\nwant:\n%s", got, c.errs)
			}
		})
	}
}

const embedHead = "package main\n\nimport _ \"embed\"\n\n"

func TestGoEmbed(t *testing.T) {
	prog := func(src string, extra ...string) map[string]string {
		m := map[string]string{"main.gp": src}
		for i := 0; i+1 < len(extra); i += 2 {
			m[extra[i]] = extra[i+1]
		}
		return m
	}
	runMacroCases(t, []macroCase{
		{name: "string", files: prog(embedHead+"//go:embed a.txt\nvar a string\n\nfunc main() {}\n", "a.txt", "alpha\n"),
			want: []string{`var a string = ("alpha\n")`}},
		{name: "bytes", files: prog(embedHead+"//go:embed a.txt\nvar a []byte\n\nfunc main() {}\n", "a.txt", "alpha"),
			want: []string{`var a []byte = []byte(("alpha"))`}},
		{name: "uint8 slice", files: prog(embedHead+"//go:embed a.txt\nvar a []uint8\n\nfunc main() {}\n", "a.txt", "x"),
			want: []string{`var a []uint8 = []uint8(("x"))`}},
		{name: "empty file", files: prog(embedHead+"//go:embed a.txt\nvar a string\n\nfunc main() {}\n", "a.txt", ""),
			want: []string{`var a string = ("")`}},
		{name: "binary file", files: prog(embedHead+"//go:embed a.bin\nvar a string\n\nfunc main() {}\n", "a.bin", "\x00\x01\xff\xfe\r\n"),
			want: []string{`var a string = ("\x00\x01\xff\xfe\r\n")`}},
		{name: "unicode file", files: prog(embedHead+"//go:embed a.txt\nvar a string\n\nfunc main() {}\n", "a.txt", "héllo 世界"),
			want: []string{`("héllo 世界")`}},
		{name: "subfolder", files: prog(embedHead+"//go:embed sub/dir/a.txt\nvar a string\n\nfunc main() {}\n", "sub/dir/a.txt", "deep"),
			want: []string{`("deep")`}},
		{name: "wildcard of one file", files: prog(embedHead+"//go:embed a.*\nvar a string\n\nfunc main() {}\n", "a.txt", "w"),
			want: []string{`("w")`}},
		{name: "quoted pattern with a space", files: prog(embedHead+"//go:embed \"a b.txt\"\nvar a string\n\nfunc main() {}\n", "a b.txt", "sp"),
			want: []string{`("sp")`}},
		{name: "back quoted pattern", files: prog(embedHead+"//go:embed `a.txt`\nvar a string\n\nfunc main() {}\n", "a.txt", "bq"),
			want: []string{`("bq")`}},
		{name: "two patterns of the same file", files: prog(embedHead+"//go:embed a.txt a.*\nvar a string\n\nfunc main() {}\n", "a.txt", "same"),
			want: []string{`("same")`}},
		{name: "two lines", files: prog(embedHead+"//go:embed a.txt\n//go:embed a.*\nvar a string\n\nfunc main() {}\n", "a.txt", "two"),
			want: []string{`("two")`}},
		{name: "comments between directive and var", files: prog(embedHead+"//go:embed a.txt\n// a note\n\n// another\nvar a string\n\nfunc main() {}\n", "a.txt", "gap"),
			want: []string{`("gap")`}},
		{name: "group of vars", files: prog(embedHead+"var (\n\t//go:embed a.txt\n\ta string\n\t//go:embed b.txt\n\tb []byte\n\tc int\n)\n\nfunc main() {}\n", "a.txt", "A", "b.txt", "B"),
			want: []string{`("A")`, `[]byte(("B"))`}},
		{name: "named type over string", files: prog(embedHead+"type S string\n\n//go:embed a.txt\nvar a S\n\nfunc main() {}\n", "a.txt", "named"),
			want: []string{`var a S = ("named")`}},
		{name: "named type over bytes", files: prog(embedHead+"type B []byte\n\n//go:embed a.txt\nvar a B\n\nfunc main() {}\n", "a.txt", "named"),
			want: []string{`var a B = B(("named"))`}},
		{name: "renamed import of embed", files: prog("package main\n\nimport e \"embed\"\n\nvar _ e.FS\n\n//go:embed a.txt\nvar a string\n\nfunc main() {}\n", "a.txt", "r"),
			want: []string{`var a string = ("r")`}},
		{name: "directive text is not a directive", files: prog("package main\n\n// go:embed a.txt\nvar a string\n\nfunc main() {}\n"),
			want: []string{`var a string`}},

		// errors
		{name: "missing file", files: prog(embedHead + "//go:embed nope.txt\nvar a string\n\nfunc main() {}\n"),
			errs: "# example.com/app\n./main.gp:5:12: pattern nope.txt: no matching files found\n"},
		{name: "wildcard without a match", files: prog(embedHead + "//go:embed *.zip\nvar a string\n\nfunc main() {}\n"),
			errs: "# example.com/app\n./main.gp:5:12: pattern *.zip: no matching files found\n"},
		{name: "no import of embed", files: prog("package main\n\n//go:embed a.txt\nvar a string\n\nfunc main() {}\n", "a.txt", "x"),
			errs: "# example.com/app\n./main.gp:3:1: go:embed requires import \"embed\" (or import _ \"embed\", if package is not used)\n"},
		{name: "escapes the module", files: prog(embedHead + "//go:embed ../a.txt\nvar a string\n\nfunc main() {}\n"),
			errs: "# example.com/app\n./main.gp:5:12: pattern ../a.txt: invalid pattern syntax\n"},
		{name: "absolute pattern", files: prog(embedHead + "//go:embed /a.txt\nvar a string\n\nfunc main() {}\n"),
			errs: "# example.com/app\n./main.gp:5:12: pattern /a.txt: invalid pattern syntax\n"},
		{name: "dot pattern", files: prog(embedHead + "//go:embed .\nvar a string\n\nfunc main() {}\n"),
			errs: "# example.com/app\n./main.gp:5:12: pattern .: invalid pattern syntax\n"},
		{name: "bad glob", files: prog(embedHead + "//go:embed [a\nvar a string\n\nfunc main() {}\n"),
			errs: "# example.com/app\n./main.gp:5:12: pattern [a: invalid pattern syntax\n"},
		{name: "no patterns", files: prog(embedHead + "//go:embed\nvar a string\n\nfunc main() {}\n"),
			errs: "# example.com/app\n./main.gp:5:1: usage: //go:embed pattern...\n"},
		{name: "two files in a string", files: prog(embedHead+"//go:embed a.txt b.txt\nvar a string\n\nfunc main() {}\n", "a.txt", "1", "b.txt", "2"),
			errs: "# example.com/app\n./main.gp:6:5: invalid go:embed: multiple files for type string\n"},
		{name: "folder in a string", files: prog(embedHead+"//go:embed d\nvar a string\n\nfunc main() {}\n", "d/x.txt", "1", "d/y.txt", "2"),
			errs: "# example.com/app\n./main.gp:6:5: invalid go:embed: multiple files for type string\n"},
		{name: "wrong type", files: prog(embedHead+"//go:embed a.txt\nvar a int\n\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:6:5: go:embed cannot apply to var of type int\n"},
		{name: "array type", files: prog(embedHead+"//go:embed a.txt\nvar a [4]byte\n\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:6:5: go:embed cannot apply to var of type [4]byte\n"},
		{name: "initializer", files: prog(embedHead+"//go:embed a.txt\nvar a string = \"x\"\n\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:6:5: go:embed cannot apply to var with initializer\n"},
		{name: "several names", files: prog(embedHead+"//go:embed a.txt\nvar a, b string\n\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:6:5: go:embed cannot apply to multiple vars\n"},
		{name: "on a const", files: prog(embedHead+"//go:embed a.txt\nconst a = 1\n\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:5:1: misplaced go:embed directive\n"},
		{name: "on a func", files: prog(embedHead+"//go:embed a.txt\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:5:1: misplaced go:embed directive\n"},
		{name: "inside a func", files: prog(embedHead+"func main() {\n\t//go:embed a.txt\n\tvar a string\n\t_ = a\n}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:6:2: go:embed cannot apply to var inside func\n"},
		{name: "at the end of the file", files: prog(embedHead+"func main() {}\n\n//go:embed a.txt\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:7:1: misplaced go:embed directive\n"},
		{name: "embed.FS is not available yet", files: prog("package main\n\nimport \"embed\"\n\n//go:embed a.txt\nvar f embed.FS\n\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:6:7: embed.FS is not available yet (Phase 09: it needs io/fs); //go:embed works on string and []byte variables\n"},
		{name: "a type over embed.FS is no FS", files: prog("package main\n\nimport \"embed\"\n\ntype T embed.FS\n\n//go:embed a.txt\nvar f T\n\nfunc main() {}\n", "a.txt", "1"),
			errs: "# example.com/app\n./main.gp:8:5: go:embed cannot apply to var of type T\n"},
		{name: "invalid file name", files: prog(embedHead+"//go:embed d\nvar a []byte\n\nfunc main() {}\n", "d/a'b.txt", "1"),
			errs: "# example.com/app\n./main.gp:5:12: pattern d: cannot embed file d/a'b.txt: invalid name a'b.txt\n"},
		{name: "folder with only hidden files", files: prog(embedHead+"//go:embed d\nvar a []byte\n\nfunc main() {}\n", "d/.x", "1", "d/_y", "2"),
			errs: "# example.com/app\n./main.gp:5:12: pattern d: cannot embed directory d: contains no embeddable files\n"},
		{name: "all: prefix on a file", files: prog(embedHead+"//go:embed all:d/.x\nvar a string\n\nfunc main() {}\n", "d/.x", "dot"),
			want: []string{`("dot")`}},
		{name: "folder of another module", files: prog(embedHead+"//go:embed d/x.txt\nvar a string\n\nfunc main() {}\n", "d/gocpp.mod", "module other\n", "d/x.txt", "1"),
			errs: "# example.com/app\n./main.gp:5:12: pattern d/x.txt: cannot embed directory d: in different module\n"},
	})
}

// TestGoEmbedBelowRoot: a package of the module below the root embeds the
// files next to its own source, not those of the root.
func TestGoEmbedBelowRoot(t *testing.T) {
	prog, _, _ := loadMacros(t, map[string]string{
		"main.gp":    "package main\n\nimport _ \"example.com/app/res\"\n\nfunc main() {}\n",
		"res/res.gp": embedHead + "//go:embed x.txt\nvar X string\n",
		"res/x.txt":  "below",
		"x.txt":      "not this one",
	})
	res := pkgOf(t, prog, "example.com/app/res")
	if got := printedFile(t, prog, res, "res.gp"); !strings.Contains(got, `("below")`) {
		t.Errorf("res.gp:\n%s", got)
	}
}

// TestEmbedWildcardHidden: a wildcard matches a name that starts with a dot,
// as path.Match does in Go's //go:embed d/*.
func TestEmbedWildcardHidden(t *testing.T) {
	prog, pkg, root := loadMacros(t, map[string]string{
		"main.gp": embedHead + "//go:embed d/*\nvar a string\n\nfunc main() {}\n",
		"d/.x":    "hidden",
	})
	if got := errorText(t, prog, root); got != "" {
		t.Errorf("errors: %s", got)
	}
	if got := printedFile(t, prog, pkg, "main.gp"); !strings.Contains(got, `("hidden")`) {
		t.Errorf("%s", got)
	}
}

// TestEmbedLimits: a file over the limit is an error that says so.
func TestEmbedLimits(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n",
		"main.gp":   embedHead + "//go:embed big.bin\nvar a string\n\nfunc main() {}\n",
	})
	f, err := os.Create(filepath.Join(root, "big.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(maxEmbedFile + 5); err != nil {
		t.Fatal(err)
	}
	f.Close()
	prog := loadIn(t, macroConfig(), root, ".")
	want := "# example.com/app\n./main.gp:5:12: cannot embed big.bin: " + strconv.FormatInt(maxEmbedFile+5, 10) + " bytes is more than the limit of 16 MiB for one file\n"
	if got := errorText(t, prog, root); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestEmbedTotalLimit: a package embeds at most maxEmbedTotal bytes.
func TestEmbedTotalLimit(t *testing.T) {
	old := maxEmbedTotal
	maxEmbedTotal = 10
	t.Cleanup(func() { maxEmbedTotal = old })
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n",
		"main.gp":   embedHead + "//go:embed a.txt\nvar a string\n\n//go:embed b.txt\nvar b string\n\nfunc main() {}\n",
		"a.txt":     "123456",
		"b.txt":     "123456",
	})
	prog := loadIn(t, macroConfig(), root, ".")
	want := "# example.com/app\n./main.gp:8:12: cannot embed b.txt: the package would embed more than 0 MiB\n"
	if got := errorText(t, prog, root); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestReadRegularFile: no link is followed, no pipe is waited for, and the limit holds for the open
// file (the read never takes more than limit+1 bytes).
func TestReadRegularFile(t *testing.T) {
	dir := t.TempDir()
	write := func(name, data string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("a", "12345")
	if got, err := ReadRegularFile(a, 5); err != nil || string(got) != "12345" {
		t.Errorf("at the limit: %q, %v", got, err)
	}
	var big *TooBigError
	if _, err := ReadRegularFile(a, 3); !errors.As(err, &big) || big.Size != 5 {
		t.Errorf("over the limit: %v", err)
	}
	if _, err := ReadRegularFile(filepath.Join(dir, "missing"), 5); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("missing: %v", err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(a, link); err != nil {
		t.Skip(err)
	}
	if _, err := ReadRegularFile(link, 100); err == nil {
		t.Errorf("a link was followed")
	}
	if _, err := ReadRegularFile("/dev/zero", 100); err == nil {
		t.Errorf("/dev/zero was read")
	}
	fifo := filepath.Join(dir, "fifo")
	if err := mkfifo(fifo); err != nil {
		t.Skip(err)
	}
	if _, err := ReadRegularFile(fifo, 100); err == nil {
		t.Errorf("a named pipe was read")
	}
}

// TestEmbedFolders: a folder with one file does for a string; a folder walk skips links and named pipes
// (an explicit link is an error); a directive above a group of variables is misplaced; a NUL byte in a
// name is quoted in the message.
func TestEmbedFolders(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n",
		"main.gp": embedHead + "//go:embed one\nvar a string\n\n//go:embed withlink\nvar b []byte\n\n" +
			"//go:embed\nvar (\n\tc string\n)\n\nfunc main() { println(#embed(\"a\\x00b\")) }\n",
		"one/f.txt":      "only",
		"withlink/f.txt": "real",
	})
	if err := mkfifo(filepath.Join(root, "withlink", "pipe")); err != nil {
		t.Skip(err)
	}
	if err := os.Symlink("f.txt", filepath.Join(root, "withlink", "link")); err != nil {
		t.Skip(err)
	}
	prog := loadIn(t, macroConfig(), root, ".")
	want := "# example.com/app\n./main.gp:11:1: usage: //go:embed pattern...\n./main.gp:16:30: #embed \"a\\x00b\": no matching files found\n"
	if got := errorText(t, prog, root); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	got := printedFile(t, prog, prog.Roots[0], "main.gp")
	for _, w := range []string{`("only")`, `[]byte(("real"))`} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %s in:\n%s", w, got)
		}
	}
}

// TestEmbedGroupDirective: //go:embed above `var (` is misplaced, as in Go.
func TestEmbedGroupDirective(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n",
		"main.gp":   embedHead + "//go:embed a.txt\nvar (\n\ta string\n)\n\nfunc main() {}\n",
		"a.txt":     "x",
	})
	prog := loadIn(t, macroConfig(), root, ".")
	want := "# example.com/app\n./main.gp:5:1: misplaced go:embed directive\n"
	if got := errorText(t, prog, root); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestEmbedSymlink: a symbolic link is not embedded, as in Go.
func TestEmbedSymlink(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n",
		"main.gp":   embedHead + "//go:embed link.txt\nvar a string\n\nfunc main() {}\n",
		"real.txt":  "real",
	})
	if err := os.Symlink("real.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Skip("no symbolic links:", err)
	}
	prog := loadIn(t, macroConfig(), root, ".")
	want := "# example.com/app\n./main.gp:5:12: pattern link.txt: cannot embed irregular file link.txt\n"
	if got := errorText(t, prog, root); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestBuiltinMacrosOff: without the syntax of Phase 10 the loader leaves
// everything alone: //go:embed is an ordinary comment.
func TestBuiltinMacrosOff(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n",
		"main.gp":   embedHead + "//go:embed nope.txt\nvar a string\n\nfunc main() { println(a) }\n",
	})
	cfg := macroConfig()
	cfg.Phase10Syntax = false
	prog := loadIn(t, cfg, root, ".")
	if got := errorText(t, prog, root); got != "" {
		t.Errorf("errors: %s", got)
	}
	if got := printedFile(t, prog, prog.Roots[0], "main.gp"); strings.Contains(got, "= (") {
		t.Errorf("expanded without the syntax:\n%s", got)
	}
}

// TestNoEmbedFiles: with Config.NoEmbedFiles (the language server) the loader checks the directive
// and the pattern but reads no file: a missing file is no error, and the value is empty.
func TestNoEmbedFiles(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n\ngocpp 0.1\n",
		"main.gp": embedHead + "//go:embed missing.txt\nvar a string\n\n//go:embed missing.bin\nvar b []byte\n\n" +
			"func main() { println(a, b, #embed(\"missing\"), #embed(\"\")) }\n",
	})
	cfg := macroConfig()
	cfg.NoEmbedFiles = true
	prog := loadIn(t, cfg, root, ".")
	want := "# example.com/app\n./main.gp:11:55: #embed : invalid file name\n"
	if got := errorText(t, prog, root); got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
	got := printedFile(t, prog, prog.Roots[0], "main.gp")
	for _, w := range []string{`var a string = ("")`, `var b []byte = []byte((""))`, `println(a, b, ("      `} {
		if !strings.Contains(got, w) {
			t.Errorf("missing %s in:\n%s", w, got)
		}
	}
}
