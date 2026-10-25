package build

import (
	"context"
	"debug/elf"
	"debug/pe"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	rtfiles "github.com/huuhait/gocpp/runtime"
)

// Tests of the runtime library and the precompiled header with the machine's own compilers
// (g++ and clang++ when both are there).

const helloCPP = "#include <gocpp/rt.hpp>\n#include <cstdio>\nint main() { std::puts(\"ok\"); }\n"

// realProgram builds helloCPP with b and runs it.
func realProgram(t *testing.T, b *Builder) string {
	t.Helper()
	src := filepath.Join(b.Work, "p.cpp")
	if err := os.WriteFile(src, []byte(helloCPP), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(b.Work, "p")
	if err := b.Program(context.Background(), src, bin); err != nil {
		var ce *CompileError
		if errors.As(err, &ce) {
			t.Fatalf("%v\n%s\n%s", ce, ce.Command, ce.Output)
		}
		t.Fatal(err)
	}
	out, err := exec.Command(bin).CombinedOutput()
	if err != nil || string(out) != "ok\n" {
		t.Fatalf("the program printed %q, %v", out, err)
	}
	return bin
}

func realBuilder(t *testing.T, cxx CXX, cache string, trace *syncBuffer, f func(b *Builder)) *Builder {
	t.Helper()
	b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: t.TempDir(), Cache: cache, Stderr: os.Stderr}
	if trace != nil {
		b.Trace = trace
	}
	if f != nil {
		f(b)
	}
	return b
}

func TestRealRuntimeLibrary(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		t.Setenv("GOCPP_AR", "")
		t.Setenv("GOCPP_NORTLIB", "")
		cache := t.TempDir()
		var tr1, tr2 syncBuffer
		realProgram(t, realBuilder(t, cxx, cache, &tr1, nil))
		realProgram(t, realBuilder(t, cxx, cache, &tr2, nil))
		dir, _ := ExtractRuntime(cache)
		libs, _ := filepath.Glob(filepath.Join(dir, "lib", "*", rtLibName))
		if len(libs) != 1 || !libOK(libs[0]) {
			t.Fatalf("libraries: %q", libs)
		}
		if !strings.Contains(tr1.String(), rtLibName) || !strings.Contains(tr1.String(), "-c ") {
			t.Errorf("the first build's trace shows no runtime compile or library:\n%s", tr1.String())
		}
		// The second build compiles only the program's unit, and links the same library.
		compiles := 0
		for _, l := range strings.Split(tr2.String(), "\n") {
			if strings.HasPrefix(l, cxx.Path+" ") && strings.Contains(l, " -c ") {
				compiles++
			}
		}
		if compiles > 1 {
			t.Errorf("the second build made %d compiles, want at most the program's unit:\n%s", compiles, tr2.String())
		}
		if !strings.Contains(tr2.String(), libs[0]) {
			t.Errorf("the second build does not link %s:\n%s", libs[0], tr2.String())
		}
		// The library holds every runtime object, and an index (the linker found symbols in it).
		if nm, err := exec.Command("ar", "t", libs[0]).Output(); err == nil && len(strings.Fields(string(nm))) != nSources() {
			t.Errorf("the library has %d members, want %d", len(strings.Fields(string(nm))), nSources())
		}
		// The same program without the library (the runtime's objects) works as well.
		t.Setenv("GOCPP_NORTLIB", "1")
		realProgram(t, realBuilder(t, cxx, cache, nil, nil))
		if objs, _ := filepath.Glob(filepath.Join(dir, "obj", "*", "*.o")); len(objs) != nSources() {
			t.Errorf("GOCPP_NORTLIB left %d object files in obj/, want %d", len(objs), nSources())
		}
	})
}

func TestRealRuntimeLibraryConcurrent(t *testing.T) {
	// Builds at the same time with the real tools: one library, every program works.
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		t.Setenv("GOCPP_AR", "")
		t.Setenv("GOCPP_NORTLIB", "")
		cache := t.TempDir()
		var wg sync.WaitGroup
		bs := make([]*Builder, 3)
		for i := range bs {
			bs[i] = realBuilder(t, cxx, cache, nil, nil)
			wg.Go(func() {
				src := filepath.Join(bs[i].Work, "p.cpp")
				os.WriteFile(src, []byte(helloCPP), 0o644)
				if err := bs[i].Program(context.Background(), src, filepath.Join(bs[i].Work, "p")); err != nil {
					t.Errorf("build %d: %v", i, err)
				}
			})
		}
		wg.Wait()
		dir, _ := ExtractRuntime(cache)
		if got := libFolders(t, dir); len(got) != 1 {
			t.Errorf("lib/ holds %q, want one folder", got)
		}
		assertNoTemp(t, filepath.Join(dir, "lib"))
		for _, b := range bs {
			if out, err := exec.Command(filepath.Join(b.Work, "p")).Output(); err != nil || string(out) != "ok\n" {
				t.Errorf("%s: %q %v", b.Work, out, err)
			}
		}
	})
}

func TestRealSanitizerRuntimeIsSeparate(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		t.Setenv("GOCPP_AR", "")
		t.Setenv("GOCPP_NORTLIB", "")
		cache := t.TempDir()
		realProgram(t, realBuilder(t, cxx, cache, nil, nil))
		realProgram(t, realBuilder(t, cxx, cache, nil, func(b *Builder) { b.ASan = true }))
		dir, _ := ExtractRuntime(cache)
		libs, _ := filepath.Glob(filepath.Join(dir, "lib", "*", rtLibName))
		if len(libs) != 2 {
			t.Fatalf("libraries: %q, want a plain one and a sanitized one", libs)
		}
		// The sanitized library really has sanitizer references; the plain one has none.
		withASan := 0
		for _, l := range libs {
			data, err := os.ReadFile(l)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), "__asan_") || strings.Contains(string(data), "__ubsan_") {
				withASan++
			}
		}
		if withASan != 1 {
			t.Errorf("%d of the 2 libraries refer to the sanitizer runtime, want 1", withASan)
		}
	})
}

func TestRealLTOSmoke(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		t.Setenv("GOCPP_AR", "")
		t.Setenv("GOCPP_NORTLIB", "")
		cache := t.TempDir()
		b := realBuilder(t, cxx, cache, nil, func(b *Builder) { b.LTO = true })
		if _, err := b.rtArchiver(context.Background()); err != nil {
			t.Skip("no LTO archiver on this machine (the build says so):", err)
		}
		realProgram(t, b)
		// A second program with LTO reuses the LTO library; the plain build makes another.
		var tr syncBuffer
		realProgram(t, realBuilder(t, cxx, cache, &tr, func(b *Builder) { b.LTO = true }))
		for _, l := range strings.Split(tr.String(), "\n") {
			if strings.HasPrefix(l, cxx.Path+" ") && strings.Contains(l, " -c ") && strings.Contains(l, "/rt/") && strings.Contains(l, "/src/") {
				t.Errorf("the LTO runtime was compiled again: %s", l)
			}
		}
		realProgram(t, realBuilder(t, cxx, cache, nil, nil))
		dir, _ := ExtractRuntime(cache)
		if got := libFolders(t, dir); len(got) != 2 {
			t.Errorf("lib/ holds %q, want the LTO library and the plain one", got)
		}
	})
}

func TestRealLTOUsesTheMatchingArchiver(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		t.Setenv("GOCPP_AR", "")
		a, err := FindArchiver(context.Background(), cxx, true)
		if err != nil {
			t.Skip("no LTO archiver:", err)
		}
		switch cxx.Family {
		case GCC:
			if a.Name != "gcc-ar" && a.Name != "ar+plugin" {
				t.Errorf("g++ with LTO got the archiver %+v", a)
			}
		case Clang:
			if a.Name != "llvm-ar" && a.Name != "ar+plugin" {
				t.Errorf("clang++ with LTO got the archiver %+v", a)
			}
		}
		// And an archive made with it has a symbol table that the linker can use: link a
		// program from a library of an LTO object.
		dir := t.TempDir()
		write := func(name, text string) string {
			p := filepath.Join(dir, name)
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				t.Fatal(err)
			}
			return p
		}
		u, m := write("u.cpp", "int answer() { return 42; }\n"), write("m.cpp", "int answer();\nint main() { return answer() == 42 ? 0 : 1; }\n")
		b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: dir, LTO: true}
		lto := LTOFlags(cxx)
		uo, mo, lib, bin := filepath.Join(dir, "u.o"), filepath.Join(dir, "m.o"), filepath.Join(dir, "libu.a"), filepath.Join(dir, "p")
		ctx := context.Background()
		for _, c := range [][]string{{"-c", u, "-o", uo}, {"-c", m, "-o", mo}} {
			if err := b.run(ctx, cxx, "the test", false, slices.Concat(lto, c)); err != nil {
				t.Fatal(err)
			}
		}
		if err := b.makeLib(ctx, a, lib, []string{uo}); err != nil {
			t.Fatal(err)
		}
		if err := b.run(ctx, cxx, "the test", true, slices.Concat(lto, []string{mo, lib, "-o", bin})); err != nil {
			t.Fatalf("a program from a library of LTO objects did not link: %v", err)
		}
		if err := exec.Command(bin).Run(); err != nil {
			t.Errorf("the program: %v", err)
		}
	})
}

// Precompiled header: an edit of a header that it holds invalidates it.
func TestPrecompiledHeaderInvalidatedByEdit(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		t.Setenv("GOCPP_NOPCH", "")
		cache := t.TempDir()
		rtDir, err := ExtractRuntime(cache)
		if err != nil {
			t.Fatal(err)
		}
		mk := func() *Builder {
			return &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: t.TempDir(), Cache: cache, Stderr: os.Stderr}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		pc := mk().precompiled(rtDir)
		if pc == nil {
			t.Skip("no precompiled header for this compiler")
		}
		pc.build(ctx)
		if !pc.ready() {
			t.Fatal("no header after a build")
		}
		// The same header is ready in another build (a new Builder: it checks the files again).
		if !mk().precompiled(rtDir).ready() {
			t.Fatal("the header is not ready for a second build")
		}
		// Edit a header that rt.hpp includes: a macro that a compile can see.
		hdr := filepath.Join(rtDir, "include", "gocpp", "rt.hpp")
		old := readText(t, hdr)
		if err := os.Chmod(hdr, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(hdr, []byte(old+"\n#define GOCPP_PCH_PROBE 7\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		// A build that uses the edited tree: the header is stale. It must not be used (g++
		// would use it without noticing: it compares flags, not the text), and it is removed.
		b2 := mk()
		pc2 := b2.precompiled(rtDir)
		if pc2.ready() {
			t.Fatal("the precompiled header is still ready after an edit of rt.hpp")
		}
		if _, err := os.Stat(pc2.file); err == nil {
			t.Error("the stale header was not removed")
		}
		// A compile now sees the edit.
		src := filepath.Join(b2.Work, "probe.cpp")
		os.WriteFile(src, []byte("#include <gocpp/rt.hpp>\n#if GOCPP_PCH_PROBE != 7\n#error the edit of rt.hpp is not seen\n#endif\nint f() { return 1; }\n"), 0o644)
		args := slices.Concat(b2.pchFlags(), []string{"-I", filepath.Join(rtDir, "include"), "-c", src, "-o", filepath.Join(b2.Work, "probe.o")})
		out, used, err := b2.runGenerated(ctx, pc2, args)
		if err != nil || used {
			t.Fatalf("the compile with the stale header: %q used=%v %v", out, used, err)
		}
		// The next build makes a new header from the edited text, and uses it.
		pc2.build(ctx)
		b3 := mk()
		pc3 := b3.precompiled(rtDir)
		if !pc3.ready() {
			t.Fatal("no new header after the rebuild")
		}
		out, used, err = b3.runGenerated(ctx, pc3, args)
		if err != nil || !used {
			t.Fatalf("the compile with the new header: %q used=%v %v", out, used, err)
		}
		// Restoring the text makes it stale again (a change in either direction).
		if err := os.WriteFile(hdr, []byte(old), 0o644); err != nil {
			t.Fatal(err)
		}
		if mk().precompiled(rtDir).ready() {
			t.Error("the header is ready after the text went back to what it was before the build")
		}
	})
}

func TestPrecompiledHeaderKeyedByFlags(t *testing.T) {
	// A header is made for a set of flags: another sanitizer, -nochecks, LTO or compiler is
	// another header (the key is in its folder name).
	cxx, err := LookCXX(context.Background(), "g++")
	if err != nil {
		t.Skip("no g++:", err)
	}
	t.Setenv("GOCPP_NOPCH", "")
	cache := t.TempDir()
	rtDir, _ := ExtractRuntime(cache)
	files := map[string]string{}
	for name, f := range map[string]func(b *Builder){
		"plain":    nil,
		"asan":     func(b *Builder) { b.ASan = true },
		"nochecks": func(b *Builder) { b.NoChecks = true },
		"lto":      func(b *Builder) { b.LTO = true },
	} {
		b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: t.TempDir(), Cache: cache}
		if f != nil {
			f(b)
		}
		files[name] = b.precompiled(rtDir).file
	}
	seen := map[string]string{}
	for name, f := range files {
		if other, dup := seen[f]; dup {
			t.Errorf("%s and %s share the header %s", name, other, f)
		}
		seen[f] = name
	}
	// Flags of the user and the environment: no header at all.
	b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: t.TempDir(), Cache: cache, Env: EnvFlags{CXXFlags: []string{"-O2"}}}
	if b.precompiled(rtDir) != nil {
		t.Error("a precompiled header with flags of the environment")
	}
}

// TestRuntimeObjectsHaveNoStaticInit: a library member is taken only for a symbol that something
// wants, so a runtime object whose job is a global constructor would be dropped from programs
// that don't reference it (rtlib.go). And a library's package initialization may run before the
// static initialization (an exported function that a host's C constructor calls, gocpp/lib.hpp):
// what it uses must be ready then, constant-initialized or made at the first use (the timer heap,
// math's CPU flags). The runtime's objects have no initializer, except testing.o (the natives of
// package testing: its alarm and running-test state is in an unnamed namespace, so only its own
// functions read it, and the object is taken by those functions only; a test binary has a main). A
// new initializer elsewhere fails here.
// A global with a destructor counts: its registration is an initializer too. The checks cover the
// natives of package testing (rtfiles.TestingSources) as well. The ELF objects of Linux and the
// COFF objects of the MinGW-w64 compilers (when they are installed) are read.
var ownDataOnly = map[string]bool{"src_testing.o": true}

// staticInitSections are the section name prefixes that hold start-up and exit code pointers: ELF
// (.init_array, .ctors, .preinit_array, .fini_array, .dtors) and COFF (.ctors and .dtors of GCC,
// .CRT$XC* and .CRT$XI* of Clang).
var staticInitSections = []string{
	".init_array", ".ctors", ".preinit_array", ".fini_array", ".dtors", ".CRT$XC", ".CRT$XI",
}

func TestRuntimeObjectsHaveNoStaticInit(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads ELF objects")
	}
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		checkNoStaticInit(t, cxx, runtime.GOOS, func(path string) ([]string, error) {
			f, err := elf.Open(path)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			var sections []string
			for _, s := range f.Sections {
				if s.Size > 0 {
					sections = append(sections, s.Name)
				}
			}
			return sections, nil
		})
	})
}

// TestRuntimeObjectsHaveNoStaticInitWindows is the same for the Windows objects, compiled by the
// MinGW-w64 cross compilers.
func TestRuntimeObjectsHaveNoStaticInitWindows(t *testing.T) {
	found := false
	for _, name := range []string{"x86_64-w64-mingw32-g++", "x86_64-w64-mingw32-clang++"} {
		cxx, err := LookCXX(context.Background(), name)
		if err != nil {
			continue
		}
		found = true
		t.Run(name, func(t *testing.T) {
			checkNoStaticInit(t, cxx, "windows", func(path string) ([]string, error) {
				f, err := pe.Open(path)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				var sections []string
				for _, s := range f.Sections {
					if s.Size > 0 {
						sections = append(sections, s.Name)
					}
				}
				return sections, nil
			})
		})
	}
	if !found {
		t.Skip("no MinGW-w64 compiler")
	}
}

// checkNoStaticInit compiles the runtime of goos with cxx and fails for each object that has a
// section of staticInitSections; sections lists the names of the non-empty sections of an object.
func checkNoStaticInit(t *testing.T, cxx CXX, goos string, sections func(path string) ([]string, error)) {
	t.Helper()
	cache := t.TempDir()
	b := &Builder{CXX: cxx, GOOS: goos, Work: t.TempDir(), Cache: cache}
	dir, err := ExtractRuntime(cache)
	if err != nil {
		t.Fatal(err)
	}
	srcs := slices.Concat(rtfiles.Sources(goos), rtfiles.TestingSources(goos))
	objs, err := b.cachedObjects(context.Background(), dir, srcs, b.compileFlags())
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range objs {
		if ownDataOnly[filepath.Base(o)] {
			continue
		}
		names, err := sections(o)
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range names {
			for _, bad := range staticInitSections {
				if strings.HasPrefix(name, bad) {
					t.Errorf("%s has the section %s: a static initializer in a library member",
						filepath.Base(o), name)
				}
			}
		}
	}
}
