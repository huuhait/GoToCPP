// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package atomic

// A Value provides an atomic load and store of a consistently typed value.
// The zero value for a Value returns nil from [Value.Load].
// Once [Value.Store] has been called, a Value must not be copied.
//
// A Value must not be copied after first use.
type Value struct {
	lk uint32 // a spin lock around v: gocpp's interface is two words and a count (gocpp/atomic.hpp)
	v  any
}

// Load returns the value set by the most recent Store.
// It returns nil if there has been no call to Store for this Value.
func (v *Value) Load() (val any) {
	valueLock(v)
	val = v.v
	valueUnlock(v)
	return val
}

// Store sets the value of the [Value] v to val.
// All calls to Store for a given Value must use values of the same concrete type.
// Store of an inconsistent type panics, as does Store(nil).
func (v *Value) Store(val any) {
	if val == nil {
		panic("sync/atomic: store of nil value into Value")
	}
	valueLock(v)
	if v.v != nil && !sameType(v.v, val) {
		valueUnlock(v)
		panic("sync/atomic: store of inconsistently typed value into Value")
	}
	v.v = val
	valueUnlock(v)
}

// Swap stores new into Value and returns the previous value. It returns nil if
// the Value is empty.
//
// All calls to Swap for a given Value must use values of the same concrete
// type. Swap of an inconsistent type panics, as does Swap(nil).
func (v *Value) Swap(new any) (old any) {
	if new == nil {
		panic("sync/atomic: swap of nil value into Value")
	}
	valueLock(v)
	if v.v != nil && !sameType(v.v, new) {
		valueUnlock(v)
		panic("sync/atomic: swap of inconsistently typed value into Value")
	}
	old = v.v
	v.v = new
	valueUnlock(v)
	return old
}

// CompareAndSwap executes the compare-and-swap operation for the [Value].
//
// All calls to CompareAndSwap for a given Value must use values of the same
// concrete type. CompareAndSwap of an inconsistent type panics, as does
// CompareAndSwap(old, nil).
func (v *Value) CompareAndSwap(old, new any) (swapped bool) {
	if new == nil {
		panic("sync/atomic: compare and swap of nil value into Value")
	}
	if old != nil && !sameType(old, new) {
		panic("sync/atomic: compare and swap of inconsistently typed values")
	}
	valueLock(v)
	if v.v == nil {
		if old != nil {
			valueUnlock(v)
			return false
		}
		v.v = new
		valueUnlock(v)
		return true
	}
	if !sameType(v.v, new) {
		valueUnlock(v)
		panic("sync/atomic: compare and swap of inconsistently typed value into Value")
	}
	// The comparison is the runtime's ==, which allows value types to be compared, something the
	// package functions don't offer. It panics when the type is not comparable ("comparing
	// uncomparable type"): valueEqual releases the lock first.
	same := valueEqual(v, old)
	if same {
		v.v = new
	}
	valueUnlock(v)
	return same
}

// valueEqual reports whether the value in v, whose lock is held, is equal to old. If the
// comparison panics, it releases the lock and goes on with the panic.
//
//gocpp:native gp::sync_atomic::value_equal
func valueEqual(v *Value, old any) bool

//gocpp:native gp::sync_atomic::value_lock
func valueLock(v *Value)

//gocpp:native gp::sync_atomic::value_unlock
func valueUnlock(v *Value)

// sameType reports whether a and b, which are not nil, have the same dynamic type.
//
//gocpp:native gp::sync_atomic::same_type
func sameType(a, b any) bool
