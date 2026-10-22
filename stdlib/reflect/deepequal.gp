// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Deep equality test via reflection

package reflect

// During deepValueEqual, must keep track of checks that are in progress. The comparison algorithm
// assumes that all checks in progress are true when it reencounters them. Visited comparisons are
// stored in a map indexed by visit.
type visit struct {
	a1  uintptr
	a2  uintptr
	typ uintptr
}

// deepFrame holds the children of one value that deepValueEqual has not compared yet: the
// elements of an array or slice, the fields of a struct, the pointee of a pointer or the element of
// an interface (n is 1), or the entries of a map (iter walks v1).
type deepFrame struct {
	v1, v2 Value
	i, n   int
	iter   *MapIter
}

// The results of deepStep and deepFrame.next.
const (
	deepDifferent = iota // the values are not deeply equal
	deepSame             // nothing is left to compare for this value
	deepMore             // there are children to compare (deepStep) or a pair to compare (next)
)

// next returns the next pair of children of f. It reports deepSame when f has no more children and
// deepDifferent when a map entry of v1 has no deeply comparable entry in v2.
func (f *deepFrame) next() (a, b Value, result int) {
	if f.iter != nil {
		if !f.iter.Next() {
			return a, b, deepSame
		}
		a = f.iter.Value()
		b = f.v2.MapIndex(f.iter.Key())
		if !a.IsValid() || !b.IsValid() {
			return a, b, deepDifferent
		}
		return a, b, deepMore
	}
	if f.i >= f.n {
		return a, b, deepSame
	}
	i := f.i
	f.i++
	switch f.v1.Kind() {
	case Array, Slice:
		return f.v1.Index(i), f.v2.Index(i), deepMore
	case Struct:
		return f.v1.Field(i), f.v2.Field(i), deepMore
	default:
		// Interface or Pointer: the one child.
		return f.v1.Elem(), f.v2.Elem(), deepMore
	}
}

// deepStep compares what it can of v1 and v2 without looking at their children. It returns
// deepMore and the frame of the children that still have to be compared (ordinary comparison of
// the pair is not decided yet), deepSame when the pair is equal, or deepDifferent.
func deepStep(v1, v2 Value, visited map[visit]bool) (frame deepFrame, result int) {
	if !v1.IsValid() || !v2.IsValid() {
		return frame, deepResult(v1.IsValid() == v2.IsValid())
	}
	if v1.typ != v2.typ {
		return frame, deepDifferent
	}

	// We want to avoid putting more in the visited map than we need to. For any possible reference
	// cycle that might be encountered, hard(v1, v2) needs to return true for at least one of the
	// types in the cycle.
	hard := func(v1, v2 Value) bool {
		switch v1.Kind() {
		case Pointer, Map, Slice, Interface:
			// Nil pointers cannot be cyclic. Avoid putting them in the visited map.
			return !v1.IsNil() && !v2.IsNil()
		}
		return false
	}

	if hard(v1, v2) {
		// For a Pointer or Map value, the address of the object; for Slice or Interface the address
		// of the storage that holds the header.
		var addr1, addr2 uintptr
		switch v1.Kind() {
		case Pointer, Map:
			addr1 = uintptr(v1.unsafePointer())
			addr2 = uintptr(v2.unsafePointer())
		default:
			addr1 = uintptr(v1.ptr)
			addr2 = uintptr(v2.ptr)
		}
		if addr1 > addr2 {
			// Canonicalize order to reduce number of entries in visited.
			addr1, addr2 = addr2, addr1
		}

		// Short circuit if references are already seen.
		v := visit{addr1, addr2, v1.typ}
		if visited[v] {
			return frame, deepSame
		}

		// Remember for later.
		visited[v] = true
	}

	switch v1.Kind() {
	case Array:
		return deepChildren(v1, v2, v1.Len())
	case Slice:
		if v1.IsNil() != v2.IsNil() {
			return frame, deepDifferent
		}
		if v1.Len() != v2.Len() {
			return frame, deepDifferent
		}
		if v1.unsafePointer() == v2.unsafePointer() {
			return frame, deepSame
		}
		// Special case for []byte, which is common.
		if tKind(tElem(v1.typ)) == int(Uint8) {
			a, b := vBytes(v1.typ, v1.ptr), vBytes(v2.typ, v2.ptr)
			return frame, deepResult(string(a) == string(b))
		}
		return deepChildren(v1, v2, v1.Len())
	case Interface:
		if v1.IsNil() || v2.IsNil() {
			return frame, deepResult(v1.IsNil() == v2.IsNil())
		}
		return deepChildren(v1, v2, 1)
	case Pointer:
		if v1.unsafePointer() == v2.unsafePointer() {
			return frame, deepSame
		}
		return deepChildren(v1, v2, 1)
	case Struct:
		if v1.typ == valueType {
			return frame, deepResult(sameValue(v1, v2))
		}
		return deepChildren(v1, v2, v1.NumField())
	case Map:
		if v1.IsNil() != v2.IsNil() {
			return frame, deepDifferent
		}
		if v1.Len() != v2.Len() {
			return frame, deepDifferent
		}
		if v1.unsafePointer() == v2.unsafePointer() {
			return frame, deepSame
		}
		return deepFrame{v1: v1, v2: v2, iter: v1.MapRange()}, deepMore
	case Func:
		// Can't do better than this: only two nil functions are equal.
		return frame, deepResult(v1.IsNil() && v2.IsNil())
	case Int, Int8, Int16, Int32, Int64:
		return frame, deepResult(v1.Int() == v2.Int())
	case Uint, Uint8, Uint16, Uint32, Uint64, Uintptr:
		return frame, deepResult(v1.Uint() == v2.Uint())
	case String:
		return frame, deepResult(v1.String() == v2.String())
	case Bool:
		return frame, deepResult(v1.Bool() == v2.Bool())
	case Float32, Float64:
		return frame, deepResult(v1.Float() == v2.Float())
	case Complex64, Complex128:
		return frame, deepResult(v1.Complex() == v2.Complex())
	default:
		// Normal equality suffices
		return frame, deepResult(vEqual(v1.typ, v1.ptr, v2.ptr))
	}
}

// valueType is the type of Value, whose fields are the type, the storage and the flags.
var valueType = typeOf(Value{})

// sameValue compares two Values (a struct that DeepEqual finds in an interface) as Go does, field
// by field. Go's pointer field is the pointer itself for the kinds that are a pointer (Pointer,
// Map, Chan, Func, UnsafePointer), so two Values of the same pointer are equal; here the field
// is the address of a box that holds it, which is a new one for every ValueOf, so these kinds
// compare the pointers they hold. For any other kind the storage is compared: it is the same for
// a copy of a Value, and for the Elem or the Field of the same object, and a new box for every
// ValueOf of a number, a string, a struct or a slice (Go shares the storage of a constant and of a
// number below 256 between the boxes, which gocpp does not: ADR-035).
func sameValue(v1, v2 Value) bool {
	a := valueInterface(v1, false).(Value)
	b := valueInterface(v2, false).(Value)
	if a.typ != b.typ || a.flag != b.flag {
		return false
	}
	if a.typ == 0 {
		return true
	}
	switch a.kind() {
	case Pointer, Map, Chan, Func, UnsafePointer:
		p := a.unsafePointer()
		q := b.unsafePointer()
		if p != nil && q != nil {
			return p == q
		}
		return vIsNil(a.typ, a.ptr) && vIsNil(b.typ, b.ptr)
	}
	return a.ptr == b.ptr
}

func deepResult(equal bool) int {
	if equal {
		return deepSame
	}
	return deepDifferent
}

// deepChildren is the frame of the n children of v1 and v2; an empty frame is already compared.
func deepChildren(v1, v2 Value, n int) (frame deepFrame, result int) {
	if n == 0 {
		return frame, deepSame
	}
	return deepFrame{v1: v1, v2: v2, n: n}, deepMore
}

// Tests for deep equality using reflected types. The map argument tracks comparisons that have
// already been seen, which allows short circuiting on recursive types.
//
// Go's function calls itself for every level of a list, a tree or a nested slice, and Go's stack
// grows to 1 GB. The 8 MiB stack of gocpp (ADR-001) would end in a stack overflow at about 6000
// levels, so the walk keeps its own stack of frames on the heap. A pair is equal only when all
// pairs below it are, so the order of the walk does not change the answer: the order is Go's.
func deepValueEqual(v1, v2 Value, visited map[visit]bool) bool {
	var stack []deepFrame
	for {
		frame, result := deepStep(v1, v2, visited)
		switch result {
		case deepDifferent:
			return false
		case deepMore:
			stack = append(stack, frame)
		}

		// The next pair: the next child of the innermost frame that still has one.
		for {
			if len(stack) == 0 {
				return true
			}
			top := &stack[len(stack)-1]
			var next int
			v1, v2, next = top.next()
			if next == deepDifferent {
				return false
			}
			if next == deepMore {
				break
			}
			stack = stack[:len(stack)-1]
		}
	}
}

// DeepEqual reports whether x and y are “deeply equal,” defined as follows. Two values of identical
// type are deeply equal if one of the following cases applies. Values of distinct types are never
// deeply equal.
//
// Array values are deeply equal when their corresponding elements are deeply equal.
//
// Struct values are deeply equal if their corresponding fields, both exported and unexported, are
// deeply equal.
//
// Func values are deeply equal if both are nil; otherwise they are not deeply equal.
//
// Interface values are deeply equal if they hold deeply equal concrete values.
//
// Map values are deeply equal when all of the following are true: they are both nil or both non-nil,
// they have the same length, and either they are the same map object or their corresponding keys
// (matched using Go equality) map to deeply equal values.
//
// Pointer values are deeply equal if they are equal using Go's == operator or if they point to
// deeply equal values.
//
// Slice values are deeply equal when all of the following are true: they are both nil or both
// non-nil, they have the same length, and either they point to the same initial entry of the same
// underlying array (that is, &x[0] == &y[0]) or their corresponding elements (up to length) are
// deeply equal. Note that a non-nil empty slice and a nil slice (for example, []byte{} and
// []byte(nil)) are not deeply equal.
//
// Other values - numbers, bools, strings, and channels - are deeply equal if they are equal using
// Go's == operator.
//
// In general DeepEqual is a recursive relaxation of Go's == operator. However, this idea is
// impossible to implement without some inconsistency. Specifically, it is possible for a value to be
// unequal to itself, either because it is of func type (uncomparable in general) or because it is a
// floating-point NaN value (not equal to itself in floating-point comparison), or because it is an
// array, struct, or interface containing such a value. On the other hand, pointer values are always
// equal to themselves, even if they point at or contain such problematic values, because they
// compare equal using Go's == operator, and that is a sufficient condition to be deeply equal,
// regardless of content. DeepEqual has been defined so that the same short-cut applies to slices and
// maps: if x and y are the same slice or the same map, they are deeply equal regardless of content.
//
// As DeepEqual traverses the data values it may find a cycle. The second and subsequent times that
// DeepEqual compares two pointer values that have been compared before, it treats the values as
// equal rather than examining the values to which they point. This ensures that DeepEqual
// terminates.
func DeepEqual(x, y any) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1 := ValueOf(x)
	v2 := ValueOf(y)
	if v1.typ != v2.typ {
		return false
	}
	return deepValueEqual(v1, v2, make(map[visit]bool))
}
