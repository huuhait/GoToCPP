// Copyright 2026 The gocpp Authors. Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reflectlite is the part of package reflect that the standard library's own packages (errors,
// sort, context, ...) use, as an alias of the reflect subset (Phase 09 step 9.3, ADR-035). Go's
// reflectlite is a smaller copy of reflect that avoids an import cycle; gocpp's reflect has no
// dependency on those packages, so the types are the same types.
package reflectlite

import "reflect"

// Type is the representation of a Go type: reflect.Type.
type Type = reflect.Type

// Value is the reflection interface to a Go value: reflect.Value.
type Value = reflect.Value

// A Kind represents the specific kind of type that a Type represents: reflect.Kind.
type Kind = reflect.Kind

// A ValueError occurs when a Value method is invoked on a Value that does not support it.
type ValueError = reflect.ValueError

const (
	Invalid       = reflect.Invalid
	Bool          = reflect.Bool
	Int           = reflect.Int
	Int8          = reflect.Int8
	Int16         = reflect.Int16
	Int32         = reflect.Int32
	Int64         = reflect.Int64
	Uint          = reflect.Uint
	Uint8         = reflect.Uint8
	Uint16        = reflect.Uint16
	Uint32        = reflect.Uint32
	Uint64        = reflect.Uint64
	Uintptr       = reflect.Uintptr
	Float32       = reflect.Float32
	Float64       = reflect.Float64
	Complex64     = reflect.Complex64
	Complex128    = reflect.Complex128
	Array         = reflect.Array
	Chan          = reflect.Chan
	Func          = reflect.Func
	Interface     = reflect.Interface
	Map           = reflect.Map
	Pointer       = reflect.Pointer
	Ptr           = reflect.Ptr
	Slice         = reflect.Slice
	String        = reflect.String
	Struct        = reflect.Struct
	UnsafePointer = reflect.UnsafePointer
)

// TypeOf returns the reflection Type that represents the dynamic type of i. If i is a nil interface
// value, TypeOf returns nil.
func TypeOf(i any) Type {
	return reflect.TypeOf(i)
}

// ValueOf returns a new Value initialized to the concrete value stored in the interface i.
// ValueOf(nil) returns the zero Value.
func ValueOf(i any) Value {
	return reflect.ValueOf(i)
}

// Swapper returns a function that swaps the elements in the provided slice. Swapper panics if the
// provided interface is not a slice.
func Swapper(slice any) func(i, j int) {
	return reflect.Swapper(slice)
}
