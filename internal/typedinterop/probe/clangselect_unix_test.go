//go:build unix

package probe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/typedinterop/collect"
)

// The probe tests use the clang++ that GOCPP_CLANG names (tsupport.Clang, the rule of FindClang), so
// that tools/clang-matrix.sh tests the version it asks for and not the clang++ of PATH. The proof is
// a wrapper that logs its calls and runs the real clang: after a probe through clangConfig the log
// must have the probe's compile line.
func TestProbeTestsUseTheClangOfTheMatrix(t *testing.T) {
	real, err := exec.LookPath("clang++")
	if err != nil {
		t.Skip("no clang++")
	}
	dir := t.TempDir()
	log := filepath.Join(dir, "calls.log")
	wrapper := filepath.Join(dir, "clang-wrapper")
	script := "#!/bin/sh\necho \"$*\" >> '" + log + "'\nexec '" + real + "' \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOCPP_CLANG", wrapper)

	cfg := clangConfig(t, t.TempDir())
	set := collect.NewSet(ref(pth("BUFSIZ")))
	gc := GenConfig{Includes: []string{"<cstdio>"}, Kinds: stdKinds}
	if _, err := new(Runner).Probe(context.Background(), cfg, set, gc); err != nil {
		t.Fatal(err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatalf("the wrapper named by GOCPP_CLANG was never called: %v", err)
	}
	if !strings.Contains(string(calls), "-ast-dump=json") {
		t.Errorf("the wrapper was called, but not for the probe:\n%s", calls)
	}
}
