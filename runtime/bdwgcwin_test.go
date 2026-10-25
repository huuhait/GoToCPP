//go:build linux

package rtfiles_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// The Windows build of the vendored bdwgc (phase 16 README §2.4, §2.5, §3 step 16.1): the smoke
// program (test/bdwgc_test.cpp) cross-compiled with the dev shell's MinGW-w64 g++ and run under
// Wine, or on the real Windows PC when GOCPP_WINDOWS_SSH is set, and the two Windows probes of
// test/bdwgc_windows_probe.cpp (thread starts and the static-root scan), which run on the real PC
// only: Wine's thread creation and address space are not Windows's, so its numbers would say
// nothing about the machine the plan measures.

// bdwgcWindowsProgram builds the vendored bdwgc for windows/amd64 through build.Builder.Bdwgc and
// builds and links the program src (a path in runtime/) with the cross compiler, as the driver
// links a Windows program (-static, the 1 GiB stack, -pthread). It returns the .exe.
func bdwgcWindowsProgram(t *testing.T, src string, cflags []string) string {
	t.Helper()
	c := mingw(t) // skips without the MinGW-w64 cross compiler
	cc := c.cc
	cc.GOOS, cc.GOARCH = "windows", "amd64"
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil { // Builder.Work must exist: the compiles run in it
		t.Fatal(err)
	}
	b := &build.Builder{
		CXX:   cc,
		GOOS:  "windows",
		Cache: filepath.Join(dir, "cache"),
		Work:  work,
		// internal/build's executor is not seen by the tests' gate: build one source at a time,
		// like bdwgcLibrary (parallel_test.go).
		Jobs: 1,
	}
	lib, err := b.Bdwgc(context.Background(), build.BdwgcConfig{CFlags: cflags})
	if err != nil {
		t.Fatalf("building bdwgc for windows/amd64: %v", err)
	}
	if !strings.HasSuffix(lib.Archive, ".a") || !filepath.IsAbs(lib.Archive) {
		t.Fatalf("the bdwgc library is %q, want an absolute .a", lib.Archive)
	}

	obj := filepath.Join(dir, "program.o")
	args := append(c.flags(cflags), "-c", sourcePath(t, src), "-o", obj)
	args = append(args, lib.CompileFlags...)
	if err := c.compileCmd(prioRun, dir, nil, args); err != nil {
		t.Fatalf("compiling %s for windows/amd64: %v", src, err)
	}
	exe := filepath.Join(dir, strings.TrimSuffix(filepath.Base(src), ".cpp")+".exe")
	link := append(c.flags(cflags), obj, lib.Archive)
	link = append(link, lib.LinkFlags...)
	link = append(link, build.LinkFlags(cc)...)
	link = append(link, "-o", exe)
	if err := c.compileCmd(prioRun, dir, nil, link); err != nil {
		t.Fatalf("linking %s for windows/amd64: %v", src, err)
	}
	return exe
}

// TestBdwgcWindowsSmoke builds and runs the bdwgc smoke program for windows/amd64: under Wine, or
// on the real PC when GOCPP_WINDOWS_SSH is set (windows_test.go). It prints "OK" last, as on
// Linux; the Windows build stops the world with SuspendThread instead of signals, so the program
// skips the two signal setters there.
func TestBdwgcWindowsSmoke(t *testing.T) {
	t.Parallel()
	for _, mode := range []struct {
		name  string
		flags []string
	}{
		{"O0", []string{"-O0"}},
		{"O2", []string{"-O2"}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			t.Parallel()
			exe := bdwgcWindowsProgram(t, "test/bdwgc_test.cpp", mode.flags)
			env := wineSetup(t)
			r := env.runLong(t, exe, nil)
			lines := strings.Split(strings.TrimSpace(strings.ReplaceAll(string(r.stdout), "\r\n", "\n")), "\n")
			ok := r.code == 0 && len(r.stderr) == 0 && len(lines) > 0 && lines[len(lines)-1] == "OK"
			if !ok {
				t.Fatalf("test/bdwgc_test.cpp (windows/amd64, %s): exit code %d\nstdout:\n%s\nstderr:\n%s\nwant exit code 0, nothing on stderr, and OK last",
					mode.name, r.code, r.stdout, r.stderr)
			}
			t.Logf("%s", strings.Join(lines, "\n"))
		})
	}
}

// The probe runs of TestBdwgcWindowsProbes. The counts are the ticket's: 1,000 and 10,000 thread
// starts, and 100 collections per static-root mode (each mode in its own process: GC_set_no_dls
// and GC_add_roots cannot be undone cleanly inside one process).
const (
	probeThreadsSmall = "1000"
	probeThreadsLarge = "10000"
	probeThreadReps   = "5"
	probeStaticReps   = "100"
)

// TestBdwgcWindowsProbes measures the two Windows-specific costs on the real PC (never Wine): the
// start of a thread that registers with the collector and exits (1,000 and 10,000), and the scan
// of the static data that bdwgc repeats in every collection, with GC_set_no_dls and with the
// executable's own sections (the plan's mitigation). The numbers are a spike: the test logs them
// and checks only what must hold (the mode semantics), never a budget.
func TestBdwgcWindowsProbes(t *testing.T) {
	t.Parallel()
	if os.Getenv("GOCPP_WINDOWS_SSH") == "" {
		t.Skip("the bdwgc Windows probes run on the real PC (GOCPP_WINDOWS_SSH=user@host), never under Wine")
	}
	exe := bdwgcWindowsProgram(t, "test/bdwgc_windows_probe.cpp", []string{"-O2"})
	env := wineSetup(t) // GOCPP_WINDOWS_SSH is set: the real PC

	run := func(args ...string) winRun {
		t.Helper()
		r := env.runLong(t, exe, nil, args...)
		if r.code != 0 || len(r.stderr) != 0 {
			t.Fatalf("%s %s: exit code %d\nstdout:\n%s\nstderr:\n%s", filepath.Base(exe), strings.Join(args, " "), r.code, r.stdout, r.stderr)
		}
		out := strings.ReplaceAll(string(r.stdout), "\r\n", "\n")
		for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
			t.Logf("%s", line)
		}
		return r
	}

	info := string(run("info").stdout)
	if !strings.Contains(info, "gc_version=8.2.12") {
		t.Errorf("info: %q, want gc_version=8.2.12", info)
	}
	// The static-root modes: the object reachable only from a global survives with the default
	// scan and with the executable's own sections added, and is collected with no static roots at
	// all. That is what makes GC_set_no_dls a real mitigation only together with GC_add_roots.
	for _, mode := range []struct {
		name    string
		survive string
	}{
		{"default", "global_survived=yes"},
		{"nodls", "global_survived=no"},
		{"own", "global_survived=yes"},
	} {
		out := string(run("static", mode.name, probeStaticReps).stdout)
		if !strings.Contains(out, mode.survive) {
			t.Errorf("static %s: %q, want %s", mode.name, out, mode.survive)
		}
		if !strings.Contains(out, "no_dls=") {
			t.Errorf("static %s: %q, want the no_dls value", mode.name, out)
		}
	}
	for _, n := range []string{probeThreadsSmall, probeThreadsLarge} {
		out := string(run("threads", n, probeThreadReps).stdout)
		if !strings.Contains(out, "summary threads n="+n) {
			t.Errorf("threads %s: %q, want the summary line", n, out)
		}
	}
}
