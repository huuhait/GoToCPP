package main

import (
	"strings"
	"testing"
)

// parseFile is the input of TestParseDump: a header import, a doc comment
// and a dotted type name with a type argument.
const parseFile = `package main

import "<vector>"

// v is a vector.
var v std.vector[int]
`

// parseDump is what gocpp parse -dump prints for parseFile: go/ast's
// Print with the positions of the file (x.gp), the comments, and no
// object resolution.
const parseDump = `     0  *ast.File {
     1  .  Package: x.gp:1:1
     2  .  Name: *ast.Ident {
     3  .  .  NamePos: x.gp:1:9
     4  .  .  Name: "main"
     5  .  }
     6  .  Decls: []ast.Decl (len = 2) {
     7  .  .  0: *ast.GenDecl {
     8  .  .  .  TokPos: x.gp:3:1
     9  .  .  .  Tok: import
    10  .  .  .  Lparen: -
    11  .  .  .  Specs: []ast.Spec (len = 1) {
    12  .  .  .  .  0: *ast.ImportSpec {
    13  .  .  .  .  .  Path: *ast.BasicLit {
    14  .  .  .  .  .  .  ValuePos: x.gp:3:8
    15  .  .  .  .  .  .  Kind: STRING
    16  .  .  .  .  .  .  Value: "\"<vector>\""
    17  .  .  .  .  .  }
    18  .  .  .  .  .  EndPos: -
    19  .  .  .  .  }
    20  .  .  .  }
    21  .  .  .  Rparen: -
    22  .  .  }
    23  .  .  1: *ast.GenDecl {
    24  .  .  .  Doc: *ast.CommentGroup {
    25  .  .  .  .  List: []*ast.Comment (len = 1) {
    26  .  .  .  .  .  0: *ast.Comment {
    27  .  .  .  .  .  .  Slash: x.gp:5:1
    28  .  .  .  .  .  .  Text: "// v is a vector."
    29  .  .  .  .  .  }
    30  .  .  .  .  }
    31  .  .  .  }
    32  .  .  .  TokPos: x.gp:6:1
    33  .  .  .  Tok: var
    34  .  .  .  Lparen: -
    35  .  .  .  Specs: []ast.Spec (len = 1) {
    36  .  .  .  .  0: *ast.ValueSpec {
    37  .  .  .  .  .  Names: []*ast.Ident (len = 1) {
    38  .  .  .  .  .  .  0: *ast.Ident {
    39  .  .  .  .  .  .  .  NamePos: x.gp:6:5
    40  .  .  .  .  .  .  .  Name: "v"
    41  .  .  .  .  .  .  }
    42  .  .  .  .  .  }
    43  .  .  .  .  .  Type: *ast.IndexExpr {
    44  .  .  .  .  .  .  X: *ast.SelectorExpr {
    45  .  .  .  .  .  .  .  X: *ast.Ident {
    46  .  .  .  .  .  .  .  .  NamePos: x.gp:6:7
    47  .  .  .  .  .  .  .  .  Name: "std"
    48  .  .  .  .  .  .  .  }
    49  .  .  .  .  .  .  .  Sel: *ast.Ident {
    50  .  .  .  .  .  .  .  .  NamePos: x.gp:6:11
    51  .  .  .  .  .  .  .  .  Name: "vector"
    52  .  .  .  .  .  .  .  }
    53  .  .  .  .  .  .  }
    54  .  .  .  .  .  .  Lbrack: x.gp:6:17
    55  .  .  .  .  .  .  Index: *ast.Ident {
    56  .  .  .  .  .  .  .  NamePos: x.gp:6:18
    57  .  .  .  .  .  .  .  Name: "int"
    58  .  .  .  .  .  .  }
    59  .  .  .  .  .  .  Rbrack: x.gp:6:21
    60  .  .  .  .  .  }
    61  .  .  .  .  }
    62  .  .  .  }
    63  .  .  .  Rparen: -
    64  .  .  }
    65  .  }
    66  .  FileStart: x.gp:1:1
    67  .  FileEnd: x.gp:6:23
    68  .  Imports: []*ast.ImportSpec (len = 1) {
    69  .  .  0: *(obj @ 12)
    70  .  }
    71  .  Comments: []*ast.CommentGroup (len = 1) {
    72  .  .  0: *(obj @ 24)
    73  .  }
    74  .  GoVersion: ""
    75  }
`

func TestParseDump(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"x.gp": parseFile})
	t.Chdir(dir)
	code, out, errOut := gocpp("parse", "-dump", "x.gp")
	if r := (result{code, out, errOut}); r != (result{0, parseDump, ""}) {
		t.Errorf("gocpp parse -dump x.gp:\n%s\nwant:\n%s", show(r), parseDump)
	}
	// Without -dump, it only parses.
	code, out, errOut = gocpp("parse", "x.gp")
	if r := (result{code, out, errOut}); r != (result{}) {
		t.Errorf("gocpp parse x.gp:\n%s\nwant exit 0 and no output", show(r))
	}
}

// TestParseErrors checks files with syntax errors and files that can't be
// read: gocpp parse prints the errors, dumps the other files and exits with
// status 1.
func TestParseErrors(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"x.gp":   parseFile,
		"bad.gp": "package main\n\nfunc f() {\n\tx := )\n\ty := ]\n}\n",
	})
	t.Chdir(dir)
	code, out, errOut := gocpp("parse", "-dump", "bad.gp", "missing.gp", "x.gp")
	errOut = portable(errOut)
	want := result{1, parseDump, "bad.gp:4:7: expected operand, found ')'\nbad.gp:6:3: expected ';', found 'EOF'\n" +
		"open missing.gp: no such file or directory\n"}
	if r := (result{code, out, errOut}); r != want {
		t.Errorf("gocpp parse -dump bad.gp missing.gp x.gp:\n%s\nwant:\n%s", show(r), show(want))
	}
}

func TestParseUsage(t *testing.T) {
	for _, args := range [][]string{{"parse"}, {"parse", "-dump"}} {
		code, out, errOut := gocpp(args...)
		if code != 2 || out != "" || errOut != "usage: gocpp parse [-dump] file.gp ...\n" {
			t.Errorf("gocpp %s: exit %d, stdout %q, stderr %q; want exit 2 and the usage line",
				strings.Join(args, " "), code, out, errOut)
		}
	}
	code, out, errOut := gocpp("parse", "-h")
	if code != 0 || out != "" || !strings.HasPrefix(errOut, parseUsage) || !strings.Contains(errOut, "-dump") {
		t.Errorf("gocpp parse -h: exit %d, stdout %q, stderr:\n%s\nwant exit 0 and the usage with the flags", code, out, errOut)
	}
	code, out, errOut = gocpp("parse", "-x", "x.gp")
	if code != 2 || out != "" || !strings.HasPrefix(errOut, "flag provided but not defined: -x\n"+parseUsage) {
		t.Errorf("gocpp parse -x x.gp: exit %d, stdout %q, stderr:\n%s\nwant exit 2, the error and the usage", code, out, errOut)
	}
}

// TestParsePhase10Syntax: gocpp parse reads the syntax of Phase 10 (a must function and call, a macro
// declaration with a lazy parameter, a macro use and a line of macros), with no switch (ADR-095).
func TestParsePhase10Syntax(t *testing.T) {
	const src = "package main\n\nmacro Assert(cond lazy bool) {}\n\n#Trace()\nfunc f!() int {\n\t#Assert(true)\n\treturn g!()\n}\n\nfunc g() (int, error) { return 0, nil }\n"
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"x.gp": src})
	t.Chdir(dir)
	code, out, errOut := gocpp("parse", "x.gp")
	if r := (result{code, out, errOut}); r != (result{}) {
		t.Errorf("gocpp parse x.gp:\n%s\nwant exit 0 and no output", show(r))
	}
	code, out, errOut = gocpp("parse", "-dump", "x.gp")
	if code != 0 || errOut != "" || !strings.Contains(out, `"f!"`) || !strings.Contains(out, `"#Assert"`) {
		t.Errorf("gocpp parse -dump x.gp: exit %d, stderr %q, stdout:\n%s", code, errOut, out)
	}
}
