package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// The tests of WaitDelay: a compiler that exits with a success status while a process that it
// started keeps its output open is the same as a compiler whose output is copied slowly on a
// loaded machine: os/exec's Wait returns ErrWaitDelay. That is not a failed compile.

// holdsOutput is the line of a fake compiler that starts a process that keeps the output open
// for 20 seconds after the compiler exited (the test's lowered WaitDelay is much shorter, and
// the tests that run in a few seconds are bounded by 10; the callers kill the process, and the
// cleanup of script kills what is left). It writes the process's pid to $0.holder.
const holdsOutput = "sleep 20 &\necho $! >> \"$0.holder\"\n"

// holderBound is how long a test may take that waits for a process that holds the output.
const holderBound = 10 * time.Second

// setWaitDelays sets waitDelay and retryWaitDelay for the rest of the test.
func setWaitDelays(t *testing.T, delay, retry time.Duration) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	oldDelay, oldRetry := waitDelay, retryWaitDelay
	waitDelay, retryWaitDelay = delay, retry
	t.Cleanup(func() { waitDelay, retryWaitDelay = oldDelay, oldRetry })
}

// script writes the shell script body to a new file and returns its path.
func script(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fake")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { killHolders(path) }) // nothing outlives the test
	return path
}

// wrapper returns the compiler cxx behind a script that first starts a process that keeps the
// output open.
func wrapper(t *testing.T, cxx CXX) CXX {
	t.Helper()
	words := []string{"exec", shellQuote(cxx.Path)}
	for _, a := range cxx.Args {
		words = append(words, shellQuote(a))
	}
	c := cxx
	c.Path, c.Args = script(t, holdsOutput+strings.Join(words, " ")+" \"$@\"\n"), nil
	c.stamp = fileStamp{}
	return c
}

// within fails the test when f takes longer than limit: a compiler that left a process
// running must not make gocpp wait for that process.
func within(t *testing.T, limit time.Duration, f func()) {
	t.Helper()
	start := time.Now()
	f()
	if d := time.Since(start); d > limit {
		t.Errorf("took %v, more than %v", d, limit)
	}
}

func TestWaitDelayCompileSucceeds(t *testing.T) {
	setWaitDelays(t, time.Millisecond, time.Millisecond)
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		c := wrapper(t, cxx)
		work := t.TempDir()
		src := filepath.Join(work, "p.cpp")
		if err := os.WriteFile(src, []byte("int main() { return 0; }\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		obj := filepath.Join(work, "p.o")
		b := &Builder{CXX: c, Work: work}
		args := []string{"-c", src, "-o", obj}
		within(t, holderBound, func() {
			// runOutput says what happened, and it is not a CompileError.
			out, err := b.runOutput(context.Background(), c, "the test", false, args)
			var ce *CompileError
			if !errors.Is(err, errOutputOpen) || errors.As(err, &ce) || len(out) != 0 {
				t.Errorf("runOutput = %q, %v; want errOutputOpen", out, err)
			}
			// run, which the link and the runtime use, succeeds.
			if err := b.run(context.Background(), c, "the test", false, args); err != nil {
				t.Errorf("run: %v", err)
			}
		})
		if _, err := os.Stat(obj); err != nil {
			t.Errorf("no object file: %v", err)
		}
	})
}

func TestWaitDelayObjCacheNotStored(t *testing.T) {
	forEachCompiler(t, func(t *testing.T, cxx CXX) {
		setWaitDelays(t, time.Millisecond, time.Millisecond)
		cache := t.TempDir()
		c := wrapper(t, cxx)
		var r objRun
		within(t, holderBound, func() { r = objCompile(t, c, cache, objTUSimple) })
		if r.err != nil || !r.ran || r.hit() {
			t.Fatalf("compile: ran=%v hit=%v err=%v", r.ran, r.hit(), r.err)
		}
		if !strings.Contains(r.trace, "# not cached: the compiler's output may be incomplete") {
			t.Errorf("no note that the output may be incomplete in the trace:\n%s", r.trace)
		}
		if objs, mans := objFiles(t, cache); len(objs)+len(mans) != 0 {
			t.Errorf("a compile with an unreliable output left %v %v", objs, mans)
		}
		// With the compiler itself the same compile is stored (the test would pass
		// vacuously if nothing could be stored).
		setWaitDelays(t, 5*time.Second, 5*time.Second)
		if r := objCompile(t, cxx, cache, objTUSimple); r.err != nil || len(r.out) != 0 {
			t.Fatalf("compile with the compiler: %v %q", r.err, r.out)
		}
		if objs, _ := objFiles(t, cache); len(objs) != 1 {
			t.Errorf("the compile with the compiler itself was not stored: %v", objs)
		}
	})
}

const objTUSimple = "int main() { return 0; }\n"

func TestWaitDelayFailureStaysFailure(t *testing.T) {
	setWaitDelays(t, time.Millisecond, time.Millisecond)
	// A compiler that fails keeps its ExitError, also when a process holds the output.
	c := CXX{Path: script(t, holdsOutput+"echo broken >&2\nexit 3\n")}
	b := &Builder{CXX: c, Work: t.TempDir()}
	within(t, holderBound, func() {
		_, err := b.runOutput(context.Background(), c, "the test", false, nil)
		var ce *CompileError
		if !errors.As(err, &ce) || errors.Is(err, errOutputOpen) || !strings.Contains(ce.Err.Error(), "exit status 3") {
			t.Errorf("runOutput = %v; want a CompileError for exit status 3", err)
		}
	})
}

func TestWaitDelayDetectRetries(t *testing.T) {
	// The output of --version is data: when it may be incomplete, the compiler runs again.
	setWaitDelays(t, time.Millisecond, 10*time.Second)
	// The first run leaves a process that holds the output, the second doesn't.
	path := script(t, `echo run >> "$0.log"
if [ ! -e "$0.first" ]; then : > "$0.first"; `+holdsOutput+`fi
echo "fake 1.0"
`)
	c := CXX{Path: path}
	within(t, holderBound, func() {
		out, err := runDetect(context.Background(), c, path, "--version")
		if err != nil || string(out) != "fake 1.0\n" {
			t.Errorf("runDetect = %q, %v", out, err)
		}
	})
	if n := runsOf(t, path); n != 2 {
		t.Errorf("the compiler ran %d times, want 2", n)
	}
}

func TestWaitDelayDetectKillsHolder(t *testing.T) {
	// The compiler of runDetect and pkg-config are in a process group, which is killed when
	// they exit: the process that held the output does not outlive the run.
	setWaitDelays(t, time.Millisecond, 20*time.Millisecond)
	for _, name := range []string{"detect", "pkg-config"} {
		path := script(t, holdsOutput+"echo \"fake 1.0\"\n")
		within(t, holderBound, func() {
			if name == "detect" {
				runDetect(context.Background(), CXX{Path: path}, path, "--version")
			} else {
				runPkgConfig(context.Background(), t.TempDir(), path, "--cflags", []string{"x"}, nil)
			}
		})
		if !waitUntil(5*time.Second, func() bool { return len(holdersLeft(path)) == 0 }) {
			t.Errorf("%s: the process that held the output is still running: %v", name, holdersLeft(path))
		}
	}
}

func TestWaitDelayDetectGivesUp(t *testing.T) {
	// A process that holds the output on every run: an error, after one more run only.
	setWaitDelays(t, time.Millisecond, 20*time.Millisecond)
	path := script(t, "echo run >> \"$0.log\"\n"+holdsOutput+"echo \"fake 1.0\"\n")
	within(t, holderBound, func() {
		_, err := runDetect(context.Background(), CXX{Path: path}, path, "--version")
		if err == nil || !strings.Contains(err.Error(), "WaitDelay") {
			t.Errorf("runDetect: %v; want an error about WaitDelay", err)
		}
	})
	if n := runsOf(t, path); n != 2 {
		t.Errorf("the compiler ran %d times, want 2", n)
	}
	// A failing exit is not run again.
	fail := script(t, "echo run >> \"$0.log\"\n"+holdsOutput+"exit 3\n")
	within(t, holderBound, func() {
		if _, err := runDetect(context.Background(), CXX{Path: fail}, fail, "--version"); err == nil || !strings.Contains(err.Error(), "exit status 3") {
			t.Errorf("runDetect of a failing compiler: %v", err)
		}
	})
	if n := runsOf(t, fail); n != 1 {
		t.Errorf("the failing compiler ran %d times, want 1", n)
	}
}

func TestWaitDelayPkgConfigRetries(t *testing.T) {
	setWaitDelays(t, time.Millisecond, 10*time.Second)
	path := script(t, `echo run >> "$0.log"
if [ ! -e "$0.first" ]; then : > "$0.first"; `+holdsOutput+`fi
echo "-I/x -DY"
`)
	within(t, holderBound, func() {
		flags, err := runPkgConfig(context.Background(), t.TempDir(), path, "--cflags", []string{"x"}, nil)
		if want := []string{"-I/x", "-DY"}; err != nil || !slices.Equal(flags, want) {
			t.Errorf("runPkgConfig = %q, %v; want %q", flags, err, want)
		}
	})
	if n := runsOf(t, path); n != 2 {
		t.Errorf("pkg-config ran %d times, want 2", n)
	}
}

func TestWaitDelayPCHRefusedOutputOpen(t *testing.T) {
	// The compiler refuses the precompiled header, and the compile without it succeeds but its
	// output is cut: that is a success, and the refused header is removed (a build that keeps
	// it compiles twice from then on).
	setWaitDelays(t, time.Millisecond, time.Millisecond)
	for _, c := range []struct {
		name, plain string
		removed     bool
	}{
		{"success", holdsOutput + "exit 0\n", true},
		{"program error", "echo 'x.cpp:1:1: error: broken' >&2\nexit 1\n", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			path := script(t, "case \"$*\" in *-include-pch*) echo \"fatal error: PCH file built from a different branch\" >&2; exit 1;; esac\n"+c.plain)
			cxx := CXX{Path: path, Family: Clang}
			b := &Builder{CXX: cxx, Work: t.TempDir()}
			dir := t.TempDir()
			header := filepath.Join(dir, "rt.hpp")
			pc := &pch{b: b, rtDir: dir, header: header, file: header + ".pch"}
			if err := os.WriteFile(pc.file, []byte("pch"), 0o644); err != nil {
				t.Fatal(err)
			}
			var out []byte
			var used bool
			var err error
			within(t, holderBound, func() {
				out, used, err = b.runGenerated(context.Background(), pc, []string{"-c", "x.cpp", "-o", "x.o"})
			})
			var ce *CompileError
			switch {
			case c.removed && !errors.Is(err, errOutputOpen):
				t.Errorf("runGenerated = %q, %v; want errOutputOpen", out, err)
			case !c.removed && !errors.As(err, &ce):
				t.Errorf("runGenerated = %q, %v; want a CompileError", out, err)
			case used:
				t.Error("the result says that the header was used")
			}
			if _, statErr := os.Stat(pc.file); (statErr != nil) != c.removed {
				t.Errorf("the header removed = %v, want %v", statErr != nil, c.removed)
			}
		})
	}
}
