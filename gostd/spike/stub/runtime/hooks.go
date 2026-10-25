// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The //go:linkname hook table of the runtime stub.
//
// Go's runtime pushes 42 symbols into the closure packages (sync,
// internal/sync, internal/synctest, sync/atomic) and 7 into the stub package
// internal/reflectlite; closure packages pull 4 more from the runtime
// (runtime.go). gostd/spike/closure.json records the whole set with the real
// runtime signature and the source line of the push
// (runtime.pushes_into_closure and runtime.pulls). This file is the part of
// that table that is the same in every build:
//
//   - the three sync/atomic uintptr forms, which are the sync/atomic package's
//     own symbols and therefore references (Go's runtime pulls them the same
//     way: runtime/atomic_pointer.go's declarations have no body);
//   - the seven internal/reflectlite hooks, bodyless natives (the reflectlite
//     stub pulls them).
//
// The 39 push definitions are split by build constraint:
//
//   - hooks_body.go (every build) holds their bodies as ordinary,
//     un-linknamed functions. Where the real runtime does work only a
//     scheduler or collector can do, the body is an honest panic naming the
//     native 16.1f must implement (steps 16.4, 16.6 and 16.10); the ordinary
//     paths of the closure are real. The real-Go tests call these functions.
//   - hooks_link.go (//go:build gocpp, the engine's compiler tag, README §2.7)
//     attaches the 39 linker names with //go:linkname, each wrapper calling
//     the body. It exists only for the engine: with the real gc toolchain the
//     real runtime already defines those symbols, so a push would be a
//     duplicate definition at link (and Go 1.25's linkname check forbids most
//     of them from a package that is not their target).
package runtime

import (
	"unsafe"
)

// notifyList stands in for the runtime's notifyList. Only the pointer crosses
// the linkname boundary; the size must match sync.notifyList (32 bytes on
// 64-bit: two uint32, a lock and two pointers) so a future native that reads
// the fields of a sync.notifyList through this type is not wrong.
type notifyList struct {
	_ [32]byte
}

// The uintptr forms of the sync/atomic hooks are the package's own to define
// (Go's runtime pulls them the same way); the pointer forms in
// hooks_body.go are the runtime's, because they need the write barrier Go's
// runtime would apply.
//
//go:linkname sync_atomic_StoreUintptr sync/atomic.StoreUintptr
func sync_atomic_StoreUintptr(ptr *uintptr, val uintptr)

//go:linkname sync_atomic_SwapUintptr sync/atomic.SwapUintptr
func sync_atomic_SwapUintptr(ptr *uintptr, new uintptr) uintptr

//go:linkname sync_atomic_CompareAndSwapUintptr sync/atomic.CompareAndSwapUintptr
func sync_atomic_CompareAndSwapUintptr(ptr *uintptr, old, new uintptr) bool

// ---- internal/reflectlite ----
//
// The seven symbols the reflectlite stub pulls. They are bodyless here: 16.1f
// implements them as natives (see the reflectlite package's hooks.go for what
// each one has to do). The push names are the linker symbols the stub's pulls
// name, exactly as Go's runtime pushes them.

//go:linkname reflectlite_resolveNameOff internal/reflectlite.resolveNameOff
func reflectlite_resolveNameOff(ptrInModule unsafe.Pointer, off int32) unsafe.Pointer

//go:linkname reflectlite_resolveTypeOff internal/reflectlite.resolveTypeOff
func reflectlite_resolveTypeOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

//go:linkname reflectlite_unsafe_New internal/reflectlite.unsafe_New
func reflectlite_unsafe_New(typ unsafe.Pointer) unsafe.Pointer

//go:linkname reflectlite_typedmemmove internal/reflectlite.typedmemmove
func reflectlite_typedmemmove(typ unsafe.Pointer, dst, src unsafe.Pointer)

//go:linkname reflectlite_ifaceE2I internal/reflectlite.ifaceE2I
func reflectlite_ifaceE2I(inter unsafe.Pointer, e any, dst unsafe.Pointer)

//go:linkname reflectlite_chanlen internal/reflectlite.chanlen
func reflectlite_chanlen(c unsafe.Pointer) int

//go:linkname reflectlite_maplen internal/reflectlite.maplen
func reflectlite_maplen(m unsafe.Pointer) int
