package load

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/diag"
)

// readDirectives parses src as main.gp with the stock go/parser (the
// parser fork gives the same go/ast nodes) and returns what
// directiveStrings returns. A syntax error in src fails the test.
func readDirectives(t *testing.T, src string) (dirs, errs []string) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return directiveStrings(fset, f)
}

// directiveStrings returns the directives of f as "line:col name [args]",
// and its errors and warnings as "line:col: message" and
// "line:col: warning: message", the way diag's Printer shows them.
func directiveStrings(fset *token.FileSet, f *ast.File) (dirs, errs []string) {
	var list diag.List
	for _, d := range ParseDirectives(fset, f, &list) {
		pos := fset.Position(d.Pos)
		dirs = append(dirs, fmt.Sprintf("%d:%d %s %q", pos.Line, pos.Column, d.Name, d.Args))
	}
	for _, d := range list.Diagnostics() {
		kind := ""
		if d.Severity == diag.Warning {
			kind = "warning: "
		}
		errs = append(errs, fmt.Sprintf("%d:%d: %s%s", d.Pos.Line, d.Pos.Column, kind, d.Msg))
	}
	return dirs, errs
}

// A directiveTest is a source and the directives and errors it gives.
type directiveTest struct {
	name string
	src  string
	dirs []string
	errs []string
}

func runDirectiveTests(t *testing.T, tests []directiveTest) {
	t.Helper()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dirs, errs := readDirectives(t, tt.src)
			if !slices.Equal(dirs, tt.dirs) {
				t.Errorf("directives:\n got: %s\nwant: %s", strings.Join(dirs, "\n      "), strings.Join(tt.dirs, "\n      "))
			}
			if !slices.Equal(errs, tt.errs) {
				t.Errorf("errors:\n got: %s\nwant: %s", strings.Join(errs, "\n      "), strings.Join(tt.errs, "\n      "))
			}
		})
	}
}

// beforePackage returns the lines, then "package p".
func beforePackage(lines ...string) string {
	return strings.Join(lines, "\n") + "\npackage p\n"
}

func TestDirectiveForms(t *testing.T) {
	runDirectiveTests(t, []directiveTest{
		{
			name: "pkg-config",
			src:  beforePackage("//gocpp:pkg-config sdl2"),
			dirs: []string{`1:1 pkg-config ["sdl2"]`},
		},
		{
			name: "pkg-config several",
			src:  beforePackage("//gocpp:pkg-config sdl2 SDL2_image gtk+-3.0 libpng16"),
			dirs: []string{`1:1 pkg-config ["sdl2" "SDL2_image" "gtk+-3.0" "libpng16"]`},
		},
		{
			name: "link",
			src:  beforePackage("//gocpp:link m pthread"),
			dirs: []string{`1:1 link ["m" "pthread"]`},
		},
		{
			name: "link names with + - _ .",
			src:  beforePackage("//gocpp:link stdc++ boost_system-mt GL foo.1"),
			dirs: []string{`1:1 link ["stdc++" "boost_system-mt" "GL" "foo.1"]`},
		},
		{
			name: "cflags",
			src:  beforePackage("//gocpp:cflags -DUSE_FAST_MATH -O2 -I/opt/foo/include"),
			dirs: []string{`1:1 cflags ["-DUSE_FAST_MATH" "-O2" "-I/opt/foo/include"]`},
		},
		{
			name: "ldflags",
			src:  beforePackage("//gocpp:ldflags -L/opt/foo/lib -Wl,-rpath,/opt/foo/lib"),
			dirs: []string{`1:1 ldflags ["-L/opt/foo/lib" "-Wl,-rpath,/opt/foo/lib"]`},
		},
		{
			name: "flags are kept as written",
			src:  beforePackage(`//gocpp:cflags -I /opt/x +x 1 -Wl,--as-needed`),
			dirs: []string{`1:1 cflags ["-I" "/opt/x" "+x" "1" "-Wl,--as-needed"]`},
		},
		{
			name: "extern-c system header",
			src:  beforePackage("//gocpp:extern-c <lua.h>"),
			dirs: []string{`1:1 extern-c ["<lua.h>"]`},
		},
		{
			name: "extern-c local header",
			src:  beforePackage("//gocpp:extern-c ./legacy.h"),
			dirs: []string{`1:1 extern-c ["./legacy.h"]`},
		},
		{
			name: "headers of every kind",
			src: beforePackage(
				"//gocpp:extern-c <SDL2/SDL.h>",
				"//gocpp:extern-c lua.h",
				"//gocpp:extern-c native/x.hh",
				"//gocpp:extern-c ../include/y.hpp",
				"//gocpp:extern-c /abs/z.hxx",
				"//gocpp:extern-c ./w.h++",
				"//gocpp:extern-c ./v.inl",
				"//gocpp:no-extern-c <vector>",
			),
			dirs: []string{
				`1:1 extern-c ["<SDL2/SDL.h>"]`,
				`2:1 extern-c ["lua.h"]`,
				`3:1 extern-c ["native/x.hh"]`,
				`4:1 extern-c ["../include/y.hpp"]`,
				`5:1 extern-c ["/abs/z.hxx"]`,
				`6:1 extern-c ["./w.h++"]`,
				`7:1 extern-c ["./v.inl"]`,
				`8:1 no-extern-c ["<vector>"]`,
			},
		},
		{
			name: "no-extern-c",
			src:  beforePackage("//gocpp:no-extern-c ./cwrap.h"),
			dirs: []string{`1:1 no-extern-c ["./cwrap.h"]`},
		},
		{
			name: "no-win32-defaults",
			src:  beforePackage("//gocpp:no-win32-defaults"),
			dirs: []string{`1:1 no-win32-defaults []`},
		},
		{
			name: "spaces and tabs",
			src:  beforePackage("//gocpp:link\tm  \t pthread \t", "//gocpp:no-win32-defaults \t"),
			dirs: []string{`1:1 link ["m" "pthread"]`, `2:1 no-win32-defaults []`},
		},
		{
			// As in a #cgo line, all Unicode white space separates, also a
			// no-break space copied from a web page, and it ends the name.
			name: "other white space",
			src:  beforePackage("//gocpp:link\u00a0m\u2003pthread\vz\fx"),
			dirs: []string{`1:1 link ["m" "pthread" "z" "x"]`},
		},
		{
			name: "CRLF line endings",
			src:  "//gocpp:link m pthread\r\n//gocpp:link -lz\r\npackage p\r\n",
			dirs: []string{`1:1 link ["m" "pthread"]`},
			errs: []string{`2:14: invalid library name "-lz": write z, without -l`},
		},
		{
			// Every line is a directive of its own; the build adds them up.
			name: "repeated directives",
			src: beforePackage(
				"//gocpp:link m",
				"//gocpp:no-win32-defaults",
				"//gocpp:link pthread m",
				"//gocpp:no-win32-defaults",
			),
			dirs: []string{
				`1:1 link ["m"]`,
				`2:1 no-win32-defaults []`,
				`3:1 link ["pthread" "m"]`,
				`4:1 no-win32-defaults []`,
			},
		},
	})
}

func TestDirectiveArgErrors(t *testing.T) {
	const headerWant = `want <name> or a path ending in .h, .hh, .hpp, .hxx, .h++ or .inl`
	runDirectiveTests(t, []directiveTest{
		// The number of arguments. The error is at the directive.
		{name: "pkg-config without packages", src: beforePackage("//gocpp:pkg-config"), errs: []string{`1:1: usage: //gocpp:pkg-config package...`}},
		{name: "pkg-config with only spaces", src: beforePackage("//gocpp:pkg-config \t "), errs: []string{`1:1: usage: //gocpp:pkg-config package...`}},
		{name: "link without libraries", src: beforePackage("//gocpp:link"), errs: []string{`1:1: usage: //gocpp:link library...`}},
		{name: "cflags without flags", src: beforePackage("//gocpp:cflags"), errs: []string{`1:1: usage: //gocpp:cflags flag...`}},
		{name: "ldflags without flags", src: beforePackage("//gocpp:ldflags "), errs: []string{`1:1: usage: //gocpp:ldflags flag...`}},
		{name: "extern-c without a header", src: beforePackage("//gocpp:extern-c"), errs: []string{`1:1: usage: //gocpp:extern-c header`}},
		{name: "extern-c with two headers", src: beforePackage("//gocpp:extern-c <lua.h> <lauxlib.h>"), errs: []string{`1:1: usage: //gocpp:extern-c header`}},
		{name: "no-extern-c without a header", src: beforePackage("//gocpp:no-extern-c"), errs: []string{`1:1: usage: //gocpp:no-extern-c header`}},
		{name: "no-extern-c with two headers", src: beforePackage("//gocpp:no-extern-c a.h b.h"), errs: []string{`1:1: usage: //gocpp:no-extern-c header`}},
		{name: "no-win32-defaults with an argument", src: beforePackage("//gocpp:no-win32-defaults yes"), errs: []string{`1:1: usage: //gocpp:no-win32-defaults`}},

		// pkg-config package names. The error is at the argument.
		{
			name: "pkg-config flag",
			src:  beforePackage("//gocpp:pkg-config --static sdl2"),
			errs: []string{`1:20: invalid pkg-config package name "--static": flags go in //gocpp:cflags or //gocpp:ldflags`},
		},
		{
			name: "pkg-config version",
			src:  beforePackage("//gocpp:pkg-config sdl2 sdl2>=2.0"),
			errs: []string{`1:25: invalid character '>' in pkg-config package name "sdl2>=2.0"`},
		},
		{
			name: "pkg-config file path",
			src:  beforePackage("//gocpp:pkg-config ./third_party/foo.pc"),
			errs: []string{`1:20: invalid character '/' in pkg-config package name "./third_party/foo.pc"`},
		},
		{
			name: "pkg-config response file",
			src:  beforePackage("//gocpp:pkg-config @args"),
			errs: []string{`1:20: invalid character '@' in pkg-config package name "@args"`},
		},

		// Library names.
		{
			name: "link -lm",
			src:  beforePackage("//gocpp:link -lm"),
			errs: []string{`1:14: invalid library name "-lm": write m, without -l`},
		},
		{
			name: "link flag",
			src:  beforePackage("//gocpp:link -pthread"),
			errs: []string{`1:14: invalid library name "-pthread": flags go in //gocpp:ldflags`},
		},
		{
			name: "link -l alone",
			src:  beforePackage("//gocpp:link -l"),
			errs: []string{`1:14: invalid library name "-l": flags go in //gocpp:ldflags`},
		},
		{
			name: "link -l with a bad name",
			src:  beforePackage("//gocpp:link -l-lm -lx/y"),
			errs: []string{
				`1:14: invalid library name "-l-lm": flags go in //gocpp:ldflags`,
				`1:20: invalid library name "-lx/y": flags go in //gocpp:ldflags`,
			},
		},
		{
			name: "link with a comma",
			src:  beforePackage("//gocpp:link m,pthread"),
			errs: []string{`1:14: invalid character ',' in library name "m,pthread"`},
		},
		{
			name: "link with a quoted space",
			src:  beforePackage(`//gocpp:link "m pthread"`),
			errs: []string{`1:14: invalid character ' ' in library name "m pthread"`},
		},
		{
			// A column counts bytes: the no-break space has two.
			name: "column after a no-break space",
			src:  beforePackage("//gocpp:link m\u00a0-lz"),
			errs: []string{`1:17: invalid library name "-lz": write z, without -l`},
		},
		{
			// %q shows a character that can't be seen, like a zero-width space
			// (which is not white space).
			name: "link with an invisible character",
			src:  beforePackage("//gocpp:link m\u200bpthread"),
			errs: []string{`1:14: invalid character '\u200b' in library name "m\u200bpthread"`},
		},

		// Headers. The check is the one of the header imports
		// (checkHeader), so the messages are those of ClassifyImports.
		{name: "header without <", src: beforePackage("//gocpp:extern-c lua.h>"), errs: []string{`1:18: invalid header "lua.h>": missing < at the start`}},
		{name: "header without >", src: beforePackage("//gocpp:extern-c <lua.h"), errs: []string{`1:18: invalid header "<lua.h": missing > at the end`}},
		{name: "empty <>", src: beforePackage("//gocpp:extern-c <>"), errs: []string{`1:18: invalid header "<>": missing file name`}},
		{name: "nested <>", src: beforePackage("//gocpp:extern-c <<lua.h>>"), errs: []string{`1:18: invalid header "<<lua.h>>": invalid character U+003C '<'`}},
		{name: "space after <", src: beforePackage(`//gocpp:extern-c "< lua.h>"`), errs: []string{`1:18: invalid header "< lua.h>": space after <`}},
		{name: "space before >", src: beforePackage(`//gocpp:extern-c "<lua.h >"`), errs: []string{`1:18: invalid header "<lua.h >": space before >`}},
		{name: "space at the start", src: beforePackage(`//gocpp:extern-c " ./a.h"`), errs: []string{`1:18: invalid header " ./a.h": space at the start`}},
		{name: "space at the end", src: beforePackage(`//gocpp:no-extern-c "<lua.h> "`), errs: []string{`1:21: invalid header "<lua.h> ": space at the end`}},
		{name: "backslash", src: beforePackage(`//gocpp:extern-c sub\\x.h`), errs: []string{`1:18: invalid header "sub\\x.h": use / in header paths`}},
		{name: "header without extension", src: beforePackage("//gocpp:extern-c lua"), errs: []string{`1:18: invalid header "lua": ` + headerWant}},
		{name: "C source, not a header", src: beforePackage("//gocpp:extern-c ./legacy.c"), errs: []string{`1:18: invalid header "./legacy.c": ` + headerWant}},
		{name: "upper-case extension", src: beforePackage("//gocpp:extern-c ./legacy.H"), errs: []string{`1:18: invalid header "./legacy.H": ` + headerWant}},
		{name: "namespace, not a header", src: beforePackage("//gocpp:extern-c ::std"), errs: []string{`1:18: invalid header "::std": ` + headerWant}},
		{name: "quote in a local header", src: beforePackage(`//gocpp:no-extern-c ./a\"b.h`), errs: []string{`1:21: invalid header "./a\"b.h": invalid character U+0022 '"'`}},
		// Headers that the import rules accept, so the directives accept
		// them too: a file named .h (a header without a package name when
		// it is imported), and a > in a local header.
		{name: "only an extension", src: beforePackage("//gocpp:extern-c ./.h"), dirs: []string{`1:1 extern-c ["./.h"]`}},
		{name: "> in a local header", src: beforePackage("//gocpp:extern-c ./a>b.h"), dirs: []string{`1:1 extern-c ["./a>b.h"]`}},

		{
			// Every bad argument is reported, at its own column.
			name: "several bad arguments",
			src:  beforePackage("//gocpp:link -lm m x/y -pthread"),
			errs: []string{
				`1:14: invalid library name "-lm": write m, without -l`,
				`1:20: invalid character '/' in library name "x/y"`,
				`1:24: invalid library name "-pthread": flags go in //gocpp:ldflags`,
			},
		},
		{
			// A directive with a mistake is left out; the others stay.
			name: "bad line between good ones",
			src:  beforePackage("//gocpp:link m", "//gocpp:link -lz", "//gocpp:link pthread"),
			dirs: []string{`1:1 link ["m"]`, `3:1 link ["pthread"]`},
			errs: []string{`2:14: invalid library name "-lz": write z, without -l`},
		},
	})
}

func TestDirectivePlacement(t *testing.T) {
	const misplaced = "//gocpp: directives must come before the package clause"
	runDirectiveTests(t, []directiveTest{
		{
			// gofmt moves directives to the end of a doc comment.
			name: "in the package doc comment",
			src:  beforePackage("// Package p does things.", "//", "//gocpp:link m"),
			dirs: []string{`3:1 link ["m"]`},
		},
		{
			name: "in the middle of the doc comment",
			src:  beforePackage("// Package p does things.", "//gocpp:link m", "// More docs."),
			dirs: []string{`2:1 link ["m"]`},
		},
		{
			name: "in a comment of its own",
			src:  beforePackage("//gocpp:link m", "", "// Package p does things."),
			dirs: []string{`1:1 link ["m"]`},
		},
		{
			// The header of examples/sqlite/main.gp.
			name: "like an example",
			src: "// phase: 04\n// pkg-config: sqlite3\n\n" +
				"// Sqlite keeps a small table in an in-memory database.\n//\n" +
				"//gocpp:pkg-config sqlite3\npackage main\n",
			dirs: []string{`6:1 pkg-config ["sqlite3"]`},
		},
		{
			name: "next to //go:build",
			src:  beforePackage("//go:build linux", "", "//gocpp:link m"),
			dirs: []string{`3:1 link ["m"]`},
		},
		{
			name: "indented",
			src:  beforePackage("\t//gocpp:link m"),
			dirs: []string{`1:2 link ["m"]`},
		},
		{
			name: "after a /* */ comment on the same line",
			src:  beforePackage("/* x */ //gocpp:link m"),
			dirs: []string{`1:9 link ["m"]`},
		},
		{
			name: "on the line of the package clause",
			src:  "package p //gocpp:link m\n",
			errs: []string{"1:11: " + misplaced},
		},
		{
			// After the package clause, only the placement is reported:
			// the misspelled name on line 6 gets no second error.
			name: "after the package clause",
			src: "package p\n" +
				"\n" +
				"//gocpp:link m\n" + // 3: above an import, where cgo puts its #cgo lines
				"import \"fmt\"\n" +
				"\n" +
				"//gocpp:lnk m\n" + // 6
				"func f() {\n" +
				"\t//gocpp:cflags -O2\n" + // 8
				"\tfmt.Println()\n" +
				"}\n" +
				"\n" +
				"//gocpp:no-win32-defaults\n", // 12
			errs: []string{"3:1: " + misplaced, "6:1: " + misplaced, "8:2: " + misplaced, "12:1: " + misplaced},
		},
		{
			// Not a directive, but before the package clause it gets a
			// warning (TestDirectiveLookalikes).
			name: "space after //",
			src:  beforePackage("// gocpp:link m", "//\tgocpp:link m") + "// gocpp:link m\n",
			errs: []string{
				`1:1: warning: "// gocpp:link" is an ordinary comment, not a directive: write //gocpp:link, with no space after //`,
				`2:1: warning: "//\tgocpp:link" is an ordinary comment, not a directive: write //gocpp:link, with no space after //`,
			},
		},
		{
			name: "/* */ comments",
			src:  beforePackage("/*gocpp:link m*/", "/*", "//gocpp:link m", "*/") + "/*\n//gocpp:link m\n*/\n",
		},
		{
			name: "commented out",
			src:  beforePackage("////gocpp:link m", "// //gocpp:link m"),
		},
		{
			name: "not at the start of the comment",
			src:  beforePackage("// See //gocpp:link m."),
		},
	})
}

func TestDirectiveQuoting(t *testing.T) {
	runDirectiveTests(t, []directiveTest{
		{
			// The example in the doc comment of go/build's splitQuoted.
			name: "go/build's example",
			src:  beforePackage(`//gocpp:cflags a b:"c d" 'e''f'  "g\""`),
			dirs: []string{`1:1 cflags ["a" "b:c d" "ef" "g\""]`},
		},
		{
			name: "double and single quotes",
			src:  beforePackage("//gocpp:cflags \"-DGREETING=hello world\" '-DNAME=a\tb' -O2"),
			dirs: []string{`1:1 cflags ["-DGREETING=hello world" "-DNAME=a\tb" "-O2"]`},
		},
		{
			// Quotes count in the middle of an argument too, and they are
			// removed, as in a #cgo line.
			name: "quotes inside an argument",
			src:  beforePackage(`//gocpp:cflags -DMSG="hello world" "a b"c x'y z' "a""b"`),
			dirs: []string{`1:1 cflags ["-DMSG=hello world" "a bc" "xy z" "ab"]`},
		},
		{
			name: "the other quote inside",
			src:  beforePackage(`//gocpp:cflags 'say "hi"' "it's"`),
			dirs: []string{`1:1 cflags ["say \"hi\"" "it's"]`},
		},
		{
			// To keep a quote, escape it or put it inside the other quotes.
			name: "a C string in a define",
			src:  beforePackage(`//gocpp:cflags -DNAME=\"x\" -DNAME='"x"'`),
			dirs: []string{`1:1 cflags ["-DNAME=\"x\"" "-DNAME=\"x\""]`},
		},
		{
			// A backslash makes the next character an ordinary one, inside
			// quotes too, so a Windows path needs \\ (or /).
			name: "backslashes",
			src:  beforePackage(`//gocpp:cflags a\ b \"x\" C:\dir C:\\dir 'it\'s' "a\"b" \a`),
			dirs: []string{`1:1 cflags ["a b" "\"x\"" "C:dir" "C:\\dir" "it's" "a\"b" "a"]`},
		},
		{
			name: "quotes next to text are not empty",
			src:  beforePackage(`//gocpp:cflags a"" ""b`),
			dirs: []string{`1:1 cflags ["a" "b"]`},
		},
		{
			name: "tab after a closing quote",
			src:  beforePackage("//gocpp:cflags \"a b\"\t-g"),
			dirs: []string{`1:1 cflags ["a b" "-g"]`},
		},
		{
			name: "quoted names and headers",
			src:  beforePackage(`//gocpp:link "m" 'pthread'`, `//gocpp:extern-c "./legacy.h"`, `//gocpp:extern-c '<lua.h>'`),
			dirs: []string{`1:1 link ["m" "pthread"]`, `2:1 extern-c ["./legacy.h"]`, `3:1 extern-c ["<lua.h>"]`},
		},
		{
			// The error is at the quote that is not closed.
			name: "unterminated double quote",
			src:  beforePackage(`//gocpp:cflags -O2 "-DX=a b`),
			errs: []string{`1:20: unterminated quote in //gocpp:cflags`},
		},
		{
			name: "unterminated quote inside an argument",
			src:  beforePackage(`//gocpp:cflags -DMSG="hello`),
			errs: []string{`1:22: unterminated quote in //gocpp:cflags`},
		},
		{
			name: "unterminated single quote",
			src:  beforePackage(`//gocpp:link 'm`),
			errs: []string{`1:14: unterminated quote in //gocpp:link`},
		},
		{
			name: "the other quote doesn't close",
			src:  beforePackage(`//gocpp:cflags "abc'`),
			errs: []string{`1:16: unterminated quote in //gocpp:cflags`},
		},
		{
			name: "escaped quote doesn't close",
			src:  beforePackage(`//gocpp:cflags "abc\"`),
			errs: []string{`1:16: unterminated quote in //gocpp:cflags`},
		},
		{
			name: "backslash at the end",
			src:  beforePackage(`//gocpp:cflags -DX=a\`),
			errs: []string{`1:21: backslash at the end of the line in //gocpp:cflags`},
		},
		{
			// As in go/build, the open quote is reported, not the backslash.
			name: "open quote and backslash at the end",
			src:  beforePackage(`//gocpp:cflags "abc\`),
			errs: []string{`1:16: unterminated quote in //gocpp:cflags`},
		},
		{
			name: "empty argument",
			src:  beforePackage(`//gocpp:cflags -O2 "" -g`),
			errs: []string{`1:20: empty argument in //gocpp:cflags`},
		},
		{
			name: "empty argument at the end",
			src:  beforePackage(`//gocpp:link ''`),
			errs: []string{`1:14: empty argument in //gocpp:link`},
		},
		{
			name: "only quotes",
			src:  beforePackage(`//gocpp:cflags ""'' -O2`),
			errs: []string{`1:16: empty argument in //gocpp:cflags`},
		},
		{
			// Only the first quoting mistake of a directive is reported.
			name: "first mistake only",
			src:  beforePackage(`//gocpp:cflags "" "x`),
			errs: []string{`1:16: empty argument in //gocpp:cflags`},
		},
	})
}

func TestDirectiveComments(t *testing.T) {
	runDirectiveTests(t, []directiveTest{
		{
			name: "comment after the arguments",
			src:  beforePackage("//gocpp:cflags -O2 // fast", "//gocpp:link m pthread //no space after the slashes"),
			dirs: []string{`1:1 cflags ["-O2"]`, `2:1 link ["m" "pthread"]`},
		},
		{
			// So tests/errors can put the error it wants at the end of a
			// directive line.
			name: "ERROR comment",
			src:  beforePackage(`//gocpp:link m // ERROR "invalid library name"`, `//gocpp:link -lz // ERROR "invalid library name"`),
			dirs: []string{`1:1 link ["m"]`},
			errs: []string{`2:14: invalid library name "-lz": write z, without -l`},
		},
		{
			name: "only a name and a comment",
			src:  beforePackage("//gocpp:no-win32-defaults // why", "//gocpp:no-win32-defaults //"),
			dirs: []string{`1:1 no-win32-defaults []`, `2:1 no-win32-defaults []`},
		},
		{
			name: "a comment instead of the arguments",
			src:  beforePackage("//gocpp:link // m"),
			errs: []string{`1:1: usage: //gocpp:link library...`},
		},
		{
			name: "quotes in the comment",
			src:  beforePackage(`//gocpp:cflags -O2 // it's "fast`),
			dirs: []string{`1:1 cflags ["-O2"]`},
		},
		{
			// A "//" that is quoted, escaped, or not at the start of an
			// argument is not a comment.
			name: "not a comment",
			src:  beforePackage(`//gocpp:cflags "//x" '//y' \//z -I//server/share ""//w a\ //b "a // b"`),
			dirs: []string{`1:1 cflags ["//x" "//y" "//z" "-I//server/share" "//w" "a //b" "a // b"]`},
		},
		{
			// The comment must be an argument of its own.
			name: "no space after the name",
			src:  beforePackage("//gocpp:link// m"),
			errs: []string{`1:1: unknown //gocpp: directive "link//"; did you mean //gocpp:link?`},
		},
	})
}

func TestDirectiveNames(t *testing.T) {
	const missing = "missing directive name after //gocpp:"
	runDirectiveTests(t, []directiveTest{
		{
			name: "pkgconfig",
			src:  beforePackage("//gocpp:pkgconfig sdl2"),
			errs: []string{`1:1: unknown //gocpp: directive "pkgconfig"; did you mean //gocpp:pkg-config?`},
		},
		{
			name: "pkg_config",
			src:  beforePackage("//gocpp:pkg_config sdl2"),
			errs: []string{`1:1: unknown //gocpp: directive "pkg_config"; did you mean //gocpp:pkg-config?`},
		},
		{
			name: "lnk",
			src:  beforePackage("//gocpp:lnk m"),
			errs: []string{`1:1: unknown //gocpp: directive "lnk"; did you mean //gocpp:link?`},
		},
		{
			name: "swapped letters",
			src:  beforePackage("//gocpp:lnik m"),
			errs: []string{`1:1: unknown //gocpp: directive "lnik"; did you mean //gocpp:link?`},
		},
		{
			// Names are lower case, but the suggestion ignores case.
			name: "upper case",
			src:  beforePackage("//gocpp:Link m"),
			errs: []string{`1:1: unknown //gocpp: directive "Link"; did you mean //gocpp:link?`},
		},
		{
			name: "cgo style",
			src:  beforePackage("//gocpp:LDFLAGS: -lm"),
			errs: []string{`1:1: unknown //gocpp: directive "LDFLAGS:"; did you mean //gocpp:ldflags?`},
		},
		{
			name: "externc",
			src:  beforePackage("//gocpp:externc <lua.h>"),
			errs: []string{`1:1: unknown //gocpp: directive "externc"; did you mean //gocpp:extern-c?`},
		},
		{
			name: "noexternc",
			src:  beforePackage("//gocpp:noexternc <lua.h>"),
			errs: []string{`1:1: unknown //gocpp: directive "noexternc"; did you mean //gocpp:no-extern-c?`},
		},
		{
			name: "no-win32-default",
			src:  beforePackage("//gocpp:no-win32-default"),
			errs: []string{`1:1: unknown //gocpp: directive "no-win32-default"; did you mean //gocpp:no-win32-defaults?`},
		},
		{
			name: "cflag",
			src:  beforePackage("//gocpp:cflag -O2"),
			errs: []string{`1:1: unknown //gocpp: directive "cflag"; did you mean //gocpp:cflags?`},
		},
		{
			name: "two names equally close",
			src:  beforePackage("//gocpp:lflags -O2"),
			errs: []string{`1:1: unknown //gocpp: directive "lflags"; did you mean //gocpp:cflags or //gocpp:ldflags?`},
		},
		{
			name: "far from every name",
			src:  beforePackage("//gocpp:frobnicate", "//gocpp:x"),
			errs: []string{`1:1: unknown //gocpp: directive "frobnicate"`, `2:1: unknown //gocpp: directive "x"`},
		},
		{
			// A zero-width space is not white space, so it is part of the name.
			name: "name with an invisible character",
			src:  beforePackage("//gocpp:link\u200bm"),
			errs: []string{`1:1: unknown //gocpp: directive "link\u200bm"; did you mean //gocpp:link?`},
		},
		{
			name: "no name",
			src:  beforePackage("//gocpp:"),
			errs: []string{"1:1: " + missing},
		},
		{
			name: "space after the colon",
			src:  beforePackage("//gocpp: link m", "//gocpp:\tcflags -O2", "//gocpp: hello world"),
			errs: []string{
				"1:1: " + missing + "; did you mean //gocpp:link?",
				"2:1: " + missing + "; did you mean //gocpp:cflags?",
				"3:1: " + missing,
			},
		},
	})
}

func TestDirectiveLookalikes(t *testing.T) {
	const noSpace = ", with no space after //"
	runDirectiveTests(t, []directiveTest{
		{
			name: "space after //",
			src:  beforePackage("// gocpp:link m", "//   gocpp:pkg-config sdl2"),
			errs: []string{
				`1:1: warning: "// gocpp:link" is an ordinary comment, not a directive: write //gocpp:link` + noSpace,
				`2:1: warning: "//   gocpp:pkg-config" is an ordinary comment, not a directive: write //gocpp:pkg-config` + noSpace,
			},
		},
		{
			// The name gets the same suggestions as a misspelled directive.
			name: "misspelled name",
			src:  beforePackage("// gocpp:Link pthread", "// gocpp:lflags -O2"),
			errs: []string{
				`1:1: warning: "// gocpp:Link" is an ordinary comment, not a directive: write //gocpp:link` + noSpace,
				`2:1: warning: "// gocpp:lflags" is an ordinary comment, not a directive: write //gocpp:cflags or //gocpp:ldflags` + noSpace,
			},
		},
		{
			// "gocpp:" in another letter case, with or without a space.
			name: "letter case of gocpp",
			src:  beforePackage("//GOCPP:link m", "// GoCpp:pkgconfig sdl2"),
			errs: []string{
				`1:1: warning: "//GOCPP:link" is an ordinary comment, not a directive: write //gocpp:link`,
				`2:1: warning: "// GoCpp:pkgconfig" is an ordinary comment, not a directive: write //gocpp:pkg-config` + noSpace,
			},
		},
		{
			name: "in the middle of the doc comment",
			src:  beforePackage("// Package p does things.", "// gocpp:no-win32-defaults", "// More docs."),
			errs: []string{`2:1: warning: "// gocpp:no-win32-defaults" is an ordinary comment, not a directive: write //gocpp:no-win32-defaults` + noSpace},
		},
		{
			// No warning: the name is far from every directive, there is no
			// name, "gocpp:" is not at the start, the directive is commented
			// out on purpose, or the comment is a /* */ comment.
			name: "ordinary comments",
			src: beforePackage(
				"// gocpp:frobnicate",
				"// gocpp: link m",
				"// gocpp:",
				"// See gocpp:link.",
				"// //gocpp:link m",
				"////gocpp:link m",
				"/* gocpp:link m */",
				"/*",
				"gocpp:link m",
				"*/",
			),
		},
		{
			// After the package clause, an ordinary comment is not checked.
			name: "after the package clause",
			src:  "package p\n\n// gocpp:link m\nfunc f() {} // gocpp:link m\n",
		},
	})
}

// TestDirectiveLookalikeAfterGofmt checks the case that the warning is for:
// gofmt (go/format) keeps only well-formed directives in a doc comment as
// they are. It moves them to the end of the doc comment and turns a
// misspelled one into an ordinary comment, which then gets the warning
// instead of the error.
func TestDirectiveLookalikeAfterGofmt(t *testing.T) {
	src := "//gocpp:link m\n//gocpp:Link pthread\npackage p\n"
	runDirectiveTests(t, []directiveTest{{
		name: "before gofmt",
		src:  src,
		dirs: []string{`1:1 link ["m"]`},
		errs: []string{`2:1: unknown //gocpp: directive "Link"; did you mean //gocpp:link?`},
	}})

	formatted, err := format.Source([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if want := "// gocpp:Link pthread\n//\n//gocpp:link m\npackage p\n"; string(formatted) != want {
		t.Fatalf("gofmt gives\n%s\nwant\n%s", formatted, want)
	}
	runDirectiveTests(t, []directiveTest{{
		name: "after gofmt",
		src:  string(formatted),
		dirs: []string{`3:1 link ["m"]`},
		errs: []string{`1:1: warning: "// gocpp:Link" is an ordinary comment, not a directive: write //gocpp:link, with no space after //`},
	}})
}

// TestDirectivesSyntaxError checks a file with a syntax error after the
// package clause, which the language server sees on almost every
// keystroke. The parser still gives the comments of the whole file, so the
// directives and their errors are the same as without the syntax error.
func TestDirectivesSyntaxError(t *testing.T) {
	src := "//gocpp:link m\n//gocpp:lnk x\npackage p\nfunc {\n//gocpp:link z\n"
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.gp", src, parser.ParseComments)
	if err == nil {
		t.Fatal("no syntax error")
	}
	dirs, errs := directiveStrings(fset, f)
	if want := []string{`1:1 link ["m"]`}; !slices.Equal(dirs, want) {
		t.Errorf("directives: %q, want %q", dirs, want)
	}
	wantErrs := []string{
		`2:1: unknown //gocpp: directive "lnk"; did you mean //gocpp:link?`,
		`5:1: //gocpp: directives must come before the package clause`,
	}
	if !slices.Equal(errs, wantErrs) {
		t.Errorf("errors: %q, want %q", errs, wantErrs)
	}
}

// TestDirectivePositions checks the whole position of a directive and of
// an error, the order of the directives, and that a directive without
// arguments has nil Args.
func TestDirectivePositions(t *testing.T) {
	src := "//go:build linux\n" + // 1
		"\n" +
		"//gocpp:pkg-config sdl2\n" + // 3
		"//gocpp:link\t-lm\n" + // 4: a tab is one column
		"\n" +
		"// Package p is a test.\n" + // 6
		"//\n" +
		"//gocpp:cflags \"-DA=1 2\" -O2\n" + // 8
		"//gocpp:lnk m\n" + // 9
		"//gocpp:no-win32-defaults\n" + // 10
		"package p\n" + // 11
		"\n" +
		"//gocpp:link m\n" // 13
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "/src/p/main.gp", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var errs diag.List
	dirs := ParseDirectives(fset, f, &errs)

	want := []struct {
		pos  token.Position // of Directive.Pos
		name string
		args []string
	}{
		{token.Position{Filename: "/src/p/main.gp", Offset: 18, Line: 3, Column: 1}, "pkg-config", []string{"sdl2"}},
		{token.Position{Filename: "/src/p/main.gp", Offset: 87, Line: 8, Column: 1}, "cflags", []string{"-DA=1 2", "-O2"}},
		{token.Position{Filename: "/src/p/main.gp", Offset: 130, Line: 10, Column: 1}, "no-win32-defaults", nil},
	}
	if len(dirs) != len(want) {
		t.Fatalf("got %d directives, want %d: %+v", len(dirs), len(want), dirs)
	}
	for i, w := range want {
		d := dirs[i]
		if pos := fset.Position(d.Pos); pos != w.pos || d.Name != w.name || !slices.Equal(d.Args, w.args) {
			t.Errorf("directive %d:\n got %v %s %q\nwant %v %s %q", i, pos, d.Name, d.Args, w.pos, w.name, w.args)
		}
	}
	if dirs[2].Args != nil {
		t.Errorf("no-win32-defaults: Args = %#v, want nil", dirs[2].Args)
	}

	wantErrs := []diag.Diagnostic{
		{Pos: token.Position{Filename: "/src/p/main.gp", Offset: 55, Line: 4, Column: 14}, Msg: `invalid library name "-lm": write m, without -l`},
		{Pos: token.Position{Filename: "/src/p/main.gp", Offset: 116, Line: 9, Column: 1}, Msg: `unknown //gocpp: directive "lnk"; did you mean //gocpp:link?`},
		{Pos: token.Position{Filename: "/src/p/main.gp", Offset: 167, Line: 13, Column: 1}, Msg: "//gocpp: directives must come before the package clause"},
	}
	got := errs.Diagnostics()
	if len(got) != len(wantErrs) {
		t.Fatalf("got %d errors, want %d: %+v", len(got), len(wantErrs), got)
	}
	for i := range wantErrs {
		if got[i].Pos != wantErrs[i].Pos || got[i].Msg != wantErrs[i].Msg || got[i].Severity != diag.Error {
			t.Errorf("error %d:\n got %+v\nwant %+v", i, got[i], wantErrs[i])
		}
	}
}

// TestDirectiveBadUTF8 checks that a byte that is not UTF-8 becomes U+FFFD
// in an argument, as in go/build's splitQuoted. The parser never gives
// such a directive: a bad byte before the package clause is a scanner
// error, and then the parser returns an empty file. So the AST is made by
// hand.
func TestDirectiveBadUTF8(t *testing.T) {
	src := "//gocpp:cflags a\xffb\npackage p\n"
	fset := token.NewFileSet()
	tf := fset.AddFile("main.gp", -1, len(src))
	tf.SetLinesForContent([]byte(src))
	f := &ast.File{
		Package:  tf.Pos(strings.Index(src, "package")),
		Name:     ast.NewIdent("p"),
		Comments: []*ast.CommentGroup{{List: []*ast.Comment{{Slash: tf.Pos(0), Text: "//gocpp:cflags a\xffb"}}}},
	}
	var errs diag.List
	dirs := ParseDirectives(fset, f, &errs)
	if len(dirs) != 1 || !slices.Equal(dirs[0].Args, []string{"a\uFFFDb"}) || errs.HasErrors() {
		t.Errorf("got %+v, errors %v; want one cflags [a\\uFFFDb]", dirs, errs.Diagnostics())
	}
}

// TestDirectivesNoPackageClause checks a file without a package clause,
// which the parser has already reported: its comments are not read.
func TestDirectivesNoPackageClause(t *testing.T) {
	f := &ast.File{
		Name:     ast.NewIdent("p"),
		Comments: []*ast.CommentGroup{{List: []*ast.Comment{{Slash: 1, Text: "//gocpp:lnk m"}}}},
	}
	var errs diag.List
	if dirs := ParseDirectives(token.NewFileSet(), f, &errs); dirs != nil {
		t.Errorf("directives: %v, want none", dirs)
	}
	if ds := errs.Diagnostics(); len(ds) != 0 {
		t.Errorf("errors: %v, want none", ds)
	}
}

func TestSuggestDirective(t *testing.T) {
	tests := []struct {
		name string
		want []string
	}{
		{"pkgconfig", []string{"pkg-config"}},
		{"PKG-CONFIG", []string{"pkg-config"}},
		{"lnk", []string{"link"}},        // 3 characters: 1 edit is close enough
		{"lk", nil},                      // 2 characters: 1 edit is the limit, and "link" is 2 away
		{"libs", []string{"link"}},       // 4 characters: the limit is rounded up to 2 edits
		{"extern", []string{"extern-c"}}, // 6 characters, 2 edits
		{"no-extern", []string{"no-extern-c"}},
		{"win32-defaults", []string{"no-win32-defaults"}},
		{"dflags", []string{"cflags", "ldflags"}},
		{"", nil},
		{"frobnicate", nil},
		{"include", nil},
	}
	for _, tt := range tests {
		if got := suggestDirective(tt.name); !slices.Equal(got, tt.want) {
			t.Errorf("suggestDirective(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}

	// A very long name is too far from every known name to compute any
	// distance, so it costs no allocations (not a table per name).
	long := strings.Repeat("x", 10000)
	if allocs := testing.AllocsPerRun(10, func() { suggestDirective(long) }); allocs > 0 {
		t.Errorf("suggestDirective of a long name: %v allocations, want 0", allocs)
	}
}

func TestOSADistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"", "abc", 3},
		{"link", "link", 0},
		{"lnk", "link", 1},   // insert
		{"links", "link", 1}, // delete
		{"lunk", "link", 1},  // change
		{"lnik", "link", 1},  // swap: Levenshtein says 2
		{"ilnk", "link", 1},  // swap at the start
		{"kitten", "sitting", 3},
		{"ca", "abc", 3}, // no character is edited twice: Damerau-Levenshtein says 2
		{"pkgconfig", "pkg-config", 1},
		{"é", "e", 1}, // runes, not bytes
		{"日本語", "日本", 1},
	}
	for _, tt := range tests {
		if got := osaDistance(tt.a, tt.b); got != tt.want {
			t.Errorf("osaDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
		if got := osaDistance(tt.b, tt.a); got != tt.want {
			t.Errorf("osaDistance(%q, %q) = %d, want %d", tt.b, tt.a, got, tt.want)
		}
	}
}
