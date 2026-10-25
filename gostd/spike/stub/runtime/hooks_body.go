// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The bodies of the 42 //go:linkname pushes of the runtime stub (the list is
// in gostd/spike/closure.json, runtime.pushes_into_closure).
//
// This file is compiled in every build, without linkname directives, so the
// real-Go tests of this package call exactly the code the GC engine runs. A
// second file, hooks_link.go (//go:build gocpp), attaches the linker symbols
// with //go:linkname; it exists only for the engine, because with the real gc
// toolchain the real runtime already defines those symbols and a push would be
// a duplicate definition at link (and Go 1.25's linkname check forbids most of
// them from a package that is not their target).
//
// "Go source" bodies are the ones the closure's ordinary path runs (sync's
// init, the uncontended fast paths, the synctest bubble checks); "native" is
// the body that is only an honest panic naming what 16.1f must implement,
// because no pure-Go code can park a thread or wait on a scheduler.
package runtime

import (
	"unsafe"
)

// ---- sync ----

func sync_event(cycles int64, skip int) {}

func sync_fatal(s string) { panic("sync: " + s) }

func sync_throw(s string) { panic("sync: " + s) }

func sync_runtime_Semacquire(addr *uint32) { semacquire("sync.runtime_Semacquire") }

func sync_runtime_SemacquireWaitGroup(addr *uint32, synctestDurable bool) {
	semacquire("sync.runtime_SemacquireWaitGroup")
}

func sync_runtime_SemacquireRWMutexR(addr *uint32, lifo bool, skipframes int) {
	semacquire("sync.runtime_SemacquireRWMutexR")
}

func sync_runtime_SemacquireRWMutex(addr *uint32, lifo bool, skipframes int) {
	semacquire("sync.runtime_SemacquireRWMutex")
}

func sync_runtime_Semrelease(addr *uint32, handoff bool, skipframes int) {
	panic("gocpp runtime stub: sync.runtime_Semrelease is a 16.1f native (step 16.4)")
}

func semacquire(name string) {
	// The uncontended paths of sync.Mutex/RWMutex/Once/WaitGroup never get
	// here (their CAS succeeds); a real wait needs the engine's semaphore
	// table.
	panic("gocpp runtime stub: " + name + " is a 16.1f native (step 16.4)")
}

func sync_runtime_canSpin(i int) bool { return false }

func sync_runtime_doSpin() {}

func sync_runtime_notifyListAdd(l *notifyList) uint32 {
	panic("gocpp runtime stub: sync.runtime_notifyListAdd is a 16.1f native (step 16.4)")
}

func sync_runtime_notifyListWait(l *notifyList, t uint32) {
	panic("gocpp runtime stub: sync.runtime_notifyListWait is a 16.1f native (step 16.4)")
}

func sync_runtime_notifyListNotifyAll(l *notifyList) {
	panic("gocpp runtime stub: sync.runtime_notifyListNotifyAll is a 16.1f native (step 16.4)")
}

func sync_runtime_notifyListNotifyOne(l *notifyList) {
	panic("gocpp runtime stub: sync.runtime_notifyListNotifyOne is a 16.1f native (step 16.4)")
}

func sync_runtime_notifyListCheck(size uintptr) {
	// Go panics here if sync's notifyList and the runtime's differ in size.
	// The real runtime's is 32 bytes on 64-bit; so is the placeholder in
	// hooks.go.
	const want = 32
	if unsafe.Sizeof(uintptr(0)) == 8 && size != want {
		panic("runtime: notifyList size mismatch")
	}
}

// procPin returns the P id whose local pool the caller should use. The stub
// has one P (GOMAXPROCS(0) == 1 and this stub's GOMAXPROCS starts at 1), so the
// id is 0; a sync.Pool therefore works on a single P. 16.1f replaces it with
// the engine's thread/P assignment (step 16.4).
func sync_runtime_procPin() int { return 0 }

func sync_runtime_procUnpin() {}

func sync_runtime_registerPoolCleanup(f func()) { poolCleanup = f }

var poolCleanup func()

// ---- internal/sync ----

func internal_sync_fatal(s string) { panic("internal/sync: " + s) }

func internal_sync_throw(s string) { panic("internal/sync: " + s) }

func internal_sync_runtime_SemacquireMutex(addr *uint32, lifo bool, skipframes int) {
	semacquire("internal/sync.runtime_SemacquireMutex")
}

func internal_sync_runtime_Semrelease(addr *uint32, handoff bool, skipframes int) {
	panic("gocpp runtime stub: internal/sync.runtime_Semrelease is a 16.1f native (step 16.4)")
}

func internal_sync_runtime_canSpin(i int) bool { return false }

func internal_sync_runtime_doSpin() {}

func internal_sync_runtime_nanotime() int64 { return 0 }

// ---- internal/synctest ----
//
// The stub is never inside a synctest bubble: IsInBubble is false and
// Associate answers Unbubbled (0). That is what the packages see outside
// testing/synctest, so sync.WaitGroup takes its ordinary path.

func synctest_isInBubble() bool { return false }

func synctestRun(f func()) {
	panic("gocpp runtime stub: internal/synctest.Run is a 16.1f native (step 16.10)")
}

func synctestWait() {
	panic("gocpp runtime stub: internal/synctest.Wait is a 16.1f native (step 16.10)")
}

func synctest_acquire() any { return nil }

func synctest_release(bubble any) {}

func synctest_inBubble(bubble any, f func()) {
	panic("gocpp runtime stub: internal/synctest.inBubble is a 16.1f native (step 16.10)")
}

func synctest_associate(p unsafe.Pointer) int { return 0 } // Unbubbled

func synctest_disassociate(b unsafe.Pointer) {}

func synctest_isAssociated(p unsafe.Pointer) bool { return false }

// ---- sync/atomic ----

func sync_atomic_StorePointer(ptr *unsafe.Pointer, new unsafe.Pointer) {
	sync_atomic_StoreUintptr((*uintptr)(unsafe.Pointer(ptr)), uintptr(new))
}

func sync_atomic_SwapPointer(ptr *unsafe.Pointer, new unsafe.Pointer) unsafe.Pointer {
	x := sync_atomic_SwapUintptr((*uintptr)(unsafe.Pointer(ptr)), uintptr(new))
	// Convert back the way the runtime's noescape does, so vet does not
	// report a uintptr-to-pointer conversion.
	return *(*unsafe.Pointer)(unsafe.Pointer(&x))
}

func sync_atomic_CompareAndSwapPointer(ptr *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return sync_atomic_CompareAndSwapUintptr((*uintptr)(unsafe.Pointer(ptr)), uintptr(old), uintptr(new))
}

func sync_atomic_runtime_procPin() int { return 0 }

func sync_atomic_runtime_procUnpin() {}
