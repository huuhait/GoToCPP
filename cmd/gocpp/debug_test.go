package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestDebugArgs(t *testing.T) {
	df := &debugFlags{debugger: "gdb", cmds: stringList{"break x.gp:3", "run"}, maps: stringList{"/trim=/src"}, batch: true}
	got := debugArgs(df, "/w/x", []string{"a", "-b"}, "")
	want := []string{"-q", "-batch", "-iex", "add-auto-load-safe-path /w/x", "-iex", "set substitute-path /trim /src",
		"-ex", "break x.gp:3", "-ex", "run", "--args", "/w/x", "a", "-b"}
	if !slices.Equal(got, want) {
		t.Errorf("gdb args:\n got %q\nwant %q", got, want)
	}
	df = &debugFlags{debugger: "lldb", cmds: stringList{"run"}, maps: stringList{"/trim=/src"}}
	got = debugArgs(df, "/w/x", nil, "/w/gocpp_lldb.py")
	want = []string{"-O", "command script import /w/gocpp_lldb.py", "-o", "settings append target.source-map /trim /src", "-o", "run", "--", "/w/x"}
	if !slices.Equal(got, want) {
		t.Errorf("lldb args:\n got %q\nwant %q", got, want)
	}

	// A name with a space is quoted for the debugger; a colon in the program's name (a TMPDIR
	// such as /tmp/a:b) can't be in gdb's safe path list: gdb gets the script by hand.
	df = &debugFlags{debugger: "lldb", maps: stringList{"/a b=/c\"d"}}
	got = debugArgs(df, "/w/x", nil, "/w 1/gocpp_lldb.py")
	want = []string{"-O", `command script import "/w 1/gocpp_lldb.py"`, "-o", `settings append target.source-map "/a b" "/c\"d"`, "--", "/w/x"}
	if !slices.Equal(got, want) {
		t.Errorf("lldb args with spaces:\n got %q\nwant %q", got, want)
	}
	df = &debugFlags{debugger: "gdb", maps: stringList{"/a b=/c"}}
	got = debugArgs(df, "/tmp/a:b/x", nil, "/tmp/a:b/gocpp-gdb.py")
	want = []string{"-q", "-iex", "set auto-load python-scripts off", "-ex", "source /tmp/a:b/gocpp-gdb.py",
		"-iex", `set substitute-path "/a b" /c`, "--args", "/tmp/a:b/x"}
	if !slices.Equal(got, want) {
		t.Errorf("gdb args with a colon:\n got %q\nwant %q", got, want)
	}
}

func TestDebugUsage(t *testing.T) {
	for _, tt := range []struct {
		args []string
		code int
		want string
	}{
		{[]string{"debug"}, 1, "gocpp debug: no .gp files listed\n"},
		{[]string{"debug", "-debugger", "ddd", "x.gp"}, 2, "gocpp debug: -debugger ddd: want gdb or lldb\n"},
		{[]string{"debug", "-map", "nofrom", "x.gp"}, 2, "gocpp debug: -map nofrom: want FROM=TO\n"},
		{[]string{"debug", "-map", "=to", "x.gp"}, 2, "gocpp debug: -map =to: want FROM=TO\n"},
		{[]string{"debug", "-strip", "x.gp"}, 2, "gocpp debug: -strip drops the symbols and the debug information"},
		{[]string{"debug", "-release", "-strip", "x.gp"}, 2, "gocpp debug: -strip drops the symbols"},
	} {
		r := gocppRun("", false, tt.args...)
		if r.code != tt.code || r.stdout != "" || !strings.HasPrefix(r.stderr, tt.want) {
			t.Errorf("gocpp %s = %+v, want exit %d and %q", strings.Join(tt.args, " "), r, tt.code, tt.want)
		}
	}
	r := gocppRun("", false, "debug", "-h")
	if r.code != 0 || !strings.HasPrefix(r.stderr, "usage: gocpp debug") || !strings.Contains(r.stderr, "-debugger") {
		t.Errorf("gocpp debug -h = %+v, want exit 0 and the usage", r)
	}
	r = gocppRun("", false, "debug", "-nosuchflag")
	if r.code != 2 || !strings.Contains(r.stderr, "flag provided but not defined: -nosuchflag") {
		t.Errorf("gocpp debug -nosuchflag = %+v, want exit 2", r)
	}
}

// debugSessionSrc: line 11 is the Println in main, after s and big are set.
const debugSessionSrc = `package main

import "fmt"

func add(a, b int) (int, bool) {
	return a + b, a+b > 10
}

func main() {
	s, big := add(7, 5)
	fmt.Println("hello, gocpp", s, big)
}
`

func TestDebugPrint(t *testing.T) {
	anyCXX(t)
	inDir(t, map[string]string{"hello.gp": debugSessionSrc})
	r := gocppRun("", false, "debug", "-print", "-ex", "break hello.gp:11", "-ex", "run", "-map", "/a=/b", "hello.gp", "arg 1")
	if r.code != 0 || r.stderr != "" {
		t.Fatalf("gocpp debug -print = %+v", r)
	}
	line := strings.TrimSpace(r.stdout)
	if !strings.HasPrefix(line, "gdb -q -iex 'add-auto-load-safe-path ") && !strings.HasPrefix(line, "lldb -O ") {
		t.Errorf("the command line is %q", line)
	}
	for _, want := range []string{"set substitute-path /a /b", "'break hello.gp:11'", "--args", "'arg 1'"} {
		if strings.HasPrefix(line, "gdb") && !strings.Contains(line, want) {
			t.Errorf("the command line %q lacks %q", line, want)
		}
	}
}

// debugSession runs gocpp debug with -batch, the debugger and the commands on hello.gp, and
// skips the test where the debugger is missing or may not trace programs.
func debugSession(t *testing.T, debugger string, cmds ...string) string {
	t.Helper()
	if _, err := exec.LookPath(debugger); err != nil {
		if debugger == "gdb" && requireGDB() {
			t.Fatalf("GOCPP_REQUIRE_GDB=1 needs gdb: %v", err)
		}
		t.Skipf("%s is not installed: %v", debugger, err)
	}
	args := []string{"debug", "-batch", "-debugger", debugger}
	for _, c := range cmds {
		args = append(args, "-ex", c)
	}
	args = append(args, "hello.gp")
	r := gocppRun("", true, args...)
	if strings.Contains(r.stdout, "Operation not permitted") || strings.Contains(r.stdout, "Could not trace") ||
		strings.Contains(r.stdout, "process launch failed") {
		if debugger == "gdb" && requireGDB() {
			t.Fatalf("GOCPP_REQUIRE_GDB=1: %s can't trace programs here:\n%s", debugger, r.stdout)
		}
		t.Skipf("%s can't trace programs here", debugger)
	}
	return r.stdout
}

func requireGDB() bool { return os.Getenv("GOCPP_REQUIRE_GDB") == "1" }

// TestDebugGDB starts gdb through gocpp debug: the program is built (g++ or clang++), gdb stops
// at the .gp line and shows the locals as Go values with the printers that the program carries.
func TestDebugGDB(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": debugSessionSrc})
		out := debugSession(t, "gdb", "break hello.gp:11", "run", "gp locals", "print s", "bt", "continue")
		for _, want := range []string{
			"Breakpoint 1, ",
			"hello.gp:11",
			"s (int) = 12\nbig (bool) = true\n",
			"$1 = 12",
			"#0  main.main () at ",
			"hello, gocpp 12 true",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("the gdb session lacks %q:\n%s", want, out)
			}
		}
	})
}

// TestDebugLLDB is the same with lldb (best effort: the summaries, not the Go frame names).
func TestDebugLLDB(t *testing.T) {
	forEachCXX(t, func(t *testing.T, c testCXX) {
		inDir(t, map[string]string{"hello.gp": debugSessionSrc})
		out := debugSession(t, "lldb", "breakpoint set -f hello.gp -l 11", "process launch", "gp_locals", "continue")
		for _, want := range []string{"hello.gp:11", "s = 12\nbig = true\n", "hello, gocpp 12 true"} {
			if !strings.Contains(out, want) {
				t.Errorf("the lldb session lacks %q:\n%s", want, out)
			}
		}
	})
}

// TestDebugGDBColonTmpdir: a TMPDIR with a colon puts the program where gdb's safe path list
// can't name it; gocpp debug then loads the script by hand and the printers still work.
func TestDebugGDBColonTmpdir(t *testing.T) {
	anyCXX(t)
	tmp := filepath.Join(t.TempDir(), "a:b")
	if err := os.Mkdir(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", tmp)
	inDir(t, map[string]string{"hello.gp": debugSessionSrc})
	out := debugSession(t, "gdb", "break hello.gp:11", "run", "gp locals", "bt", "continue")
	for _, want := range []string{"s (int) = 12\nbig (bool) = true\n", "#0  main.main () at "} {
		if !strings.Contains(out, want) {
			t.Errorf("the gdb session lacks %q:\n%s", want, out)
		}
	}
}

// On a Windows host the program is C:\...\prog.exe: the debugger gets slashes (a backslash starts an
// escape in gdb's commands), and a drive letter is no colon problem (the safe path list of gdb is
// separated by semicolons there).
func TestDebugArgsWindows(t *testing.T) {
	const exe = `C:\Users\me\AppData\Local\Temp\gocpp-build1\prog.exe`
	df := &debugFlags{debugger: "gdb", hostOS: "windows", batch: true, cmds: stringList{"break x.gp:3"},
		maps: stringList{`C:\trim=D:\src dir`}}
	got := debugArgs(df, exe, []string{"a b"}, "")
	want := []string{"-q", "-batch", "-iex",
		"add-auto-load-safe-path C:/Users/me/AppData/Local/Temp/gocpp-build1/prog.exe",
		"-iex", `set substitute-path "C:\\trim" "D:\\src dir"`, "-ex", "break x.gp:3",
		"--args", "C:/Users/me/AppData/Local/Temp/gocpp-build1/prog.exe", "a b"}
	if !slices.Equal(got, want) {
		t.Errorf("gdb args on Windows:\n got %q\nwant %q", got, want)
	}

	t.Run("a folder with spaces is one word of the safe path", func(t *testing.T) {
		df := &debugFlags{debugger: "gdb", hostOS: "windows"}
		got := debugArgs(df, `C:\my work\prog.exe`, nil, "")
		want := []string{"-q", "-iex", "add-auto-load-safe-path C:/my work/prog.exe", "--args", "C:/my work/prog.exe"}
		if !slices.Equal(got, want) {
			t.Errorf("got %q\nwant %q", got, want)
		}
	})
	t.Run("a semicolon in the name is not on the safe path list", func(t *testing.T) {
		df := &debugFlags{debugger: "gdb", hostOS: "windows"}
		got := debugArgs(df, `C:\a;b\prog.exe`, nil, `C:\a;b\gocpp-gdb.py`)
		want := []string{"-q", "-iex", "set auto-load python-scripts off", "-ex", "source C:/a;b/gocpp-gdb.py",
			"--args", "C:/a;b/prog.exe"}
		if !slices.Equal(got, want) {
			t.Errorf("got %q\nwant %q", got, want)
		}
	})
	t.Run("lldb", func(t *testing.T) {
		df := &debugFlags{debugger: "lldb", hostOS: "windows", cmds: stringList{"run"}}
		got := debugArgs(df, `C:\w d\prog.exe`, []string{"x"}, `C:\w d\gocpp_lldb.py`)
		want := []string{"-O", `command script import "C:/w d/gocpp_lldb.py"`, "-o", "run", "--", "C:/w d/prog.exe", "x"}
		if !slices.Equal(got, want) {
			t.Errorf("got %q\nwant %q", got, want)
		}
	})
	t.Run("a colon is fine on Windows and a problem elsewhere", func(t *testing.T) {
		if needsHandLoad("windows", "C:/x/prog.exe") || !needsHandLoad("linux", "/tmp/a:b/prog") {
			t.Error("needsHandLoad is wrong for a drive letter or for a colon in a Linux path")
		}
	})
}

// The messages for a missing debugger name what to install on Windows, and the search goes through
// toolchain.FindTool (a stub here).
func TestDebugNoDebugger(t *testing.T) {
	anyCXX(t)
	inDir(t, map[string]string{"hello.gp": debugSessionSrc})
	old := findTool
	findTool = func(string) (string, string, bool) { return "", "", false }
	t.Cleanup(func() { findTool = old })
	r := gocppRun("", false, "debug", "-batch", "hello.gp")
	if r.code != 1 || !strings.HasPrefix(r.stderr, "gocpp debug: gdb was not found in PATH") {
		t.Fatalf("gocpp debug with no gdb = %+v", r)
	}
	for _, host := range []string{"windows"} {
		for _, name := range []string{"gdb", "lldb"} {
			msg := noDebuggerMessage(name, host)
			if !strings.Contains(msg, "winget") && !strings.Contains(msg, "scoop") && !strings.Contains(msg, "pacman") {
				t.Errorf("the %s message for %s says nothing about installing it: %q", name, host, msg)
			}
		}
	}
}

// defaultDebugger prefers gdb and takes lldb only when gdb is missing.
func TestDefaultDebugger(t *testing.T) {
	old := findTool
	t.Cleanup(func() { findTool = old })
	for _, c := range []struct {
		have []string
		want string
	}{
		{[]string{"gdb", "lldb"}, "gdb"}, {[]string{"lldb"}, "lldb"}, {[]string{"gdb"}, "gdb"}, {nil, "gdb"},
	} {
		findTool = func(name string) (string, string, bool) { return name, "PATH", slices.Contains(c.have, name) }
		if got := defaultDebugger(); got != c.want {
			t.Errorf("with %v installed, defaultDebugger() = %s, want %s", c.have, got, c.want)
		}
	}
}

// A program for another system cannot be started here: the error says so before anything is built,
// and -print still shows the command line (built with MinGW-w64 when it is installed).
func TestDebugCrossTarget(t *testing.T) {
	noTargetEnv(t)
	inDir(t, map[string]string{"hello.gp": debugSessionSrc})
	other := "windows"
	if runtime.GOOS == "windows" {
		other = "linux"
	}
	r := gocppRun("", false, "debug", "-os", other, "hello.gp")
	if r.code != 1 || !strings.Contains(r.stderr, "cannot be debugged on "+runtime.GOOS) ||
		!strings.Contains(r.stderr, "-print") {
		t.Errorf("gocpp debug -os %s = %+v", other, r)
	}
	if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil || runtime.GOOS == "windows" {
		return
	}
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	r = gocppRun("", false, "debug", "-os", "windows", "-print", "-ex", "break hello.gp:11", "hello.gp")
	line := strings.TrimSpace(r.stdout)
	if r.code != 0 || !strings.HasPrefix(line, "gdb -q -iex 'add-auto-load-safe-path ") ||
		!strings.Contains(line, ".exe") ||
		!strings.Contains(line, "'break hello.gp:11'") {
		t.Errorf("gocpp debug -os windows -print = %+v", r)
	}
}

// TestDebugReleaseSaysNoDebugInfo: -release has no debug information (ADR-136), and gocpp debug says so in one
// line, unless the user's flags put it back at the compiles and at the link. -vscode builds nothing, so only the line is looked at.
func TestDebugReleaseSaysNoDebugInfo(t *testing.T) {
	const note = "gocpp debug: -release builds have no debug information (add -cflags=-g and -ldflags=-g to keep it)\n"
	inDir(t, map[string]string{"x.gp": helloSrc})
	for _, tt := range []struct {
		name string
		args []string
		note bool
	}{
		{"release", []string{"-release"}, true},
		{"release with -g", []string{"-release", "-cflags=-g", "-ldflags=-g"}, false},
		{"release with -ggdb in cxxflags", []string{"-release", "-cxxflags=-ggdb", "-ldflags=-ggdb"}, false},
		{"release with -g in -cflags only: the link has -g0", []string{"-release", "-cflags=-g"}, true},
		{"release with -g in -ldflags only", []string{"-release", "-ldflags=-g"}, true},
		{"release with -g, then -g0 at the link", []string{"-release", "-cflags=-g", "-ldflags=-g", "-ldflags=-g0"}, true},
		{"release with -g0", []string{"-release", "-cflags=-g0", "-ldflags=-g0"}, true},
		{"no flag", nil, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := gocppRun("", false, append(append([]string{"debug", "-vscode"}, tt.args...), "x.gp")...)
			if r.code != 0 {
				t.Fatalf("gocpp debug -vscode %v = %+v", tt.args, r)
			}
			if got := strings.Contains(r.stderr, note); got != tt.note {
				t.Errorf("stderr %q: has the note = %v, want %v", r.stderr, got, tt.note)
			}
		})
	}
}

// TestDebugReleaseNoteSeesTheEnvironment: GOCPP_CFLAGS and GOCPP_LDFLAGS count like -cflags and -ldflags.
func TestDebugReleaseNoteSeesTheEnvironment(t *testing.T) {
	inDir(t, map[string]string{"x.gp": helloSrc})
	t.Setenv("GOCPP_CFLAGS", "-g")
	t.Setenv("GOCPP_LDFLAGS", "-g")
	r := gocppRun("", false, "debug", "-vscode", "-release", "x.gp")
	if r.code != 0 || strings.Contains(r.stderr, "no debug information") {
		t.Errorf("gocpp debug -vscode -release with -g in the environment = %+v, want no note", r)
	}
	t.Setenv("GOCPP_LDFLAGS", "")
	r = gocppRun("", false, "debug", "-vscode", "-release", "x.gp")
	if r.code != 0 || !strings.Contains(r.stderr, "no debug information") {
		t.Errorf("gocpp debug -vscode -release with -g only in GOCPP_CFLAGS = %+v, want the note", r)
	}
}
