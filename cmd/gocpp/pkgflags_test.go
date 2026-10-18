package main

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/build/testlib"
)

// The tests of the flags of //gocpp: directives in gocpp build and run
// (Phase 04, step 4.15). Until step 4.3 writes the #include of a header
// import, the C++ that uses the library comes from testHookCPP.

// useTestlib sets testHookCPP for the rest of the test so that the
// program includes gptestlib.h and prints gptestlib_answer() before main
// runs: then the generated C++ compiles only with the -I and -D of
// gptestlib.pc, and links only with its -L and -l.
func useTestlib(t *testing.T) {
	t.Helper()
	testHookCPP = inMainUnit(func(cpp []byte) []byte {
		return []byte("#include <gptestlib.h>\n#include <cstdio>\n" + string(cpp) +
			"\nstatic int gp_test_answer = (std::printf(\"answer %d\\n\", gptestlib_answer()), std::fflush(stdout), 0);\n")
	})
	t.Cleanup(func() { testHookCPP = nil })
}

// pkgConfigSrc is a program that uses gptestlib through pkg-config.
const pkgConfigSrc = `//gocpp:pkg-config gptestlib
package main

func main() {
	println("main")
}
`

func TestBuildPkgConfig(t *testing.T) {
	testlib.NeedPkgConfig(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		prefix := filepath.Clean(t.TempDir())
		pcDir := testlib.Install(t, prefix, &c.cxx)
		t.Setenv("PKG_CONFIG_PATH", pcDir)
		t.Setenv("PKG_CONFIG", "")
		t.Setenv("GOCPP_GATE_PHASE", "04") // directives

		// The library needs every flag of its .pc file: without them, the
		// compile or the link fails (so a build that drops them fails).
		dir := inDir(t, map[string]string{"main.gp": pkgConfigSrc, "use.cpp": "#include <gptestlib.h>\nint main() { return gptestlib_answer() != 42; }\n"})
		inc, lib := "-I"+filepath.Join(prefix, "include"), "-L"+filepath.Join(prefix, "lib")
		obj := filepath.Join(dir, "use.o")
		for _, flags := range [][]string{
			{"-c", "use.cpp", "-o", obj},
			{inc, "-c", "use.cpp", "-o", obj},
			{"-DGPTESTLIB_ON", "-c", "use.cpp", "-o", obj},
			{inc, "-DGPTESTLIB_ON", "use.cpp", "-lgptestlib", "-o", filepath.Join(dir, "use")},
		} {
			if out, err := c.cxx.Command(context.Background(), flags...).CombinedOutput(); err == nil {
				t.Fatalf("gptestlib builds with only %q:\n%s", flags, out)
			}
		}
		os.Remove("use.cpp")

		useTestlib(t)
		if r := gocppRun("", true, "run", "."); r != (result{0, "answer 42\nmain\n", ""}) {
			t.Fatalf("gocpp run . = %+v, want exit 0 and the library's answer", r)
		}
		// -x shows the flags: the compile gets those of --cflags, and the
		// link those of --libs, after the object files (the object cache is off in
		// cmd/gocpp's tests: TestMain).
		r := gocppRun("", false, "build", "-x", "-o", filepath.Join(dir, "prog"), ".")
		if r.code != 0 {
			t.Fatalf("gocpp build -x = %+v", r)
		}
		if !strings.Contains(r.stderr, "cd "+dir+"\npkg-config --cflags -- gptestlib\ncd "+dir+"\npkg-config --libs -- gptestlib\n") {
			t.Errorf("-x doesn't show the pkg-config runs:\n%s", r.stderr)
		}
		compile := regexp.MustCompile(`(?m)^.* -c -x c\+\+ \S*/gpkg/\S*\.cpp -o .*$`).FindString(r.stderr)
		if !strings.Contains(compile, " "+inc+" ") || !strings.Contains(compile, " -DGPTESTLIB_ON ") {
			t.Errorf("the compile of the generated C++ lacks the flags of pkg-config --cflags:\n%s\nstderr:\n%s", compile, r.stderr)
		}
		link := regexp.MustCompile(`(?m)^.* \$WORK/[^ ]*\.o .* -o \$WORK/[^ ]*$`).FindString(r.stderr)
		if !regexp.MustCompile(`\.o ` + regexp.QuoteMeta(lib) + ` -lgptestlib -o `).MatchString(link) {
			t.Errorf("the link lacks the flags of pkg-config --libs after the objects:\n%s\nstderr:\n%s", link, r.stderr)
		}
	})
}

// TestBuildFlagErrors checks that a flag that the allow-list refuses stops
// gocpp build before any compiler runs, with the error at the directive,
// and that a library that isn't there is a link error of the program, not
// an internal error of gocpp.
func TestBuildFlagErrors(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_GATE_PHASE", "04")
	inDir(t, map[string]string{"main.gp": "//gocpp:cflags -fplugin=./evil.so\npackage main\n\nfunc main() {}\n"})
	r := gocppRun("", false, "build", "-x", ".")
	want := result{1, "", "# command-line-arguments\n./main.gp:1:1: invalid flag in //gocpp:cflags: -fplugin=./evil.so\n"}
	if r != want {
		t.Errorf("gocpp build -x with -fplugin = %+v, want %+v", r, want)
	}

	inDir(t, map[string]string{"main.gp": "//gocpp:link gocpp_no_such_lib\npackage main\n\nfunc main() {}\n"})
	r = gocppRun("", false, "build", ".")
	if r.code != 1 || !strings.HasPrefix(r.stderr, "# command-line-arguments\nthe program did not link: ") ||
		!strings.Contains(r.stderr, "gocpp_no_such_lib") ||
		!strings.HasSuffix(r.stderr, "(the link flags of the package's //gocpp: directives: -lgocpp_no_such_lib)\n") {
		t.Errorf("gocpp build with a library that isn't there = %+v, want exit 1 and the link error", r)
	}

	t.Setenv("GOCPP_LDFLAGS", "'-lx")
	r = gocppRun("", false, "build", ".")
	if want := (result{1, "", "gocpp build: GOCPP_LDFLAGS: unterminated ' string\n"}); r != want {
		t.Errorf("gocpp build with a bad GOCPP_LDFLAGS = %+v, want %+v", r, want)
	}
}

// TestBuildEnvFlags checks that GOCPP_CXXFLAGS and GOCPP_LDFLAGS reach
// the compiles (the runtime's too) and the link, without the allow-list.
func TestBuildEnvFlags(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		prefix := filepath.Clean(t.TempDir())
		testlib.Install(t, prefix, &c.cxx)
		inDir(t, map[string]string{"main.gp": "package main\n\nfunc main() {\n\tprintln(\"main\")\n}\n"})
		useTestlib(t)
		t.Setenv("GOCPP_CXXFLAGS", "-I"+filepath.Join(prefix, "include")+" -DGPTESTLIB_ON")
		t.Setenv("GOCPP_LDFLAGS", "-L"+filepath.Join(prefix, "lib")+" -lgptestlib")
		// The runtime is compiled again, with the flags (they are part of
		// its key) and gocpp's language flags after them, and the link
		// gets GOCPP_LDFLAGS last.
		r := gocppRun("", true, "run", "-x", ".")
		if r.code != 0 || !strings.HasSuffix(r.stdout, "answer 42\nmain\n") ||
			!regexp.MustCompile(`(?m) -DGPTESTLIB_ON `+regexp.QuoteMeta(strings.Join(build.LanguageFlags(), " "))+` -Wno-error -x c\+\+ -I \S+/include -c \S+/src/\S+\.cpp `).MatchString(r.stdout) ||
			!regexp.MustCompile(`(?m)\.o -L\S+ -lgptestlib -o \$WORK/\S+$`).MatchString(r.stdout) {
			t.Errorf("gocpp run with GOCPP_CXXFLAGS and GOCPP_LDFLAGS = %+v", r)
		}
	})
}

// TestBuildRelativeFlags checks that the relative paths of //gocpp:cflags
// work in the build as in the check, though the compiles run in the work
// folder: -isystem inc and -include inc/pre.h are made absolute.
func TestBuildRelativeFlags(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_GATE_PHASE", "04")
	inDir(t, map[string]string{
		"main.gp":      "//gocpp:cflags -isystem inc -include inc/pre.h\npackage main\n\nfunc main() {\n\tprintln(\"main\")\n}\n",
		"inc/gp_rel.h": "#define GP_REL_ANSWER 7\n",
		"inc/pre.h":    "#define GP_PRE 1\n",
	})
	testHookCPP = inMainUnit(func(cpp []byte) []byte {
		return []byte("#include <gp_rel.h>\n#include <cstdio>\n" + string(cpp) +
			"\nstatic int gp_test_answer = (std::printf(\"%d %d\\n\", GP_REL_ANSWER, GP_PRE), std::fflush(stdout), 0);\n")
	})
	t.Cleanup(func() { testHookCPP = nil })
	if r := gocppRun("", true, "run", "."); r != (result{0, "7 1\nmain\n", ""}) {
		t.Errorf("gocpp run . = %+v, want exit 0 and \"7 1\\nmain\\n\"", r)
	}
}

// TestBuildCompileFlagsError checks that generated C++ that doesn't compile
// with the package's flags is reported with the flags, as an error of the
// build (exit 1), not as a bug in gocpp.
func TestBuildCompileFlagsError(t *testing.T) {
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_GATE_PHASE", "04")
	inDir(t, map[string]string{"main.gp": "//gocpp:cflags -DGP_BREAK\npackage main\n\nfunc main() {}\n"})
	testHookCPP = inMainUnit(func(cpp []byte) []byte {
		return append([]byte("#ifdef GP_BREAK\n#error broken by the flag\n#endif\n"), cpp...)
	})
	t.Cleanup(func() { testHookCPP = nil })
	r := gocppRun("", false, "build", ".")
	if m := regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr); m != nil {
		defer os.RemoveAll(m[1])
	}
	if r.code != 1 || !strings.HasPrefix(r.stderr, "# command-line-arguments\ngenerated C++ did not compile with the flags below\n") ||
		!strings.Contains(r.stderr, "broken by the flag") ||
		!strings.Contains(r.stderr, "\n(the compile flags of the package's //gocpp: directives: -DGP_BREAK)\nThe work folder is kept: ") ||
		strings.Contains(r.stderr, "bug in gocpp") {
		t.Errorf("gocpp build = %+v, want exit 1 and the error with the flags", r)
	}
}
