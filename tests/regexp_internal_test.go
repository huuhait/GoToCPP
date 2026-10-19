package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRegexpInternalCopy checks that the copy of package regexp in tests/pkgs/regexp_internal/regexp, the package that Go's internal
// tests (package regexp, naming unexported functions and fields) run in, is the port in stdlib/regexp, file for file.
func TestRegexpInternalCopy(t *testing.T) {
	copyDir := filepath.Join(repoRoot, "tests", "pkgs", "regexp_internal", "regexp")
	portDir := filepath.Join(repoRoot, "stdlib", "regexp")
	ports, err := filepath.Glob(filepath.Join(portDir, "*.gp"))
	if err != nil || len(ports) == 0 {
		t.Fatalf("no port files in %s (%v)", portDir, err)
	}
	for _, p := range ports {
		name := filepath.Base(p)
		want, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(copyDir, name))
		if err != nil {
			t.Errorf("%s: %v (copy stdlib/regexp/%s there)", name, err, name)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("tests/pkgs/regexp_internal/regexp/%s differs from stdlib/regexp/%s: copy it again", name, name)
		}
	}
	// The other files of the copy are the converted tests of Go (named *_gotest.gp) and the harness.
	ents, err := os.ReadDir(copyDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range ents {
		n := e.Name()
		if _, err := os.Stat(filepath.Join(portDir, n)); err == nil {
			continue
		}
		if !strings.HasSuffix(n, "_gotest.gp") && n != "harness.gp" {
			t.Errorf("tests/pkgs/regexp_internal/regexp/%s is neither a file of the port nor a converted test", n)
		}
	}
}
