// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	s := make([]int, 2, 10)
	a := [4]string{}
	m := map[int]int{1: 1, 2: 2, 3: 3}
	ch := make(chan int, 5)
	ch <- 1
	ch <- 2
	str := "héllo"
	pa := &[3]int{}
	for _, x := range []any{s, a, m, ch, str, pa, []int(nil), map[int]int(nil), (chan int)(nil)} {
		v := reflect.ValueOf(x)
		switch v.Kind() {
		case reflect.Map, reflect.String:
			fmt.Println(v.Kind(), v.Len())
		default:
			fmt.Println(v.Kind(), v.Len(), v.Cap())
		}
	}
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1).Len()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(&s).Len()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(m).Cap()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(&s).Cap()
	}()
}

// Output:
// slice 2 10
// array 4 4
// map 3
// chan 2 5
// string 6
// ptr 3 3
// slice 0 0
// map 0
// chan 0 0
// recovered: reflect: call of reflect.Value.Len on int Value
// recovered: reflect: call of reflect.Value.Len on ptr to non-array Value
// recovered: reflect: call of reflect.Value.Cap on map Value
// recovered: reflect: call of reflect.Value.Cap on ptr to non-array Value
