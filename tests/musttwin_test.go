package tests

import (
	"os"
	"path/filepath"
	"strings"
)

// mustTwin returns the path of the hand-written Go twin of a run test that uses the `!` forms of
// Phase 10 (tests/run/testdata/must_twin/NAME.go for tests/run/NAME.gp), or "" if the test has none.
//
// `!` is sugar for the `if err != nil` code that a Go programmer writes, so a test program with
// `!` has a twin that is valid Go and does the same by hand (its errors.Wrapf is a small Go
// function). TestRunVsGo runs the twin with real Go and compares what it prints with the test's
// // Output: block, so the expected output of a `!` program comes from real Go, not from
// gocpp (tests/run/README.md, "Programs with !").
func mustTwin(p *program) string {
	if !strings.HasPrefix(p.name, "must_") {
		return ""
	}
	twin := filepath.Join(p.dir, "testdata", "must_twin", p.name+".go")
	if _, err := os.Stat(twin); err != nil {
		return ""
	}
	return twin
}
