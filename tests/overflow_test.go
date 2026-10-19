package tests

import (
	"slices"
	"testing"
)

// overflowFormat is the format of tests/overflow: a run test (an // Output: block, // ExitCode: and
// // Stderr: lines). See tests/overflow/README.md.
var overflowFormat = format{"overflow", outputLine | exitCodeLine | stderrLine | goBuildLine, true}

// TestStackOverflow runs the programs of tests/overflow: endless recursion on the main goroutine and
// in a goroutine must end with Go's "fatal error: stack overflow" and exit code 2, not with a bare
// SIGSEGV. Under a sanitizer (GOCPP_FLAGS=-asan or -race) the overflow is the sanitizer's report
// instead of gocpp's (the runtime's own tests, runtime/goroutine_test.go, cover that mode), so the
// test is skipped there.
func TestStackOverflow(t *testing.T) {
	c := setup(t)
	for _, f := range []string{"-asan", "-race"} {
		if slices.Contains(c.flags, f) {
			t.Skipf("GOCPP_FLAGS has %s: the sanitizer reports the overflow, not gocpp's handler", f)
		}
	}
	c.runPrograms(t, "tests/overflow", loadTests(t, overflowFormat), c.runTest)
}
