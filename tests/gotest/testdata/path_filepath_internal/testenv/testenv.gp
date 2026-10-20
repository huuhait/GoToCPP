// Package testenv stands in for internal/testenv in Go's tests of the ported packages: the few functions they call, for a Linux machine that is not a Go builder.
package testenv

import "testing"

// Builder returns the name of the builder (the GO_BUILDER_NAME variable); a developer's machine has none.
func Builder() string { return "" }

// MustHaveSymlink reports whether the OS supports symlinks (Linux does).
func MustHaveSymlink(t testing.TB) {}

// SkipFlaky skips t because of a known flaky test (a Go issue number).
func SkipFlaky(t testing.TB, issue int) {
	t.Helper()
	t.Skipf("skipping known flaky test; see golang.org/issue/%d", issue)
}
