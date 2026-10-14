package server

import (
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"testing"

	gpformat "github.com/huuhait/gocpp/internal/format"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/internal/lsp/cache"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
	"github.com/huuhait/gocpp/internal/syntax/parser"
)

// organize runs organizeImports on src, with the imports of the paths in
// unused taken as unused.
func organize(t *testing.T, src string, unused ...string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "a.gp", src, cache.ParseFull)
	if err != nil {
		t.Fatal(err)
	}
	tok := fset.File(f.FileStart)
	offsets := make(map[int]bool)
	for _, s := range f.Imports {
		p, _ := strconv.Unquote(s.Path.Value)
		if slices.Contains(unused, p) {
			offsets[tok.Offset(s.Pos())] = true
		}
	}
	res, err := organizeImports("a.gp", []byte(src), f, tok, offsets)
	if err != nil {
		t.Fatal(err)
	}
	return string(res)
}

func TestOrganizeImports(t *testing.T) {
	const body = "\nfunc main() {}\n"
	for _, tc := range []struct {
		name, src string
		unused    []string
		want      string // "" means: src, unchanged
	}{
		{name: "no imports", src: "package p\n" + body},
		{name: "organized", src: "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"example.com/x\"\n\n\t\"<vector>\"\n)\n" + body},
		{name: "single", src: "package p\n\nimport \"fmt\"\n" + body},
		{name: "single in parentheses", src: "package p\n\nimport (\n\t\"fmt\"\n)\n" + body},
		{
			name: "sort",
			src:  "package p\n\nimport (\n\t\"strings\"\n\t\"fmt\"\n\t\"bytes\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"bytes\"\n\t\"fmt\"\n\t\"strings\"\n)\n" + body,
		},
		{
			name: "standard first, then the others",
			src:  "package p\n\nimport (\n\t\"example.com/z\"\n\t\"strings\"\n\t\"example.com/a\"\n\t\"fmt\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"example.com/a\"\n\t\"example.com/z\"\n)\n" + body,
		},
		{
			name: "groups stay groups",
			src:  "package p\n\nimport (\n\t\"strings\"\n\t\"fmt\"\n\n\t\"example.com/b\"\n\t\"example.com/a\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"example.com/a\"\n\t\"example.com/b\"\n)\n" + body,
		},
		{
			name: "headers last, in their order",
			src:  "package p\n\nimport (\n\t\"<winsock2.h>\"\n\t\"strings\"\n\t\"<windows.h>\"\n\t\"fmt\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\n\t\"<winsock2.h>\"\n\t\"<windows.h>\"\n)\n" + body,
		},
		{
			name: "header groups keep their order and comments",
			src: "package p\n\nimport (\n\t// Drawing.\n\t\"./engine.hpp\"\n\trl \"<raylib.h>\"\n\n\t\"fmt\"\n\n" +
				"\tfs \"::std::filesystem\"\n\t\"<vector>\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\n\t// Drawing.\n\t\"./engine.hpp\"\n\trl \"<raylib.h>\"\n\n" +
				"\tfs \"::std::filesystem\"\n\t\"<vector>\"\n)\n" + body,
		},
		{
			name:   "unused gocpp import",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\t\"os\" // for Exit\n\t\"strings\"\n)\n" + body,
			unused: []string{"os"},
			want:   "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n" + body,
		},
		{
			name:   "unused leaves one: no parentheses",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n" + body,
			unused: []string{"os"},
			want:   "package p\n\nimport \"fmt\"\n" + body,
		},
		{
			name:   "unused leaves none",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n" + body,
			unused: []string{"os", "fmt"},
			want:   "package p\n" + body,
		},
		{
			name:   "unused single",
			src:    "package p\n\nimport \"os\" // why\n" + body,
			unused: []string{"os"},
			want:   "package p\n" + body,
		},
		{
			name:   "a header is never removed",
			src:    "package p\n\nimport (\n\t\"<vector>\"\n\t\"fmt\"\n)\n" + body,
			unused: []string{"<vector>", "fmt"},
			want:   "package p\n\nimport \"<vector>\"\n" + body,
		},
		{
			name: "comments stay with their imports",
			src: "package p\n\nimport (\n\t// The standard ones.\n\t\"strings\" // Builder\n\t\"fmt\"     // Println\n\n" +
				"\t/* x */ \"example.com/x\"\n\t// At the end.\n)\n" + body,
			want: "package p\n\nimport (\n\t// The standard ones.\n\t\"fmt\"     // Println\n\t\"strings\" // Builder\n\n" +
				"\t/* x */\n\t\"example.com/x\"\n\t// At the end.\n)\n" + body, // gofmt's place for /* x */
		},
		{
			name: "a comment line splits a group, as in gofmt",
			src:  "package p\n\nimport (\n\t\"strings\"\n\t// fmt next\n\t\"fmt\"\n)\n" + body,
		},
		{
			name: "dot, blank and named imports",
			src:  "package p\n\nimport (\n\t_ \"strings\"\n\tf \"fmt\"\n\t. \"bytes\"\n\t\"fmt\"\n)\n" + body,
			want: "package p\n\nimport (\n\t. \"bytes\"\n\t\"fmt\"\n\tf \"fmt\"\n\t_ \"strings\"\n)\n" + body,
		},
		{
			name: "the same import twice",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n\t\"fmt\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n" + body,
		},
		{
			name: "declarations become one",
			src:  "package p\n\nimport \"strings\"\n\n// Printing.\nimport \"fmt\"\nimport \"<vector>\"\n" + body,
			want: "package p\n\nimport (\n\t\"strings\"\n\n\t// Printing.\n\t\"fmt\"\n\n\t\"<vector>\"\n)\n" + body,
		},
		{
			name: "the declaration's doc stays",
			src:  "package p\n\n// Imports.\nimport (\n\t\"strings\"\n\t\"fmt\"\n)\n" + body,
			want: "package p\n\n// Imports.\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n" + body,
		},
		// Code on a line of an import declaration: no organizing.
		{name: "code after an import", src: "package p\n\nimport \"os\"; var x = 1 /* d */\n", unused: []string{"os"}},
		{name: "code after an import, then a comment", src: "package p\n\nimport \"strings\"\nimport \"fmt\"; var x = fmt.Sprint() // c\n"},
		{name: "code after a second declaration", src: "package p\n\nimport \"fmt\" // c\nimport \"os\"; var x = 1 // d\n"},
		{name: "code after the parenthesis", src: "package p\n\nimport (\n\t\"fmt\"\n\t// tail\n); var x = 1\n", unused: []string{"fmt"}},
		{
			name: "imports on the line of the parentheses (not code)",
			src:  "package p\n\nimport (\"os\"; \"fmt\")\n",
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n",
		},
		{name: "code before an import", src: "package p; import (\n\t\"os\"\n\t\"fmt\"\n)\n"},
		{
			name:   "comments after the parenthesis, and no import left",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\t// tail\n) // end\n" + body,
			unused: []string{"fmt"},
			want:   "package p\n\n// tail\n// end\n" + body,
		},
		{
			name:   "one import left, a comment after the parenthesis",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n) // end\n" + body,
			unused: []string{"os"},
			want:   "package p\n\nimport (\n\t\"fmt\"\n) // end\n" + body,
		},
		{
			name:   "one gocpp import and a header left, a comment after the parenthesis",
			src:    "package p\n\nimport \"<vector>\"\nimport (\n\t\"fmt\"\n\t\"os\"\n) // end\n" + body,
			unused: []string{"os", "fmt"},
			want:   "package p\n\nimport (\n\t\"<vector>\"\n) // end\n" + body,
		},
		{
			name: "a comment line above a comment before an import",
			src:  "package p\n\nimport (\n\t\"strings\"\n\t// c\n\t/* p */ al \"strings\"\n\t\"fmt\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"strings\"\n\t// c\n\t\"fmt\"\n\t/* p */ al \"strings\"\n)\n" + body,
		},
		{
			name:   "unused, a comment after the parenthesis",
			src:    "package p\n\nimport (\n\t\"fmt\"\n) // end\n" + body,
			unused: []string{"fmt"},
			want:   "package p\n\n// end\n" + body,
		},
		{
			name:   "unused, at the end of the file",
			src:    "package p\n\nimport \"fmt\"\n",
			unused: []string{"fmt"},
			want:   "package p\n",
		},
		{
			name:   "unused, at the end of a crlf file",
			src:    "package p\r\n\r\nimport \"fmt\"\r\n",
			unused: []string{"fmt"},
			want:   "package p\r\n",
		},
		{
			name: "comments of a mixed group go with its first import",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\t// About the headers.\n\t\"<vector>\"\n\t\"os\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\n\t// About the headers.\n\t\"<vector>\"\n)\n" + body,
		},
		{
			name: "a gocpp part without comments joins the group before",
			src:  "package p\n\nimport (\n\t\"example.com/x\"\n\t// About the headers.\n\t\"<vector>\"\n\t\"fmt\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\n\t\"example.com/x\"\n\n\t// About the headers.\n\t\"<vector>\"\n)\n" + body,
		},
		{
			name:   "the comments of a removed group go to the next one",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\n\t// Debugging.\n\t\"log\"\n\n\t// Others.\n\t\"example.com/x\"\n)\n" + body,
			unused: []string{"log"},
			want:   "package p\n\nimport (\n\t\"fmt\"\n\n\t// Debugging.\n\n\t// Others.\n\t\"example.com/x\"\n)\n" + body,
		},
		{
			name:   "the comments of a removed last group go to the end",
			src:    "package p\n\nimport (\n\t\"fmt\"\n\t// end of the first block\n)\n\nimport \"os\"\n" + body,
			unused: []string{"os"},
			want:   "package p\n\nimport (\n\t\"fmt\"\n\t// end of the first block\n)\n" + body,
		},
		{
			name:   "the comments stay when no import is left",
			src:    "package p\n\nimport (\n\t// Debugging.\n\t\"log\"\n)\n" + body,
			unused: []string{"log"},
			want:   "package p\n\n// Debugging.\n" + body,
		},
		{
			name: "an empty line before a comment after two imports (goimports)",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t// c\n\t\"example.com/x\"\n)\n" + body,
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\n\t// c\n\t\"example.com/x\"\n)\n" + body,
		},
		{
			name: "no empty line before a comment after one import (goimports)",
			src:  "package p\n\nimport (\n\t\"fmt\"\n\t// c\n\t\"example.com/x\"\n)\n" + body,
		},
		{
			name: "crlf",
			src:  "package p\r\n\r\nimport (\r\n\t\"strings\" // x\r\n\t\"fmt\"\r\n)\r\n\r\nfunc main() {}\r\n",
			want: "package p\r\n\r\nimport (\n\t\"fmt\"\n\t\"strings\" // x\n)\r\n\r\nfunc main() {}\r\n",
		},
		{
			name: "the rest is not formatted (but for the empty line after the imports)",
			src:  "package p\n\nimport (\n\t\"strings\"\n\t\"fmt\"\n)\nfunc main()   {  }\n",
			want: "package p\n\nimport (\n\t\"fmt\"\n\t\"strings\"\n)\n\nfunc main()   {  }\n", // gofmt's empty line after the imports
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := tc.want
			if want == "" {
				want = tc.src
			}
			got := organize(t, tc.src, tc.unused...)
			if got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
			// Organized imports stay as they are.
			if again := organize(t, got); again != got {
				t.Errorf("a second time:\n%s\nwant:\n%s", again, got)
			}
			// The edits make the same text (the CRLF case keeps "\r\n").
			wantText := want
			if strings.Contains(tc.src, "\r\n") {
				wantText = strings.ReplaceAll(noCRLF(want), "\n", "\r\n")
			}
			if res := applyEdits(t, tc.src, computeEdits(context.Background(), protocol.NewMapper(tc.src), got)); res != wantText {
				t.Errorf("the edits give:\n%q\nwant:\n%q", res, wantText)
			}
		})
	}
}

// TestOrganizeImportsRandom organizes random import declarations (gocpp
// and header imports, names, comment lines, comments before and after
// imports, empty lines, several declarations, code after them, sometimes
// on their lines) with random unused imports, and checks that the result
// parses, keeps the code after the imports, every comment line and the
// comments of the imports that stay, keeps the headers in order after the
// gocpp imports, is formatted when the file was, and stays the same when
// it is organized again. A file with code on a line of an import
// declaration stays as it is. (organize fails when the safety net of
// organizeImports, sameCode, finds changed code outside the imports.)
func TestOrganizeImportsRandom(t *testing.T) {
	r := rand.New(rand.NewSource(4))
	pool := []string{`"fmt"`, `"os"`, `"strings"`, `"bytes"`, `"example.com/x"`, `"example.com/y"`, `f "fmt"`, `_ "os"`, `. "bytes"`,
		`"<vector>"`, `"<a.h>"`, `"./b.hpp"`, `fs "::std::filesystem"`}
	paths := []string{"fmt", "os", "strings", "bytes", "example.com/x", "example.com/y"}
	n, changed := 0, 0
	for i := range 3000 {
		var b strings.Builder
		codeLine := r.Intn(8) == 0 // code on a line of an import declaration
		if codeLine && r.Intn(3) == 0 {
			b.WriteString("package p; ")
		} else {
			b.WriteString("package p\n\n")
			codeLine = codeLine && r.Intn(2) == 0
		}
		comments := 0
		decls := 1 + r.Intn(3)
		for d := range decls {
			last := d == decls-1
			if d > 0 && r.Intn(2) == 0 {
				fmt.Fprintf(&b, "// line-%d\n", comments)
				comments++
			}
			spec := func(indent, code string) {
				s := pool[r.Intn(len(pool))]
				p := strings.Trim(s[strings.Index(s, `"`):], `"`)
				b.WriteString(indent)
				if r.Intn(6) == 0 {
					fmt.Fprintf(&b, "/* pre-%s */ ", p)
				}
				b.WriteString(s + code)
				if r.Intn(4) == 0 {
					fmt.Fprintf(&b, " // own-%s", p)
				}
				b.WriteString("\n")
			}
			if r.Intn(3) == 0 {
				b.WriteString("import ")
				code := ""
				if last && codeLine {
					code = "; var onLine = 1"
				}
				spec("", code)
				continue
			}
			b.WriteString("import (\n")
			for range 1 + r.Intn(6) {
				switch r.Intn(5) {
				case 0:
					fmt.Fprintf(&b, "\t// line-%d\n", comments)
					comments++
				case 1:
					b.WriteString("\n")
				}
				spec("\t", "")
			}
			if r.Intn(5) == 0 {
				fmt.Fprintf(&b, "\t// line-%d\n", comments)
				comments++
			}
			switch {
			case last && codeLine:
				b.WriteString("); var onLine = 1\n")
			case r.Intn(6) == 0:
				fmt.Fprintf(&b, ") // line-%d\n", comments) // a comment after ")"
				comments++
			default:
				b.WriteString(")\n")
			}
		}
		b.WriteString("\nvar sentinel = 1 // keep\n")
		if r.Intn(2) == 0 {
			b.WriteString("\n")
		}
		if r.Intn(3) > 0 {
			b.WriteString("func main() {}\n")
		}
		src := b.String()
		var unused []string
		for _, p := range paths {
			if r.Intn(3) == 0 {
				unused = append(unused, p)
			}
		}
		fset := token.NewFileSet()
		if _, err := parser.ParseFile(fset, "a.gp", src, cache.ParseFull); err != nil {
			continue
		}
		got := organize(t, src, unused...)
		n++
		if got != src {
			changed++
		}
		fail := func(format string, args ...any) {
			t.Helper()
			t.Fatalf("case %d (unused %v): %s\nsrc:\n%s\ngot:\n%s", i, unused, fmt.Sprintf(format, args...), src, got)
		}
		f, err := parser.ParseFile(token.NewFileSet(), "a.gp", got, cache.ParseFull)
		if err != nil {
			fail("the result doesn't parse: %v", err)
		}
		if codeLine && got != src {
			fail("code on a line of an import declaration, but the file changed")
		}
		for _, code := range []string{"var sentinel = 1 // keep\n", "func main() {}\n", "var onLine = 1"} {
			if strings.Contains(src, code) && !strings.Contains(got, code) {
				fail("the code %q is lost", code)
			}
		}
		if codeLine {
			continue // not organized
		}
		for k := range comments {
			if !strings.Contains(got, fmt.Sprintf("// line-%d\n", k)) {
				fail("the comment line-%d is lost", k)
			}
		}
		for _, s := range f.Imports {
			p, _ := strconv.Unquote(s.Path.Value)
			if slices.Contains(unused, p) && !load.IsHeaderOrNamespace(p) {
				fail("the unused %s stays", p)
			}
		}
		for _, p := range paths {
			for _, own := range []string{"// own-" + p + "\n", "/* pre-" + p + " */"} {
				if !slices.Contains(unused, p) && strings.Contains(src, own) && !strings.Contains(got, own) {
					fail("the comment %s is lost", own)
				}
			}
		}
		var headers []string
		for _, line := range strings.Split(src, "\n") {
			if strings.Contains(line, `"<`) || strings.Contains(line, `"./`) || strings.Contains(line, `"::`) {
				h := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "import"))
				if strings.HasPrefix(h, "/*") {
					_, h, _ = strings.Cut(h, "*/ ")
				}
				headers = append(headers, h)
			}
		}
		var gotHeaders []string
		seen := false
		for _, s := range f.Imports {
			p, _ := strconv.Unquote(s.Path.Value)
			if load.IsHeaderOrNamespace(p) {
				seen = true
				h := s.Path.Value
				if s.Name != nil {
					h = s.Name.Name + " " + h
				}
				gotHeaders = append(gotHeaders, h)
			} else if seen {
				fail("the gocpp import %s comes after a header", p)
			}
		}
		for k := range headers {
			headers[k], _, _ = strings.Cut(headers[k], " //")
		}
		if !slices.Equal(headers, gotHeaders) {
			fail("the headers were %v, now %v", headers, gotHeaders)
		}
		if fsrc, err := gpformat.Source("a.gp", []byte(src)); err == nil && string(fsrc) == src {
			if fgot, err := gpformat.Source("a.gp", []byte(got)); err != nil || string(fgot) != got {
				fail("the result is not formatted (%v)", err)
			}
		}
		if again := organize(t, got, unused...); again != got {
			fail("a second time:\n%s", again)
		}
	}
	t.Logf("%d files organized, %d changed", n, changed)
}

// TestSameCode checks the safety net of organizeImports: code outside the
// import declarations that is lost, or that becomes a comment, is found.
func TestSameCode(t *testing.T) {
	const src = "package p\n\nimport (\n\t\"fmt\"\n\t// tail\n) // end\n\nvar x = 1 // c\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "a.gp", src, cache.ParseFull)
	if err != nil {
		t.Fatal(err)
	}
	var decls []*ast.GenDecl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			decls = append(decls, g)
		}
	}
	tok, toks := fset.File(f.FileStart), scanFile("a.gp", []byte(src))
	for _, tc := range []struct {
		res  string
		left [2]int
		ok   bool
	}{
		{"package p\n\nimport \"fmt\" // end\n\nvar x = 1 // c\n", [2]int{}, false},         // the comment after ")" moved into the imports
		{"package p\n\nimport (\n\t\"fmt\"\n) // end\n\nvar x = 1 // c\n", [2]int{}, true},  // a comment in the imports went
		{"package p\n\nimport (\n\t\"fmt\"\n) // end\n\nvar x = 2 // c\n", [2]int{}, false}, // changed code
		{"package p\n\nimport (\n\t\"fmt\"\n) // end\n\n// c\n", [2]int{}, false},           // lost code
		{"package p\n\n// tail\n// end\n\nvar x = 1 // c\n", [2]int{11, 18}, true},          // no import left
		{"package p\n\n// tail // end; var x = 1 // c\n", [2]int{11, 18}, false},            // code in a comment
		{"package p\n\nvar y = 1\n// end\n\nvar x = 1 // c\n", [2]int{11, 20}, false},       // code where comments are left
	} {
		// When no import is left, fmt was unused.
		var unused map[int]bool
		if tc.left != [2]int{} {
			unused = map[int]bool{tok.Offset(f.Imports[0].Pos()): true}
		}
		err := sameCode("a.gp", []byte(src), decls, tok, toks, unused, []byte(tc.res), tc.left)
		if (err == nil) != tc.ok {
			t.Errorf("%q: got %v, want ok %v", tc.res, err, tc.ok)
		}
	}
}

// TestSameImports checks the safety net on the imports themselves: a
// header that goes, two headers that change places, a new name, a gocpp
// import that goes. The same import twice may become one.
func TestSameImports(t *testing.T) {
	specs := func(src string) []*ast.ImportSpec {
		f, err := parser.ParseFile(token.NewFileSet(), "a.gp", "package p\n\nimport (\n"+src+")\n", parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		return f.Imports
	}
	before := "\"fmt\"\n\"fmt\"\nrl \"<raylib.h>\"\n\"<vector>\"\n\"os\"\n"
	for _, tc := range []struct {
		after string
		ok    bool
	}{
		{before, true},
		{"\"fmt\"\n\"os\"\nrl \"<raylib.h>\"\n\"<vector>\"\n", true},    // sorted, fmt once
		{"\"fmt\"\n\"os\"\n\"<vector>\"\n", false},                      // a header went
		{"\"fmt\"\n\"os\"\n\"<vector>\"\nrl \"<raylib.h>\"\n", false},   // headers swapped
		{"\"fmt\"\n\"os\"\nr \"<raylib.h>\"\n\"<vector>\"\n", false},    // a new name
		{"\"fmt\"\nrl \"<raylib.h>\"\n\"<vector>\"\n", false},           // os went
		{"f \"fmt\"\n\"os\"\nrl \"<raylib.h>\"\n\"<vector>\"\n", false}, // a new name
		{"\"fmt\"\n\"fmt\"\n\"fmt\"\n\"os\"\nrl \"<raylib.h>\"\n\"<vector>\"\n", false},
	} {
		if err := sameImports(specs(before), specs(tc.after)); (err == nil) != tc.ok {
			t.Errorf("%q: got %v, want ok %v", tc.after, err, tc.ok)
		}
	}
}
