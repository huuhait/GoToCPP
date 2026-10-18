package check

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The cxx recipes in Check (Phase 06 step 6.1d): the include folders and
// defines of a library's recipe are flags of the namespace probe and of the C
// header check, as they are of the build, and the errors of a recipe are
// reported at its line of gocpp.mod. The language server runs Check, so it
// sees the same flags.

func recipeTree(recipe string) map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/m\nrequire example.org/lib v1.0.0\nreplace example.org/lib => ./lib\n" +
			"cxx example.org/lib {\n" + recipe + "\n}\n",
		"app/main.gp":         "package main\n\nimport \"example.org/lib/lib.hpp\"\n\nfunc main() { _ = libns.g() }\n",
		"lib/lib.hpp":         "#pragma once\n#ifndef LIB_PUBLIC\n#error the public define of the recipe did not reach the probe\n#endif\n#include <libx/base.h>\nnamespace libns { inline int g() { return base(); } }\n",
		"lib/inc/libx/base.h": "#pragma once\ninline int base() { return 3; }\n",
		"lib/src/lib.cpp":     "int unused;\n",
	}
}

func TestRecipeProbeFlags(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		rec := new(probeRecorder)
		cfg := probeConfig(cxx, 4)
		cfg.Probe = rec.probe
		prog, root := loadModuleTree(t, recipeTree("\tsrc src/lib.cpp\n\tinclude inc\n\tdefine LIB_PUBLIC=1\n\tprivate-define LIB_PRIVATE=1"))
		r := Check(prog, cfg)
		if got := errorText(t, prog, r, root, true); got != "" {
			t.Fatalf("errors:\n%s", got)
		}
		p := mainPkg(t, prog, r)
		lib := filepath.Join(root, "lib")
		want := []string{"-I", filepath.Join(lib, "inc"), "-DLIB_PUBLIC=1", "-iquote", lib}
		if !slices.Equal(p.Flags.Compile, want) || len(p.Flags.Libs) != 1 || p.Flags.Directive != 0 {
			t.Errorf("Flags = %+v, want the compile flags %q", p.Flags, want)
		}
		if len(rec.calls) != 1 || !slices.Equal(rec.calls[0].flags, p.ProbeFlags) || !slices.Equal(p.ProbeFlags, slices.Concat(want, build.LanguageFlags())) {
			t.Errorf("the probe got %+v, ProbeFlags %q", rec.calls, p.ProbeFlags)
		}
		// check builds no library: the source is not even looked at.
	})
}

func TestRecipeWithoutIncludeFails(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		// The same library without the `define` (or the `include`): the header
		// doesn't compile for the probe, so the probe sees the flags and nothing else.
		prog, root := loadModuleTree(t, recipeTree("\tsrc src/lib.cpp\n\tinclude inc"))
		got := errorText(t, prog, Check(prog, probeConfig(cxx, 4)), root, true)
		if !strings.Contains(got, "the public define of the recipe did not reach the probe") {
			t.Errorf("errors:\n%s", got)
		}
	})
}

func TestRecipeErrorsAtTheLine(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		prog, root := loadModuleTree(t, recipeTree("\tinclude inc\n\tdefine LIB_PUBLIC\n\tcflags -fplugin=evil.so\n\tfrobnicate"))
		got := errorText(t, prog, Check(prog, probeConfig(cxx, 4)), root, true)
		for _, want := range []string{
			"gocpp.mod:7:", "cxx example.org/lib: cflags: invalid flag in the cxx recipe of example.org/lib: -fplugin=evil.so",
			"gocpp.mod:8:", `cxx example.org/lib: unknown statement "frobnicate"`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("errors do not have %q:\n%s", want, got)
			}
		}
	})
}
