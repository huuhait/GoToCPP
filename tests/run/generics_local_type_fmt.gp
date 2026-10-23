// phase: 07

// fmt prints a generic type declared inside a function with every verb: struct fields, pointers, arrays, maps, and the
// function's type parameter inside the type.
package main

import "fmt"

func Fmt[T any](x T) {
	type Box[U any] struct {
		U  U
		T  T
		hi []U
		m  map[string]T
	}
	b := Box[int]{1, x, []int{1, 2}, map[string]T{"a": x}}
	fmt.Printf("%v|%+v|%#v|%T\n", b, b, b, b)
	pb := &b
	fmt.Printf("%v %+v\n", pb.hi, pb.m)
	var arr [2]Box[bool]
	fmt.Println(len(arr), arr[0].hi == nil)
}

func main() {
	Fmt("s")
	Fmt(3)
}

// Output:
// {1 s [1 2] map[a:s]}|{U:1 T:s hi:[1 2] m:map[a:s]}|main.Box[string;int]{U:1, T:"s", hi:[]int{1, 2}, m:map[string]string{"a":"s"}}|main.Box[string;int]
// [1 2] map[a:s]
// 2 true
// {1 3 [1 2] map[a:3]}|{U:1 T:3 hi:[1 2] m:map[a:3]}|main.Box[int;int]{U:1, T:3, hi:[]int{1, 2}, m:map[string]int{"a":3}}|main.Box[int;int]
// [1 2] map[a:3]
// 2 true
