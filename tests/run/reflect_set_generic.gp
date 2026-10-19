// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }
type Shape interface{ Area() int }
type Rect struct{ W, H int }

func (r Rect) Area() int { return r.W * r.H }

type MyInts []int

func main() {
	var i int
	var s string
	var p Pt
	var sh Shape
	var a any
	var e error
	var ints []int
	var named MyInts
	var m map[string]int
	reflect.ValueOf(&i).Elem().Set(reflect.ValueOf(42))
	reflect.ValueOf(&s).Elem().Set(reflect.ValueOf("str"))
	reflect.ValueOf(&p).Elem().Set(reflect.ValueOf(Pt{1, 2}))
	reflect.ValueOf(&sh).Elem().Set(reflect.ValueOf(Rect{2, 3}))
	reflect.ValueOf(&a).Elem().Set(reflect.ValueOf(3.5))
	reflect.ValueOf(&e).Elem().Set(reflect.ValueOf(fmt.Errorf("err")))
	reflect.ValueOf(&ints).Elem().Set(reflect.ValueOf([]int{1, 2}))
	reflect.ValueOf(&named).Elem().Set(reflect.ValueOf([]int{3, 4}))
	reflect.ValueOf(&m).Elem().Set(reflect.ValueOf(map[string]int{"k": 1}))
	fmt.Println(i, s, p, sh.Area(), a, e, ints, named, m)
	reflect.ValueOf(&a).Elem().Set(reflect.ValueOf(&i))
	fmt.Println(*(a.(*int)))
	reflect.ValueOf(&sh).Elem().Set(reflect.Zero(reflect.TypeOf(&sh).Elem()))
	fmt.Println(sh == nil)
	for _, f := range []func(){
		func() { reflect.ValueOf(&i).Elem().Set(reflect.ValueOf("str")) },
		func() { reflect.ValueOf(&i).Elem().Set(reflect.ValueOf(int64(1))) },
		func() { reflect.ValueOf(&sh).Elem().Set(reflect.ValueOf(Pt{})) },
		func() { reflect.ValueOf(&ints).Elem().Set(reflect.ValueOf([]int64{1})) },
		func() { reflect.ValueOf(i).Set(reflect.ValueOf(1)) },
		func() { reflect.ValueOf(&i).Elem().Set(reflect.Value{}) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// 42 str {1 2} 6 3.5 err [1 2] [3 4] map[k:1]
// 42
// true
// recovered: reflect.Set: value of type string is not assignable to type int
// recovered: reflect.Set: value of type int64 is not assignable to type int
// recovered: reflect.Set: value of type main.Pt is not assignable to type main.Shape
// recovered: reflect.Set: value of type []int64 is not assignable to type []int
// recovered: reflect: reflect.Value.Set using unaddressable value
// recovered: reflect: call of reflect.Value.Set on zero Value
