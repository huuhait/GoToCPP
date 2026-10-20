// phase: 09
// encoding/json Marshal of the basic kinds: scalars, strings, slices, arrays, maps, pointers, interfaces, nil values, nested values.

package main

import (
	"encoding/json"
	"fmt"
)

type Point struct {
	X, Y int
}

type Inner struct {
	A string
	B []int
}

type Outer struct {
	Name  string
	In    Inner
	Ptr   *Inner
	Any   any
	M     map[string]int
	Arr   [3]bool
	Bytes []byte
}

type MyInt int
type MyStr string
type MySlice []string

func show(label string, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Printf("%s: error: %v\n", label, err)
		return
	}
	fmt.Printf("%s: %s\n", label, b)
}

func main() {
	show("nil", nil)
	show("true", true)
	show("false", false)
	show("int", 42)
	show("negative", -17)
	show("int8", int8(-128))
	show("uint8", uint8(255))
	show("int64", int64(-9223372036854775808))
	show("uint64", uint64(18446744073709551615))
	show("uintptr", uintptr(7))
	show("float", 3.5)
	show("float32", float32(0.1))
	show("string", "hello")
	show("empty string", "")
	show("slice", []int{1, 2, 3})
	show("nil slice", []int(nil))
	show("empty slice", []int{})
	show("array", [3]string{"a", "b", "c"})
	show("empty array", [0]int{})
	show("bytes", []byte("hello, world"))
	show("nil bytes", []byte(nil))
	show("byte array", [3]byte{1, 2, 3})
	show("map", map[string]int{"b": 2, "a": 1, "c": 3})
	show("nil map", map[string]int(nil))
	show("empty map", map[string]int{})
	show("struct", Point{1, 2})
	show("ptr struct", &Point{3, 4})
	var np *Point
	show("nil ptr", np)
	pp := &np
	show("ptr ptr nil", pp)
	n := 5
	pn := &n
	show("ptr int", pn)
	show("named int", MyInt(9))
	show("named string", MyStr("x"))
	show("named slice", MySlice{"a", "b"})
	show("slice of any", []any{1, "two", 3.0, nil, true, []int{1}, map[string]any{"k": "v"}})
	show("slice of struct", []Point{{1, 2}, {3, 4}})
	show("slice of ptr", []*Point{{1, 2}, nil})
	show("map of slice", map[string][]int{"x": {1, 2}, "y": nil})
	show("map of map", map[string]map[string]int{"o": {"i": 1}})
	show("outer zero", Outer{})
	show("outer", Outer{
		Name:  "n",
		In:    Inner{"a", []int{1}},
		Ptr:   &Inner{"p", nil},
		Any:   []string{"s"},
		M:     map[string]int{"z": 26},
		Arr:   [3]bool{true, false, true},
		Bytes: []byte{0, 255},
	})
	var e any = Point{5, 6}
	show("interface holding struct", e)
	show("struct with interface", struct{ V any }{Point{7, 8}})
	show("anonymous struct", struct {
		A int
		b int
	}{1, 2})
	show("empty struct", struct{}{})
	show("nested slices", [][]int{{1}, {2, 3}, {}})
	show("array of array", [2][2]int{{1, 2}, {3, 4}})
}

// Output:
// nil: null
// true: true
// false: false
// int: 42
// negative: -17
// int8: -128
// uint8: 255
// int64: -9223372036854775808
// uint64: 18446744073709551615
// uintptr: 7
// float: 3.5
// float32: 0.1
// string: "hello"
// empty string: ""
// slice: [1,2,3]
// nil slice: null
// empty slice: []
// array: ["a","b","c"]
// empty array: []
// bytes: "aGVsbG8sIHdvcmxk"
// nil bytes: null
// byte array: [1,2,3]
// map: {"a":1,"b":2,"c":3}
// nil map: null
// empty map: {}
// struct: {"X":1,"Y":2}
// ptr struct: {"X":3,"Y":4}
// nil ptr: null
// ptr ptr nil: null
// ptr int: 5
// named int: 9
// named string: "x"
// named slice: ["a","b"]
// slice of any: [1,"two",3,null,true,[1],{"k":"v"}]
// slice of struct: [{"X":1,"Y":2},{"X":3,"Y":4}]
// slice of ptr: [{"X":1,"Y":2},null]
// map of slice: {"x":[1,2],"y":null}
// map of map: {"o":{"i":1}}
// outer zero: {"Name":"","In":{"A":"","B":null},"Ptr":null,"Any":null,"M":null,"Arr":[false,false,false],"Bytes":null}
// outer: {"Name":"n","In":{"A":"a","B":[1]},"Ptr":{"A":"p","B":null},"Any":["s"],"M":{"z":26},"Arr":[true,false,true],"Bytes":"AP8="}
// interface holding struct: {"X":5,"Y":6}
// struct with interface: {"V":{"X":7,"Y":8}}
// anonymous struct: {"A":1}
// empty struct: {}
// nested slices: [[1],[2,3],[]]
// array of array: [[1,2],[3,4]]
