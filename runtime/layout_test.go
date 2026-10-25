package rtfiles_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
)

// TestLayout compiles test/layout.cpp and test/gc_layout_test.cpp, with g++ and with clang++:
// their static_asserts check the sizes, alignments and field offsets of C++ structs like those
// the emitter writes, against the numbers that go/types gets from gocppSizes
// (internal/check/sizes.go generates the first) and from Go's own sizes for the GC engine
// (internal/check/gcsizes.go generates the second). Every file that includes gocpp/rt.hpp also
// checks gocpp/sizes.hpp, the layouts of the reference-counting engine's types; gc/rt.hpp
// checks gc/sizes.hpp. TestWindowsSyntax compiles layout.cpp with MinGW-w64 too.
func TestLayout(t *testing.T) {
	t.Parallel()
	ccs := compilers(t)
	rt := extractRuntime(t)
	srcs := []string{sourcePath(t, "test/layout.cpp"), sourcePath(t, "test/gc_layout_test.cpp")}
	for _, c := range ccs {
		t.Run(c.name, func(t *testing.T) {
			if c.missing != "" {
				t.Skip(c.missing)
			}
			t.Parallel()
			for _, src := range srcs {
				args := append(c.flags([]string{"-O0", "-fsyntax-only"}), "-I", filepath.Join(rt, "include"), src)
				out, _, code, err := command(t.TempDir(), build.CompilerEnv(), true, c.cc.Path, args...)
				if err != nil || code != 0 || len(out) != 0 {
					t.Errorf("%s %s: exit %d %v\n%s", c.cc.Path, strings.Join(args, " "), code, err, out)
				}
			}
		})
	}
}
