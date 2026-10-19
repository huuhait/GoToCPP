// phase: 09

package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

type I interface{ M() int }
type A struct{ N int }

func (a A) M() int { return a.N }

type PA struct{ N int }

func (a *PA) M() int { return a.N }

func main() {
	var i1, i2 I = A{1}, A{1}
	fmt.Println(reflect.DeepEqual(i1, i2), reflect.DeepEqual(i1, A{1}), reflect.DeepEqual([]I{A{1}}, []I{A{1}}), reflect.DeepEqual([]I{A{1}}, []I{A{2}}))
	var p1, p2 I = &PA{1}, &PA{1}
	fmt.Println(reflect.DeepEqual(p1, p2), reflect.DeepEqual(p1, A{1}), reflect.DeepEqual([]I{&PA{1}}, []I{&PA{1}}))
	x := 1
	u1, u2 := unsafe.Pointer(&x), unsafe.Pointer(&x)
	fmt.Println(reflect.DeepEqual(u1, u2), reflect.DeepEqual(u1, unsafe.Pointer(new(int))), reflect.DeepEqual(unsafe.Pointer(nil), unsafe.Pointer(nil)))
	fmt.Println(reflect.DeepEqual(struct{ I I }{A{1}}, struct{ I I }{A{1}}), reflect.DeepEqual(struct{ I I }{}, struct{ I I }{}), reflect.DeepEqual(struct{ I I }{}, struct{ I I }{A{1}}))
	fmt.Println(reflect.DeepEqual(map[string]I{"a": A{1}}, map[string]I{"a": A{1}}), reflect.DeepEqual(map[string]any{"a": []int{1}}, map[string]any{"a": []int{1}}))
	fmt.Println(reflect.DeepEqual(reflect.TypeOf(1), reflect.TypeOf(1)), reflect.DeepEqual(reflect.TypeOf(1), reflect.TypeOf("")))
}

// Output:
// true true true false
// true false true
// true false true
// true true false
// true true
// true false
