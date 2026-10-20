// Package tsupport has the helpers that the tests of typed interop share:
// loading and checking a .gp source, and finding clang++.
package tsupport

import (
	"context"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/huuhait/gocpp/internal/build"
	"github.com/huuhait/gocpp/internal/check"
	"github.com/huuhait/gocpp/internal/load"
	"github.com/huuhait/gocpp/stdlib"
)

// Main is the body of TestMain for the packages of typed interop: no module
// cache, no network, no disk cache for the kind probe (GOCPP_PROBE_CACHE=1 keeps it).
func Main(m *testing.M) {
	os.Setenv("GOCPP_MODCACHE", "off")
	os.Setenv("GOCPP_OFFLINE", "1")
	// A real clang++ run takes about a second on a quiet machine and minutes when the
	// machine is full; a test must not fail for that (probe.EnvTimeout).
	if os.Getenv("GOCPP_PROBE_TIMEOUT") == "" {
		os.Setenv("GOCPP_PROBE_TIMEOUT", "5m")
	}
	if os.Getenv("GOCPP_PROBE_CACHE") == "" {
		os.Setenv("GOCPP_NOPROBECACHE", "1")
	}
	os.Exit(m.Run())
}

// CXX returns a C++ compiler for the tests (the namespace probe of
// check needs one), or skips the test.
func CXX(t testing.TB) build.CXX {
	t.Helper()
	for _, prog := range []string{"clang++", "g++"} {
		if _, err := exec.LookPath(prog); err != nil {
			continue
		}
		cxx, err := build.LookCXX(context.Background(), prog)
		if err == nil {
			return cxx
		}
	}
	for _, want := range strings.Split(os.Getenv("GOCPP_REQUIRE_CXX"), ",") {
		if want = strings.TrimSpace(want); want != "" {
			t.Fatalf("GOCPP_REQUIRE_CXX needs %s, but no C++ compiler works", want)
		}
	}
	t.Skip("no C++ compiler")
	return build.CXX{}
}

// CheckDir loads and checks the .gp file name in dir (as gocpp check does)
// and returns its package.
func CheckDir(t testing.TB, dir, name string) (*check.Package, *token.FileSet) {
	t.Helper()
	return CheckDirConfig(t, dir, name, nil)
}

// CheckDirConfig is CheckDir with a change to the check's configuration (the hook of the second
// pass for promoted C++ members, check.Config.CppMembers).
func CheckDirConfig(t testing.TB, dir, name string, change func(*check.Config)) (*check.Package, *token.FileSet) {
	t.Helper()
	cxx := CXX(t)
	cfg := load.Config{Dir: dir, Stdlib: stdlib.FS(), GoStd: stdlib.IsStd, GOOS: "linux", GOARCH: "amd64"}
	prog := load.Load(cfg, name)
	ccfg := check.Config{Phase: check.LastPhase, CXX: func(context.Context) (build.CXX, error) { return cxx, nil }}
	if change != nil {
		change(&ccfg)
	}
	r := check.Check(prog, ccfg)
	for _, p := range r.Packages {
		if p.Load.Dir == dir || len(prog.Packages) == 1 {
			return p, prog.Fset
		}
	}
	t.Fatalf("no package for %s", name)
	return nil, nil
}

// CheckSource writes src as main.gp (plus the extra files) in a new folder
// and checks it.
func CheckSource(t testing.TB, src string, extra map[string]string) (*check.Package, *token.FileSet) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.gp"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for name, text := range extra {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return CheckDir(t, dir, "main.gp")
}

// ClangProgram is the clang++ the tests probe with: the program in GOCPP_CLANG when it is set
// (tools/clang-matrix.sh sets it for each version), else clang++ in PATH. It is the rule of
// probe.FindClang.
func ClangProgram() string {
	if prog := os.Getenv("GOCPP_CLANG"); prog != "" {
		return prog
	}
	return "clang++"
}

// Clang returns clang++ for the probes (ClangProgram), or skips the test (and fails it when
// GOCPP_REQUIRE_CXX names clang++).
func Clang(t testing.TB) build.CXX {
	t.Helper()
	cxx, err := build.LookCXX(context.Background(), ClangProgram())
	if err == nil && cxx.Family == build.Clang {
		return cxx
	}
	if strings.Contains(os.Getenv("GOCPP_REQUIRE_CXX"), "clang++") {
		t.Fatalf("GOCPP_REQUIRE_CXX needs clang++, but it doesn't work: %v", err)
	}
	t.Skip("no clang++ (typed interop needs it)")
	return build.CXX{}
}
