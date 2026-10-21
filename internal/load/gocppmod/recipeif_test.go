package gocppmod

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// Tests of the if blocks of a recipe (ADR-129).

// A target is the set of tags that are satisfied.
type target map[string]bool

func (tg target) tag(name string) bool { return tg[name] }

var (
	linuxAMD64   = target{"linux": true, "amd64": true, "unix": true}
	linuxARM64   = target{"linux": true, "arm64": true, "unix": true}
	darwinARM64  = target{"darwin": true, "arm64": true, "unix": true}
	windowsAMD64 = target{"windows": true, "amd64": true}
)

// applied returns the statements of the recipe that apply to the target, as
// "verb arg arg" in file order.
func applied(r *Recipe, tg target) []string {
	var out []string
	for _, s := range r.Stmts {
		if s.Cond.Match(tg.tag) {
			out = append(out, s.Verb+" "+strings.Join(s.Args, " "))
		}
	}
	return out
}

func TestRecipeIfBlocks(t *testing.T) {
	// What the block with braces in its arguments gives where it is taken.
	inBlock := []string{
		`define OPEN={ CLOSE=}`, `cflags -DBRACES={}`, "src a}.c", `define PAIR=}{`, "src after.c",
	}

	type targets struct {
		linux, linuxARM, darwin, windows []string
	}
	for _, tc := range []struct {
		name string
		body []string
		want targets
	}{
		{
			name: "a block for each system",
			body: []string{
				"src all.c",
				"if windows {",
				"\tlink gdi32",
				"}",
				"if linux {",
				"\tlink GL",
				"}",
				"if darwin {",
				"\tlink objc",
				"}",
				"define LAST",
			},
			want: targets{
				linux:    []string{"src all.c", "link -lGL", "define LAST"},
				linuxARM: []string{"src all.c", "link -lGL", "define LAST"},
				darwin:   []string{"src all.c", "link -lobjc", "define LAST"},
				windows:  []string{"src all.c", "link -lgdi32", "define LAST"},
			},
		},
		{
			name: "else",
			body: []string{
				"if windows {",
				"\tsrc win.c",
				"} else {",
				"\tsrc posix.c",
				"}",
			},
			want: targets{
				linux:    []string{"src posix.c"},
				linuxARM: []string{"src posix.c"},
				darwin:   []string{"src posix.c"},
				windows:  []string{"src win.c"},
			},
		},
		{
			name: "else if with a final else",
			body: []string{
				"if windows {",
				"\tsrc win.c",
				"} else if linux {",
				"\tsrc linux.c",
				"} else if darwin {",
				"\tsrc mac.c",
				"} else {",
				"\tsrc other.c",
				"}",
			},
			want: targets{
				linux:    []string{"src linux.c"},
				linuxARM: []string{"src linux.c"},
				darwin:   []string{"src mac.c"},
				windows:  []string{"src win.c"},
			},
		},
		{
			// A branch is taken only if no earlier branch of its chain was: linux is also unix.
			name: "the first true branch only",
			body: []string{
				"if linux {",
				"\tsrc linux.c",
				"} else if unix {",
				"\tsrc unix.c",
				"}",
			},
			want: targets{
				linux:    []string{"src linux.c"},
				linuxARM: []string{"src linux.c"},
				darwin:   []string{"src unix.c"},
			},
		},
		{
			name: "negation",
			body: []string{
				"if !windows {",
				"\tdefine UNIX",
				"}",
			},
			want: targets{
				linux:    []string{"define UNIX"},
				linuxARM: []string{"define UNIX"},
				darwin:   []string{"define UNIX"},
			},
		},
		{
			name: "or",
			body: []string{
				"if linux || darwin {",
				"\tlink m",
				"}",
			},
			want: targets{
				linux:    []string{"link -lm"},
				linuxARM: []string{"link -lm"},
				darwin:   []string{"link -lm"},
			},
		},
		{
			name: "and",
			body: []string{
				"if linux && arm64 {",
				"\tdefine ARM",
				"}",
				"if windows && amd64 {",
				"\tdefine WIN64",
				"}",
			},
			want: targets{
				linuxARM: []string{"define ARM"},
				windows:  []string{"define WIN64"},
			},
		},
		{
			name: "parentheses and negated groups",
			body: []string{
				"if (linux || darwin) && arm64 {",
				"\tdefine ARM_UNIX",
				"}",
				"if !(linux || windows) {",
				"\tdefine NEITHER",
				"}",
			},
			want: targets{
				linuxARM: []string{"define ARM_UNIX"},
				darwin:   []string{"define ARM_UNIX", "define NEITHER"},
			},
		},
		{
			name: "nesting",
			body: []string{
				"src a.c",
				"if unix {",
				"\tsrc posix.c",
				"\tif linux {",
				"\t\tsrc linux.c",
				"\t} else {",
				"\t\tsrc notlinux.c",
				"\t}",
				"\tdefine AFTER_INNER",
				"} else {",
				"\tif amd64 {",
				"\t\tsrc win64.c",
				"\t}",
				"}",
			},
			want: targets{
				linux:    []string{"src a.c", "src posix.c", "src linux.c", "define AFTER_INNER"},
				linuxARM: []string{"src a.c", "src posix.c", "src linux.c", "define AFTER_INNER"},
				darwin:   []string{"src a.c", "src posix.c", "src notlinux.c", "define AFTER_INNER"},
				windows:  []string{"src a.c", "src win64.c"},
			},
		},
		{
			// A tag that is not known is not satisfied, as in a //go:build line.
			name: "an unknown tag is false",
			body: []string{
				"if widnows {",
				"\tsrc typo.c",
				"} else {",
				"\tsrc fine.c",
				"}",
			},
			want: targets{
				linux:    []string{"src fine.c"},
				linuxARM: []string{"src fine.c"},
				darwin:   []string{"src fine.c"},
				windows:  []string{"src fine.c"},
			},
		},
		{
			name: "comments and blank lines",
			body: []string{
				"// the system libraries",
				"if windows { // GLFW's import library",
				"",
				"\t// two",
				"\tlink glfw3dll // one",
				"} // end of windows",
				"if linux {",
				"\tlink GL",
				"} else { // not linux",
				"\tlink m",
				"}",
			},
			want: targets{
				linux:    []string{"link -lGL"},
				linuxARM: []string{"link -lGL"},
				darwin:   []string{"link -lm"},
				windows:  []string{"link -lglfw3dll", "link -lm"},
			},
		},
		{
			name: "spaces instead of tabs, braces without space before the condition",
			body: []string{
				"if   windows   {",
				"    link gdi32",
				"}   else   if   linux   {",
				"    link GL",
				"}",
			},
			want: targets{
				linux:    []string{"link -lGL"},
				linuxARM: []string{"link -lGL"},
				windows:  []string{"link -lgdi32"},
			},
		},
		{
			// A brace in a quoted argument or in a comment is not a block.
			name: "braces in quotes and comments",
			body: []string{
				`define OPEN="{" CLOSE='}' // } {`,
				"if linux { // {",
				`	cflags "-DBRACES={}" // }`,
				"	src `a}.c`",
				`	define PAIR="}{"`,
				"} // }",
				"src after.c // {",
			},
			want: targets{
				linux:    inBlock,
				linuxARM: inBlock,
				darwin:   []string{`define OPEN={ CLOSE=}`, "src after.c"},
				windows:  []string{`define OPEN={ CLOSE=}`, "src after.c"},
			},
		},
		{
			name: "four levels",
			body: []string{
				"if unix {",
				"\tif linux {",
				"\t\tif amd64 {",
				"\t\t\tif linux || darwin {",
				"\t\t\t\tdefine DEEP",
				"\t\t\t}",
				"\t\t}",
				"\t}",
				"}",
			},
			want: targets{
				linux: []string{"define DEEP"},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, errs := recipeOf(t, tc.body...)
			if len(errs) > 0 {
				t.Fatalf("errors: %v", errs)
			}
			for _, row := range []struct {
				name string
				tg   target
				want []string
			}{
				{"linux/amd64", linuxAMD64, tc.want.linux},
				{"linux/arm64", linuxARM64, tc.want.linuxARM},
				{"darwin/arm64", darwinARM64, tc.want.darwin},
				{"windows/amd64", windowsAMD64, tc.want.windows},
			} {
				got := applied(r, row.tg)
				if !reflect.DeepEqual(got, row.want) {
					t.Errorf("%s: applied\n%q\nwant\n%q", row.name, got, row.want)
				}
			}
		})
	}
}

// TestRecipeIfLines checks the line of each statement and its condition as
// text, including the conditions of an else that is the negation of the earlier
// branches.
func TestRecipeIfLines(t *testing.T) {
	r, errs := recipeOf(t,
		"\tsrc a.c",
		"\tif windows {",
		"\t\tsrc b.c",
		"\t} else if linux && amd64 {",
		"\t\tsrc c.c",
		"\t} else {",
		"\t\tsrc d.c",
		"\t}",
		"\tsrc e.c",
	)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	var got []string
	for _, s := range r.Stmts {
		got = append(got, fmt.Sprintf("%d %s [%s]", s.Line, s.Args[0], s.Cond))
	}
	want := []string{
		"4 a.c []",
		"6 b.c [windows]",
		"8 c.c [!windows && linux && amd64]",
		"10 d.c [!(windows || (linux && amd64))]",
		"12 e.c []",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("statements\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if (Cond{}).String() != "" || !(Cond{}).Match(func(string) bool { return false }) {
		t.Error("the empty condition applies to every target and prints as nothing")
	}
}

// rawRecipe reads the lines as the body of a recipe that starts at line 4
// of its file, with no gocpp.mod around it: for the shapes that the file
// reader would not let through (the reader and ParseRecipe agree on every
// body that the file reader accepts).
func rawRecipe(lines ...string) (*Recipe, ErrorList) {
	b := &Block{Start: Position{Line: 3, LineRune: 1}, Raw: lines}
	return ParseRecipe("gocpp.mod", "example.com/lib", b)
}

func TestRecipeIfErrors(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []string

		// The first error: its line in the file, its column and a part of its message.
		line, col int
		want      string
	}{
		{
			name: "old form",
			body: []string{"\twindows: link gdi32"},
			line: 4, col: 2,
			want: `a condition is written as a block, if windows { ... }, since ADR-129`,
		},
		{
			name: "old form with a list",
			body: []string{"\tsrc a.c", "\tlinux,darwin: link dl"},
			line: 5, col: 2,
			want: "if linux || darwin { ... }",
		},
		{
			name: "old form with a negation",
			body: []string{"\t!windows: define X"},
			line: 4, col: 2,
			want: "if !windows { ... }",
		},
		{
			name: "old form inside a block",
			body: []string{"\tif linux {", "\t\tlinux: link GL", "\t}"},
			line: 5, col: 3,
			want: "since ADR-129",
		},
		{
			name: "unknown statement",
			body: []string{"\tsrc a.c", "\tfrobnicate x"},
			line: 5, col: 2,
			want: `unknown statement "frobnicate"`,
		},
		{
			name: "unknown statement in a block",
			body: []string{"\tif linux {", "\t\tfrobnicate x", "\t}"},
			line: 5, col: 3,
			want: `unknown statement "frobnicate"`,
		},
		{
			name: "an if without a condition",
			body: []string{"\tif {", "\t\tsrc a.c", "\t}"},
			line: 4, col: 5,
			want: "if without a condition",
		},
		{
			name: "an if with no brace",
			body: []string{"\tif windows", "\t\tsrc a.c"},
			line: 4, col: 5,
			want: "an if line ends with {",
		},
		{
			name: "an if with a glued brace",
			body: []string{"\tif windows{", "\t\tsrc a.c", "\t}"},
			line: 4, col: 5,
			want: `put a space before the "{"`,
		},
		{
			name: "an if with nothing else",
			body: []string{"\tif"},
			line: 4, col: 2,
			want: "an if line ends with {",
		},
		{
			name: "a one-line if",
			body: []string{"\tif windows { link gdi32 }"},
			line: 4, col: 26,
			want: "an if line ends with {",
		},
		{
			name: "a condition that does not parse",
			body: []string{"\tif windows && {", "\t}"},
			line: 4, col: 15,
			want: "the condition \"windows &&\" is not valid: unexpected end of expression",
		},
		{
			name: "a condition with a bad token",
			body: []string{"\tif win-dows {", "\t}"},
			line: 4, col: 8,
			want: "is not valid: invalid syntax at -",
		},
		{
			name: "a colon in a condition",
			body: []string{"\tif windows: {", "\t}"},
			line: 4, col: 12,
			want: "is not valid",
		},
		{
			name: "a double negation",
			body: []string{"\tif !!linux {", "\t}"},
			line: 4, col: 6,
			want: "is not valid",
		},
		{
			name: "a missing parenthesis",
			body: []string{"\tif (linux || darwin {", "\t}"},
			line: 4, col: 5,
			want: "is not valid",
		},
		{
			name: "a quoted condition",
			body: []string{"\tif \"linux\" {", "\t}"},
			line: 4, col: 5,
			want: "is not valid",
		},
		{
			name: "a glued if",
			body: []string{"\tif(linux) {", "\t}"},
			line: 4, col: 2,
			want: `write a space after "if"`,
		},
		{
			name: "else without if",
			body: []string{"\telse {", "\t\tsrc a.c", "\t}"},
			line: 4, col: 2,
			want: `"else" without "if"`,
		},
		{
			name: "else after the brace line",
			body: []string{"\tif linux {", "\t}", "\telse {", "\t}"},
			line: 6, col: 2,
			want: `"else" without "if"`,
		},
		{
			name: "a closing brace with no block",
			body: []string{"\tsrc a.c", "\t}"},
			line: 5, col: 2,
			want: "unexpected }: no if block is open",
		},
		{
			name: "else with no block",
			body: []string{"\t} else {", "\t}"},
			line: 4, col: 2,
			want: "unexpected }: no if block is open",
		},
		{
			name: "an else with no brace",
			body: []string{"\tif linux {", "\t} else", "\t}"},
			line: 5, col: 4,
			want: "an else line ends with {",
		},
		{
			name: "an else with a statement",
			body: []string{"\tif linux {", "\t} else link m {", "\t}"},
			line: 5, col: 9,
			want: "after else comes { or if",
		},
		{
			name: "an else if with no condition",
			body: []string{"\tif linux {", "\t} else if {", "\t}"},
			line: 5, col: 12,
			want: "if without a condition",
		},
		{
			name: "an else if with a bad condition",
			body: []string{"\tif linux {", "\t} else if || {", "\t}"},
			line: 5, col: 12,
			want: "is not valid",
		},
		{
			name: "else after else",
			body: []string{"\tif linux {", "\t} else {", "\t} else {", "\t}"},
			line: 6, col: 4,
			want: "else after else",
		},
		{
			name: "else if after else",
			body: []string{"\tif linux {", "\t} else {", "\t} else if darwin {", "\t}"},
			line: 6, col: 4,
			want: "else after else",
		},
		{
			name: "a statement after the closing brace",
			body: []string{"\tif linux {", "\t} src a.c"},
			line: 5, col: 4,
			want: "after a } only else may follow on the line",
		},
		{
			name: "a statement after the closing brace of an else",
			body: []string{"\tif linux {", "\t} else {", "\t} link m"},
			line: 6, col: 4,
			want: "after a } only else may follow on the line",
		},
		{
			name: "a glued else",
			body: []string{"\tif linux {", "\t}else {", "\t}"},
			line: 5, col: 2,
			want: `a "}" is a word of its own`,
		},
		{
			name: "a closing brace at the end of a statement",
			body: []string{"\tif linux {", "\t\tlink m }", "\t}"},
			line: 5, col: 10,
			want: "blocks inside a recipe are not part of the recipe language",
		},
		{
			name: "another block",
			body: []string{"\tdefine {", "\t\tA=1", "\t}"},
			line: 4, col: 9,
			want: "a block in a recipe is an if block",
		},
		{
			name: "a block with a verb",
			body: []string{"\tif linux {", "\t\tlink {", "\t\t}", "\t}"},
			line: 5, col: 8,
			want: "a block in a recipe is an if block",
		},
		{
			name: "a lone brace",
			body: []string{"\t{", "\t}"},
			line: 4, col: 2,
			want: "a block in a recipe is an if block",
		},
		{
			name: "blocks nested too deep",
			body: []string{
				"\tif a {",
				"\t\tif b {",
				"\t\t\tif c {",
				"\t\t\t\tif d {",
				"\t\t\t\t\tif e {",
				"\t\t\t\t\t}",
				"\t\t\t\t}",
				"\t\t\t}",
				"\t\t}",
				"\t}",
			},
			line: 8, col: 6,
			want: "nest more than 4 deep",
		},
		{
			name: "an if that is not closed",
			body: []string{"\tsrc a.c", "\tif linux {", "\t\tsrc b.c"},
			line: 5, col: 2,
			want: "the if block is not closed: a } is missing",
		},
		{
			name: "an else that is not closed",
			body: []string{"\tif linux {", "\t} else {", "\t\tsrc b.c"},
			line: 5, col: 4,
			want: "the else block is not closed",
		},
		{
			name: "an inner block that is not closed",
			body: []string{"\tif linux {", "\t\tif amd64 {", "\t\t\tsrc b.c", "\t\t}"},
			line: 4, col: 2,
			want: "the if block is not closed",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The file reader reports a block that does not close, and the recipe is not read then;
			// the lines go straight to the recipe reader here.
			_, errs := rawRecipe(tc.body...)
			if len(errs) == 0 {
				t.Fatalf("no error")
			}
			e := errs[0]
			wrongPlace := e.Pos.Line != tc.line || e.Pos.LineRune != max(tc.col, 1)
			if wrongPlace || !strings.Contains(e.Err, tc.want) {
				t.Errorf("first error: line %d column %d: %s\nwant line %d column %d with %q\nall: %v",
					e.Pos.Line, e.Pos.LineRune, e.Err, tc.line, tc.col, tc.want, errs)
			}
			if !strings.HasPrefix(e.Err, "cxx example.com/lib: ") {
				t.Errorf("the error does not name the module: %q", e.Err)
			}
			wantPrefix := fmt.Sprintf("gocpp.mod:%d:%d: ", tc.line, tc.col)
			if tc.col <= 1 {
				wantPrefix = fmt.Sprintf("gocpp.mod:%d: ", tc.line)
			}
			if got := e.Error(); !strings.HasPrefix(got, wantPrefix) {
				t.Errorf("Error() = %q, want the prefix %q", got, wantPrefix)
			}
		})
	}
}

// TestRecipeIfErrorsKeepReading checks that an error in an if block does not
// hide the lines after it: the statements around are read and the later errors
// are reported too.
func TestRecipeIfErrorsKeepReading(t *testing.T) {
	r, errs := recipeOf(t,
		"\tsrc a.c",
		"\tif linux {",
		"\t\tfrobnicate",
		"\t\tsrc b.c",
		"\t} else if windows && {",
		"\t\tsrc c.c",
		"\t}",
		"\tsrc d.c",
		"\twindows: link x",
	)
	if len(errs) != 3 || errs[0].Pos.Line != 6 || errs[1].Pos.Line != 8 || errs[2].Pos.Line != 12 {
		t.Errorf("errors: %v", errs)
	}
	var got []string
	for _, s := range r.Stmts {
		got = append(got, s.Args[0])
	}
	if !reflect.DeepEqual(got, []string{"a.c", "b.c", "c.c", "d.c"}) {
		t.Errorf("statements: %v", got)
	}
}

func TestRecipeTooManyConditions(t *testing.T) {
	lines := []string{"\tif linux {", "\t\tsrc a.c", "\t}"}
	var body []string
	for range MaxRecipeConditions + 5 {
		body = append(body, lines...)
	}
	r, errs := recipeOf(t, body...)
	if len(errs) != 1 || !strings.Contains(errs[0].Err, "more than 1024 conditions") {
		t.Fatalf("errors: %v", errs)
	}
	if len(r.Stmts) != MaxRecipeConditions {
		t.Errorf("%d statements were read before the limit", len(r.Stmts))
	}
}

// TestRecipeIfInTheFile checks the file reader and the recipe reader together:
// a block with if and else lines is one block that closes at its own },
// and the lines reach the recipe reader as they are.
func TestRecipeIfInTheFile(t *testing.T) {
	src := "module example.com/app\n\ncxx example.com/lib {\n" +
		"\tsrc a.c\n" +
		"\tif windows {\n" +
		"\t\tlink gdi32\n" +
		"\t} else if linux {\n" +
		"\t\tlink GL\n" +
		"\t} else {\n" +
		"\t\tlink m\n" +
		"\t}\n" +
		"}\n" +
		"\n" +
		"cflags -DX\n"
	f, errs := Parse("gocpp.mod", []byte(src), nil)
	if len(errs) > 0 {
		t.Fatalf("Parse: %v", errs)
	}
	if len(f.Cflags) != 1 || len(f.Unknown) != 1 {
		t.Errorf("cflags %d, unknown %d: the block did not end at its own }", len(f.Cflags), len(f.Unknown))
	}
	blocks, rerrs := f.Recipes()
	if len(rerrs) > 0 {
		t.Fatal(rerrs)
	}
	b := blocks["example.com/lib"]
	if len(b.Raw) != 8 {
		t.Errorf("%d raw lines, want 8", len(b.Raw))
	}
	r, perrs := ParseRecipe("gocpp.mod", "example.com/lib", b)
	if len(perrs) > 0 {
		t.Fatal(perrs)
	}
	if got := applied(r, linuxAMD64); !reflect.DeepEqual(got, []string{"src a.c", "link -lGL"}) {
		t.Errorf("linux: %q", got)
	}
	if got := applied(r, target{"freebsd": true}); !reflect.DeepEqual(got, []string{"src a.c", "link -lm"}) {
		t.Errorf("freebsd: %q", got)
	}
	// Format writes the file back as it was (the lines of a recipe are kept verbatim), and
	// reading that gives the same recipe: what gocpp mod vendor copies is this text.
	out := Format(f.Syntax)
	if string(out) != src {
		t.Errorf("Format changed the file:\n%s", out)
	}
	f2, errs := Parse("gocpp.mod", out, nil)
	if len(errs) > 0 {
		t.Fatal(errs)
	}
	blocks2, _ := f2.Recipes()
	r2, _ := ParseRecipe("gocpp.mod", "example.com/lib", blocks2["example.com/lib"])
	if !reflect.DeepEqual(blocks2["example.com/lib"].Raw, b.Raw) || len(r2.Stmts) != len(r.Stmts) {
		t.Errorf("the recipe changed in the round trip")
	}
}

// TestRecipeIfUnclosedInTheFile checks the error of the file reader for an if
// block that is not closed: it names the brace that is open.
func TestRecipeIfUnclosedInTheFile(t *testing.T) {
	for _, tc := range []struct{ name, src, want string }{
		{
			"an if block with no }",
			"module m\ncxx a {\n\tif windows {\n\t\tlink x\n}\n",
			`gocpp.mod:6: syntax error (unterminated block started at gocpp.mod:2:1)`,
		},
		{
			"the recipe's } is taken by the if",
			"module m\ncxx a {\n\tsrc a.c\n\tif windows {\n\t\tlink x\n}\n",
			`gocpp.mod:7: syntax error (unterminated block started at gocpp.mod:2:1)`,
		},
		{
			"an else block with no }",
			"module m\ncxx a {\n\tif windows {\n\t} else {\n\t\tlink x\n",
			`gocpp.mod:6: syntax error (unterminated block started at gocpp.mod:2:1; ` +
				`the "{" at gocpp.mod:4:9 is not closed)`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, errs := Parse("gocpp.mod", []byte(tc.src), nil)
			if len(errs) != 1 || errs[0].Error() != tc.want {
				t.Errorf("errors %v\nwant %s", errs, tc.want)
			}
		})
	}
}

// TestRecipeIfErrorsThroughTheFile checks that the file reader lets these
// mistakes through, because it agrees with the recipe reader on where the
// blocks close, so that the error comes from the recipe reader at its line
// and column, not as an unterminated block of the whole file.
func TestRecipeIfErrorsThroughTheFile(t *testing.T) {
	for _, tc := range []struct {
		body      string
		line, col int
		want      string
	}{
		{"\tif linux {\n\t} src a.c\n", 5, 4, "after a } only else may follow on the line"},
		{"\tif linux {\n\t\tlink m }\n\t}\n", 5, 10, "blocks inside a recipe"},
		{"\tif linux {\n\t} else\n", 5, 4, "an else line ends with {"},
		{"\tif linux {\n\t} else {\n\t} else {\n\t}\n", 6, 4, "else after else"},
		{"\tif linux { link m }\n", 4, 20, "an if line ends with {"},
		{"\tif linux {\n\t}else {\n\t}\n", 5, 2, `a "}" is a word of its own`},
		{"\telse {\n\t}\n", 4, 2, `"else" without "if"`},
		{"\twindows: link m\n", 4, 2, "since ADR-129"},
		{"\tif linux {\n\t\tfrobnicate\n\t}\n", 5, 3, "unknown statement"},
		{"\tif {\n\t}\n", 4, 5, "without a condition"},
		{"\tif windows{\n\t\tlink gdi32\n\t}\n", 4, 5, `put a space before the "{"`},
		{
			"\tif windows{ // open\n\t\tlink gdi32\n\t}\n", 4, 5,
			`put a space before the "{"`,
		},
		{
			"\tif //go:build windows {\n\t\tlink gdi32\n\t}\n", 4, 2,
			`the "{" of an if line is not in a comment`,
		},
		{"\tif(windows) {\n\t\tlink gdi32\n\t}\n", 4, 2, `write a space after "if"`},
		{
			"\tif linux {\n\t} else{\n\t\tlink m\n\t}\n", 5, 4,
			`put a space before the "{" at the end of an else line`,
		},
		{
			"\tif linux {\n\t} else // {\n\t\tlink m\n\t}\n", 5, 4,
			`the "{" of an else line is not in a comment`,
		},
		{"\tif linux ||| darwin {\n\t}\n", 4, 13, "is not valid"},
	} {
		src := "module example.com/app\n\ncxx example.com/lib {\n" + tc.body + "}\n"
		f, errs := Parse("gocpp.mod", []byte(src), nil)
		if len(errs) > 0 {
			t.Errorf("%q: Parse reports %v", tc.body, errs)
			continue
		}
		blocks, rerrs := f.Recipes()
		if len(rerrs) > 0 || blocks["example.com/lib"] == nil {
			t.Errorf("%q: Recipes() = %v, %v", tc.body, blocks, rerrs)
			continue
		}
		_, perrs := ParseRecipe("gocpp.mod", "example.com/lib", blocks["example.com/lib"])
		if len(perrs) == 0 {
			t.Errorf("%q: no error", tc.body)
			continue
		}
		e := perrs[0]
		if e.Pos.Line != tc.line || e.Pos.LineRune != tc.col || !strings.Contains(e.Err, tc.want) {
			t.Errorf("%q: first error %v, want line %d column %d with %q",
				tc.body, e, tc.line, tc.col, tc.want)
		}
	}
}

func TestRecipeUnclosedOtherBlock(t *testing.T) {
	_, errs := rawRecipe("\tdefine {", "\t\tA=1")
	found := false
	for _, e := range errs {
		found = found || strings.Contains(e.Err, "the block is not closed: a } is missing")
		if strings.Contains(e.Err, "block block") {
			t.Errorf("error %q", e.Err)
		}
	}
	if !found {
		t.Errorf("errors: %v", errs)
	}
}

func TestRecipeTags(t *testing.T) {
	r, errs := recipeOf(t,
		"\tif windows && amd64 {",
		"\t} else if (linux || darwin) && !windows {",
		"\t} else if go1.18 || vulkan {",
		"\t}",
		"\tif linux {",
		"\t}",
		"\tif bad cond {", // not valid: its tags are not recorded
		"\t}",
	)
	if len(errs) != 1 {
		t.Fatalf("errors: %v", errs)
	}
	var got []string
	for _, tag := range r.Tags {
		got = append(got, fmt.Sprintf("%s@%d:%d", tag.Name, tag.Line, tag.Col))
	}
	want := []string{"windows@4:5", "amd64@4:16", "linux@5:13", "darwin@5:22", "go1.18@6:12", "vulkan@6:22"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tags %v, want %v", got, want)
	}
}

func TestRecipeErrorLimit(t *testing.T) {
	var body []string
	for range 1000 {
		body = append(body, "\tfrobnicate x")
	}
	r, errs := recipeOf(t, body...)
	if len(errs) != MaxRecipeErrors+1 ||
		!strings.Contains(errs[len(errs)-1].Err, "more than 100 errors in the recipe") {
		t.Fatalf("%d errors, the last: %v", len(errs), errs[len(errs)-1])
	}
	if len(r.Stmts) != 0 {
		t.Errorf("statements: %d", len(r.Stmts))
	}
	// Blocks that open and never close: the same limit.
	body = body[:0]
	for range 1000 {
		body = append(body, "\tx {")
	}
	if _, errs := rawRecipe(body...); len(errs) != MaxRecipeErrors+1 {
		t.Errorf("%d errors for 1000 blocks", len(errs))
	}
}

func TestRecipeLongConditionIsCut(t *testing.T) {
	_, errs := recipeOf(t, "\tif "+strings.Repeat("a ", 5000)+"{", "\t}")
	if len(errs) != 1 || len(errs[0].Err) > 400 || !strings.Contains(errs[0].Err, "...") {
		t.Errorf("errors: %d, first %.120q (%d bytes)", len(errs), errs[0].Err, len(errs[0].Err))
	}
}

// An error about a statement at the first column has that column; one about
// the whole line has none (LineRune 0).
func TestRecipeColumnOne(t *testing.T) {
	_, errs := rawRecipe("frobnicate x", "src "+strings.Repeat("a", MaxRecipeLine))
	if len(errs) != 2 || errs[0].Pos.LineRune != 1 || errs[1].Pos.LineRune != 0 {
		t.Errorf("columns: %v", errs)
	}
	if !strings.HasPrefix(errs[0].Error(), "gocpp.mod:4: cxx") {
		t.Errorf("Error() = %.30q: the go command's form leaves column 1 out", errs[0].Error())
	}
}
