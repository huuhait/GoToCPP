package main

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// The tests of gocpp list (list.go, Phase 06 step 6.12): the packages of a module with files,
// imports, C and C++ headers and directives, in the formats of go list.

func listProject() map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": `//gocpp:cflags -DONE=1
//gocpp:link m
//gocpp:pkg-config zlib

package main

import (
	"fmt"

	"example.com/m/a"
	"<vector>"
)

func main() { fmt.Println(a.F()) }
`,
		"main_test.gp":  "package main\n",
		"only_linux.gp": "package main\n",
		"only_other.gp": "//go:build otherplatform\n\npackage main\n",
		"main_extra.gp": "//go:build extra\n\npackage main\n\nconst Extra = 1\n",
		"helper.c":      "int helper(void) { return 1; }\n",
		"helper.cpp":    "int helper2() { return 2; }\n",
		"a/a.gp":        "package a\n\nimport \"<cstdio>\"\n\nfunc F() int { return 1 }\n",
		"b/b.gp":        "package b\n\nimport \"example.com/m/a\"\n\nfunc G() int { return a.F() }\n",
		"broken/x.gp":   "package broken\n\nfunc F() int { return \n",
	}
}

func listIn(t *testing.T, args ...string) result {
	t.Helper()
	return gocppRun("", false, append([]string{"list"}, args...)...)
}

func TestListDefault(t *testing.T) {
	inDir(t, listProject())
	if r := listIn(t, "./a", "./b"); r != (result{0, "example.com/m/a\nexample.com/m/b\n", ""}) {
		t.Errorf("gocpp list ./a ./b = %+v", r)
	}
	// Without arguments: the package of the current folder.
	if r := listIn(t); r.code != 0 || r.stdout != "example.com/m\n" {
		t.Errorf("gocpp list = %+v", r)
	}
	t.Chdir("a")
	if r := listIn(t); r != (result{0, "example.com/m/a\n", ""}) {
		t.Errorf("gocpp list in a/ = %+v", r)
	}
}

func TestListPatternSorted(t *testing.T) {
	inDir(t, listProject())
	r := listIn(t, "./a/...", "./b/...")
	if r != (result{0, "example.com/m/a\nexample.com/m/b\n", ""}) {
		t.Errorf("gocpp list patterns = %+v", r)
	}
	// The order of the patterns is the order of the output.
	if r := listIn(t, "./b", "./a"); r.stdout != "example.com/m/b\nexample.com/m/a\n" {
		t.Errorf("gocpp list ./b ./a = %+v", r)
	}
}

func TestListJSON(t *testing.T) {
	dir := inDir(t, listProject())
	r := listIn(t, "-json", ".")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("gocpp list -json . = %+v", r)
	}
	var p listPackage
	if err := json.Unmarshal([]byte(r.stdout), &p); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, r.stdout)
	}
	want := func(name string, got, want []string) {
		if strings.Join(got, "|") != strings.Join(want, "|") {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if p.ImportPath != "example.com/m" || p.Name != "main" || filepath.Base(p.Dir) != filepath.Base(dir) {
		t.Errorf("ImportPath, Name, Dir = %q %q %q", p.ImportPath, p.Name, p.Dir)
	}
	want("GpFiles", p.GpFiles, []string{"main.gp", "only_linux.gp"})
	want("IgnoredGpFiles", p.IgnoredGpFiles, []string{"main_extra.gp", "only_other.gp"})
	want("TestGpFiles", p.TestGpFiles, []string{"main_test.gp"})
	want("CFiles", p.CFiles, []string{"helper.c"})
	want("CXXFiles", p.CXXFiles, []string{"helper.cpp"})
	want("CImports", p.CImports, []string{"<vector>"})
	want("Imports", p.Imports, []string{"example.com/m/a", "fmt"})
	want("CFlags", p.CFlags, []string{"-DONE=1"})
	want("Link", p.Link, []string{"m"})
	want("PkgConfig", p.PkgConfig, []string{"zlib"})
	if p.Module == nil || p.Module.Path != "example.com/m" || !p.Module.Main || filepath.Base(p.Module.GoMod) != "gocpp.mod" {
		t.Errorf("Module = %+v", p.Module)
	}
	if p.Standard || p.DepOnly || p.Incomplete || p.Error != nil {
		t.Errorf("flags: %+v", p)
	}
}

func TestListJSONOmitsEmpty(t *testing.T) {
	inDir(t, listProject())
	r := listIn(t, "-json", "./b")
	for _, field := range []string{`"CFiles"`, `"CXXFiles"`, `"CImports"`, `"CFlags"`, `"Error"`, `"Standard"`, `"DepOnly"`, `"Incomplete"`} {
		if strings.Contains(r.stdout, field) {
			t.Errorf("an empty field %s is in the JSON:\n%s", field, r.stdout)
		}
	}
	for _, field := range []string{`"ImportPath"`, `"Dir"`, `"GpFiles"`, `"Deps"`} {
		if !strings.Contains(r.stdout, field) {
			t.Errorf("the field %s is not in the JSON:\n%s", field, r.stdout)
		}
	}
}

func TestListFormat(t *testing.T) {
	inDir(t, listProject())
	r := listIn(t, "-f", "{{.ImportPath}} {{.Name}} {{join .GpFiles \",\"}} [{{join .CImports \" \"}}]", ".", "./a")
	if want := "example.com/m main main.gp,only_linux.gp [<vector>]\nexample.com/m/a a a.gp [<cstdio>]\n"; r != (result{0, want, ""}) {
		t.Errorf("gocpp list -f = %+v, want %q", r, want)
	}
	if r := listIn(t, "-f", "{{.Module.Path}}", "./a"); r.stdout != "example.com/m\n" {
		t.Errorf("gocpp list -f module = %+v", r)
	}
	if r := listIn(t, "-f", "{{json .Imports}}", "./b"); r.stdout != "[\"example.com/m/a\"]\n" {
		t.Errorf("gocpp list -f json = %+v", r)
	}
}

func TestListFormatErrors(t *testing.T) {
	inDir(t, listProject())
	if r := listIn(t, "-f", "{{.Nope}}", "./a"); r.code != exitErrors && r.code != 1 || !strings.Contains(r.stderr, "Nope") {
		t.Errorf("gocpp list -f {{.Nope}} = %+v, want an error that names the field", r)
	}
	if r := listIn(t, "-f", "{{", "./a"); r.code != 2 || !strings.Contains(r.stderr, "-f:") {
		t.Errorf("gocpp list -f '{{' = %+v", r)
	}
	if r := listIn(t, "-json", "-f", "x", "./a"); r.code != 2 || !strings.Contains(r.stderr, "-f cannot be used with -json") {
		t.Errorf("gocpp list -json -f = %+v", r)
	}
}

func TestListDeps(t *testing.T) {
	inDir(t, listProject())
	r := listIn(t, "-deps", "-f", "{{.ImportPath}} {{.DepOnly}}", "./b")
	lines := strings.Split(strings.TrimSpace(r.stdout), "\n")
	if r.code != 0 || len(lines) < 2 {
		t.Fatalf("gocpp list -deps = %+v", r)
	}
	// A dependency comes before the package that imports it, and only the named one is not DepOnly.
	last := lines[len(lines)-1]
	if last != "example.com/m/b false" {
		t.Errorf("the last line = %q, want the named package, not DepOnly", last)
	}
	if !strings.Contains(r.stdout, "example.com/m/a true\n") {
		t.Errorf("the dependency a is not listed as DepOnly:\n%s", r.stdout)
	}
	if strings.Index(r.stdout, "example.com/m/a ") > strings.Index(r.stdout, "example.com/m/b ") {
		t.Errorf("a dependency after the package that imports it:\n%s", r.stdout)
	}
}

func TestListTags(t *testing.T) {
	inDir(t, listProject())
	files := func(args ...string) string {
		r := listIn(t, append(args, "-f", "{{join .GpFiles \",\"}}|{{join .IgnoredGpFiles \",\"}}", ".")...)
		if r.code != 0 {
			t.Fatalf("gocpp list %v = %+v", args, r)
		}
		return strings.TrimSpace(r.stdout)
	}
	if got, want := files(), "main.gp,only_linux.gp|main_extra.gp,only_other.gp"; got != want {
		t.Errorf("no tags: %q, want %q", got, want)
	}
	if got, want := files("-tags", "extra"), "main.gp,main_extra.gp,only_linux.gp|only_other.gp"; got != want {
		t.Errorf("-tags extra: %q, want %q", got, want)
	}
	if got, want := files("-tags", "extra,otherplatform"), "main.gp,main_extra.gp,only_linux.gp,only_other.gp|"; got != want {
		t.Errorf("-tags extra,otherplatform: %q, want %q", got, want)
	}
}

func TestListErrors(t *testing.T) {
	inDir(t, listProject())
	// A package with a syntax error: the errors, no list, status 1.
	r := listIn(t, "./a", "./broken")
	if r.code != 1 || r.stdout != "" || !strings.Contains(r.stderr, "broken") {
		t.Errorf("gocpp list ./a ./broken = %+v", r)
	}
	// -e: the error is in the package, the list goes on, the status is 0 (like go list -e).
	r = listIn(t, "-e", "-json", "./a", "./broken")
	if r.code != 0 {
		t.Errorf("gocpp list -e status = %d, want 0", r.code)
	}
	if !strings.Contains(r.stdout, `"ImportPath": "example.com/m/a"`) || !strings.Contains(r.stdout, `"Error"`) || !strings.Contains(r.stdout, `"Incomplete": true`) {
		t.Errorf("gocpp list -e -json:\n%s", r.stdout)
	}
	// A pattern that matches nothing.
	if r := listIn(t, "./nothing"); r.code == 0 {
		t.Errorf("gocpp list ./nothing = %+v, want failure", r)
	}
}

func TestListFlagErrors(t *testing.T) {
	inDir(t, listProject())
	if r := listIn(t, "-nope"); r.code != 2 || !strings.Contains(r.stderr, "flag provided but not defined: -nope") {
		t.Errorf("gocpp list -nope = %+v", r)
	}
	if r := listIn(t, "-tags", "bad tag!", "."); r.code != 2 || !strings.Contains(r.stderr, "invalid build tag") {
		t.Errorf("gocpp list -tags 'bad tag!' = %+v", r)
	}
	if r := listIn(t, "-h"); r.code != 0 || !strings.Contains(r.stderr, "usage: gocpp list") {
		t.Errorf("gocpp list -h = %+v", r)
	}
}

func TestListFiles(t *testing.T) {
	// .gp files named on the command line: the package of go's "command-line-arguments".
	inDir(t, map[string]string{"x.gp": "package main\n\nfunc main() {}\n", "y.gp": "package main\n"})
	r := listIn(t, "-f", "{{.ImportPath}} {{join .GpFiles \",\"}}|{{join .IgnoredGpFiles \",\"}}", "x.gp")
	if r != (result{0, "command-line-arguments x.gp|\n", ""}) {
		t.Errorf("gocpp list x.gp = %+v", r)
	}
}

func TestListStd(t *testing.T) {
	inDir(t, listProject())
	r := listIn(t, "-f", "{{.ImportPath}} {{.Standard}} {{.Module}}", "fmt")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "fmt true") {
		t.Errorf("gocpp list fmt = %+v", r)
	}
}

func TestListDeterministic(t *testing.T) {
	inDir(t, listProject())
	a := listIn(t, "-json", "-deps", "./b")
	for range 3 {
		if b := listIn(t, "-json", "-deps", "./b"); b != a {
			t.Fatalf("two runs of gocpp list differ:\n%s\n%s", a.stdout, b.stdout)
		}
	}
}

func TestListMisplacedDirectiveSaysWhy(t *testing.T) {
	inDir(t, map[string]string{"gocpp.mod": "module example.com/m\n", "main.gp": "package main\n\n//gocpp:link m\n\nfunc main() {}\n"})
	r := listIn(t, ".")
	if r.code != 1 || !strings.Contains(r.stderr, "directives must come before the package clause") {
		t.Errorf("gocpp list = %+v, want the reason of the failure", r)
	}
}
