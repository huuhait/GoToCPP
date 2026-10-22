package load

import (
	"strings"
	"testing"
)

// TestKeywordHeaderName checks the error for a header whose default name is a Go keyword, used by
// that name: <sys/select.h> is select, and select.FD_SETSIZE is a syntax error at the keyword.
// The message asks for a name, as for the other names a header can't have. Other syntax errors
// of the file, and the keyword where it is not a header's name, keep the parser's text.
func TestKeywordHeaderName(t *testing.T) {
	const hint = `<sys/select.h> has no package name, because select is a Go keyword: ` +
		`give it a name: import cselect "<sys/select.h>"`
	for _, c := range []struct{ name, src, want string }{
		{"use as a call", `package main

import "<sys/select.h>"

func main() {
	println(select.FD_SETSIZE)
}
`, "# example.com/app\n./main.gp:6:10: " + hint + "\n"},
		{"two uses on different lines", `package main

import "<sys/select.h>"

func main() {
	println(select.FD_SETSIZE)
	println(select.FD_SETSIZE)
}
`, "# example.com/app\n./main.gp:6:10: " + hint + "\n./main.gp:7:10: " + hint + "\n"},
		{"a space before the dot", `package main

import "<sys/select.h>"

func main() {
	println(select .FD_SETSIZE)
}
`, "# example.com/app\n./main.gp:6:10: " + hint + "\n"},
		{"use in a declaration", `package main

import "<sys/select.h>"

var set select.fd_set
`, "# example.com/app\n./main.gp:5:9: " + hint + "\n"},
		{"use at the start of a statement", `package main

import "<sys/select.h>"

func main() {
	var set int
	select.FD_ZERO(&set)
}
`, "# example.com/app\n./main.gp:7:2: " + hint + "\n"},
		{"uses of both forms and a statement after them", `package main

import "<sys/select.h>"

func main() {
	var set int
	select.FD_ZERO(&set)
	println(select.FD_SETSIZE)
	var n int = 1
	_ = n
}
`, "# example.com/app\n./main.gp:7:2: " + hint + "\n./main.gp:8:10: " + hint + "\n"},
		{"no use", `package main

import "<sys/select.h>"

func main() {
}
`, ""},
		{"a name is given", `package main

import cselect "<sys/select.h>"

func main() {
	println(cselect.FD_SETSIZE)
}
`, ""},
		{"the keyword after a dot is a name", `package main

import (
	"<foo.h>"
	"<sys/select.h>"
)

func main() {
	println(foo.select.FD)
}
`, ""},
		{"a use next to the keyword after a dot", `package main

import (
	"<foo.h>"
	"<sys/select.h>"
)

func main() {
	println(foo.select.FD)
	println(select.FD_SETSIZE)
}
`, "# example.com/app\n./main.gp:10:10: " + hint + "\n"},
		{"a select statement is not the name", `package main

import "<sys/select.h>"

func main() {
	var c chan int
	select {
	case <-c:
	}
	x :=
}
`, "# example.com/app\n./main.gp:11:1: expected operand, found '}'\n"},
		{"an error of the file before a use stays", `package main

import "<sys/select.h>"

func main() {
	x := 1 +
	println(select.FD_SETSIZE)
}
`, "# example.com/app\n./main.gp:7:10: " + hint + "\n"},
		{"an error of the file in another function stays", `package main

import "<sys/select.h>"

func f() {
	println(select.FD_SETSIZE)
}

func g() {
	var 1x int
}
`, "# example.com/app\n./main.gp:6:10: " + hint + "\n./main.gp:10:6: expected 'IDENT', found 1\n./main.gp:11:3: expected '}', found 'EOF'\n"},
		{"another keyword has no header", `package main

import "<sys/select.h>"

func main() {
	println(range.x)
}
`, "# example.com/app\n./main.gp:6:10: expected operand, found 'range'\n./main.gp:7:3: missing ',' in argument list\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			prog, _, root := loadMacros(t, map[string]string{"main.gp": c.src})
			got := errorText(t, prog, root)
			if got != c.want {
				t.Errorf("errors:\n%s\nwant:\n%s", got, c.want)
			}
		})
	}
}

// TestKeywordHeaderNameRange checks the place of the hint: it starts at the keyword and ends
// after it, also where the parser stopped at the dot (a statement that starts with select.),
// because the editor underlines the range.
func TestKeywordHeaderNameRange(t *testing.T) {
	const src = "package main\n\nimport \"<sys/select.h>\"\n\nfunc main() {\n\tvar set int\n" +
		"\tselect.FD_ZERO(&set)\n\tprintln(select.FD_SETSIZE)\n}\n"
	prog, p, _ := loadMacros(t, map[string]string{"main.gp": src})
	_ = prog
	ds := p.Errors.Diagnostics()
	if len(ds) != 2 {
		t.Fatalf("%d errors, want 2: %v", len(ds), ds)
	}
	for i, want := range []string{"\tselect.FD_ZERO", "println(select.FD_SETSIZE"} {
		start := strings.Index(src, want) + strings.Index(want, "select")
		d := ds[i]
		if d.Pos.Offset != start || d.End.Offset != start+len("select") || d.End.Line != d.Pos.Line ||
			d.End.Column != d.Pos.Column+len("select") {
			t.Errorf("error %d is at %v to %v, want offsets %d to %d", i, d.Pos, d.End, start, start+len("select"))
		}
	}
}

// TestKeywordHeaderNameTwoHeaders checks a file with two headers that would both be named
// select: the first one is the one in the hint.
func TestKeywordHeaderNameTwoHeaders(t *testing.T) {
	const src = "package main\n\nimport (\n\t\"<sys/select.h>\"\n\t\"./select.h\"\n)\n\nvar set select.fd_set\n"
	prog, _, root := loadMacros(t, map[string]string{"main.gp": src, "select.h": "int n;\n"})
	got := errorText(t, prog, root)
	want := "# example.com/app\n./main.gp:8:9: <sys/select.h> has no package name, because select is a Go keyword: " +
		"give it a name: import cselect \"<sys/select.h>\"\n"
	if got != want {
		t.Errorf("errors:\n%s\nwant:\n%s", got, want)
	}
}

// TestIncludeHints checks Include.Hint: it is set for a header without a name whose reason is a
// keyword, a name that is no Go name or a name that the package declares, with the name the
// hint would suggest, and empty for every other header.
func TestIncludeHints(t *testing.T) {
	_, p, _ := loadMacros(t, map[string]string{
		"main.gp": `package main

import (
	"<sys/select.h>"
	"./main.h"
	"./3d.h"
	"<string.h>"
	"<vector>"
)

func main() {}
`,
		"main.h": "int mn(void);\n",
		"3d.h":   "int d3(void);\n",
	})
	got := make(map[string]string)
	for _, f := range p.Files {
		if f.Info == nil {
			continue
		}
		for _, inc := range f.Info.Includes {
			got[inc.Path] = inc.Hint
		}
	}
	want := map[string]string{
		"<sys/select.h>": `<sys/select.h> has no package name, because select is a Go keyword: give it a name: import cselect "<sys/select.h>"`,
		"./main.h":       `"./main.h" has no package name, because main is declared by the package: give it a name: import cmain "./main.h"`,
		"./3d.h":         `"./3d.h" has no package name, because 3d is not a valid Go name: give it a name: import c3d "./3d.h"`,
		"<string.h>":     "",
		"<vector>":       "",
	}
	for path, hint := range want {
		if got[path] != hint {
			t.Errorf("hint of %s: %q, want %q", path, got[path], hint)
		}
	}
}
