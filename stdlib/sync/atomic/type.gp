// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package atomic

// An Int32 is an atomic int32. The zero value is zero.
//
// Int32 must not be copied after first use.
type Int32 struct {
	v int32
}

// Load atomically loads and returns the value stored in x.
func (x *Int32) Load() int32 {
	return loadInt32(x)
}

// Store atomically stores val into x.
func (x *Int32) Store(val int32) {
	storeInt32(x, val)
}

// Swap atomically stores new into x and returns the previous value.
func (x *Int32) Swap(new int32) (old int32) {
	return swapInt32(x, new)
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Int32) CompareAndSwap(old, new int32) (swapped bool) {
	return casInt32(x, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Int32) Add(delta int32) (new int32) {
	return addInt32(x, delta)
}

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int32) And(mask int32) (old int32) {
	return andInt32(x, mask)
}

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int32) Or(mask int32) (old int32) {
	return orInt32(x, mask)
}

// An Int64 is an atomic int64. The zero value is zero.
//
// Int64 must not be copied after first use.
type Int64 struct {
	v int64
}

// Load atomically loads and returns the value stored in x.
func (x *Int64) Load() int64 {
	return loadInt64(x)
}

// Store atomically stores val into x.
func (x *Int64) Store(val int64) {
	storeInt64(x, val)
}

// Swap atomically stores new into x and returns the previous value.
func (x *Int64) Swap(new int64) (old int64) {
	return swapInt64(x, new)
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Int64) CompareAndSwap(old, new int64) (swapped bool) {
	return casInt64(x, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Int64) Add(delta int64) (new int64) {
	return addInt64(x, delta)
}

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int64) And(mask int64) (old int64) {
	return andInt64(x, mask)
}

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Int64) Or(mask int64) (old int64) {
	return orInt64(x, mask)
}

// A Uint32 is an atomic uint32. The zero value is zero.
//
// Uint32 must not be copied after first use.
type Uint32 struct {
	v uint32
}

// Load atomically loads and returns the value stored in x.
func (x *Uint32) Load() uint32 {
	return loadUint32(x)
}

// Store atomically stores val into x.
func (x *Uint32) Store(val uint32) {
	storeUint32(x, val)
}

// Swap atomically stores new into x and returns the previous value.
func (x *Uint32) Swap(new uint32) (old uint32) {
	return swapUint32(x, new)
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Uint32) CompareAndSwap(old, new uint32) (swapped bool) {
	return casUint32(x, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Uint32) Add(delta uint32) (new uint32) {
	return addUint32(x, delta)
}

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint32) And(mask uint32) (old uint32) {
	return andUint32(x, mask)
}

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint32) Or(mask uint32) (old uint32) {
	return orUint32(x, mask)
}

// A Uint64 is an atomic uint64. The zero value is zero.
//
// Uint64 must not be copied after first use.
type Uint64 struct {
	v uint64
}

// Load atomically loads and returns the value stored in x.
func (x *Uint64) Load() uint64 {
	return loadUint64(x)
}

// Store atomically stores val into x.
func (x *Uint64) Store(val uint64) {
	storeUint64(x, val)
}

// Swap atomically stores new into x and returns the previous value.
func (x *Uint64) Swap(new uint64) (old uint64) {
	return swapUint64(x, new)
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Uint64) CompareAndSwap(old, new uint64) (swapped bool) {
	return casUint64(x, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Uint64) Add(delta uint64) (new uint64) {
	return addUint64(x, delta)
}

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint64) And(mask uint64) (old uint64) {
	return andUint64(x, mask)
}

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uint64) Or(mask uint64) (old uint64) {
	return orUint64(x, mask)
}

// A Uintptr is an atomic uintptr. The zero value is zero.
//
// Uintptr must not be copied after first use.
type Uintptr struct {
	v uintptr
}

// Load atomically loads and returns the value stored in x.
func (x *Uintptr) Load() uintptr {
	return loadUintptr(x)
}

// Store atomically stores val into x.
func (x *Uintptr) Store(val uintptr) {
	storeUintptr(x, val)
}

// Swap atomically stores new into x and returns the previous value.
func (x *Uintptr) Swap(new uintptr) (old uintptr) {
	return swapUintptr(x, new)
}

// CompareAndSwap executes the compare-and-swap operation for x.
func (x *Uintptr) CompareAndSwap(old, new uintptr) (swapped bool) {
	return casUintptr(x, old, new)
}

// Add atomically adds delta to x and returns the new value.
func (x *Uintptr) Add(delta uintptr) (new uintptr) {
	return addUintptr(x, delta)
}

// And atomically performs a bitwise AND operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uintptr) And(mask uintptr) (old uintptr) {
	return andUintptr(x, mask)
}

// Or atomically performs a bitwise OR operation on x using the bitmask
// provided as mask and returns the old value.
func (x *Uintptr) Or(mask uintptr) (old uintptr) {
	return orUintptr(x, mask)
}

// A Bool is an atomic boolean value.
// The zero value is false.
//
// Bool must not be copied after first use.
type Bool struct {
	v uint32
}

func b32(b bool) uint32 {
	if b {
		return 1
	}
	return 0
}

// Load atomically loads and returns the value stored in x.
func (x *Bool) Load() bool {
	return loadBool(x) != 0
}

// Store atomically stores val into x.
func (x *Bool) Store(val bool) {
	storeBool(x, b32(val))
}

// Swap atomically stores new into x and returns the previous value.
func (x *Bool) Swap(new bool) (old bool) {
	return swapBool(x, b32(new)) != 0
}

// CompareAndSwap executes the compare-and-swap operation for the boolean value x.
func (x *Bool) CompareAndSwap(old, new bool) (swapped bool) {
	return casBool(x, b32(old), b32(new))
}

// The native functions of the typed API (gocpp/atomic.hpp): the C++ side is a template over the
// struct, which has one field v.

//gocpp:native gp::sync_atomic::ty_load
func loadInt32(x *Int32) int32

//gocpp:native gp::sync_atomic::ty_store
func storeInt32(x *Int32, val int32)

//gocpp:native gp::sync_atomic::ty_swap
func swapInt32(x *Int32, val int32) int32

//gocpp:native gp::sync_atomic::ty_cas
func casInt32(x *Int32, old, new int32) bool

//gocpp:native gp::sync_atomic::ty_add
func addInt32(x *Int32, delta int32) int32

//gocpp:native gp::sync_atomic::ty_and
func andInt32(x *Int32, mask int32) int32

//gocpp:native gp::sync_atomic::ty_or
func orInt32(x *Int32, mask int32) int32

//gocpp:native gp::sync_atomic::ty_load
func loadInt64(x *Int64) int64

//gocpp:native gp::sync_atomic::ty_store
func storeInt64(x *Int64, val int64)

//gocpp:native gp::sync_atomic::ty_swap
func swapInt64(x *Int64, val int64) int64

//gocpp:native gp::sync_atomic::ty_cas
func casInt64(x *Int64, old, new int64) bool

//gocpp:native gp::sync_atomic::ty_add
func addInt64(x *Int64, delta int64) int64

//gocpp:native gp::sync_atomic::ty_and
func andInt64(x *Int64, mask int64) int64

//gocpp:native gp::sync_atomic::ty_or
func orInt64(x *Int64, mask int64) int64

//gocpp:native gp::sync_atomic::ty_load
func loadUint32(x *Uint32) uint32

//gocpp:native gp::sync_atomic::ty_store
func storeUint32(x *Uint32, val uint32)

//gocpp:native gp::sync_atomic::ty_swap
func swapUint32(x *Uint32, val uint32) uint32

//gocpp:native gp::sync_atomic::ty_cas
func casUint32(x *Uint32, old, new uint32) bool

//gocpp:native gp::sync_atomic::ty_add
func addUint32(x *Uint32, delta uint32) uint32

//gocpp:native gp::sync_atomic::ty_and
func andUint32(x *Uint32, mask uint32) uint32

//gocpp:native gp::sync_atomic::ty_or
func orUint32(x *Uint32, mask uint32) uint32

//gocpp:native gp::sync_atomic::ty_load
func loadUint64(x *Uint64) uint64

//gocpp:native gp::sync_atomic::ty_store
func storeUint64(x *Uint64, val uint64)

//gocpp:native gp::sync_atomic::ty_swap
func swapUint64(x *Uint64, val uint64) uint64

//gocpp:native gp::sync_atomic::ty_cas
func casUint64(x *Uint64, old, new uint64) bool

//gocpp:native gp::sync_atomic::ty_add
func addUint64(x *Uint64, delta uint64) uint64

//gocpp:native gp::sync_atomic::ty_and
func andUint64(x *Uint64, mask uint64) uint64

//gocpp:native gp::sync_atomic::ty_or
func orUint64(x *Uint64, mask uint64) uint64

//gocpp:native gp::sync_atomic::ty_load
func loadUintptr(x *Uintptr) uintptr

//gocpp:native gp::sync_atomic::ty_store
func storeUintptr(x *Uintptr, val uintptr)

//gocpp:native gp::sync_atomic::ty_swap
func swapUintptr(x *Uintptr, val uintptr) uintptr

//gocpp:native gp::sync_atomic::ty_cas
func casUintptr(x *Uintptr, old, new uintptr) bool

//gocpp:native gp::sync_atomic::ty_add
func addUintptr(x *Uintptr, delta uintptr) uintptr

//gocpp:native gp::sync_atomic::ty_and
func andUintptr(x *Uintptr, mask uintptr) uintptr

//gocpp:native gp::sync_atomic::ty_or
func orUintptr(x *Uintptr, mask uintptr) uintptr

//gocpp:native gp::sync_atomic::ty_load
func loadBool(x *Bool) uint32

//gocpp:native gp::sync_atomic::ty_store
func storeBool(x *Bool, val uint32)

//gocpp:native gp::sync_atomic::ty_swap
func swapBool(x *Bool, val uint32) uint32

//gocpp:native gp::sync_atomic::ty_cas
func casBool(x *Bool, old, new uint32) bool
