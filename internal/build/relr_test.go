package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The linkers of the dev shell (GNU ld 2.46, and lld where clang uses it) make DT_RELR: the probe
// says so, and a Builder's link has the flag.
func TestRelrProbe(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("DT_RELR is ELF's")
	}
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		ok, err := relrProbe(context.Background(), cxx, t.TempDir(), nil)
		if err != nil || !ok {
			t.Fatalf("relrProbe = %v, %v: want true (the dev shell's linker packs relative relocations)", ok, err)
		}
		b := &Builder{CXX: cxx, GOOS: "linux", Work: t.TempDir()}
		if got := b.relrFlags(context.Background()); len(got) != 1 || got[0] != relrFlag {
			t.Errorf("relrFlags = %q, want [%s]", got, relrFlag)
		}
		p, err := b.linkParts([]string{"a.o"}, nil, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		if !containsStr(p.pre, relrFlag) {
			t.Errorf("the link's flags %q lack %s", p.pre, relrFlag)
		}
		// Not for Windows (PE has base relocations), nor for a C compiler.
		w := &Builder{CXX: cxx, GOOS: "windows", Work: t.TempDir()}
		if got := w.relrFlags(context.Background()); got != nil {
			t.Errorf("relrFlags for windows = %q, want none", got)
		}
	})
}

// A linker that refuses the option (or warns about it) gives no flag.
func TestRelrProbeRefused(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("a shell script compiler")
	}
	dir := t.TempDir()
	fake := filepath.Join(dir, "c++")
	// The fake compiler answers the detection like a g++ and fails every other run, as a linker
	// that does not know -z pack-relative-relocs fails under --fatal-warnings.
	script := "#!/bin/sh\ncase \"$*\" in\n*--version*) echo 'g++ (GCC) 15.3.0';;\n*-dM*) echo '#define __GNUC__ 15';;\n*) echo 'ld: warning: -z pack-relative-relocs ignored' >&2; exit 1;;\nesac\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cxx, err := LookCXX(context.Background(), fake)
	if err != nil {
		t.Fatal(err)
	}
	b := &Builder{CXX: cxx, GOOS: "linux", Work: t.TempDir()}
	if got := b.relrFlags(context.Background()); got != nil {
		t.Errorf("relrFlags = %q, want none", got)
	}
	p, err := b.linkParts([]string{"a.o"}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if containsStr(p.pre, relrFlag) {
		t.Errorf("the link's flags %q have %s", p.pre, relrFlag)
	}
}

// The probe runs what it links: a program whose read through a relocated pointer fails (a loader that
// ignores DT_RELR leaves the pointer alone) gives no flag, although its ELF file has the entry.
func TestRelrProbeRunFails(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("a shell script compiler")
	}
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		fake := filepath.Join(t.TempDir(), "c++")
		// The probe's source (the link with --fatal-warnings) is replaced by a program with a
		// relocated pointer that exits with status 1, as the crash of the real thing does.
		script := "#!/bin/sh\ncase \"$*\" in\n*--fatal-warnings*) cat >/dev/null; echo 'static int x; int* p = &x; int main(void) { return 1; }' | " +
			cxx.Path + " \"$@\"; exit $?;;\nesac\nexec " + cxx.Path + " \"$@\"\n"
		if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		fcxx, err := LookCXX(context.Background(), fake)
		if err != nil {
			t.Fatal(err)
		}
		ok, err := relrProbe(context.Background(), fcxx, t.TempDir(), nil)
		if err != nil || ok {
			t.Errorf("relrProbe of a program that fails = %v, %v: want false (the ELF file has a DT_RELR, the run says no)", ok, err)
		}
		b := &Builder{CXX: fcxx, GOOS: "linux", Work: t.TempDir()}
		if got := b.relrFlags(context.Background()); got != nil {
			t.Errorf("relrFlags = %q, want none", got)
		}
	})
}

// The probe links with the build's link flags: a linker that does not know the option (gold) says no.
func TestRelrProbeLinkFlags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("DT_RELR is ELF's")
	}
	if _, err := exec.LookPath("ld.gold"); err != nil {
		t.Skip("no gold linker")
	}
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		gold := []string{"-fuse-ld=gold"}
		if ok, err := relrProbe(context.Background(), cxx, t.TempDir(), gold); err != nil || ok {
			t.Errorf("relrProbe with -fuse-ld=gold = %v, %v: want false", ok, err)
		}
		b := &Builder{CXX: cxx, GOOS: "linux", Work: t.TempDir(), Env: EnvFlags{LDFlags: gold}}
		if got := b.relrFlags(context.Background()); got != nil {
			t.Errorf("relrFlags with GOCPP_LDFLAGS=-fuse-ld=gold = %q, want none", got)
		}
		b = &Builder{CXX: cxx, GOOS: "linux", Work: t.TempDir()}
		if got := b.relrFlags(context.Background()); len(got) != 1 {
			t.Errorf("relrFlags without flags = %q, want the flag (a cached answer of the other flags was used?)", got)
		}
	})
}

// fakeLinkEnv makes a shell script compiler that answers the detection like g++, names the linker, C
// library and loader in files of a folder, and logs each run in the file log (when it is not "").
type fakeLinkEnv struct {
	dir, ld, libc, loader, gold string
	cxx                         CXX
}

func newFakeLinkEnv(t *testing.T, log string) *fakeLinkEnv {
	t.Helper()
	e := &fakeLinkEnv{dir: t.TempDir()}
	e.ld = e.touch(t, "ld", "")
	e.gold = e.touch(t, "my-ld.gold", "")
	e.libc = e.touch(t, "libc.so.6", "")
	e.loader = e.touch(t, "ld-loader", "")
	fake := filepath.Join(e.dir, "c++")
	logging := ""
	if log != "" {
		logging = "echo run >> " + log + "\n"
	}
	script := "#!/bin/sh\n" + logging + "case \"$*\" in\n*--version*) echo 'g++ (GCC) 15.3.0';;\n*-dM*) echo '#define __GNUC__ 15';;\n" +
		"*-print-prog-name=ld.gold*) echo " + e.dir + "/ld.gold;;\n*-print-prog-name=ld*) echo " + e.ld + ";;\n" +
		"*-print-file-name=libc.so.6*) echo " + e.libc + ";;\n*-print-file-name=ld-linux*) echo " + e.loader + ";;\nesac\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	cxx, err := LookCXX(context.Background(), fake)
	if err != nil {
		t.Fatal(err)
	}
	e.cxx = cxx
	return e
}

// touch writes the file name of the folder with new content, so that its size changes.
func (e *fakeLinkEnv) touch(t *testing.T, name, text string) string {
	t.Helper()
	p := filepath.Join(e.dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+text), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// The key of the answer changes with the link flags, and its deps (the files that it depends on, which a
// cached answer only stats) with the linker program (an upgraded binutils is asked again, also one named by
// an absolute -fuse-ld=, by --ld-path= or by the compiler's own arguments) and with the C library and
// loader of the probe's run.
func TestRelrDeps(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("a shell script compiler")
	}
	e := newFakeLinkEnv(t, "")
	ctx := context.Background()
	if relrKey(e.cxx, nil) != relrKey(e.cxx, nil) {
		t.Errorf("the key changes between two calls")
	}
	if relrKey(e.cxx, []string{"-fuse-ld=gold"}) == relrKey(e.cxx, nil) || relrKey(e.cxx, []string{"-B/opt/bin"}) == relrKey(e.cxx, nil) {
		t.Errorf("the key does not change with -fuse-ld=gold or -B")
	}
	withArgs := func(arg string) CXX {
		c := e.cxx
		c.Args = append(slices.Clone(e.cxx.Args), arg)
		return c
	}
	for _, tc := range []struct {
		what string
		c    CXX
		flag []string
		file string // the file to replace
	}{
		{"the linker program", e.cxx, nil, e.ld},
		{"the C library", e.cxx, nil, e.libc},
		{"the loader", e.cxx, nil, e.loader},
		{"-fuse-ld=FILE in the flags", e.cxx, []string{"-fuse-ld=" + e.gold}, e.gold},
		{"-fuse-ld=FILE in the compiler's arguments", withArgs("-fuse-ld=" + e.gold), nil, e.gold},
		{"--ld-path=FILE in the flags", e.cxx, []string{"--ld-path=" + e.gold}, e.gold},
		{"--ld-path=FILE in the compiler's arguments", withArgs("--ld-path=" + e.gold), nil, e.gold},
	} {
		deps := relrDeps(ctx, tc.c, tc.flag)
		if !relrDepsValid(deps) {
			t.Errorf("%s: fresh deps are not valid", tc.what)
		}
		e.touch(t, filepath.Base(tc.file), "# replaced "+tc.what+"\n")
		if relrDepsValid(deps) {
			t.Errorf("%s: a replaced file is not seen by the deps %v", tc.what, deps)
		}
		e.touch(t, filepath.Base(tc.file), "") // (as it was, for the next case)
	}
}

// A cached answer spawns no compiler: from the process's memory and from the disk record it costs a stat
// of each file that it depends on, as the build's one detection run is all that dev spawns; and a file
// that changed makes the probe (and the questions about the files) run again.
func TestRelrAnswerCachedSpawnsNothing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("a shell script compiler")
	}
	log := filepath.Join(t.TempDir(), "runs")
	e := newFakeLinkEnv(t, log)
	SetDetectCache(t.TempDir())
	t.Cleanup(func() { SetDetectCache("") })
	runs := func() int {
		data, _ := os.ReadFile(log)
		return strings.Count(string(data), "run\n")
	}
	ctx := context.Background()
	forget := func() {
		relrProbes.Lock()
		clear(relrProbes.m)
		relrProbes.Unlock()
	}
	forget()
	relrAnswer(ctx, e.cxx, t.TempDir(), nil)
	if runs() < 5 { // the detection runs of LookCXX, the 3 questions and the probe link
		t.Fatalf("the first answer spawned only %d compiler runs", runs())
	}
	before := runs()
	relrAnswer(ctx, e.cxx, t.TempDir(), nil) // memory
	if n := runs() - before; n != 0 {
		t.Errorf("an answer cached in memory spawned %d compiler runs, want 0", n)
	}
	forget()
	relrAnswer(ctx, e.cxx, t.TempDir(), nil) // disk
	if n := runs() - before; n != 0 {
		t.Errorf("an answer cached on disk spawned %d compiler runs, want 0", n)
	}
	// A replaced linker is asked again.
	e.touch(t, "ld", "# a new binutils\n")
	relrAnswer(ctx, e.cxx, t.TempDir(), nil)
	if n := runs() - before; n < 4 {
		t.Errorf("after the linker changed, %d compiler runs: want the questions and the probe again", n)
	}
}

func containsStr(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}
