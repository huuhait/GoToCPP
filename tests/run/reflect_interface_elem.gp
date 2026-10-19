// phase: 09

package main

import (
	"errors"
	"fmt"
	"reflect"
)

type Box struct {
	Any  any
	Err  error
	Str  fmt.Stringer
	Nil  any
	List []any
}

type S string

func (s S) String() string { return "S:" + string(s) }

func main() {
	b := Box{Any: 42, Err: errors.New("bad"), Str: S("x"), List: []any{1, "two", nil, 3.5, []int{4}}}
	v := reflect.ValueOf(b)
	for i := 0; i < v.NumField()-1; i++ {
		f := v.Field(i)
		fmt.Println(v.Type().Field(i).Name, f.Kind(), f.Type(), f.IsNil())
		if !f.IsNil() {
			e := f.Elem()
			fmt.Println("  elem:", e.Kind(), e.Type(), e)
		}
	}
	l := v.Field(4)
	for i := 0; i < l.Len(); i++ {
		e := l.Index(i)
		fmt.Println(i, e.Kind(), e.IsNil(), e.Elem().IsValid())
		if e.Elem().IsValid() {
			fmt.Println("  ", e.Elem().Kind(), e.Elem().Type())
		}
	}
	pb := reflect.ValueOf(&b).Elem()
	pb.Field(0).Set(reflect.ValueOf("now a string"))
	pb.Field(3).Set(reflect.ValueOf(1.5))
	pb.Field(1).Set(reflect.ValueOf(fmt.Errorf("wrapped: %w", b.Err)))
	fmt.Println(b.Any, b.Nil, b.Err)
	pb.Field(0).Set(reflect.Zero(pb.Field(0).Type()))
	fmt.Println(b.Any == nil)
	fmt.Println(pb.Field(0).Elem().IsValid(), pb.Field(0).Elem().Kind())
}

// Output:
// Any interface interface {} false
//   elem: int int 42
// Err interface error false
//   elem: ptr *errors.errorString bad
// Str interface fmt.Stringer false
//   elem: string main.S S:x
// Nil interface interface {} true
// 0 interface false true
//    int int
// 1 interface false true
//    string string
// 2 interface true false
// 3 interface false true
//    float64 float64
// 4 interface false true
//    slice []int
// now a string 1.5 wrapped: bad
// true
// false invalid
