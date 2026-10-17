package rtfiles_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// TestLayout compiles test/layout.cpp, with g++ and with clang++: its static_asserts check the
// sizes, alignments and field offsets of C++ structs like those the emitter writes, against
// the numbers that go/types gets from gocppSizes (internal/check/sizes.go generates the file).
// Every file that includes gocpp/rt.hpp also checks gocpp/sizes.hpp, the layouts of the
// runtime's types. TestWindowsSyntax compiles layout.cpp with MinGW-w64 too.
func TestLayout(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	src := sourcePath(t, "test/layout.cpp")
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			args := append(c.flags([]string{"-O0", "-fsyntax-only"}), "-I", filepath.Join(rt, "include"), src)
			out, _, code, err := command(t.TempDir(), build.CompilerEnv(), true, c.cc.Path, args...)
			if err != nil || code != 0 || len(out) != 0 {
				t.Errorf("%s %s: exit %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
			}
		})
	}
}
