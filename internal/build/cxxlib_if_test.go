package build_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/diag"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// Tests of the if blocks of a recipe in the build (ADR-129): the statements of
// the branches that hold for the target are the recipe, and only they.

// targetFlags reads the flags of the project's package for the target goos/goarch
// and the extra tags.
func targetFlags(t *testing.T, lp *load.Package, prog *load.Program, goos, goarch string,
	tags ...string) build.PackageFlags {
	t.Helper()
	cm := lp.CxxModules[0]
	cm.GOOS, cm.GOARCH, cm.Tags = goos, goarch, tags
	flags, errs := readFlags(t, lp, prog, nil)
	if len(errs) > 0 {
		t.Fatalf("%s/%s: %s", goos, goarch, errText(errs))
	}
	return flags
}

func TestRecipeIfTargets(t *testing.T) {
	lp, prog, dir := loadTree(t, recipeProject(
		"\tsrc lib.c",
		"\tinclude inc",
		"\tif windows {",
		"\t\tdefine ON_WINDOWS",
		"\t\tlink gdi32 opengl32",
		"\t} else if linux {",
		"\t\tdefine ON_LINUX",
		"\t\tlink GL",
		"\t\tif arm64 {",
		"\t\t\tdefine LINUX_ARM",
		"\t\t} else {",
		"\t\t\tdefine LINUX_OTHER",
		"\t\t}",
		"\t} else {",
		"\t\tdefine ON_OTHER",
		"\t}",
		"\tif vulkan || darwin {",
		"\t\tdefine NEEDS_VULKAN",
		"\t}",
		"\tdefine ALWAYS",
	))
	inc := filepath.Join(dir, "lib", "inc")
	for _, tc := range []struct {
		goos, goarch string
		tags         []string
		defines      []string // the -D flags of the public flags, in order
		link         []string
	}{
		{"linux", "amd64", nil, []string{"-DON_LINUX", "-DLINUX_OTHER", "-DALWAYS"}, []string{"-lGL"}},
		{"linux", "arm64", nil, []string{"-DON_LINUX", "-DLINUX_ARM", "-DALWAYS"}, []string{"-lGL"}},
		{"windows", "amd64", nil, []string{"-DON_WINDOWS", "-DALWAYS"}, []string{"-lgdi32", "-lopengl32"}},
		{"darwin", "arm64", nil, []string{"-DON_OTHER", "-DNEEDS_VULKAN", "-DALWAYS"}, nil},
		{"freebsd", "amd64", nil, []string{"-DON_OTHER", "-DALWAYS"}, nil},
		{"freebsd", "amd64", []string{"vulkan"}, []string{"-DON_OTHER", "-DNEEDS_VULKAN", "-DALWAYS"}, nil},
	} {
		flags := targetFlags(t, lp, prog, tc.goos, tc.goarch, tc.tags...)
		var defines []string
		for _, f := range flags.Libs[0].Public {
			if strings.HasPrefix(f, "-D") {
				defines = append(defines, f)
			}
		}
		if !slices.Equal(defines, tc.defines) {
			t.Errorf("%s/%s %v: defines %q, want %q", tc.goos, tc.goarch, tc.tags, defines, tc.defines)
		}
		if !slices.Equal(flags.Link, tc.link) {
			t.Errorf("%s/%s %v: link %q, want %q", tc.goos, tc.goarch, tc.tags, flags.Link, tc.link)
		}
		if !slices.Contains(flags.Libs[0].Public, inc) {
			t.Errorf("%s/%s: the include folder is a statement outside the blocks: %q",
				tc.goos, tc.goarch, flags.Libs[0].Public)
		}
	}
}

// TestRecipeIfSources checks that the sources of a branch that is not taken are
// not looked at: a missing file there is no error for another target.
func TestRecipeIfSources(t *testing.T) {
	lp, prog, _ := loadTree(t, recipeProject(
		"\tif windows {",
		"\t\tsrc missing_on_this_pc.c",
		"\t} else {",
		"\t\tsrc lib.c",
		"\t}",
	))
	flags := targetFlags(t, lp, prog, "linux", "amd64")
	srcs, errs := flags.Libs[0].Sources()
	if len(errs) > 0 || !slices.Equal(sourceRels(srcs), []string{"lib.c"}) {
		t.Errorf("linux: sources %v, errors %v", sourceRels(srcs), errs)
	}
	flags = targetFlags(t, lp, prog, "windows", "amd64")
	_, errs = flags.Libs[0].Sources()
	if len(errs) != 1 || !strings.Contains(errs[0].Msg, "no such file in the module") {
		t.Errorf("windows: errors %v", errs)
	}
}

// TestRecipeIfErrorsAtTheirPlace checks the position of an error in an if block,
// as the build reports it: the line and the column in gocpp.mod.
func TestRecipeIfErrorsAtTheirPlace(t *testing.T) {
	const module = "cxx example.com/lib: "
	for _, tc := range []struct {
		recipe []string
		want   string
	}{
		{
			[]string{"\tsrc lib.c", "\twindows: link gdi32"},
			"gocpp.mod:8:2: " + module + "a condition is written as a block, if windows { ... }, since ADR-129",
		},
		{
			[]string{"\tif windows {", "\t\tfrobnicate x", "\t}"},
			"gocpp.mod:8:3: " + module + `unknown statement "frobnicate"`,
		},
		{
			[]string{"\tif windows && {", "\t}"},
			"gocpp.mod:7:15: " + module + `the condition "windows &&" is not valid`,
		},
		{
			[]string{"\tsrc lib.c", "\tif linux {", "\t} src a.c"},
			"gocpp.mod:9:4: " + module + "after a } only else may follow on the line",
		},
		// A flag that the allow-list refuses is an error of the statement's line, in a block too.
		{
			[]string{"\tif linux {", "\t\tcflags -fplugin=evil.so", "\t}"},
			"gocpp.mod:8:0: " + module + "cflags: invalid flag",
		},
	} {
		lp, prog, _ := loadTree(t, recipeProject(tc.recipe...))
		_, errs := readFlags(t, lp, prog, nil)
		if len(errs) == 0 || !strings.HasPrefix(errText(errs), tc.want) {
			t.Errorf("%q: errors\n%s\nwant a start of %q", tc.recipe, errText(errs), tc.want)
		}
	}
}

// ifKeyRecipe is a recipe with blocks for three systems; withDarwin adds a
// block that no Linux or Windows target takes.
func ifKeyRecipe(withDarwin bool) []string {
	lines := []string{
		"\tsrc lib.c",
		"\tif windows {",
		"\t\tprivate-define ON_WINDOWS",
		"\t} else {",
		"\t\tprivate-define ON_UNIX",
		"\t}",
	}
	if withDarwin {
		lines = append(lines, "\tif darwin {", "\t\tprivate-define ON_DARWIN", "\t}")
	}
	return lines
}

// TestRecipeIfObjectKey checks that the key of the object cache follows the
// recipe as evaluated for the target: the same recipe for two targets gives two
// keys, and a block that is not taken does not change the key of the target.
func TestRecipeIfObjectKey(t *testing.T) {
	build.ForEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cache := t.TempDir()
		with, withProg, dir := loadTree(t, recipeProject(ifKeyRecipe(true)...))
		without, withoutProg, _ := loadTree(t, recipeProject(ifKeyRecipe(false)...))
		src := filepath.Join(dir, "lib", "lib.c")
		if err := os.WriteFile(src, []byte("int lib_f(void) { return 1; }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		lib := func(lp *load.Package, prog *load.Program, goos string) *build.CxxLib {
			return targetFlags(t, lp, prog, goos, "amd64").Libs[0]
		}
		compile := func(l *build.CxxLib) bool {
			return build.CompileLibSource(t, cxx, cache, src, l)
		}
		// The first target compiles; the same again is a hit.
		if !compile(lib(with, withProg, "linux")) {
			t.Error("linux: the first compile was a hit")
		}
		if compile(lib(with, withProg, "linux")) {
			t.Error("linux again: not a hit")
		}
		// The same recipe for another target has another key.
		if !compile(lib(with, withProg, "windows")) {
			t.Error("windows: the same recipe for another target was a hit")
		}
		if !compile(lib(with, withProg, "darwin")) {
			t.Error("darwin: a block that only it takes was a hit")
		}
		// The recipe without the darwin block: the blocks it does not take change nothing
		// for linux and windows (hits), and its darwin target is the unix one, which
		// is the key of linux.
		if compile(lib(without, withoutProg, "linux")) {
			t.Error("linux without the darwin block: a block that is not taken changed the key")
		}
		if compile(lib(without, withoutProg, "windows")) {
			t.Error("windows without the darwin block: a block that is not taken changed the key")
		}
		if compile(lib(without, withoutProg, "darwin")) {
			t.Error("darwin without its block is the unix key: a hit")
		}
	})
}

// TestRecipeIfTargetsInOrder checks that the evaluation does not depend on the
// order of the reads: the target is the one of the module at the time of the read.
func TestRecipeIfTargetsInOrder(t *testing.T) {
	lp, prog, _ := loadTree(t, recipeProject("\tsrc lib.c", "\tif windows {", "\t\tlink gdi32", "\t}"))
	for _, goos := range []string{"windows", "linux", "windows", "linux"} {
		flags := targetFlags(t, lp, prog, goos, "amd64")
		if got := slices.Contains(flags.Link, "-lgdi32"); got != (goos == "windows") {
			t.Errorf("%s: gdi32 linked = %v", goos, got)
		}
	}
}

// TestRecipeIfVendored checks a library that ships its recipe with blocks in its
// own gocpp.mod, read from the vendor folder.
func TestRecipeIfVendored(t *testing.T) {
	libMod := "module example.com/lib\n\ncxx example.com/lib {\n\tsrc lib.c\n\tinclude inc\n" +
		"\tif windows {\n\t\tlink ws2_32\n\t} else {\n\t\tlink m\n\t}\n}\n"
	files := map[string]string{
		"gocpp.mod":                        "module example.com/app\n\nrequire example.com/lib v1.2.3\n",
		"main.gp":                          recipeMain,
		"vendor/modules.txt":               "# example.com/lib v1.2.3\n## explicit\nexample.com/lib\n",
		"vendor/example.com/lib/lib.h":     "void lib_f(void);\n",
		"vendor/example.com/lib/lib.c":     "void lib_f(void) {}\n",
		"vendor/example.com/lib/inc/a.h":   "\n",
		"vendor/example.com/lib/gocpp.mod": libMod,
	}
	lp, prog, _ := loadTree(t, files)
	for goos, want := range map[string][]string{"linux": {"-lm"}, "windows": {"-lws2_32"}} {
		flags := targetFlags(t, lp, prog, goos, "amd64")
		if !slices.Equal(flags.Link, want) || !flags.Libs[0].FromLibrary {
			t.Errorf("%s: link %q from the library %v, want %q",
				goos, flags.Link, flags.Libs[0].FromLibrary, want)
		}
	}
}

// TestRecipeIfTargetOfTheLoad checks that the loader gives the recipe the target
// it loads for (GOOS, GOARCH and the -tags), not the target of the machine.
func TestRecipeIfTargetOfTheLoad(t *testing.T) {
	dir := filepath.Clean(t.TempDir())
	for name, src := range recipeProject(
		"\tsrc lib.c",
		"\tif windows && arm64 {",
		"\t\tdefine WIN_ARM",
		"\t}",
		"\tif vulkan {",
		"\t\tdefine VULKAN",
		"\t}",
	) {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		goos, goarch string
		tags         []string
		want         []string
	}{
		{"windows", "arm64", []string{"vulkan"}, []string{"-DWIN_ARM", "-DVULKAN"}},
		{"windows", "amd64", nil, nil},
		{"linux", "arm64", []string{"vulkan"}, []string{"-DVULKAN"}},
	} {
		cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: tc.goos, GOARCH: tc.goarch, Tags: tc.tags}
		prog := load.Load(cfg, ".")
		lp := prog.Roots[0]
		flags, errs := readFlags(t, lp, prog, nil)
		if len(errs) > 0 {
			t.Fatal(errText(errs))
		}
		var defines []string
		for _, f := range flags.Libs[0].Public {
			if strings.HasPrefix(f, "-D") {
				defines = append(defines, f)
			}
		}
		if !slices.Equal(defines, tc.want) {
			t.Errorf("%s/%s %v: defines %q, want %q", tc.goos, tc.goarch, tc.tags, defines, tc.want)
		}
	}
}

// loadTags is loadTree for the target goos/goarch with the extra tags; it returns the program too.
func loadTags(t *testing.T, files map[string]string, goos, goarch string,
	tags ...string) (*load.Package, *load.Program) {
	t.Helper()
	dir := filepath.Clean(t.TempDir())
	for name, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: goos, GOARCH: goarch, Tags: tags}
	prog := load.Load(cfg, ".")
	return prog.Roots[0], prog
}

// warnings returns the warnings of the load as "line:column: message".
func warnings(prog *load.Program) []string {
	var out []string
	for _, d := range prog.Errors.Diagnostics() {
		if d.Severity == diag.Warning && strings.Contains(d.Msg, "the tag ") {
			out = append(out, fmt.Sprintf("%d:%d: %s", d.Pos.Line, d.Pos.Column, d.Msg))
		}
	}
	return out
}

// TestRecipeUnknownTagWarning checks the warning for a tag of a condition that
// nobody defines: it is only a warning (the tag is false, as in a build line), at
// the tag, with a guess for a typo, and none for the tags that are defined.
func TestRecipeUnknownTagWarning(t *testing.T) {
	const eg = "example.com/lib: the tag "
	for _, tc := range []struct {
		name   string
		recipe []string
		tags   []string
		want   []string // the start of each warning, "line:column: cxx " + ...
	}{
		{"a typo", []string{"\tif widnows {", "\t\tlink gdi32", "\t}"},
			nil, []string{`8:5: cxx ` + eg + `"widnows" in a condition is not an operating system`}},
		{"a guess", []string{"\tif linux && amd46 {", "\t}"}, nil,
			[]string{`8:14: cxx ` + eg + `"amd46"`}},
		{"far from every name", []string{"\tif zzzzzz {", "\t}"}, nil,
			[]string{`8:5: cxx ` + eg + `"zzzzzz"`}},
		{"in an else if, once", []string{"\tif windows {", "\t} else if frob || frob {", "\t}"}, nil,
			[]string{`9:12: cxx ` + eg + `"frob"`}},
		{"every defined tag", []string{
			"\tif windows || linux || darwin || freebsd || android || ios || js || wasip1 {", "\t}",
			"\tif amd64 || arm64 || 386 || arm || riscv64 || wasm || loong64 || ppc64le || s390x {", "\t}",
			"\tif unix || gocpp || gc || cgo || go1.18 || go1.25 || go1.99 {", "\t}",
			"\tif !(linux && arm64) {", "\t}"}, nil, nil},
		{"a tag of -tags", []string{"\tif vulkan {", "\t}"}, []string{"vulkan"}, nil},
		{"a tag that -tags does not have", []string{"\tif vulkan {", "\t}"}, nil,
			[]string{`8:5: cxx ` + eg + `"vulkan"`}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := recipeProject(append([]string{"\tsrc lib.c"}, tc.recipe...)...)
			lp, prog := loadTags(t, files, "linux", "amd64", tc.tags...)
			got := warnings(prog)
			if len(got) != len(tc.want) {
				t.Fatalf("warnings %q, want %d", got, len(tc.want))
			}
			for i, w := range tc.want {
				if !strings.HasPrefix(got[i], w) {
					t.Errorf("warning %q, want a start of %q", got[i], w)
				}
			}
			// A warning is not an error: the flags are read, and the recipe applies.
			if _, errs := readFlags(t, lp, prog, nil); len(errs) > 0 {
				t.Errorf("errors: %s", errText(errs))
			}
		})
	}
	// The guess.
	_, prog := loadTags(t, recipeProject("\tsrc lib.c", "\tif widnows {", "\t}"), "linux", "amd64")
	if got := warnings(prog); len(got) != 1 || !strings.HasSuffix(got[0], "; did you mean windows?") {
		t.Errorf("warnings %q", got)
	}
	_, prog = loadTags(t, recipeProject("\tsrc lib.c", "\tif zzzzzz {", "\t}"), "linux", "amd64")
	if got := warnings(prog); len(got) != 1 || strings.Contains(got[0], "did you mean") {
		t.Errorf("warnings %q", got)
	}
}
