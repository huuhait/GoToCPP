// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflect

import "unsafe"

// Value is the reflection interface to a Go value.
//
// A Value holds the type of the value (the address of its descriptor) and the storage of one object
// of that type: an unsafe.Pointer whose address is the object and whose owner keeps the storage alive
// (runtime/include/gocpp/reflect.hpp). The zero Value represents no value (IsValid reports false).
// Values are comparable with ==, but compare the storage: use Equal.
type Value struct {
	typ  uintptr
	ptr  unsafe.Pointer
	flag flag
}

type flag uintptr

const (
	flagKindWidth      = 5 // there are 27 kinds
	flagKindMask  flag = 1<<flagKindWidth - 1
	flagStickyRO  flag = 1 << 5
	flagEmbedRO   flag = 1 << 6
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

// A ValueError occurs when a Value method is invoked on a Value that does not support it. Such
// cases are documented in the description of each method.
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

// unknownMethod is what Go's valueMethodName gives when the check is not made by a method of Value
// (Append, AppendSlice, Copy).
const unknownMethod = "unknown method"

func (f flag) mustBe(method string, expected Kind) {
	if Kind(f&flagKindMask) != expected {
		panic(&ValueError{method, f.kind()})
	}
}

func (f flag) mustBeExported(method string) {
	if f == 0 {
		panic(&ValueError{method, Invalid})
	}
	if f&flagRO != 0 {
		panic("reflect: " + method + " using value obtained using unexported field")
	}
}

func (f flag) mustBeAssignable(method string) {
	if f == 0 {
		panic(&ValueError{method, Invalid})
	}
	if f&flagRO != 0 {
		panic("reflect: " + method + " using value obtained using unexported field")
	}
	if f&flagAddr == 0 {
		panic("reflect: " + method + " using unaddressable value")
	}
}

var (
	uint8Type = typeOf(uint8(0))
	bytesType = typeOf([]byte(nil))
)

func (v Value) kind() Kind {
	return Kind(v.flag & flagKindMask)
}

// ValueOf returns a new Value initialized to the concrete value stored in the interface i.
// ValueOf(nil) returns the zero Value.
func ValueOf(i any) Value {
	if i == nil {
		return Value{}
	}
	t, p := vUnpack(i)
	return Value{t, p, flag(tKind(t))}
}

// Kind returns v's Kind. If v is the zero Value (IsValid returns false), Kind returns Invalid.
func (v Value) Kind() Kind {
	return v.kind()
}

// IsValid reports whether v represents a value. It returns false if v is the zero Value.
func (v Value) IsValid() bool {
	return v.flag != 0
}

// Type returns v's type.
func (v Value) Type() Type {
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.Type", Invalid})
	}
	return toType(v.typ)
}

// CanAddr reports whether the value's address can be obtained with [Value.Addr].
func (v Value) CanAddr() bool {
	return v.flag&flagAddr != 0
}

// CanSet reports whether the value of v can be changed. A Value can be changed only if it is
// addressable and was not obtained by the use of unexported struct fields.
func (v Value) CanSet() bool {
	return v.flag&(flagAddr|flagRO) == flagAddr
}

// CanInterface reports whether [Value.Interface] can be used without panicking.
func (v Value) CanInterface() bool {
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.CanInterface", Invalid})
	}
	return v.flag&flagRO == 0
}

// Addr returns a pointer value representing the address of v. It panics if [Value.CanAddr] returns
// false.
func (v Value) Addr() Value {
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.Addr of unaddressable value")
	}
	// Preserve flagRO instead of using v.flag.ro() so that v.Addr().Elem() is equivalent to v.
	fl := v.flag & flagRO
	pt := tPtrTo(v.typ)
	return Value{pt, vPtrTo(pt, v.ptr), fl | flag(Pointer)}
}

// UnsafeAddr returns a pointer to v's data, as a uintptr. It panics if v is not addressable.
func (v Value) UnsafeAddr() uintptr {
	if v.typ == 0 {
		panic(&ValueError{"reflect.Value.UnsafeAddr", Invalid})
	}
	if v.flag&flagAddr == 0 {
		panic("reflect.Value.UnsafeAddr of unaddressable value")
	}
	return uintptr(v.ptr)
}

// Bool returns v's underlying value. It panics if v's kind is not [Bool].
func (v Value) Bool() bool {
	v.flag.mustBe("reflect.Value.Bool", Bool)
	return vBool(v.typ, v.ptr)
}

// Bytes returns v's underlying value. It panics if v's underlying value is not a slice of bytes or an
// addressable array of bytes.
func (v Value) Bytes() []byte {
	switch v.kind() {
	case Slice:
		if tKind(tElem(v.typ)) != int(Uint8) {
			panic("reflect.Value.Bytes of non-byte slice")
		}
		return vBytes(v.typ, v.ptr)
	case Array:
		if tKind(tElem(v.typ)) != int(Uint8) {
			panic("reflect.Value.Bytes of non-byte array")
		}
		if !v.CanAddr() {
			panic("reflect.Value.Bytes of unaddressable byte array")
		}
		n := tLen(v.typ)
		st := tSliceOf(tElem(v.typ))
		return vBytes(st, vReslice(v.typ, v.ptr, st, 0, n, n))
	}
	panic(&ValueError{"reflect.Value.Bytes", v.kind()})
}

// runes returns v's underlying value. It panics if v's underlying value is not a slice of runes.
func (v Value) runes() []rune {
	v.flag.mustBe("reflect.Value.Bytes", Slice)
	if tKind(tElem(v.typ)) != int(Int32) {
		panic("reflect.Value.Bytes of non-rune slice")
	}
	return nil
}

// CanInt reports whether Int can be used without panicking.
func (v Value) CanInt() bool {
	switch v.kind() {
	case Int, Int8, Int16, Int32, Int64:
		return true
	}
	return false
}

// Int returns v's underlying value, as an int64. It panics if v's Kind is not [Int], [Int8], [Int16],
// [Int32], or [Int64].
func (v Value) Int() int64 {
	switch v.kind() {
	case Int, Int8, Int16, Int32, Int64:
		return vInt(v.typ, v.ptr)
	}
	panic(&ValueError{"reflect.Value.Int", v.kind()})
}

// CanUint reports whether [Value.Uint] can be used without panicking.
func (v Value) CanUint() bool {
	switch v.kind() {
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return true
	}
	return false
}

// Uint returns v's underlying value, as a uint64. It panics if v's Kind is not [Uint], [Uintptr],
// [Uint8], [Uint16], [Uint32], or [Uint64].
func (v Value) Uint() uint64 {
	switch v.kind() {
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return vUint(v.typ, v.ptr)
	}
	panic(&ValueError{"reflect.Value.Uint", v.kind()})
}

// CanFloat reports whether [Value.Float] can be used without panicking.
func (v Value) CanFloat() bool {
	switch v.kind() {
	case Float32, Float64:
		return true
	}
	return false
}

// Float returns v's underlying value, as a float64. It panics if v's Kind is not [Float32] or
// [Float64].
func (v Value) Float() float64 {
	switch v.kind() {
	case Float32, Float64:
		return vFloat(v.typ, v.ptr)
	}
	panic(&ValueError{"reflect.Value.Float", v.kind()})
}

// CanComplex reports whether [Value.Complex] can be used without panicking.
func (v Value) CanComplex() bool {
	switch v.kind() {
	case Complex64, Complex128:
		return true
	}
	return false
}

// Complex returns v's underlying value, as a complex128. It panics if v's Kind is not [Complex64] or
// [Complex128].
func (v Value) Complex() complex128 {
	switch v.kind() {
	case Complex64, Complex128:
		return vComplex(v.typ, v.ptr)
	}
	panic(&ValueError{"reflect.Value.Complex", v.kind()})
}

// String returns the string v's underlying value, as a string. String is a special case because of
// Go's String method convention. Unlike the other getters, it does not panic if v's Kind is not
// [String]. Instead, it returns a string of the form "<T value>" where T is v's type. The fmt package
// treats Values specially. It does not call their String method implicitly but instead prints the
// concrete values they hold.
func (v Value) String() string {
	if v.kind() == String {
		return vString(v.typ, v.ptr)
	}
	return v.stringNonString()
}

func (v Value) stringNonString() string {
	if v.kind() == Invalid {
		return "<invalid Value>"
	}
	return "<" + v.Type().String() + " Value>"
}

// Elem returns the value that the interface v contains or that the pointer v points to. It panics if
// v's Kind is not [Interface] or [Pointer]. It returns the zero Value if v is nil.
func (v Value) Elem() Value {
	switch v.kind() {
	case Interface:
		t, p := vIfaceElem(v.typ, v.ptr)
		if t == 0 {
			return Value{}
		}
		x := Value{t, p, flag(tKind(t))}
		x.flag |= v.flag.ro()
		return x
	case Pointer:
		p := vElem(v.typ, v.ptr)
		// The returned value's address is v's value.
		if p == nil {
			return Value{}
		}
		et := tElem(v.typ)
		fl := v.flag&flagRO | flagAddr
		fl |= flag(tKind(et))
		return Value{et, p, fl}
	}
	panic(&ValueError{"reflect.Value.Elem", v.kind()})
}

// Indirect returns the value that v points to. If v is a nil pointer, Indirect returns a zero Value.
// If v is not a pointer, Indirect returns v.
func Indirect(v Value) Value {
	if v.Kind() != Pointer {
		return v
	}
	return v.Elem()
}

// NumField returns the number of fields in the struct v. It panics if v's Kind is not [Struct].
func (v Value) NumField() int {
	v.flag.mustBe("reflect.Value.NumField", Struct)
	return tNumField(v.typ)
}

// Field returns the i'th field of the struct v. It panics if v's Kind is not [Struct] or i is out of
// range.
func (v Value) Field(i int) Value {
	if v.kind() != Struct {
		panic(&ValueError{"reflect.Value.Field", v.kind()})
	}
	if uint(i) >= uint(tNumField(v.typ)) {
		panic("reflect: Field index out of range")
	}
	ft := fType(v.typ, i)

	// Inherit permission bits from v, but clear flagEmbedRO.
	fl := v.flag&(flagStickyRO|flagAddr) | flag(tKind(ft))
	// Using an unexported field forces flagRO.
	if fPkgPath(v.typ, i) != "" {
		if fEmbedded(v.typ, i) {
			fl |= flagEmbedRO
		} else {
			fl |= flagStickyRO
		}
	}
	return Value{ft, vOffset(v.ptr, fOffset(v.typ, i)), fl}
}

// FieldByIndex returns the nested field corresponding to index. It panics if evaluation requires
// stepping through a nil pointer or a field that is not a struct.
func (v Value) FieldByIndex(index []int) Value {
	if len(index) == 1 {
		return v.Field(index[0])
	}
	v.flag.mustBe("reflect.Value.FieldByIndex", Struct)
	for i, x := range index {
		if i > 0 {
			if v.Kind() == Pointer && tKind(tElem(v.typ)) == int(Struct) {
				if v.IsNil() {
					panic("reflect: indirection through nil pointer to embedded struct")
				}
				v = v.Elem()
			}
		}
		v = v.Field(x)
	}
	return v
}

// FieldByName returns the struct field with the given name. It returns the zero Value if no field was
// found. It panics if v's Kind is not [Struct].
func (v Value) FieldByName(name string) Value {
	v.flag.mustBe("reflect.Value.FieldByName", Struct)
	if f, ok := (&rtype{v.typ}).FieldByName(name); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

// FieldByNameFunc returns the struct field with a name that satisfies the match function. It panics
// if v's Kind is not [Struct]. It returns the zero Value if no field was found.
func (v Value) FieldByNameFunc(match func(string) bool) Value {
	if f, ok := (&rtype{v.typ}).FieldByNameFunc(match); ok {
		return v.FieldByIndex(f.Index)
	}
	return Value{}
}

// NumMethod returns the number of methods in the value's method set. For a non-interface type, it
// returns the number of exported methods. For an interface type, it returns the number of exported
// and unexported methods.
func (v Value) NumMethod() int {
	if v.typ == 0 {
		panic(&ValueError{"reflect.Value.NumMethod", Invalid})
	}
	return tNumMethod(v.typ)
}

// Index returns v's i'th element. It panics if v's Kind is not [Array], [Slice], or [String] or i is
// out of range.
func (v Value) Index(i int) Value {
	switch v.kind() {
	case Array:
		if uint(i) >= uint(tLen(v.typ)) {
			panic("reflect: array index out of range")
		}
		et := tElem(v.typ)
		fl := v.flag&flagAddr | v.flag.ro() | flag(tKind(et)) // bits same as overall array
		return Value{et, vIndex(v.typ, v.ptr, i), fl}

	case Slice:
		// Element flag same as Elem of Pointer. Addressable, possibly read-only.
		if uint(i) >= uint(vLen(v.typ, v.ptr)) {
			panic("reflect: slice index out of range")
		}
		et := tElem(v.typ)
		fl := flagAddr | v.flag.ro() | flag(tKind(et))
		return Value{et, vIndex(v.typ, v.ptr, i), fl}

	case String:
		s := vString(v.typ, v.ptr)
		if uint(i) >= uint(len(s)) {
			panic("reflect: string index out of range")
		}
		p := vNew(uint8Type)
		vSetUint(uint8Type, p, uint64(s[i]))
		fl := v.flag.ro() | flag(Uint8)
		return Value{uint8Type, p, fl}
	}
	panic(&ValueError{"reflect.Value.Index", v.kind()})
}

// Len returns v's length. It panics if v's Kind is not [Array], [Chan], [Map], [Slice], [String], or
// pointer to [Array].
func (v Value) Len() int {
	switch v.kind() {
	case Array, Chan, Map, Slice, String:
		return vLen(v.typ, v.ptr)
	case Pointer:
		if tKind(tElem(v.typ)) == int(Array) {
			return tLen(tElem(v.typ))
		}
		panic("reflect: call of reflect.Value.Len on ptr to non-array Value")
	}
	panic(&ValueError{"reflect.Value.Len", v.kind()})
}

// Cap returns v's capacity. It panics if v's Kind is not [Array], [Chan], [Slice] or pointer to
// [Array].
func (v Value) Cap() int {
	switch v.kind() {
	case Array, Chan, Slice:
		return vCap(v.typ, v.ptr)
	case Pointer:
		if tKind(tElem(v.typ)) == int(Array) {
			return tLen(tElem(v.typ))
		}
		panic("reflect: call of reflect.Value.Cap on ptr to non-array Value")
	}
	panic(&ValueError{"reflect.Value.Cap", v.kind()})
}

// IsNil reports whether its argument v is nil. The argument must be a chan, func, interface, map,
// pointer, or slice value; if it is not, IsNil panics.
func (v Value) IsNil() bool {
	switch v.kind() {
	case Chan, Func, Map, Pointer, UnsafePointer, Interface, Slice:
		return vIsNil(v.typ, v.ptr)
	}
	panic(&ValueError{"reflect.Value.IsNil", v.kind()})
}

// IsZero reports whether v is the zero value for its type. It panics if the argument is invalid.
func (v Value) IsZero() bool {
	switch v.kind() {
	case Bool:
		return !v.Bool()
	case Int, Int8, Int16, Int32, Int64:
		return v.Int() == 0
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return v.Uint() == 0
	case Float32, Float64:
		return v.Float() == 0
	case Complex64, Complex128:
		return v.Complex() == 0
	case Array:
		n := tLen(v.typ)
		for i := 0; i < n; i++ {
			if !v.Index(i).IsZero() {
				return false
			}
		}
		return true
	case Chan, Func, Interface, Map, Pointer, Slice, UnsafePointer:
		return v.IsNil()
	case String:
		return v.Len() == 0
	case Struct:
		n := v.NumField()
		for i := 0; i < n; i++ {
			if !v.Field(i).IsZero() && fName(v.typ, i) != "_" {
				return false
			}
		}
		return true
	default:
		panic(&ValueError{"reflect.Value.IsZero", v.Kind()})
	}
}

// SetZero sets v to be the zero value of v's type. It panics if [Value.CanSet] returns false.
func (v Value) SetZero() {
	v.flag.mustBeAssignable("reflect.Value.SetZero")
	z := vNew(v.typ)
	vAssign(v.typ, v.ptr, z)
}

// Pointer returns v's value as a uintptr. It panics if v's Kind is not [Chan], [Func], [Map],
// [Pointer], [Slice], [String], or [UnsafePointer].
func (v Value) Pointer() uintptr {
	switch v.kind() {
	case Pointer, Chan, Map, UnsafePointer, Func, Slice, String:
		return uintptr(v.unsafePointer())
	}
	panic(&ValueError{"reflect.Value.Pointer", v.kind()})
}

// UnsafePointer returns v's value as a [unsafe.Pointer]. It panics if v's Kind is not [Chan], [Func],
// [Map], [Pointer], [Slice], [String] or [UnsafePointer].
func (v Value) UnsafePointer() unsafe.Pointer {
	switch v.kind() {
	case Pointer, Chan, Map, UnsafePointer, Func, Slice, String:
		return v.unsafePointer()
	}
	panic(&ValueError{"reflect.Value.UnsafePointer", v.kind()})
}

func (v Value) unsafePointer() unsafe.Pointer {
	return vUnsafePtr(v.typ, v.ptr)
}

// Interface returns v's current value as an interface{}. It is equivalent to:
//
//	var i interface{} = (v's underlying value)
//
// It panics if the Value was obtained by accessing unexported struct fields.
func (v Value) Interface() (i any) {
	return valueInterface(v, true)
}

func valueInterface(v Value, safe bool) any {
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.Interface", Invalid})
	}
	if safe && v.flag&flagRO != 0 {
		// Do not allow access to unexported values via Interface, because they might be pointers that
		// should not be writable or methods or function that should not be callable.
		panic("reflect.Value.Interface: cannot return value obtained from unexported field or method")
	}
	return vInterface(v.typ, v.ptr)
}

// assignTo returns a value v that can be assigned directly to dst. It panics if v is not assignable
// to dst.
func (v Value) assignTo(context string, dst uintptr) Value {
	if v.typ == dst {
		return Value{dst, v.ptr, v.flag&flagAddr | v.flag.ro() | flag(tKind(dst))}
	}
	if !tAssignable(v.typ, dst) {
		panic(context + ": value of type " + tString(v.typ) + " is not assignable to type " + tString(dst))
	}
	if tKind(dst) == int(Interface) {
		if v.Kind() == Interface && v.IsNil() {
			// A nil ReadWriter passed to nil Reader is OK.
			return Value{dst, vNew(dst), flag(Interface)}
		}
		target := vNew(dst)
		vAssignIface(dst, target, v.typ, v.ptr)
		return Value{dst, target, flag(Interface)}
	}
	// Identical underlying types: the same memory layout, so overwrite the type.
	return Value{dst, v.ptr, v.flag&flagAddr | v.flag.ro() | flag(tKind(dst))}
}

// Set assigns x to the value v. It panics if [Value.CanSet] returns false. As in Go, x's value must
// be assignable to v's type and must not be derived from an unexported field.
func (v Value) Set(x Value) {
	v.flag.mustBeAssignable("reflect.Value.Set")
	x.flag.mustBeExported("reflect.Value.Set") // do not let unexported x leak
	x = x.assignTo("reflect.Set", v.typ)
	vAssign(v.typ, v.ptr, x.ptr)
}

// SetBool sets v's underlying value to x. It panics if v's Kind is not [Bool] or if [Value.CanSet]
// returns false.
func (v Value) SetBool(x bool) {
	v.flag.mustBeAssignable("reflect.Value.SetBool")
	v.flag.mustBe("reflect.Value.SetBool", Bool)
	vSetBool(v.typ, v.ptr, x)
}

// SetBytes sets v's underlying value. It panics if v's underlying value is not a slice of bytes.
func (v Value) SetBytes(x []byte) {
	v.flag.mustBeAssignable("reflect.Value.SetBytes")
	v.flag.mustBe("reflect.Value.SetBytes", Slice)
	if tKind(tElem(v.typ)) != int(Uint8) {
		panic("reflect.Value.SetBytes of non-byte slice")
	}
	tmp := ValueOf(x)
	vAssign(v.typ, v.ptr, tmp.ptr)
}

// SetComplex sets v's underlying value to x. It panics if v's Kind is not [Complex64] or
// [Complex128], or if [Value.CanSet] returns false.
func (v Value) SetComplex(x complex128) {
	v.flag.mustBeAssignable("reflect.Value.SetComplex")
	switch v.kind() {
	default:
		panic(&ValueError{"reflect.Value.SetComplex", v.kind()})
	case Complex64, Complex128:
		vSetComplex(v.typ, v.ptr, x)
	}
}

// SetFloat sets v's underlying value to x. It panics if v's Kind is not [Float32] or [Float64], or if
// [Value.CanSet] returns false.
func (v Value) SetFloat(x float64) {
	v.flag.mustBeAssignable("reflect.Value.SetFloat")
	switch v.kind() {
	default:
		panic(&ValueError{"reflect.Value.SetFloat", v.kind()})
	case Float32, Float64:
		vSetFloat(v.typ, v.ptr, x)
	}
}

// SetInt sets v's underlying value to x. It panics if v's Kind is not [Int], [Int8], [Int16],
// [Int32], or [Int64], or if [Value.CanSet] returns false.
func (v Value) SetInt(x int64) {
	v.flag.mustBeAssignable("reflect.Value.SetInt")
	switch v.kind() {
	default:
		panic(&ValueError{"reflect.Value.SetInt", v.kind()})
	case Int, Int8, Int16, Int32, Int64:
		vSetInt(v.typ, v.ptr, x)
	}
}

// SetUint sets v's underlying value to x. It panics if v's Kind is not [Uint], [Uintptr], [Uint8],
// [Uint16], [Uint32], or [Uint64], or if [Value.CanSet] returns false.
func (v Value) SetUint(x uint64) {
	v.flag.mustBeAssignable("reflect.Value.SetUint")
	switch v.kind() {
	default:
		panic(&ValueError{"reflect.Value.SetUint", v.kind()})
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		vSetUint(v.typ, v.ptr, x)
	}
}

// SetString sets v's underlying value to x. It panics if v's Kind is not [String] or if
// [Value.CanSet] returns false.
func (v Value) SetString(x string) {
	v.flag.mustBeAssignable("reflect.Value.SetString")
	v.flag.mustBe("reflect.Value.SetString", String)
	vSetString(v.typ, v.ptr, x)
}

// SetLen sets v's length to n. It panics if v's Kind is not [Slice], or if n is negative or greater
// than the capacity of the slice, or if [Value.CanSet] returns false.
func (v Value) SetLen(n int) {
	v.flag.mustBeAssignable("reflect.Value.SetLen")
	v.flag.mustBe("reflect.Value.SetLen", Slice)
	if uint(n) > uint(vCap(v.typ, v.ptr)) {
		panic("reflect: slice length out of range in SetLen")
	}
	vSetLen(v.typ, v.ptr, n)
}

// SetCap sets v's capacity to n. It panics if v's Kind is not [Slice], or if n is smaller than the
// length or greater than the capacity of the slice, or if [Value.CanSet] returns false.
func (v Value) SetCap(n int) {
	v.flag.mustBeAssignable("reflect.Value.SetCap")
	v.flag.mustBe("reflect.Value.SetCap", Slice)
	if n < vLen(v.typ, v.ptr) || n > vCap(v.typ, v.ptr) {
		panic("reflect: slice capacity out of range in SetCap")
	}
	s := vReslice(v.typ, v.ptr, v.typ, 0, vLen(v.typ, v.ptr), n)
	vAssign(v.typ, v.ptr, s)
}

// Grow increases the slice's capacity, if necessary, to guarantee space for another n elements. After
// Grow(n), at least n elements can be appended to the slice without another allocation. It panics if
// v's Kind is not a [Slice], or if n is negative or too large to allocate the memory, or if
// [Value.CanSet] returns false.
func (v Value) Grow(n int) {
	v.flag.mustBeAssignable("reflect.Value.Grow")
	v.flag.mustBe("reflect.Value.Grow", Slice)
	v.grow(n)
}

func (v Value) grow(n int) {
	l := vLen(v.typ, v.ptr)
	switch {
	case n < 0:
		panic("reflect.Value.Grow: negative len")
	case l+n < 0:
		panic("reflect.Value.Grow: slice overflow")
	case l+n > vCap(v.typ, v.ptr):
		x := vExtend(v.typ, v.ptr, n)
		vAssign(v.typ, v.ptr, x)
		vSetLen(v.typ, v.ptr, l)
	}
}

// extendSlice extends a slice by n elements: it returns a new slice value with the length
// incremented by the number of specified elements.
func (v Value) extendSlice(n int) Value {
	v.flag.mustBeExported(unknownMethod)
	v.flag.mustBe(unknownMethod, Slice)
	if n < 0 {
		panic("reflect.Value.Grow: negative len")
	}
	return Value{v.typ, vExtend(v.typ, v.ptr, n), flag(Slice)}
}

// Slice returns v[i:j]. It panics if v's Kind is not [Array], [Slice] or [String], or if v is an
// unaddressable array, or if the indexes are out of bounds.
func (v Value) Slice(i, j int) Value {
	var cap int
	var st uintptr
	switch kind := v.kind(); kind {
	default:
		panic(&ValueError{"reflect.Value.Slice", v.kind()})

	case Array:
		if v.flag&flagAddr == 0 {
			panic("reflect.Value.Slice: slice of unaddressable array")
		}
		cap = tLen(v.typ)
		st = tSliceOf(tElem(v.typ))

	case Slice:
		st = v.typ
		cap = vCap(v.typ, v.ptr)

	case String:
		n := vLen(v.typ, v.ptr)
		if i < 0 || j < i || j > n {
			panic("reflect.Value.Slice: string slice index out of bounds")
		}
		return Value{v.typ, vReslice(v.typ, v.ptr, v.typ, i, j, j), v.flag &^ flagAddr}
	}

	if i < 0 || j < i || j > cap {
		panic("reflect.Value.Slice: slice index out of bounds")
	}
	fl := v.flag.ro() | flag(Slice)
	return Value{st, vReslice(v.typ, v.ptr, st, i, j, cap), fl}
}

// Slice3 is the 3-index form of the slice operation: it returns v[i:j:k]. It panics if v's Kind is
// not [Array] or [Slice], or if v is an unaddressable array, or if the indexes are out of bounds.
func (v Value) Slice3(i, j, k int) Value {
	var cap int
	var st uintptr
	switch kind := v.kind(); kind {
	default:
		panic(&ValueError{"reflect.Value.Slice3", v.kind()})

	case Array:
		if v.flag&flagAddr == 0 {
			panic("reflect.Value.Slice3: slice of unaddressable array")
		}
		cap = tLen(v.typ)
		st = tSliceOf(tElem(v.typ))

	case Slice:
		st = v.typ
		cap = vCap(v.typ, v.ptr)
	}

	if i < 0 || j < i || k < j || k > cap {
		panic("reflect.Value.Slice3: slice index out of bounds")
	}
	fl := v.flag.ro() | flag(Slice)
	return Value{st, vReslice(v.typ, v.ptr, st, i, j, k), fl}
}

// OverflowComplex reports whether the complex128 x cannot be represented by v's type. It panics if
// v's Kind is not [Complex64] or [Complex128].
func (v Value) OverflowComplex(x complex128) bool {
	switch v.kind() {
	case Complex64:
		return overflowFloat32(real(x)) || overflowFloat32(imag(x))
	case Complex128:
		return false
	}
	panic(&ValueError{"reflect.Value.OverflowComplex", v.kind()})
}

// OverflowFloat reports whether the float64 x cannot be represented by v's type. It panics if v's
// Kind is not [Float32] or [Float64].
func (v Value) OverflowFloat(x float64) bool {
	switch v.kind() {
	case Float32:
		return overflowFloat32(x)
	case Float64:
		return false
	}
	panic(&ValueError{"reflect.Value.OverflowFloat", v.kind()})
}

// OverflowInt reports whether the int64 x cannot be represented by v's type. It panics if v's Kind is
// not [Int], [Int8], [Int16], [Int32], or [Int64].
func (v Value) OverflowInt(x int64) bool {
	switch v.kind() {
	case Int, Int8, Int16, Int32, Int64:
		bitSize := tSize(v.typ) * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic(&ValueError{"reflect.Value.OverflowInt", v.kind()})
}

// OverflowUint reports whether the uint64 x cannot be represented by v's type. It panics if v's Kind
// is not [Uint], [Uintptr], [Uint8], [Uint16], [Uint32], or [Uint64].
func (v Value) OverflowUint(x uint64) bool {
	switch v.kind() {
	case Uint, Uintptr, Uint8, Uint16, Uint32, Uint64:
		bitSize := tSize(v.typ) * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic(&ValueError{"reflect.Value.OverflowUint", v.kind()})
}

// Comparable reports whether the value v is comparable. If the type of v is an interface, this checks
// the dynamic type. If this reports true then v.Interface() == x will not panic for any x, nor will
// v.Equal(u) for any Value u.
func (v Value) Comparable() bool {
	switch v.Kind() {
	case Invalid:
		return false

	case Array:
		switch v.Type().Elem().Kind() {
		case Interface, Array, Struct:
			for i := 0; i < v.Type().Len(); i++ {
				if !v.Index(i).Comparable() {
					return false
				}
			}
			return true
		}
		return v.Type().Comparable()

	case Interface:
		return v.IsNil() || v.Elem().Comparable()

	case Struct:
		for i := 0; i < v.NumField(); i++ {
			if !v.Field(i).Comparable() {
				return false
			}
		}
		return true

	case Func, Map, Slice:
		return false
	}
	return true
}

// Equal reports true if v is equal to u. For two invalid values, Equal will report true. For an
// interface value, Equal will compare the value within the interface. Otherwise, If the values have
// different types, Equal will report false. Otherwise, for arrays and structs Equal will compare each
// element in order, and report false if it finds non-equal elements. During all comparisons, if
// values of the same type are compared, and the type is not comparable, Equal will panic.
func (v Value) Equal(u Value) bool {
	if v.Kind() == Interface {
		v = v.Elem()
	}
	if u.Kind() == Interface {
		u = u.Elem()
	}

	if !v.IsValid() || !u.IsValid() {
		return v.IsValid() == u.IsValid()
	}

	if v.Kind() != u.Kind() || v.typ != u.typ {
		return false
	}

	switch v.Kind() {
	case Bool:
		return v.Bool() == u.Bool()
	case Int, Int8, Int16, Int32, Int64:
		return v.Int() == u.Int()
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return v.Uint() == u.Uint()
	case Float32, Float64:
		return v.Float() == u.Float()
	case Complex64, Complex128:
		return v.Complex() == u.Complex()
	case String:
		return v.String() == u.String()
	case Chan, Pointer, UnsafePointer:
		return v.Pointer() == u.Pointer()
	case Array:
		// u and v have the same type so they have the same length
		vl := v.Len()
		if vl == 0 {
			// panic on [0]func()
			if !v.Type().Elem().Comparable() {
				break
			}
			return true
		}
		for i := 0; i < vl; i++ {
			if !v.Index(i).Equal(u.Index(i)) {
				return false
			}
		}
		return true
	case Struct:
		// u and v have the same type so they have the same fields
		nf := v.NumField()
		for i := 0; i < nf; i++ {
			if !v.Field(i).Equal(u.Field(i)) {
				return false
			}
		}
		return true
	case Func, Map, Slice:
		break
	}
	panic("reflect.Value.Equal: values of type " + v.Type().String() + " are not comparable")
}

// CanConvert reports whether the value v can be converted to type t. If v.CanConvert(t) returns true
// then v.Convert(t) will not panic.
func (v Value) CanConvert(t Type) bool {
	vt := v.Type()
	if !vt.ConvertibleTo(t) {
		return false
	}
	// Converting from slice to array or to pointer-to-array can panic depending on the value.
	switch {
	case vt.Kind() == Slice && t.Kind() == Array:
		if t.Len() > v.Len() {
			return false
		}
	case vt.Kind() == Slice && t.Kind() == Pointer && t.Elem().Kind() == Array:
		n := t.Elem().Len()
		if n > v.Len() {
			return false
		}
	}
	return true
}

// Convert returns the value v converted to type t. If the usual Go conversion rules do not allow
// conversion of the value v to type t, or if converting v to type t panics, Convert panics.
func (v Value) Convert(t Type) Value {
	dt := t.(*rtype).t
	if v.flag == 0 {
		panic(&ValueError{"reflect.Value.Convert", Invalid})
	}
	if !tConvertible(v.typ, dt) {
		panic("reflect.Value.Convert: value of type " + tString(v.typ) + " cannot be converted to type " + t.String())
	}
	if tKind(dt) == int(Interface) {
		return v.assignTo("reflect.Value.Convert", dt)
	}
	return Value{dt, vConvert(v.typ, v.ptr, dt), v.flag.ro() | flag(tKind(dt))}
}

// MapIndex returns the value associated with key in the map v. It panics if v's Kind is not [Map]. It
// returns the zero Value if key is not found in the map or if v represents a nil map. As in Go, the
// key's value must be assignable to the map's key type.
func (v Value) MapIndex(key Value) Value {
	v.flag.mustBe("reflect.Value.MapIndex", Map)
	key = key.assignTo("reflect.Value.MapIndex", tKey(v.typ))
	p := vMapIndex(v.typ, v.ptr, key.ptr)
	if p == nil {
		return Value{}
	}
	et := tElem(v.typ)
	// Copy result so future changes to the map won't change the underlying value.
	fl := (v.flag | key.flag).ro()
	fl |= flag(tKind(et))
	return Value{et, p, fl}
}

// MapKeys returns a slice containing all the keys present in the map, in unspecified order. It panics
// if v's Kind is not [Map]. It returns an empty slice if v represents a nil map.
func (v Value) MapKeys() []Value {
	v.flag.mustBe("reflect.Value.MapKeys", Map)
	kt := tKey(v.typ)
	fl := v.flag.ro() | flag(tKind(kt))
	h := vMapSnap(v.typ, v.ptr)
	n := vSnapLen(v.typ, h)
	a := make([]Value, n)
	for i := 0; i < n; i++ {
		a[i] = Value{kt, vSnapKey(v.typ, h, i), fl}
	}
	return a
}

// A MapIter is an iterator for ranging over a map. See [Value.MapRange].
type MapIter struct {
	m       Value
	h       unsafe.Pointer
	n       int
	i       int
	started bool
}

// Key returns the key of iter's current map entry.
func (iter *MapIter) Key() Value {
	if !iter.started {
		panic("MapIter.Key called before Next")
	}
	if iter.i >= iter.n {
		panic("MapIter.Key called on exhausted iterator")
	}
	t := iter.m.typ
	kt := tKey(t)
	fl := iter.m.flag.ro() | flag(tKind(kt))
	return Value{kt, vSnapKey(t, iter.h, iter.i), fl}
}

// Value returns the value of iter's current map entry.
func (iter *MapIter) Value() Value {
	if !iter.started {
		panic("MapIter.Value called before Next")
	}
	if iter.i >= iter.n {
		panic("MapIter.Value called on exhausted iterator")
	}
	t := iter.m.typ
	et := tElem(t)
	fl := iter.m.flag.ro() | flag(tKind(et))
	return Value{et, vSnapVal(t, iter.h, iter.i), fl}
}

// Next advances the map iterator and reports whether there is another entry. It returns false when
// iter is exhausted; subsequent calls to [MapIter.Key], [MapIter.Value], or [MapIter.Next] will
// panic.
func (iter *MapIter) Next() bool {
	if !iter.m.IsValid() {
		panic("MapIter.Next called on an iterator that does not have an associated map Value")
	}
	if !iter.started {
		iter.started = true
		iter.h = vMapSnap(iter.m.typ, iter.m.ptr)
		iter.n = vSnapLen(iter.m.typ, iter.h)
		iter.i = 0
	} else {
		if iter.i >= iter.n {
			panic("MapIter.Next called on exhausted iterator")
		}
		iter.i++
	}
	return iter.i < iter.n
}

// Reset modifies iter to iterate over v. It panics if v's Kind is not [Map] and v is not the zero
// Value. Reset(Value{}) causes iter to not to refer to any map, which may allow the previously
// iterated-over map to be garbage collected.
func (iter *MapIter) Reset(v Value) {
	if v.IsValid() {
		v.flag.mustBe("reflect.MapIter.Reset", Map)
	}
	iter.m = v
	iter.h = nil
	iter.n = 0
	iter.i = 0
	iter.started = false
}

// MapRange returns a range iterator for a map. It panics if v's Kind is not [Map].
//
// Call [MapIter.Next] to advance the iterator, and [MapIter.Key]/[MapIter.Value] to access each
// entry. The iterator ranges over a snapshot of the map taken at the first Next (gocpp: Go's
// iterator sees entries that are deleted or added during the iteration).
func (v Value) MapRange() *MapIter {
	// This is inlinable to take advantage of "function outlining".
	if v.kind() != Map {
		panic(&ValueError{"reflect.Value.MapRange", v.kind()})
	}
	return &MapIter{m: v}
}

// SetMapIndex sets the element associated with key in the map v to elem. It panics if v's Kind is not
// [Map]. If elem is the zero Value, SetMapIndex deletes the key from the map. Otherwise if v holds a
// nil map, SetMapIndex will panic. As in Go, key's elem must be assignable to the map's key type, and
// elem's value must be assignable to the map's elem type.
func (v Value) SetMapIndex(key, elem Value) {
	v.flag.mustBe("reflect.Value.SetMapIndex", Map)
	v.flag.mustBeExported("reflect.Value.SetMapIndex")
	key.flag.mustBeExported("reflect.Value.SetMapIndex")
	key = key.assignTo("reflect.Value.SetMapIndex", tKey(v.typ))
	if elem.typ == 0 {
		vMapDel(v.typ, v.ptr, key.ptr)
		return
	}
	elem.flag.mustBeExported("reflect.Value.SetMapIndex")
	elem = elem.assignTo("reflect.Value.SetMapIndex", tElem(v.typ))
	vMapSet(v.typ, v.ptr, key.ptr, elem.ptr)
}

// Clear clears the contents of a map or zeros the contents of a slice. It panics if v's Kind is not
// [Map] or [Slice].
func (v Value) Clear() {
	switch v.Kind() {
	case Slice:
		n := vLen(v.typ, v.ptr)
		for i := 0; i < n; i++ {
			e := v.Index(i)
			vAssign(e.typ, e.ptr, vNew(e.typ))
		}
	case Map:
		vMapClear(v.typ, v.ptr)
	default:
		panic(&ValueError{"reflect.Value.Clear", v.Kind()})
	}
}

// Append appends the values x to a slice s and returns the resulting slice. As in Go, each x's value
// must be assignable to the slice's element type.
func Append(s Value, x ...Value) Value {
	s.flag.mustBe(unknownMethod, Slice)
	n := s.Len()
	s = s.extendSlice(len(x))
	for i, v := range x {
		s.Index(n + i).Set(v)
	}
	return s
}

// AppendSlice appends a slice t to a slice s and returns the resulting slice. The slices s and t must
// have the same element type.
func AppendSlice(s, t Value) Value {
	s.flag.mustBe(unknownMethod, Slice)
	t.flag.mustBe(unknownMethod, Slice)
	typesMustMatch("reflect.AppendSlice", s.Type().Elem(), t.Type().Elem())
	ns := s.Len()
	nt := t.Len()
	s = s.extendSlice(nt)
	Copy(s.Slice(ns, ns+nt), t)
	return s
}

func typesMustMatch(what string, t1, t2 Type) {
	if t1 != t2 {
		panic(what + ": " + t1.String() + " != " + t2.String())
	}
}

// Copy copies the contents of src into dst until either dst has been filled or src has been
// exhausted. It returns the number of elements copied. Dst and src each must have kind [Slice] or
// [Array], and dst and src must have the same element type. It dst is an [Array], it panics if
// [Value.CanSet] returns false.
//
// As a special case, src can have kind [String] if the element type of dst is kind [Uint8].
func Copy(dst, src Value) int {
	dk := dst.kind()
	if dk != Array && dk != Slice {
		panic(&ValueError{"reflect.Copy", dk})
	}
	if dk == Array {
		dst.flag.mustBeAssignable(unknownMethod)
	}
	dst.flag.mustBeExported(unknownMethod)

	sk := src.kind()
	var stringCopy bool
	if sk != Array && sk != Slice {
		stringCopy = sk == String && tKind(tElem(dst.typ)) == int(Uint8)
		if !stringCopy {
			panic(&ValueError{"reflect.Copy", sk})
		}
	}
	src.flag.mustBeExported(unknownMethod)

	if !stringCopy {
		typesMustMatch("reflect.Copy", toType(tElem(dst.typ)), toType(tElem(src.typ)))
	}
	return vCopy(dst.typ, dst.ptr, src.typ, src.ptr)
}

// MakeSlice creates a new zero-initialized slice value for the specified slice type, length, and
// capacity.
func MakeSlice(typ Type, len, cap int) Value {
	if typ.Kind() != Slice {
		panic("reflect.MakeSlice of non-slice type")
	}
	if len < 0 {
		panic("reflect.MakeSlice: negative len")
	}
	if cap < 0 {
		panic("reflect.MakeSlice: negative cap")
	}
	if len > cap {
		panic("reflect.MakeSlice: len > cap")
	}
	t := typ.(*rtype).t
	return Value{t, vMkSlice(t, len, cap), flag(Slice)}
}

// MakeMap creates a new map with the specified type.
func MakeMap(typ Type) Value {
	return MakeMapWithSize(typ, 0)
}

// MakeMapWithSize creates a new map with the specified type and initial space for approximately n
// elements.
func MakeMapWithSize(typ Type, n int) Value {
	if typ.Kind() != Map {
		panic("reflect.MakeMapWithSize of non-map type")
	}
	t := typ.(*rtype).t
	return Value{t, vMapNew(t), flag(Map)}
}

// Zero returns a Value representing the zero value for the specified type. The result is different
// from the zero value of the Value struct, which represents no value at all. For example,
// Zero(TypeOf(42)) returns a Value with Kind [Int] and value 0. The returned value is neither
// addressable nor settable.
func Zero(typ Type) Value {
	if typ == nil {
		panic("reflect: Zero(nil)")
	}
	t := typ.(*rtype).t
	return Value{t, vNew(t), flag(tKind(t))}
}

// New returns a Value representing a pointer to a new zero value for the specified type. That is, the
// returned Value's Type is [PointerTo](typ).
func New(typ Type) Value {
	if typ == nil {
		panic("reflect: New(nil)")
	}
	t := typ.(*rtype).t
	pt := tPtrTo(t)
	return Value{pt, vPtrTo(pt, vNew(t)), flag(Pointer)}
}

// Swapper returns a function that swaps the elements in the provided slice.
//
// Swapper panics if the provided interface is not a slice.
func Swapper(slice any) func(i, j int) {
	v := ValueOf(slice)
	if v.Kind() != Slice {
		panic(&ValueError{Method: "Swapper", Kind: v.Kind()})
	}
	return swapper(slice)
}

// Seq-style iteration over values is not part of the subset.

var _ = bytesType
