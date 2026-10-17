package interop

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The tests of the probe cache on disk (diskcache.go), with the real
// compilers (see nsprobe_test.go). A "second process" is a new cache that
// has the same folder on disk: nothing is in its memory.

// diskCacheAt returns a cache with an empty memory and the disk folder dir.
func diskCacheAt(dir string) *cache {
	c := newCache()
	c.disk = &diskStore{dir: dir}
	return c
}

// diskSet is the arguments of one include set of the tests.
type diskSet struct {
	cxx, cc  build.CXX
	dir      string // the folder of the headers
	flags    []string
	includes []string
	cheader  string
}

func (s diskSet) clone() diskSet {
	s.flags = slices.Clone(s.flags)
	s.includes = slices.Clone(s.includes)
	return s
}

// newDiskSet returns the include set of the tests: <vector> and local
// headers.
func newDiskSet(t *testing.T, cxx, cc build.CXX) diskSet {
	t.Helper()
	dir := testDir(t)
	writeFiles(t, dir, map[string]string{"c.h": "int cfn(void);\n"})
	return diskSet{cxx: cxx, cc: cc, dir: dir, includes: []string{"<vector>", `"./physics.h"`}, cheader: `"./c.h"`}
}

// All the answers for an include set.
type diskAnswers struct {
	kinds, namespaces Result
	macros            Macros
	cheader           CHeader
}

var (
	diskKindPaths = []string{"std", "::physics_ns", "vecns", "stdd", "gp_x"}
	diskNSPaths   = []string{"std", "physics_ns", "vecns", "stdd"}
)

// ask asks the four probes of c, as the build does.
func (s diskSet) ask(t *testing.T, c *cache) diskAnswers { return s.askWith(t, c, true) }

func (s diskSet) askWith(t *testing.T, c *cache, withC bool) diskAnswers {
	t.Helper()
	ctx := context.Background()
	var a diskAnswers
	var err error
	if a.kinds, err = c.probe(ctx, s.cxx, s.dir, s.flags, s.includes, diskKindPaths); err != nil {
		t.Fatal(err)
	}
	if a.namespaces, err = c.probeNamespaces(ctx, s.cxx, s.dir, s.flags, s.includes, diskNSPaths); err != nil {
		t.Fatal(err)
	}
	if a.macros, err = c.probeMacros(ctx, s.cxx, s.dir, s.flags, s.includes); err != nil {
		t.Fatal(err)
	}
	if withC {
		if a.cheader, err = c.checkCHeader(ctx, s.cc, s.dir, s.flags, s.cheader); err != nil {
			t.Fatal(err)
		}
	}
	return a
}

// askKM asks the kind probe, the namespace probe and the macro probe.
func (s diskSet) askKM(t *testing.T, c *cache) diskAnswers {
	t.Helper()
	return s.askWith(t, c, false)
}

// runs returns the compiler runs of c: the kind probe's, the macro probe's
// and the C header check's.
func runsOf(c *cache) [3]int64 { return [3]int64{c.runs.Load(), c.macroRuns.Load(), c.cRuns.Load()} }

func (a diskAnswers) equal(b diskAnswers) bool {
	return reflect.DeepEqual(a, b)
}

// TestDiskCacheHit: a second process with unchanged inputs answers from the
// disk without the compiler, with the answers of a fresh probe.
func TestDiskCacheHit(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		disk := t.TempDir()

		cold := s.ask(t, newCache()) // no disk at all: a fresh probe
		first := diskCacheAt(disk)
		got := s.ask(t, first)
		if !got.equal(cold) {
			t.Errorf("the first process answers\n%+v\nwant\n%+v", got, cold)
		}
		if n := runsOf(first); n != [3]int64{1, 1, 1} {
			t.Errorf("the first process ran the compiler %v times, want once for each probe", n)
		}
		if n := first.diskStores.Load(); n < 3 {
			t.Errorf("the first process wrote %d entries, want at least 3 (kinds, macros, C header)", n)
		}

		second := diskCacheAt(disk)
		got = s.ask(t, second)
		if n := runsOf(second); n != [3]int64{} {
			t.Errorf("the second process ran the compiler %v times, want none", n)
		}
		if n := second.diskHits.Load(); n != 3 {
			t.Errorf("the second process had %d answers from the disk, want 3 (the namespace probe's come from memory)", n)
		}
		if !got.equal(cold) {
			t.Errorf("the disk's answers\n%+v\nwant those of a fresh probe\n%+v", got, cold)
		}
		if cold.macros.Len() == 0 || cold.kinds.Kinds["std"] != Namespace || !cold.cheader.C {
			t.Errorf("the test's answers are not what it needs: %+v", cold)
		}

		// A probe without paths (it checks the includes) is kept too.
		bare := func() *cache {
			c := diskCacheAt(disk)
			if res, err := c.probe(context.Background(), cxx, s.dir, []string{"-DBARE"}, s.includes, nil); err != nil || len(res.IncludeErrors) != 0 {
				t.Fatalf("a probe without paths: %+v, %v", res, err)
			}
			return c
		}
		if c := bare(); c.runs.Load() != 1 || c.diskStores.Load() != 1 {
			t.Errorf("a probe without paths: %d runs, %d entries written; want 1 and 1", c.runs.Load(), c.diskStores.Load())
		}
		if c := bare(); c.runs.Load() != 0 || c.diskHits.Load() != 1 {
			t.Errorf("a probe without paths, again: %d runs, %d disk hits; want 0 and 1", c.runs.Load(), c.diskHits.Load())
		}

		// A new process asks for other paths of the same include set: the
		// compiler runs once for them, and the entry keeps both.
		third := diskCacheAt(disk)
		res, err := third.probe(context.Background(), cxx, s.dir, s.flags, s.includes, []string{"std", "native_ns"})
		if err != nil || res.Kinds["native_ns"] != NotFound || res.Kinds["std"] != Namespace {
			t.Fatalf("other paths: %+v, %v", res, err)
		}
		if n := third.runs.Load(); n != 1 {
			t.Errorf("other paths: %d runs, want 1", n)
		}
		fourth := diskCacheAt(disk)
		a := s.ask(t, fourth)
		if _, err := fourth.probe(context.Background(), cxx, s.dir, s.flags, s.includes, []string{"native_ns", "vecns"}); err != nil {
			t.Fatal(err)
		}
		if n := runsOf(fourth); n != [3]int64{} || !a.equal(cold) {
			t.Errorf("after the merge: runs %v, answers equal %v; want no runs and the same answers", n, a.equal(cold))
		}
	})
}

// TestDiskCacheIncludeErrors: a result with include errors is not written.
func TestDiskCacheIncludeErrors(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		s.includes = []string{"<vector>", "<gocpp_no_such_header.h>"}
		disk := t.TempDir()
		for i := range 2 {
			c := diskCacheAt(disk)
			res, err := c.probe(context.Background(), cxx, s.dir, nil, s.includes, []string{"std"})
			if err != nil || len(res.IncludeErrors) != 1 {
				t.Fatalf("probe %d: %+v, %v", i, res, err)
			}
			if n := c.runs.Load(); n != 1 {
				t.Errorf("probe %d: %d runs, want 1: include errors are not read from the disk", i, n)
			}
			if n := c.diskStores.Load(); n != 0 {
				t.Errorf("probe %d wrote %d entries", i, n)
			}
		}
	})
}

// An invCase is a change after which TestDiskCacheInvalidation checks
// whether a second process runs the compiler.
type invCase struct {
	name string
	env  bool // changes the environment: the case can't run in parallel
	// change changes the include set or the world.
	change  func(t *testing.T, w *invWorld)
	wantRun bool // the kind probe and the macro probe run again
}

// An invWorld is what a case of TestDiskCacheInvalidation changes.
type invWorld struct {
	s      *diskSet  // the include set, as the second process asks for it
	real   build.CXX // the compiler that the set's wrapper runs
	sysDir string    // a folder of the flags, -isystem, with <sys.h>
}

// TestDiskCacheInvalidation: the second process runs the compiler again
// when something the answer depends on changed, and only then.
func TestDiskCacheInvalidation(t *testing.T) {
	// Not parallel: its cases that change the environment can't be.
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := diskCC(t, cxx)
		for _, c := range []invCase{
			{"nothing", false, func(t *testing.T, w *invWorld) {}, false},
			{"a header touched, same content", false, func(t *testing.T, w *invWorld) {
				p := filepath.Join(w.s.dir, "physics.h")
				data, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, data, 0o644); err != nil {
					t.Fatal(err)
				}
				old := time.Now().Add(-30 * time.Minute)
				os.Chtimes(p, old, old)
			}, false},
			{"a local header changed", false, func(t *testing.T, w *invWorld) {
				writeFiles(t, w.s.dir, map[string]string{"physics.h": "#pragma once\nnamespace physics_ns2 {}\n"})
			}, true},
			{"a header that it includes changed", false, func(t *testing.T, w *invWorld) {
				writeFiles(t, w.s.dir, map[string]string{"vec.h": "#pragma once\nnamespace vecns2 {}\n"})
			}, true},
			{"a header written with the same size and modification time", false, func(t *testing.T, w *invWorld) {
				// Only the change time (and the content) differ.
				p := filepath.Join(w.s.dir, "vec.h")
				fi, err := os.Stat(p)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte("#pragma once\nnamespace vecnt { struct V {}; }\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				os.Chtimes(p, fi.ModTime(), fi.ModTime())
			}, true},
			{"a flag changed", false, func(t *testing.T, w *invWorld) { w.s.flags = append(w.s.flags, "-DGOCPP_TEST_FLAG") }, true},
			{"another include", false, func(t *testing.T, w *invWorld) { w.s.includes = append(w.s.includes, "<string>") }, true},
			{"another folder", false, func(t *testing.T, w *invWorld) { w.s.dir = testDir(t) }, true},
			{"a system header changed", false, func(t *testing.T, w *invWorld) {
				writeFiles(t, w.sysDir, map[string]string{"sys.h": "#pragma once\nnamespace sys_ns2 {}\n"})
			}, true},
			{"the compiler's program file was replaced", false, func(t *testing.T, w *invWorld) {
				wrapper(t, filepath.Dir(w.s.cxx.Path), w.real, "# replaced\n")
			}, true},
			{"CPATH", true, func(t *testing.T, w *invWorld) { t.Setenv("CPATH", t.TempDir()) }, true},
			{"CPLUS_INCLUDE_PATH", true, func(t *testing.T, w *invWorld) { t.Setenv("CPLUS_INCLUDE_PATH", t.TempDir()) }, true},
			{"C_INCLUDE_PATH", true, func(t *testing.T, w *invWorld) { t.Setenv("C_INCLUDE_PATH", t.TempDir()) }, true},
			{"NIX_CFLAGS_COMPILE", true, func(t *testing.T, w *invWorld) {
				t.Setenv("NIX_CFLAGS_COMPILE", os.Getenv("NIX_CFLAGS_COMPILE")+" -DGOCPP_NIX_TEST")
			}, true},
			{"NIX_HARDENING_ENABLE", true, func(t *testing.T, w *invWorld) { t.Setenv("NIX_HARDENING_ENABLE", "fortify") }, true},
			{"an unrelated variable", true, func(t *testing.T, w *invWorld) { t.Setenv("GOCPP_UNRELATED_VARIABLE", "1") }, false},
		} {
			t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
				if !c.env {
					t.Parallel()
				}
				s := newDiskSet(t, cxx, cc)
				// The compiler is a wrapper script around the real one, so
				// that a test can replace it; and a system folder (-isystem)
				// has a header.
				s.cxx = wrapper(t, t.TempDir(), cxx, "")
				sysDir := t.TempDir()
				writeFiles(t, sysDir, map[string]string{"sys.h": "#pragma once\nnamespace sys_ns {}\n"})
				s.flags = []string{"-isystem", sysDir}
				s.includes = append(s.includes, "<sys.h>")
				disk := t.TempDir()
				want := s.askKM(t, diskCacheAt(disk))

				w := &invWorld{s: new(diskSet), real: cxx, sysDir: sysDir}
				*w.s = s.clone()
				c.change(t, w)
				second := diskCacheAt(disk)
				got := w.s.askKM(t, second)
				runs := runsOf(second)
				if (runs[0] > 0 || runs[1] > 0) != c.wantRun {
					t.Errorf("runs %v after %q, want a run: %v", runs, c.name, c.wantRun)
				}
				if !c.wantRun && !got.equal(want) {
					t.Errorf("answers\n%+v\nwant those of the first process\n%+v", got, want)
				}
				if c.wantRun && (c.name == "a local header changed" || c.name == "a system header changed") {
					// The answers are what a fresh probe says, also the
					// ones of the first run's world that changed.
					if fresh := w.s.askKM(t, newCache()); !got.equal(fresh) {
						t.Errorf("answers\n%+v\nwant those of a fresh probe\n%+v", got, fresh)
					}
				}
			})
		}
	})
}

// TestDiskCacheCHeaderInvalidation: the C header check's entries follow
// their files, the places of a header that wasn't found, and the flags.
func TestDiskCacheCHeaderInvalidation(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		cc := diskCC(t, cxx)
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{
			"c.h":     "#include \"inner.h\"\n",
			"inner.h": "int inner(void);\n",
			"gen.h":   "int gen(void);\n",
			"m.h":     "#include \"later.h\"\n",
		})
		ctx := context.Background()
		disk := t.TempDir()
		var c *cache
		check := func(why, include string, flags []string, wantRun bool, want CHeader) {
			t.Helper()
			c = diskCacheAt(disk)
			got, err := c.checkCHeader(ctx, cc, dir, flags, include)
			if err != nil || got != want {
				t.Errorf("%s: %+v, %v; want %+v", why, got, err, want)
			}
			if run := c.cRuns.Load() > 0; run != wantRun {
				t.Errorf("%s: ran the compiler: %v, want %v", why, run, wantRun)
			}
		}
		check("first", `"./c.h"`, nil, true, CHeader{C: true})
		check("second", `"./c.h"`, nil, false, CHeader{C: true})
		check("a flag", `"./c.h"`, []string{"-DX"}, true, CHeader{C: true})
		check("the flag, again", `"./c.h"`, []string{"-DX"}, false, CHeader{C: true})
		writeFiles(t, dir, map[string]string{"inner.h": "class Inner {};\n"})
		check("inner.h became C++", `"./c.h"`, nil, true, CHeader{})
		check("inner.h became C++, again", `"./c.h"`, nil, false, CHeader{})
		writeFiles(t, dir, map[string]string{"c.h": "#ifdef __cplusplus\n#endif\n"})
		check("c.h got a guard", `"./c.h"`, nil, true, CHeader{C: true, Guard: true})

		// A header that isn't found is not written to the disk: the places
		// that are watched are only some of the places where it can appear.
		check("later.h missing", `"./m.h"`, nil, true, CHeader{})
		check("later.h missing, again", `"./m.h"`, nil, true, CHeader{})
		writeFiles(t, dir, map[string]string{"later.h": "int later(void);\n"})
		check("later.h appeared", `"./m.h"`, nil, true, CHeader{C: true})
		check("later.h appeared, again", `"./m.h"`, nil, false, CHeader{C: true})
	})
}

// TestDiskCacheCHeaderMissingOnCPATH: a header that wasn't found and then
// appears in a folder of CPATH (which the check doesn't watch) is found by
// the next process.
func TestDiskCacheCHeaderMissingOnCPATH(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		cc := diskCC(t, cxx)
		dir, cp := t.TempDir(), t.TempDir()
		writeFiles(t, dir, map[string]string{"a.h": "#include <newlib.h>\n"})
		t.Setenv("CPATH", cp)
		disk := t.TempDir()
		for i := range 2 {
			c := diskCacheAt(disk)
			got, err := c.checkCHeader(context.Background(), cc, dir, nil, `"./a.h"`)
			if err != nil || got != (CHeader{}) || c.cRuns.Load() != 1 || c.diskStores.Load() != 0 {
				t.Fatalf("missing, process %d: %+v, %v, %d runs, %d entries written; want not C, 1 run, none written", i, got, err, c.cRuns.Load(), c.diskStores.Load())
			}
		}
		writeFiles(t, cp, map[string]string{"newlib.h": "int newlib(void);\n"})
		c := diskCacheAt(disk)
		got, err := c.checkCHeader(context.Background(), cc, dir, nil, `"./a.h"`)
		if err != nil || !got.C {
			t.Errorf("newlib.h appeared on CPATH: %+v, %v; want C", got, err)
		}
	})
}

// TestDiskCacheMaxAge: an entry older than maxDiskAge is a miss, and an
// entry that merges results keeps the age of the oldest.
func TestDiskCacheMaxAge(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		disk := t.TempDir()
		want := s.ask(t, diskCacheAt(disk))
		entries, _ := filepath.Glob(filepath.Join(disk, "*.json"))
		setAge := func(age time.Duration) {
			t.Helper()
			for _, e := range entries {
				data, err := os.ReadFile(e)
				if err != nil {
					t.Fatal(err)
				}
				var de diskEntry
				if err := json.Unmarshal([]byte(bodyOf(data)), &de); err != nil {
					t.Fatal(err)
				}
				de.Created = time.Now().Add(-age).UnixNano()
				body, _ := json.Marshal(de)
				if err := os.WriteFile(e, resum(string(body)), 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		created := func(prefix string) time.Time {
			t.Helper()
			files, _ := filepath.Glob(filepath.Join(disk, prefix+"-*.json"))
			data, err := os.ReadFile(files[0])
			if err != nil {
				t.Fatal(err)
			}
			var de diskEntry
			if err := json.Unmarshal([]byte(bodyOf(data)), &de); err != nil {
				t.Fatal(err)
			}
			return time.Unix(0, de.Created)
		}

		setAge(maxDiskAge - 5*time.Minute)
		c := diskCacheAt(disk)
		if got := s.ask(t, c); runsOf(c) != [3]int64{} || !got.equal(want) {
			t.Errorf("an entry just under the age limit: runs %v", runsOf(c))
		}
		// New paths merge into the entry; it keeps its age.
		c = diskCacheAt(disk)
		if _, err := c.probe(context.Background(), cxx, s.dir, s.flags, s.includes, []string{"std", "merge_ns"}); err != nil {
			t.Fatal(err)
		}
		if age := time.Since(created("k")); age < maxDiskAge-10*time.Minute {
			t.Errorf("a merged entry is %v old, want the age of the first result (about %v)", age, maxDiskAge-5*time.Minute)
		}

		for _, age := range []time.Duration{maxDiskAge + time.Minute, -time.Hour} {
			setAge(age)
			c = diskCacheAt(disk)
			if got := s.ask(t, c); runsOf(c) != [3]int64{1, 1, 1} || !got.equal(want) {
				t.Errorf("an entry made %v ago: runs %v, want every probe to run", age, runsOf(c))
			}
			if age := time.Since(created("m")); age > time.Minute {
				t.Errorf("the entry was not replaced: %v old", age)
			}
		}
	})
}

// TestDiskCacheOldMemorySet: with the cache on disk, a set in memory that
// is older than maxDiskAge is probed again, and the entry that it writes is
// fresh (a long-running process never writes an expired entry over a fresh
// one). Without the cache on disk, the set stays.
func TestDiskCacheOldMemorySet(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		ctx := context.Background()
		backdate := func(c *cache, age time.Duration) {
			t.Helper()
			if len(c.kinds.sets) != 1 {
				t.Fatalf("%d sets", len(c.kinds.sets))
			}
			for _, set := range c.kinds.sets {
				set.val.born = time.Now().Add(-age)
			}
		}
		probe := func(c *cache, paths ...string) {
			t.Helper()
			if res, err := c.probe(ctx, cxx, s.dir, nil, s.includes, paths); err != nil || res.Kinds["std"] != Namespace {
				t.Fatalf("probe: %+v, %v", res, err)
			}
		}

		disk := t.TempDir()
		c := diskCacheAt(disk)
		probe(c, "std")
		probe(c, "std")
		if c.runs.Load() != 1 {
			t.Fatalf("%d runs, want 1", c.runs.Load())
		}
		backdate(c, maxDiskAge-5*time.Minute)
		probe(c, "std")
		if c.runs.Load() != 1 {
			t.Errorf("a set just under the age limit was probed again")
		}
		// Older than the limit in memory, but the entry on disk (written by
		// another process, say) is fresh: the set is read from it.
		backdate(c, maxDiskAge+5*time.Minute)
		probe(c, "std")
		if c.runs.Load() != 1 || c.diskHits.Load() != 1 {
			t.Errorf("an old set, a fresh entry: %d runs, %d disk hits; want 1 and 1", c.runs.Load(), c.diskHits.Load())
		}
		for _, set := range c.kinds.sets {
			if age := time.Since(set.val.born); age > time.Minute {
				t.Errorf("the set is %v old after it was read again", age)
			}
		}
		// Old in memory and on disk: probed again.
		backdate(c, maxDiskAge+5*time.Minute)
		entries, _ := filepath.Glob(filepath.Join(disk, "k-*.json"))
		for _, e := range entries {
			data, err := os.ReadFile(e)
			if err != nil {
				t.Fatal(err)
			}
			var de diskEntry
			if err := json.Unmarshal([]byte(bodyOf(data)), &de); err != nil {
				t.Fatal(err)
			}
			de.Created = time.Now().Add(-maxDiskAge - 5*time.Minute).UnixNano()
			body, _ := json.Marshal(de)
			if err := os.WriteFile(e, resum(string(body)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		probe(c, "std")
		if c.runs.Load() != 2 {
			t.Errorf("%d runs, want 2: a set older than the limit is probed again", c.runs.Load())
		}
		// New paths add to the set; the entry that is written is fresh.
		probe(c, "std", "old_ns")
		files, _ := filepath.Glob(filepath.Join(disk, "k-*.json"))
		if len(files) != 1 {
			t.Fatalf("entries %v", files)
		}
		data, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		var de diskEntry
		if err := json.Unmarshal([]byte(bodyOf(data)), &de); err != nil {
			t.Fatal(err)
		}
		if age := time.Since(time.Unix(0, de.Created)); age > time.Minute {
			t.Errorf("the entry is %v old, want a fresh one", age)
		}
		fresh := diskCacheAt(disk)
		probe(fresh, "std", "old_ns")
		if fresh.runs.Load() != 0 || fresh.diskHits.Load() != 1 {
			t.Errorf("a new process: %d runs, %d disk hits; want 0 and 1", fresh.runs.Load(), fresh.diskHits.Load())
		}

		// No cache on disk: the memory set has no age limit.
		mem := newCache()
		probe(mem, "std")
		backdate(mem, 10*maxDiskAge)
		probe(mem, "std")
		if mem.runs.Load() != 1 {
			t.Errorf("without the disk cache: %d runs, want 1", mem.runs.Load())
		}
	})
}

// TestDiskCacheNative: results that depend on the host's CPU are not
// written.
func TestDiskCacheNative(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		s.flags = []string{"-march=native"}
		disk := t.TempDir()
		for range 2 {
			c := diskCacheAt(disk)
			s.askKM(t, c)
			if c.diskStores.Load() != 0 || runsOf(c) != [3]int64{1, 1, 0} {
				t.Errorf("-march=native: %d entries written, runs %v; want none written, runs 1 1 0", c.diskStores.Load(), runsOf(c))
			}
		}
	})
}

// TestDiskCacheFolderMode: the folder is private.
func TestDiskCacheFolderMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no modes")
	}
	folder := filepath.Join(t.TempDir(), "a", "probe")
	(&diskStore{dir: folder}).store(diskKey{path: filepath.Join(folder, "k-x.json"), full: "x", kind: "k"}, &diskEntry{})
	fi, err := os.Stat(folder)
	if err != nil || fi.Mode().Perm()&0o077 != 0 {
		t.Errorf("the folder: %v, %v; want no access for others", fi, err)
	}
}

// wrapper writes a compiler wrapper in dir that runs the compiler cxx, with
// the lines extra after the first one, and returns it as a compiler of the
// family and version of cxx.
func wrapper(t *testing.T, dir string, cxx build.CXX, extra string) build.CXX {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the wrapper is a shell script")
	}
	path := filepath.Join(dir, "wrap++")
	// The wrapper replaces the script, as a package manager replaces a
	// compiler: another file, not a write into the same one (writeScript
	// runs it once).
	os.Remove(path)
	body := "exec " + strings.Join(append([]string{cxx.Path}, cxx.Args...), " ") + ` "$@"` + "\n" + extra
	writeScript(t, path, "#!/bin/sh\n"+body)
	// A modification time of its own, a minute from the last one, so that
	// a replacement is different even when the clock stands still.
	mod := time.Now().Add(-time.Hour + time.Duration(len(extra))*time.Minute)
	os.Chtimes(path, mod, mod)
	return build.CXX{Path: path, Family: cxx.Family, Version: cxx.Version}
}

// TestDiskCacheCompilerIdentity: a compiler program that was replaced by
// another with the same path, family and version line has other entries,
// also when its size is the same.
func TestDiskCacheCompilerIdentity(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		wdir := t.TempDir()
		s.cxx = wrapper(t, wdir, cxx, "")
		disk := t.TempDir()
		first := diskCacheAt(disk)
		s.ask(t, first)

		// The same file again: hit.
		again := diskCacheAt(disk)
		s.ask(t, again)
		if n := runsOf(again); n != [3]int64{} {
			t.Fatalf("the same compiler: runs %v", n)
		}

		// Same size, other modification time.
		p := s.cxx.Path
		mod := time.Now().Add(-3 * time.Hour)
		if err := os.Chtimes(p, mod, mod); err != nil {
			t.Fatal(err)
		}
		c := diskCacheAt(disk)
		s.ask(t, c)
		if n := runsOf(c); n[0] != 1 || n[1] != 1 {
			t.Errorf("another modification time: runs %v, want one kind probe and one macro probe", n)
		}

		// Another path to the same program.
		link := filepath.Join(t.TempDir(), "link++")
		if err := os.Symlink(s.cxx.Path, link); err != nil {
			t.Skip(err)
		}
		linked := s.clone()
		linked.cxx.Path = link
		c = diskCacheAt(disk)
		linked.ask(t, c)
		if n := runsOf(c); n[0] != 1 {
			t.Errorf("another path to the compiler: runs %v", n)
		}
	})
}

// TestDiskCacheKey checks the parts of the key, with fake compilers and no
// compiler runs.
func TestDiskCacheKey(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	d := &diskStore{dir: t.TempDir()}
	base := fakeCompiler(t, build.GCC, "exit 0")
	base.Version = "fake 1.0"
	key := func(cxx build.CXX, dir string, flags, includes []string) string {
		t.Helper()
		k, ok := d.keyFor("k", cxx, dir, flags, includes)
		if !ok {
			t.Fatalf("no key for %+v", cxx)
		}
		return k.full
	}
	includes := []string{"<vector>"}
	want := key(base, "/p", []string{"-O2"}, includes)
	if got := key(base, "/p", []string{"-O2"}, includes); got != want {
		t.Errorf("the key is not stable")
	}
	same := func(why string, got string) {
		t.Helper()
		if got != want {
			t.Errorf("%s: the key changed", why)
		}
	}
	differs := func(why string, got string) {
		t.Helper()
		if got == want {
			t.Errorf("%s: the key is the same", why)
		}
	}
	same("a clone of the compiler", key(slices.Clone([]build.CXX{base})[0], "/p", []string{"-O2"}, includes))
	differs("dir", key(base, "/q", []string{"-O2"}, includes))
	differs("flags", key(base, "/p", []string{"-O3"}, includes))
	differs("no flags", key(base, "/p", nil, includes))
	differs("flags that move a word", key(base, "/p", []string{"-O", "2"}, includes))
	differs("includes", key(base, "/p", []string{"-O2"}, []string{"<string>"}))
	differs("an extern \"C\" include", key(base, "/p", []string{"-O2"}, []string{ExternC("<vector>")}))
	other := base
	other.Family = build.Clang
	differs("family", key(other, "/p", []string{"-O2"}, includes))
	other = base
	other.Version = "fake 1.1"
	differs("version", key(other, "/p", []string{"-O2"}, includes))
	other = base
	other.Args = []string{"-m32"}
	differs("arguments", key(other, "/p", []string{"-O2"}, includes))
	other = fakeCompiler(t, build.GCC, "exit 0")
	other.Version = "fake 1.0"
	differs("path", key(other, "/p", []string{"-O2"}, includes))
	// The program file's identity.
	other = base
	mod := time.Now().Add(-5 * time.Hour)
	os.Chtimes(base.Path, mod, mod)
	differs("modification time", key(other, "/p", []string{"-O2"}, includes))
	want = key(base, "/p", []string{"-O2"}, includes)
	if err := os.WriteFile(base.Path, []byte("#!/bin/sh\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.Chtimes(base.Path, mod, mod)
	differs("size", key(base, "/p", []string{"-O2"}, includes))
	want = key(base, "/p", []string{"-O2"}, includes)

	// The environment.
	for _, name := range []string{"CPATH", "CPLUS_INCLUDE_PATH", "C_INCLUDE_PATH", "NIX_CFLAGS_COMPILE", "NIX_CFLAGS_COMPILE_FOR_TARGET", "NIX_HARDENING_ENABLE", "SDKROOT", "NIX_ENFORCE_NO_NATIVE", "NIX_ENFORCE_PURITY", "MACOSX_DEPLOYMENT_TARGET"} {
		t.Run(name, func(t *testing.T) {
			t.Setenv(name, "/one")
			one := key(base, "/p", []string{"-O2"}, includes)
			differs(name, one)
			t.Setenv(name, "/two")
			if two := key(base, "/p", []string{"-O2"}, includes); two == one {
				t.Errorf("%s: the key is the same for another value", name)
			}
		})
		same("after "+name, key(base, "/p", []string{"-O2"}, includes))
	}
	t.Setenv("GOCPP_UNRELATED", "x")
	same("an unrelated variable", key(base, "/p", []string{"-O2"}, includes))
	// PATH only counts for a compiler with arguments (a launcher).
	t.Setenv("PATH", os.Getenv("PATH")+":/extra")
	same("PATH, no arguments", key(base, "/p", []string{"-O2"}, includes))
	withArgs := base
	withArgs.Args = []string{"g++"}
	before := key(withArgs, "/p", []string{"-O2"}, includes)
	t.Setenv("PATH", os.Getenv("PATH")+":/more")
	if key(withArgs, "/p", []string{"-O2"}, includes) == before {
		t.Errorf("PATH, with arguments: the key is the same")
	}
	// The probe: kinds, macros and C headers have their own entries.
	k1, _ := d.keyFor("k", base, "/p", nil, includes)
	k2, _ := d.keyFor("m", base, "/p", nil, includes)
	if k1.path == k2.path {
		t.Errorf("one entry for two probes")
	}
	// Results that depend on the host's CPU have no key (they stay in memory).
	for _, flags := range [][]string{{"-march=native"}, {"-O2", "-mtune=native"}, {"-mcpu=native"}, {"-Xclang", "-target-cpu", "x", "-march=native"}} {
		if _, ok := d.keyFor("k", base, "/p", flags, includes); ok {
			t.Errorf("a key for flags %q", flags)
		}
	}
	withNative := base
	withNative.Args = []string{"-march=native"}
	if _, ok := d.keyFor("k", withNative, "/p", nil, includes); ok {
		t.Errorf("a key for a compiler with -march=native in its arguments")
	}
	t.Run("native in the environment", func(t *testing.T) {
		t.Setenv("NIX_CFLAGS_COMPILE", "-O2 -march=native")
		if _, ok := d.keyFor("k", base, "/p", nil, includes); ok {
			t.Errorf("a key with -march=native in NIX_CFLAGS_COMPILE")
		}
	})
	if _, ok := d.keyFor("k", base, "/p", []string{"-march=x86-64"}, includes); !ok {
		t.Errorf("no key for -march=x86-64")
	}
	// Without a folder, the working folder is part of the key.
	a1, _ := d.keyFor("k", base, "", nil, includes)
	t.Chdir(t.TempDir())
	a2, _ := d.keyFor("k", base, "", nil, includes)
	if a1.full == a2.full {
		t.Errorf("no folder: the key is the same in another working folder")
	}
	// No key without a compiler file, or without a cache.
	if _, ok := d.keyFor("k", build.CXX{Path: "/nonexistent/g++", Family: build.GCC}, "/p", nil, includes); ok {
		t.Errorf("a key for a compiler that isn't there")
	}
	if _, ok := (*diskStore)(nil).keyFor("k", base, "/p", nil, includes); ok {
		t.Errorf("a key without a disk cache")
	}
	if _, ok := d.keyFor("k", base, "/p\xff", nil, includes); ok {
		t.Errorf("a key with text that isn't UTF-8")
	}
}

// TestDiskCacheCorrupt: an entry that can't be used is a miss, and the next
// probe replaces it.
func TestDiskCacheCorrupt(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		disk := t.TempDir()
		want := s.ask(t, diskCacheAt(disk))
		entries, err := filepath.Glob(filepath.Join(disk, "*.json"))
		if err != nil || len(entries) != 3 {
			t.Fatalf("entries %v, %v; want 3", entries, err)
		}
		original := map[string][]byte{}
		for _, e := range entries {
			data, err := os.ReadFile(e)
			if err != nil {
				t.Fatal(err)
			}
			original[e] = data
		}
		restore := func() {
			for e, data := range original {
				if err := os.WriteFile(e, data, 0o644); err != nil {
					t.Fatal(err)
				}
			}
		}
		for _, c := range []struct {
			name    string
			corrupt func(data []byte) []byte
		}{
			{"empty", func([]byte) []byte { return nil }},
			{"garbage", func([]byte) []byte { return []byte("not an entry at all\n") }},
			{"truncated", func(d []byte) []byte { return d[:len(d)/2] }},
			{"a flipped byte", func(d []byte) []byte {
				d = slices.Clone(d)
				d[len(d)-10] ^= 1
				return d
			}},
			{"no checksum", func(d []byte) []byte { _, body, _ := strings.Cut(string(d), "\n"); return []byte(body) }},
			{"another format", func(d []byte) []byte {
				return resum(strings.Replace(bodyOf(d), fmt.Sprintf(`"format":%d`, diskFormat), `"format":999`, 1))
			}},
			{"another probe's key", func(d []byte) []byte {
				return resum(strings.Replace(bodyOf(d), `"key":"`, `"key":"x`, 1))
			}},
			{"a kind that doesn't exist", func(d []byte) []byte {
				return resum(strings.Replace(bodyOf(d), `"std":1`, `"std":77`, 1))
			}},
			{"a kind of Unknown", func(d []byte) []byte {
				return resum(strings.Replace(bodyOf(d), `"std":1`, `"std":0`, 1))
			}},
			{"a path that isn't a name", func(d []byte) []byte {
				return resum(strings.Replace(bodyOf(d), `"std":1`, `"s td":1`, 1))
			}},
			{"a macro that isn't a name", func(d []byte) []byte {
				return resum(strings.Replace(bodyOf(d), `"objects":["`, `"objects":["bad name","`, 1))
			}},
			{"a bad hash", func(d []byte) []byte {
				return resum(strings.Replace(bodyOf(d), `"hash":"`, `"hash":"zz`, 1))
			}},
		} {
			t.Run(strings.ReplaceAll(c.name, " ", "_"), func(t *testing.T) {
				restore()
				for e, data := range original {
					if err := os.WriteFile(e, c.corrupt(data), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				second := diskCacheAt(disk)
				got := s.ask(t, second)
				if !got.equal(want) {
					t.Errorf("answers\n%+v\nwant\n%+v", got, want)
				}
				wantRuns := [3]int64{1, 1, 1}
				switch c.name {
				case "a kind that doesn't exist", "a kind of Unknown", "a path that isn't a name":
					wantRuns = [3]int64{1, 0, 0} // only the kind entry has kinds
				case "a macro that isn't a name":
					wantRuns = [3]int64{0, 1, 0}
				}
				if n := runsOf(second); n != wantRuns {
					t.Errorf("runs %v, want %v (a corrupt entry is a miss)", n, wantRuns)
				}
				// The probe replaced the entries.
				third := diskCacheAt(disk)
				got = s.ask(t, third)
				if n := runsOf(third); n != [3]int64{} || !got.equal(want) {
					t.Errorf("after the repair: runs %v, answers equal %v", n, got.equal(want))
				}
			})
		}

		t.Run("a folder instead of the entry", func(t *testing.T) {
			restore()
			for e := range original {
				if err := os.Remove(e); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(e, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			second := diskCacheAt(disk)
			if got := s.ask(t, second); !got.equal(want) {
				t.Errorf("answers\n%+v\nwant\n%+v", got, want)
			}
			if n := runsOf(second); n != [3]int64{1, 1, 1} {
				t.Errorf("runs %v", n)
			}
			for e := range original {
				os.RemoveAll(e)
			}
		})
	})
}

func bodyOf(d []byte) string {
	_, body, _ := strings.Cut(string(d), "\n")
	return body
}

// resum returns a file with the body and its checksum, as store writes it.
func resum(body string) []byte {
	return []byte(checksum([]byte(body)) + "\n" + body)
}

// TestDiskCacheUnwritable: a cache folder that can't be made or written
// is no error: the answers come from the compiler.
func TestDiskCacheUnwritable(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		want := s.ask(t, newCache())

		// A file where the folder should be.
		blocker := filepath.Join(t.TempDir(), "file")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		places := map[string]string{"under a file": filepath.Join(blocker, "probe")}
		// A folder that can't be written (not as root, which can).
		if runtime.GOOS != "windows" && os.Geteuid() != 0 {
			ro := filepath.Join(t.TempDir(), "ro")
			if err := os.Mkdir(ro, 0o500); err != nil {
				t.Fatal(err)
			}
			places["read-only"] = ro
		}
		for name, place := range places {
			t.Run(name, func(t *testing.T) {
				for range 2 {
					c := diskCacheAt(place)
					if got := s.ask(t, c); !got.equal(want) {
						t.Errorf("answers\n%+v\nwant\n%+v", got, want)
					}
					if n := runsOf(c); n != [3]int64{1, 1, 1} {
						t.Errorf("runs %v, want one of each", n)
					}
				}
			})
		}
	})
}

// TestDiskCacheConcurrent: many processes (here caches with their own
// memory) probing the same include sets at once all get the right answers,
// leave good entries, and no temporary files.
func TestDiskCacheConcurrent(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		want := s.ask(t, newCache())
		disk := t.TempDir()
		var wg sync.WaitGroup
		for i := range 16 {
			wg.Go(func() {
				c := diskCacheAt(disk)
				// Different processes ask for different paths too: their
				// entries merge or replace each other, never break.
				if i%2 == 1 {
					res, err := c.probe(context.Background(), cxx, s.dir, s.flags, s.includes, []string{"std", fmt.Sprint("other_", i)})
					if err != nil || res.Kinds["std"] != Namespace {
						t.Errorf("process %d: %+v, %v", i, res, err)
					}
				}
				if got := s.ask(t, c); !got.equal(want) {
					t.Errorf("process %d: answers\n%+v\nwant\n%+v", i, got, want)
				}
			})
		}
		wg.Wait()
		if tmps, _ := filepath.Glob(filepath.Join(disk, ".tmp-*")); len(tmps) > 0 {
			t.Errorf("temporary files left: %v", tmps)
		}
		if entries, _ := filepath.Glob(filepath.Join(disk, "*.json")); len(entries) != 3 {
			t.Errorf("%d entries, want 3 (one per probe)", len(entries))
		}
		c := diskCacheAt(disk)
		got := s.ask(t, c)
		if n := runsOf(c); n != [3]int64{} || !got.equal(want) {
			t.Errorf("after all: runs %v, answers equal %v", n, got.equal(want))
		}
	})
}

// TestDiskCacheRacy: a result that depends on a file that changed within
// racyWindow before the probe started is not written.
func TestDiskCacheRacy(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		vec := filepath.Join(s.dir, "vec.h")
		soon := time.Now().Add(time.Hour) // as if it changed after the compiler started
		if err := os.Chtimes(vec, soon, soon); err != nil {
			t.Fatal(err)
		}
		disk := t.TempDir()
		first := diskCacheAt(disk)
		s.ask(t, first)
		if n := first.diskStores.Load(); n != 1 {
			t.Errorf("wrote %d entries, want only the C header check's (it doesn't depend on vec.h)", n)
		}
		second := diskCacheAt(disk)
		s.ask(t, second)
		// Nothing is kept in memory either (the kind probe runs for the
		// namespace probe's call again), the C header check comes from disk.
		if n := runsOf(second); n != [3]int64{2, 1, 0} {
			t.Errorf("runs %v, want the kind and macro probes again, and the C header check from the disk", n)
		}

		// An old file: written.
		writeFiles(t, s.dir, map[string]string{"vec.h": testHeaders["vec.h"]})
		third := diskCacheAt(disk)
		s.ask(t, third)
		if n := third.diskStores.Load(); n != 2 {
			t.Errorf("after vec.h got old: wrote %d entries, want 2", n)
		}
		fourth := diskCacheAt(disk)
		s.ask(t, fourth)
		if n := runsOf(fourth); n != [3]int64{} {
			t.Errorf("runs %v, want none", n)
		}
	})
}

// TestDiskCacheLoadRacy: an entry whose file has changed in the stat only
// (same content) is used, and one whose content changed is not.
func TestDiskCacheLoadValidation(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		disk := t.TempDir()
		s.ask(t, diskCacheAt(disk))
		// A header that went away.
		if err := os.Remove(filepath.Join(s.dir, "vec.h")); err != nil {
			t.Fatal(err)
		}
		c := diskCacheAt(disk)
		res, err := c.probe(context.Background(), cxx, s.dir, s.flags, s.includes, []string{"std"})
		if err != nil || len(res.IncludeErrors) == 0 {
			t.Errorf("a header that went away: %+v, %v; want an include error", res, err)
		}
		if n := c.runs.Load(); n != 1 {
			t.Errorf("%d runs, want 1", n)
		}
		// A header that appeared in a place that hides another one: a
		// quoted include that was found in the system folders.
		writeFiles(t, s.dir, map[string]string{"vec.h": testHeaders["vec.h"]})
		s.includes = []string{`"stdio.h"`}
		a := diskCacheAt(disk)
		if _, err := a.probe(context.Background(), cxx, s.dir, nil, s.includes, []string{"shadow_ns"}); err != nil {
			t.Fatal(err)
		}
		writeFiles(t, s.dir, map[string]string{"stdio.h": "#pragma once\nnamespace shadow_ns {}\n"})
		b := diskCacheAt(disk)
		res, err = b.probe(context.Background(), cxx, s.dir, nil, s.includes, []string{"shadow_ns"})
		if err != nil || res.Kinds["shadow_ns"] != Namespace || b.runs.Load() != 1 {
			t.Errorf("a local stdio.h appeared: %+v, %v, %d runs", res, err, b.runs.Load())
		}
	})
}

// TestDiskCacheEviction: the folder holds at most maxEntries entries and
// maxBytes bytes; the entries used least recently go.
func TestDiskCacheEviction(t *testing.T) {
	folder := t.TempDir()
	d := &diskStore{dir: folder, maxEntries: 3}
	names := func() []string {
		t.Helper()
		var names []string
		des, err := os.ReadDir(folder)
		if err != nil {
			t.Fatal(err)
		}
		for _, de := range des {
			names = append(names, de.Name())
		}
		slices.Sort(names)
		return names
	}
	age := func(name string, ago time.Duration) {
		t.Helper()
		mod := time.Now().Add(-ago)
		if err := os.Chtimes(filepath.Join(folder, name), mod, mod); err != nil {
			t.Fatal(err)
		}
	}
	put := func(name string, size int, ago time.Duration) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(folder, name), make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
		age(name, ago)
	}
	put("k-a.json", 10, 5*time.Hour)
	put("k-b.json", 10, 4*time.Hour)
	put("m-c.json", 10, 3*time.Hour)
	put("c-d.json", 10, 2*time.Hour)
	put("k-new.json", 10, 0)
	put("notes.txt", 10, 9*time.Hour) // not an entry: left alone
	put(".tmp-old", 10, 3*time.Hour)  // a killed process's file
	put(".tmp-young", 10, time.Minute)
	d.evict(folder, "k-new.json")
	if got, want := names(), []string{".tmp-young", "c-d.json", "k-new.json", "m-c.json", "notes.txt"}; !slices.Equal(got, want) {
		t.Errorf("after evicting by count: %v, want %v", got, want)
	}

	// The oldest is the entry that was just written: it stays.
	age("k-new.json", 100*time.Hour)
	put("k-e.json", 10, 0)
	d.evict(folder, "k-new.json")
	if got, want := names(), []string{".tmp-young", "c-d.json", "k-e.json", "k-new.json", "notes.txt"}; !slices.Equal(got, want) {
		t.Errorf("the new entry is the oldest: %v, want %v", got, want)
	}

	// By size: three entries of 10 bytes, 25 allowed: the oldest goes.
	d = &diskStore{dir: folder, maxBytes: 25}
	age("k-new.json", 6*time.Hour)
	age("k-e.json", 5*time.Hour)
	age("c-d.json", 4*time.Hour)
	d.evict(folder, "c-d.json")
	if got, want := names(), []string{".tmp-young", "c-d.json", "k-e.json", "notes.txt"}; !slices.Equal(got, want) {
		t.Errorf("after evicting by size: %v, want %v", got, want)
	}
}

// TestDiskCacheEvictionByProbes: with a cap of two entries, a process that
// probes many include sets leaves two entries, and the answers stay right.
func TestDiskCacheEvictionByProbes(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		disk := t.TempDir()
		c := newCache()
		c.disk = &diskStore{dir: disk, maxEntries: 2}
		for i := range 5 {
			res, err := c.probe(context.Background(), cxx, s.dir, []string{fmt.Sprintf("-DGOCPP_SET_%d", i)}, s.includes, []string{"std"})
			if err != nil || res.Kinds["std"] != Namespace {
				t.Fatalf("set %d: %+v, %v", i, res, err)
			}
			if entries, _ := filepath.Glob(filepath.Join(disk, "*.json")); len(entries) > 2 {
				t.Errorf("after set %d: %d entries, want at most 2", i, len(entries))
			}
		}
		if entries, _ := filepath.Glob(filepath.Join(disk, "*.json")); len(entries) != 2 {
			t.Errorf("%d entries, want 2", len(entries))
		}
	})
}

// TestDiskCacheEnv: GOCPP_NOPROBECACHE turns the disk cache off, a test
// binary has none unless GOCPP_PROBE_CACHE names a folder, and a relative
// folder is not used.
func TestDiskCacheEnv(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		folder := filepath.Join(t.TempDir(), "probe")
		count := func() int {
			entries, _ := filepath.Glob(filepath.Join(folder, "*.json"))
			return len(entries)
		}
		env := func(off, dir string) *cache {
			t.Setenv(EnvNoProbeCache, off)
			t.Setenv(EnvProbeCache, dir)
			c := newCache()
			c.disk = &diskStore{} // from the environment, like the default
			return c
		}

		c := env("", "")
		s.ask(t, c)
		if c.diskStores.Load() != 0 || probeDiskDir() != "" {
			t.Errorf("a test binary uses the user's cache: %q", probeDiskDir())
		}
		c = env("", "relative/dir")
		s.ask(t, c)
		if c.diskStores.Load() != 0 {
			t.Errorf("a relative folder is used")
		}
		c = env("1", folder)
		s.ask(t, c)
		if c.diskStores.Load() != 0 || count() != 0 {
			t.Errorf("GOCPP_NOPROBECACHE=1 doesn't turn the cache off")
		}
		c = env("", folder)
		s.ask(t, c)
		if count() != 3 {
			t.Errorf("%d entries in %s, want 3", count(), folder)
		}
		c = env("0", folder)
		s.ask(t, c)
		if n := runsOf(c); n != [3]int64{} {
			t.Errorf("GOCPP_NOPROBECACHE=0: runs %v, want none", n)
		}
		c = env("1", folder)
		s.ask(t, c)
		if n := runsOf(c); n != [3]int64{1, 1, 1} {
			t.Errorf("GOCPP_NOPROBECACHE=1 with entries: runs %v, want every probe to run", n)
		}
	})
}

// answersText is a text of the answers for the comparison between processes.
func (a diskAnswers) text() string {
	sum := sha256.Sum256([]byte(fmt.Sprint(a.kinds, a.namespaces, a.macros.Names(), a.cheader)))
	return fmt.Sprintf("%x", sum)
}

// TestDiskCacheProcesses: processes that start at the same time with one
// cache folder all get the right answers, and the next one gets them from
// the disk. The test runs itself as the processes (TestDiskCacheChild).
func TestDiskCacheProcesses(t *testing.T) {
	t.Parallel()
	forEachCompiler(t, func(t *testing.T, cxx build.CXX) {
		t.Parallel()
		s := newDiskSet(t, cxx, diskCC(t, cxx))
		want := s.askKM(t, newCache()).text()
		disk := t.TempDir()
		var wg sync.WaitGroup
		for i := range 8 {
			wg.Go(func() {
				cmd := exec.Command(os.Args[0], "-test.run=^TestDiskCacheChild$", "-test.count=1")
				cmd.Env = append(os.Environ(), "GOCPP_TEST_DISKCACHE_CHILD="+disk, "GOCPP_TEST_DISKCACHE_CXX="+cxx.Path, "GOCPP_TEST_DISKCACHE_DIR="+s.dir)
				out, err := cmd.CombinedOutput()
				if err != nil {
					t.Errorf("process %d: %v\n%s", i, err, out)
					return
				}
				if !strings.Contains(string(out), "ANSWERS "+want+"\n") {
					t.Errorf("process %d: the answers differ from a fresh probe's:\n%s", i, out)
				}
			})
		}
		wg.Wait()
		if tmps, _ := filepath.Glob(filepath.Join(disk, ".tmp-*")); len(tmps) > 0 {
			t.Errorf("temporary files left: %v", tmps)
		}
		c := diskCacheAt(disk)
		s.askKM(t, c)
		if n := runsOf(c); n != [3]int64{} {
			t.Errorf("the next process ran the compiler: %v", n)
		}
	})
}

// TestDiskCacheChild is one of the processes of TestDiskCacheProcesses; it
// does nothing otherwise.
func TestDiskCacheChild(t *testing.T) {
	disk := os.Getenv("GOCPP_TEST_DISKCACHE_CHILD")
	if disk == "" {
		t.Skip("only as a process of TestDiskCacheProcesses")
	}
	cxx, err := build.LookCXX(context.Background(), os.Getenv("GOCPP_TEST_DISKCACHE_CXX"))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CC", "")
	cc, err := build.FindCC(context.Background(), cxx)
	if err != nil {
		t.Fatal(err)
	}
	s := diskSet{cxx: cxx, cc: cc, dir: os.Getenv("GOCPP_TEST_DISKCACHE_DIR"), includes: []string{"<vector>", `"./physics.h"`}, cheader: `"./c.h"`}
	fmt.Printf("ANSWERS %s\n", s.askKM(t, diskCacheAt(disk)).text())
}

// diskCC returns the C compiler of cxx, as testCC does, but without a
// Setenv (the tests here run in parallel); GOCPP_CC is the environment's.
func diskCC(t *testing.T, cxx build.CXX) build.CXX {
	t.Helper()
	cc, err := build.FindCC(context.Background(), cxx)
	if err != nil {
		t.Fatal(err)
	}
	return cc
}
