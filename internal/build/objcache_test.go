package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
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
)

// The object cache's tests compile small C++ files with the real compilers (the dependency
// file is the compiler's), through compileCached. The caches are always folders of the test:
// never the user's.

// forEachCompiler runs f for g++ and clang++, skipping a compiler that isn't installed.
func forEachCompiler(t *testing.T, f func(t *testing.T, cxx CXX)) {
	t.Helper()
	found := false
	for _, name := range []string{"g++", "clang++"} {
		cxx, err := LookCXX(context.Background(), name)
		if err != nil {
			continue
		}
		found = true
		t.Run(name, func(t *testing.T) {
			t.Setenv("GOCPP_NOOBJCACHE", "")
			t.Setenv("GOCPP_OBJCACHE_SIZE", "")
			f(t, cxx)
		})
	}
	if !found {
		t.Skip("no C++ compiler")
	}
}

// An objRun is the result of one objCompile.
type objRun struct {
	out   []byte // what the compiler printed
	err   error
	trace string
	obj   []byte // the object file, when there is one
	ran   bool   // the compiler ran
	work  string
}

func (r objRun) hit() bool { return strings.Contains(r.trace, "# cache hit ") }

// objCompile compiles the C++ text tu with flags (after -std=c++20 -g -O0) in a new work
// folder with the cache folder cache.
func objCompile(t *testing.T, cxx CXX, cache, tu string, flags ...string) objRun {
	t.Helper()
	work := t.TempDir()
	src := filepath.Join(work, "p.cpp")
	if err := os.WriteFile(src, []byte(tu), 0o644); err != nil {
		t.Fatal(err)
	}
	return objCompileFile(t, cxx, cache, work, src, flags...)
}

func objCompileFile(t *testing.T, cxx CXX, cache, work, src string, flags ...string) objRun {
	t.Helper()
	return objCompileAfter(t, cxx, cache, work, src, nil, flags...)
}

// objCompileAfter is objCompileFile, and after calls after (if not nil) once the compiler has
// finished, before the cache looks at the result.
func objCompileAfter(t *testing.T, cxx CXX, cache, work, src string, after func(), flags ...string) objRun {
	t.Helper()
	var tr strings.Builder
	b := &Builder{CXX: cxx, Work: work, Cache: cache, Trace: &tr}
	obj := filepath.Join(work, "p.o")
	args := slices.Concat([]string{"-std=c++20", "-g", "-O0"}, flags, []string{"-c", src, "-o", obj})
	var r objRun
	r.work = work
	r.out, r.err = b.compileCached(context.Background(), cxx, src, obj, args, func(a []string) ([]byte, error) {
		r.ran = true
		out, err := b.runOutput(context.Background(), cxx, "the test", false, a)
		if after != nil {
			after()
		}
		return out, err
	}, nil)
	b.stores.Wait()
	r.trace = tr.String()
	if r.err == nil {
		var err error
		if r.obj, err = os.ReadFile(obj); err != nil {
			t.Fatalf("no object file: %v", err)
		}
	}
	return r
}

// writeOld writes a file and sets its time to an hour ago: a header that changed less than two
// seconds ago is never cached against (the racy window).
func writeOld(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

// objFiles returns the names in the cache's obj folder, without the temporary ones and the
// sets of files (.deps).
func objFiles(t *testing.T, cache string) (objs, mans []string) {
	t.Helper()
	entries, _ := os.ReadDir(filepath.Join(cache, "obj"))
	for _, e := range entries {
		switch {
		case strings.HasPrefix(e.Name(), ".tmp-"):
			t.Errorf("a temporary file %s is left", e.Name())
		case strings.HasSuffix(e.Name(), ".o"):
			objs = append(objs, e.Name())
		case strings.HasSuffix(e.Name(), ".man"):
			mans = append(mans, e.Name())
		}
	}
	return
}

const objTU = `#include "h.h"
#include <vector>
int main() { std::vector<int> v{1, 2}; return value() + int(v.size()) - 3; }
`

func TestObjCacheHit(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, inc := t.TempDir(), t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "inline int value() { return 1; }\n")
		first := objCompile(t, cxx, cache, objTU, "-I", inc)
		if first.err != nil || !first.ran || first.hit() || len(first.out) > 0 {
			t.Fatalf("first compile: %+v", first)
		}
		if !strings.Contains(first.trace, "-MD -MF ") || !strings.Contains(first.trace, "-ffile-prefix-map=") {
			t.Errorf("the compile that stores has no -MD or prefix map: %s", first.trace)
		}
		objs, mans := objFiles(t, cache)
		if len(objs) != 1 || len(mans) != 1 {
			t.Fatalf("the cache holds %v and %v, want one object and one manifest", objs, mans)
		}
		// The same input in another work folder is a hit with the same object, and no compiler runs.
		second := objCompile(t, cxx, cache, objTU, "-I", inc)
		if second.err != nil || second.ran || !second.hit() {
			t.Fatalf("second compile: %+v", second)
		}
		if strings.Contains(second.trace, "-c ") {
			t.Errorf("a hit shows a compile: %s", second.trace)
		}
		if !bytes.Equal(first.obj, second.obj) {
			t.Error("the cached object differs from the compiled one")
		}
		// The work folder is in neither the object nor its debug info (g++ compresses the
		// debug info, so the text can't tell): the same program compiled in another work
		// folder and another cache gives the same object, so a cached object is right for
		// any folder.
		if third := objCompile(t, cxx, t.TempDir(), objTU, "-I", inc); !third.ran || !bytes.Equal(third.obj, first.obj) {
			t.Error("the object depends on the work folder")
		}
		// Another program with the same headers has a manifest of its own and shares the set
		// of files; a comment doesn't change the object, which is stored once (it is named by
		// its content); code does.
		objCompile(t, cxx, cache, objTU+"// another program\n", "-I", inc)
		objs, mans = objFiles(t, cache)
		sets, _ := filepath.Glob(filepath.Join(cache, "obj", "*.deps"))
		if len(objs) != 1 || len(mans) != 2 || len(sets) != 1 {
			t.Errorf("the cache holds %v, %v and %v, want 1 object, 2 manifests and 1 set", objs, mans, sets)
		}
		objCompile(t, cxx, cache, objTU+"int another_program;\n", "-I", inc)
		if objs, _ = objFiles(t, cache); len(objs) != 2 {
			t.Errorf("the cache holds %v, want 2 objects", objs)
		}
		// A header that is only touched is still the same.
		now := time.Now().Add(-time.Minute)
		if err := os.Chtimes(filepath.Join(inc, "h.h"), now, now); err != nil {
			t.Fatal(err)
		}
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); r.ran || !r.hit() {
			t.Errorf("after a touch: %+v", r)
		}
	})
}

func TestObjCacheTUWithWorkPath(t *testing.T) {
	// The driver's `#line N "<work>/x.cpp"` makes the text differ in every build: the key sees
	// it as $WORK.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		compile := func() objRun {
			work := t.TempDir()
			src := filepath.Join(work, "p.cpp")
			tu := fmt.Sprintf("int f() {\n#line 40 \"/some/where/x.gp\"\n    return 1;\n#line 5 %q\n}\nint main() { return f() - 1; }\n", src)
			if err := os.WriteFile(src, []byte(tu), 0o644); err != nil {
				t.Fatal(err)
			}
			return objCompileFile(t, cxx, cache, work, src)
		}
		first, second := compile(), compile()
		if other := objCompileFile(t, cxx, t.TempDir(), first.work, filepath.Join(first.work, "p.cpp")); !other.ran || !bytes.Equal(other.obj, first.obj) {
			t.Error("the object depends on the work folder")
		}
		if first.err != nil || !first.ran || second.err != nil || second.ran || !second.hit() {
			t.Fatalf("first %+v, second %+v", first, second)
		}
		if third := compile(); !third.hit() {
			t.Errorf("a third build is not a hit: %+v", third)
		}
	})
}

func TestObjCacheHeaderChange(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, inc := t.TempDir(), t.TempDir()
		h := filepath.Join(inc, "h.h")
		v1, v2 := "inline int value() { return 1; }\n", "inline int value() { return 2; }\n" // the same size
		writeOld(t, h, v1)
		a := objCompile(t, cxx, cache, objTU, "-I", inc)
		writeOld(t, h, v2)
		b := objCompile(t, cxx, cache, objTU, "-I", inc)
		if !b.ran || b.hit() || bytes.Equal(a.obj, b.obj) {
			t.Fatalf("after the header changed: %+v", b)
		}
		// Both results are in the manifest: going back to the first one is a hit.
		writeOld(t, h, v1)
		if c := objCompile(t, cxx, cache, objTU, "-I", inc); c.ran || !c.hit() || !bytes.Equal(c.obj, a.obj) {
			t.Errorf("after going back: ran=%v hit=%v", c.ran, c.hit())
		}
		if objs, mans := objFiles(t, cache); len(objs) != 2 || len(mans) != 1 {
			t.Errorf("the cache holds %v and %v, want two objects and one manifest", objs, mans)
		}
		// A header read through another header is a dependency too.
		deep := filepath.Join(inc, "deep.h")
		writeOld(t, deep, "#define DEEP 1\n")
		writeOld(t, h, "#include \"deep.h\"\ninline int value() { return DEEP; }\n")
		d := objCompile(t, cxx, cache, objTU, "-I", inc)
		writeOld(t, deep, "#define DEEP 2 // changed\n")
		if e := objCompile(t, cxx, cache, objTU, "-I", inc); !e.ran || e.hit() || bytes.Equal(d.obj, e.obj) {
			t.Errorf("after the nested header changed: ran=%v hit=%v", e.ran, e.hit())
		}
		// A header that changed within the racy window is not stored.
		cache2 := t.TempDir()
		if err := os.WriteFile(h, []byte(v1), 0o644); err != nil { // now
			t.Fatal(err)
		}
		objCompile(t, cxx, cache2, objTU, "-I", inc)
		if objs, _ := objFiles(t, cache2); len(objs) != 0 {
			t.Errorf("a header that changed a moment ago was cached: %v", objs)
		}
	})
}

func TestObjCacheKey(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, inc := t.TempDir(), t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "#ifndef V\n#define V 1\n#endif\ninline int value() { return V; }\n")
		objCompile(t, cxx, cache, objTU, "-I", inc)
		miss := func(what string, r objRun) {
			t.Helper()
			if r.err != nil || !r.ran || r.hit() {
				t.Errorf("%s: ran=%v hit=%v err=%v", what, r.ran, r.hit(), r.err)
			}
		}
		miss("a flag", objCompile(t, cxx, cache, objTU, "-I", inc, "-DV=2"))
		miss("another source text", objCompile(t, cxx, cache, objTU+"// more\n", "-I", inc))
		inc2 := t.TempDir()
		writeOld(t, filepath.Join(inc2, "h.h"), "#define V 1\ninline int value() { return V; }\n")
		miss("another include folder", objCompile(t, cxx, cache, objTU, "-I", inc2))
		// The compiler: the same program with another version line, or with other arguments.
		other := cxx
		other.Version += " (other)"
		if r := objCompile(t, other, cache, objTU, "-I", inc); !r.ran || r.hit() {
			t.Error("another compiler version is a hit")
		}
		other = cxx
		other.stamp.size++
		if r := objCompile(t, other, cache, objTU, "-I", inc); !r.ran || r.hit() {
			t.Error("another compiler program file is a hit")
		}
		// The environment of the compiler wrapper (a Nix compiler wrapper reads NIX_CFLAGS_COMPILE).
		t.Setenv("NIX_CFLAGS_COMPILE", os.Getenv("NIX_CFLAGS_COMPILE")+" -DGOCPP_TEST_WRAPPER=1")
		miss("the wrapper's environment", objCompile(t, cxx, cache, objTU, "-I", inc))
		t.Setenv("CPLUS_INCLUDE_PATH", inc2)
		miss("an include path in the environment", objCompile(t, cxx, cache, objTU, "-I", inc))
		// And a variable that doesn't matter doesn't.
		t.Setenv("GOCPP_TEST_UNRELATED", "1")
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); !r.hit() {
			t.Error("an unrelated variable is a miss")
		}
	})
}

func TestObjCacheNothingPrinted(t *testing.T) {
	// A compile that printed something is not cached: its warnings must come every time.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		const warns = "int main() { int unused; return 0; }\n"
		for i := range 2 {
			r := objCompile(t, cxx, cache, warns, "-Wall", "-Wextra")
			if r.err != nil || !r.ran || r.hit() || !strings.Contains(string(r.out), "unused") {
				t.Fatalf("compile %d: ran=%v hit=%v err=%v out=%q", i, r.ran, r.hit(), r.err, r.out)
			}
		}
		if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
			t.Errorf("a compile with a warning left %v %v", objs, mans)
		}
	})
}

func TestObjCacheFailureNotCached(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		for i := range 2 {
			r := objCompile(t, cxx, cache, "#error broken\nint main() {}\n")
			var ce *CompileError
			if !errors.As(r.err, &ce) || !r.ran || r.hit() {
				t.Fatalf("compile %d: ran=%v hit=%v err=%v", i, r.ran, r.hit(), r.err)
			}
		}
		if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
			t.Errorf("a failed compile left %v %v", objs, mans)
		}
	})
}

func TestObjCacheNotCacheable(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		for _, c := range []struct {
			name, tu string
			flags    []string
		}{
			{"__DATE__", "const char* d = __DATE__;\nint main() {}\n", nil},
			{"__TIME__", "const char* d = __TIME__;\nint main() {}\n", nil},
			{"__TIMESTAMP__", "const char* d = __TIMESTAMP__;\nint main() {}\n", nil},
			{"-save-temps", "int main() {}\n", []string{"-save-temps=obj"}},
			{"-MMD", "int main() {}\n", []string{"-MMD"}},
		} {
			cache := t.TempDir()
			for range 2 {
				r := objCompile(t, cxx, cache, c.tu, c.flags...)
				if r.err != nil || !r.ran || r.hit() {
					t.Fatalf("%s: ran=%v hit=%v err=%v out=%s", c.name, r.ran, r.hit(), r.err, r.out)
				}
			}
			if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
				t.Errorf("%s: left %v %v", c.name, objs, mans)
			}
		}
		// A header with __DATE__ too.
		cache, inc := t.TempDir(), t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "inline const char* built() { return __DATE__; }\ninline int value() { return 1; }\n")
		for range 2 {
			if r := objCompile(t, cxx, cache, objTU, "-I", inc); !r.ran || r.hit() {
				t.Fatalf("a header with __DATE__: ran=%v hit=%v", r.ran, r.hit())
			}
		}
	})
}

func TestObjCacheShadowingHeader(t *testing.T) {
	// A new file earlier on the include path would win now: the entry notices.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, first, second := t.TempDir(), t.TempDir(), t.TempDir()
		writeOld(t, filepath.Join(second, "h.h"), "inline int value() { return 1; }\n")
		args := []string{"-I", first, "-I", second}
		a := objCompile(t, cxx, cache, objTU, args...)
		if r := objCompile(t, cxx, cache, objTU, args...); r.ran || !r.hit() {
			t.Fatalf("no hit before the new file: %+v", r)
		}
		writeOld(t, filepath.Join(first, "h.h"), "inline int value() { return 7; }\n")
		b := objCompile(t, cxx, cache, objTU, args...)
		if !b.ran || b.hit() || bytes.Equal(a.obj, b.obj) {
			t.Errorf("a header that hides the one found is ignored: ran=%v hit=%v", b.ran, b.hit())
		}
	})
}

func TestObjCacheCorrupt(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		first := objCompile(t, cxx, cache, objTU, "-I", t.TempDir()) // h.h is missing: stays uncached
		if first.err == nil {
			t.Fatal("the compile without h.h works")
		}
		inc := t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "inline int value() { return 1; }\n")
		good := objCompile(t, cxx, cache, objTU, "-I", inc)
		objs, mans := objFiles(t, cache)
		if good.err != nil || len(objs) != 1 || len(mans) != 1 {
			t.Fatalf("setup: %+v %v %v", good.err, objs, mans)
		}
		objPath, manPath := filepath.Join(cache, "obj", objs[0]), filepath.Join(cache, "obj", mans[0])
		manData, _ := os.ReadFile(manPath)
		var man objManifest
		if err := json.Unmarshal(manData, &man); err != nil || len(man.Entries) != 1 {
			t.Fatalf("the manifest: %v", err)
		}
		setName := man.Entries[0].Set
		for _, c := range []struct {
			name string
			do   func()
		}{
			{"a truncated object", func() { os.Truncate(objPath, 100) }},
			{"an object with another content", func() {
				d := slices.Clone(good.obj)
				d[len(d)/2] ^= 0xff
				os.WriteFile(objPath, d, 0o644)
			}},
			{"a missing object", func() { os.Remove(objPath) }},
			{"a missing set of files", func() { os.Remove(filepath.Join(cache, "obj", setName+".deps")) }},
			{"a broken set of files", func() { os.WriteFile(filepath.Join(cache, "obj", setName+".deps"), []byte("[]"), 0o644) }},
			{"a manifest that is not JSON", func() { os.WriteFile(manPath, []byte("garbage\x00"), 0o644) }},
			{"an empty manifest", func() { os.WriteFile(manPath, nil, 0o644) }},
			{"a manifest of another format", func() { os.WriteFile(manPath, []byte(`{"v":"0","e":[]}`), 0o644) }},
			{"a manifest with a path in the object's name", func() {
				os.WriteFile(manPath, bytes.ReplaceAll(manData, []byte(`"obj":"`), []byte(`"obj":"../`)), 0o644)
			}},
		} {
			c.do()
			r := objCompile(t, cxx, cache, objTU, "-I", inc)
			if r.err != nil || !r.ran || r.hit() || !bytes.Equal(r.obj, good.obj) {
				t.Errorf("%s: ran=%v hit=%v err=%v", c.name, r.ran, r.hit(), r.err)
				continue
			}
			// The compile replaced it: the next one is a hit.
			if r := objCompile(t, cxx, cache, objTU, "-I", inc); r.ran || !r.hit() || !bytes.Equal(r.obj, good.obj) {
				t.Errorf("%s: not repaired: ran=%v hit=%v", c.name, r.ran, r.hit())
			}
		}
	})
}

func TestObjCacheConcurrent(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, inc := t.TempDir(), t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "inline int value() { return 1; }\n")
		const n = 16
		runs := make([]objRun, n)
		works := make([]string, n)
		for i := range works {
			works[i] = t.TempDir()
			if err := os.WriteFile(filepath.Join(works[i], "p.cpp"), []byte(objTU), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		var wg sync.WaitGroup
		for i := range n {
			wg.Go(func() {
				runs[i] = objCompileFile(t, cxx, cache, works[i], filepath.Join(works[i], "p.cpp"), "-I", inc)
			})
		}
		wg.Wait()
		for i, r := range runs {
			if r.err != nil || !bytes.Equal(r.obj, runs[0].obj) {
				t.Fatalf("run %d: %v, object equal: %v", i, r.err, bytes.Equal(r.obj, runs[0].obj))
			}
		}
		if objs, mans := objFiles(t, cache); len(objs) != 1 || len(mans) != 1 {
			t.Errorf("the cache holds %v and %v", objs, mans)
		}
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); r.ran || !r.hit() {
			t.Errorf("after the concurrent builds: ran=%v", r.ran)
		}
		// Different programs at the same time share the folder.
		for i := range n {
			wg.Go(func() { objCompile(t, cxx, cache, fmt.Sprintf("%sint extra_%d;\n", objTU, i%4), "-I", inc) })
		}
		wg.Wait()
		if objs, _ := objFiles(t, cache); len(objs) != 5 {
			t.Errorf("the cache holds %d objects, want 5", len(objs))
		}
	})
}

func TestObjCacheUnusable(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		// A cache folder that is a file, a folder that can't be written, no folder, and the
		// switch: the compile works every time, with no message.
		file := filepath.Join(t.TempDir(), "file")
		os.WriteFile(file, nil, 0o644)
		readonly := t.TempDir()
		os.MkdirAll(filepath.Join(readonly, "obj"), 0o755)
		os.Chmod(filepath.Join(readonly, "obj"), 0o500)
		t.Cleanup(func() { os.Chmod(filepath.Join(readonly, "obj"), 0o755) })
		for _, c := range []struct {
			name, cache string
			off         bool
		}{{"a file", file, false}, {"no folder", "", false}, {"a read-only folder", readonly, false}, {"GOCPP_NOOBJCACHE", t.TempDir(), true}} {
			if c.name == "a read-only folder" && os.Getuid() == 0 {
				continue // root writes anywhere
			}
			if c.off {
				t.Setenv("GOCPP_NOOBJCACHE", "1")
			}
			for range 2 {
				r := objCompile(t, cxx, c.cache, "int main() {}\n")
				if r.err != nil || !r.ran || r.hit() || len(r.out) > 0 {
					t.Errorf("%s: ran=%v hit=%v err=%v out=%q", c.name, r.ran, r.hit(), r.err, r.out)
				}
			}
			if c.off {
				if objs, mans := objFiles(t, c.cache); len(objs)+len(mans) != 0 {
					t.Errorf("%s: left %v %v", c.name, objs, mans)
				}
			}
		}
	})
}

func TestObjCacheEviction(t *testing.T) {
	// trim: the oldest files go first, down to 90% of the limit; stale temporary files go too.
	dir := t.TempDir()
	oc := &objCache{dir: dir}
	now := time.Now()
	for i := range 10 {
		p := filepath.Join(dir, fmt.Sprintf("%d.o", i))
		if err := os.WriteFile(p, make([]byte, 100), 0o644); err != nil {
			t.Fatal(err)
		}
		mod := now.Add(time.Duration(i-20) * time.Minute) // 0.o is the oldest
		os.Chtimes(p, mod, mod)
	}
	stale := filepath.Join(dir, ".tmp-stale")
	os.WriteFile(stale, []byte("x"), 0o644)
	os.Chtimes(stale, now.Add(-2*staleAge), now.Add(-2*staleAge))
	fresh := filepath.Join(dir, ".tmp-fresh")
	os.WriteFile(fresh, []byte("x"), 0o644)
	oc.trim(2000) // fits: nothing goes
	if _, err := os.Stat(filepath.Join(dir, "0.o")); err != nil {
		t.Error("trim removed a file with room left")
	}
	if _, err := os.Stat(stale); err == nil {
		t.Error("a stale temporary file is left")
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Error("a fresh temporary file was removed")
	}
	oc.trim(700) // 1000 bytes: down to 630, so the four oldest files go
	for i := range 10 {
		_, err := os.Stat(filepath.Join(dir, fmt.Sprintf("%d.o", i)))
		if gone := err != nil; gone != (i < 4) {
			t.Errorf("file %d: gone=%v", i, gone)
		}
	}
	left, _ := filepath.Glob(filepath.Join(dir, "*.o"))
	if len(left) != 6 {
		t.Errorf("%d files left, want 6", len(left))
	}

	// Through a store: with a small limit the cache stays small, and a hit keeps its files young.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		t.Setenv("GOCPP_OBJCACHE_SIZE", "30K")
		for i := range 12 {
			// Each marker file is younger than the trim marker, so make it run every time.
			os.Remove(filepath.Join(cache, "obj", ".trimmed"))
			if r := objCompile(t, cxx, cache, fmt.Sprintf("int main() { return %d; }\n", i)); r.err != nil {
				t.Fatal(r.err)
			}
		}
		var total int64
		entries, _ := os.ReadDir(filepath.Join(cache, "obj"))
		for _, e := range entries {
			if fi, err := e.Info(); err == nil && e.Name() != ".trimmed" {
				total += fi.Size()
			}
		}
		if total > 30<<10 || total == 0 {
			t.Errorf("the cache holds %d bytes with a limit of %d", total, 30<<10)
		}
	})
}

func TestObjCacheTouch(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		objCompile(t, cxx, cache, "int main() {}\n")
		objs, mans := objFiles(t, cache)
		old := time.Now().Add(-48 * time.Hour)
		for _, f := range append(objs, mans...) {
			os.Chtimes(filepath.Join(cache, "obj", f), old, old)
		}
		if r := objCompile(t, cxx, cache, "int main() {}\n"); !r.hit() {
			t.Fatal("no hit")
		}
		for _, f := range append(objs, mans...) {
			if fi, err := os.Stat(filepath.Join(cache, "obj", f)); err != nil || time.Since(fi.ModTime()) > time.Hour {
				t.Errorf("%s was not touched by a hit", f)
			}
		}
	})
}

func TestParseDepfile(t *testing.T) {
	for _, c := range []struct {
		in   string
		want []string
	}{
		{"p.o: p.cpp \\\n /usr/include/a.h \\\n  /usr/include/b.h\n", []string{"p.cpp", "/usr/include/a.h", "/usr/include/b.h"}},
		{"p.o: p.cpp /a\\ b/c.h $$x/d.h /e\\#f.h\n", []string{"p.cpp", "/a b/c.h", "$x/d.h", "/e#f.h"}},
		{"a\\ b.o: p.cpp\n", []string{"p.cpp"}},
		{"p.o: /x/y\\\\ \\\n/z\n", []string{"/x/y\\", "/z"}},
		{"p.o:\n", nil},
		{"", nil},
		{"p.o: p.cpp\r\n", []string{"p.cpp"}},
	} {
		if got := parseDepfile([]byte(c.in)); !reflect.DeepEqual(got, c.want) && !(len(got) == 0 && len(c.want) == 0) {
			t.Errorf("parseDepfile(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestObjCacheMax(t *testing.T) {
	for _, c := range []struct {
		env  string
		want int64
	}{{"", defaultObjCacheMax}, {"1000", 1000}, {"5k", 5 << 10}, {"2M", 2 << 20}, {"3GB", 3 << 30}, {"x", defaultObjCacheMax}, {"-5", defaultObjCacheMax}, {"0", defaultObjCacheMax}} {
		t.Setenv("GOCPP_OBJCACHE_SIZE", c.env)
		if got := objCacheMax(); got != c.want {
			t.Errorf("GOCPP_OBJCACHE_SIZE=%q: %d, want %d", c.env, got, c.want)
		}
	}
}

func TestSearchDirs(t *testing.T) {
	got := searchDirs([]string{"-I", "/a", "-iquote", "/q", "-I/b", "-isystem", "/s", "-I", "rel", "-c", "x", "-idirafter", "/z"})
	want := []string{"/q", "/a", "/b", "/s", "/z"}
	if !slices.Equal(got, want) {
		t.Errorf("searchDirs = %q, want %q", got, want)
	}
}

// TestProgramObjCache builds a program with the runtime twice: the second build only links.
// With the precompiled header, a compile that misses uses it, and the entry still lists the
// headers it replaced (g++ doesn't list them itself).
func TestProgramObjCache(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		pkgDir := t.TempDir()
		writeOld(t, filepath.Join(pkgDir, "local.h"), "inline int local() { return 3; }\n")
		writeOld(t, filepath.Join(pkgDir, "x.cpp"), "#include \"local.h\"\nint x_value() { return local(); }\n")
		const prog = "#include <gocpp/rt.hpp>\n#include \"./local.h\"\n#include <cstdio>\nint main() { std::printf(\"%d\\n\", local()); }\n"
		build := func(text string) (trace string, out string) {
			work := t.TempDir()
			src := filepath.Join(work, "p.cpp")
			if err := os.WriteFile(src, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			var tr strings.Builder
			b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Trace: &tr, Stderr: os.Stderr, PackageDir: pkgDir, Dir: pkgDir, CXXFiles: []string{"x.cpp"}}
			bin := filepath.Join(work, "p")
			if err := b.Program(context.Background(), src, bin); err != nil {
				var ce *CompileError
				if errors.As(err, &ce) {
					t.Fatalf("%v\n%s\n%s", ce, ce.Command, ce.Output)
				}
				t.Fatal(err)
			}
			o, err := exec.Command(bin).Output()
			if err != nil {
				t.Fatal(err)
			}
			return tr.String(), string(o)
		}
		// compiles counts the compiles of p.cpp and of the package's x.cpp.
		compiles := func(trace string) (n int) {
			for _, line := range strings.Split(trace, "\n") {
				if strings.HasPrefix(line, cxx.Path+" ") && (strings.Contains(line, " -c $WORK/p.cpp ") || strings.Contains(line, "x.cpp -o $WORK/cobj/x.cpp.o")) {
					n++
				}
			}
			return
		}
		rt, err := ExtractRuntime(cache)
		if err != nil {
			t.Fatal(err)
		}
		first, out := build(prog)
		if out != "3\n" || compiles(first) != 2 {
			t.Fatalf("first build: %q, %d compiles:\n%s", out, compiles(first), first)
		}
		second, out := build(prog)
		if out != "3\n" || compiles(second) != 0 || strings.Count(second, "# cache hit ") != 2 {
			t.Fatalf("second build: %q, %d compiles:\n%s", out, compiles(second), second)
		}
		// The precompiled header exists now: a changed program compiles with it, and its entry
		// has the headers of the standard library.
		changed := prog + "// changed\n"
		third, _ := build(changed)
		if compiles(third) != 1 || !(strings.Contains(third, " -include-pch ") || strings.Contains(third, " -include ")) {
			t.Fatalf("third build doesn't compile with the header:\n%s", third)
		}
		fourth, _ := build(changed)
		if compiles(fourth) != 0 || !strings.Contains(fourth, "# cache hit ") {
			t.Fatalf("fourth build:\n%s", fourth)
		}
		_, mans := objFiles(t, cache)
		sawStd, sawLocal := false, false
		for _, m := range mans {
			data, _ := os.ReadFile(filepath.Join(cache, "obj", m))
			var man objManifest
			if err := json.Unmarshal(data, &man); err != nil {
				t.Fatal(err)
			}
			for _, e := range man.Entries {
				set, err := os.ReadFile(filepath.Join(cache, "obj", e.Set+".deps"))
				if err != nil {
					t.Fatal(err)
				}
				var deps []depRec
				if err := json.Unmarshal(set, &deps); err != nil {
					t.Fatal(err)
				}
				for _, d := range deps {
					sawStd = sawStd || strings.HasSuffix(d.Path, "/vector")
					sawLocal = sawLocal || filepath.Base(d.Path) == "local.h"
				}
			}
		}
		if !sawStd || !sawLocal {
			t.Errorf("the entries miss a header: <vector> %v, local.h %v", sawStd, sawLocal)
		}
		// Another header of the package changes the program: a miss.
		writeOld(t, filepath.Join(pkgDir, "local.h"), "inline int local() { return 4; }\n")
		fifth, out := build(changed)
		if out != "4\n" || compiles(fifth) != 2 { // local.h is included by p.cpp and x.cpp
			t.Errorf("after a change of local.h: %q, %d compiles", out, compiles(fifth))
		}
		// A precompiled header that the compiler can't use: the compile without it is stored,
		// and the next build hits.
		pchs, _ := filepath.Glob(filepath.Join(rt, "pch", "*", "rt.hpp.[pg]ch"))
		if len(pchs) != 1 {
			t.Fatalf("precompiled headers: %q", pchs)
		}
		if err := os.WriteFile(pchs[0], []byte("not a precompiled header"), 0o644); err != nil {
			t.Fatal(err)
		}
		stale := changed + "// stale\n"
		sixth, out := build(stale)
		if out != "4\n" || compiles(sixth) < 1 {
			t.Fatalf("with a stale header: %q, %d compiles:\n%s", out, compiles(sixth), sixth)
		}
		if cxx.Family == Clang {
			if _, err := os.Stat(pchs[0] + ".deps"); err == nil {
				t.Error("the list of the refused header is left")
			}
		}
		if seventh, out := build(stale); out != "4\n" || compiles(seventh) != 0 || !strings.Contains(seventh, "# cache hit ") {
			t.Errorf("after the fallback: %q, %d compiles:\n%s", out, compiles(seventh), seventh)
		}
	})
}

func TestObjCachePoisonedObject(t *testing.T) {
	// An object that is broken while its manifest is gone is not blessed by the next store:
	// objects are named by their content, and one that is there is checked, not trusted.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, inc := t.TempDir(), t.TempDir()
		writeOld(t, filepath.Join(inc, "h.h"), "inline int value() { return 1; }\n")
		good := objCompile(t, cxx, cache, objTU, "-I", inc)
		objs, mans := objFiles(t, cache)
		if good.err != nil || len(objs) != 1 || len(mans) != 1 {
			t.Fatalf("setup: %v %v %v", good.err, objs, mans)
		}
		if err := os.Truncate(filepath.Join(cache, "obj", objs[0]), 0); err != nil {
			t.Fatal(err)
		}
		os.Remove(filepath.Join(cache, "obj", mans[0]))
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); !r.ran || r.hit() || !bytes.Equal(r.obj, good.obj) {
			t.Fatalf("after the damage: ran=%v hit=%v", r.ran, r.hit())
		}
		for range 2 {
			if r := objCompile(t, cxx, cache, objTU, "-I", inc); r.err != nil || r.ran || !r.hit() || !bytes.Equal(r.obj, good.obj) {
				t.Fatalf("the object is not repaired: ran=%v hit=%v err=%v", r.ran, r.hit(), r.err)
			}
		}
		// The object's name is the hash of its bytes.
		objs, _ = objFiles(t, cache)
		data, _ := os.ReadFile(filepath.Join(cache, "obj", objs[0]))
		if sum := sha256.Sum256(data); objs[0] != hex.EncodeToString(sum[:])+".o" {
			t.Errorf("the object %s is not named by its hash", objs[0])
		}
	})
}

func TestObjCacheInclusionProbe(t *testing.T) {
	// #if __has_include: the depfile never lists a header that wasn't found, so such a
	// compile is not cached (unless the probe is in a system header).
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		inc := t.TempDir()
		writeOld(t, filepath.Join(inc, "probe.h"), "#if __has_include(\"extra.h\")\n#include \"extra.h\"\n#endif\ninline int value() { return 1; }\n")
		for _, c := range []struct{ name, tu string }{
			{"in the source", "#if __has_include_next(<nothing.h>)\n#endif\n#if __has_include(\"extra.h\")\n#include \"extra.h\"\n#endif\nint main() {}\n"},
			{"in a header", "#include \"probe.h\"\nint main() { return value() - 1; }\n"},
		} {
			cache := t.TempDir()
			for range 2 {
				if r := objCompile(t, cxx, cache, c.tu, "-I", inc); r.err != nil || !r.ran || r.hit() {
					t.Fatalf("%s: ran=%v hit=%v err=%v", c.name, r.ran, r.hit(), r.err)
				}
			}
			if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
				t.Errorf("%s: left %v %v", c.name, objs, mans)
			}
		}
		// The standard library's headers use it, and are cached (every other test).
		cache := t.TempDir()
		objCompile(t, cxx, cache, "#include <vector>\nint main() { std::vector<int> v; return int(v.size()); }\n")
		if r := objCompile(t, cxx, cache, "#include <vector>\nint main() { std::vector<int> v; return int(v.size()); }\n"); !r.hit() {
			t.Error("a program with the standard library is not cached")
		}
	})
}

func TestObjCacheShadowNextToIncluder(t *testing.T) {
	// #include "x.h" looks next to the including file first: a new x.h there wins over the
	// one that was found in another folder of the include path.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, a, b := t.TempDir(), t.TempDir(), t.TempDir()
		writeOld(t, filepath.Join(a, "sub", "h.h"), "#include \"x.h\"\ninline int value() { return X; }\n")
		writeOld(t, filepath.Join(b, "x.h"), "#define X 1\n")
		const tu = "#include \"sub/h.h\"\nint main() { return value() - 1; }\n"
		first := objCompile(t, cxx, cache, tu, "-I", a, "-I", b)
		if r := objCompile(t, cxx, cache, tu, "-I", a, "-I", b); first.err != nil || r.ran || !r.hit() {
			t.Fatalf("no hit before the new file: %+v", r)
		}
		writeOld(t, filepath.Join(a, "sub", "x.h"), "#define X 2\n")
		if r := objCompile(t, cxx, cache, tu, "-I", a, "-I", b); !r.ran || r.hit() || bytes.Equal(r.obj, first.obj) {
			t.Errorf("a new header next to the includer is ignored: ran=%v hit=%v", r.ran, r.hit())
		}
	})
}

func TestObjCacheKeepWork(t *testing.T) {
	// With -work the cache is not used: no lookup, no store, no prefix map.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		for range 2 {
			work := t.TempDir()
			src := filepath.Join(work, "p.cpp")
			os.WriteFile(src, []byte("int main() {}\n"), 0o644)
			var tr strings.Builder
			b := &Builder{CXX: cxx, Work: work, Cache: cache, Trace: &tr, KeepWork: true}
			ran := false
			out, err := b.compileCached(context.Background(), cxx, src, filepath.Join(work, "p.o"),
				[]string{"-c", src, "-o", filepath.Join(work, "p.o")}, func(a []string) ([]byte, error) {
					ran = true
					return b.runOutput(context.Background(), cxx, "the test", false, a)
				}, nil)
			if err != nil || len(out) > 0 || !ran || strings.Contains(tr.String(), "-MD") || strings.Contains(tr.String(), "prefix-map") || strings.Contains(tr.String(), "cache hit") {
				t.Fatalf("ran=%v err=%v trace:\n%s", ran, err, tr.String())
			}
		}
		if entries, _ := os.ReadDir(filepath.Join(cache, "obj")); len(entries) != 0 {
			t.Errorf("the cache folder has %d files", len(entries))
		}
	})
}

func TestObjCacheSourceEditedDuringCompile(t *testing.T) {
	// A source that changes between the key and the compiler's read is not stored under the
	// key of the old text.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		work := t.TempDir()
		src := filepath.Join(work, "p.cpp")
		os.WriteFile(src, []byte("int main() { return 0; }\n"), 0o644)
		r := objCompileAfter(t, cxx, cache, work, src, func() {
			os.WriteFile(src, []byte("int main() { return 1; }\n"), 0o644)
		})
		if r.err != nil || !r.ran {
			t.Fatal(r.err)
		}
		if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
			t.Errorf("an edited source was stored: %v %v", objs, mans)
		}
		// And the old text still compiles to its own object, which the cache stores.
		os.WriteFile(src, []byte("int main() { return 0; }\n"), 0o644)
		objCompileFile(t, cxx, cache, work, src)
		if objs, _ := objFiles(t, cache); len(objs) != 1 {
			t.Errorf("the unedited source was not stored: %v", objs)
		}
	})
}

func TestCacheableFlags(t *testing.T) {
	for _, bad := range []string{
		"-MD", "-MMD", "-MF", "-MJ", "-MT", "-Wp,-MD,x.d", "-Wp,-E", "-Xpreprocessor", "-Xclang", "-Wa,-aln=x.lst", "-specs=x.specs", "--specs=x",
		"-gsplit-dwarf", "-fdump-tree-all", "-fdump-rtl-expand", "-fstack-usage", "-fcallgraph-info", "-fopt-info", "-fopt-info-vec=x.txt",
		"-fsave-optimization-record", "-fprofile-generate", "-fprofile-use=x", "-fauto-profile=x", "--coverage", "-ftest-coverage",
		"-fplugin=x.so", "-fcrash-diagnostics-dir=x", "-save-temps", "-save-temps=obj", "--save-temps", "-fmodules", "-include-pch",
		"@opts.rsp", "@/tmp/x.rsp", "--config=x.cfg", "--config",
		"-E", "-S", "-fsyntax-only", "-###", "-v", "-print-search-dirs", "-dumpmachine", "-xc++-header", "-fsanitize-ignorelist=x", "-emit-llvm", "-ftime-report",
	} {
		if cacheableFlags([]string{"-std=c++20", bad, "-c", "p.cpp", "-o", "p.o"}) {
			t.Errorf("%s is cacheable", bad)
		}
	}
	if cacheableFlags([]string{"-c", "-x", "c++-header", "p.hpp"}) {
		t.Error("a header compile is cacheable")
	}
	for _, good := range [][]string{
		{"-std=c++20", "-g", "-O0", "-fwrapv", "-DX=1", "-I", "/a", "-iquote", "/b", "-Wall", "-Werror=narrowing", "-x", "c++", "-c", "p.cpp", "-o", "p.o"},
		{"-fsanitize=address,undefined", "-fno-sanitize-recover=all", "-ffile-prefix-map=/a=/b", "-include", "/x/nofortify.h"},
		{"-imacros", "/x/m.h", "--sysroot=/s", "-isysroot", "/s", "-iprefix", "/p", "-iwithprefix", "inc", "-idirafter", "/d", "-imultilib", "m", "-Ifoo@bar"},
	} {
		if !cacheableFlags(good) {
			t.Errorf("%v is not cacheable", good)
		}
	}
}

func TestObjCacheRefreshStat(t *testing.T) {
	// A header that was touched has the recorded content: the hit saves its new stat, so
	// that the next lookup needn't read it, once the change time is older than the racy
	// window too (a file moved into place keeps an old modification time).
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache, inc := t.TempDir(), t.TempDir()
		h := filepath.Join(inc, "h.h")
		writeOld(t, h, "inline int value() { return 1; }\n")
		objCompile(t, cxx, cache, objTU, "-I", inc)
		// savedStat returns the stat of h.h in the saved set.
		savedStat := func() depRec {
			t.Helper()
			_, mans := objFiles(t, cache)
			data, _ := os.ReadFile(filepath.Join(cache, "obj", mans[0]))
			var man objManifest
			if err := json.Unmarshal(data, &man); err != nil || len(man.Entries) != 1 {
				t.Fatalf("the manifest: %v", err)
			}
			set, err := os.ReadFile(filepath.Join(cache, "obj", man.Entries[0].Set+".deps"))
			if err != nil {
				t.Fatal(err)
			}
			var deps []depRec
			json.Unmarshal(set, &deps)
			for _, d := range deps {
				if d.Path == h {
					return d
				}
			}
			t.Fatal("h.h is not in the set")
			return depRec{}
		}
		touched := time.Now().Add(-time.Minute)
		if err := os.Chtimes(h, touched, touched); err != nil {
			t.Fatal(err)
		}
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); !r.hit() {
			t.Fatal("no hit after a touch")
		}
		fi, _ := os.Stat(h)
		if sysStatOf(fi).ctime != 0 && savedStat().Chg == sysStatOf(fi).ctime {
			t.Error("the stat was saved while the change time was younger than the racy window")
		}
		if sysStatOf(fi).ctime == 0 {
			return // no change time on this system
		}
		time.Sleep(objRacyWindow + 100*time.Millisecond)
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); !r.hit() {
			t.Fatal("no hit after the wait")
		}
		if d := savedStat(); d.Mod != fi.ModTime().UnixNano() || d.Chg != sysStatOf(fi).ctime {
			t.Errorf("the set has the old stat of h.h: %+v, now %v", d, fi.ModTime())
		}
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); !r.hit() {
			t.Error("no hit with the new stat")
		}
	})
}

func TestObjCachePCHDeps(t *testing.T) {
	// What a precompiled header was made from is checked against the hashes of the time it was
	// built: a file that changed since makes the object unstorable.
	work, cache, dir := t.TempDir(), t.TempDir(), t.TempDir()
	src := filepath.Join(work, "p.cpp")
	os.WriteFile(src, []byte("int main() {}\n"), 0o644)
	hdr := filepath.Join(dir, "sys.h")
	writeOld(t, hdr, "#define SYS 1\n")
	rec, _, err := recordFile(hdr, time.Now(), false)
	if err != nil {
		t.Fatal(err)
	}
	depfile := filepath.Join(work, "p.o.d")
	os.WriteFile(depfile, []byte("p.o: "+src+"\n"), 0o644)
	b := &Builder{Work: work, Cache: cache}
	pchFile := filepath.Join(dir, "rt.hpp.gch.deps")
	save := func(pd pchDeps) {
		data, _ := json.Marshal(pd)
		os.WriteFile(pchFile, data, 0o600)
	}
	save(pchDeps{Deps: []depRec{rec}})
	entry, err2 := b.recordDeps([]string{depfile}, pchFile, src, time.Now(), nil, CXX{})
	ok := err2 == nil
	if !ok || len(entry.deps) != 1 || entry.deps[0].Hash != rec.Hash {
		t.Fatalf("recordDeps = %+v, %v", entry, ok)
	}
	save(pchDeps{Uncacheable: true, Deps: []depRec{rec}})
	if _, err := b.recordDeps([]string{depfile}, pchFile, src, time.Now(), nil, CXX{}); err == nil {
		t.Error("a list that says uncacheable is stored")
	}
	save(pchDeps{Deps: []depRec{rec}})
	writeOld(t, hdr, "#define SYS 2\n") // the same size: changed in place under the header
	if _, err := b.recordDeps([]string{depfile}, pchFile, src, time.Now(), nil, CXX{}); err == nil {
		t.Error("a file that changed since the header was built is stored")
	}
	if _, err := b.recordDeps([]string{depfile}, filepath.Join(dir, "missing"), src, time.Now(), nil, CXX{}); err == nil {
		t.Error("a missing list is stored")
	}
}

func TestObjCacheOffValues(t *testing.T) {
	for _, c := range []struct {
		v   string
		off bool
	}{{"", false}, {"0", false}, {"false", false}, {"FALSE", false}, {"1", true}, {"yes", true}, {"true", true}} {
		t.Setenv("GOCPP_NOOBJCACHE", c.v)
		b := &Builder{Cache: t.TempDir()}
		if got := b.objects() == nil; got != c.off {
			t.Errorf("GOCPP_NOOBJCACHE=%q: off=%v, want %v", c.v, got, c.off)
		}
	}
	b := &Builder{Cache: t.TempDir(), KeepWork: true}
	t.Setenv("GOCPP_NOOBJCACHE", "")
	if b.objects() != nil {
		t.Error("KeepWork doesn't turn the cache off")
	}
}

func TestObjCacheAbsentCap(t *testing.T) {
	// The files that must stay absent are a product of folders and names, written as such,
	// and a compile that would ask for more than maxAbsentPaths is not stored, which -x says.
	defer func(old int) { maxAbsentPaths = old }(maxAbsentPaths)
	maxAbsentPaths = 2000
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		// gen makes headers headers in folders folders (d0 ... ) under one -iquote folder, and
		// a program that includes them all.
		gen := func(folders, headers int) (root, tu string) {
			root = t.TempDir()
			var b strings.Builder
			for i := range headers {
				rel := fmt.Sprintf("d%d/h%d.h", i%folders, i)
				writeOld(t, filepath.Join(root, rel), fmt.Sprintf("inline int f%d() { return %d; }\n", i, i))
				fmt.Fprintf(&b, "#include %q\n", rel)
			}
			b.WriteString("int main() {}\n")
			return root, b.String()
		}
		// Small: stored, hits, and the manifest is small (no product written out).
		root, tu := gen(10, 30)
		cache := t.TempDir()
		objCompile(t, cxx, cache, tu, "-iquote", root)
		if r := objCompile(t, cxx, cache, tu, "-iquote", root); !r.hit() {
			t.Fatal("a package with 30 headers in 10 folders is not cached")
		}
		_, mans := objFiles(t, cache)
		if fi, err := os.Stat(filepath.Join(cache, "obj", mans[0])); err != nil || fi.Size() > 8<<10 {
			t.Errorf("the manifest is %v bytes (%v): the absent files are written out", fi.Size(), err)
		}
		// A new file that no include names is no reason to miss (the product form still
		// catches the files that hide one: TestObjCacheShadowNextToIncluder,
		// TestObjCacheShadowingHeader).
		writeOld(t, filepath.Join(root, "d4", "unrelated.h"), "inline int other() { return 0; }\n")
		if r := objCompile(t, cxx, cache, tu, "-iquote", root); !r.hit() {
			t.Error("an unrelated new file is a miss")
		}
		// Too many: 61 headers in 40 folders ask for 40 * 61 files.
		root, tu = gen(40, 61)
		cache = t.TempDir()
		for range 2 {
			r := objCompile(t, cxx, cache, tu, "-iquote", root)
			if r.err != nil || !r.ran || r.hit() {
				t.Fatalf("over the cap: ran=%v hit=%v err=%v", r.ran, r.hit(), r.err)
			}
			if !strings.Contains(r.trace, "# not cached: too many include candidates (") {
				t.Errorf("-x doesn't say why the compile is not stored:\n%s", r.trace)
			}
		}
		if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
			t.Errorf("over the cap: left %v %v", objs, mans)
		}
	})
}

func TestObjCacheNative(t *testing.T) {
	// Objects made for the host's CPU are neither stored nor looked up, however the flag
	// comes: in the flags, the compiler's arguments or the wrapper's environment.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		withArgs := cxx
		withArgs.Args = []string{"-mtune=native"}
		for _, c := range []struct {
			name  string
			cxx   CXX
			env   string
			flags []string
		}{
			{"flags", cxx, "", []string{"-march=native"}},
			{"compiler arguments", withArgs, "", nil},
			{"environment", cxx, "-O2 -march=native", nil},
		} {
			cache := t.TempDir()
			if c.env != "" {
				t.Setenv("NIX_CFLAGS_COMPILE", c.env)
			}
			for range 2 {
				work := t.TempDir()
				src := filepath.Join(work, "p.cpp")
				os.WriteFile(src, []byte("int main() {}\n"), 0o644)
				var tr strings.Builder
				b := &Builder{CXX: c.cxx, Work: work, Cache: cache, Trace: &tr}
				ran := false
				obj := filepath.Join(work, "p.o")
				_, err := b.compileCached(context.Background(), c.cxx, src, obj, slices.Concat(c.flags, []string{"-c", src, "-o", obj}),
					func(a []string) ([]byte, error) {
						ran = true
						return b.runOutput(context.Background(), c.cxx, "the test", false, a)
					}, nil)
				b.stores.Wait()
				if err != nil {
					t.Fatalf("%s: %v", c.name, err)
				}
				if !ran || strings.Contains(tr.String(), "cache hit") || strings.Contains(tr.String(), "-MD") {
					t.Errorf("%s: ran=%v trace:\n%s", c.name, ran, tr.String())
				}
			}
			if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
				t.Errorf("%s: left %v %v", c.name, objs, mans)
			}
		}
	})
}

func TestNativeArch(t *testing.T) {
	for _, c := range []struct {
		parts []string
		want  bool
	}{
		{nil, false},
		{[]string{"-O2", "-march=x86-64"}, false},
		{[]string{"-march=native"}, true},
		{[]string{"-O2", "-mtune=native"}, true},
		{[]string{"-mcpu=native"}, true},
		{[]string{"NIX_CFLAGS_COMPILE=-O2 -march=native -g"}, true},
		{[]string{"-Xclang", "-target-cpu", "x", "-march=native"}, true},
		{[]string{"-march=nativeish"}, true}, // as the probe cache always treated it
		{[]string{"-march", "native"}, false},
	} {
		if got := NativeArch(c.parts...); got != c.want {
			t.Errorf("NativeArch(%q) = %v, want %v", c.parts, got, c.want)
		}
	}
}

func TestEnvOff(t *testing.T) {
	for _, c := range []struct {
		v    string
		want bool
	}{{"", false}, {"0", false}, {"false", false}, {"FALSE", false}, {"False", false}, {"1", true}, {"yes", true}, {"true", true}, {"00", true}, {" ", true}} {
		t.Setenv("GOCPP_TEST_SWITCH", c.v)
		if got := EnvOff("GOCPP_TEST_SWITCH"); got != c.want {
			t.Errorf("EnvOff with %q = %v, want %v", c.v, got, c.want)
		}
	}
}

func TestUserFile(t *testing.T) {
	rt := "/cache/rt/"
	dirs := []string{"/home/u/proj/inc", "/usr/local/include/lib"}
	for _, c := range []struct {
		p    string
		want bool
	}{
		{"/home/u/proj/inc/a.h", true},
		{"/home/u/proj/other/b.h", true},  // outside the include folders, and not the system's
		{"/home/u/bin/include/c.h", true}, // a compiler in ~/bin says nothing
		{"/usr/local/include/lib/d.h", true},
		{"/usr/include/c++/15/vector", false},
		{"/nix/store/abc-gcc/include/e.h", false},
		{"/cache/rt/x/include/gocpp/rt.hpp", false},
	} {
		if got := userFile(c.p, dirs, rt); got != c.want {
			t.Errorf("userFile(%s) = %v, want %v", c.p, got, c.want)
		}
	}
}

func TestObjCacheResponseFile(t *testing.T) {
	// A response file's content is not in the key and not in the dependency file: the compile
	// is never cached (a changed file must change the object). Files of -include and -imacros
	// are dependencies, and a change of them is a miss.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		const tu = "#ifndef VAL\n#define VAL 0\n#endif\nint main() { return VAL; }\n"
		dir := t.TempDir()
		rsp := filepath.Join(dir, "opts.rsp")
		cache := t.TempDir()
		var objs [][]byte
		for _, v := range []string{"1", "2"} {
			os.WriteFile(rsp, []byte("-DVAL="+v+"\n"), 0o644)
			r := objCompile(t, cxx, cache, tu, "@"+rsp)
			if r.err != nil || !r.ran || r.hit() || !strings.Contains(r.trace, "# not cached: the flag @") {
				t.Fatalf("VAL=%s: ran=%v hit=%v err=%v trace:\n%s", v, r.ran, r.hit(), r.err, r.trace)
			}
			objs = append(objs, r.obj)
		}
		if bytes.Equal(objs[0], objs[1]) {
			t.Error("a changed response file gave the same object")
		}
		if o, m := objFiles(t, cache); len(o)+len(m) != 0 {
			t.Errorf("left %v %v", o, m)
		}
		for _, flag := range []string{"-include", "-imacros"} {
			cache := t.TempDir()
			h := filepath.Join(dir, "val.h")
			writeOld(t, h, "#define VAL 1\n")
			a := objCompile(t, cxx, cache, tu, flag, h)
			if r := objCompile(t, cxx, cache, tu, flag, h); a.err != nil || !r.hit() {
				t.Fatalf("%s: no hit: %+v", flag, r)
			}
			writeOld(t, h, "#define VAL 2\n")
			if r := objCompile(t, cxx, cache, tu, flag, h); !r.ran || r.hit() || bytes.Equal(r.obj, a.obj) {
				t.Errorf("%s: a changed file is ignored: ran=%v hit=%v", flag, r.ran, r.hit())
			}
		}
	})
}

func TestObjCacheNotCachedNotes(t *testing.T) {
	// Every reason for not storing a compile has a "# not cached" line under -x.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		for _, c := range []struct {
			name, tu, want string
			flags          []string
		}{
			{"time macro", "const char* d = __DATE__;\nint main() {}\n", "uses __DATE__", nil},
			{"inclusion probe", "#if __has_include(\"x.h\")\n#endif\nint main() {}\n", "uses __has_include", nil},
			{"warning", "int main() { int unused; return 0; }\n", "printed something", []string{"-Wall", "-Wextra"}},
			{"flag", "int main() {}\n", "the flag -save-temps", []string{"-save-temps=obj"}},
			{"native", "int main() {}\n", "-march=native", []string{"-march=native"}},
		} {
			r := objCompile(t, cxx, t.TempDir(), c.tu, c.flags...)
			if r.err != nil || !strings.Contains(r.trace, "# not cached: ") || !strings.Contains(r.trace, c.want) {
				t.Errorf("%s: err=%v trace:\n%s", c.name, r.err, r.trace)
			}
		}
		// A header changed a moment ago.
		cache, inc := t.TempDir(), t.TempDir()
		os.WriteFile(filepath.Join(inc, "h.h"), []byte("inline int value() { return 1; }\n"), 0o644)
		if r := objCompile(t, cxx, cache, objTU, "-I", inc); !strings.Contains(r.trace, "# not cached: ") || !strings.Contains(r.trace, "less than two seconds") {
			t.Errorf("racy header: trace:\n%s", r.trace)
		}
	})
}

func TestPchRefused(t *testing.T) {
	const file = "/c/rt/pch/k/rt.hpp.pch"
	for _, c := range []struct {
		out  string
		want bool
	}{
		{"error: PCH file built from a different branch (x) than the AST file (y)", true},
		{"fatal error: file 'x' has been modified since the precompiled header 'y' was built", true},
		{"error: definition of macro 'VAL' differs between the precompiled file 'h.pch' ('2') and the command line ('3')", true},
		{"error: input is not a PCH file: 'bad.pch'", true},
		{"fatal error: file 'bad.pch' is not a valid PCH file: file doesn't start with precompiled file magic", true},
		{"clang: error: no such file or directory: '-include-pch'", true},
		{"error: cannot open " + file, true},
		{"x.cpp:3:5: error: use of undeclared identifier 'y'", false},
		{"", false},
	} {
		if got := pchRefused([]byte(c.out), file); got != c.want {
			t.Errorf("pchRefused(%q) = %v, want %v", c.out, got, c.want)
		}
	}
}

func TestProgramResponseFileInCompilerArgs(t *testing.T) {
	// A response file in the compiler's arguments changes what a precompiled header means, and
	// the key only has its name: no header is used, and a changed file doesn't break builds (it
	// did with clang: "definition of macro differs between the precompiled file ...").
	const prog = "#include <gocpp/rt.hpp>\n#include <cstdio>\n#ifndef VAL\n#define VAL 0\n#endif\nint main() { std::printf(\"%d\\n\", VAL); }\n"
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		rsp := filepath.Join(t.TempDir(), "o.rsp")
		cxx.Args = append(slices.Clone(cxx.Args), "@"+rsp)
		for _, v := range []string{"1", "2", "3"} {
			if err := os.WriteFile(rsp, []byte("-DVAL="+v+"\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			work := t.TempDir()
			src := filepath.Join(work, "p.cpp")
			if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
				t.Fatal(err)
			}
			var tr strings.Builder
			b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Trace: &tr, Stderr: os.Stderr}
			bin := filepath.Join(work, "p")
			if err := b.Program(context.Background(), src, bin); err != nil {
				var ce *CompileError
				if errors.As(err, &ce) {
					t.Fatalf("VAL=%s: %v\n%s\n%s", v, ce, ce.Command, ce.Output)
				}
				t.Fatal(err)
			}
			if out, err := exec.Command(bin).Output(); err != nil || string(out) != v+"\n" {
				t.Fatalf("VAL=%s: the program prints %q, %v", v, out, err)
			}
			if strings.Contains(tr.String(), " -include-pch ") || strings.Contains(tr.String(), "c++-header") || strings.Contains(tr.String(), "# cache hit") {
				t.Errorf("VAL=%s: a header or a cache hit with a response file:\n%s", v, tr.String())
			}
		}
	})
}
