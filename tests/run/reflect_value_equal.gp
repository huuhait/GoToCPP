// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Pt struct{ X, Y int }
type W struct {
	P Pt
	I any
}

func main() {
	v := reflect.ValueOf
	fmt.Println(v(1).Equal(v(1)), v(1).Equal(v(2)), v(1).Equal(v(int64(1))), v("a").Equal(v("a")), v(1.5).Equal(v(1.5)), v(true).Equal(v(true)))
	fmt.Println(v(Pt{1, 2}).Equal(v(Pt{1, 2})), v(Pt{1, 2}).Equal(v(Pt{2, 1})), v([2]int{1, 2}).Equal(v([2]int{1, 2})), v([2]int{1, 2}).Equal(v([2]int{1, 3})))
	x := 1
	fmt.Println(v(&x).Equal(v(&x)), v(&x).Equal(v(new(int))), v((*int)(nil)).Equal(v((*int)(nil))))
	fmt.Println(v(W{Pt{1, 1}, "a"}).Equal(v(W{Pt{1, 1}, "a"})), v(W{Pt{1, 1}, "a"}).Equal(v(W{Pt{1, 1}, "b"})), v(W{I: 1}).Equal(v(W{I: int8(1)})))
	ch := make(chan int)
	fmt.Println(v(ch).Equal(v(ch)), v(ch).Equal(v(make(chan int))))
	fmt.Println(reflect.Value{}.Equal(reflect.Value{}), reflect.Value{}.Equal(v(1)), v(1).Equal(reflect.Value{}))
	var i1, i2 any = 1, 1
	h := struct{ A, B any }{i1, i2}
	hv := v(&h).Elem()
	fmt.Println(hv.Field(0).Equal(hv.Field(1)), hv.Field(0).Equal(v(1)), hv.Field(0).Equal(v("x")))
	for _, f := range []func(){
		func() { v([]int{1}).Equal(v([]int{1})) }, func() { v(map[int]int{}).Equal(v(map[int]int{})) },
		func() { v(func() {}).Equal(v(func() {})) }, func() { v(W{I: []int{}}).Equal(v(W{I: []int{}})) },
		func() { v([1][]int{}).Equal(v([1][]int{})) },
	} {
		func() {
			defer func() { fmt.Println("recovered:", recover()) }()
			f()
		}()
	}
}

// Output:
// true false false true true true
// true false true false
// true false true
// true false false
// true false
// true false false
// true true false
// recovered: reflect.Value.Equal: values of type []int are not comparable
// recovered: reflect.Value.Equal: values of type map[int]int are not comparable
// recovered: reflect.Value.Equal: values of type func() are not comparable
// recovered: reflect.Value.Equal: values of type []int are not comparable
// recovered: reflect.Value.Equal: values of type []int are not comparable
