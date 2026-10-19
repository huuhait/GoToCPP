// phase: 09

package main

import (
	"fmt"
	"reflect"
	"unsafe"
)

type I interface{ M() }
type S struct{ A int }

func main() {
	var (
		ch   chan int
		fn   func()
		m    map[string]int
		p    *int
		sl   []int
		up   unsafe.Pointer
		e    error
		any_ any
	)
	holder := struct {
		E error
		A any
		I I
	}{}
	vals := []any{ch, fn, m, p, sl, up, make(chan int), func() {}, map[int]int{}, new(int), []int{}, unsafe.Pointer(new(int)), (*S)(nil)}
	for _, v := range vals {
		rv := reflect.ValueOf(v)
		fmt.Println(rv.Kind(), rv.IsNil())
	}
	_, _ = e, any_
	h := reflect.ValueOf(holder)
	for i := 0; i < h.NumField(); i++ {
		fmt.Println(h.Field(i).Kind(), h.Field(i).IsNil())
	}
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1).IsNil()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(S{}).IsNil()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.Value{}.IsNil()
	}()
}

// Output:
// chan true
// func true
// map true
// ptr true
// slice true
// unsafe.Pointer true
// chan false
// func false
// map false
// ptr false
// slice false
// unsafe.Pointer false
// ptr true
// interface true
// interface true
// interface true
// recovered: reflect: call of reflect.Value.IsNil on int Value
// recovered: reflect: call of reflect.Value.IsNil on struct Value
// recovered: reflect: call of reflect.Value.IsNil on zero Value
