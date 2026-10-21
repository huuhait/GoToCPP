// Package testgo is for tests that read a Go library or run a Go command and compare it with what the
// repository follows: they must run when the Go version is the one that go.mod names, and skip
// (with the reason) when it is another one, whatever version that is.
package testgo

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Minor returns the language version of a Go version, which is how go.mod says it: "go1.25.14" is
// "1.25", "go1.25" is "1.25" and "go1.27.1" is "1.27". It returns "" for a text that is no version.
func Minor(version string) string {
	rest, ok := strings.CutPrefix(strings.TrimSpace(version), "go")
	if !ok {
		return ""
	}
	parts := strings.Split(rest, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[0] + "." + parts[1]
}

// ModuleMinor returns the language version of the "go" line of the go.mod that is in dir or in the
// closest folder above it.
func ModuleMinor(dir string) (string, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for {
		file, err := os.Open(filepath.Join(dir, "go.mod"))
		if err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) == 2 && fields[0] == "go" {
					if minor := Minor("go" + fields[1]); minor != "" {
						return minor, nil
					}
				}
			}
			return "", fmt.Errorf("%s has no go line", file.Name())
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod above %s", dir)
		}
		dir = parent
	}
}

// SkipUnlessModuleVersion skips the test when version, the version of a Go library or command
// ("go1.25.14"), is not of the language version that go.mod names, which is the one the repository
// follows. what says what has that version, for the message.
func SkipUnlessModuleVersion(t testing.TB, what, version string) {
	t.Helper()
	want, err := ModuleMinor(".")
	if err != nil {
		t.Fatal(err)
	}
	if got := Minor(version); got != want {
		t.Skipf("%s is %s, and this test follows Go %s (the go line of go.mod)", what, strings.TrimSpace(version), want)
	}
}
