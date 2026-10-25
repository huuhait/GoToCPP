// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The part of Go 1.25's internal/abi that internal/reflectlite needs, mirrored
// inside the stub so the stub has no import but "unsafe".
//
// Go's own internal/reflectlite imports internal/abi and reads the compiler's
// type descriptors directly. This stub cannot: internal/abi is a standard
// library package (and the engine translates it, Phase 16 README §2.8, not
// this stub), and Go's internal rule would forbid a test binary in this module
// from importing it. So the layouts are restated here, field for field, at the
// offsets internal/abi/type.go gives them:
//
//	abi.Type:            48 bytes on 64-bit (Size_ 0, PtrBytes 8, Hash 16,
//	                     TFlag 20, Align_ 21, FieldAlign_ 22, Kind_ 23,
//	                     Equal 24, GCData 32, Str 40, PtrToThis 44)
//	abi.PtrType/SliceType:   Type + Elem (*Type) at 48
//	abi.ArrayType:           Type + Elem 48, Slice 56, Len 64
//	abi.ChanType:            Type + Elem 48, Dir 56
//	abi.FuncType:            Type + InCount 48, OutCount 50, then []*Type
//	abi.StructType:          Type + PkgPath 48, Fields 56 ([]StructField)
//	abi.InterfaceType:       Type + PkgPath 48, Methods 56 ([]Imethod)
//	abi.SwissMapType (the    Type + Key 48, Elem 56, Group 64, Hasher 72,
//	  goexperiment.swissmap  GroupSize 80, SlotSize 88, ElemOff 96, Flags 104
//	  tag set of README §2.7)
//	abi.UncommonType:        PkgPath 0, Mcount 4, Xcount 6, Moff 8
//
// The GC engine's interface values are Go's {type, data} (README §2.2 and
// §2.8), and its type descriptors have abi.Type's layout (§2.8), so the code
// below works on the engine's descriptors as well as on the real Go runtime's.
// The tests next to this file check it against the real runtime's descriptors,
// which is the only way to validate the offsets from inside this module.
//
// If Go's internal/abi changes, this file changes with it (the plan's
// gostdcheck drift check of README §2.7 covers the same hazard).

package reflectlite

import (
	"unsafe"
)

// Kind mirrors abi.Kind. The values match Go 1.25's internal/abi (and
// reflect.Kind).
type Kind uint8

const (
	Invalid Kind = iota
	Bool
	Int
	Int8
	Int16
	Int32
	Int64
	Uint
	Uint8
	Uint16
	Uint32
	Uint64
	Uintptr
	Float32
	Float64
	Complex64
	Complex128
	Array
	Chan
	Func
	Interface
	Map
	Pointer
	Slice
	String
	Struct
	UnsafePointer
)

const (
	kindDirectIface Kind = 1 << 5
	kindMask        Kind = (1 << 5) - 1
)

// tflag mirrors abi.TFlag.
type tflag uint8

const (
	tflagUncommon  tflag = 1 << 0
	tflagExtraStar tflag = 1 << 1
	tflagNamed     tflag = 1 << 2
)

// nameOff and typeOff mirror abi.NameOff/abi.TypeOff: offsets resolved by the
// runtime's resolveNameOff/resolveTypeOff (see hooks.go).
type nameOff int32
type typeOff int32

// abiType mirrors abi.Type.
type abiType struct {
	Size_       uintptr
	PtrBytes    uintptr
	Hash        uint32
	TFlag       tflag
	Align_      uint8
	FieldAlign_ uint8
	Kind_       Kind
	Equal       func(unsafe.Pointer, unsafe.Pointer) bool
	GCData      *byte
	Str         nameOff
	PtrToThis   typeOff
}

// abiName mirrors abi.Name.
type abiName struct {
	Bytes *byte
}

// abiPtrType mirrors abi.PtrType.
type abiPtrType struct {
	abiType
	Elem *abiType
}

// abiSliceType mirrors abi.SliceType.
type abiSliceType struct {
	abiType
	Elem *abiType
}

// abiArrayType mirrors abi.ArrayType.
type abiArrayType struct {
	abiType
	Elem  *abiType
	Slice *abiType
	Len   uintptr
}

// abiChanType mirrors abi.ChanType.
type abiChanType struct {
	abiType
	Elem *abiType
	Dir  chanDir
}

// abiFuncType mirrors abi.FuncType.
type abiFuncType struct {
	abiType
	InCount  uint16
	OutCount uint16
}

// abiStructField mirrors abi.StructField.
type abiStructField struct {
	Name   abiName
	Typ    *abiType
	Offset uintptr
}

// abiStructType mirrors abi.StructType.
type abiStructType struct {
	abiType
	PkgPath abiName
	Fields  []abiStructField
}

// abiImethod mirrors abi.Imethod.
type abiImethod struct {
	Name nameOff
	Typ  typeOff
}

// abiInterfaceType mirrors abi.InterfaceType.
type abiInterfaceType struct {
	abiType
	PkgPath abiName
	Methods []abiImethod
}

// abiMethod mirrors abi.Method.
type abiMethod struct {
	Name nameOff
	Mtyp typeOff
	Ifn  int32 // textOff
	Tfn  int32 // textOff
}

// abiUncommonType mirrors abi.UncommonType.
type abiUncommonType struct {
	PkgPath nameOff
	Mcount  uint16
	Xcount  uint16
	Moff    uint32
	_       uint32
}

// abiSwissMapType mirrors abi.SwissMapType, the map type of the
// goexperiment.swissmap tag set the GC engine builds with (README §2.7).
type abiSwissMapType struct {
	abiType
	Key       *abiType
	Elem      *abiType
	Group     *abiType
	Hasher    func(unsafe.Pointer, uintptr) uintptr
	GroupSize uintptr
	SlotSize  uintptr
	ElemOff   uintptr
	Flags     uint32
}

// chanDir mirrors abi.ChanDir.
type chanDir int

const (
	recvDir    chanDir = 1 << iota         // <-chan
	sendDir                                // chan<-
	bothDir            = recvDir | sendDir // chan
	invalidDir chanDir = 0
)

// Kind returns the specific kind of this type.
func (t *abiType) Kind() Kind { return t.Kind_ & kindMask }

// HasName reports whether t has a name.
func (t *abiType) HasName() bool { return t.TFlag&tflagNamed != 0 }

// Pointers reports whether t contains pointers.
func (t *abiType) Pointers() bool { return t.PtrBytes != 0 }

// IfaceIndir reports whether t is stored indirectly in an interface value.
func (t *abiType) IfaceIndir() bool { return t.Kind_&kindDirectIface == 0 }

// Size returns the size of data with type t.
func (t *abiType) Size() uintptr { return t.Size_ }

// Elem returns the element type for t if t is an array, channel, map, pointer,
// or slice, otherwise nil.
func (t *abiType) Elem() *abiType {
	switch t.Kind() {
	case Array:
		return (*abiArrayType)(unsafe.Pointer(t)).Elem
	case Chan:
		return (*abiChanType)(unsafe.Pointer(t)).Elem
	case Map:
		return (*abiSwissMapType)(unsafe.Pointer(t)).Elem
	case Pointer:
		return (*abiPtrType)(unsafe.Pointer(t)).Elem
	case Slice:
		return (*abiSliceType)(unsafe.Pointer(t)).Elem
	}
	return nil
}

// Key returns the key type for t if t is a map, otherwise nil.
func (t *abiType) Key() *abiType {
	if t.Kind() == Map {
		return (*abiSwissMapType)(unsafe.Pointer(t)).Key
	}
	return nil
}

// Len returns the length of t if t is an array type, otherwise 0.
func (t *abiType) Len() int {
	if t.Kind() == Array {
		return int((*abiArrayType)(unsafe.Pointer(t)).Len)
	}
	return 0
}

// ChanDir returns the direction of t if t is a channel type.
func (t *abiType) ChanDir() chanDir {
	if t.Kind() == Chan {
		return (*abiChanType)(unsafe.Pointer(t)).Dir
	}
	return invalidDir
}

// InterfaceType returns t cast to an interface type, or nil.
func (t *abiType) InterfaceType() *abiInterfaceType {
	if t.Kind() != Interface {
		return nil
	}
	return (*abiInterfaceType)(unsafe.Pointer(t))
}

// Uncommon returns a pointer to t's "uncommon" data if there is any,
// otherwise nil.
func (t *abiType) Uncommon() *abiUncommonType {
	if t.TFlag&tflagUncommon == 0 {
		return nil
	}
	switch t.Kind() {
	case Struct:
		return &(*struct {
			abiStructType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	case Pointer:
		return &(*struct {
			abiPtrType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	case Func:
		return &(*struct {
			abiFuncType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	case Slice:
		return &(*struct {
			abiSliceType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	case Array:
		return &(*struct {
			abiArrayType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	case Chan:
		return &(*struct {
			abiChanType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	case Map:
		return &(*struct {
			abiSwissMapType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	case Interface:
		return &(*struct {
			abiInterfaceType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	default:
		return &(*struct {
			abiType
			u abiUncommonType
		})(unsafe.Pointer(t)).u
	}
}

// NumMethod returns the number of methods of t: the interface method count
// for an interface type, the exported method count otherwise.
func (t *abiType) NumMethod() int {
	if tt := t.InterfaceType(); tt != nil {
		return len(tt.Methods)
	}
	return len(t.ExportedMethods())
}

// ExportedMethods returns t's exported methods.
func (t *abiType) ExportedMethods() []abiMethod {
	ut := t.Uncommon()
	if ut == nil || ut.Xcount == 0 {
		return nil
	}
	return (*[1 << 16]abiMethod)(addChecked(unsafe.Pointer(ut), uintptr(ut.Moff), "t.xcount > 0"))[:ut.Xcount:ut.Xcount]
}

// Methods returns all of t's methods.
func (ut *abiUncommonType) Methods() []abiMethod {
	if ut.Mcount == 0 {
		return nil
	}
	return (*[1 << 16]abiMethod)(addChecked(unsafe.Pointer(ut), uintptr(ut.Moff), "t.mcount > 0"))[:ut.Mcount:ut.Mcount]
}

// InSlice returns the input types of a function type.
func (t *abiFuncType) InSlice() []*abiType {
	uadd := unsafe.Sizeof(*t)
	if t.TFlag&tflagUncommon != 0 {
		uadd += unsafe.Sizeof(abiUncommonType{})
	}
	if t.InCount == 0 {
		return nil
	}
	return (*[1 << 16]*abiType)(addChecked(unsafe.Pointer(t), uadd, "t.inCount > 0"))[:t.InCount:t.InCount]
}

// OutSlice returns the output types of a function type.
func (t *abiFuncType) OutSlice() []*abiType {
	outCount := uint16(t.NumOut())
	if outCount == 0 {
		return nil
	}
	uadd := unsafe.Sizeof(*t)
	if t.TFlag&tflagUncommon != 0 {
		uadd += unsafe.Sizeof(abiUncommonType{})
	}
	return (*[1 << 17]*abiType)(addChecked(unsafe.Pointer(t), uadd, "outCount > 0"))[t.InCount : t.InCount+outCount : t.InCount+outCount]
}

// NumIn returns the number of input parameters of a function type.
func (t *abiFuncType) NumIn() int { return int(t.InCount) }

// NumOut returns the number of output parameters of a function type.
func (t *abiFuncType) NumOut() int { return int(t.OutCount & (1<<15 - 1)) }

// In returns the i-th input type of a function type.
func (t *abiFuncType) In(i int) *abiType { return t.InSlice()[i] }

// Out returns the i-th output type of a function type.
func (t *abiFuncType) Out(i int) *abiType { return t.OutSlice()[i] }

// addChecked returns p+x.
func addChecked(p unsafe.Pointer, x uintptr, whySafe string) unsafe.Pointer {
	return unsafe.Pointer(uintptr(p) + x)
}

// IsExported reports whether n is exported.
func (n abiName) IsExported() bool { return *n.Bytes&(1<<0) != 0 }

// HasTag reports whether tag data follows n.
func (n abiName) HasTag() bool { return *n.Bytes&(1<<1) != 0 }

// IsEmbedded reports whether n is an embedded (anonymous) field.
func (n abiName) IsEmbedded() bool { return *n.Bytes&(1<<3) != 0 }

// DataChecked does pointer arithmetic on n.Bytes.
func (n abiName) DataChecked(off int, whySafe string) *byte {
	return (*byte)(addChecked(unsafe.Pointer(n.Bytes), uintptr(off), whySafe))
}

// ReadVarint parses a varint as encoding/binary writes it.
func (n abiName) ReadVarint(off int) (int, int) {
	v := 0
	for i := 0; ; i++ {
		x := *n.DataChecked(off+i, "read varint")
		v += int(x&0x7f) << (7 * i)
		if x&0x80 == 0 {
			return i + 1, v
		}
	}
}

// Name returns n's name.
func (n abiName) Name() string {
	if n.Bytes == nil {
		return ""
	}
	i, l := n.ReadVarint(1)
	return unsafe.String(n.DataChecked(1+i, "non-empty string"), l)
}

// Tag returns n's tag, or the empty string.
func (n abiName) Tag() string {
	if !n.HasTag() {
		return ""
	}
	i, l := n.ReadVarint(1)
	i2, l2 := n.ReadVarint(1 + i + l)
	return unsafe.String(n.DataChecked(1+i+l+i2, "non-empty string"), l2)
}

// Embedded reports whether f is an embedded field.
func (f *abiStructField) Embedded() bool { return f.Name.IsEmbedded() }
