package load

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The tests of module headers (moduleheader.go, Phase 06 step 6.1b).

// headerOf returns the first include of the first file of the package with
// the import path path.
func headerOf(t *testing.T, prog *Program, path string) *Include {
	t.Helper()
	p := pkgOf(t, prog, path)
	for _, f := range p.Files {
		for _, inc := range f.Info.Includes {
			return inc
		}
	}
	t.Fatalf("package %s has no header import", path)
	return nil
}

func TestIsModuleHeaderPath(t *testing.T) {
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"github.com/ocornut/imgui/imgui.h", true},
		{"example.com/x.h", true},
		{"gopkg.in/yaml.v3/y.hpp", true},
		{"a.b/c/d/e.inl", true},
		{"native/x.h", false},
		{"x.h", false},
		{"my-lib.hpp", false},
		{"./x.h", false},
		{"../x.h", false},
		{"./a.b/x.h", false},
		{".hidden/x.h", false},
		{"/abs/x.h", false},
		{"a.b/", false},
		{"", false},
	} {
		if got := IsModuleHeaderPath(tc.path); got != tc.want {
			t.Errorf("IsModuleHeaderPath(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func mainImporting(paths ...string) string {
	s := "package main\n\nimport (\n"
	for _, p := range paths {
		s += "\t" + strconv.Quote(p) + "\n"
	}
	return s + ")\n\nfunc main() {}\n"
}

// TestModuleHeaderOwnModule checks the header of the project's own module,
// from the main package and from a package in a subfolder.
func TestModuleHeaderOwnModule(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":      "module example.com/app\n",
		"main.gp":        mainImporting("example.com/app/native/x.h", "example.com/app/sub/s.hpp", "example.com/app/sub/deep/d.inl", "example.com/app/sub"),
		"native/x.h":     "int x;\n",
		"sub/s.hpp":      "int s;\n",
		"sub/deep/d.inl": "int d;\n",
		"sub/sub.gp":     "package sub\n\nimport \"example.com/app/native/x.h\"\n\nvar V = x.x\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")

	p := pkgOf(t, prog, "example.com/app")
	var got []string
	for _, inc := range p.Files[0].Info.Includes {
		if inc.Kind != LocalHeader || inc.Module == nil || inc.Module.Path != "example.com/app" || inc.Module.Dir != root {
			t.Errorf("include %+v", inc)
		}
		got = append(got, inc.Path+" "+inc.Name+" "+inc.File+" "+inc.IncludeText())
	}
	want := []string{
		"example.com/app/native/x.h x " + filepath.ToSlash(root) + "/native/x.h \"" + filepath.ToSlash(root) + "/native/x.h\"",
		"example.com/app/sub/s.hpp s " + filepath.ToSlash(root) + "/sub/s.hpp \"" + filepath.ToSlash(root) + "/sub/s.hpp\"",
		"example.com/app/sub/deep/d.inl d " + filepath.ToSlash(root) + "/sub/deep/d.inl \"" + filepath.ToSlash(root) + "/sub/deep/d.inl\"",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("includes:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(p.IncludeDirs) != 1 || p.IncludeDirs[0] != root {
		t.Errorf("IncludeDirs = %q, want just the module folder", p.IncludeDirs)
	}
	// The package in the subfolder imports it by the module path too.
	inc := headerOf(t, prog, "example.com/app/sub")
	if inc.File != filepath.ToSlash(root)+"/native/x.h" || inc.Name != "x" {
		t.Errorf("header of sub: %+v", inc)
	}
	if hn := pkgOf(t, prog, "example.com/app/sub").Files[0].Info.HeaderNames["x"]; hn != inc {
		t.Errorf("the header has no name x in sub")
	}
}

// TestModuleHeaderRequired checks the headers of required modules: replaced
// by a local folder, vendored, and from the download hook, with the packages
// of the module next to them.
func TestModuleHeaderRequired(t *testing.T) {
	lib := map[string]string{
		"lib/gocpp.mod":   "module example.org/lib\n",
		"lib/lib.h":       "#include \"util/dep.h\"\nint lib;\n",
		"lib/util/dep.h":  "int dep;\n",
		"lib/util/dep.gp": "package util\n\nconst D = 1\n",
	}
	merge := func(extra map[string]string) map[string]string {
		m := map[string]string{
			"app/main.gp": mainImporting("example.org/lib/lib.h", "example.org/lib/util/dep.h"),
		}
		for k, v := range lib {
			m[k] = v
		}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}

	t.Run("replace", func(t *testing.T) {
		root := writeTree(t, merge(map[string]string{
			"app/gocpp.mod": "module example.com/app\nrequire example.org/lib v1.2.3\nreplace example.org/lib => ../lib\n",
		}))
		prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "")
		p := pkgOf(t, prog, "example.com/app")
		incs := p.Files[0].Info.Includes
		if len(incs) != 2 || incs[0].File != filepath.ToSlash(root)+"/lib/lib.h" || incs[1].File != filepath.ToSlash(root)+"/lib/util/dep.h" ||
			incs[0].Module.Path != "example.org/lib" || incs[0].Module.Version != "v1.2.3" || incs[0].Module != incs[1].Module {
			t.Errorf("includes %+v %+v", incs[0], incs[1])
		}
		// The module's folder once, not once for each header.
		if len(p.IncludeDirs) != 1 || p.IncludeDirs[0] != filepath.Join(root, "lib") {
			t.Errorf("IncludeDirs = %q", p.IncludeDirs)
		}
	})

	t.Run("vendor", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			"app/main.gp":                      mainImporting("example.org/lib/lib.h"),
			"app/gocpp.mod":                    "module example.com/app\nrequire example.org/lib v1.2.3\n",
			"app/vendor/modules.txt":           "# example.org/lib v1.2.3\n## explicit\nexample.org/lib\n",
			"app/vendor/example.org/lib/lib.h": "int lib;\n",
		})
		prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "")
		inc := headerOf(t, prog, "example.com/app")
		want := filepath.ToSlash(root) + "/app/vendor/example.org/lib/lib.h"
		if inc.File != want || !inc.Module.Vendored {
			t.Errorf("File = %s, module %+v; want %s, vendored", inc.File, inc.Module, want)
		}
	})

	t.Run("download hook", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			"app/main.gp":   mainImporting("example.org/lib/lib.h"),
			"app/gocpp.mod": "module example.com/app\nrequire example.org/lib v1.2.3\n",
			// A plain C++ repository: no gocpp.mod in it.
			"cache/lib@v1.2.3/lib.h": "int lib;\n",
		})
		var calls []string
		cfg := Config{ModuleDir: func(path, version string) (string, error) {
			calls = append(calls, path+"@"+version)
			return filepath.Join(root, "cache", "lib@v1.2.3"), nil
		}}
		prog := loadIn(t, cfg, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "")
		inc := headerOf(t, prog, "example.com/app")
		if inc.File != filepath.ToSlash(root)+"/cache/lib@v1.2.3/lib.h" || inc.Module.Path != "example.org/lib" || inc.Module.Version != "v1.2.3" {
			t.Errorf("include %+v, module %+v", inc, inc.Module)
		}
		if strings.Join(calls, " ") != "example.org/lib@v1.2.3" {
			t.Errorf("the hook was called as %q", calls)
		}

		// The hook fails: the message of the module, and the no-download hint.
		cfg.ModuleDir = func(path, version string) (string, error) { return "", os.ErrPermission }
		prog = loadIn(t, cfg, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "# example.com/app\napp/main.gp:4:2: header example.org/lib/lib.h: module example.org/lib@v1.2.3 is required, "+
			"but is not available: permission denied\n")
		// The default hook is modfetch.Dir: the tests have no module cache folder.
		prog = loadIn(t, Config{}, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "# example.com/app\napp/main.gp:4:2: header example.org/lib/lib.h: module example.org/lib@v1.2.3 is required, "+
			"but is not available: no module cache folder (set GOCPP_MODCACHE)\n")
		// A hook that returns a folder that is not there.
		cfg.ModuleDir = func(path, version string) (string, error) { return filepath.Join(root, "gone"), nil }
		prog = loadIn(t, cfg, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "# example.com/app\napp/main.gp:4:2: header example.org/lib/lib.h: module example.org/lib@v1.2.3 is required, "+
			"but is not available: the module folder $ROOT/gone is not an existing absolute folder\n")
	})

	t.Run("packages from the hook", func(t *testing.T) {
		root := writeTree(t, map[string]string{
			"app/main.gp":   "package main\n\nimport \"example.org/lib/util\"\n\nfunc main() { _ = util.D }\n",
			"app/gocpp.mod": "module example.com/app\nrequire example.org/lib v1.2.3\n",
			// A package of the module imports another package of its own module.
			"cache/util/u.gp": "package util\n\nimport \"example.org/lib/base\"\n\nconst D = base.B\n",
			"cache/base/b.gp": "package base\n\nconst B = 1\n",
			// A gocpp.mod above the cache folder must not count as the module's own.
			"gocpp.mod": "module example.com/outer\n",
		})
		cfg := Config{ModuleDir: func(path, version string) (string, error) { return filepath.Join(root, "cache"), nil }}
		prog := loadIn(t, cfg, filepath.Join(root, "app"), ".")
		wantErrors(t, prog, root, "")
		if m := pkgOf(t, prog, "example.org/lib/util").Module; m.Path != "example.org/lib" || m.Dir != filepath.Join(root, "cache") {
			t.Errorf("module %+v", m)
		}
	})
}

// TestModuleHeaderLongestPrefix checks that the longest module path that is
// a prefix names the module, among the own module and the required ones.
func TestModuleHeaderLongestPrefix(t *testing.T) {
	root := writeTree(t, map[string]string{
		"app/gocpp.mod": "module example.com/app\nrequire example.com/a v1.0.0\nrequire example.com/a/b v1.0.0\nrequire example.com/app/third v1.0.0\n" +
			"replace example.com/a => ../a\nreplace example.com/a/b => ../ab\nreplace example.com/app/third => ../third\n",
		"app/main.gp": mainImporting("example.com/a/x.h", "example.com/a/b/x.h", "example.com/a/c/x.h", "example.com/a/b/c/x.h", "example.com/a/bc/x.h",
			"example.com/app/third/t.h", "example.com/app/own.h"),
		"app/own.h":       "int own;\n",
		"app/third/t.h":   "int in_app_third_folder;\n", // the own module's folder is hidden by the longer module
		"a/gocpp.mod":     "module example.com/a\n",
		"a/x.h":           "int ax;\n",
		"a/b/x.h":         "int a_b_x_in_a;\n", // a folder of module a, hidden by the module a/b
		"a/c/x.h":         "int acx;\n",
		"a/bc/x.h":        "int abcx_in_a;\n", // a/bc is a folder of module a: a/b is not its path prefix
		"ab/gocpp.mod":    "module example.com/a/b\n",
		"ab/x.h":          "int abx;\n",
		"ab/c/x.h":        "int abcx;\n",
		"third/gocpp.mod": "module example.com/app/third\n",
		"third/t.h":       "int third;\n",
	})
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
	var got []string
	for _, inc := range pkgOf(t, prog, "example.com/app").Files[0].Info.Includes {
		got = append(got, inc.Path+" => "+strings.TrimPrefix(inc.File, filepath.ToSlash(root)+"/")+" ("+inc.Module.Path+")")
	}
	want := []string{
		"example.com/a/x.h => a/x.h (example.com/a)",
		"example.com/a/b/x.h => ab/x.h (example.com/a/b)",
		"example.com/a/c/x.h => a/c/x.h (example.com/a)",
		"example.com/a/b/c/x.h => ab/c/x.h (example.com/a/b)",
		"example.com/a/bc/x.h => a/bc/x.h (example.com/a)",
		"example.com/app/third/t.h => third/t.h (example.com/app/third)",
		"example.com/app/own.h => app/own.h (example.com/app)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("resolved to:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	p := pkgOf(t, prog, "example.com/app")
	if len(p.IncludeDirs) != 4 {
		t.Errorf("IncludeDirs = %q, want the 4 modules", p.IncludeDirs)
	}
}

// TestModuleHeaderLocalUnchanged checks that "./x.h", "native/x.h" and "x.h"
// stay headers of the package folder, also when a module has such a file.
func TestModuleHeaderLocalUnchanged(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":    "module example.com/app\n",
		"main.gp":      mainImporting("./x.h", "native/y.hpp", "z.h", "./v1.2/w.h"),
		"x.h":          "int x;\n",
		"native/y.hpp": "int y;\n",
		"z.h":          "int z;\n",
		"v1.2/w.h":     "int w;\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")
	p := pkgOf(t, prog, "example.com/app")
	for _, inc := range p.Files[0].Info.Includes {
		if inc.Module != nil || inc.File != "" || inc.Target() != inc.Path || inc.IncludeText() != `"`+inc.Path+`"` {
			t.Errorf("local header %q changed: %+v", inc.Path, inc)
		}
	}
	if len(p.IncludeDirs) != 0 {
		t.Errorf("IncludeDirs = %q, want none", p.IncludeDirs)
	}
}

func TestModuleHeaderOverlay(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod": "module example.com/app\n",
		"main.gp":   mainImporting("example.com/app/native/x.h"),
	})
	prog := loadIn(t, Config{Overlay: map[string][]byte{filepath.Join(root, "native", "x.h"): []byte("int x;\n")}}, root, ".")
	wantErrors(t, prog, root, "")
	if inc := headerOf(t, prog, "example.com/app"); inc.File != filepath.ToSlash(root)+"/native/x.h" {
		t.Errorf("File = %s", inc.File)
	}
}

// TestModuleHeaderErrors checks the messages. The position is the import.
func TestModuleHeaderErrors(t *testing.T) {
	const modApp = "module example.com/app\nrequire example.org/lib v1.0.0\nreplace example.org/lib => ../lib\n"
	files := map[string]string{
		"lib/gocpp.mod":        "module example.org/lib\n",
		"lib/lib.h":            "int lib;\n",
		"lib/folder.h/x":       "x\n",
		"secret.h":             "int secret;\n",
		"app/gocpp.mod":        modApp,
		"app/own.h":            "int own;\n",
		"app/dir.h/y":          "y\n",
		"unrequired/u.gp":      "package u\n",
		"unrequired/gocpp.mod": "module example.net/unrequired\n",
	}
	for _, tc := range []struct {
		name, imp, want string
		mod             string // replaces the gocpp.mod of app
	}{
		{name: "no module has it", imp: "example.net/unrequired/u.h",
			want: `no required module provides header example.net/unrequired/u.h; add the module that has it to gocpp.mod with a require line, ` +
				`check the path for a typo, or for a header in a folder of the package write "./example.net/unrequired/u.h"`},
		{name: "replaced, not required", imp: "example.net/unrequired/u.h",
			mod:  "module example.com/app\nreplace example.net/unrequired => ../unrequired\n",
			want: `no required module provides header example.net/unrequired/u.h; example.net/unrequired is replaced in gocpp.mod, but not required (add: require example.net/unrequired v0.0.0)`},
		// An element boundary: example.com/appx is not in example.com/app.
		{name: "own module path is only a text prefix", imp: "example.com/appx/x.h",
			want: `no required module provides header example.com/appx/x.h; add the module that has it to gocpp.mod with a require line, ` +
				`check the path for a typo, or for a header in a folder of the package write "./example.com/appx/x.h"`},
		{name: "own module path with the extension", imp: "example.com/app.h",
			want: `no required module provides header example.com/app.h; add the module that has it to gocpp.mod with a require line, ` +
				`check the path for a typo, or for a header in a folder of the package write "./example.com/app.h"`},
		{name: "required module path is only a text prefix", imp: "example.org/lib2/lib.h",
			want: `no required module provides header example.org/lib2/lib.h; add the module that has it to gocpp.mod with a require line, ` +
				`check the path for a typo, or for a header in a folder of the package write "./example.org/lib2/lib.h"`},
		{name: "typo in the module path", imp: "example.org/libb/lib.h",
			want: `no required module provides header example.org/libb/lib.h; add the module that has it to gocpp.mod with a require line, ` +
				`check the path for a typo, or for a header in a folder of the package write "./example.org/libb/lib.h"`},
		{name: "not in the required module", imp: "example.org/lib/nothere.h",
			want: `module example.org/lib@v1.0.0 found ($ROOT/lib), but does not contain header example.org/lib/nothere.h`},
		{name: "not in the own module", imp: "example.com/app/native/nothere.h",
			want: `header example.com/app/native/nothere.h is not in module example.com/app ($ROOT/app/native/nothere.h)`},
		{name: "a folder", imp: "example.org/lib/folder.h",
			want: `header example.org/lib/folder.h ($ROOT/lib/folder.h) is a folder, not a file`},
		{name: "a folder of the own module", imp: "example.com/app/dir.h",
			want: `header example.com/app/dir.h ($ROOT/app/dir.h) is a folder, not a file`},
		{name: "path traversal", imp: "example.org/lib/../../secret.h",
			want: `invalid header path "example.org/lib/../../secret.h": path element ".." is not allowed (a module header is inside its module)`},
		{name: "traversal of the own module", imp: "example.com/app/../secret.h",
			want: `invalid header path "example.com/app/../secret.h": path element ".." is not allowed (a module header is inside its module)`},
		{name: "traversal in the middle", imp: "example.org/lib/a/../lib.h",
			want: `invalid header path "example.org/lib/a/../lib.h": path element ".." is not allowed (a module header is inside its module)`},
		{name: "dot element", imp: "example.org/lib/./lib.h",
			want: `invalid header path "example.org/lib/./lib.h": path element "." is not allowed`},
		{name: "double slash", imp: "example.org/lib//lib.h",
			want: `invalid header path "example.org/lib//lib.h": empty path element (a header path has no // and is not absolute)`},
		{name: "absolute path after the module", imp: "example.org/lib//etc/passwd.h",
			want: `invalid header path "example.org/lib//etc/passwd.h": empty path element (a header path has no // and is not absolute)`},
		{name: "drive letter", imp: "example.org/lib/C:/x.h",
			want: `invalid header path "example.org/lib/C:/x.h": path element "C:" has a : or a \`},
		{name: "stream name", imp: "example.org/lib/lib.h:evil.h",
			want: `invalid header path "example.org/lib/lib.h:evil.h": path element "lib.h:evil.h" has a : or a \`},
		{name: "backslash", imp: `example.org\lib\lib.h`,
			want: `invalid header path "example.org\\lib\\lib.h": use / in header paths`},
		{name: "backslash after the module", imp: `example.org/lib/sub\..\..\secret.h`,
			want: `invalid header path "example.org/lib/sub\\..\\..\\secret.h": use / in header paths`},
		{name: "trailing dot in an element", imp: "example.org/lib./lib.h",
			want: `invalid header path "example.org/lib./lib.h": path element "lib." ends in a dot or a space`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := map[string]string{"app/main.gp": mainImporting(tc.imp)}
			for k, v := range files {
				fs[k] = v
			}
			if tc.mod != "" {
				fs["app/gocpp.mod"] = tc.mod
			}
			root := writeTree(t, fs)
			prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
			// errorText writes '/' for a Windows separator, in a path from the test too.
			want := slashed("# example.com/app\napp/main.gp:4:2: " + tc.want + "\n")
			if got := errorText(t, prog, root); got != want {
				t.Errorf("errors:\n%s\nwant:\n%s", got, want)
			}
			p := pkgOf(t, prog, "example.com/app")
			if !p.Incomplete {
				t.Errorf("the package has no errors")
			}
			// The failed header does not reach the compiler.
			for _, inc := range p.Files[0].Info.Includes {
				t.Errorf("include %q is left in the file's includes", inc.Path)
			}
			if len(p.IncludeDirs) != 0 {
				t.Errorf("IncludeDirs = %q", p.IncludeDirs)
			}
		})
	}
}

// TestModuleHeaderNoModule checks the error outside every module, and that a
// header in a folder with a dot in its name is still a local header with ./.
func TestModuleHeaderNoModule(t *testing.T) {
	root := writeTree(t, map[string]string{
		"x/main.gp":         mainImporting("example.com/app/x.h", "./third.party/t.h"),
		"x/third.party/t.h": "int t;\n",
	})
	prog := loadIn(t, Config{}, filepath.Join(root, "x"), ".")
	wantErrors(t, prog, root, "# command-line-arguments\nx/main.gp:4:2: no required module provides header example.com/app/x.h: "+
		"gocpp.mod file not found in $ROOT/x or any parent directory\n")
	inc := headerOf(t, prog, "command-line-arguments")
	if inc.Path != "./third.party/t.h" || inc.File != "" {
		t.Errorf("include %+v", inc)
	}
}

// TestModuleHeaderDedup checks a header that is imported twice, and the
// name of the header package: the file name, as for every header.
func TestModuleHeaderNames(t *testing.T) {
	root := writeTree(t, map[string]string{
		"gocpp.mod":  "module example.com/app\n",
		"main.gp":    "package main\n\nimport (\n\t\"example.com/app/my-lib.h\"\n\tg \"example.com/app/sub/gl.hpp\"\n\t\"example.com/app/sub/gl.hpp\"\n)\n\nfunc main() {}\n",
		"my-lib.h":   "int a;\n",
		"sub/gl.hpp": "int b;\n",
	})
	prog := loadIn(t, Config{}, root, ".")
	wantErrors(t, prog, root, "")
	info := pkgOf(t, prog, "example.com/app").Files[0].Info
	if info.HeaderNames["my_lib"] == nil || info.HeaderNames["g"] == nil || info.HeaderNames["gl"] == nil || len(info.Includes) != 3 {
		t.Errorf("names %v, includes %d", info.HeaderNames, len(info.Includes))
	}
}

// TestReplaceWithoutModFile checks that a replacement folder that is a plain C
// or C++ repository (no gocpp.mod, a git clone) is the module of the replace
// line: its headers import, and so do the packages in it, also those that
// import another package of the module. A gocpp.mod there must still declare
// the same module path (TestReplaceErrors).
func TestReplaceWithoutModFile(t *testing.T) {
	root := writeTree(t, map[string]string{
		"app/gocpp.mod":      "module example.com/app\nrequire example.org/lib v1.0.0\nreplace example.org/lib => ../deps/lib\n",
		"app/main.gp":        "package main\n\nimport (\n\t\"example.org/lib/lib.h\"\n\t\"example.org/lib/util\"\n)\n\nfunc main() { _ = util.D }\n",
		"deps/lib/lib.h":     "int lib;\n",
		"deps/lib/util/u.gp": "package util\n\nimport \"example.org/lib/base\"\n\nconst D = base.B\n",
		"deps/lib/base/b.gp": "package base\n\nconst B = 1\n",
	})
	prog := loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
	inc := headerOf(t, prog, "example.com/app")
	if inc.File != filepath.ToSlash(root)+"/deps/lib/lib.h" || inc.Module.Path != "example.org/lib" || inc.Module.Version != "v1.0.0" {
		t.Errorf("include %+v, module %+v", inc, inc.Module)
	}
	// A gocpp.mod above the folder only (it is another module's, not this one's).
	root = writeTree(t, map[string]string{
		"app/gocpp.mod":  "module example.com/app\nrequire example.org/lib v1.0.0\nreplace example.org/lib => ../deps/sub\n",
		"app/main.gp":    mainImporting("example.org/lib/lib.h"),
		"deps/gocpp.mod": "module example.org/other\n",
		"deps/sub/lib.h": "int lib;\n",
	})
	prog = loadIn(t, Config{}, filepath.Join(root, "app"), ".")
	wantErrors(t, prog, root, "")
}
