package check

import (
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/interop"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// The module headers in Check (Phase 06 step 6.1b): the includes of the
// probes are the absolute file of the header, the module's folder is an -I
// flag of the probe and of the C header check, and the extern "C" decision
// works for them like for any local header.

// moduleHeaderTree is the module example.com/m with its main package in app/,
// a header of the module (native/x.h) that includes another one by its name in
// the module (native/deep.h), a C header (native/c.h) that does so too, a
// required module in lib/ with the same, and a header in the package folder
// (app/native/x.h) that must not hide the module's.
func moduleHeaderTree(mainSrc string) map[string]string {
	return map[string]string{
		"gocpp.mod":        "module example.com/m\nrequire example.org/lib v1.0.0\nreplace example.org/lib => ./lib\n",
		"app/main.gp":      mainSrc,
		"app/native/x.h":   "#error the header of the package folder hides the header of the module\n",
		"native/x.h":       "#pragma once\n#include <cstring>\n#include \"native/deep.h\"\nnamespace nx { inline int f() { return deep(); } }\n",
		"native/deep.h":    "#pragma once\ninline int deep() { return 2; }\n",
		"native/c.h":       "#include <string.h>\n#include \"native/cdep.h\"\nint c_add(cint, cint);\n",
		"native/cdep.h":    "typedef int cint;\n",
		"lib/gocpp.mod":    "module example.org/lib\n",
		"lib/lib.hpp":      "#pragma once\n#include <cstring>\n#include \"inc/base.hpp\"\nnamespace libns { inline int g() { return base(); } }\n",
		"lib/inc/base.hpp": "#pragma once\ninline int base() { return 3; }\n",
		// A header in a sub-folder that includes a header at the root of its module by that name.
		"lib/backends/b.hpp": "#pragma once\n#include \"lib.hpp\"\nnamespace bk { inline int h() { return libns::g(); } }\n",
		// Files named like system headers, at the root of the own module and of the required one:
		// -iquote leaves <cstring> and <string.h> alone.
		"string.h":     "#error the string.h of the module hides the system's\n",
		"lib/string.h": "#error the string.h of the library hides the system's\n",
	}
}

func loadModuleTree(t *testing.T, files map[string]string) (*load.Program, string) {
	t.Helper()
	root := writeTree(t, files)
	prog := load.Load(load.Config{Dir: root, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}, "./app")
	return prog, root
}

func TestModuleHeaderProbe(t *testing.T) {
	t.Parallel()
	const src = `package main

import (
	"example.com/m/native/x.h"
	"example.org/lib/lib.hpp"
	"example.org/lib/backends/b.hpp"
)

func main() {
	_ = nx.f()
	_ = libns.g()
	_ = bk.h()
	_ = nxx.f()
}
`
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		rec := new(probeRecorder)
		cfg := probeConfig(cxx, 4)
		cfg.Probe = rec.probe
		prog, root := loadModuleTree(t, moduleHeaderTree(src))
		r := Check(prog, cfg)
		// Only the typo is an error: the probe found both headers and both
		// namespaces, with the module folders on the include path.
		want := "# example.com/m/app\napp/main.gp:13:6: undefined: nxx\n"
		if got := errorText(t, prog, r, root, true); got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
		p := mainPkg(t, prog, r)
		slash := filepath.ToSlash(root)
		wantIncludes := []string{`"` + slash + `/native/x.h"`, `"` + slash + `/lib/lib.hpp"`, `"` + slash + `/lib/backends/b.hpp"`}
		wantFlags := []string{"-iquote", root, "-iquote", filepath.Join(root, "lib")}
		if len(rec.calls) != 1 || !slices.Equal(rec.calls[0].includes, wantIncludes) {
			t.Fatalf("probe calls %+v; want one with the includes %q", rec.calls, wantIncludes)
		}
		if !slices.Equal(p.Flags.Compile, wantFlags) || !slices.Equal(p.ProbeFlags, slices.Concat(wantFlags, build.LanguageFlags())) ||
			!slices.Equal(rec.calls[0].flags, p.ProbeFlags) {
			t.Errorf("Flags.Compile %q, ProbeFlags %q, the probe got %q; want %q", p.Flags.Compile, p.ProbeFlags, rec.calls[0].flags, wantFlags)
		}
	})
}

// TestModuleHeaderIncludeError checks the error for a module header that
// doesn't compile, shown with the import path the user wrote.
func TestModuleHeaderIncludeError(t *testing.T) {
	t.Parallel()
	files := moduleHeaderTree("package main\n\nimport \"example.com/m/native/bad.h\"\n\nfunc main() {}\n")
	files["native/bad.h"] = "#include \"native/missing.h\"\n"
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		prog, root := loadModuleTree(t, maps.Clone(files))
		got := errorText(t, prog, Check(prog, probeConfig(cxx, 4)), root, true)
		if !strings.HasPrefix(got, "# example.com/m/app\napp/main.gp:3:8: header \"example.com/m/native/bad.h\" doesn't compile: ") ||
			!strings.Contains(got, "missing.h") {
			t.Errorf("errors:\n%s", got)
		}
	})
}

// TestModuleHeaderExternC checks that a module header that is a C header is
// wrapped in extern "C" by the same rule as a local one (the C header check
// sees the module's folder as -I), and the directives work with the import
// path.
func TestModuleHeaderExternC(t *testing.T) {
	t.Parallel()
	slash := func(root, rel string) string { return `"` + filepath.ToSlash(root) + "/" + rel + `"` }
	for _, tc := range []struct {
		name, directive string
		wrapped         bool
	}{
		{"default: a C header", "", true},
		{"no-extern-c", "//gocpp:no-extern-c example.com/m/native/c.h\n", false},
		{"extern-c", "//gocpp:extern-c example.com/m/native/c.h\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
				t.Parallel()
				rec := new(probeRecorder)
				cfg := probeConfig(cxx, 4)
				cfg.Probe = rec.probe
				prog, root := loadModuleTree(t, moduleHeaderTree(tc.directive+"package main\n\nimport \"example.com/m/native/c.h\"\n\nfunc main() {}\n"))
				r := Check(prog, cfg)
				if got := errorText(t, prog, r, root, true); got != "" {
					t.Fatalf("errors:\n%s", got)
				}
				inc := slash(root, "native/c.h")
				p := mainPkg(t, prog, r)
				if p.ExternC[inc] != tc.wrapped {
					t.Errorf("ExternC = %v, want %s: %v", p.ExternC, inc, tc.wrapped)
				}
				want := inc
				if tc.wrapped {
					want = interop.ExternC(inc)
				}
				if len(rec.calls) != 1 || !slices.Equal(rec.calls[0].includes, []string{want}) {
					t.Errorf("probe calls %+v, want the include %s", rec.calls, want)
				}
			})
		})
	}

	// A directive for a module header that no file imports is an error.
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		prog, root := loadModuleTree(t, moduleHeaderTree("//gocpp:extern-c example.com/m/native/other.h\npackage main\n\nimport \"example.com/m/native/c.h\"\n\nfunc main() {}\n"))
		want := "# example.com/m/app\napp/main.gp:1:1: //gocpp:extern-c names header example.com/m/native/other.h, which no file of the package imports\n"
		if got := errorText(t, prog, Check(prog, probeConfig(cxx, 4)), root, true); got != want {
			t.Errorf("errors:\n%s\nwant:\n%s", got, want)
		}
	})
}
