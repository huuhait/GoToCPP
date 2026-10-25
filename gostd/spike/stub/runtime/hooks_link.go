// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gocpp

// The //go:linkname pushes of the runtime stub: the 42 symbols Go's runtime
// pushes into sync, internal/sync, internal/synctest and sync/atomic (see
// gostd/spike/closure.json, runtime.pushes_into_closure). This file is
// compiled only with the gocpp compiler tag (the GC engine's build, Phase 16
// README 2.7); each function is a thin wrapper around its body in
// hooks_body.go, which is the code the real-Go tests exercise. The bodies live
// outside this file because //go:linkname must sit directly on the declaration
// it renames, and because with the real gc toolchain the real runtime already
// defines every one of these symbols.
package runtime

import (
	"unsafe"
)

//go:linkname link_sync_event sync.event
func link_sync_event(cycles int64, skip int) {
	sync_event(cycles, skip)
}

//go:linkname link_sync_fatal sync.fatal
func link_sync_fatal(s string) {
	sync_fatal(s)
}

//go:linkname link_sync_throw sync.throw
func link_sync_throw(s string) {
	sync_throw(s)
}

//go:linkname link_sync_runtime_Semacquire sync.runtime_Semacquire
func link_sync_runtime_Semacquire(addr *uint32) {
	sync_runtime_Semacquire(addr)
}

//go:linkname link_sync_runtime_SemacquireWaitGroup sync.runtime_SemacquireWaitGroup
func link_sync_runtime_SemacquireWaitGroup(addr *uint32, synctestDurable bool) {
	sync_runtime_SemacquireWaitGroup(addr, synctestDurable)
}

//go:linkname link_sync_runtime_SemacquireRWMutexR sync.runtime_SemacquireRWMutexR
func link_sync_runtime_SemacquireRWMutexR(addr *uint32, lifo bool, skipframes int) {
	sync_runtime_SemacquireRWMutexR(addr, lifo, skipframes)
}

//go:linkname link_sync_runtime_SemacquireRWMutex sync.runtime_SemacquireRWMutex
func link_sync_runtime_SemacquireRWMutex(addr *uint32, lifo bool, skipframes int) {
	sync_runtime_SemacquireRWMutex(addr, lifo, skipframes)
}

//go:linkname link_sync_runtime_Semrelease sync.runtime_Semrelease
func link_sync_runtime_Semrelease(addr *uint32, handoff bool, skipframes int) {
	sync_runtime_Semrelease(addr, handoff, skipframes)
}

//go:linkname link_sync_runtime_canSpin sync.runtime_canSpin
func link_sync_runtime_canSpin(i int) bool {
	return sync_runtime_canSpin(i)
}

//go:linkname link_sync_runtime_doSpin sync.runtime_doSpin
func link_sync_runtime_doSpin() {
	sync_runtime_doSpin()
}

//go:linkname link_sync_runtime_notifyListAdd sync.runtime_notifyListAdd
func link_sync_runtime_notifyListAdd(l *notifyList) uint32 {
	return sync_runtime_notifyListAdd(l)
}

//go:linkname link_sync_runtime_notifyListWait sync.runtime_notifyListWait
func link_sync_runtime_notifyListWait(l *notifyList, t uint32) {
	sync_runtime_notifyListWait(l, t)
}

//go:linkname link_sync_runtime_notifyListNotifyAll sync.runtime_notifyListNotifyAll
func link_sync_runtime_notifyListNotifyAll(l *notifyList) {
	sync_runtime_notifyListNotifyAll(l)
}

//go:linkname link_sync_runtime_notifyListNotifyOne sync.runtime_notifyListNotifyOne
func link_sync_runtime_notifyListNotifyOne(l *notifyList) {
	sync_runtime_notifyListNotifyOne(l)
}

//go:linkname link_sync_runtime_notifyListCheck sync.runtime_notifyListCheck
func link_sync_runtime_notifyListCheck(size uintptr) {
	sync_runtime_notifyListCheck(size)
}

//go:linkname link_sync_runtime_procPin sync.runtime_procPin
func link_sync_runtime_procPin() int {
	return sync_runtime_procPin()
}

//go:linkname link_sync_runtime_procUnpin sync.runtime_procUnpin
func link_sync_runtime_procUnpin() {
	sync_runtime_procUnpin()
}

//go:linkname link_sync_runtime_registerPoolCleanup sync.runtime_registerPoolCleanup
func link_sync_runtime_registerPoolCleanup(f func()) {
	sync_runtime_registerPoolCleanup(f)
}

//go:linkname link_internal_sync_fatal internal/sync.fatal
func link_internal_sync_fatal(s string) {
	internal_sync_fatal(s)
}

//go:linkname link_internal_sync_throw internal/sync.throw
func link_internal_sync_throw(s string) {
	internal_sync_throw(s)
}

//go:linkname link_internal_sync_runtime_SemacquireMutex internal/sync.runtime_SemacquireMutex
func link_internal_sync_runtime_SemacquireMutex(addr *uint32, lifo bool, skipframes int) {
	internal_sync_runtime_SemacquireMutex(addr, lifo, skipframes)
}

//go:linkname link_internal_sync_runtime_Semrelease internal/sync.runtime_Semrelease
func link_internal_sync_runtime_Semrelease(addr *uint32, handoff bool, skipframes int) {
	internal_sync_runtime_Semrelease(addr, handoff, skipframes)
}

//go:linkname link_internal_sync_runtime_canSpin internal/sync.runtime_canSpin
func link_internal_sync_runtime_canSpin(i int) bool {
	return internal_sync_runtime_canSpin(i)
}

//go:linkname link_internal_sync_runtime_doSpin internal/sync.runtime_doSpin
func link_internal_sync_runtime_doSpin() {
	internal_sync_runtime_doSpin()
}

//go:linkname link_internal_sync_runtime_nanotime internal/sync.runtime_nanotime
func link_internal_sync_runtime_nanotime() int64 {
	return internal_sync_runtime_nanotime()
}

//go:linkname link_synctest_isInBubble internal/synctest.IsInBubble
func link_synctest_isInBubble() bool {
	return synctest_isInBubble()
}

//go:linkname link_synctestRun internal/synctest.Run
func link_synctestRun(f func()) {
	synctestRun(f)
}

//go:linkname link_synctestWait internal/synctest.Wait
func link_synctestWait() {
	synctestWait()
}

//go:linkname link_synctest_acquire internal/synctest.acquire
func link_synctest_acquire() any {
	return synctest_acquire()
}

//go:linkname link_synctest_release internal/synctest.release
func link_synctest_release(bubble any) {
	synctest_release(bubble)
}

//go:linkname link_synctest_inBubble internal/synctest.inBubble
func link_synctest_inBubble(bubble any, f func()) {
	synctest_inBubble(bubble, f)
}

//go:linkname link_synctest_associate internal/synctest.associate
func link_synctest_associate(p unsafe.Pointer) int {
	return synctest_associate(p)
}

//go:linkname link_synctest_disassociate internal/synctest.disassociate
func link_synctest_disassociate(b unsafe.Pointer) {
	synctest_disassociate(b)
}

//go:linkname link_synctest_isAssociated internal/synctest.isAssociated
func link_synctest_isAssociated(p unsafe.Pointer) bool {
	return synctest_isAssociated(p)
}

//go:linkname link_sync_atomic_StorePointer sync/atomic.StorePointer
func link_sync_atomic_StorePointer(ptr *unsafe.Pointer, new unsafe.Pointer) {
	sync_atomic_StorePointer(ptr, new)
}

//go:linkname link_sync_atomic_SwapPointer sync/atomic.SwapPointer
func link_sync_atomic_SwapPointer(ptr *unsafe.Pointer, new unsafe.Pointer) unsafe.Pointer {
	return sync_atomic_SwapPointer(ptr, new)
}

//go:linkname link_sync_atomic_CompareAndSwapPointer sync/atomic.CompareAndSwapPointer
func link_sync_atomic_CompareAndSwapPointer(ptr *unsafe.Pointer, old, new unsafe.Pointer) bool {
	return sync_atomic_CompareAndSwapPointer(ptr, old, new)
}

//go:linkname link_sync_atomic_runtime_procPin sync/atomic.runtime_procPin
func link_sync_atomic_runtime_procPin() int {
	return sync_atomic_runtime_procPin()
}

//go:linkname link_sync_atomic_runtime_procUnpin sync/atomic.runtime_procUnpin
func link_sync_atomic_runtime_procUnpin() {
	sync_atomic_runtime_procUnpin()
}
