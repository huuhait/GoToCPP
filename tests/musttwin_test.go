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
//
// The same goes for the tests of the user macros (step 10.9), umacro_*.gp, whose twins are in
// tests/run/testdata/macro_twin: a macro is sugar for a function and its call, so the twin writes
// them by hand (the hidden arguments as constants, a lazy argument as a func literal, a wrap
// macro's code as a func literal) and real Go runs it.
//
// And for the tests of #Derive (step 10.10), derive_*.gp, whose twins are in
// tests/run/testdata/derive_twin: the twin writes the generated methods by hand.
func mustTwin(p *program) string {
	dir := ""
	switch {
	case strings.HasPrefix(p.name, "must_"):
		dir = "must_twin"
	case strings.HasPrefix(p.name, "umacro_"):
		dir = "macro_twin"
	case strings.HasPrefix(p.name, "derive_"):
		dir = "derive_twin"
	default:
		return ""
	}
	twin := filepath.Join(p.dir, "testdata", dir, p.name+".go")
	if _, err := os.Stat(twin); err != nil {
		return ""
	}
	return twin
}
