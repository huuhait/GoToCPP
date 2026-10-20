package tests

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/wine"
	"github.com/huuhait/gocpp/internal/winssh"
)

// The Windows mode of the harness (Phase 06 step 6.13, ADR-055): with GOCPP_TEST_TARGET=windows,
// TestRunWindows builds the run tests for windows/amd64 (gocpp build -os windows, MinGW-w64) and
// runs the .exe files under Wine, and compares stdout, stderr and the exit code with the same
// expectations that TestRun uses (the // Output: block, // ExitCode:, // Stderr:). The default
// is the sample of windows_sample.txt (the OS-independent tests that cover the runtime's pieces:
// goroutines, channels, select, panics, exit codes, maps, strings, fmt); GOCPP_TEST_WINDOWS=all
// runs every run test that windows_skip.txt does not leave out, each with its reason.
//
//   - The builds run in parallel (the harness's usual rule); the programs run under Wine at most
//     GOCPP_WINE_JOBS at a time (default 4): Wine is slow and its server is a single process.
//   - Wine gets its own prefix, HOME and XDG folders under the harness's temporary folder (the
//     internal/wine package: nothing lands in the real home), DISPLAY and WAYLAND_DISPLAY are
//     removed for the program, and the server is ended by its PID at the end of the test.
//   - Without wine or a MinGW-w64 compiler the test is skipped with the reason; GOCPP_REQUIRE_WINE=1
//     makes that a failure (the dev shell has both).
//
// windows_skip.txt has one test a line, "name: reason" (# starts a comment): the tests that are
// not OS-independent (they read /proc, use signals, the file system's Unix paths, ...) or that hit a
// known gap of the Windows runtime (docs/spec/build.md, "What works on Windows").

const (
	windowsSampleFile = "windows_sample.txt"
	windowsSkipFile   = "windows_skip.txt"
)

// readNameList reads a file of the tests folder: one entry a line, "name" or "name: reason".
func readNameList(name string) (map[string]string, []string, error) {
	f, err := os.Open(filepath.Join(repoRoot, "tests", name))
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	m := make(map[string]string)
	var order []string
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, reason, _ := strings.Cut(line, ":")
		key, reason = strings.TrimSpace(key), strings.TrimSpace(reason)
		if _, dup := m[key]; dup {
			return nil, nil, fmt.Errorf("%s:%d: %s is listed twice", name, n, key)
		}
		m[key] = reason
		order = append(order, key)
	}
	return m, order, sc.Err()
}

// windowsTag reports whether the build tag holds for windows/amd64.
func windowsTag(tag string) bool {
	switch tag {
	case "windows", "amd64":
		return true
	}
	if minor, ok := strings.CutPrefix(tag, "go1."); ok {
		n, err := strconv.Atoi(minor)
		return err == nil && n >= 1 && n <= goMinor && strconv.Itoa(n) == minor
	}
	return false
}

// A windowsRunner runs programs on Windows: on a real PC over ssh when GOCPP_WINDOWS_SSH=user@host
// is set (internal/winssh; the main way, as for the runtime's Windows tests), else under Wine.
type windowsRunner struct {
	env    *wine.Env      // Wine; nil when remote is set
	remote *winssh.Remote // the real PC; nil for Wine
	sem    chan struct{}  // the programs that run now
}

// windowsHost is the machine of the ssh runner: GOCPP_WINDOWS_SSH, which only the environment names.
func windowsHost() string { return os.Getenv("GOCPP_WINDOWS_SSH") }

func (w *windowsRunner) close() {
	if w.remote != nil {
		w.remote.Close()
	}
	if w.env != nil {
		w.env.Close()
	}
}

// execute runs the program exe (built for Windows) with the arguments and the extra environment
// (KEY=value) and returns what it did. bigOutput is for a program that prints a megabyte or more.
func (w *windowsRunner) execute(t *testing.T, c *config, dir, exe string, args, env []string, show string, bigOutput bool) result {
	t.Helper()
	if w.remote != nil {
		stop := c.timeout
		if d, ok := t.Deadline(); ok && time.Until(d)-deadlineMargin < stop {
			stop = time.Until(d) - deadlineMargin
		}
		res, err := w.remote.Run(t.Context(), stop, exe, env, args, bigOutput)
		r := result{cmd: show, stdout: res.Stdout, stderr: res.Stderr, exit: res.Code, status: fmt.Sprintf("exit status %d", res.Code)}
		switch {
		case err != nil:
			r.exit, r.problem = -1, err.Error()
		case res.TimedOut:
			r.exit, r.problem = -1, fmt.Sprintf("it timed out after %v (GOCPP_TEST_TIMEOUT changes the limit)", stop)
		}
		return r
	}
	argv := append([]string{"-u", "DISPLAY", "-u", "WAYLAND_DISPLAY", "-u", "WAYLAND_SOCKET", w.env.Wine(), exe}, args...)
	return c.run(t, job{dir: dir, env: append(w.env.Environ(), env...), name: "env", args: argv, show: "wine " + show})
}

var (
	windowsMu   sync.Mutex
	windowsRun  *windowsRunner
	windowsDir  string // the folder of windowsRun's environment
	windowsRefs int    // the tests that use windowsRun now
)

// windowsStop ends the wine server and its helpers when the harness is stopped by a signal
// (stopRunning calls it).
func windowsStop() {
	winssh.Interrupt() // a test that waits for the PC lock (it holds windowsMu while it waits) gives up
	suiteStop()        // the machine of the Windows suite, if it runs (windows_suite_test.go)
	windowsMu.Lock()
	defer windowsMu.Unlock()
	if windowsRun != nil {
		windowsRun.close()
	}
}

// winrunner returns the Wine environment, made on the first call (the prefix takes several seconds)
// in a folder of its own and shared by the tests that run at once: the last one to finish (its
// t.Cleanup) ends the server and removes the folder, and a test that starts later makes a new one.
func winrunner(t *testing.T) *windowsRunner {
	t.Helper()
	windowsMu.Lock()
	defer windowsMu.Unlock()
	if windowsRun == nil && windowsHost() != "" {
		remote, err := winssh.New(windowsHost())
		if err != nil {
			t.Fatal(err)
		}
		windowsRun = &windowsRunner{remote: remote, sem: make(chan struct{}, windowsJobs(t))}
	}
	if windowsRun == nil {
		dir, err := os.MkdirTemp("", "gocpp-wine-")
		if err != nil {
			t.Fatal(err)
		}
		env, err := wine.New(dir)
		if err != nil {
			os.RemoveAll(dir)
			t.Fatal(err)
		}
		windowsRun = &windowsRunner{env: env, sem: make(chan struct{}, windowsJobs(t))}
		windowsDir = dir
	}
	windowsRefs++
	t.Cleanup(func() {
		windowsMu.Lock()
		defer windowsMu.Unlock()
		if windowsRefs--; windowsRefs == 0 {
			windowsRun.close()
			if windowsDir != "" {
				os.RemoveAll(windowsDir)
				windowsDir = ""
			}
			windowsRun = nil
		}
	})
	return windowsRun
}

// windowsJobs is the number of programs that run at once: GOCPP_WINE_JOBS, 4 by default.
func windowsJobs(t *testing.T) int {
	t.Helper()
	if s := os.Getenv("GOCPP_WINE_JOBS"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			t.Fatalf("GOCPP_WINE_JOBS=%s: want a positive number", s)
		}
		return n
	}
	return 4
}

// windowsUnavailable returns why the Windows mode can't run here, or "".
func windowsUnavailable() string {
	if windowsHost() == "" {
		if why := wine.Available(); why != "" {
			return why
		}
	}
	if cxx := os.Getenv("GOCPP_CXX"); cxx != "" {
		return "" // the user named the compiler for the target
	}
	for _, prog := range []string{"x86_64-w64-mingw32-g++", "x86_64-w64-mingw32-clang++"} {
		if _, err := lookPath(prog); err == nil {
			return ""
		}
	}
	if os.Getenv("GOCPP_TOOLCHAIN") == "pinned" {
		return ""
	}
	return "no MinGW-w64 compiler (x86_64-w64-mingw32-g++) in PATH, and GOCPP_CXX is not set"
}

func lookPath(prog string) (string, error) {
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		p := filepath.Join(dir, prog)
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("%s not found", prog)
}

// windowsCounts are the numbers of the summary that TestRunWindows logs.
type windowsCounts struct {
	pass, fail, skip atomic.Int64
	skipped          sync.Map // reason kind -> *atomic.Int64
}

func TestRunWindows(t *testing.T) {
	switch target := os.Getenv("GOCPP_TEST_TARGET"); target {
	case "":
		t.Skip("GOCPP_TEST_TARGET=windows turns it on")
	case "windows":
	default:
		t.Fatalf("GOCPP_TEST_TARGET=%s: want windows", target)
	}
	c := setup(t)
	if why := windowsUnavailable(); why != "" {
		if os.Getenv("GOCPP_REQUIRE_WINE") == "1" {
			t.Fatalf("GOCPP_REQUIRE_WINE=1, but the Windows mode can't run: %s", why)
		}
		t.Skip(why)
	}
	mode := os.Getenv("GOCPP_TEST_WINDOWS")
	if mode != "" && mode != "sample" && mode != "all" {
		t.Fatalf("GOCPP_TEST_WINDOWS=%s: want sample or all", mode)
	}
	skips, _, err := readNameList(windowsSkipFile)
	if err != nil {
		t.Fatal(err)
	}
	_, sampleOrder, err := readNameList(windowsSampleFile)
	if err != nil {
		t.Fatal(err)
	}
	sample := make(map[string]bool)
	for _, n := range sampleOrder {
		sample[n] = true
	}
	progs := loadTests(t, runFormat)
	byName := make(map[string]bool)
	for _, p := range progs {
		byName[p.name] = true
	}
	for name := range skips {
		if !byName[name] {
			t.Errorf("%s lists %s, which is not a run test", windowsSkipFile, name)
		}
	}
	for _, name := range sampleOrder {
		if !byName[name] {
			t.Errorf("%s lists %s, which is not a run test", windowsSampleFile, name)
		}
		if reason, skipped := skips[name]; skipped {
			t.Errorf("%s is in %s and in %s (%s)", name, windowsSampleFile, windowsSkipFile, reason)
		}
	}
	if t.Failed() {
		return
	}
	w := winrunner(t)
	var counts windowsCounts
	where := "Wine"
	if windowsHost() != "" {
		where = "a real PC over ssh"
	}
	t.Cleanup(func() {
		var kinds []string
		counts.skipped.Range(func(k, v any) bool {
			kinds = append(kinds, fmt.Sprintf("%s: %d", k, v.(*atomic.Int64).Load()))
			return true
		})
		sort.Strings(kinds)
		t.Logf("windows/amd64 on %s (%s): passed %d, failed %d, skipped %d (%s)", where,
			map[bool]string{true: "all", false: "sample"}[mode == "all"], counts.pass.Load(), counts.fail.Load(), counts.skip.Load(), strings.Join(kinds, ", "))
	})
	skip := func(t *testing.T, kind, reason string) {
		v, _ := counts.skipped.LoadOrStore(kind, new(atomic.Int64))
		v.(*atomic.Int64).Add(1)
		counts.skip.Add(1)
		t.Skip(reason)
	}
	for _, p := range progs {
		t.Run(p.name, func(t *testing.T) {
			t.Parallel()
			if p.err != nil {
				t.Fatal(p.err)
			}
			switch {
			case phaseGate(p.header.phase, c.phase, c.phaseFrom) != "":
				skip(t, "needs a later phase", phaseGate(p.header.phase, c.phase, c.phaseFrom))
			case p.header.build != nil && !p.header.build.Eval(windowsTag):
				skip(t, "builds for another target", fmt.Sprintf("its %q line excludes windows/amd64", p.header.buildLine))
			case p.header.pending != "" && !c.pending:
				skip(t, "pending", "pending: "+p.header.pending)
			case skips[p.name] != "":
				skip(t, "not OS-independent or a known gap", skips[p.name])
			case mode != "all" && !sample[p.name]:
				skip(t, "not in the sample", "not in "+windowsSampleFile+" (GOCPP_TEST_WINDOWS=all runs every test)")
			}
			passed := c.windowsTest(t, w, p)
			if passed {
				counts.pass.Add(1)
			} else {
				counts.fail.Add(1)
			}
		})
	}
}

// windowsTest builds the run test p for windows/amd64 and runs it under Wine; it reports whether
// it passed.
func (c *config) windowsTest(t *testing.T, w *windowsRunner, p *program) bool {
	t.Helper()
	exeDir := t.TempDir()
	out := filepath.Join(exeDir, p.name+".exe")
	b := c.run(t, gocppJob(p.dir, nil, "", p.header.phase, "build", "-os", "windows", "-o", out, p.arg))
	if b.problem != "" || b.exit != 0 {
		t.Errorf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stderr)))
		return false
	}
	w.sem <- struct{}{}
	defer func() { <-w.sem }()
	r := w.execute(t, c, p.dir, out, nil, nil, p.name+".exe (build of "+p.path+")", false)
	before := t.Failed()
	r.stderr = windowsStderr(r.stderr)
	checkRun(t, r, expect{
		stdout:   p.header.output,
		from:     "the // Output: block",
		exitCode: p.header.exitCode,
		stderr:   p.header.stderr,
	})
	return t.Failed() == before
}

// What differs on Windows in a panic's message (the first lines of stderr), which the tests'
// // Stderr: regexps describe for Linux:
//
//   - a fault is "[signal 0xc0000005 code=0x0 addr=0x0 pc=0x...]" (the SEH exception code, as Go's
//     own Windows runtime prints it) instead of "[signal SIGSEGV: segmentation violation code=0x1
//     ...]": windowsStderr writes the Linux words for it;
//
// The frames of a trace are the same as on Linux: the Windows runtime reads the program's PE file
// (runtime/src/os/windows/trace.cpp, ADR-053).
var sehSignal = regexp.MustCompile(`\[signal 0xc0000005 code=0x0 (addr=0x[0-9a-f]+) (pc=0x[0-9a-f]+)\]`)

// wineNoise is a diagnostic of Wine itself, not of the program: when a process ends while its other
// threads (goroutines that were never woken) still talk to the Wine server, the client library
// prints "wine client error:<tid>: sendmsg: Socket operation on non-socket" on the process's stderr,
// and WINEDEBUG=-all does not silence it (found by the sweep of every run test under load:
// chan_goroutine_leak_ok, once in 827). Only this exact line is removed.
var wineNoise = regexp.MustCompile(`(?m)^wine client error:[0-9a-f]+: sendmsg: Socket operation on non-socket\n`)

func windowsStderr(b []byte) []byte {
	b = wineNoise.ReplaceAll(b, nil)
	return sehSignal.ReplaceAll(b, []byte("[signal SIGSEGV: segmentation violation code=0x1 $1 $2]"))
}
