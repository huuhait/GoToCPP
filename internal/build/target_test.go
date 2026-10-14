package build

import (
	"context"
	"runtime"
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
			"no C++ compiler for windows/amd64: x86_64-w64-mingw32-g++ is not in PATH (set GOCPP_CXX)"},
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
