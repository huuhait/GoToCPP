package build

import (
	"context"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestFindCC(t *testing.T) {
	dir, other, empty := t.TempDir(), t.TempDir(), t.TempDir()
	gxx := fakeCXX(t, dir, "g++", "g++ (GCC) 99.1.0", gccMacros...)
	gcc := fakeCXX(t, dir, "gcc", "gcc (GCC) 99.1.0", gccMacros...)
	clangxx := fakeCXX(t, dir, "clang++-18", "clang version 99.0.0", clangMacros...)
	clang := fakeCXX(t, dir, "clang-18", "clang version 99.0.0", clangMacros...)
	mingwxx := fakeCXX(t, dir, "x86_64-w64-mingw32-g++", "x86_64-w64-mingw32-g++ (GCC) 99.1.0", gccMacros...)
	mingw := fakeCXX(t, dir, "x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-gcc (GCC) 99.1.0", gccMacros...)
	ccache := fakeCXX(t, dir, "ccache", "ccache version 4.10", gccMacros...)
	odd := fakeCXX(t, dir, "mycompiler", "my compiler 1.0", gccMacros...)
	lone := fakeCXX(t, empty, "g++", "g++ (GCC) 99.1.0", gccMacros...) // no gcc next to it
	otherGCC := fakeCXX(t, other, "gcc", "gcc (GCC) 99.1.0", gccMacros...)
	mycc := fakeCXX(t, other, "my-cc", "my cc 1.0", clangMacros...)

	cxxOf := func(path string, args ...string) CXX {
		t.Helper()
		c, err := lookPath(context.Background(), path, args, false)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	for _, c := range []struct {
		name    string
		cxx     CXX
		path    string // PATH
		gocppCC string
		want    CXX
		err     string // a part of the error, instead of want
	}{
		{"g++ gives gcc next to it", cxxOf(gxx), other, "", CXX{Path: gcc, Family: GCC, Version: "gcc (GCC) 99.1.0", Major: 99, C: true}, ""},
		{"versioned clang++", cxxOf(clangxx), "", "", CXX{Path: clang, Family: Clang, Version: "clang version 99.0.0", Major: 99, C: true}, ""},
		{"MinGW", cxxOf(mingwxx), "", "", CXX{Path: mingw, Family: GCC, Version: "x86_64-w64-mingw32-gcc (GCC) 99.1.0", Major: 99, C: true}, ""},
		{"then PATH", cxxOf(lone), other, "", CXX{Path: otherGCC, Family: GCC, Version: "gcc (GCC) 99.1.0", Major: 99, C: true}, ""},
		{"a launcher gets the name", cxxOf(ccache, "g++"), "", "", CXX{Path: ccache, Args: []string{"gcc"}, Family: GCC, Version: "ccache version 4.10", Major: 99, C: true}, ""},
		{"a launcher with a path", cxxOf(ccache, gxx), "", "", CXX{Path: ccache, Args: []string{gcc}, Family: GCC, Version: "ccache version 4.10", Major: 99, C: true}, ""},
		{"GOCPP_CC wins", cxxOf(gxx), other, "my-cc -O1", CXX{Path: mycc, Args: []string{"-O1"}, Family: Clang, Version: "my cc 1.0", Major: 99, C: true}, ""},
		{"not found", cxxOf(lone), empty, "", CXX{}, "no C compiler for the package's .c files: gcc is not next to the C++ compiler g++, nor in PATH (GOCPP_CC can name the C compiler)"},
		{"unknown name", cxxOf(odd), other, "", CXX{}, "gocpp can't tell which C compiler goes with the C++ compiler mycompiler (GOCPP_CC can name it)"},
		{"GOCPP_CC missing", cxxOf(gxx), other, "nope", CXX{}, `GOCPP_CC: the C compiler "nope" was not found`},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("PATH", c.path)
			t.Setenv("GOCPP_CC", c.gocppCC)
			got, err := FindCC(context.Background(), c.cxx)
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("FindCC(%s) = %+v, %v; want an error with %q", c.cxx.Name(), got, err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(noStamp(got), c.want) {
				t.Errorf("FindCC(%s) = %+v, want %+v", c.cxx.Name(), noStamp(got), c.want)
			}
		})
	}
	// The C compiler's macros come from C (-x c), and the answer is cached
	// apart from the same program as a C++ compiler.
	log, err := os.ReadFile(gcc + ".log")
	if err != nil || !strings.Contains(string(log), "-dM -E -x c -\n") {
		t.Errorf("gcc ran with:\n%s(%v), want -dM -E -x c -", log, err)
	}
	if c := cxxOf(gcc); c.C {
		t.Errorf("LookCXX of gcc after FindCC found a C compiler: %+v", c)
	}
}

func TestSourceFlags(t *testing.T) {
	gcc, clang := CXX{Family: GCC, C: true}, CXX{Family: Clang}
	for _, c := range []struct {
		c    CXX
		asan bool
		want string
	}{
		{gcc, false, "-g -O0"},
		{clang, false, "-g -O0 -Qunused-arguments"},
		{gcc, true, "-g -O0 -fsanitize=address,undefined -fno-omit-frame-pointer -fno-sanitize-recover=all"},
	} {
		if got := strings.Join(SourceFlags(c.c, c.asan), " "); got != c.want {
			t.Errorf("SourceFlags(%v, %v) = %s, want %s", c.c.Family, c.asan, got, c.want)
		}
	}
	// None of gocpp's language flags: they are Go's semantics, not C's.
	for _, f := range LanguageFlags() {
		if slices.Contains(SourceFlags(clang, true), f) {
			t.Errorf("SourceFlags has the language flag %s", f)
		}
	}
}
