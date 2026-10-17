package build

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCompilerDied(t *testing.T) {
	for _, c := range []struct {
		out  string
		want bool
	}{
		{"g++: internal compiler error: Segmentation fault signal terminated program cc1plus\nPlease submit a full bug report", true},
		{"g++: fatal error: Killed signal terminated program cc1plus\ncompilation terminated.", true},
		{"clang++: error: clang frontend command failed due to signal (use -v to see invocation)\nPLEASE submit a bug report to", true},
		{"p.cpp:3:1: error: expected ';' after expression", false},
		{"", false},
	} {
		if got := compilerDied(&CompileError{Output: []byte(c.out)}); got != c.want {
			t.Errorf("compilerDied(%q) = %v, want %v", c.out, got, c.want)
		}
	}
	// The driver itself killed by a signal, whatever it printed. Not SEGV: a shell that dies of
	// it goes through the system's core dump handler, which stalls for minutes under load.
	if runtime.GOOS != "windows" {
		err := exec.Command("sh", "-c", "kill -KILL $$").Run()
		if !compilerDied(&CompileError{Err: err}) {
			t.Errorf("compilerDied(%v) = false for a driver killed by a signal", err)
		}
	}
	if err := exec.Command("sh", "-c", "exit 1").Run(); err != nil && compilerDied(&CompileError{Err: err}) {
		t.Errorf("compilerDied(%v) = true for an exit status", err)
	}
}

func TestPrecompiledHeaderCompilerDies(t *testing.T) {
	// A compiler that dies with the precompiled header (an ICE: g++ crashes on a .gch that is
	// damaged, as happened when two builds wrote one) must not fail the build: the program
	// compiles again without the header, the header is removed, and -x says so. The compiler
	// is a wrapper that dies like g++ and clang do when it is asked to use a header.
	if runtime.GOOS == "windows" {
		t.Skip("the wrapper is a shell script")
	}
	t.Setenv("GOCPP_NOOBJCACHE", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute) // a clean failure, not the test binary's timeout
	defer cancel()
	for _, name := range []string{"clang++", "g++"} {
		t.Run(name, func(t *testing.T) {
			real, err := LookCXX(ctx, name)
			if err != nil {
				t.Skip("no C++ compiler:", err)
			}
			dir := t.TempDir()
			wrapper := filepath.Join(dir, "dying")
			die := filepath.Join(dir, "die") // the wrapper dies once this file exists
			script := "#!/bin/sh\n[ -e '" + die + "' ] && case \" $* \" in\n*\" -include \"*|*\" -include-pch \"*)\n" +
				"\techo \"$0: internal compiler error: Segmentation fault signal terminated program cc1plus\" >&2\n" +
				"\techo \"PLEASE submit a bug report\" >&2\n\tkill -KILL $$ 2>/dev/null; exit 4;;\nesac\nexec \"$@\"\n"
			if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
				t.Fatal(err)
			}
			cxx := real
			cxx.Args = slices.Concat([]string{real.Path}, real.Args)
			cxx.Path = wrapper
			cache := t.TempDir()
			build := func() string {
				work := t.TempDir()
				src := filepath.Join(work, "p.cpp")
				if err := os.WriteFile(src, []byte("#include <gocpp/rt.hpp>\n#include <cstdio>\nint main() { std::puts(\"alive\"); }\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				var tr strings.Builder
				b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Trace: &tr, Stderr: os.Stderr}
				bin := filepath.Join(work, "p")
				if err := b.Program(ctx, src, bin); err != nil {
					var ce *CompileError
					if errors.As(err, &ce) {
						t.Fatalf("%v\n%s\n%s", ce, ce.Command, ce.Output)
					}
					t.Fatal(err)
				}
				if out, err := exec.Command(bin).Output(); err != nil || string(out) != "alive\n" {
					t.Fatalf("the program prints %q, %v", out, err)
				}
				return tr.String()
			}
			build() // builds the header while the runtime compiles; the compile may use it
			dirRT, _ := ExtractRuntime(cache)
			found, _ := filepath.Glob(filepath.Join(dirRT, "pch", "*", "rt.hpp.[pg]ch"))
			if len(found) != 1 {
				t.Fatalf("the first build left %q, want one precompiled header", found)
			}
			if err := os.WriteFile(die, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			second := build()
			if !strings.Contains(second, "# the compiler died with the precompiled header") {
				t.Errorf("no note about the compiler that died:\n%s", second)
			}
			if _, err := os.Stat(found[0]); err == nil {
				t.Error("the header that the compiler died with is left")
			}
			if _, err := os.Stat(found[0] + ".deps"); err == nil {
				t.Error("the list of the header that the compiler died with is left")
			}
			// The next build makes a new header, and works again.
			os.Remove(die)
			build()
			if again, _ := filepath.Glob(filepath.Join(dirRT, "pch", "*", "rt.hpp.[pg]ch")); len(again) != 1 {
				t.Errorf("after the fallback: headers %q, want a new one", again)
			}
		})
	}
}

// pchTestHeader returns a precompiled header of the runtime for g++ in a new cache, with a
// source that includes it, and a function that compiles the source with it (to a file n).
func pchTestHeader(t *testing.T, ctx context.Context, trace io.Writer) (newPCH func() *pch, compile func(n int) error) {
	t.Helper()
	cxx, err := LookCXX(ctx, "g++")
	if err != nil {
		t.Skip("no g++:", err)
	}
	cache := t.TempDir()
	rtDir, err := ExtractRuntime(cache)
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	src := filepath.Join(work, "p.cpp")
	if err := os.WriteFile(src, []byte("#include <gocpp/rt.hpp>\nint f() { return 1; }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newPCH = func() *pch {
		b := &Builder{CXX: cxx, GOOS: runtime.GOOS, Work: work, Cache: cache, Stderr: os.Stderr, Trace: trace}
		pc := b.precompiled(rtDir)
		if pc == nil {
			t.Skip("no precompiled header for this compiler")
		}
		return pc
	}
	pc := newPCH()
	compile = func(n int) error {
		args := slices.Concat(pc.b.pchFlags(), pc.args(), []string{"-I", filepath.Join(rtDir, "include"), "-c", src, "-o", filepath.Join(work, fmt.Sprintf("p%d.o", n))})
		_, err := pc.b.runOutput(ctx, cxx, "the test", false, args)
		return err
	}
	return newPCH, compile
}

// syncBuffer is a strings.Builder that can be written by many goroutines.
type syncBuffer struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func TestPrecompiledHeaderConcurrentBuildOnce(t *testing.T) {
	// Runs of buildOnce at the same time (as in several gocpp processes, or containers with
	// equal pids, which the guard of one process can't keep apart) must not write one file:
	// g++ crashed on a header that another run was still writing (see pch.go). Each run has
	// a temporary file of its own, and compiles that use the header as soon as it exists, and
	// many at once when all are done, all work.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var trace syncBuffer
	newPCH, compile := pchTestHeader(t, ctx, &trace)
	pc := newPCH()
	const runs = 6
	var wg sync.WaitGroup
	done := make(chan struct{})
	for range runs {
		b := newPCH()
		wg.Go(func() { b.buildOnce(ctx) })
	}
	go func() { wg.Wait(); close(done) }()
	compiles := 0
	for finished := false; !finished && compiles < 12; {
		select {
		case <-done:
			finished = true
		case <-time.After(50 * time.Millisecond):
			if pc.ready() {
				compiles++
				if err := compile(0); err != nil {
					cancel()
					<-done
					var ce *CompileError
					if errors.As(err, &ce) {
						t.Fatalf("a compile with a header that is built by others: %v\n%s", ce, ce.Output)
					}
					t.Fatal(err)
				}
			}
		}
	}
	<-done
	// Every run wrote its own file: the -o of the header compiles are all different.
	outs := map[string]bool{}
	for _, line := range strings.Split(trace.String(), "\n") {
		if !strings.Contains(line, " c++-header ") {
			continue
		}
		words := strings.Fields(line)
		if i := slices.Index(words, "-o"); i >= 0 && i+1 < len(words) {
			outs[words[i+1]] = true
		}
	}
	if len(outs) != runs {
		t.Errorf("%d runs of buildOnce wrote %d different temporary files, want %d:\n%s", runs, len(outs), runs, trace.String())
	}
	if !pc.ready() {
		t.Fatal("no header after the builds")
	}
	assertNoTemp(t, filepath.Dir(pc.header))
	var cwg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		cwg.Go(func() { errs[i] = compile(i) })
	}
	cwg.Wait()
	for _, err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
}

func TestPrecompiledHeaderSingleFlight(t *testing.T) {
	// While this process builds a header, another build of it returns at once without running
	// the compiler; after that one is done, a build works again.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	var trace syncBuffer
	newPCH, _ := pchTestHeader(t, ctx, &trace)
	pc := newPCH()
	if _, loaded := pchBuilds.LoadOrStore(pc.file, struct{}{}); loaded {
		t.Fatal("a build of the header is running")
	}
	newPCH().build(ctx) // returns at once
	if trace.String() != "" || pc.ready() {
		t.Errorf("a second build ran while the first was not done (ready %v):\n%s", pc.ready(), trace.String())
	}
	pchBuilds.Delete(pc.file)
	pc.build(ctx)
	if !pc.ready() || !strings.Contains(trace.String(), " c++-header ") {
		t.Fatalf("no header after a build (ready %v):\n%s", pc.ready(), trace.String())
	}
	// A guard that a panicking or failing build left would stop all later builds: build
	// leaves none.
	if _, loaded := pchBuilds.Load(pc.file); loaded {
		t.Error("the guard of a finished build is left")
	}
}

func TestPrecompiledHeaderNeedsItsList(t *testing.T) {
	// A header without its list of files (another process removed or replaced them between
	// its two steps) is not ready: it is built again, and the list comes back.
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	newPCH, _ := pchTestHeader(t, ctx, nil)
	pc := newPCH()
	pc.build(ctx)
	if !pc.ready() {
		t.Fatal("no header after a build")
	}
	if err := os.Remove(pc.depfile()); err != nil {
		t.Fatal(err)
	}
	if pc.ready() {
		t.Fatal("a header without its list is ready")
	}
	pc.build(ctx)
	if !pc.ready() {
		t.Fatal("the header is not rebuilt with its list")
	}
	// remove takes the list first: no moment with a header and no list.
	pc.remove()
	if _, err := os.Stat(pc.file); err == nil {
		t.Error("remove left the header")
	}
	if _, err := os.Stat(pc.depfile()); err == nil {
		t.Error("remove left the list")
	}
}
