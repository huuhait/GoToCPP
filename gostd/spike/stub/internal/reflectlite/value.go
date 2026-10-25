// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectlite

import (
	"unsafe"
)

// Value is the reflection interface to a Go value.
//
// Not all methods apply to all kinds of values. Restrictions, if any, are
// noted in the documentation for each method. Use the Kind method to find out
// the kind of a value before calling kind-specific methods. Calling a method
// inappropriate to the kind of value causes a run-time panic.
//
// The zero Value represents no value. Its IsValid method returns false, its
// Kind method returns Invalid, and all other methods panic. Most functions
// and methods never return an invalid value.
type Value struct {
	// typ_ holds the type of the value represented by a Value.
	typ_ *abiType

	// Pointer-valued data or, if flagIndir is set, pointer to data.
	// Valid when either flagIndir is set or typ_.Pointers() is true.
	ptr unsafe.Pointer

	// flag holds metadata about the value. The lowest five bits give the
	// Kind of the value, mirroring typ_.Kind(). flagIndir means ptr points to
	// the data; flagAddr means the value is addressable.
	flag
}

type flag uintptr

const (
	flagKindWidth      = 5 // there are 27 kinds
	flagKindMask  flag = 1<<flagKindWidth - 1
	flagStickyRO  flag = 1 << 5
	flagEmbedRO   flag = 1 << 6
	flagIndir     flag = 1 << 7
	flagAddr      flag = 1 << 8
	flagRO        flag = flagStickyRO | flagEmbedRO
)

func (f flag) kind() Kind {
	return Kind(f & flagKindMask)
}

func (f flag) ro() flag {
	if f&flagRO != 0 {
		return flagStickyRO
	}
	return 0
}

func (v Value) typ() *abiType {
	return v.typ_
}

// pointer returns the underlying pointer represented by v. v.Kind() must be
// Pointer, Map, Chan, Func, or UnsafePointer.
func (v Value) pointer() unsafe.Pointer {
	if v.typ().Size() != unsafe.Sizeof(uintptr(0)) || !v.typ().Pointers() {
		panic("can't call pointer on a non-pointer Value")
	}
	if v.flag&flagIndir != 0 {
		return *(*unsafe.Pointer)(v.ptr)
	}
	return v.ptr
}

// packEface converts v to the empty interface.
func packEface(v Value) any {
	t := v.typ()
	var i any
	e := (*emptyInterface)(unsafe.Pointer(&i))
	switch {
	case t.IfaceIndir():
		if v.flag&flagIndir == 0 {
			panic("bad indir")
		}
		// The value and the interface are both indirect.
		ptr := v.ptr
		if v.flag&flagAddr != 0 {
			c := unsafe_New(t)
			typedmemmove(t, c, ptr)
			ptr = c
		}
		e.Data = ptr
	case v.flag&flagIndir != 0:
		// The value is indirect, but the interface is direct: load the data
		// word into the interface.
		e.Data = *(*unsafe.Pointer)(v.ptr)
	default:
		// Both direct.
		e.Data = v.ptr
	}
	// Fill in the type last, as Go does, so the collector never sees a
	// half-built interface.
	e.Type = t
	return i
}

// unpackEface converts the empty interface i to a Value.
func unpackEface(i any) Value {
	e := (*emptyInterface)(unsafe.Pointer(&i))
	t := e.Type
	if t == nil {
		return Value{}
	}
	f := flag(t.Kind())
	if t.IfaceIndir() {
		f |= flagIndir
	}
	return Value{t, e.Data, f}
}

// A ValueError occurs when a Value method is invoked on a Value that does not
// support it.
type ValueError struct {
	Method string
	Kind   Kind
}

func (e *ValueError) Error() string {
	if e.Kind == 0 {
		return "reflect: call of " + e.Method + " on zero Value"
	}
	return "reflect: call of " + e.Method + " on " + e.Kind.String() + " Value"
}

// methodName returns the name of the calling method. Go's version walks the
// stack with runtime.Caller; this stub only produces the panic text, so it
// returns a fixed name (a documented divergence of the message, not of the
// behavior).
func methodName() string {
	return "reflectlite.Value"
}

// mustBeAssignable panics if f records that the value is not assignable.
func (f flag) mustBeAssignable() {
	if f == 0 {
		panic(&ValueError{methodName(), Invalid})
	}
	if f&flagRO != 0 {
		panic("reflect: " + methodName() + " using value obtained using unexported field")
	}
	if f&flagAddr == 0 {
		panic("reflect: " + methodName() + " using unaddressable value")
	}
}

// mustBeExported panics if f records that the value was obtained using an
// unexported field.
func (f flag) mustBeExported() {
	if f == 0 {
		panic(&ValueError{methodName(), Invalid})
	}
	if f&flagRO != 0 {
		panic("reflect: " + methodName() + " using value obtained using unexported field")
	}
}

// Elem returns the value that the interface v contains or that the pointer v
// points to. It panics if v's Kind is not Interface or Pointer. It returns the
// zero Value if v is nil.
func (v Value) Elem() Value {
	k := v.kind()
	switch k {
	case Interface:
		var eface any
		if v.typ().NumMethod() == 0 {
			eface = *(*any)(v.ptr)
		} else {
			eface = (any)(*(*interface{ M() })(v.ptr))
		}
		x := unpackEface(eface)
		if x.flag != 0 {
			x.flag |= v.flag.ro()
		}
		return x
	case Pointer:
		ptr := v.ptr
		if v.flag&flagIndir != 0 {
			ptr = *(*unsafe.Pointer)(ptr)
		}
		// The returned value's address is v's value.
		if ptr == nil {
			return Value{}
		}
		tt := (*abiPtrType)(unsafe.Pointer(v.typ()))
		typ := tt.Elem
		fl := v.flag&flagRO | flagIndir | flagAddr
		fl |= flag(typ.Kind())
		return Value{typ, ptr, fl}
	}
	panic(&ValueError{"reflectlite.Value.Elem", v.kind()})
}

// valueInterface returns v as an interface value.
func valueInterface(v Value) any {
	if v.flag == 0 {
		panic(&ValueError{"reflectlite.Value.Interface", 0})
	}
	if v.kind() == Interface {
		// Special case: return the element inside the interface. The empty
		// interface has one layout, all interfaces with methods a second.
		if v.typ().NumMethod() == 0 {
			return *(*any)(v.ptr)
		}
		return *(*interface{ M() })(v.ptr)
	}
	return packEface(v)
}

// IsNil reports whether its argument v is nil. The argument must be a chan,
// func, interface, map, pointer, or slice value.
func (v Value) IsNil() bool {
	k := v.kind()
	switch k {
	case Chan, Func, Map, Pointer, UnsafePointer:
		ptr := v.ptr
		if v.flag&flagIndir != 0 {
			ptr = *(*unsafe.Pointer)(ptr)
		}
		return ptr == nil
	case Interface, Slice:
		// Both interface and slice are nil if their first word is 0.
		return *(*unsafe.Pointer)(v.ptr) == nil
	}
	panic(&ValueError{"reflectlite.Value.IsNil", v.kind()})
}

// IsValid reports whether v represents a value.
func (v Value) IsValid() bool {
	return v.flag != 0
}

// Kind returns v's Kind. If v is the zero Value, Kind returns Invalid.
func (v Value) Kind() Kind {
	return v.kind()
}

// sliceHeader is Go's slice header (internal/unsafeheader.Slice, restated so
// the stub has no import but unsafe).
type sliceHeader struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// stringHeader is Go's string header (internal/unsafeheader.String).
type stringHeader struct {
	Data unsafe.Pointer
	Len  int
}

// Implemented by the runtime (see the package comment and hooks.go).
//
//go:linkname chanlen internal/reflectlite.chanlen
//go:noescape
func chanlen(unsafe.Pointer) int

//go:linkname maplen internal/reflectlite.maplen
//go:noescape
func maplen(unsafe.Pointer) int

// Len returns v's length. It panics if v's Kind is not Array, Chan, Map, Slice
// or String.
func (v Value) Len() int {
	k := v.kind()
	switch k {
	case Array:
		tt := (*abiArrayType)(unsafe.Pointer(v.typ()))
		return int(tt.Len)
	case Chan:
		return chanlen(v.pointer())
	case Map:
		return maplen(v.pointer())
	case Slice:
		// Slice is bigger than a word; assume flagIndir.
		return (*sliceHeader)(v.ptr).Len
	case String:
		// String is bigger than a word; assume flagIndir.
		return (*stringHeader)(v.ptr).Len
	}
	panic(&ValueError{"reflect.Value.Len", v.kind()})
}

// Set assigns x to the value v. It panics if v is not addressable or was
// obtained from an unexported field. As in Go, x's value must be assignable to
// v's type.
func (v Value) Set(x Value) {
	v.mustBeAssignable()
	x.mustBeExported()
	var target unsafe.Pointer
	if v.kind() == Interface {
		target = v.ptr
	}
	x = x.assignTo("reflectlite.Set", v.typ(), target)
	if x.flag&flagIndir != 0 {
		typedmemmove(v.typ(), v.ptr, x.ptr)
	} else {
		*(*unsafe.Pointer)(v.ptr) = x.ptr
	}
}

// Type returns v's type.
func (v Value) Type() Type {
	f := v.flag
	if f == 0 {
		panic(&ValueError{"reflectlite.Value.Type", Invalid})
	}
	return toRType(v.typ())
}

// Implemented by the runtime (see the package comment and hooks.go).
//
//go:linkname unsafe_New internal/reflectlite.unsafe_New
//go:noescape
func unsafe_New(*abiType) unsafe.Pointer

// ValueOf returns a new Value initialized to the concrete value stored in the
// interface i. ValueOf(nil) returns the zero Value.
func ValueOf(i any) Value {
	if i == nil {
		return Value{}
	}
	return unpackEface(i)
}

// assignTo returns a value v that can be assigned directly to typ. It panics
// if v is not assignable to typ. For a conversion to an interface type, target
// is a suggested scratch space to use.
func (v Value) assignTo(context string, dst *abiType, target unsafe.Pointer) Value {
	switch {
	case directlyAssignable(dst, v.typ()):
		// Overwrite the type so that they match. Same memory layout, so no
		// harm done.
		fl := v.flag&(flagAddr|flagIndir) | v.flag.ro()
		fl |= flag(dst.Kind())
		return Value{dst, v.ptr, fl}

	case implements(dst, v.typ()):
		if target == nil {
			target = unsafe_New(dst)
		}
		if v.Kind() == Interface && v.IsNil() {
			// A nil ReadWriter passed to nil Reader is OK, but ifaceE2I
			// would panic. Return a nil dst explicitly.
			return Value{dst, nil, flag(Interface)}
		}
		x := valueInterface(v)
		if dst.NumMethod() == 0 {
			*(*any)(target) = x
		} else {
			ifaceE2I(dst, x, target)
		}
		return Value{dst, target, flagIndir | flag(Interface)}
	}

	panic(context + ": value of type " + toRType(v.typ()).String() + " is not assignable to type " + toRType(dst).String())
}

// arrayAt returns the i-th element of p, an array whose elements are eltSize
// bytes wide.
func arrayAt(p unsafe.Pointer, i int, eltSize uintptr, whySafe string) unsafe.Pointer {
	return add(p, uintptr(i)*eltSize, "i < len")
}

// Implemented by the runtime (see the package comment and hooks.go).
//
//go:linkname ifaceE2I internal/reflectlite.ifaceE2I
func ifaceE2I(t *abiType, src any, dst unsafe.Pointer)

// typedmemmove copies a value of type t to dst from src. Implemented by the
// runtime.
//
//go:linkname typedmemmove internal/reflectlite.typedmemmove
//go:noescape
func typedmemmove(t *abiType, dst, src unsafe.Pointer)

// add returns p+x.
func add(p unsafe.Pointer, x uintptr, whySafe string) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}
