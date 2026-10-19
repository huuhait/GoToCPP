// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Msg struct{ N int }

func main() {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	v := reflect.ValueOf(ch)
	fmt.Println(v.Kind(), v.Type(), v.Len(), v.Cap(), v.IsNil(), v.Type().ChanDir(), v.Type().Elem())
	var nilch chan string
	nv := reflect.ValueOf(nilch)
	fmt.Println(nv.Kind(), nv.Len(), nv.Cap(), nv.IsNil())
	ro := make(<-chan Msg)
	fmt.Println(reflect.TypeOf(ro), reflect.TypeOf(ro).ChanDir(), reflect.ValueOf(ro).Len())
	fmt.Println(v.Interface().(chan int) == ch, v.Pointer() == reflect.ValueOf(ch).Pointer())
	type Holder struct{ C chan int }
	h := Holder{}
	reflect.ValueOf(&h).Elem().Field(0).Set(v)
	fmt.Println(h.C == ch, len(h.C))
	z := reflect.Zero(reflect.TypeOf(ch))
	fmt.Println(z.IsNil(), z.Kind())
	fmt.Println(reflect.ValueOf(ro).Type().Elem().Field(0).Name)
}

// Output:
// chan chan int 2 3 false chan int
// chan 0 0 true
// <-chan main.Msg <-chan 0
// true true
// true 2
// true chan
// N
