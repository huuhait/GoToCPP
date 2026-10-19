package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeCache makes a cache folder (the XDG_CACHE_HOME of the test) with something in each folder.
func fakeCache(t *testing.T) string {
	t.Helper()
	xdg := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", xdg)
	root := filepath.Join(xdg, "gocpp")
	for _, f := range []string{"obj/a.o", "obj/b.man", "rt/x/y.o", "gen/prog1/gpkg/a.cpp", "gen/prog1.lock", "gen/prog2/gpkg/b.cpp", "gen/prog2.lock",
		"stamp/k.json", "cxxid/c.json", "probe/p", "mod/example.com/m@v1.0.0/x.gp", "gocpp-keep"} {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(strings.Repeat("x", 100)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func TestClean(t *testing.T) {
	root := fakeCache(t)
	in := func(rel string) string { return filepath.Join(root, filepath.FromSlash(rel)) }

	// No flags: nothing is removed.
	if code, out, errOut := gocpp("clean"); code != 0 || out != "" || errOut != "" || !exists(in("obj/a.o")) {
		t.Fatalf("gocpp clean = %d %q %q", code, out, errOut)
	}
	if code, _, errOut := gocpp("clean", "pkg"); code != 2 || !strings.Contains(errOut, "unexpected argument") {
		t.Errorf("gocpp clean pkg = %d %q", code, errOut)
	}

	// -n says what it would remove, with the sizes, and removes nothing.
	code, out, _ := gocpp("clean", "-cache", "-n")
	if code != 0 || !exists(in("obj/a.o")) || !exists(in("stamp/k.json")) {
		t.Fatalf("gocpp clean -cache -n = %d %q, and the files must stay", code, out)
	}
	for _, want := range []string{
		"rm -rf " + in("obj") + " # 2 files, 200 B\n",
		"rm -rf " + in("rt") + " # 1 files, 100 B\n",
		"rm -rf " + in("gen") + " # 4 files, 400 B\n",
		"rm -rf " + in("stamp") + " # 1 files, 100 B\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("gocpp clean -cache -n lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "mod") {
		t.Errorf("gocpp clean -cache -n names the module cache:\n%s", out)
	}

	// -stamp: only the records of finished builds.
	if code, _, _ := gocpp("clean", "-stamp"); code != 0 || exists(in("stamp")) || !exists(in("obj/a.o")) || !exists(in("gen/prog1/gpkg/a.cpp")) {
		t.Errorf("gocpp clean -stamp removed the wrong things")
	}

	// -gen: a folder whose lock is held is the folder of a build that runs, and stays.
	l, busy, err := lockFile(in("gen/prog1.lock"))
	if err != nil || busy {
		t.Skipf("no file locks here: %v", err)
	}
	if code, _, errOut := gocpp("clean", "-gen"); code != 0 || errOut != "" {
		t.Errorf("gocpp clean -gen = %d %q", code, errOut)
	}
	if !exists(in("gen/prog1/gpkg/a.cpp")) || exists(in("gen/prog2")) || !exists(in("obj/a.o")) {
		t.Errorf("gocpp clean -gen did not keep exactly the folder that is in use")
	}
	l.unlock()
	if code, _, _ := gocpp("clean", "-gen"); code != 0 || exists(in("gen/prog1")) {
		t.Errorf("gocpp clean -gen did not remove the folder that is not in use any more")
	}

	// -cache: all but the module cache and the others.
	if code, _, errOut := gocpp("clean", "-cache"); code != 0 || errOut != "" {
		t.Fatalf("gocpp clean -cache = %d %q", code, errOut)
	}
	for _, f := range []string{"obj", "rt", "gen", "stamp", "cxxid", "probe"} {
		if exists(in(f)) && f != "gen" {
			t.Errorf("gocpp clean -cache left %s", f)
		}
	}
	if !exists(in("mod/example.com/m@v1.0.0/x.gp")) || !exists(in("gocpp-keep")) {
		t.Errorf("gocpp clean -cache removed the module cache or a file that is not its own")
	}

	// -modcache removes the module cache, also its read-only folders.
	mod := in("mod")
	os.Chmod(filepath.Join(mod, "example.com", "m@v1.0.0"), 0o555)
	t.Setenv("GOCPP_MODCACHE", mod)
	if code, out, errOut := gocpp("clean", "-modcache", "-n"); code != 0 || !strings.Contains(out, "rm -rf "+mod) || !exists(mod) {
		t.Errorf("gocpp clean -modcache -n = %d %q %q", code, out, errOut)
	}
	if code, _, errOut := gocpp("clean", "-modcache"); code != 0 || exists(filepath.Join(mod, "example.com")) {
		t.Errorf("gocpp clean -modcache = %d %q", code, errOut)
	}
	if !exists(in("gocpp-keep")) {
		t.Errorf("-modcache removed a file that is not in the module cache")
	}
}

// After gocpp clean -cache a build makes everything again, and gives the same program.
func TestCleanThenBuild(t *testing.T) {
	withOneCXX(t, func(t *testing.T, c testCXX) {
		t.Setenv("XDG_CACHE_HOME", t.TempDir()) // a cache of its own: the runtime is built again here
		cacheOn(t)
		t.Setenv("GOCPP_NOUPTODATE", "") // the second build must be up to date
		inDir(t, map[string]string{"hello.gp": helloSrc})
		build := func() (result, counts) {
			r := gocppRun("", false, "build", "-x", "-o", "hello", "hello.gp")
			if r.code != 0 {
				t.Fatalf("gocpp build = %+v", r)
			}
			if strings.Contains(r.stderr, "WORK=") {
				workDir(t, r.stderr)
			}
			return r, actionCounts(t, r.stderr)
		}
		_, n1 := build()
		if n1.compile == 0 || n1.runtime == 0 || n1.link != 1 {
			t.Errorf("the first build did %+v, want the compiles, the runtime and the link", n1)
		}
		if r, n := build(); !isUpToDate(r.stderr) || n != (counts{}) {
			t.Errorf("the second build did %+v:\n%s", n, r.stderr)
		}
		if code, _, errOut := gocpp("clean", "-cache"); code != 0 {
			t.Fatalf("gocpp clean -cache = %d %q", code, errOut)
		}
		r, n := build()
		if isUpToDate(r.stderr) || n.compile == 0 || n.runtime == 0 || n.hit != 0 || n.link != 1 {
			t.Errorf("the build after gocpp clean -cache did %+v, want everything again:\n%s", n, r.stderr)
		}
		out, err := exec.Command("./hello").Output()
		if err != nil || !strings.HasPrefix(string(out), "hello, gocpp 12 true") {
			t.Errorf("the program prints %q, %v", out, err)
		}
	})
}

// A folder of the cache that is a symbolic link is removed as a link: nothing of what it points
// at is read or removed (the folder of the generated files is the one that is listed entry by entry).
func TestCleanSymlinkedFolders(t *testing.T) {
	root := fakeCache(t)
	canary := t.TempDir()
	for _, f := range []string{"gen/x/y.cpp", "obj/a.o", "stamp/s.json"} {
		p := filepath.Join(canary, filepath.FromSlash(f))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte("canary"), 0o644)
	}
	for _, f := range []string{"gen", "obj", "stamp"} {
		os.RemoveAll(filepath.Join(root, f))
		if err := os.Symlink(filepath.Join(canary, f), filepath.Join(root, f)); err != nil {
			t.Skip(err)
		}
	}
	if code, out, _ := gocpp("clean", "-cache", "-n"); code != 0 || strings.Contains(out, "canary") || !exists(filepath.Join(canary, "gen", "x", "y.cpp")) {
		t.Fatalf("gocpp clean -cache -n = %d %q", code, out)
	}
	if code, _, errOut := gocpp("clean", "-cache"); code != 0 || errOut != "" {
		t.Fatalf("gocpp clean -cache = %d %q", code, errOut)
	}
	for _, f := range []string{"gen/x/y.cpp", "obj/a.o", "stamp/s.json"} {
		if !exists(filepath.Join(canary, filepath.FromSlash(f))) {
			t.Errorf("gocpp clean removed %s, which is outside the cache (a link points at it)", f)
		}
	}
	for _, f := range []string{"gen", "obj", "stamp"} {
		if _, err := os.Lstat(filepath.Join(root, f)); err == nil {
			t.Errorf("the link %s is still in the cache", f)
		}
	}
}
