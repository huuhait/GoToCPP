package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// docMod is the module of the tests of internal/doc.
func docMod(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../../internal/doc/testdata/gpmod")
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestDocPackage(t *testing.T) {
	t.Chdir(docMod(t))
	code, out, errOut := gocpp("doc", "shapes.Stack.Push")
	want := "package shapes // import \"example.com/gp/shapes\"\n\nfunc (s *Stack[T]) Push(v T)\n    Push adds v on top.\n\n"
	if code != 0 || errOut != "" || out != want {
		t.Errorf("exit %d, stderr %q, stdout\n%s\nwant\n%s", code, errOut, out, want)
	}
}

func TestDocFlags(t *testing.T) {
	t.Chdir(docMod(t))
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{"doc", "-short", "plat"}, "func Always()\nfunc OnlyLinux()\n"},
		{[]string{"doc", "-u", "shapes", "helper"}, "func helper()"},
		{[]string{"doc", "-c", "shapes", "circle"}, ""},
		{[]string{"doc", "-src", "macros.Parse"}, "func Parse!(s string) int {"},
		{[]string{"doc", "-cmd", "cmdtool"}, "func Helper()"},
		{[]string{"doc", "-all", "macros.Point"}, "#Derive(String, Equal, Clone)"},
		{[]string{"doc", "-examples", "examples.Add"}, "ExampleAdd"},
		{[]string{"doc", "./physics/physics.h", "vec_add"}, "Adds two vectors."},
	} {
		code, out, errOut := gocpp(c.args...)
		if c.want == "" {
			if code != 1 || !strings.Contains(errOut, "gocpp doc: no symbol circle") {
				t.Errorf("%v: exit %d, stderr %q", c.args, code, errOut)
			}
			continue
		}
		if code != 0 || !strings.Contains(out, c.want) {
			t.Errorf("%v: exit %d, stderr %q, stdout\n%s\nwant %q", c.args, code, errOut, out, c.want)
		}
	}
}

func TestDocChdir(t *testing.T) {
	mod := docMod(t)
	t.Chdir(t.TempDir())
	code, out, errOut := gocpp("doc", "-C", mod, "-short", "shapes.Level")
	if code != 0 || errOut != "" || !strings.Contains(out, "type Level uint8") {
		t.Errorf("exit %d, stderr %q, stdout\n%s", code, errOut, out)
	}
	code, _, errOut = gocpp("doc", "-C", filepath.Join(mod, "nosuchfolder"), "x")
	if code != 1 || !strings.Contains(errOut, "gocpp doc:") {
		t.Errorf("bad -C: exit %d, stderr %q", code, errOut)
	}
}

func TestDocStandardLibrary(t *testing.T) {
	t.Chdir(t.TempDir())
	code, out, errOut := gocpp("doc", "errors.New")
	if code != 0 || errOut != "" || !strings.Contains(out, "func New(text string) error") {
		t.Errorf("exit %d, stderr %q, stdout\n%s", code, errOut, out)
	}
}

func TestDocExitStatuses(t *testing.T) {
	t.Chdir(docMod(t))
	for _, c := range []struct {
		args   []string
		code   int
		stderr string
	}{
		{[]string{"doc", "a", "b", "c"}, 2, "gocpp doc: too many arguments\nusage: gocpp doc [flags]"},
		{[]string{"doc", "shapes", "a.b.c"}, 2, "too many periods"},
		{[]string{"doc", "-nosuchflag"}, 2, "flag provided but not defined: -nosuchflag"},
		{[]string{"doc", "shapes", "Nope"}, 1, "gocpp doc: no symbol Nope in package example.com/gp/shapes\n"},
		{[]string{"doc", "./physics/nope.h"}, 1, "no such file"},
	} {
		code, out, errOut := gocpp(c.args...)
		if code != c.code || !strings.Contains(errOut, c.stderr) || (code != 0 && out != "") {
			t.Errorf("%v: exit %d, stdout %q, stderr %q; want exit %d and stderr with %q", c.args, code, out, errOut, c.code, c.stderr)
		}
	}
}

func TestDocHelp(t *testing.T) {
	code, out, errOut := gocpp("doc", "-h")
	if code != 0 || out != "" {
		t.Errorf("exit %d, stdout %q", code, out)
	}
	for _, w := range []string{"usage: gocpp doc [flags]", "-all", "-src", "-short", "-u", "-cmd", "-c", "A C or C++ header is an opaque package"} {
		if !strings.Contains(errOut, w) {
			t.Errorf("the help lacks %q:\n%s", w, errOut)
		}
	}
}
