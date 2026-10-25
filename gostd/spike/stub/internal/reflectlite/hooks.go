// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The runtime hooks of the stub.
//
// Go's internal/reflectlite has no bodies for seven functions; its asm.s is a
// marker and the runtime provides them through //go:linkname pushes into the
// internal/reflectlite package (runtime/runtime1.go, runtime/mbarrier.go,
// runtime/malloc.go, runtime/iface.go, runtime/chan.go, runtime/map_swiss.go):
//
//	internal/reflectlite.resolveNameOff(unsafe.Pointer, int32) unsafe.Pointer
//	internal/reflectlite.resolveTypeOff(unsafe.Pointer, int32) unsafe.Pointer
//	internal/reflectlite.unsafe_New(*abi.Type) unsafe.Pointer
//	internal/reflectlite.typedmemmove(*abi.Type, unsafe.Pointer, unsafe.Pointer)
//	internal/reflectlite.ifaceE2I(*abi.Type, any, unsafe.Pointer)
//	internal/reflectlite.chanlen(unsafe.Pointer) int
//	internal/reflectlite.maplen(unsafe.Pointer) int
//
// The stub declares the same seven functions with //go:linkname pulls of those
// linker symbols (see the declarations in type.go and value.go). With the real
// Go toolchain the real runtime's pushes satisfy them, which is what makes the
// differential tests next to this file possible. In the GC engine, the runtime
// stub (gostd/spike/stub/runtime/hooks.go) has to provide the same seven
// symbols as natives; that is the part of 16.1f's C++ work this package needs:
//
//   - resolveNameOff/resolveTypeOff: the module data pointer tables of Phase 16
//     README §2.8 (resolve*Off natives);
//   - unsafe_New: an allocation of Size_ zeroed bytes in the GC heap;
//   - typedmemmove: a copy of Size_ bytes (no write barrier is needed with a
//     conservative collector, but a copying or generational one would need
//     the type's pointer mask);
//   - ifaceE2I: build a non-empty interface value (itab for (type, interface)),
//     the getitab path of README §2.8;
//   - chanlen/maplen: the length word of a channel/map (the stub's Value.Len
//     supports both kinds, though the closure uses only slices and strings).
package reflectlite
