// Package testenv stands in for internal/testenv in Go's tests of the standard packages (step 9.10): the few functions they call, for a
// Linux machine that is not a Go builder and has no Go tool and no os/exec. A test that needs a process or the go command is dropped
// by its specification (tools/portstd/gotests/std), with the reason.
package testenv

import "testing"

// Builder returns the name of the builder (the GO_BUILDER_NAME variable); a developer's machine has none.
func Builder() string { return "" }

// MustHaveSymlink reports whether the OS supports symlinks (Linux does).
func MustHaveSymlink(t testing.TB) {}

// HasSymlink reports whether the OS supports symlinks.
func HasSymlink() bool { return true }

// HasLink reports whether the OS supports hard links.
func HasLink() bool { return true }

// MustHaveLink skips t when the OS has no hard links (Linux has them).
func MustHaveLink(t testing.TB) {}

// SkipFlaky skips t because of a known flaky test (a Go issue number).
func SkipFlaky(t testing.TB, issue int) {
	t.Helper()
	t.Skipf("skipping known flaky test; see golang.org/issue/%d", issue)
}

// OptimizationOff reports whether the program was built with the Go compiler's optimizations off (-N); gocpp's builds are not Go's.
func OptimizationOff() bool { return false }

// SkipIfOptimizationOff skips t when the Go compiler's optimizations are off; they never are.
func SkipIfOptimizationOff(t testing.TB) {}

// NoSpecialBuildTypes skips t on the special builds of Go's own builders (race, msan, asan...): none is one.
func NoSpecialBuildTypes(t testing.TB) {}

// SyscallIsNotSupported reports whether err says that a system call is unavailable in the sandbox.
func SyscallIsNotSupported(err error) bool { return false }

// MustHaveExec skips t: gocpp has no os/exec.
func MustHaveExec(t testing.TB) {
	t.Helper()
	t.Skip("skipping test: gocpp has no os/exec")
}

// MustHaveExecPath skips t: gocpp has no os/exec.
func MustHaveExecPath(t testing.TB, path string) {
	t.Helper()
	t.Skip("skipping test: gocpp has no os/exec")
}

// MustHaveGoBuild skips t: a gocpp program has no Go tool to build with.
func MustHaveGoBuild(t testing.TB) {
	t.Helper()
	t.Skip("skipping test: gocpp tests do not run the go command")
}

// MustHaveGoRun skips t: a gocpp program has no Go tool to run.
func MustHaveGoRun(t testing.TB) {
	t.Helper()
	t.Skip("skipping test: gocpp tests do not run the go command")
}

// MustInternalLink skips t: gocpp tests do not link Go programs.
func MustInternalLink(t testing.TB, withCgo bool) {
	t.Helper()
	t.Skip("skipping test: gocpp tests do not link Go programs")
}

// GOROOT skips t: a gocpp program has no GOROOT.
func GOROOT(t testing.TB) string {
	t.Helper()
	t.Skip("skipping test: a gocpp program has no GOROOT")
	return ""
}

// GoToolPath skips t: a gocpp program has no Go tool.
func GoToolPath(t testing.TB) string {
	t.Helper()
	t.Skip("skipping test: a gocpp program has no go command")
	return ""
}
