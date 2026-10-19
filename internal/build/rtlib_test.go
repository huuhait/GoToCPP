package build

import (
	"context"
	"errors"
	"fmt"
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

// Tests of the runtime library (rtlib.go, Phase 06 step 6.8) with a fake compiler and a fake
// archiver that record their command lines: what is compiled and archived when, and which
// changes make another library.

func skipWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake tools are shell scripts")
	}
}

// fakeCompiler writes a compiler that logs its arguments (one line each) to <path>.log and
// writes its output file; extra is shell code that runs before it exits (it may use "$out").
func fakeCompiler(t *testing.T, family Family, extra string) CXX {
	t.Helper()
	skipWindows(t)
	path := script(t, `echo "$*" >> "$0.log"
out=""; prev=""
for a in "$@"; do if [ "$prev" = "-o" ]; then out="$a"; fi; prev="$a"; done
`+extra+`
if [ -n "$out" ]; then echo obj > "$out"; fi
exit 0
`)
	return CXX{Path: path, Family: family, Version: "15"}
}

// fakeArchiver writes an archiver that logs its arguments to <path>.log and writes a library
// (the archive magic, then the members' text) to its second argument; extra runs first.
func fakeArchiver(t *testing.T, extra string) string {
	t.Helper()
	skipWindows(t)
	ar, err := exec.LookPath("ar")
	if err != nil {
		t.Skip("no ar:", err)
	}
	return script(t, `echo "$*" >> "$0.log"
`+extra+`
exec `+shellQuote(ar)+` "$@"
`)
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path + ".log")
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

// rtEnv is a cache with a fake compiler and archiver.
type rtEnv struct {
	t     *testing.T
	cache string
	cxx   CXX
	ar    string
	trace *syncBuffer
}

func newRTEnv(t *testing.T) *rtEnv {
	t.Helper()
	skipWindows(t)
	t.Setenv("GOCPP_NOPCH", "1")
	t.Setenv("GOCPP_NORTLIB", "")
	e := &rtEnv{t: t, cache: t.TempDir(), cxx: fakeCompiler(t, GCC, ""), ar: fakeArchiver(t, ""), trace: &syncBuffer{}}
	t.Setenv("GOCPP_AR", e.ar)
	return e
}

func (e *rtEnv) builder(f func(b *Builder)) *Builder {
	b := &Builder{CXX: e.cxx, GOOS: "linux", Work: e.t.TempDir(), Cache: e.cache, Trace: e.trace}
	if f != nil {
		f(b)
	}
	return b
}

// runtime asks for the runtime of a build with the flags of the build.
func (e *rtEnv) runtime(b *Builder) (dir string, objs []string, err error) {
	return b.runtime(context.Background(), b.runtimeFlags(), false)
}

func nSources() int { return len(rtfiles.Sources("linux")) }

func (e *rtEnv) libFolders() []string {
	dir, _ := ExtractRuntime(e.cache)
	return libFolders(e.t, dir)
}

func TestRuntimeLibBuiltOnceThenReused(t *testing.T) {
	e := newRTEnv(t)
	_, objs, err := e.runtime(e.builder(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 1 || filepath.Base(objs[0]) != rtLibName || !libOK(objs[0]) {
		t.Fatalf("runtime() = %q, want the one library", objs)
	}
	if got := len(lines(t, e.cxx.Path)); got != nSources() {
		t.Errorf("the first build compiled %d files, want %d (one per runtime source)", got, nSources())
	}
	if got := len(lines(t, e.ar)); got != 1 {
		t.Errorf("the archiver ran %d times, want 1", got)
	}
	// The archive command: flags, library, then every object in the order of the sources.
	ar := strings.Fields(lines(t, e.ar)[0])
	if ar[0] != "rcsD" || len(ar) != 2+nSources() {
		t.Errorf("archive command %q", ar)
	}
	for i, src := range rtfiles.Sources("linux") {
		if filepath.Base(ar[2+i]) != runtimeObjectName(src) {
			t.Errorf("member %d is %s, want %s: the order of the members must not depend on timing", i, filepath.Base(ar[2+i]), runtimeObjectName(src))
		}
	}
	// A second build, also with another Builder (another process): nothing runs.
	_, again, err := e.runtime(e.builder(nil))
	if err != nil || !slices.Equal(again, objs) {
		t.Fatalf("second runtime() = %q, %v; want %q", again, err, objs)
	}
	if got := len(lines(t, e.cxx.Path)); got != nSources() {
		t.Errorf("a warm cache compiled again: %d compiles", got)
	}
	if got := len(lines(t, e.ar)); got != 1 {
		t.Errorf("a warm cache archived again: %d runs", got)
	}
	assertNoTemp(t, filepath.Join(filepath.Dir(filepath.Dir(objs[0])))) // lib/
}

func TestRuntimeLibKeyedBy(t *testing.T) {
	// Each change makes another library (and the old one stays): the flags, the sanitizer,
	// -nochecks, LTO, the compiler, the archiver, the environment that the compiler reads.
	for _, c := range []struct {
		name string
		f    func(t *testing.T, e *rtEnv) *Builder
	}{
		{"asan", func(t *testing.T, e *rtEnv) *Builder { return e.builder(func(b *Builder) { b.ASan = true }) }},
		{"nochecks", func(t *testing.T, e *rtEnv) *Builder { return e.builder(func(b *Builder) { b.NoChecks = true }) }},
		{"lto", func(t *testing.T, e *rtEnv) *Builder { return e.builder(func(b *Builder) { b.LTO = true }) }},
		{"environment flags", func(t *testing.T, e *rtEnv) *Builder {
			return e.builder(func(b *Builder) { b.Env = EnvFlags{CXXFlags: []string{"-O2"}} })
		}},
		{"another compiler", func(t *testing.T, e *rtEnv) *Builder {
			other := fakeCompiler(t, GCC, "")
			return e.builder(func(b *Builder) { b.CXX = other })
		}},
		{"another compiler family", func(t *testing.T, e *rtEnv) *Builder {
			c := e.cxx
			c.Family = Clang
			return e.builder(func(b *Builder) { b.CXX = c })
		}},
		{"another compiler version", func(t *testing.T, e *rtEnv) *Builder {
			c := e.cxx
			c.Version = "16"
			return e.builder(func(b *Builder) { b.CXX = c })
		}},
		{"another archiver", func(t *testing.T, e *rtEnv) *Builder {
			t.Setenv("GOCPP_AR", fakeArchiver(t, "# another"))
			return e.builder(nil)
		}},
		{"archiver changed in place", func(t *testing.T, e *rtEnv) *Builder {
			if err := os.WriteFile(e.ar, []byte("#!/bin/sh\n# a new version of the archiver\n"+readText(t, e.ar)[len("#!/bin/sh\n"):]), 0o755); err != nil {
				t.Fatal(err)
			}
			return e.builder(nil)
		}},
		{"include path variable", func(t *testing.T, e *rtEnv) *Builder {
			t.Setenv("CPLUS_INCLUDE_PATH", "/somewhere")
			return e.builder(nil)
		}},
		{"nix flags variable", func(t *testing.T, e *rtEnv) *Builder {
			t.Setenv("NIX_CFLAGS_COMPILE", "-DX")
			return e.builder(nil)
		}},
		{"target system", func(t *testing.T, e *rtEnv) *Builder {
			return e.builder(func(b *Builder) { b.GOOS = "darwin" })
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := newRTEnv(t)
			if _, _, err := e.runtime(e.builder(nil)); err != nil {
				t.Fatal(err)
			}
			b := c.f(t, e)
			srcs := len(rtfiles.Sources(b.GOOS))
			before := len(lines(t, b.CXX.Path))
			if _, _, err := e.runtime(b); err != nil {
				t.Fatal(err)
			}
			if got := e.libFolders(); len(got) != 2 {
				t.Errorf("lib/ holds %d folders after the change, want 2 (old and new): %q", len(got), got)
			}
			if got := len(lines(t, b.CXX.Path)) - before; got != srcs {
				t.Errorf("the change compiled %d files, want %d", got, srcs)
			}
			// Both builds are warm now.
			compiles := len(lines(t, b.CXX.Path))
			if _, _, err := e.runtime(b); err != nil {
				t.Fatal(err)
			}
			if got := len(lines(t, b.CXX.Path)); got != compiles {
				t.Errorf("the changed build is not cached: %d more compiles", got-compiles)
			}
		})
	}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestRuntimeLibSourcesInKey(t *testing.T) {
	// A change of a runtime source: the folder of the runtime itself is named by the hash of its
	// files (runtimeName); and a library is looked for under the sources that it was made
	// from.
	e := newRTEnv(t)
	b := e.builder(nil)
	a, err := b.rtArchiver(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	srcs := rtfiles.Sources("linux")
	k1 := b.runtimeLibKey(srcs, b.runtimeFlags(), a)
	k2 := b.runtimeLibKey(srcs[1:], b.runtimeFlags(), a)
	k3 := b.runtimeLibKey(srcs, b.runtimeFlags(), a)
	if k1 == k2 || k1 != k3 {
		t.Errorf("keys: %s %s %s; want the first two different and the first and third equal", k1, k2, k3)
	}
	// and not the same key as the runtime's object folder of the same flags
	if k1 == b.runtimeKey(srcs, b.runtimeFlags()) {
		t.Error("the library and the object folder share a key")
	}
}

func TestRuntimeLibConcurrentBuildsCompileOnce(t *testing.T) {
	e := newRTEnv(t)
	slow := fakeCompiler(t, GCC, "sleep 0.05")
	e.cxx = slow
	const n = 6
	var wg sync.WaitGroup
	libs := make([]string, n)
	errs := make([]error, n)
	for i := range n {
		wg.Go(func() {
			_, objs, err := e.runtime(e.builder(nil))
			errs[i] = err
			if len(objs) == 1 {
				libs[i] = objs[0]
			}
		})
	}
	wg.Wait()
	for i := range n {
		if errs[i] != nil || libs[i] == "" || libs[i] != libs[0] || !libOK(libs[i]) {
			t.Fatalf("build %d: %q, %v", i, libs[i], errs[i])
		}
	}
	if got := len(lines(t, slow.Path)); got != nSources() {
		t.Errorf("%d builds at the same time compiled %d files, want %d: one build, the others wait", n, got, nSources())
	}
	if got := len(lines(t, e.ar)); got != 1 {
		t.Errorf("the archiver ran %d times, want 1", got)
	}
	if got := e.libFolders(); len(got) != 1 {
		t.Errorf("lib/ holds %q, want one folder", got)
	}
	dir, _ := ExtractRuntime(e.cache)
	assertNoTemp(t, filepath.Join(dir, "lib"))
}

func TestRuntimeLibWaitsForTheLockHolder(t *testing.T) {
	// Another process holds the lock and finishes its library: this build uses it and compiles
	// nothing.
	e := newRTEnv(t)
	b := e.builder(nil)
	a, _ := b.rtArchiver(context.Background())
	dir, _ := ExtractRuntime(e.cache)
	root := filepath.Join(dir, "lib")
	key := b.runtimeLibKey(rtfiles.Sources("linux"), b.runtimeFlags(), a)
	if err := os.MkdirAll(filepath.Join(root, key), 0o755); err != nil {
		t.Fatal(err)
	}
	lock, busy, err := tryLockFile(filepath.Join(root, key+".lock"))
	if err != nil || busy {
		t.Fatalf("tryLockFile: %v %v", err, busy)
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		writeArchive(t, filepath.Join(root, key, rtLibName), "from the other process")
		lock.unlock()
	}()
	_, objs, err := e.runtime(b)
	if err != nil || len(objs) != 1 || !strings.Contains(readText(t, objs[0]), "from the other process") {
		t.Fatalf("runtime() = %q, %v", objs, err)
	}
	if got := len(lines(t, e.cxx.Path)); got != 0 {
		t.Errorf("compiled %d files while another process was building the library", got)
	}
}

func TestRuntimeLibLockHolderStuck(t *testing.T) {
	// A holder that never finishes (a hung process): after the wait this build compiles itself.
	e := newRTEnv(t)
	old := libLockWait
	libLockWait = 100 * time.Millisecond
	t.Cleanup(func() { libLockWait = old })
	b := e.builder(nil)
	a, _ := b.rtArchiver(context.Background())
	dir, _ := ExtractRuntime(e.cache)
	root := filepath.Join(dir, "lib")
	key := b.runtimeLibKey(rtfiles.Sources("linux"), b.runtimeFlags(), a)
	os.MkdirAll(filepath.Join(root, key), 0o755)
	lock, _, err := tryLockFile(filepath.Join(root, key+".lock"))
	if err != nil {
		t.Fatal(err)
	}
	defer lock.unlock()
	_, objs, err := e.runtime(b)
	if err != nil || len(objs) != 1 || !libOK(objs[0]) {
		t.Fatalf("runtime() = %q, %v", objs, err)
	}
	if got := len(lines(t, e.cxx.Path)); got != nSources() {
		t.Errorf("compiled %d files, want %d", got, nSources())
	}
}

func TestRuntimeLibWaitCancelled(t *testing.T) {
	e := newRTEnv(t)
	b := e.builder(nil)
	a, _ := b.rtArchiver(context.Background())
	dir, _ := ExtractRuntime(e.cache)
	root := filepath.Join(dir, "lib")
	key := b.runtimeLibKey(rtfiles.Sources("linux"), b.runtimeFlags(), a)
	os.MkdirAll(filepath.Join(root, key), 0o755)
	lock, _, _ := tryLockFile(filepath.Join(root, key+".lock"))
	defer lock.unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, _, err := b.runtime(ctx, b.runtimeFlags(), false)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("runtime() waiting for a lock: %v, want the context's error", err)
	}
}

func TestRuntimeLibKilledMidBuildLeavesNoLibrary(t *testing.T) {
	// A build that is cancelled in the middle of the compiles (the fake compiler hangs on one
	// source) leaves no library; a library that is not finished is never used; the next build
	// works.
	e := newRTEnv(t)
	hang := fakeCompiler(t, GCC, `case "$out" in *fmt*) exec sleep 30;; esac`)
	e.cxx = hang
	ctx, cancel := context.WithTimeout(context.Background(), 700*time.Millisecond)
	defer cancel()
	b := e.builder(nil)
	if _, _, err := b.runtime(ctx, b.runtimeFlags(), false); err == nil {
		t.Fatal("a cancelled build succeeded")
	}
	dir, _ := ExtractRuntime(e.cache)
	if found, _ := filepath.Glob(filepath.Join(dir, "lib", "*", rtLibName)); len(found) != 0 {
		t.Errorf("a killed build left %q", found)
	}
	// Whatever is left (a temporary folder, an empty folder, the lock file) doesn't matter:
	e.cxx = fakeCompiler(t, GCC, "")
	_, objs, err := e.runtime(e.builder(nil))
	if err != nil || !libOK(objs[0]) {
		t.Fatalf("the build after the kill: %q, %v", objs, err)
	}
}

func TestRuntimeLibDamagedIsRebuilt(t *testing.T) {
	for name, content := range map[string]string{
		"empty":      "",
		"truncated":  "!<ar",
		"not a lib":  "not an archive at all, but longer than the magic",
		"a text lib": "\x7fELF",
	} {
		t.Run(name, func(t *testing.T) {
			e := newRTEnv(t)
			_, objs, err := e.runtime(e.builder(nil))
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(objs[0], []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			before := len(lines(t, e.cxx.Path))
			_, again, err := e.runtime(e.builder(nil))
			if err != nil || !libOK(again[0]) {
				t.Fatalf("runtime() = %q, %v", again, err)
			}
			if got := len(lines(t, e.cxx.Path)) - before; got != nSources() {
				t.Errorf("a damaged library was rebuilt with %d compiles, want %d", got, nSources())
			}
		})
	}
}

func TestLibOK(t *testing.T) {
	dir := t.TempDir()
	write := func(name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	for _, c := range []struct {
		path string
		want bool
	}{
		{write("ok.a", archiveText("member")), true},
		{write("ok2.a", archiveText("odd", "even")), true},
		{write("magic.a", "!<arch>\n"), false}, // no member: what a killed or damaged copy can leave
		{write("garbage.a", "!<arch>\nmember"), false},
		{write("truncated.a", archiveText("member data of some length")[:80]), false},
		{write("cut-header.a", archiveText("member")[:40]), false},
		{write("extra.a", archiveText("member")+"xx"), false},
		{write("badsize.a", strings.Replace(archiveText("member"), "6         `", "9999      `", 1)), false},
		{write("short.a", "!<arch>"), false},
		{write("empty.a", ""), false},
		{write("elf.a", "\x7fELF\x02\x01\x01\x00"), false},
		{dir, false},
		{filepath.Join(dir, "missing.a"), false},
	} {
		if got := libOK(c.path); got != c.want {
			t.Errorf("libOK(%s) = %v, want %v", filepath.Base(c.path), got, c.want)
		}
	}
}

func TestRuntimeLibArchiverFailure(t *testing.T) {
	e := newRTEnv(t)
	t.Setenv("GOCPP_AR", fakeArchiver(t, "echo 'ar: broken' >&2; exit 1"))
	_, _, err := e.runtime(e.builder(nil))
	var ce *CompileError
	if !errors.As(err, &ce) || ce.What != "the archiver" || !strings.Contains(string(ce.Output), "ar: broken") {
		t.Fatalf("runtime() = %v; want a CompileError of the archiver with the archiver's output", err)
	}
	dir, _ := ExtractRuntime(e.cache)
	if found, _ := filepath.Glob(filepath.Join(dir, "lib", "*", rtLibName)); len(found) != 0 {
		t.Errorf("a failed archive left %q", found)
	}
	assertNoTemp(t, filepath.Join(dir, "lib"))
}

func TestRuntimeLibArchiverWritesNothing(t *testing.T) {
	// An archiver that exits 0 and makes no library (or one that is not an archive).
	e := newRTEnv(t)
	t.Setenv("GOCPP_AR", script(t, "exit 0\n"))
	// Not a compile error: the cache is not usable, and the build goes on with the runtime's
	// objects in its work folder.
	_, objs, err := e.runtime(e.builder(nil))
	if err != nil || len(objs) != nSources() {
		t.Errorf("runtime() = %d files, %v; want the object files", len(objs), err)
	}
	dir, _ := ExtractRuntime(e.cache)
	if found, _ := filepath.Glob(filepath.Join(dir, "lib", "*", rtLibName)); len(found) != 0 {
		t.Errorf("left %q", found)
	}
}

func TestRuntimeLibCompileErrorIsReported(t *testing.T) {
	e := newRTEnv(t)
	e.cxx = fakeCompiler(t, GCC, `echo "rt.cpp:1: error: broken" >&2; exit 1`)
	_, _, err := e.runtime(e.builder(nil))
	var ce *CompileError
	if !errors.As(err, &ce) || ce.What != "the runtime" {
		t.Fatalf("runtime() = %v; want a CompileError of the runtime", err)
	}
	if got := e.libFolders(); len(got) > 1 {
		t.Errorf("lib/ holds %q", got)
	}
}

func TestRuntimeWithoutArchiverLinksObjects(t *testing.T) {
	// No archiver and no LTO: the runtime's objects as before.
	e := newRTEnv(t)
	t.Setenv("GOCPP_AR", "")
	empty := t.TempDir()
	t.Setenv("PATH", empty)
	_, objs, err := e.runtime(e.builder(nil))
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != nSources() || !strings.HasSuffix(objs[0], ".o") {
		t.Errorf("runtime() = %d files (%q), want the %d object files", len(objs), objs[0], nSources())
	}
	if got := e.libFolders(); len(got) != 0 {
		t.Errorf("lib/ holds %q without an archiver", got)
	}
}

func TestRuntimeLTOWithoutArchiverFails(t *testing.T) {
	e := newRTEnv(t)
	t.Setenv("GOCPP_AR", "")
	t.Setenv("PATH", t.TempDir())
	_, _, err := e.runtime(e.builder(func(b *Builder) { b.LTO = true }))
	if err == nil || !strings.Contains(err.Error(), "-lto needs an archiver") || !strings.Contains(err.Error(), "GOCPP_AR") {
		t.Fatalf("runtime() = %v; want a clear error about the archiver", err)
	}
	for _, l := range lines(t, e.cxx.Path) {
		if strings.Contains(l, " -c ") {
			t.Errorf("a compile ran before the missing archiver was noticed: %s", l)
		}
	}
}

func TestRuntimeNoRtLibSwitch(t *testing.T) {
	e := newRTEnv(t)
	t.Setenv("GOCPP_NORTLIB", "1")
	_, objs, err := e.runtime(e.builder(nil))
	if err != nil || len(objs) != nSources() {
		t.Fatalf("runtime() = %d files, %v", len(objs), err)
	}
	if got := len(lines(t, e.ar)); got != 0 {
		t.Errorf("the archiver ran %d times with GOCPP_NORTLIB", got)
	}
}

func TestRuntimeWithoutCacheNeedsNoLibrary(t *testing.T) {
	// No cache folder: the objects in the work folder, and the archiver is not asked.
	e := newRTEnv(t)
	b := e.builder(func(b *Builder) { b.Cache = "" })
	_, objs, err := e.runtime(b)
	if err != nil || len(objs) != nSources() {
		t.Fatalf("runtime() = %d files, %v", len(objs), err)
	}
	if got := len(lines(t, e.ar)); got != 0 {
		t.Errorf("the archiver ran %d times without a cache", got)
	}
}

func TestRuntimeLibWarmBuildDoesNotRunTools(t *testing.T) {
	// The no-op rebuild: a warm library costs a stat, not a compiler or archiver run, also
	// with the compiler's own flags in the environment.
	e := newRTEnv(t)
	for range 3 {
		if _, _, err := e.runtime(e.builder(nil)); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := len(lines(t, e.cxx.Path)), nSources(); got != want {
		t.Errorf("3 builds ran the compiler %d times, want %d", got, want)
	}
	if got := len(lines(t, e.ar)); got != 1 {
		t.Errorf("3 builds ran the archiver %d times, want 1", got)
	}
}

func TestRuntimeLibTraceShowsCommands(t *testing.T) {
	// -x shows the compiles, the archive step and the move, as shell lines.
	e := newRTEnv(t)
	if _, _, err := e.runtime(e.builder(nil)); err != nil {
		t.Fatal(err)
	}
	tr := e.trace.String()
	for _, want := range []string{"mkdir -p ", " rcsD ", rtLibName, "mv "} {
		if !strings.Contains(tr, want) {
			t.Errorf("the trace has no %q:\n%s", want, tr)
		}
	}
	// The archive step comes after every compile.
	last := strings.LastIndex(tr, " -c ")
	if ar := strings.Index(tr, " rcsD "); ar < last {
		t.Errorf("the archive step is before the last compile:\n%s", tr)
	}
}

func TestArchiverAskedOncePerBuild(t *testing.T) {
	e := newRTEnv(t)
	b := e.builder(nil)
	a1, err1 := b.rtArchiver(context.Background())
	t.Setenv("GOCPP_AR", "")
	t.Setenv("PATH", t.TempDir())
	a2, err2 := b.rtArchiver(context.Background())
	if err1 != nil || err2 != nil || a1.Path != a2.Path {
		t.Errorf("the archiver of one build changed: %v %v %v %v", a1, a2, err1, err2)
	}
}

// archiveText returns an archive with one member for each text (names m0, m1, ...), as ar writes it.
func archiveText(texts ...string) string {
	var b strings.Builder
	b.WriteString("!<arch>\n")
	for i, t := range texts {
		fmt.Fprintf(&b, "%-16s%-12d%-6d%-6d%-8s%-10d`\n", fmt.Sprintf("m%d/", i), 0, 0, 0, "100644", len(t))
		b.WriteString(t)
		if len(t)%2 == 1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func writeArchive(t *testing.T, path string, texts ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(archiveText(texts...)), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeLibKilledAfterArchiveStarted(t *testing.T) {
	// The archiver writes its output and then hangs; the build is cancelled: the library is not
	// at its final name (it is renamed only when the archiver has finished).
	e := newRTEnv(t)
	t.Setenv("GOCPP_AR", fakeArchiver(t, `printf '!<arch>\n' > "$2"; exec sleep 30`))
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	b := e.builder(nil)
	if _, _, err := b.runtime(ctx, b.runtimeFlags(), false); err == nil {
		t.Fatal("a cancelled build succeeded")
	}
	dir, _ := ExtractRuntime(e.cache)
	if found, _ := filepath.Glob(filepath.Join(dir, "lib", "*", rtLibName)); len(found) != 0 {
		t.Errorf("a build killed in the archiver left %q", found)
	}
}

func TestRuntimeBogusArchiverIsReported(t *testing.T) {
	// GOCPP_AR names something that does not exist: an error, not a silent fallback.
	e := newRTEnv(t)
	t.Setenv("GOCPP_AR", "/no/such/archiver-xyz")
	_, _, err := e.runtime(e.builder(nil))
	if err == nil {
		t.Fatal("a GOCPP_AR that names nothing was ignored")
	}
}

func TestRuntimeUserFltoNeedsLTOArchiver(t *testing.T) {
	// -flto in the user's flags (GOCPP_CXXFLAGS, `-O2 -flto`): the objects are LTO objects, so the
	// archiver must be the one that reads them, without -lto.
	e := newRTEnv(t)
	t.Setenv("GOCPP_AR", "")
	t.Setenv("PATH", toolDir(t, "ar"))
	_, _, err := e.runtime(e.builder(func(b *Builder) { b.Env = EnvFlags{CXXFlags: []string{"-O2", "-flto"}} }))
	if err == nil || !strings.Contains(err.Error(), "-lto needs an archiver") {
		t.Fatalf("runtime() with -flto in GOCPP_CXXFLAGS and only a plain ar = %v; want the archiver error", err)
	}
	// The same through the compiler's own arguments.
	cxx := e.cxx
	cxx.Args = []string{"-flto=thin"}
	_, _, err = e.runtime(e.builder(func(b *Builder) { b.CXX = cxx }))
	if err == nil || !strings.Contains(err.Error(), "-lto needs an archiver") {
		t.Fatalf("runtime() with -flto in the compiler's arguments = %v", err)
	}
	// And a build without any LTO flag is fine with a plain ar.
	if _, _, err := e.runtime(e.builder(nil)); err != nil {
		t.Fatalf("runtime() without LTO and a plain ar: %v", err)
	}
	b := &Builder{CXX: CXX{Family: GCC}}
	for flag, want := range map[string]bool{"-flto": true, "-flto=auto": true, "-fuse-linker-plugin": true, "-fno-lto": false, "-O2": false} {
		b.Env = EnvFlags{CXXFlags: []string{flag}}
		if got := b.ltoOn(); got != want {
			t.Errorf("ltoOn with %s = %v, want %v", flag, got, want)
		}
	}
}

func TestLinkLTOWithArchiveModeRefused(t *testing.T) {
	for _, mode := range []Mode{ModeCArchive, ModeCPPLib} {
		b := &Builder{CXX: CXX{Family: GCC}, LTO: true, Mode: mode, Work: t.TempDir()}
		err := b.Link(context.Background(), nil, filepath.Join(t.TempDir(), "x.a"))
		if err == nil || !strings.Contains(err.Error(), "-lto cannot be combined with -buildmode="+string(mode)) {
			t.Errorf("%s: Link = %v; want the -lto refusal", mode, err)
		}
	}
}

func TestLinkerTempTraceLines(t *testing.T) {
	// The linker's -Wl,-t lists the temporary objects of the LTO step, which are gone when the link
	// ends: not messages (linkTrace drops them).
	tmp := os.TempDir()
	work := t.TempDir()
	trace := strings.Join([]string{
		filepath.Join(tmp, "ccAbCd12.debug.temp.o"),
		filepath.Join(tmp, "lto-llvm-123abc.o"),
		filepath.Join(tmp, "ccXyZ.ltrans0.ltrans.o"),
		"/home/u/missing.debug.temp.o", // elsewhere: a message
		"ld: warning: x",
	}, "\n")
	rest, _ := linkTrace([]byte(trace), work)
	if got, want := strings.TrimSpace(string(rest)), "/home/u/missing.debug.temp.o\nld: warning: x"; got != want {
		t.Errorf("linkTrace kept %q, want %q", got, want)
	}
}

func TestRuntimeLibHasNoTestingNatives(t *testing.T) {
	// Package testing's natives are object files of a set of their own, in front of the library:
	// not members of it (a member is taken only when something wants it, and the global operator
	// new and delete of testing must not be in a program that doesn't import testing).
	e := newRTEnv(t)
	b := e.builder(nil)
	_, plain, err := b.runtime(context.Background(), b.runtimeFlags(), false)
	if err != nil || len(plain) != 1 {
		t.Fatalf("runtime(testing=false) = %q, %v; want the library alone", plain, err)
	}
	b = e.builder(nil)
	_, objs, err := b.runtime(context.Background(), b.runtimeFlags(), true)
	tsrcs := rtfiles.TestingSources("linux")
	if err != nil || len(objs) != len(tsrcs)+1 || objs[len(objs)-1] != plain[0] {
		t.Fatalf("runtime(testing=true) = %q, %v; want %d testing objects, then the same library", objs, err, len(tsrcs))
	}
	for i, s := range tsrcs {
		if filepath.Base(objs[i]) != runtimeObjectName(s) || filepath.Ext(objs[i]) != ".o" {
			t.Errorf("object %d is %s, want the object of %s", i, objs[i], s)
		}
	}
	// The library was archived from the runtime's objects only.
	ar := strings.Fields(lines(t, e.ar)[0])
	for _, m := range ar[2:] {
		if strings.Contains(filepath.Base(m), "testing") {
			t.Errorf("the library has the member %s", m)
		}
	}
	if len(ar) != 2+nSources() {
		t.Errorf("the library has %d members, want %d", len(ar)-2, nSources())
	}
}
