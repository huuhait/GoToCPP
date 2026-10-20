package build

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// The output of x86_64-w64-mingw32-g++ -E -v -x c++ - on a Linux host (shortened): the folders of
// the <...> list, with GCC's own headers (lib/gcc/.../include and include-fixed) among them.
const mingwVerbose = `Using built-in specs.
COLLECT_GCC=x86_64-w64-mingw32-g++
Target: x86_64-w64-mingw32
Thread model: mcf
gcc version 15.3.0 (GCC)
#include "..." search starts here:
#include <...> search starts here:
 /nix/store/aaa-gcc-15.3.0/include/c++/15.3.0
 /nix/store/aaa-gcc-15.3.0/include/c++/15.3.0/x86_64-w64-mingw32
 /nix/store/aaa-gcc-15.3.0/include/c++/15.3.0/backward
 /nix/store/aaa-gcc-15.3.0/lib/gcc/x86_64-w64-mingw32/15.3.0/include
 /nix/store/aaa-gcc-15.3.0/lib/gcc/x86_64-w64-mingw32/15.3.0/include-fixed
 /nix/store/bbb-mingw-w64/x86_64-w64-mingw32/include
End of search list.
# 0 "<stdin>"
`

func TestMingwSearchDirs(t *testing.T) {
	got := mingwSearchDirs(mingwVerbose)
	want := []string{
		"/nix/store/aaa-gcc-15.3.0/include/c++/15.3.0",
		"/nix/store/aaa-gcc-15.3.0/include/c++/15.3.0/x86_64-w64-mingw32",
		"/nix/store/aaa-gcc-15.3.0/include/c++/15.3.0/backward",
		"/nix/store/bbb-mingw-w64/x86_64-w64-mingw32/include",
	}
	if !slices.Equal(got, want) {
		t.Errorf("mingwSearchDirs = %q\nwant %q", got, want)
	}
	if got := mingwSearchDirs("Segmentation fault\n"); len(got) != 0 {
		t.Errorf("mingwSearchDirs of no list = %q, want none", got)
	}
	// "include" folders that are not GCC's are kept: <mingw>/include is the Windows headers.
	crlf := "#include <...> search starts here:\r\n C:/mingw64/include\r\n" +
		" C:/mingw64/lib/gcc/x86_64-w64-mingw32/15/include\r\nEnd of search list.\r\n"
	if got := mingwSearchDirs(crlf); !slices.Equal(got, []string{"C:/mingw64/include"}) {
		t.Errorf("mingwSearchDirs of a Windows list = %q", got)
	}
}

// fakeMingwGXX puts a x86_64-w64-mingw32-g++ that prints mingwVerbose in an otherwise empty PATH.
func fakeMingwGXX(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake compilers are shell scripts")
	}
	dir := t.TempDir()
	// Only builtins: PATH has nothing but this folder.
	script := "#!/bin/sh\nprintf '%s' " + shellQuote(mingwVerbose) + " >&2\n"
	if err := os.WriteFile(filepath.Join(dir, mingwGXX), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
}

func TestProbeTargetFlags(t *testing.T) {
	fakeMingwGXX(t)
	sysroot := []string{
		"-nostdlibinc", "-nostdinc++",
		"-isystem", "/nix/store/aaa-gcc-15.3.0/include/c++/15.3.0",
		"-isystem", "/nix/store/aaa-gcc-15.3.0/include/c++/15.3.0/x86_64-w64-mingw32",
		"-isystem", "/nix/store/aaa-gcc-15.3.0/include/c++/15.3.0/backward",
		"-isystem", "/nix/store/bbb-mingw-w64/x86_64-w64-mingw32/include",
	}
	cross := slices.Concat(
		[]string{"--target=x86_64-w64-mingw32", "-D_WIN32_WINNT=0x0A00", "-fno-file-reproducible"},
		sysroot)
	hostIsWindows := runtime.GOOS == "windows"
	for _, c := range []struct {
		name string
		cxx  CXX
		want []string
	}{
		{"the host's compiler", CXX{Family: Clang, Path: "/bin/clang++"}, nil},
		{"mingw g++", CXX{Family: GCC, Path: "/bin/x86_64-w64-mingw32-g++", GOOS: "windows", GOARCH: "amd64"},
			[]string{"-D_WIN32_WINNT=0x0A00"}},
		// llvm-mingw's clang knows its target and its headers.
		{"llvm-mingw clang++", CXX{Family: Clang, Path: "/bin/x86_64-w64-mingw32-clang++", GOOS: "windows", GOARCH: "amd64"},
			[]string{"-D_WIN32_WINNT=0x0A00"}},
		{"clang++ of the host for windows",
			CXX{Family: Clang, Path: "/bin/clang++", GOOS: "windows", GOARCH: "amd64"}, cross},
		{"clang++-21 of the host for windows",
			CXX{Family: Clang, Path: "/bin/clang++-21", GOOS: "windows", GOARCH: "amd64"}, cross},
		{"clang++ for linux/arm64", CXX{Family: Clang, Path: "/bin/clang++", GOOS: "linux", GOARCH: "arm64"},
			[]string{"--target=aarch64-linux-gnu"}},
	} {
		want := c.want
		if hostIsWindows && len(want) > len(sysroot) {
			want = []string{"--target=x86_64-w64-mingw32", "-D_WIN32_WINNT=0x0A00"} // a Windows host's clang has its own headers
		}
		if got := ProbeTargetFlags(c.cxx); !slices.Equal(got, want) {
			t.Errorf("%s: ProbeTargetFlags =\n%q\nwant\n%q", c.name, got, want)
		}
	}
}

// Without the MinGW-w64 g++ (no headers to name), a clang for Windows is left to find its own.
func TestProbeTargetFlagsWithoutMingw(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	got := ProbeTargetFlags(CXX{Family: Clang, Path: "/bin/clang++", GOOS: "windows", GOARCH: "amd64"})
	if slices.ContainsFunc(got, func(f string) bool { return strings.HasPrefix(f, "-isystem") || f == "-nostdlibinc" }) {
		t.Errorf("flags without the MinGW-w64 g++ name its headers: %q", got)
	}
	if !slices.Contains(got, "--target=x86_64-w64-mingw32") {
		t.Errorf("flags = %q, want the target", got)
	}
}

func TestWin32DefaultsFor(t *testing.T) {
	t.Setenv("GOCPP_CFLAGS", "")
	t.Setenv("GOCPP_CXXFLAGS", "")
	want := []string{"-DWIN32_LEAN_AND_MEAN", "-DNOMINMAX", "-DUNICODE", "-D_UNICODE"}
	if got := Win32DefaultsFor("windows", nil); !slices.Equal(got, want) {
		t.Errorf("Win32DefaultsFor(windows) = %q, want %q", got, want)
	}
	if got := Win32DefaultsFor("linux", nil); got != nil {
		t.Errorf("Win32DefaultsFor(linux) = %q, want none", got)
	}
	// A macro that the user defines or undefines is left to the user.
	got := Win32DefaultsFor("windows", []string{"-DNOMINMAX=0", "-UUNICODE"})
	if want := []string{"-DWIN32_LEAN_AND_MEAN", "-D_UNICODE"}; !slices.Equal(got, want) {
		t.Errorf("Win32DefaultsFor with the user's macros = %q, want %q", got, want)
	}
}
