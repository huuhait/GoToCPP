// phase: 09

package main

import (
	"fmt"
	"reflect"
)

func main() {
	fmt.Println(reflect.TypeOf(nil) == nil)
	var e error
	fmt.Println(reflect.TypeOf(e) == nil)
	var p *int
	fmt.Println(reflect.TypeOf(p), reflect.TypeOf(p) == nil)
	var s []int
	fmt.Println(reflect.TypeOf(s), reflect.ValueOf(s).IsNil())
	v := reflect.ValueOf(nil)
	fmt.Println(v.IsValid(), v.Kind(), v)
	fmt.Println(reflect.ValueOf(e).IsValid())
	fmt.Println(reflect.ValueOf(p).IsValid(), reflect.ValueOf(p).IsNil())
}

// Output:
// true
// true
// *int false
// []int true
// false invalid <invalid reflect.Value>
// false
// true true
