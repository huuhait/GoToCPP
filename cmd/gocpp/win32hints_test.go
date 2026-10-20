package main

import (
	"os/exec"
	"strings"
	"testing"
)

func TestWin32DefaultsHint(t *testing.T) {
	wide := []string{"C++: cannot convert 'const wchar_t*' to 'LPCSTR' {aka 'const char*'}"}
	for _, c := range []struct {
		name     string
		goos     string
		defaults int
		msgs     []string
		want     string // a substring of the hint, or "" for none
	}{
		{"defaults on", "windows", 4, wide, "are on by default for Windows packages"},
		{"defaults off", "windows", 0, wide, "does not get the Win32 defaults"},
		{"not windows", "linux", 0, wide, ""},
		{"an unrelated error", "windows", 4, []string{"no member named 'x' in 'T'"}, ""},
		{"UNICODE in the message", "windows", 4, []string{"use of undeclared identifier 'UNICODE'"}, "are on by default"},
		{"TCHAR", "windows", 0, []string{"cannot convert 'TCHAR*' to 'char*'"}, "does not get the Win32 defaults"},
	} {
		got := win32DefaultsHint(c.goos, c.defaults, c.msgs)
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: hint %q, want one with %q", c.name, got, c.want)
		}
	}
}

func TestImportLibHint(t *testing.T) {
	for _, c := range []struct {
		syms []string
		want string
	}{
		{[]string{"__imp_socket"}, "//gocpp:link ws2_32;"},
		{[]string{"__imp_WSAStartup", "__imp_socket", "__imp_timeGetTime"}, "//gocpp:link ws2_32 winmm;"},
		{[]string{"__imp_GetStockObject", "CoInitializeEx", "__imp_SysAllocString"}, "//gocpp:link gdi32 ole32 oleaut32;"},
		{[]string{"IID_IDXGIFactory1", "IID_IUnknown", "__imp_CreateDXGIFactory1"}, "//gocpp:link dxguid uuid dxgi;"},
		{[]string{"__imp_BCryptGenRandom", "__imp_CertOpenStore", "__imp_PathFileExistsW", "__imp_InternetOpenW", "__imp_GetAdaptersInfo", "__imp_GetFileVersionInfoSizeW"}, "//gocpp:link bcrypt crypt32 shlwapi wininet iphlpapi version;"},
		{[]string{"__imp_GetOpenFileNameW", "__imp_InitCommonControlsEx", "__imp_SetupDiGetClassDevsW", "__imp_D3D11CreateDevice"}, "//gocpp:link comdlg32 comctl32 setupapi d3d11;"},
		{[]string{"__imp_RegOpenKeyExW", "__imp_ShellExecuteW", "__imp_MessageBoxW"}, "//gocpp:link advapi32 shell32 user32;"},
		{[]string{"my_function", "__imp_NotKnown"}, ""},
	} {
		got := importLibHint("windows", c.syms)
		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%q: hint %q, want one with %q", c.syms, got, c.want)
		}
		if importLibHint("linux", c.syms) != "" {
			t.Errorf("%q: a hint on linux", c.syms)
		}
	}
}

// TestWindowsInteropMessages builds small programs for windows with the real MinGW-w64 compiler and checks what a
// user sees: the hint after an error that comes from the ANSI/Wide switch, the hint for a missing import library,
// the clean error for a macro in an array length, and the LLP64 range of C.long (step 14.4, ADR-082).
func TestWindowsInteropMessages(t *testing.T) {
	noTargetEnv(t)
	if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil {
		t.Skip("no MinGW-w64 compiler in PATH")
	}
	t.Setenv("GOCPP_GATE_PHASE", "14")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("GOCPP_CXX", "")
	t.Setenv("GOCPP_CFLAGS", "")
	t.Setenv("GOCPP_CXXFLAGS", "")
	build := func(t *testing.T, src string) result {
		t.Helper()
		inDir(t, map[string]string{"main.gp": src})
		return gocppRun("", false, "build", "-compdb=false", "-os", "windows", "-o", "out.exe", "main.gp")
	}
	const file = `CreateFile(C.wstr("x"), 0, 0, nil, 0, 0, nil)`
	t.Run("ANSI function with the defaults", func(t *testing.T) {
		r := build(t, "package main\n\nimport \"<windows.h>\"\n\nfunc main() {\n\twindows.CreateFileA(C.wstr(\"x\"), 0, 0, nil, 0, 0, nil)\n}\n")
		if r.code != 1 || !strings.Contains(r.stderr, "wchar_t") || !strings.Contains(r.stderr, "are on by default for Windows packages") {
			t.Errorf("exit %d, stderr:\n%s", r.code, r.stderr)
		}
	})
	t.Run("the defaults turned off", func(t *testing.T) {
		r := build(t, "//gocpp:no-win32-defaults\n\npackage main\n\nimport \"<windows.h>\"\n\nfunc main() {\n\twindows."+file+"\n}\n")
		if r.code != 1 || !strings.Contains(r.stderr, "wchar_t") || !strings.Contains(r.stderr, "does not get the Win32 defaults") {
			t.Errorf("exit %d, stderr:\n%s", r.code, r.stderr)
		}
	})
	t.Run("a missing import library", func(t *testing.T) {
		r := build(t, "package main\n\nimport \"<winsock2.h>\"\n\nfunc main() {\n\twinsock2.socket(2, 1, 6)\n}\n")
		if r.code != 1 || !strings.Contains(r.stderr, "undefined reference to '__imp_socket'") || !strings.Contains(r.stderr, "add //gocpp:link ws2_32;") {
			t.Errorf("exit %d, stderr:\n%s", r.code, r.stderr)
		}
	})
	t.Run("a macro in an array length", func(t *testing.T) {
		r := build(t, "package main\n\nimport \"<windows.h>\"\n\nfunc main() {\n\tvar buf [windows.MAX_PATH]windows.WCHAR\n\t_ = buf\n}\n")
		want := "a C/C++ macro in an array length (windows.MAX_PATH) needs typed interop (-interop=typed, which needs clang++)"
		if r.code != 1 || !strings.Contains(r.stderr, want) || strings.Contains(r.stderr, "internal error") || !strings.Contains(r.stderr, "[260]windows.WCHAR") {
			t.Errorf("exit %d, stderr:\n%s", r.code, r.stderr)
		}
	})
	t.Run("C.long has 32 bits", func(t *testing.T) {
		for _, c := range []struct{ decl, want string }{
			{"var x C.long = 1 << 31", "cannot use 1 << 31 (untyped int constant 2147483648) as C.long value in variable declaration (overflows)"},
			{"var x C.ulong = 1 << 32", "cannot use 1 << 32 (untyped int constant 4294967296) as C.ulong value in variable declaration (overflows)"},
			{"var x C.wchar_t = 1 << 16", "cannot use 1 << 16 (untyped int constant 65536) as C.wchar_t value in variable declaration (overflows)"},
		} {
			r := build(t, "package main\n\nimport \"<windows.h>\"\n\nfunc main() {\n\t"+c.decl+"\n\twindows.Sleep(0)\n\t_ = x\n}\n")
			if r.code != 1 || !strings.Contains(r.stderr, c.want) {
				t.Errorf("%s: exit %d, want %q, stderr:\n%s", c.decl, r.code, c.want, r.stderr)
			}
		}
	})
}
