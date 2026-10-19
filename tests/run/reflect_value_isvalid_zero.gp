// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	var z reflect.Value
	fmt.Println(z.IsValid(), z.Kind(), z.String())
	fmt.Println(reflect.ValueOf(0).IsValid(), reflect.Zero(reflect.TypeOf(0)).IsValid())
	fmt.Println(reflect.ValueOf(map[string]int{}).MapIndex(reflect.ValueOf("x")).IsValid())
	var p *struct{ A int }
	fmt.Println(reflect.ValueOf(p).Elem().IsValid())
	var i any
	fmt.Println(reflect.ValueOf(&i).Elem().Elem().IsValid())
	fmt.Println(reflect.ValueOf(struct{ A int }{}).FieldByName("B").IsValid())
	fmt.Println(reflect.Zero(reflect.TypeOf("")).String() == "")
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		z.Type()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		z.Interface()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		z.Int()
	}()
	func() {
		defer func() { fmt.Println("recovered:", recover()) }()
		z.CanInterface()
	}()
	fmt.Println(z.CanAddr(), z.CanSet())
}

// Output:
// false invalid <invalid Value>
// true true
// false
// false
// false
// false
// true
// recovered: reflect: call of reflect.Value.Type on zero Value
// recovered: reflect: call of reflect.Value.Interface on zero Value
// recovered: reflect: call of reflect.Value.Int on zero Value
// recovered: reflect: call of reflect.Value.CanInterface on zero Value
// false false
