package tests

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/wine"
)

// The programs of tests/testdata/windows, built for windows/amd64 and run under Wine (Phase 06
// step 6.13, ADR-055): what the run tests can't say, because the harness gives them no arguments,
// no environment and no exit code of their own. Each one checks a part of the Windows OS layer
// (runtime/src/os/windows): the console and pipe output, the arguments (UTF-16 to UTF-8), the
// environment, process exit, the main thread's stack, threads for goroutines. Like TestRunWindows
// they need GOCPP_TEST_TARGET=windows, wine and a MinGW-w64 compiler.

type windowsProgram struct {
	name   string
	args   []string
	env    []string
	stdout string // the exact stdout, unless check is set
	check  func(t *testing.T, stdout string)
	exit   int
	stderr *regexp.Regexp // stderr must match; nil: stderr must be empty
	phase  int
	// wineGap says why a result is Wine's, not the program's, and the test then skips: it returns the
	// reason for a result that this Wine produces and a real Windows doesn't, or "".
	wineGap func(exit int, stdout, stderr string) string
}

var windowsPrograms = []windowsProgram{
	{name: "hello", stdout: "hello, world\n", phase: 6},
	{name: "goroutines", stdout: "total 18018000\n", phase: 8},
	{
		name: "panic", stdout: "before\n", exit: 2, phase: 6,
		stderr: regexp.MustCompile(`^panic: boom\n\ngoroutine 1 \[running\]:\n`),
	},
	{name: "cexit", stdout: "before exit\n", exit: 3, phase: 6},
	{
		// The command line is read as UTF-16 and given to the program as UTF-8: spaces, quotes and
		// non-ASCII characters keep their meaning (wine's own conversion of the Unix arguments
		// decides which bytes arrive; this checks the runtime's half).
		name: "args", args: []string{"a", "b c", "h\u00e9llo", "\u00e9\u4e16\u754c", `"q"`}, phase: 6,
		check: func(t *testing.T, stdout string) {
			lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
			if len(lines) != 6 {
				t.Fatalf("the program prints %d lines, want 6 (its name and five arguments):\n%s", len(lines), stdout)
			}
			// The ssh runner uploads the program under a name of its own (gp<run>_<n>.exe).
			if !strings.HasSuffix(strings.ToLower(lines[0]), ".exe") {
				t.Errorf("argument 0 is %q, want the program's path", lines[0])
			}
			want := []string{"a", "b c", "h\u00e9llo", "\u00e9\u4e16\u754c", `"q"`}
			if got := lines[1:]; strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("arguments %q, want %q", got, want)
			}
		},
	},
	{name: "env", env: []string{"GOCPP_WIN_VAR=value with spaces=and equals"}, stdout: "value with spaces=and equals\n", phase: 6},
	{name: "env", stdout: "unset\n", phase: 6},
	{
		// The main thread's stack is 8 MiB (-Wl,--stack), a vectored exception handler turns the
		// overflow into Go's message, and the exit code is 2.
		name: "overflow", exit: 2, phase: 6,
		// Wine 11.0 on a CPU with a large XSAVE area (AVX-512) cannot dispatch a stack overflow: it
		// ends the program with exit code 1 and no message (runtime/README.md, "The Windows OS
		// layer"; 14.1's overflow-* cases fail the same way under Wine there). A real PC passes.
		wineGap: func(exit int, stdout, stderr string) string {
			if exit == 1 && stdout == "start\n" && stderr == "" {
				return "this Wine does not dispatch a stack overflow (exit 1, no message; Wine 11.0 on AVX-512 CPUs); GOCPP_WINDOWS_SSH runs it on a real PC"
			}
			return ""
		},
		stdout: "start\n",
		stderr: regexp.MustCompile(`^runtime: goroutine stack exceeds 1000000000-byte limit\n(runtime: sp=0x[0-9a-f]+ stack=\[0x[0-9a-f]+, 0x[0-9a-f]+\]\n)?fatal error: stack overflow\n`),
	},
	{
		// More output than any pipe holds: the writer waits for the reader, and nothing is lost.
		name: "large", phase: 6,
		check: func(t *testing.T, stdout string) {
			if n := strings.Count(stdout, "\n"); n != 60000 {
				t.Fatalf("%d lines, want 60000", n)
			}
			const first = "line 0 of a large output that is longer than any pipe buffer\n"
			const last = "line 59999 of a large output that is longer than any pipe buffer\n"
			if !strings.HasPrefix(stdout, first) || !strings.HasSuffix(stdout, last) {
				t.Errorf("output starts %q and ends %q", firstN(stdout, 80), lastN(stdout, 80))
			}
		},
	},
	{
		// The same through package os: one write of 1.7 MB, then lines by os.Stdout and by fmt, in order.
		name: "osout", phase: 9,
		check: func(t *testing.T, stdout string) {
			lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
			if len(lines) != 20000+200+1 {
				t.Fatalf("%d lines, want 20201", len(lines))
			}
			if lines[0] != "line 0 of a large output written through package os, longer than any pipe buffer" ||
				lines[19999] != "line 19999 of a large output written through package os, longer than any pipe buffer" {
				t.Errorf("the big write starts %q and ends %q", lines[0], lines[19999])
			}
			for i := 0; i < 100; i++ {
				if lines[20000+2*i] != fmt.Sprintf("line %d via WriteString", 20000+i) || lines[20001+2*i] != fmt.Sprintf("line %d via fmt", 20000+i) {
					t.Fatalf("line %d is %q then %q", 20000+2*i, lines[20000+2*i], lines[20001+2*i])
				}
			}
			if lines[20200] != "end" {
				t.Errorf("the last line is %q", lines[20200])
			}
		},
	},
	{
		// To a pipe, text is written as the UTF-8 it is (to a console the runtime writes UTF-16:
		// Phase 14 checks that on a real console).
		name: "unicode", phase: 6, stdout: "h\u00e9llo \u4e16\u754c \U0001F642\n\"\u4e16\" 6\n",
	},
}

// TestWindowsConsole runs tests/testdata/windows/console.gp on a real PC with a console (ssh -tt: a
// pseudo console, ConPTY), which Wine cannot give: the program's output through fmt and os.Stdout
// (WriteConsoleW, with UTF-8 turned into UTF-16), stderr in the same console, and a read of the console
// (ReadConsoleW back to UTF-8), as the user of a terminal window sees it. (Step 14.3.)
func TestWindowsConsole(t *testing.T) {
	if os.Getenv("GOCPP_TEST_TARGET") != "windows" {
		t.Skip("GOCPP_TEST_TARGET=windows turns it on")
	}
	if windowsHost() == "" {
		t.Skip("needs a real Windows PC (GOCPP_WINDOWS_SSH=user@host): Wine has no pseudo console over ssh")
	}
	c := setup(t)
	dir := filepath.Join(repoRoot, "tests", "testdata", "windows")
	exe := filepath.Join(t.TempDir(), "console.exe")
	b := c.run(t, gocppJob(dir, nil, "", 9, "build", "-os", "windows", "-o", exe, "console.gp"))
	if b.problem != "" || b.exit != 0 {
		t.Fatalf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stderr)))
	}
	w := winrunner(t)
	screen, code, err := w.remote.RunConsole(t.Context(), 3*time.Minute, exe, "type: ", "h\u00e9llo w\u00f6rld\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code %d, want 0", code)
	}
	want := "console: <nil> true\n" +
		"fmt: h\u00e9llo \u4e16\u754c \U0001F642\n" +
		"os: h\u00e9llo \u4e16\u754c \U0001F642\n" +
		"err: \u00e9\n" +
		"type: h\u00e9llo w\u00f6rld\n" +
		"read 15 <nil> \"h\u00e9llo w\u00f6rld\\r\\n\"\n"
	if screen != want {
		t.Errorf("the screen shows\n%q\nwant\n%q", screen, want)
	}
}

func firstN(s string, n int) string { return s[:min(len(s), n)] }
func lastN(s string, n int) string  { return s[max(0, len(s)-n):] }

func TestWindowsPrograms(t *testing.T) {
	if os.Getenv("GOCPP_TEST_TARGET") != "windows" {
		t.Skip("GOCPP_TEST_TARGET=windows turns it on")
	}
	c := setup(t)
	if why := windowsUnavailable(); why != "" {
		if os.Getenv("GOCPP_REQUIRE_WINE") == "1" {
			t.Fatalf("GOCPP_REQUIRE_WINE=1, but the Windows mode can't run: %s", why)
		}
		t.Skip(why)
	}
	w := winrunner(t)
	dir := filepath.Join(repoRoot, "tests", "testdata", "windows")
	for _, p := range windowsPrograms {
		name := p.name
		if len(p.env) == 0 && p.name == "env" {
			name += "_unset"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			exe := filepath.Join(t.TempDir(), p.name+".exe")
			b := c.run(t, gocppJob(dir, nil, "", p.phase, "build", "-os", "windows", "-o", exe, p.name+".gp"))
			if b.problem != "" || b.exit != 0 {
				t.Fatalf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stderr)))
			}
			w.sem <- struct{}{}
			defer func() { <-w.sem }()
			r := w.execute(t, c, dir, exe, p.args, p.env, p.name+".exe "+strings.Join(p.args, " "), p.name == "large" || p.name == "osout")
			if r.problem != "" {
				t.Fatalf("%s: %s\nstderr:\n%s", r.cmd, r.problem, indent(string(r.stderr)))
			}
			if p.wineGap != nil {
				if why := p.wineGap(r.exit, string(r.stdout), string(r.stderr)); why != "" && windowsHost() == "" && os.Getenv("GOCPP_REQUIRE_WINE_OVERFLOW") != "1" {
					t.Skip(why)
				}
			}
			if r.exit != p.exit {
				t.Errorf("%s, want exit code %d\nstderr:\n%s", r.status, p.exit, indent(string(r.stderr)))
			}
			if p.check != nil {
				p.check(t, string(r.stdout))
			} else if string(r.stdout) != p.stdout {
				t.Errorf("stdout %q, want %q", r.stdout, p.stdout)
			}
			switch {
			case p.stderr == nil && len(r.stderr) > 0:
				t.Errorf("stderr must be empty, but it is:\n%s", indent(string(r.stderr)))
			case p.stderr != nil && !p.stderr.Match(r.stderr):
				t.Errorf("stderr doesn't match %q:\n%s", p.stderr, indent(string(r.stderr)))
			}
		})
	}
}

// realHomeTraces are what Wine leaves in a real home folder when it is not isolated: the prefix,
// the desktop entries, the menus, the MIME database, the icons, Wine's cache.
func realHomeTraces(home string) map[string]string {
	paths := []string{".wine", ".config/menus", ".config/menus/applications-merged", ".local/share/applications",
		".local/share/mime", ".local/share/icons", ".local/share/desktop-directories", ".cache/wine", ".local/share/wine"}
	snap := make(map[string]string)
	for _, p := range paths {
		full := filepath.Join(home, p)
		fi, err := os.Lstat(full)
		if err != nil {
			snap[p] = "absent"
			continue
		}
		var names []string
		if fi.IsDir() {
			entries, _ := os.ReadDir(full)
			for _, e := range entries {
				info, err := e.Info()
				if err != nil {
					continue
				}
				names = append(names, fmt.Sprintf("%s:%d:%d", e.Name(), info.Size(), info.ModTime().UnixNano()))
			}
			sort.Strings(names)
		}
		snap[p] = fmt.Sprintf("%v %d %s", fi.IsDir(), fi.ModTime().UnixNano(), strings.Join(names, ","))
	}
	return snap
}

// TestWineIsolation is the canary of the Windows mode: a fresh Wine environment makes its prefix,
// runs a program and ends, and nothing of the real home changed (~/.wine, desktop entries, menus,
// icons, MIME files), and no process of the environment survives its Close. It uses its own
// environment and a real .exe built here (hello).
func TestWineIsolation(t *testing.T) {
	if os.Getenv("GOCPP_TEST_TARGET") != "windows" {
		t.Skip("GOCPP_TEST_TARGET=windows turns it on")
	}
	if windowsHost() != "" {
		t.Skip("GOCPP_WINDOWS_SSH is set: the programs run on a real PC, Wine is not used")
	}
	c := setup(t)
	if why := windowsUnavailable(); why != "" {
		if os.Getenv("GOCPP_REQUIRE_WINE") == "1" {
			t.Fatalf("GOCPP_REQUIRE_WINE=1, but the Windows mode can't run: %s", why)
		}
		t.Skip(why)
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("no home folder to watch")
	}
	dir := filepath.Join(repoRoot, "tests", "testdata", "windows")
	exe := filepath.Join(t.TempDir(), "hello.exe")
	b := c.run(t, gocppJob(dir, nil, "", 6, "build", "-os", "windows", "-o", exe, "hello.gp"))
	if b.problem != "" || b.exit != 0 {
		t.Fatalf("%s: %s\n%s", b.cmd, b.why(), indent(string(b.stderr)))
	}
	before := realHomeTraces(home)
	envDir, err := os.MkdirTemp("", "gocpp-wine-canary-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(envDir)
	e, err := wine.New(envDir)
	if err != nil {
		t.Fatal(err)
	}
	var closed bool
	defer func() {
		if !closed {
			e.Close()
		}
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := e.Command(ctx, exe)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		t.Fatalf("wine hello.exe: %v\n%s", err, errb.String())
	}
	if out.String() != "hello, world\n" || errb.Len() != 0 {
		t.Errorf("hello.exe printed %q, stderr %q", out.String(), errb.String())
	}
	if len(e.Processes()) == 0 {
		t.Error("the environment has no process after the run (the server should be up): the hygiene check below would prove nothing")
	}
	e.Close()
	closed = true
	// The prefix is in the environment's own folder.
	if _, err := os.Stat(filepath.Join(e.Prefix(), "system.reg")); err != nil {
		t.Errorf("no prefix in the environment's folder: %v", err)
	}
	if left := e.Processes(); len(left) != 0 {
		var names []string
		for _, pid := range left {
			cmdline, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
			names = append(names, fmt.Sprintf("%d %s", pid, strings.ReplaceAll(string(cmdline), "\x00", " ")))
		}
		t.Errorf("processes of the Wine environment survive Close:\n%s", strings.Join(names, "\n"))
		for _, pid := range left {
			exec.Command("kill", "-9", fmt.Sprint(pid)).Run()
		}
	}
	after := realHomeTraces(home)
	for k, v := range before {
		if after[k] != v {
			t.Errorf("Wine changed %s in the real home folder:\nbefore: %s\nafter:  %s", filepath.Join(home, k), v, after[k])
		}
	}
}

func TestWindowsStderrNormalization(t *testing.T) {
	// A fault's SEH code reads as Go's SIGSEGV line, and Wine's own teardown noise is dropped; a
	// message of the program with a similar look is kept.
	in := "panic: x\n[signal 0xc0000005 code=0x0 addr=0x8 pc=0x401000]\nwine client error:60fc: sendmsg: Socket operation on non-socket\nwine client error:6104: sendmsg: Socket operation on non-socket\nwine client error: kept\n"
	want := "panic: x\n[signal SIGSEGV: segmentation violation code=0x1 addr=0x8 pc=0x401000]\nwine client error: kept\n"
	if got := string(windowsStderr([]byte(in))); got != want {
		t.Errorf("windowsStderr = %q, want %q", got, want)
	}
}
