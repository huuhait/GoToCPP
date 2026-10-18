package build

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The object cache for the sources of a library (Phase 06 step 6.1d): the key
// has the module path and version (the salt), the flags and the compiler, and
// the headers the compiler read.

// libCompile compiles src as a library source of (module, version) with the
// extra flags through compileCachedSalt, in a new work folder.
func libCompile(t *testing.T, cxx CXX, cache, src, module, version string, flags ...string) objRun {
	t.Helper()
	work := t.TempDir()
	var tr strings.Builder
	b := &Builder{CXX: cxx, Work: work, Cache: cache, Trace: &tr}
	obj := filepath.Join(work, "lib.o")
	args := slices.Concat([]string{"-std=c++20", "-g", "-O0", "-w"}, flags, []string{"-c", "-x", "c++", src, "-o", obj})
	var r objRun
	r.work = work
	r.out, r.err = b.compileCachedSalt(context.Background(), cxx, src, obj, args, func(a []string) ([]byte, error) {
		r.ran = true
		return b.runOutput(context.Background(), cxx, "the library", false, a)
	}, nil, []string{"cxx", module, version})
	b.stores.Wait()
	r.trace = tr.String()
	if r.err != nil {
		t.Fatalf("compile: %v\n%s", r.err, r.out)
	}
	return r
}

func TestLibCacheMatrix(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		cache := t.TempDir()
		src := filepath.Join(t.TempDir(), "lib.cpp")
		writeOld(t, src, "#include \"api.h\"\nint f() { return API_VALUE; }\n#ifdef EXTRA\nint extra;\n#endif\n")
		inc := t.TempDir()
		writeOld(t, filepath.Join(inc, "api.h"), "#define API_VALUE 1\n")
		base := []string{"-I", inc}

		const mod, ver = "example.com/lib", "v1.0.0"
		if r := libCompile(t, cxx, cache, src, mod, ver, base...); !r.ran || r.hit() {
			t.Fatalf("the first compile: ran %v, hit %v\n%s", r.ran, r.hit(), r.trace)
		}
		if r := libCompile(t, cxx, cache, src, mod, ver, base...); r.ran || !r.hit() {
			t.Fatalf("the same compile again (another work folder) must be a cache hit: ran %v\n%s", r.ran, r.trace)
		}
		for _, tc := range []struct {
			name, module, version string
			flags                 []string
		}{
			{"another module", "example.com/other", ver, base},
			{"another version", mod, "v1.0.1", base},
			{"a define", mod, ver, slices.Concat(base, []string{"-DEXTRA=1"})},
			{"cflags", mod, ver, slices.Concat(base, []string{"-O1"})},
			{"the sanitizers", mod, ver, slices.Concat(base, asanFlags)},
			{"the include folder", mod, ver, []string{"-I", inc + "/"}},
		} {
			if r := libCompile(t, cxx, cache, src, tc.module, tc.version, tc.flags...); !r.ran || r.hit() {
				t.Errorf("%s: the compile must miss: ran %v, hit %v\n%s", tc.name, r.ran, r.hit(), r.trace)
			}
		}
		// Every one of them was kept: all hit now, and the first one still does.
		if r := libCompile(t, cxx, cache, src, "example.com/other", ver, base...); r.ran {
			t.Errorf("another module again: not a hit")
		}
		if r := libCompile(t, cxx, cache, src, mod, ver, base...); r.ran {
			t.Errorf("the first compile again: not a hit")
		}
		// A header that changed: a miss, and the new object is kept.
		writeOld(t, filepath.Join(inc, "api.h"), "#define API_VALUE 2\n")
		if r := libCompile(t, cxx, cache, src, mod, ver, base...); !r.ran || r.hit() {
			t.Errorf("a changed header must recompile: ran %v\n%s", r.ran, r.trace)
		}
		if r := libCompile(t, cxx, cache, src, mod, ver, base...); r.ran {
			t.Errorf("after the header changed: not a hit")
		}
		// The text of the source: a miss.
		writeOld(t, src, "#include \"api.h\"\nint f() { return API_VALUE + 1; }\n")
		if r := libCompile(t, cxx, cache, src, mod, ver, base...); !r.ran || r.hit() {
			t.Errorf("a changed source must recompile: ran %v\n%s", r.ran, r.trace)
		}
		_ = os.Remove(src)
	})
}

// TestCompileLibsArgs checks the command line of a library compile: the
// sanitizers with ASan, -w, the standard, the module folder as -iquote, the public
// and private flags in order, and the environment's flags for the right language.
func TestCompileLibsArgs(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		dir := t.TempDir()
		writeTree(t, dir, "a.cpp", "b.cc")
		rt := t.TempDir()
		writeOld(t, filepath.Join(rt, "include", "gocpp", "nofortify.h"), "\n")
		lib := libOf(t, dir, "\tsrc a.cpp b.cc")
		lib.Public = []string{"-I", dir, "-DPUB"}
		lib.Private = []string{"-DPRIV"}
		lib.PrivateCXX = []string{"-fno-rtti"}
		lib.Std = "c++17"
		for _, asan := range []bool{false, true} {
			var tr strings.Builder
			b := &Builder{CXX: cxx, ASan: asan, Work: t.TempDir(), Trace: &tr, Package: PackageFlags{Libs: []*CxxLib{lib}},
				Env: EnvFlags{CFlags: []string{"-DENVC"}, CXXFlags: []string{"-DENVCXX"}}}
			objs, err := b.compileLibsOf(context.Background(), rt, b.Package.Libs)
			if err != nil || len(objs) != 2 {
				t.Fatalf("compileLibs: %v %v", objs, err)
			}
			var compiles []string
			for _, l := range strings.Split(tr.String(), "\n") {
				if strings.Contains(l, " -c -x c++ ") {
					compiles = append(compiles, l)
				}
			}
			if len(compiles) != 2 {
				t.Fatalf("compiles: %q", compiles)
			}
			for _, l := range compiles {
				for _, want := range []string{" -std=c++17 ", " -w ", " -iquote " + dir + " ", " -DENVC ", " -DENVCXX ", " -I " + dir + " -DPUB -DPRIV -fno-rtti -c -x c++ "} {
					if !strings.Contains(l, want) {
						t.Errorf("asan %v: the compile doesn't have %q:\n%s", asan, want, l)
					}
				}
				if got := strings.Contains(l, "-fsanitize="); got != asan {
					t.Errorf("asan %v: -fsanitize in the compile: %v\n%s", asan, got, l)
				}
			}
		}
	})
}
