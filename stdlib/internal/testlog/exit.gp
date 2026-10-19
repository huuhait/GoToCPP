// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Copied from go1.25.14's src/internal/testlog/exit.go (see stdlib/internal/PORT.md), without the mutex (sync is not
// built yet, Phase 08 step 8.7) and without the //go:linkname of the runtime's hook: the flag is set by
// testing before the tests run and read by os.Exit, as in Go.

package testlog

var panicOnExit0 bool

// PanicOnExit0 reports whether to panic on a call to os.Exit(0).
func PanicOnExit0() bool {
	return panicOnExit0
}

// SetPanicOnExit0 sets panic on os.Exit(0).
func SetPanicOnExit0(v bool) {
	panicOnExit0 = v
}
