package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/huuhait/gocpp/internal/winssh"
)

// gocpp.exe running NATIVELY on Windows (Phase 14 step 14.2, ADR-081): the build driver on a Windows
// host, not the cross build of TestRunWindows. gocpp.exe is cross-built here, copied to the real PC
// over ssh (GOCPP_WINDOWS_SSH=user@host; the repository never names a machine) with the sources of
// each case, and run there with the PC's own MinGW-w64 (WinLibs) in PATH. Everything lands in the
// run's folder %USERPROFILE%\gocpp-wintest\<run id>\, which the test deletes at the end; the cache
// (LOCALAPPDATA), the temporary files (TEMP) and the module cache are inside it too, so nothing of the
// user's profile is read or written and every run starts with an empty cache.
//
// The cases: the compiler is found (and what the message says when there is none), hello with
// the .exe, the manifest and compile_commands.json; gocpp run and its exit code; CRLF sources and
// gocpp.mod; a folder with spaces; a module whose path is longer than 260 characters; a DLL with
// its import library and a C program that calls it, and a static library; programs that use C++
// libraries (the interop tests that need only std); a command line that no CreateProcess takes
// (response files); errors with Windows paths; two builds at once. GOCPP_TEST_DOWNLOAD=1 adds the
// download of the pinned llvm-mingw (190 MB; the toolchain is deleted afterwards). Windows-only unit
// tests of the packages (job objects, LockFileEx, MoveFileEx retries, paths) run on the PC as
// well (TestWindowsNativeUnits).

type native struct {
	r    *winssh.Remote
	home string // %USERPROFILE%
	exe  string // gocpp.exe, relative to the run folder
	env  []string

	scripts int // the PowerShell scripts written so far
}

var nativeMu sync.Mutex

// nativeRemote connects to the PC (a failure to connect is a failure of the test, not a skip) and
// builds gocpp.exe for it.
func nativeRemote(t *testing.T) *native {
	t.Helper()
	host := windowsHost()
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host turns it on: it runs gocpp.exe on a real Windows PC")
	}
	r, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is not reachable: %v", err)
	}
	t.Cleanup(r.Close)
	ctx := t.Context()
	home, err := r.Home(ctx)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "gocpp.exe")
	build := exec.CommandContext(ctx, "go", "build", "-o", bin, "./cmd/gocpp")
	build.Dir = repoRoot
	build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build for windows: %v\n%s", err, out)
	}
	if err := r.Put(ctx, bin, "gocpp.exe"); err != nil {
		t.Fatal(err)
	}
	n := &native{r: r, home: home, exe: "gocpp.exe"}
	run := home + `\` + r.Dir()
	n.env = []string{
		"LOCALAPPDATA=" + run + `\lad`, "APPDATA=" + run + `\appdata`, "TEMP=" + run + `\tmp`, "TMP=" + run + `\tmp`,
		"GOCPP_OFFLINE=1", "GOCPP_MODCACHE=off", "GOCPP_TOOLCHAIN=local", "GOCPP_GATE_PHASE=15",
	}
	n.cmd(t, "", nil, `mkdir lad appdata tmp`)
	return n
}

// abs returns the absolute Windows path of rel, a path below the run's folder.
func (n *native) abs(rel string) string {
	p := n.home + `\` + n.r.Dir()
	if rel != "" {
		p += `\` + strings.ReplaceAll(rel, "/", `\`)
	}
	return p
}

// quote quotes the arguments for cmd.exe and the C runtime's rules.
func quote(args ...string) string {
	var out []string
	for _, a := range args {
		out = append(out, winssh.EscapeArg(a))
	}
	return strings.Join(out, " ")
}

// cmd runs a cmd.exe line in the folder sub of the run (a path with slashes, "" for the run's folder)
// with the base environment and extra, and returns what it did.
func (n *native) cmd(t *testing.T, sub string, extra []string, line string) winssh.Result {
	t.Helper()
	if sub != "" {
		line = `cd /d ` + quote(strings.ReplaceAll(sub, "/", `\`)) + ` && ` + line
	}
	timeout := 10 * time.Minute
	if d, ok := t.Deadline(); ok && time.Until(d)-deadlineMargin < timeout {
		timeout = time.Until(d) - deadlineMargin
	}
	res, err := n.r.Exec(context.Background(), timeout, append(n.env[:len(n.env):len(n.env)], extra...), line)
	if err != nil {
		t.Fatal(err)
	}
	if res.TimedOut {
		t.Fatalf("%s: it timed out after %v\nstdout:\n%s\nstderr:\n%s", line, timeout, res.Stdout, res.Stderr)
	}
	return res
}

// gocpp runs gocpp.exe with args in the folder sub.
func (n *native) gocpp(t *testing.T, sub string, extra []string, args ...string) winssh.Result {
	t.Helper()
	return n.cmd(t, sub, extra, n.abs(n.exe)+" "+quote(args...))
}

// put writes the files (name -> contents) to the folder sub of the run.
func (n *native) put(t *testing.T, sub string, files map[string]string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "put")
	for name, data := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := n.r.Put(context.Background(), dir, sub); err != nil {
		t.Fatal(err)
	}
}

// powershell runs a PowerShell script in the run's folder (a file of the run, with the execution policy
// bypassed for this process only: no setting of the machine is changed). PowerShell takes the
// \\?\ prefix for paths over 260 characters, which cmd.exe's rmdir and robocopy do not.
func (n *native) powershell(t *testing.T, script string, args ...string) winssh.Result {
	t.Helper()
	nativeMu.Lock()
	n.scripts++
	name := fmt.Sprintf("ps/s%d.ps1", n.scripts)
	nativeMu.Unlock()
	// A BOM, so that Windows PowerShell reads the script as UTF-8 (the scripts name folders with characters outside the ANSI code page).
	n.putFile(t, name, "\ufeff"+script)
	return n.cmd(t, "", nil, `powershell -NoProfile -ExecutionPolicy Bypass -File `+strings.ReplaceAll(name, "/", `\`)+" "+quote(args...))
}

// putFile writes one file to the run's folder.
func (n *native) putFile(t *testing.T, rel, data string) {
	t.Helper()
	local := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(local, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := n.r.Put(context.Background(), local, rel); err != nil {
		t.Fatal(err)
	}
}

func okExit(t *testing.T, res winssh.Result, what string) {
	t.Helper()
	if res.Code != 0 {
		t.Fatalf("%s: exit %d\nstdout:\n%s\nstderr:\n%s", what, res.Code, res.Stdout, res.Stderr)
	}
}

func normNL(b []byte) string { return strings.ReplaceAll(string(b), "\r\n", "\n") }

const nativeHello = "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"hello from windows\")\n}\n"

// checkManifest builds and runs the probe of the manifest in the folder sub with the toolchain that env
// selects: what the manifest does, seen by a C++ library (see the "manifest" case).
func (n *native) checkManifest(t *testing.T, sub string, env []string) {
	t.Helper()
	// What the manifest does, seen by a C++ library: the narrow code page is UTF-8 (a gocpp string is UTF-8, and fopen takes
	// it as it is), and a path of more than 260 characters opens with the plain Win32 call (the PC has LongPathsEnabled). GOCPP_NO_MANIFEST takes it out.
	n.put(t, sub, map[string]string{
		"main.gp": `package main

import (
	"fmt"

	"./probe.hpp"
)

func main() {
	fmt.Println("acp", int(probe.acp()))
	fmt.Println("long", int(probe.write_long("data.txt")))
}
`,
		"probe.hpp": `#pragma once
#include <windows.h>
#include <cstdio>
#include <string>
inline int acp() { return (int)GetACP(); }
// writes a file below a path of more than 260 characters with fopen (the narrow, plain call)
inline int write_long(const char *name) {
    char cwd[1024];
    if (!GetCurrentDirectoryA(sizeof cwd, cwd)) return -1;
    std::string dir = cwd;
    for (int i = 0; i < 6; i++) {
        dir += "\\" + std::string(50, 'p');
        if (!CreateDirectoryA(dir.c_str(), nullptr) && GetLastError() != ERROR_ALREADY_EXISTS) return -2;
    }
    std::string file = dir + "\\" + name;
    FILE *f = fopen(file.c_str(), "w");
    if (!f) return 0;
    fputs("x", f);
    fclose(f);
    return 1;
}
`,
	})
	t.Cleanup(func() {
		n.powershell(t, `Remove-Item -LiteralPath '\\?\`+n.abs(sub)+`\`+strings.Repeat("p", 50)+`' -Recurse -Force -ErrorAction SilentlyContinue`)
	})
	res := n.gocpp(t, sub, env, "run", ".")
	if res.Code != 0 || normNL(res.Stdout) != "acp 65001\nlong 1\n" {
		t.Errorf("with the manifest: exit %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
	}
	res = n.gocpp(t, sub, append(env[:len(env):len(env)], "GOCPP_NO_MANIFEST=1"), "run", ".")
	if out := normNL(res.Stdout); res.Code != 0 || strings.Contains(out, "acp 65001") {
		t.Errorf("GOCPP_NO_MANIFEST=1 gave exit %d, stdout %q, stderr %q: the manifest is still in", res.Code, res.Stdout, res.Stderr)
	}
}

func TestWindowsNative(t *testing.T) {
	n := nativeRemote(t)

	t.Run("env", func(t *testing.T) {
		res := n.gocpp(t, "", nil, "env", "GOCPPCXX", "GOCPPCXXSOURCE", "GOCPPCACHE", "GOOS", "GOARCH")
		okExit(t, res, "gocpp env")
		lines := strings.Split(strings.TrimSpace(normNL(res.Stdout)), "\n")
		if len(lines) != 5 {
			t.Fatalf("gocpp env printed %d lines:\n%s", len(lines), res.Stdout)
		}
		if !strings.HasSuffix(strings.ToLower(lines[0]), ".exe") || !filepath.IsAbs(lines[0]) && !regexp.MustCompile(`^[A-Za-z]:\\`).MatchString(lines[0]) {
			t.Errorf("GOCPPCXX = %q, want a Windows path to an .exe", lines[0])
		}
		if lines[1] != "PATH" {
			t.Errorf("GOCPPCXXSOURCE = %q, want PATH (the PC's WinLibs is in its PATH)", lines[1])
		}
		if want := n.abs("lad") + `\gocpp`; !strings.EqualFold(lines[2], want) {
			t.Errorf("GOCPPCACHE = %q, want %q (%%LOCALAPPDATA%%\\gocpp)", lines[2], want)
		}
		if lines[3] != "windows" || lines[4] != "amd64" {
			t.Errorf("GOOS/GOARCH = %q/%q", lines[3], lines[4])
		}
	})

	t.Run("hello", func(t *testing.T) {
		n.put(t, "hello", map[string]string{"hello.gp": nativeHello, "gocpp.mod": "module example.com/hello\n\ngo 1.25\n"})
		// -o without an extension names an .exe.
		okExit(t, n.gocpp(t, "hello", nil, "build", "-o", "prog", "hello.gp"), "gocpp build")
		res := n.cmd(t, "hello", nil, `dir /b`)
		if names := normNL(res.Stdout); !strings.Contains(names, "prog.exe\n") || strings.Contains(names, "prog\n") {
			t.Errorf("after build -o prog the folder has:\n%s", names)
		}
		res = n.cmd(t, "hello", nil, `prog.exe`)
		if res.Code != 0 || normNL(res.Stdout) != "hello from windows\n" {
			t.Errorf("prog.exe: exit %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
		}
		// compile_commands.json (a debug build writes it next to the module) holds Windows paths and the target's flags.
		local := filepath.Join(t.TempDir(), "cc.json")
		if err := n.r.Get(context.Background(), "hello/compile_commands.json", local); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(local)
		var entries []struct {
			Directory, File string
			Arguments       []string
		}
		if err := json.Unmarshal(data, &entries); err != nil || len(entries) == 0 {
			t.Fatalf("compile_commands.json: %v\n%s", err, data)
		}
		drive := regexp.MustCompile(`^[A-Za-z]:\\`)
		for _, e := range entries {
			if !drive.MatchString(e.File) || !drive.MatchString(e.Directory) {
				t.Errorf("entry %q in %q: not Windows paths", e.File, e.Directory)
			}
			if len(e.Arguments) == 0 || !strings.HasSuffix(strings.ToLower(e.Arguments[0]), ".exe") {
				t.Errorf("entry %q: compiler %v", e.File, e.Arguments)
			}
		}
		// The first unit's flags: the API level of <windows.h>, and no -fPIC.
		if args := strings.Join(entries[0].Arguments, " "); !strings.Contains(args, "-D_WIN32_WINNT=0x0A00") || strings.Contains(args, "-fPIC") {
			t.Errorf("flags of %s: %s", entries[0].File, args)
		}
	})

	t.Run("run", func(t *testing.T) {
		n.put(t, "run", map[string]string{"exit.gp": `package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("args:", len(os.Args)-1, os.Args[1], os.Args[2])
	fmt.Fprintln(os.Stderr, "to stderr")
	os.Exit(3)
}
`})
		res := n.gocpp(t, "run", nil, "run", "exit.gp", "a", "b c")
		if res.Code != 3 || normNL(res.Stdout) != "args: 2 a b c\n" || normNL(res.Stderr) != "to stderr\n" {
			t.Errorf("gocpp run: exit %d (want 3), stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
		}
	})

	t.Run("crlf", func(t *testing.T) {
		// Windows checkouts turn LF into CRLF: .gp files and gocpp.mod are read the same. A raw string drops the \r, as in Go.
		n.put(t, "crlf", map[string]string{
			"gocpp.mod": "module example.com/crlf\r\n\r\ngo 1.25\r\n",
			"main.gp":   "package main\r\n\r\nimport \"fmt\"\r\n\r\nfunc main() {\r\n\tfmt.Println(`raw\r\nline`, \"crlf ok\")\r\n}\r\n",
		})
		res := n.gocpp(t, "crlf", nil, "run", ".")
		if res.Code != 0 || normNL(res.Stdout) != "raw\nline crlf ok\n" {
			t.Errorf("exit %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
		}
	})

	t.Run("spaces", func(t *testing.T) {
		// A folder with spaces for the sources, another for the output, a program name with a space.
		n.put(t, "spsrc", map[string]string{"main.gp": nativeHello})
		okExit(t, n.cmd(t, "", nil, `mkdir "my project" "out dir" && move spsrc "my project\src dir"`), "mkdir and move")
		res := n.gocpp(t, "my project/src dir", nil, "build", "-o", n.abs("out dir")+`\my prog`, "main.gp")
		okExit(t, res, "gocpp build")
		res = n.cmd(t, "out dir", nil, `"my prog.exe"`)
		if res.Code != 0 || normNL(res.Stdout) != "hello from windows\n" {
			t.Errorf("my prog.exe: exit %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
		}
	})

	t.Run("long module path", func(t *testing.T) {
		// The module lives below a path of more than 260 characters: its files are reached with the \\?\ prefix,
		// and the generated C++ is in the cache, so the compiler never needs the long name.
		n.put(t, "longsrc", map[string]string{"gocpp.mod": "module example.com/long\n\ngo 1.25\n", "main.gp": nativeHello})
		long := n.abs("lm")
		for len(long) < 300 {
			long += `\` + strings.Repeat("d", 40)
		}
		t.Cleanup(func() {
			n.powershell(t, `Remove-Item -LiteralPath '\\?\`+n.abs("lm")+`' -Recurse -Force -ErrorAction SilentlyContinue`)
		})
		okExit(t, n.powershell(t, `New-Item -ItemType Directory -Force -Path '\\?\`+long+`' | Out-Null
Copy-Item -LiteralPath '`+n.abs("longsrc")+`\main.gp','`+n.abs("longsrc")+`\gocpp.mod' -Destination '\\?\`+long+`'`), "copying into the long path")
		res := n.gocpp(t, "", nil, "build", "-o", n.abs("long.exe"), long+`\main.gp`)
		okExit(t, res, "gocpp build of a file below a long path")
		res = n.cmd(t, "", nil, `long.exe`)
		if res.Code != 0 || normNL(res.Stdout) != "hello from windows\n" {
			t.Errorf("long.exe: exit %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
		}
		// And as a package pattern: gocpp build with the module's folder as the argument.
		res = n.gocpp(t, "", nil, "build", "-o", n.abs("long2.exe"), long)
		okExit(t, res, "gocpp build of a folder below a long path")
	})

	t.Run("c-shared", func(t *testing.T) {
		n.put(t, "dll", map[string]string{
			"lib.gp": `package main

//export Add
func Add(a, b int32) int32 { return a + b }

// Upper returns s in upper case.
//
//export Upper
func Upper(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 32
		}
	}
	return string(b)
}

func main() {}
`,
			"caller.c": `#include <stdio.h>
#include <stdlib.h>
#include <windows.h>
#include "mylib.h"
int main(void) {
	printf("Add=%d\n", (int)Add(40, 2));
	GoString s = {"hello", 5};
	GoString u = Upper(s);
	printf("Upper=%.*s\n", (int)u.n, u.p);
	free((void *)u.p);
	/* the same DLL by name, as a plug-in loader would */
	HMODULE h = LoadLibraryA("mylib.dll");
	FARPROC f = h ? GetProcAddress(h, "Add") : 0;
	FARPROC hidden = h ? GetProcAddress(h, "gp_init_all") : 0;
	printf("loaded=%d add=%d hidden=%d\n", h != 0, f != 0, hidden != 0);
	return 0;
}
`,
			"caller_lib.c": `#include <stdio.h>
#include "mylib.h"
int main(void) { printf("Add=%d\n", (int)Add(1, 2)); return 0; }
`,
			"caller_a.c": `#include <stdio.h>
#include "libmya.h"
int main(void) { printf("Add=%d\n", (int)Add(1, 2)); return 0; }
`,
		})
		okExit(t, n.gocpp(t, "dll", nil, "build", "-buildmode=c-shared", "-o", "mylib.dll", "lib.gp"), "gocpp build -buildmode=c-shared")
		names := normNL(n.cmd(t, "dll", nil, `dir /b mylib.*`).Stdout)
		for _, want := range []string{"mylib.dll\n", "mylib.dll.a\n", "mylib.h\n", "mylib.lib\n"} {
			if !strings.Contains(names, want) {
				t.Errorf("the build left %q without %s", names, strings.TrimSpace(want))
			}
		}
		// The header marks the functions for export, and the DLL exports them and nothing else.
		hdr := normNL(n.cmd(t, "dll", nil, `type mylib.h`).Stdout)
		if !strings.Contains(hdr, "__declspec(dllexport)") {
			t.Errorf("mylib.h has no dllexport:\n%s", hdr)
		}
		var exported []string
		for _, line := range strings.Split(normNL(n.cmd(t, "dll", nil, `objdump -p mylib.dll`).Stdout), "\n") {
			if strings.Contains(line, "+base[") && !strings.Contains(line, "Export RVA") {
				f := strings.Fields(line)
				exported = append(exported, f[len(f)-1])
			}
		}
		if strings.Join(exported, " ") != "Add Upper" {
			t.Errorf("the DLL exports %v, want Add and Upper only", exported)
		}
		// A C program links the import library (-lmylib finds mylib.dll.a) and calls the DLL.
		okExit(t, n.cmd(t, "dll", nil, `gcc caller.c -I. -L. -lmylib -o caller.exe`), "gcc caller.c")
		res := n.cmd(t, "dll", nil, `caller.exe`)
		if res.Code != 0 || normNL(res.Stdout) != "Add=42\nUpper=HELLO\nloaded=1 add=1 hidden=0\n" {
			t.Errorf("caller.exe: exit %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
		}
		// The .lib copy is a usable import library too: named by its file.
		okExit(t, n.cmd(t, "dll", nil, `gcc caller_lib.c -I. mylib.lib -o caller_lib.exe`), "gcc caller_lib.c mylib.lib")
		if res := n.cmd(t, "dll", nil, `caller_lib.exe`); normNL(res.Stdout) != "Add=3\n" {
			t.Errorf("caller_lib.exe: %q %q", res.Stdout, res.Stderr)
		}
		// The static library: the program takes the C++ library and the runtime's needs from the header's line.
		okExit(t, n.gocpp(t, "dll", nil, "build", "-buildmode=c-archive", "-o", "libmya.a", "lib.gp"), "gocpp build -buildmode=c-archive")
		okExit(t, n.cmd(t, "dll", nil, `gcc caller_a.c -I. libmya.a -lstdc++ -pthread -lm -o caller_a.exe`), "gcc with libmya.a")
		if res := n.cmd(t, "dll", nil, `caller_a.exe`); res.Code != 0 || normNL(res.Stdout) != "Add=3\n" {
			t.Errorf("caller_a.exe: exit %d %q %q", res.Code, res.Stdout, res.Stderr)
		}
	})

	t.Run("interop", func(t *testing.T) {
		// The interop tests that need C++'s standard library only, built and run natively.
		progs := loadTests(t, interopFormat)
		want := map[string]bool{"cmath_funcs": true, "cookbook_exceptions": true, "cookbook_iterate": true, "cookbook_std_containers": true, "cxx_containers": true,
			"doc_first_std": true, "map_ops": true, "stdio_calls": true, "string_ops": true, "vector_ops": true, "vector_values": true, "thread_interplay": true}
		n.cmd(t, "", nil, `mkdir interop`)
		found := 0
		for _, p := range progs {
			if !want[p.name] {
				continue
			}
			found++
			t.Run(p.name, func(t *testing.T) {
				data, err := os.ReadFile(filepath.Join(p.dir, p.name+".gp"))
				if err != nil {
					t.Fatal(err)
				}
				n.put(t, "interop/"+p.name, map[string]string{p.name + ".gp": string(data)})
				res := n.gocpp(t, "interop/"+p.name, nil, "run", p.name+".gp")
				checkRun(t, result{cmd: "gocpp run " + p.name + ".gp", stdout: []byte(normNL(res.Stdout)), stderr: []byte(normNL(res.Stderr)),
					exit: res.Code, status: fmt.Sprintf("exit status %d", res.Code)},
					expect{stdout: p.header.output, from: "the // Output: block", exitCode: p.header.exitCode, stderr: p.header.stderr})
			})
		}
		if found != len(want) {
			t.Errorf("found %d of the %d interop tests of the list", found, len(want))
		}
	})

	t.Run("manifest", func(t *testing.T) {
		n.checkManifest(t, "manifest", nil)
	})

	t.Run("long command lines", func(t *testing.T) {
		// More characters of flags than CreateProcess takes (32,767) in a compile and in the link, so these commands go through response files.
		// An environment variable holds at most 32,767 characters itself, so the compile flags come from two (GOCPP_CXXFLAGS and GOCPP_CFLAGS).
		var cxx, cflags, ld strings.Builder
		for i := 0; i < 1000; i++ {
			fmt.Fprintf(&cxx, " -DGP_LONG_FLAG_%05d=1", i)
			fmt.Fprintf(&cflags, " -DGP_LONG_CFLAG_%05d=1", i)
		}
		for i := 0; i < 640; i++ {
			fmt.Fprintf(&ld, " -LC:\\gocpp-no-such-folder\\a-long-folder-name-%04d", i)
		}
		if len(cxx.String()) > 32000 || len(cflags.String()) > 32000 || len(ld.String()) > 32000 || len(cxx.String())+len(cflags.String()) < 40000 || len(ld.String()) < 30000 {
			t.Fatalf("the flags are not what the test needs: %d %d %d", len(cxx.String()), len(cflags.String()), len(ld.String()))
		}
		script := "foreach ($n in 'GOCPP_CXXFLAGS=cxxflags.txt','GOCPP_CFLAGS=cflags.txt','GOCPP_LDFLAGS=ldflags.txt') {\r\n" +
			"  $k, $f = $n.Split('=')\r\n" +
			"  $v = [IO.File]::ReadAllText($f).Trim()\r\n" +
			"  [Environment]::SetEnvironmentVariable($k, $v, 'Process')\r\n" +
			"  if ([Environment]::GetEnvironmentVariable($k).Length -ne $v.Length) { Write-Output \"$k was cut\"; exit 9 }\r\n" +
			"}\r\n" +
			"cmd /c \"\"" + n.abs(n.exe) + "\" build -x -o prog hello.gp 2> x.txt\"\r\n" + // cmd writes the file as bytes; PowerShell's 2> writes UTF-16

			"exit $LASTEXITCODE\r\n"
		n.put(t, "rsp", map[string]string{"cxxflags.txt": cxx.String(), "cflags.txt": cflags.String(), "ldflags.txt": ld.String(), "run.ps1": script, "hello.gp": nativeHello})
		res := n.cmd(t, "rsp", nil, `powershell -NoProfile -ExecutionPolicy Bypass -File run.ps1`)
		if res.Code != 0 {
			t.Fatalf("build with commands of over 32,767 characters: exit %d\nstdout:\n%s\nstderr:\n%s", res.Code, res.Stdout, res.Stderr)
		}
		// -x shows a compile command whole (more than CreateProcess takes, so it was run through a response file) and the link command with its response file.
		local := filepath.Join(t.TempDir(), "x.txt")
		if err := n.r.Get(context.Background(), "rsp/x.txt", local); err != nil {
			t.Fatal(err)
		}
		data, _ := os.ReadFile(local)
		var compile, link int
		for _, line := range strings.Split(string(data), "\n") {
			switch {
			case len(line) > 32767 && strings.Contains(line, "GP_LONG_FLAG_00999") && strings.Contains(line, " -c "):
				compile++
			case strings.Contains(line, `link.rsp`) && !strings.Contains(line, "a-long-folder-name-0639"):
				link++ // the link line shows the response file that holds the flags
			}
		}
		if compile == 0 || link == 0 {
			t.Errorf("-x shows %d compile commands over 32,767 characters and %d links through link.rsp, want some of each (the flags did not reach the compiler)", compile, link)
		}
		if res := n.cmd(t, "rsp", nil, `prog.exe`); normNL(res.Stdout) != "hello from windows\n" {
			t.Errorf("prog.exe: %q %q", res.Stdout, res.Stderr)
		}
	})

	t.Run("flags and compiler with spaces", func(t *testing.T) {
		// GOCPP_CXXFLAGS is split like the go command splits CC (a word that starts with a quote keeps its spaces; there are no escapes),
		// so a Windows path with spaces and a definition with a space work. GOCPP_CXX names a compiler below a folder with a space in
		// its name ("C:\Program Files\..."), here a small wrapper program that starts the PC's own compiler.
		wrapSrc := filepath.Join(t.TempDir(), "wrap")
		if err := os.MkdirAll(wrapSrc, 0o755); err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(wrapSrc, "go.mod"), []byte("module wrap\n\ngo 1.21\n"), 0o644)
		os.WriteFile(filepath.Join(wrapSrc, "main.go"), []byte(`package main

import (
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command(os.Getenv("REAL_CXX"), os.Args[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		os.Exit(127)
	}
}
`), 0o644)
		wrap := filepath.Join(t.TempDir(), "g++.exe")
		build := exec.CommandContext(t.Context(), "go", "build", "-o", wrap, ".")
		build.Dir = wrapSrc
		build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
		if out, err := build.CombinedOutput(); err != nil {
			t.Fatalf("building the wrapper: %v\n%s", err, out)
		}
		okExit(t, n.cmd(t, "", nil, `mkdir "Program Files\My Tools\bin" "Program Files\inc dir"`), "mkdir")
		if err := n.r.Put(context.Background(), wrap, "Program Files/My Tools/bin/g++.exe"); err != nil {
			t.Fatal(err)
		}
		n.put(t, "flags", map[string]string{
			"main.gp": `package main

import (
	"fmt"

	"./probe.hpp"
)

func main() {
	fmt.Println(C.GoString(probe.message()), int(probe.plain()), int(probe.from_include_dir()))
}
`,
			"probe.hpp": `#pragma once
#include "from_dir.h"
inline const char *message() { return GP_MSG; }
inline int plain() { return PLAIN; }
inline int from_include_dir() { return FROM_DIR; }
`,
		})
		n.putFile(t, "Program Files/inc dir/from_dir.h", "#define FROM_DIR 42\n")
		realCXX := strings.TrimSpace(normNL(n.gocpp(t, "", nil, "env", "GOCPPCXX").Stdout))
		wrapped := n.abs("Program Files/My Tools/bin/g++.exe")
		script := "[Environment]::SetEnvironmentVariable('REAL_CXX', '" + realCXX + "', 'Process')\r\n" +
			"[Environment]::SetEnvironmentVariable('GOCPP_CXX', '\"" + wrapped + "\"', 'Process')\r\n" +
			"[Environment]::SetEnvironmentVariable('GOCPP_CXXFLAGS', '''-DGP_MSG=\"a b\"'' \"-I" + n.abs("Program Files/inc dir") + "\" -DPLAIN=7', 'Process')\r\n" +
			"Set-Location flags\r\n" +
			"& '" + n.abs(n.exe) + "' run .\r\n" +
			"exit $LASTEXITCODE\r\n"
		res := n.powershell(t, script)
		if res.Code != 0 || normNL(res.Stdout) != "a b 7 42\n" {
			t.Errorf("exit %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
		}
		// env shows the wrapper as the compiler (a path with a space is kept whole).
		res = n.powershell(t, "[Environment]::SetEnvironmentVariable('GOCPP_CXX', '\""+wrapped+"\"', 'Process')\r\n& '"+n.abs(n.exe)+"' env GOCPPCXX\r\n")
		if out := normNL(res.Stdout); out != wrapped+"\n" {
			t.Errorf("gocpp env with GOCPP_CXX in a folder with spaces:\n%s%s", out, res.Stderr)
		}
	})

	t.Run("errors", func(t *testing.T) {
		n.put(t, "errs", map[string]string{
			"err.gp":     "package main\n\nfunc main() {\n\tvar x int = \"no\"\n\t_ = x\n}\n",
			"cppfail.gp": "package main\n\nimport \"./bad.hpp\"\n\nfunc main() {\n\tbad.f()\n}\n",
			"bad.hpp":    "#pragma once\ninline void f() { undeclared_thing(); }\n",
		})
		res := n.gocpp(t, "errs", nil, "build", "err.gp")
		if res.Code != 1 || !strings.Contains(normNL(res.Stderr), `.\err.gp:4:14: cannot use "no"`) {
			t.Errorf("a type error: exit %d, stderr:\n%s", res.Code, res.Stderr)
		}
		res = n.gocpp(t, "errs", nil, "build", "cppfail.gp")
		if e := normNL(res.Stderr); res.Code != 1 || !strings.Contains(e, `.\cppfail.gp:3:8: header "./bad.hpp" doesn't compile:`) || !strings.Contains(e, "'undeclared_thing' was not declared") {
			t.Errorf("a C++ error: exit %d, stderr:\n%s", res.Code, res.Stderr)
		}
		// An absolute path in the argument gives absolute positions in the messages' file names (drive letter and all).
		res = n.gocpp(t, "", nil, "build", n.abs("errs")+`\err.gp`)
		if e := normNL(res.Stderr); res.Code != 1 || !regexp.MustCompile(`(?m)^[^\n]*err\.gp:4:14: cannot use "no"`).MatchString(e) {
			t.Errorf("a type error through an absolute path: exit %d, stderr:\n%s", res.Code, res.Stderr)
		}
	})

	t.Run("no compiler", func(t *testing.T) {
		n.put(t, "nocxx", map[string]string{"hello.gp": nativeHello})
		sys := `C:\Windows\System32`
		for _, c := range []struct {
			name string
			env  []string
			want []string
		}{
			{"local, broken PATH", []string{"PATH=" + sys, "GOCPP_TOOLCHAIN=local"},
				[]string{"no C++ compiler for windows/amd64", "no MinGW-w64 g++ or clang++ is in PATH", "MSYS2", "GOCPP_TOOLCHAIN=local never downloads", "WinLibs", "GOCPP_CXX"}},
			{"auto and offline", []string{"PATH=" + sys, "GOCPP_TOOLCHAIN=auto"},
				[]string{"no C++ compiler for windows/amd64", "pinned toolchain is not usable", "GOCPP_OFFLINE is set"}},
			{"pinned and offline", []string{"GOCPP_TOOLCHAIN=pinned"},
				[]string{"the pinned toolchain is not usable", "GOCPP_OFFLINE is set"}},
			{"GOCPP_CXX that is not there", []string{"GOCPP_CXX=C:\\no such folder\\g++.exe"},
				[]string{"GOCPP_CXX", "was not found"}},
			{"a mode that does not exist", []string{"GOCPP_TOOLCHAIN=sometimes"},
				[]string{"GOCPP_TOOLCHAIN=sometimes: want auto, local or pinned"}},
		} {
			res := n.gocpp(t, "nocxx", c.env, "build", "-o", "x", "hello.gp")
			e := normNL(res.Stderr)
			if res.Code == 0 {
				t.Errorf("%s: the build succeeded", c.name)
			}
			for _, w := range c.want {
				if !strings.Contains(e, w) {
					t.Errorf("%s: the message lacks %q:\n%s", c.name, w, e)
				}
			}
		}
	})

	t.Run("two builds at once", func(t *testing.T) {
		// Both builds write the same cache; the locks (LockFileEx) and the renames (MoveFileEx) keep it whole.
		n.put(t, "twin", map[string]string{"hello.gp": nativeHello})
		var wg sync.WaitGroup
		results := make([]winssh.Result, 2)
		for i := range results {
			wg.Add(1)
			go func() {
				defer wg.Done()
				results[i] = n.gocpp(t, "twin", []string{"GOCPP_OBJCACHE_SIZE=1G"}, "build", "-o", "prog"+strconv.Itoa(i), "hello.gp")
			}()
		}
		wg.Wait()
		for i, res := range results {
			if res.Code != 0 {
				t.Errorf("build %d: exit %d\n%s", i, res.Code, res.Stderr)
			}
			if r := n.cmd(t, "twin", nil, `prog`+strconv.Itoa(i)+`.exe`); normNL(r.Stdout) != "hello from windows\n" {
				t.Errorf("prog%d.exe: %q %q", i, r.Stdout, r.Stderr)
			}
		}
	})

	t.Run("clean", func(t *testing.T) {
		// gocpp clean -toolchain with nothing downloaded: no error, and it says so.
		res := n.gocpp(t, "", nil, "clean", "-toolchain")
		if res.Code != 0 {
			t.Errorf("gocpp clean -toolchain: exit %d\n%s%s", res.Code, res.Stdout, res.Stderr)
		}
	})

	t.Run("nothing outside the run folder", func(t *testing.T) {
		// The cache is below the run's folder, as LOCALAPPDATA said: it is there, and it holds the runtime.
		res := n.cmd(t, "", nil, `dir /s /b lad\gocpp | findstr /C:libgocpprt`)
		if !strings.Contains(normNL(res.Stdout), "libgocpprt") {
			t.Errorf("the runtime library is not in the cache of the run:\n%s", res.Stdout)
		}
	})
}

// TestWindowsNativeUnits runs the Windows-only tests of the packages on the PC: the packages' test
// binaries are built here (go test -c, GOOS=windows) and run there.
func TestWindowsNativeUnits(t *testing.T) {
	host := windowsHost()
	if host == "" {
		t.Skip("GOCPP_WINDOWS_SSH=user@host turns it on")
	}
	r, err := winssh.New(host)
	if err != nil {
		t.Fatalf("the Windows PC is not reachable: %v", err)
	}
	t.Cleanup(r.Close)
	for _, c := range []struct {
		pkg  string
		runs []string // -test.run patterns, one run each (cmd.exe reads | and ^)
	}{
		{"./internal/build", []string{"TestJob", "TestManifest", "TestFitCommand", "TestRspQuoteWindows", "TestFindCXXForWindowsHost"}},
		{"./internal/filelock", []string{"."}},
		{"./internal/fsretry", []string{"."}},
		{"./internal/modfetch", []string{"TestConcurrentProcesses"}},
		{"./internal/winpath", []string{"."}},
		{"./internal/toolchain", []string{"TestLock", "TestRemove", "TestDownloadZip", "TestUnpackZip"}},
		{"./cmd/gocpp", []string{"TestCanonPath", "TestWithExeSuffix", "TestNormalizeEnvPaths"}},
	} {
		t.Run(strings.TrimPrefix(c.pkg, "./"), func(t *testing.T) {
			bin := filepath.Join(t.TempDir(), "unit.test.exe")
			build := exec.CommandContext(t.Context(), "go", "test", "-c", "-o", bin, c.pkg)
			build.Dir = repoRoot
			build.Env = append(os.Environ(), "GOOS=windows", "GOARCH=amd64", "CGO_ENABLED=0")
			if out, err := build.CombinedOutput(); err != nil {
				t.Fatalf("go test -c: %v\n%s", err, out)
			}
			for _, run := range c.runs {
				ctx, cancel := context.WithTimeout(t.Context(), 8*time.Minute)
				env := []string{"GOCPP_OFFLINE=1", "GOCPP_MODCACHE=off"}
				if c.pkg == "./internal/modfetch" {
					env = nil // its tests start servers of their own and set what they need
				}
				res, err := r.Run(ctx, 7*time.Minute, bin, env, []string{"-test.run=" + run, "-test.v", "-test.count=1"}, false)
				cancel()
				if err != nil {
					t.Fatal(err)
				}
				if res.TimedOut || res.Code != 0 {
					t.Fatalf("%s -test.run=%s: exit %d (timed out: %v)\n%s\n%s", c.pkg, run, res.Code, res.TimedOut, res.Stdout, res.Stderr)
				}
				if !strings.Contains(string(res.Stdout), "--- PASS") {
					t.Errorf("%s -test.run=%s: no test ran:\n%s", c.pkg, run, res.Stdout)
				}
			}
		})
	}
}

// TestWindowsNativeDownload downloads the pinned llvm-mingw on the PC (GOCPP_TEST_DOWNLOAD=1: 190 MB, unpacks to
// 750 MB, deleted at the end), builds hello with it (clang, UCRT), and removes it with gocpp clean -toolchain.
func TestWindowsNativeDownload(t *testing.T) {
	if os.Getenv("GOCPP_TEST_DOWNLOAD") != "1" {
		t.Skip("GOCPP_TEST_DOWNLOAD=1 turns it on (190 MB from GitHub)")
	}
	n := nativeRemote(t)
	n.put(t, "dl", map[string]string{"hello.gp": nativeHello})
	env := []string{"GOCPP_OFFLINE=0", "GOCPP_TOOLCHAIN=pinned"}
	res := n.gocpp(t, "dl", env, "build", "-o", "prog", "hello.gp")
	if res.Code != 0 || !strings.Contains(normNL(res.Stderr), "gocpp: downloading llvm-mingw") {
		t.Fatalf("gocpp build: exit %d\nstderr:\n%s", res.Code, res.Stderr)
	}
	if r := n.cmd(t, "dl", nil, `prog.exe`); normNL(r.Stdout) != "hello from windows\n" {
		t.Errorf("prog.exe: %q %q", r.Stdout, r.Stderr)
	}
	r := n.gocpp(t, "dl", env, "env", "GOCPPCXX", "GOCPPCXXSOURCE", "GOCPPCXXVERSION")
	if e := normNL(r.Stdout); !strings.Contains(e, "x86_64-w64-mingw32-clang++.exe") || !strings.Contains(e, "pinned llvm-mingw 20261006 (cached)") || !strings.Contains(e, "clang version") {
		t.Errorf("gocpp env:\n%s", e)
	}
	// The pinned toolchain is clang with UCRT: the manifest and a DLL work with it too.
	n.checkManifest(t, "dl-manifest", env)
	n.put(t, "dl-dll", map[string]string{"lib.gp": "package main\n\n//export Add\nfunc Add(a, b int32) int32 { return a + b }\n\nfunc main() {}\n"})
	if res := n.gocpp(t, "dl-dll", env, "build", "-buildmode=c-shared", "-o", "mylib.dll", "lib.gp"); res.Code != 0 {
		t.Errorf("c-shared with the pinned toolchain: exit %d\n%s", res.Code, res.Stderr)
	}
	if names := normNL(n.cmd(t, "dl-dll", nil, `dir /b`).Stdout); !strings.Contains(names, "mylib.dll.a\n") || !strings.Contains(names, "mylib.lib\n") {
		t.Errorf("the DLL build of the pinned toolchain left:\n%s", names)
	}
	// The second build does not download again (offline works now).
	if res := n.gocpp(t, "dl", []string{"GOCPP_TOOLCHAIN=pinned"}, "build", "-o", "prog2", "hello.gp"); res.Code != 0 {
		t.Errorf("offline build with the toolchain cached: exit %d\n%s", res.Code, res.Stderr)
	}
	if res := n.gocpp(t, "dl", nil, "clean", "-toolchain"); res.Code != 0 {
		t.Errorf("gocpp clean -toolchain: exit %d\n%s%s", res.Code, res.Stdout, res.Stderr)
	}
	if out := normNL(n.cmd(t, "", nil, `if exist lad\gocpp\toolchain\llvm-mingw-20261006-windows-amd64 (echo still there) else (echo gone)`).Stdout); !strings.Contains(out, "gone") {
		t.Errorf("after clean -toolchain: %s", out)
	}
}
