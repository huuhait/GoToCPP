package load

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/load/gocppmod"
)

// The tests of step 6.1: gocpp.mod with require, replace and vendor/. A
// workspace is a folder with the main module in app/ and other modules
// next to it.

// mathx is a module with two packages; geom imports the other module vecs.
var mathx = map[string]string{
	"mathx/gocpp.mod":       "module github.com/huuhait/mathx\n\ngocpp 0.1\n\nrequire example.com/vecs v1.0.0\n",
	"mathx/mathx.gp":        "package mathx\n\nimport \"github.com/huuhait/mathx/geom\"\n\nvar Unit = geom.Origin\n",
	"mathx/geom/geom.gp":    "package geom\n\nimport \"example.com/vecs\"\n\nvar Origin = vecs.Zero\n",
	"mathx/internal/i/i.gp": "package i\n\nconst N = 1\n",
	"vecs/gocpp.mod":        "module example.com/vecs\n",
	"vecs/vecs.gp":          "package vecs\n\nconst Zero = 0\n",
}

const appMain = `package main

import (
	"fmt"

	"github.com/huuhait/mathx"
	"github.com/huuhait/mathx/geom"
)

func main() { fmt.Println(mathx.Unit, geom.Origin) }
`

func workspace(t *testing.T, appMod string, extra ...map[string]string) string {
	t.Helper()
	files := map[string]string{"app/gocpp.mod": appMod, "app/main.gp": appMain}
	for k, v := range mathx {
		files[k] = v
	}
	for _, m := range extra {
		for k, v := range m {
			files[k] = v
		}
	}
	return writeTree(t, files)
}

const appModReplace = `module example.com/app

require github.com/huuhait/mathx v0.3.0
require example.com/vecs v1.0.0 // indirect

replace github.com/huuhait/mathx => ../mathx
replace example.com/vecs => ../vecs
`

func TestRequireReplace(t *testing.T) {
	root := workspace(t, appModReplace)
	app := filepath.Join(root, "app")
	prog := loadIn(t, Config{}, app, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"example.com/vecs vecs: vecs.gp",
		"fmt fmt: print.gp",
		"github.com/huuhait/mathx/geom geom: geom.gp; example.com/vecs",
		"github.com/huuhait/mathx mathx: mathx.gp; github.com/huuhait/mathx/geom",
		"example.com/app main: main.gp; fmt github.com/huuhait/mathx github.com/huuhait/mathx/geom",
	)
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
	// The module of each package: the main module for app, the replacement
	// for the others, with the version of the require line.
	main := pkgOf(t, prog, "example.com/app")
	if m := main.Module; m.Path != "example.com/app" || m.Dir != app || m.Version != "" || m.Vendored || m.File() == nil {
		t.Errorf("main module = %+v", m)
	}
	for path, want := range map[string]struct{ mod, dir, version string }{
		"github.com/huuhait/mathx":      {"github.com/huuhait/mathx", filepath.Join(root, "mathx"), "v0.3.0"},
		"github.com/huuhait/mathx/geom": {"github.com/huuhait/mathx", filepath.Join(root, "mathx"), "v0.3.0"},
		"example.com/vecs":              {"example.com/vecs", filepath.Join(root, "vecs"), "v1.0.0"},
	} {
		p := pkgOf(t, prog, path)
		m := p.Module
		if m == nil || m.Path != want.mod || m.Dir != want.dir || m.Version != want.version || m.Vendored || m.File() == nil {
			t.Errorf("%s: module %+v", path, m)
		}
		if p.Std || p.Dir != filepath.Join(want.dir, strings.TrimPrefix(strings.TrimPrefix(path, want.mod), "/")) {
			t.Errorf("%s: dir %s", path, p.Dir)
		}
	}
	// The files of a replaced module are in the program.
	if f := pkgOf(t, prog, "example.com/vecs").Files[0].Path; f != filepath.Join(root, "vecs", "vecs.gp") {
		t.Errorf("vecs file %s", f)
	}
}

// TestReplaceAbsolute checks an absolute replacement, and that the folder of
// the main module can be anywhere: the replacement is relative to it.
func TestReplaceAbsolute(t *testing.T) {
	root := workspace(t, "")
	mod := "module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nrequire example.com/vecs v1.0.0\n" +
		"replace github.com/huuhait/mathx => " + filepath.Join(root, "mathx") + "\nreplace example.com/vecs => ../vecs/../vecs\n"
	if err := os.WriteFile(filepath.Join(root, "app", "gocpp.mod"), []byte(mod), 0o644); err != nil {
		t.Fatal(err)
	}
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
	if m := pkgOf(t, prog, "example.com/vecs").Module; m.Dir != filepath.Join(root, "vecs") {
		t.Errorf("a replacement with .. is not cleaned: %s", m.Dir)
	}
}

// TestReplaceByVersion checks that a replace line with a version replaces
// only that version, and that the line for the version wins over one for all.
func TestReplaceByVersion(t *testing.T) {
	root := workspace(t, `module example.com/app
require github.com/huuhait/mathx v0.3.0
require example.com/vecs v1.0.0
replace github.com/huuhait/mathx v0.2.0 => ../nowhere
replace github.com/huuhait/mathx v0.3.0 => ../mathx
replace example.com/vecs => ../nowhere
replace example.com/vecs v1.0.0 => ../vecs
`)
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")

	// Only the version that is not required: the replace does not apply.
	root = workspace(t, "module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nreplace github.com/huuhait/mathx v0.2.0 => ../mathx\n")
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, `# example.com/app
app/main.gp:6:2: package github.com/huuhait/mathx: module github.com/huuhait/mathx@v0.3.0 is required, but is not available: gocpp can't download modules yet (replace it with a local folder: replace github.com/huuhait/mathx => ../dir, or vendor it)
app/main.gp:7:2: package github.com/huuhait/mathx/geom: module github.com/huuhait/mathx@v0.3.0 is required, but is not available: gocpp can't download modules yet (replace it with a local folder: replace github.com/huuhait/mathx => ../dir, or vendor it)
`)
}

func TestReplaceErrors(t *testing.T) {
	const head = "module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nrequire example.com/vecs v1.0.0\nreplace example.com/vecs => ../vecs\n"
	for _, tc := range []struct {
		name, mod string
		extra     map[string]string
		want      string // the error of the import of mathx; geom has the same
	}{
		{"not replaced", "module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nrequire example.com/vecs v1.0.0\n", nil,
			"module github.com/huuhait/mathx@v0.3.0 is required, but is not available: gocpp can't download modules yet " +
				"(replace it with a local folder: replace github.com/huuhait/mathx => ../dir, or vendor it)"},
		{"folder missing", head + "replace github.com/huuhait/mathx => ../missing\n", nil,
			"module github.com/huuhait/mathx is replaced by ../missing, but the replacement folder $ROOT/missing does not exist"},
		{"a file, not a folder", head + "replace github.com/huuhait/mathx => ../vecs/vecs.gp\n", nil,
			"module github.com/huuhait/mathx is replaced by ../vecs/vecs.gp, but the replacement folder $ROOT/vecs/vecs.gp does not exist"},
		{"no gocpp.mod", head + "replace github.com/huuhait/mathx => ../plain\n", map[string]string{"plain/p.gp": "package p\n"},
			"module github.com/huuhait/mathx is replaced by ../plain, which has no gocpp.mod file"},
		{"gocpp.mod in a parent only", head + "replace github.com/huuhait/mathx => ../mathx/geom\n", nil,
			"module github.com/huuhait/mathx is replaced by ../mathx/geom, which has no gocpp.mod file"},
		{"other module path", head + "replace github.com/huuhait/mathx => ../vecs\n", nil,
			"module github.com/huuhait/mathx is replaced by ../vecs, but the gocpp.mod there declares its path as example.com/vecs"},
		{"broken module line", head + "replace github.com/huuhait/mathx => ../broken\n", map[string]string{"broken/gocpp.mod": "gocpp 0.1\n"},
			"module github.com/huuhait/mathx is replaced by ../broken, whose gocpp.mod has no valid module line (see its errors)"},
		{"remote replacement", head + "replace github.com/huuhait/mathx => github.com/fork/mathx v0.3.1\n", nil,
			"module github.com/huuhait/mathx is replaced by github.com/fork/mathx@v0.3.1, which is not a local folder: gocpp can't download modules yet"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := workspace(t, tc.mod, tc.extra)
			prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
			got := errorText(t, prog, root)
			for _, p := range []string{"github.com/huuhait/mathx", "github.com/huuhait/mathx/geom"} {
				want := "package " + p + ": " + tc.want
				if !strings.Contains(got, want+"\n") {
					t.Errorf("errors:\n%s\nwant a line with:\n%s", got, want)
				}
			}
			if !prog.HasErrors() && !prog.Roots[0].Incomplete {
				t.Errorf("no errors")
			}
		})
	}
}

// TestRequireProblems checks the errors that are about gocpp.mod itself.
func TestRequireProblems(t *testing.T) {
	// Syntax errors: file:line:col like Go's, then the program goes on with
	// what could be read.
	root := workspace(t, "module example.com/app\nrequire github.com/huuhait/mathx v1\nreplace github.com/huuhait/mathx => ../mathx\nrequire (\n\texample.com/vecs\n)\n")
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	got := errorText(t, prog, root)
	for _, want := range []string{
		"app/gocpp.mod:2: require github.com/huuhait/mathx: invalid module version \"v1\": must be of the form v1.2.3\n",
		"app/gocpp.mod:5:2: usage: require module/path v1.2.3\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("errors:\n%s\nwant a line %q", got, want)
		}
	}
	if !prog.HasErrors() {
		t.Error("HasErrors is false")
	}

	// A replace line without a require line does not make the module
	// importable; the error says so.
	root = workspace(t, "module example.com/app\nreplace github.com/huuhait/mathx => ../mathx\n")
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, `# example.com/app
app/main.gp:6:2: no required module provides package github.com/huuhait/mathx; github.com/huuhait/mathx is replaced in gocpp.mod, but not required (add: require github.com/huuhait/mathx v0.0.0)
app/main.gp:7:2: no required module provides package github.com/huuhait/mathx/geom; github.com/huuhait/mathx is replaced in gocpp.mod, but not required (add: require github.com/huuhait/mathx v0.0.0)
`)

	// A module that the main module does not require, imported by a
	// required module: only the main module's lines count (no version
	// selection yet).
	root = workspace(t, "module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nreplace github.com/huuhait/mathx => ../mathx\n")
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, `# github.com/huuhait/mathx/geom
mathx/geom/geom.gp:3:8: no required module provides package example.com/vecs
`)
}

// TestLongestModule checks that of two modules whose paths contain the
// import path, the longer one has the package, also when it is nested in the
// main module or in a required module.
func TestLongestModule(t *testing.T) {
	root := writeTree(t, map[string]string{
		"app/gocpp.mod":     "module example.com/m\nrequire example.com/m/sub v1.0.0\nreplace example.com/m/sub => ../sub\n",
		"app/main.gp":       "package main\n\nimport (\n\t_ \"example.com/m/sub\"\n\t_ \"example.com/m/sub/x\"\n\t_ \"example.com/m/other\"\n)\n",
		"app/other/o.gp":    "package other\n",
		"app/sub/shadow.gp": "package shadow\n", // the folder of the main module is not used
		"sub/gocpp.mod":     "module example.com/m/sub\n",
		"sub/s.gp":          "package sub\n",
		"sub/x/x.gp":        "package x\n",
	})
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
	if got := pkgOf(t, prog, "example.com/m/sub").Dir; got != filepath.Join(root, "sub") {
		t.Errorf("sub is in %s", got)
	}
	if got := pkgOf(t, prog, "example.com/m/sub/x").Dir; got != filepath.Join(root, "sub", "x") {
		t.Errorf("sub/x is in %s", got)
	}
	if got := pkgOf(t, prog, "example.com/m/other").Module.Path; got != "example.com/m" {
		t.Errorf("other is in module %s", got)
	}
}

func TestRequiredPackageMissing(t *testing.T) {
	root := workspace(t, appModReplace)
	if err := os.WriteFile(filepath.Join(root, "app", "main.gp"), []byte(
		"package main\n\nimport (\n\t_ \"github.com/huuhait/mathx/nope\"\n\t_ \"github.com/huuhait/mathx/internal/i\"\n\t_ \"github.com/huuhait/mathx/geom\"\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, `# example.com/app
app/main.gp:4:2: module github.com/huuhait/mathx@v0.3.0 found ($ROOT/mathx), but does not contain package github.com/huuhait/mathx/nope
app/main.gp:5:2: use of internal package github.com/huuhait/mathx/internal/i not allowed
`)
}

// TestRequiredPattern loads a package of a required module as a root: its
// imports go through the main module's gocpp.mod.
func TestRequiredPattern(t *testing.T) {
	root := workspace(t, appModReplace)
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), "github.com/huuhait/mathx/geom")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"example.com/vecs vecs: vecs.gp",
		"github.com/huuhait/mathx/geom geom: geom.gp; example.com/vecs",
	)
	if got := prog.Roots[0].Module.Path; got != "github.com/huuhait/mathx" {
		t.Errorf("module %s", got)
	}
	// Not required: the error is go build's.
	root = workspace(t, "module example.com/app\n")
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), "github.com/huuhait/mathx/geom")
	wantErrors(t, prog, root, "no required module provides package github.com/huuhait/mathx/geom\n")
}

// vendored is the main module with vendor/: the modules of mathx and vecs
// copied under their module paths (without their gocpp.mod files, as Go does).
var vendored = map[string]string{
	"app/vendor/modules.txt": `# example.com/vecs v1.0.0
## explicit
example.com/vecs
# github.com/huuhait/mathx v0.3.0
## explicit
github.com/huuhait/mathx
github.com/huuhait/mathx/geom
`,
	"app/vendor/github.com/huuhait/mathx/mathx.gp":     mathx["mathx/mathx.gp"],
	"app/vendor/github.com/huuhait/mathx/geom/geom.gp": mathx["mathx/geom/geom.gp"],
	"app/vendor/example.com/vecs/vecs.gp":              mathx["vecs/vecs.gp"],
}

const appModVendor = `module example.com/app

require github.com/huuhait/mathx v0.3.0
require example.com/vecs v1.0.0
`

func TestVendor(t *testing.T) {
	root := workspace(t, appModVendor, vendored)
	app := filepath.Join(root, "app")
	prog := loadIn(t, Config{}, app, ".")
	wantErrors(t, prog, root, "")
	wantSummary(t, prog,
		"example.com/vecs vecs: vecs.gp",
		"fmt fmt: print.gp",
		"github.com/huuhait/mathx/geom geom: geom.gp; example.com/vecs",
		"github.com/huuhait/mathx mathx: mathx.gp; github.com/huuhait/mathx/geom",
		"example.com/app main: main.gp; fmt github.com/huuhait/mathx github.com/huuhait/mathx/geom",
	)
	if got := typeCheckProgram(t, prog, root); got != "" {
		t.Errorf("go/types errors:\n%s", got)
	}
	m := pkgOf(t, prog, "github.com/huuhait/mathx/geom").Module
	if m.Path != "github.com/huuhait/mathx" || m.Dir != filepath.Join(app, "vendor", "github.com", "huuhait", "mathx") ||
		m.Version != "v0.3.0" || !m.Vendored || m.File() != nil {
		t.Errorf("module %+v", m)
	}
	if got := pkgOf(t, prog, "github.com/huuhait/mathx").Module; got != m {
		t.Errorf("the packages of a module have different modules: %p %p", got, m)
	}
}

// TestVendorWins checks that with vendor/modules.txt the vendored copy is
// used, also for a replaced module and also when the replacement folder is
// not there: the vendor folder is for offline builds.
func TestVendorWins(t *testing.T) {
	root := workspace(t, appModVendor+"replace github.com/huuhait/mathx => ../gone\nreplace example.com/vecs => ../vecs\n",
		vendored)
	// The vendor file records the replacements.
	vendored := map[string]string{"app/vendor/modules.txt": `# example.com/vecs v1.0.0 => ../vecs
## explicit
example.com/vecs
# github.com/huuhait/mathx v0.3.0 => ../gone
## explicit
github.com/huuhait/mathx
github.com/huuhait/mathx/geom
`}
	for k, v := range vendored {
		if err := os.WriteFile(filepath.Join(root, k), []byte(v), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
	if m := pkgOf(t, prog, "example.com/vecs").Module; !m.Vendored {
		t.Errorf("vecs is not vendored: %+v", m)
	}
}

// TestVendorWithoutModulesTxt checks that a vendor folder without
// modules.txt is not vendoring: C and C++ projects keep third-party sources
// in a folder with that name.
func TestVendorWithoutModulesTxt(t *testing.T) {
	root := workspace(t, appModReplace, map[string]string{
		"app/vendor/sqlite/sqlite3.h": "int x;\n",
		"app/vendor/sqlite/sqlite3.c": "int x;\n",
	})
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
	if m := pkgOf(t, prog, "example.com/vecs").Module; m.Vendored || m.Dir != filepath.Join(root, "vecs") {
		t.Errorf("module %+v", m)
	}
}

func TestVendorErrors(t *testing.T) {
	vendorErr := func(problems ...string) string {
		return "app/vendor/modules.txt: inconsistent vendoring in $ROOT/app:\n\t" +
			strings.Join(problems, "\n\t") + "\n\n\tTo ignore the vendor folder, delete vendor/modules.txt.\n"
	}
	const notAvailable = ": module %s is not available: vendor/modules.txt does not match gocpp.mod (see its errors)\n"
	for _, tc := range []struct {
		name  string
		mod   string
		files map[string]string
		want  string
	}{
		{"module missing in modules.txt", appModVendor,
			map[string]string{"app/vendor/modules.txt": "# example.com/vecs v1.0.0\n## explicit\n"},
			vendorErr("github.com/huuhait/mathx@v0.3.0: is explicitly required in gocpp.mod, but not marked as explicit in vendor/modules.txt")},
		{"other version", appModVendor,
			map[string]string{"app/vendor/modules.txt": strings.Replace(vendored["app/vendor/modules.txt"], "v0.3.0", "v0.2.0", 1)},
			vendorErr("github.com/huuhait/mathx@v0.3.0: is required in gocpp.mod, but vendor/modules.txt has version v0.2.0",
				"github.com/huuhait/mathx@v0.2.0: is marked as explicit in vendor/modules.txt, but not explicitly required in gocpp.mod")},
		{"replace not recorded", appModVendor + "replace example.com/vecs => ../vecs\n", nil,
			vendorErr("example.com/vecs@v1.0.0: is replaced in gocpp.mod, but not marked as replaced in vendor/modules.txt")},
		{"malformed", appModVendor, map[string]string{"app/vendor/modules.txt": "# only-a-path\nexample.com/x\n"},
			"app/vendor/modules.txt:1: malformed line: # only-a-path\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{}
			for k, v := range vendored {
				files[k] = v
			}
			for k, v := range tc.files {
				files[k] = v
			}
			root := workspace(t, tc.mod, files)
			prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
			got := errorText(t, prog, root)
			if !strings.HasPrefix(got, tc.want) {
				t.Errorf("errors:\n%s\nwant a start of:\n%s", got, tc.want)
			}
			// Every import of a required module fails with a short message; the
			// details are reported once.
			wantLine := "app/main.gp:6:2: package github.com/huuhait/mathx" + strings.Replace(notAvailable, "%s", "github.com/huuhait/mathx@v0.3.0", 1)
			if !strings.Contains(got, wantLine) {
				t.Errorf("errors:\n%s\nwant a line:\n%s", got, wantLine)
			}
			if strings.Count(got, "inconsistent vendoring") > 1 {
				t.Errorf("reported more than once:\n%s", got)
			}
		})
	}

	// The package is not in the vendor folder: the module is listed, but its
	// folder is missing, or a package of it is.
	root := workspace(t, appModVendor, vendored)
	if err := os.RemoveAll(filepath.Join(root, "app", "vendor", "github.com", "huuhait", "mathx", "geom")); err != nil {
		t.Fatal(err)
	}
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, `# github.com/huuhait/mathx
app/vendor/github.com/huuhait/mathx/mathx.gp:3:8: package github.com/huuhait/mathx/geom is not in module github.com/huuhait/mathx ($ROOT/app/vendor/github.com/huuhait/mathx/geom)
# example.com/app
app/main.gp:7:2: module github.com/huuhait/mathx@v0.3.0 found ($ROOT/app/vendor/github.com/huuhait/mathx), but does not contain package github.com/huuhait/mathx/geom
`)
	root = workspace(t, appModVendor, vendored)
	if err := os.RemoveAll(filepath.Join(root, "app", "vendor", "example.com")); err != nil {
		t.Fatal(err)
	}
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	if got := errorText(t, prog, root); !strings.Contains(got,
		"package example.com/vecs: module example.com/vecs@v1.0.0 is listed in vendor/modules.txt, but the folder vendor/example.com/vecs does not exist\n") {
		t.Errorf("errors:\n%s", got)
	}
}

// TestVendorNotNeeded checks that a program that imports only the standard
// library and its own packages never reads vendor/modules.txt, so an
// inconsistent one is not reported (lazy, like the replace folders).
func TestVendorNotNeeded(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":          "module example.com/app\nrequire example.com/gone v1.0.0\nreplace example.com/gone => ./gone\n",
		"main.gp":            "package main\n\nimport _ \"example.com/app/p\"\n",
		"p/p.gp":             "package p\n",
		"vendor/modules.txt": "# nonsense\n",
	})
	wantErrors(t, loadIn(t, Config{}, root, "."), root, "")
}

func TestListModules(t *testing.T) {
	root := workspace(t, `module example.com/app
require github.com/huuhait/mathx v0.3.0
require example.com/vecs v1.0.0 // indirect
require example.com/far v2.0.0
require example.com/fork v1.0.0
replace github.com/huuhait/mathx => ../mathx
replace example.com/vecs => `+"../vecs"+`
replace example.com/fork => example.org/fork v1.0.1
`)
	ml := ListModules(Config{Dir: filepath.Join(root, "app")}, "sub/..")
	if ml.Main == nil || ml.Main.Path != "example.com/app" || ml.Main.Dir != filepath.Join(root, "app") {
		t.Fatalf("main = %+v", ml.Main)
	}
	type dep struct {
		path, version, replace, dir, err string
		indirect, vendored               bool
	}
	var got []dep
	for _, d := range ml.Deps {
		got = append(got, dep{d.Path, d.Version, d.Replace, d.Dir, d.Err, d.Indirect, d.Vendored})
	}
	want := []dep{
		{"github.com/huuhait/mathx", "v0.3.0", "../mathx", filepath.Join(root, "mathx"), "", false, false},
		{"example.com/vecs", "v1.0.0", "../vecs", filepath.Join(root, "vecs"), "", true, false},
		{"example.com/far", "v2.0.0", "", "", "module example.com/far@v2.0.0 is required, but is not available: gocpp can't download modules yet " +
			"(replace it with a local folder: replace example.com/far => ../dir, or vendor it)", false, false},
		{"example.com/fork", "v1.0.0", "example.org/fork v1.0.1", "", "module example.com/fork is replaced by example.org/fork@v1.0.1, which is not a local folder: gocpp can't download modules yet", false, false},
	}
	if len(got) != len(want) {
		t.Fatalf("deps = %+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("dep %d = %+v\nwant   %+v", i, got[i], want[i])
		}
	}
	if ml.Deps[0].Module == nil || ml.Deps[0].Module.Path != "github.com/huuhait/mathx" || ml.Deps[2].Module != nil {
		t.Errorf("modules: %+v %+v", ml.Deps[0].Module, ml.Deps[2].Module)
	}
	if len(ml.Errors.Diagnostics()) != 0 {
		t.Errorf("errors: %v", ml.Errors.Diagnostics())
	}

	// Vendored, and with an inconsistent modules.txt.
	root = workspace(t, appModVendor, vendored)
	ml = ListModules(Config{Dir: root}, "app")
	if len(ml.Deps) != 2 || !ml.Deps[0].Vendored || ml.Deps[0].Dir != filepath.Join(root, "app", "vendor", "github.com", "huuhait", "mathx") || ml.Deps[0].Err != "" {
		t.Errorf("vendored: %+v", ml.Deps)
	}
	root = workspace(t, appModVendor+"replace example.com/vecs => ../vecs\n", vendored)
	ml = ListModules(Config{Dir: root}, "app")
	if len(ml.Errors.Diagnostics()) != 1 || len(ml.Deps) != 2 || ml.Deps[1].Err == "" || ml.Deps[1].Dir != "" {
		t.Errorf("inconsistent: %d errors, %+v", len(ml.Errors.Diagnostics()), ml.Deps[1])
	}

	// Outside every module, and a module without a valid module line.
	if ml := ListModules(Config{Dir: t.TempDir()}, "."); ml.Main != nil || len(ml.Deps) != 0 {
		t.Errorf("outside a module: %+v", ml)
	}
	root = writeTree(t, map[string]string{"gocpp.mod": "require a v1.0.0\n"})
	if ml := ListModules(Config{Dir: root}, "."); ml.Main == nil || ml.Main.Path != "" || len(ml.Deps) != 0 || len(ml.Errors.Diagnostics()) == 0 {
		t.Errorf("no module line: %+v", ml)
	}
}

// TestHeadersUnchanged checks that the require, replace and vendor lines of
// gocpp.mod change nothing for the imports that are not packages: local
// headers (quoted), system headers (<...>), namespaces, and a header path
// that looks like a module header (Phase 06 §2.11: not resolved yet, the same
// as without the lines).
func TestHeadersUnchanged(t *testing.T) {
	const mainGp = `package main

import (
	"fmt"
	"<stdio.h>"
	"<vector>"
	"./native/x.h"
	"native/y.hpp"
	"github.com/huuhait/mathx/z.h"
	"example.com/other/w.hpp"
	"::std::filesystem"

	"example.com/app/sub"
	"github.com/huuhait/mathx"
)

func main() { fmt.Println(x.A, y.B, stdio.puts, std.vector[int]{}, sub.S, mathx.Unit) }
`
	files := map[string]string{
		"main.gp":         mainGp,
		"native/x.h":      "int A;\n",
		"native/y.hpp":    "int B;\n",
		"sub/sub.gp":      "package sub\n\nconst S = 1\n",
		"sub/local.h":     "int L;\n",
		"sub/other.gp":    "package sub\n\nimport \"./local.h\"\n\nvar _ = local.L\n",
		"vendor/c/c.h":    "int c;\n",
		"vendor/c/c.c":    "int c;\n",
		"mathx/mathx.gp":  "package mathx\n\nconst Unit = 0\n",
		"mathx/gocpp.mod": "module github.com/huuhait/mathx\n",
	}
	run := func(mod string) (string, []string) {
		fs := map[string]string{"gocpp.mod": mod}
		for k, v := range files {
			fs[k] = v
		}
		root := writeTree(t, fs)
		prog := loadIn(t, Config{}, root, ".")
		var inc []string
		for _, p := range prog.Packages {
			for _, f := range p.Files {
				for _, i := range f.Info.Includes {
					inc = append(inc, p.ImportPath+" "+i.Path+" name="+i.Name+" kind="+string(rune('0'+int(i.Kind))))
				}
			}
		}
		return errorText(t, prog, root), inc
	}
	plainErrs, plainInc := run("module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nreplace github.com/huuhait/mathx => ./mathx\n")
	if len(plainInc) < 7 {
		t.Fatalf("includes: %q", plainInc)
	}
	for name, mod := range map[string]string{
		"no lines": "module example.com/app\n",
		"replace of the dotted header's module": "module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nrequire example.com/other v1.0.0\n" +
			"replace github.com/huuhait/mathx => ./mathx\nreplace example.com/other => ./nowhere\n",
		"settings": "module example.com/app\ngocpp 0.1\ncxx c++20\ncflags -Inative\npkg-config sdl2\nrequire github.com/huuhait/mathx v0.3.0\nreplace github.com/huuhait/mathx => ./mathx\n",
		"recipe":   "module example.com/app\nrequire github.com/huuhait/mathx v0.3.0\nreplace github.com/huuhait/mathx => ./mathx\ncxx example.com/other {\n\tsources *.cpp\n\tinclude .\n}\n",
	} {
		errs, inc := run(mod)
		if name == "no lines" {
			// Without require lines the import of mathx fails; the rest is the same.
			if !strings.Contains(errs, "no required module provides package github.com/huuhait/mathx") {
				t.Errorf("%s: errors:\n%s", name, errs)
			}
		} else if errs != plainErrs {
			t.Errorf("%s: errors:\n%s\nwant\n%s", name, errs, plainErrs)
		}
		if strings.Join(inc, "\n") != strings.Join(plainInc, "\n") && name != "no lines" {
			t.Errorf("%s: includes:\n%s\nwant\n%s", name, strings.Join(inc, "\n"), strings.Join(plainInc, "\n"))
		}
	}
}

// TestModuleRoot checks how the module of a folder is found: by walking up to
// the nearest gocpp.mod, from any subfolder, also when the module file has
// errors; a folder outside every module has none, and the program still loads.
func TestModuleRoot(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":            "module example.com/app\nrequire x.y/z v1.0.0\n",
		"a/b/c/c.gp":           "package c\n",
		"inner/gocpp.mod":      "module example.com/inner\n",
		"inner/deep/d/d.gp":    "package d\n",
		"bad/gocpp.mod":        "module\n",
		"bad/e/e.gp":           "package e\n",
		"standalone/x/main.gp": "package main\n\nfunc main() {}\n",
	})
	for _, tc := range []struct{ dir, mod, path string }{
		{"", root, "example.com/app"},
		{"a", root, "example.com/app"},
		{"a/b/c", root, "example.com/app"},
		{"a/b/c/../c", root, "example.com/app"},
		{"inner", filepath.Join(root, "inner"), "example.com/inner"},
		{"inner/deep/d", filepath.Join(root, "inner"), "example.com/inner"},
		{"bad/e", filepath.Join(root, "bad"), ""},
		{"not/there/yet", root, "example.com/app"},
	} {
		m := FindModule(filepath.Join(root, filepath.FromSlash(tc.dir)))
		if m == nil || m.Dir != tc.mod || m.Path != tc.path {
			t.Errorf("FindModule(%q) = %+v, want %s in %s", tc.dir, m, tc.path, tc.mod)
		}
	}
	if m := FindModule(filepath.Join(root, "a", "b", "c")); m.File() == nil || len(m.File().Require) != 1 {
		t.Errorf("File of the module: %+v", m.File())
	}

	// Outside every module: the temporary folder is outside the tree above.
	out := writeTree(t, map[string]string{"x/main.gp": "package main\n\nimport \"fmt\"\n\nfunc main() { fmt.Println() }\n", "x/y.gp": "package main\n"})
	if m := FindModule(filepath.Join(out, "x")); m != nil {
		t.Errorf("FindModule outside every module = %+v", m)
	}
	for _, patterns := range [][]string{{"."}, {"main.gp", "y.gp"}, {"main.gp"}} {
		prog := loadIn(t, Config{}, filepath.Join(out, "x"), patterns...)
		wantErrors(t, prog, out, "")
		if p := prog.Roots[0]; p.ImportPath != commandLine || p.Module != nil || p.Name != "main" {
			t.Errorf("%v: import path %s, module %v", patterns, p.ImportPath, p.Module)
		}
	}
	// A module that requires something, loaded from outside its own folder
	// by file name, imports as before.
	prog := loadIn(t, Config{}, root, "./a/b/c")
	if p := prog.Roots[0]; p.ImportPath != "example.com/app/a/b/c" || p.Module.Dir != root {
		t.Errorf("root package: %s %+v", p.ImportPath, p.Module)
	}
}

// TestOverlayModule checks that an unsaved gocpp.mod (Config.Overlay) is
// what resolves the imports, and so is the gocpp.mod of a replaced folder.
func TestOverlayModule(t *testing.T) {
	root := workspace(t, "module example.com/app\n")
	app := filepath.Join(root, "app")
	overlay := map[string][]byte{
		filepath.Join(app, "gocpp.mod"): []byte(appModReplace),
	}
	prog := loadIn(t, Config{Overlay: overlay}, app, ".")
	wantErrors(t, prog, root, "")

	// The replaced module's own file in the overlay: another module path.
	overlay[filepath.Join(root, "vecs", "gocpp.mod")] = []byte("module example.com/changed\n")
	prog = loadIn(t, Config{Overlay: overlay}, app, ".")
	if got := errorText(t, prog, root); !strings.Contains(got,
		"module example.com/vecs is replaced by ../vecs, but the gocpp.mod there declares its path as example.com/changed") {
		t.Errorf("errors:\n%s", got)
	}

	// A replacement folder that is only in the overlay, and a vendor file
	// that is only in the overlay.
	overlay = map[string][]byte{
		filepath.Join(app, "gocpp.mod"):                                []byte(appModVendor),
		filepath.Join(app, "vendor", "modules.txt"):                    []byte(vendored["app/vendor/modules.txt"]),
		filepath.Join(app, "vendor", "example.com", "vecs", "vecs.gp"): []byte(mathx["vecs/vecs.gp"]),
	}
	for k, v := range vendored {
		if strings.HasSuffix(k, ".gp") && !strings.Contains(k, "vecs") {
			overlay[filepath.Join(root, filepath.FromSlash(k))] = []byte(v)
		}
	}
	prog = loadIn(t, Config{Overlay: overlay}, app, ".")
	wantErrors(t, prog, root, "")
	if m := pkgOf(t, prog, "example.com/vecs").Module; !m.Vendored {
		t.Errorf("not vendored: %+v", m)
	}
}

// TestRepositoryModFiles checks that the gocpp.mod files of the examples and
// of the test data parse without errors.
func TestRepositoryModFiles(t *testing.T) {
	n := 0
	for _, top := range []string{"../../examples", "../../tests", "."} {
		filepath.WalkDir(top, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() != modFile {
				return nil
			}
			n++
			data, err := os.ReadFile(path)
			if err != nil {
				t.Error(err)
				return nil
			}
			if _, errs := gocppmod.Parse(path, data, nil); len(errs) > 0 {
				t.Errorf("%v", errs)
			}
			return nil
		})
	}
	if n == 0 {
		t.Error("found no gocpp.mod file")
	}
}

// TestLongestRequire checks that of two required modules with the import path
// in both, the one with the longer module path has the package, in the order
// of the lines either way.
func TestLongestRequire(t *testing.T) {
	for _, order := range []string{
		"require example.com/a v1.0.0\nrequire example.com/a/b v1.0.0\n",
		"require example.com/a/b v1.0.0\nrequire example.com/a v1.0.0\n",
	} {
		root := writeTree(t, map[string]string{
			"app/gocpp.mod": "module example.com/app\n" + order + "replace example.com/a => ../a\nreplace example.com/a/b => ../ab\n",
			"app/main.gp":   "package main\n\nimport (\n\t_ \"example.com/a/b/x\"\n\t_ \"example.com/a/y\"\n\t_ \"example.com/a/b\"\n)\n",
			"a/gocpp.mod":   "module example.com/a\n",
			"a/b/x/x.gp":    "package wrong\n", // in module a, but b is longer
			"a/y/y.gp":      "package y\n",
			"ab/gocpp.mod":  "module example.com/a/b\n",
			"ab/b.gp":       "package b\n",
			"ab/x/x.gp":     "package x\n",
		})
		prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "")
		for path, want := range map[string]string{
			"example.com/a/b/x": filepath.Join(root, "ab", "x"), "example.com/a/b": filepath.Join(root, "ab"), "example.com/a/y": filepath.Join(root, "a", "y"),
		} {
			if got := pkgOf(t, prog, path).Dir; got != want {
				t.Errorf("%q: %s is in %s, want %s", order, path, got, want)
			}
		}
	}
}

// TestOwnModuleWinsATie checks that when the main module requires its own
// path, its own folders are used: a longer required module path wins, an equal
// one does not.
func TestOwnModuleWinsATie(t *testing.T) {
	root := writeTree(t, map[string]string{
		"app/gocpp.mod":   "module example.com/m\nrequire example.com/m v1.0.0\nrequire example.com/m/sub v1.0.0\nreplace example.com/m => ../other\nreplace example.com/m/sub => ../sub\n",
		"app/main.gp":     "package main\n\nimport (\n\t_ \"example.com/m/x\"\n\t_ \"example.com/m/sub\"\n)\n",
		"app/x/x.gp":      "package x\n",
		"other/gocpp.mod": "module example.com/m\n",
		"other/x/x.gp":    "package wrong\n",
		"sub/gocpp.mod":   "module example.com/m/sub\n",
		"sub/sub.gp":      "package sub\n",
	})
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
	if got := pkgOf(t, prog, "example.com/m/x").Dir; got != filepath.Join(root, "app", "x") {
		t.Errorf("x is in %s", got)
	}
	if got := pkgOf(t, prog, "example.com/m/sub").Dir; got != filepath.Join(root, "sub") {
		t.Errorf("sub is in %s", got)
	}
}

// TestModuleGob checks that a Module survives the encoding of the language
// server's disk cache: the exported fields come back, the parsed file does not.
func TestModuleGob(t *testing.T) {
	root := workspace(t, appModReplace)
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	vroot := workspace(t, appModVendor, vendored)
	vprog := loadIn(t, Config{}, filepath.Join(vroot, "app"), ".")
	for _, m := range []*Module{
		pkgOf(t, prog, "example.com/app").Module,
		pkgOf(t, prog, "github.com/huuhait/mathx").Module,
		pkgOf(t, vprog, "github.com/huuhait/mathx").Module,
		nil,
	} {
		var buf bytes.Buffer
		type holder struct{ Module *Module }
		if err := gob.NewEncoder(&buf).Encode(&holder{m}); err != nil {
			t.Fatalf("encode %+v: %v", m, err)
		}
		var got holder
		if err := gob.NewDecoder(&buf).Decode(&got); err != nil {
			t.Fatalf("decode %+v: %v", m, err)
		}
		switch {
		case m == nil:
			if got.Module != nil {
				t.Errorf("nil module came back as %+v", got.Module)
			}
		case got.Module == nil || got.Module.Path != m.Path || got.Module.Dir != m.Dir || got.Module.Version != m.Version ||
			got.Module.Vendored != m.Vendored || got.Module.File() != nil || m.File() == nil && m.Vendored == false:
			t.Errorf("module %+v came back as %+v", m, got.Module)
		}
	}
}

// TestVendoredFolder checks that a folder under vendor/ that is loaded by its
// path (as the language server loads every package) is the package of the
// vendored module, not a package of the main module called app/vendor/...
func TestVendoredFolder(t *testing.T) {
	root := workspace(t, appModVendor, vendored)
	app := filepath.Join(root, "app")
	vend := filepath.Join(app, "vendor")
	prog := loadIn(t, Config{}, app, "./vendor/github.com/huuhait/mathx/geom")
	wantErrors(t, prog, root, "")
	p := prog.Roots[0]
	if p.ImportPath != "github.com/huuhait/mathx/geom" || p.Module == nil || !p.Module.Vendored || p.Module.Path != "github.com/huuhait/mathx" ||
		p.Dir != filepath.Join(vend, "github.com", "huuhait", "mathx", "geom") {
		t.Errorf("root: %s %+v", p.ImportPath, p.Module)
	}
	if got := pkgOf(t, prog, "example.com/vecs").Module; !got.Vendored {
		t.Errorf("vecs: %+v", got)
	}
	// The same package from the folder of the program that imports it: one
	// package, with the same import path.
	prog = loadIn(t, Config{}, app, ".", "./vendor/example.com/vecs")
	wantErrors(t, prog, root, "")
	if len(prog.Roots) != 2 || prog.Roots[1] != pkgOf(t, prog, "example.com/vecs") {
		t.Errorf("roots: %+v", prog.Roots)
	}

	// Without vendoring, or with a folder that no require names, it is a
	// package of the main module, as before.
	root = workspace(t, appModReplace, map[string]string{"app/vendor/github.com/huuhait/mathx/geom/geom.gp": "package geom\n"})
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), "./vendor/github.com/huuhait/mathx/geom")
	if got := prog.Roots[0].ImportPath; got != "example.com/app/vendor/github.com/huuhait/mathx/geom" {
		t.Errorf("without vendoring: %s", got)
	}
	root = workspace(t, appModVendor, vendored)
	if err := os.MkdirAll(filepath.Join(root, "app", "vendor", "other.org", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "app", "vendor", "other.org", "x", "x.gp"), []byte("package x\n"), 0o644)
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), "./vendor/other.org/x")
	if got := prog.Roots[0].ImportPath; got != "example.com/app/vendor/other.org/x" {
		t.Errorf("not required: %s", got)
	}
	// An inconsistent vendor folder: the folder is not taken for a module's.
	root = workspace(t, appModVendor+"replace example.com/vecs => ../vecs\n", vendored)
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), "./vendor/example.com/vecs")
	if got := prog.Roots[0].ImportPath; got != "example.com/app/vendor/example.com/vecs" {
		t.Errorf("inconsistent: %s", got)
	}
}

// TestModFileTooLarge checks the limit on the size of gocpp.mod and of
// vendor/modules.txt: an error, not a long parse.
func TestModFileTooLarge(t *testing.T) {
	big := strings.Repeat("// x\n", gocppmod.MaxFileSize/5+1)
	root := writeTree(t, map[string]string{"gocpp.mod": big, "main.gp": "package main\n"})
	prog := loadIn(t, Config{}, root, ".")
	got := errorText(t, prog, root)
	if !strings.HasPrefix(got, "./gocpp.mod:1: file is too large (") || strings.Contains(got, "missing module declaration") {
		t.Errorf("errors:\n%s", got)
	}

	root = workspace(t, appModVendor, vendored)
	if err := os.WriteFile(filepath.Join(root, "app", "vendor", "modules.txt"), []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	if got := errorText(t, prog, root); !strings.Contains(got, "app/vendor/modules.txt:1: file is too large (") {
		t.Errorf("errors:\n%s", got)
	}
}
