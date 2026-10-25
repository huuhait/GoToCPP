package rtfiles_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The vendored bdwgc (phase 16 README §2.4): the embedded tree, the build of the smoke program
// through internal/build's API (build.Builder.Bdwgc) with both compilers and the sanitizers, and
// the opt-in rule (a program that does not ask for bdwgc links no bdwgc symbol).

// TestBdwgcFS checks that the embedded bdwgc holds every file of the vendored tree on disk, and
// that no name starts with "." or "_" (//go:embed skips such names: a source that started with
// one would silently not be embedded).
func TestBdwgcFS(t *testing.T) {
	t.Parallel()
	var disk []string
	root := "third_party/bdwgc"
	err := fs.WalkDir(os.DirFS("."), root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != root && (strings.HasPrefix(d.Name(), ".") || strings.HasPrefix(d.Name(), "_")) {
			t.Errorf("%s starts with %q: //go:embed skips it", p, d.Name()[:1])
		}
		if !d.IsDir() {
			disk = append(disk, strings.TrimPrefix(p, root+"/"))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	var embedded []string
	err = fs.WalkDir(rtfiles.BdwgcFS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		embedded = append(embedded, p)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(disk)
	slices.Sort(embedded)
	if !slices.Equal(disk, embedded) {
		t.Errorf("the embedded bdwgc holds\n\t%s\nwant the files of %s\n\t%s", strings.Join(embedded, "\n\t"), root, strings.Join(disk, "\n\t"))
	}
	if got, want := rtfiles.BdwgcVersion(), "8.2.12"; got != want {
		t.Errorf("BdwgcVersion() = %q, want %q", got, want)
	}
	linux := rtfiles.BdwgcSources("linux")
	if len(linux) != 27 || !slices.IsSorted(linux) {
		t.Errorf(`BdwgcSources("linux") = %q, want the 27 sorted files of the Linux build`, linux)
	}
	for _, s := range linux {
		if _, err := fs.Stat(rtfiles.BdwgcFS(), s); err != nil {
			t.Errorf(`BdwgcSources("linux") has %s, which is not in BdwgcFS: %v`, s, err)
		}
	}
	if slices.Contains(linux, "src/windows/win32_threads.c") || !slices.Contains(linux, "src/pthread_support.c") {
		t.Errorf(`BdwgcSources("linux") = %q, want the POSIX thread layer and not the Windows source`, linux)
	}
	// The Windows list is bdwgc's CMake Win32-threads branch (M10), 23 files: the Linux list
	// without the five POSIX-thread sources (gc_dlopen.c, pthread_start.c, pthread_stop_world.c,
	// pthread_support.c, specific.c), plus src/windows/win32_threads.c. Pin it exactly: a
	// regression that kept one of the pthread sources would otherwise pass.
	wantWindows := []string{
		"src/allchblk.c", "src/alloc.c", "src/blacklst.c", "src/dbg_mlc.c", "src/dyn_load.c",
		"src/finalize.c", "src/fnlz_mlc.c", "src/gcj_mlc.c", "src/headers.c", "src/mach_dep.c",
		"src/malloc.c", "src/mallocx.c", "src/mark.c", "src/mark_rts.c", "src/misc.c",
		"src/new_hblk.c", "src/obj_map.c", "src/os_dep.c", "src/ptr_chck.c", "src/reclaim.c",
		"src/thread_local_alloc.c", "src/typd_mlc.c", "src/windows/win32_threads.c",
	}
	windows := rtfiles.BdwgcSources("windows")
	if !slices.Equal(windows, wantWindows) {
		t.Errorf(`BdwgcSources("windows") = %q, want bdwgc's Win32-threads branch %q`, windows, wantWindows)
	}
	for _, s := range windows {
		if _, err := fs.Stat(rtfiles.BdwgcFS(), s); err != nil {
			t.Errorf(`BdwgcSources("windows") has %s, which is not in BdwgcFS: %v`, s, err)
		}
	}
	for _, want := range []string{"src/alloc.c", "src/finalize.c", "src/pthread_support.c", "src/windows/win32_threads.c", "include/gc.h", "LICENSE", "README.md", "VERSION", "README.gocpp"} {
		if _, err := fs.Stat(rtfiles.BdwgcFS(), want); err != nil {
			t.Errorf("the embedded bdwgc has no %s: %v", want, err)
		}
	}
}

// bdwgcModes are the builds of the smoke program: the driver's debug and optimized builds, the
// sanitizers (the runtime's asan mode, address and undefined), and ThreadSanitizer. TestCXX also
// builds the program in its O0, O2 and asan modes; this test adds TSan and checks the build path
// of internal/build itself.
var bdwgcModes = []struct {
	name  string
	flags []string
}{
	{"O0", []string{"-O0"}},
	{"O2", []string{"-O2"}},
	{"asan", []string{"-O0", "-fsanitize=address,undefined", "-fno-omit-frame-pointer", "-fno-sanitize-recover=all", "-gdwarf-4"}},
	{"tsan", []string{"-O1", "-fsanitize=thread", "-fno-omit-frame-pointer"}},
}

// TestBdwgcSmoke builds and runs runtime/test/bdwgc_test.cpp with g++ and clang++ through
// build.Builder.Bdwgc (the build path of the driver), at -O0, -O2, under ASan and under TSan. The
// program checks threads, registration, allocation, blocking sections, a collection after the
// threads exit, finalizer order and a weak link, and prints "OK" last.
func TestBdwgcSmoke(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("bdwgc is vendored for linux (phase 16 M10 adds Windows)")
	}
	rt := extractRuntime(t)
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			for _, m := range bdwgcModes {
				t.Run(m.name, func(t *testing.T) {
					t.Parallel()
					bins := c.startBuilds(t, rt, t.TempDir(), m.flags,
						[]prog{{src: "test/bdwgc_test.cpp", bdwgc: true, weight: 4}})
					runBdwgc(t, bins.bin(t, "test/bdwgc_test.cpp"))
				})
			}
		})
	}
}

// runBdwgc runs the smoke program: exit code 0, nothing on stderr, and "OK" as the last line of
// stdout.
func runBdwgc(t *testing.T, bin string) {
	t.Helper()
	stdout, stderr, code, err := command(filepath.Dir(bin), nil, false, bin)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(stdout)), "\n")
	ok := code == 0 && len(stderr) == 0 && len(lines) > 0 && lines[len(lines)-1] == "OK"
	if !ok {
		t.Fatalf("test/bdwgc_test.cpp: exit code %d\nstdout:\n%s\nstderr:\n%s\nwant exit code 0, nothing on stderr, and OK last", code, stdout, stderr)
	}
	t.Logf("%s", strings.TrimSpace(string(stdout)))
}

// TestBdwgcOptIn checks that a program that does not ask for bdwgc links no bdwgc symbol: the
// smoke program's object, linked with the runtime but without the bdwgc archive, has an undefined
// reference to GC_INIT or GC_malloc. (The successful link, with the archive, is TestBdwgcSmoke.)
func TestBdwgcOptIn(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("bdwgc is vendored for linux (phase 16 M10 adds Windows)")
	}
	rt := extractRuntime(t)
	for _, c := range compilers(t) {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			// The library is not linked; its compile flags are what gc.h needs.
			lib, err := c.bdwgcLibrary(rt, []string{"-O0"})
			if err != nil {
				t.Fatal(err)
			}
			objs, err := c.runtimeObjects(rt, []string{"-O0"})
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			obj := filepath.Join(dir, "bdwgc_test.o")
			args := append(c.flags([]string{"-O0"}), "-I", filepath.Join(rt, "include"), "-c", sourcePath(t, "test/bdwgc_test.cpp"), "-o", obj)
			args = append(args, lib.CompileFlags...)
			if err := c.compileCmd(prioRun, dir, nil, args); err != nil {
				t.Fatal(err)
			}
			bin := filepath.Join(dir, "bdwgc_test"+c.exe())
			linkArgs := append(c.flags([]string{"-O0"}), obj)
			linkArgs = append(linkArgs, objs...)
			linkArgs = append(linkArgs, "-o", bin)
			out, _, code, err := command(dir, build.CompilerEnv(), true, c.cc.Path, linkArgs...)
			if err != nil {
				t.Fatal(err)
			}
			if code == 0 {
				t.Fatalf("the smoke program linked without bdwgc: a program that does not ask for it must link no bdwgc symbol")
			}
			if !bytes.Contains(out, []byte("GC_INIT")) && !bytes.Contains(out, []byte("GC_malloc")) {
				t.Fatalf("the link without bdwgc failed, but not on a bdwgc symbol:\n%s", out)
			}
		})
	}
}

// TestBdwgcConfig checks the parts of the build configuration that the driver will share: the
// compile flags of a user of gc.h and the link flags. The fixed defines and the cache key are
// tested in internal/build.
func TestBdwgcConfig(t *testing.T) {
	t.Parallel()
	if runtime.GOOS != "linux" {
		t.Skip("bdwgc is vendored for linux")
	}
	c := cxx{name: "host"}
	cc, err := build.LookCXX(context.Background(), "g++")
	if err != nil {
		t.Skipf("no g++: %v", err)
	}
	c.cc = cc
	rt := extractRuntime(t)
	lib, err := c.bdwgcLibrary(rt, []string{"-O0"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(lib.CompileFlags, "-DGC_THREADS") || !slices.Contains(lib.CompileFlags, "-DGC_NO_THREAD_REDIRECTS") || !slices.Contains(lib.CompileFlags, "-I") {
		t.Errorf("CompileFlags = %q, want -I<dir>/include, -DGC_THREADS and -DGC_NO_THREAD_REDIRECTS", lib.CompileFlags)
	}
	if !slices.Contains(lib.LinkFlags, "-lpthread") || !slices.Contains(lib.LinkFlags, "-ldl") {
		t.Errorf("LinkFlags = %q, want -lpthread and -ldl", lib.LinkFlags)
	}
	if !filepath.IsAbs(lib.Archive) {
		t.Errorf("Archive = %q, want an absolute path", lib.Archive)
	}
}
