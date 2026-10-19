package main

import (
	"os"
	"os/exec"
	"path/filepath"
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
