// Package racetest has what the tests of the race detector's builds share.
package racetest

import "os"

// ChildExitSetting is the race detector option that stops a process from sleeping at exit.
const ChildExitSetting = "atexit_sleep_ms=0"

// ExitChildrenQuickly stops the child processes of a test from sleeping one second at exit.
// A test binary built with -race waits atexit_sleep_ms (default 1000) before it exits, to let
// the reports of other goroutines print. Tests that run the test binary again as a fake compiler
// or as a helper process pay that second for every child, and a timeout of a few hundred
// milliseconds cannot be met. The setting is read when a process starts, so calling this from
// TestMain changes only the children that the test starts afterwards, not the test process.
// The option is set whether or not the binary was built with -race (it is ignored otherwise, and
// on Windows). The cost is that a child can now exit before the race report of another of its
// goroutines is printed.
func ExitChildrenQuickly() {
	if existing := os.Getenv("GORACE"); existing != "" {
		os.Setenv("GORACE", existing+" "+ChildExitSetting)
		return
	}
	os.Setenv("GORACE", ChildExitSetting)
}
