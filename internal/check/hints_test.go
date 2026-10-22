package check

import (
	"context"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
)

// hintErrors checks the files with a probe that runs no compiler and finds no namespace, so that
// the names of a header that has no name are ordinary undefined names, and returns what gocpp
// check prints.
func hintErrors(t *testing.T, files map[string]string) string {
	t.Helper()
	cfg := fakeProbeConfig(LastPhase)
	cfg.Probe = func(_ context.Context, _ build.CXX, _ string, _, _, candidates []string) (interop.Result, error) {
		kinds := make(map[string]interop.Kind)
		for _, c := range candidates {
			kinds[c] = interop.NotFound
		}
		return interop.Result{Kinds: kinds}, nil
	}
	return probeErrors(t, cfg, files)
}

// TestHeaderNameHints checks the hint for a header without a name that the code uses by its file
// name: the name is declared by the package (main.h next to func main) or is no Go name (3d.h).
// The hint comes after go/types' own message, which is what the user may have meant.
func TestHeaderNameHints(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"declared by the package", map[string]string{
			"main.gp": `package main

import "./main.h"

func main() {
	println(main.mn())
}
`, "main.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:6:15: main.mn undefined (type func() has no field or method mn); "./main.h" has no package name, because main is declared by the package: give it a name: import cmain "./main.h"
`},
		{"declared by another file of the package", map[string]string{
			"main.gp": `package main

import "./util.h"

func main() {
	println(util.mn())
}
`, "other.gp": "package main\n\nfunc util() {}\n", "util.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:6:15: util.mn undefined (type func() has no field or method mn); "./util.h" has no package name, because util is declared by the package: give it a name: import cutil "./util.h"
`},
		{"not a Go name", map[string]string{
			"main.gp": `package main

import "./3d.h"

func main() {
	println(d3.mn())
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:6:10: undefined: d3; "./3d.h" has no package name, because 3d is not a valid Go name: give it a name: import d3 "./3d.h"
`},
		{"the name of the hint is the one that is used", map[string]string{
			"main.gp": `package main

import "./3d.h"

func main() {
	println(vec.mn())
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:6:10: undefined: vec; "./3d.h" has no package name, because 3d is not a valid Go name: give it a name: import vec "./3d.h"
`},
		{"a typo of an imported package is no use of the header", map[string]string{
			"main.gp": `package main

import (
	"./3d.h"
	"sort"
)

func main() {
	sortt.Ints(nil)
	sort.Ints(nil)
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:9:2: undefined: sortt
`},
		{"a typo of a predeclared name is no use of the header", map[string]string{
			"main.gp": `package main

import "./3d.h"

func main() {
	prinln.x()
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:6:2: undefined: prinln
`},
		{"a typo of two letters in a long imported name is no use of the header", map[string]string{
			"main.gp": `package main

import (
	"./3d.h"
	"strings"
)

func main() {
	println(strnigs.Join(nil, ""), strings.ToUpper(""))
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:9:10: undefined: strnigs
`},
		{"a typo of a package-level function is no use of the header", map[string]string{
			"main.gp": `package main

import "./3d.h"

func vector() {}

func main() {
	vectr.x()
	vector()
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:8:2: undefined: vectr
`},
		{"a short name next to a name of the file is still a use of the header", map[string]string{
			"main.gp": `package main

import "./3d.h"

func d() {}

func main() {
	d3.mn()
	d()
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:8:2: undefined: d3; "./3d.h" has no package name, because 3d is not a valid Go name: give it a name: import d3 "./3d.h"
`},
		{"a name that is no typo of a name of the file is a use of the header", map[string]string{
			"main.gp": `package main

import (
	"./3d.h"
	"strings"
)

func main() {
	vec.x(strings.ToUpper(""))
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:9:2: undefined: vec; "./3d.h" has no package name, because 3d is not a valid Go name: give it a name: import vec "./3d.h"
`},
		{"a header with a name has no hint", map[string]string{
			"main.gp": `package main

import "./main.h"

func main() {
	println(nosuch.mn())
}
`, "main.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:6:10: undefined: nosuch
`},
		{"a file without headers has no hint", map[string]string{
			"main.gp": `package main

func main() {
	println(main.mn())
}
`},
			`# example.com/m
./main.gp:4:15: main.mn undefined (type func() has no field or method mn)
`},
		{"a local variable named like the package function", map[string]string{
			"main.gp": `package main

import "./main.h"

type T struct{ n int }

func f() {
	main := T{}
	println(main.m)
}

func main() {
	f()
}
`, "main.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:9:15: main.m undefined (type T has no field or method m)
`},
		{"a field of a variable is no use of the header name", map[string]string{
			"main.gp": `package main

import "./3d.h"

type T struct{ n int }

func main() {
	var d3 T
	println(d3.m)
}
`, "3d.h": "int mn(void);\n"},
			`# example.com/m
./main.gp:9:13: d3.m undefined (type T has no field or method m)
`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			if got := hintErrors(t, c.files); got != c.want {
				t.Errorf("errors:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}
