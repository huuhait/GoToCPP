package load

import (
	"go/ast"
	"path/filepath"
	"testing"
)

// The tests of Config.SkipBodies: the files that it names are parsed without the bodies of their
// functions.

func skipBodiesTree(t *testing.T) string {
	t.Helper()
	return writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a.gp":      "package main\n\nfunc a() int {\n\treturn 1\n}\n\nfunc init() {\n\ta()\n}\n",
		"b.gp":      "package main\n\nfunc main() {\n\t_ = a()\n}\n",
	})
}

// bodiesOf returns whether each function of the files of the root package has a body, by
// "file.func".
func bodiesOf(prog *Program) map[string]bool {
	got := make(map[string]bool)
	for _, file := range prog.Roots[0].Files {
		for _, decl := range file.AST.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				got[filepath.Base(file.Path)+"."+fn.Name.Name] = fn.Body != nil
			}
		}
	}
	return got
}

func TestSkipBodiesNamedFilesOnly(t *testing.T) {
	root := skipBodiesTree(t)
	skip := func(path string) bool { return filepath.Base(path) == "b.gp" }

	got := bodiesOf(loadIn(t, Config{SkipBodies: skip}, root, "."))

	if !got["a.gp.a"] {
		t.Errorf("a.gp lost a body: %v", got)
	}
	if got["b.gp.main"] {
		t.Errorf("b.gp kept a body: %v", got)
	}
}

func TestSkipBodiesKeepsInit(t *testing.T) {
	root := skipBodiesTree(t)

	got := bodiesOf(loadIn(t, Config{SkipBodies: func(string) bool { return true }}, root, "."))

	if !got["a.gp.init"] || got["a.gp.a"] {
		t.Errorf("bodies: %v", got)
	}
}

func TestSkipBodiesOffByDefault(t *testing.T) {
	root := skipBodiesTree(t)

	for name, has := range bodiesOf(loadIn(t, Config{}, root, ".")) {
		if !has {
			t.Errorf("%s has no body", name)
		}
	}
}

func TestSkipBodiesMarksTheFiles(t *testing.T) {
	root := skipBodiesTree(t)
	skip := func(path string) bool { return filepath.Base(path) == "b.gp" }

	prog := loadIn(t, Config{SkipBodies: skip}, root, ".")

	got := make(map[string]bool)
	for _, file := range prog.Roots[0].Files {
		got[filepath.Base(file.Path)] = file.BodiesSkipped
	}
	if got["a.gp"] || !got["b.gp"] {
		t.Errorf("BodiesSkipped by file: %v, want only b.gp", got)
	}
}

func TestSkipBodiesOffMarksNoFile(t *testing.T) {
	root := skipBodiesTree(t)

	prog := loadIn(t, Config{}, root, ".")

	for _, file := range prog.Roots[0].Files {
		if file.BodiesSkipped {
			t.Errorf("%s is marked as skipped without SkipBodies", filepath.Base(file.Path))
		}
	}
}
