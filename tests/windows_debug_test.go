package tests

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// gocpp debug on a Windows host (Phase 14 step 14.6): gocpp.exe, cross-built here, builds a program on
// the real PC (GOCPP_WINDOWS_SSH=user@host, never Wine) and starts the PC's own gdb on it in batch
// mode. The PC has WinLibs' gdb (and no lldb); nothing is installed there. Everything lands in the
// run's folder, which nativeRemote deletes.
//
// What it checks: the program carries gocpp's gdb script in its .debug_gdb_scripts section and gdb
// loads it (the program is a safe path; the printers, Go names and `gp` commands work), breakpoints
// on .gp lines, a folder and a TEMP with spaces, gdb found in the folders of an install when PATH
// does not name it, a stack overflow that stops at its .gp line, and a Go panic that `catch throw`
// stops at. lldb is not on the PC: its error message is checked instead.

const debugWinHello = `package main

import "fmt"

func add(a, b int) (int, bool) {
	return a + b, a+b > 10
}

func main() {
	s, big := add(7, 5)
	xs := []int{1, 2, 3}
	m := map[string]int{"a": 1, "b": 2}
	fmt.Println("hello, gocpp", s, big, xs, m)
}
`

const debugWinDeep = `package main

import "fmt"

func down(n int) int {
	return down(n+1) + 1
}

func main() {
	fmt.Println("before")
	fmt.Println(down(0))
}
`

const debugWinBoom = `package main

import "fmt"

func at(xs []int, i int) int {
	return xs[i]
}

func main() {
	xs := []int{1, 2, 3}
	fmt.Println("before")
	fmt.Println(at(xs, 7))
}
`

func TestWindowsDebug(t *testing.T) {
	n := nativeRemote(t)
	// A folder with spaces for the sources, and a TEMP with spaces for the work folder of the build
	// (the program that gdb starts is in it).
	n.put(t, "dbg dir", map[string]string{
		"hello.gp": debugWinHello,
		"deep.gp":  debugWinDeep,
		"boom.gp":  debugWinBoom,
	})
	n.cmd(t, "", nil, `mkdir "tmp dir"`)
	spaced := []string{"TEMP=" + n.abs("tmp dir"), "TMP=" + n.abs("tmp dir")}
	debug := func(t *testing.T, extra []string, args ...string) string {
		t.Helper()
		res := n.gocpp(t, "dbg dir", extra, append([]string{"debug", "-batch"}, args...)...)
		return normNL(res.Stdout) + normNL(res.Stderr)
	}
	needs := func(t *testing.T, out string, wants ...string) {
		t.Helper()
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("the gdb session lacks %q:\n%s", want, out)
			}
		}
	}

	t.Run("breakpoint and printers", func(t *testing.T) {
		out := debug(t, spaced, "-ex", "break hello.gp:13", "-ex", "run", "-ex", "gp locals", "-ex", "print xs",
			"-ex", "print m", "-ex", "bt", "-ex", "gp goroutines", "-ex", "continue", "hello.gp")
		needs(t, out,
			"Breakpoint 1, ",
			"hello.gp:13",
			"s (int) = 12\nbig (bool) = true\n",
			"xs ([]int) = [1 2 3] (len 3, cap 3)\n",
			"$1 = [1 2 3] (len 3, cap 3)",
			`$2 = map["a":1 "b":2] (len 2)`,
			"#0  main.main () at ",
			"hello, gocpp 12 true [1 2 3] map[a:1 b:2]",
			"exited normally",
		)
	})

	t.Run("x says which gdb was started", func(t *testing.T) {
		res := n.gocpp(t, "dbg dir", spaced, "debug", "-x", "-batch", "-ex", "break hello.gp:13", "-ex", "run",
			"-ex", "gp locals", "hello.gp")
		out := normNL(res.Stdout) + normNL(res.Stderr)
		needs(t, out, "s (int) = 12\n")
		if !regexp.MustCompile(`(?m)^# gdb: .*gdb\.exe \(PATH\)$`).MatchString(out) {
			t.Errorf("-x does not say which gdb it started:\n%s", out)
		}
	})

	t.Run("print shows slashes and the program", func(t *testing.T) {
		res := n.gocpp(t, "dbg dir", spaced, "debug", "-print", "-ex", "break hello.gp:13", "hello.gp", "arg 1")
		out := normNL(res.Stdout)
		safePath := regexp.MustCompile(`^gdb -q -iex 'add-auto-load-safe-path [A-Za-z]:/[^\\']*\.exe'`)
		if res.Code != 0 || !safePath.MatchString(out) {
			t.Errorf("gocpp debug -print: exit %d\n%s", res.Code, out)
		}
		if strings.Contains(out, `\`) || !strings.Contains(out, "'arg 1'") {
			t.Errorf("gocpp debug -print has a backslash or lost the argument:\n%s", out)
		}
	})

	t.Run("stack overflow stops at its .gp line", func(t *testing.T) {
		out := debug(t, spaced, "-ex", "run", "-ex", "bt 3", "-ex", "kill", "deep.gp")
		needs(t, out,
			"before",
			"received signal SIGSEGV",
			"deep.gp:6",
			"#0  ", "main.down (n=",
			"#1  ", "main.down (n=",
		)
	})

	t.Run("a Go panic stops at its throw", func(t *testing.T) {
		out := debug(t, spaced, "-ex", "catch throw", "-ex", "run", "-ex", "bt 40", "-ex", "kill", "boom.gp")
		needs(t, out,
			"Catchpoint 1 (exception thrown)",
			"runtime error: index out of range [7] with length 3",
			"main.at (",
			"boom.gp:6",
			"main.main () at ",
			"boom.gp:12",
		)
	})

	t.Run("a panic that is not caught is Go's report", func(t *testing.T) {
		out := debug(t, spaced, "-ex", "run", "boom.gp")
		needs(t, out, "panic: runtime error: index out of range [7] with length 3", "boom.gp:6", "exited with code 02")
	})

	t.Run("gp goroutines finds the goroutine of each thread", func(t *testing.T) {
		src, err := os.ReadFile(filepath.Join(repoRoot, "tests", "gdb", "goroutines.gp"))
		if err != nil {
			t.Fatal(err)
		}
		n.put(t, "dbg dir/gor", map[string]string{"goroutines.gp": string(src)})
		// The breakpoint waits until six goroutines are blocked, so the list never depends on timing
		// (the same session as tests/gdb/goroutines.gdb, whose output on Linux is goroutines.golden).
		res := n.gocpp(t, "dbg dir/gor", spaced, "debug", "-batch", "-ex", "break goroutines.gp:60 if $gp_blocked() == 6",
			"-ex", "run", "-ex", "gp goroutines", "-ex", "kill", "goroutines.gp")
		out := normNL(res.Stdout) + normNL(res.Stderr)
		row := regexp.MustCompile(`(?m)^[* ] +(\d+)  (running|waiting)( \[[a-z ()]+\])? +thread \d+ +` +
			`(main\.[A-Za-z.0-9]+) \(goroutines\.gp:(\d+)\)$`)
		rows := row.FindAllStringSubmatch(out, -1)
		var got []string
		for _, r := range rows {
			got = append(got, strings.Join(r[1:2], "")+" "+r[2]+r[3]+" "+r[4]+":"+r[5])
		}
		want := []string{
			"1 running main.poll:60",
			"2 waiting [chan receive] main.worker:13",
			"3 waiting [chan receive] main.worker:13",
			"4 waiting [chan receive] main.main.func1:34",
			"5 waiting [chan send] main.sender:20",
			"6 waiting [select] main.main.func2:39",
			"7 waiting [select (no cases)] main.main.func3:46",
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("gp goroutines:\n got %q\nwant %q\n\nthe session:\n%s", got, want, out)
		}
	})

	t.Run("lldb is not on the PC: the error says what to install", func(t *testing.T) {
		if res := n.cmd(t, "", nil, `where lldb`); res.Code == 0 {
			t.Skipf("lldb is installed on this PC: %s", strings.TrimSpace(string(res.Stdout)))
		}
		// Nor in a folder that gocpp searches (MSYS2, MinGW, w64devkit, WinLibs, Scoop): then it would
		// find it and the message would not appear.
		found := n.powershell(t, `$d = 'C:\msys64\ucrt64\bin','C:\msys64\clang64\bin','C:\msys64\mingw64\bin',
	'C:\msys2\ucrt64\bin','C:\mingw64\bin','C:\w64devkit\bin','C:\winlibs\mingw64\bin'
$d | Where-Object { Test-Path -LiteralPath (Join-Path $_ 'lldb.exe') }
Get-ChildItem (Join-Path $env:USERPROFILE 'scoop\apps\*\current\bin\lldb.exe') -ErrorAction SilentlyContinue |
	ForEach-Object { $_.FullName }
`)
		if out := strings.TrimSpace(normNL(found.Stdout)); out != "" {
			t.Skipf("lldb is in a folder that gocpp searches: %s", out)
		}
		// With a PATH of the system folders only (and a LOCALAPPDATA that has no WinGet folder): none.
		system := []string{`PATH=C:\Windows\System32;C:\Windows`}
		res := n.gocpp(t, "dbg dir", append(system, spaced...), "debug", "-debugger", "lldb", "-batch", "hello.gp")
		out := normNL(res.Stderr)
		if res.Code != 1 || !strings.Contains(out, "gocpp debug: lldb was not found in PATH") ||
			!strings.Contains(out, "llvm-mingw") {
			t.Errorf("gocpp debug -debugger lldb: exit %d\n%s", res.Code, out)
		}
	})

	t.Run("vscode prints the launch.json with the gdb that was found", func(t *testing.T) {
		res := n.gocpp(t, "dbg dir", nil, "debug", "-vscode", "hello.gp")
		out := normNL(res.Stdout)
		want := regexp.MustCompile(`"miDebuggerPath": "[A-Za-z]:/[^"\\]*/gdb\.exe"`)
		program := `"program": "${workspaceFolder}/bin/hello.exe"`
		if res.Code != 0 || !want.MatchString(out) || !strings.Contains(out, program) {
			t.Errorf("gocpp debug -vscode: exit %d\n%s", res.Code, out)
		}
	})
}
