package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The typed probes for the Windows target (Phase 14 step 14.6, ADR-116) on this machine: clang++ with
// the target of MinGW-w64 and its headers (the g++ of the cross build says where they are), so that
// <windows.h> and its types are probed, not the host's. Nothing runs on Windows: the question is what
// clang makes of the headers, which is the same on every system (LLP64: DWORD is an unsigned long of 4
// bytes, HANDLE a void pointer). Skipped without clang++ and a MinGW-w64 g++ (on a Windows host,
// without clang++).
const winProbeProgram = `package main

import (
	"fmt"
	"<windows.h>"
)

func main() {
	var ms windows.DWORD = 5
	var h windows.HANDLE = windows.INVALID_HANDLE_VALUE
	fmt.Println(ms, h == nil, windows.GetCurrentProcessId())
}
`

func needWindowsClang(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("clang++"); err != nil {
		t.Skip("no clang++ for the probe")
	}
	if runtime.GOOS != "windows" {
		if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil {
			t.Skip("no MinGW-w64 g++: nothing names the Windows headers for clang++")
		}
	}
}

func TestTypedProbeWindowsHeaders(t *testing.T) {
	needWindowsClang(t)
	t.Setenv("GOCPP_MODCACHE", "off")
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_PROBE_TIMEOUT", "5m")
	t.Setenv("GOCPP_NOPROBECACHE", "1")
	t.Setenv("GOCPP_CLANG", "")
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_OS", "windows")
	t.Setenv("GOCPP_ARCH", "amd64")
	inDir(t, map[string]string{"main.gp": winProbeProgram})

	r := gocppRun("", false, "__typed-probe", "-decls", ".")
	if r.code == 3 {
		t.Skip("no clang++ for the probe")
	}
	if r.code != 0 {
		t.Fatalf("__typed-probe for windows: %+v", r)
	}
	var rep struct {
		Refs []struct {
			Key    string
			Mapped string
			Result struct {
				OK    bool
				Error string
				Type  string
			}
		}
		HeaderErrors []struct{ Msg string }
	}
	if err := json.Unmarshal([]byte(r.stdout), &rep); err != nil {
		t.Fatalf("the report is not JSON: %v\n%s", err, r.stdout)
	}
	if len(rep.HeaderErrors) > 0 {
		t.Fatalf("<windows.h> does not include with the target's flags: %+v", rep.HeaderErrors)
	}
	found := map[string]string{}
	for _, ref := range rep.Refs {
		if !ref.Result.OK && ref.Result.Error != "" {
			t.Errorf("%s: the probe failed: %s", ref.Key, ref.Result.Error)
		}
		found[strings.TrimPrefix(ref.Key, "::")] = ref.Result.Type
	}
	// LLP64: long is 32 bits, so a DWORD is an unsigned long and the typed view maps it to C.ulong,
	// a 4-byte type (which is the Windows ABI's, not the host's).
	if got := found["DWORD"]; got != "unsigned long" {
		t.Errorf("DWORD is %q, want unsigned long (refs: %v)", got, found)
	}
	if got := found["HANDLE"]; got != "void *" {
		t.Errorf("HANDLE is %q, want void * (refs: %v)", got, found)
	}
	if !strings.Contains(r.stdout, "type C.ulong") {
		t.Errorf("DWORD is not mapped to C.ulong:\n%s", r.stdout)
	}
}

// -v says that typed mode is on for the Windows target (it was opaque on a Windows host and for any
// target that was not this machine's).
func TestTypedCheckWindowsTargetIsTyped(t *testing.T) {
	needWindowsClang(t)
	t.Setenv("GOCPP_MODCACHE", "off")
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_CLANG", "")
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_INTEROP", "auto")
	inDir(t, map[string]string{"main.gp": winProbeProgram})
	r := gocppRun("", true, "check", "-os", "windows", "-v", ".")
	if r.code != 0 || !strings.Contains(r.stdout, "interop: typed mode") {
		t.Errorf("gocpp check -os windows -v: %+v", r)
	}
}

// The precompiled header of the library block (Phase 12 step 12.14, ADR-117) is made and loaded with the
// target's flags: the probe of <windows.h> gives the same answers with the header as without it, the runs
// load it, and the compiler never refuses it.
func TestTypedProbeWindowsHeadersWithThePrecompiledHeader(t *testing.T) {
	needWindowsClang(t)
	t.Setenv("GOCPP_MODCACHE", "off")
	t.Setenv("GOCPP_OFFLINE", "1")
	t.Setenv("GOCPP_PROBE_TIMEOUT", "5m")
	t.Setenv("GOCPP_CLANG", "")
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("CXX", "")
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("GOCPP_OS", "windows")
	t.Setenv("GOCPP_ARCH", "amd64")
	t.Setenv("GOCPP_NOPROBECACHE", "")
	inDir(t, map[string]string{"main.gp": winProbeProgram})
	statsLine := regexp.MustCompile(`stats (\{.*\})`)
	probe := func(mode string) (stdout string, uses, builds, rejects int) {
		cache := t.TempDir()
		t.Setenv("XDG_CACHE_HOME", cache)
		t.Setenv("GOCPP_PROBE_CACHE", filepath.Join(cache, "probe"))
		t.Setenv("GOCPP_PROBE_PCH", mode)
		r := gocppRun("", false, "__typed-probe", "-decls", "-stable", "-stats", ".")
		if r.code == 3 {
			t.Skip("no clang++ for the probe")
		}
		if r.code != 0 {
			t.Fatalf("__typed-probe for windows with GOCPP_PROBE_PCH=%s: %+v", mode, r)
		}
		for _, m := range statsLine.FindAllStringSubmatch(r.stderr, -1) {
			var st struct {
				Process struct{ PCHUses, PCHBuilds, PCHRejects int }
			}
			if json.Unmarshal([]byte(m[1]), &st) == nil {
				uses, builds, rejects = st.Process.PCHUses, st.Process.PCHBuilds, st.Process.PCHRejects
			}
		}
		return r.stdout, uses, builds, rejects
	}
	plain, _, _, _ := probe("off")
	with, uses, builds, rejects := probe("wait")
	if plain != with {
		t.Errorf("the answers for the Windows target differ with the header:\n%s\n---\n%s", plain, with)
	}
	if builds == 0 || uses == 0 || rejects != 0 {
		t.Errorf("%d headers made, %d runs loaded one, %d refused", builds, uses, rejects)
	}
}
