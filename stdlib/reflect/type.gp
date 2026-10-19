// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package reflect implements run-time reflection: a SUBSET of Go's package, written over gocpp's type
// descriptors (runtime/include/gocpp/type.hpp, ADR-035; divergence D6). Not included: MakeFunc,
// Value.Call, calling methods by name, Method values, StructOf, FuncOf, MapOf, ArrayOf, ChanOf, Select,
// and sending and receiving on channels. The type of a method (Type.Method(i).Type) is nil.
package reflect

import "unsafe"

// A Kind represents the specific kind of type that a [Type] represents.
// The zero Kind is not a valid kind.
type Kind uint

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

// Ptr is the old name for the [Pointer] kind.
const Ptr = Pointer

var kindNames = []string{
	Invalid:       "invalid",
	Bool:          "bool",
	Int:           "int",
	Int8:          "int8",
	Int16:         "int16",
	Int32:         "int32",
	Int64:         "int64",
	Uint:          "uint",
	Uint8:         "uint8",
	Uint16:        "uint16",
	Uint32:        "uint32",
	Uint64:        "uint64",
	Uintptr:       "uintptr",
	Float32:       "float32",
	Float64:       "float64",
	Complex64:     "complex64",
	Complex128:    "complex128",
	Array:         "array",
	Chan:          "chan",
	Func:          "func",
	Interface:     "interface",
	Map:           "map",
	Pointer:       "ptr",
	Slice:         "slice",
	String:        "string",
	Struct:        "struct",
	UnsafePointer: "unsafe.Pointer",
}

// String returns the name of k.
func (k Kind) String() string {
	if uint(k) < uint(len(kindNames)) {
		return kindNames[uint(k)]
	}
	return "kind" + itoa(int(k))
}

// itoa is strconv.Itoa (package strconv is not a real package yet).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	var u uint64
	if neg {
		u = uint64(-(n + 1)) + 1
	} else {
		u = uint64(n)
	}
	var b [24]byte
	i := len(b)
	for u > 0 {
		i--
		b[i] = byte('0' + u%10)
		u /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

// ChanDir represents a channel type's direction.
type ChanDir int

const (
	RecvDir ChanDir             = 1 << iota // <-chan
	SendDir                                 // chan<-
	BothDir = RecvDir | SendDir             // chan
)

func (d ChanDir) String() string {
	switch d {
	case SendDir:
		return "chan<-"
	case RecvDir:
		return "<-chan"
	case BothDir:
		return "chan"
	}
	return "ChanDir" + itoa(int(d))
}

// Type is the representation of a Go type.
//
// Not all methods apply to all kinds of types. Restrictions, if any, are noted in the documentation
// for each method. Use the Kind method to find out the kind of type before calling kind-specific
// methods. Calling a method inappropriate to the kind of type causes a run-time panic.
//
// Type values are comparable, such as with the == operator, so they can be used as map keys. Two Type
// values are equal if they represent identical types.
type Type interface {
	// Align returns the alignment in bytes of a value of this type when allocated in memory.
	Align() int

	// FieldAlign returns the alignment in bytes of a value of this type when used as a field in a struct.
	FieldAlign() int

	// Method returns the i'th method in the type's method set. It panics if i is not in the range
	// [0, NumMethod()). The Type and Func fields of the result are not set.
	Method(int) Method

	// MethodByName returns the method with that name in the type's method set and a boolean indicating
	// if the method was found.
	MethodByName(string) (Method, bool)

	// NumMethod returns the number of methods accessible using Method: the exported methods of a
	// non-interface type, all methods of an interface type.
	NumMethod() int

	// Name returns the type's name within its package for a defined type. For other (non-defined)
	// types it returns the empty string.
	Name() string

	// PkgPath returns a defined type's package path.
	PkgPath() string

	// Size returns the number of bytes needed to store a value of the given type (the size of gocpp's
	// C++ type, which is not always Go's: a pointer is 16 bytes, a string 32).
	Size() uintptr

	// String returns a string representation of the type.
	String() string

	// Kind returns the specific kind of this type.
	Kind() Kind

	// Implements reports whether the type implements the interface type u.
	Implements(u Type) bool

	// AssignableTo reports whether a value of the type is assignable to type u.
	AssignableTo(u Type) bool

	// ConvertibleTo reports whether a value of the type is convertible to type u.
	ConvertibleTo(u Type) bool

	// Comparable reports whether values of this type are comparable.
	Comparable() bool

	// Bits returns the size of the type in bits. It panics if the type's Kind is not one of the sized
	// or unsized Int, Uint, Float, or Complex kinds.
	Bits() int

	// ChanDir returns a channel type's direction.
	ChanDir() ChanDir

	// IsVariadic reports whether a function type's final input parameter is a "..." parameter.
	IsVariadic() bool

	// Elem returns a type's element type. It panics if the type's Kind is not Array, Chan, Map,
	// Pointer, or Slice.
	Elem() Type

	// Field returns a struct type's i'th field.
	Field(i int) StructField

	// FieldByIndex returns the nested field corresponding to the index sequence.
	FieldByIndex(index []int) StructField

	// FieldByName returns the struct field with the given name and a boolean indicating if the field
	// was found. If the returned field is promoted from an embedded struct, then Offset in the
	// returned StructField is the offset in the embedded struct.
	FieldByName(name string) (StructField, bool)

	// FieldByNameFunc returns the struct field with a name that satisfies the match function.
	FieldByNameFunc(match func(string) bool) (StructField, bool)

	// In returns the type of a function type's i'th input parameter.
	In(i int) Type

	// Key returns a map type's key type.
	Key() Type

	// Len returns an array type's length.
	Len() int

	// NumField returns a struct type's field count.
	NumField() int

	// NumIn returns a function type's input parameter count.
	NumIn() int

	// NumOut returns a function type's output parameter count.
	NumOut() int

	// Out returns the type of a function type's i'th output parameter.
	Out(i int) Type

	// OverflowComplex reports whether the complex128 x cannot be represented by type t.
	OverflowComplex(x complex128) bool

	// OverflowFloat reports whether the float64 x cannot be represented by type t.
	OverflowFloat(x float64) bool

	// OverflowInt reports whether the int64 x cannot be represented by type t.
	OverflowInt(x int64) bool

	// OverflowUint reports whether the uint64 x cannot be represented by type t.
	OverflowUint(x uint64) bool

	// CanSeq reports whether a [Value] with this type can be iterated over using [Value.Seq].
	CanSeq() bool

	// CanSeq2 reports whether a [Value] with this type can be iterated over using [Value.Seq2].
	CanSeq2() bool
}

// rtype is the implementation of Type: the address of a type descriptor. There is one rtype for each
// type (rtypeOf), so two Types of one type compare equal.
type rtype struct {
	t uintptr
}

// toType returns the Type of the descriptor t (nil for 0).
func toType(t uintptr) Type {
	if t == 0 {
		return nil
	}
	return (*rtype)(rtypeOf(t))
}

// TypeOf returns the reflection [Type] that represents the dynamic type of i.
// If i is a nil interface value, TypeOf returns nil.
func TypeOf(i any) Type {
	return toType(typeOf(i))
}

// TypeFor returns the [Type] that represents the type argument T.
func TypeFor[T any]() Type {
	var v T
	if t := TypeOf(v); t != nil {
		return t // optimize for T being a non-interface kind
	}
	return TypeOf((*T)(nil)).Elem() // only for an interface kind
}

// A Method represents a single method.
type Method struct {
	// Name is the method name.
	Name string

	// PkgPath is the package path that qualifies a lower case (unexported) method name. It is empty
	// for upper case (exported) method names.
	PkgPath string

	Type  Type  // method type: nil in gocpp
	Func  Value // func with receiver as first argument: the zero Value in gocpp
	Index int   // index for Type.Method
}

// IsExported reports whether the method is exported.
func (m Method) IsExported() bool {
	return m.PkgPath == ""
}

// A StructField describes a single field in a struct.
type StructField struct {
	// Name is the field name.
	Name string

	// PkgPath is the package path that qualifies a lower case (unexported) field name. It is empty for
	// upper case (exported) field names.
	PkgPath string

	Type      Type      // field type
	Tag       StructTag // field tag string
	Offset    uintptr   // offset within struct, in bytes
	Index     []int     // index sequence for Type.FieldByIndex
	Anonymous bool      // is an embedded field
}

// IsExported reports whether the field is exported.
func (f StructField) IsExported() bool {
	return f.PkgPath == ""
}

// A StructTag is the tag string in a struct field.
//
// By convention, tag strings are a concatenation of optionally space-separated key:"value" pairs.
// Each key is a non-empty string consisting of non-control characters other than space (U+0020 ' '),
// quote (U+0022 '"'), and colon (U+003A ':'). Each value is quoted using U+0022 '"' characters and
// Go string literal syntax.
type StructTag string

// Get returns the value associated with key in the tag string. If there is no such key in the tag,
// Get returns the empty string.
func (tag StructTag) Get(key string) string {
	v, _ := tag.Lookup(key)
	return v
}

// Lookup returns the value associated with key in the tag string. If the key is present in the tag
// the value (which may be empty) is returned. Otherwise the returned value will be the empty string.
// The ok return value reports whether the value was explicitly set in the tag string. If the tag does
// not have the conventional format, the value returned by Lookup is unspecified.
func (tag StructTag) Lookup(key string) (value string, ok bool) {
	for tag != "" {
		// Skip leading space.
		i := 0
		for i < len(tag) && tag[i] == ' ' {
			i++
		}
		tag = tag[i:]
		if tag == "" {
			break
		}

		// Scan to colon. A space, a quote or a control character is a syntax error.
		i = 0
		for i < len(tag) && tag[i] > ' ' && tag[i] != ':' && tag[i] != '"' && tag[i] != 0x7f {
			i++
		}
		if i == 0 || i+1 >= len(tag) || tag[i] != ':' || tag[i+1] != '"' {
			break
		}
		name := string(tag[:i])
		tag = tag[i+1:]

		// Scan quoted string to find value.
		i = 1
		for i < len(tag) && tag[i] != '"' {
			if tag[i] == '\\' {
				i++
			}
			i++
		}
		if i >= len(tag) {
			break
		}
		qvalue := string(tag[:i+1])
		tag = tag[i+1:]

		if key == name {
			value, ok := unquote(qvalue)
			if !ok {
				break
			}
			return value, true
		}
	}
	return "", false
}

func isHex(c byte) (int, bool) {
	switch {
	case '0' <= c && c <= '9':
		return int(c - '0'), true
	case 'a' <= c && c <= 'f':
		return int(c-'a') + 10, true
	case 'A' <= c && c <= 'F':
		return int(c-'A') + 10, true
	}
	return 0, false
}

// unquote is strconv.Unquote for a double-quoted Go string literal.
func unquote(s string) (string, bool) {
	n := len(s)
	if n < 2 || s[0] != '"' || s[n-1] != '"' {
		return "", false
	}
	s = s[1 : n-1]
	var out []byte
	for i := 0; i < len(s); {
		c := s[i]
		if c == '"' || c == '\n' {
			return "", false
		}
		if c != '\\' {
			out = append(out, c)
			i++
			continue
		}
		i++
		if i >= len(s) {
			return "", false
		}
		e := s[i]
		i++
		switch e {
		case 'a':
			out = append(out, '\a')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'v':
			out = append(out, '\v')
		case '\\':
			out = append(out, '\\')
		case '"':
			out = append(out, '"')
		case 'x', 'u', 'U':
			k := 2
			if e == 'u' {
				k = 4
			} else if e == 'U' {
				k = 8
			}
			if i+k > len(s) {
				return "", false
			}
			v := 0
			for j := 0; j < k; j++ {
				d, ok := isHex(s[i+j])
				if !ok {
					return "", false
				}
				v = v<<4 | d
			}
			i += k
			if e == 'x' {
				out = append(out, byte(v))
			} else {
				if v > 0x10FFFF || (v >= 0xD800 && v < 0xE000) {
					return "", false
				}
				out = append(out, string(rune(v))...)
			}
		case '0', '1', '2', '3', '4', '5', '6', '7':
			v := int(e - '0')
			if i+2 > len(s) {
				return "", false
			}
			for j := 0; j < 2; j++ {
				d := int(s[i+j]) - '0'
				if d < 0 || d > 7 {
					return "", false
				}
				v = v<<3 | d
			}
			i += 2
			if v > 255 {
				return "", false
			}
			out = append(out, byte(v))
		default:
			return "", false
		}
	}
	return string(out), true
}

func (t *rtype) Align() int      { return int(tAlign(t.t)) }
func (t *rtype) FieldAlign() int { return int(tAlign(t.t)) }
func (t *rtype) Size() uintptr   { return tSize(t.t) }
func (t *rtype) Name() string    { return tName(t.t) }
func (t *rtype) PkgPath() string { return tPkgPath(t.t) }
func (t *rtype) String() string  { return tString(t.t) }
func (t *rtype) Kind() Kind      { return Kind(tKind(t.t)) }

func (t *rtype) Bits() int {
	if t == nil {
		panic("reflect: Bits of nil Type")
	}
	k := t.Kind()
	if k < Int || k > Complex128 {
		panic("reflect: Bits of non-arithmetic Type " + t.String())
	}
	return int(tSize(t.t)) * 8
}

func (t *rtype) NumMethod() int {
	return tNumMethod(t.t)
}

func (t *rtype) Method(i int) (m Method) {
	if i < 0 || i >= tNumMethod(t.t) {
		panic("reflect: Method index out of range")
	}
	m.Name = mName(t.t, i)
	m.PkgPath = mPkgPath(t.t, i)
	m.Index = i
	return m
}

func (t *rtype) MethodByName(name string) (m Method, ok bool) {
	n := tNumMethod(t.t)
	for i := 0; i < n; i++ {
		if mName(t.t, i) == name {
			return t.Method(i), true
		}
	}
	return Method{}, false
}

func (t *rtype) ChanDir() ChanDir {
	if t.Kind() != Chan {
		panic("reflect: ChanDir of non-chan type " + t.String())
	}
	return ChanDir(tChanDir(t.t))
}

func (t *rtype) IsVariadic() bool {
	if t.Kind() != Func {
		panic("reflect: IsVariadic of non-func type " + t.String())
	}
	return tVariadic(t.t)
}

func (t *rtype) Elem() Type {
	e := tElem(t.t)
	if e == 0 {
		panic("reflect: Elem of invalid type " + t.String())
	}
	return toType(e)
}

func (t *rtype) Key() Type {
	if t.Kind() != Map {
		panic("reflect: Key of non-map type " + t.String())
	}
	return toType(tKey(t.t))
}

func (t *rtype) Len() int {
	if t.Kind() != Array {
		panic("reflect: Len of non-array type " + t.String())
	}
	return tLen(t.t)
}

func (t *rtype) NumField() int {
	if t.Kind() != Struct {
		panic("reflect: NumField of non-struct type " + t.String())
	}
	return tNumField(t.t)
}

func (t *rtype) NumIn() int {
	if t.Kind() != Func {
		panic("reflect: NumIn of non-func type " + t.String())
	}
	return tNumIn(t.t)
}

func (t *rtype) NumOut() int {
	if t.Kind() != Func {
		panic("reflect: NumOut of non-func type " + t.String())
	}
	return tNumOut(t.t)
}

func (t *rtype) In(i int) Type {
	if t.Kind() != Func {
		panic("reflect: In of non-func type " + t.String())
	}
	if uint(i) >= uint(tNumIn(t.t)) {
		panic("reflect: In index out of range")
	}
	return toType(tIn(t.t, i))
}

func (t *rtype) Out(i int) Type {
	if t.Kind() != Func {
		panic("reflect: Out of non-func type " + t.String())
	}
	if uint(i) >= uint(tNumOut(t.t)) {
		panic("reflect: Out index out of range")
	}
	return toType(tOut(t.t, i))
}

func (t *rtype) Comparable() bool {
	return tComparable(t.t)
}

func (t *rtype) Implements(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.Implements")
	}
	if u.Kind() != Interface {
		panic("reflect: non-interface type passed to Type.Implements")
	}
	return tImplements(t.t, u.(*rtype).t)
}

func (t *rtype) AssignableTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.AssignableTo")
	}
	return tAssignable(t.t, u.(*rtype).t)
}

func (t *rtype) ConvertibleTo(u Type) bool {
	if u == nil {
		panic("reflect: nil type passed to Type.ConvertibleTo")
	}
	return tConvertible(t.t, u.(*rtype).t)
}

func (t *rtype) Field(i int) (f StructField) {
	if t.Kind() != Struct {
		panic("reflect: Field of non-struct type " + t.String())
	}
	if i < 0 || i >= tNumField(t.t) {
		panic("reflect: Field index out of bounds")
	}
	f.Type = toType(fType(t.t, i))
	f.Name = fName(t.t, i)
	f.Anonymous = fEmbedded(t.t, i)
	f.PkgPath = fPkgPath(t.t, i)
	if tag := fTag(t.t, i); tag != "" {
		f.Tag = StructTag(tag)
	}
	f.Offset = fOffset(t.t, i)
	f.Index = []int{i}
	return
}

func (t *rtype) FieldByIndex(index []int) StructField {
	if t.Kind() != Struct {
		panic("reflect: FieldByIndex of non-struct type " + t.String())
	}
	var f StructField
	var tt Type = t
	for i, x := range index {
		if i > 0 {
			ft := f.Type
			if ft.Kind() == Pointer && ft.Elem().Kind() == Struct {
				ft = ft.Elem()
			}
			tt = ft
		}
		f = tt.Field(x)
	}
	return f
}

// A fieldScan represents an item on the fieldByNameFunc scan work list.
type fieldScan struct {
	typ   uintptr
	index []int
}

func (t *rtype) FieldByNameFunc(match func(string) bool) (result StructField, ok bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByNameFunc of non-struct type " + t.String())
	}
	// This uses the same condition that the Go language does: there must be a unique instance of the
	// match at a given depth level. If there are multiple instances of a match at the same depth, they
	// annihilate each other and inhibit any possible match at a lower level. The algorithm is breadth
	// first search, one depth level at a time.
	current := []fieldScan{}
	next := []fieldScan{{typ: t.t}}

	var nextCount map[uintptr]int
	visited := map[uintptr]bool{}

	for len(next) > 0 {
		current, next = next, current[:0]
		count := nextCount
		nextCount = nil

		for _, scan := range current {
			st := scan.typ
			if visited[st] {
				continue
			}
			visited[st] = true
			n := tNumField(st)
			for i := 0; i < n; i++ {
				fname := fName(st, i)
				var ntyp uintptr
				if fEmbedded(st, i) {
					ntyp = fType(st, i)
					if tKind(ntyp) == int(Pointer) {
						ntyp = tElem(ntyp)
					}
				}

				if match(fname) {
					if count[st] > 1 || ok {
						return StructField{}, false
					}
					result = (&rtype{st}).Field(i)
					result.Index = nil
					result.Index = append(result.Index, scan.index...)
					result.Index = append(result.Index, i)
					ok = true
					continue
				}

				if ok || ntyp == 0 || tKind(ntyp) != int(Struct) {
					continue
				}
				if nextCount[ntyp] > 0 {
					nextCount[ntyp] = 2 // exact multiple doesn't matter
					continue
				}
				if nextCount == nil {
					nextCount = map[uintptr]int{}
				}
				nextCount[ntyp] = 1
				if count[st] > 1 {
					nextCount[ntyp] = 2
				}
				var index []int
				index = append(index, scan.index...)
				index = append(index, i)
				next = append(next, fieldScan{ntyp, index})
			}
		}
		if ok {
			break
		}
	}
	return
}

func (t *rtype) FieldByName(name string) (f StructField, present bool) {
	if t.Kind() != Struct {
		panic("reflect: FieldByName of non-struct type " + t.String())
	}
	hasEmbeds := false
	if name != "" {
		n := tNumField(t.t)
		for i := 0; i < n; i++ {
			if fName(t.t, i) == name {
				return t.Field(i), true
			}
			if fEmbedded(t.t, i) {
				hasEmbeds = true
			}
		}
	}
	if !hasEmbeds {
		return
	}
	return t.FieldByNameFunc(func(s string) bool { return s == name })
}

const (
	maxFloat32 = 0x1p127 * (1 + (1 - 0x1p-23))
	maxFloat64 = 0x1p1023 * (1 + (1 - 0x1p-52))
)

func overflowFloat32(x float64) bool {
	if x < 0 {
		x = -x
	}
	return maxFloat32 < x && x <= maxFloat64
}

func (t *rtype) OverflowComplex(x complex128) bool {
	k := t.Kind()
	switch k {
	case Complex64:
		return overflowFloat32(real(x)) || overflowFloat32(imag(x))
	case Complex128:
		return false
	}
	panic("reflect: OverflowComplex of non-complex type " + t.String())
}

func (t *rtype) OverflowFloat(x float64) bool {
	k := t.Kind()
	switch k {
	case Float32:
		return overflowFloat32(x)
	case Float64:
		return false
	}
	panic("reflect: OverflowFloat of non-float type " + t.String())
}

func (t *rtype) OverflowInt(x int64) bool {
	k := t.Kind()
	switch k {
	case Int, Int8, Int16, Int32, Int64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowInt of non-int type " + t.String())
}

func (t *rtype) OverflowUint(x uint64) bool {
	k := t.Kind()
	switch k {
	case Uint, Uintptr, Uint8, Uint16, Uint32, Uint64:
		bitSize := t.Size() * 8
		trunc := (x << (64 - bitSize)) >> (64 - bitSize)
		return x != trunc
	}
	panic("reflect: OverflowUint of non-uint type " + t.String())
}

func (t *rtype) CanSeq() bool {
	switch t.Kind() {
	case Int8, Int16, Int32, Int64, Int, Uint8, Uint16, Uint32, Uint64, Uint, Uintptr, Array, Slice, Chan, String, Map:
		return true
	case Func:
		return canRangeFunc(t, 1)
	case Pointer:
		return t.Elem().Kind() == Array
	}
	return false
}

func canRangeFunc(t Type, yieldIn int) bool {
	if t.NumIn() != 1 || t.NumOut() != 0 {
		return false
	}
	y := t.In(0)
	if y.Kind() != Func || y.NumIn() != yieldIn || y.NumOut() != 1 {
		return false
	}
	return y.Out(0).Kind() == Bool
}

func (t *rtype) CanSeq2() bool {
	switch t.Kind() {
	case Array, Slice, String, Map:
		return true
	case Func:
		return canRangeFunc(t, 2)
	case Pointer:
		return t.Elem().Kind() == Array
	}
	return false
}

// PtrTo is the old spelling of [PointerTo].
func PtrTo(t Type) Type { return PointerTo(t) }

// PointerTo returns the pointer type with element t.
func PointerTo(t Type) Type {
	return toType(tPtrTo(t.(*rtype).t))
}

// SliceOf returns the slice type with element type t.
func SliceOf(t Type) Type {
	return toType(tSliceOf(t.(*rtype).t))
}

var _ unsafe.Pointer
