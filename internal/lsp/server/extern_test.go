package server

import (
	"strings"
	"testing"
)

// The editor with extern functions (//gocpp:extern, ADR-128): a function declared in .gp and
// defined in C or C++ is an ordinary function of the package for hover and go-to-definition (its
// directive is not part of its documentation), also from another package, and a use of it that
// is not a call is an error of the editor as of gocpp check.

var externFiles = map[string]string{
	"gocpp.mod": "module example.com/m\n",
	"lib/lib.gp": `package lib

// Add3 adds three numbers. It is defined in add3.c.
//
//gocpp:extern C
func Add3(a, b, c C.int) C.int
`,
	"main.gp": `package main

import "example.com/m/lib"

//gocpp:extern C
func twice(x C.int) C.int

func main() {
	_ = lib.Add3(1, 2, 3)
	_ = twice(4)
}
`,
}

func TestExternHoverAndDefinition(t *testing.T) {
	c := startNav(t, testConfig(), externFiles, plainInit)
	src := externFiles["main.gp"]
	c.open("main.gp", src)
	for _, tc := range []struct{ name, mark, want string }{
		{"other package", "lib.Ad|d3(1", "func lib.Add3(a C.int, b C.int, c C.int) C.int\nAdd3 adds three numbers. It is defined in add3.c."},
		{"same package", "_ = tw|ice(4)", "func twice(x C.int) C.int"},
		{"declaration", "func tw|ice(x", "func twice(x C.int) C.int"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text, _ := c.hover("main.gp", src, tc.mark)
			if !strings.Contains(text, tc.want) {
				t.Errorf("hover %q:\n%s\nwant %q", tc.mark, text, tc.want)
			}
			if strings.Contains(text, "gocpp:extern") {
				t.Errorf("the directive is in the hover:\n%s", text)
			}
		})
	}
	t.Run("definition", func(t *testing.T) {
		got := c.definitions("textDocument/definition", "main.gp", src, "lib.Ad|d3(1")
		if len(got) != 1 || !strings.HasPrefix(got[0], "lib/lib.gp:6:6-6:10 Add3") {
			t.Errorf("definition of lib.Add3: %v", got)
		}
		got = c.definitions("textDocument/definition", "main.gp", src, "_ = tw|ice(4)")
		if len(got) != 1 || !strings.HasPrefix(got[0], "main.gp:6:6-6:11 twice") {
			t.Errorf("definition of twice: %v", got)
		}
	})
}

func TestExternDiagnostics(t *testing.T) {
	files := map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": `package main

//gocpp:extern C
func twice(x C.int) C.int

//gocpp:extern C
func bad(s string)

func main() {
	f := twice
	_ = f
	bad("x")
}
`,
	}
	c := startDiag(t, testConfig(), files, nil)
	c.open("main.gp", files["main.gp"])
	want(t, c.next("main.gp"), 1,
		"6:11-6:17 error: extern function bad: parameter type string is a Go type that C++ does not have (use a C type such as *C.char, or a number type of Go)\n"+
			"9:6-9:11 error: cannot use extern function twice as a value: it is a C or C++ function, only a call is possible (wrap it in a Go function to pass it on)\n")
}
