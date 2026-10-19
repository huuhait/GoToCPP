package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build/testlib"
)

// The tests of the executor and of the incremental build (Phase 06 steps 6.5 to 6.7): -p, the
// aggregation of errors, the actions that a build runs (the "# actions:" line of -x counts them),
// and the check that a build is up to date. They count operations; none reads a clock.

// threePkgs returns a module in which main imports the independent packages a, b and c.
func threePkgs() map[string]string {
	files := map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"main.gp": `package main

import (
	"fmt"

	"example.com/m/a"
	"example.com/m/b"
	"example.com/m/c"
)

func main() { fmt.Println(a.F() + b.F() + c.F()) }
`,
	}
	for i, p := range []string{"a", "b", "c"} {
		files[p+"/"+p+".gp"] = fmt.Sprintf("package %s\n\nfunc F() int { return %d }\n", p, i+1)
	}
	return files
}

// workFolders returns the work folders that the messages of a failed build name, for the cleanup.
func workFolders(t *testing.T, stderr string) []string {
	t.Helper()
	var dirs []string
	for _, m := range regexp.MustCompile(`The work folder is kept: (\S+)\n`).FindAllStringSubmatch(stderr, -1) {
		dirs = append(dirs, m[1])
		t.Cleanup(func() { os.RemoveAll(m[1]) })
	}
	return dirs
}

// The compiles of three packages fail: the build shows all three errors, in the order of the
// packages whatever -p says, with a log file for each, and links nothing.
func TestBuildAggregatesErrors(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, threePkgs())
		testHookCPP = func(path string, cpp []byte) []byte {
			for _, p := range []string{"a", "b", "c"} {
				if path == "gpkg/example.com/m/"+p+"/"+p+".gp.cpp" {
					return append(cpp, []byte("\n#error boom-"+p+"\n")...)
				}
			}
			return cpp
		}
		t.Cleanup(func() { testHookCPP = nil })
		var outputs []string
		for _, p := range []string{"1", "2", "8"} {
			r := gocppRun("", false, "build", "-p", p, "-o", "prog", ".")
			if r.code != 2 || r.stdout != "" {
				t.Fatalf("-p %s: gocpp build = %+v, want exit 2", p, r)
			}
			dirs := workFolders(t, r.stderr)
			if len(dirs) != 1 {
				t.Fatalf("-p %s: %d work folder messages, want one after the last error:\n%s", p, len(dirs), r.stderr)
			}
			// The errors come in the order of the packages, each under its own "# package" line.
			last := -1
			for _, p2 := range []string{"a", "b", "c"} {
				head := strings.Index(r.stderr, "# example.com/m/"+p2+"\ninternal error: generated C++ did not compile\n")
				boom := strings.Index(r.stderr, "error: #error boom-"+p2)
				if boom < 0 {
					boom = strings.Index(r.stderr, "error: boom-"+p2) // clang
				}
				if head < 0 || boom < head || head < last {
					t.Fatalf("-p %s: the error of package %s is missing or out of order (head %d, boom %d, after %d):\n%s", p, p2, head, boom, last, r.stderr)
				}
				last = boom
			}
			for i, name := range []string{"compile.log", "compile-2.log", "compile-3.log"} {
				if _, err := os.Stat(dirs[0] + "/" + name); err != nil {
					t.Errorf("-p %s: error %d has no %s: %v", p, i+1, name, err)
				}
			}
			if _, err := os.Stat("prog"); err == nil {
				t.Errorf("-p %s: the program was linked although compiles failed", p)
			}
			// The same output for every -p, apart from the names of the work folders.
			out := r.stderr
			for _, d := range dirs {
				out = strings.ReplaceAll(out, d, "$WORK")
			}
			outputs = append(outputs, out)
		}
		for i := 1; i < len(outputs); i++ {
			if outputs[i] != outputs[0] {
				t.Errorf("the output with another -p differs:\n%s\n----\n%s", outputs[0], outputs[i])
			}
		}
	})
}

// The C files of three packages do not compile: all three are errors of the user (exit 1), shown in
// the order of the packages.
func TestBuildAggregatesSourceErrors(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	withOneCXX(t, func(t *testing.T, c testCXX) {
		files := threePkgs()
		for _, p := range []string{"a", "b", "c"} {
			files[p+"/"+p+".c"] = "#error bad-" + p + "\n"
		}
		inDir(t, files)
		for _, p := range []string{"1", "6"} {
			r := gocppRun("", false, "build", "-p", p, "-o", "prog", ".")
			if r.code != 1 {
				t.Fatalf("-p %s: gocpp build = %+v, want exit 1", p, r)
			}
			last := -1
			for _, p2 := range []string{"a", "b", "c"} {
				i := strings.Index(r.stderr, "bad-"+p2)
				if i < 0 || i < last {
					t.Fatalf("-p %s: the error of %s.c is missing or out of order:\n%s", p, p2, r.stderr)
				}
				last = i
			}
		}
	})
}

func TestBuildJobsFlag(t *testing.T) {
	inDir(t, map[string]string{"hello.gp": helloSrc})
	for _, v := range []string{"0", "-1", "x", ""} {
		r := gocppRun("", false, "build", "-p", v, "hello.gp")
		if r.code != 2 || !strings.Contains(r.stderr, "invalid value") || !strings.Contains(r.stderr, "must be a positive number") {
			t.Errorf("gocpp build -p %q = %+v, want a usage error", v, r)
		}
	}
	if r := gocppRun("", false, "build", "-h"); !strings.Contains(r.stderr, "-p n") {
		t.Errorf("gocpp build -h does not describe -p:\n%s", r.stderr)
	}
}

// With -p 1 the compiles run one after the other, in the order of the packages (the order of
// the lines of -x), and the units of a package come in the order of its files.
func TestBuildJobsOneOrder(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	withOneCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, threePkgs())
		r := gocppRun("", false, "build", "-p", "1", "-x", "-o", "prog", ".")
		if r.code != 0 {
			t.Fatalf("gocpp build = %+v", r)
		}
		workDir(t, r.stderr)
		got := unitCompiles(r.stderr)
		want := []string{"gpkg/example.com/m/a/a.gp.cpp", "gpkg/example.com/m/b/b.gp.cpp", "gpkg/example.com/m/c/c.gp.cpp", "gpkg/example.com/m/main.gp.cpp", "gpkg/example.com/m/pkg_init.cpp"}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("-p 1 compiles %v, want %v", got, want)
		}
	})
}

// ---- the actions of a build, and the up-to-date check (steps 6.5 and 6.6)

// counts are the numbers of the "# actions:" line of -x.
type counts struct {
	load, check, emit, write, compile, hit, link, runtime int
}

func actionCounts(t *testing.T, stderr string) counts {
	t.Helper()
	m := regexp.MustCompile(`(?m)^# actions: load=(\d+) check=(\d+) emit=(\d+) write=(\d+) compile=(\d+) hit=(\d+) link=(\d+) runtime=(\d+)$`).FindStringSubmatch(stderr)
	if m == nil {
		t.Fatalf("no \"# actions:\" line in stderr:\n%s", stderr)
	}
	var n [8]int
	for i := range n {
		fmt.Sscan(m[i+1], &n[i])
	}
	return counts{n[0], n[1], n[2], n[3], n[4], n[5], n[6], n[7]}
}

// isUpToDate reports whether the -x output says that the build found its program current.
func isUpToDate(stderr string) bool {
	return regexp.MustCompile(`(?m)^# up to date: `).MatchString(stderr)
}

// notUpToDate returns the reason that -x gives for a build that was not up to date, or "".
func notUpToDate(stderr string) string {
	m := regexp.MustCompile(`(?m)^# not up to date: (.*)$`).FindStringSubmatch(stderr)
	if m == nil {
		return ""
	}
	return m[1]
}

// incr is a project built over and over with the object cache and the up-to-date check on.
type incr struct {
	t     *testing.T
	dir   string
	args  []string
	stamp string // a text that makes the program's objects new to the object cache
}

// newIncr starts a project: files, in a folder of its own, with the object cache on. Each file
// gets the unique stamp in place of @STAMP@, so that the first build misses whatever the shared
// cache holds.
func newIncr(t *testing.T, files map[string]string, args ...string) *incr {
	t.Helper()
	t.Setenv("GOCPP_GATE_PHASE", "06")
	cacheOn(t)
	t.Setenv("GOCPP_NOUPTODATE", "") // the check is what these tests are about
	stamp := fmt.Sprintf("%s-%d", strings.ReplaceAll(t.Name(), "/", "-"), time.Now().UnixNano())
	st := &incr{t: t, args: args, stamp: stamp}
	fixed := make(map[string]string)
	for name, text := range files {
		fixed[name] = strings.ReplaceAll(text, "@STAMP@", stamp)
	}
	st.dir = inDir(t, fixed)
	ageTree(t, st.dir)
	return st
}

// write changes files (with @STAMP@ replaced) and ages the folder, so that the files are old
// enough for the stat of a file to be trusted, as it would be in a project that was not just edited.
func (s *incr) write(files map[string]string) {
	s.t.Helper()
	fixed := make(map[string]string)
	for name, text := range files {
		fixed[name] = strings.ReplaceAll(text, "@STAMP@", s.stamp)
	}
	writeFiles(s.t, s.dir, fixed)
	ageTree(s.t, s.dir)
}

// build runs gocpp build -x with the arguments of the project.
func (s *incr) build() (r result, c counts) {
	s.t.Helper()
	args := append([]string{"build", "-x"}, s.args...)
	args = append(args, "-o", "prog", ".")
	r = gocppRun("", false, args...)
	if strings.Contains(r.stderr, "WORK=") { // a build that is up to date has no work folder
		workDir(s.t, r.stderr) // removes it at the end of the test
	}
	if r.code != 0 {
		s.t.Fatalf("gocpp build = %+v", r)
	}
	return r, actionCounts(s.t, r.stderr)
}

// run runs the program that the last build made.
func (s *incr) run() string {
	s.t.Helper()
	out, err := exec.Command(filepath.Join(s.dir, "prog")).Output()
	if err != nil {
		s.t.Fatalf("prog: %v", err)
	}
	return string(out)
}

// stampProject is pkgsProject with the stamp in the text of package a (its tag).
func stampProject(limit int) map[string]string {
	files := pkgsProject("@STAMP@", limit)
	// A package variable gives main's pkg_init.cpp a text of its own, so that no unit of the
	// program is in the object cache from another test.
	files["main.gp"] += "\nvar stampVar = \"@STAMP@\"\n"
	return files
}

func TestUpToDateNoop(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		first, n := s.build()
		if n.load != 1 || n.emit != 3 || n.compile != 4 || n.hit != 0 || n.link != 1 {
			t.Errorf("the first build did %+v, want load 1, emit 3, compile 4, link 1:\n%s", n, first.stderr)
		}
		if got := s.run(); got != "8 10 "+s.stamp+"\n" {
			t.Fatalf("the program prints %q", got)
		}
		if why := notUpToDate(first.stderr); !strings.Contains(why, "no record") {
			t.Errorf("the first build says %q, want that there is no record", why)
		}
		fi1, err := os.Stat("prog")
		if err != nil {
			t.Fatal(err)
		}
		// Nothing changed: no load, no check, no emit, no compile, no link, and the program is
		// not touched.
		second, n := s.build()
		if !isUpToDate(second.stderr) || n != (counts{}) {
			t.Errorf("the second build did %+v, want nothing, up to date:\n%s", n, second.stderr)
		}
		fi2, _ := os.Stat("prog")
		if !os.SameFile(fi1, fi2) || !fi1.ModTime().Equal(fi2.ModTime()) {
			t.Errorf("the up-to-date build touched the program")
		}
		// Without -x it prints nothing at all.
		if r := gocppRun("", false, "build", "-o", "prog", "."); r != (result{}) {
			t.Errorf("gocpp build of an up-to-date program = %+v, want no output", r)
		}
		// And again after a second: still up to date (the folders were looked at once more).
		if third, n := s.build(); !isUpToDate(third.stderr) || n != (counts{}) {
			t.Errorf("the third build did %+v:\n%s", n, third.stderr)
		}
	})
}

// A change in a function body of package a: the front end runs again (nothing is cached across
// builds of the front end), one generated file changes, one unit compiles, the others are
// answered by the object cache, and the program is linked.
func TestUpToDateBodyChange(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		s.write(map[string]string{"a/a.gp": pkgsProject("@STAMP@ body", 5)["a/a.gp"]})
		r, n := s.build()
		if isUpToDate(r.stderr) || !strings.Contains(notUpToDate(r.stderr), "a.gp changed") {
			t.Errorf("the build after a change says %q:\n%s", notUpToDate(r.stderr), r.stderr)
		}
		want := counts{load: 1, emit: 3, write: 1, compile: 1, hit: 3, link: 1}
		n.check = 0 // the number of packages (with the standard ones) is not what this test is about
		if n != want {
			t.Errorf("a changed body did %+v, want %+v:\n%s", n, want, r.stderr)
		}
		if got := unitCompiles(r.stderr); fmt.Sprint(got) != fmt.Sprint([]string{"gpkg/example.com/m/a/a.gp.cpp"}) {
			t.Errorf("a changed body compiles %v, want only a.gp.cpp", got)
		}
		// The API hash of a (the hash of pkg.hpp) is the same, so its dependents did not compile.
		if !strings.Contains(r.stderr, "# sources changed: example.com/m/a\n") || strings.Contains(r.stderr, "# api changed") {
			t.Errorf("a changed body: the summaries should say that only the sources of a changed:\n%s", r.stderr)
		}
		if got := s.run(); got != "8 10 "+s.stamp+" body\n" {
			t.Errorf("the program prints %q", got)
		}
		// Built again, it is up to date.
		if r, n := s.build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Errorf("after the rebuild: %+v:\n%s", n, r.stderr)
		}
	})
}

// A change of an exported constant of a: its header changes, and the units of the packages that
// include it compile again (the API hash changes).
func TestUpToDateConstantChange(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		s.write(map[string]string{"a/a.gp": stampProject(6)["a/a.gp"]})
		r, n := s.build()
		if n.compile != 4 || n.hit != 0 || n.link != 1 || n.write < 2 {
			t.Errorf("a changed constant did %+v, want 4 compiles (a, b, main and its init) and a link:\n%s", n, r.stderr)
		}
		if !strings.Contains(r.stderr, "# api changed: example.com/m/a\n") || strings.Contains(r.stderr, "# api changed: example.com/m/a example.com/m/b") {
			t.Errorf("a changed constant: the summaries should say that the API of a changed:\n%s", r.stderr)
		}
		if got := s.run(); got != "8 12 "+s.stamp+"\n" {
			t.Errorf("the program prints %q", got)
		}
	})
}

// A file added to a package, and one removed: the loader's listing of the folder changed.
func TestUpToDateAddRemoveFile(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		// A new file in package b that main prints.
		s.write(map[string]string{
			"b/extra.gp": "package b\n\nfunc Extra() int { return 40 }\n",
			"main.gp":    strings.Replace(stampProject(5)["main.gp"], "a.Tag())", "a.Tag(), b.Extra())", 1),
		})
		r, n := s.build()
		if reason := notUpToDate(r.stderr); !strings.Contains(reason, "b changed") && !strings.Contains(reason, "main.gp changed") {
			t.Errorf("the build after adding a file says %q", reason)
		}
		if n.link != 1 || n.compile < 2 {
			t.Errorf("adding a file did %+v:\n%s", n, r.stderr)
		}
		if got := s.run(); got != "8 10 "+s.stamp+" 40\n" {
			t.Errorf("the program prints %q", got)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date after the build:\n%s", r.stderr)
		}
		// Removing it again (and its use): the object of the removed file is not linked.
		os.Remove("b/extra.gp")
		s.write(map[string]string{"main.gp": stampProject(5)["main.gp"]})
		r, n = s.build()
		if isUpToDate(r.stderr) || n.link != 1 {
			t.Errorf("removing a file did %+v:\n%s", n, r.stderr)
		}
		if got := s.run(); got != "8 10 "+s.stamp+"\n" {
			t.Errorf("the program prints %q", got)
		}
		// A file that is no source (a note, the program's own output) changes nothing.
		os.WriteFile("b/NOTES.txt", []byte("hello"), 0o644)
		if r, n := s.build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Errorf("a new text file made the build not up to date:\n%s", r.stderr)
		}
	})
}

// The flags, the environment, the compiler and the program's own state are all part of the check.
func TestUpToDateInputs(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Fatalf("not up to date:\n%s", r.stderr)
		}
		// A flag of the environment: another question, compiled again; going back finds the old
		// stamp, but the program was replaced by the other build's, so it is linked again (and every
		// unit is in the object cache).
		t.Setenv("GOCPP_CXXFLAGS", "-DEXTRA_FLAG=1")
		r, n := s.build()
		if n.compile != 4 || n.link != 1 {
			t.Errorf("a new flag did %+v, want all four units compiled:\n%s", n, r.stderr)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date with the flag:\n%s", r.stderr)
		}
		t.Setenv("GOCPP_CXXFLAGS", "")
		r, n = s.build()
		if isUpToDate(r.stderr) || n.compile != 0 || n.hit != 4 || n.link != 1 {
			t.Errorf("going back to the first flags did %+v, want 4 hits and a link:\n%s", n, r.stderr)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date after going back:\n%s", r.stderr)
		}

		// The program: removed, or changed, or touched; each is built again (the units hit).
		os.Remove("prog")
		r, n = s.build()
		if isUpToDate(r.stderr) || n.link != 1 || n.compile != 0 || n.load != 1 {
			t.Errorf("a removed program did %+v:\n%s", n, r.stderr)
		}
		if s.run() != "8 10 "+s.stamp+"\n" {
			t.Errorf("the program is wrong after it was built again")
		}
		os.WriteFile("prog", []byte("#!/bin/sh\necho changed\n"), 0o755)
		r, n = s.build()
		if isUpToDate(r.stderr) || n.link != 1 {
			t.Errorf("a changed program did %+v:\n%s", n, r.stderr)
		}
		if s.run() != "8 10 "+s.stamp+"\n" {
			t.Errorf("the changed program was not replaced")
		}
		// (A program that was only touched is still the same program: when its time is too new to
		// trust, the check compares its hash, and finds it the same.)

		// -asan and -nochecks are part of the question.
		s2 := *s
		s2.args = []string{"-nochecks"}
		if r, n := s2.build(); isUpToDate(r.stderr) || n.compile == 0 {
			t.Errorf("-nochecks did %+v:\n%s", n, r.stderr)
		}
		if r, _ := s2.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date with -nochecks:\n%s", r.stderr)
		}

		// The other compiler.
		other := "g++"
		if filepath.Base(c.name) == "g++" {
			other = "clang++"
		}
		if _, err := exec.LookPath(other); err == nil {
			t.Setenv("GOCPP_CXX", other)
			if r, n := s.build(); isUpToDate(r.stderr) || n.compile == 0 {
				t.Errorf("the other compiler did %+v:\n%s", n, r.stderr)
			}
		}
	})
}

// gocpp.mod is read by the loader, so a change of it (here a comment) is a change of the trace.
func TestUpToDateModuleFile(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		s.write(map[string]string{"gocpp.mod": "module example.com/m\n\n// a note\n"})
		r, n := s.build()
		if !strings.Contains(notUpToDate(r.stderr), "gocpp.mod changed") || n.link != 1 || n.compile != 0 {
			t.Errorf("a changed gocpp.mod did %+v, reason %q:\n%s", n, notUpToDate(r.stderr), r.stderr)
		}
		// So is gocpp.sum, which the downloads of modules write and check.
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Fatalf("not up to date:\n%s", r.stderr)
		}
		s.write(map[string]string{"gocpp.sum": "example.com/x v1.0.0 h1:abc=\n"})
		r, n = s.build()
		if notUpToDate(r.stderr) == "" || n.link != 1 {
			t.Errorf("a new gocpp.sum did %+v, reason %q:\n%s", n, notUpToDate(r.stderr), r.stderr)
		}
	})
}

// A header that the package's C file includes, from the package's folder and from another one
// (-I): the C++ compiler read them, the loader did not, so they are in the stamp through the
// compile.
func TestUpToDateHeaders(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		files := map[string]string{
			"proj/gocpp.mod": "module example.com/h\n",
			"proj/main.gp":   "//gocpp:cflags -I../inc\npackage main\n\nfunc main() { println(\"hdr @STAMP@\") }\n",
			"proj/util.c":    "#include \"util.h\"\n#include \"shared.h\"\nint util_val(void) { return UTIL_VAL + SHARED_VAL; }\n",
			"proj/util.h":    "#define UTIL_VAL 1\n",
			"inc/shared.h":   "#define SHARED_VAL 2\n",
		}
		s := newIncr(t, files)
		s.dir = filepath.Join(s.dir, "proj")
		t.Chdir(s.dir)
		_, n := s.build()
		if n.link != 1 || n.compile < 3 {
			t.Fatalf("the first build did %+v", n)
		}
		if r, n := s.build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Fatalf("not up to date:\n%s", r.stderr)
		}
		for _, h := range []struct{ file, text string }{
			{"util.h", "#define UTIL_VAL 10\n"},
			{"../inc/shared.h", "#define SHARED_VAL 20\n"},
		} {
			writeFiles(t, s.dir, map[string]string{h.file: h.text})
			ageTree(t, filepath.Dir(s.dir))
			r, n := s.build()
			if isUpToDate(r.stderr) || !strings.Contains(notUpToDate(r.stderr), filepath.Base(h.file)) || n.compile != 1 || n.link != 1 {
				t.Errorf("a changed %s did %+v, reason %q:\n%s", h.file, n, notUpToDate(r.stderr), r.stderr)
			}
			if r, _ := s.build(); !isUpToDate(r.stderr) {
				t.Errorf("not up to date after the build with %s:\n%s", h.file, r.stderr)
			}
		}
		// The C source itself.
		writeFiles(t, s.dir, map[string]string{"util.c": "#include \"util.h\"\n#include \"shared.h\"\nint util_val(void) { return 7; }\n"})
		ageTree(t, filepath.Dir(s.dir))
		if r, n := s.build(); isUpToDate(r.stderr) || n.compile != 1 {
			t.Errorf("a changed C file did %+v:\n%s", n, r.stderr)
		}
	})
}

// A compile that printed a warning is not kept by the object cache, so the build is not stamped
// and the warning comes with every build.
func TestUpToDateWarningsRepeat(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, map[string]string{
			"gocpp.mod": "module example.com/w\n",
			"main.gp":   "package main\n\nfunc main() { println(\"w @STAMP@\") }\n",
			"warn.c":    "#warning from-warn-c\nint warn_val(void) { return 1; }\n",
		})
		for i := range 3 {
			r, _ := s.build()
			if isUpToDate(r.stderr) || !strings.Contains(r.stderr, "from-warn-c") || !strings.Contains(r.stderr, "# not stamped: ") {
				t.Fatalf("build %d: the warning must come again and the build is not stamped:\n%s", i, r.stderr)
			}
		}
	})
}

// Things that are never stamped say so under -x and still build.
func TestUpToDateOff(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		s.build()
		cases := []struct {
			name, why string
			setup     func(t *testing.T)
			args      []string
		}{
			{"env", "GOCPP_NOUPTODATE", func(t *testing.T) { t.Setenv("GOCPP_NOUPTODATE", "1") }, []string{"."}},
			{"work", "-work", func(t *testing.T) {}, []string{"-work", "."}},
			{"pattern", "a pattern with ...", func(t *testing.T) {}, []string{"./..."}},
			{"nocache", "the object cache is off", func(t *testing.T) { t.Setenv("GOCPP_NOOBJCACHE", "1") }, []string{"."}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				tc.setup(t)
				args := append(append([]string{"build", "-x", "-o", "prog"}, tc.args[:len(tc.args)-1]...), tc.args[len(tc.args)-1])
				r := gocppRun("", false, args...)
				workDir(t, r.stderr)
				if r.code != 0 || isUpToDate(r.stderr) || !strings.Contains(r.stderr, "# not stamped: ") || !strings.Contains(r.stderr, tc.why) {
					t.Errorf("gocpp %v = %+v, want a build that says it is not stamped because of %q", args, r, tc.why)
				}
			})
		}
	})
}

// pkg-config's answers and the libraries that the link reads are inputs of the check: a .pc file
// that gives other flags, and a library that was replaced, make the build not up to date.
func TestUpToDatePkgConfigAndLibs(t *testing.T) {
	testlib.NeedPkgConfig(t)
	forEachCXX(t, func(t *testing.T, c testCXX) {
		prefix := filepath.Clean(t.TempDir())
		pcDir := testlib.Install(t, prefix, &c.cxx)
		t.Setenv("PKG_CONFIG_PATH", pcDir)
		t.Setenv("PKG_CONFIG", "")
		s := newIncr(t, map[string]string{
			"gocpp.mod": "module example.com/pc\n",
			"main.gp":   "//gocpp:pkg-config gptestlib\n//gocpp:link m\npackage main\n\nfunc main() { println(\"pc @STAMP@\") }\n",
			// This file needs the library: the linker takes a member out of the archive, and says so
			// in its trace, which must not come out as a message of the build.
			"user.c": "extern int gptestlib_answer(void);\nint use_it(void) { return gptestlib_answer(); }\n",
		})
		r, n := s.build()
		if n.link != 1 || !strings.Contains(r.stderr, "pkg-config --libs -- gptestlib") {
			t.Fatalf("the first build did %+v:\n%s", n, r.stderr)
		}
		for _, l := range strings.Split(r.stderr, "\n") {
			if strings.Contains(l, "libgptestlib.a)") || strings.HasPrefix(l, "(/") {
				t.Errorf("the trace of the linker is in the output of the build: %q", l)
			}
		}
		if r, n := s.build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Fatalf("a program with pkg-config and -lm is not up to date:\n%s", r.stderr)
		}

		// A flag that the .pc file gives changes: the answers of pkg-config are asked again.
		pc := filepath.Join(pcDir, "gptestlib.pc")
		data, err := os.ReadFile(pc)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(pc, []byte(strings.Replace(string(data), "-DGPTESTLIB_ON", "-DGPTESTLIB_ON -DEXTRA_PC=1", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		r, n = s.build()
		if why := notUpToDate(r.stderr); !strings.Contains(why, "pkg-config gptestlib answers differently") || n.compile == 0 || n.link != 1 {
			t.Errorf("a changed .pc file: reason %q, %+v:\n%s", why, n, r.stderr)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date after the build with the new .pc file:\n%s", r.stderr)
		}

		// The library that -lgptestlib finds is replaced by another archive.
		lib := filepath.Join(prefix, "lib", "libgptestlib.a")
		obj := filepath.Join(prefix, "src", "gptestlib.o")
		src := filepath.Join(prefix, "src", "gptestlib.cpp")
		if err := os.WriteFile(src, []byte("#include \"gptestlib.h\"\nextern \"C\" int gptestlib_answer(void) { return 43; }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if out, err := c.cxx.Command(context.Background(), "-std=c++20", "-DGPTESTLIB_ON", "-I", filepath.Join(prefix, "include"), "-c", src, "-o", obj).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		if out, err := exec.Command("ar", "rcs", lib, obj).CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		r, n = s.build()
		if why := notUpToDate(r.stderr); !strings.Contains(why, "libgptestlib.a changed") || n.link != 1 || n.compile != 0 {
			t.Errorf("a changed library: reason %q, %+v:\n%s", why, n, r.stderr)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date after the build with the new library:\n%s", r.stderr)
		}
	})
}

// A link flag that may read a file the check can't follow makes the build unstampable, with the
// reason: it works, and every build of it does its work (the object cache answers the compiles).
func TestUpToDateUnstampableLink(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("GOCPP_LDFLAGS", "-Wl,-Map=/dev/null")
		s := newIncr(t, map[string]string{
			"gocpp.mod": "module example.com/l\n",
			"main.gp":   "package main\n\nfunc main() { println(\"l @STAMP@\") }\n",
		})
		for i := range 2 {
			r, n := s.build()
			if isUpToDate(r.stderr) || n.link != 1 || !strings.Contains(r.stderr, "# not stamped: the link flag -Wl,-Map=/dev/null") {
				t.Errorf("build %d: %+v:\n%s", i, n, r.stderr)
			}
		}
	})
}

// Two builds of the same program at the same time (the same folder, the same output): both work, and
// what they leave is a program and a stamp that a third build finds up to date or rebuilds, never
// a wrong program.
func TestUpToDateConcurrentBuilds(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		// The units are built once first, so that the two builds race on the stamp and the program
		// and not on a cold cache.
		s.build()
		os.Remove("prog")
		os.Remove("prog2")
		var wg sync.WaitGroup
		results := make([]result, 4)
		for i := range results {
			wg.Go(func() {
				out := "prog"
				if i%2 == 1 {
					out = "prog2"
				}
				results[i] = gocppRun("", false, "build", "-o", out, ".")
			})
		}
		wg.Wait()
		for i, r := range results {
			if r != (result{}) {
				t.Errorf("concurrent build %d = %+v", i, r)
			}
		}
		for _, p := range []string{"prog", "prog2"} {
			out, err := exec.Command(filepath.Join(s.dir, p)).Output()
			if err != nil || string(out) != "8 10 "+s.stamp+"\n" {
				t.Errorf("%s prints %q, %v", p, out, err)
			}
		}
		// A build after them works, and the one after that is up to date.
		s.build()
		if r, n := s.build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Errorf("not up to date after the concurrent builds:\n%s", r.stderr)
		}
		if got := s.run(); got != "8 10 "+s.stamp+"\n" {
			t.Errorf("the program prints %q", got)
		}
	})
}

// A warning of the check comes with every build, so a build that printed one is not stamped.
func TestUpToDateCheckWarnings(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, map[string]string{
			"gocpp.mod": "module example.com/w\n",
			"main.gp":   "// gocpp:link m\npackage main\n\nfunc main() { println(\"w @STAMP@\") }\n",
		})
		for i := range 2 {
			r, _ := s.build()
			if isUpToDate(r.stderr) || !strings.Contains(r.stderr, "is an ordinary comment, not a directive") || !strings.Contains(r.stderr, "# not stamped: the check printed warnings") {
				t.Fatalf("build %d: the warning must come again and the build is not stamped:\n%s", i, r.stderr)
			}
		}
	})
}

// The content of gocpp.sum (which the loader does not read: the downloads of modules do) is part of
// the check.
func TestUpToDateSumContent(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		files := stampProject(5)
		files["gocpp.sum"] = "example.com/x v1.0.0 h1:aaa=\n"
		s := newIncr(t, files)
		s.build()
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Fatalf("not up to date:\n%s", r.stderr)
		}
		s.write(map[string]string{"gocpp.sum": "example.com/x v1.0.0 h1:bbb=\n"})
		if r, n := s.build(); !strings.Contains(notUpToDate(r.stderr), "gocpp.sum changed") || n.link != 1 {
			t.Errorf("a changed gocpp.sum did %+v, reason %q:\n%s", n, notUpToDate(r.stderr), r.stderr)
		}
	})
}

// Two programs of one module built into the same output file: the record of one is never taken for
// the other's, and an output file that was not asked for before is written.
func TestUpToDateWhichProgram(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		files := stampProject(5)
		files["cmd/other/main.gp"] = "package main\n\nimport \"example.com/m/a\"\n\nfunc main() { println(\"other\", a.Limit) }\n"
		files["cmd/other/stamp.gp"] = "package main\n\nvar stampVar = \"@STAMP@\"\n"
		newIncr(t, files) // (the helper's own build is not used: the arguments differ)
		build := func(args ...string) result {
			t.Helper()
			r := gocppRun("", false, append([]string{"build", "-x"}, args...)...)
			if strings.Contains(r.stderr, "WORK=") {
				workDir(t, r.stderr)
			}
			if r.code != 0 {
				t.Fatalf("gocpp build %v = %+v", args, r)
			}
			return r
		}
		out := func(p string) string {
			t.Helper()
			b, err := exec.Command("./" + p).CombinedOutput()
			if err != nil {
				t.Fatalf("%s: %v", p, err)
			}
			return string(b)
		}
		build("-o", "prog", ".")
		if !strings.HasPrefix(out("prog"), "8 10 ") {
			t.Fatalf("prog prints %q", out("prog"))
		}
		build("-o", "prog", "./cmd/other") // the same output file, another program
		if got := out("prog"); got != "other 5\n" {
			t.Fatalf("prog prints %q after the other program", got)
		}
		r := build("-o", "prog", ".") // the first again: the file is the other program's
		if isUpToDate(r.stderr) || !strings.HasPrefix(out("prog"), "8 10 ") {
			t.Errorf("the first program again: up to date %v, prog prints %q", isUpToDate(r.stderr), out("prog"))
		}
		// Another output file: it is written, though the program is current.
		os.Remove("prog2")
		r = build("-o", "prog2", ".")
		if isUpToDate(r.stderr) || !strings.HasPrefix(out("prog2"), "8 10 ") {
			t.Errorf("another output: up to date %v", isUpToDate(r.stderr))
		}
		if r := build("-o", "prog2", "."); !isUpToDate(r.stderr) {
			t.Errorf("prog2 is not up to date:\n%s", r.stderr)
		}
		// And the first output file is still there and still current.
		if r := build("-o", "prog", "."); !isUpToDate(r.stderr) {
			t.Errorf("prog is not up to date again:\n%s", r.stderr)
		}
	})
}

// traceShape reduces the -x output of a build to its shape: the lines of gocpp itself ("# ..."), with
// the folders and the hashes written as placeholders, each compile as COMPILE <unit>, the link as LINK
// and the move as MOVE.
func traceShape(t *testing.T, dir, stderr string) string {
	t.Helper()
	work := regexp.MustCompile(`(?m)^WORK=(.+)$`).FindStringSubmatch(stderr)
	cache := os.Getenv("XDG_CACHE_HOME")
	var out []string
	for _, l := range strings.Split(strings.TrimRight(stderr, "\n"), "\n") {
		switch {
		case strings.HasPrefix(l, "WORK="):
			out = append(out, "WORK=$WORK")
			continue
		case strings.HasPrefix(l, "mv "):
			out = append(out, "MOVE")
			continue
		case !strings.HasPrefix(l, "#"):
			if m := regexp.MustCompile(` -c (?:-x c\+\+ )?\S*/(gpkg/\S+) -o `).FindStringSubmatch(l); m != nil {
				out = append(out, "COMPILE "+m[1])
			} else if strings.Contains(l, " -Wl,--gc-sections ") {
				out = append(out, "LINK")
			} else {
				out = append(out, "?? "+l)
			}
			continue
		}
		l = strings.ReplaceAll(l, dir, "$DIR")
		if work != nil {
			l = strings.ReplaceAll(l, work[1], "$WORK")
		}
		l = strings.ReplaceAll(l, cache, "$CACHE")
		l = regexp.MustCompile(`\$CACHE/gocpp/gen/[0-9a-f]{16}`).ReplaceAllString(l, "$$GEN")
		l = regexp.MustCompile(`(# cache hit \S+) [0-9a-f]{16}$`).ReplaceAllString(l, "$1 HASH")
		l = regexp.MustCompile(`(# api \S+) [0-9a-f]{16}$`).ReplaceAllString(l, "$1 API")
		out = append(out, l)
	}
	return strings.Join(out, "\n") + "\n"
}

// The whole -x output of the builds of a program of three packages, with -p 1 (the order of the
// compiles is fixed): a first build, one with nothing to do, one after a change of a body, one after
// a change of a constant. This is the format of what -x says about the actions.
func TestBuildTraceGolden(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5), "-p", "1")
		shape := func() string {
			r, _ := s.build()
			return traceShape(t, s.dir, r.stderr)
		}
		const header = "# not up to date: no record of an earlier build\nWORK=$WORK\n# gocpp emit > $GEN\n"
		const apis = "# api example.com/m/a API\n# api example.com/m/b API\n# api example.com/m API\n"
		first := shape()
		wantFirst := header + apis +
			"COMPILE gpkg/example.com/m/a/a.gp.cpp\nCOMPILE gpkg/example.com/m/b/b.gp.cpp\nCOMPILE gpkg/example.com/m/main.gp.cpp\nCOMPILE gpkg/example.com/m/pkg_init.cpp\nLINK\nMOVE\n" +
			"# actions: load=1 check=5 emit=3 write=7 compile=4 hit=0 link=1 runtime=0\n"
		if first != wantFirst {
			t.Errorf("the first build:\n%s\nwant\n%s", first, wantFirst)
		}
		if got, want := shape(), "# up to date: $DIR/prog\n# actions: load=0 check=0 emit=0 write=0 compile=0 hit=0 link=0 runtime=0\n"; got != want {
			t.Errorf("a build with nothing to do:\n%s\nwant\n%s", got, want)
		}
		s.write(map[string]string{"a/a.gp": pkgsProject("@STAMP@ body", 5)["a/a.gp"]})
		got := shape()
		want := "# not up to date: $DIR/a/a.gp changed\nWORK=$WORK\n# gocpp emit > $GEN\n" + apis +
			"# sources changed: example.com/m/a\n" +
			"COMPILE gpkg/example.com/m/a/a.gp.cpp\n" +
			"# cache hit gpkg/example.com/m/b/b.gp.cpp HASH\n# cache hit gpkg/example.com/m/main.gp.cpp HASH\n# cache hit gpkg/example.com/m/pkg_init.cpp HASH\n" +
			"LINK\nMOVE\n# actions: load=1 check=5 emit=3 write=1 compile=1 hit=3 link=1 runtime=0\n"
		if got != want {
			t.Errorf("after a change of a body:\n%s\nwant\n%s", got, want)
		}
		s.write(map[string]string{"a/a.gp": stampProject(6)["a/a.gp"]})
		got = shape()
		want = "# not up to date: $DIR/a/a.gp changed\nWORK=$WORK\n# gocpp emit > $GEN\n" + apis +
			"# sources changed: example.com/m/a\n# api changed: example.com/m/a\n" +
			"COMPILE gpkg/example.com/m/a/a.gp.cpp\nCOMPILE gpkg/example.com/m/b/b.gp.cpp\nCOMPILE gpkg/example.com/m/main.gp.cpp\nCOMPILE gpkg/example.com/m/pkg_init.cpp\n" +
			"LINK\nMOVE\n# actions: load=1 check=5 emit=3 write=3 compile=4 hit=0 link=1 runtime=0\n"
		if got != want {
			t.Errorf("after a change of a constant:\n%s\nwant\n%s", got, want)
		}
	})
}

// Units of one package that fail for the same reason (an error in a header that they include) show
// the errors once.
func TestBuildSameErrorsOnce(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	withOneCXX(t, func(t *testing.T, c testCXX) {
		files := threePkgs()
		files["a/a2.gp"] = "package a\n\nfunc G() int { return 9 }\n"
		inDir(t, files)
		testHookCPP = func(path string, cpp []byte) []byte {
			if path == "gpkg/example.com/m/a/pkg.hpp" {
				return append(cpp, []byte("\n#error shared-header-error\n")...)
			}
			return cpp
		}
		t.Cleanup(func() { testHookCPP = nil })
		r := gocppRun("", false, "build", "-p", "1", "-o", "prog", ".")
		workFolders(t, r.stderr)
		if r.code != 2 {
			t.Fatalf("gocpp build = %+v, want exit 2", r)
		}
		// Package a: two units, the error of the header once; the second one says "the same".
		if n := strings.Count(r.stderr, "shared-header-error"); n != 2 { // a's first unit, and main's first unit
			t.Errorf("the header error is shown %d times, want once for a and once for the package that imports it:\n%s", n, r.stderr)
		}
		if !strings.Contains(r.stderr, "# example.com/m/a\n(the same errors in a2.gp.cpp)\n") {
			t.Errorf("the second unit of a does not say that its errors are the same:\n%s", r.stderr)
		}
		if strings.Count(r.stderr, "This is a bug in gocpp. The work folder is kept") != 1 {
			t.Errorf("the message about the work folder should come once, at the end:\n%s", r.stderr)
		}
	})
}

// Every variable that the compiler, the wrappers and the linker read is part of the question: set after
// a build, it makes the build not up to date (the program may differ: a NIX_LDFLAGS with -rpath changes
// the RUNPATH of the program).
func TestUpToDateLinkerEnv(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(5))
		for _, v := range []struct{ name, value string }{
			{"NIX_LDFLAGS", os.Getenv("NIX_LDFLAGS") + " -rpath /tmp/gocpp-test-rpath"},
			{"NIX_LDFLAGS_BEFORE", "-rpath /tmp/gocpp-test-rpath"},
			{"NIX_LDFLAGS_AFTER", "-rpath /tmp/gocpp-test-rpath"},
			{"NIX_CFLAGS_LINK", "-Wl,-rpath,/tmp/gocpp-test-rpath"},
			{"NIX_CFLAGS_COMPILE", os.Getenv("NIX_CFLAGS_COMPILE") + " -DGOCPP_TEST_X=1"},
			{"NIX_BINTOOLS_WRAPPER_FLAGS_SET_x", "1"},
			{"NIX_DONT_SET_RPATH", "1"},
			{"NIX_SOMETHING_NEW", "1"},
			{"LD_RUN_PATH", "/tmp/gocpp-test-rpath"},
			{"LD_LIBRARY_PATH", "/tmp/gocpp-test-ldlib"},
			{"LIBRARY_PATH", "/tmp/gocpp-test-lib"},
			{"CPATH", "/tmp/gocpp-test-inc"},
			{"C_INCLUDE_PATH", "/tmp/gocpp-test-inc"},
			{"CPLUS_INCLUDE_PATH", "/tmp/gocpp-test-inc"},
			{"COMPILER_PATH", "/tmp/gocpp-test-cp"}, // (GCC_EXEC_PREFIX is in the key too, but a wrong one breaks the compiler)
			{"SOURCE_DATE_EPOCH", "1"},
			{"CCACHE_DIR", "/tmp/gocpp-test-ccache"},
			{"SDKROOT", "/tmp/gocpp-test-sdk"},
			{"PATH", os.Getenv("PATH") + ":/tmp/gocpp-test-path"},
			{"GNUTARGET", "default"},
		} {
			s.build()
			if r, _ := s.build(); !isUpToDate(r.stderr) {
				t.Fatalf("not up to date before %s is set:\n%s", v.name, r.stderr)
			}
			old, had := os.LookupEnv(v.name)
			t.Setenv(v.name, v.value)
			r, n := s.build()
			if isUpToDate(r.stderr) || n.link != 1 {
				t.Errorf("%s set after a build: up to date %v, %+v", v.name, isUpToDate(r.stderr), n)
			}
			if had {
				t.Setenv(v.name, old)
			} else {
				os.Unsetenv(v.name)
			}
		}
	})
}

// A compiler replaced in place by a file of the same size, with its old modification time, is
// another compiler: the stamp, the object cache and the detection record all see it, and the
// program is the new compiler's.
func TestUpToDateCompilerReplacedInPlace(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		real, err := exec.LookPath(c.name)
		if err != nil {
			t.Skip(err)
		}
		sh, err := exec.LookPath("sh")
		if err != nil {
			t.Skip(err)
		}
		wrapper := filepath.Join(t.TempDir(), filepath.Base(c.name))
		past := time.Now().Add(-24 * time.Hour)
		write := func(n int) {
			t.Helper()
			text := "#!" + sh + "\nexec " + real + " -DXDS=\\\"" + fmt.Sprint(n) + "\\\" \"$@\"\n"
			if err := os.WriteFile(wrapper, []byte(text), 0o755); err != nil {
				t.Fatal(err)
			}
			os.Chtimes(wrapper, past, past)
		}
		write(1)
		t.Setenv("GOCPP_CXX", wrapper)
		s := newIncr(t, map[string]string{
			"gocpp.mod": "module example.com/w\n",
			"main.gp":   "package main\n\nfunc main() { println(\"w @STAMP@\") }\n",
			"mark.cpp":  "extern \"C\" __attribute__((used, retain)) const char xd_marker[] = \"XD=\" XDS \"=@STAMP@\";\n",
		})
		has := func(want string) bool {
			t.Helper()
			data, err := os.ReadFile(filepath.Join(s.dir, "prog"))
			if err != nil {
				t.Fatal(err)
			}
			return strings.Contains(string(data), want)
		}
		s.build()
		if !has("XD=1=" + s.stamp) {
			t.Fatal("the program has not the marker of the first compiler")
		}
		if r, n := s.build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Fatalf("not up to date:\n%s", r.stderr)
		}
		write(2) // the same size, the same time
		r, n := s.build()
		if isUpToDate(r.stderr) || !strings.Contains(notUpToDate(r.stderr), "compiler") || n.link != 1 || n.hit != 0 {
			t.Errorf("a compiler that was replaced: up to date %v, reason %q, %+v", isUpToDate(r.stderr), notUpToDate(r.stderr), n)
		}
		if !has("XD=2=" + s.stamp) {
			t.Errorf("the program is not the new compiler's: the object cache or the detection kept the old one")
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date after the build with the new compiler:\n%s", r.stderr)
		}
	})
}

// A program that lost its execute permission is not current, whether the build was just made
// (its record is racy) or the record was renewed after the racy window.
func TestUpToDateProgramMode(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		s := newIncr(t, stampProject(3))
		s.build()
		if err := os.Chmod("prog", 0o644); err != nil {
			t.Fatal(err)
		}
		if r, n := s.build(); isUpToDate(r.stderr) || n.link != 1 {
			t.Errorf("a program with another mode: up to date %v, %+v", isUpToDate(r.stderr), n)
		}
		if fi, _ := os.Stat("prog"); fi.Mode().Perm()&0o100 == 0 {
			t.Errorf("the program is still not executable: %v", fi.Mode())
		}
		// Past the racy window: the build renews the stat of the record (and keeps the hash).
		time.Sleep(2500 * time.Millisecond)
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Fatalf("not up to date:\n%s", r.stderr)
		}
		if err := os.Chmod("prog", 0o644); err != nil {
			t.Fatal(err)
		}
		if r, n := s.build(); isUpToDate(r.stderr) || n.link != 1 {
			t.Errorf("a program with another mode, after the racy window: up to date %v, %+v", isUpToDate(r.stderr), n)
		}
		if got := s.run(); !strings.HasSuffix(got, " "+s.stamp+"\n") {
			t.Errorf("the program prints %q", got)
		}
	})
}

// The sources of a library come from the patterns of its recipe, which are expanded by listing the
// module's folders: a new file that matches a pattern is a new source, so the build is not up to
// date; a build after that is (the listing is watched, and is recorded again).
func TestUpToDateRecipeGlob(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		ctor := func(name string) string {
			return "#include <stdio.h>\n__attribute__((constructor)) static void init_" + name + "(void) { puts(\"" + name + " ctor\"); }\n"
		}
		files := libProject("\tsrc src/*.c\n\tinclude include", "\tfmt.Println(clib.clib_scaled(2), \"@STAMP@\")",
			map[string]string{"include/clib.h": clibH,
				"src/a.c": "#include \"clib.h\"\nint clib_scaled(int x) { return x * 3; }\nint clib_square(int x) { return x * x; }\n"})
		s := newIncr(t, files)
		s.build()
		if got := s.run(); got != "6 "+s.stamp+"\n" {
			t.Fatalf("the program prints %q", got)
		}
		if r, n := s.build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Fatalf("not up to date:\n%s", r.stderr)
		}
		s.write(map[string]string{"clib/src/b.c": ctor("B")})
		r, n := s.build()
		if isUpToDate(r.stderr) || !strings.Contains(notUpToDate(r.stderr), "clib/src") || n.link != 1 {
			t.Errorf("a new file that matches the pattern: up to date %v, reason %q, %+v", isUpToDate(r.stderr), notUpToDate(r.stderr), n)
		}
		if got := s.run(); !strings.Contains(got, "B ctor") {
			t.Errorf("the program does not have the new source: %q", got)
		}
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("not up to date after the build with the new file:\n%s", r.stderr)
		}
		// A file that is not a kind of source (the listing is of the names that matter) is no change.
		s.write(map[string]string{"clib/src/c.txt": "x\n"})
		if r, _ := s.build(); !isUpToDate(r.stderr) {
			t.Errorf("a text file in the folder of the library made the build not up to date:\n%s", r.stderr)
		}
	})
}
