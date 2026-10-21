package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/lsp/debounce"
	"github.com/huuhait/gocpp/internal/lsp/protocol"
)

// fastFiles is a module for the shortcut of the check of a rename (renamefast.go): a
// declares names; b uses them by their qualified names, c uses b and a, d
// dot-imports a, e imports a under another name, and f has a name of its own
// that the new name would clash with only if the use were not qualified.
var fastFiles = map[string]string{
	"gocpp.mod": "module example.com/fast\n",
	"a/a.gp": `package a

type T struct{ N int }

func (t T) M() int { return t.N }

func F() int { return 1 }

var V = 2

const K = 3

func inA() int { return F() + V + K }
`,
	"b/b.gp": `package b

import "example.com/fast/a"

func Call() int { return a.F() + a.V + a.K + a.T{N: 1}.M() }
`,
	"c/c.gp": `package c

import (
	"example.com/fast/a"
	"example.com/fast/b"
)

func Use() int { return a.F() + b.Call() + a.V }
`,
	"d/d.gp": `package d

import . "example.com/fast/a"

func Dot() int { return F() + V + K }
`,
	"e/e.gp": `package e

import x "example.com/fast/a"

func Alias() int { return x.F() + x.V + x.K }
`,
	"mixed/qualified.gp": `package mixed

import "example.com/fast/a"

func Qualified() int { return a.F() + a.V + a.K }
`,
	"mixed/dotted.gp": `package mixed

import . "example.com/fast/a"

var Keep T
func MixedFunc() int { return 1 }
var MixedVar = 2
const MixedConst = 3
`,
	"g/g.gp": `package g

import . "example.com/fast/a"

func Zed() int { return K }
`,
	"f/f.gp": `package f

import "example.com/fast/a"

func G() int { return 7 }

var W = 9

func Fn() int { return a.F() + G() + W }
`,
}

// fastRename renames at the mark of a/a.gp in a new server, and returns the
// edited files (or the error) and the number of package checks the rename made.
func fastRename(t *testing.T, all bool, mark, newName string) (string, int) {
	t.Helper()
	return fastRenameFiles(t, fastFiles, "a/a.gp", all, mark, newName)
}

func fastRenameFiles(
	t *testing.T, sourceFiles map[string]string, file string, all bool, mark, newName string,
) (string, int) {
	t.Helper()
	recheckAllImporters = all
	defer func() { recheckAllImporters = false }()
	root := writeTree(t, sourceFiles)
	srv := newServer(nil, testConfig(), debounce.RealClock{})
	c := &refClient{dclient: &dclient{client: startServer(t, srv, nil), root: root}, src: sourceFiles}
	c.call("initialize", plainInit)
	c.notify("initialized", "{}")
	c.skipShown = true
	c.open(file, sourceFiles[file])
	c.next(file)
	// The searches check the importers once; the rename's own checks are counted.
	if _, err := c.rename(file, mark, newName+"Warm"); err != nil {
		_ = err // (a refusal also leaves the importers checked)
	}
	_, _, before := srv.pipeline.Stats()
	edits, rerr := c.rename(file, mark, newName)
	_, _, after := srv.pipeline.Stats()
	if rerr != nil {
		return "error: " + rerr.Message, int(after - before)
	}
	var files []string
	for name, text := range edits {
		files = append(files, name+":\n"+text)
	}
	slices.Sort(files)
	return strings.Join(files, "\n"), int(after - before)
}

func bindingProofFiles() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/binding\n",
		"a/a.gp": `package a
// Emoji before the declaration exercises UTF-16 positions: 🙂
func F(n int) int {
	if n == 0 { return 0 }
	return F(n - 1)
}
func Use() int { return F(2) }
var V = 1
const K = 2
type T struct{}
func (T) M() int { return 1 }
`,
		"a/other.gp": `package a
func Shadowed() int {
	F := func(int) int { return 9 }
	return F(0)
}
`,
		"b/b.gp": `package b
import alias "example.com/binding/a"
func Use() int { return alias.F(1) }
`,
	}
}

// genericTypeSource is a generic type of the package a with a method and a function that makes it.
const genericTypeSource = `
type G[P any] struct{ Item P }

func (g G[P]) Get() P { return g.Item }

func NewG[P any](item P) G[P] { return G[P]{Item: item} }
`

// addHelpers gives the package a names that are not exported: a function, a variable, a constant
// and a type, used by the package.
func addHelpers(files map[string]string) {
	files["a/a.gp"] += `
func helper() int { return hidden + limit }

var hidden = 1

const limit = 2

type token struct{ n int }

func Use2() int { return helper() + token{n: limit}.n }
`
}

func TestRenameDeclaringBindingProof(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(map[string]string)
		file    string
		mark    string
		newName string
		outcome string
	}{
		{name: "recursive and shadowed old-name uses", outcome: "proved"},
		{name: "request at own-package use", mark: "return |F(n", outcome: "proved"},
		{name: "request at qualified importer use", file: "b/b.gp", mark: "return alias.|F(",
			outcome: "proved"},
		{name: "unrelated local new-name declaration", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = strings.ReplaceAll(files["a/other.gp"], "F", "Renamed")
			}},
		{name: "Unicode identifier and UTF-16 positions", mark: "func |𝑭(",
			newName: "Édit", outcome: "proved", setup: func(files map[string]string) {
				for name, text := range files {
					files[name] = strings.ReplaceAll(text, "F", "𝑭")
				}
			}},
		{name: "declaring-scope collision", outcome: "refuse", setup: func(files map[string]string) {
			files["a/other.gp"] += "\nvar Renamed = 1\n"
		}},
		{name: "same-typed local capture", outcome: "refuse", setup: func(files map[string]string) {
			files["a/other.gp"] = "package a\nfunc Other() int {\n" +
				"Renamed := func(int) int { return 9 }; return F(2)\n}\n"
		}},
		{name: "unedited declaring-file import collision", outcome: "refuse",
			setup: func(files map[string]string) {
				files["c/c.gp"] = "package c\nconst Value = 1\n"
				files["a/other.gp"] = "package a\nimport Renamed \"example.com/binding/c\"\n" +
					"func Other() int { return Renamed.Value }\n"
			}},
		{name: "declaring-file dot-import binding", outcome: "refuse",
			setup: func(files map[string]string) {
				files["c/c.gp"] = "package c\nfunc Renamed() int { return 1 }\n"
				files["a/other.gp"] = "package a\nimport . \"example.com/binding/c\"\n" +
					"func Other() int { return Renamed() }\n"
			}},
		{name: "mixed qualified and dot-import counterexample", outcome: "refuse",
			setup: func(files map[string]string) {
				files["b/dot.gp"] = "package b\nimport . \"example.com/binding/a\"\n" +
					"var Keep = K\nfunc Renamed() int { return 1 }\n"
			}},
		{name: "generic caller type-parameter capture", outcome: "refuse",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\n" +
					"func Generic[Renamed any](n Renamed) int { return F(1) }\n"
			}},
		{name: "unrelated generic caller", outcome: "proved", setup: func(files map[string]string) {
			files["a/other.gp"] = "package a\nfunc Generic[U any](n U) int { return F(1) }\n"
		}},
		{name: "broken importer retains scratch validation", outcome: "fallback",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nvar Broken = undefinedName\n"
			}},
		{name: "visibility loss", newName: "renamed", outcome: "refuse"},
		{name: "reserved C name", newName: "C", outcome: "refuse"},
		{name: "stdlib-shaped module path", outcome: "fallback", setup: func(files map[string]string) {
			for name, text := range files {
				files[name] = strings.ReplaceAll(text, "example.com/binding", "fmt")
			}
		}},
		{name: "generic function target", mark: "func |F[", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/a.gp"] = strings.ReplaceAll(files["a/a.gp"],
					"func F(n int) int", "func F[T ~int](n T) T")
			}},
		{name: "generic function target instantiated by an importer", mark: "func |F[",
			outcome: "proved", setup: func(files map[string]string) {
				files["a/a.gp"] = strings.ReplaceAll(files["a/a.gp"],
					"func F(n int) int", "func F[T ~int](n T) T")
				files["b/b.gp"] += "\nfunc Twice() int { return alias.F[int](2) }\n"
			}},
		{name: "generic function target hidden by its own type parameter", mark: "func |F[",
			outcome: "refuse", setup: func(files map[string]string) {
				files["a/a.gp"] = strings.ReplaceAll(files["a/a.gp"],
					"func F(n int) int", "func F[Renamed ~int](n Renamed) Renamed")
			}},
		{name: "generic function target renamed to a type that it converts to", mark: "func |F[",
			outcome: "refuse", setup: func(files map[string]string) {
				files["a/a.gp"] = strings.ReplaceAll(files["a/a.gp"],
					"func F(n int) int", "func F[T ~int](n T) T")
				files["a/other.gp"] = "package a\ntype Renamed int\n"
			}},
		{name: "variable target", mark: "var |V", outcome: "proved"},
		{name: "constant target", mark: "const |K", outcome: "proved"},
		{name: "variable target used by an importer", mark: "var |V", outcome: "proved",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nvar Held = alias.V + alias.K\n"
			}},
		{name: "variable target of a group with another name", file: "a/other.gp", mark: "|Second",
			outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nvar (\n\tFirst = 1\n\tSecond = First + 1\n)\n"
			}},
		{name: "variable target of a multiple declaration", file: "a/other.gp", mark: "|Q",
			outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nvar P, Q = 1, 2\n"
			}},
		{name: "constant target of an iota group", file: "a/other.gp", mark: "|Warn", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nconst (\n\tDebug = iota\n\tWarn\n\tFatal\n)\n" +
					"func Level() int { return Warn }\n"
			}},
		{name: "variable target hidden by a local of the new name", mark: "var |V",
			outcome: "refuse", setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nfunc Hide() int {\n\tRenamed := 1\n\treturn V + Renamed\n}\n"
			}},
		{name: "constant target hiding a local of the new name", mark: "const |K",
			outcome: "refuse", setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nfunc Hide() int {\n\tRenamed := 1\n\treturn K + Renamed\n}\n"
			}},
		{name: "variable target with a declared new name", mark: "var |V", outcome: "refuse",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nconst Renamed = 4\n"
			}},
		{name: "variable target used by a dot importer", mark: "var |V", outcome: "refuse",
			setup: func(files map[string]string) {
				files["b/dot.gp"] = "package b\nimport . \"example.com/binding/a\"\n" +
					"var Dotted = V\nvar Renamed = 1\n"
			}},
		{name: "variable target named like a field", mark: "var |V", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\ntype Box struct{ V int }\nvar Item = Box{V: V}\n"
			}},
		{name: "variable target renamed to the name of a field", mark: "var |V", newName: "N",
			outcome: "proved", setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\ntype Box struct{ N int }\nvar Item = Box{N: V}\n"
			}},
		{name: "variable target used by a broken importer", mark: "var |V", outcome: "fallback",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nvar Broken = undefinedName\n"
			}},
		{name: "constant target of a generic declaring package", mark: "const |K", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nfunc Generic[U any](n U) int { return K }\n"
			}},
		{name: "constant target hidden by a type parameter", mark: "const |K", outcome: "refuse",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nfunc Generic[Renamed any](n Renamed) int { return K }\n"
			}},
		{name: "local variable of the same name is not the target", mark: "var |V",
			outcome: "proved", setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nfunc Local() int {\n\tV := 3\n\treturn V\n}\n"
			}},
		{name: "type target", mark: "type |T", outcome: "proved"},
		{name: "type target used by an importer", mark: "type |T", outcome: "proved",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nvar Held alias.T\n\nfunc Make() alias.T { return alias.T{} }\n"
			}},
		{name: "type target used by a dot importer", mark: "type |T", outcome: "refuse",
			setup: func(files map[string]string) {
				files["b/dot.gp"] = "package b\nimport . \"example.com/binding/a\"\n" +
					"var Dotted T\nvar Renamed = 1\n"
			}},
		{name: "type target named like a field that does not embed it", mark: "type |T",
			outcome: "proved", setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\ntype Holder struct{ T int }\n"
			}},
		{name: "type target embedded in its package", mark: "type |T", outcome: "fallback",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\ntype Outer struct{ T }\n"
			}},
		{name: "type target embedded by pointer in an importer", mark: "type |T",
			outcome: "fallback", setup: func(files map[string]string) {
				files["b/b.gp"] += "\ntype Outer struct{ *alias.T }\n"
			}},
		{name: "type target with a clashing field of an embedder", mark: "type |T",
			outcome: "refuse", setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\ntype Outer struct {\n\tT\n\tRenamed int\n}\n"
			}},
		{name: "type target hidden by a parameter", mark: "type |T", outcome: "refuse",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nfunc Keep(Renamed int) T { return T{} }\n"
			}},
		{name: "type target with a declared new name", mark: "type |T", outcome: "refuse",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\ntype Renamed struct{}\n"
			}},
		{name: "type target that shadows a universe name it is renamed to", mark: "type |T",
			newName: "String", outcome: "proved"},
		{name: "type target renamed over a use of a universe name", mark: "type |T",
			newName: "Error", outcome: "proved", setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\nfunc Fail() error { return nil }\n"
			}},
		{name: "generic type target", mark: "type |G", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/a.gp"] += genericTypeSource
			}},
		{name: "generic type target instantiated by an importer", mark: "type |G",
			outcome: "proved", setup: func(files map[string]string) {
				files["a/a.gp"] += genericTypeSource
				files["b/b.gp"] += "\nvar Held alias.G[int]\n\n" +
					"func Make() alias.G[string] { return alias.NewG(\"x\") }\n"
			}},
		{name: "generic type target hidden by a type parameter of a method", mark: "type |G",
			outcome: "refuse", setup: func(files map[string]string) {
				files["a/a.gp"] += genericTypeSource +
					"\nfunc (g G[Renamed]) Peek() G[Renamed] { return g }\n"
			}},
		{name: "generic type target embedded in its package", mark: "type |G",
			outcome: "fallback", setup: func(files map[string]string) {
				files["a/a.gp"] += genericTypeSource + "\ntype Outer struct{ G[int] }\n"
			}},
		{name: "alias type target", mark: "type |Alias", outcome: "fallback",
			setup: func(files map[string]string) {
				files["a/a.gp"] += "\ntype Alias = T\n"
			}},
		{name: "type target used by a broken importer", mark: "type |T", outcome: "fallback",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nvar Broken = undefinedName\n"
			}},
		{name: "unexported function", mark: "func |helper(", newName: "assist", outcome: "proved",
			setup: addHelpers},
		{name: "unexported variable", mark: "var |hidden", newName: "secret", outcome: "proved",
			setup: addHelpers},
		{name: "unexported constant", mark: "const |limit", newName: "ceiling", outcome: "proved",
			setup: addHelpers},
		{name: "unexported type", mark: "type |token", newName: "mark", outcome: "proved",
			setup: addHelpers},
		{name: "unexported function renamed to an exported name", mark: "func |helper(",
			newName: "Assist", outcome: "fallback", setup: addHelpers},
		{name: "unexported function renamed to a name of the package", mark: "func |helper(",
			newName: "limit", outcome: "refuse", setup: addHelpers},
		{name: "unexported function renamed to main", mark: "func |helper(", newName: "main",
			outcome: "fallback", setup: addHelpers},
		{name: "unexported function hidden by a local of the new name", mark: "func |helper(",
			newName: "assist", outcome: "refuse", setup: func(files map[string]string) {
				addHelpers(files)
				files["a/other.gp"] += "\nfunc Hide() int {\n\tassist := 1\n\treturn helper() + assist\n}\n"
			}},
		{name: "unexported type embedded in its package", mark: "type |token", newName: "mark",
			outcome: "fallback", setup: func(files map[string]string) {
				addHelpers(files)
				files["a/other.gp"] += "\ntype Wrapper struct{ token }\n"
			}},
		{name: "unexported function used by a test file", mark: "func |helper(", newName: "assist",
			outcome: "fallback", setup: func(files map[string]string) {
				addHelpers(files)
				files["a/helper_test.gp"] = "package a\nfunc testHelper() int { return helper() }\n"
			}},
		{name: "import name at its declaration", file: "b/b.gp", mark: "import |alias",
			outcome: "proved"},
		{name: "import name at a use", file: "b/b.gp", mark: "return |alias.F", outcome: "proved"},
		{name: "import without a name", file: "b/b.gp", mark: "return |a.F(1)",
			outcome: "proved", setup: func(files map[string]string) {
				files["b/b.gp"] = "package b\nimport \"example.com/binding/a\"\n" +
					"func Use() int { return a.F(1) }\n"
			}},
		{name: "import name renamed to a name of the package", file: "b/b.gp",
			mark: "import |alias", outcome: "refuse", setup: func(files map[string]string) {
				files["b/b.gp"] += "\nvar Renamed = 1\n"
			}},
		{name: "import name renamed to a name of another file's package scope", file: "b/b.gp",
			mark: "import |alias", outcome: "refuse", setup: func(files map[string]string) {
				files["b/other.gp"] = "package b\nvar Renamed = 1\n"
			}},
		{name: "import name hidden by a local of the new name", file: "b/b.gp",
			mark: "import |alias", outcome: "refuse", setup: func(files map[string]string) {
				files["b/b.gp"] += "\nfunc Hide() int {\n\tRenamed := 2\n\treturn alias.F(1) + Renamed\n}\n"
			}},
		{name: "import name renamed to a predeclared name that the file uses", file: "b/b.gp",
			mark: "import |alias", newName: "len", outcome: "refuse",
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nfunc Size(s string) int { return len(s) }\n"
			}},
		{name: "import name renamed to the name of another import", file: "b/b.gp",
			mark: "|alias \"example", newName: "fmt", outcome: "refuse",
			setup: func(files map[string]string) {
				files["b/b.gp"] = strings.Replace(files["b/b.gp"], "import alias \"example.com/binding/a\"",
					"import (\n\t\"fmt\"\n\talias \"example.com/binding/a\"\n)", 1) +
					"\nfunc Show() { fmt.Println() }\n"
			}},
		{name: "import name renamed to a local of another file", file: "b/b.gp",
			mark: "import |alias", outcome: "proved", setup: func(files map[string]string) {
				files["b/other.gp"] = "package b\nfunc Other() int {\n\tRenamed := 1\n\treturn Renamed\n}\n"
			}},
		{name: "import name renamed to a field name", file: "b/b.gp", mark: "import |alias",
			newName: "Item", outcome: "proved", setup: func(files map[string]string) {
				files["b/b.gp"] += "\ntype Box struct{ Item int }\n\nvar Held = Box{Item: alias.F(1)}\n"
			}},
		{name: "import name renamed to init", file: "b/b.gp", mark: "import |alias",
			newName: "init", outcome: "refuse"},
		{name: "import name used by a name that the loader does not write", file: "b/b.gp",
			mark: "import |alias", outcome: "fallback", setup: func(files map[string]string) {
				files["b/b.gp"] += "\nfunc Place() string { return #file() }\n"
			}},
		{name: "method target", mark: "func (T) |M", outcome: "proved"},
		{name: "import name renamed to a variable that a test file declares", file: "b/b.gp",
			mark: "import |alias", outcome: "refuse", setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\nvar Renamed = 1\n"
			}},
		{name: "import name renamed to a function that a test file declares", file: "b/b.gp",
			mark: "import |alias", outcome: "refuse", setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\nfunc Renamed() int { return 1 }\n"
			}},
		{name: "import name renamed to a type that a test file declares", file: "b/b.gp",
			mark: "import |alias", outcome: "refuse", setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\ntype Renamed int\n"
			}},
		{name: "import name renamed to a constant that a test file declares", file: "b/b.gp",
			mark: "import |alias", outcome: "refuse", setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\nconst Renamed = 1\n"
			}},
		{name: "import name renamed to the name that a test file gives its own import", file: "b/b.gp",
			mark: "import |alias", outcome: "fallback", setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\nimport Renamed \"example.com/binding/a\"\n\n" +
					"var Used = Renamed.F(1)\n"
			}},
		{name: "import name of a package that has a test file", file: "b/b.gp", mark: "import |alias",
			outcome: "fallback", setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\nvar Unrelated = 1\n"
			}},
		{name: "import name of a package that has an external test file", file: "b/b.gp",
			mark: "import |alias", outcome: "fallback", setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b_test\n\nvar Renamed = 1\n"
			}},
		{name: "untouched test file", outcome: "fallback", setup: func(files map[string]string) {
			files["a/unrelated_test.gp"] = "package a\nfunc TestHelper() int { return 3 }\n"
		}},
		{name: "bang-origin package", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] += "\nfunc Might() (int, error) { return 1, nil }\n" +
				"func Handler() (int, error) { return Might!(), nil }\n"
		}},
		{name: "hash character in a string and a comment", outcome: "proved",
			setup: func(files map[string]string) {
				files["a/other.gp"] = "package a\n// issue #12\nvar Tag = \"#tag\"\nvar Mark = '#'\n"
			}},
		{name: "builtin macro origin", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] += "\nfunc Place() string { return #file() }\n"
		}},
		{name: "macro declaration", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] += "\nmacro Identity(v int) int { return v }\n"
		}},
		{name: "Go-only validator with untracked companion source", outcome: "proved",
			setup: func(files map[string]string) {
				// The snapshot/full validator models GP files, not C++ references.
				files["a/native.c"] = "int native_value = 1;\n"
			}},
		{name: "directive-bearing package", outcome: "fallback", setup: func(files map[string]string) {
			files["a/a.gp"] = "//gocpp:link m\n" + files["a/a.gp"]
		}},
		{name: "broken declaring package", outcome: "fallback", setup: func(files map[string]string) {
			files["a/other.gp"] = "package a\nvar Broken = undefinedName\n"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := bindingProofFiles()
			if tc.setup != nil {
				tc.setup(files)
			}
			mark := tc.mark
			if mark == "" {
				mark = "func |F("
			}
			newName := tc.newName
			if newName == "" {
				newName = "Renamed"
			}
			file := tc.file
			if file == "" {
				file = "a/a.gp"
			}
			full, fullChecks := fastRenameFiles(t, files, file, true, mark, newName)
			fast, fastChecks := fastRenameFiles(t, files, file, false, mark, newName)
			if full != fast {
				t.Fatalf("proof changes the full-validator answer\nfull:\n%s\nproof:\n%s", full, fast)
			}
			refused := strings.HasPrefix(fast, "error:")
			if refused != (tc.outcome == "refuse") {
				t.Fatalf("unexpected refusal/success: %s", fast)
			}
			if tc.outcome == "proved" && (fastChecks != 0 || fullChecks == 0) {
				t.Fatalf("proof=%d checks, full=%d; want 0 vs positive", fastChecks, fullChecks)
			}
			if tc.outcome == "fallback" && fastChecks == 0 {
				t.Fatal("unproven form did not fall back to scratch validation")
			}
		})
	}
}

func TestRenameBindingProofRequiresConflictGuards(t *testing.T) {
	skipConflictChecks = true
	defer func() { skipConflictChecks = false }()
	files := bindingProofFiles()
	full, _ := fastRenameFiles(t, files, "a/a.gp", true, "func |F(", "Renamed")
	fast, checks := fastRenameFiles(t, files, "a/a.gp", false, "func |F(", "Renamed")
	if full != fast || strings.HasPrefix(fast, "error:") || checks == 0 {
		t.Fatal("binding proof was used without the existing conflict guards")
	}
}

func bindingProofRenamer(t *testing.T) *renamer {
	t.Helper()
	return renamerFor(t, bindingProofFiles(), "a/a.gp", "func |F(", "Renamed")
}

// renamerFor returns the renamer of the name at the mark of the file, before
// any check of it (the test sets what the guards would).
func renamerFor(
	t *testing.T, files map[string]string, file, mark, newName string,
) *renamer {
	t.Helper()
	root := writeTree(t, files)
	srv := newServer(nil, testConfig(), debounce.RealClock{})
	client := &refClient{
		dclient: &dclient{client: startServer(t, srv, nil), root: root},
		src:     files,
	}
	client.call("initialize", plainInit)
	client.notify("initialized", "{}")
	client.skipShown = true
	client.open(file, files[file])
	client.next(file)
	snapshot, release := srv.session.Snapshot()
	t.Cleanup(release)
	var params protocol.TextDocumentPositionParams
	if err := json.Unmarshal([]byte(client.posParams(file, files[file], mark)), &params); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	query, err := newRefQuery(ctx, snapshot, params.TextDocument.URI, params.Position)
	if err != nil || query == nil {
		t.Fatalf("query: %v", err)
	}
	symbol, err := query.symbolAt(ctx, true)
	if err != nil || symbol == nil {
		t.Fatalf("symbol: %v", err)
	}
	result, err := srv.search(ctx, symbol)
	if err != nil {
		t.Fatal(err)
	}
	return &renamer{
		s: srv, e: env{snapshot: snapshot}, sym: symbol, res: result,
		newName: newName, docs: newDocs(snapshot),
	}
}

func TestRenameBindingProofRequiresEveryOccurrence(t *testing.T) {
	rn := bindingProofRenamer(t)
	result := *rn.res
	result.hits = slices.Clone(result.hits)
	rn.res = &result
	removed := false
	for i, h := range result.hits {
		if !h.def && h.path == rn.sym.q.file.Path {
			result.hits = append(result.hits[:i:i], result.hits[i+1:]...)
			removed = true
			break
		}
	}
	if !removed {
		t.Fatal("fixture has no own-package occurrence to omit")
	}
	recheckAllImporters = true
	defer func() { recheckAllImporters = false }()
	_, fullErr := rn.edits(context.Background())
	recheckAllImporters = false
	_, proofErr := rn.edits(context.Background())
	if fullErr == nil || proofErr == nil || fullErr.Error() != proofErr.Error() {
		t.Fatalf("missing occurrence: full=%v, proof=%v", fullErr, proofErr)
	}
}

func TestRenameBindingFactsRequireExactOrdinaryOrigin(t *testing.T) {
	rn := bindingProofRenamer(t)
	normal := rn.sym.q.view()
	if normal.pkg.IsVariant() || !normal.normalBindingFacts() {
		t.Fatal("eligible exact ordinary facts were disabled")
	}
	// Warm the immutable package memo, then reject each view-specific mismatch before consulting it.
	t.Run("inexact info", func(t *testing.T) {
		inexact := *normal
		info := *normal.info
		inexact.info = &info
		if inexact.normalBindingFacts() {
			t.Fatal("inexact type information reused normal facts")
		}
	})
	t.Run("inexact file set", func(t *testing.T) {
		inexact := *normal
		inexact.fset = token.NewFileSet()
		if inexact.normalBindingFacts() {
			t.Fatal("inexact file positions reused normal facts")
		}
	})
	t.Run("different syntax view", func(t *testing.T) {
		inexact := *normal
		inexact.files = make(map[string]*ast.File, len(normal.files))
		for path, file := range normal.files {
			inexact.files[path] = file
		}
		inexact.files[rn.sym.q.file.Path] = new(ast.File)
		if inexact.normalBindingFacts() {
			t.Fatal("a different syntax view reused normal facts")
		}
	})
	path := rn.sym.q.file.Path
	src, err := rn.e.snapshot.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		text []byte
	}{
		{"identical replacement bytes", append([]byte(nil), src...)},
		{"complete plain variant", append(append([]byte(nil), src...), []byte("\n// patched input\n")...)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for range 2 { // fresh computation and cached hit must retain the same immutable origin
				pkg, err := rn.e.snapshot.PackageWithFile(context.Background(), path, tc.text)
				if err != nil {
					t.Fatal(err)
				}
				if !pkg.IsVariant() || viewOf(pkg).normalBindingFacts() {
					t.Fatal("a complete plain patched package entered the ordinary binding proof")
				}
			}
		})
	}
	if !normal.normalBindingFacts() {
		t.Fatal("negative provenance checks poisoned the eligible ordinary package memo")
	}
}

func TestRenameBindingProofCancellation(t *testing.T) {
	rn := bindingProofRenamer(t)
	defer func() { recheckAllImporters = false }()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, all := range []bool{true, false} {
		recheckAllImporters = all
		_, err := rn.edits(ctx)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("full=%v: got %v, want context.Canceled", all, err)
		}
	}
	recheckAllImporters = false
}

// TestRenameQualifiedOnlyShortcut: the answer of a rename is what it is without
// the shortcut, also with the conflict checks off (then the check of the result
// is the only thing that refuses an error), and it checks fewer packages.
func TestRenameQualifiedOnlyShortcut(t *testing.T) {
	for _, noConflicts := range []bool{false, true} {
		t.Run(fmt.Sprintf("conflicts-off=%v", noConflicts), func(t *testing.T) {
			skipConflictChecks = noConflicts
			defer func() { skipConflictChecks = false }()
			testQualifiedOnly(t)
		})
	}
}

func testQualifiedOnly(t *testing.T) {
	for _, tc := range []struct {
		mark, name string
		fewer      bool // the shortcut checks fewer packages
	}{
		{"func |F()", "Zed", false}, // g dot-imports a and has Zed: it has no use of F, and still breaks
		{"var |V = 2", "Zed", false},
		{"func |F()", "G", true},          // b, c, e use it by a qualified name; d (dot) and a are checked
		{"var |V = 2", "W", true},         // a variable
		{"const |K = 3", "L", true},       // a constant
		{"func |F()", "g", false},         // not exported: the importers can't use it
		{"func |F()", "V", false},         // the name is taken in a (an error there)
		{"func |F()", "G", true},          // (again, in a fresh server)
		{"type |T struct", "U", true},     // a type nothing embeds: b uses it qualified
		{"func (t T) |M()", "N", false},   // a method: not shortcut
		{"func inA|() int", "inB", false}, // unexported: no importer
		{"var |V = 2", "F", false},        // clashes with F in a
		{"const |K = 3", "K2", true},      // a constant, another name
		{"func |F()", "Fn", true},         // f has Fn, but it uses a.F qualified: fine in f, and a is fine
		{"func |F()", "TypeName", true},   // any other name
		{"func |F()", "Call", true},       // b has Call: qualified use in b is fine
		{"var |V = 2", "Use", true},       // c has Use
		{"const |K = 3", "Dot", false},    // d dot-imports a and has Dot: the dot import clashes only if it is checked
		{"func |F()", "Alias", false},     // e has Alias, d dot-imports a: the dot-importer sees both
		{"var |V = 2", "Dot", false},      // the same for a variable
		{"func |F()", "x", false},         // lowercase
		{"const |K = 3", "NoSuchName", true},
		{"func |F()", "MixedFunc", false}, // qualified uses and a dot import are in different files
		{"var |V = 2", "MixedVar", false},
		{"const |K = 3", "MixedConst", false},
		{"func |F()", "MixedSafe", true}, // mixed imports without a clash still give the same edits
	} {
		t.Run(fmt.Sprintf("%s->%s", strings.Trim(tc.mark, "|"), tc.name), func(t *testing.T) {
			full, nFull := fastRename(t, true, tc.mark, tc.name)
			fast, nFast := fastRename(t, false, tc.mark, tc.name)
			mixedClash := tc.name == "MixedFunc" || tc.name == "MixedVar" || tc.name == "MixedConst"
			if mixedClash && !strings.HasPrefix(full, "error:") {
				t.Fatal("mixed-file dot-import conflict was not refused")
			}
			t.Logf("without: %d checks; with: %d checks; %.60q", nFull, nFast, fast)
			if full != fast {
				t.Errorf("the shortcut changes the answer\nwithout:\n%s\nwith:\n%s", full, fast)
			}
			if tc.fewer && nFast >= nFull {
				t.Errorf("%d package checks with the shortcut, %d without: want fewer", nFast, nFull)
			}
			if !tc.fewer && nFast > nFull {
				t.Errorf("%d package checks with the shortcut, %d without", nFast, nFull)
			}
		})
	}
}

func TestRenameTypeProofHelpers(t *testing.T) {
	files := bindingProofFiles()
	files["a/a.gp"] += "\ntype G[P any] struct{ Item P }\n\ntype Alias = T\n"
	files["b/b.gp"] += "\ntype Outer struct{ *alias.T }\n"
	for _, tc := range []struct {
		mark                string
		ordinary, embedding bool
	}{
		{"type |T struct", true, true},
		{"type |G[", true, false},
		{"type |Alias =", false, false},
	} {
		t.Run(tc.mark, func(t *testing.T) {
			rn := renamerFor(t, files, "a/a.gp", tc.mark, "Renamed")
			tn, ok := rn.sym.objs[0].(*types.TypeName)
			if !ok {
				t.Fatalf("not a type: %v", rn.sym.objs[0])
			}
			var declaration hit
			for _, h := range rn.res.hits {
				if h.def && filepath.Dir(h.path) == rn.sym.declDir {
					declaration = h
				}
			}
			file := declaration.v.files[declaration.path]
			if got := ordinaryType(tn, file, declaration.id); got != tc.ordinary {
				t.Errorf("ordinaryType = %v, want %v", got, tc.ordinary)
			}
			if got := rn.embedsType(tn); got != tc.embedding {
				t.Errorf("embedsType = %v, want %v", got, tc.embedding)
			}
		})
	}
}

func TestHasMacroName(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want bool
	}{
		{"no hash", "package a\nvar V = 1\n", false},
		{"string", "package a\nvar V = \"#x(y)\"\n", false},
		{"raw string", "package a\nvar V = `#z`\n", false},
		{"character", "package a\nvar V = '#'\n", false},
		{"line comment", "package a\n// issue #12\nvar V = 1\n", false},
		{"block comment", "package a\n/* #file() */\nvar V = 1\n", false},
		{"builtin macro", "package a\nvar V = #file()\n", true},
		{"macro use", "package a\nfunc F() { #mac.Dbg(1) }\n", true},
		{"hash then string", "package a\nvar V = \"#\" + #line()\n", true},
		{"bare hash", "package a\nvar V = # 1\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hasMacroName([]byte(tc.src)); got != tc.want {
				t.Fatalf("hasMacroName = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestImportNameProofChecksTheFile: with the outcome of the guards assumed, the proof of the
// rename of an import holds exactly where nothing in the file or the package scope has the new
// name.
func TestImportNameProofChecksTheFile(t *testing.T) {
	for _, tc := range []struct {
		name    string
		setup   func(map[string]string)
		mark    string
		newName string
		proved  bool
	}{
		{name: "nothing of the new name", proved: true},
		{name: "a declaration of the package", proved: false, setup: func(files map[string]string) {
			files["b/b.gp"] += "\nvar Renamed = 1\n"
		}},
		{name: "a declaration of another file of the package", proved: false,
			setup: func(files map[string]string) {
				files["b/other.gp"] = "package b\nvar Renamed = 1\n"
			}},
		{name: "a local that hides the import", proved: false, setup: func(files map[string]string) {
			files["b/b.gp"] += "\nfunc Hide() int {\n\tRenamed := 2\n\treturn alias.F(1) + Renamed\n}\n"
		}},
		{name: "a parameter of the new name", proved: false, setup: func(files map[string]string) {
			files["b/b.gp"] += "\nfunc Param(Renamed int) int { return alias.F(Renamed) }\n"
		}},
		{name: "a use of a predeclared name", newName: "len", proved: false,
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\nfunc Size(s string) int { return len(s) }\n"
			}},
		{name: "another import of the name", newName: "fmt", proved: false,
			mark: "|alias \"example",
			setup: func(files map[string]string) {
				files["b/b.gp"] = strings.Replace(files["b/b.gp"], "import alias \"example.com/binding/a\"",
					"import (\n\t\"fmt\"\n\talias \"example.com/binding/a\"\n)", 1) +
					"\nfunc Show() { fmt.Println() }\n"
			}},
		{name: "a local of another file", proved: true, setup: func(files map[string]string) {
			files["b/other.gp"] = "package b\nfunc Other() int {\n\tRenamed := 1\n\treturn Renamed\n}\n"
		}},
		{name: "a field of the new name", newName: "Item", proved: true,
			setup: func(files map[string]string) {
				files["b/b.gp"] += "\ntype Box struct{ Item int }\n\nvar Held = Box{Item: alias.F(1)}\n"
			}},
		{name: "a name that a dot import brings", newName: "K", proved: false,
			mark: "|alias \"example",
			setup: func(files map[string]string) {
				// The file dot-imports a, which has K, and imports it by a name too. The package
				// declares nothing that a has, so the check has no error: only the file scope of K
				// tells that the new name is taken.
				files["b/b.gp"] = "package b\n\nimport (\n\t. \"example.com/binding/a\"\n" +
					"\talias \"example.com/binding/a\"\n)\n\nfunc Mine() int { return alias.F(1) + V }\n"
			}},
		{name: "a test file of the package that declares the new name", proved: false,
			setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\nvar Renamed = 1\n"
			}},
		{name: "a test file of the package that declares something else", proved: false,
			setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b\n\nvar Other = 1\n"
			}},
		{name: "an external test file of the package", proved: false,
			setup: func(files map[string]string) {
				files["b/b_test.gp"] = "package b_test\n\nvar Other = 1\n"
			}},
		{name: "a test file of another package", proved: true, setup: func(files map[string]string) {
			files["a/a_test.gp"] = "package a\n\nvar Renamed = 1\n"
		}},
		{name: "a blank name", newName: "_", proved: false},
		{name: "init", newName: "init", proved: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := bindingProofFiles()
			if tc.setup != nil {
				tc.setup(files)
			}
			newName := tc.newName
			if newName == "" {
				newName = "Renamed"
			}
			mark := tc.mark
			if mark == "" {
				mark = "import |alias"
			}
			rn := renamerFor(t, files, "b/b.gp", mark, newName)
			if rn.importNameProvesResult(context.Background(), map[string]bool{rn.sym.declDir: true}) {
				t.Fatal("a rename without the guards is proved")
			}
			rn.bindingsChecked = true
			dirs := map[string]bool{rn.sym.declDir: true}
			if got := rn.importNameProvesResult(context.Background(), dirs); got != tc.proved {
				t.Fatalf("proved = %v, want %v", got, tc.proved)
			}
		})
	}
}

// TestImportNameProofNeedsEveryOccurrence: a use of the import, or an import without a name, that
// the search did not find keeps the scratch check.
func TestImportNameProofNeedsEveryOccurrence(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(map[string]string)
		mark  string
		drop  func(hit) bool
	}{
		{name: "a use", mark: "import |alias", drop: func(h hit) bool { return !h.def && h.spec == nil }},
		{name: "an import without a name", mark: "return |a.F(1)", setup: func(files map[string]string) {
			files["b/b.gp"] = "package b\nimport \"example.com/binding/a\"\n" +
				"func Use() int { return a.F(1) }\n"
		}, drop: func(h hit) bool { return h.spec != nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := bindingProofFiles()
			if tc.setup != nil {
				tc.setup(files)
			}
			rn := renamerFor(t, files, "b/b.gp", tc.mark, "Renamed")
			rn.bindingsChecked = true
			dirs := map[string]bool{rn.sym.declDir: true}
			if !rn.importNameProvesResult(context.Background(), dirs) {
				t.Fatal("the complete search is not proved")
			}
			partial := *rn.res
			partial.hits = nil
			dropped := false
			for _, h := range rn.res.hits {
				if !dropped && tc.drop(h) {
					dropped = true
					continue
				}
				partial.hits = append(partial.hits, h)
			}
			if !dropped {
				t.Fatal("the fixture has nothing to drop")
			}
			rn.res = &partial
			if rn.importNameProvesResult(context.Background(), dirs) {
				t.Fatal("a search without one occurrence is proved")
			}
		})
	}
}
