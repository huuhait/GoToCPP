package typedinterop

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/typedinterop/probe"
	"github.com/huuhait/gocpp/internal/typedinterop/tsupport"
)

// The clang that the tests probe with (Phase 12 step 12.12, the matrix of ADR-108). The result of a
// run is only evidence for the clang that ran: this test logs its version, checks that it is one
// gocpp supports (build.MinClang and newer), and, when tools/clang-matrix.sh says which major it
// asked for in GOCPP_EXPECT_CLANG_MAJOR, that GOCPP_CLANG reached the probes (a test that cleared
// the variable would otherwise test the default clang for every version of the matrix).
func TestClangInUse(t *testing.T) {
	cxx, err := probe.FindClang(context.Background())
	if err != nil {
		t.Skipf("no clang++: %v", err)
	}
	// The helper that most tests use must pick the same compiler as the product does.
	if other := tsupport.Clang(t); other.Path != cxx.Path {
		t.Errorf("tsupport.Clang uses %s, probe.FindClang uses %s", other.Path, cxx.Path)
	}
	t.Logf("probing with %s %d (%s)", cxx.Name(), cxx.Major, cxx.Version)
	if cxx.Family != build.Clang || cxx.Major < build.MinClang {
		t.Errorf("the probe compiler is %v %d, want clang %d or newer",
			cxx.Family, cxx.Major, build.MinClang)
	}
	if want := os.Getenv("GOCPP_EXPECT_CLANG_MAJOR"); want != "" {
		n, err := strconv.Atoi(want)
		if err != nil {
			t.Fatalf("GOCPP_EXPECT_CLANG_MAJOR=%q is no number", want)
		}
		if cxx.Major != n {
			t.Errorf("the matrix asked for clang %d, the probes use clang %d (%s)",
				n, cxx.Major, cxx.Version)
		}
	}
}
