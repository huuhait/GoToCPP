// phase: 09

package main

import (
	"fmt"
	"reflect"
)

type MyInt int
type IDs []int
type Name string
type Pt struct{ X, Y int }
type Pt2 struct{ X, Y int }
type Str interface{ String() string }
type S string

func (s S) String() string { return string(s) }

func main() {
	ty := func(v any) reflect.Type { return reflect.TypeOf(v) }
	pairs := [][2]reflect.Type{
		{ty(1), ty(1)}, {ty(1), ty(int64(1))}, {ty(1), ty(MyInt(1))}, {ty(MyInt(1)), ty(1)},
		{ty(IDs{}), ty([]int{})}, {ty([]int{}), ty(IDs{})}, {ty(IDs{}), ty([]int64{})},
		{ty("s"), ty(Name(""))}, {ty(Name("")), ty("")},
		{ty(Pt{}), ty(Pt2{})}, {ty(Pt{}), ty(struct{ X, Y int }{})}, {ty(struct{ X, Y int }{}), ty(Pt{})},
		{ty(S("")), reflect.TypeOf((*Str)(nil)).Elem()}, {ty(1), reflect.TypeOf((*Str)(nil)).Elem()},
		{ty(1), reflect.TypeOf((*any)(nil)).Elem()}, {reflect.TypeOf((*Str)(nil)).Elem(), reflect.TypeOf((*any)(nil)).Elem()},
		{ty(make(chan int)), ty(make(<-chan int))}, {ty(make(<-chan int)), ty(make(chan int))},
		{ty(1), ty(1.5)}, {ty(1.5), ty(1)}, {ty(1), ty("x")}, {ty([]byte{}), ty("x")}, {ty("x"), ty([]byte{})}, {ty("x"), ty([]rune{})},
		{ty(65), ty("x")}, {ty(uint8(1)), ty(int8(1))}, {ty(complex64(1)), ty(complex128(1))}, {ty(1), ty(complex128(1))},
		{ty(&Pt{}), ty(&Pt2{})}, {ty([]int{}), ty([2]int{})}, {ty(Pt{}), ty(&Pt{})},
	}
	for _, p := range pairs {
		fmt.Printf("%-24v -> %-24v assignable=%-5v convertible=%v\n", p[0], p[1], p[0].AssignableTo(p[1]), p[0].ConvertibleTo(p[1]))
	}
}

// Output:
// int                      -> int                      assignable=true  convertible=true
// int                      -> int64                    assignable=false convertible=true
// int                      -> main.MyInt               assignable=false convertible=true
// main.MyInt               -> int                      assignable=false convertible=true
// main.IDs                 -> []int                    assignable=true  convertible=true
// []int                    -> main.IDs                 assignable=true  convertible=true
// main.IDs                 -> []int64                  assignable=false convertible=false
// string                   -> main.Name                assignable=false convertible=true
// main.Name                -> string                   assignable=false convertible=true
// main.Pt                  -> main.Pt2                 assignable=false convertible=true
// main.Pt                  -> struct { X int; Y int }  assignable=true  convertible=true
// struct { X int; Y int }  -> main.Pt                  assignable=true  convertible=true
// main.S                   -> main.Str                 assignable=true  convertible=true
// int                      -> main.Str                 assignable=false convertible=false
// int                      -> interface {}             assignable=true  convertible=true
// main.Str                 -> interface {}             assignable=true  convertible=true
// chan int                 -> <-chan int               assignable=true  convertible=true
// <-chan int               -> chan int                 assignable=false convertible=false
// int                      -> float64                  assignable=false convertible=true
// float64                  -> int                      assignable=false convertible=true
// int                      -> string                   assignable=false convertible=true
// []uint8                  -> string                   assignable=false convertible=true
// string                   -> []uint8                  assignable=false convertible=true
// string                   -> []int32                  assignable=false convertible=true
// int                      -> string                   assignable=false convertible=true
// uint8                    -> int8                     assignable=false convertible=true
// complex64                -> complex128               assignable=false convertible=true
// int                      -> complex128               assignable=false convertible=false
// *main.Pt                 -> *main.Pt2                assignable=false convertible=true
// []int                    -> [2]int                   assignable=false convertible=true
// main.Pt                  -> *main.Pt                 assignable=false convertible=false
