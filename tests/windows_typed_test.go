package tests

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Typed interop and gocpp doc on a Windows host (Phase 14 step 14.6, ADR-116), on the real PC
// (GOCPP_WINDOWS_SSH=user@host, never Wine). The PC has no clang, so the test downloads the pinned
// llvm-mingw into its run folder (GOCPP_TEST_DOWNLOAD=1: 190 MB from GitHub, unpacked below the faked
// LOCALAPPDATA of the run, so it is deleted with the folder) and checks that the probes use its clang
// with the Windows target: auto mode is typed, <windows.h> is probed as LLP64, a program that imports
// it builds in typed mode and runs, and gocpp doc finds the header and the kind of a name.

const winTypedProgram = `package main

import (
	"fmt"
	"<windows.h>"
)

func main() {
	var ms windows.DWORD = 5
	var h windows.HANDLE = windows.INVALID_HANDLE_VALUE
	fmt.Println(ms+1, h == nil, windows.GetCurrentProcessId() != 0)
}
`

func TestWindowsTypedNative(t *testing.T) {
	if os.Getenv("GOCPP_TEST_DOWNLOAD") != "1" {
		t.Skip("GOCPP_TEST_DOWNLOAD=1 turns it on (the PC has no clang: it downloads llvm-mingw, 190 MB from GitHub)")
	}
	n := nativeRemote(t)
	n.put(t, "ty", map[string]string{"win.gp": winTypedProgram})
	online := []string{"GOCPP_OFFLINE=0", "GOCPP_TOOLCHAIN=pinned"}
	// The first build downloads the toolchain (untyped: GOCPP_INTEROP=opaque); the rest finds it installed.
	opaque := append([]string{"GOCPP_INTEROP=opaque"}, online...)
	res := n.gocpp(t, "ty", opaque, "build", "-o", "opaque", "win.gp")
	if res.Code != 0 || !strings.Contains(normNL(res.Stderr), "gocpp: downloading llvm-mingw") {
		t.Fatalf("gocpp build: exit %d\nstderr:\n%s", res.Code, res.Stderr)
	}
	typed := []string{"GOCPP_TOOLCHAIN=pinned", "GOCPP_INTEROP=auto"}

	t.Run("auto mode is typed", func(t *testing.T) {
		res := n.gocpp(t, "ty", typed, "check", "-v", "win.gp")
		if out := normNL(res.Stderr) + normNL(res.Stdout); res.Code != 0 || !strings.Contains(out, "interop: typed mode") {
			t.Errorf("gocpp check -v: exit %d\n%s", res.Code, out)
		}
	})

	t.Run("windows.h is probed with the target", func(t *testing.T) {
		res := n.gocpp(t, "ty", typed, "__typed-probe", "-decls", "win.gp")
		if res.Code != 0 {
			t.Fatalf("__typed-probe: exit %d\n%s", res.Code, res.Stderr)
		}
		var rep struct {
			Refs []struct {
				Key    string
				Mapped string
				Result struct{ Type string }
			}
			HeaderErrors []struct{ Msg string }
		}
		if err := json.Unmarshal(res.Stdout, &rep); err != nil {
			t.Fatalf("not JSON: %v\n%s", err, res.Stdout)
		}
		if len(rep.HeaderErrors) > 0 {
			t.Fatalf("<windows.h> does not include: %+v", rep.HeaderErrors)
		}
		types := map[string]string{}
		for _, r := range rep.Refs {
			types[strings.TrimPrefix(r.Key, "::")] = r.Result.Type
		}
		if types["DWORD"] != "unsigned long" || types["HANDLE"] != "void *" {
			t.Errorf("the types of DWORD and HANDLE: %v", types)
		}
	})

	t.Run("a typed build runs", func(t *testing.T) {
		env := append([]string{"GOCPP_INTEROP=typed"}, typed[:1]...)
		if res := n.gocpp(t, "ty", env, "build", "-o", "typed", "win.gp"); res.Code != 0 {
			t.Fatalf("typed build: exit %d\n%s", res.Code, res.Stderr)
		}
		if out := normNL(n.cmd(t, "ty", nil, `typed.exe`).Stdout); out != "6 false true\n" {
			t.Errorf("typed.exe printed %q, want %q", out, "6 false true\n")
		}
	})

	t.Run("gocpp doc finds the header and the kind", func(t *testing.T) {
		// cmd.exe would take <windows.h> for a redirection: the double quotes keep it a word.
		doc := func(name string) string {
			res := n.cmd(t, "ty", typed, n.abs(n.exe)+` doc "<windows.h>" `+name)
			if res.Code != 0 {
				t.Errorf("gocpp doc <windows.h> %s: exit %d\n%s%s", name, res.Code, res.Stdout, res.Stderr)
			}
			return normNL(res.Stdout) + normNL(res.Stderr)
		}
		if out := doc(""); !strings.Contains(out, "Header file: ") || !strings.Contains(strings.ToLower(out), "windows.h") {
			t.Errorf("gocpp doc <windows.h> does not say where the header is:\n%s", out)
		}
		if out := doc("DWORD"); !strings.Contains(out, "DWORD") || !strings.Contains(out, "type") {
			t.Errorf("gocpp doc <windows.h> DWORD does not say it is a type:\n%s", out)
		}
	})

	if res := n.gocpp(t, "ty", nil, "clean", "-toolchain"); res.Code != 0 {
		t.Errorf("gocpp clean -toolchain: exit %d\n%s%s", res.Code, res.Stdout, res.Stderr)
	}
}
