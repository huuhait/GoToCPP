// Copyright 2026 The gocpp Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package reflectlite

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"unsafe"
)

// The stub's contract (job 16.1f-stubs) is Go's behavior for the cases the
// closure exercises: what sort's Slice/SliceStable/SliceIsSorted pass to
// ValueOf/Swapper/Value.Len, and what errors' Is/As pass to TypeOf/ValueOf and
// the Type and Value methods. The reference is the real reflect package of the
// same Go toolchain, applied to the same values.
//
// The tests run against the real Go runtime, so the type descriptors they see
// are the real internal/abi.Type ones: passing them proves the restated
// layouts of abi.go and the seven runtime hooks of hooks.go.

type namedInt int
type namedStruct struct {
	A int
	B string
}

var (
	nilAny    any
	nilPtr    *int
	nilSlice  []int
	nilMap    map[string]int
	nilChan   chan int
	nilFunc   func()
	oneInt        = 1
	intVal        = 42
	stringVal     = "hello"
	sliceVal      = []int{1, 2, 3}
	strSlice      = []string{"a", "b"}
	bigSlice      = []namedStruct{{1, "a"}, {2, "b"}}
	ptrSlice      = []*int{&oneInt, &intVal}
	mapVal        = map[string]int{"a": 1, "b": 2}
	chanVal       = make(chan int, 4)
	ptrVal        = &intVal
	ifaceVal  any = "boxed"
)

func TestValueOfKindAndType(t *testing.T) {
	values := []any{
		true, int(1), int8(1), int16(1), int32(1), int64(1),
		uint(1), uint8(1), uint16(1), uint32(1), uint64(1), uintptr(1),
		float32(1), float64(1), complex64(1), complex128(1),
		"", "x", [3]int{}, []int{}, map[string]int{}, make(chan int), func() {},
		(*int)(nil), unsafePointer(), namedInt(3), namedStruct{}, struct{ X int }{},
		[]string{}, [][]int{}, (*namedStruct)(nil),
	}
	for _, v := range values {
		got := ValueOf(v)
		want := reflect.ValueOf(v)
		if int(got.Kind()) != int(want.Kind()) {
			t.Errorf("ValueOf(%T).Kind() = %v, want %v", v, got.Kind(), want.Kind())
		}
		if !got.IsValid() {
			t.Errorf("ValueOf(%T).IsValid() = false", v)
		}
		gt := got.Type()
		wt := reflect.TypeOf(v)
		if gt.Size() != wt.Size() {
			t.Errorf("ValueOf(%T).Type().Size() = %d, want %d", v, gt.Size(), wt.Size())
		}
		if gt.Kind() != got.Kind() {
			t.Errorf("ValueOf(%T): Type().Kind() = %v, Value.Kind() = %v", v, gt.Kind(), got.Kind())
		}
		if gt.String() != wt.String() {
			t.Errorf("ValueOf(%T).Type().String() = %q, want %q", v, gt.String(), wt.String())
		}
		if gt.Name() != wt.Name() {
			t.Errorf("ValueOf(%T).Type().Name() = %q, want %q", v, gt.Name(), wt.Name())
		}
		if gt.PkgPath() != wt.PkgPath() {
			t.Errorf("ValueOf(%T).Type().PkgPath() = %q, want %q", v, gt.PkgPath(), wt.PkgPath())
		}
		if gt.Comparable() != wt.Comparable() {
			t.Errorf("ValueOf(%T).Type().Comparable() = %v, want %v", v, gt.Comparable(), wt.Comparable())
		}
	}

	if ValueOf(nil).IsValid() {
		t.Error("ValueOf(nil).IsValid() = true")
	}
	if TypeOf(nil) != nil {
		t.Error("TypeOf(nil) != nil")
	}
}

func unsafePointer() unsafe.Pointer { return nil }

func TestTypesElem(t *testing.T) {
	pairs := []struct {
		v    any
		want reflect.Type
	}{
		{(*int)(nil), reflect.TypeOf(0)},
		{([]string)(nil), reflect.TypeOf("")},
		{([3]int{}), reflect.TypeOf(0)},
		{(map[string]int{}), reflect.TypeOf(0)},
		{(make(chan string)), reflect.TypeOf("")},
	}
	for _, p := range pairs {
		got := TypeOf(p.v).Elem()
		if got == nil {
			t.Errorf("TypeOf(%T).Elem() = nil", p.v)
			continue
		}
		if got.String() != p.want.String() {
			t.Errorf("TypeOf(%T).Elem() = %v, want %v", p.v, got, p.want)
		}
	}
}

func TestLenAndIsNil(t *testing.T) {
	lengths := []any{
		[]int{}, []int{1, 2, 3}, [4]int{}, "", "hello", map[string]int{},
		map[string]int{"a": 1, "b": 2}, make(chan int), chanVal,
	}
	for _, v := range lengths {
		got, want := ValueOf(v).Len(), reflect.ValueOf(v).Len()
		if got != want {
			t.Errorf("ValueOf(%T).Len() = %d, want %d", v, got, want)
		}
	}
	nils := []any{nilPtr, nilSlice, nilMap, nilChan, nilFunc}
	for _, v := range nils {
		if got, want := ValueOf(v).IsNil(), reflect.ValueOf(v).IsNil(); got != want {
			t.Errorf("ValueOf(%T).IsNil() = %v, want %v", v, got, want)
		}
	}
	nonNils := []any{&intVal, sliceVal, mapVal, chanVal, func() {}}
	for _, v := range nonNils {
		if ValueOf(v).IsNil() {
			t.Errorf("ValueOf(%T).IsNil() = true", v)
		}
	}
}

func TestComparableAndImplements(t *testing.T) {
	errType := TypeOf((*error)(nil)).Elem()
	realErrType := reflect.TypeOf((*error)(nil)).Elem()
	values := []any{
		errors.New("x"), fmt.Errorf("x: %w", errors.New("y")),
		int(1), []int{}, map[string]int{}, namedStruct{}, &namedStruct{},
	}
	for _, v := range values {
		tv := TypeOf(v)
		if got, want := tv.Comparable(), reflect.TypeOf(v).Comparable(); got != want {
			t.Errorf("TypeOf(%T).Comparable() = %v, want %v", v, got, want)
		}
		if got, want := tv.Implements(errType), reflect.TypeOf(v).Implements(realErrType); got != want {
			t.Errorf("TypeOf(%T).Implements(error) = %v, want %v", v, got, want)
		}
	}

	// An interface type against an interface type.
	if !TypeOf((*error)(nil)).Elem().Implements(TypeOf((*any)(nil)).Elem()) {
		t.Error("error does not implement any")
	}
	if TypeOf((*any)(nil)).Elem().Implements(errType) {
		t.Error("any implements error")
	}

	// AssignableTo: same type, pointer to its interface, unrelated types.
	checks := []struct{ v, u any }{
		{int(1), int(2)},
		{int(1), ""},
		{namedInt(1), int(0)},
		{errors.New("x"), (*any)(nil)},
		{errors.New("x"), (*error)(nil)},
		{(*namedStruct)(nil), (*namedStruct)(nil)},
		{(*namedStruct)(nil), (*int)(nil)},
	}
	for _, c := range checks {
		got := TypeOf(c.v).AssignableTo(TypeOf(c.u))
		want := reflect.TypeOf(c.v).AssignableTo(reflect.TypeOf(c.u))
		if got != want {
			t.Errorf("TypeOf(%T).AssignableTo(%T) = %v, want %v", c.v, c.u, got, want)
		}
	}
}

func TestSwapper(t *testing.T) {
	cases := []struct {
		name string
		make func() any
		gone func(any)
	}{
		{"ints", func() any { return []int{1, 2, 3, 4} }, func(v any) { gotCompare(t, v, []int{1, 4, 3, 2}) }},
		{"strings", func() any { return []string{"a", "b", "c", "d"} }, func(v any) { gotCompare(t, v, []string{"a", "d", "c", "b"}) }},
		{"bytes", func() any { return []byte{1, 2, 3, 4} }, func(v any) { gotCompare(t, v, []byte{1, 4, 3, 2}) }},
		{"int16s", func() any { return []int16{1, 2, 3, 4} }, func(v any) { gotCompare(t, v, []int16{1, 4, 3, 2}) }},
		{"int64s", func() any { return []int64{1, 2, 3, 4} }, func(v any) { gotCompare(t, v, []int64{1, 4, 3, 2}) }},
		{"ptrs", func() any {
			a, b, c := 1, 2, 3
			return []*int{&a, &b, &c}
		}, func(v any) {
			got := v.([]*int)
			if *got[0] != 1 || *got[1] != 3 || *got[2] != 2 {
				t.Errorf("ptrs swapped = %v, %v, %v", *got[0], *got[1], *got[2])
			}
		}},
		{"structs", func() any { return []namedStruct{{1, "a"}, {2, "b"}, {3, "c"}} }, func(v any) {
			gotCompare(t, v, []namedStruct{{1, "a"}, {3, "c"}, {2, "b"}})
		}},
		{"ptr-structs", func() any { return []*namedStruct{{1, "a"}, {2, "b"}, {3, "c"}} }, func(v any) {
			got := v.([]*namedStruct)
			if got[1].A != 3 || got[2].A != 2 {
				t.Errorf("ptr-structs swapped = %v %v", got[1], got[2])
			}
		}},
	}
	for _, c := range cases {
		v := c.make()
		swap := Swapper(v)
		swap(1, lenOf(v)-1)
		c.gone(v)

		// The reference: the same swap with the real reflect package.
		r := c.make()
		reflect.Swapper(r)(1, lenOf(r)-1)
		if !reflect.DeepEqual(r, v) {
			t.Errorf("%s: stub result %v, real reflect result %v", c.name, v, r)
		}
	}

}

func TestSwapperPanics(t *testing.T) {
	if !didPanic(func() { Swapper([]int{})(0, 0) }) {
		t.Error("Swapper of an empty slice: index 0 did not panic")
	}
	if didPanic(func() { Swapper([]int{1})(0, 0) }) {
		t.Error("Swapper of a one-element slice: (0,0) panicked")
	}
	if didPanic(func() { Swapper([]int{1, 2})(1, 1) }) {
		t.Error("Swapper: (1,1) panicked")
	}
	if !didPanic(func() { Swapper([]int{1, 2})(0, 2) }) {
		t.Error("Swapper: out-of-range index did not panic")
	}
	if !didPanic(func() { Swapper(42) }) {
		t.Error("Swapper of an int did not panic")
	}
}

func didPanic(f func()) (panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	f()
	return false
}

func lenOf(v any) int { return reflect.ValueOf(v).Len() }

func gotCompare[T comparable](t *testing.T, got any, want []T) {
	t.Helper()
	g := got.([]T)
	if !reflect.DeepEqual(g, want) {
		t.Errorf("swapped %T = %v, want %v", got, g, want)
	}
}

func TestSet(t *testing.T) {
	// Same concrete type.
	i := 1
	ValueOf(&i).Elem().Set(ValueOf(42))
	if i != 42 {
		t.Errorf("after Set, i = %d, want 42", i)
	}

	// A concrete value into a *any target.
	var anyTarget any
	ValueOf(&anyTarget).Elem().Set(ValueOf("boxed"))
	if anyTarget != "boxed" {
		t.Errorf("anyTarget = %v, want boxed", anyTarget)
	}

	// A concrete error into a *error target (the ifaceE2I path).
	var errTarget error
	want := errors.New("boom")
	ValueOf(&errTarget).Elem().Set(ValueOf(want))
	if errTarget != want {
		t.Errorf("errTarget = %v, want %v", errTarget, want)
	}

	// Same through the real reflect package: the results must agree.
	var realErrTarget error
	reflect.ValueOf(&realErrTarget).Elem().Set(reflect.ValueOf(want))
	if realErrTarget != errTarget {
		t.Errorf("stub target %v, real target %v", errTarget, realErrTarget)
	}

	// A named type to its underlying type is not assignable.
	func() {
		defer func() {
			if recover() == nil {
				t.Error("Set of an int into a namedInt did not panic")
			}
		}()
		ni := namedInt(0)
		ValueOf(&ni).Elem().Set(ValueOf(3))
	}()
}

// ---- errors.Is / errors.As semantics, re-implemented on the stub ----
//
// errors/wrap.go is in the closure and uses exactly these operations. The test
// copies its Is/As logic on top of the stub and compares the answers with the
// real errors package over the same trees.

type wrapErr struct{ err error }

func (w *wrapErr) Error() string { return "wrap: " + w.err.Error() }
func (w *wrapErr) Unwrap() error { return w.err }

type isErr struct{ match error }

func (i *isErr) Error() string        { return "is" }
func (i *isErr) Is(target error) bool { return target == i.match }
func (i *isErr) Unwrap() error        { return i.match }

type asErr struct{}

func (*asErr) Error() string { return "as" }

func TestErrorsIsAs(t *testing.T) {
	base := errors.New("base")
	we := &wrapErr{base}
	ie := &isErr{base}
	ae := &asErr{}

	tree := fmt.Errorf("top: %w", we)
	multi := errors.Join(base, ae)

	isCases := []struct{ err, target error }{
		{tree, base},
		{tree, we},
		{tree, tree},
		{ie, base},
		{multi, ae},
		{base, ae},
	}
	for _, c := range isCases {
		stub := stubIs(c.err, c.target)
		want := errors.Is(c.err, c.target)
		if stub != want {
			t.Errorf("stub errors.Is(%v, %v) = %v, real = %v", c.err, c.target, stub, want)
		}
	}

	asCases := []struct {
		err    error
		target any
	}{
		{tree, new(*asErr)},
		{tree, new(error)},
		{tree, new(any)},
		{multi, new(*asErr)},
		{base, new(*asErr)},
	}
	for _, c := range asCases {
		stubTarget := c.target
		realTarget := reflect.New(reflect.TypeOf(c.target).Elem()).Interface()
		stubOK := stubAs(c.err, stubTarget)
		realOK := errors.As(c.err, realTarget)
		if stubOK != realOK {
			t.Errorf("stub errors.As(%v, %T) = %v, real = %v", c.err, c.target, stubOK, realOK)
			continue
		}
		if stubOK && !reflect.DeepEqual(reflect.ValueOf(stubTarget).Elem().Interface(), reflect.ValueOf(realTarget).Elem().Interface()) {
			t.Errorf("errors.As(%v, %T): stub set %v, real set %v", c.err, c.target,
				reflect.ValueOf(stubTarget).Elem().Interface(), reflect.ValueOf(realTarget).Elem().Interface())
		}
	}
}

var stubErrorType = TypeOf((*error)(nil)).Elem()

func stubIs(err, target error) bool {
	if err == nil || target == nil {
		return err == target
	}
	isComparable := TypeOf(target).Comparable()
	return stubIsWalk(err, target, isComparable)
}

func stubIsWalk(err, target error, targetComparable bool) bool {
	for {
		if targetComparable && err == target {
			return true
		}
		if x, ok := err.(interface{ Is(error) bool }); ok && x.Is(target) {
			return true
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return false
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if stubIsWalk(err, target, targetComparable) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
}

func stubAs(err error, target any) bool {
	if err == nil {
		return false
	}
	if target == nil {
		panic("errors: target cannot be nil")
	}
	val := ValueOf(target)
	typ := val.Type()
	if typ.Kind() != Ptr || val.IsNil() {
		panic("errors: target must be a non-nil pointer")
	}
	targetType := typ.Elem()
	if targetType.Kind() != Interface && !targetType.Implements(stubErrorType) {
		panic("errors: *target must be interface or implement error")
	}
	return stubAsWalk(err, target, val, targetType)
}

func stubAsWalk(err error, target any, targetVal Value, targetType Type) bool {
	for {
		if TypeOf(err).AssignableTo(targetType) {
			targetVal.Elem().Set(ValueOf(err))
			return true
		}
		if x, ok := err.(interface{ As(any) bool }); ok && x.As(target) {
			return true
		}
		switch x := err.(type) {
		case interface{ Unwrap() error }:
			err = x.Unwrap()
			if err == nil {
				return false
			}
		case interface{ Unwrap() []error }:
			for _, err := range x.Unwrap() {
				if err == nil {
					continue
				}
				if stubAsWalk(err, target, targetVal, targetType) {
					return true
				}
			}
			return false
		default:
			return false
		}
	}
}
