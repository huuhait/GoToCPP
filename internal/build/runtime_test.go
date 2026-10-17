package build

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	rtfiles "github.com/huuhait/gocpp/runtime"
)

func TestExtractRuntime(t *testing.T) {
	cache := t.TempDir()
	// Many at the same time, like parallel gocpp runs: all get the same
	// complete folder, and no temporary folder is left.
	const n = 8
	dirs := make([]string, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { dirs[i], errs[i] = ExtractRuntime(cache) })
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if dirs[i] != dirs[0] {
			t.Errorf("ExtractRuntime returned %s and %s", dirs[0], dirs[i])
		}
	}
	dir := dirs[0]
	if want := filepath.Join(cache, "rt", runtimeName()); dir != want {
		t.Errorf("ExtractRuntime = %s, want %s", dir, want)
	}
	if !strings.HasPrefix(filepath.Base(dir), "dev-") || len(filepath.Base(dir)) != len("dev-")+16 {
		t.Errorf("the runtime folder %s is not named <version>-<hash>", filepath.Base(dir))
	}
	if err := checkRuntime(dir); err != nil {
		t.Fatal(err)
	}
	for _, s := range rtfiles.Sources(runtime.GOOS) {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(s))); err != nil {
			t.Error(err)
		}
	}
	assertNoTemp(t, filepath.Join(cache, "rt"))

	// A changed file (another size) and a removed one are noticed, and the
	// folder is replaced.
	for _, breakIt := range []func(string) error{
		func(p string) error { return os.WriteFile(p, []byte("#error broken\n"), 0o644) },
		os.Remove,
	} {
		rt := filepath.Join(dir, "include", "gocpp", "rt.hpp")
		if err := breakIt(rt); err != nil {
			t.Fatal(err)
		}
		if checkRuntime(dir) == nil {
			t.Fatal("checkRuntime doesn't notice the broken file")
		}
		got, err := ExtractRuntime(cache)
		if err != nil || got != dir {
			t.Fatalf("ExtractRuntime = %s, %v; want %s", got, err, dir)
		}
		if err := checkRuntime(dir); err != nil {
			t.Errorf("the folder is not replaced: %v", err)
		}
		assertNoTemp(t, filepath.Join(cache, "rt"))
	}
}

// assertNoTemp fails if the folder has a temporary folder (.tmp-, .old-).
func assertNoTemp(t *testing.T, dir string) {
	t.Helper()
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Errorf("%s has the temporary folder %s left", dir, e.Name())
		}
	}
}

func TestShellQuoteWork(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"/w/x.cpp", "$WORK/x.cpp"},
		{"/w", "$WORK"},
		{"/wx/a", "/wx/a"},
		{"-O0", "-O0"},
		{"a b", "'a b'"},
		{"it's", `'it'\''s'`},
		{"", "''"},
		{"/w/a b.cpp", "$WORK'/a b.cpp'"},
	} {
		if got := ShellQuoteWork(c.in, "/w"); got != c.want {
			t.Errorf("ShellQuoteWork(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestProgramParallel(t *testing.T) {
	// Builds with one compiler at the same time share the runtime's object
	// files: one folder in obj/, no temporary folder left, and every
	// program works.
	cxx, err := LookCXX(context.Background(), "g++")
	if err != nil {
		if cxx, err = LookCXX(context.Background(), "clang++"); err != nil {
			t.Skip("no C++ compiler:", err)
		}
	}
	cache := t.TempDir()
	const n = 4
	errs := make([]error, n)
	bins := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		work := t.TempDir()
		src := filepath.Join(work, "p.cpp")
		if err := os.WriteFile(src, []byte("#include <gocpp/rt.hpp>\n#include <cstdio>\nint main() { std::puts(\"ok\"); }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		bins[i] = filepath.Join(work, "p")
		b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Stderr: os.Stderr}
		wg.Go(func() { errs[i] = b.Program(context.Background(), src, bins[i]) })
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil {
			var ce *CompileError
			if errors.As(errs[i], &ce) {
				t.Fatalf("%v\n%s\n%s", ce, ce.Command, ce.Output)
			}
			t.Fatal(errs[i])
		}
		if out, err := exec.Command(bins[i]).Output(); err != nil || string(out) != "ok\n" {
			t.Errorf("%s: %q, %v", bins[i], out, err)
		}
	}
	dir, _ := ExtractRuntime(cache)
	objs, _ := os.ReadDir(filepath.Join(dir, "obj"))
	if len(objs) != 1 || strings.HasPrefix(objs[0].Name(), ".") {
		var names []string
		for _, o := range objs {
			names = append(names, o.Name())
		}
		t.Errorf("obj/ holds %q, want one folder", names)
	}
}

func TestProgramNoChecks(t *testing.T) {
	// -nochecks defines GOCPP_NOCHECKS for the program's compile and the
	// runtime's, not for the link, and its runtime objects are cached apart.
	cxx, err := LookCXX(context.Background(), "g++")
	if err != nil {
		if cxx, err = LookCXX(context.Background(), "clang++"); err != nil {
			t.Skip("no C++ compiler:", err)
		}
	}
	cache := t.TempDir()
	const prog = "#include <gocpp/rt.hpp>\n#include <cstdio>\nint main() {\n" +
		"    try { gp::check_index(INT64_C(5), 3); std::puts(\"unchecked\"); }\n" +
		"    catch (const gp::panic_error&) { std::puts(\"checked\"); }\n}\n"
	for _, c := range []struct {
		nochecks bool
		want     string
	}{{false, "checked\n"}, {true, "unchecked\n"}} {
		work := t.TempDir()
		src := filepath.Join(work, "p.cpp")
		if err := os.WriteFile(src, []byte(prog), 0o644); err != nil {
			t.Fatal(err)
		}
		var trace strings.Builder
		b := &Builder{CXX: cxx, NoChecks: c.nochecks, GOOS: runtime.GOOS, Work: work, Cache: cache, Trace: &trace, Stderr: os.Stderr}
		bin := filepath.Join(work, "p")
		if err := b.Program(context.Background(), src, bin); err != nil {
			var ce *CompileError
			if errors.As(err, &ce) {
				t.Fatalf("%v\n%s\n%s", ce, ce.Command, ce.Output)
			}
			t.Fatal(err)
		}
		if out, err := exec.Command(bin).Output(); err != nil || string(out) != c.want {
			t.Errorf("nochecks=%v: the program prints %q, %v; want %q", c.nochecks, out, err, c.want)
		}
		// Each compile (the runtime's, as the cache is new for this setting,
		// and the program's) has the define, and the link doesn't.
		compiles := 0
		for _, line := range strings.Split(strings.TrimSpace(trace.String()), "\n") {
			if !strings.HasPrefix(line, cxx.Path+" ") {
				continue
			}
			has := strings.Contains(line, " -DGOCPP_NOCHECKS ")
			if strings.Contains(line, " -c ") {
				compiles++
				if has != c.nochecks {
					t.Errorf("nochecks=%v: a compile with -DGOCPP_NOCHECKS=%v: %s", c.nochecks, has, line)
				}
			} else if has {
				t.Errorf("the link has -DGOCPP_NOCHECKS: %s", line)
			}
		}
		if want := len(rtfiles.Sources(runtime.GOOS)) + 1; compiles != want {
			t.Errorf("nochecks=%v: -x shows %d compiles, want %d (the runtime's and the program's):\n%s", c.nochecks, compiles, want, trace.String())
		}
	}
	dir, _ := ExtractRuntime(cache)
	if objs, _ := os.ReadDir(filepath.Join(dir, "obj")); len(objs) != 2 {
		t.Errorf("obj/ holds %d folders, want 2: the runtime with and without -nochecks", len(objs))
	}
}

func TestRemoveStale(t *testing.T) {
	// A killed gocpp leaves its temporary folders: those older than an
	// hour go when the next one is made; younger ones may be in use.
	cache := t.TempDir()
	root := filepath.Join(cache, "rt")
	old := []string{".tmp-1", ".old-2"}
	keep := []string{".tmp-young", "other"}
	for _, n := range append(old, keep...) {
		if err := os.MkdirAll(filepath.Join(root, n, "x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	long := time.Now().Add(-2 * staleAge)
	for _, n := range append(old, "other") {
		if err := os.Chtimes(filepath.Join(root, n), long, long); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ExtractRuntime(cache); err != nil {
		t.Fatal(err)
	}
	for _, n := range old {
		if _, err := os.Stat(filepath.Join(root, n)); err == nil {
			t.Errorf("the stale %s is still there", n)
		}
	}
	for _, n := range keep {
		if _, err := os.Stat(filepath.Join(root, n)); err != nil {
			t.Errorf("%s was removed: %v", n, err)
		}
	}
}

func TestBrokenObjects(t *testing.T) {
	// A folder of runtime objects with a file missing is replaced.
	cxx, err := LookCXX(context.Background(), "g++")
	if err != nil {
		if cxx, err = LookCXX(context.Background(), "clang++"); err != nil {
			t.Skip("no C++ compiler:", err)
		}
	}
	cache := t.TempDir()
	work := t.TempDir()
	var trace strings.Builder
	b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Trace: &trace}
	flags := Flags(cxx, false)
	dir, objs, err := b.runtime(context.Background(), flags)
	if err != nil {
		t.Fatal(err)
	}
	// -x shows the temporary folder being made and moved into place.
	if !strings.Contains(trace.String(), "mkdir -p "+filepath.Join(dir, "obj", ".tmp-")) || !strings.Contains(trace.String(), "mv ") {
		t.Errorf("-x doesn't show mkdir and mv:\n%s", trace.String())
	}
	if err := os.Remove(objs[0]); err != nil {
		t.Fatal(err)
	}
	_, again, err := b.runtime(context.Background(), flags)
	if err != nil || !allExist(again) || again[0] != objs[0] {
		t.Fatalf("runtime after a missing object file = %q, %v", again, err)
	}
	assertNoTemp(t, filepath.Join(dir, "obj"))
}

// TestKeyEnv: the variables that the keys of the compiler's results hold.
func TestKeyEnv(t *testing.T) {
	for _, name := range []string{
		"CPATH", "CPLUS_INCLUDE_PATH", "C_INCLUDE_PATH", "GCC_EXEC_PREFIX", "COMPILER_PATH",
		"SDKROOT", "DEVELOPER_DIR", "CCC_ADD_ARGS", "CCC_OVERRIDE_OPTIONS",
		"MACOSX_DEPLOYMENT_TARGET", "IPHONEOS_DEPLOYMENT_TARGET", "TVOS_DEPLOYMENT_TARGET", "WATCHOS_DEPLOYMENT_TARGET",
		"NIX_CFLAGS_COMPILE", "NIX_CFLAGS_COMPILE_FOR_TARGET", "NIX_HARDENING_ENABLE", "NIX_HARDENING_ENABLE_x86_64_unknown_linux_gnu",
		"NIX_CXXSTDLIB_COMPILE", "NIX_ENFORCE_NO_NATIVE", "NIX_ENFORCE_PURITY", "NIX_CC_WRAPPER_FLAGS_SET_x", "NIX_BINTOOLS_WRAPPER_FLAGS_SET_x",
	} {
		if !keyEnv(name) {
			t.Errorf("keyEnv(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"PATH", "HOME", "TMPDIR", "LC_ALL", "NIX_PATH", "NIX_PROFILES", "NIX_LD", "GOCPP_CXX", "PWD"} {
		if keyEnv(name) {
			t.Errorf("keyEnv(%q) = true, want false", name)
		}
	}
	t.Setenv("NIX_ENFORCE_NO_NATIVE", "1")
	t.Setenv("GOCPP_UNRELATED_VARIABLE", "1")
	env := KeyEnv()
	var got []string
	for _, kv := range env {
		if strings.HasPrefix(kv, "NIX_ENFORCE_NO_NATIVE=") || strings.HasPrefix(kv, "GOCPP_UNRELATED") {
			got = append(got, kv)
		}
	}
	if len(got) != 1 || got[0] != "NIX_ENFORCE_NO_NATIVE=1" || !sort.StringsAreSorted(env) {
		t.Errorf("KeyEnv() = %v (sorted: %v), want NIX_ENFORCE_NO_NATIVE=1 and not the unrelated variable", env, sort.StringsAreSorted(env))
	}
}
