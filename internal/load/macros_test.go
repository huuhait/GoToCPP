package load

import (
	"fmt"
	"strings"
	"testing"
)

// The tests of the user macros (step 10.9): each loads a small module with the syntax of Phase 10 on,
// prints the main package's files as they are after the rewrite, and compares text, or errors.

const macroHead = "package main\n\nimport \"fmt\"\n\n"

// macroDecls are macros that many cases use.
const macroDecls = `
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

macro Note(s string) {
	fmt.Println(s, #line())
}

macro Wrap() {
	#body()
}

macro Twice(n int) {
	#Note("n")
}
`

// ln returns the line of the first place in src that has marker.
func ln(src, marker string) int {
	i := strings.Index(src, marker)
	if i < 0 {
		panic("no " + marker)
	}
	return strings.Count(src[:i], "\n") + 1
}

// with returns the source of a main package that has the macros above and body.
func with(body string) string { return macroHead + macroDecls + body }

func TestMacroDeclRewrite(t *testing.T) {
	runMacroCases(t, []macroCase{
		{name: "lazy parameter", files: map[string]string{"main.gp": macroHead + macroDecls + "func main() {}\n"},
			want: []string{
				// hidden parameters first, the lazy type is a type parameter constrained to func() T
				"func Assert[gp_LN_msg ~func() string](gp_file string, gp_line int, gp_func string, gp_text_cond string, cond bool, msg gp_LN_msg) {",
				// #file(), #line() and #text(cond) read the hidden parameters
				`panic(fmt.Sprintf("%s:%d: %s: %s", (gp_file), (gp_line), (gp_text_cond), msg()))`,
			}},
		{name: "generic keeps its type parameters", files: map[string]string{"main.gp": macroHead + macroDecls + "func main() {}\n"},
			want: []string{
				"func Dbg[T any](gp_file string, gp_line int, gp_func string, gp_text_v string, v T) T {",
				`fmt.Println((gp_text_v), v)`,
			}},
		{name: "no #text of a parameter: no text parameter", files: map[string]string{"main.gp": macroHead + macroDecls + "func main() {}\n"},
			want: []string{"func Trace(gp_file string, gp_line int, gp_func string) func() {", `fmt.Println("enter", (gp_func))`}},
		{name: "wrap macro gets the code as a hidden parameter", files: map[string]string{"main.gp": macroHead + macroDecls + "func main() {}\n"},
			want: []string{"func Wrap[gp_BN ~func()](gp_file string, gp_line int, gp_func string, gp_body gp_BN) {", "gp_body()"}},
		{name: "a macro used in a macro forwards the place", files: map[string]string{"main.gp": macroHead + macroDecls + "func main() {}\n"},
			want: []string{`Note(gp_file, gp_line, gp_func, "n"`}},
	})
}

func TestMacroUseRewrite(t *testing.T) {
	var cases []macroCase
	add := func(name, body string, want func(src string) []string) {
		src := with(body)
		cases = append(cases, macroCase{name: name, files: map[string]string{"main.gp": src}, want: want(src)})
	}
	add("lazy argument is a func literal", `
func main() {
	n := 1
	#Assert(n > 0, fmt.Sprint("n=", n))
}
`, func(src string) []string {
		return []string{fmt.Sprintf(`Assert("main.gp", %d, "main.main", "n > 0", n > 0, func() string {`, ln(src, "#Assert(n")),
			`return fmt.Sprint("n=", n)`}
	})
	add("type arguments stay", `
func main() { _ = #Dbg[int8](1) }
`, func(src string) []string {
		return []string{fmt.Sprintf(`Dbg[int8]("main.gp", %d, "main.main", "1", 1)`, ln(src, "#Dbg[int8]"))}
	})
	add("text of an argument as written", `
func main() {
	x := 1
	_ = #Dbg(x +
		1)
}
`, func(src string) []string { return []string{`"x + 1"`} })
	add("a closure names the function as the runtime does", `
func main() {
	f := func() { #Note("a") }
	f()
}
`, func(src string) []string {
		return []string{fmt.Sprintf(`Note("main.gp", %d, "main.main.func1", "a")`, ln(src, "#Note(\"a\")"))}
	})
	add("start style: first statement, func() deferred when not nil", `
#Trace()
#Note("x")
func work(n int) int { return n }

func main() {}
`, func(src string) []string {
		return []string{
			fmt.Sprintf(`if gp_defer1 := Trace("main.gp", %d, "main.work"); gp_defer1 != nil {`, ln(src, "#Trace()\n#Note")),
			`defer gp_defer1()`,
			fmt.Sprintf(`Note("main.gp", %d, "main.work", "x")`, ln(src, "#Note(\"x\")")),
		}
	})
	add("wrap style: results named, returns set them", `
#Wrap()
func two(a int) (int, string) {
	if a > 0 {
		return a, "pos"
	}
	return 0, "neg"
}

func main() {}
`, func(src string) []string {
		return []string{
			"func two(a int) (gp_r0 int, gp_r1 string) {",
			fmt.Sprintf(`Wrap("main.gp", %d, "main.two", func() {`, ln(src, "#Wrap()\nfunc two")),
			// the code is untouched, inside a literal with the old results
			"gp_r0, gp_r1 = func() (int, string) {",
			`return a, "pos"`,
			`return 0, "neg"`,
		}
	})
	add("wrap style: named results stay, a blank one is named, a bare return stays", `
#Wrap()
func named(a int) (_ int, err error) {
	if a > 0 {
		return
	}
	return a, nil
}

func main() {}
`, func(src string) []string {
		return []string{"func named(a int) (gp_r0 int, gp_r1 error) {", "gp_r0, gp_r1 = func() (_ int, err error) {", "return a, nil"}
	})
	add("wrap style: the return of a func literal is its own", `
#Wrap()
func lit() int {
	f := func() int { return 7 }
	return f()
}

func main() {}
`, func(src string) []string { return []string{"f := func() int { return 7 }", "return f()"} })
	add("a result that an inner variable hides: the code is not rewritten", `
#Wrap()
func shadow(a int) (n int, err error) {
	if err := fmt.Errorf("x"); err != nil {
		return 7, err
	}
	return a, nil
}

func main() {}
`, func(src string) []string {
		return []string{"gp_r0, gp_r1 = func() (n int, err error) {", "return 7, err"}
	})
	add("stacked: the first line is the outermost", `
#Wrap()
#Trace()
func stack() {}

func main() {}
`, func(src string) []string {
		// Wrap is outermost: it holds the code, which starts with the Trace statement
		return []string{
			fmt.Sprintf(`Wrap("main.gp", %d, "main.stack", func() {`, ln(src, "#Wrap()\n#Trace()")),
			fmt.Sprintf(`if gp_defer1 := Trace("main.gp", %d, "main.stack")`, ln(src, "#Trace()\nfunc stack")),
		}
	})
	runMacroCases(t, cases)
}

func TestMacroAcrossPackages(t *testing.T) {
	util := `package util

import "fmt"

type Level int

type Result struct{ N int }

macro Show(r lazy Result, l lazy Level, e lazy fmt.Stringer, s lazy []Result, m lazy map[string]*Result) {}

macro Plain(n int) int { return n }

macro hidden() {}
`
	runMacroCases(t, []macroCase{
		{name: "lazy types are written with the names of the use", files: map[string]string{
			"util/util.gp": util,
			"main.gp": `package main

import (
	"fmt"

	"example.com/app/util"
)

func main() {
	#util.Show(util.Result{}, 1, nil, nil, nil)
	_ = fmt.Sprint()
}
`},
			want: []string{
				"util.Show(", "func() util.Result {", "func() util.Level {", "func() fmt.Stringer {", "func() []util.Result {", "func() map[string]*util.Result {",
			}},
		{name: "another name for the package and a dot import", files: map[string]string{
			"util/util.gp": util,
			"main.gp": `package main

import (
	"fmt"

	. "example.com/app/util"
	u "example.com/app/util"
)

func main() {
	#u.Plain(1)
	#Plain(2)
	_ = fmt.Sprint()
}
`},
			want: []string{`u.Plain("main.gp", 11, "main.main", 1)`, `Plain("main.gp", 12, "main.main", 2)`}},
		{name: "the summary of a package keeps its macros", files: map[string]string{"util/util.gp": util, "main.gp": "package main\n\nimport \"example.com/app/util\"\n\nfunc main() { _ = util.Level(1) }\n"},
			want: []string{"func main()"}},
	})
	prog, _, _ := loadMacros(t, map[string]string{"util/util.gp": util, "main.gp": "package main\n\nimport \"example.com/app/util\"\n\nfunc main() { _ = util.Level(1) }\n"})
	dep := pkgOf(t, prog, "example.com/app/util")
	sum := dep.Summary()
	if len(sum.Macros) != 3 || sum.Macros["Show"] == nil || !sum.Macros["Show"].Params[0].Lazy || sum.Macros["Show"].Params[0].Result != "\x01Result" {
		t.Errorf("summary macros = %+v", sum.Macros)
	}
	if sum.Macros["Plain"].decl != nil {
		t.Errorf("a summary keeps a declaration")
	}
	if s := NewSummary(sum, nil); len(s.Macros) != 3 {
		t.Errorf("NewSummary lost the macros: %v", s.Macros)
	}
}

func TestMacroErrors(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"undefined", macroHead + "func main() { #Nope(1) }\n", "undefined macro #Nope"},
		{"too few", macroHead + macroDecls + "func main() { #Assert(true) }\n", "#Assert: not enough arguments (have 1, want 2)"},
		{"too many", macroHead + macroDecls + "func main() { #Note(\"a\", \"b\") }\n", "#Note: too many arguments (have 2, want 1)"},
		{"wrap in code", macroHead + macroDecls + "func main() { #Wrap() }\n", "#Wrap uses #body(): it is a wrap macro"},
		{"func statement", macroHead + macroDecls + "func main() { #Trace() }\n", "#Trace returns a func(): as a statement"},
		{"defer", macroHead + macroDecls + "func main() { defer #Note(\"x\") }\n", "#Note can't be the call of a defer or go statement"},
		{"redeclared", macroHead + "macro A() {}\nmacro A() {}\n", "macro A redeclared in this package"},
		{"builtin name", macroHead + "macro line() {}\n", "line is a built-in macro"},
		{"gp_ name", macroHead + "macro A(gp_x int) {}\n", "the name gp_x is reserved"},
		{"lazy type parameter", macroHead + "macro A[T any](v lazy T) {}\n", "the type T is a type parameter of the macro"},
		{"must in lazy", macroHead + macroDecls + "func f() (int, error) { return 0, nil }\nfunc main() { #Assert(true, fmt.Sprint(f!())) }\n", "f!: ! can't be used in a lazy macro argument"},
		{"unexported", macroHead + "func main() {}\n", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			prog, _, root := loadMacros(t, map[string]string{"main.gp": c.src})
			got := errorText(t, prog, root)
			if c.want == "" && got != "" || !strings.Contains(got, c.want) {
				t.Errorf("errors:\n%s\nwant %q", got, c.want)
			}
		})
	}
}

// TestMacroNoEscape: a closure parameter that the macro only calls where it stands is gp_LN_x / gp_BN (the lambda
// for it captures by reference); any other use makes it gp_L_x / gp_B.
func TestMacroNoEscape(t *testing.T) {
	src := macroHead + `
macro Calls(f lazy int) int { return f() + f() }

macro Keeps(f lazy int) func() int { return f }

macro InLiteral(f lazy int) func() int { return func() int { return f() } }

macro InGo(f lazy int) { go f() }

macro InDefer(f lazy int) { defer fmt.Println(f()) }

macro PassOn(f lazy int) int { return #Calls(f()) }

macro Text(f lazy int) string { return #text(f) }

macro Run() { #body() }

macro InClosure() { func() { #body() }() }

macro InGoBody() { go func() { #body() }() }

func main() {}
`
	_, p, _ := loadMacros(t, map[string]string{"main.gp": src})
	want := map[string]struct {
		lazy, body bool
	}{
		"Calls": {true, false}, "Keeps": {false, false}, "InLiteral": {false, false}, "InGo": {false, false},
		"InDefer": {false, false}, "PassOn": {false, false}, "Text": {true, false},
		"Run": {false, true}, "InClosure": {false, false}, "InGoBody": {false, false},
	}
	for name, w := range want {
		sig := p.Macros[name]
		if sig == nil {
			t.Fatalf("no macro %s", name)
		}
		if len(sig.Params) > 0 && sig.Params[0].NoEscape != w.lazy {
			t.Errorf("%s: lazy NoEscape = %v, want %v", name, sig.Params[0].NoEscape, w.lazy)
		}
		if sig.Wrap && sig.BodyNoEscape != w.body {
			t.Errorf("%s: BodyNoEscape = %v, want %v", name, sig.BodyNoEscape, w.body)
		}
	}
}
