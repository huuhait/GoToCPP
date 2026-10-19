// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type A struct{ X, Y int }
type B struct{ X, Y int }
type Shape interface{ Area() int }
type Sq struct{ S int }

func (s Sq) Area() int { return s.S * s.S }

type Fn func(int) int
type Ptr *A

func main() {
	a := A{1, 2}
	b := reflect.ValueOf(a).Convert(reflect.TypeOf(B{}))
	fmt.Println(b.Type(), b.Interface(), b.CanSet())
	anon := reflect.ValueOf(a).Convert(reflect.TypeOf(struct{ X, Y int }{}))
	fmt.Println(anon.Type(), anon.Interface())
	sh := reflect.ValueOf(Sq{3}).Convert(reflect.TypeOf((*Shape)(nil)).Elem())
	fmt.Println(sh.Kind(), sh.Type(), sh.Elem().Type(), sh.Interface().(Shape).Area())
	an := reflect.ValueOf(Sq{2}).Convert(reflect.TypeOf((*any)(nil)).Elem())
	fmt.Println(an.Kind(), an.Elem().Type())
	pa := reflect.ValueOf(&a).Convert(reflect.TypeOf((*B)(nil)))
	fmt.Println(pa.Type(), pa.Elem().Field(0))
	pa.Elem().Field(0).SetInt(50)
	fmt.Println(a)
	f := reflect.ValueOf(func(int) int { return 1 }).Convert(reflect.TypeOf(Fn(nil)))
	fmt.Println(f.Type(), f.IsNil())
	fmt.Println(reflect.TypeOf(a).ConvertibleTo(reflect.TypeOf(B{})), reflect.TypeOf(a).ConvertibleTo(reflect.TypeOf(1)), reflect.TypeOf(&a).ConvertibleTo(reflect.TypeOf(Ptr(nil))))
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		reflect.ValueOf(1).Convert(reflect.TypeOf((*Shape)(nil)).Elem())
	}()
}

// Output:
// main.B {1 2} false
// struct { X int; Y int } {1 2}
// interface main.Shape main.Sq 9
// interface main.Sq
// *main.B 1
// {50 2}
// main.Fn false
// true false true
// recovered: reflect.Value.Convert: value of type int cannot be converted to type main.Shape
