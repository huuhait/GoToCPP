package main

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// The tests of gocpp check compare it with real go build 1.25.14. Each
// case is a Go program (goTree) and what go build printed for it, captured
// once with the dev shell's go (go build <args> in the folder of the
// tree, GOROOT shown as $GOROOT). The test writes the program as gocpp
// files (.go becomes .gp, go.mod becomes gocpp.mod), runs gocpp check with
// the same arguments and compares the output, with the .gp names turned
// back into .go (goNames) and the standard library's folder
// ($GOCPP/stdlib) shown as Go's ($GOROOT/src).

// gpTree returns the Go files of tree as gocpp files: x.go becomes x.gp,
// and a go.mod becomes a gocpp.mod with its module line.
func gpTree(tree map[string]string) map[string]string {
	gp := make(map[string]string)
	for name, src := range tree {
		switch {
		case name == "go.mod" || strings.HasSuffix(name, "/go.mod"):
			module, _, _ := strings.Cut(src, "\n")
			gp[strings.TrimSuffix(name, "go.mod")+"gocpp.mod"] = module + "\n"
		case strings.HasSuffix(name, ".go"):
			gp[strings.TrimSuffix(name, ".go")+".gp"] = src
		default:
			gp[name] = src
		}
	}
	return gp
}

// checkIn writes the gocpp files of the Go tree into a new folder and runs
// gocpp with args there. It returns the result with the .gp names turned
// into .go, the standard library's folder into Go's ($GOROOT/src) and the
// folder into $DIR.
//
// A value "symlink:target" in tree makes a symbolic link to target, but
// not on Windows; the test is skipped where links can't be made. On Windows, the separators
// in the output become slashes, and Windows' message for a missing file
// becomes "no such file or directory".
func checkIn(t *testing.T, tree map[string]string, args ...string) result {
	t.Helper()
	dir := t.TempDir()
	files := make(map[string]string)
	for name, src := range gpTree(tree) {
		if target, ok := strings.CutPrefix(src, "symlink:"); ok {
			if runtime.GOOS == "windows" {
				continue // links need rights there; the tests with links skip
			}
			link := filepath.Join(dir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.FromSlash(target), link); err != nil {
				t.Skipf("can't make a symbolic link: %v", err)
			}
			continue
		}
		files[name] = src
	}
	writeFiles(t, dir, files)
	t.Chdir(dir)
	code, out, errOut := gocpp(gpArgs(args)...)
	clean := func(s string) string {
		// The loader cleans the folder's name, which can have ".." in it
		// (from TMPDIR).
		s = portable(strings.ReplaceAll(s, filepath.Clean(dir), "$DIR"))
		s = goNames(s)
		return strings.ReplaceAll(s, "$GOCPP/stdlib/", "$GOROOT/src/")
	}
	return result{code, clean(out), clean(errOut)}
}

// portable returns the output s of gocpp as it is on Unix: on Windows,
// its separators become slashes, and Windows' messages for a missing file
// become Unix's ("GetFileAttributesEx ./x/: File not found." is go's
// "lstat ./x/: no such file or directory" there too). The texts differ
// between Windows and Wine, so this compares them loosely.
func portable(s string) string {
	if runtime.GOOS != "windows" {
		return s
	}
	s = filepath.ToSlash(s)
	s = windowsLstat.ReplaceAllString(s, "lstat $1: no such file or directory")
	return windowsMessages.Replace(s)
}

var (
	windowsLstat    = regexp.MustCompile(`GetFileAttributesEx (\S+): (File|Path) not found\.`)
	windowsMessages = strings.NewReplacer(
		"The system cannot find the file specified.", "no such file or directory",
		"The system cannot find the path specified.", "no such file or directory",
		"File not found.", "no such file or directory", // Wine's
		"Path not found.", "no such file or directory")
)

// goMod is a go.mod for the module path.
func goMod(path string) string {
	return "module " + path + "\n\ngo 1.25\n"
}

// A goBuildCase is a Go program, the arguments of gocpp check, and what go
// build printed with the same arguments (with "check" instead of "build").
type goBuildCase struct {
	name   string
	tree   map[string]string
	args   []string
	stderr string // go build's
}

var goBuildCases = []goBuildCase{
	{
		name: "type error",
		tree: map[string]string{
			"go.mod":  goMod("example.com/a"),
			"main.go": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(count)\n}\n",
		},
		args: []string{"./..."},
		stderr: `# example.com/a
./main.go:6:14: undefined: count
`,
	},
	{
		// The packages come in the order go build compiles them: each one
		// after its imports, and main is not compiled, as its imports
		// failed.
		name:   "two packages",
		tree:   twoPackages,
		args:   []string{"./..."},
		stderr: twoPackagesErrors,
	},
	{
		name:   "two packages, imported",
		tree:   twoPackages,
		args:   []string{"."},
		stderr: twoPackagesErrors,
	},
	{
		// An import that can't be resolved stops everything, like go list
		// inside go build: no "#" lines, no "./", and util's type error
		// is not reported.
		name: "unresolved import",
		tree: map[string]string{
			"go.mod":       goMod("example.com/c"),
			"main.go":      "package main\n\nimport (\n\t\"nosuch\"\n\n\t\"example.com/c/util\"\n)\n\nfunc main() {\n\tnosuch.X()\n\tutil.G()\n}\n",
			"util/util.go": "package util\n\nfunc G() int {\n\treturn undefinedThing\n}\n",
		},
		args: []string{"./..."},
		stderr: `main.go:4:2: package nosuch is not in std ($GOROOT/src/nosuch)
`,
	},
	{
		name: "unresolved imports in two packages",
		tree: map[string]string{
			"go.mod":  goMod("example.com/h"),
			"main.go": "package main\n\nimport (\n\t\"nosuch\"\n\n\t\"example.com/h/a\"\n)\n\nfunc main() {\n\tnosuch.X()\n\ta.F()\n}\n",
			"a/a.go":  "package a\n\nimport \"other/nope\"\n\nfunc F() { nope.Y() }\n",
			"b/b.go":  "package b\n\nfunc G() int { return undefinedThing }\n",
		},
		args: []string{"./..."},
		stderr: `a/a.go:3:8: package other/nope is not in std ($GOROOT/src/other/nope)
main.go:4:2: package nosuch is not in std ($GOROOT/src/nosuch)
`,
	},
	{
		// go list doesn't parse the function bodies, so the syntax error
		// in main waits.
		name: "unresolved imports and a syntax error",
		tree: map[string]string{
			"go.mod":  goMod("example.com/o"),
			"main.go": "package main\n\nimport (\n\t\"nosuch1\"\n\t\"nosuch2\"\n)\n\nfunc main() {\n\tx :=\n}\n",
		},
		args: []string{"."},
		stderr: `main.go:4:2: package nosuch1 is not in std ($GOROOT/src/nosuch1)
main.go:5:2: package nosuch2 is not in std ($GOROOT/src/nosuch2)
`,
	},
	{
		name: "syntax error in the imports",
		tree: map[string]string{
			"go.mod":  goMod("example.com/i"),
			"main.go": "package main\n\nimport \"fmt\n\nfunc main() { fmt.Println() }\n",
			"b/b.go":  "package b\n\nfunc G() int { return undefinedThing }\n",
		},
		args: []string{"./..."},
		stderr: `main.go:3:8: string literal not terminated
`,
	},
	{
		name: "import cycle",
		tree: map[string]string{
			"go.mod":  goMod("example.com/j"),
			"main.go": "package main\n\nimport \"example.com/j/a\"\n\nfunc main() { a.F() }\n",
			"a/a.go":  "package a\n\nimport \"example.com/j/b\"\n\nfunc F() { b.G() }\n",
			"b/b.go":  "package b\n\nimport \"example.com/j/a\"\n\nfunc G() { a.F() }\n",
		},
		args: []string{"./..."},
		stderr: `package example.com/j
	imports example.com/j/a from main.go
	imports example.com/j/b from a.go
	imports example.com/j/a from b.go: import cycle not allowed
`,
	},
	{
		// A syntax error in a body: that package is not type-checked, the
		// others are. go build's message is the compiler's, "a/a.go:5:1:
		// syntax error: unexpected }, expected expression"; this is what
		// go vet ./... prints (after "vet: "), go/parser's message.
		name: "syntax error and type error",
		tree: map[string]string{
			"go.mod": goMod("example.com/d"),
			"a/a.go": "package a\n\nfunc F() {\n\tx :=\n}\n",
			"b/b.go": "package b\n\nfunc G() int {\n\treturn undefinedThing\n}\n",
		},
		args: []string{"./..."},
		stderr: `# example.com/d/a
a/a.go:5:1: expected operand, found '}'
# example.com/d/b
b/b.go:4:9: undefined: undefinedThing
`,
	},
	{
		// main imports a package with a syntax error (the message is go
		// vet's, see above), so main is not type-checked.
		name: "syntax error in an import",
		tree: map[string]string{
			"go.mod":  goMod("example.com/k"),
			"main.go": "package main\n\nimport \"example.com/k/a\"\n\nfunc main() {\n\ta.F()\n\tvar s string = 1\n}\n",
			"a/a.go":  "package a\n\nfunc F() {\n\tx :=\n}\n",
		},
		args: []string{"."},
		stderr: `# example.com/k/a
a/a.go:5:1: expected operand, found '}'
`,
	},
	{
		// go build doesn't compile main, whose import failed, so main's
		// own error is not reported.
		name: "type error in an import",
		tree: map[string]string{
			"go.mod":  goMod("example.com/n"),
			"main.go": "package main\n\nimport \"example.com/n/a\"\n\nfunc main() {\n\ta.F()\n\tvar s string = 1\n}\n",
			"a/a.go":  "package a\n\nfunc F() { var x int = \"s\"; _ = x }\n",
		},
		args: []string{"."},
		stderr: `# example.com/n/a
a/a.go:3:24: cannot use "s" (untyped string constant) as int value in variable declaration
`,
	},
	{
		name: "too many errors",
		tree: manyErrors,
		args: []string{"."},
		stderr: `# example.com/e
./main.go:4:6: undefined: u1
./main.go:5:6: undefined: u2
./main.go:6:6: undefined: u3
./main.go:7:6: undefined: u4
./main.go:8:6: undefined: u5
./main.go:9:6: undefined: u6
./main.go:10:6: undefined: u7
./main.go:11:6: undefined: u8
./main.go:12:6: undefined: u9
./main.go:13:6: undefined: u10
./main.go:13:6: too many errors
`,
	},
	{
		// go build -gcflags=-e .
		name: "too many errors, -e",
		tree: manyErrors,
		args: []string{"-e", "."},
		stderr: `# example.com/e
./main.go:4:6: undefined: u1
./main.go:5:6: undefined: u2
./main.go:6:6: undefined: u3
./main.go:7:6: undefined: u4
./main.go:8:6: undefined: u5
./main.go:9:6: undefined: u6
./main.go:10:6: undefined: u7
./main.go:11:6: undefined: u8
./main.go:12:6: undefined: u9
./main.go:13:6: undefined: u10
./main.go:14:6: undefined: u11
./main.go:15:6: undefined: u12
`,
	},
	{
		// go build -gcflags=-e m.go: the compiler keeps only the first
		// parser error of a line, even with -e (go/parser with AllErrors
		// reports 8 errors on line 3). go build's message is "syntax
		// error: missing condition in if statement".
		name: "one parser error per line, -e",
		tree: map[string]string{
			"m.go": "package main\n\nfunc main() { if { } else { } for ;;; {} }\n",
		},
		args: []string{"-e", "m.go"},
		stderr: `# command-line-arguments
./m.go:3:18: missing condition in if statement
`,
	},
	{
		// The scanner's errors all stay, also with -e and on one line,
		// like the compiler's (whose messages differ: "more than one
		// character in rune literal", "unknown escape"). The second
		// "illegal rune literal" of line 3 is a duplicate: same line, same
		// message.
		name: "scanner errors, -e",
		tree: map[string]string{
			"m.go": "package main\n\nvar x = 'ab' + 'cd' + \"\\q\"\nvar a, b = 0x, 1e\nvar s = \"\\z\\y\"\n\nfunc main() {}\n",
		},
		args: []string{"-e", "m.go"},
		stderr: `# command-line-arguments
./m.go:3:9: illegal rune literal
./m.go:3:25: unknown escape sequence
./m.go:4:14: hexadecimal literal has no digits
./m.go:4:18: exponent has no digits
./m.go:5:11: unknown escape sequence
`,
	},
	{
		name: "files",
		tree: map[string]string{
			"go.mod": goMod("example.com/f"),
			"x.go":   "package main\n\nfunc main() {\n\tvar s string = 1\n}\n",
			"y.go":   "package main\n\nfunc g() { undefinedY() }\n",
		},
		args: []string{"x.go", "y.go"},
		stderr: `# command-line-arguments
./x.go:4:6: declared and not used: s
./x.go:4:17: cannot use 1 (untyped int constant) as string value in variable declaration
./y.go:3:12: undefined: undefinedY
`,
	},
	{
		name: "files with a bad import",
		tree: map[string]string{
			"go.mod": goMod("example.com/r"),
			"x.go":   "package main\n\nimport _ \"nosuch\"\n\nfunc main() {}\n",
		},
		args: []string{"x.go"},
		stderr: `x.go:3:8: package nosuch is not in std ($GOROOT/src/nosuch)
`,
	},
	{
		name: "clean",
		tree: map[string]string{
			"go.mod":  goMod("example.com/g"),
			"main.go": "package main\n\nfunc main() {}\n",
		},
		args: []string{"./..."},
	},
	{
		// ./... leaves out _ and testdata folders, folders of another
		// module, a folder with only tests and one whose files the build
		// constraints exclude (go build ./ex reports that one).
		name:   "./... skips folders",
		tree:   skippedFolders,
		args:   []string{"./..."},
		stderr: "",
	},
	{
		name: "a folder the build constraints exclude",
		tree: skippedFolders,
		args: []string{"./ex"},
		stderr: `package example.com/u/ex: build constraints exclude all Go files in $DIR/ex
`,
	},
	{
		name: "./... and the folder the build constraints exclude",
		tree: skippedFolders,
		args: []string{"./...", "./ex"},
		stderr: `package example.com/u/ex: build constraints exclude all Go files in $DIR/ex
`,
	},
	{
		name: "./... matches nothing",
		tree: map[string]string{"go.mod": goMod("example.com/v")},
		args: []string{"./..."},
		stderr: `go: warning: "./..." matched no packages
`,
	},
	{
		name: "missing folder",
		tree: map[string]string{
			"go.mod": goMod("example.com/n"),
			"a/a.go": "package a\n\nfunc F() { var x int = \"s\"; _ = x }\n",
		},
		args: []string{"./nosuchdir", "./a"},
		stderr: `stat $DIR/nosuchdir: directory not found
`,
	},
	{
		// go list goes through the imports by import path: nosuch10 comes
		// before nosuch2.
		name: "twelve unresolved imports",
		tree: map[string]string{
			"go.mod": goMod("example.com/p"),
			"main.go": func() string {
				src := "package main\n\nimport (\n"
				for i := 1; i <= 12; i++ {
					src += "\t_ \"nosuch" + strconv.Itoa(i) + "\"\n"
				}
				return src + ")\n"
			}(),
		},
		args: []string{"."},
		stderr: `main.go:4:2: package nosuch1 is not in std ($GOROOT/src/nosuch1)
main.go:13:2: package nosuch10 is not in std ($GOROOT/src/nosuch10)
main.go:14:2: package nosuch11 is not in std ($GOROOT/src/nosuch11)
main.go:15:2: package nosuch12 is not in std ($GOROOT/src/nosuch12)
main.go:5:2: package nosuch2 is not in std ($GOROOT/src/nosuch2)
main.go:6:2: package nosuch3 is not in std ($GOROOT/src/nosuch3)
main.go:7:2: package nosuch4 is not in std ($GOROOT/src/nosuch4)
main.go:8:2: package nosuch5 is not in std ($GOROOT/src/nosuch5)
main.go:9:2: package nosuch6 is not in std ($GOROOT/src/nosuch6)
main.go:10:2: package nosuch7 is not in std ($GOROOT/src/nosuch7)
main.go:11:2: package nosuch8 is not in std ($GOROOT/src/nosuch8)
main.go:12:2: package nosuch9 is not in std ($GOROOT/src/nosuch9)
`,
	},
	{
		// go list shows only the first syntax error of a package.
		name: "syntax error in a list of imports",
		tree: map[string]string{
			"go.mod": goMod("example.com/s1"),
			"a.go":   "package main\n\nimport ( \"fmt\" 3 )\n\nfunc main() { fmt.Println() }\n",
		},
		args: []string{"."},
		stderr: `a.go:3:16: expected ';', found 3
`,
	},
	{
		// ... and only that of the first file with one.
		name: "syntax errors in the imports of two files",
		tree: map[string]string{
			"go.mod": goMod("example.com/s2"),
			"a.go":   "package main\n\nimport ( \"fmt\" 3 )\n\nfunc main() { fmt.Println() }\n",
			"b.go":   "package main\n\nimport \"os\" 4\n\nvar _ = os.Args\n",
		},
		args: []string{"."},
		stderr: `a.go:3:16: expected ';', found 3
`,
	},
	{
		// An import that can't be resolved is reported once, at its first
		// place in go list's order: main imports example.com/s3/q before
		// nosuch, so q's import comes first.
		name: "the same unresolved import twice",
		tree: sameUnresolved,
		args: []string{"./..."},
		stderr: `q/q.go:3:8: package nosuch is not in std ($GOROOT/src/nosuch)
`,
	},
	{
		name: "the same unresolved import twice, q first",
		tree: sameUnresolved,
		args: []string{"./q", "."},
		stderr: `q/q.go:3:8: package nosuch is not in std ($GOROOT/src/nosuch)
`,
	},
	{
		// A syntax error just after the package clause or the imports is
		// not in the part of the file that go list reads (go/build's
		// header), so go build compiles the packages and reports c too.
		// This is go build -p 1 (with more jobs, c can come first) and go
		// vet's message: go build's is the compiler's ("a/a.go:1:14: syntax
		// error: unexpected name x, expected semicolon or newline", "b/b.go:
		// 3:14: syntax error: unexpected name x after top level
		// declaration").
		name: "syntax errors after the header",
		tree: map[string]string{
			"go.mod": goMod("example.com/s4"),
			"a/a.go": "package main x\n\nfunc main() {}\n",
			"b/b.go": "package b\n\nimport \"fmt\" x\n\nvar _ = fmt.Sprint\n",
			"c/c.go": "package c\n\nfunc G() int { return undefinedThing }\n",
		},
		args: []string{"./..."},
		stderr: `# example.com/s4/a
a/a.go:1:14: expected ';', found x
# example.com/s4/b
b/b.go:3:14: expected ';', found x
# example.com/s4/c
c/c.go:3:23: undefined: undefinedThing
`,
	},
	{
		// The compiler keeps the scanner's error and the first parser
		// error of the line, and drops the one at the end of the file,
		// with and without -e (go/parser with AllErrors also reports
		// "5:3: expected ';', found 'EOF'"). go build's messages are "4:7:
		// more than one character in rune literal" and "4:14: syntax
		// error: unexpected ), expected expression".
		name:   "parser errors after a scanner error",
		tree:   scannerThenParser,
		args:   []string{"."},
		stderr: scannerThenParserErrors,
	},
	{
		name:   "parser errors after a scanner error, -e",
		tree:   scannerThenParser,
		args:   []string{"-e", "."},
		stderr: scannerThenParserErrors,
	},
	{
		// A pattern error is a load error: go build reports it with the
		// errors of go list, and builds nothing.
		name: "a pattern error and an unresolved import",
		tree: map[string]string{
			"go.mod":   goMod("example.com/s6"),
			"bad/b.go": "package bad\n\nimport _ \"nosuchpkg\"\n",
		},
		args: []string{"./nosuch/...", "./bad"},
		stderr: `pattern ./nosuch/...: lstat ./nosuch/: no such file or directory
bad/b.go:3:8: package nosuchpkg is not in std ($GOROOT/src/nosuchpkg)
`,
	},
	{
		name: "two pattern errors",
		tree: map[string]string{
			"go.mod":        goMod("example.com/s6"),
			"nested/go.mod": goMod("example.com/nested"),
			"nested/n.go":   "package n\n",
		},
		args: []string{"./nested/...", "./nosuch/..."},
		stderr: `pattern ./nested/...: directory prefix nested does not contain main module or its selected dependencies
pattern ./nosuch/...: lstat ./nosuch/: no such file or directory
`,
	},
	{
		// The warning, then the errors of the other pattern.
		name: "a pattern that matches nothing and one with errors",
		tree: map[string]string{
			"go.mod":   goMod("example.com/s6"),
			"_u/u.go":  "package u\n\nfunc F() { undefinedU }\n",
			"lib/x.go": "package x\n\nfunc F() { undefinedX }\n",
		},
		args: []string{"./_u/...", "./lib"},
		stderr: `go: warning: "./_u/..." matched no packages
# example.com/s6/lib
lib/x.go:3:12: undefined: undefinedX
`,
	},
	{
		// Without a module, a pattern with ... is an error (outside a
		// module, go build ... matches only the standard library).
		name:   "./... outside a module",
		tree:   map[string]string{"m.go": "package main\n\nfunc main() { undefinedM }\n"},
		args:   []string{"./..."},
		stderr: "pattern ./...: directory prefix . does not contain main module or its selected dependencies\n",
	},
	{
		// When go/build's reader finds a syntax error in the imports, it
		// gives the parser the whole file, but go list shows only the first
		// error, and none of the function bodies (with and without -e).
		name:   "unterminated import and a body error",
		tree:   unterminatedImport,
		args:   []string{"."},
		stderr: "a.go:3:8: string literal not terminated\n",
	},
	{
		name:   "unterminated import and a body error, -e",
		tree:   unterminatedImport,
		args:   []string{"-e", "."},
		stderr: "a.go:3:8: string literal not terminated\n",
	},
	{
		// The misplaced //gocpp: line is a gocpp error (a comment in Go),
		// after the imports: go list never sees it.
		name: "a syntax error in the imports and a directive error in a body",
		tree: map[string]string{
			"go.mod": goMod("example.com/c2"),
			"a.go":   "package main\n\nimport ( \"fmt\" 3 )\n\nfunc main() {\n\t//gocpp:lnik m\n\tfmt.Println()\n}\n",
		},
		args:   []string{"."},
		stderr: "a.go:3:16: expected ';', found 3\n",
	},
	{
		// go/build parses without AllErrors: the scanner's error comes
		// first, not go/parser's "expected ';', found 'ILLEGAL'".
		name: "an illegal character in the imports",
		tree: map[string]string{
			"go.mod": goMod("example.com/c3"),
			"a.go":   "package main\n\nimport ( \"fmt\" # )\n\nfunc main() { fmt.Println() }\n",
		},
		args:   []string{"."},
		stderr: "a.go:3:16: illegal character U+0023 '#'\n",
	},
	{
		name: "a NUL byte in the imports",
		tree: map[string]string{
			"go.mod": goMod("example.com/c4"),
			"a.go":   "package main\n\nimport \"f\x00mt\"\n\nfunc main() {}\n",
		},
		args:   []string{"."},
		stderr: "read $DIR/a.go: unexpected NUL in input\n",
	},
	{
		// After the syntax error in the imports, go/build reads the rest
		// of the file too, and its NUL byte wins.
		name: "a syntax error in the imports and a NUL byte in a body",
		tree: map[string]string{
			"go.mod": goMod("example.com/n"),
			"a.go":   "package main\n\nimport ( \"fmt\" 3 )\n\nfunc main() {\n\tx := \"a\x00b\"\n\tfmt.Println(x)\n}\n",
		},
		args:   []string{"."},
		stderr: "read $DIR/a.go: unexpected NUL in input\n",
	},
	{
		// A folder of tests is matched and then left out, without a
		// warning.
		name: "./... with only tests",
		tree: map[string]string{
			"go.mod":       goMod("example.com/c5"),
			"main.go":      "package main\n\nfunc main() {}\n",
			"ot/x_test.go": "package ot\n",
		},
		args: []string{"./ot/..."},
	},
	{
		name: "... outside a module",
		tree: map[string]string{"m.go": "package main\n\nfunc main() { undefinedM }\n"},
		args: []string{"..."},
	},
}

// unterminatedImport has an unterminated import path and a syntax error
// in a function body.
var unterminatedImport = map[string]string{
	"go.mod": goMod("example.com/c1"),
	"a.go":   "package main\n\nimport \"fmt\n\nfunc main() {\n\tx := )\n}\n",
}

// sameUnresolved is a program where main and q import nosuch.
var sameUnresolved = map[string]string{
	"go.mod":  goMod("example.com/s3"),
	"main.go": "package main\n\nimport (\n\t_ \"nosuch\"\n\n\t_ \"example.com/s3/q\"\n)\n\nfunc main() {}\n",
	"q/q.go":  "package q\n\nimport _ \"nosuch\"\n",
}

// scannerThenParser has a scanner error and two parser errors.
var scannerThenParser = map[string]string{
	"go.mod":  goMod("example.com/s5"),
	"main.go": "package main\n\nfunc main() {\n\tx := 'ab' + )\n}\n",
}

const scannerThenParserErrors = `# example.com/s5
./main.go:4:7: illegal rune literal
./main.go:4:14: expected operand, found ')'
`

// twoPackagesErrors is go build's output for twoPackages, for both "."
// and "./...".
const twoPackagesErrors = `# example.com/b/geom
geom/geom.go:4:6: declared and not used: x
geom/geom.go:4:14: cannot use "s" (untyped string constant) as int value in variable declaration
# example.com/b/util
util/util.go:4:9: undefined: undefinedThing
`

// twoPackages is a program whose main imports two packages with errors.
var twoPackages = map[string]string{
	"go.mod":       goMod("example.com/b"),
	"main.go":      "package main\n\nimport (\n\t\"example.com/b/geom\"\n\t\"example.com/b/util\"\n)\n\nfunc main() {\n\tgeom.F()\n\tutil.G()\n}\n",
	"geom/geom.go": "package geom\n\nfunc F() {\n\tvar x int = \"s\"\n}\n",
	"util/util.go": "package util\n\nfunc G() int {\n\treturn undefinedThing\n}\n",
}

// manyErrors is a program with 12 errors.
var manyErrors = func() map[string]string {
	src := "package main\n\nfunc main() {\n"
	for i := 1; i <= 12; i++ {
		src += "\t_ = u" + strconv.Itoa(i) + "\n"
	}
	return map[string]string{"go.mod": goMod("example.com/e"), "main.go": src + "}\n"}
}()

// skippedFolders is a module whose only package is main, but whose
// folders ./... looks at.
var skippedFolders = map[string]string{
	"go.mod":             goMod("example.com/u"),
	"main.go":            "package main\n\nfunc main() {}\n",
	"ex/ex.go":           "//go:build ignore\n\npackage ex\n",
	"onlytest/x_test.go": "package onlytest\n",
	"_skip/s.go":         "package skip\n\nfunc F() { undefinedS }\n",
	"testdata/t/t.go":    "package t\n\nfunc F() { undefinedT }\n",
	"nested/go.mod":      goMod("example.com/nested"),
	"nested/n.go":        "package nested\n\nfunc F() { undefinedN }\n",
	"empty/README":       "no Go files\n",
}

// goWords turns the words of gocpp's messages that differ from go's only
// because gocpp has .gp files into go's: "Go files" for ".gp files"
// (".go files" after goNames), and "go: warning" for "gocpp: warning".
var goWords = strings.NewReplacer(".go files", "Go files", "gocpp: warning:", "go: warning:")

func TestCheckLikeGoBuild(t *testing.T) {
	for _, c := range goBuildCases {
		t.Run(c.name, func(t *testing.T) {
			r := checkIn(t, c.tree, append([]string{"check"}, c.args...)...)
			r.stderr = goWords.Replace(r.stderr)
			want := result{stderr: c.stderr}
			for line := range strings.Lines(c.stderr) {
				if !strings.HasPrefix(line, "go: warning:") {
					want.code = 1 // an error
				}
			}
			if r != want {
				t.Errorf("gocpp check %s:\n%s\nwant (go build):\n%s", strings.Join(c.args, " "), show(r), show(want))
			}
		})
	}
}

// TestCheckWarning checks that a warning is printed like an error, with
// "warning: ", and doesn't change the exit status.
func TestCheckWarning(t *testing.T) {
	tree := map[string]string{"main.go": "// gocpp:link m\n\npackage main\n\nfunc main() {}\n"}
	r := checkIn(t, tree, "check", "main.go")
	want := result{0, "", `# command-line-arguments
./main.go:1:1: warning: "// gocpp:link" is an ordinary comment, not a directive: write //gocpp:link, with no space after //
`}
	if r != want {
		t.Errorf("gocpp check main.gp:\n%s\nwant:\n%s", show(r), show(want))
	}

	// With an error: exit status 1, and the warning comes in its place.
	tree["main.go"] = "// gocpp:link m\n\npackage main\n\nfunc main() { undefinedX() }\n"
	r = checkIn(t, tree, "check", "main.go")
	want = result{1, "", `# command-line-arguments
./main.go:1:1: warning: "// gocpp:link" is an ordinary comment, not a directive: write //gocpp:link, with no space after //
./main.go:5:15: undefined: undefinedX
`}
	if r != want {
		t.Errorf("gocpp check main.gp:\n%s\nwant:\n%s", show(r), show(want))
	}
}

// examplesGates reads internal/check/testdata/gates/examples.txt: the
// phase gate errors of each example at Phase 02, which gocpp check
// reports now (version.Phase is 02, and the gates allow Phase 02's
// subset). It maps each example to its error lines, as check prints them
// for the example's folder (win32 for windows/amd64).
func examplesGates(t *testing.T) map[string][]string {
	t.Helper()
	data, err := os.ReadFile("../../internal/check/testdata/gates/examples.txt")
	if err != nil {
		t.Fatal(err)
	}
	gates := make(map[string][]string)
	var name string
	for line := range strings.Lines(string(data)) {
		line = strings.TrimSuffix(line, "\n")
		if n, ok := strings.CutPrefix(line, "== "); ok {
			name = n
			gates[name] = []string{}
		} else if line != "" && name != "" {
			gates[name] = append(gates[name], line)
		}
	}
	return gates
}

// TestCheckExamples runs gocpp check in the folder of each target
// program (win32 with GOOS=windows): with -e, its errors are exactly the
// gate errors of examples.txt, and without -e the first 10 of them, then
// "too many errors", under the "# <import path>" line of their package.
// (examples.txt lists the packages in the order go/types checks them;
// gocpp check prints them in go build -p 1's order, so the lines are
// compared as sets, each package's own lines in order.)
func TestCheckExamples(t *testing.T) {
	gates := examplesGates(t)
	mains, err := filepath.Glob("../../examples/*/main.gp")
	if err != nil {
		t.Fatal(err)
	}
	if len(mains) != 19 || len(gates) != 19 {
		t.Errorf("found %d examples and %d in examples.txt, want 19", len(mains), len(gates))
	}
	for _, m := range mains {
		dir, err := filepath.Abs(filepath.Dir(m))
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(dir)
		t.Run(name, func(t *testing.T) {
			want, ok := gates[name]
			if !ok {
				t.Fatalf("%s is not in examples.txt", name)
			}
			if name == "win32" {
				t.Setenv("GOOS", "windows")
				t.Setenv("GOARCH", "amd64")
				// The namespace probe uses x86_64-w64-mingw32-g++, not the
				// host compiler that GOCPP_CXX may name.
				t.Setenv("GOCPP_CXX", "")
			}
			t.Chdir(dir)

			code, out, errOut := gocpp("check", "-e", ".")
			blocks := packageBlocks(t, portable(errOut))
			var got []string
			for _, b := range blocks {
				got = append(got, b.lines...)
			}
			wantCode := 0
			if len(want) > 0 {
				wantCode = 1
			}
			if code != wantCode || out != "" || !sameLines(got, want) {
				t.Errorf("gocpp check -e .: exit %d, stdout %q, stderr:\n%s\nwant exit %d and these errors:\n%s",
					code, out, errOut, wantCode, strings.Join(want, "\n"))
			}
			// Each package's lines are in the order of examples.txt.
			for _, b := range blocks {
				if !slices.Equal(b.lines, subsequence(want, b.lines)) {
					t.Errorf("# %s: lines not in the order of examples.txt:\n%s", b.pkg, strings.Join(b.lines, "\n"))
				}
			}

			// Without -e: at most 10 errors per package, like go build.
			_, _, errOut = gocpp("check", ".")
			for i, b := range packageBlocks(t, portable(errOut)) {
				all := blocks[i].lines
				if len(all) < 10 {
					if !slices.Equal(b.lines, all) {
						t.Errorf("# %s without -e:\n%s\nwant:\n%s", b.pkg, strings.Join(b.lines, "\n"), strings.Join(all, "\n"))
					}
					continue
				}
				pos, _, _ := strings.Cut(strings.TrimPrefix(all[9], ""), ": ")
				wantLines := append(slices.Clone(all[:10]), pos+": too many errors")
				if !slices.Equal(b.lines, wantLines) {
					t.Errorf("# %s without -e:\n%s\nwant:\n%s", b.pkg, strings.Join(b.lines, "\n"), strings.Join(wantLines, "\n"))
				}
			}
		})
	}
}

// A packageBlock is the "# <import path>" line of a package in the
// output of gocpp check and the lines under it.
type packageBlock struct {
	pkg   string
	lines []string
}

// packageBlocks splits the output of gocpp check into its packages. Every
// line must be under a "# " line.
func packageBlocks(t *testing.T, out string) []packageBlock {
	t.Helper()
	var blocks []packageBlock
	for line := range strings.Lines(out) {
		line = strings.TrimSuffix(line, "\n")
		if pkg, ok := strings.CutPrefix(line, "# "); ok {
			blocks = append(blocks, packageBlock{pkg: pkg})
			continue
		}
		if len(blocks) == 0 {
			t.Fatalf("line %q comes before a # line:\n%s", line, out)
		}
		blocks[len(blocks)-1].lines = append(blocks[len(blocks)-1].lines, line)
	}
	return blocks
}

// sameLines reports whether a and b have the same lines, in any order.
func sameLines(a, b []string) bool {
	return slices.Equal(slices.Sorted(slices.Values(a)), slices.Sorted(slices.Values(b)))
}

// subsequence returns the lines of all that are in lines, in the order of
// all.
func subsequence(all, lines []string) []string {
	var sub []string
	for _, l := range all {
		if slices.Contains(lines, l) {
			sub = append(sub, l)
		}
	}
	return sub
}

// TestCheckExamplesPattern checks ./... on the examples folder, which is
// in no module: like go build, gocpp check refuses it.
func TestCheckExamplesPattern(t *testing.T) {
	t.Chdir("../..")
	code, out, errOut := gocpp("check", "./examples/...")
	want := "pattern ./examples/...: directory prefix examples does not contain main module or its selected dependencies\n"
	if code != 1 || out != "" || errOut != want {
		t.Errorf("gocpp check ./examples/...: exit %d, stdout %q, stderr %q; want exit 1 and %q", code, out, errOut, want)
	}
}

func TestCheckUsage(t *testing.T) {
	code, out, errOut := gocpp("check", "-h")
	if code != 0 || out != "" || !strings.HasPrefix(errOut, checkUsage) || !strings.Contains(errOut, "-types") {
		t.Errorf("gocpp check -h: exit %d, stdout %q, stderr:\n%s\nwant exit 0 and the usage with the flags on stderr", code, out, errOut)
	}
	code, out, errOut = gocpp("check", "-x")
	if code != 2 || out != "" || !strings.HasPrefix(errOut, "flag provided but not defined: -x\n"+checkUsage) {
		t.Errorf("gocpp check -x: exit %d, stdout %q, stderr:\n%s\nwant exit 2, the error and the usage on stderr", code, out, errOut)
	}
}

// TestCheckOther checks errors that go build has no exact counterpart for.
func TestCheckOther(t *testing.T) {
	for _, c := range []struct {
		name   string
		tree   map[string]string
		args   []string
		code   int
		stderr string
	}{{
		// A gocpp error at an import is a load error, like go list's
		// "invalid import path".
		name:   "header import error",
		tree:   map[string]string{"main.go": "package main\n\nimport \"./x\"\n\nfunc main() {}\n"},
		args:   []string{"main.gp"},
		code:   1,
		stderr: "main.go:3:8: invalid import path \"./x\": a local header ends in .h, .hh, .hpp, .hxx, .h++ or .inl\n",
	}, {
		// go build a.go prints "go: reading a.go: unexpected NUL in input".
		name:   "a NUL byte in a file named on the command line",
		tree:   map[string]string{"a.go": "package main\n\nimport \"f\x00mt\"\n"},
		args:   []string{"a.gp"},
		code:   1,
		stderr: "gocpp: reading a.go: unexpected NUL in input\n",
	}, {
		// More than 10 syntax errors: the loader's AST is cut short, but
		// the error is still in a function body, not a load error.
		name: "many syntax errors",
		tree: map[string]string{"main.go": "package main\n\nfunc main() {\n" + strings.Repeat("\tx := #\n", 12) + "}\n"},
		args: []string{"main.gp"},
		code: 1,
		stderr: "# command-line-arguments\n" + strings.Repeat("./main.go:LINE:7: illegal character U+0023 '#'\n", 10) +
			"./main.go:LINE:7: too many errors\n",
	}} {
		t.Run(c.name, func(t *testing.T) {
			r := checkIn(t, c.tree, append([]string{"check"}, c.args...)...)
			want := result{c.code, "", numberLines(c.stderr)}
			if r != want {
				t.Errorf("gocpp check %s:\n%s\nwant:\n%s", strings.Join(c.args, " "), show(r), show(want))
			}
		})
	}
}

// numberLines replaces the LINE of each line of s with 4, 5, 6, ... and
// the LINE of a last "too many errors" line with the line before it.
func numberLines(s string) string {
	n := 3
	var b strings.Builder
	for line := range strings.Lines(s) {
		if strings.Contains(line, "LINE") {
			if !strings.Contains(line, "too many errors") {
				n++
			}
			line = strings.Replace(line, "LINE", strconv.Itoa(n), 1)
		}
		b.WriteString(line)
	}
	return b.String()
}

// typesFile and typesSDL are the files of the package of TestCheckTypes:
// header imports, a namespace, opaque values and types, and ordinary Go.
const typesFile = `package main

import (
	"fmt"

	cstdio "<stdio.h>"
	"<vector>"
)

type point struct{ x, y int }

func main() {
	n := 3
	p := point{x: 1, y: n}
	v := std.vector[int]{}
	v.push_back(n)
	k := cstdio.getchar()
	fmt.Println(p.x, len("ab"), k, v.size())
}
`

// typesSDL is the second file of the package of TestCheckTypes.
const typesSDL = `package main

import "<SDL2/SDL.h>"

type window struct {
	w *SDL.SDL_Window
	n int
}

func id(w *SDL.SDL_Window) {
	t := window{w: w, n: 1}
	_ = SDL.SDL_GetWindowID(t.w)
}
`

// typesWant is what gocpp check -types prints for the package. Each file
// has columns of its own.
const typesWant = `./main.gp:10:6   def type   point                                     point
./main.gp:10:12  type       struct{ x, y int }                        struct{x int; y int}
./main.gp:10:20  def field  x                                         int
./main.gp:10:23  def field  y                                         int
./main.gp:10:25  type       int                                       int
./main.gp:12:6   def func   main                                      func()
./main.gp:13:2   def var    n                                         int
./main.gp:13:7   const      3                                         int = 3
./main.gp:14:2   def var    p                                         point
./main.gp:14:7   value      point{x: 1, y: n}                         point
./main.gp:14:7   type       point                                     point
./main.gp:14:16  const      1                                         int = 1
./main.gp:14:22  var        n                                         int
./main.gp:15:2   def var    v                                         opaque
./main.gp:15:7   value      std.vector[int]{}                         opaque
./main.gp:15:7   type       std.vector[int]                           opaque
./main.gp:15:7   type       std.vector                                opaque
./main.gp:15:18  type       int                                       int
./main.gp:16:2   value      v.push_back(n)                            opaque
./main.gp:16:2   value      v.push_back                               opaque
./main.gp:16:2   var        v                                         opaque
./main.gp:16:14  var        n                                         int
./main.gp:17:2   def var    k                                         opaque
./main.gp:17:7   value      cstdio.getchar()                          opaque
./main.gp:17:7   value      cstdio.getchar                            opaque
./main.gp:18:2   value      fmt.Println(p.x, len("ab"), k, v.size())  (n int, err error)
./main.gp:18:2   value      fmt.Println                               func(a ...any) (n int, err error)
./main.gp:18:14  var        p.x                                       int
./main.gp:18:14  var        p                                         point
./main.gp:18:19  const      len("ab")                                 int = 2
./main.gp:18:19  builtin    len                                       -
./main.gp:18:23  const      "ab"                                      untyped string = "ab"
./main.gp:18:30  var        k                                         opaque
./main.gp:18:33  value      v.size()                                  opaque
./main.gp:18:33  value      v.size                                    opaque
./main.gp:18:33  var        v                                         opaque
./sdl.gp:5:6    def type   window                              opaque (struct { w *SDL.SDL_Window n int })
./sdl.gp:5:13   type       struct { w *SDL.SDL_Window n int }  opaque
./sdl.gp:6:2    def field  w                                   opaque (*SDL.SDL_Window)
./sdl.gp:6:4    type       *SDL.SDL_Window                     opaque
./sdl.gp:6:5    type       SDL.SDL_Window                      opaque
./sdl.gp:7:2    def field  n                                   int
./sdl.gp:7:4    type       int                                 int
./sdl.gp:10:6   def func   id                                  func(w opaque (*SDL.SDL_Window))
./sdl.gp:10:9   def var    w                                   opaque (*SDL.SDL_Window)
./sdl.gp:10:11  type       *SDL.SDL_Window                     opaque
./sdl.gp:10:12  type       SDL.SDL_Window                      opaque
./sdl.gp:11:2   def var    t                                   window
./sdl.gp:11:7   value      window{w: w, n: 1}                  window
./sdl.gp:11:7   type       window                              window
./sdl.gp:11:17  var        w                                   opaque
./sdl.gp:11:23  const      1                                   int = 1
./sdl.gp:12:6   value      SDL.SDL_GetWindowID(t.w)            opaque
./sdl.gp:12:6   value      SDL.SDL_GetWindowID                 opaque
./sdl.gp:12:26  var        t.w                                 opaque
./sdl.gp:12:26  var        t                                   window
`

func TestCheckTypes(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"gocpp.mod": "module example.com/types\n", "main.gp": typesFile, "sdl.gp": typesSDL})
	t.Chdir(dir)
	for _, args := range [][]string{{"-types", "."}, {"-types", "main.gp", "sdl.gp"}, {"-types", "-e", "./..."}} {
		code, out, errOut := gocpp(append([]string{"check"}, args...)...)
		// The types are there also with the phase gates' errors.
		pkg := "example.com/types"
		if args[1] == "main.gp" {
			pkg = "command-line-arguments"
		}
		want := result{1, typesWant, "# " + pkg + "\n" + typesGates}
		if r := (result{code, portable(out), portable(errOut)}); r != want {
			t.Errorf("gocpp check %s:\n%s\nwant:\n%s", strings.Join(args, " "), show(r), show(want))
		}
	}
}

// typesGates are the phase gates' errors for the package of TestCheckTypes.
const typesGates = `./main.gp:6:9: header imports are not supported yet (Phase 04)
./main.gp:10:6: type declarations are not supported yet (Phase 03)
./main.gp:10:12: composite types are not supported yet (Phase 03)
./sdl.gp:3:8: header imports are not supported yet (Phase 04)
./sdl.gp:5:6: type declarations are not supported yet (Phase 03)
./sdl.gp:5:13: composite types are not supported yet (Phase 03)
./sdl.gp:6:4: pointers are not supported yet (Phase 03)
`

// TestCheckTypesConversions checks -types on conversions of opaque values,
// which Check rewrote (check.Conversion): they print as written, T(x), as
// a value of type T, without the nodes of the rewrite, also as a statement
// (gp_cgoh.Unused[T](x)[0]) and after defer (T(gp_cgoh.Value[T](x))). A
// function with C types in its signature prints them as opaque, and so
// does a builtin that go/types gives such a signature.
func TestCheckTypesConversions(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"main.gp": `package main

import "<cstdio>"

func f(n int, p *cstdio.FILE, rest ...C.int) (*cstdio.FILE, int) { return p, n }

func main() {
	x := cstdio.getchar()
	n := int(x) + 1
	int(x)
	defer int(x)
	_, _ = f(n, nil)
	_ = append([]C.int{}, x)
	var s struct{ fs []cstdio.FILE }
	s.fs = append(s.fs, cstdio.FILE{})
}
`})
	t.Chdir(dir)
	code, out, _ := gocpp("check", "-types", "main.gp")
	want := `./main.gp:5:6    def func   f                            func(n int, p opaque (*cstdio.FILE), rest opaque (...C.int)) (opaque, int)
./main.gp:5:8    def var    n                            int
./main.gp:5:10   type       int                          int
./main.gp:5:15   def var    p                            opaque (*cstdio.FILE)
./main.gp:5:17   type       *cstdio.FILE                 opaque
./main.gp:5:18   type       cstdio.FILE                  opaque
./main.gp:5:31   def var    rest                         opaque (...C.int)
./main.gp:5:36   type       ...C.int                     opaque
./main.gp:5:39   type       C.int                        opaque
./main.gp:5:47   type       *cstdio.FILE                 opaque
./main.gp:5:48   type       cstdio.FILE                  opaque
./main.gp:5:61   type       int                          int
./main.gp:5:75   var        p                            opaque
./main.gp:5:78   var        n                            int
./main.gp:7:6    def func   main                         func()
./main.gp:8:2    def var    x                            opaque
./main.gp:8:7    value      cstdio.getchar()             opaque
./main.gp:8:7    value      cstdio.getchar               opaque
./main.gp:9:2    def var    n                            int
./main.gp:9:7    value      int(x) + 1                   int
./main.gp:9:7    value      int(x)                       int
./main.gp:9:7    type       int                          int
./main.gp:9:11   var        x                            opaque
./main.gp:9:16   const      1                            int = 1
./main.gp:10:2   value      int(x)                       int
./main.gp:10:2   type       int                          int
./main.gp:10:6   var        x                            opaque
./main.gp:11:8   value      int(x)                       int
./main.gp:11:8   type       int                          int
./main.gp:11:12  var        x                            opaque
./main.gp:12:9   value      f(n, nil)                    opaque
./main.gp:12:9   value      f                            opaque
./main.gp:12:11  var        n                            int
./main.gp:12:14  nil        nil                          untyped nil
./main.gp:13:6   value      append([]C.int{}, x)         opaque
./main.gp:13:6   builtin    append                       -
./main.gp:13:13  value      []C.int{}                    opaque
./main.gp:13:13  type       []C.int                      opaque
./main.gp:13:15  type       C.int                        opaque
./main.gp:13:24  var        x                            opaque
./main.gp:14:6   def var    s                            opaque (struct{ fs []cstdio.FILE })
./main.gp:14:8   type       struct{ fs []cstdio.FILE }   opaque
./main.gp:14:16  def field  fs                           opaque ([]cstdio.FILE)
./main.gp:14:19  type       []cstdio.FILE                opaque
./main.gp:14:21  type       cstdio.FILE                  opaque
./main.gp:15:2   var        s.fs                         opaque
./main.gp:15:2   var        s                            opaque
./main.gp:15:9   value      append(s.fs, cstdio.FILE{})  opaque
./main.gp:15:9   builtin    append                       func(opaque, ...opaque) opaque
./main.gp:15:16  var        s.fs                         opaque
./main.gp:15:16  var        s                            opaque
./main.gp:15:22  value      cstdio.FILE{}                opaque
./main.gp:15:22  type       cstdio.FILE                  opaque
`
	if code != 1 || portable(out) != want {
		t.Errorf("gocpp check -types main.gp: exit %d\n%s\nwant:\n%s", code, portable(out), want)
	}
}

// TestCheckTypesErrors checks -types on a package with a type error: the
// types go to stdout, the error to stderr, and the exit status is 1. A
// package that is not type-checked prints no types.
func TestCheckTypesErrors(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"main.gp": "package main\n\nfunc main() {\n\tvar s string = 1\n\t_ = s\n}\n",
		"bad.gp":  "package main\n\nfunc f() {\n",
	})
	t.Chdir(dir)
	code, out, errOut := gocpp("check", "-types", "main.gp")
	out, errOut = portable(out), portable(errOut)
	want := result{1, `./main.gp:3:6   def func  main    func()
./main.gp:4:6   def var   s       string
./main.gp:4:8   type      string  string
./main.gp:4:17  const     1       untyped int = 1
./main.gp:5:6   var       s       string
`, "# command-line-arguments\n./main.gp:4:17: cannot use 1 (untyped int constant) as string value in variable declaration\n"}
	if r := (result{code, out, errOut}); r != want {
		t.Errorf("gocpp check -types main.gp:\n%s\nwant:\n%s", show(r), show(want))
	}
	code, out, errOut = gocpp("check", "-types", "bad.gp")
	errOut = portable(errOut)
	want = result{1, "", "# command-line-arguments\n./bad.gp:3:12: expected '}', found 'EOF'\n"}
	if r := (result{code, out, errOut}); r != want {
		t.Errorf("gocpp check -types bad.gp:\n%s\nwant:\n%s", show(r), show(want))
	}
}

// patternTree is a module with the folders that go's pattern matching
// treats in special ways, as gocpp files (so gpTree changes nothing): the
// folder x.gp has a .gp file, sym/ln is a link to sym/target, sym2/inner a
// link to it from another folder.
var patternTree = map[string]string{
	"gocpp.mod":          "module example.com/s6\n",
	"main.gp":            "package main\n\nfunc main() {}\n",
	"_u/u.gp":            "package u\n\nfunc F() { undefinedU }\n",
	"testdata/t.gp":      "package t\n\nfunc F() { undefinedT }\n",
	".hid/h.gp":          "package h\n\nfunc F() { undefinedH }\n",
	"nested/gocpp.mod":   "module example.com/nested\n",
	"nested/n.gp":        "package n\n\nfunc F() { undefinedN }\n",
	"vendor/v/v.gp":      "package v\n\nfunc F() { undefinedV }\n",
	"x.gp/x.gp":          "package x\n\nfunc F() { undefinedX }\n",
	"sym/target/l.gp":    "package target\n\nfunc F() { undefinedL }\n",
	"sym/ln":             "symlink:target",
	"sym2/inner":         "symlink:../sym/target",
	"win/w.gp":           "//go:build solaris\n\npackage win\n",
	"onlytest/x_test.gp": "package onlytest\n",
}

// TestCheckPatterns checks patterns with "..." against what go build
// printed for the same tree with .go files (shown here with gocpp's
// names: .gp, gocpp.mod, "gocpp: warning").
func TestCheckPatterns(t *testing.T) {
	for _, c := range []struct {
		args   []string
		code   int
		stderr string
	}{
		{[]string{"./_u/..."}, 0, "gocpp: warning: \"./_u/...\" matched no packages\n"},
		{[]string{"./testdata/..."}, 0, "gocpp: warning: \"./testdata/...\" matched no packages\n"},
		{[]string{"./.hid/..."}, 0, "gocpp: warning: \"./.hid/...\" matched no packages\n"},
		// A root that is a link is followed, under its own name.
		{[]string{"./sym/ln/..."}, 1, "# example.com/s6/sym/ln\nsym/ln/l.gp:3:12: undefined: undefinedL\n"},
		// A link below the root is not.
		{[]string{"./sym2/..."}, 0, "gocpp: warning: \"./sym2/...\" matched no packages\n"},
		{[]string{"./nosuch/..."}, 1, "pattern ./nosuch/...: lstat ./nosuch/: no such file or directory\n"},
		{[]string{"./nested/..."}, 1, "pattern ./nested/...: directory prefix nested does not contain main module or its selected dependencies\n"},
		// A folder named x.gp is a folder, also when it is named alone.
		{[]string{"./x.gp/..."}, 1, "# example.com/s6/x.gp\nx.gp/x.gp:3:12: undefined: undefinedX\n"},
		{[]string{"./x.gp"}, 1, "# example.com/s6/x.gp\nx.gp/x.gp:3:12: undefined: undefinedX\n"},
		// ./... leaves out _u, testdata, .hid, nested (another module), the
		// packages in vendor folders, the link sym/ln, win (its build
		// constraints) and onlytest. The order is go build -p 1's (with
		// more jobs, x.gp can come first). gocpp has no vendoring: go
		// build ./vendor/... wants a vendor/modules.txt there.
		{[]string{"./..."}, 1, "# example.com/s6/sym/target\nsym/target/l.gp:3:12: undefined: undefinedL\n" +
			"# example.com/s6/x.gp\nx.gp/x.gp:3:12: undefined: undefinedX\n"},
		{[]string{"./vendor/..."}, 1, "# example.com/s6/vendor/v\nvendor/v/v.gp:3:12: undefined: undefinedV\n"},
		// ... is every package of the main module and of the standard
		// library. The walk warns about the links (go build prints each
		// warning twice).
		{[]string{"..."}, 1, "warning: ignoring symlink $DIR/sym/ln\nwarning: ignoring symlink $DIR/sym2/inner\n" +
			"# example.com/s6/sym/target\nsym/target/l.gp:3:12: undefined: undefinedL\n" +
			"# example.com/s6/x.gp\nx.gp/x.gp:3:12: undefined: undefinedX\n"},
		{[]string{"example.com/s6/sym/..."}, 1, "warning: ignoring symlink $DIR/sym/ln\n" +
			"# example.com/s6/sym/target\nsym/target/l.gp:3:12: undefined: undefinedL\n"},
		// A pattern without ... names the folder of the build constraints
		// or of tests (as an import path too), so its error shows.
		{[]string{"./...", "example.com/s6/win"}, 1, "package example.com/s6/win: build constraints exclude all .gp files in $DIR/win\n"},
		{[]string{"./onlytest", "./..."}, 1, "example.com/s6/onlytest: no non-test .gp files in $DIR/onlytest\n"},
	} {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			if runtime.GOOS == "windows" && (strings.Contains(c.args[0], "sym") || strings.Contains(c.stderr, "ignoring symlink")) {
				t.Skip("needs symbolic links")
			}
			r := checkIn(t, patternTree, append([]string{"check"}, c.args...)...)
			r.stdout, r.stderr = gpNames(r.stdout), gpNames(r.stderr)
			if want := (result{c.code, "", c.stderr}); r != want {
				t.Errorf("gocpp check %s:\n%s\nwant:\n%s", strings.Join(c.args, " "), show(r), show(want))
			}
		})
	}
}

// gpNames undoes goNames of checkIn, for a test whose tree has gocpp
// names already.
func gpNames(s string) string {
	return regexp.MustCompile(`\.go([\n: ])`).ReplaceAllString(s, ".gp$1")
}

// TestCheckBrokenFolders checks that ./... doesn't skip a folder with a
// broken //go:build line or a file that can't be read, like go build,
// which printed "b.go: parsing //go:build line: unexpected end of
// expression" and "open $DIR/unr/u.go: permission denied" (the loader
// names the file in its own way).
func TestCheckBrokenFolders(t *testing.T) {
	tree := map[string]string{
		"gocpp.mod":  "module example.com/w\n",
		"main.gp":    "package main\n\nfunc main() {}\n",
		"brk/b.gp":   "//go:build linux &&\n\npackage brk\n",
		"unr/u.gp":   "package unr\n",
		"ign/i.gp":   "//go:build ignore\n\npackage ign\n",
		"tonly/x.gp": "//go:build solaris\n\npackage tonly\n",
	}
	dir := t.TempDir()
	writeFiles(t, dir, tree)
	unreadable := filepath.Join(dir, "unr", "u.gp")
	if err := os.Chmod(unreadable, 0); err != nil {
		t.Fatal(err)
	}
	want := "brk/b.gp: parsing //go:build line: unexpected end of expression\n"
	if _, err := os.ReadFile(unreadable); err != nil {
		want += "unr/u.gp: open " + unreadable + ": permission denied\n"
	} // else root, or Windows: the file can be read
	t.Chdir(dir)
	code, out, errOut := gocpp("check", "./...")
	if r := (result{code, portable(out), portable(errOut)}); r != (result{1, "", portable(want)}) {
		t.Errorf("gocpp check ./...:\n%s\nwant:\n%s", show(r), want)
	}
}

// TestCheckWarningsKept checks that the loader's warnings show when the
// packages can't be loaded and for a package whose import failed: they
// don't depend on the failure.
func TestCheckWarningsKept(t *testing.T) {
	const lookalike = "// gocpp:link m\n\n"
	const warning = `warning: "// gocpp:link" is an ordinary comment, not a directive: write //gocpp:link, with no space after //`
	r := checkIn(t, map[string]string{
		"main.go": lookalike + "package main\n\nimport _ \"nosuch\"\n\nfunc main() {}\n",
	}, "check", "main.gp")
	// The import error first, in go list's order, then the package's own.
	want := result{1, "", "main.go:5:8: package nosuch is not in std ($GOROOT/src/nosuch)\nmain.go:1:1: " + warning + "\n"}
	if r != want {
		t.Errorf("load error:\n%s\nwant:\n%s", show(r), show(want))
	}

	r = checkIn(t, map[string]string{
		"go.mod":  goMod("example.com/n"),
		"main.go": lookalike + "package main\n\nimport \"example.com/n/a\"\n\nfunc main() {\n\ta.F()\n\tvar s string = 1\n}\n",
		"a/a.go":  "package a\n\nfunc F() { var x int = \"s\"; _ = x }\n",
	}, "check", ".")
	want = result{1, "", "# example.com/n/a\na/a.go:3:24: cannot use \"s\" (untyped string constant) as int value in variable declaration\n" +
		"# example.com/n\n./main.go:1:1: " + warning + "\n"}
	if r != want {
		t.Errorf("type error in an import:\n%s\nwant:\n%s", show(r), show(want))
	}
}

// TestCheckTarget checks GOOS and GOARCH: they choose the files, and a
// pair that go doesn't know is an error with go's message ("go:
// unsupported GOOS/GOARCH pair foo/amd64", exit status 2).
func TestCheckTarget(t *testing.T) {
	win32, err := filepath.Abs("../../examples/win32")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		goos, goarch string
		code         int
		stderr       string
	}{
		// For windows, win32 has the gates' errors of examples.txt.
		{"windows", "amd64", 1, "gates"},
		{"linux", "amd64", 1, "package command-line-arguments: build constraints exclude all .gp files in " + win32 + "\n"},
		{"foo", "amd64", 2, "gocpp: unsupported GOOS/GOARCH pair foo/amd64\n"},
		{"linux", "bar", 2, "gocpp: unsupported GOOS/GOARCH pair linux/bar\n"},
		{"Linux", "amd64", 2, "gocpp: unsupported GOOS/GOARCH pair Linux/amd64\n"},
		// No C++ compiler for windows/arm64: the namespace probe's error
		// at the first header import, then the gates.
		{"windows", "arm64", 1, "no compiler"},
	} {
		t.Run(c.goos+"/"+c.goarch, func(t *testing.T) {
			t.Setenv("GOOS", c.goos)
			t.Setenv("GOARCH", c.goarch)
			t.Setenv("GOCPP_CXX", "") // the probe finds the target's compiler
			t.Chdir(win32)
			code, out, errOut := gocpp("check", ".")
			switch c.stderr {
			case "gates":
				c.stderr = "# command-line-arguments\n" + strings.Join(examplesGates(t)["win32"], "\n") + "\n"
				errOut = portable(errOut)
			case "no compiler":
				var lines []string
				for _, l := range examplesGates(t)["win32"] {
					if strings.HasSuffix(l, "header imports are not supported yet (Phase 04)") {
						pos, _, _ := strings.Cut(l, " ")
						lines = append(lines, pos+" cannot check the header imports: no C++ compiler for windows/arm64 (set GOCPP_CXX)")
					}
					lines = append(lines, l)
				}
				c.stderr = "# command-line-arguments\n" + strings.Join(lines, "\n") + "\n"
				errOut = portable(errOut)
			}
			if r := (result{code, out, errOut}); r != (result{c.code, "", c.stderr}) {
				t.Errorf("GOOS=%s GOARCH=%s gocpp check .:\n%s\nwant exit %d and stderr %q", c.goos, c.goarch, show(r), c.code, c.stderr)
			}
		})
	}
}
