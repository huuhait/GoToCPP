// Copyright 2026 The gocpp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package runtime

import (
	"sync/atomic"
	"testing"
	"unsafe"
)

// The runtime stub is tested for the behavior the closure relies on before any
// native exists: the two directly called functions, the two pulls
// (rand/randn), the sync/atomic hook bodies (which call the real sync/atomic
// package through their //go:linkname pulls), and the hooks that run on the
// ordinary path (sync's init: notifyListCheck and registerPoolCleanup).

func TestGOMAXPROCS(t *testing.T) {
	old := GOMAXPROCS(0)
	if old < 1 {
		t.Fatalf("GOMAXPROCS(0) = %d", old)
	}
	prev := GOMAXPROCS(7)
	if prev != old {
		t.Errorf("GOMAXPROCS(7) returned %d, want %d", prev, old)
	}
	if got := GOMAXPROCS(0); got != 7 {
		t.Errorf("after GOMAXPROCS(7), GOMAXPROCS(0) = %d", got)
	}
	GOMAXPROCS(prev)
}

func TestRand(t *testing.T) {
	seen := map[uint64]bool{}
	for i := 0; i < 100; i++ {
		v := rand()
		if seen[v] {
			t.Fatalf("rand() repeated %d", v)
		}
		seen[v] = true
	}
	for _, n := range []uint32{1, 2, 3, 4, 7, 100} {
		for i := 0; i < 200; i++ {
			if v := randn(n); v >= n {
				t.Fatalf("randn(%d) = %d", n, v)
			}
		}
	}
	if randn(0) != 0 {
		t.Error("randn(0) != 0")
	}
}

func TestNotifyListCheck(t *testing.T) {
	// sync's init calls this with unsafe.Sizeof(notifyList{}).
	sync_runtime_notifyListCheck(unsafe.Sizeof(notifyList{}))
	// A wrong size is a panic (as in the real runtime).
	func() {
		defer func() {
			if recover() == nil {
				t.Error("notifyListCheck(1) did not panic")
			}
		}()
		sync_runtime_notifyListCheck(1)
	}()
}

func TestPoolCleanupRegistration(t *testing.T) {
	// sync's init does this; it must not panic. With the real toolchain the
	// call goes to the real runtime's implementation, in the engine to the
	// stub's (hooks_body.go through hooks_link.go).
	sync_runtime_registerPoolCleanup(func() {})
	sync_runtime_registerPoolCleanup(nil)
}

func TestProcPin(t *testing.T) {
	// procPin must return a valid P id: sync.Pool uses it as an index into a
	// poolLocal array of GOMAXPROCS entries. The stub has one P, so 0.
	pid := sync_runtime_procPin()
	if pid < 0 || pid >= GOMAXPROCS(0) {
		t.Fatalf("sync_runtime_procPin() = %d, GOMAXPROCS = %d", pid, GOMAXPROCS(0))
	}
	sync_runtime_procUnpin()
	if pid := sync_atomic_runtime_procPin(); pid < 0 {
		t.Fatalf("sync_atomic_runtime_procPin() = %d", pid)
	}
	sync_atomic_runtime_procUnpin()
}

func TestSynctestNotInBubble(t *testing.T) {
	if synctest_isInBubble() {
		t.Error("IsInBubble() = true")
	}
	if got := synctest_associate(unsafe.Pointer(&t)); got != 0 {
		t.Errorf("associate() = %d, want Unbubbled (0)", got)
	}
	if synctest_isAssociated(unsafe.Pointer(&t)) {
		t.Error("isAssociated() = true")
	}
	synctest_disassociate(unsafe.Pointer(&t))
}

func TestAtomicPointerHooks(t *testing.T) {
	var p unsafe.Pointer
	a, b := 1, 2
	sync_atomic_StorePointer(&p, unsafe.Pointer(&a))
	if p != unsafe.Pointer(&a) {
		t.Errorf("StorePointer: p = %p, want %p", p, &a)
	}
	if !sync_atomic_CompareAndSwapPointer(&p, unsafe.Pointer(&a), unsafe.Pointer(&b)) {
		t.Error("CompareAndSwapPointer(old) = false")
	}
	if p != unsafe.Pointer(&b) {
		t.Errorf("CompareAndSwapPointer: p = %p, want %p", p, &b)
	}
	if sync_atomic_CompareAndSwapPointer(&p, unsafe.Pointer(&a), unsafe.Pointer(&b)) {
		t.Error("CompareAndSwapPointer(wrong old) = true")
	}
	if got := sync_atomic_SwapPointer(&p, unsafe.Pointer(&a)); got != unsafe.Pointer(&b) {
		t.Errorf("SwapPointer = %p, want %p", got, &b)
	}
	if p != unsafe.Pointer(&a) {
		t.Errorf("SwapPointer: p = %p, want %p", p, &a)
	}

	// The real sync/atomic package must agree about the same word (the hook
	// pulls sync/atomic.StoreUintptr etc., which the real package defines).
	var u uintptr
	sync_atomic_StoreUintptr(&u, 42)
	if got := atomic.LoadUintptr(&u); got != 42 {
		t.Errorf("StoreUintptr then atomic.LoadUintptr = %d", got)
	}
	if got := sync_atomic_SwapUintptr(&u, 7); got != 42 {
		t.Errorf("SwapUintptr = %d, want 42", got)
	}
	if !sync_atomic_CompareAndSwapUintptr(&u, 7, 9) {
		t.Error("CompareAndSwapUintptr(7,9) = false")
	}
}
