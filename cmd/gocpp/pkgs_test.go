package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// The tests of programs of several packages in gocpp build, run and emit (Phase 06 step 6.2):
// the layout of the generated files, what gocpp build compiles, the object cache (a unit that
// did not change is not compiled again), and the commands with patterns.

// pkgsProject returns the files of a module of three packages: main imports b, b imports a.
// The tag text is in a function body of a (it changes no header), and Limit is an exported
// constant of a (it is in a's header).
func pkgsProject(tag string, limit int) map[string]string {
	return map[string]string{
		"gocpp.mod": "module example.com/m\n",
		"a/a.gp": fmt.Sprintf(`package a

const Limit = %d

type Box struct{ N int }

func (b Box) Twice() int { return 2 * b.N }

func Tag() string { return %q }
`, limit, tag),
		"b/b.gp": `package b

import "example.com/m/a"

func Make(n int) a.Box { return a.Box{N: n} }

func Cap() int { return a.Limit * 2 }
`,
		"main.gp": `package main

import (
	"fmt"

	"example.com/m/a"
	"example.com/m/b"
)

func main() {
	fmt.Println(b.Make(4).Twice(), b.Cap(), a.Tag())
}
`,
	}
}

// unitCompiles returns the generated files that the -x output compiles, as gpkg/<path>/<file>.
func unitCompiles(stderr string) []string {
	rx := regexp.MustCompile(`(?m)^\S+ .* -c \S*/(gpkg/\S+\.cpp) -o `)
	var units []string
	for _, m := range rx.FindAllStringSubmatch(stderr, -1) {
		units = append(units, m[1])
	}
	return units
}

// unitHits returns the generated files that the -x output took from the object cache.
func unitHits(stderr string) []string {
	rx := regexp.MustCompile(`(?m)^# cache hit (gpkg/\S+\.cpp) [0-9a-f]{16}$`)
	var units []string
	for _, m := range rx.FindAllStringSubmatch(stderr, -1) {
		units = append(units, m[1])
	}
	return units
}

func TestPackagesBuild(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, pkgsProject("tag", 5))
		r := gocppRun("", false, "run", "-x", ".")
		if r.code != 0 || !strings.HasSuffix(r.stdout, "8 10 tag\n") {
			t.Fatalf("gocpp run -x . = %+v", r)
		}
		workDir(t, r.stderr)
		// -x shows one compile for each translation unit of each package: a.gp.cpp, b.gp.cpp,
		// main.gp.cpp and the pkg_init.cpp of the main package (the others have nothing in it).
		got := unitCompiles(r.stderr)
		want := []string{"gpkg/example.com/m/a/a.gp.cpp", "gpkg/example.com/m/b/b.gp.cpp", "gpkg/example.com/m/main.gp.cpp", "gpkg/example.com/m/pkg_init.cpp"}
		if fmt.Sprint(sorted(got)) != fmt.Sprint(want) {
			t.Errorf("-x compiles %v, want %v", sorted(got), want)
		}
	})
}

func sorted(s []string) []string {
	s = append([]string(nil), s...)
	for i := range s {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
	return s
}

func TestPackagesIncremental(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		cacheOn(t)
		// The text of the program is its own for each run of the test, so that the first build
		// misses whatever the shared cache holds.
		stamp := fmt.Sprintf("%s %d", c.name, time.Now().UnixNano())
		dir := inDir(t, pkgsProject("tag "+stamp, 5))
		ageTree(t, dir)
		build := func() result {
			r := gocppRun("", false, "build", "-x", "-o", "prog", ".")
			workDir(t, r.stderr)
			if r.code != 0 {
				t.Fatalf("gocpp build = %+v", r)
			}
			return r
		}
		all := []string{"gpkg/example.com/m/a/a.gp.cpp", "gpkg/example.com/m/b/b.gp.cpp", "gpkg/example.com/m/main.gp.cpp", "gpkg/example.com/m/pkg_init.cpp"}

		first := build()
		if got := sorted(unitCompiles(first.stderr)); fmt.Sprint(got) != fmt.Sprint(all) || len(unitHits(first.stderr)) != 0 {
			t.Errorf("the first build compiles %v and hits %v, want the four units and no hit:\n%s", got, unitHits(first.stderr), first.stderr)
		}
		// Nothing changed: every unit is a hit, and the program is linked again.
		second := build()
		if got := sorted(unitHits(second.stderr)); fmt.Sprint(got) != fmt.Sprint(all) || len(unitCompiles(second.stderr)) != 0 {
			t.Errorf("the second build compiles %v and hits %v, want four hits and no compile:\n%s", unitCompiles(second.stderr), got, second.stderr)
		}

		// A change in a function body of package a: only its unit is compiled again. The header
		// of a has no line of its function bodies, so the units of b and main hit.
		files := pkgsProject("tag "+stamp+" changed", 5)
		writeFiles(t, ".", map[string]string{"a/a.gp": files["a/a.gp"]})
		ageTree(t, dir)
		third := build()
		if got := unitCompiles(third.stderr); fmt.Sprint(got) != fmt.Sprint([]string{"gpkg/example.com/m/a/a.gp.cpp"}) {
			t.Errorf("a changed function body compiles %v, want only a.gp.cpp:\n%s", got, third.stderr)
		}
		if got := sorted(unitHits(third.stderr)); len(got) != 3 {
			t.Errorf("a changed function body: hits %v, want the other three units", got)
		}

		// A change of an exported constant of a: its header changes, and so every unit that
		// includes it (b and main through b) is compiled again.
		files = pkgsProject("tag "+stamp+" changed", 6)
		writeFiles(t, ".", map[string]string{"a/a.gp": files["a/a.gp"]})
		ageTree(t, dir)
		fourth := build()
		if got := sorted(unitCompiles(fourth.stderr)); fmt.Sprint(got) != fmt.Sprint(all) {
			t.Errorf("a changed constant compiles %v, want the dependents too:\n%s", got, fourth.stderr)
		}
		out, err := os.ReadFile("prog")
		_ = out
		if err != nil {
			t.Error(err)
		}
		if r := gocppRun("", false, "run", "."); !strings.HasSuffix(r.stdout, "8 12 tag "+stamp+" changed\n") {
			t.Errorf("the program after the change prints %+v", r)
		}
	})
}

func TestPackagesEmitDeterministic(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	// The same program twice, and in another folder (the .gp names in the #line directives
	// are the only difference), gives the same files.
	first := inDir(t, pkgsProject("tag", 5))
	r1 := gocppRun("", false, "emit", ".")
	r2 := gocppRun("", false, "emit", "./...")
	if r1.code != 0 || r1 != r2 {
		t.Fatalf("gocpp emit . = %+v, emit ./... = %+v", r1, r2)
	}
	second := inDir(t, pkgsProject("tag", 5))
	r3 := gocppRun("", false, "emit", ".")
	if got, want := strings.ReplaceAll(r3.stdout, second, "$DIR"), strings.ReplaceAll(r1.stdout, first, "$DIR"); got != want {
		t.Errorf("gocpp emit in another folder differs:\n%s\nwant\n%s", got, want)
	}
	// The files are in the order of the packages: a, b, main; a header, its units, pkg_init.cpp.
	var order []string
	for _, l := range strings.Split(r1.stdout, "\n") {
		if path, ok := strings.CutPrefix(l, "//// "); ok {
			order = append(order, path)
		}
	}
	want := []string{
		"gpkg/example.com/m/a/pkg.hpp", "gpkg/example.com/m/a/a.gp.cpp",
		"gpkg/example.com/m/b/pkg.hpp", "gpkg/example.com/m/b/b.gp.cpp",
		"gpkg/example.com/m/pkg.hpp", "gpkg/example.com/m/main.gp.cpp", "gpkg/example.com/m/pkg_init.cpp",
	}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Errorf("gocpp emit writes the files %v, want %v", order, want)
	}
}

func TestPackagesPatterns(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	files := pkgsProject("tag", 5)
	files["cmd/tool/main.gp"] = "package main\n\nimport \"example.com/m/a\"\n\nfunc main() { println(a.Limit) }\n"
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, files)
		// ./... has two main packages: an error, like go build with -o.
		if r := gocppRun("", false, "build", "./..."); r.code != 1 || !strings.Contains(r.stderr, "the patterns match 2 main packages") {
			t.Errorf("gocpp build ./... = %+v, want the error about two main packages", r)
		}
		// One package of the module is the program, named after its folder.
		if r := gocppRun("", false, "build", "./cmd/tool"); r != (result{}) {
			t.Errorf("gocpp build ./cmd/tool = %+v", r)
		}
		defer os.Remove("tool")
		if r := gocppRun("", false, "run", "./cmd/tool"); r.code != 0 || r.stderr != "5\n" {
			t.Errorf("gocpp run ./cmd/tool = %+v, want the constant 5 on stderr (println)", r)
		}
		// A package that is not a main package is not a program.
		if r := gocppRun("", false, "run", "./a"); r.code != 1 || !strings.Contains(r.stderr, "is not a main package") {
			t.Errorf("gocpp run ./a = %+v", r)
		}
		// check takes all of them.
		if r := gocppRun("", false, "check", "./..."); r != (result{}) {
			t.Errorf("gocpp check ./... = %+v", r)
		}
	})
}

func TestPackagesImportCycle(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	inDir(t, map[string]string{
		"gocpp.mod": "module example.com/c\n",
		"a/a.gp":    "package a\n\nimport \"example.com/c/b\"\n\nfunc A() int { return b.B() }\n",
		"b/b.gp":    "package b\n\nimport \"example.com/c/a\"\n\nfunc B() int { return a.A() }\n",
		"main.gp":   "package main\n\nimport \"example.com/c/a\"\n\nfunc main() { println(a.A()) }\n",
	})
	want := gocppRun("", false, "check", ".")
	if want.code != 1 || !strings.Contains(want.stderr, "import cycle not allowed") {
		t.Fatalf("gocpp check . = %+v, want the import cycle error", want)
	}
	for _, cmd := range []string{"build", "run", "emit"} {
		if got := gocppRun("", false, cmd, "."); got != want {
			t.Errorf("gocpp %s . = %+v, want what gocpp check prints", cmd, got)
		}
	}
}

func TestPackagesInternal(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	inDir(t, map[string]string{
		"gocpp.mod":           "module example.com/i\n",
		"lib/lib.gp":          "package lib\n\nimport \"example.com/i/lib/internal/s\"\n\nfunc L() int { return s.S() }\n",
		"lib/internal/s/s.gp": "package s\n\nfunc S() int { return 1 }\n",
		"main.gp":             "package main\n\nimport (\n\t\"example.com/i/lib\"\n\t\"example.com/i/lib/internal/s\"\n)\n\nfunc main() { println(lib.L(), s.S()) }\n",
	})
	r := gocppRun("", false, "build", ".")
	if r.code != 1 || !strings.Contains(r.stderr, "use of internal package example.com/i/lib/internal/s not allowed") {
		t.Errorf("gocpp build . = %+v, want the internal package error", r)
	}
	_ = filepath.Join
}

func TestPackagesCppError(t *testing.T) {
	// An error of the C++ compiler at a line of C++ code in a package other than main is the
	// user's, reported under that package at its .gp line, like in the main package.
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{
			"gocpp.mod": "module example.com/e\n",
			"lib/lib.gp": `package lib

import (
	"fmt"

	"<cstdio>"
)

func Hello() {
	fmt.Println("hello")
	cstdio.puts(5, 6)
}
`,
			"main.gp": "package main\n\nimport \"example.com/e/lib\"\n\nfunc main() { lib.Hello() }\n",
		})
		r := gocppRun("", false, "build", ".")
		defer os.Remove("e")
		if r.code != 1 || !strings.Contains(r.stderr, "# example.com/e/lib\nlib/lib.gp:11: C++: ") || !strings.Contains(r.stderr, "cstdio.puts(5, 6)") {
			t.Errorf("gocpp build = %+v, want the C++ error at lib/lib.gp:11 under the package", r)
		}
	})
}

func TestTrimGen(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	mk := func(name string, age time.Duration) string {
		p := filepath.Join(dir, name)
		if err := os.Mkdir(p, 0o755); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(p, now.Add(-age), now.Add(-age))
		return p
	}
	// More than maxGenFolders: the old ones go, oldest first, down to the limit; the young ones
	// and the folder in use stay, whatever their number.
	keep := mk("keep", 100*24*time.Hour)
	for i := 0; i < maxGenFolders; i++ {
		mk(fmt.Sprintf("old%03d", i), 10*24*time.Hour+time.Duration(i)*time.Minute)
	}
	young := mk("young", time.Hour)
	trimGen(dir, keep, now)
	entries, _ := os.ReadDir(dir)
	if len(entries) != maxGenFolders {
		t.Errorf("%d folders left, want %d", len(entries), maxGenFolders)
	}
	for _, p := range []string{keep, young} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was removed", p)
		}
	}
	// (old000 is the youngest of the olds, so the oldest are gone: old100's age is the highest.)
	if _, err := os.Stat(filepath.Join(dir, "old000")); err != nil {
		t.Error("the youngest of the old folders was removed before an older one")
	}
}

func TestPackagesLineDirective(t *testing.T) {
	// A //line directive changes the file name that go/token reports for the code after it. The
	// function still belongs to the translation unit of the file it is in.
	t.Setenv("GOCPP_GATE_PHASE", "06")
	anyCXX(t)
	inDir(t, map[string]string{"main.gp": "package main\n\n//line other.gp:100\nfunc helper() int { return 7 }\n\nfunc main() { println(helper()) }\n"})
	if r := gocppRun("", false, "run", "main.gp"); r.code != 0 || r.stderr != "7\n" {
		t.Errorf("gocpp run main.gp = %+v, want 7", r)
	}
}

func TestPackagesReadOnlyCache(t *testing.T) {
	// A cache folder that can't be written is no reason to fail: the generated files go to the work
	// folder, and the runtime is built there.
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("a read-only folder can be written by this user")
	}
	t.Setenv("GOCPP_GATE_PHASE", "06")
	anyCXX(t)
	inDir(t, pkgsProject("tag", 5))
	ro := t.TempDir()
	if err := os.Chmod(ro, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(ro, 0o755) })
	t.Setenv("XDG_CACHE_HOME", ro)
	t.Setenv("HOME", ro)
	if r := gocppRun("", false, "run", "."); r.code != 0 || !strings.HasSuffix(r.stdout, "8 10 tag\n") {
		t.Errorf("gocpp run . with a cache folder that can't be written = %+v", r)
	}
}

func TestPackagesLinkFlags(t *testing.T) {
	// The link flags of all the packages reach the link, those of the packages that import the others
	// first (a static library is read once, so the one a package needs comes after its users), and a
	// library that two packages name once.
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{
			"gocpp.mod": "module example.com/l\n",
			"a/a.gp":    "//gocpp:link m z1\npackage a\n\nimport \"example.com/l/b\"\n\nfunc A() int { return b.B() }\n",
			"b/b.gp":    "//gocpp:link pthread m\n//gocpp:ldflags -Wl,--as-needed\npackage b\n\nfunc B() int { return 1 }\n",
			"main.gp":   "//gocpp:link dl\npackage main\n\nimport \"example.com/l/a\"\n\nfunc main() { println(a.A()) }\n",
		})
		r := gocppRun("", false, "build", "-x", "-o", "prog", ".")
		defer os.Remove("prog")
		workDir(t, r.stderr)
		// (The link fails for z1, which is no library: the line is what matters.)
		link := regexp.MustCompile(`(?m)^.* \$WORK/\S*\.o .*$`).FindAllString(r.stderr, -1)
		if len(link) == 0 {
			t.Fatalf("no link line in -x:\n%s", r.stderr)
		}
		line := link[len(link)-1]
		if !strings.Contains(line, " -ldl -lm -lz1 -lpthread -Wl,--as-needed ") && !strings.Contains(line, " -ldl -lm -lz1 -lpthread -Wl,--as-needed") {
			t.Errorf("the link flags are not those of main, a and b in this order, without the second -lm:\n%s", line)
		}
	})
}

// TestPackagesConcurrentBuilds: two builds of the same program, the second after an edit, must not
// read each other's generated files. The first build is paused after it wrote its files and holds
// the lock of the program's folder; the second waits for it, and the first's program is the one of
// the sources it started with.
func TestPackagesConcurrentBuilds(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		stamp := fmt.Sprintf("%s %d", c.name, time.Now().UnixNano())
		dir := inDir(t, pkgsProject("v1 "+stamp, 5))
		var writes atomic.Int32
		paused, release, waiting := make(chan struct{}), make(chan struct{}), make(chan struct{})
		var waitOnce sync.Once
		testHookGenWritten = func() {
			if writes.Add(1) == 1 {
				close(paused)
				<-release
			}
		}
		testHookGenWait = func() { waitOnce.Do(func() { close(waiting) }) }
		t.Cleanup(func() { testHookGenWritten, testHookGenWait = nil, nil })

		results := make(chan result, 2)
		go func() { results <- gocppRun("", false, "build", "-o", filepath.Join(dir, "progA"), ".") }()
		<-paused // A has written the files of v1 and holds the lock
		files := pkgsProject("v2 "+stamp, 5)
		writeFiles(t, ".", map[string]string{"a/a.gp": files["a/a.gp"]})
		go func() { results <- gocppRun("", false, "build", "-o", filepath.Join(dir, "progB"), ".") }()
		select {
		case <-waiting: // B found the lock taken: it has written nothing
		case r := <-results:
			close(release)
			t.Fatalf("the second build ended (%+v) while the first holds the lock of the program's folder", r)
		}
		found := 0
		cache, _ := os.UserCacheDir()
		unitFiles, _ := filepath.Glob(filepath.Join(cache, "gocpp", "gen", "*", "gpkg", "example.com", "m", "a", "a.gp.cpp"))
		for _, f := range unitFiles {
			if data, _ := os.ReadFile(f); strings.Contains(string(data), "v1 "+stamp) {
				found++
			} else if strings.Contains(string(data), "v2 "+stamp) {
				t.Errorf("%s has the files of the second build while the first one compiles", f)
			}
		}
		if found != 1 {
			t.Errorf("the generated files of the first build are in %d folders, want 1", found)
		}
		close(release)
		for range 2 {
			if r := <-results; r.code != 0 {
				t.Fatalf("gocpp build = %+v", r)
			}
		}
		for prog, want := range map[string]string{"progA": "8 10 v1 " + stamp + "\n", "progB": "8 10 v2 " + stamp + "\n"} {
			out, err := exec.Command(filepath.Join(dir, prog)).Output()
			if err != nil || string(out) != want {
				t.Errorf("%s prints %q (%v), want %q", prog, out, err, want)
			}
		}
	})
}

func TestPackagesManyConcurrentBuilds(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		stamp := fmt.Sprintf("%s %d", c.name, time.Now().UnixNano())
		dir := inDir(t, pkgsProject("many "+stamp, 5))
		var wg sync.WaitGroup
		results := make([]result, 6)
		for i := range results {
			wg.Go(func() {
				results[i] = gocppRun("", false, "build", "-o", filepath.Join(dir, fmt.Sprint("prog", i)), ".")
			})
		}
		wg.Wait()
		for i, r := range results {
			out, err := exec.Command(filepath.Join(dir, fmt.Sprint("prog", i))).Output()
			if r.code != 0 || err != nil || string(out) != "8 10 many "+stamp+"\n" {
				t.Errorf("build %d: %+v, program prints %q (%v)", i, r, out, err)
			}
		}
	})
}

// A system without file locks: each build gets a folder of its own, which it removes when it succeeds.
func TestLockGenPrivateFolder(t *testing.T) {
	lockFile = func(string) (*genLock, bool, error) { return nil, false, errors.New("no locks here") }
	t.Cleanup(func() { lockFile = tryLockFile })
	cache := t.TempDir()
	gen := filepath.Join(cache, "gen", "abc")
	d := &driver{ctx: context.Background()}
	d1, rel1 := d.lockGen(gen, filepath.Join(cache, "work"))
	d2, rel2 := d.lockGen(gen, filepath.Join(cache, "work"))
	if d1 == gen || d2 == gen || d1 == d2 || !strings.HasPrefix(d1, gen+"-") {
		t.Fatalf("private folders %q and %q for %q: want two different ones next to it", d1, d2, gen)
	}
	for _, d := range []string{d1, d2} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	rel1(true)
	rel2(false) // a failed build keeps its files
	if _, err := os.Stat(d1); err == nil {
		t.Errorf("%s is kept after a successful build", d1)
	}
	if _, err := os.Stat(d2); err != nil {
		t.Errorf("%s is removed after a failed build", d2)
	}
}

// TestStdPackageObjectsAreShared: the units of a standard package are the same text in every
// program (their #line directives name $GOCPP/gen/..., not the folder of the program), so the
// object cache gives the second program that imports errors the objects of the first.
func TestStdPackageObjectsAreShared(t *testing.T) {
	t.Setenv("GOCPP_GATE_PHASE", "06")
	forEachCXX(t, func(t *testing.T, c testCXX) {
		cacheOn(t)
		stamp := fmt.Sprintf("%s %d", c.name, time.Now().UnixNano())
		program := func(body string) string {
			return "package main\n\nimport \"errors\"\n\nfunc main() {\n" + body + "}\n"
		}
		errorsUnits := []string{"gpkg/errors/errors.gp.cpp", "gpkg/errors/join.gp.cpp", "gpkg/errors/wrap.gp.cpp"}
		build := func(body string) result {
			dir := inDir(t, map[string]string{"main.gp": program(body)})
			ageTree(t, dir)
			r := gocppRun("", false, "build", "-x", "-o", "prog", "main.gp")
			workDir(t, r.stderr)
			if r.code != 0 {
				t.Fatalf("gocpp build = %+v", r)
			}
			return r
		}
		build("\tprintln(errors.New(" + fmt.Sprintf("%q", stamp+" one") + ").Error())\n")
		second := build("\tprintln(errors.Is(errors.New(" + fmt.Sprintf("%q", stamp+" two") + "), errors.ErrUnsupported))\n")
		hits, compiles := unitHits(second.stderr), unitCompiles(second.stderr)
		for _, u := range append(errorsUnits, "gpkg/errors/pkg_desc.cpp", "gpkg/errors/pkg_init.cpp") {
			if !slices.Contains(hits, u) || slices.Contains(compiles, u) {
				t.Errorf("the second program does not take %s from the cache: hits %v, compiles %v", u, hits, compiles)
			}
		}
		for _, u := range compiles {
			if strings.HasPrefix(u, "gpkg/errors/") {
				t.Errorf("the second program compiles %s", u)
			}
		}
	})
}
