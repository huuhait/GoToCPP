// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type Stringer interface{ String() string }
type N int

func (n N) String() string { return fmt.Sprint("N", int(n)) }

type Holder struct {
	S Stringer
	A any
	E error
}

func main() {
	var h Holder
	v := reflect.ValueOf(&h).Elem()
	v.Field(0).Set(reflect.ValueOf(N(5)))
	v.Field(1).Set(reflect.ValueOf([]int{1}))
	v.Field(2).Set(reflect.ValueOf(fmt.Errorf("e%d", 1)))
	fmt.Println(h.S, h.A, h.E)
	v.Field(1).Set(v.Field(0))
	fmt.Printf("%T %v\n", h.A, h.A)
	v.Field(0).Set(reflect.Zero(v.Field(0).Type()))
	fmt.Println(h.S == nil, v.Field(0).IsNil())
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		v.Field(0).Set(reflect.ValueOf(3))
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		v.Field(2).Set(reflect.ValueOf(N(1)))
	}()
}

// Output:
// N5 [1] e1
// main.N N5
// true true
// recovered: reflect.Set: value of type int is not assignable to type main.Stringer
// recovered: reflect.Set: value of type main.N is not assignable to type error
