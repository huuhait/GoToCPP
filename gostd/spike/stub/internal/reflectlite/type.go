// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reflectlite is job 16.1f-stubs' pure-Go stand-in for Go 1.25's
// internal/reflectlite.
//
// Go's package is Go source except for seven functions that the runtime
// provides through //go:linkname pushes (resolveNameOff, resolveTypeOff,
// unsafe_New, typedmemmove, ifaceE2I, chanlen, maplen) and one asm.s marker
// file. This package is Go's own internal/reflectlite with internal/abi's type
// descriptors restated in abi.go, local slice and string headers instead of
// internal/unsafeheader, and the seven runtime hooks declared here and in
// value.go as //go:linkname pulls of the same linker symbols. In the GC engine
// the runtime stub (gostd/spike/stub/runtime) provides those symbols as
// natives; with the real Go toolchain the real runtime's pushes satisfy them,
// so the tests next to this file compare against Go's real reflection.
//
// Only the part the closure uses is kept (gostd/spike/closure.json): TypeOf,
// ValueOf, Swapper, Type's Kind/Comparable/Elem/Implements/AssignableTo and
// Value's Kind/Len/IsNil/Elem/Set. The methods for arrays beyond Elem, maps,
// channels, structs, funcs and fields are left out; a new use is a compile
// error, not a silent wrong answer.
package reflectlite

import (
	"unsafe"
)

// Type is the representation of a Go type.
//
// Not all methods apply to all kinds of types. Restrictions, if any, are
// noted in the documentation for each method. Use the Kind method to find out
// the kind of a type before calling kind-specific methods. Calling a method
// inappropriate to the kind of type causes a run-time panic.
//
// Type values are comparable, such as with the == operator, so they can be
// used as map keys. Two Type values are equal if they represent identical
// types.
type Type interface {
	// Name returns the type's name within its package for a defined type.
	// For other (non-defined) types it returns the empty string.
	Name() string

	// PkgPath returns a defined type's package path.
	PkgPath() string

	// Size returns the number of bytes needed to store a value of the given
	// type; it is analogous to unsafe.Sizeof.
	Size() uintptr

	// Kind returns the specific kind of this type.
	Kind() Kind

	// Implements reports whether the type implements the interface type u.
	Implements(u Type) bool

	// AssignableTo reports whether a value of the type is assignable to type u.
	AssignableTo(u Type) bool

	// Comparable reports whether values of this type are comparable.
	Comparable() bool

	// String returns a string representation of the type.
	String() string

	// Elem returns a type's element type. It panics if the type's Kind is not
	// Ptr.
	Elem() Type

	common() *abiType
	uncommon() *abiUncommonType
}

// Ptr is the Kind of a pointer type.
const Ptr = Pointer

// rtype is the Type value of a Go type: a view of the compiler's descriptor.
type rtype struct {
	*abiType
}

// toRType wraps a descriptor as a Type.
func toRType(t *abiType) rtype {
	return rtype{t}
}

// toType converts a descriptor to a Type, mapping nil to nil.
func toType(t *abiType) Type {
	if t == nil {
		return nil
	}
	return toRType(t)
}

// TypeOf returns the reflection Type that represents the dynamic type of i.
// If i is a nil interface value, TypeOf returns nil.
func TypeOf(i any) Type {
	return toType(typeOf(i))
}

// typeOf returns the descriptor in i's empty interface, or nil. It is the
// stub's abi.TypeOf: the {type, data} layout of Go's empty interface (README
// §2.2 and §2.8).
func typeOf(i any) *abiType {
	e := (*emptyInterface)(unsafe.Pointer(&i))
	return e.Type
}

// emptyInterface is the layout of Go's empty interface value.
type emptyInterface struct {
	Type *abiType
	Data unsafe.Pointer
}

// resolveNameOff resolves a name offset from a base pointer. Implemented by
// the runtime (see the package comment and hooks.go).
//
//go:linkname resolveNameOff internal/reflectlite.resolveNameOff
//go:noescape
func resolveNameOff(ptrInModule unsafe.Pointer, off int32) unsafe.Pointer

// resolveTypeOff resolves a type offset from a base type. Implemented by the
// runtime.
//
//go:linkname resolveTypeOff internal/reflectlite.resolveTypeOff
//go:noescape
func resolveTypeOff(rtype unsafe.Pointer, off int32) unsafe.Pointer

func (t rtype) nameOff(off nameOff) abiName {
	return abiName{Bytes: (*byte)(resolveNameOff(unsafe.Pointer(t.abiType), int32(off)))}
}

func (t rtype) typeOff(off typeOff) *abiType {
	return (*abiType)(resolveTypeOff(unsafe.Pointer(t.abiType), int32(off)))
}

// pkgPath returns the package path stored in an encoded name, if any.
func pkgPath(n abiName) string {
	if n.Bytes == nil || *n.DataChecked(0, "name flag field")&(1<<2) == 0 {
		return ""
	}
	i, l := n.ReadVarint(1)
	off := 1 + i + l
	if n.HasTag() {
		i2, l2 := n.ReadVarint(off)
		off += i2 + l2
	}
	var nameOff int32
	// The field may not be aligned in memory, so copy it.
	copy((*[4]byte)(unsafe.Pointer(&nameOff))[:], (*[4]byte)(unsafe.Pointer(n.DataChecked(off, "name offset field")))[:])
	pkgPathName := abiName{(*byte)(resolveTypeOff(unsafe.Pointer(n.Bytes), nameOff))}
	return pkgPathName.Name()
}

func (t rtype) common() *abiType { return t.abiType }

func (t rtype) uncommon() *abiUncommonType { return t.Uncommon() }

func (t rtype) Size() uintptr { return t.abiType.Size() }

func (t rtype) Kind() Kind { return t.abiType.Kind() }

func (t rtype) Comparable() bool { return t.Equal != nil }

func (t rtype) String() string {
	s := t.nameOff(t.Str).Name()
	if t.TFlag&tflagExtraStar != 0 {
		return s[1:]
	}
	return s
}

func (t rtype) PkgPath() string {
	if t.TFlag&tflagNamed == 0 {
		return ""
	}
	ut := t.uncommon()
	if ut == nil {
		return ""
	}
	return t.nameOff(ut.PkgPath).Name()
}

func (t rtype) Name() string {
	if !t.HasName() {
		return ""
	}
	s := t.String()
	i := len(s) - 1
	sqBrackets := 0
	for i >= 0 && (s[i] != '.' || sqBrackets != 0) {
		switch s[i] {
		case ']':
			sqBrackets++
		case '[':
			sqBrackets--
		}
		i--
	}
	return s[i+1:]
}

func elem(t *abiType) *abiType {
	et := t.Elem()
	if et != nil {
		return et
	}
	panic("reflect: Elem of invalid type " + toRType(t).String())
}

func (t rtype) Elem() Type {
	return toType(elem(t.common()))
}

func (t rtype) Implements(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.Implements")
	}
	if u.Kind() != Interface {
		panic("reflect: non-interface type passed to Type.Implements")
	}
	return implements(u.common(), t.common())
}

func (t rtype) AssignableTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.AssignableTo")
	}
	uu := u.common()
	tt := t.common()
	return directlyAssignable(uu, tt) || implements(uu, tt)
}

// implements reports whether the type V implements the interface type T.
//
// The algorithm is Go's internal/reflectlite/type.go: a linear scan over the
// two sorted method lists.
func implements(T, V *abiType) bool {
	t := T.InterfaceType()
	if t == nil {
		return false
	}
	if len(t.Methods) == 0 {
		return true
	}
	rT := toRType(T)
	rV := toRType(V)
	if V.Kind() == Interface {
		v := (*abiInterfaceType)(unsafe.Pointer(V))
		i := 0
		for j := 0; j < len(v.Methods); j++ {
			tm := &t.Methods[i]
			tmName := rT.nameOff(tm.Name)
			vm := &v.Methods[j]
			vmName := rV.nameOff(vm.Name)
			if vmName.Name() == tmName.Name() && rV.typeOff(vm.Typ) == rT.typeOff(tm.Typ) {
				if !tmName.IsExported() {
					tmPkgPath := pkgPath(tmName)
					if tmPkgPath == "" {
						tmPkgPath = t.PkgPath.Name()
					}
					vmPkgPath := pkgPath(vmName)
					if vmPkgPath == "" {
						vmPkgPath = v.PkgPath.Name()
					}
					if tmPkgPath != vmPkgPath {
						continue
					}
				}
				if i++; i >= len(t.Methods) {
					return true
				}
			}
		}
		return false
	}

	v := V.Uncommon()
	if v == nil {
		return false
	}
	i := 0
	vmethods := v.Methods()
	for j := 0; j < int(v.Mcount); j++ {
		tm := &t.Methods[i]
		tmName := rT.nameOff(tm.Name)
		vm := vmethods[j]
		vmName := rV.nameOff(vm.Name)
		if vmName.Name() == tmName.Name() && rV.typeOff(vm.Mtyp) == rT.typeOff(tm.Typ) {
			if !tmName.IsExported() {
				tmPkgPath := pkgPath(tmName)
				if tmPkgPath == "" {
					tmPkgPath = t.PkgPath.Name()
				}
				vmPkgPath := pkgPath(vmName)
				if vmPkgPath == "" {
					vmPkgPath = rV.nameOff(v.PkgPath).Name()
				}
				if tmPkgPath != vmPkgPath {
					continue
				}
			}
			if i++; i >= len(t.Methods) {
				return true
			}
		}
	}
	return false
}

// directlyAssignable reports whether a value x of type V can be directly
// assigned (using memmove) to a value of type T.
// https://golang.org/doc/go-spec.html#Assignability
// Ignoring the interface rules (implemented elsewhere) and the ideal constant
// rules (no ideal constants at run time).
func directlyAssignable(T, V *abiType) bool {
	if T == V {
		return true
	}
	if T.HasName() && V.HasName() || T.Kind() != V.Kind() {
		return false
	}
	return haveIdenticalUnderlyingType(T, V, true)
}

func haveIdenticalType(T, V *abiType, cmpTags bool) bool {
	if cmpTags {
		return T == V
	}
	if toRType(T).Name() != toRType(V).Name() || T.Kind() != V.Kind() {
		return false
	}
	return haveIdenticalUnderlyingType(T, V, false)
}

func haveIdenticalUnderlyingType(T, V *abiType, cmpTags bool) bool {
	if T == V {
		return true
	}
	kind := T.Kind()
	if kind != V.Kind() {
		return false
	}
	// Non-composite types of equal kind have the same underlying type.
	if Bool <= kind && kind <= Complex128 || kind == String || kind == UnsafePointer {
		return true
	}
	switch kind {
	case Array:
		return T.Len() == V.Len() && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)
	case Chan:
		if V.ChanDir() == bothDir && haveIdenticalType(T.Elem(), V.Elem(), cmpTags) {
			return true
		}
		return V.ChanDir() == T.ChanDir() && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)
	case Func:
		t := (*abiFuncType)(unsafe.Pointer(T))
		v := (*abiFuncType)(unsafe.Pointer(V))
		if t.OutCount != v.OutCount || t.InCount != v.InCount {
			return false
		}
		for i := 0; i < t.NumIn(); i++ {
			if !haveIdenticalType(t.In(i), v.In(i), cmpTags) {
				return false
			}
		}
		for i := 0; i < t.NumOut(); i++ {
			if !haveIdenticalType(t.Out(i), v.Out(i), cmpTags) {
				return false
			}
		}
		return true
	case Interface:
		t := (*abiInterfaceType)(unsafe.Pointer(T))
		v := (*abiInterfaceType)(unsafe.Pointer(V))
		if len(t.Methods) == 0 && len(v.Methods) == 0 {
			return true
		}
		return false
	case Map:
		return haveIdenticalType(T.Key(), V.Key(), cmpTags) && haveIdenticalType(T.Elem(), V.Elem(), cmpTags)
	case Ptr, Slice:
		return haveIdenticalType(T.Elem(), V.Elem(), cmpTags)
	case Struct:
		t := (*abiStructType)(unsafe.Pointer(T))
		v := (*abiStructType)(unsafe.Pointer(V))
		if len(t.Fields) != len(v.Fields) {
			return false
		}
		if t.PkgPath.Name() != v.PkgPath.Name() {
			return false
		}
		for i := range t.Fields {
			tf := &t.Fields[i]
			vf := &v.Fields[i]
			if tf.Name.Name() != vf.Name.Name() {
				return false
			}
			if !haveIdenticalType(tf.Typ, vf.Typ, cmpTags) {
				return false
			}
			if cmpTags && tf.Name.Tag() != vf.Name.Tag() {
				return false
			}
			if tf.Offset != vf.Offset {
				return false
			}
			if tf.Embedded() != vf.Embedded() {
				return false
			}
		}
		return true
	}
	return false
}
