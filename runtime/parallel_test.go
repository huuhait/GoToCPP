package rtfiles_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	rtfiles "github.com/huuhait/gocpp/runtime"
)

// The tests of this package run a few hundred C++ compiles (the programs of test/, in each
// compiler and mode, the runtime's own files, the reject cases). Most of them are independent,
// so they run concurrently, and one gate bounds the external commands of the whole test binary:
// go test's -parallel limits only the tests that call t.Parallel, not the goroutines a test
// starts, and the machine may be shared with other jobs. The bound is GOCPP_TEST_JOBS if set,
// else min(NumCPU, 48). It is also the memory bound: measured, 48 commands at once peak at about
// 15 GB in all, and the largest compiler process takes about 0.8 GB.

// jobs returns the number of commands that may run at the same time.
func jobs() int {
	if s := os.Getenv("GOCPP_TEST_JOBS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			panic("GOCPP_TEST_JOBS=" + s + ": want a positive number")
		}
		return n
	}
	return min(runtime.NumCPU(), 48)
}

// A slots is the gate: jobs() commands run at the same time, and when more wait, the one with
// the highest priority goes first (the longest compile first finishes the whole set sooner than
// the longest last: the longest ones take a minute, the shortest a second), the oldest of equal
// priority first.
type slots struct {
	mu      sync.Mutex
	free    int
	waiting []*waiter
}

type waiter struct {
	prio int
	ch   chan struct{}
}

var gate = &slots{free: jobs()}

// Priorities of the commands, higher first: the programs' runs and links go before all compiles.
// A compile of a test program has its own (prog.weight, about the seconds it takes, plus its
// longest run), so the slowest ones start first. The runtime's objects (about 2 s each) come
// after the slow compiles, which are what ends the run, but before the small ones (weights
// below prioRT): every link waits for them.
const (
	prioRun = 1000
	prioRT  = 10
)

// acquire waits for a free slot.
func (g *slots) acquire(prio int) {
	g.mu.Lock()
	if g.free > 0 {
		g.free--
		g.mu.Unlock()
		return
	}
	w := &waiter{prio, make(chan struct{})}
	g.waiting = append(g.waiting, w)
	g.mu.Unlock()
	<-w.ch
}

// release gives the slot to the waiter with the highest priority, or frees it.
func (g *slots) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	best := -1
	for i, w := range g.waiting {
		if best < 0 || w.prio > g.waiting[best].prio {
			best = i
		}
	}
	if best < 0 {
		g.free++
		return
	}
	w := g.waiting[best]
	g.waiting = slices.Delete(g.waiting, best, best+1)
	close(w.ch)
}

// parallel calls f(0) to f(n-1), each in its own goroutine, waits for all of them and returns
// their errors joined, in the order of i (nil if none failed). A call must not use t.Fatal or
// other methods of testing.T that end the test: it returns its error instead. The commands
// that the calls run are bounded by gate.
func parallel(n int, f func(i int) error) error {
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = f(i)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

var (
	rtDirOnce sync.Once
	rtDir     string
	rtDirErr  error
)

// extractRuntime writes the embedded runtime into a temporary folder, as the driver will do it,
// once for the whole test binary (TestMain removes it), and returns the folder. The tests only
// read it.
func extractRuntime(t *testing.T) string {
	t.Helper()
	rtDirOnce.Do(func() {
		rtDir, rtDirErr = os.MkdirTemp("", "gocpp-rt-")
		if rtDirErr != nil {
			return
		}
		rtDirErr = fs.WalkDir(rtfiles.FS(), ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			dst := filepath.Join(rtDir, filepath.FromSlash(p))
			if d.IsDir() {
				return os.MkdirAll(dst, 0o755)
			}
			b, err := fs.ReadFile(rtfiles.FS(), p)
			if err != nil {
				return err
			}
			return os.WriteFile(dst, b, 0o644)
		})
	})
	if rtDirErr != nil {
		t.Fatal(rtDirErr)
	}
	return rtDir
}

// atExit are the functions that TestMain calls after the tests (windows_test.go ends its server).
var atExit []func()

// TestMain removes the extracted runtime and the objects built in it at the end.
func TestMain(m *testing.M) {
	code := m.Run()
	for _, f := range atExit {
		f()
	}
	if rtDir != "" {
		os.RemoveAll(rtDir)
	}
	os.Exit(code)
}

// rtObjects is the runtime's .cpp files compiled for one compiler and one flag set.
type rtObjects struct {
	once sync.Once
	objs []string
	err  error
}

var rtObjSets sync.Map // compiler path + flags -> *rtObjects

// runtimeObjects compiles the runtime's .cpp files (rtfiles.Sources) with c's flags and more,
// once for each compiler and flag set in the test binary, each file in its own command, and
// returns the object files. A program of the tests links them instead of compiling the runtime
// again: with the same flags, this is the same code as one compile of the program and the
// runtime together. If a file doesn't build, every call gets the same error, with the
// compiler's messages (startBuilds reports it once for all the programs that got it).
func (c cxx) runtimeObjects(rt string, more []string) ([]string, error) {
	key := c.cc.Path + "\x00" + c.targetOS() + "\x00" + strings.Join(more, "\x00")
	v, _ := rtObjSets.LoadOrStore(key, new(rtObjects))
	r := v.(*rtObjects)
	r.once.Do(func() {
		dir, err := os.MkdirTemp(rt, "obj-")
		if err != nil {
			r.err = err
			return
		}
		srcs := rtfiles.Sources(c.targetOS())
		r.objs = make([]string, len(srcs))
		r.err = parallel(len(srcs), func(i int) error {
			obj := filepath.Join(dir, fmt.Sprintf("%d.o", i))
			r.objs[i] = obj
			args := append(c.flags(more), "-I", filepath.Join(rt, "include"), "-c",
				filepath.Join(rt, filepath.FromSlash(srcs[i])), "-o", obj)
			return c.compileCmd(prioRT, dir, nil, args)
		})
	})
	return r.objs, r.err
}

// compileCmd runs the compiler with args in dir, with the environment of a driver build plus
// env, and returns an error unless it exits with 0 and prints nothing: no errors, and no
// warnings either. prio is the priority for the gate.
func (c cxx) compileCmd(prio int, dir string, env, args []string) error {
	out, _, code, err := commandPrio(prio, dir, append(build.CompilerEnv(), env...), true, c.cc.Path, args...)
	if err != nil || code != 0 {
		return fmt.Errorf("%s %s: exit %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
	}
	if len(out) != 0 {
		return fmt.Errorf("%s %s printed warnings:\n%s", c.cc.Path, strings.Join(args, " "), out)
	}
	return nil
}

// A prog is a test program to build: a file of runtime/test/, and flags that it has on top of
// its mode's (which the runtime's objects it links have too), and the weight of its compile.
type prog struct {
	src     string
	flags   []string
	weight  int  // about how many seconds its compile takes: the order of the compiles (gate)
	testing bool // also link the natives of package testing (rtfiles.TestingSources)
	bdwgc   bool // also link the vendored bdwgc, built for the mode's sanitizer
}

// builds is the programs of one mode, built at the same time (startBuilds).
type builds struct {
	progs map[string]*built
}

// built is one program of builds: res is valid once done is closed.
type built struct {
	done chan struct{}
	res  buildResult
}

// buildResult is what building a program gave: the binary, or why there is none. own is the
// program's own compile; shared is the failure of something that other programs of the mode
// need too (the runtime's objects), the same error value for all of them.
type buildResult struct {
	bin         string
	own, shared error
}

// reporter is the part of *testing.T that startBuilds uses (a fake in the test of startBuilds).
type reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Cleanup(func())
}

// startBuilds starts building the programs progs, all at the same time with build, and returns
// at once. The test then calls bin for each program, in the subtests that run it, so a program
// runs as soon as it is built, while the others still compile. The compiles wait for the gate
// by the weights of progs. The cleanup of t waits for all builds to end, and then fails t with
// the messages of every build that failed, whether a subtest asked for the program or not (a
// subtest may be skipped), each shared failure once.
func startBuilds(t reporter, progs []prog, build func(p prog) buildResult) *builds {
	t.Helper()
	b := &builds{progs: make(map[string]*built)}
	var wg sync.WaitGroup
	for _, p := range progs {
		x := &built{done: make(chan struct{})}
		b.progs[p.src] = x
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(x.done)
			x.res = build(p)
		}()
	}
	// The cleanup runs before the removal of the folder of the programs (the test's TempDir was
	// made first): the compilers write there.
	t.Cleanup(func() {
		wg.Wait()
		seen := make(map[error]bool)
		for _, p := range progs {
			r := b.progs[p.src].res
			if r.own != nil {
				t.Errorf("%s did not build:\n%v", p.src, r.own)
			}
			if r.shared != nil && !seen[r.shared] {
				seen[r.shared] = true
				t.Errorf("the runtime's objects for %s did not build:\n%v", p.src, r.shared)
			}
		}
	})
	return b
}

// startBuilds starts building the programs progs with the runtime in rt, in the folder dir, with
// the flags of a mode (see the function startBuilds).
func (c cxx) startBuilds(t *testing.T, rt, dir string, flags []string, progs []prog) *builds {
	t.Helper()
	abs := make(map[string]string)
	for _, p := range progs {
		abs[p.src] = sourcePath(t, p.src)
	}
	return startBuilds(t, progs, func(p prog) buildResult {
		return c.buildWith(rt, abs[p.src], dir, slices.Concat(flags, p.flags), p.weight, p.testing, p.bdwgc)
	})
}

// bin returns the binary of the program src (a prog's src), waiting until it is built. If it
// doesn't build, t fails; the compiler's messages are in the failure of the test of the mode.
func (b *builds) bin(t *testing.T, src string) string {
	t.Helper()
	x := b.progs[src]
	<-x.done
	if x.res.own != nil || x.res.shared != nil {
		t.Fatalf("%s did not build (the compiler's messages are in the test of the mode)", src)
	}
	return x.res.bin
}

// build compiles the program src (the absolute path of a file of runtime/test/) with the flags
// and links it with the runtime in rt, and returns the binary, which is in outDir. The compiler
// must print nothing. It compiles src while the runtime's objects are built. It may run in any
// goroutine.
func (c cxx) build(rt, src, outDir string, flags []string, weight int) buildResult {
	return c.buildWith(rt, src, outDir, flags, weight, false, false)
}

// buildWith is build; with testing it also compiles and links the natives of package testing
// (src/testing*.cpp and src/os/<os>/testing*.cpp), which no other program links. With bdwgc it
// also builds the vendored collector for the mode's sanitizer (bdwgcLibrary) and links it.
func (c cxx) buildWith(rt, src, outDir string, flags []string, weight int, testing, bdwgc bool) buildResult {
	var objs []string
	var objsErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		objs, objsErr = c.runtimeObjects(rt, flags)
	}()
	var lib *build.BdwgcLib
	var libErr error
	if bdwgc {
		lib, libErr = c.bdwgcLibrary(rt, flags)
	}
	name := strings.TrimSuffix(filepath.Base(src), ".cpp")
	obj := filepath.Join(outDir, name+".o")
	args := append(c.flags(flags), "-I", filepath.Join(rt, "include"), "-c", src, "-o", obj)
	if lib != nil {
		args = append(args, lib.CompileFlags...)
	}
	err := c.compileCmd(weight, outDir, nil, args)
	<-done
	if libErr != nil {
		return buildResult{own: libErr}
	}
	if err != nil || objsErr != nil {
		return buildResult{own: err, shared: objsErr}
	}
	if testing {
		tsrcs := rtfiles.TestingSources(c.targetOS())
		tobjs := make([]string, len(tsrcs))
		err = parallel(len(tsrcs), func(i int) error {
			tobjs[i] = filepath.Join(outDir, fmt.Sprintf("%s-testing%d.o", name, i))
			args := append(c.flags(flags), "-I", filepath.Join(rt, "include"), "-c",
				filepath.Join(rt, filepath.FromSlash(tsrcs[i])), "-o", tobjs[i])
			return c.compileCmd(prioRT, outDir, nil, args)
		})
		if err != nil {
			return buildResult{own: err}
		}
		objs = append(slices.Clone(objs), tobjs...)
	}
	bin := filepath.Join(outDir, name+c.exe())
	args = append(c.flags(flags), obj)
	args = append(args, objs...)
	if lib != nil {
		args = append(args, lib.Archive)
		args = append(args, lib.LinkFlags...)
	}
	args = append(args, "-o", bin)
	return buildResult{bin: bin, own: c.compileCmd(prioRun, outDir, nil, args)}
}

// bdwgcLibrary builds (once per compiler, target and flag set in this test binary) the vendored
// bdwgc for the sanitizer of flags, through internal/build's API, as the driver will build it.
// bdwgc's build runs its compiles itself (internal/build's executor, Builder.Jobs), which the
// gate of the tests does not see: one library at a time is built, one source at a time.
var (
	bdwgcMu     sync.Mutex
	bdwgcBuilds sync.Map // compiler + target + flags -> *bdwgcBuild
)

type bdwgcBuild struct {
	once sync.Once
	lib  *build.BdwgcLib
	err  error
}

func (c cxx) bdwgcLibrary(rt string, flags []string) (*build.BdwgcLib, error) {
	key := c.cc.Path + "\x00" + c.targetOS() + "\x00" + strings.Join(flags, "\x00")
	v, _ := bdwgcBuilds.LoadOrStore(key, new(bdwgcBuild))
	r := v.(*bdwgcBuild)
	r.once.Do(func() {
		bdwgcMu.Lock()
		defer bdwgcMu.Unlock()
		san := build.BdwgcNoSanitizer
		switch {
		case slices.Contains(flags, "-fsanitize=thread"):
			san = build.BdwgcTSan
		case slices.ContainsFunc(flags, func(f string) bool { return strings.HasPrefix(f, "-fsanitize=address") }):
			san = build.BdwgcASan
		}
		var cflags []string
		for _, f := range flags {
			switch f {
			case "-O0", "-O1", "-O2", "-O3", "-g", "-g0":
				cflags = append(cflags, f)
			}
		}
		work, err := os.MkdirTemp(rt, "bdwgc-work-")
		if err != nil {
			r.err = err
			return
		}
		b := &build.Builder{
			CXX:   c.cc,
			GOOS:  c.targetOS(),
			Cache: filepath.Join(rt, "bdwgc-cache"),
			Work:  work,
			// The gate of the tests bounds the compiles they start themselves, not the ones of
			// internal/build's executor: bdwgc builds one source at a time here, so that the whole
			// test binary stays within its CPU share. (The driver builds it with its own -p.)
			Jobs: 1,
		}
		r.lib, r.err = b.Bdwgc(context.Background(), build.BdwgcConfig{Sanitizer: san, CFlags: cflags})
	})
	return r.lib, r.err
}
