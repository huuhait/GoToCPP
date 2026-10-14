package server

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// sigFiles is a package with calls to look at.
var sigFiles = map[string]string{
	"gocpp.mod": "module example.com/s\n",
	"main.gp": `package main

import (
	"fmt"
	"strings"
	"./engine.hpp"
)

// Sum adds the numbers, after the base.
func Sum(base int, nums ...int) int {
	for _, n := range nums {
		base += n
	}
	return base
}

// Pair returns two things.
func Pair(a string, b bool) (n int, err error) { return 0, nil }

type T struct{}

// Run runs t.
func (t T) Run(name string, f func()) {}

func main() {
	fmt.Println(Sum(1, 2, 3, 4), strings.Repeat("x", 3))
	Pair("a", true)
	var t T
	t.Run("x", main)
	s := []int{}
	s = append(s, len(s))
	engine.start(1)
	f := Sum
	f(1)
	println(Sum())
}
`,
	"engine.hpp": "void start(int);\n",
}

// signature returns the signature help at the | of mark in main.gp: its
// label, its active parameter (-1 for none) and its documentation; "" when
// there is none.
func (c *dclient) signature(src, mark string) (label string, active int, doc string) {
	c.t.Helper()
	resp := c.call("textDocument/signatureHelp", c.posParams("main.gp", src, mark))
	if resp.Error != nil {
		c.t.Fatalf("signatureHelp %q: %v", mark, resp.Error)
	}
	if string(resp.Result) == "null" {
		return "", -1, ""
	}
	var h protocol.SignatureHelp
	if err := json.Unmarshal(resp.Result, &h); err != nil || len(h.Signatures) != 1 {
		c.t.Fatalf("signatureHelp %q: %s", mark, resp.Result)
	}
	s := h.Signatures[0]
	active = -1
	if h.ActiveParameter != nil {
		active = int(*h.ActiveParameter)
		if s.ActiveParameter == nil || *s.ActiveParameter != *h.ActiveParameter {
			c.t.Errorf("signatureHelp %q: the active parameters %v and %v differ", mark, h.ActiveParameter, s.ActiveParameter)
		}
	}
	for i, p := range s.Parameters {
		if !strings.Contains(s.Label, p.Label) {
			c.t.Errorf("signatureHelp %q: parameter %d %q is not in the label %q", mark, i, p.Label, s.Label)
		}
	}
	if s.Documentation != nil {
		m, ok := s.Documentation.Value.(protocol.MarkupContent)
		if !ok {
			c.t.Fatalf("signatureHelp %q: documentation %#v", mark, s.Documentation.Value)
		}
		doc = string(m.Kind) + ": " + m.Value
	}
	return s.Label, active, doc
}

func TestSignatureHelp(t *testing.T) {
	c := startNav(t, testConfig(), sigFiles, plainInit)
	src := sigFiles["main.gp"]
	c.open("main.gp", src)
	sum := "Sum(base int, nums ...int) int"
	sumDoc := "plaintext: Sum adds the numbers, after the base.\n"
	for _, tc := range []struct {
		mark, label string
		active      int
		doc         string
	}{
		// The active parameter, up to the variadic one, which stays.
		{"Sum(|1, 2", sum, 0, sumDoc},
		{"Sum(1|, 2", sum, 0, sumDoc},
		{"Sum(1,| 2", sum, 1, sumDoc},
		{"Sum(1, 2, 3, |4)", sum, 1, sumDoc},
		{"Sum(1, 2, 3, 4|)", sum, 1, sumDoc},
		// Nested calls: the innermost.
		{"4)|, strings", "Println(a ...any) (n int, err error)", 0, "plaintext: Println formats using the default formats for its operands and writes to standard output.\nSpaces are always added between operands and a newline is appended.\nIt returns the number of bytes written and any write error encountered.\n"},
		{"4), |strings", "Println(a ...any) (n int, err error)", 0, // variadic: stays at 0
			"plaintext: Println formats using the default formats for its operands and writes to standard output.\nSpaces are always added between operands and a newline is appended.\nIt returns the number of bytes written and any write error encountered.\n"},
		{"strings.Repeat(\"x\", |3)", "Repeat(s string, count int) string", 1, "plaintext: Repeat returns a new string consisting of count copies of the string s.\n\nIt panics if count is negative or if the result of (len(s) * count)\noverflows.\n"},
		// On the name of a function in a call: that function (gopls).
		{"fmt.Println(|Sum(", sum, -1, sumDoc},
		// Named results, a method, a function as an argument (its own
		// signature, as gopls shows it).
		{"Pair(\"a\", |true)", "Pair(a string, b bool) (n int, err error)", 1, "plaintext: Pair returns two things.\n"},
		{"t.Run(\"x\",| main", "Run(name string, f func())", 1, "plaintext: Run runs t.\n"},
		// In a string: only when the client asks (gopls).
		{"t.Run(\"|x\"", "", -1, ""},
		{"t.Run(\"x\", ma|in)", "main()", -1, ""},
		// Builtins.
		{"append(s,| len", "append(slice []Type, elems ...Type) []Type", 1, "plaintext: " + builtinDoc(t, "append") + "\n"},
		{"len(|s)", "len(v Type) int", 0, "plaintext: " + builtinDoc(t, "len") + "\n"},
		// A function value: no name, no doc.
		{"f(|1)", "f(base int, nums ...int) int", 0, ""},
		{"Sum(|))", sum, 0, sumDoc},
		// Nothing: a C/C++ function (opaque), outside a call.
		{"engine.start(|1)", "", -1, ""},
		{"var |t T", "", -1, ""},
	} {
		label, active, doc := c.signature(src, tc.mark)
		if label != tc.label || active != tc.active || doc != tc.doc {
			t.Errorf("at %q: %q, active %d, doc %q\nwant %q, active %d, doc %q", tc.mark, label, active, doc, tc.label, tc.active, tc.doc)
		}
	}
}

// TestSignatureHelpTyping: while a call is typed, the ) is missing, and the
// file doesn't parse.
func TestSignatureHelpTyping(t *testing.T) {
	c := startNav(t, testConfig(), sigFiles, markdownInit)
	src := strings.Replace(sigFiles["main.gp"], "Pair(\"a\", true)", "Pair(\"a\", ", 1)
	c.open("main.gp", src)
	label, active, doc := c.signature(src, "Pair(\"a\", |")
	if label != "Pair(a string, b bool) (n int, err error)" || active != 1 || doc != "markdown: Pair returns two things." {
		t.Errorf("got %q, active %d, doc %q", label, active, doc)
	}
}
