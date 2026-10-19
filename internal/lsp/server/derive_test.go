package server

import (
	"strings"
	"testing"
)

const deriveSrc = `package main

import "fmt"

#Derive(String)
type Level int

const (
	Debug Level = iota
	Info
)

#Derive(Equal, Clone)
type Event struct {
	ID   int
	Tags []string
	Lvl  Level
}

func main() {
	e := Event{ID: 1}
	f := e.Clone()
	fmt.Println(e.Equal(f), Debug.String())
}
`

// TestDeriveNavigation: the methods that #Derive writes have the place of their name in the #Derive line,
// so hover shows their signature and go to definition lands on the line (step 10.10); the code that
// the generators write is not seen by the features, and renaming the type changes the text only.
func TestDeriveNavigation(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": deriveSrc}
	c := startRefs(t, files, plainInit)
	c.open("main.gp", deriveSrc)

	for _, tc := range []hoverCase{
		{"e.Equ|al(f)", "func (e Event) Equal(other Event) bool", "Equal"},
		{"e.Cl|one()", "func (e Event) Clone() Event", "Clone"},
		{"Debug.Str|ing()", "func (l Level) String() string", "String"},
	} {
		text, rng := c.hover("main.gp", deriveSrc, tc.mark)
		if text != tc.text || rng != tc.rng {
			t.Errorf("hover at %q: %q (range %q), want %q (range %q)", tc.mark, text, rng, tc.text, tc.rng)
		}
	}
	sameList(t, "definition of Equal", c.definitions("textDocument/definition", "main.gp", deriveSrc, "e.Equ|al(f)"),
		[]string{"main.gp:13:9-13:14 Equal"})
	sameList(t, "definition of Level.String", c.definitions("textDocument/definition", "main.gp", deriveSrc, "Debug.Str|ing()"),
		[]string{"main.gp:5:9-5:15 String"})

	for _, h := range c.hints("main.gp", nil) {
		if strings.Contains(h, "gp_") {
			t.Errorf("a hint for the generated code: %q", h)
		}
	}
	for _, tok := range c.semTokens("main.gp", nil) {
		if strings.Contains(tok, " gp_") {
			t.Errorf("a token for a hidden name: %q", tok)
		}
	}

	// references to the type do not include the generated receivers; rename keeps the text right
	refs := c.references("main.gp", "type Ev|ent struct", true)
	for _, r := range refs {
		if strings.HasPrefix(r, "main.gp:13:") || strings.HasPrefix(r, "main.gp:5:") {
			t.Errorf("a reference at the #Derive line: %v", refs)
		}
	}
	edited, err := c.rename("main.gp", "type Ev|ent struct", "Ev")
	if err != nil {
		t.Fatal(err)
	}
	sameList(t, "the changed lines", diffLinesAgainst(t, deriveSrc, edited["main.gp"]),
		[]string{"14: type Ev struct {", "21: e := Ev{ID: 1}"})
}

// TestDiagnosticsDerive: an error of a generator is reported at the name of the generator in the
// #Derive line, and a clean document has no diagnostics.
func TestDiagnosticsDerive(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{"gocpp.mod": "module example.com/m\n"}
	c := startDiag(t, testConfig(), files, nil)
	good := "package main\n\n#Derive(Equal, Clone)\ntype T struct{ A []int }\n\nfunc main() { _ = T{}.Equal(T{}) }\n"
	c.open("good/good.gp", good)
	want(t, c.next("good/good.gp"), 1, "")
	bad := "package main\n\n#Derive(Equal)\ntype B struct{ F func() }\n\nfunc main() { _ = B{}.Equal(B{}) }\n"
	c.open("bad/bad.gp", bad)
	want(t, c.next("bad/bad.gp"), 1,
		"2:8-2:13 error: #Derive(Equal): field F has a func type, which cannot be compared\n")
}

// TestDeriveCrossPackage: go to definition on a derived method of another package lands on the
// #Derive line there, as it does for a method written by hand.
func TestDeriveCrossPackage(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"model/model.gp": "package model\n\n#Derive(Equal, Clone)\ntype Thing struct{ A []int }\n\n" +
			"type Hand struct{ A int }\n\nfunc (h Hand) Same(o Hand) bool { return h == o }\n",
		"main.gp": "package main\n\nimport \"example.com/m/model\"\n\nfunc main() {\n\tt := model.Thing{}\n\tu := model.Thing{}\n" +
			"\t_ = t.Equal(u)\n\t_ = model.Hand{}.Same(model.Hand{})\n}\n",
	}
	c := startRefs(t, files, plainInit)
	c.open("main.gp", files["main.gp"])
	sameList(t, "definition of Same", c.definitions("textDocument/definition", "main.gp", files["main.gp"], "Hand{}.Sa|me("),
		[]string{"model/model.gp:8:15-8:19 Same"})
	sameList(t, "definition of Equal", c.definitions("textDocument/definition", "main.gp", files["main.gp"], "t.Equ|al(u)"),
		[]string{"model/model.gp:3:9-3:14 Equal"})
	text, rng := c.hover("main.gp", files["main.gp"], "t.Equ|al(u)")
	if text != "func (t model.Thing) Equal(other model.Thing) bool" || rng != "Equal" {
		t.Errorf("hover: %q (range %q)", text, rng)
	}
}

// TestDeriveRename: the name of a derived method is the generator's: neither prepareRename nor rename
// accepts it (rename of the type works, TestDeriveNavigation).
func TestDeriveRename(t *testing.T) {
	checkGoroutines(t)
	files := map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": deriveSrc}
	c := startRefs(t, files, plainInit)
	c.open("main.gp", deriveSrc)
	for _, mark := range []string{"e.Equ|al(f)", "#Derive(Equ|al, Clone)"} {
		resp := c.call("textDocument/prepareRename", c.posParams("main.gp", deriveSrc, mark))
		if resp.Error == nil || (mark[0] == 'e' && !strings.Contains(resp.Error.Message, "#Derive writes it")) {
			t.Errorf("prepareRename at %q: %+v", mark, resp)
		}
		if _, err := c.rename("main.gp", mark, "Same"); err == nil {
			t.Errorf("rename at %q was accepted", mark)
		}
	}
}
