// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type K struct{ A, B int }

func main() {
	m := map[string]int{"a": 1}
	v := reflect.ValueOf(m)
	v.SetMapIndex(reflect.ValueOf("b"), reflect.ValueOf(2))
	v.SetMapIndex(reflect.ValueOf("a"), reflect.ValueOf(10))
	fmt.Println(m)
	v.SetMapIndex(reflect.ValueOf("a"), reflect.Value{})
	v.SetMapIndex(reflect.ValueOf("nope"), reflect.Value{})
	fmt.Println(m, len(m))
	sm := map[K][]int{}
	sv := reflect.ValueOf(sm)
	sv.SetMapIndex(reflect.ValueOf(K{1, 2}), reflect.ValueOf([]int{1}))
	sv.SetMapIndex(reflect.ValueOf(K{1, 2}), reflect.ValueOf([]int{1, 2}))
	fmt.Println(sm)
	im := map[any]any{}
	iv := reflect.ValueOf(im)
	iv.SetMapIndex(reflect.ValueOf(1), reflect.ValueOf("one"))
	iv.SetMapIndex(reflect.ValueOf("k"), reflect.ValueOf(2.5))
	fmt.Println(len(im), im[1], im["k"])
	mp := map[string]*int{}
	n := 5
	reflect.ValueOf(mp).SetMapIndex(reflect.ValueOf("p"), reflect.ValueOf(&n))
	*mp["p"] = 6
	fmt.Println(n)
	em := map[string]error{}
	reflect.ValueOf(em).SetMapIndex(reflect.ValueOf("e"), reflect.ValueOf(fmt.Errorf("x")))
	fmt.Println(em)
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(m).SetMapIndex(reflect.ValueOf(1), reflect.ValueOf(1))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(m).SetMapIndex(reflect.ValueOf("x"), reflect.ValueOf("s"))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		var nm map[string]int
		reflect.ValueOf(nm).SetMapIndex(reflect.ValueOf("x"), reflect.ValueOf(1))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		var nm map[string]int
		reflect.ValueOf(nm).SetMapIndex(reflect.ValueOf("x"), reflect.Value{})
		fmt.Println("delete on nil map ok")
	}()
}

// Output:
// map[a:10 b:2]
// map[b:2] 1
// map[{1 2}:[1 2]]
// 2 one 2.5
// 6
// map[e:x]
// recovered: reflect.Value.SetMapIndex: value of type int is not assignable to type string
// recovered: reflect.Value.SetMapIndex: value of type string is not assignable to type int
// recovered: assignment to entry in nil map
// delete on nil map ok
// recovered: <nil>
