package main

import (
	"regexp"
	"strings"
	"testing"
)

// `gocpp check` and `gocpp build` give the same errors for a program that a C/C++ name or header
// breaks: the build runs the check first, so what the check lets through, the lowering or the C++
// compiler reports in the build, and the user sees an error that `gocpp check` and the editor
// did not show. These programs are the ones where that happened: each is
// run through both commands, in both interop modes, and the errors must be the same lines, at the
// same positions, with the same text.

// agreeError matches the lines of an error, with a line and maybe a column.
var agreeError = regexp.MustCompile(`^\./\S+\.gp:\d+(:\d+)?: `)

// agreeLines returns the lines of out that are errors with a position, without the warnings.
func agreeLines(out string) []string {
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if agreeError.MatchString(l) && !strings.Contains(l, ": warning: ") {
			lines = append(lines, l)
		}
	}
	return lines
}

func TestCheckAndBuildAgree(t *testing.T) {
	for _, c := range []struct {
		name  string
		files map[string]string
		want  string // the text that the errors must have, after the position
	}{
		{"const from a C variable", map[string]string{"main.gp": `package main

func main() {
	var x C.int = 2
	const d = x + 1
	_ = d
}
`}, "x + 1 (C/C++ value) is not constant"},
		{"const from a package variable of a C type", map[string]string{"main.gp": `package main

var g C.int = 4

const k = g * 2

func main() {
	println(k)
}
`}, "g * 2 (C/C++ value) is not constant"},
		{"header named like a keyword", map[string]string{"main.gp": `package main

import "<sys/select.h>"

var set select.fd_set

func main() {
	_ = set
}
`}, `<sys/select.h> has no package name, because select is a Go keyword: give it a name: import cselect "<sys/select.h>"`},
		{"header named like the package function", map[string]string{"main.gp": `package main

import "./main.h"

func main() {
	println(main.mn())
}
`, "main.h": "int mn(void);\n"}, `main.mn undefined (type func() has no field or method mn); "./main.h" has no package name, because main is declared by the package: give it a name: import cmain "./main.h"`},
		{"header named like no Go name", map[string]string{"main.gp": `package main

import "./3d.h"

func main() {
	println(d3.mn())
}
`, "3d.h": "int mn(void);\n"}, `undefined: d3; "./3d.h" has no package name, because 3d is not a valid Go name: give it a name: import d3 "./3d.h"`},
		{"header that ends in a declaration", map[string]string{"main.gp": `package main

import "./cut.h"

func main() {
	println(cut.f())
}
`, "cut.h": "#pragma once\nint f(void)"}, `header "./cut.h" ends inside a declaration or a block that it doesn't close`},
		{"header that ends in a struct", map[string]string{"main.gp": `package main

import "./nosemi.h"

func main() {
	var s nosemi.NoSemi
	println(s.x)
}
`, "nosemi.h": "#pragma once\nstruct NoSemi { int x; }\n"}, `header "./nosemi.h" doesn't compile: nosemi.h:2:25: expected ';' after struct`},
		{"headers of two files that conflict", map[string]string{
			"a.gp":    "package main\n\nimport \"./a.h\"\n\nfunc A() { println(a.f(1)) }\n",
			"main.gp": "package main\n\nimport \"./b.h\"\n\nfunc main() {\n\tA()\n\tprintln(b.f(2))\n}\n",
			"a.h":     "#pragma once\n#include <string>\nint f(int x);\n",
			"b.h":     "int f(int x);\n",
		}, `header "./b.h" doesn't compile with the headers of the other files of the package: b.h:1:5: `},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			forEachCXX(t, func(t *testing.T, cxx testCXX) {
				for _, mode := range []string{"opaque", "typed"} {
					t.Run(mode, func(t *testing.T) {
						typedEnv(t)
						if mode == "typed" {
							needClang(t)
						}
						dir := inDir(t, c.files)
						checked := gocppRun("", true, "check", "-interop="+mode, ".")
						built := gocppRun("", true, "build", "-interop="+mode, "-o", dir+"/out", ".")
						if checked.code != 1 || built.code != 1 {
							t.Fatalf("exit codes: check %d, build %d, want 1 and 1\ncheck:\n%s\nbuild:\n%s",
								checked.code, built.code, checked.stdout, built.stdout)
						}
						got, want := agreeLines(checked.stdout), agreeLines(built.stdout)
						if strings.Join(got, "\n") != strings.Join(want, "\n") {
							t.Errorf("check:\n%s\nbuild:\n%s\nwant the same errors", checked.stdout, built.stdout)
						}
						if len(got) == 0 || !strings.Contains(got[0], c.want) {
							t.Errorf("check:\n%s\nwant an error with %q", checked.stdout, c.want)
						}
					})
				}
			})
		})
	}
}

// TestKeywordHeaderUses checks the CLI output for a header named like a Go keyword: one error line
// for each use of the name, the hint, with no follow-up errors of the parser (gocpp check, the
// build and the editor print the same lines), at the keyword (also when it starts a statement).
func TestKeywordHeaderUses(t *testing.T) {
	const hint = `<sys/select.h> has no package name, because select is a Go keyword: give it a name: import cselect "<sys/select.h>"`
	const head = "package main\n\nimport \"<sys/select.h>\"\n\nfunc main() {\n"
	for _, c := range []struct {
		name, body string
		want       []string
	}{
		{"argument", "\tprintln(select.FD_SETSIZE)\n}\n", []string{"./main.gp:6:10: " + hint}},
		{"two arguments on different lines", "\tprintln(select.FD_SETSIZE)\n\tprintln(select.FD_SETSIZE)\n}\n",
			[]string{"./main.gp:6:10: " + hint, "./main.gp:7:10: " + hint}},
		{"statement", "\tvar set int\n\tselect.FD_ZERO(&set)\n}\n", []string{"./main.gp:7:2: " + hint}},
		{"statements and arguments", "\tvar set int\n\tselect.FD_ZERO(&set)\n\tprintln(select.FD_SETSIZE)\n\tvar n int\n\t_ = n\n}\n",
			[]string{"./main.gp:7:2: " + hint, "./main.gp:8:10: " + hint}},
	} {
		t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
			typedEnv(t)
			dir := inDir(t, map[string]string{"main.gp": head + c.body})
			for _, cmd := range [][]string{{"check", "-e", "."}, {"build", "-o", dir + "/out", "."}} {
				r := gocppRun("", true, cmd...)
				got := agreeLines(r.stdout)
				if r.code != 1 || strings.Join(got, "\n") != strings.Join(c.want, "\n") {
					t.Errorf("%s: exit %d, output:\n%s\nwant the lines %q", cmd[0], r.code, r.stdout, c.want)
				}
			}
		})
	}
}
