package build

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

// TestFindCXXFor checks the compiler for a target: FindCXX for the host,
// else GOCPP_CXX, else the known cross compiler (never CXX), else an
// error that says to set GOCPP_CXX. It uses the fake compilers of
// cxx_test.go, and windows/amd64 as the other target (on a windows/amd64
// host the fakes, shell scripts, skip the test).
func TestFindCXXFor(t *testing.T) {
	dir := t.TempDir()
	clang := fakeCXX(t, dir, "clang++", "clang version 99.0.0", clangMacros...)
	mingw := fakeCXX(t, dir, "x86_64-w64-mingw32-g++", "x86_64-w64-mingw32-g++ (GCC) 99.1.0", gccMacros...)
	mine := fakeCXX(t, dir, "my-c++", "my c++ 1.0", gccMacros...)
	gcc := fakeCXX(t, dir, "g++", "g++ (GCC) 99.1.0", gccMacros...)
	noMingw := t.TempDir()
	// Nothing is downloaded or read from the user's cache: the rule is local.
	t.Setenv("GOCPP_TOOLCHAIN", "local")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	fakeCXX(t, noMingw, "clang++", "clang version 99.0.0", clangMacros...)

	for _, c := range []struct {
		name          string
		goos, goarch  string
		path          string // PATH
		gocppCXX, cxx string
		want          string // the compiler's path, or the error
	}{
		{"host", runtime.GOOS, runtime.GOARCH, dir, "", "", clang},
		{"host with CXX", runtime.GOOS, runtime.GOARCH, dir, "", "g++", gcc},
		{"windows/amd64", "windows", "amd64", dir, "", "g++", mingw},
		{"windows/amd64 with GOCPP_CXX", "windows", "amd64", dir, "my-c++", "g++", mine},
		{"windows/arm64 with GOCPP_CXX", "windows", "arm64", dir, "my-c++", "", mine},
		{"windows/amd64 without MinGW", "windows", "amd64", noMingw, "", "clang++",
			"no C++ compiler for windows/amd64: neither x86_64-w64-mingw32-clang++ nor x86_64-w64-mingw32-g++ is in PATH (GOCPP_TOOLCHAIN=local never downloads one; install MinGW-w64, or llvm-mingw, or set GOCPP_CXX)"},
		{"windows/arm64", "windows", "arm64", dir, "", "g++", "no C++ compiler for windows/arm64 (set GOCPP_CXX)"},
		{"GOCPP_CXX missing", "windows", "arm64", dir, "c++-404", "",
			`GOCPP_CXX: the C++ compiler "c++-404" was not found: exec: "c++-404": executable file not found in $PATH`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if c.goos == runtime.GOOS && c.goarch == runtime.GOARCH && c.name != "host" && c.name != "host with CXX" {
				t.Skip("the target is the host")
			}
			setEnv(t, c.path, c.gocppCXX, c.cxx)
			got, err := FindCXXFor(context.Background(), c.goos, c.goarch)
			switch {
			case err != nil && err.Error() != c.want:
				t.Errorf("FindCXXFor(%s, %s) error = %q, want %q", c.goos, c.goarch, err, c.want)
			case err == nil && got.Path != c.want:
				t.Errorf("FindCXXFor(%s, %s) = %+v, want %s", c.goos, c.goarch, got, c.want)
			}
		})
	}
}

func TestFindCXXForArguments(t *testing.T) {
	// GOCPP_CXX for another target is split into words like for the host.
	dir := t.TempDir()
	mine := fakeCXX(t, dir, "my-c++", "my c++ 1.0", gccMacros...)
	setEnv(t, dir, "my-c++ -m64", "")
	got, err := FindCXXFor(context.Background(), "windows", "arm64")
	if err != nil || got.Path != mine || len(got.Args) != 1 || got.Args[0] != "-m64" {
		t.Errorf("FindCXXFor(windows, arm64) with GOCPP_CXX=\"my-c++ -m64\" = %+v, %v", got, err)
	}
	setEnv(t, dir, "'my-c++", "")
	if _, err := FindCXXFor(context.Background(), "windows", "arm64"); err == nil || err.Error() != "GOCPP_CXX='my-c++: unterminated ' string" {
		t.Errorf("FindCXXFor with an unterminated quote: %v", err)
	}
}

// A host g++ in GOCPP_CXX for a Windows target is refused with a message that says so, not left to
// fail at the assembler; a MinGW-w64 g++ there is taken.
func TestFindCXXForWindowsRefusesHostGCC(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the host's g++ is a Windows compiler")
	}
	if _, err := exec.LookPath("g++"); err != nil {
		t.Skip("no g++ in PATH")
	}
	t.Setenv("GOCPP_CXX", "g++")
	_, err := FindCXXFor(context.Background(), "windows", "amd64")
	if err == nil || !strings.Contains(err.Error(), "GOCPP_CXX=g++ is not a Windows compiler") {
		t.Errorf("FindCXXFor(windows) with the host's g++ = %v, want the error that it is not a Windows compiler", err)
	}
	if _, err := exec.LookPath("x86_64-w64-mingw32-g++"); err != nil {
		return
	}
	t.Setenv("GOCPP_CXX", "x86_64-w64-mingw32-g++")
	if _, err := FindCXXFor(context.Background(), "windows", "amd64"); err != nil {
		t.Errorf("FindCXXFor(windows) with MinGW-w64's g++ = %v, want no error", err)
	}
}
