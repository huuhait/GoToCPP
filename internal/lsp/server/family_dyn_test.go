package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// The unexported method of an interface: the use that ties a type to the
// interface is in another package (main passes a base.Dog to base.Describe,
// which takes a base.Animal), and the rename has to change Dog.name too. It
// used to leave it, and the program did not compile.
var unexportedTieFiles = map[string]string{
	"gocpp.mod": "module example.com/m5\n",
	"base/base.gp": `package base

type Animal interface {
	Sound() string
	name() string
}

type Dog struct{ Name string }

func (d Dog) Sound() string { return "woof" }
func (d Dog) name() string  { return d.Name }

func Describe(a Animal) string { return a.name() + ":" + a.Sound() }
`,
	"main.gp": `package main

import (
	"fmt"

	"example.com/m5/base"
)

func main() {
	d := base.Dog{"rex"}
	fmt.Println(base.Describe(d))
}
`,
}

// TestRenameUnexportedMethodTieInImporter: the same edits from the interface's
// method, from the type's, and from the call through the interface; references
// agree.
func TestRenameUnexportedMethodTieInImporter(t *testing.T) {
	c := startRefs(t, unexportedTieFiles, plainInit)
	want := []string{
		"base/base.gp:5:2 name", "base/base.gp:11:14 name", "base/base.gp:13:43 name",
	}
	for _, mark := range []string{"\tname|() string\n}", "(d Dog) na|me()", "a.na|me()"} {
		sameList(t, "references from "+mark, c.references("base/base.gp", mark, true), want)
	}
	wantEdit := []string{
		"base/base.gp:5: title() string",
		"base/base.gp:11: func (d Dog) title() string  { return d.Name }",
		"base/base.gp:13: func Describe(a Animal) string { return a.title() + \":\" + a.Sound() }",
	}
	var cases []renameCase
	for _, mark := range []string{"\tname|() string\n}", "(d Dog) na|me()", "a.na|me()"} {
		cases = append(cases, renameCase{"base/base.gp", mark, "title", wantEdit, ""})
	}
	c.runRenames(t, cases)
}

// TestRenameUnexportedMethodCheckResult: when the search does not see the tie
// (skipImporterTies), the check of the result still looks at the importers of
// the edited folders, and refuses a rename that leaves them with an error.
func TestRenameUnexportedMethodCheckResult(t *testing.T) {
	c := startRefs(t, unexportedTieFiles, plainInit)
	skipImporterTies = true
	defer func() { skipImporterTies = false }()
	_, msg := c.renamed(t, "base/base.gp", "a.na|me()", "title")
	if !strings.Contains(msg, "it would make an error") || !strings.Contains(msg, "main.gp") {
		t.Errorf("rename without the tie: %q, want a refusal that names main.gp", msg)
	}
}

// promotedTieFiles: the type that the tie is about embeds a type of another
// package (promoted_cross), and the tie is in the main package only.
var promotedTieFiles = map[string]string{
	"gocpp.mod": "module example.com/prom\n",
	"base/base.gp": `package base

type Animal interface {
	Sound() string
	name() string
}

type Base struct{ N string }

func (b Base) name() string  { return "base-" + b.N }
func (b Base) Sound() string { return "..." }

func Describe(a Animal) string { return a.name() + ":" + a.Sound() }
`,
	"mid/mid.gp": `package mid

import "example.com/prom/base"

type Cat struct {
	base.Base
	Lives int
}

func (c Cat) Sound() string { return "meow" }
`,
	"main.gp": `package main

import (
	"fmt"

	"example.com/prom/base"
	"example.com/prom/mid"
)

func main() {
	c := mid.Cat{Base: base.Base{N: "tom"}, Lives: 9}
	fmt.Println(base.Describe(c))
}
`,
}

// TestRenameUnexportedMethodPromoted: Cat has base.Base's name() by
// promotion; main passes a Cat to Describe. All go.
func TestRenameUnexportedMethodPromoted(t *testing.T) {
	c := startRefs(t, promotedTieFiles, plainInit)
	got, rerr := c.rename("base/base.gp", "(b Base) na|me()", "title")
	if rerr != nil {
		t.Fatal(rerr)
	}
	text := got["base/base.gp"]
	if !strings.Contains(text, "func (b Base) title()") || !strings.Contains(text, "\ttitle() string") || !strings.Contains(text, "a.title()") {
		t.Errorf("base.gp after the rename:\n%s", text)
	}
}

// assertFiles: the methods reach an interface only through a type assertion.
var assertFiles = map[string]string{
	"gocpp.mod": "module example.com/asr\n",
	"m/m.gp": `package m

type Quacker interface{ Quack() string }

type Duck struct{}

func (Duck) Quack() string { return "quack" }

type Cat struct{}

func (Cat) Quack() string { return "meow" }

func Speak(v any) string {
	if q, ok := v.(Quacker); ok {
		return q.Quack()
	}
	return ""
}

func Use() string { return Speak(Cat{}) + Speak(Duck{}) }
`,
}

// TestRenameAssertionFollowed: a type that reaches the interface only through a
// type assertion (Cat, from an any) is renamed with it, or the assertion
// would fail at run time with no compile-time error; from either side.
func TestRenameAssertionFollowed(t *testing.T) {
	c := startRefs(t, assertFiles, plainInit)
	for _, mark := range []string{"Quacker interface{ Qu|ack", "(Duck) Qu|ack", "(Cat) Qu|ack", "q.Qu|ack()"} {
		got, rerr := c.rename("m/m.gp", mark, "Talk")
		if rerr != nil {
			t.Fatalf("%s: %v", mark, rerr)
		}
		text := got["m/m.gp"]
		if strings.Contains(text, "Quack()") || strings.Count(text, "Talk") != 4 {
			t.Errorf("rename from %q must change all four:\n%s", mark, text)
		}
		c.takeShown()
	}
}

// TestRenameAssertionTypeSwitchAndErrorsAs: a type switch case and the target
// of errors.As are followed too.
func TestRenameAssertionTypeSwitchAndErrorsAs(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/swa\n",
		"e/e.gp": `package e

type Temporary interface{ Temporary() bool }

type TimeoutErr struct{}

func (TimeoutErr) Error() string   { return "timeout" }
func (TimeoutErr) Temporary() bool { return true }

type FatalErr struct{}

func (FatalErr) Error() string   { return "fatal" }
func (FatalErr) Temporary() bool { return false }
`,
		"main.gp": `package main

import (
	"errors"
	"fmt"

	"example.com/swa/e"
)

func kind(v any) string {
	switch x := v.(type) {
	case e.Temporary:
		return fmt.Sprint(x.Temporary())
	}
	return "?"
}

func main() {
	var err error = e.TimeoutErr{}
	var tmp e.Temporary
	fmt.Println(errors.As(err, &tmp), kind(e.FatalErr{}))
}
`,
	}
	c := startRefs(t, files, plainInit)
	for _, mark := range []string{"(TimeoutErr) Temp|orary", "Temporary interface{ Temp|orary"} {
		got, rerr := c.rename("e/e.gp", mark, "Transient")
		if rerr != nil {
			t.Fatalf("%s: %v", mark, rerr)
		}
		if text := got["e/e.gp"]; strings.Contains(text, "Temporary()") || strings.Count(text, "Transient") != 3 {
			t.Errorf("rename from %q: e.gp\n%s", mark, text)
		}
		if text := got["main.gp"]; strings.Contains(text, ".Temporary()") {
			t.Errorf("rename from %q: main.gp\n%s", mark, text)
		}
		c.takeShown()
	}
	// errors.As alone: no switch, no assertion.
	files["main.gp"] = strings.Replace(files["main.gp"], "switch x := v.(type) {\n\tcase e.Temporary:\n\t\treturn fmt.Sprint(x.Temporary())\n\t}", "_ = v", 1)
	c = startRefs(t, files, plainInit)
	got, rerr := c.rename("e/e.gp", "(FatalErr) Temp|orary", "Transient")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if text := got["e/e.gp"]; strings.Count(text, "Transient") != 3 {
		t.Errorf("errors.As target not followed from FatalErr:\n%s", text)
	}
}

// TestRenameAssertionAcrossPackages: the interface is asserted in main, the
// types are in two other packages (same_names): both are renamed.
func TestRenameAssertionAcrossPackages(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/same\n",
		"x/x.gp": `package x

type T struct{}

func (T) Name() string { return "x" }

type I interface{ Name() string }
`,
		"y/y.gp": `package y

type T struct{}

func (T) Name() string { return "y" }
`,
		"main.gp": `package main

import (
	"fmt"

	"example.com/same/x"
	"example.com/same/y"
)

func main() {
	var b any = y.T{}
	_, ok := b.(x.I)
	fmt.Println(ok, x.T{}.Name())
}
`,
	}
	c := startRefs(t, files, plainInit)
	// From the interface: y.T is converted to any in main and implements it, so
	// the assertion b.(x.I) takes it out: it is renamed with the interface.
	// x.T is converted nowhere, no value of it reaches any assertion: it stays.
	got, rerr := c.rename("x/x.gp", "I interface{ Na|me", "Label")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if got["y/y.gp"] == "" || strings.Contains(got["y/y.gp"], "Name") || !strings.Contains(got["x/x.gp"], "func (T) Name()") {
		t.Errorf("from the interface: %v", got)
	}
	c.takeShown()
	// From y.T (which reaches the assertion): the interface and y.T, not x.T.
	got, rerr = c.rename("y/y.gp", "(T) Na|me()", "Label")
	if rerr != nil || !strings.Contains(got["x/x.gp"], "Label() string }") || !strings.Contains(got["x/x.gp"], "func (T) Name()") {
		t.Errorf("from y.T: %v %v", got, rerr)
	}
	c.takeShown()
	// From x.T: nothing reaches the assertion through it; only x.T.
	got, rerr = c.rename("x/x.gp", "(T) Na|me()", "Label")
	if rerr != nil || got["y/y.gp"] != "" || !strings.Contains(got["x/x.gp"], "interface{ Name()") {
		t.Errorf("from x.T: %v %v", got, rerr)
	}
}

// TestRenameNoAssertionNoWiden: without an assertion or a tie, a type that
// merely could be the interface stays out of the family.
func TestRenameNoAssertionNoWiden(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/nw\n",
		"m/m.gp": `package m

type Lener interface{ Len() int }

type Real struct{}

func (Real) Len() int { return 1 }

type Other struct{}

func (Other) Len() int { return 2 }

func Use(l Lener) int { return l.Len() }

func f() int { return Use(Real{}) + Other{}.Len() }
`,
	}
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("m/m.gp", "(Real) Le|n()", "Size")
	if rerr != nil {
		t.Fatal(rerr)
	}
	if text := got["m/m.gp"]; !strings.Contains(text, "func (Other) Len()") || strings.Contains(text, "func (Real) Len()") {
		t.Errorf("Other.Len must stay:\n%s", text)
	}
}

// TestRenameRefusedBySemantics: an init function and the Unwrap, Is and As
// methods of an error (errors.Is and the others call them by name, through
// interfaces written in the errors package) are not renamed: the program
// would behave differently and the build would not say so.
func TestRenameRefusedBySemantics(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/sem\n",
		"lib/lib.gp": `package lib

import "errors"

var Ran bool

func init() { Ran = true }

type E struct{ cause error }

func (E) Error() string        { return "e" }
func (e E) Unwrap() error      { return e.cause }
func (E) Is(target error) bool { return target == ErrX }
func (E) As(target any) bool   { return false }

var ErrX = errors.New("x")

type Box struct{ n int }

func (b Box) Unwrap() int { return b.n }
`,
	}
	c := startRefs(t, files, plainInit)
	for _, tc := range []struct{ mark, want string }{
		{"func in|it()", "the runtime calls it"},
		{"E) Unw|rap()", "errors.Is, errors.As and errors.Unwrap"},
		{"E) I|s(", "errors.Is"},
		{"E) A|s(", "errors.As"},
	} {
		_, msg := c.renamed(t, "lib/lib.gp", tc.mark, "Other")
		if !strings.Contains(msg, tc.want) {
			t.Errorf("rename at %q: %q, want a refusal with %q", tc.mark, msg, tc.want)
		}
	}
	// Another signature is no protocol method.
	if got, msg := c.renamed(t, "lib/lib.gp", "(b Box) Unw|rap()", "Get"); msg != "" || got["lib/lib.gp"] == "" {
		t.Errorf("Box.Unwrap() int: %q", msg)
	}
	// The wide (legacy) family leaves them as before.
	c.s.wideFamily = true
	if _, msg := c.renamed(t, "lib/lib.gp", "E) Unw|rap()", "Other"); msg != "" {
		t.Errorf("wide family: %q", msg)
	}
}

// TestPanicInSearchGoroutines: a panic in a goroutine that a request starts
// (the packages of a search, the files of a workspace symbol query) is the
// request's InternalError, as a panic in its own goroutine is: the server does
// not end, and the next request is answered.
func TestPanicInSearchGoroutines(t *testing.T) {
	c := startRefs(t, wsFiles, plainInit)
	faultInViews = func(string) { var m map[string]int; m["x"] = 1 }
	p := at(t, c.text("lib/lib.gp"), "func New|(w")
	resp := c.call("textDocument/references", fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"context":{"includeDeclaration":true}}`, c.uri("lib/lib.gp"), p.Line, p.Character))
	faultInViews = nil
	if resp.Error == nil || resp.Error.Code != int64(protocol.InternalError) || !strings.Contains(resp.Error.Message, "nil map") {
		t.Errorf("references with a panic in a package goroutine: %+v", resp.Error)
	}
	if got := c.references("lib/lib.gp", "func New|(w", true); len(got) < 3 {
		t.Errorf("references after the panic: %q", got)
	}

	// The index of the workspace symbols is warmed in the background when the
	// server starts, and a request makes the files that it lacks: with the
	// fault in place from the start, both panic (the first is logged), and the
	// request is answered with an InternalError.
	faultInSymbols = func(string) { var m map[string]int; m["x"] = 1 }
	c2 := startRefsRoot(t, wsFiles)
	resp = c2.call("workspace/symbol", `{"query":"rect"}`)
	faultInSymbols = nil
	if resp.Error == nil || resp.Error.Code != int64(protocol.InternalError) || !strings.Contains(resp.Error.Message, "nil map") {
		t.Errorf("workspace/symbol with a panic in a file goroutine: %+v", resp.Error)
	}
	if got := c2.workspaceSymbols("rect"); len(got) == 0 {
		t.Errorf("workspace symbols after the panic: none")
	}
}

// TestRenameAnonymousStructField: the fields of an anonymous struct type are those
// of every identical struct type: renaming x in Hidden's result renames it in
// ShowHidden's parameter too (they would stop being the same type), and a
// field of another package's struct of the same shape (another x) stays.
func TestRenameAnonymousStructField(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/anon\n",
		"a/a.gp": `package a

func Hidden() struct{ x int } { return struct{ x int }{7} }

func ShowHidden(h struct{ x int }) int { return h.x }

type Named struct{ x int }
`,
		"b/b.gp": `package b

func Hidden() struct{ x int } { return struct{ x int }{8} }
`,
		"main.gp": `package main

import (
	"fmt"

	"example.com/anon/a"
)

func main() { fmt.Println(a.ShowHidden(a.Hidden())) }
`,
	}
	c := startRefs(t, files, plainInit)
	got, rerr := c.rename("a/a.gp", "Hidden() struct{ |x int }", "y")
	if rerr != nil {
		t.Fatal(rerr)
	}
	text := got["a/a.gp"]
	if strings.Count(text, "struct{ y int }") != 3 || !strings.Contains(text, "h.y") || !strings.Contains(text, "type Named struct{ x int }") {
		t.Errorf("a.gp after the rename:\n%s", text)
	}
	if _, ok := got["b/b.gp"]; ok {
		t.Errorf("b has another x (a different package's): %v", got["b/b.gp"])
	}
}

// TestObjectNameInterfaceLiteral: a method of an interface literal is named
// by its package and that it is a literal, not "a.Len".
func TestObjectNameInterfaceLiteral(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/lit\n",
		"a/a.gp": `package a

type T struct{}

func (T) Len() int { return 1 }

func Use(x interface{ Len() int }) int { return x.Len() }

func f() int { return Use(T{}) }
`,
	}
	c := startRefs(t, files, plainInit)
	if _, msg := c.renamed(t, "a/a.gp", "(T) Le|n()", "Size"); msg != "" {
		t.Fatal(msg)
	}
	shown := c.takeShown()
	if len(shown) != 1 || !strings.Contains(shown[0].Message, "a.(interface literal).Len") || strings.Contains(shown[0].Message, " a.Len ") {
		t.Errorf("messages: %+v", shown)
	}
}
