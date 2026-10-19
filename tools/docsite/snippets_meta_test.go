package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The checks of the snippets are not vacuous: these tests feed them wrong input.

func TestParseSnippetsErrors(t *testing.T) {
	cases := []struct{ name, doc, want string }{
		{"unmarked go block", "text\n\n```go\npackage main\n```\n", "a go block with no <!-- program"},
		{"marker before the wrong block", "<!-- program: 05 -->\n```text\nx\n```\n", `the marker "program" is in front of a "text" block`},
		{"marker with no block", "<!-- excerpt: a.go -->\n\ntext\n", `the marker "excerpt" is not followed by a block`},
		{"marker at the end", "<!-- output -->\n", `the marker "output" is not followed by a block`},
		{"two markers", "<!-- output -->\n<!-- output -->\n```text\n```\n", `the marker "output" is not followed by a block`},
		{"output with no program", "<!-- output -->\n```text\nx\n```\n", "an output block with no program before it"},
		{"program without phase", "<!-- program: go -->\n```go\npackage main\n```\n", "starts with the two-digit phase"},
		{"program with bad phase", "<!-- program: 8 -->\n```go\npackage main\n```\n", "starts with the two-digit phase"},
		{"program with unknown flag", "<!-- program: 05 fast -->\n```go\npackage main\n```\n", `unknown flag "fast"`},
		{"illustration without reason", "<!-- illustration -->\n```go\nx\n```\n", "needs a reason"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var errs []string
			parseSnippets("d.md", c.doc, func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) })
			if !strings.Contains(strings.Join(errs, "\n"), c.want) {
				t.Errorf("want an error containing %q, got %q", c.want, errs)
			}
		})
	}
}

func TestParseSnippetsGood(t *testing.T) {
	doc := "<!-- program: 08 id=w go exit=2 check -->\n```go\npackage main\n```\n\n<!-- output -->\n```text\nhi\n```\n\n<!-- emitted: w -->\n```cpp\nint x;\n```\n\n<!-- excerpt: e/f.gp -->\n```go\nl\n```\n\n<!-- illustration: why -->\n```go\nfrag\n```\n\n```sh\nnot checked\n```\n"
	var errs []string
	got := parseSnippets("d.md", doc, func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) })
	if len(errs) != 0 {
		t.Fatalf("errors %q", errs)
	}
	if len(got) != 4 {
		t.Fatalf("%d snippets", len(got))
	}
	p := got[0]
	if p.kind != "program" || p.phase != "08" || p.id != "w" || !p.goOK || !p.check || p.exit != 2 || !p.hasOut || p.output != "hi" {
		t.Errorf("program: %+v", p)
	}
	if got[1].kind != "emitted" || got[1].ref != "w" || got[2].kind != "excerpt" || got[2].ref != "e/f.gp" || got[3].kind != "illustration" {
		t.Errorf("the others: %+v %+v %+v", got[1], got[2], got[3])
	}
}

func TestContainsInOrder(t *testing.T) {
	text := nonBlank("a\n  b\n\nc\nd\ne\n#line 5 \"x\"\nf\n", func(l string) bool { return strings.HasPrefix(l, "#line") })
	norm := func(s string) string { return strings.TrimSpace(s) }
	cases := []struct {
		name, block string
		ok          bool
	}{
		{"run", "a\nb\nc", true},
		{"with elision", "a\n...\nd\n// ...\nf", true},
		{"indent does not count", "  b\n\tc", true},
		{"directive dropped", "e\nf", true},
		{"gap without elision", "a\nc", false},
		{"wrong order", "d\n...\nc", false},
		{"missing line", "a\nb\nzzz", false},
		{"one line off", "a\nB", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, ok := containsInOrder(text, lineSegments(c.block, norm))
			if ok != c.ok {
				t.Errorf("containsInOrder(%q) = %v, want %v", c.block, ok, c.ok)
			}
		})
	}
}

// TestGoRunReportsTheExitStatus: the check against the real go command compares the exit status
// of the program, so a program that exits with 1 where the doc says 0 fails (go run itself
// reports 1 for every failing program, which hid it).
func TestGoRunReportsTheExitStatus(t *testing.T) {
	requireTool(t, "go", "GOCPP_REQUIRE_GO")
	for _, want := range []int{0, 1, 3} {
		dir := t.TempDir()
		src := fmt.Sprintf("package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n\tfmt.Println(\"hi\")\n\tos.Exit(%d)\n}\n", want)
		for name, body := range map[string]string{"main.go": src, "go.mod": "module snippet\n\ngo 1.25\n"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		out, code := goRun(t, dir)
		if code != want || out != "hi\n" {
			t.Errorf("exit %d: got status %d and output %q", want, code, out)
		}
	}
}
