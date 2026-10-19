package server

import (
	"fmt"
	"strings"
	"testing"
)

// parenBlankSrc has the shapes of tests/run/blank_label_paren.gp (step
// 15.10): go/types records no type for the blank identifier, nor for the
// parentheses around it.
const parenBlankSrc = `package main

import "fmt"

func two() (int, int) { return 1, 2 }

func labels() {
_:
	fmt.Println("label")
_:
}

func main() {
	labels()
	s := []int{1, 2, 3}
	x := 1
	(_) = x
	((_)) = 3
	(x) = 5
	(_), (x) = two()
	for (_) = range s {
	}
	switch (_) = 7; {
	}
	fmt.Println(x)
}
`

// TestParenBlankRequests: hover, definition, type definition, references,
// highlights, rename and semantic tokens at a parenthesized blank
// identifier answer, and the definition of a parenthesized name still
// finds the name.
func TestParenBlankRequests(t *testing.T) {
	files := map[string]string{
		"gocpp.mod":   "module example.com/app\n",
		"app/main.gp": parenBlankSrc,
	}
	c := startNav(t, testConfig(), files, plainInit)
	c.open("app/main.gp", parenBlankSrc)
	for _, mark := range []string{
		"(|_) = x", "(_|) = x", "((|_)) = 3", "(|(_)) = 3", "(_), (|x) = two()",
		"(|_), (x) = two()", "for (|_) = range", "switch (|_) = 7", "|_:\n\tfmt", "_|:\n\tfmt",
	} {
		for _, method := range []string{"textDocument/hover", "textDocument/definition", "textDocument/typeDefinition", "textDocument/documentHighlight"} {
			if resp := c.call(method, c.posParams("app/main.gp", parenBlankSrc, mark)); resp.Error != nil {
				t.Errorf("%s at %q: %v", method, mark, resp.Error)
			}
		}
		p := at(t, parenBlankSrc, mark)
		params := fmt.Sprintf(`{"textDocument":{"uri":%q},"position":{"line":%d,"character":%d},"context":{"includeDeclaration":true}}`, c.uri("app/main.gp"), p.Line, p.Character)
		if resp := c.call("textDocument/references", params); resp.Error != nil {
			t.Errorf("references at %q: %v", mark, resp.Error)
		}
	}
	// A parenthesized name is still the name.
	got := c.definitions("textDocument/definition", "app/main.gp", parenBlankSrc, "(_), (|x) = two()")
	if len(got) != 1 || !strings.HasSuffix(got[0], " x") {
		t.Errorf("definition of (x): %q", got)
	}
	for _, method := range []string{"textDocument/semanticTokens/full", "textDocument/documentSymbol"} {
		if resp := c.call(method, fmt.Sprintf(`{"textDocument":{"uri":%q}}`, c.uri("app/main.gp"))); resp.Error != nil {
			t.Errorf("%s: %v", method, resp.Error)
		}
	}
	// A hover over the name next to the blank still has its text.
	if text, _ := c.hover("app/main.gp", parenBlankSrc, "(|x) = 5"); !strings.Contains(text, "x int") {
		t.Errorf("hover over (x): %q", text)
	}
}
