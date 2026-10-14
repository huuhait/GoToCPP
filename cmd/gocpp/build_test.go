package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests of gocpp build, run and emit compile real programs with g++
// and clang++, or only with the compiler in GOCPP_CXX, like the test
// harness (tests/README.md): a compiler that isn't installed is a skip,
// unless GOCPP_REQUIRE_CXX (a comma list, "g++,clang++" in make ci) names
// it. They run gocpp in the test process (run), with GOCPP_CXX set to the
// compiler, except the tests of signals, which run the test binary as
// gocpp (TestMain).
//
// The emitter of Phase 02 doesn't write if, for or os.Exit yet (steps 2.3
// and 2.4), so a test that needs a program with arguments, input, an
// exit status or a crash replaces the C++ main function of a small program
// with its own (withMain).

// TestMain runs the tests with a TMPDIR of their own, which holds a cache
// folder of their own, so that they extract and compile the runtime
// themselves and leave the user's cache alone. At the end, the TMPDIR must
// hold only the cache: a work folder or a cache that a gocpp (also one
// that a signal killed, in a test process) left behind fails the tests.
// With GOCPP_TEST_MAIN=1, the test binary is gocpp instead: the signal
// tests run it as a program; GOCPP_TEST_MAIN_CPP then names a file with a
// C++ main function for withMain. With GOCPP_CHECK_PROCESS=1, it runs
// only TestCheckProcess, in the parent's folders.
func TestMain(m *testing.M) {
	if os.Getenv("GOCPP_TEST_MAIN") == "1" {
		if file := os.Getenv("GOCPP_TEST_MAIN_CPP"); file != "" {
			src, err := os.ReadFile(file)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(3)
			}
			testHookCPP = replaceMain(string(src))
		}
		main()
	}
	if os.Getenv("GOCPP_CHECK_PROCESS") == "1" {
		os.Exit(m.Run())
	}
	tmp, err := os.MkdirTemp("", "gocpp-cmd-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cache := filepath.Join(tmp, "cache")
	os.Setenv("TMPDIR", tmp)
	os.Setenv("XDG_CACHE_HOME", cache)
	code := m.Run()
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if e.Name() != "cache" {
			fmt.Fprintf(os.Stderr, "the tests left %s in their TMPDIR\n", e.Name())
			code = 1
		}
	}
	os.RemoveAll(tmp)
	os.Exit(code)
}

// A testCXX is a C++ compiler of the tests.
type testCXX struct {
	name    string // "g++", "clang++": the value of GOCPP_CXX
	cxx     build.CXX
	missing string
}

// testCompilers returns the compilers to test with.
func testCompilers(t *testing.T) []testCXX {
	t.Helper()
	names := []string{"g++", "clang++"}
	env := os.Getenv("GOCPP_CXX")
	if env != "" {
		names = []string{env}
	}
	var list []testCXX
	for _, n := range names {
		c := testCXX{name: n}
		cxx, err := build.LookCXX(context.Background(), n)
		if err != nil {
			c.missing = err.Error()
		}
		c.cxx = cxx
		list = append(list, c)
	}
	for _, want := range strings.Split(os.Getenv("GOCPP_REQUIRE_CXX"), ",") {
		if want = strings.TrimSpace(want); want == "" {
			continue
		}
		i := slices.IndexFunc(list, func(c testCXX) bool { return c.name == want || filepath.Base(c.name) == want })
		switch {
		case i < 0:
			t.Fatalf("GOCPP_REQUIRE_CXX needs %s, but the tests use only %q", want, names)
		case list[i].missing != "":
			t.Fatalf("GOCPP_REQUIRE_CXX needs %s, but %s", want, list[i].missing)
		}
	}
	return list
}

// forEachCXX runs fn once per compiler, as subtests, with GOCPP_CXX set
// to the compiler (and CXX unset).
func forEachCXX(t *testing.T, fn func(t *testing.T, c testCXX)) {
	for _, c := range testCompilers(t) {
		t.Run(filepath.Base(c.name), func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Setenv("GOCPP_CXX", c.name)
			t.Setenv("CXX", "")
			fn(t, c)
		})
	}
}

// anyCXX returns one compiler that is installed, or skips the test.
func anyCXX(t *testing.T) testCXX {
	t.Helper()
	for _, c := range testCompilers(t) {
		if c.missing == "" {
			return c
		}
	}
	t.Skip("no C++ compiler")
	return testCXX{}
}

// helloSrc is a program that the Phase 02 emitter writes: functions with
// several results, fmt and println.
const helloSrc = `package main

import "fmt"

func add(a, b int) (int, bool) {
	return a + b, a+b > 10
}

func main() {
	s, big := add(7, 5)
	fmt.Println("hello, gocpp", s, big)
	println("to stderr", s)
	fmt.Println("after")
}
`

// helloOut is what helloSrc prints, stdout and stderr in one stream.
const helloOut = "hello, gocpp 12 true\nto stderr 12\nafter\n"

// inDir writes the files into a new folder and makes it the current
// folder for the rest of the test.
func inDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, files)
	t.Chdir(dir)
	return dir
}

// gocppRun runs gocpp with args in the current folder, with stdin as the
// standard input. With combined, stdout and stderr are one stream
// (result.stdout), in the order of the writes.
func gocppRun(stdin string, combined bool, args ...string) result {
	var out, errOut bytes.Buffer
	stderr := &errOut
	if combined {
		stderr = &out
	}
	code := run(args, strings.NewReader(stdin), &out, stderr)
	return result{code, out.String(), errOut.String()}
}

// withMain sets testHookCPP for the rest of the test so that the program
// gets the C++ main function src instead of the generated one, which
// calls gp::run_main (the rest of the generated C++ stays).
func withMain(t *testing.T, src string) {
	t.Helper()
	testHookCPP = replaceMain(src)
	t.Cleanup(func() { testHookCPP = nil })
}

// replaceMain returns a testHookCPP that replaces the C++ main function
// with src.
func replaceMain(src string) func([]byte) []byte {
	return func(cpp []byte) []byte {
		i := bytes.Index(cpp, []byte("int main(int argc, char** argv) {"))
		if i < 0 {
			panic("no C++ main function in the generated C++")
		}
		return append(cpp[:i:i], src...)
	}
}

// echoMain is a C++ main function that prints its arguments and its
// standard input, writes to stderr and exits with status 7.
const echoMain = `#include <cstdio>
#include <iostream>
#include <string>
int main(int argc, char** argv) {
    for (int i = 1; i < argc; i++) std::printf("arg %s\n", argv[i]);
    std::string line;
    while (std::getline(std::cin, line)) std::printf("in %s\n", line.c_str());
    std::fflush(stdout);
    std::fprintf(stderr, "to stderr\n");
    return 7;
}
`

func TestRunProgram(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		// stdout and stderr in one stream: the order of the writes stays.
		if r := gocppRun("", true, "run", "hello.gp"); r != (result{0, helloOut, ""}) {
			t.Errorf("gocpp run hello.gp = %+v, want exit 0 and output %q", r, helloOut)
		}
		r := gocppRun("", false, "run", "hello.gp")
		if want := (result{0, "hello, gocpp 12 true\nafter\n", "to stderr 12\n"}); r != want {
			t.Errorf("gocpp run hello.gp = %+v, want %+v", r, want)
		}
	})
}

func TestRunArgsStdinExitCode(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		withMain(t, echoMain)
		// Like go run: the arguments after the .gp files go to the program,
		// also ones that look like flags.
		r := gocppRun("line 1\nline 2\n", false, "run", "hello.gp", "a b", "-x", "hello.gp2")
		want := result{7, "arg a b\narg -x\narg hello.gp2\nin line 1\nin line 2\n", "to stderr\n"}
		if r != want {
			t.Errorf("gocpp run = %+v, want %+v", r, want)
		}
		// A folder: the first argument is the package.
		os.Mkdir("sub", 0o755)
		os.Rename("hello.gp", "sub/main.gp")
		r = gocppRun("", false, "run", "./sub", "z")
		if want := (result{7, "arg z\n", "to stderr\n"}); r != want {
			t.Errorf("gocpp run ./sub z = %+v, want %+v", r, want)
		}
	})
}

func TestRunSignalDeath(t *testing.T) {
	// In the test process, run returns 128+15 and records the signal for
	// main, which dies of it (signal_unix_test.go checks that).
	if runtime.GOOS == "windows" {
		t.Skip("no signals")
	}
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		withMain(t, "#include <csignal>\n#include <cstdio>\nint main() { std::puts(\"before\"); std::fflush(stdout); std::raise(SIGTERM); }\n")
		exitSignal.Store(nil)
		r := gocppRun("", false, "run", "hello.gp")
		if want := (result{128 + 15, "before\n", ""}); r != want {
			t.Errorf("gocpp run of a program that a SIGTERM kills = %+v, want %+v", r, want)
		}
		if sig := exitSignal.Load(); sig == nil || *sig != syscall.SIGTERM {
			t.Errorf("exitSignal = %v, want SIGTERM", sig)
		}
		exitSignal.Store(nil)
	})
}

func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{{"run"}, {"run", "-work"}} {
		r := gocppRun("", false, args...)
		if r != (result{1, "", "gocpp run: no .gp files listed\n"}) {
			t.Errorf("gocpp %s = %+v, want exit 1 and the message", strings.Join(args, " "), r)
		}
	}
	for _, cmd := range []string{"build", "run", "emit"} {
		r := gocppRun("", false, cmd, "-nosuchflag")
		if r.code != 2 || !strings.Contains(r.stderr, "flag provided but not defined: -nosuchflag") || !strings.Contains(r.stderr, "usage: gocpp "+cmd) {
			t.Errorf("gocpp %s -nosuchflag = %+v, want exit 2, the error and the usage", cmd, r)
		}
		r = gocppRun("", false, cmd, "-h")
		if r.code != 0 || !strings.HasPrefix(r.stderr, "usage: gocpp "+cmd) {
			t.Errorf("gocpp %s -h = %+v, want exit 0 and the usage", cmd, r)
		}
	}
	// -o is for build only.
	if r := gocppRun("", false, "run", "-o", "x", "hello.gp"); r.code != 2 {
		t.Errorf("gocpp run -o x = %+v, want exit 2", r)
	}
}

func TestBuild(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, map[string]string{"hello.gp": helloSrc, "app/main.gp": helloSrc, "mod/gocpp.mod": "module example.com/tool/v2\n", "mod/main.gp": helloSrc})
		runProg := func(t *testing.T, prog string) {
			t.Helper()
			out, err := exec.Command(prog).CombinedOutput()
			if err != nil || string(out) != helloOut {
				t.Errorf("%s: %v, output %q; want %q", prog, err, out, helloOut)
			}
		}
		for _, b := range []struct {
			args []string
			dir  string // where gocpp runs
			prog string // the program it must write
		}{
			{[]string{"hello.gp"}, ".", "hello"},
			{[]string{"-o", "out/prog", "hello.gp"}, ".", "out/prog"},
			{[]string{"-o", "out/", "hello.gp"}, ".", "out/hello"},
			{[]string{"-o", "out", "./app"}, ".", "out/app"}, // out is a folder now
			{nil, "app", "app/app"},                          // a folder outside a module: its name
			{[]string{"."}, "mod", "mod/tool"},               // the import path, without /v2
		} {
			os.Mkdir(filepath.Join(dir, "out"), 0o755)
			t.Chdir(filepath.Join(dir, b.dir))
			r := gocppRun("", false, append([]string{"build"}, b.args...)...)
			if r != (result{}) {
				t.Errorf("gocpp build %s (in %s) = %+v, want exit 0 and no output", strings.Join(b.args, " "), b.dir, r)
				continue
			}
			runProg(t, filepath.Join(dir, b.prog+exeSuffix))
			os.Remove(filepath.Join(dir, b.prog+exeSuffix))
		}
	})
}

func TestEmit(t *testing.T) {
	inDir(t, map[string]string{"hello.gp": helloSrc})
	r := gocppRun("", false, "emit", "hello.gp")
	if r.code != 0 || r.stderr != "" || !strings.HasPrefix(r.stdout, "// hello.gp.cpp: generated by gocpp, do not edit\n#include <gocpp/rt.hpp>\n") {
		t.Fatalf("gocpp emit hello.gp = %+v, want the C++", r)
	}
	// emit leaves Config.OutputPath empty: the generated lines get no
	// #line of their own.
	if strings.Contains(r.stdout, ".cpp\"") {
		t.Errorf("gocpp emit writes #line directives for the C++ file:\n%s", r.stdout)
	}
	if dir := gocppRun("", false, "emit", "."); dir != r {
		t.Errorf("gocpp emit . = %+v, want the same as gocpp emit hello.gp", dir)
	}
}

func TestOutputPathLines(t *testing.T) {
	// The driver sets cppgen's Config.OutputPath: the main wrapper's lines
	// point at the C++ file in the work folder, not at a .gp line.
	inDir(t, map[string]string{"hello.gp": helloSrc})
	anyCXX(t)
	r := gocppRun("", false, "build", "-work", "hello.gp")
	work := workDir(t, r.stderr)
	cpp, err := os.ReadFile(filepath.Join(work, "hello.gp.cpp"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(cpp, []byte(`"`+filepath.Join(work, "hello.gp.cpp")+`"`)) {
		t.Errorf("the C++ has no #line for its own file %s:\n%s", filepath.Join(work, "hello.gp.cpp"), cpp)
	}
}

// workDir returns the work folder of the WORK= line in stderr, and
// removes it at the end of the test.
func workDir(t *testing.T, stderr string) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^WORK=(.+)$`).FindStringSubmatch(stderr)
	if m == nil {
		t.Fatalf("no WORK= line in stderr:\n%s", stderr)
	}
	t.Cleanup(func() { os.RemoveAll(m[1]) })
	return m[1]
}

func TestWork(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		r := gocppRun("", false, "run", "-work", "hello.gp")
		if r.code != 0 || r.stdout != "hello, gocpp 12 true\nafter\n" {
			t.Fatalf("gocpp run -work = %+v", r)
		}
		work := workDir(t, r.stderr)
		if want := "WORK=" + work + "\nto stderr 12\n"; r.stderr != want {
			t.Errorf("stderr = %q, want %q", r.stderr, want)
		}
		for _, f := range []string{"hello.gp.cpp", "hello.gp.o", "hello" + exeSuffix} {
			if _, err := os.Stat(filepath.Join(work, f)); err != nil {
				t.Errorf("the work folder has no %s: %v", f, err)
			}
		}
	})
}

func TestWorkRemoved(t *testing.T) {
	// Without -work, nothing is left in TMPDIR: after a build, a run, and
	// a check error.
	anyCXX(t)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	inDir(t, map[string]string{"hello.gp": helloSrc, "bad/bad.gp": "package main\n\nfunc main() { x }\n"})
	for _, args := range [][]string{{"run", "hello.gp"}, {"build", "hello.gp"}, {"build", "./bad"}} {
		gocppRun("", false, args...)
		if left, _ := os.ReadDir(tmp); len(left) > 0 {
			t.Errorf("gocpp %s left %s in TMPDIR", strings.Join(args, " "), left[0].Name())
		}
	}
}

func TestTraceCommands(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		r := gocppRun("", false, "run", "-x", "hello.gp", "arg 1")
		if r.code != 0 {
			t.Fatalf("gocpp run -x = %+v", r)
		}
		work := workDir(t, r.stderr) // the folder is gone: removing it again is fine
		lines := strings.Split(strings.TrimSuffix(r.stderr, "\n"), "\n")
		// The runtime may have been compiled before; the program's commands
		// are the last ones.
		flags := "-std=c++20 -fwrapv -fno-strict-aliasing -ffp-contract=off -g -gno-column-info -O0"
		if c.cxx.Family == build.Clang {
			flags += " -Qunused-arguments"
		}
		want := []*regexp.Regexp{
			regexp.MustCompile(`^# gocpp emit > \$WORK/hello\.gp\.cpp$`),
			// The generated C++ gets the warning flags of its family.
			regexp.MustCompile(`^` + regexp.QuoteMeta(c.cxx.Path+" "+flags+" "+strings.Join(build.GeneratedFlags(c.cxx), " ")+" -I ") + `\S+/include -c \$WORK/hello\.gp\.cpp -o \$WORK/hello\.gp\.o$`),
			regexp.MustCompile(`^` + regexp.QuoteMeta(c.cxx.Path+" "+flags+" $WORK/hello.gp.o ") + `(\S+\.o ?)+ -o \$WORK/hello$`),
			regexp.MustCompile(`^\$WORK/hello 'arg 1'$`),
		}
		if lines[0] != "WORK="+work {
			t.Errorf("the first line is %q, want WORK=%s", lines[0], work)
		}
		var got []string
		for _, l := range lines {
			if !strings.Contains(l, "/rt/") || strings.Contains(l, "-o $WORK") {
				got = append(got, l)
			}
		}
		// got: WORK=, emit, compile, link, run, then the program's stderr.
		if len(got) < 6 {
			t.Fatalf("stderr has %d lines, want more:\n%s", len(got), r.stderr)
		}
		for i, rx := range want {
			if !rx.MatchString(got[1+i]) {
				t.Errorf("line %d of -x is\n\t%s\nwant it to match\n\t%s", i+2, got[1+i], rx)
			}
		}
		if got[5] != "to stderr 12" {
			t.Errorf("after the commands comes %q, want the program's stderr", got[5])
		}
	})
}

func TestASan(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		if r := gocppRun("", true, "run", "-asan", "hello.gp"); r != (result{0, helloOut, ""}) {
			t.Fatalf("gocpp run -asan hello.gp = %+v, want exit 0 and %q", r, helloOut)
		}
		r := gocppRun("", false, "run", "-asan", "-x", "hello.gp")
		workDir(t, r.stderr)
		if !strings.Contains(r.stderr, "-O0 -fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=all") {
			t.Errorf("gocpp run -asan -x doesn't show the sanitizer flags:\n%s", r.stderr)
		}
		// A heap overflow and a signed overflow of int (not wrapped: the
		// sanitizers stop the program with exit status 1).
		withMain(t, "#include <cstdio>\nint main(int argc, char**) { int* p = new int[2]; int v = p[argc + 1]; delete[] p; std::printf(\"%d\\n\", v); }\n")
		r = gocppRun("", false, "run", "-asan", "hello.gp")
		if r.code != 1 || !strings.Contains(r.stderr, "ERROR: AddressSanitizer: heap-buffer-overflow") {
			t.Errorf("gocpp run -asan of a heap overflow = exit %d, stderr:\n%s\nwant exit 1 and an AddressSanitizer report", r.code, r.stderr)
		}
		withMain(t, "#include <cstdio>\nint main(int argc, char**) { int a[2] = {1, 2}; std::printf(\"%d\\n\", a[argc + 1]); }\n")
		r = gocppRun("", false, "run", "-asan", "hello.gp")
		if r.code != 1 || !strings.Contains(r.stderr, "runtime error: index 2 out of bounds") {
			t.Errorf("gocpp run -asan of an index out of bounds = exit %d, stderr:\n%s\nwant exit 1 and an UndefinedBehaviorSanitizer report", r.code, r.stderr)
		}
	})
}

func TestCheckErrors(t *testing.T) {
	// Errors before the C++: exactly what gocpp check prints, exit 1, and
	// nothing is compiled (no compiler is needed: GOCPP_CXX names none).
	t.Setenv("GOCPP_CXX", "no-such-c++")
	inDir(t, map[string]string{
		"typeerr/main.gp": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(count)\n}\n",
		"gate/main.gp":    "package main\n\nimport \"fmt\"\n\nfunc main() {\n\ts := []int{1}\n\tfmt.Println(s)\n}\n",
		"syntax/main.gp":  "package main\n\nfunc main() {\n\tx :=\n}\n",
		"load/main.gp":    "package main\n\nimport \"example.com/nowhere\"\n\nfunc main() {}\n",
		"warn/main.gp":    "package main\n\nfunc main() {}\n",
		"lib/lib.gp":      "package lib\n\nfunc F() {}\n",
		"nomain/main.gp":  "package main\n\nfunc f() {}\n",
	})
	for _, dir := range []string{"./typeerr", "./gate", "./syntax", "./load"} {
		want := gocppRun("", false, "check", dir)
		if want.code != 1 || want.stderr == "" {
			t.Fatalf("gocpp check %s = %+v, want errors", dir, want)
		}
		for _, cmd := range []string{"build", "run", "emit"} {
			if got := gocppRun("", false, cmd, dir); got != want {
				t.Errorf("gocpp %s %s = %+v, want what gocpp check prints: %+v", cmd, dir, got, want)
			}
		}
	}
	// After the checks: the package must be a program.
	for _, c := range []struct{ dir, stderr string }{
		{"./lib", "package command-line-arguments is not a main package (until Phase 06, gocpp build builds only programs)\n"},
		{"./nomain", "# command-line-arguments\nfunction main is undeclared in the main package\n"},
	} {
		if got := gocppRun("", false, "build", c.dir); got != (result{1, "", c.stderr}) {
			t.Errorf("gocpp build %s = %+v, want exit 1 and %q", c.dir, got, c.stderr)
		}
	}
	// Two packages.
	if got := gocppRun("", false, "build", "./warn", "./lib"); got.code != 1 || !strings.Contains(got.stderr, "the patterns match 2 packages") {
		t.Errorf("gocpp build ./warn ./lib = %+v, want exit 1 and an error", got)
	}
	// A package that checks: now the compiler is needed, and it is missing.
	got := gocppRun("", false, "build", "./warn")
	if want := "gocpp build: GOCPP_CXX: the C++ compiler \"no-such-c++\" was not found: "; got.code != 1 || !strings.HasPrefix(got.stderr, want) {
		t.Errorf("gocpp build with GOCPP_CXX=no-such-c++ = %+v, want exit 1 and %q", got, want)
	}
}

func TestCompilerChoice(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the wrappers are shell scripts")
	}
	c := anyCXX(t)
	bin := t.TempDir()
	dir := inDir(t, map[string]string{"hello.gp": helloSrc})
	// A launcher with the compiler as its argument, like ccache g++.
	launcher := filepath.Join(bin, "launch")
	log := filepath.Join(dir, "launch.log")
	writeScript(t, launcher, "echo \"$1\" >> "+shellQuote(log)+"\nexec \"$@\"\n")
	// A compiler that says it is older than gocpp needs.
	old := filepath.Join(bin, "old-c++")
	major := "__GNUC__ 12"
	if c.cxx.Family == build.Clang {
		major = "__clang_major__ 16"
	}
	writeScript(t, old, "for a in \"$@\"; do if [ \"$a\" = -dM ]; then "+shellQuote(c.cxx.Path)+" \"$@\" | sed 's/^#define "+strings.Fields(major)[0]+" .*/#define "+major+"/'; exit; fi; done\nexec "+shellQuote(c.cxx.Path)+" \"$@\"\n")

	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", launcher+" "+c.cxx.Path)
	if r := gocppRun("", true, "run", "hello.gp"); r != (result{0, helloOut, ""}) {
		t.Errorf("gocpp run with CXX=%q = %+v", launcher+" "+c.cxx.Path, r)
	}
	if data, _ := os.ReadFile(log); !strings.Contains(string(data), c.cxx.Path) {
		t.Errorf("the launcher didn't run the compiler: its log is %q", data)
	}

	for _, e := range []struct{ gocppCXX, cxx, want string }{
		{"no-such-c++", "", `gocpp run: GOCPP_CXX: the C++ compiler "no-such-c++" was not found`},
		{"", "no-such-c++", `gocpp run: CXX: the C++ compiler "no-such-c++" was not found`},
		{"", "'" + launcher, "gocpp run: CXX='" + launcher + ": unterminated ' string"},
		{old, "", "gocpp run: GOCPP_CXX: the C++ compiler " + old + " (" + c.cxx.Version + ") is too old: gocpp needs"},
	} {
		t.Setenv("GOCPP_CXX", e.gocppCXX)
		t.Setenv("CXX", e.cxx)
		if r := gocppRun("", false, "run", "hello.gp"); r.code != 1 || r.stdout != "" || !strings.HasPrefix(r.stderr, e.want) {
			t.Errorf("gocpp run with GOCPP_CXX=%q CXX=%q = %+v, want exit 1 and %q", e.gocppCXX, e.cxx, r, e.want)
		}
	}
}

// writeScript writes a shell script.
func writeScript(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// shellQuote quotes s for sh.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func TestInternalError(t *testing.T) {
	// Step 2.9: C++ that doesn't compile is a gocpp bug. The fault: the
	// call of fmt.Println on line 11 names a function that doesn't exist.
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": helloSrc})
		testHookCPP = func(cpp []byte) []byte {
			return bytes.Replace(cpp, []byte("gp::fmtshim::Println(\"hello"), []byte("gp::fmtshim::Printlnx(\"hello"), 1)
		}
		t.Cleanup(func() { testHookCPP = nil })
		r := gocppRun("", false, "run", "hello.gp")
		if r.code != 2 || r.stdout != "" {
			t.Fatalf("gocpp run = %+v, want exit 2", r)
		}
		lines := strings.Split(r.stderr, "\n")
		if lines[0] != "# command-line-arguments" || lines[1] != "internal error: generated C++ did not compile" {
			t.Errorf("stderr starts with %q, want the internal error:\n%s", lines[:2], r.stderr)
		}
		if !regexp.MustCompile(`(?m)^\./hello\.gp:11:\d+: error: .*Printlnx`).MatchString(r.stderr) {
			t.Errorf("stderr has no error at ./hello.gp:11:\n%s", r.stderr)
		}
		m := regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr)
		if m == nil {
			t.Fatalf("stderr doesn't name the work folder:\n%s", r.stderr)
		}
		defer os.RemoveAll(m[1])
		for _, f := range []string{"hello.gp.cpp", "compile.log"} {
			if _, err := os.Stat(filepath.Join(m[1], f)); err != nil {
				t.Errorf("the work folder has no %s: %v", f, err)
			}
		}

		// A link error: a function that is declared, but not defined.
		testHookCPP = replaceMain("void gp_test_missing();\nint main() { gp_test_missing(); }\n")
		r = gocppRun("", false, "build", "hello.gp")
		if r.code != 2 || !strings.Contains(r.stderr, "internal error: generated C++ did not link\n") || !strings.Contains(r.stderr, "gp_test_missing") {
			t.Errorf("gocpp build of C++ that doesn't link = %+v, want exit 2 and the link error", r)
		}
		if m := regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindStringSubmatch(r.stderr); m != nil {
			os.RemoveAll(m[1])
		}
	})
}

// nixHardening is nixpkgs' default NIX_HARDENING_ENABLE (the setup hook of
// its compiler wrapper), as in the runtime's TestNoFortify: what a user's
// Nix shell has unless it turns hardening off, as the project's dev shell
// does.
const nixHardening = "bindnow format fortify fortify3 libcxxhardeningfast pic relro stackclashprotection stackprotector strictflexarrays1 strictoverflow zerocallusedregs"

func TestNixHardening(t *testing.T) {
	// Under a Nix compiler wrapper with hardening on, which adds
	// -D_FORTIFY_SOURCE after gocpp's flags, gocpp build prints no
	// warning: gocpp/nofortify.h undefines it at -O0. The runtime is
	// compiled again (its cache key has NIX_HARDENING_ENABLE).
	forEachCXX(t, func(t *testing.T, c testCXX) {
		dir := inDir(t, map[string]string{"hello.gp": helloSrc, "plain.cpp": "#include <cstdio>\nint main() { std::puts(\"x\"); }\n"})
		t.Setenv("NIX_HARDENING_ENABLE", nixHardening)
		out, _ := c.cxx.Command(context.Background(), append(build.Flags(c.cxx, false), "-c", "plain.cpp", "-o", filepath.Join(dir, "plain.o"))...).CombinedOutput()
		if !strings.Contains(string(out), "_FORTIFY_SOURCE requires compiling with optimization") {
			t.Skipf("%s with NIX_HARDENING_ENABLE doesn't warn about _FORTIFY_SOURCE at -O0: not a Nix compiler wrapper", c.cxx.Path)
		}
		if r := gocppRun("", true, "run", "hello.gp"); r != (result{0, helloOut, ""}) {
			t.Errorf("gocpp run with hardening = %+v, want exit 0, %q and no warnings", r, helloOut)
		}
	})
}

func TestRunExampleHello(t *testing.T) {
	// examples/hello through gocpp run ., in its folder.
	forEachCXX(t, func(t *testing.T, c testCXX) {
		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Chdir(filepath.Join(wd, "..", "..", "examples", "hello"))
		want, err := os.ReadFile("output.txt")
		if err != nil {
			t.Fatal(err)
		}
		if r := gocppRun("", false, "run", "."); r != (result{0, string(want), ""}) {
			t.Errorf("gocpp run . in examples/hello = %+v, want %q", r, want)
		}
	})
}

func TestRuntimeCache(t *testing.T) {
	// The runtime is extracted and compiled once per compiler and flags:
	// a second build compiles only the program. A broken extraction (a
	// file that someone changed) is replaced.
	c := anyCXX(t)
	t.Setenv("GOCPP_CXX", c.name)
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	inDir(t, map[string]string{"hello.gp": helloSrc})
	count := func(r result) int {
		workDir(t, r.stderr)
		return strings.Count(r.stderr, "/src/rt.cpp ")
	}
	if n := count(gocppRun("", false, "build", "-x", "hello.gp")); n != 1 {
		t.Errorf("the first build compiled rt.cpp %d times, want 1", n)
	}
	if n := count(gocppRun("", false, "build", "-x", "hello.gp")); n != 0 {
		t.Errorf("the second build compiled rt.cpp %d times, want 0", n)
	}
	if n := count(gocppRun("", false, "build", "-x", "-asan", "hello.gp")); n != 1 {
		t.Errorf("the first -asan build compiled rt.cpp %d times, want 1", n)
	}
	rts, _ := filepath.Glob(filepath.Join(cache, "gocpp", "rt", "*", "include", "gocpp", "rt.hpp"))
	if len(rts) != 1 {
		t.Fatalf("the cache has %d runtimes, want 1: %q", len(rts), rts)
	}
	if err := os.WriteFile(rts[0], []byte("#error broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := gocppRun("", true, "run", "hello.gp")
	if r != (result{0, helloOut, ""}) {
		t.Errorf("gocpp run after the runtime in the cache broke = %+v", r)
	}
	if left, _ := filepath.Glob(filepath.Join(cache, "gocpp", "rt", ".*")); len(left) > 0 {
		t.Errorf("temporary folders left in the cache: %q", left)
	}
	// Without a usable cache folder, the runtime goes into the work folder.
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	if r := gocppRun("", true, "run", "hello.gp"); r != (result{0, helloOut, ""}) {
		t.Errorf("gocpp run without a cache folder = %+v", r)
	}
}

func TestGeneratedWarnings(t *testing.T) {
	// Valid Go whose C++ makes the compilers warn without
	// build.GeneratedFlags: clang's -Wtautological-compare (x == x) and
	// g++'s -Woverflow (65535 * uint16(z|-1), folded in int). gocpp run
	// prints only the program's output (from real Go).
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"warn.gp": "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tx := 7\n\tvar z int16 = 3\n\tfmt.Println(x == x, x <= x, 65535*uint16(z|-1))\n}\n"})
		if r := gocppRun("", true, "run", "warn.gp"); r != (result{0, "true true 1\n", ""}) {
			t.Errorf("gocpp run warn.gp = %+v, want exit 0 and only the output", r)
		}
	})
}

func TestNoPackages(t *testing.T) {
	// A pattern that matches nothing: go build only warns; go run fails.
	inDir(t, map[string]string{"gocpp.mod": "module example.com/m\n", "empty/README": "nothing\n"})
	warn := "gocpp: warning: \"./empty/...\" matched no packages\n"
	if r := gocppRun("", false, "build", "./empty/..."); r != (result{0, "", warn}) {
		t.Errorf("gocpp build ./empty/... = %+v, want exit 0 and the warning", r)
	}
	for _, cmd := range []string{"run", "emit"} {
		want := result{1, "", warn + "gocpp " + cmd + ": no packages loaded from ./empty/...\n"}
		if r := gocppRun("", false, cmd, "./empty/..."); r != want {
			t.Errorf("gocpp %s ./empty/... = %+v, want %+v", cmd, r, want)
		}
	}
}

func TestEmitOtherTarget(t *testing.T) {
	// gocpp emit compiles nothing: any GOOS works, like gocpp check; build
	// and run refuse another target.
	inDir(t, map[string]string{"hello.gp": helloSrc})
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	t.Setenv("GOOS", other)
	t.Setenv("GOARCH", "amd64")
	if r := gocppRun("", false, "emit", "hello.gp"); r.code != 0 || !strings.HasPrefix(r.stdout, "// hello.gp.cpp") {
		t.Errorf("GOOS=%s gocpp emit = %+v, want the C++", other, r)
	}
	for _, cmd := range []string{"build", "run"} {
		r := gocppRun("", false, cmd, "hello.gp")
		if r.code != 1 || !strings.Contains(r.stderr, "gocpp "+cmd+": building for "+other+"/amd64 on ") || !strings.HasSuffix(r.stderr, "is not supported yet (Phase 06)\n") {
			t.Errorf("GOOS=%s gocpp %s = %+v, want exit 1 and the Phase 06 message", other, cmd, r)
		}
	}
}
