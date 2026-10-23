//go:build linux

package rtfiles_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/build"
)

// The Windows OS layer's tests (Phase 14, step 14.1; ADR-053): the runtime's C++ tests, built for
// windows/amd64 with the MinGW-w64 cross compiler of the dev shell (-static, as the driver will
// link) and run under Wine. They are skipped, with the reason, when MinGW or Wine is missing, and
// fail instead when GOCPP_REQUIRE_WINE is set (make ci on a machine that must have both).
//
// Wine hygiene (the rules of Phase 06 step 6.13): the prefix, HOME and XDG folders of Wine are in
// a temporary folder of the test binary, never the user's real home; no DISPLAY or Wayland socket
// (wine can't reach a desktop); winemenubuilder is off (no desktop entries); the server is a child
// of the test binary, in its own process group, and dies with it (Pdeathsig); at the end it is
// ended by its PID and then every process that has the prefix in its environment. A canary
// compares ~/.wine, ~/.local/share/applications and ~/.config/menus before and after the run.

// wineOverrides is Wine's WINEDLLOVERRIDES in every process of the tests.
const wineOverrides = "winemenubuilder.exe=d;mscoree,mshtml=;winewayland.drv=d"

// The bounds of one Windows program: a case that must end at once (a hang is the failure) gets
// wineCaseTimeout, a long one (the unit programs, which run many checks, and the Go programs with
// thousands of goroutines) wineLongTimeout. A cold Wine start takes seconds, a loaded machine more.
const (
	wineCaseTimeout = 60 * time.Second
	wineLongTimeout = 4 * time.Minute
)

// winExtraFlags are what a Windows build has on top of baseFlags: the Windows version of the
// headers (Windows 10) and static linking (an .exe that needs no DLL: ADR-053, Phase 14 §2.2).
var winExtraFlags = []string{"-O0", "-D_WIN32_WINNT=0x0A00", "-static"}

// A wineEnv is a Wine prefix with its server.
type wineEnv struct {
	remote *remoteWin // not nil with GOCPP_WINDOWS_SSH: a real Windows machine instead of Wine
	dir    string
	prefix string
	wine   string
	server *exec.Cmd
	exited chan struct{}
}

var (
	wineOnce   sync.Once
	wineShared *wineEnv
	wineErr    error
	canary     []string // the real home's state before Wine started
)

// realHomeState lists what Wine would create in the user's real home if it escaped the isolation:
// the prefix, the desktop entries, the menus, icons and MIME types, every entry below them with its
// modification time and size. Folders that aren't there count too (a new ~/.wine is a change).
func realHomeState() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{"no home: " + err.Error()}
	}
	var st []string
	for _, p := range []string{".wine", ".cache/wine", ".local/share/wineprefixes", ".config/menus", ".local/share/applications", ".local/share/icons", ".local/share/mime"} {
		root := filepath.Join(home, filepath.FromSlash(p))
		if _, err := os.Lstat(root); err != nil {
			st = append(st, p+": absent")
			continue
		}
		filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				st = append(st, fmt.Sprintf("%s: error %v", path, err))
				return nil
			}
			info, err := d.Info()
			if err != nil {
				st = append(st, fmt.Sprintf("%s: error %v", path, err))
				return nil
			}
			st = append(st, fmt.Sprintf("%s: %v %d %v", path, info.ModTime().UnixNano(), info.Size(), info.Mode()))
			return nil
		})
	}
	sort.Strings(st)
	return st
}

// diffLines returns the lines of a that b does not have, marked with tag.
func diffLines(a, b []string, tag string) []string {
	var out []string
	for _, l := range a {
		if !slices.Contains(b, l) {
			out = append(out, tag+": "+l)
		}
	}
	return out
}

// wineSkip skips t with why, or fails it when GOCPP_REQUIRE_WINE is set.
func wineSkip(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("GOCPP_REQUIRE_WINE") != "" {
		t.Fatalf("GOCPP_REQUIRE_WINE is set, but %s", why)
	}
	t.Skip(why)
}

// mingw returns the MinGW-w64 cross compiler as a cxx, or skips the test.
func mingw(t *testing.T) cxx {
	t.Helper()
	cc, err := build.LookCXX(t.Context(), mingwCXX)
	if err != nil {
		wineSkip(t, "no MinGW-w64 cross compiler: "+err.Error())
	}
	return cxx{name: "mingw", cc: cc, goos: "windows"}
}

// wineSetup returns the Wine environment of the test binary, made on first use (it takes a few
// seconds), or skips the test.
func wineSetup(t *testing.T) *wineEnv {
	t.Helper()
	wineOnce.Do(func() {
		if host := os.Getenv("GOCPP_WINDOWS_SSH"); host != "" {
			// A real Windows machine (winremote_test.go): no Wine, no canary.
			r, err := newRemoteWin(host)
			if err != nil {
				wineErr = err
				return
			}
			wineShared = &wineEnv{remote: r}
			atExit = append(atExit, r.close)
			return
		}
		if _, err := exec.LookPath("wine"); err != nil {
			wineErr = errors.New("wine is not in PATH")
			return
		}
		if _, err := exec.LookPath("wineserver"); err != nil {
			wineErr = errors.New("wineserver is not in PATH")
			return
		}
		canary = realHomeState()
		dir, err := os.MkdirTemp("", "gocpp-wine-")
		if err != nil {
			wineErr = err
			return
		}
		e, err := newWine(dir)
		if err != nil {
			os.RemoveAll(dir)
			wineErr = err
			return
		}
		wineShared = e
		atExit = append(atExit, func() {
			e.close()
			os.RemoveAll(dir)
			if after := realHomeState(); !slices.Equal(after, canary) {
				fmt.Fprintf(os.Stderr, "FAIL: Wine changed the real home (entries that differ, at most 30):\n")
				n := 0
				for _, l := range slices.Concat(diffLines(canary, after, "before"), diffLines(after, canary, "after")) {
					if n++; n <= 30 {
						fmt.Fprintln(os.Stderr, "  "+l)
					}
				}
				os.Exit(1)
			}
		})
	})
	if wineErr != nil {
		if os.Getenv("GOCPP_WINDOWS_SSH") != "" {
			t.Fatal(wineErr)
		}
		wineSkip(t, wineErr.Error())
	}
	return wineShared
}

func newWine(dir string) (*wineEnv, error) {
	wine, _ := exec.LookPath("wine")
	wineserver, _ := exec.LookPath("wineserver")
	e := &wineEnv{dir: dir, prefix: filepath.Join(dir, "prefix"), wine: wine}
	home := e.home()
	for _, d := range []string{e.prefix, filepath.Join(home, ".local", "share"), filepath.Join(home, ".config"), filepath.Join(home, ".cache")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return nil, err
		}
	}
	// The server first, in the foreground of a child process whose PID is ours to end: it is
	// persistent (-p), so its cold start (seconds) is paid once.
	e.server = exec.Command(wineserver, "-f", "-p")
	e.server.Env = e.environ()
	e.server.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	if err := e.server.Start(); err != nil {
		return nil, fmt.Errorf("starting wineserver: %w", err)
	}
	e.exited = make(chan struct{})
	go func() { e.server.Wait(); close(e.exited) }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	// The prefix is made here, once, up front: wineboot prints lines on stderr, and its helper
	// processes keep their output open for as long as the server runs, so it writes to a file.
	log, err := os.Create(filepath.Join(dir, "wine.log"))
	if err != nil {
		e.close()
		return nil, err
	}
	defer log.Close()
	for _, args := range [][]string{{"wineboot", "-i"}, {"cmd", "/c", "exit"}} {
		cmd := e.command(ctx, args...)
		cmd.Stdout, cmd.Stderr = log, log
		if err := cmd.Run(); err != nil {
			e.close()
			b, _ := os.ReadFile(log.Name())
			return nil, fmt.Errorf("wine %s: %v\n%s", strings.Join(args, " "), err, b)
		}
	}
	return e, nil
}

func (e *wineEnv) home() string { return filepath.Join(e.dir, "home") }

// environ is the environment of a process that runs Wine for e: the test's own, with the Wine
// variables set, no display, and HOME and XDG folders of e's own.
func (e *wineEnv) environ() []string {
	var env []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		switch name {
		case "DISPLAY", "WAYLAND_DISPLAY", "WAYLAND_SOCKET", "WINEPREFIX", "WINEDEBUG", "WINEDLLOVERRIDES", "WINEARCH", "WINESERVER",
			"HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "XDG_STATE_HOME", "XDG_DATA_DIRS", "XDG_CONFIG_DIRS":
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"WINEPREFIX="+e.prefix, "WINEDEBUG=-all", "WINEDLLOVERRIDES="+wineOverrides,
		"HOME="+e.home(),
		"XDG_DATA_HOME="+filepath.Join(e.home(), ".local", "share"),
		"XDG_CONFIG_HOME="+filepath.Join(e.home(), ".config"),
		"XDG_CACHE_HOME="+filepath.Join(e.home(), ".cache"))
}

// command is "wine args..." in e, in a process group of its own that is killed when ctx ends.
func (e *wineEnv) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, e.wine, args...)
	cmd.Env = e.environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pdeathsig: syscall.SIGKILL}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	return cmd
}

// close ends the server by its PID, then every process that has e's prefix in its environment
// (the helper processes of Wine: services.exe, winedevice.exe, ...), and waits for them.
func (e *wineEnv) close() {
	if e.server != nil && e.server.Process != nil {
		syscall.Kill(-e.server.Process.Pid, syscall.SIGTERM)
		select {
		case <-e.exited:
		case <-time.After(10 * time.Second):
			syscall.Kill(-e.server.Process.Pid, syscall.SIGKILL)
			<-e.exited
		}
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		pids := e.processes()
		if len(pids) == 0 {
			return
		}
		sig := syscall.SIGTERM
		if time.Until(deadline) < 8*time.Second {
			sig = syscall.SIGKILL
		}
		for _, pid := range pids {
			syscall.Kill(pid, sig)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// processes returns the PIDs (not ours) of the processes that have e's prefix in their
// environment.
func (e *wineEnv) processes() []int {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	want := []byte("\x00WINEPREFIX=" + e.prefix + "\x00")
	var pids []int
	for _, ent := range entries {
		pid, err := strconv.Atoi(ent.Name())
		if err != nil || pid == os.Getpid() {
			continue
		}
		env, err := os.ReadFile("/proc/" + ent.Name() + "/environ")
		if err != nil {
			continue
		}
		if bytes.Contains(append([]byte{0}, env...), want) {
			pids = append(pids, pid)
		}
	}
	return pids
}

// A winRun is the result of running a Windows program.
type winRun struct {
	stdout, stderr []byte
	code           int
}

// run runs the Windows program exe with args under Wine, with extra environment variables (the
// runtime's test marker is always set), and returns what it printed and its exit code. The Wine
// processes of one run are in one process group, killed at the timeout. Programs run one at a
// time: the one server of the prefix serializes them anyway, and the machine is shared.
var wineRunMu sync.Mutex

func (e *wineEnv) run(t *testing.T, exe string, env []string, args ...string) winRun {
	t.Helper()
	return e.runTimeout(t, wineCaseTimeout, exe, env, args...)
}

// runLong is run with the long bound.
func (e *wineEnv) runLong(t *testing.T, exe string, env []string, args ...string) winRun {
	t.Helper()
	return e.runTimeout(t, wineLongTimeout, exe, env, args...)
}

func (e *wineEnv) runTimeout(t *testing.T, timeout time.Duration, exe string, env []string, args ...string) winRun {
	t.Helper()
	return e.runVia(t, timeout, false, exe, env, args...)
}

// runViaFiles is run for a program that writes a great deal to a pipe (winremote_test.go says why).
func (e *wineEnv) runViaFiles(t *testing.T, exe string, env []string, args ...string) winRun {
	t.Helper()
	return e.runVia(t, wineCaseTimeout, true, exe, env, args...)
}

func (e *wineEnv) runVia(t *testing.T, timeout time.Duration, viaFiles bool, exe string, env []string, args ...string) winRun {
	t.Helper()
	wineRunMu.Lock()
	defer wineRunMu.Unlock()
	if e.remote != nil {
		return e.remote.run(t, timeout, exe, append([]string{"GOCPP_RT_TEST=env-ok"}, env...), args, viaFiles)
	}
	ctx, cancel := context.WithTimeout(t.Context(), timeout)
	defer cancel()
	cmd := e.command(ctx, append([]string{exe}, args...)...)
	cmd.Env = append(cmd.Env, "GOCPP_RT_TEST=env-ok")
	cmd.Env = append(cmd.Env, env...)
	cmd.Dir = filepath.Dir(exe)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	switch {
	case errors.As(err, &ee):
		code = ee.ExitCode()
	case err != nil:
		t.Fatalf("wine %s %q: %v\nstdout:\n%s\nstderr:\n%s", exe, args, err, out.Bytes(), errb.Bytes())
	}
	return winRun{out.Bytes(), errb.Bytes(), code}
}

// winPrograms are the test programs of runtime/test/ that TestWindowsRuntime runs; the unit ones print "ok:
// N checks" (runUnit). Not here: diff.cpp (TestWindowsDiff), the Linux-only ones.
var winUnitPrograms = []prog{
	{src: "test/unit.cpp", weight: 50},
	{src: "test/map_test.cpp", weight: 24},
	{src: "test/map_swiss_test.cpp", weight: 30},
	{src: "test/map_portable_test.cpp", weight: 30},
	{src: "test/fmt_test.cpp", weight: 14},
	{src: "test/fmt_close_test.cpp", weight: 3},
	{src: "test/interop_test.cpp", weight: 15},
	{src: "test/array_test.cpp", weight: 7},
	{src: "test/slice_test.cpp", weight: 7},
	{src: "test/cxxvalue_test.cpp", weight: 7},
	{src: "test/unsafe_test.cpp", weight: 7},
	{src: "test/trace_test.cpp", weight: 3},
	{src: "test/defer_test.cpp", weight: 3},
	{src: "test/func_test.cpp", weight: 3},
	{src: "test/type_test.cpp", weight: 4},
	{src: "test/iface_test.cpp", weight: 7},
	{src: "test/ifaceconv_test.cpp", weight: 4},
	{src: "test/errors_test.cpp", weight: 4},
	{src: "test/hresult_test.cpp", weight: 3},
	{src: "test/goroutine_test.cpp", weight: 3},
	{src: "test/chan_test.cpp", weight: 4},
	{src: "test/select_test.cpp", weight: 4},
	{src: "test/deadlock_test.cpp", weight: 4},
	{src: "test/time_test.cpp", weight: 6},
	{src: "test/sync_test.cpp", weight: 4},
	{src: "test/nochecks_test.cpp", flags: []string{"-DGOCPP_NOCHECKS"}, weight: 3},
}

// winRunUnit runs a unit program like runUnit: the arguments "arg1" and "arg two", the marker
// variable; it must print "ok: N checks" and nothing on stderr.
func winRunUnit(t *testing.T, e *wineEnv, bin string) {
	t.Helper()
	r := e.runLong(t, bin, nil, "arg1", "arg two")
	if r.code != 0 || len(r.stderr) != 0 || !bytes.HasPrefix(r.stdout, []byte("ok: ")) {
		t.Fatalf("%s: exit code %d\nstdout:\n%s\nstderr:\n%s", filepath.Base(bin), r.code, r.stdout, r.stderr)
	}
	t.Logf("%s", bytes.TrimSpace(r.stdout))
}

// TestWindowsRuntime builds the runtime for windows/amd64 with MinGW-w64 and runs its C++ tests under
// Wine: the unit programs of the portable runtime (strings, slices, maps, fmt, interfaces,
// defer, goroutines, channels, select, interop, the stack trace), the thread layer's cases, and
// test/winos_test.cpp, which tests the Windows OS layer itself. The subtests run one at a time
// (Wine's server serializes them anyway); the programs are built at the same time.
func TestWindowsRuntime(t *testing.T) {
	c := mingw(t)
	e := wineSetup(t)
	rt := extractRuntime(t)
	// GOCPP_WINDOWS_PROGS=winos_test,thread_test builds and runs only those programs (a quick run
	// while working on one file of the layer; mutation checks).
	only := strings.Split(os.Getenv("GOCPP_WINDOWS_PROGS"), ",")
	want := func(src string) bool {
		return os.Getenv("GOCPP_WINDOWS_PROGS") == "" || slices.Contains(only, strings.TrimSuffix(filepath.Base(src), ".cpp"))
	}
	var progs []prog
	for _, p := range slices.Concat(winUnitPrograms, []prog{
		{src: "test/winos_test.cpp", weight: 5},
		{src: "test/thread_test.cpp", weight: 3},
		{src: "test/testing_natives_test.cpp", weight: 4, testing: true},
	}) {
		if want(p.src) {
			progs = append(progs, p)
		}
	}
	bins := c.startBuilds(t, rt, t.TempDir(), winExtraFlags, progs)

	t.Run("units", func(t *testing.T) {
		for _, p := range winUnitPrograms {
			name := strings.TrimSuffix(filepath.Base(p.src), ".cpp")
			if !want(p.src) {
				continue
			}
			t.Run(name, func(t *testing.T) {
				bin := bins.bin(t, p.src)
				winRunUnit(t, e, bin)
				if name == "deadlock_test" {
					runWinDeadlock(t, e, bin)
				}
			})
		}
	})
	if want("test/winos_test.cpp") {
		t.Run("winos", func(t *testing.T) { runWinOS(t, e, bins.bin(t, "test/winos_test.cpp")) })
	}
	if want("test/thread_test.cpp") {
		t.Run("thread", func(t *testing.T) { runWinThread(t, e, bins.bin(t, "test/thread_test.cpp")) })
	}
	// The natives of package testing for Windows (src/os/windows/testing_os.cpp); the same program
	// runs on Linux in TestTestingNatives.
	if want("test/testing_natives_test.cpp") {
		t.Run("testing-natives", func(t *testing.T) { winRunUnit(t, e, bins.bin(t, "test/testing_natives_test.cpp")) })
	}
}

// runWinDeadlock runs the cases of test/deadlock_test.cpp (deadlock_test.go): the report of the
// deadlock detector, the Goexit message, and the programs in which nothing is a deadlock. The goroutines'
// stacks are captured on their own threads, so the Windows OS layer's stack walk is what is tested.
func runWinDeadlock(t *testing.T, e *wineEnv, bin string) {
	for _, c := range deadlockCases {
		t.Run(c.name, func(t *testing.T) {
			r := e.run(t, bin, nil, c.name)
			out := strings.ReplaceAll(string(r.stdout)+string(r.stderr), "\r\n", "\n")
			if r.code != c.exit || !regexp.MustCompile(c.rx).MatchString(out) {
				t.Errorf("%s: exit code %d, output:\n%s\nwant exit code %d and %s", c.name, r.code, out, c.exit, c.rx)
			}
		})
	}
}

// --- test/winos_test.cpp ---

func runWinOS(t *testing.T, e *wineEnv, bin string) {
	// The tests in the process.
	t.Run("unit", func(t *testing.T) {
		r := e.runLong(t, bin, nil, "arg1", "arg two")
		if r.code != 0 || len(r.stderr) != 0 || !bytes.HasPrefix(r.stdout, []byte("ok: ")) {
			t.Fatalf("exit code %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
		}
		t.Logf("%s", bytes.TrimSpace(r.stdout))
	})

	// What standard output holds, byte for byte: Go writes bytes, never "\r\n".
	bigWant := func() []byte {
		b := make([]byte, 4<<20)
		for i := range b {
			b[i] = byte(i*7 + i/251)
		}
		return b
	}()
	all := make([]byte, 256)
	for i := range all {
		all[i] = byte(i)
	}
	outputs := []struct {
		name string
		want []byte
	}{
		{"out-newline", []byte("a\nb\n\nc")},
		{"out-utf8", []byte("héllo 日本語 \U0001F389\n")},
		{"out-binary", all},
		{"out-split", []byte("a 日 b\n")},
		{"out-big", bigWant},
		{"print", []byte("")}, // println writes to stderr: checked below
	}
	for _, tc := range outputs {
		t.Run(tc.name, func(t *testing.T) {
			r := e.run(t, bin, nil, tc.name)
			if tc.name == "print" {
				if r.code != 0 || string(r.stderr) != "hello 42 true\nno newline" || len(r.stdout) != 0 {
					t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
				}
				return
			}
			if r.code != 0 || len(r.stderr) != 0 {
				t.Errorf("exit code %d, stderr %q", r.code, r.stderr)
			}
			if !bytes.Equal(r.stdout, tc.want) {
				t.Errorf("stdout: got %d bytes (sha256 %x), want %d bytes (sha256 %x)\ngot %.200q", len(r.stdout), sha256.Sum256(r.stdout), len(tc.want), sha256.Sum256(tc.want), r.stdout)
			}
		})
	}
	t.Run("out-stdio", func(t *testing.T) {
		// C's printf: bytes as they are (no "\r\n": the layer sets binary mode), flushed at exit.
		r := e.run(t, bin, nil, "out-stdio")
		if r.code != 0 || string(r.stdout) != "printf line\nfputs line\n" || string(r.stderr) != "to stderr\n" {
			t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
		}
	})
	t.Run("out-err-order", func(t *testing.T) {
		r := e.run(t, bin, nil, "out-err-order")
		var wantOut, wantErr string
		for i := range 5 {
			wantOut += fmt.Sprintf("out%d\n", i)
			wantErr += fmt.Sprintf("err%d\n", i)
		}
		if r.code != 0 || string(r.stdout) != wantOut || string(r.stderr) != wantErr {
			t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
		}
	})

	// Arguments: Go's rules for the command line, UTF-8 from UTF-16. Wine builds the command line
	// from its own arguments with the C runtime's quoting, so each of these round-trips.
	t.Run("args", func(t *testing.T) {
		args := []string{"a b", `q"uote`, "héllo 日本", "", `back\slash`, `tail\`, "x  y", "\U0001F389", `two\\`, "tab\there"}
		r := e.run(t, bin, nil, append([]string{"args"}, args...)...)
		want := "4:args\n"
		for _, a := range args {
			want += fmt.Sprintf("%d:%s\n", len(a), a)
		}
		if r.code != 0 || string(r.stdout) != want || len(r.stderr) != 0 {
			t.Errorf("exit %d, stdout:\n%s\nwant:\n%s\nstderr %q", r.code, r.stdout, want, r.stderr)
		}
	})
	t.Run("env", func(t *testing.T) {
		vars := []string{"GOCPP_WIN_UTF8=héllo 日本 \U0001F389", "GOCPP_WIN_EMPTY=", "GOCPP_WIN_EQ=a=b=c"}
		wantEmpty := "GOCPP_WIN_EMPTY=\n"
		if e.remote != nil {
			// cmd.exe's "set NAME=" removes a variable: it cannot carry an empty one.
			vars = []string{vars[0], vars[2]}
			wantEmpty = "GOCPP_WIN_EMPTY=-\n"
		}
		r := e.run(t, bin, vars,
			"env", "GOCPP_WIN_UTF8", "GOCPP_WIN_EMPTY", "GOCPP_WIN_EQ", "GOCPP_WIN_UNSET", "GOCPP_RT_TEST")
		want := "GOCPP_WIN_UTF8=héllo 日本 \U0001F389\n" + wantEmpty + "GOCPP_WIN_EQ=a=b=c\nGOCPP_WIN_UNSET=-\nGOCPP_RT_TEST=env-ok\n"
		if r.code != 0 || string(r.stdout) != want {
			t.Errorf("exit %d, stdout:\n%s\nwant:\n%s\nstderr %q", r.code, r.stdout, want, r.stderr)
		}
	})

	// The ends of a process.
	exits := []struct {
		name   string
		code   int
		stdout string
	}{
		{"exit-7", 7, ""},
		{"exit-now-9", 9, "before\n"},
		// Like Go's os.Exit: no atexit function, no static destructor.
		{"exit-atexit", 0, "main\n"},
		// C stdio is flushed at the exit of the program.
		{"exit-stdio-flush", 3, "buffered\n"},
		// os.Exit from a goroutine's thread ends the whole process with its code.
		{"exit-thread", 5, ""},
		{"exit-main-return", 0, ""},
	}
	for _, tc := range exits {
		t.Run(tc.name, func(t *testing.T) {
			r := e.run(t, bin, nil, tc.name)
			if r.code != tc.code || string(r.stdout) != tc.stdout || len(r.stderr) != 0 {
				t.Errorf("exit %d, stdout %q, stderr %q; want exit %d, stdout %q, no stderr", r.code, r.stdout, r.stderr, tc.code, tc.stdout)
			}
		})
	}

	// A panic: Go's message and the goroutine, exit code 2. The trace names the function of this
	// test file, with its line.
	panics := []struct {
		name, msg, fn string
		gor           string
	}{
		{"panic", "boom", "run_case", "1"},
		{"panic-goroutine", "in goroutine", "run_case", "[2-9]|\\d\\d+"},
		{"cxx-exception", "C++ exception std::runtime_error: plain C++", "run_case", "1"},
	}
	for _, tc := range panics {
		t.Run(tc.name, func(t *testing.T) {
			r := e.run(t, bin, nil, tc.name)
			s := string(r.stderr)
			rx := regexp.MustCompile(`^panic: ` + regexp.QuoteMeta(tc.msg) + `\n\ngoroutine (` + tc.gor + `) \[running\]:\n\S[^\n]*` + tc.fn + `[^\n]*\n\t[^\n]*winos_test\.cpp:\d+ \+0x[0-9a-f]+\n`)
			if r.code != 2 || len(r.stdout) != 0 || !rx.MatchString(s) {
				t.Errorf("exit %d, stdout %q, stderr:\n%s\nwant exit 2 and Go's panic with a trace through %s", r.code, r.stdout, s, tc.fn)
			}
		})
	}

	// Faults of the CPU that nothing handles: Go's report, exit code 2.
	nilLine := `panic: runtime error: invalid memory address or nil pointer dereference\n\[signal 0xc0000005 code=0x%s addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine (\d+) \[running\]:\n`
	faults := []struct {
		name  string
		rx    string
		frame string // the function of the trace's first line
		gor   string
	}{
		{"fault-read", fmt.Sprintf(nilLine, "0"), "fault_read", "1"},
		{"fault-write", fmt.Sprintf(nilLine, "1"), "fault_write", "1"},
		{"fault-thread", fmt.Sprintf(nilLine, "0"), "fault_read", "2"},
		{"fault-wild", `unexpected fault address 0x100000000000\nfatal error: fault\n\[signal 0xc0000005 code=0x1 addr=0x100000000000 pc=0x[0-9a-f]+\]\n\ngoroutine (\d+) \[running\]:\n`, "fault_wild", "1"},
		{"fault-divzero", `panic: runtime error: integer divide by zero\n\[signal 0xc0000094 code=0x0 addr=0x0 pc=0x[0-9a-f]+\]\n\ngoroutine (\d+) \[running\]:\n`, "divide", "1"},
		{"fault-illegal", `Exception 0xc000001d 0x0 0x0 0x[0-9a-f]+\nPC=0x[0-9a-f]+\n\ngoroutine (\d+) \[running\]:\n`, "run_case", "1"},
	}
	for _, tc := range faults {
		t.Run(tc.name, func(t *testing.T) {
			r := e.run(t, bin, nil, tc.name)
			rx := regexp.MustCompile(`^` + tc.rx + `[^\n]*` + tc.frame + `[^\n]*\n\t[^\n]*winos_test\.cpp:\d+ \+0x[0-9a-f]+\n`)
			m := rx.FindStringSubmatch(string(r.stderr))
			if r.code != 2 || len(r.stdout) != 0 || m == nil || m[1] != tc.gor {
				t.Errorf("exit %d, stdout %q, stderr:\n%s\nwant exit 2, Go's report, goroutine %s, first frame %s", r.code, r.stdout, r.stderr, tc.gor, tc.frame)
			}
		})
	}
	t.Run("fault-hook", func(t *testing.T) {
		r := e.run(t, bin, nil, "fault-hook")
		if r.code != 11 || string(r.stdout) != "hook: code=3221225477 access=1 addr=0 pcs=ok\n" || len(r.stderr) != 0 {
			t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
		}
	})
	t.Run("fault-hook-returns", func(t *testing.T) {
		// A hook that returns leaves the fault to Windows: the process dies of the access
		// violation (exit status 0xc0000005; Wine's launcher shows the low byte).
		r := e.run(t, bin, nil, "fault-hook-returns")
		if r.code != 0xc0000005 && r.code != 5 || len(r.stdout) != 0 {
			t.Errorf("exit %#x, stdout %q, stderr %q; want the access violation", r.code, r.stdout, r.stderr)
		}
	})
}

// --- test/thread_test.cpp ---

// runWinThread runs the unit tests of the thread layer (tests of the Linux layer, here on
// CreateThread) and its cases that end the process: the stack overflows with Go's report (main
// thread and threads of thread_start, one report for several threads), the hook, the faults.
func runWinThread(t *testing.T, e *wineEnv, bin string) {
	t.Run("unit", func(t *testing.T) {
		r := e.runLong(t, bin, nil, "arg1", "arg two")
		if r.code != 0 || len(r.stderr) != 0 || !bytes.HasPrefix(r.stdout, []byte("ok: ")) {
			t.Fatalf("exit code %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
		}
		t.Logf("%s", bytes.TrimSpace(r.stdout))
	})
	overflow := func(name string, minStack uint64) {
		t.Run(name, func(t *testing.T) {
			r := e.run(t, bin, nil, name)
			out := string(r.stdout) + string(r.stderr)
			m := overflowRx.FindStringSubmatch(out)
			if r.code != 2 || m == nil {
				t.Fatalf("%s: exit code %d, output:\n%s\nwant Go's stack overflow report and exit code 2", name, r.code, out)
			}
			sp, lo, hi := parseHex(t, m[1]), parseHex(t, m[2]), parseHex(t, m[3])
			if !(lo < hi && hi-lo >= minStack && sp < hi && sp+(1<<20) >= lo) {
				t.Errorf("%s: sp=%#x stack=[%#x, %#x]: want a stack of at least %d bytes with sp near its end", name, sp, lo, hi, minStack)
			}
			switch {
			case name == "overflow-main" && m[4] != "1":
				t.Errorf("%s: goroutine %q, want 1:\n%s", name, m[4], out)
			case strings.Contains(name, "-thread") && m[4] != "":
				t.Errorf("%s: goroutine %q, want none (a thread has no record):\n%s", name, m[4], out)
			}
			if strings.Count(out, "fatal error") != 1 {
				t.Errorf("%s: more than one report:\n%s", name, out)
			}
		})
	}
	overflow("overflow-main", 64<<10)
	overflow("overflow-thread", 256<<10)
	overflow("overflow-thread-default", 8<<20)
	overflow("overflow-thread-big-frames", 1<<20)
	overflow("overflow-thread-many", 256<<10)
	overflow("overflow-thread-huge-frame", 1<<20)
	for _, name := range []string{"overflow-hook-thread", "overflow-hook-main"} {
		t.Run(name, func(t *testing.T) {
			r := e.run(t, bin, nil, name)
			if r.code != 7 || string(r.stdout)+string(r.stderr) != "hook: sp near the stack, bounds as stack_bounds\n" {
				t.Errorf("exit %d, stdout %q, stderr %q", r.code, r.stdout, r.stderr)
			}
		})
	}
	// A fault that is not a stack overflow: the nil pointer panic (Linux's layer leaves a
	// crash; Windows gets Go's report), in the main thread and in a thread; and the overflow of a
	// thread that the runtime didn't start has no bounds, so no report of ours: Windows' crash.
	for _, name := range []string{"segv-main", "segv-thread", "wild-near-end-thread"} {
		t.Run(name, func(t *testing.T) {
			r := e.run(t, bin, nil, name)
			out := string(r.stdout) + string(r.stderr)
			if r.code != 2 || !strings.Contains(out, "panic: runtime error: invalid memory address or nil pointer dereference\n[signal 0xc0000005 code=0x1 addr=0x40 pc=0x") {
				t.Errorf("exit %d, output:\n%s", r.code, out)
			}
		})
	}
	t.Run("overflow-foreign", func(t *testing.T) {
		// A std::thread has no bounds: no hook. Windows then runs the thread's own code past
		// its guard page, which is the OS's stack overflow, handled as a crash of the process.
		r := e.run(t, bin, nil, "overflow-foreign")
		out := string(r.stdout) + string(r.stderr)
		if strings.Contains(out, "fatal error: stack overflow") {
			t.Errorf("a thread without bounds got Go's report:\n%s", out)
		}
		// (Real Windows ends the process with 0xc00000fd; Wine ends only the thread, and the
		// program goes on: the exit code isn't checked.)
	})
}

var signalCodeRx = regexp.MustCompile(`( code=)0x[01]( addr=)`)

// TestWindowsDiff is the differential test of the portable runtime (test/diff.cpp against
// testdata/diff/main.go: panics, runtime errors, number formatting, exit codes), for Windows:
// both programs are built for windows/amd64 (the C++ with MinGW-w64, the Go with real Go) and run
// under Wine, and what they print up to the goroutine line, and the exit code, must be the same.
func TestWindowsDiff(t *testing.T) {
	c := mingw(t)
	e := wineSetup(t)
	gocmd, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("no go command: %v", err)
	}
	rt := extractRuntime(t)
	bins := c.startBuilds(t, rt, t.TempDir(), winExtraFlags, []prog{{src: "test/diff.cpp", weight: 15}})

	dir := t.TempDir()
	goBin := filepath.Join(dir, "godiff.exe")
	goEnv := append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "GOAMD64=v1", "CGO_ENABLED=0")
	out, _, code, err := command(".", goEnv, true, gocmd, "build", "-o", goBin, sourcePath(t, "testdata/diff/main.go"))
	if err != nil || code != 0 {
		t.Fatalf("go build for windows: exit %d %v\n%s", code, err, out)
	}
	cppBin := bins.bin(t, "test/diff.cpp")
	// As in goProgramResults: no asynchronous preemption, so no SIGURG-like interruptions.
	goRunEnv := []string{"GODEBUG=asyncpreemptoff=1"}
	for _, name := range diffCases {
		t.Run(name, func(t *testing.T) {
			want := e.runViaFiles(t, goBin, goRunEnv, name)
			got := e.run(t, cppBin, nil, name)
			w := maskPC(cutTrace(append(append([]byte{}, want.stdout...), want.stderr...)))
			g := maskPC(cutTrace(append(append([]byte{}, got.stdout...), got.stderr...)))
			if name == "nil-store" {
				// Go on Windows prints code=0x1 for a write and code=0x0 for a read; gocpp checks
				// the pointer before it knows which one follows, and prints the read's. (A fault
				// of C code does print the right one: the winos test "fault-write".)
				w = signalCodeRx.ReplaceAll(w, []byte("${1}N${2}"))
				g = signalCodeRx.ReplaceAll(g, []byte("${1}N${2}"))
			}
			if got.code != want.code || !bytes.Equal(g, w) {
				t.Errorf("gocpp: exit %d, output:\n%.1500q\nGo: exit %d, output:\n%.1500q", got.code, g, want.code, w)
			}
		})
	}
}

// TestWindowsDriverFlags builds the Windows OS layer's tests with the flags the 6.13 driver adds for
// a Windows program (-pthread, -ffunction-sections -fdata-sections, -Wl,--gc-sections, and
// -Wl,--stack,1073741824), and without the stack flag: the linker directive in os.cpp that asks for
// the 1 GiB main stack must survive --gc-sections, and agree with the flag (test_main_thread_bounds
// checks the size). Only GNU ld reads the directive; the clang toolchain relies on the flag.
func TestWindowsDriverFlags(t *testing.T) {
	c := mingw(t)
	e := wineSetup(t)
	rt := extractRuntime(t)
	gc := []string{"-pthread", "-ffunction-sections", "-fdata-sections", "-Wl,--gc-sections"}
	for _, tc := range []struct {
		name  string
		extra []string
	}{
		{"gc-sections", gc},
		{"gc-sections-and-stack-flag", append(slices.Clone(gc), "-Wl,--stack,1073741824")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bins := c.startBuilds(t, rt, t.TempDir(), slices.Concat(winExtraFlags, tc.extra), []prog{{src: "test/winos_test.cpp", weight: 5}})
			bin := bins.bin(t, "test/winos_test.cpp")
			r := e.runLong(t, bin, nil, "arg1", "arg two")
			if r.code != 0 || len(r.stderr) != 0 || !bytes.HasPrefix(r.stdout, []byte("ok: ")) {
				t.Fatalf("exit code %d\nstdout:\n%s\nstderr:\n%s", r.code, r.stdout, r.stderr)
			}
		})
	}
}
