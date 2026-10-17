package main

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
)

// The tests of module headers in gocpp check, emit, build and run (Phase 06
// step 6.1b): "example.com/p/native/x.h" is native/x.h of the module, and
// "example.org/lib/lib.hpp" a header of a required module.

// moduleHeaderFiles is a module example.com/p whose main package imports a
// header of the module, a header of a required module (replaced by lib/) and
// a header of the package folder. Each library header includes another one
// by its name in its module.
var moduleHeaderFiles = map[string]string{
	"gocpp.mod": "module example.com/p\nrequire example.org/lib v1.0.0\nreplace example.org/lib => ./lib\n",
	"main.gp": `package main

import (
	"./local.h"
	"example.com/p/native/x.hpp"
	"example.org/lib/lib.hpp"
	"example.org/lib/backends/b.hpp"
)

func main() {
	println(int(x.x_twice(21)), int(lib.lib_base()), int(local.local_one()), int(b.lib_b()))
}
`,
	"local.h":       "inline int local_one() { return 1; }\n",
	"native/x.hpp":  "#pragma once\n#include <cstring>\n#include \"native/y.hpp\"\ninline int x_twice(int v) { return y_double(v); }\n",
	"native/y.hpp":  "#pragma once\ninline int y_double(int v) { return 2 * v; }\n",
	"native/x.h":    "#error the module's native/x.hpp is not this file\n",
	"lib/gocpp.mod": "module example.org/lib\n",
	"lib/lib.hpp":   "#pragma once\n#include <cstring>\n#include \"inc/base.hpp\"\n",
	// A header in a sub-folder that includes a header at the root of its module by that name: it
	// compiles only with the module's folder on the include path.
	"lib/backends/b.hpp": "#pragma once\n#include \"lib.hpp\"\ninline int lib_b() { return lib_base() + 1; }\n",
	// Files named like system headers at the root of both modules: -iquote doesn't let them hide <cstring>.
	"string.h":         "#error the string.h of the module hides the system's\n",
	"lib/string.h":     "#error the string.h of the library hides the system's\n",
	"lib/inc/base.hpp": "#pragma once\ninline int lib_base() { return 40; }\n",
}

func TestModuleHeaderRunEmit(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04") // header imports
		dir := inDir(t, moduleHeaderFiles)
		if r := gocppRun("", false, "check", "."); r != (result{0, "", ""}) {
			t.Fatalf("gocpp check . = %+v", r)
		}
		if r := gocppRun("", false, "run", "."); r != (result{0, "", "42 40 1 41\n"}) {
			t.Fatalf("gocpp run . = %+v", r)
		}
		// The #include has the absolute file of each module header, at the line
		// of its import, and the local one stays as written.
		r := gocppRun("", false, "emit", ".")
		if r.code != 0 {
			t.Fatalf("gocpp emit . = %+v", r)
		}
		slash := filepath.ToSlash(dir)
		lines := mappedLines(r.stdout)
		for _, inc := range []struct {
			line int
			text string
		}{
			{4, `"./local.h"`},
			{5, `"` + slash + `/native/x.hpp"`},
			{6, `"` + slash + `/lib/lib.hpp"`},
			{7, `"` + slash + `/lib/backends/b.hpp"`},
		} {
			found := false
			for _, l := range lines {
				found = found || l.text == "#include "+inc.text && l.line == inc.line
			}
			if !found {
				t.Errorf("#include %s is not at main.gp:%d in:\n%s", inc.text, inc.line, r.stdout)
			}
		}
		// -x shows the include folders of the modules: the compile has them as -I.
		r = gocppRun("", false, "build", "-x", "-o", filepath.Join(dir, "prog"), ".")
		if r.code != 0 {
			t.Fatalf("gocpp build -x . = %+v", r)
		}
		for _, flag := range []string{"-iquote " + dir, "-iquote " + filepath.Join(dir, "lib")} {
			if !strings.Contains(r.stderr, flag) {
				t.Errorf("-x doesn't show %s:\n%s", flag, r.stderr)
			}
		}
	})
}

func TestModuleHeaderCheckErrors(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_GATE_PHASE", "04")
		dir := inDir(t, moduleHeaderFiles)
		src := func(imports ...string) {
			t.Helper()
			var b strings.Builder
			b.WriteString("package main\n\nimport (\n")
			for _, i := range imports {
				b.WriteString("\t\"" + i + "\"\n")
			}
			b.WriteString(")\n\nfunc main() {}\n")
			writeFiles(t, dir, map[string]string{"main.gp": b.String()})
		}
		for _, tc := range []struct{ imp, want string }{
			{"example.net/other/z.h", "main.gp:4:2: no required module provides header example.net/other/z.h; add the module that has it to gocpp.mod with a require line, " +
				"check the path for a typo, or for a header in a folder of the package write \"./example.net/other/z.h\"\n"},
			{"example.com/p/native/none.hpp", "main.gp:4:2: header example.com/p/native/none.hpp is not in module example.com/p (" +
				filepath.Join(dir, "native", "none.hpp") + ")\n"},
			{"example.org/lib/none.hpp", "main.gp:4:2: module example.org/lib@v1.0.0 found (" + filepath.Join(dir, "lib") +
				"), but does not contain header example.org/lib/none.hpp\n"},
			{"example.org/lib/../../etc/passwd.h", "main.gp:4:2: invalid header path \"example.org/lib/../../etc/passwd.h\": " +
				"path element \"..\" is not allowed (a module header is inside its module)\n"},
		} {
			src(tc.imp)
			r := gocppRun("", false, "check", ".")
			if want := tc.want; r.code != 1 || r.stdout != "" || r.stderr != want {
				t.Errorf("import %q: gocpp check . = %+v, want exit 1 and\n%s", tc.imp, r, want)
			}
		}
	})
}

// TestDirectiveCompileFlags checks that the include folders of the modules
// are not listed as the package's //gocpp: flags in the message of an
// internal compile failure.
func TestDirectiveCompileFlags(t *testing.T) {
	pkg := &check.Package{Load: &load.Package{IncludeDirs: []string{"/m", "/n"}}}
	pkg.Flags.Compile = []string{"-DX", "-iquote", "/m", "-iquote", "/n"}
	if got := directiveCompileFlags(pkg); !slices.Equal(got, []string{"-DX"}) {
		t.Errorf("directiveCompileFlags = %q", got)
	}
	pkg.Load.IncludeDirs = nil
	pkg.Flags.Compile = []string{"-DX"}
	if got := directiveCompileFlags(pkg); !slices.Equal(got, []string{"-DX"}) {
		t.Errorf("without modules: %q", got)
	}
}
