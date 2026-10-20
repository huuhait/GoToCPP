package server

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// mustFiles is a program with the ! forms of Phase 10. The check lowers them to code with
// names of its own (must_e1, must_t1...) that have the places of the user's text; the features must
// take the user's names there, and no hint for the code that was written for them.
var mustFiles = map[string]string{
	"gocpp.mod": "module example.com/m\n",
	"m/m.gp": `package main

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

func load!(name string) (n int, err error) {
	defer errors.Wrapf(&err, "load %s", name)
	n = port(name)
	return n, nil
}

func use(name string) (int, error) {
	p := port!(name)
	q := load!(name)
	report(p, port!(name))
	return p + q, nil
}

func report(a, b int) { fmt.Println(a, b) }

func main() {
	fmt.Println(use("app"))
}
`,
}

func TestMustNamesInTheEditor(t *testing.T) {
	c := startRefs(t, mustFiles, plainInit)
	src := mustFiles["m/m.gp"]
	c.open("m/m.gp", src)

	// hover: the user's names, never the temporaries of the lowering
	for _, tc := range []struct{ mark, want, rng string }{
		{"p := po|rt!(name)", "func port(name string) (int, error)", "port"},
		{"q := lo|ad!(name)", "func load(name string) (n int, err error)", "load"},
		{"|p := port!", "var p int", "p"},
		{"n = po|rt(name)", "func port(name string) (int, error)", "port"},
	} {
		text, rng := c.hover("m/m.gp", src, tc.mark)
		// The declaration (the first line) is the user's; the code of a ! form that follows (step 11.13) is the compiler's.
		decl, _, _ := strings.Cut(text, "\n")
		if !strings.Contains(decl, tc.want) || strings.Contains(decl, "must_") || rng != tc.rng {
			t.Errorf("hover at %q: %q (range %q), want %q (range %q)", tc.mark, text, rng, tc.want, tc.rng)
		}
	}

	// definition: the declaration of the function
	got := c.definitions("textDocument/definition", "m/m.gp", src, "p := po|rt!(name)")
	sameList(t, "the definition of port!", got, []string{"m/m.gp:8:6-8:10 port"})
	got = c.definitions("textDocument/definition", "m/m.gp", src, "q := lo|ad!(name)")
	sameList(t, "the definition of load!", got, []string{"m/m.gp:15:6-15:10 load"})

	// rename: the function and its uses, not the variable p
	edited, err := c.rename("m/m.gp", "p := po|rt!(name)", "portal")
	if err != nil {
		t.Fatal(err)
	}
	lines := diffLinesAgainst(t, src, edited["m/m.gp"])
	sameList(t, "the changed lines of m.gp", lines, []string{
		"8: func portal(name string) (int, error) {",
		"17: n = portal(name)",
		"22: p := portal!(name)",
		"24: report(p, portal!(name))",
	})

	// hints: the type of p and q, nothing for the error variables of the lowering
	hints := c.hints("m/m.gp", nil)
	for _, h := range hints {
		if strings.Contains(h, " b: ") {
			// report(p, port!(name)): the argument is a temporary of the lowering, whose place is the end of the call
			t.Errorf("a hint for an argument that a ! form wrote: %q (all: %q)", h, hints)
		}
		if strings.Contains(h, "error") && !strings.Contains(h, "kind 2") {
			t.Errorf("a hint for the code of a ! form: %q (all: %q)", h, hints)
		}
	}
	if len(hints) == 0 {
		t.Errorf("no hints at all: the types of p and q are expected")
	}
}

// TestMustSemanticTokensAndHighlight: the tokens and the highlights of a file with ! forms are those
// of the user's names only: the names of the lowering have the places of the user's text, and must
// neither add a token nor make a read a write.
func TestMustSemanticTokensAndHighlight(t *testing.T) {
	c := startRefs(t, mustFiles, plainInit)
	src := mustFiles["m/m.gp"]
	c.open("m/m.gp", src)
	c.hover("m/m.gp", src, "p := po|rt!(name)") // the check has run
	got := c.semTokens("m/m.gp", nil)
	for _, want := range []string{
		"17:2 n parameter modification",
		"17:6 port function ",
		"17:11 name parameter ",
		"22:2 p variable declaration",
		"22:7 port function ",
		"22:13 name parameter ",
		"24:2 report function ",
		"24:9 p variable ",
		"24:12 port function ",
		"23:2 q variable declaration",
		"23:7 load function ",
	} {
		if !slices.Contains(got, want) {
			t.Errorf("no token %q in\n%s", want, strings.Join(got, "\n"))
		}
	}
	// one token for each place: sorted, and no two at the same place
	seen := map[string]bool{}
	for _, tok := range got {
		place := strings.Fields(tok)[0]
		if seen[place] {
			t.Errorf("two tokens at %s: %s", place, strings.Join(got, "\n"))
		}
		seen[place] = true
	}
	highlights := func(mark string) []string {
		resp := c.call("textDocument/documentHighlight", c.posParams("m/m.gp", src, mark))
		var hl []protocol.DocumentHighlight
		if resp.Error != nil || json.Unmarshal(resp.Result, &hl) != nil {
			t.Fatalf("highlight %q: %s %v", mark, resp.Result, resp.Error)
		}
		var out []string
		for _, h := range hl {
			out = append(out, fmt.Sprintf("%d:%d kind %d", h.Range.Start.Line+1, h.Range.Start.Character+1, h.Kind))
		}
		return out
	}
	// the declaration writes, the calls read (also the one in load, where the call drops its error)
	sameList(t, "highlights of port", highlights("p := po|rt!(name)"), []string{"8:6 kind 3", "17:6 kind 2", "22:7 kind 2", "24:12 kind 2"})
	sameList(t, "highlights of port in load", highlights("n = po|rt(name)"), []string{"8:6 kind 3", "17:6 kind 2", "22:7 kind 2", "24:12 kind 2"})
	sameList(t, "highlights of p", highlights("|p := port!(name)"), []string{"22:2 kind 3", "24:9 kind 2", "25:9 kind 2"})
	// report starts a lowered statement: it is read, not written by the names of the lowering
	sameList(t, "highlights of report", highlights("re|port(p, port!"), []string{"24:2 kind 2", "28:6 kind 3"})
}
