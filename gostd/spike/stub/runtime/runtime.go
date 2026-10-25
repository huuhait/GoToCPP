// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package runtime is job 16.1f-stubs' pure-Go stand-in for the parts of Go's
// runtime package that the four spike packages and their closure (minus the
// three stub packages) actually use.
//
// The closure analysis (gostd/spike/closure.json) found exactly:
//
//	direct API, called by closure packages:
//	    sync  -> runtime.GOMAXPROCS
//	    iter  -> runtime.Goexit
//	//go:linkname pulls into the runtime's namespace:
//	    iter          -> runtime.newcoro, runtime.coroswitch
//	    internal/sync -> runtime.rand
//	    sync          -> runtime.randn
//	//go:linkname pushes from the runtime into closure packages:
//	    42 symbols in sync, internal/sync, internal/synctest and sync/atomic
//	    (hooks.go; the same list is runtime.pushes_into_closure in the JSON)
//	//go:linkname pushes from the runtime into the stub packages:
//	    the seven internal/reflectlite.* hooks (hooks.go)
//
// Go's runtime is 71.7k lines over more than 120 assembly functions and is
// never translated (Phase 16 README §2.7): gocpp's own runtime is Go source
// over natives. This stub is the smallest version of that idea for the spike.
// Functions with a real implementation here are marked "Go source"; functions
// that the engine must implement as natives are marked "native" and are
// declarations without bodies (Go's own runtime style). With the real Go
// toolchain the declarations are accepted because of their //go:linkname; the
// engine's emitter turns a bodyless function into a native call.
package runtime

import (
	"unsafe"
)

// GOMAXPROCS sets the maximum number of CPUs that can be executing
// simultaneously and returns the previous setting.
//
// Go source in the stub: it remembers the value. 16.1f must make it a native
// over the engine's thread configuration (the engine has one OS thread per
// goroutine, README §2.5, so the meaningful value is the thread cap).
func GOMAXPROCS(n int) int {
	old := maxprocs
	if n > 0 {
		maxprocs = n
	}
	return old
}

var maxprocs = 1

// Goexit terminates the goroutine that calls it. No other goroutine is
// affected. Goexit runs all deferred calls before terminating the goroutine.
//
// Placeholder in the stub (panic). 16.1f must make it a native that unwinds
// the engine's goroutine stack and runs defers (step 16.6).
func Goexit() {
	panic("runtime.Goexit: the GC engine's Goexit is a 16.1f native (step 16.6)")
}

var randState uint64 = 0x9e3779b97f4a7c15

// rand returns a random uint64. It is the symbol behind
// //go:linkname runtime_rand runtime.rand in internal/sync/hashtriemap.go.
//
// Go source in the stub (splitmix64). The engine may keep it as Go source or
// make it a native over the shared random source; the algorithm does not
// matter to the closure.
func rand() uint64 {
	randState += 0x9e3779b97f4a7c15
	z := randState
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// randn returns a pseudo-random number in [0, n). It is the symbol behind
// //go:linkname runtime_randn runtime.randn in sync/pool.go.
func randn(n uint32) uint32 {
	if n == 0 {
		return 0
	}
	return uint32(rand() % uint64(n))
}

// coro is the opaque coroutine handle of iter.Pull. iter's own coro is an
// empty struct; only the pointer crosses the linkname boundary.
type coro struct{ _ byte }

// newcoro creates a coroutine. It is the symbol behind
// //go:linkname newcoro runtime.newcoro in iter/iter.go.
//
// Native in the stub: the engine's goroutines are OS threads (README §2.5),
// so iter.Pull needs a native that starts a goroutine and switches to it. This
// declaration is the native's signature; 16.1f implements it on the C++ side
// over the engine's goroutine primitive.
//
//go:linkname newcoro runtime.newcoro
func newcoro(f func(*coro)) *coro

// coroswitch switches to the given coroutine. It is the symbol behind
// //go:linkname coroswitch runtime.coroswitch in iter/iter.go.
//
// Native in the stub, like newcoro.
//
//go:linkname coroswitch runtime.coroswitch
func coroswitch(*coro)

// The unsafe marker: it is what lets the compiler accept the //go:linkname
// directives of bodyless declarations above and in hooks.go.
var _ unsafe.Pointer
